package subscription

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mumu-lhl/my-vpn/internal/config"
)

func TestParseAllFormatsAC03(t *testing.T) {
	sbJSON := `{"outbounds":[{"type":"vless","tag":"sb-jp","server":"jp.example.com","server_port":443,"uuid":"1111"},{"type":"direct","tag":"direct"}]}`
	nodes, err := ParseAuto([]byte(sbJSON), "sub1")
	if err != nil || len(nodes) != 1 || nodes[0].Protocol != "vless" || nodes[0].Name != "sb-jp" {
		t.Fatalf("sing-box JSON parse failed: err=%v nodes=%+v", err, nodes)
	}

	clashYAML := `
proxies:
  - name: clash-sg
    type: trojan
    server: sg.example.com
    port: 443
    password: secret
    sni: sg.example.com
    udp: true
  - name: clash-hy2
    type: hysteria2
    server: us.example.com
    port: 8443
    password: pass
`
	nodes, err = ParseAuto([]byte(clashYAML), "sub2")
	if err != nil || len(nodes) != 2 || nodes[0].Protocol != "trojan" || nodes[1].Protocol != "hysteria2" {
		t.Fatalf("Clash YAML parse failed: err=%v nodes=%+v", err, nodes)
	}

	vmessObj := base64.StdEncoding.EncodeToString([]byte(`{"add":"vm.example.com","port":"443","id":"uuid-vm","aid":"0","ps":"vm-node","sni":"vm.example.com"}`))
	ssUser := base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:sspass"))
	uriBundle := "" +
		"vless://uuid-1@vless.example.com:443?sni=vless.example.com&pbk=pub123&sid=ab#vless-node\n" +
		"vmess://" + vmessObj + "\n" +
		"trojan://tpass@trojan.example.com:443?sni=trojan.example.com#trojan-node\n" +
		"ss://" + ssUser + "@ss.example.com:8388#ss-node\n" +
		"hy2://hpass@hy2.example.com:443?sni=hy2.example.com#hy2-node\n" +
		"tuic://tuuid:tpass@tuic.example.com:443?sni=tuic.example.com#tuic-node\n" +
		"wg://privkey@wg.example.com:51820?publickey=pubkey&address=10.0.0.2/32#wg-node\n"

	encoded := base64.StdEncoding.EncodeToString([]byte(uriBundle))
	nodes, err = ParseAuto([]byte(encoded), "sub3")
	if err != nil {
		t.Fatalf("Base64 URI bundle parse failed: %v", err)
	}
	if len(nodes) != 7 {
		t.Fatalf("expected 7 nodes across all URI protocols, got %d", len(nodes))
	}
	wantProtos := []string{"vless", "vmess", "trojan", "shadowsocks", "hysteria2", "tuic", "wireguard"}
	for i, wp := range wantProtos {
		if nodes[i].Protocol != wp {
			t.Errorf("node[%d]: got %s, want %s", i, nodes[i].Protocol, wp)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(encoded))
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client()}
	fetched, err := f.Fetch(context.Background(), config.Subscription{ID: "sub-http", URL: srv.URL})
	if err != nil || len(fetched) != 7 {
		t.Fatalf("HTTP Fetch failed: err=%v len=%d", err, len(fetched))
	}
}
