package geph

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mumu-lhl/my-vpn/internal/config"
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
