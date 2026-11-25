package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/config"
	"github.com/mumu-lhl/my-vpn/internal/core"
	"github.com/mumu-lhl/my-vpn/internal/geph"
	"github.com/mumu-lhl/my-vpn/internal/platform"
	"github.com/mumu-lhl/my-vpn/internal/speedtest"
	"github.com/mumu-lhl/my-vpn/internal/storage"
	"github.com/mumu-lhl/my-vpn/internal/subscription"
)

type AppService struct {
	store  *storage.Store
	engine *core.Engine
	geph   *geph.Supervisor
}

func NewAppService(dataDir string) (*AppService, error) {
	st, err := storage.NewStore(dataDir, "my-vpn-local-device")
	if err != nil {
		return nil, err
	}
	return &AppService{
		store:  st,
		engine: core.NewEngine(filepath.Join(dataDir, "runtime")),
		geph:   geph.NewSupervisor(os.Getenv("MYVPN_GEPH5_BIN"), filepath.Join(dataDir, "geph")),
	}, nil
}

func (s *AppService) GetConfig() *config.AppConfig {
	return s.store.Get()
}

func (s *AppService) SaveConfig(cfg *config.AppConfig) error {
	return s.store.Save(cfg)
}

func (s *AppService) Connect(ctx context.Context) (*core.CompileResult, error) {
	cfg := s.store.Get()
	if runtime.GOOS == "linux" && cfg.Inbound.Mode == config.InboundTUN && cfg.Linux.CoreHelperPath != "" {
		ok, _, _ := platform.HasTUNCapabilities(cfg.Linux.CoreHelperPath)
		if !ok {
			if err := platform.GrantTUNCapabilitiesOnce(cfg.Linux.CoreHelperPath); err != nil {
				return nil, fmt.Errorf("TUN elevation required: %w", err)
			}
		}
	}
	res, err := s.engine.Start(ctx, cfg, 0)
	if err != nil {
		return nil, err
	}
	if runtime.GOOS == "linux" && cfg.Inbound.Mode == config.InboundProxy && cfg.Linux.SetSystemProxy {
		_ = platform.SetGNOMESystemProxy(true, cfg.Inbound.HTTPPort, cfg.Inbound.SocksPort)
	}
	return res, nil
}

func (s *AppService) Disconnect() error {
	cfg := s.store.Get()
	if runtime.GOOS == "linux" && cfg.Linux.SetSystemProxy {
		_ = platform.SetGNOMESystemProxy(false, 0, 0)
	}
	_ = s.geph.Stop()
	return s.engine.Stop()
}

func (s *AppService) GetStats() core.TrafficStats {
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
