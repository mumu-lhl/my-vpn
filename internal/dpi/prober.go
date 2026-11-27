package dpi

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/bootstrap"
)

// StrategyProbeResult captures the outcome of testing a single strategy against a target host.
type StrategyProbeResult struct {
	StrategyID   string `json:"strategy_id"`
	StrategyName string `json:"strategy_name"`
	Success      bool   `json:"success"`
	LatencyMs    int64  `json:"latency_ms"`
	UsedECH      bool   `json:"used_ech"`
	Error        string `json:"error,omitempty"`
}

// AutoProbeReport summarizes a blockcheck-style multi-strategy probe run for a domain.
type AutoProbeReport struct {
	Domain          string                `json:"domain"`
	ResolvedIPs     []string              `json:"resolved_ips,omitempty"`
	ECHAvailable    bool                  `json:"ech_available"`
	ECHHandshakeOK  bool                  `json:"ech_handshake_ok"`
	RecommendedID   string                `json:"recommended_id,omitempty"`
	RecommendedName string                `json:"recommended_name,omitempty"`
	BestLatencyMs   int64                 `json:"best_latency_ms,omitempty"`
	Capability      CapabilityStatus      `json:"capability"`
	Results         []StrategyProbeResult `json:"results"`
	Error           string                `json:"error,omitempty"`
}

// AutoProber runs blockcheck-style automated strategy discovery and native Go ECH checks.
type AutoProber struct {
	resolver   *bootstrap.Resolver
	cache      *DomainStrategyCache
	dialTCP    func(ctx context.Context, addr string) (net.Conn, error)
	tlsConfig  *tls.Config // optional override for local loopback tests
	targetPort int
	timeout    time.Duration
}

type AutoProberOptions struct {
	Resolver   *bootstrap.Resolver
	Cache      *DomainStrategyCache
	DialTCP    func(ctx context.Context, addr string) (net.Conn, error)
	TLSConfig  *tls.Config
	TargetPort int
	Timeout    time.Duration
}

func NewAutoProber(opts AutoProberOptions) *AutoProber {
	res := opts.Resolver
	if res == nil {
		res = bootstrap.NewResolver(bootstrap.Options{Timeout: 5 * time.Second})
	}
	dial := opts.DialTCP
	if dial == nil {
		d := &net.Dialer{Timeout: 5 * time.Second}
		dial = func(ctx context.Context, addr string) (net.Conn, error) {
			return d.DialContext(ctx, "tcp", addr)
		}
	}
	port := opts.TargetPort
	if port <= 0 {
		port = 443
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	return &AutoProber{
		resolver:   res,
		cache:      opts.Cache,
		dialTCP:    dial,
		tlsConfig:  opts.TLSConfig,
		targetPort: port,
		timeout:    timeout,
	}
}

// Probe tests ECH (via DoH Type 65 HTTPS RR) and all builtin anti-DPI presets against domain,
// ranks successful strategies by lowest RTT, and caches the winning strategy per domain.
func (p *AutoProber) Probe(ctx context.Context, domain string) *AutoProbeReport {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		domain = "www.pixiv.net"
	}
	rawCap := HasRawSocketCapability()
	report := &AutoProbeReport{
		Domain: domain,
		Capability: CapabilityStatus{
			RawCapAvailable: rawCap,
		},
	}

	ips, err := p.resolver.Resolve(ctx, domain)
	if err != nil || len(ips) == 0 {
		report.Error = fmt.Sprintf("DoH 解析失败: %v", err)
		return report
	}
	report.ResolvedIPs = ips

	// Check DoH Type 65 (HTTPS RR) for ECHConfigList and attempt native Go ECH TLS handshake.
	var echConfigs []byte
	if httpsRec, err := p.resolver.ResolveHTTPS(ctx, domain); err == nil && httpsRec != nil && len(httpsRec.ECHConfigList) > 0 {
		report.ECHAvailable = true
		echConfigs = httpsRec.ECHConfigList
		if ok, _ := p.probeStrategy(ctx, domain, ips[0], BuiltinPresets()[0], echConfigs); ok {
			report.ECHHandshakeOK = true
		}
	}

	presets := BuiltinPresets()
	results := make([]StrategyProbeResult, 0, len(presets))
	for _, s := range presets {
		eff := s
		if eff.RequiresRaw && !rawCap {
			eff.Methods = stripRawMethods(eff.Methods)
			if !eff.HasMethod(MethodDisorder) {
				eff.Methods = append([]DesyncMethod{MethodDisorder, MethodOOB}, eff.Methods...)
			}
			eff.RequiresRaw = false
		}
		start := time.Now()
		ok, probeErr := p.probeStrategy(ctx, domain, ips[0], eff, nil)
		rtt := time.Since(start).Milliseconds()
		res := StrategyProbeResult{
			StrategyID:   s.ID,
			StrategyName: s.Name,
			Success:      ok,
			LatencyMs:    rtt,
			UsedECH:      false,
		}
		if probeErr != nil {
			res.Error = probeErr.Error()
		}
		results = append(results, res)
	}

	// Sort: working strategies first, then lowest latency.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Success != results[j].Success {
			return results[i].Success
		}
		return results[i].LatencyMs < results[j].LatencyMs
	})
	report.Results = results

	if len(results) > 0 && results[0].Success {
		report.RecommendedID = results[0].StrategyID
		report.RecommendedName = results[0].StrategyName
		report.BestLatencyMs = results[0].LatencyMs
		if p.cache != nil {
			_ = p.cache.RecordSuccess(domain, results[0].StrategyID, results[0].LatencyMs, report.ECHHandshakeOK)
		}
	}
	return report
}

func (p *AutoProber) probeStrategy(ctx context.Context, host, ip string, s Strategy, echConfigList []byte) (bool, error) {
	pctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	rawConn, err := p.dialTCP(pctx, net.JoinHostPort(ip, strconv.Itoa(p.targetPort)))
	if err != nil {
		return false, err
	}
	defer rawConn.Close()
	if tc, ok := rawConn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	desync := NewDesyncConn(rawConn, s, DesyncHooks{})
	var tlsCfg *tls.Config
	if p.tlsConfig != nil {
		tlsCfg = p.tlsConfig.Clone()
	} else {
		tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if tlsCfg.ServerName == "" {
		tlsCfg.ServerName = host
	}
	if len(echConfigList) > 0 {
		tlsCfg.MinVersion = tls.VersionTLS13
		tlsCfg.EncryptedClientHelloConfigList = echConfigList
	}

	tlsConn := tls.Client(desync, tlsCfg)
	if err := tlsConn.HandshakeContext(pctx); err != nil {
		return false, err
	}
	return true, nil
}
