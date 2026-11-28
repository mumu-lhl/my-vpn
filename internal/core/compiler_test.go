package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mumu-lhl/looma/internal/config"
)

func TestCompileWarpPsiphonGephAndAutoMasqueH2AC02AC04AC06(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Inbound.Mode = config.InboundTUN
	cfg.Inbound.KillSwitch = true
	cfg.Chain.TwoHop = true
	cfg.Chain.Hop1 = config.HopRef{Kind: config.HopPsiphon}
	cfg.Chain.Hop2 = config.HopRef{Kind: config.HopWarp}
	cfg.Psiphon.EgressRegion = "JP"
	cfg.Warp.Mode = "wg"
	cfg.Warp.LicenseKey = "WARP-KEY-1"

	res, err := CompileSingBoxConfig(cfg, 42)
	if err != nil {
		t.Fatalf("CompileSingBoxConfig: %v", err)
	}
	if res.FinalOutboundTag != "hop-2" {
		t.Fatalf("expected final tag hop-2, got %s", res.FinalOutboundTag)
	}
	hasAutoWarn := false
	for _, w := range res.Warnings {
		if w.Code == "WARP_AUTO_MASQUE_H2" {
			hasAutoWarn = true
		}
	}
	if !hasAutoWarn {
		t.Fatalf("expected WARP_AUTO_MASQUE_H2 warning, got %+v", res.Warnings)
	}

	var parsed map[string]any
	if err := json.Unmarshal(res.JSON, &parsed); err != nil {
		t.Fatalf("invalid compiled JSON: %v", err)
	}
	eps := parsed["endpoints"].([]any)
	warpEP := eps[0].(map[string]any)
	if warpEP["mode"] != "masque" || warpEP["detour"] != "hop-1" {
		t.Fatalf("expected WARP mode=masque detour=hop-1, got %+v", warpEP)
	}
	masque := warpEP["masque"].(map[string]any)
	if masque["vhttp"] != "h2" {
		t.Fatalf("expected WARP masque vhttp=h2, got %+v", masque)
	}

	rawStr := string(res.JSON)
	if !strings.Contains(rawStr, `"hijack-dns"`) || !strings.Contains(rawStr, `"dns-proxy"`) || !strings.Contains(rawStr, `"file_descriptor": 42`) {
		t.Fatalf("missing anti-leak DNS hijack or TUN fd in compiled config")
	}
	if !strings.Contains(rawStr, `"tag": "direct-bypass"`) || !strings.Contains(rawStr, `"tls_fragment": true`) || !strings.Contains(rawStr, `"pixiv.net"`) {
		t.Fatalf("missing direct-bypass TLS fragment outbound or preset route in compiled config")
	}
	if !strings.Contains(rawStr, `"default_mark": 8230`) {
		t.Fatalf("expected route.default_mark=8230 when KillSwitch is enabled")
	}
	if !strings.Contains(rawStr, `"find_process": true`) || !strings.Contains(rawStr, `"geph5-client"`) {
		t.Fatalf("missing geph5-client anti-loop route/dns bypass in compiled config")
	}
}

func TestLoopbackTwoHopChainingAndAdBlockIntegration(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("HELLO_THROUGH_CHAIN"))
	}))
	defer target.Close()

	p1, p2, p3, p4 := freePort(t), freePort(t), freePort(t), freePort(t)

	cfg1 := config.DefaultConfig()
	cfg1.Inbound.Mode = config.InboundProxy
	cfg1.Inbound.SocksPort = p1
	cfg1.Inbound.HTTPPort = p2
	cfg1.Routing.BypassPrivate = false
	eng1 := NewEngine(t.TempDir())
	if _, err := eng1.Start(context.Background(), cfg1, 0); err != nil {
		t.Fatalf("start hop1: %v", err)
	}
	defer eng1.Stop()

	cfg2 := config.DefaultConfig()
	cfg2.Inbound.Mode = config.InboundProxy
	cfg2.Inbound.SocksPort = p3
	cfg2.Inbound.HTTPPort = p4
	cfg2.Routing.BypassPrivate = false
	cfg2.Routing.BlockAds = true
	eng2 := NewEngine(t.TempDir())
	eng2.SetUpstreamSocks(fmt.Sprintf("127.0.0.1:%d", p1))
	if _, err := eng2.Start(context.Background(), cfg2, 0); err != nil {
		t.Fatalf("start hop2: %v", err)
	}
	defer eng2.Stop()

	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", p4))
	cli := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}}
	resp, err := cli.Get(target.URL)
	if err != nil {
		t.Fatalf("chained HTTP request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "HELLO_THROUGH_CHAIN" {
		t.Fatalf("unexpected body: %q", string(body))
	}
	if eng1.Stats().UpBytes == 0 && eng1.Stats().DownBytes == 0 {
		t.Fatalf("expected traffic to traverse hop1, stats=%+v", eng1.Stats())
	}

	respAd, err := cli.Get("http://stats.doubleclick.net/pixel")
	if err != nil {
		t.Fatalf("ad request err: %v", err)
	}
	_ = respAd.Body.Close()
	if respAd.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for blocked ad domain, got %d", respAd.StatusCode)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestCompileSingBoxDirectBypassSocksBridge(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Chain.Hop1 = config.HopRef{Kind: config.HopDirectBypass}
	cfg.Chain.TwoHop = false

	res, err := CompileSingBoxConfigWithOptions(cfg, 0, CompileOptions{BypassSocksPort: 19999})
	if err != nil {
		t.Fatalf("CompileSingBoxConfigWithOptions failed: %v", err)
	}
	outbounds, ok := res.Config["outbounds"].([]map[string]any)
	if !ok {
		t.Fatalf("expected outbounds slice")
	}
	var found bool
	for _, ob := range outbounds {
		if ob["tag"] == "direct-bypass" {
			found = true
			if ob["type"] != "socks" || ob["server"] != "127.0.0.1" || ob["server_port"] != 19999 {
				t.Fatalf("expected direct-bypass bridged to socks 127.0.0.1:19999, got %+v", ob)
			}
		}
	}
	if !found {
		t.Fatalf("direct-bypass outbound not found")
	}
}

func TestCompileVPNGateOpenVPNClientEndpointAndChainACVPNGATE02(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Chain.TwoHop = true
	cfg.Chain.Hop1 = config.HopRef{Kind: config.HopVPNGate}
	cfg.Chain.Hop2 = config.HopRef{Kind: config.HopWarp}
	cfg.VPNGate.SelectedNode = &config.VPNGateSelectedNode{
		ID:           "vpn-jp-01-203.0.113.10-tcp-1195",
		HostName:     "vpn-jp-01",
		IP:           "203.0.113.10",
		CountryLong:  "Japan",
		CountryShort: "JP",
		Server:       "203.0.113.10",
		ServerPort:   1195,
		Network:      "tcp",
		SupportsUDP:  false,
		CACert:       "CA-PEM",
		ClientCert:   "CERT-PEM",
		ClientKey:    "KEY-PEM",
		Username:     "vpn",
		Password:     "vpn",
	}

	res, err := CompileSingBoxConfig(cfg, 0)
	if err != nil {
		t.Fatalf("CompileSingBoxConfig: %v", err)
	}
	endpoints, ok := res.Config["endpoints"].([]map[string]any)
	if !ok || len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints (openvpn-client + warp), got %+v", res.Config["endpoints"])
	}
	if endpoints[0]["type"] != "openvpn-client" || endpoints[0]["network"] != "tcp" || endpoints[0]["server_port"] != 1195 {
		t.Fatalf("unexpected openvpn-client endpoint: %+v", endpoints[0])
	}
	if endpoints[1]["type"] != "warp" || endpoints[1]["detour"] != "hop-1" {
		t.Fatalf("expected hop-2 warp detoured via hop-1, got %+v", endpoints[1])
	}
	var hasAutoH2 bool
	for _, w := range res.Warnings {
		if w.Code == "WARP_AUTO_MASQUE_H2" {
			hasAutoH2 = true
		}
	}
	if !hasAutoH2 {
		t.Fatalf("expected WARP_AUTO_MASQUE_H2 warning when hop1 is TCP VPNGate, warnings=%+v", res.Warnings)
	}
}
