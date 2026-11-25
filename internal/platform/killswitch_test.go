package platform

import (
	"context"
	"strings"
	"testing"
)

func TestGenerateKillSwitchNFTAndLifecycleAC18(t *testing.T) {
	script := GenerateKillSwitchNFT(KillSwitchOptions{
		TUNName:     "myvpn0",
		RoutingMark: KillSwitchRoutingMark,
		AllowLAN:    true,
	})
	for _, want := range []string{
		"table inet myvpn_killswitch",
		"policy drop;",
		`oifname "lo" accept`,
		`oifname "myvpn0" accept`,
		"meta mark 0x2026 accept",
		"192.168.0.0/16",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("missing %q in nft script:\n%s", want, script)
		}
	}

	noLAN := GenerateKillSwitchNFT(KillSwitchOptions{TUNName: "myvpn0", AllowLAN: false})
	if strings.Contains(noLAN, "192.168.0.0/16") {
		t.Fatalf("LAN subnet must not be allowed when AllowLAN=false")
	}

	var executed []string
	mgr := NewKillSwitchManager(func(_ context.Context, s string) error {
		executed = append(executed, s)
		return nil
	})
	if mgr.Active() {
		t.Fatalf("expected inactive initially")
	}
	if err := mgr.Enable(context.Background(), KillSwitchOptions{TUNName: "myvpn0", AllowLAN: true}); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !mgr.Active() {
		t.Fatalf("expected active after Enable")
	}
	if err := mgr.Disable(context.Background()); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if mgr.Active() {
		t.Fatalf("expected inactive after Disable")
	}
	if len(executed) != 2 || !strings.Contains(executed[1], "delete table inet myvpn_killswitch") {
		t.Fatalf("unexpected nft calls: %+v", executed)
	}
}
