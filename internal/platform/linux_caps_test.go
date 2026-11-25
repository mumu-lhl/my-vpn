package platform

import "testing"

func TestParseGetcapOutputAC10(t *testing.T) {
	if !ParseGetcapOutput("/usr/local/bin/core-helper cap_net_admin,cap_net_raw=ep") {
		t.Fatalf("expected true for valid getcap output")
	}
	if !ParseGetcapOutput("/opt/my-vpn/core-helper = cap_net_admin,cap_net_raw+ep") {
		t.Fatalf("expected true for legacy getcap format")
	}
	if ParseGetcapOutput("/opt/my-vpn/core-helper cap_net_bind_service=ep") {
		t.Fatalf("expected false when cap_net_admin/raw missing")
	}
}
