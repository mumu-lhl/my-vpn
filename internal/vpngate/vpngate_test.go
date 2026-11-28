package vpngate

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func sampleOVPN(proto, host, port string, withAuth bool) string {
	var b strings.Builder
	b.WriteString("client\n")
	b.WriteString("dev tun\n")
	b.WriteString("proto " + proto + "\n")
	b.WriteString("remote " + host + " " + port + "\n")
	if withAuth {
		b.WriteString("auth-user-pass\n")
	}
	b.WriteString("<ca>\n-----BEGIN CERTIFICATE-----\nMIIBCA\n-----END CERTIFICATE-----\n</ca>\n")
	b.WriteString("<cert>\n-----BEGIN CERTIFICATE-----\nMIIBCERT\n-----END CERTIFICATE-----\n</cert>\n")
	b.WriteString("<key>\n-----BEGIN PRIVATE KEY-----\nMIIBKEY\n-----END PRIVATE KEY-----\n</key>\n")
	return base64.StdEncoding.EncodeToString([]byte(b.String()))
}

func TestParseVPNGateCSVAndFetchACVPNGATE01(t *testing.T) {
	csvBody := strings.Join([]string{
		"*vpn_servers",
		"#HostName,IP,Score,Ping,Speed,CountryLong,CountryShort,NumVpnSessions,Uptime,TotalUsers,TotalTraffic,LogType,Operator,Message,OpenVPN_ConfigData_Base64",
		"vpn-jp-01,203.0.113.10,2500000,18,125000000,Japan,JP,42,864000,1200,9876543210,2weeks,Univ,,"+sampleOVPN("tcp", "203.0.113.10", "1195", true),
		"vpn-us-02,198.51.100.20,1800000,65,85000000,United States,US,15,432000,500,1234567890,2weeks,Volunteer,,"+sampleOVPN("udp", "198.51.100.20", "1194", false),
		"*",
	}, "\n")

	nodes, err := ParseCSV(strings.NewReader(csvBody))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 parsed nodes, got %d", len(nodes))
	}

	jp := nodes[0]
	if jp.CountryShort != "JP" || jp.Network != "tcp" || jp.SupportsUDP || jp.ServerPort != 1195 {
		t.Fatalf("unexpected JP node: %+v", jp)
	}
	if jp.Username != "vpn" || jp.Password != "vpn" || !strings.Contains(jp.CACert, "MIIBCA") {
		t.Fatalf("expected auth & inline CA on JP node: %+v", jp)
	}

	us := nodes[1]
	if us.CountryShort != "US" || us.Network != "udp" || !us.SupportsUDP || us.ServerPort != 1194 {
		t.Fatalf("unexpected US node: %+v", us)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(csvBody))
	}))
	defer srv.Close()

	fetched, err := FetchFromAPI(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("FetchFromAPI: %v", err)
	}
	if len(fetched) != 2 {
		t.Fatalf("expected 2 fetched nodes, got %d", len(fetched))
	}

	cachePath := filepath.Join(t.TempDir(), "vpngate_cache.json")
	if err := SaveCache(cachePath, fetched); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}
	loaded, err := LoadCache(cachePath)
	if err != nil || len(loaded) != 2 || loaded[0].ID != fetched[0].ID {
		t.Fatalf("unexpected LoadCache result: len=%d err=%v", len(loaded), err)
	}
}
