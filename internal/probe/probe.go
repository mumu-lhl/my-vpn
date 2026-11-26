package probe

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/config"
)

type TestURLPreset struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

// BuiltinTestURLs defines standard and censorship-resistant connectivity test endpoints.
var BuiltinTestURLs = []TestURLPreset{
	{
		ID:    "gstatic-204",
		Label: "Google 204（默认）",
		URL:   "http://connectivitycheck.gstatic.com/generate_204",
	},
	{
		ID:    "cloudflare-204",
		Label: "Cloudflare 204",
		URL:   "http://cp.cloudflare.com/generate_204",
	},
	{
		ID:    "gstatic-https-204",
		Label: "Google HTTPS 204",
		URL:   "https://www.gstatic.com/generate_204",
	},
	{
		ID:    "apple-captive",
		Label: "Apple Captive",
		URL:   "http://captive.apple.com/hotspot-detect.html",
	},
	{
		ID:    "google-doh",
		Label: "Google DoH",
		URL:   "https://dns.google/dns-query",
	},
	{
		ID:    "cloudflare-ip",
		Label: "Cloudflare 1.1.1.1",
		URL:   "http://1.1.1.1",
	},
}

// DefaultIPInfoEndpoints are queried in order until one returns exit IP and ISO-2 country code.
var DefaultIPInfoEndpoints = []string{
	"https://ipwho.is/",
	"https://ipapi.co/json/",
	"https://ifconfig.co/json",
	"https://www.cloudflare.com/cdn-cgi/trace",
}

type ConnectivityResult struct {
	URL        string `json:"url"`
	Reachable  bool   `json:"reachable"`
	LatencyMs  int64  `json:"latency_ms"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

type ExitIPInfo struct {
	IP          string `json:"ip"`
	CountryCode string `json:"country_code"`
	Flag        string `json:"flag"`
	City        string `json:"city,omitempty"`
	Org         string `json:"org,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Error       string `json:"error,omitempty"`
}

type HopProbeResult struct {
	HopIndex     int                `json:"hop_index"`
	Label        string             `json:"label"`
	Kind         config.HopKind     `json:"kind"`
	ProxyAddr    string             `json:"proxy_addr,omitempty"`
	Connectivity ConnectivityResult `json:"connectivity"`
	ExitIP       ExitIPInfo         `json:"exit_ip"`
}

type ChainProbeResult struct {
	TestURL string           `json:"test_url"`
	Hops    []HopProbeResult `json:"hops"`
}

// CountryCodeToFlag converts an ISO 3166-1 alpha-2 country code (e.g. "JP") into its flag emoji ("🇯🇵").
func CountryCodeToFlag(code string) string {
	c := strings.ToUpper(strings.TrimSpace(code))
	if len(c) != 2 || c[0] < 'A' || c[0] > 'Z' || c[1] < 'A' || c[1] > 'Z' {
		return "🌐"
	}
	const base = rune(0x1F1E6)
	return string([]rune{base + rune(c[0]-'A'), base + rune(c[1]-'A')})
}

// ParseIPInfo parses JSON or Cloudflare trace responses from common GeoIP providers.
func ParseIPInfo(body []byte, provider string) (ExitIPInfo, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ExitIPInfo{}, errors.New("empty IP info response")
	}

	// Cloudflare /cdn-cgi/trace key=value format
	if !strings.HasPrefix(trimmed, "{") {
		var ip, cc string
		for _, line := range strings.Split(trimmed, "\n") {
			k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
			if !ok {
				continue
			}
			switch k {
			case "ip":
				ip = strings.TrimSpace(v)
			case "loc":
				cc = strings.ToUpper(strings.TrimSpace(v))
			}
		}
		if ip == "" {
			return ExitIPInfo{}, errors.New("missing ip in trace response")
		}
		return ExitIPInfo{
			IP:          ip,
			CountryCode: cc,
			Flag:        CountryCodeToFlag(cc),
			Provider:    provider,
		}, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return ExitIPInfo{}, fmt.Errorf("decode IP info JSON: %w", err)
	}
	if okVal, exists := raw["success"]; exists {
		if b, isBool := okVal.(bool); isBool && !b {
			return ExitIPInfo{}, errors.New("provider returned success=false")
		}
	}

	strField := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := raw[k]; ok {
				if s, isStr := v.(string); isStr && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
		return ""
	}

	ip := strField("ip", "query")
	cc := strings.ToUpper(strField("country_code", "country_iso", "countryCode", "country"))
	if len(cc) != 2 {
		cc = strings.ToUpper(strField("country_code", "country_iso", "countryCode"))
	}
	city := strField("city")
	org := strField("org", "asn_org", "isp")
	if org == "" {
		if conn, ok := raw["connection"].(map[string]any); ok {
			if isp, ok := conn["isp"].(string); ok {
				org = strings.TrimSpace(isp)
			}
		}
	}
	if ip == "" {
		return ExitIPInfo{}, errors.New("missing ip in JSON response")
	}
	return ExitIPInfo{
		IP:          ip,
		CountryCode: cc,
		Flag:        CountryCodeToFlag(cc),
		City:        city,
		Org:         org,
		Provider:    provider,
	}, nil
}

type Prober struct {
	Timeout         time.Duration
	IPInfoEndpoints []string
	// ClientForProxy optionally overrides HTTP client construction (used in unit tests).
	ClientForProxy func(socksAddr string, timeout time.Duration) *http.Client
}

func NewProber() *Prober {
	return &Prober{
		Timeout:         5 * time.Second,
		IPInfoEndpoints: append([]string(nil), DefaultIPInfoEndpoints...),
	}
}

func (p *Prober) client(socksAddr string, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = p.Timeout
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if p.ClientForProxy != nil {
		return p.ClientForProxy(socksAddr, timeout)
	}
	tr := &http.Transport{
		DisableKeepAlives: true,
	}
	if strings.TrimSpace(socksAddr) != "" {
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialSocks5Context(ctx, socksAddr, addr)
		}
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
	}
}

func (p *Prober) CheckConnectivity(ctx context.Context, testURL, socksAddr string, timeout time.Duration) ConnectivityResult {
	res := ConnectivityResult{URL: testURL}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	req.Header.Set("User-Agent", "Looma/0.1")

	start := time.Now()
	resp, err := p.client(socksAddr, timeout).Do(req)
	elapsed := time.Since(start).Milliseconds()
	if elapsed < 1 {
		elapsed = 1
	}
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	res.StatusCode = resp.StatusCode
	res.LatencyMs = elapsed
	res.Reachable = resp.StatusCode >= 200 && resp.StatusCode < 400
	if !res.Reachable {
		res.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return res
}

func (p *Prober) FetchExitIP(ctx context.Context, socksAddr string, timeout time.Duration) ExitIPInfo {
	endpoints := p.IPInfoEndpoints
	if len(endpoints) == 0 {
		endpoints = DefaultIPInfoEndpoints
	}
	var lastErr string
	for _, ep := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep, nil)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		req.Header.Set("User-Agent", "Looma/0.1")
		resp, err := p.client(socksAddr, timeout).Do(req)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16384))
		resp.Body.Close()
		if readErr != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Sprintf("%s: HTTP %d", ep, resp.StatusCode)
			continue
		}
		info, parseErr := ParseIPInfo(body, ep)
		if parseErr != nil {
			lastErr = parseErr.Error()
			continue
		}
		return info
	}
	return ExitIPInfo{Flag: "🌐", Error: lastErr}
}

// ProbeChain tests connectivity and exit IP/country/flag for every active hop in the configured chain.
func (p *Prober) ProbeChain(ctx context.Context, cfg *config.AppConfig) *ChainProbeResult {
	testURL := cfg.EffectiveProbeURL()
	timeout := time.Duration(cfg.Probe.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = p.Timeout
	}

	finalSocks := fmt.Sprintf("127.0.0.1:%d", cfg.Inbound.SocksPort)
	out := &ChainProbeResult{TestURL: testURL}

	if !cfg.Chain.TwoHop {
		hop1 := HopProbeResult{
			HopIndex:     1,
			Label:        fmt.Sprintf("第一跳 (%s)", cfg.Chain.Hop1.Kind),
			Kind:         cfg.Chain.Hop1.Kind,
			ProxyAddr:    finalSocks,
			Connectivity: p.CheckConnectivity(ctx, testURL, finalSocks, timeout),
			ExitIP:       p.FetchExitIP(ctx, finalSocks, timeout),
		}
		out.Hops = []HopProbeResult{hop1}
		return out
	}

	hop1Addr := ""
	if cfg.Chain.Hop1.Kind == config.HopGeph {
		port := cfg.Geph.LocalSocksPort
		if port <= 0 {
			port = 19909
		}
		hop1Addr = fmt.Sprintf("127.0.0.1:%d", port)
	} else {
		port := cfg.Probe.Hop1ProbePort
		if port <= 0 {
			port = 10810
		}
		hop1Addr = fmt.Sprintf("127.0.0.1:%d", port)
	}

	hop1 := HopProbeResult{
		HopIndex:     1,
		Label:        fmt.Sprintf("第一跳 (%s)", cfg.Chain.Hop1.Kind),
		Kind:         cfg.Chain.Hop1.Kind,
		ProxyAddr:    hop1Addr,
		Connectivity: p.CheckConnectivity(ctx, testURL, hop1Addr, timeout),
		ExitIP:       p.FetchExitIP(ctx, hop1Addr, timeout),
	}
	hop2 := HopProbeResult{
		HopIndex:     2,
		Label:        fmt.Sprintf("第二跳 (%s)", cfg.Chain.Hop2.Kind),
		Kind:         cfg.Chain.Hop2.Kind,
		ProxyAddr:    finalSocks,
		Connectivity: p.CheckConnectivity(ctx, testURL, finalSocks, timeout),
		ExitIP:       p.FetchExitIP(ctx, finalSocks, timeout),
	}
	out.Hops = []HopProbeResult{hop1, hop2}
	return out
}

func dialSocks5Context(ctx context.Context, proxyAddr, targetAddr string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return nil, err
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid target port: %s", portStr)
	}
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = c.SetDeadline(dl)
	}
	if _, err := c.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		_ = c.Close()
		return nil, err
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(c, resp); err != nil || resp[0] != 0x05 || resp[1] != 0x00 {
		_ = c.Close()
		return nil, errors.New("socks5 auth handshake failed")
	}
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, []byte(host)...)
	p := make([]byte, 2)
	binary.BigEndian.PutUint16(p, uint16(port))
	req = append(req, p...)
	if _, err := c.Write(req); err != nil {
		_ = c.Close()
		return nil, err
	}
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(c, hdr); err != nil || hdr[1] != 0x00 {
		_ = c.Close()
		return nil, errors.New("socks5 connect failed")
	}
	switch hdr[3] {
	case 0x01:
		_, _ = io.CopyN(io.Discard, c, 6)
	case 0x03:
		l := make([]byte, 1)
		_, _ = io.ReadFull(c, l)
		_, _ = io.CopyN(io.Discard, c, int64(l[0])+2)
	case 0x04:
		_, _ = io.CopyN(io.Discard, c, 18)
	}
	_ = c.SetDeadline(time.Time{})
	return c, nil
}
