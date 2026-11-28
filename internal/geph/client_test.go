package geph

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mumu-lhl/looma/internal/config"
)

func TestGephYAMLAndExitFetchAC01(t *testing.T) {
	if _, err := GenerateYAML(config.GephConfig{}); err == nil {
		t.Fatalf("expected error for empty geph secret")
	}

	raw, err := GenerateYAML(config.GephConfig{
		Secret:         "geph-secret-abc",
		ExitCountry:    "jp",
		ExitCity:       "Tokyo",
		LocalSocksPort: 19909,
		ControlPort:    19910,
	})
	if err != nil {
		t.Fatalf("GenerateYAML: %v", err)
	}
	y := string(raw)
	if !strings.Contains(y, "geph-secret-abc") || !strings.Contains(y, "127.0.0.1:19909") || !strings.Contains(y, "JP") || !strings.Contains(y, "Tokyo") {
		t.Fatalf("unexpected YAML output:\n%s", y)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"exits":[{"country":"jp","city":"Tokyo","load":0.2},{"country":"us","city":"Newark","load":0.5},{"country":"jp","city":"Tokyo","load":0.8}]}}`))
	}))
	defer srv.Close()

	sup := NewSupervisor("", t.TempDir())
	exits, err := sup.FetchExits(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchExits: %v", err)
	}
	if len(exits) != 2 || exits[0].Country != "JP" || exits[0].City != "Tokyo" || exits[0].Load != 0.2 || exits[1].Country != "US" {
		t.Fatalf("unexpected exits: %+v", exits)
	}
}

func TestResolveBinaryPathPrecedence(t *testing.T) {
	dir := t.TempDir()
	explicitBin := filepath.Join(dir, "explicit-geph5")
	envBin := filepath.Join(dir, "env-geph5")
	legacyEnvBin := filepath.Join(dir, "legacy-geph5")
	nonExec := filepath.Join(dir, "non-exec-geph5")

	for _, p := range []string{explicitBin, envBin, legacyEnvBin} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	if err := os.WriteFile(nonExec, []byte("plain"), 0o644); err != nil {
		t.Fatalf("write %s: %v", nonExec, err)
	}

	t.Setenv("LOOMA_GEPH5_BIN", envBin)
	t.Setenv("MYVPN_GEPH5_BIN", legacyEnvBin)

	if got := ResolveBinaryPath(explicitBin); got != explicitBin {
		t.Fatalf("explicit path = %q, want %q", got, explicitBin)
	}
	if got := ResolveBinaryPath(nonExec); got != envBin {
		t.Fatalf("non-executable explicit should fall back to LOOMA_GEPH5_BIN: got %q, want %q", got, envBin)
	}

	t.Setenv("LOOMA_GEPH5_BIN", "")
	if got := ResolveBinaryPath(""); got != legacyEnvBin {
		t.Fatalf("legacy env fallback = %q, want %q", got, legacyEnvBin)
	}
}

func TestFetchExitsOnceWithoutConnectACGEPH01(t *testing.T) {
	// Spawn a lightweight fake geph5-client using python3/sh that reads -c <yaml>,
	// serves net_status on control_listen, and verifies FetchExitsOnce cleans it up.
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "fake.pid")
	fakeBin := filepath.Join(dir, "geph5-client")
	script := `#!/bin/sh
set -eu
cfg=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -c) cfg="$2"; shift 2 ;;
    *) shift ;;
  esac
done
addr=$(awk '/control_listen:/{print $2}' "$cfg" | tr -d '"')
host="${addr%:*}"
port="${addr##*:}"
echo "$$" > "` + pidFile + `"
exec python3 -c "
import http.server, json, sys
class H(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        self.rfile.read(int(self.headers.get('Content-Length', 0)))
        body = json.dumps({'jsonrpc':'2.0','id':1,'result':{'exits':[{'country':'jp','city':'Tokyo','load':0.24},{'country':'sg','city':'Singapore','load':0.62}]}}).encode()
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, *args): pass
http.server.HTTPServer(('${host}', int('${port}')), H).serve_forever()
"
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake geph5-client: %v", err)
	}

	sup := NewSupervisor(fakeBin, dir)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	exits, err := sup.FetchExitsOnce(ctx, config.GephConfig{Secret: "test-secret"})
	if err != nil {
		t.Fatalf("FetchExitsOnce failed: %v", err)
	}
	if len(exits) != 2 || exits[0].Country != "JP" || exits[0].City != "Tokyo" || exits[1].Country != "SG" {
		t.Fatalf("unexpected exits: %+v", exits)
	}
	if sup.IsRunning() {
		t.Fatalf("Supervisor.IsRunning() should remain false after FetchExitsOnce")
	}
}

func TestFetchConnInfoBridgeIPAndProtocol(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"state":"Connected","sessions":[{"protocol":"sosistab2","exit":{"country":"JP","city":"Tokyo"},"bridge":"203.0.113.55:12345"}]}}`))
	}))
	defer srv.Close()

	sup := NewSupervisor("", t.TempDir())
	info, err := sup.FetchConnInfo(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchConnInfo: %v", err)
	}
	if info.State != "Connected" || len(info.Sessions) != 1 {
		t.Fatalf("unexpected ConnInfo: %+v", info)
	}
	if info.Sessions[0].Bridge != "203.0.113.55:12345" || info.Sessions[0].Protocol != "sosistab2" {
		t.Fatalf("unexpected session: %+v", info.Sessions[0])
	}
}
