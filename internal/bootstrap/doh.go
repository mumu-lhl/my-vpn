package bootstrap

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"time"
)

type DoHEndpoint struct {
	URL        string   `json:"url"`
	StaticIPs  []string `json:"static_ips"`
	DisableSNI bool     `json:"disable_sni"`
}

var DefaultDoHEndpoints = []DoHEndpoint{
	{
		URL:        "https://1dot1dot1dot1.cloudflare-dns.com/dns-query",
		StaticIPs:  []string{"104.16.248.249", "104.16.249.249"},
		DisableSNI: true,
	},
	{
		URL:        "https://dns.alidns.com/resolve",
		StaticIPs:  []string{"223.5.5.5", "223.6.6.6"},
		DisableSNI: false,
	},
}

type cacheEntry struct {
	ips       []string
	expiresAt time.Time
}

// Resolver resolves hostnames via static-IP-pinned DoH endpoints without invoking system DNS.
type Resolver struct {
	endpoints  []DoHEndpoint
	timeout    time.Duration
	tlsConfig  *tls.Config
	mu         sync.RWMutex
	cache      map[string]cacheEntry
	dialDirect func(ctx context.Context, network, addr string) (net.Conn, error)
}

type Options struct {
	Endpoints []DoHEndpoint
	Timeout   time.Duration
	// TLSConfig is optional; tests may inject a custom RootCAs pool for local HTTPS DoH mocks.
	TLSConfig *tls.Config
}

func NewResolver(opts Options) *Resolver {
	eps := opts.Endpoints
	if len(eps) == 0 {
		eps = DefaultDoHEndpoints
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	d := &net.Dialer{Timeout: timeout}
	return &Resolver{
		endpoints:  eps,
		timeout:    timeout,
		tlsConfig:  opts.TLSConfig,
		cache:      make(map[string]cacheEntry),
		dialDirect: d.DialContext,
	}
}

func (r *Resolver) Resolve(ctx context.Context, host string) ([]string, error) {
	if _, err := netip.ParseAddr(host); err == nil {
		return []string{host}, nil
	}
	if host == "localhost" {
		return []string{"127.0.0.1"}, nil
	}

	r.mu.RLock()
	if ent, ok := r.cache[host]; ok && time.Now().Before(ent.expiresAt) && len(ent.ips) > 0 {
		ips := append([]string(nil), ent.ips...)
		r.mu.RUnlock()
		return ips, nil
	}
	r.mu.RUnlock()

	var lastErr error
	for _, ep := range r.endpoints {
		ips, ttl, err := r.queryEndpoint(ctx, ep, host)
		if err == nil && len(ips) > 0 {
			if ttl < 30*time.Second {
				ttl = 30 * time.Second
			}
			r.mu.Lock()
			r.cache[host] = cacheEntry{ips: ips, expiresAt: time.Now().Add(ttl)}
			r.mu.Unlock()
			return ips, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no DoH endpoints configured")
	}
	return nil, fmt.Errorf("bootstrap DoH resolve %s: %w", host, lastErr)
}

func (r *Resolver) queryEndpoint(ctx context.Context, ep DoHEndpoint, host string) ([]string, time.Duration, error) {
	u, err := url.Parse(ep.URL)
	if err != nil {
		return nil, 0, err
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	if len(ep.StaticIPs) == 0 {
		return nil, 0, errors.New("DoH endpoint requires at least one static IP")
	}

	q := u.Query()
	q.Set("name", host)
	q.Set("type", "A")
	u.RawQuery = q.Encode()

	var tlsCfg *tls.Config
	if r.tlsConfig != nil {
		tlsCfg = r.tlsConfig.Clone()
	} else {
		tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if ep.DisableSNI {
		// Omit plaintext SNI while still verifying the endpoint certificate against its hostname.
		tlsCfg.ServerName = ""
		tlsCfg.InsecureSkipVerify = true
		expectedHost := u.Hostname()
		tlsCfg.VerifyConnection = func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("no peer certificates")
			}
			return cs.PeerCertificates[0].VerifyHostname(expectedHost)
		}
	} else if tlsCfg.ServerName == "" {
		tlsCfg.ServerName = u.Hostname()
	}

	tr := &http.Transport{
		TLSClientConfig: tlsCfg,
		DialContext: func(dctx context.Context, network, _ string) (net.Conn, error) {
			var dialErr error
			for _, ip := range ep.StaticIPs {
				c, err := r.dialDirect(dctx, network, net.JoinHostPort(ip, port))
				if err == nil {
					return c, nil
				}
				dialErr = err
			}
			return nil, dialErr
		},
	}
	defer tr.CloseIdleConnections()

	cli := &http.Client{Transport: tr, Timeout: r.timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/dns-json")
	resp, err := cli.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("DoH HTTP %d", resp.StatusCode)
	}

	var dnsResp struct {
		Status int `json:"Status"`
		Answer []struct {
			Type int    `json:"type"`
			TTL  int    `json:"TTL"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dnsResp); err != nil {
		return nil, 0, err
	}
	if dnsResp.Status != 0 {
		return nil, 0, fmt.Errorf("DoH RCODE %d", dnsResp.Status)
	}

	var ips []string
	minTTL := 300
	for _, ans := range dnsResp.Answer {
		if ans.Type == 1 || ans.Type == 28 {
			if _, err := netip.ParseAddr(ans.Data); err == nil {
				ips = append(ips, ans.Data)
				if ans.TTL > 0 && ans.TTL < minTTL {
					minTTL = ans.TTL
				}
			}
		}
	}
	if len(ips) == 0 {
		return nil, 0, errors.New("empty DoH answer")
	}
	return ips, time.Duration(minTTL) * time.Second, nil
}

// NewHTTPClient returns an HTTP client whose TCP dials resolve hostnames through the static-IP DoH Resolver.
func NewHTTPClient(opts Options) *http.Client {
	res := NewResolver(opts)
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	tr := &http.Transport{
		TLSClientConfig: opts.TLSConfig,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := res.Resolve(ctx, host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, ip := range ips {
				c, err := res.dialDirect(ctx, network, net.JoinHostPort(ip, port))
				if err == nil {
					return c, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}
