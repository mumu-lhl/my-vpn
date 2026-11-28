package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"time"

	"github.com/mumu-lhl/looma/internal/config"
	"github.com/mumu-lhl/looma/internal/core"
	"github.com/mumu-lhl/looma/internal/ipc"
	"github.com/mumu-lhl/looma/internal/platform"
)

func TestAppServiceCoreHelperIPCAndKillSwitchLifecycle(t *testing.T) {
	dataDir := t.TempDir()
	helperDir := t.TempDir()
	sock := fmt.Sprintf("/tmp/looma-svc-%d.sock", time.Now().UnixNano())
	defer os.Remove(sock)

	var nftScripts []string
	helperEng := core.NewEngine(helperDir)
	srv := ipc.NewServer(sock, helperEng)
	srv.SetKillSwitchManager(platform.NewKillSwitchManager(func(_ context.Context, script string) error {
		nftScripts = append(nftScripts, script)
		return nil
	}))
	if err := srv.Start(); err != nil {
		t.Fatalf("start helper IPC server: %v", err)
	}
	defer srv.Close()

	svc, err := NewAppService(dataDir)
	if err != nil {
		t.Fatalf("NewAppService: %v", err)
	}
	svc.SetHelperSocket(sock)

	cfg := svc.GetConfig()
	cfg.Inbound.Mode = config.InboundProxy
	cfg.Inbound.KillSwitch = true
	cfg.Inbound.AllowLAN = true
	cfg.Inbound.SocksPort = freePort(t)
	cfg.Inbound.HTTPPort = freePort(t)
	cfg.Linux.SetSystemProxy = false
	if err := svc.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	if _, err := svc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect with KillSwitch via IPC failed: %v", err)
	}
	if st := svc.GetStats(); !st.Running {
		t.Fatalf("expected running stats via helper IPC, got %+v", st)
	}
	if len(nftScripts) != 1 || !strings.Contains(nftScripts[0], "policy drop;") {
		t.Fatalf("expected nftables killswitch enabled on Connect, got %v", nftScripts)
	}

	if err := svc.Disconnect(); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
	if st := svc.GetStats(); st.Running {
		t.Fatalf("expected stopped stats after Disconnect, got %+v", st)
	}
	if len(nftScripts) != 2 || !strings.Contains(nftScripts[1], "delete table inet looma_killswitch") {
		t.Fatalf("expected nftables killswitch deleted on Disconnect, got %v", nftScripts)
	}
}

func TestAppServiceRollbackWhenKillSwitchEnableFails(t *testing.T) {
	dataDir := t.TempDir()
	helperDir := t.TempDir()
	sock := fmt.Sprintf("/tmp/looma-svc-fail-%d.sock", time.Now().UnixNano())
	defer os.Remove(sock)

	helperEng := core.NewEngine(helperDir)
	srv := ipc.NewServer(sock, helperEng)
	srv.SetKillSwitchManager(platform.NewKillSwitchManager(func(_ context.Context, _ string) error {
		return errors.New("nft permission denied")
	}))
	if err := srv.Start(); err != nil {
		t.Fatalf("start helper IPC server: %v", err)
	}
	defer srv.Close()

	svc, err := NewAppService(dataDir)
	if err != nil {
		t.Fatalf("NewAppService: %v", err)
	}
	svc.SetHelperSocket(sock)

	cfg := svc.GetConfig()
	cfg.Inbound.Mode = config.InboundProxy
	cfg.Inbound.KillSwitch = true
	cfg.Inbound.SocksPort = freePort(t)
	cfg.Inbound.HTTPPort = freePort(t)
	cfg.Linux.SetSystemProxy = false
	_ = svc.SaveConfig(cfg)

	if _, err := svc.Connect(context.Background()); err == nil {
		t.Fatalf("expected Connect error when KillSwitch enable fails")
	}
	if st := helperEng.Stats(); st.Running {
		t.Fatalf("expected helper engine rolled back (stopped) when KillSwitch fails")
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

func TestMyGoBindAppService(t *testing.T) {
	svc, err := NewAppService(t.TempDir())
	if err != nil {
		t.Fatalf("NewAppService: %v", err)
	}
	// Ensures every exported method on AppService is JSON-bindable by mygo.Bind.
	mygo.Bind(svc)
}

func TestGetRuntimePlatformAndFriendlyGephExitError(t *testing.T) {
	svc, err := NewAppService(t.TempDir())
	if err != nil {
		t.Fatalf("NewAppService: %v", err)
	}
	if plat := svc.GetRuntimePlatform(); plat == "" {
		t.Fatalf("expected non-empty runtime platform")
	}
	// Without Geph secret, FetchGephExits must return a friendly message (never raw 127.0.0.1:19910).
	_, err = svc.FetchGephExits(context.Background())
	if err == nil || strings.Contains(err.Error(), "19910") {
		t.Fatalf("expected friendly error without raw 19910 port, got: %v", err)
	}
}

type fakeAndroidBridge struct {
	available    bool
	prepareFD    int
	prepareErr   error
	prepareCalls []androidPrepareRequest
	stopCalls    int
	apps         []platform.InstalledApp
	openResult   bool
}

func (f *fakeAndroidBridge) Available() bool { return f.available }
func (f *fakeAndroidBridge) PrepareAndStart(_ context.Context, req androidPrepareRequest) (int, error) {
	f.prepareCalls = append(f.prepareCalls, req)
	if f.prepareErr != nil {
		return 0, f.prepareErr
	}
	return f.prepareFD, nil
}
func (f *fakeAndroidBridge) Stop(_ context.Context) error {
	f.stopCalls++
	return nil
}
func (f *fakeAndroidBridge) ListInstalledApps(_ context.Context) ([]platform.InstalledApp, error) {
	return f.apps, nil
}
func (f *fakeAndroidBridge) OpenVpnSettings(_ context.Context) (bool, error) {
	return f.openResult, nil
}
func (f *fakeAndroidBridge) OnTunStopped(_ func()) func() { return func() {} }

func TestAppServiceAndroidVPNLifecycleAndBridgeMethods(t *testing.T) {
	svc, err := NewAppService(t.TempDir())
	if err != nil {
		t.Fatalf("NewAppService: %v", err)
	}
	bridge := &fakeAndroidBridge{
		available:  true,
		prepareFD:  42,
		openResult: true,
		apps: []platform.InstalledApp{
			{PackageName: "com.tencent.mm", Label: "WeChat", IsSystem: false},
		},
	}
	svc.androidBridge = bridge

	apps, err := svc.ListInstalledApps(context.Background())
	if err != nil || len(apps) != 1 || apps[0].PackageName != "com.tencent.mm" {
		t.Fatalf("unexpected ListInstalledApps result: apps=%+v err=%v", apps, err)
	}
	opened, err := svc.OpenVpnSettings(context.Background())
	if err != nil || !opened {
		t.Fatalf("unexpected OpenVpnSettings result: opened=%v err=%v", opened, err)
	}

	cfg := svc.GetConfig()
	cfg.Inbound.Mode = config.InboundTUN
	cfg.Inbound.SocksPort = freePort(t)
	cfg.Inbound.HTTPPort = freePort(t)
	cfg.Android.PerAppMode = config.PerAppBlacklist
	cfg.Android.PerAppPackages = []string{"com.tencent.mm", "com.eg.android.AlipayGphone"}
	if err := svc.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	res, err := svc.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect with Android VPN bridge failed: %v", err)
	}
	if len(bridge.prepareCalls) != 1 {
		t.Fatalf("expected 1 PrepareAndStart call, got %d", len(bridge.prepareCalls))
	}
	req := bridge.prepareCalls[0]
	if req.PerAppMode != "blacklist" || req.PerAppPackages != "com.tencent.mm,com.eg.android.AlipayGphone" {
		t.Fatalf("unexpected prepare request: %+v", req)
	}
	if !strings.Contains(string(res.JSON), `"file_descriptor": 42`) {
		t.Fatalf("expected compiled sing-box config to contain tun file_descriptor 42")
	}

	if err := svc.Disconnect(); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
	if bridge.stopCalls != 1 {
		t.Fatalf("expected 1 Stop call on Disconnect, got %d", bridge.stopCalls)
	}

	// Verify permission denial fails Connect cleanly.
	bridge.prepareErr = errors.New("PermissionDenied")
	if _, err := svc.Connect(context.Background()); err == nil {
		t.Fatalf("expected Connect error when VPN permission is denied")
	}
}
