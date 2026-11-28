package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/mumu-lhl/looma/internal/config"
	"github.com/mumu-lhl/looma/internal/platform"
)

const androidPluginID = "android_netlink"

type androidPrepareRequest struct {
	ConfigJSON     string `json:"config_json,omitempty"`
	PerAppMode     string `json:"per_app_mode"`
	PerAppPackages string `json:"per_app_packages"`
	AllowLAN       bool   `json:"allow_lan"`
	KillSwitch     bool   `json:"kill_switch"`
	ChainDesc      string `json:"chain_desc"`
}

type androidNetBridge interface {
	Available() bool
	PrepareAndStart(ctx context.Context, req androidPrepareRequest) (int, error)
	Stop(ctx context.Context) error
	ListInstalledApps(ctx context.Context) ([]platform.InstalledApp, error)
	OpenVpnSettings(ctx context.Context) (bool, error)
	OnTunStopped(fn func()) func()
}

type mygoAndroidBridge struct {
	plugin *mygo.NativePlugin
}

func newMyGoAndroidBridge() androidNetBridge {
	return &mygoAndroidBridge{plugin: mygo.Native(androidPluginID)}
}

func (b *mygoAndroidBridge) Available() bool {
	return runtime.GOOS == "android" && b.plugin.Available()
}

func (b *mygoAndroidBridge) PrepareAndStart(ctx context.Context, req androidPrepareRequest) (int, error) {
	if !b.Available() {
		return 0, errors.ErrUnsupported
	}

	fdCh := make(chan int, 1)
	off := b.plugin.On("tun_ready", func(data []byte) {
		var payload struct {
			FD int `json:"fd"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return
		}
		select {
		case fdCh <- payload.FD:
		default:
		}
	})
	defer off()

	var resp struct {
		Started bool `json:"started"`
	}
	if err := b.plugin.Call(ctx, "prepareAndStart", req, &resp); err != nil {
		return 0, err
	}
	if !resp.Started {
		return 0, errors.New("android vpn service did not start")
	}

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case fd := <-fdCh:
		if fd <= 0 {
			return 0, fmt.Errorf("android VpnService.Builder.establish returned invalid fd (%d)", fd)
		}
		return fd, nil
	case <-timer.C:
		return 0, errors.New("timed out waiting for android VPN TUN interface")
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (b *mygoAndroidBridge) Stop(ctx context.Context) error {
	if !b.Available() {
		return errors.ErrUnsupported
	}
	return b.plugin.Call(ctx, "stop", nil, nil)
}

func (b *mygoAndroidBridge) ListInstalledApps(ctx context.Context) ([]platform.InstalledApp, error) {
	if !b.Available() {
		return nil, errors.ErrUnsupported
	}
	var resp struct {
		Apps []platform.InstalledApp `json:"apps"`
	}
	if err := b.plugin.Call(ctx, "listInstalledApps", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Apps == nil {
		return []platform.InstalledApp{}, nil
	}
	return resp.Apps, nil
}

func (b *mygoAndroidBridge) OpenVpnSettings(ctx context.Context) (bool, error) {
	if !b.Available() {
		return false, errors.ErrUnsupported
	}
	var resp struct {
		Opened bool `json:"opened"`
	}
	if err := b.plugin.Call(ctx, "openVpnSettings", nil, &resp); err != nil {
		return false, err
	}
	return resp.Opened, nil
}

func (b *mygoAndroidBridge) OnTunStopped(fn func()) func() {
	if !b.Available() {
		return func() {}
	}
	return b.plugin.On("tun_stopped", func(_ []byte) {
		if fn != nil {
			fn()
		}
	})
}

func formatChainDesc(cfg *config.AppConfig) string {
	if cfg == nil {
		return "Looma"
	}
	if cfg.Chain.Hop1.Kind == config.HopDirectBypass {
		return string(config.HopDirectBypass)
	}
	if cfg.Chain.TwoHop {
		return fmt.Sprintf("%s -> %s", cfg.Chain.Hop1.Kind, cfg.Chain.Hop2.Kind)
	}
	return string(cfg.Chain.Hop1.Kind)
}
