package ipc

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/mumu-lhl/looma/internal/config"
	"github.com/mumu-lhl/looma/internal/core"
	"github.com/mumu-lhl/looma/internal/platform"
)

func TestUnixSocketIPCStartStopStatsAC10(t *testing.T) {
	dir := t.TempDir()
	sock := fmt.Sprintf("/tmp/looma-ipc-%d.sock", time.Now().UnixNano())
	defer os.Remove(sock)
	eng := core.NewEngine(dir)
	srv := NewServer(sock, eng)
	srv.SetKillSwitchManager(platform.NewKillSwitchManager(func(_ context.Context, _ string) error { return nil }))
	if err := srv.Start(); err != nil {
		t.Fatalf("ipc server start: %v", err)
	}
	defer srv.Close()

	cli := NewClient(sock)
	cfg := config.DefaultConfig()
	cfg.Inbound.Mode = config.InboundProxy
	cfg.Inbound.SocksPort = freePort(t)
	cfg.Inbound.HTTPPort = freePort(t)

	res, err := cli.Start(context.Background(), cfg, 0)
	if err != nil || !res.OK {
		t.Fatalf("ipc start failed: err=%v res=%+v", err, res)
	}
	st, err := cli.Stats(context.Background())
	if err != nil || !st.Running {
		t.Fatalf("expected running stats, err=%v st=%+v", err, st)
	}
	ksOn, err := cli.SetKillSwitch(context.Background(), KillSwitchRequest{Enabled: true, TUNName: "looma0", AllowLAN: true})
	if err != nil || !ksOn.Active {
		t.Fatalf("expected killswitch active via IPC, err=%v st=%+v", err, ksOn)
	}
	ksOff, err := cli.SetKillSwitch(context.Background(), KillSwitchRequest{Enabled: false})
	if err != nil || ksOff.Active {
		t.Fatalf("expected killswitch inactive via IPC, err=%v st=%+v", err, ksOff)
	}
	if err := cli.Stop(context.Background()); err != nil {
		t.Fatalf("ipc stop failed: %v", err)
	}
	st2, err := cli.Stats(context.Background())
	if err != nil || st2.Running {
		t.Fatalf("expected stopped stats, err=%v st=%+v", err, st2)
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
