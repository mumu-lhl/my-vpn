package dpi

import (
	"crypto/rand"
	"math/big"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DesyncHooks allows unit tests to intercept socket TTL, OOB, sleep, and raw fake calls.
type DesyncHooks struct {
	SleepFn     func(time.Duration)
	GetTTLFn    func(net.Conn) (int, error)
	SetTTLFn    func(net.Conn, int) error
	SendOOBFn   func(net.Conn, byte) error
	InjectRawFn func(srcIP, dstIP netip.Addr, srcPort, dstPort uint16, seq, ack uint32, opts RawFakeOptions) error
}

// DesyncConn wraps a TCP net.Conn and applies the configured anti-DPI Strategy
// on the first outbound Write (TLS ClientHello or plaintext HTTP request).
type DesyncConn struct {
	net.Conn
	mu        sync.Mutex
	sentFirst bool
	strategy  Strategy
	hooks     DesyncHooks
}

func NewDesyncConn(c net.Conn, s Strategy, hooks DesyncHooks) *DesyncConn {
	if hooks.SleepFn == nil {
		hooks.SleepFn = time.Sleep
	}
	if hooks.GetTTLFn == nil {
		hooks.GetTTLFn = GetSocketTTL
	}
	if hooks.SetTTLFn == nil {
		hooks.SetTTLFn = SetSocketTTL
	}
	if hooks.SendOOBFn == nil {
		hooks.SendOOBFn = SendOOBByte
	}
	if hooks.InjectRawFn == nil {
		hooks.InjectRawFn = InjectRawFake
	}
	return &DesyncConn{Conn: c, strategy: s, hooks: hooks}
}

func (c *DesyncConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	if c.sentFirst {
		c.mu.Unlock()
		return c.Conn.Write(p)
	}
	c.sentFirst = true
	c.mu.Unlock()

	minSize, maxSize := ParseRangeInt(c.strategy.FragmentSize, 10, 30)
	minSleep, maxSleep := ParseRangeDuration(c.strategy.FragmentSleep, 2*time.Millisecond, 8*time.Millisecond)

	if IsTLSClientHello(p) {
		payload := p
		if c.strategy.MixSNICase {
			payload = MutateClientHelloSNICase(payload)
		}

		// 1. Optional Raw Socket fake ClientHello injection (Linux CAP_NET_RAW).
		if c.strategy.HasMethod(MethodRawBadSum) || c.strategy.HasMethod(MethodRawBadSeq) {
			c.tryInjectRawFake(c.strategy)
		}

		// 2. Compute split chunks (SNI-midpoint aware when enabled).
		var chunks [][]byte
		if c.strategy.HasMethod(MethodTLSRec) {
			if c.strategy.SplitAtSNI {
				offsets := ComputeSNISplitPoints(payload, c.strategy.SplitCount)
				chunks = SplitTLSRecordsAtOffsets(payload, offsets)
			} else {
				chunks = splitTLSRecordsByRange(payload, minSize, maxSize)
			}
		} else if c.strategy.SplitAtSNI {
			offsets := ComputeSNISplitPoints(payload, c.strategy.SplitCount)
			chunks = SliceAtOffsets(payload, offsets)
		} else {
			chunks = SplitRandomChunks(payload, minSize, maxSize)
		}

		if err := c.writeDesyncedChunks(chunks, minSleep, maxSleep); err != nil {
			return 0, err
		}
		return len(p), nil
	}

	if IsPlainHTTPRequest(p) && (c.strategy.HTTPMixHostCase || c.strategy.HTTPHeaderSpace) {
		mutated := ObfuscateHTTPRequest(p, c.strategy.HTTPMixHostCase, c.strategy.HTTPHeaderSpace)
		chunks := SplitRandomChunks(mutated, minSize, maxSize)
		if err := c.writeDesyncedChunks(chunks, minSleep, maxSleep); err != nil {
			return 0, err
		}
		return len(p), nil
	}

	return c.Conn.Write(p)
}

func (c *DesyncConn) tryInjectRawFake(s Strategy) {
	la, okL := c.Conn.LocalAddr().(*net.TCPAddr)
	ra, okR := c.Conn.RemoteAddr().(*net.TCPAddr)
	if !okL || !okR || la == nil || ra == nil {
		return
	}
	srcIP, ok1 := netip.AddrFromSlice(la.IP.To4())
	dstIP, ok2 := netip.AddrFromSlice(ra.IP.To4())
	if !ok1 || !ok2 {
		return
	}
	ttl := uint8(s.TTL)
	if ttl == 0 {
		ttl = 4
	}
	fakePayload := BuildFakeClientHello(s.FakeSNI)
	opts := RawFakeOptions{
		TTL:         ttl,
		BadChecksum: s.HasMethod(MethodRawBadSum),
		Payload:     fakePayload,
	}
	if s.HasMethod(MethodRawBadSeq) {
		opts.BadSeqOffset = -10000
	}
	if s.HasMethod(MethodIPFrag) {
		opts.IPFragmentMTU = 48
	}
	// Best-effort: if CAP_NET_RAW is absent or injection fails, userspace desync still proceeds.
	_ = c.hooks.InjectRawFn(srcIP, dstIP, uint16(la.Port), uint16(ra.Port), 1, 1, opts)
}

func (c *DesyncConn) writeDesyncedChunks(chunks [][]byte, minSleep, maxSleep time.Duration) error {
	if len(chunks) == 0 {
		return nil
	}
	useDisorder := c.strategy.HasMethod(MethodDisorder)
	useOOB := c.strategy.HasMethod(MethodOOB)
	lowTTL := c.strategy.TTL
	if lowTTL <= 0 {
		lowTTL = 4
	}
	oobByte := c.strategy.OOBByte
	if oobByte == 0 {
		oobByte = 'a'
	}

	origTTL := 64
	if useDisorder || c.strategy.HasMethod(MethodFakeTTL) {
		if t, err := c.hooks.GetTTLFn(c.Conn); err == nil && t > 0 {
			origTTL = t
		}
	}

	for i, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}
		// Disorder: send the first segment with a low TTL so it drops mid-path and
		// arrives via kernel retransmission after subsequent segments.
		if i == 0 && useDisorder && len(chunks) > 1 {
			_ = c.hooks.SetTTLFn(c.Conn, lowTTL)
			if _, err := c.Conn.Write(chunk); err != nil {
				_ = c.hooks.SetTTLFn(c.Conn, origTTL)
				return err
			}
			_ = c.hooks.SetTTLFn(c.Conn, origTTL)
		} else {
			if _, err := c.Conn.Write(chunk); err != nil {
				return err
			}
		}

		// OOB: inject a single urgent out-of-band byte right after the first SNI-split chunk.
		if i == 0 && useOOB && len(chunks) > 1 {
			_ = c.hooks.SendOOBFn(c.Conn, oobByte)
		}

		if i < len(chunks)-1 && maxSleep > 0 {
			c.hooks.SleepFn(RandomDuration(minSleep, maxSleep))
		}
	}
	return nil
}

func splitTLSRecordsByRange(p []byte, minSize, maxSize int) [][]byte {
	if !IsTLSClientHello(p) {
		return [][]byte{p}
	}
	var offsets []int
	for off := 5; off < len(p); {
		n := RandomInt(minSize, maxSize)
		off += n
		if off < len(p) {
			offsets = append(offsets, off)
		}
	}
	return SplitTLSRecordsAtOffsets(p, offsets)
}

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
		n := RandomInt(minSize, maxSize)
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

func RandomInt(minVal, maxVal int) int {
	if maxVal <= minVal {
		return minVal
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxVal-minVal+1)))
	if err != nil {
		return minVal
	}
	return minVal + int(n.Int64())
}

func RandomDuration(minVal, maxVal time.Duration) time.Duration {
	if maxVal <= minVal {
		return minVal
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxVal-minVal+1)))
	if err != nil {
		return minVal
	}
	return minVal + time.Duration(n.Int64())
}
