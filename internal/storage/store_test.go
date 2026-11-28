package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mumu-lhl/looma/internal/config"
)

func TestStoreEncryptsSecretsAtRest(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir, "unit-test-seed")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Geph.Secret = "super-secret-geph-token-123"
	cfg.Warp.LicenseKey = "WARP-PLUS-LICENSE-XYZ"
	if err := st.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "super-secret-geph-token-123") {
		t.Fatalf("geph secret leaked in plaintext on disk")
	}
	if strings.Contains(string(raw), "WARP-PLUS-LICENSE-XYZ") {
		t.Fatalf("warp license leaked in plaintext on disk")
	}

	st2, err := NewStore(dir, "unit-test-seed")
	if err != nil {
		t.Fatalf("reload NewStore: %v", err)
	}
	got := st2.Get()
	if got.Geph.Secret != "super-secret-geph-token-123" || got.Warp.LicenseKey != "WARP-PLUS-LICENSE-XYZ" {
		t.Fatalf("decrypted secrets mismatch: %+v", got)
	}
}
