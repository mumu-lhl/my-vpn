package dpi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mumu-lhl/looma/internal/bootstrap"
	"github.com/mumu-lhl/looma/internal/config"
)

type mockConn struct {
	mu     sync.Mutex
	writes [][]byte
	local  net.Addr
	remote net.Addr
}

func (m *mockConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return m.local }
func (m *mockConn) RemoteAddr() net.Addr               { return m.remote }
func (m *mockConn) SetDeadline(_ time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(_ time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(_ time.Time) error { return nil }
func (m *mockConn) Write(p []byte) (int, error) {
	m.mu.Lock()
	cp := make([]byte, len(p))
	copy(cp, p)
	m.writes = append(m.writes, cp)
	m.mu.Unlock()
	return len(p), nil
}

func TestFindSNIAndExactMidpointSplit(t *testing.T) {
	hello := BuildFakeClientHello("www.pixiv.net")
	span, ok := FindSNIExtension(hello)
	if !ok {
		t.Fatalf("expected FindSNIExtension to locate SNI")
	}
	if span.Host != "www.pixiv.net" {
		t.Fatalf("span.Host = %q, want www.pixiv.net", span.Host)
	}

	pts := ComputeSNISplitPoints(hello, 4)
	hitInsideSNI := false
	for _, pt := range pts {
		if pt > span.HostStart && pt < span.HostEnd {
			hitInsideSNI = true
		}
	}
	if !hitInsideSNI {
		t.Fatalf("expected at least one split point inside [%d, %d], got %v", span.HostStart, span.HostEnd, pts)
	}

	mutated := MutateClientHelloSNICase(hello)
	mSpan, ok := FindSNIExtension(mutated)
	if !ok || !strings.EqualFold(mSpan.Host, "www.pixiv.net") || mSpan.Host == "www.pixiv.net" {
		t.Fatalf("expected mixed-case SNI mutation, got %q", mSpan.Host)
	}

	records := SplitTLSRecordsAtOffsets(hello, pts)
	if len(records) < 2 {
		t.Fatalf("expected >= 2 TLS records, got %d", len(records))
	}
	var reassembled []byte
	for i, r := range records {
		if len(r) < 6 || r[0] != 0x16 || r[1] != 0x03 {
			t.Fatalf("record %d invalid header", i)
		}
		rlen := int(binary.BigEndian.Uint16(r[3:5]))
		if rlen != len(r)-5 {
			t.Fatalf("record %d length mismatch", i)
		}
		reassembled = append(reassembled, r[5:]...)
	}
	if !bytes.Equal(reassembled, hello[5:]) {
		t.Fatalf("reassembled TLS record payload mismatch")
	}
}

func TestDesyncConnDisorderOOBAndRawFallback(t *testing.T) {
	cfg := config.DefaultConfig().Routing.DirectBypass
	cfg.PresetID = PresetRawFakeMultidisorder
	cfg.EnableRawFake = true

	// Without CAP_NET_RAW, strategy must seamlessly degrade to rootless disorder + OOB.
	degraded, status := ResolveEffectiveStrategy(cfg, false)
	if !status.Degraded || status.Notice == "" {
		t.Fatalf("expected graceful degradation status with notice, got %+v", status)
	}
	if degraded.HasMethod(MethodRawBadSum) || !degraded.HasMethod(MethodDisorder) || !degraded.HasMethod(MethodOOB) {
		t.Fatalf("expected degraded strategy to strip raw methods and keep disorder+oob, got %+v", degraded.Methods)
	}

	mc := &mockConn{
		local:  &net.TCPAddr{IP: net.ParseIP("192.168.1.2"), Port: 51234},
		remote: &net.TCPAddr{IP: net.ParseIP("1.1.1.1"), Port: 443},
	}
	var ttls []int
	var oobBytes []byte
	dc := NewDesyncConn(mc, degraded, DesyncHooks{
		SleepFn:   func(time.Duration) {},
		GetTTLFn:  func(net.Conn) (int, error) { return 64, nil },
		SetTTLFn:  func(_ net.Conn, ttl int) error { ttls = append(ttls, ttl); return nil },
		SendOOBFn: func(_ net.Conn, b byte) error { oobBytes = append(oobBytes, b); return nil },
	})

	hello := BuildFakeClientHello("www.wikipedia.org")
	if _, err := dc.Write(hello); err != nil {
		t.Fatalf("DesyncConn.Write failed: %v", err)
	}
	if len(ttls) < 2 || ttls[0] != degraded.TTL || ttls[1] != 64 {
		t.Fatalf("expected disorder TTL drop to %d then restore to 64, got %v", degraded.TTL, ttls)
	}
	if len(oobBytes) != 1 {
		t.Fatalf("expected 1 MSG_OOB byte injected, got %v", oobBytes)
	}

	// With CAP_NET_RAW enabled, verify InjectRawFn is invoked with badsum + badseq.
	rawStrat, rawStatus := ResolveEffectiveStrategy(cfg, true)
	if rawStatus.Degraded || !rawStrat.HasMethod(MethodRawBadSum) {
		t.Fatalf("expected raw strategy active when rawCap=true, got %+v %+v", rawStrat, rawStatus)
	}
	var rawCalled bool
	var capturedOpts RawFakeOptions
	dcRaw := NewDesyncConn(mc, rawStrat, DesyncHooks{
		SleepFn:  func(time.Duration) {},
		GetTTLFn: func(net.Conn) (int, error) { return 64, nil },
		SetTTLFn: func(net.Conn, int) error { return nil },
		InjectRawFn: func(_, _ netip.Addr, _, _ uint16, _, _ uint32, opts RawFakeOptions) error {
			rawCalled = true
			capturedOpts = opts
			return nil
		},
	})
	if _, err := dcRaw.Write(hello); err != nil {
		t.Fatalf("dcRaw.Write failed: %v", err)
	}
	if !rawCalled || !capturedOpts.BadChecksum || capturedOpts.BadSeqOffset == 0 {
		t.Fatalf("expected raw fake injection with badsum+badseq, called=%v opts=%+v", rawCalled, capturedOpts)
	}
}

func TestBuildFakeTCPPacketsBadSumBadSeqAndIPFrag(t *testing.T) {
	src := netip.MustParseAddr("192.168.1.10")
	dst := netip.MustParseAddr("104.16.132.229")
	decoy := BuildFakeClientHello("www.microsoft.com")

	// Valid checksum baseline
	validPkts, err := BuildFakeTCPPackets(src, dst, 40000, 443, 100000, 50000, RawFakeOptions{
		TTL:         5,
		BadChecksum: false,
		Payload:     decoy,
	})
	if err != nil || len(validPkts) != 1 {
		t.Fatalf("BuildFakeTCPPackets valid failed: %v", err)
	}
	if !VerifyTCPChecksumIPv4(validPkts[0]) {
		t.Fatalf("expected valid TCP checksum when BadChecksum=false")
	}
	if validPkts[0][8] != 5 {
		t.Fatalf("expected IPv4 TTL=5, got %d", validPkts[0][8])
	}

	// Bad checksum + shifted seq
	badPkts, err := BuildFakeTCPPackets(src, dst, 40000, 443, 100000, 50000, RawFakeOptions{
		TTL:          4,
		BadChecksum:  true,
		BadSeqOffset: -10000,
		Payload:      decoy,
	})
	if err != nil || len(badPkts) != 1 {
		t.Fatalf("BuildFakeTCPPackets bad failed: %v", err)
	}
	if VerifyTCPChecksumIPv4(badPkts[0]) {
		t.Fatalf("expected corrupted TCP checksum when BadChecksum=true")
	}
	gotSeq := binary.BigEndian.Uint32(badPkts[0][24:28])
	if gotSeq != 90000 {
		t.Fatalf("expected shifted seq=90000, got %d", gotSeq)
	}

	// IPv4 fragmentation
	fragPkts, err := BuildFakeTCPPackets(src, dst, 40000, 443, 100000, 50000, RawFakeOptions{
		TTL:           4,
		BadChecksum:   true,
		IPFragmentMTU: 48,
		Payload:       decoy,
	})
	if err != nil || len(fragPkts) < 2 {
		t.Fatalf("expected >= 2 IPv4 fragments, got %d (%v)", len(fragPkts), err)
	}
	if (binary.BigEndian.Uint16(fragPkts[0][6:8]) & 0x2000) == 0 {
		t.Fatalf("expected MF flag set on first IPv4 fragment")
	}
	if (binary.BigEndian.Uint16(fragPkts[len(fragPkts)-1][6:8]) & 0x2000) != 0 {
		t.Fatalf("expected MF flag cleared on final IPv4 fragment")
	}
}

type mockPacketConn struct {
	net.PacketConn
	sent [][]byte
}

func (m *mockPacketConn) WriteTo(p []byte, _ net.Addr) (int, error) {
	cp := make([]byte, len(p))
	copy(cp, p)
	m.sent = append(m.sent, cp)
	return len(p), nil
}

func TestQUICInitialDetectionFakeAndBlock(t *testing.T) {
	initial := BuildFakeQUICInitial(1200)
	if !IsQUICInitial(initial) {
		t.Fatalf("expected BuildFakeQUICInitial to be detected as QUIC Initial")
	}
	if IsQUICInitial([]byte{0x40, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00}) {
		t.Fatalf("short header packet should not match QUIC Initial")
	}

	mc := &mockPacketConn{}
	dropped, err := DesyncQUICPacket(mc, &net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 443}, initial, "block")
	if err != nil || !dropped || len(mc.sent) != 0 {
		t.Fatalf("expected QUIC block mode to drop Initial, dropped=%v sent=%d err=%v", dropped, len(mc.sent), err)
	}

	dropped, err = DesyncQUICPacket(mc, &net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 443}, initial, "fake")
	if err != nil || dropped || len(mc.sent) != 2 {
		t.Fatalf("expected QUIC fake mode to send decoy + real (2 packets), sent=%d err=%v", len(mc.sent), err)
	}
}

func TestDoHHTTPSRecordAndECHParsing(t *testing.T) {
	sampleECH := []byte{0x00, 0x20, 0xFE, 0x0D, 0x01, 0x02, 0x03, 0x04}
	echB64 := base64.StdEncoding.EncodeToString(sampleECH)

	dohSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-json")
		if r.URL.Query().Get("type") == "HTTPS" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Status": 0,
				"Answer": []map[string]any{
					{
						"type": 65,
						"TTL":  120,
						"data": fmt.Sprintf(`1 . alpn="h3,h2" ipv4hint=104.16.132.229 ech=%s`, echB64),
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Status": 0,
			"Answer": []map[string]any{{"type": 1, "TTL": 60, "data": "127.0.0.1"}},
		})
	}))
	defer dohSrv.Close()
	dohURL, _ := url.Parse(dohSrv.URL)

	res := bootstrap.NewResolver(bootstrap.Options{
		Endpoints: []bootstrap.DoHEndpoint{
			{URL: fmt.Sprintf("http://mock-doh.invalid:%s/dns-query", dohURL.Port()), StaticIPs: []string{"127.0.0.1"}},
		},
		Timeout: 2 * time.Second,
	})

	rec, err := res.ResolveHTTPS(context.Background(), "cloudflare.com")
	if err != nil {
		t.Fatalf("ResolveHTTPS failed: %v", err)
	}
	if !bytes.Equal(rec.ECHConfigList, sampleECH) {
		t.Fatalf("ECHConfigList mismatch: got %x, want %x", rec.ECHConfigList, sampleECH)
	}
	if len(rec.ALPN) != 2 || rec.ALPN[0] != "h3" {
		t.Fatalf("unexpected ALPN: %v", rec.ALPN)
	}
}

func TestDomainStrategyCachePersistenceAndAutoFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "domain_cache.json")
	c := NewDomainStrategyCache(path)
	if err := c.RecordSuccess("www.pixiv.net", PresetDisorderOOB, 42, true); err != nil {
		t.Fatalf("RecordSuccess: %v", err)
	}

	// Reload from disk and verify persistence.
	c2 := NewDomainStrategyCache(path)
	ent, ok := c2.Get("WWW.PIXIV.NET.")
	if !ok || ent.StrategyID != PresetDisorderOOB || !ent.ECHSupported {
		t.Fatalf("unexpected reloaded entry: %+v ok=%v", ent, ok)
	}

	// Two consecutive failures should evict unlocked entry.
	evicted, _ := c2.RecordFailure("www.pixiv.net")
	if evicted {
		t.Fatalf("should not evict on first failure")
	}
	evicted, _ = c2.RecordFailure("www.pixiv.net")
	if !evicted {
		t.Fatalf("expected eviction on second consecutive failure")
	}
	if _, ok := c2.Get("www.pixiv.net"); ok {
		t.Fatalf("expected evicted domain absent from cache")
	}

	// Locked entries must never be evicted by failures.
	_ = c2.Lock("github.com", PresetStandardSplit)
	for i := 0; i < 3; i++ {
		evicted, _ = c2.RecordFailure("github.com")
		if evicted {
			t.Fatalf("locked entry must not be evicted")
		}
	}
}

func TestAutoProberRanksStrategiesAndCachesWinner(t *testing.T) {
	// Self-signed TLS server on localhost for loopback AutoProber verification.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "probe.example.test"},
		DNSNames:     []string{"probe.example.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}
	pool := x509.NewCertPool()
	parsedCert, _ := x509.ParseCertificate(der)
	pool.AddCert(parsedCert)

	tlsLn, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatalf("tls listen: %v", err)
	}
	defer tlsLn.Close()
	go func() {
		for {
			c, err := tlsLn.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_ = conn.(*tls.Conn).Handshake()
			}(c)
		}
	}()
	tlsPort := tlsLn.Addr().(*net.TCPAddr).Port

	dohSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Status": 0,
			"Answer": []map[string]any{{"type": 1, "TTL": 60, "data": "127.0.0.1"}},
		})
	}))
	defer dohSrv.Close()
	dohURL, _ := url.Parse(dohSrv.URL)

	res := bootstrap.NewResolver(bootstrap.Options{
		Endpoints: []bootstrap.DoHEndpoint{
			{URL: fmt.Sprintf("http://mock-doh.invalid:%s/dns-query", dohURL.Port()), StaticIPs: []string{"127.0.0.1"}},
		},
		Timeout: 2 * time.Second,
	})
	cache := NewDomainStrategyCache(filepath.Join(t.TempDir(), "cache.json"))
	prober := NewAutoProber(AutoProberOptions{
		Resolver:   res,
		Cache:      cache,
		TLSConfig:  &tls.Config{RootCAs: pool, ServerName: "probe.example.test"},
		TargetPort: tlsPort,
		Timeout:    3 * time.Second,
	})

	report := prober.Probe(context.Background(), "probe.example.test")
	if report.Error != "" || report.RecommendedID == "" {
		t.Fatalf("expected successful AutoProbe recommendation, got %+v", report)
	}
	if len(report.Results) != len(BuiltinPresets()) {
		t.Fatalf("expected %d preset results, got %d", len(BuiltinPresets()), len(report.Results))
	}
	if ent, ok := cache.Get("probe.example.test"); !ok || ent.StrategyID != report.RecommendedID {
		t.Fatalf("expected winning strategy %q cached, got %+v ok=%v", report.RecommendedID, ent, ok)
	}
}

func TestMethodFakeTTLAndLiveSeqInjection(t *testing.T) {
	preset, ok := FindPreset(PresetLowTTLFake)
	if !ok {
		t.Fatalf("expected PresetLowTTLFake to exist")
	}
	mc := &mockConn{
		local:  &net.TCPAddr{IP: net.ParseIP("192.168.1.2"), Port: 55000},
		remote: &net.TCPAddr{IP: net.ParseIP("104.16.132.229"), Port: 443},
	}
	var ttls []int
	var gotSeq, gotAck uint32
	var gotTTL uint8
	dc := NewDesyncConn(mc, preset, DesyncHooks{
		SleepFn:  func(time.Duration) {},
		GetTTLFn: func(net.Conn) (int, error) { return 64, nil },
		SetTTLFn: func(_ net.Conn, ttl int) error { ttls = append(ttls, ttl); return nil },
		InjectRawFn: func(_, _ netip.Addr, _, _ uint16, seq, ack uint32, opts RawFakeOptions) error {
			gotSeq = seq
			gotAck = ack
			gotTTL = opts.TTL
			return nil
		},
	})
	dc.SetInitialTCPSeq(0x11223344, 0x55667788)

	hello := BuildFakeClientHello("www.pixiv.net")
	if _, err := dc.Write(hello); err != nil {
		t.Fatalf("DesyncConn.Write failed: %v", err)
	}
	if len(ttls) < 2 || ttls[0] != preset.TTL || ttls[1] != 64 {
		t.Fatalf("expected FakeTTL to drop TTL to %d then restore 64, got %v", preset.TTL, ttls)
	}
	if gotSeq != 0x11223344 || gotAck != 0x55667788 || gotTTL != uint8(preset.TTL) {
		t.Fatalf("expected live seq/ack/ttl (0x11223344, 0x55667788, %d), got (0x%x, 0x%x, %d)", preset.TTL, gotSeq, gotAck, gotTTL)
	}
}

func TestExtractSeqAckFromSYNACK(t *testing.T) {
	// Build IPv4 header with 4 bytes of IP options (IHL=6 -> 24 bytes) + 20-byte TCP SYN-ACK.
	pkt := make([]byte, 44)
	pkt[0] = 0x46 // IPv4, IHL=6 (24 bytes)
	pkt[9] = 6    // TCP
	src := netip.MustParseAddr("104.16.132.229")
	s4 := src.As4()
	copy(pkt[12:16], s4[:])
	tcp := pkt[24:]
	binary.BigEndian.PutUint16(tcp[0:2], 443)
	binary.BigEndian.PutUint16(tcp[2:4], 54321)
	binary.BigEndian.PutUint32(tcp[4:8], 0xAABBCCDD)  // server ISN
	binary.BigEndian.PutUint32(tcp[8:12], 0x12345679) // ack of client SYN (client next seq)
	tcp[12] = 5 << 4
	tcp[13] = 0x12 // SYN | ACK

	seq, ack, ok := ExtractSeqAckFromSYNACK(pkt, src, 443, 54321)
	if !ok {
		t.Fatalf("expected SYN-ACK extraction to succeed")
	}
	if seq != 0x12345679 || ack != 0xAABBCCDE {
		t.Fatalf("unexpected seq/ack: got (0x%x, 0x%x), want (0x12345679, 0xAABBCCDE)", seq, ack)
	}

	// Non-SYN-ACK (pure ACK) must be ignored.
	tcp[13] = 0x10
	if _, _, ok := ExtractSeqAckFromSYNACK(pkt, src, 443, 54321); ok {
		t.Fatalf("expected pure ACK to be rejected")
	}
}
