package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mumu-lhl/looma/internal/config"
	"github.com/mumu-lhl/looma/internal/core"
	"github.com/mumu-lhl/looma/internal/dpi"
	"github.com/mumu-lhl/looma/internal/geph"
	"github.com/mumu-lhl/looma/internal/ipc"
	"github.com/mumu-lhl/looma/internal/platform"
	"github.com/mumu-lhl/looma/internal/probe"
	"github.com/mumu-lhl/looma/internal/speedtest"
	"github.com/mumu-lhl/looma/internal/storage"
	"github.com/mumu-lhl/looma/internal/subscription"
	"github.com/mumu-lhl/looma/internal/vpngate"
)

type AppService struct {
	mu              sync.Mutex
	dataDir         string
	store           *storage.Store
	engine          *core.Engine
	geph            *geph.Supervisor
	prober          *probe.Prober
	helperSock      string
	helperCmd       *exec.Cmd
	ipcClient       *ipc.Client
	usingHelper     bool
	androidBridge   androidNetBridge
	usingAndroidVPN bool
	tunStoppedOff   func()
	bridgeGeoCache  map[string]string
	subStopCh       chan struct{}
	subStopOnce     sync.Once
	onStateChange   func()
}

func NewAppService(dataDir string) (*AppService, error) {
	st, err := storage.NewStore(dataDir, "looma-local-device")
	if err != nil {
		return nil, err
	}
	defaultSock := filepath.Join(os.TempDir(), fmt.Sprintf("looma-%d", os.Getuid()), "looma-helper.sock")
	svc := &AppService{
		dataDir:        dataDir,
		store:          st,
		engine:         core.NewEngine(filepath.Join(dataDir, "runtime")),
		geph:           geph.NewSupervisor(geph.ResolveBinaryPath(""), filepath.Join(dataDir, "geph")),
		prober:         probe.NewProber(),
		helperSock:     defaultSock,
		androidBridge:  newMyGoAndroidBridge(),
		bridgeGeoCache: map[string]string{},
		subStopCh:      make(chan struct{}),
	}
	if svc.androidBridge.Available() {
		svc.tunStoppedOff = svc.androidBridge.OnTunStopped(func() {
			go func() {
				_ = svc.Disconnect()
			}()
		})
	}
	svc.startSubscriptionScheduler()
	return svc, nil
}

// setOnStateChange registers a callback invoked after connection or config
// changes so the desktop SNI tray stays synchronized with frontend actions.
func (s *AppService) setOnStateChange(fn func()) {
	s.mu.Lock()
	s.onStateChange = fn
	s.mu.Unlock()
}

func (s *AppService) notifyStateChange() {
	s.mu.Lock()
	fn := s.onStateChange
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// SetHelperSocket overrides the core-helper Unix socket path (used for custom deployments and tests).
func (s *AppService) SetHelperSocket(sockPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.helperSock = sockPath
	s.ipcClient = ipc.NewClient(sockPath)
}

func (s *AppService) GetRuntimePlatform() string {
	return runtime.GOOS
}

func (s *AppService) GetConfig() *config.AppConfig {
	return s.store.Get()
}

func (s *AppService) SaveConfig(cfg *config.AppConfig) error {
	if err := s.store.Save(cfg); err != nil {
		return err
	}
	s.notifyStateChange()
	return nil
}

// SwitchInboundMode updates the inbound mode (proxy or tun) and, if the proxy
// is currently connected, reconnects cleanly to apply the new mode.
func (s *AppService) SwitchInboundMode(ctx context.Context, mode config.InboundMode) error {
	if mode != config.InboundProxy && mode != config.InboundTUN {
		return fmt.Errorf("unsupported inbound mode: %s", mode)
	}
	cfg := s.store.Get()
	if cfg.Inbound.Mode == mode {
		return nil
	}
	wasRunning := s.GetStats().Running
	cfg.Inbound.Mode = mode
	if err := s.store.Save(cfg); err != nil {
		return err
	}
	if wasRunning {
		if err := s.Disconnect(); err != nil {
			s.notifyStateChange()
			return fmt.Errorf("disconnect before mode switch: %w", err)
		}
		if _, err := s.Connect(ctx); err != nil {
			s.notifyStateChange()
			return fmt.Errorf("reconnect in %s mode: %w", mode, err)
		}
		return nil
	}
	s.notifyStateChange()
	return nil
}

// Shutdown disconnects any active tunnel, clears system proxy and Kill Switch,
// and gracefully terminates a locally spawned looma-helper process.
func (s *AppService) Shutdown() error {
	s.subStopOnce.Do(func() {
		if s.subStopCh != nil {
			close(s.subStopCh)
		}
	})
	if s.tunStoppedOff != nil {
		s.tunStoppedOff()
		s.tunStoppedOff = nil
	}
	firstErr := s.Disconnect()

	s.mu.Lock()
	cmd := s.helperCmd
	s.helperCmd = nil
	s.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() {
			_, _ = cmd.Process.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}
	return firstErr
}

func resolveHelperBinary(cfg *config.AppConfig) string {
	for _, candidate := range []string{
		cfg.Linux.CoreHelperPath,
		os.Getenv("LOOMA_HELPER_BIN"),
		os.Getenv("MYVPN_CORE_HELPER_BIN"),
	} {
		if candidate != "" {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		dir := filepath.Dir(exe)
		for _, name := range []string{"looma-helper", "core-helper"} {
			p := filepath.Join(dir, name)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p
			}
		}
	}
	for _, name := range []string{"looma-helper", "core-helper"} {
		if found, err := exec.LookPath(name); err == nil && found != "" {
			return found
		}
	}
	return ""
}

func (s *AppService) ensureHelperClient(ctx context.Context, cfg *config.AppConfig) (*ipc.Client, error) {
	if s.ipcClient == nil {
		s.ipcClient = ipc.NewClient(s.helperSock)
	}
	if err := s.ipcClient.Health(ctx); err == nil {
		return s.ipcClient, nil
	}
	helperBin := resolveHelperBinary(cfg)
	if helperBin == "" {
		return nil, fmt.Errorf("helper IPC not reachable at %s and looma-helper binary not found", s.helperSock)
	}
	ok, _, _ := platform.HasTUNCapabilities(helperBin)
	if !ok {
		if err := platform.GrantTUNCapabilitiesOnce(helperBin); err != nil {
			return nil, fmt.Errorf("TUN/KillSwitch elevation required: %w", err)
		}
	}
	cmd := exec.Command(helperBin, "-socket", s.helperSock, "-workdir", filepath.Join(s.dataDir, "runtime"))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("spawn helper: %w", err)
	}
	s.helperCmd = cmd
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := s.ipcClient.Health(ctx); err == nil {
			return s.ipcClient, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("helper did not become healthy on %s", s.helperSock)
}

func (s *AppService) Connect(ctx context.Context) (*core.CompileResult, error) {
	s.ensureAutoExternalTested(ctx)

	s.mu.Lock()

	cfg := s.store.Get()
	usesGeph := cfg.Chain.Hop1.Kind == config.HopGeph || (cfg.Chain.TwoHop && cfg.Chain.Hop2.Kind == config.HopGeph)
	if usesGeph && cfg.Geph.Secret != "" && s.geph.BinaryPath() != "" {
		if err := s.geph.Start(ctx, cfg.Geph); err != nil {
			s.mu.Unlock()
			return nil, fmt.Errorf("start geph5 sidecar: %w", err)
		}
	}

	isAndroid := s.androidBridge != nil && s.androidBridge.Available()
	mandatoryHelper := !isAndroid && runtime.GOOS == "linux" && (cfg.Inbound.Mode == config.InboundTUN || cfg.Inbound.KillSwitch)
	wantsRawFakeHelper := !isAndroid && runtime.GOOS == "linux" &&
		cfg.Chain.Hop1.Kind == config.HopDirectBypass &&
		(cfg.Routing.DirectBypass.EnableRawFake || cfg.Routing.DirectBypass.PresetID == dpi.PresetRawFakeMultidisorder)

	if mandatoryHelper || wantsRawFakeHelper {
		cli, err := s.ensureHelperClient(ctx, cfg)
		if err != nil {
			if mandatoryHelper {
				_ = s.geph.Stop()
				s.mu.Unlock()
				return nil, err
			}
			// Best-effort Raw Fake helper unavailable in proxy mode: gracefully fall back to rootless userspace engine.
		} else {
			startResp, err := cli.Start(ctx, cfg, 0)
			if err != nil {
				if mandatoryHelper {
					_ = s.geph.Stop()
					s.mu.Unlock()
					return nil, err
				}
			} else {
				if cfg.Inbound.KillSwitch {
					ksReq := ipc.KillSwitchRequest{
						Enabled:        true,
						TUNName:        cfg.Inbound.TUNName,
						AllowLAN:       cfg.Inbound.AllowLAN,
						BypassCgroupV2: firstEnv("LOOMA_GEPH_CGROUPV2", "MYVPN_GEPH_CGROUPV2"),
					}
					if _, err := cli.SetKillSwitch(ctx, ksReq); err != nil {
						_ = cli.Stop(ctx)
						_ = s.geph.Stop()
						s.mu.Unlock()
						return nil, fmt.Errorf("enable kill switch: %w", err)
					}
				}
				s.usingHelper = true
				if runtime.GOOS == "linux" && cfg.Inbound.Mode == config.InboundProxy && cfg.Linux.SetSystemProxy {
					_ = platform.SetGNOMESystemProxy(true, cfg.Inbound.HTTPPort, cfg.Inbound.SocksPort)
				}
				s.mu.Unlock()
				s.notifyStateChange()
				return &core.CompileResult{Warnings: startResp.Warnings}, nil
			}
		}
	}

	tunFD := 0
	if s.androidBridge != nil && s.androidBridge.Available() && cfg.Inbound.Mode == config.InboundTUN {
		fd, err := s.androidBridge.PrepareAndStart(ctx, androidPrepareRequest{
			PerAppMode:     string(cfg.Android.PerAppMode),
			PerAppPackages: strings.Join(cfg.Android.PerAppPackages, ","),
			AllowLAN:       cfg.Inbound.AllowLAN,
			KillSwitch:     cfg.Inbound.KillSwitch,
			ChainDesc:      formatChainDesc(cfg),
		})
		if err != nil {
			_ = s.geph.Stop()
			s.mu.Unlock()
			return nil, fmt.Errorf("start android vpn: %w", err)
		}
		tunFD = fd
		s.usingAndroidVPN = true
	}

	res, err := s.engine.Start(ctx, cfg, tunFD)
	if err != nil {
		if s.usingAndroidVPN && s.androidBridge != nil {
			stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = s.androidBridge.Stop(stopCtx)
			cancel()
			s.usingAndroidVPN = false
		}
		_ = s.geph.Stop()
		s.mu.Unlock()
		return nil, err
	}
	s.usingHelper = false
	if runtime.GOOS == "linux" && cfg.Inbound.Mode == config.InboundProxy && cfg.Linux.SetSystemProxy {
		_ = platform.SetGNOMESystemProxy(true, cfg.Inbound.HTTPPort, cfg.Inbound.SocksPort)
	}
	s.mu.Unlock()
	s.notifyStateChange()
	return res, nil
}

func (s *AppService) Disconnect() error {
	s.mu.Lock()

	cfg := s.store.Get()
	if runtime.GOOS == "linux" && cfg.Linux.SetSystemProxy {
		_ = platform.SetGNOMESystemProxy(false, 0, 0)
	}
	var firstErr error
	if s.usingHelper && s.ipcClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = s.ipcClient.SetKillSwitch(ctx, ipc.KillSwitchRequest{Enabled: false})
		if err := s.ipcClient.Stop(ctx); err != nil {
			firstErr = err
		}
		cancel()
		s.usingHelper = false
	}
	if s.usingAndroidVPN && s.androidBridge != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := s.androidBridge.Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		cancel()
		s.usingAndroidVPN = false
	}
	_ = s.geph.Stop()
	if err := s.engine.Stop(); err != nil && firstErr == nil {
		firstErr = err
	}
	s.mu.Unlock()
	s.notifyStateChange()
	return firstErr
}

func (s *AppService) SetKillSwitch(ctx context.Context, enabled bool) (bool, error) {
	s.mu.Lock()

	cfg := s.store.Get()
	cfg.Inbound.KillSwitch = enabled
	if err := s.store.Save(cfg); err != nil {
		s.mu.Unlock()
		return false, err
	}
	if runtime.GOOS != "linux" {
		s.mu.Unlock()
		s.notifyStateChange()
		return enabled, nil
	}
	if !s.usingHelper && !enabled {
		s.mu.Unlock()
		s.notifyStateChange()
		return false, nil
	}
	cli, err := s.ensureHelperClient(ctx, cfg)
	if err != nil {
		s.mu.Unlock()
		return false, err
	}
	st, err := cli.SetKillSwitch(ctx, ipc.KillSwitchRequest{
		Enabled:        enabled,
		TUNName:        cfg.Inbound.TUNName,
		AllowLAN:       cfg.Inbound.AllowLAN,
		BypassCgroupV2: firstEnv("LOOMA_GEPH_CGROUPV2", "MYVPN_GEPH_CGROUPV2"),
	})
	if err != nil {
		s.mu.Unlock()
		return false, err
	}
	s.mu.Unlock()
	s.notifyStateChange()
	return st.Active, nil
}

func (s *AppService) GetStats() core.TrafficStats {
	s.mu.Lock()
	useHelper := s.usingHelper
	cli := s.ipcClient
	s.mu.Unlock()

	if useHelper && cli != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		st, err := cli.Stats(ctx)
		cancel()
		if err == nil && st != nil {
			return *st
		}
	}
	return s.engine.Stats()
}

func (s *AppService) GetLANInfo() map[string]any {
	cfg := s.store.Get()
	return map[string]any{
		"ip":         platform.LocalLANIP(),
		"socks_port": cfg.Inbound.SocksPort,
		"http_port":  cfg.Inbound.HTTPPort,
		"allow_lan":  cfg.Inbound.AllowLAN,
	}
}

func (s *AppService) GetBuiltinProbeURLs() []probe.TestURLPreset {
	return probe.BuiltinTestURLs
}

func (s *AppService) ProbeChain(ctx context.Context) *probe.ChainProbeResult {
	cfg := s.store.Get()
	return s.prober.ProbeChain(ctx, cfg)
}

func (s *AppService) resolveSubscriptionProxyURL(cfg *config.AppConfig, proxyMode string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(proxyMode))
	if mode == "" {
		mode = "auto"
	}
	switch mode {
	case "direct":
		return "", nil
	case "proxy":
		if !s.GetStats().Running {
			return "", fmt.Errorf("该订阅设置为“仅走代理更新”，请先连接代理或 VPN")
		}
		return fmt.Sprintf("http://127.0.0.1:%d", cfg.Inbound.HTTPPort), nil
	default: // "auto"
		if s.GetStats().Running && cfg.Inbound.HTTPPort > 0 {
			return fmt.Sprintf("http://127.0.0.1:%d", cfg.Inbound.HTTPPort), nil
		}
		return "", nil
	}
}

func isHTTPURL(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// ImportSubscription imports either a remote subscription URL (http/https) or
// direct node share links / Base64 / Clash YAML / sing-box JSON payloads.
func (s *AppService) ImportSubscription(ctx context.Context, name, subURL string, intervalHours int, proxyMode string) ([]config.ExternalNode, error) {
	trimmed := strings.TrimSpace(subURL)
	if trimmed == "" {
		return nil, fmt.Errorf("请输入订阅链接或节点分享内容")
	}

	cfg := s.store.Get()
	if !isHTTPURL(trimmed) {
		nodes, err := subscription.ParseAuto([]byte(trimmed), "manual")
		if err != nil {
			return nil, fmt.Errorf("解析分享链接失败：%w", err)
		}
		existing := make(map[string]int, len(cfg.ExternalNodes))
		for i, n := range cfg.ExternalNodes {
			existing[n.ID] = i
		}
		for _, n := range nodes {
			if idx, ok := existing[n.ID]; ok {
				cfg.ExternalNodes[idx] = n
			} else {
				cfg.ExternalNodes = append(cfg.ExternalNodes, n)
			}
		}
		if err := s.store.Save(cfg); err != nil {
			return nil, err
		}
		s.notifyStateChange()
		return nodes, nil
	}

	if intervalHours < 0 {
		intervalHours = 24
	}
	if proxyMode == "" {
		proxyMode = "auto"
	}
	sub := config.Subscription{
		ID:                 fmt.Sprintf("sub-%d", time.Now().UnixNano()),
		Name:               strings.TrimSpace(name),
		URL:                trimmed,
		ProxyMode:          proxyMode,
		UpdateIntervalHour: intervalHours,
		LastUpdatedUnix:    time.Now().Unix(),
		Enabled:            true,
	}
	if sub.Name == "" {
		sub.Name = "订阅"
	}

	proxyURL, err := s.resolveSubscriptionProxyURL(cfg, sub.ProxyMode)
	if err != nil {
		return nil, err
	}
	f := &subscription.Fetcher{}
	res, err := f.FetchWithProxy(ctx, sub, proxyURL)
	if err != nil {
		return nil, err
	}
	sub.UploadBytes = res.UserInfo.UploadBytes
	sub.DownloadBytes = res.UserInfo.DownloadBytes
	sub.TotalBytes = res.UserInfo.TotalBytes
	sub.ExpireUnix = res.UserInfo.ExpireUnix

	cfg.Subscriptions = append(cfg.Subscriptions, sub)
	cfg.ExternalNodes = append(cfg.ExternalNodes, res.Nodes...)
	if err := s.store.Save(cfg); err != nil {
		return nil, err
	}
	s.notifyStateChange()
	return res.Nodes, nil
}

// RefreshSubscription fetches the latest nodes for a single subscription and
// atomically replaces its previous nodes while keeping other groups intact.
func (s *AppService) RefreshSubscription(ctx context.Context, subID string) ([]config.ExternalNode, error) {
	cfg := s.store.Get()
	subIdx := -1
	for i := range cfg.Subscriptions {
		if cfg.Subscriptions[i].ID == subID {
			subIdx = i
			break
		}
	}
	if subIdx < 0 {
		return nil, fmt.Errorf("未找到订阅：%s", subID)
	}
	sub := cfg.Subscriptions[subIdx]
	proxyURL, err := s.resolveSubscriptionProxyURL(cfg, sub.ProxyMode)
	if err != nil {
		cfg.Subscriptions[subIdx].LastError = err.Error()
		_ = s.store.Save(cfg)
		return nil, err
	}

	f := &subscription.Fetcher{}
	res, err := f.FetchWithProxy(ctx, sub, proxyURL)
	if err != nil {
		cfg.Subscriptions[subIdx].LastError = err.Error()
		_ = s.store.Save(cfg)
		return nil, err
	}

	cfg.Subscriptions[subIdx].LastUpdatedUnix = time.Now().Unix()
	cfg.Subscriptions[subIdx].LastError = ""
	cfg.Subscriptions[subIdx].UploadBytes = res.UserInfo.UploadBytes
	cfg.Subscriptions[subIdx].DownloadBytes = res.UserInfo.DownloadBytes
	cfg.Subscriptions[subIdx].TotalBytes = res.UserInfo.TotalBytes
	cfg.Subscriptions[subIdx].ExpireUnix = res.UserInfo.ExpireUnix

	kept := make([]config.ExternalNode, 0, len(cfg.ExternalNodes))
	for _, n := range cfg.ExternalNodes {
		if n.SubscriptionID != subID {
			kept = append(kept, n)
		}
	}
	cfg.ExternalNodes = append(kept, res.Nodes...)
	if err := s.store.Save(cfg); err != nil {
		return nil, err
	}
	s.notifyStateChange()
	return res.Nodes, nil
}

func (s *AppService) UpdateSubscriptionSettings(subID, name string, intervalHours int, proxyMode string) error {
	cfg := s.store.Get()
	for i := range cfg.Subscriptions {
		if cfg.Subscriptions[i].ID == subID {
			if strings.TrimSpace(name) != "" {
				cfg.Subscriptions[i].Name = strings.TrimSpace(name)
			}
			if intervalHours >= 0 {
				cfg.Subscriptions[i].UpdateIntervalHour = intervalHours
			}
			if proxyMode == "auto" || proxyMode == "proxy" || proxyMode == "direct" {
				cfg.Subscriptions[i].ProxyMode = proxyMode
			}
			if err := s.store.Save(cfg); err != nil {
				return err
			}
			s.notifyStateChange()
			return nil
		}
	}
	return fmt.Errorf("未找到订阅：%s", subID)
}

func (s *AppService) DeleteSubscription(subID string) error {
	cfg := s.store.Get()
	subs := make([]config.Subscription, 0, len(cfg.Subscriptions))
	for _, sub := range cfg.Subscriptions {
		if sub.ID != subID {
			subs = append(subs, sub)
		}
	}
	nodes := make([]config.ExternalNode, 0, len(cfg.ExternalNodes))
	for _, n := range cfg.ExternalNodes {
		if n.SubscriptionID != subID {
			nodes = append(nodes, n)
		}
	}
	cfg.Subscriptions = subs
	cfg.ExternalNodes = nodes
	if err := s.store.Save(cfg); err != nil {
		return err
	}
	s.notifyStateChange()
	return nil
}

func (s *AppService) DeleteExternalNode(nodeID string) error {
	cfg := s.store.Get()
	nodes := make([]config.ExternalNode, 0, len(cfg.ExternalNodes))
	for _, n := range cfg.ExternalNodes {
		if n.ID != nodeID {
			nodes = append(nodes, n)
		}
	}
	cfg.ExternalNodes = nodes
	if err := s.store.Save(cfg); err != nil {
		return err
	}
	s.notifyStateChange()
	return nil
}

func (s *AppService) RefreshDueSubscriptions(ctx context.Context) int {
	cfg := s.store.Get()
	now := time.Now().Unix()
	refreshed := 0
	for _, sub := range cfg.Subscriptions {
		if !sub.Enabled || sub.UpdateIntervalHour <= 0 {
			continue
		}
		if now-sub.LastUpdatedUnix >= int64(sub.UpdateIntervalHour)*3600 {
			if _, err := s.RefreshSubscription(ctx, sub.ID); err == nil {
				refreshed++
			}
		}
	}
	return refreshed
}

func (s *AppService) startSubscriptionScheduler() {
	go func() {
		select {
		case <-time.After(5 * time.Second):
		case <-s.subStopCh:
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_ = s.RefreshDueSubscriptions(ctx)
		cancel()

		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				_ = s.RefreshDueSubscriptions(ctx)
				cancel()
			case <-s.subStopCh:
				return
			}
		}
	}()
}

func (s *AppService) ensureAutoExternalTested(ctx context.Context) {
	cfg := s.store.Get()
	needsTest := func(ref config.HopRef) bool {
		if ref.Kind != config.HopExternal {
			return false
		}
		if ref.NodeID != "auto" && !strings.HasPrefix(ref.NodeID, "auto:") {
			return false
		}
		subFilter := strings.TrimPrefix(ref.NodeID, "auto:")
		if ref.NodeID == "auto" {
			subFilter = ""
		}
		for _, n := range cfg.ExternalNodes {
			if subFilter != "" && n.SubscriptionID != subFilter {
				continue
			}
			if n.LatencyMs > 0 {
				return false
			}
		}
		return len(cfg.ExternalNodes) > 0
	}
	if needsTest(cfg.Chain.Hop1) || (cfg.Chain.TwoHop && needsTest(cfg.Chain.Hop2)) {
		testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_ = s.TestAllNodes(testCtx)
		cancel()
	}
}

func (s *AppService) TestAllNodes(ctx context.Context) []speedtest.Result {
	cfg := s.store.Get()
	tester := speedtest.NewTester()
	results := tester.TestAll(ctx, cfg.ExternalNodes)
	latMap := make(map[string]int, len(results))
	for _, r := range results {
		latMap[r.NodeID] = r.LatencyMs
	}
	for i := range cfg.ExternalNodes {
		if lat, ok := latMap[cfg.ExternalNodes[i].ID]; ok {
			cfg.ExternalNodes[i].LatencyMs = lat
		}
	}
	_ = s.store.Save(cfg)
	s.notifyStateChange()
	return results
}

type GephBridgeStatus struct {
	State       string `json:"state"`
	BridgeAddr  string `json:"bridge_addr"`
	BridgeIP    string `json:"bridge_ip"`
	CountryCode string `json:"country_code"`
	Protocol    string `json:"protocol"`
}

// GetGephBridgeStatus queries the running geph5-client conn_info RPC and resolves
// the active bridge IP's ISO-2 country code (cached in memory).
func (s *AppService) GetGephBridgeStatus(ctx context.Context) (*GephBridgeStatus, error) {
	if !s.geph.IsRunning() {
		return &GephBridgeStatus{State: "Disconnected"}, nil
	}
	cfg := s.store.Get()
	ctrlPort := cfg.Geph.ControlPort
	if ctrlPort <= 0 {
		ctrlPort = 19910
	}
	info, err := s.geph.FetchConnInfo(ctx, fmt.Sprintf("http://127.0.0.1:%d", ctrlPort))
	if err != nil || info == nil {
		return &GephBridgeStatus{State: "Connecting"}, nil
	}
	if info.State != "Connected" || len(info.Sessions) == 0 {
		st := info.State
		if st == "" {
			st = "Connecting"
		}
		return &GephBridgeStatus{State: st}, nil
	}
	sess := info.Sessions[0]
	bridgeIP := sess.Bridge
	if host, _, err := net.SplitHostPort(sess.Bridge); err == nil && host != "" {
		bridgeIP = host
	}
	countryCode := ""
	if bridgeIP != "" {
		s.mu.Lock()
		cached := s.bridgeGeoCache[bridgeIP]
		s.mu.Unlock()
		if cached != "" {
			countryCode = cached
		} else {
			countryCode = lookupBridgeCountry(ctx, bridgeIP, cfg.Inbound.HTTPPort)
			if countryCode != "" {
				s.mu.Lock()
				s.bridgeGeoCache[bridgeIP] = countryCode
				s.mu.Unlock()
			}
		}
	}
	return &GephBridgeStatus{
		State:       "Connected",
		BridgeAddr:  sess.Bridge,
		BridgeIP:    bridgeIP,
		CountryCode: countryCode,
		Protocol:    sess.Protocol,
	}, nil
}

func lookupBridgeCountry(ctx context.Context, ip string, httpProxyPort int) string {
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tr := &http.Transport{}
	if httpProxyPort > 0 {
		if proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", httpProxyPort)); err == nil {
			tr.Proxy = http.ProxyURL(proxyURL)
		}
	}
	cli := &http.Client{Timeout: 3 * time.Second, Transport: tr}
	req, err := http.NewRequestWithContext(lookupCtx, http.MethodGet, "https://ipwho.is/"+url.PathEscape(ip), nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "looma/0.1")
	resp, err := cli.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var payload struct {
		CountryCode string `json:"country_code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(payload.CountryCode))
}

func (s *AppService) FetchGephExits(ctx context.Context) ([]geph.ExitLocation, error) {
	cfg := s.store.Get()
	if strings.TrimSpace(cfg.Geph.Secret) == "" {
		return nil, fmt.Errorf("请先填写迷雾通账号 Secret 并保存")
	}
	if s.geph.IsRunning() {
		ctrlPort := cfg.Geph.ControlPort
		if ctrlPort <= 0 {
			ctrlPort = 19910
		}
		exits, err := s.geph.FetchExits(ctx, fmt.Sprintf("http://127.0.0.1:%d", ctrlPort))
		if err != nil {
			return nil, fmt.Errorf("迷雾通控制端口暂不可达，请稍后重试")
		}
		return exits, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	exits, err := s.geph.FetchExitsOnce(probeCtx, cfg.Geph)
	if err != nil {
		return nil, fmt.Errorf("拉取迷雾通出口列表失败：%w", err)
	}
	return exits, nil
}

// SelectGephExit persists the selected Geph5 exit country/city ("auto" for automatic
// selection) and smoothly reconnects if the tunnel is currently running with Geph.
func (s *AppService) SelectGephExit(ctx context.Context, country, city string) error {
	nextCountry := strings.TrimSpace(country)
	nextCity := strings.TrimSpace(city)
	if nextCountry == "" || strings.EqualFold(nextCountry, "auto") {
		nextCountry = "auto"
		nextCity = ""
	} else {
		nextCountry = strings.ToUpper(nextCountry)
	}

	cfg := s.store.Get()
	changed := cfg.Geph.ExitCountry != nextCountry || cfg.Geph.ExitCity != nextCity
	cfg.Geph.ExitCountry = nextCountry
	cfg.Geph.ExitCity = nextCity
	if err := s.store.Save(cfg); err != nil {
		return err
	}

	usesGeph := cfg.Chain.Hop1.Kind == config.HopGeph || (cfg.Chain.TwoHop && cfg.Chain.Hop2.Kind == config.HopGeph)
	if changed && usesGeph && s.GetStats().Running {
		if err := s.Disconnect(); err != nil {
			s.notifyStateChange()
			return fmt.Errorf("disconnect before switching geph exit: %w", err)
		}
		if _, err := s.Connect(ctx); err != nil {
			s.notifyStateChange()
			return fmt.Errorf("reconnect with new geph exit: %w", err)
		}
		return nil
	}
	s.notifyStateChange()
	return nil
}

func (s *AppService) GetClassicCNApps() []platform.ClassicCNApp {
	return platform.ClassicCNApps
}

func (s *AppService) DetectCNApps(apps []platform.InstalledApp) []platform.DetectedCNApp {
	return platform.DetectChinaApps(apps)
}

func (s *AppService) ListInstalledApps(ctx context.Context) ([]platform.InstalledApp, error) {
	if s.androidBridge == nil {
		return nil, fmt.Errorf("android_netlink bridge not initialized")
	}
	return s.androidBridge.ListInstalledApps(ctx)
}

func (s *AppService) OpenVpnSettings(ctx context.Context) (bool, error) {
	if s.androidBridge == nil {
		return false, fmt.Errorf("android_netlink bridge not initialized")
	}
	return s.androidBridge.OpenVpnSettings(ctx)
}

func (s *AppService) vpngateCachePath() string {
	return filepath.Join(s.dataDir, "vpngate_cache.json")
}

// GetVPNGateNodes returns the locally cached VPNGate node list.
func (s *AppService) GetVPNGateNodes() ([]vpngate.Node, error) {
	return vpngate.LoadCache(s.vpngateCachePath())
}

// FetchVPNGateNodes fetches the live VPNGate node list via the active local proxy
// and persists it to the local cache.
func (s *AppService) FetchVPNGateNodes(ctx context.Context) ([]vpngate.Node, error) {
	if !s.GetStats().Running {
		return nil, fmt.Errorf("请先连接任意可用代理或 VPN 后再拉取 VPNGate 节点列表")
	}
	cfg := s.store.Get()
	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Inbound.HTTPPort)
	nodes, err := vpngate.FetchFromAPI(ctx, cfg.VPNGate.APIURL, proxyURL)
	if err != nil {
		return nil, fmt.Errorf("拉取 VPNGate 节点失败：%w", err)
	}
	_ = vpngate.SaveCache(s.vpngateCachePath(), nodes)
	return nodes, nil
}

// SelectVPNGateNode persists the selected VPNGate OpenVPN node and smoothly
// reconnects if the tunnel is currently running with VPNGate.
func (s *AppService) SelectVPNGateNode(ctx context.Context, node *vpngate.Node) error {
	cfg := s.store.Get()
	var selected *config.VPNGateSelectedNode
	if node != nil {
		selected = &config.VPNGateSelectedNode{
			ID:           node.ID,
			HostName:     node.HostName,
			IP:           node.IP,
			CountryLong:  node.CountryLong,
			CountryShort: node.CountryShort,
			Server:       node.Server,
			ServerPort:   node.ServerPort,
			Network:      node.Network,
			SupportsUDP:  node.SupportsUDP,
			CACert:       node.CACert,
			ClientCert:   node.ClientCert,
			ClientKey:    node.ClientKey,
			Username:     node.Username,
			Password:     node.Password,
		}
	}
	prevID := ""
	if cfg.VPNGate.SelectedNode != nil {
		prevID = cfg.VPNGate.SelectedNode.ID
	}
	nextID := ""
	if selected != nil {
		nextID = selected.ID
	}
	cfg.VPNGate.SelectedNode = selected
	if err := s.store.Save(cfg); err != nil {
		return err
	}

	usesVPNGate := cfg.Chain.Hop1.Kind == config.HopVPNGate || (cfg.Chain.TwoHop && cfg.Chain.Hop2.Kind == config.HopVPNGate)
	if prevID != nextID && usesVPNGate && s.GetStats().Running {
		if err := s.Disconnect(); err != nil {
			s.notifyStateChange()
			return fmt.Errorf("disconnect before switching vpngate node: %w", err)
		}
		if _, err := s.Connect(ctx); err != nil {
			s.notifyStateChange()
			return fmt.Errorf("reconnect with new vpngate node: %w", err)
		}
		return nil
	}
	s.notifyStateChange()
	return nil
}

func (s *AppService) GetBypassPresets() []dpi.Strategy {
	return dpi.BuiltinPresets()
}

func (s *AppService) GetBypassCapabilityStatus() dpi.CapabilityStatus {
	s.mu.Lock()
	useHelper := s.usingHelper
	s.mu.Unlock()
	if useHelper {
		st := s.GetStats()
		if st.BypassCapability.OS != "" {
			return st.BypassCapability
		}
	}
	cfg := s.store.Get()
	_, status := dpi.ResolveEffectiveStrategy(cfg.Routing.DirectBypass, dpi.HasRawSocketCapability())
	return status
}

func (s *AppService) AutoProbeBypass(ctx context.Context, domain string) *dpi.AutoProbeReport {
	cfg := s.store.Get()
	dialer := core.NewBypassDialerWithCache(cfg.Routing.DirectBypass, s.engine.DomainCache())
	prober := dpi.NewAutoProber(dpi.AutoProberOptions{
		Resolver: dialer.Resolver(),
		Cache:    s.engine.DomainCache(),
	})
	return prober.Probe(ctx, domain)
}

func (s *AppService) GetDomainBypassCache() []dpi.DomainCacheEntry {
	return s.engine.DomainCache().List()
}

func (s *AppService) LockDomainBypassStrategy(domain, strategyID string) error {
	return s.engine.DomainCache().Lock(domain, strategyID)
}

func (s *AppService) ClearDomainBypassCache(domain string) error {
	if domain == "" {
		return s.engine.DomainCache().Clear()
	}
	return s.engine.DomainCache().Delete(domain)
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}
