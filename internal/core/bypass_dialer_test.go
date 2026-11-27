package core

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/bootstrap"
	"github.com/mumu-lhl/my-vpn/internal/config"
)

type recordingConn struct {
	net.Conn
	mu     sync.Mutex
	writes [][]byte
}

func (r *recordingConn) Write(p []byte) (int, error) {
	r.mu.Lock()
	cp := make([]byte, len(p))
	copy(cp, p)
	r.writes = append(r.writes, cp)
	r.mu.Unlock()
	return len(p), nil
}

func (r *recordingConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (r *recordingConn) Close() error                       { return nil }
func (r *recordingConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (r *recordingConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (r *recordingConn) SetDeadline(_ time.Time) error      { return nil }
func (r *recordingConn) SetReadDeadline(_ time.Time) error  { return nil }
func (r *recordingConn) SetWriteDeadline(_ time.Time) error { return nil }

func buildSampleClientHello(sni string) []byte {
	// Minimal syntactically valid TLS 1.2 ClientHello handshake record carrying SNI.
	sniBytes := []byte(sni)
	body := append([]byte{0x01, 0x00, 0x00, byte(len(sniBytes) + 40)}, bytes.Repeat([]byte{0xAA}, 38)...)
	body = append(body, 0x00, byte(len(sniBytes)))
	body = append(body, sniBytes...)
	rec := make([]byte, 5+len(body))
	rec[0] = 0x16
	rec[1] = 0x03
	rec[2] = 0x01
	binary.BigEndian.PutUint16(rec[3:5], uint16(len(body)))
	copy(rec[5:], body)
	return rec
}

func TestBypassConnTLSClientHelloTCPAndRecordFragmentation(t *testing.T) {
	hello := buildSampleClientHello("www.pixiv.net")
	origPayload := hello[5:]

	t.Run("TLS record fragmentation reassembles identically", func(t *testing.T) {
		rec := &recordingConn{}
		var sleeps []time.Duration
		bc := newBypassConn(rec, config.DirectBypassConfig{
			TLSFragmentSize:   "10-20",
			TLSFragmentSleep:  "1-3ms",
			TLSRecordFragment: true,
		}, func(d time.Duration) { sleeps = append(sleeps, d) })

		n, err := bc.Write(hello)
		if err != nil || n != len(hello) {
			t.Fatalf("Write returned (%d, %v), want (%d, nil)", n, err, len(hello))
		}
		if len(rec.writes) < 2 {
			t.Fatalf("expected >= 2 fragmented TLS records, got %d", len(rec.writes))
		}
		if len(sleeps) != len(rec.writes)-1 {
			t.Fatalf("expected %d jitter sleeps, got %d", len(rec.writes)-1, len(sleeps))
		}
		var reassembled []byte
		for i, w := range rec.writes {
			if len(w) < 6 || w[0] != 0x16 || w[1] != 0x03 || w[2] != 0x01 {
				t.Fatalf("record %d has invalid TLS header: %x", i, w[:min(len(w), 5)])
			}
			rlen := int(binary.BigEndian.Uint16(w[3:5]))
			if rlen != len(w)-5 {
				t.Fatalf("record %d length mismatch: header=%d actual=%d", i, rlen, len(w)-5)
			}
			reassembled = append(reassembled, w[5:]...)
		}
		if !bytes.Equal(reassembled, origPayload) {
			t.Fatalf("reassembled ClientHello payload mismatch")
		}

		// Subsequent writes must pass through unfragmented and without extra sleeps.
		prevWrites := len(rec.writes)
		prevSleeps := len(sleeps)
		if _, err := bc.Write([]byte("second-flight-application-data")); err != nil {
			t.Fatalf("second Write failed: %v", err)
		}
		if len(rec.writes) != prevWrites+1 || len(sleeps) != prevSleeps {
			t.Fatalf("second write should pass through directly")
		}
	})

	t.Run("TCP-only fragmentation preserves exact bytes", func(t *testing.T) {
		rec := &recordingConn{}
		bc := newBypassConn(rec, config.DirectBypassConfig{
			TLSFragmentSize:   "8-16",
			TLSFragmentSleep:  "0ms",
			TLSRecordFragment: false,
		}, func(time.Duration) {})

		if _, err := bc.Write(hello); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if len(rec.writes) < 2 {
			t.Fatalf("expected >= 2 TCP chunks, got %d", len(rec.writes))
		}
		if !bytes.Equal(bytes.Join(rec.writes, nil), hello) {
			t.Fatalf("concatenated TCP chunks do not match original ClientHello")
		}
	})
}

func TestBypassConnPlainHTTPHostObfuscation(t *testing.T) {
	req := []byte("GET /index.html HTTP/1.1\r\nHost: interfered.example.org\r\nUser-Agent: test\r\n\r\n")
	rec := &recordingConn{}
	var sleepCount int
	bc := newBypassConn(rec, config.DirectBypassConfig{
		TLSFragmentSize:  "12-20",
		TLSFragmentSleep: "1-2ms",
		HTTPMixHostCase:  true,
	}, func(time.Duration) { sleepCount++ })

	n, err := bc.Write(req)
	if err != nil || n != len(req) {
		t.Fatalf("Write returned (%d, %v)", n, err)
	}
	if len(rec.writes) < 2 || sleepCount != len(rec.writes)-1 {
		t.Fatalf("expected fragmented HTTP request with jitter, writes=%d sleeps=%d", len(rec.writes), sleepCount)
	}
	joined := bytes.Join(rec.writes, nil)
	if !bytes.Contains(joined, []byte("\r\nHoSt: interfered.example.org\r\n")) {
		t.Fatalf("expected mixed-case HoSt header, got %q", string(joined))
	}
}

func TestCompileSingBoxDirectBypassServerlessMode(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Chain.Hop1 = config.HopRef{Kind: config.HopDirectBypass}
	cfg.Chain.TwoHop = true
	cfg.Inbound.Mode = config.InboundTUN
	cfg.Routing.DirectBypass.Scope = config.DirectBypassScopeGlobal
	cfg.Routing.DirectBypass.DoHURL = "https://1.0.0.1/dns-query"

	res, err := CompileSingBoxConfig(cfg, 0)
	if err != nil {
		t.Fatalf("CompileSingBoxConfig failed: %v", err)
	}
	if res.FinalOutboundTag != "direct-bypass" {
		t.Fatalf("expected final_outbound_tag=direct-bypass, got %q", res.FinalOutboundTag)
	}
	if res.Hop1ProbePort != 0 {
		t.Fatalf("expected no hop1_probe_port in single-hop serverless mode, got %d", res.Hop1ProbePort)
	}
	if len(res.Warnings) == 0 || res.Warnings[0].Code != "DIRECT_BYPASS_SINGLE_HOP" {
		t.Fatalf("expected DIRECT_BYPASS_SINGLE_HOP warning, got %+v", res.Warnings)
	}

	// Verify list scope sets final outbound to plain direct.
	cfg.Chain.TwoHop = false
	cfg.Routing.DirectBypass.Scope = config.DirectBypassScopeList
	resList, err := CompileSingBoxConfig(cfg, 0)
	if err != nil {
		t.Fatalf("CompileSingBoxConfig list scope failed: %v", err)
	}
	if resList.FinalOutboundTag != "direct" {
		t.Fatalf("expected final_outbound_tag=direct in list scope, got %q", resList.FinalOutboundTag)
	}
}

func TestServerlessBypassEngineLoopbackWithMockDoH(t *testing.T) {
	var dohCalls atomic.Int32
	dohSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dohCalls.Add(1)
		w.Header().Set("Content-Type", "application/dns-json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Status": 0,
			"Answer": []map[string]any{
				{"type": 1, "TTL": 60, "data": "127.0.0.1"},
			},
		})
	}))
	defer dohSrv.Close()
	dohURL, _ := url.Parse(dohSrv.URL)

	// Local TCP peer that echoes back whatever it receives so we can inspect the fragmented bytes.
	peerLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen peer: %v", err)
	}
	defer peerLn.Close()
	peerPort := peerLn.Addr().(*net.TCPAddr).Port

	receivedCh := make(chan []byte, 1)
	go func() {
		conn, err := peerLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 4096)
		var all []byte
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				all = append(all, buf[:n]...)
				if len(all) >= 50 {
					_, _ = conn.Write([]byte("OK"))
					break
				}
			}
			if err != nil {
				break
			}
		}
		receivedCh <- all
	}()

	cfg := config.DefaultConfig()
	cfg.Chain = config.ChainConfig{
		TwoHop: false,
		Hop1:   config.HopRef{Kind: config.HopDirectBypass},
	}
	cfg.Inbound.Mode = config.InboundProxy
	cfg.Inbound.SocksPort = freeLocalPort(t)
	cfg.Inbound.HTTPPort = freeLocalPort(t)
	cfg.Routing.DirectBypass.Scope = config.DirectBypassScopeGlobal
	cfg.Routing.DirectBypass.TLSFragmentSize = "12-18"
	cfg.Routing.DirectBypass.TLSFragmentSleep = "0ms"
	cfg.Routing.DirectBypass.TLSRecordFragment = true

	mockResolver := bootstrap.NewResolver(bootstrap.Options{
		Endpoints: []bootstrap.DoHEndpoint{
			{URL: fmt.Sprintf("http://mock-doh.invalid:%s/dns-query", dohURL.Port()), StaticIPs: []string{"127.0.0.1"}},
		},
		Timeout: 2 * time.Second,
	})

	eng := NewEngine(t.TempDir())
	eng.SetBypassDialer(NewBypassDialerWithDeps(cfg.Routing.DirectBypass, mockResolver, nil, func(time.Duration) {}))
	if _, err := eng.Start(context.Background(), cfg, 0); err != nil {
		t.Fatalf("Engine.Start failed: %v", err)
	}
	defer func() { _ = eng.Stop() }()

	conn, err := dialViaSocks5(fmt.Sprintf("127.0.0.1:%d", cfg.Inbound.SocksPort), "interfered.example.net", peerPort)
	if err != nil {
		t.Fatalf("SOCKS5 dial failed: %v", err)
	}
	defer conn.Close()

	hello := buildSampleClientHello("interfered.example.net")
	if _, err := conn.Write(hello); err != nil {
		t.Fatalf("write hello via SOCKS5: %v", err)
	}
	ack := make([]byte, 2)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(conn, ack); err != nil || string(ack) != "OK" {
		t.Fatalf("expected OK ack from peer, got %q (%v)", string(ack), err)
	}

	if dohCalls.Load() < 1 {
		t.Fatalf("expected mock DoH server to be queried for non-CN domain")
	}
	raw := <-receivedCh
	// Because TLSRecordFragment=true splits 1 record into multiple 5-byte-headed records,
	// received wire length must strictly exceed original single-record ClientHello length.
	if len(raw) <= len(hello) {
		t.Fatalf("expected multi-record fragmented TLS wire bytes (> %d), got %d", len(hello), len(raw))
	}
}

func freeLocalPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freeLocalPort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}
