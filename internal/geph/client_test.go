package geph

import (
	"context"
	"net/http"
	"net/http/httptest"
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
