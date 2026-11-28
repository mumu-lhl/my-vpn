package core

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/mumu-lhl/looma/internal/bootstrap"
	"github.com/mumu-lhl/looma/internal/config"
	"github.com/mumu-lhl/looma/internal/dpi"
)

// BypassDialer performs serverless DPI-bypass dialing using static-IP DoH resolution,
// per-domain optimal strategy lookup, and first-flight TLS/HTTP desync.
type BypassDialer struct {
	cfg        config.DirectBypassConfig
	strategy   dpi.Strategy
	capability dpi.CapabilityStatus
	resolver   *bootstrap.Resolver
	cache      *dpi.DomainStrategyCache
	dialTCP    func(ctx context.Context, addr string) (net.Conn, error)
	hooks      dpi.DesyncHooks
}

func NewBypassDialer(cfg config.DirectBypassConfig) *BypassDialer {
	return NewBypassDialerWithCache(cfg, nil)
}

func NewBypassDialerWithCache(cfg config.DirectBypassConfig, cache *dpi.DomainStrategyCache) *BypassDialer {
	var endpoints []bootstrap.DoHEndpoint
	if u := strings.TrimSpace(cfg.DoHURL); u != "" {
		ips := cfg.DoHStaticIPs
		if len(ips) == 0 {
			ips = extractHostIPs(u)
		}
		if len(ips) > 0 {
			endpoints = append(endpoints, bootstrap.DoHEndpoint{
				URL:        u,
				StaticIPs:  ips,
				DisableSNI: cfg.DoHDisableSNI,
			})
		}
	}
	endpoints = append(endpoints, bootstrap.DefaultDoHEndpoints...)

	res := bootstrap.NewResolver(bootstrap.Options{
		Endpoints: endpoints,
		Timeout:   5 * time.Second,
	})
	d := NewBypassDialerWithDeps(cfg, res, nil, nil)
	d.cache = cache
	return d
}

func NewBypassDialerWithDeps(
	cfg config.DirectBypassConfig,
	resolver *bootstrap.Resolver,
	dialTCP func(ctx context.Context, addr string) (net.Conn, error),
	sleepFn func(time.Duration),
) *BypassDialer {
	if dialTCP == nil {
		d := &net.Dialer{Timeout: 8 * time.Second}
		dialTCP = func(ctx context.Context, addr string) (net.Conn, error) {
			return d.DialContext(ctx, "tcp", addr)
		}
	}
	strat, capStatus := dpi.ResolveEffectiveStrategy(cfg, dpi.HasRawSocketCapability())
	return &BypassDialer{
		cfg:        cfg,
		strategy:   strat,
		capability: capStatus,
		resolver:   resolver,
		dialTCP:    dialTCP,
		hooks:      dpi.DesyncHooks{SleepFn: sleepFn},
	}
}

func (d *BypassDialer) CapabilityStatus() dpi.CapabilityStatus {
	return d.capability
}

func (d *BypassDialer) Resolver() *bootstrap.Resolver {
	return d.resolver
}

func extractHostIPs(rawURL string) []string {
	rawURL = strings.TrimPrefix(rawURL, "https://")
	rawURL = strings.TrimPrefix(rawURL, "http://")
	hostPort, _, _ := strings.Cut(rawURL, "/")
	host := hostPort
	if h, _, err := net.SplitHostPort(hostPort); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if addr, err := netip.ParseAddr(host); err == nil {
		return []string{addr.String()}
	}
	return nil
}

// strategyForHost returns a per-domain cached strategy if available, otherwise the default strategy.
func (d *BypassDialer) strategyForHost(host string) dpi.Strategy {
	if d.cache != nil && d.cfg.AutoLearnPerDomain {
		if ent, ok := d.cache.Get(host); ok {
			if preset, found := dpi.FindPreset(ent.StrategyID); found {
				overrideCfg := d.cfg
				overrideCfg.PresetID = preset.ID
				strat, _ := dpi.ResolveEffectiveStrategy(overrideCfg, d.capability.RawCapAvailable)
				return strat
			}
		}
	}
	return d.strategy
}

// Dial resolves host via static-IP DoH (unless host is already an IP literal),
// opens a TCP connection with TCP_NODELAY (capturing live TCP seq/ack when CAP_NET_RAW
// raw fake injection is active), and wraps it with dpi.DesyncConn.
func (d *BypassDialer) Dial(ctx context.Context, host string, port int) (net.Conn, error) {
	var targets []string
	if addr, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		targets = []string{addr.String()}
	} else if d.resolver != nil {
		ips, err := d.resolver.Resolve(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("bypass DoH resolve %s: %w", host, err)
		}
		targets = ips
	} else {
		targets = []string{host}
	}

	strat := d.strategyForHost(host)
	captureSeq := d.capability.RawCapAvailable &&
		(strat.HasMethod(dpi.MethodRawBadSum) || strat.HasMethod(dpi.MethodRawBadSeq) || strat.HasMethod(dpi.MethodFakeTTL))

	var lastErr error
	portStr := strconv.Itoa(port)
	for _, target := range targets {
		var (
			conn   net.Conn
			seq    uint32
			ack    uint32
			hasSeq bool
			err    error
		)
		if captureSeq {
			conn, seq, ack, hasSeq, err = dpi.DialAndCaptureTCPSeq(ctx, target, port, d.dialTCP)
		} else {
			conn, err = d.dialTCP(ctx, net.JoinHostPort(target, portStr))
		}
		if err != nil {
			lastErr = err
			continue
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true)
		}
		dc := dpi.NewDesyncConn(conn, strat, d.hooks)
		if hasSeq {
			dc.SetInitialTCPSeq(seq, ack)
		}
		return dc, nil
	}
	if d.cache != nil && d.cfg.AutoLearnPerDomain {
		_, _ = d.cache.RecordFailure(host)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no resolved addresses for %s", host)
	}
	return nil, lastErr
}

// Legacy helper wrappers preserved for existing callers and unit tests.
func newBypassConn(c net.Conn, cfg config.DirectBypassConfig, sleepFn func(time.Duration)) *dpi.DesyncConn {
	strat, _ := dpi.ResolveEffectiveStrategy(cfg, false)
	// Legacy tests expect pure range-based splitting when SplitAtSNI is unset.
	strat.SplitAtSNI = cfg.SplitAtSNI
	strat.MixSNICase = cfg.MixSNICase
	if !cfg.EnableDisorder {
		strat.Methods = removeMethod(strat.Methods, dpi.MethodDisorder)
	}
	if !cfg.EnableOOB {
		strat.Methods = removeMethod(strat.Methods, dpi.MethodOOB)
	}
	if !cfg.EnableFakeTTL {
		strat.Methods = removeMethod(strat.Methods, dpi.MethodFakeTTL)
	}
	return dpi.NewDesyncConn(c, strat, dpi.DesyncHooks{SleepFn: sleepFn})
}

func removeMethod(in []dpi.DesyncMethod, target dpi.DesyncMethod) []dpi.DesyncMethod {
	out := make([]dpi.DesyncMethod, 0, len(in))
	for _, m := range in {
		if m != target {
			out = append(out, m)
		}
	}
	return out
}

func FragmentTLSClientHelloRecords(p []byte, minSize, maxSize int) [][]byte {
	var offsets []int
	for off := 5; off < len(p); {
		off += dpi.RandomInt(minSize, maxSize)
		if off < len(p) {
			offsets = append(offsets, off)
		}
	}
	return dpi.SplitTLSRecordsAtOffsets(p, offsets)
}

func ObfuscateHTTPHostHeader(p []byte) []byte {
	return dpi.ObfuscateHTTPRequest(p, true, false)
}

func SplitRandomChunks(p []byte, minSize, maxSize int) [][]byte {
	return dpi.SplitRandomChunks(p, minSize, maxSize)
}

func ParseRangeInt(spec string, defMin, defMax int) (int, int) {
	return dpi.ParseRangeInt(spec, defMin, defMax)
}

func ParseRangeDuration(spec string, defMin, defMax time.Duration) (time.Duration, time.Duration) {
	return dpi.ParseRangeDuration(spec, defMin, defMax)
}
