package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mumu-lhl/looma/internal/core"
	"github.com/mumu-lhl/looma/internal/ipc"
)

func main() {
	defaultDir := filepath.Join(os.TempDir(), fmt.Sprintf("looma-%d", os.Getuid()))
	sockPath := flag.String("socket", filepath.Join(defaultDir, "core-helper.sock"), "Unix domain socket path")
	workDir := flag.String("workdir", defaultDir, "Working directory for runtime sing-box configs")
	flag.Parse()

	eng := core.NewEngine(*workDir)
	srv := ipc.NewServer(*sockPath, eng)
	if err := srv.Start(); err != nil {
		log.Fatalf("start core-helper IPC server: %v", err)
	}
	defer srv.Close()

	log.Printf("looma-helper listening on %s", *sockPath)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	// Clean up nftables table on intentional graceful shutdown; unexpected crashes intentionally leave it active.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_ = srv.DisableKillSwitch(ctx)
	cancel()
	_ = eng.Stop()
}
