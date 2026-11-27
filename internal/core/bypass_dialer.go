package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/bootstrap"
	"github.com/mumu-lhl/my-vpn/internal/config"
)

// BypassDialer performs serverless DPI-bypass dialing using static-IP DoH resolution
// (to evade ISP DNS poisoning) and first-flight TLS ClientHello / HTTP Host fragmentation.
type BypassDialer struct {
	cfg      config.DirectBypassConfig
	resolver *bootstrap.Resolver
	dialTCP  func(ctx context.Context, addr string) (net.Conn, error)
	sleepFn  func(time.Duration)
}

func NewBypassDialer(cfg config.DirectBypassConfig) *BypassDialer {
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
	return NewBypassDialerWithDeps(cfg, res, nil, nil)
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
	if sleepFn == nil {
		sleepFn = time.Sleep
	}
	return &BypassDialer{
		cfg:      cfg,
		resolver: resolver,
		dialTCP:  dialTCP,
		sleepFn:  sleepFn,
	}
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

// Dial resolves host via static-IP DoH (unless host is already an IP literal),
// opens a TCP connection with TCP_NODELAY, and wraps it with first-flight DPI bypass.
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

	var lastErr error
	portStr := strconv.Itoa(port)
	for _, target := range targets {
		conn, err := d.dialTCP(ctx, net.JoinHostPort(target, portStr))
		if err != nil {
			lastErr = err
			continue
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true)
		}
		return newBypassConn(conn, d.cfg, d.sleepFn), nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no resolved addresses for %s", host)
	}
	return nil, lastErr
}

// BypassConn applies TLS ClientHello / HTTP Host fragmentation on the first Write only.
type BypassConn struct {
	net.Conn
	mu        sync.Mutex
	sentFirst bool
	cfg       config.DirectBypassConfig
	sleepFn   func(time.Duration)
}

func newBypassConn(c net.Conn, cfg config.DirectBypassConfig, sleepFn func(time.Duration)) *BypassConn {
	if sleepFn == nil {
		sleepFn = time.Sleep
	}
	return &BypassConn{Conn: c, cfg: cfg, sleepFn: sleepFn}
}

func (c *BypassConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	if c.sentFirst {
		c.mu.Unlock()
		return c.Conn.Write(p)
	}
	c.sentFirst = true
	c.mu.Unlock()

	minSize, maxSize := ParseRangeInt(c.cfg.TLSFragmentSize, 10, 30)
	minSleep, maxSleep := ParseRangeDuration(c.cfg.TLSFragmentSleep, 2*time.Millisecond, 8*time.Millisecond)

	if isTLSClientHello(p) {
		if c.cfg.TLSRecordFragment {
			records := FragmentTLSClientHelloRecords(p, minSize, maxSize)
			if err := c.writeChunksWithJitter(records, minSleep, maxSleep); err != nil {
				return 0, err
			}
			return len(p), nil
		}
		chunks := SplitRandomChunks(p, minSize, maxSize)
		if err := c.writeChunksWithJitter(chunks, minSleep, maxSleep); err != nil {
			return 0, err
		}
		return len(p), nil
	}

	if c.cfg.HTTPMixHostCase && isPlainHTTPRequest(p) {
		mutated := ObfuscateHTTPHostHeader(p)
		chunks := SplitRandomChunks(mutated, minSize, maxSize)
		if err := c.writeChunksWithJitter(chunks, minSleep, maxSleep); err != nil {
			return 0, err
		}
		return len(p), nil
	}

	return c.Conn.Write(p)
}

func (c *BypassConn) writeChunksWithJitter(chunks [][]byte, minSleep, maxSleep time.Duration) error {
	for i, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}
		if _, err := c.Conn.Write(chunk); err != nil {
			return err
		}
		if i < len(chunks)-1 && maxSleep > 0 {
			c.sleepFn(randomDuration(minSleep, maxSleep))
		}
	}
	return nil
}

// isTLSClientHello reports whether p begins with a TLS Handshake (0x16) ClientHello (0x01) record.
func isTLSClientHello(p []byte) bool {
	if len(p) < 6 {
		return false
	}
	if p[0] != 0x16 || p[1] != 0x03 {
		return false
	}
	recLen := int(binary.BigEndian.Uint16(p[3:5]))
	if recLen <= 0 || len(p) < 5+recLen {
		return false
	}
	return p[5] == 0x01
}

// FragmentTLSClientHelloRecords splits a single TLS Handshake record into multiple valid
// TLS Handshake records of randomized payload lengths in [minSize, maxSize].
func FragmentTLSClientHelloRecords(p []byte, minSize, maxSize int) [][]byte {
	if !isTLSClientHello(p) {
		return [][]byte{p}
	}
	recLen := int(binary.BigEndian.Uint16(p[3:5]))
	verMajor, verMinor := p[1], p[2]
	payload := p[5 : 5+recLen]
	trailing := p[5+recLen:]

	pieces := SplitRandomChunks(payload, minSize, maxSize)
	out := make([][]byte, 0, len(pieces)+1)
	for _, piece := range pieces {
		rec := make([]byte, 5+len(piece))
		rec[0] = 0x16
		rec[1] = verMajor
		rec[2] = verMinor
		binary.BigEndian.PutUint16(rec[3:5], uint16(len(piece)))
		copy(rec[5:], piece)
		out = append(out, rec)
	}
	if len(trailing) > 0 {
		out = append(out, trailing)
	}
	return out
}

func isPlainHTTPRequest(p []byte) bool {
	for _, method := range [][]byte{
		[]byte("GET "),
		[]byte("POST "),
		[]byte("HEAD "),
		[]byte("PUT "),
		[]byte("DELETE "),
		[]byte("OPTIONS "),
		[]byte("PATCH "),
	} {
		if bytes.HasPrefix(p, method) {
			return true
		}
	}
	return false
}

// ObfuscateHTTPHostHeader rewrites the HTTP Host header name to mixed case ("HoSt:")
// within the header section while keeping the request RFC 9110 compliant.
func ObfuscateHTTPHostHeader(p []byte) []byte {
	hdrEnd := bytes.Index(p, []byte("\r\n\r\n"))
	if hdrEnd < 0 {
		hdrEnd = len(p)
	}
	out := make([]byte, len(p))
	copy(out, p)

	lower := bytes.ToLower(out[:hdrEnd])
	idx := bytes.Index(lower, []byte("\r\nhost:"))
	if idx >= 0 {
		copy(out[idx+2:idx+7], []byte("HoSt:"))
	}
	return out
}

// SplitRandomChunks slices p into chunks whose lengths are drawn randomly from [minSize, maxSize].
func SplitRandomChunks(p []byte, minSize, maxSize int) [][]byte {
	if len(p) == 0 {
		return nil
	}
	if minSize < 1 {
		minSize = 1
	}
	if maxSize < minSize {
		maxSize = minSize
	}
	var chunks [][]byte
	for off := 0; off < len(p); {
		n := randomInt(minSize, maxSize)
		if off+n > len(p) {
			n = len(p) - off
		}
		chunk := make([]byte, n)
		copy(chunk, p[off:off+n])
		chunks = append(chunks, chunk)
		off += n
	}
	return chunks
}

func ParseRangeInt(spec string, defMin, defMax int) (int, int) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return defMin, defMax
	}
	if lo, hi, ok := strings.Cut(spec, "-"); ok {
		a, errA := strconv.Atoi(strings.TrimSpace(lo))
		b, errB := strconv.Atoi(strings.TrimSpace(hi))
		if errA == nil && errB == nil && a > 0 && b >= a {
			return a, b
		}
	} else if v, err := strconv.Atoi(spec); err == nil && v > 0 {
		return v, v
	}
	return defMin, defMax
}

func ParseRangeDuration(spec string, defMin, defMax time.Duration) (time.Duration, time.Duration) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return defMin, defMax
	}
	if lo, hi, ok := strings.Cut(spec, "-"); ok {
		hi = strings.TrimSpace(hi)
		lo = strings.TrimSpace(lo)
		// Allow shorthand like "2-8ms" where only the upper bound carries the unit.
		if !hasDurationUnit(lo) && strings.HasSuffix(hi, "ms") {
			lo += "ms"
		} else if !hasDurationUnit(lo) && strings.HasSuffix(hi, "s") {
			lo += "s"
		}
		a, errA := time.ParseDuration(lo)
		b, errB := time.ParseDuration(hi)
		if errA == nil && errB == nil && a >= 0 && b >= a {
			return a, b
		}
	} else if d, err := time.ParseDuration(spec); err == nil && d >= 0 {
		return d, d
	}
	return defMin, defMax
}

func hasDurationUnit(s string) bool {
	return strings.HasSuffix(s, "ns") || strings.HasSuffix(s, "us") || strings.HasSuffix(s, "µs") ||
		strings.HasSuffix(s, "ms") || strings.HasSuffix(s, "s") || strings.HasSuffix(s, "m")
}

func randomInt(minVal, maxVal int) int {
	if maxVal <= minVal {
		return minVal
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxVal-minVal+1)))
	if err != nil {
		return minVal
	}
	return minVal + int(n.Int64())
}

func randomDuration(minVal, maxVal time.Duration) time.Duration {
	if maxVal <= minVal {
		return minVal
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxVal-minVal+1)))
	if err != nil {
		return minVal
	}
	return minVal + time.Duration(n.Int64())
}
