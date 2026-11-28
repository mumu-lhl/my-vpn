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

func TestStoreGetDeepCloneIsolationACPERF01(t *testing.T) {
	st, err := NewStore(t.TempDir(), "clone-test")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.ExternalNodes = []config.ExternalNode{
		{
			ID:   "n1",
			Name: "Node1",
			RawOutbound: map[string]any{
				"type": "vless",
				"tls":  map[string]any{"enabled": true, "server_name": "a.example.com"},
			},
		},
	}
	cfg.Android.PerAppPackages = []string{"com.example.a"}
	if err := st.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got1 := st.Get()
	got1.ExternalNodes[0].Name = "Mutated"
	got1.ExternalNodes[0].RawOutbound["tls"].(map[string]any)["server_name"] = "mutated.example.com"
	got1.Android.PerAppPackages[0] = "com.mutated"

	got2 := st.Get()
	if got2.ExternalNodes[0].Name != "Node1" {
		t.Fatalf("snapshot mutated via Name: %q", got2.ExternalNodes[0].Name)
	}
	if sni := got2.ExternalNodes[0].RawOutbound["tls"].(map[string]any)["server_name"]; sni != "a.example.com" {
		t.Fatalf("snapshot mutated via nested RawOutbound map: %v", sni)
	}
	if got2.Android.PerAppPackages[0] != "com.example.a" {
		t.Fatalf("snapshot mutated via PerAppPackages slice: %v", got2.Android.PerAppPackages)
	}
}

func BenchmarkStoreGet(b *testing.B) {
	st, err := NewStore(b.TempDir(), "bench-seed")
	if err != nil {
		b.Fatalf("NewStore: %v", err)
	}
	cfg := config.DefaultConfig()
	for i := 0; i < 100; i++ {
		cfg.ExternalNodes = append(cfg.ExternalNodes, config.ExternalNode{
			ID:       "node",
			Name:     "Node",
			Protocol: "vless",
			RawOutbound: map[string]any{
				"type": "vless",
				"tls":  map[string]any{"enabled": true},
			},
		})
	}
	if err := st.Save(cfg); err != nil {
		b.Fatalf("Save: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = st.Get()
	}
}
