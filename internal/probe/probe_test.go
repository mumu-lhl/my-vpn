package probe

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/config"
)

func TestCountryCodeToFlagAC20(t *testing.T) {
	cases := map[string]string{
		"JP":  "🇯🇵",
		"us":  "🇺🇸",
		"TW":  "🇹🇼",
		"NL":  "🇳🇱",
		"":    "🌐",
		"123": "🌐",
	}
	for in, want := range cases {
		if got := CountryCodeToFlag(in); got != want {
			t.Fatalf("CountryCodeToFlag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseIPInfoMultipleProvidersAC20(t *testing.T) {
	samples := []struct {
		name   string
		body   string
		wantIP string
		wantCC string
	}{
		{
			name:   "ipwho.is",
			body:   `{"success":true,"ip":"104.28.1.2","country_code":"JP","city":"Tokyo","connection":{"isp":"Cloudflare"}}`,
			wantIP: "104.28.1.2",
			wantCC: "JP",
		},
		{
			name:   "ifconfig.co",
			body:   `{"ip":"45.67.89.10","country_iso":"NL","city":"Amsterdam","asn_org":"M247"}`,
			wantIP: "45.67.89.10",
			wantCC: "NL",
		},
		{
			name:   "cloudflare-trace",
			body:   "fl=123f\nh=www.cloudflare.com\nip=198.51.100.7\nloc=SG\n",
			wantIP: "198.51.100.7",
			wantCC: "SG",
		},
	}

	for _, tc := range samples {
		info, err := ParseIPInfo([]byte(tc.body), tc.name)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if info.IP != tc.wantIP || info.CountryCode != tc.wantCC || info.Flag != CountryCodeToFlag(tc.wantCC) {
			t.Fatalf("%s: got %+v", tc.name, info)
		}
	}
}

func TestProbeChainPerHopConnectivityAndExitIPAC19AC20(t *testing.T) {
	// Mock connectivity + GeoIP server distinguishing Hop 1 vs Hop 2 via request header injected by ClientForProxy.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hop := r.Header.Get("X-Test-Hop")
		if r.URL.Path == "/generate_204" {
			if hop == "hop2" {
				time.Sleep(15 * time.Millisecond)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if hop == "hop1" {
			_, _ = w.Write([]byte(`{"success":true,"ip":"203.0.113.11","country_code":"JP","city":"Tokyo"}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"ip":"198.51.100.22","country_code":"US","city":"Los Angeles"}`))
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Chain.TwoHop = true
	cfg.Chain.Hop1 = config.HopRef{Kind: config.HopGeph}
	cfg.Chain.Hop2 = config.HopRef{Kind: config.HopWarp}
	cfg.Probe.Mode = "custom"
	cfg.Probe.CustomURL = srv.URL + "/generate_204"

	hop1Addr := fmt.Sprintf("127.0.0.1:%d", cfg.Geph.LocalSocksPort)
	hop2Addr := fmt.Sprintf("127.0.0.1:%d", cfg.Inbound.SocksPort)

	prober := NewProber()
	prober.IPInfoEndpoints = []string{srv.URL + "/geoip"}
	prober.ClientForProxy = func(socksAddr string, timeout time.Duration) *http.Client {
		tag := "hop2"
		if socksAddr == hop1Addr {
			tag = "hop1"
		}
		return &http.Client{
			Timeout: timeout,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				cloned := req.Clone(req.Context())
				cloned.Header.Set("X-Test-Hop", tag)
				return http.DefaultTransport.RoundTrip(cloned)
			}),
		}
	}

	res := prober.ProbeChain(context.Background(), cfg)
	if res.TestURL != cfg.Probe.CustomURL {
		t.Fatalf("expected custom URL %q, got %q", cfg.Probe.CustomURL, res.TestURL)
	}
	if len(res.Hops) != 2 {
		t.Fatalf("expected 2 hop probe results, got %d", len(res.Hops))
	}
	if res.Hops[0].ProxyAddr != hop1Addr || res.Hops[1].ProxyAddr != hop2Addr {
		t.Fatalf("unexpected hop proxy addrs: %+v", res.Hops)
	}
	if !res.Hops[0].Connectivity.Reachable || !res.Hops[1].Connectivity.Reachable {
		t.Fatalf("expected both hops reachable: %+v", res.Hops)
	}
	if res.Hops[0].ExitIP.CountryCode != "JP" || res.Hops[0].ExitIP.Flag != "🇯🇵" || res.Hops[0].ExitIP.IP != "203.0.113.11" {
		t.Fatalf("unexpected hop1 exit IP: %+v", res.Hops[0].ExitIP)
	}
	if res.Hops[1].ExitIP.CountryCode != "US" || res.Hops[1].ExitIP.Flag != "🇺🇸" || res.Hops[1].ExitIP.IP != "198.51.100.22" {
		t.Fatalf("unexpected hop2 exit IP: %+v", res.Hops[1].ExitIP)
	}
	if len(BuiltinTestURLs) < 4 || !strings.Contains(BuiltinTestURLs[0].URL, "generate_204") {
		t.Fatalf("expected Hiddify-style builtin connectivity test URLs")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
