package routing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/mumu-lhl/my-vpn/internal/config"
)

func TestRoutingRulesAndAntiLeakAC05AC07(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Routing.CustomRules = []config.CustomRule{
		{ID: "r1", Enabled: true, DomainSuffix: []string{"corp.internal"}, Action: "direct"},
		{ID: "r2", Enabled: true, ProcessName: []string{"curl-blocked"}, Action: "reject"},
		{ID: "r3", Enabled: true, IPCIDR: []string{"203.0.113.0/24"}, Port: []int{443}, Action: "direct"},
	}

	if got := Evaluate(cfg.Routing, cfg.DNS, MatchQuery{Domain: "broker.geph.io", ProcessName: "geph5-client"}, true, false); got != "direct" {
		t.Fatalf("geph5-client process must bypass tunnel to prevent loop, got %s", got)
	}
	if got := Evaluate(cfg.Routing, cfg.DNS, MatchQuery{IP: "192.168.1.50"}, true, false); got != "direct" {
		t.Fatalf("private IP should be direct, got %s", got)
	}
	if got := Evaluate(cfg.Routing, cfg.DNS, MatchQuery{Domain: "stats.doubleclick.net"}, true, false); got != "reject" {
		t.Fatalf("ad domain should be rejected, got %s", got)
	}
	if got := Evaluate(cfg.Routing, cfg.DNS, MatchQuery{Domain: "git.corp.internal"}, true, false); got != "direct" {
		t.Fatalf("custom suffix rule failed, got %s", got)
	}
	if got := Evaluate(cfg.Routing, cfg.DNS, MatchQuery{Domain: "example.com", ProcessName: "curl-blocked"}, true, false); got != "reject" {
		t.Fatalf("process_name rule failed, got %s", got)
	}
	if got := Evaluate(cfg.Routing, cfg.DNS, MatchQuery{IP: "203.0.113.10", Port: 443}, true, false); got != "direct" {
		t.Fatalf("IPCIDR+Port rule failed, got %s", got)
	}
	if got := Evaluate(cfg.Routing, cfg.DNS, MatchQuery{Domain: "www.baidu.com"}, true, false); got != "direct" {
		t.Fatalf("CN domain should be direct, got %s", got)
	}
	if got := Evaluate(cfg.Routing, cfg.DNS, MatchQuery{Domain: "stun.l.google.com", Network: "udp"}, false, false); got != "reject" {
		t.Fatalf("non-direct UDP on TCP-only chain must be rejected, got %s", got)
	}
	if got := Evaluate(cfg.Routing, cfg.DNS, MatchQuery{IP: "2606:4700:4700::1111", Network: "tcp"}, true, false); got != "reject" {
		t.Fatalf("non-direct IPv6 without IPv6 exit must be rejected, got %s", got)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("SRS-BINARY-MOCK"))
	}))
	defer srv.Close()
	dest, err := DownloadRuleSet(context.Background(), srv.Client(), "geoip-cn", srv.URL, t.TempDir())
	if err != nil {
		t.Fatalf("DownloadRuleSet: %v", err)
	}
	b, _ := os.ReadFile(dest)
	if string(b) != "SRS-BINARY-MOCK" {
		t.Fatalf("unexpected rule-set content: %q", string(b))
	}
}
