package core

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/config"
	"github.com/mumu-lhl/my-vpn/internal/routing"
)

type TrafficStats struct {
	Running   bool   `json:"running"`
	UpBytes   int64  `json:"up_bytes"`
	DownBytes int64  `json:"down_bytes"`
	Mode      string `json:"mode"`
	ChainDesc string `json:"chain_desc"`
}

// Engine manages the lifecycle of the local proxy / sing-box core.
// If MYVPN_SINGBOX_BIN is set and executable, it runs hiddify-sing-box directly;
// otherwise it runs the built-in Go SOCKS5/HTTP routing & chaining proxy engine
// (used for local dev, unit/integration loopback tests, and proxy-only fallback).
type Engine struct {
	mu           sync.Mutex
	running      bool
	cfg          *config.AppConfig
	compileRes   *CompileResult
	workDir      string
	singboxBin   string
	cmd          *exec.Cmd
	socksLn      net.Listener
	httpSrv      *http.Server
	upstreamAddr string // optional SOCKS5 upstream for loopback/sidecar chaining
	bypassDialer *BypassDialer
	customBypass bool
	upBytes      atomic.Int64
	downBytes    atomic.Int64
}

func NewEngine(workDir string) *Engine {
	return &Engine{
		workDir:    workDir,
		singboxBin: os.Getenv("MYVPN_SINGBOX_BIN"),
	}
}

func (e *Engine) SetUpstreamSocks(addr string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.upstreamAddr = addr
}

// SetBypassDialer injects a custom BypassDialer (used by tests to wire mock DoH/TCP peers).
func (e *Engine) SetBypassDialer(d *BypassDialer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bypassDialer = d
	e.customBypass = d != nil
}

func (e *Engine) Start(ctx context.Context, cfg *config.AppConfig, tunFD int) (*CompileResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		_ = e.stopLocked()
	}

	res, err := CompileSingBoxConfig(cfg, tunFD)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(e.workDir, 0o700); err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(e.workDir, "sing-box.json")
	if err := os.WriteFile(cfgPath, res.JSON, 0o600); err != nil {
		return nil, err
	}

	e.cfg = cfg
	e.compileRes = res
	if !e.customBypass {
		e.bypassDialer = NewBypassDialer(cfg.Routing.DirectBypass)
	}

	if e.singboxBin != "" {
		cmd := exec.CommandContext(ctx, e.singboxBin, "run", "-c", cfgPath)
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("start hiddify-sing-box: %w", err)
		}
		e.cmd = cmd
		e.running = true
		return res, nil
	}

	host := "127.0.0.1"
	if cfg.Inbound.AllowLAN {
		host = "0.0.0.0"
	}
	socksLn, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, cfg.Inbound.SocksPort))
	if err != nil {
		return nil, fmt.Errorf("listen socks5: %w", err)
	}
	httpLn, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, cfg.Inbound.HTTPPort))
	if err != nil {
		_ = socksLn.Close()
		return nil, fmt.Errorf("listen http: %w", err)
	}

	e.socksLn = socksLn
	httpSrv := &http.Server{
		Handler: http.HandlerFunc(e.handleHTTP),
	}
	e.httpSrv = httpSrv
	e.running = true

	go e.acceptSocksLoop(socksLn)
	go func() { _ = httpSrv.Serve(httpLn) }()
	return res, nil
}

func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stopLocked()
}

func (e *Engine) stopLocked() error {
	if !e.running {
		return nil
	}
	e.running = false
	if e.cmd != nil && e.cmd.Process != nil {
		_ = e.cmd.Process.Kill()
		_, _ = e.cmd.Process.Wait()
		e.cmd = nil
	}
	if e.socksLn != nil {
		_ = e.socksLn.Close()
		e.socksLn = nil
	}
	if e.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = e.httpSrv.Shutdown(ctx)
		cancel()
		e.httpSrv = nil
	}
	return nil
}

func (e *Engine) Stats() TrafficStats {
	e.mu.Lock()
	defer e.mu.Unlock()
	desc := ""
	mode := ""
	if e.cfg != nil {
		mode = string(e.cfg.Inbound.Mode)
		if e.cfg.Chain.Hop1.Kind == config.HopDirectBypass {
			desc = string(config.HopDirectBypass)
		} else if e.cfg.Chain.TwoHop {
			desc = fmt.Sprintf("%s -> %s", e.cfg.Chain.Hop1.Kind, e.cfg.Chain.Hop2.Kind)
		} else {
			desc = string(e.cfg.Chain.Hop1.Kind)
		}
	}
	return TrafficStats{
		Running:   e.running,
		UpBytes:   e.upBytes.Load(),
		DownBytes: e.downBytes.Load(),
		Mode:      mode,
		ChainDesc: desc,
	}
}

func (e *Engine) dialTarget(targetHost string, targetPort int) (net.Conn, error) {
	e.mu.Lock()
	cfg := e.cfg
	up := e.upstreamAddr
	bd := e.bypassDialer
	supportsUDP := true
	if e.compileRes != nil {
		supportsUDP = e.compileRes.ChainSupportsUDP
	}
	e.mu.Unlock()

	if cfg != nil {
		action := routing.Evaluate(cfg.Routing, cfg.DNS, routing.MatchQuery{
			Domain:  targetHost,
			IP:      targetHost,
			Port:    targetPort,
			Network: "tcp",
		}, supportsUDP, false)
		if action == "reject" {
			return nil, errors.New("blocked by routing rule")
		}
		if cfg.Chain.Hop1.Kind == config.HopDirectBypass && action == "proxy" {
			if cfg.Routing.DirectBypass.Scope == config.DirectBypassScopeList {
				action = "direct"
			} else {
				action = "direct-bypass"
			}
		}
		if action == "direct-bypass" && bd != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return bd.Dial(ctx, targetHost, targetPort)
		}
		if action == "direct" || up == "" {
			return net.DialTimeout("tcp", net.JoinHostPort(targetHost, fmt.Sprintf("%d", targetPort)), 5*time.Second)
		}
	}
	if up != "" {
		return dialViaSocks5(up, targetHost, targetPort)
	}
	return net.DialTimeout("tcp", net.JoinHostPort(targetHost, fmt.Sprintf("%d", targetPort)), 5*time.Second)
}

func (e *Engine) acceptSocksLoop(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go e.handleSocks5(c)
	}
}

func (e *Engine) handleSocks5(c net.Conn) {
	defer c.Close()
	br := bufio.NewReader(c)
	ver, err := br.ReadByte()
	if err != nil || ver != 0x05 {
		return
	}
	nMethods, err := br.ReadByte()
	if err != nil {
		return
	}
	if _, err := io.CopyN(io.Discard, br, int64(nMethods)); err != nil {
		return
	}
	_, _ = c.Write([]byte{0x05, 0x00})

	hdr := make([]byte, 4)
	if _, err := io.ReadFull(br, hdr); err != nil || hdr[1] != 0x01 {
		_, _ = c.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	var host string
	switch hdr[3] {
	case 0x01:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(br, ip); err != nil {
			return
		}
		host = net.IP(ip).String()
	case 0x03:
		l, err := br.ReadByte()
		if err != nil {
			return
		}
		d := make([]byte, l)
		if _, err := io.ReadFull(br, d); err != nil {
			return
		}
		host = string(d)
	case 0x04:
		ip := make([]byte, 16)
		if _, err := io.ReadFull(br, ip); err != nil {
			return
		}
		host = net.IP(ip).String()
	default:
		return
	}
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(br, portBuf); err != nil {
		return
	}
	port := int(binary.BigEndian.Uint16(portBuf))

	rc, err := e.dialTarget(host, port)
	if err != nil {
		_, _ = c.Write([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer rc.Close()
	_, _ = c.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	e.relay(c, rc)
}

func (e *Engine) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		host, portStr, err := net.SplitHostPort(r.Host)
		if err != nil {
			http.Error(w, "bad host", http.StatusBadRequest)
			return
		}
		var port int
		_, _ = fmt.Sscanf(portStr, "%d", &port)
		rc, err := e.dialTarget(host, port)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		defer rc.Close()
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack unsupported", http.StatusInternalServerError)
			return
		}
		cc, _, err := hj.Hijack()
		if err != nil {
			return
		}
		defer cc.Close()
		_, _ = cc.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		e.relay(cc, rc)
		return
	}

	host := r.URL.Hostname()
	port := 80
	if p := r.URL.Port(); p != "" {
		_, _ = fmt.Sscanf(p, "%d", &port)
	}
	rc, err := e.dialTarget(host, port)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	defer rc.Close()
	tr := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) { return rc, nil },
	}
	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	resp, err := tr.RoundTrip(outReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	n, _ := io.Copy(w, resp.Body)
	e.downBytes.Add(n)
}

func (e *Engine) relay(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		n, _ := io.Copy(b, a)
		e.upBytes.Add(n)
	}()
	go func() {
		defer wg.Done()
		n, _ := io.Copy(a, b)
		e.downBytes.Add(n)
	}()
	wg.Wait()
}

func dialViaSocks5(proxyAddr, targetHost string, targetPort int) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		return nil, err
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
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(targetHost))}
	req = append(req, []byte(targetHost)...)
	p := make([]byte, 2)
	binary.BigEndian.PutUint16(p, uint16(targetPort))
	req = append(req, p...)
	if _, err := c.Write(req); err != nil {
		_ = c.Close()
		return nil, err
	}
	hdr := make([]byte, 10)
	if _, err := io.ReadFull(c, hdr); err != nil || hdr[1] != 0x00 {
		_ = c.Close()
		return nil, errors.New("socks5 connect failed")
	}
	return c, nil
}
