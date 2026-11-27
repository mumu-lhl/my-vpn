package main

import (
	"bytes"
	"context"
	"image/png"
	"sync/atomic"
	"testing"

	"github.com/mumu-lhl/my-vpn/internal/config"
)

func TestShouldKeepAliveInTray(t *testing.T) {
	cfgOn := config.DefaultConfig()
	cfgOn.Linux.MinimizeToTray = true

	cfgOff := config.DefaultConfig()
	cfgOff.Linux.MinimizeToTray = false

	tests := []struct {
		name       string
		cfg        *config.AppConfig
		trayActive bool
		want       bool
	}{
		{"minimize enabled and tray active", cfgOn, true, true},
		{"minimize enabled but tray unavailable", cfgOn, false, false},
		{"minimize disabled and tray active", cfgOff, true, false},
		{"minimize disabled and tray unavailable", cfgOff, false, false},
		{"nil config", nil, true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldKeepAliveInTray(tc.cfg, tc.trayActive); got != tc.want {
				t.Fatalf("ShouldKeepAliveInTray() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTrayIconPNGValidityAndDistinctStates(t *testing.T) {
	connected := trayIconPNG(true)
	disconnected := trayIconPNG(false)

	if len(connected) == 0 || len(disconnected) == 0 {
		t.Fatalf("expected non-empty PNG tray icons")
	}
	if bytes.Equal(connected, disconnected) {
		t.Fatalf("expected distinct tray icon visuals for connected vs disconnected states")
	}

	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"connected", connected},
		{"disconnected", disconnected},
	} {
		img, err := png.Decode(bytes.NewReader(tc.data))
		if err != nil {
			t.Fatalf("decode %s tray icon PNG: %v", tc.name, err)
		}
		if b := img.Bounds(); b.Dx() != 32 || b.Dy() != 32 {
			t.Fatalf("%s tray icon size = %dx%d, want 32x32", tc.name, b.Dx(), b.Dy())
		}
	}
}

func TestTrayLabelsAndToolTipFormatting(t *testing.T) {
	idle := TrayStateSnapshot{Running: false, Mode: config.InboundProxy}
	if got := formatTrayToolTip(idle); got != "Looma · 未连接 (代理模式)" {
		t.Fatalf("unexpected idle tooltip: %q", got)
	}

	activeTUN := TrayStateSnapshot{Running: true, Mode: config.InboundTUN, Chain: "geph -> warp"}
	if got := formatTrayToolTip(activeTUN); got != "Looma · 已连接 (TUN 模式)" {
		t.Fatalf("unexpected active TUN tooltip: %q", got)
	}
	if got := formatTrayStatusLabel(activeTUN); got != "状态：已连接 · VPN (TUN) (geph -> warp)" {
		t.Fatalf("unexpected active status label: %q", got)
	}
}

func TestAppServiceStateSyncModeSwitchAndShutdown(t *testing.T) {
	svc, err := NewAppService(t.TempDir())
	if err != nil {
		t.Fatalf("NewAppService: %v", err)
	}

	var notifications atomic.Int32
	svc.setOnStateChange(func() {
		notifications.Add(1)
	})

	cfg := svc.GetConfig()
	cfg.Inbound.Mode = config.InboundProxy
	cfg.Inbound.KillSwitch = false
	cfg.Inbound.SocksPort = freePort(t)
	cfg.Inbound.HTTPPort = freePort(t)
	cfg.Linux.SetSystemProxy = false
	cfg.Linux.MinimizeToTray = true
	if err := svc.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if notifications.Load() == 0 {
		t.Fatalf("expected state change notification on SaveConfig")
	}

	if _, err := svc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if st := svc.GetStats(); !st.Running {
		t.Fatalf("expected running after Connect, got %+v", st)
	}

	// Closing the main window with MinimizeToTray=true and an active tray must
	// keep the proxy running untouched.
	if !ShouldKeepAliveInTray(svc.GetConfig(), true) {
		t.Fatalf("expected keep-alive in tray")
	}
	if st := svc.GetStats(); !st.Running {
		t.Fatalf("expected proxy to stay connected when hiding window to tray")
	}

	// Explicit tray Quit invokes Shutdown, which disconnects cleanly.
	if err := svc.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if st := svc.GetStats(); st.Running {
		t.Fatalf("expected stopped stats after Shutdown, got %+v", st)
	}

	// Mode switch while disconnected persists the new mode and notifies tray.
	before := notifications.Load()
	if err := svc.SwitchInboundMode(context.Background(), config.InboundTUN); err != nil {
		t.Fatalf("SwitchInboundMode(tun): %v", err)
	}
	if got := svc.GetConfig().Inbound.Mode; got != config.InboundTUN {
		t.Fatalf("mode = %q, want %q", got, config.InboundTUN)
	}
	if notifications.Load() <= before {
		t.Fatalf("expected state change notification on SwitchInboundMode")
	}
}
