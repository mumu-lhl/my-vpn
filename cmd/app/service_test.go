package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/config"
	"github.com/mumu-lhl/my-vpn/internal/core"
	"github.com/mumu-lhl/my-vpn/internal/ipc"
	"github.com/mumu-lhl/my-vpn/internal/platform"
)

func TestAppServiceCoreHelperIPCAndKillSwitchLifecycle(t *testing.T) {
	dataDir := t.TempDir()
	helperDir := t.TempDir()
	sock := fmt.Sprintf("/tmp/myvpn-svc-%d.sock", time.Now().UnixNano())
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
	if len(nftScripts) != 2 || !strings.Contains(nftScripts[1], "delete table inet myvpn_killswitch") {
		t.Fatalf("expected nftables killswitch deleted on Disconnect, got %v", nftScripts)
	}
}

func TestAppServiceRollbackWhenKillSwitchEnableFails(t *testing.T) {
	dataDir := t.TempDir()
	helperDir := t.TempDir()
	sock := fmt.Sprintf("/tmp/myvpn-svc-fail-%d.sock", time.Now().UnixNano())
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
