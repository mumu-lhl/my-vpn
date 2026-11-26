package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/config"
	"github.com/mumu-lhl/my-vpn/internal/core"
	"github.com/mumu-lhl/my-vpn/internal/ipc"
)

func TestFullPipelineCoreHelperIPCAndAntiLeakConfig(t *testing.T) {
	workDir := t.TempDir()
	sock := fmt.Sprintf("/tmp/looma-e2e-%d.sock", time.Now().UnixNano())
	defer os.Remove(sock)

	eng := core.NewEngine(workDir)
	srv := ipc.NewServer(sock, eng)
	if err := srv.Start(); err != nil {
		t.Fatalf("start ipc server: %v", err)
	}
	defer srv.Close()

	cli := ipc.NewClient(sock)
	cfg := config.DefaultConfig()
	cfg.Inbound.Mode = config.InboundProxy
	cfg.Inbound.AllowLAN = true
	cfg.Inbound.SocksPort = freePort(t)
	cfg.Inbound.HTTPPort = freePort(t)
	cfg.Chain.TwoHop = true
	cfg.Chain.Hop1 = config.HopRef{Kind: config.HopPsiphon}
	cfg.Chain.Hop2 = config.HopRef{Kind: config.HopWarp}

	res, err := cli.Start(context.Background(), cfg, 0)
	if err != nil || !res.OK {
		t.Fatalf("start via IPC: err=%v res=%+v", err, res)
	}
	defer cli.Stop(context.Background())

	raw, err := os.ReadFile(workDir + "/sing-box.json")
	if err != nil {
		t.Fatalf("read compiled sing-box.json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("invalid sing-box.json: %v", err)
	}
	inbounds := doc["inbounds"].([]any)
	firstIn := inbounds[0].(map[string]any)
	if firstIn["listen"] != "0.0.0.0" {
		t.Fatalf("AllowLAN=true must listen on 0.0.0.0, got %v", firstIn["listen"])
	}
	foundHop1Probe := false
	for _, rawIn := range inbounds {
		if m, ok := rawIn.(map[string]any); ok && m["tag"] == "hop1-probe-in" && m["listen"] == "127.0.0.1" {
			foundHop1Probe = true
		}
	}
	if !foundHop1Probe {
		t.Fatalf("expected loopback hop1-probe-in inbound for 2-hop per-hop probing")
	}
}

func TestAppIdentityContainsNoSensitiveKeywordsAC21(t *testing.T) {
	root := filepath.Join("..", "..")
	checks := []struct {
		file     string
		mustHave []string
		forbid   []string
	}{
		{
			file:     "mygo.config.ts",
			mustHave: []string{`name: "Looma"`, `identifier: "io.github.mumulhl.looma"`},
			forbid:   []string{"vpn", "proxy", "tunnel"},
		},
		{
			file:     "frontend/index.html",
			mustHave: []string{"<title>Looma</title>"},
			forbid:   []string{"vpn", "proxy"},
		},
		{
			file:     "cmd/app/main.go",
			mustHave: []string{`app.SetName("Looma")`, `Title:     "Looma"`},
			forbid:   []string{"MyVPN", "my-vpn"},
		},
	}

	for _, tc := range checks {
		data, err := os.ReadFile(filepath.Join(root, tc.file))
		if err != nil {
			t.Fatalf("read %s: %v", tc.file, err)
		}
		text := string(data)
		for _, want := range tc.mustHave {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", tc.file, want)
			}
		}
		lower := strings.ToLower(text)
		for _, bad := range tc.forbid {
			if strings.Contains(lower, strings.ToLower(bad)) {
				t.Fatalf("%s must not contain sensitive keyword %q", tc.file, bad)
			}
		}
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
