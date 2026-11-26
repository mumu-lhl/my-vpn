package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/config"
	"github.com/mumu-lhl/my-vpn/internal/core"
	"github.com/mumu-lhl/my-vpn/internal/geph"
	"github.com/mumu-lhl/my-vpn/internal/ipc"
	"github.com/mumu-lhl/my-vpn/internal/platform"
	"github.com/mumu-lhl/my-vpn/internal/probe"
	"github.com/mumu-lhl/my-vpn/internal/speedtest"
	"github.com/mumu-lhl/my-vpn/internal/storage"
	"github.com/mumu-lhl/my-vpn/internal/subscription"
)

type AppService struct {
	mu          sync.Mutex
	dataDir     string
	store       *storage.Store
	engine      *core.Engine
	geph        *geph.Supervisor
	prober      *probe.Prober
	helperSock  string
	helperCmd   *exec.Cmd
	ipcClient   *ipc.Client
	usingHelper bool
}

func NewAppService(dataDir string) (*AppService, error) {
	st, err := storage.NewStore(dataDir, "looma-local-device")
	if err != nil {
		return nil, err
	}
	defaultSock := filepath.Join(os.TempDir(), fmt.Sprintf("looma-%d", os.Getuid()), "looma-helper.sock")
	return &AppService{
		dataDir:    dataDir,
		store:      st,
		engine:     core.NewEngine(filepath.Join(dataDir, "runtime")),
		geph:       geph.NewSupervisor(os.Getenv("MYVPN_GEPH5_BIN"), filepath.Join(dataDir, "geph")),
		prober:     probe.NewProber(),
		helperSock: defaultSock,
	}, nil
}

// SetHelperSocket overrides the core-helper Unix socket path (used for custom deployments and tests).
func (s *AppService) SetHelperSocket(sockPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.helperSock = sockPath
	s.ipcClient = ipc.NewClient(sockPath)
}

func (s *AppService) GetConfig() *config.AppConfig {
	return s.store.Get()
}

func (s *AppService) SaveConfig(cfg *config.AppConfig) error {
	return s.store.Save(cfg)
}

func (s *AppService) ensureHelperClient(ctx context.Context, cfg *config.AppConfig) (*ipc.Client, error) {
	if s.ipcClient == nil {
		s.ipcClient = ipc.NewClient(s.helperSock)
	}
	if err := s.ipcClient.Health(ctx); err == nil {
		return s.ipcClient, nil
	}
	helperBin := cfg.Linux.CoreHelperPath
	if helperBin == "" {
		helperBin = os.Getenv("MYVPN_CORE_HELPER_BIN")
	}
	if helperBin == "" {
		return nil, fmt.Errorf("helper IPC not reachable at %s and core_helper_path not configured", s.helperSock)
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
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.store.Get()
	usesGeph := cfg.Chain.Hop1.Kind == config.HopGeph || (cfg.Chain.TwoHop && cfg.Chain.Hop2.Kind == config.HopGeph)
	if usesGeph && os.Getenv("MYVPN_GEPH5_BIN") != "" && cfg.Geph.Secret != "" {
		if err := s.geph.Start(ctx, cfg.Geph); err != nil {
			return nil, fmt.Errorf("start geph5 sidecar: %w", err)
		}
	}

	needsHelper := runtime.GOOS == "linux" && (cfg.Inbound.Mode == config.InboundTUN || cfg.Inbound.KillSwitch)
	if needsHelper {
		cli, err := s.ensureHelperClient(ctx, cfg)
		if err != nil {
			_ = s.geph.Stop()
			return nil, err
		}
		startResp, err := cli.Start(ctx, cfg, 0)
		if err != nil {
			_ = s.geph.Stop()
			return nil, err
		}
		if cfg.Inbound.KillSwitch {
			ksReq := ipc.KillSwitchRequest{
				Enabled:        true,
				TUNName:        cfg.Inbound.TUNName,
				AllowLAN:       cfg.Inbound.AllowLAN,
				BypassCgroupV2: os.Getenv("MYVPN_GEPH_CGROUPV2"),
			}
			if _, err := cli.SetKillSwitch(ctx, ksReq); err != nil {
				_ = cli.Stop(ctx)
				_ = s.geph.Stop()
				return nil, fmt.Errorf("enable kill switch: %w", err)
			}
		}
		s.usingHelper = true
		return &core.CompileResult{Warnings: startResp.Warnings}, nil
	}

	res, err := s.engine.Start(ctx, cfg, 0)
	if err != nil {
		_ = s.geph.Stop()
		return nil, err
	}
	s.usingHelper = false
	if runtime.GOOS == "linux" && cfg.Inbound.Mode == config.InboundProxy && cfg.Linux.SetSystemProxy {
		_ = platform.SetGNOMESystemProxy(true, cfg.Inbound.HTTPPort, cfg.Inbound.SocksPort)
	}
	return res, nil
}

func (s *AppService) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	_ = s.geph.Stop()
	if err := s.engine.Stop(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *AppService) SetKillSwitch(ctx context.Context, enabled bool) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.store.Get()
	cfg.Inbound.KillSwitch = enabled
	if err := s.store.Save(cfg); err != nil {
		return false, err
	}
	if runtime.GOOS != "linux" {
		return enabled, nil
	}
	if !s.usingHelper && !enabled {
		return false, nil
	}
	cli, err := s.ensureHelperClient(ctx, cfg)
	if err != nil {
		return false, err
	}
	st, err := cli.SetKillSwitch(ctx, ipc.KillSwitchRequest{
		Enabled:        enabled,
		TUNName:        cfg.Inbound.TUNName,
		AllowLAN:       cfg.Inbound.AllowLAN,
		BypassCgroupV2: os.Getenv("MYVPN_GEPH_CGROUPV2"),
	})
	if err != nil {
		return false, err
	}
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

func (s *AppService) ImportSubscription(ctx context.Context, name, subURL string, intervalHours int) ([]config.ExternalNode, error) {
	sub := config.Subscription{
		ID:                 fmt.Sprintf("sub-%d", time.Now().UnixNano()),
		Name:               name,
		URL:                subURL,
		UpdateIntervalHour: intervalHours,
		LastUpdatedUnix:    time.Now().Unix(),
		Enabled:            true,
	}
	f := &subscription.Fetcher{}
	nodes, err := f.Fetch(ctx, sub)
	if err != nil {
		return nil, err
	}
	cfg := s.store.Get()
	cfg.Subscriptions = append(cfg.Subscriptions, sub)
	cfg.ExternalNodes = append(cfg.ExternalNodes, nodes...)
	if err := s.store.Save(cfg); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (s *AppService) TestAllNodes(ctx context.Context) []speedtest.Result {
	cfg := s.store.Get()
	tester := speedtest.NewTester()
	return tester.TestAll(ctx, cfg.ExternalNodes)
}

func (s *AppService) FetchGephExits(ctx context.Context) ([]geph.ExitLocation, error) {
	cfg := s.store.Get()
	return s.geph.FetchExits(ctx, fmt.Sprintf("http://127.0.0.1:%d", cfg.Geph.ControlPort))
}

func (s *AppService) GetClassicCNApps() []platform.ClassicCNApp {
	return platform.ClassicCNApps
}

func (s *AppService) DetectCNApps(apps []platform.InstalledApp) []platform.DetectedCNApp {
	return platform.DetectChinaApps(apps)
}
