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

	"github.com/mumu-lhl/my-vpn/internal/config"
)

func TestCompileWarpPsiphonGephAndAutoMasqueH2AC02AC04AC06(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Inbound.Mode = config.InboundTUN
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
