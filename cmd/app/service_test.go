package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"time"

	"github.com/mumu-lhl/looma/internal/config"
	"github.com/mumu-lhl/looma/internal/core"
	"github.com/mumu-lhl/looma/internal/ipc"
	"github.com/mumu-lhl/looma/internal/platform"
	"github.com/mumu-lhl/looma/internal/vpngate"
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

func TestAppServiceSelectGephExitPersistsCountryAndCityACGEPH04(t *testing.T) {
	svc, err := NewAppService(t.TempDir())
	if err != nil {
		t.Fatalf("NewAppService: %v", err)
	}
	if err := svc.SelectGephExit(context.Background(), "jp", "Tokyo"); err != nil {
		t.Fatalf("SelectGephExit(jp, Tokyo): %v", err)
	}
	cfg := svc.GetConfig()
	if cfg.Geph.ExitCountry != "JP" || cfg.Geph.ExitCity != "Tokyo" {
		t.Fatalf("expected JP/Tokyo, got %q/%q", cfg.Geph.ExitCountry, cfg.Geph.ExitCity)
	}

	if err := svc.SelectGephExit(context.Background(), "auto", ""); err != nil {
		t.Fatalf("SelectGephExit(auto): %v", err)
	}
	cfg = svc.GetConfig()
	if cfg.Geph.ExitCountry != "auto" || cfg.Geph.ExitCity != "" {
		t.Fatalf("expected auto/empty, got %q/%q", cfg.Geph.ExitCountry, cfg.Geph.ExitCity)
	}
}

func TestAppServiceVPNGateFetchRequiresProxyAndSelectsNodeACVPNGATE01ACVPNGATE04(t *testing.T) {
	svc, err := NewAppService(t.TempDir())
	if err != nil {
		t.Fatalf("NewAppService: %v", err)
	}

	// Disconnected state must return a friendly error telling user to connect a proxy first.
	if _, err := svc.FetchVPNGateNodes(context.Background()); err == nil {
		t.Fatalf("expected FetchVPNGateNodes to fail when disconnected")
	}

	node := &vpngate.Node{
		ID:           "vpn-jp-1-203.0.113.10-tcp-1195",
		HostName:     "vpn-jp-1",
		IP:           "203.0.113.10",
		CountryLong:  "Japan",
		CountryShort: "JP",
		Server:       "203.0.113.10",
		ServerPort:   1195,
		Network:      "tcp",
		SupportsUDP:  false,
	}
	if err := svc.SelectVPNGateNode(context.Background(), node); err != nil {
		t.Fatalf("SelectVPNGateNode: %v", err)
	}
	cfg := svc.GetConfig()
	if cfg.VPNGate.SelectedNode == nil || cfg.VPNGate.SelectedNode.ID != node.ID {
		t.Fatalf("expected selected VPNGate node %q, got %+v", node.ID, cfg.VPNGate.SelectedNode)
	}
}

func TestAppServiceSubscriptionDirectShareLinkRefreshDeleteAutoBestACSUB01To05(t *testing.T) {
	svc, err := NewAppService(t.TempDir())
	if err != nil {
		t.Fatalf("NewAppService: %v", err)
	}
	defer svc.Shutdown()

	// AC-SUB-01: Direct share link import without HTTP request.
	manualNodes, err := svc.ImportSubscription(context.Background(), "Manual", "vless://uuid-1@127.0.0.1:443?sni=example.com#manual-vless", 24, "auto")
	if err != nil || len(manualNodes) != 1 || manualNodes[0].SubscriptionID != "manual" {
		t.Fatalf("expected 1 manual node, got nodes=%+v err=%v", manualNodes, err)
	}

	// AC-SUB-02: Subscription URL import + RefreshSubscription replacement + proxy mode check.
	subPayload := "vless://uuid-2@127.0.0.1:8443#sub-node-1\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=100; download=200; total=1000")
		_, _ = w.Write([]byte(subPayload))
	}))
	defer srv.Close()

	subNodes, err := svc.ImportSubscription(context.Background(), "MySub", srv.URL, 6, "direct")
	if err != nil || len(subNodes) != 1 {
		t.Fatalf("ImportSubscription URL failed: nodes=%+v err=%v", subNodes, err)
	}
	cfg := svc.GetConfig()
	if len(cfg.Subscriptions) != 1 || cfg.Subscriptions[0].TotalBytes != 1000 {
		t.Fatalf("expected 1 subscription with userinfo, got %+v", cfg.Subscriptions)
	}
	subID := cfg.Subscriptions[0].ID

	// Update server payload and refresh subscription: old sub node replaced, manual node preserved.
	subPayload = "vless://uuid-3@127.0.0.1:9443#sub-node-2\nvless://uuid-4@127.0.0.1:9444#sub-node-3\n"
	refreshed, err := svc.RefreshSubscription(context.Background(), subID)
	if err != nil || len(refreshed) != 2 {
		t.Fatalf("RefreshSubscription failed: nodes=%+v err=%v", refreshed, err)
	}
	cfg = svc.GetConfig()
	if len(cfg.ExternalNodes) != 3 {
		t.Fatalf("expected 1 manual + 2 refreshed nodes = 3, got %d", len(cfg.ExternalNodes))
	}

	// Proxy-only mode while disconnected must fail cleanly.
	if err := svc.UpdateSubscriptionSettings(subID, "MySubRenamed", 12, "proxy"); err != nil {
		t.Fatalf("UpdateSubscriptionSettings failed: %v", err)
	}
	if _, err := svc.RefreshSubscription(context.Background(), subID); err == nil {
		t.Fatalf("expected proxy-only refresh to fail when disconnected")
	}

	// AC-SUB-05: Auto-best selection picks lowest positive latency within scope.
	cfg = svc.GetConfig()
	cfg.ExternalNodes[0].LatencyMs = 150
	cfg.ExternalNodes[1].LatencyMs = 25
	cfg.ExternalNodes[2].LatencyMs = -1
	_ = svc.SaveConfig(cfg)
	if best := svc.GetConfig().FindExternalNode("auto"); best == nil || best.ID != cfg.ExternalNodes[1].ID {
		t.Fatalf("expected auto to pick lowest latency node %s, got %+v", cfg.ExternalNodes[1].ID, best)
	}
	if bestSub := svc.GetConfig().FindExternalNode("auto:" + subID); bestSub == nil || bestSub.ID != cfg.ExternalNodes[1].ID {
		t.Fatalf("expected auto:%s to pick %s, got %+v", subID, cfg.ExternalNodes[1].ID, bestSub)
	}

	// Delete subscription removes its nodes while keeping manual node.
	if err := svc.DeleteSubscription(subID); err != nil {
		t.Fatalf("DeleteSubscription failed: %v", err)
	}
	if got := svc.GetConfig().ExternalNodes; len(got) != 1 || got[0].SubscriptionID != "manual" {
		t.Fatalf("expected only manual node remaining, got %+v", got)
	}
}

func TestAppServiceLogsToggleClearAndExportACLOGS0102(t *testing.T) {
	svc, err := NewAppService(t.TempDir())
	if err != nil {
		t.Fatalf("NewAppService: %v", err)
	}
	defer svc.Shutdown()

	snap := svc.GetLogs(context.Background())
	if snap.Enabled || len(snap.Entries) != 0 {
		t.Fatalf("expected logs disabled and empty by default, got %+v", snap)
	}

	if err := svc.SetLogsEnabled(true); err != nil {
		t.Fatalf("SetLogsEnabled(true): %v", err)
	}
	if !svc.GetConfig().Logs.Enabled {
		t.Fatalf("expected cfg.Logs.Enabled to persist as true")
	}
	snap = svc.GetLogs(context.Background())
	if !snap.Enabled || len(snap.Entries) == 0 {
		t.Fatalf("expected enabled logs with startup entry, got %+v", snap)
	}

	exported, err := svc.ExportLogs(context.Background())
	if err != nil {
		t.Fatalf("ExportLogs failed: %v", err)
	}
	if exported.Path == "" || !strings.Contains(exported.Content, "diagnostic logging enabled") {
		t.Fatalf("unexpected export result: %+v", exported)
	}

	svc.ClearLogs()
	if got := svc.GetLogs(context.Background()).Entries; len(got) != 0 {
		t.Fatalf("expected 0 entries after ClearLogs, got %d", len(got))
	}

	if err := svc.SetLogsEnabled(false); err != nil {
		t.Fatalf("SetLogsEnabled(false): %v", err)
	}
	if svc.GetConfig().Logs.Enabled {
		t.Fatalf("expected cfg.Logs.Enabled to persist as false")
	}
}
