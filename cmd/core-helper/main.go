package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mumu-lhl/my-vpn/internal/core"
	"github.com/mumu-lhl/my-vpn/internal/ipc"
)

func main() {
	defaultDir := filepath.Join(os.TempDir(), fmt.Sprintf("my-vpn-%d", os.Getuid()))
	sockPath := flag.String("socket", filepath.Join(defaultDir, "core-helper.sock"), "Unix domain socket path")
	workDir := flag.String("workdir", defaultDir, "Working directory for runtime sing-box configs")
	flag.Parse()

	eng := core.NewEngine(*workDir)
	srv := ipc.NewServer(*sockPath, eng)
	if err := srv.Start(); err != nil {
		log.Fatalf("start core-helper IPC server: %v", err)
	}
	defer srv.Close()

	log.Printf("my-vpn core-helper listening on %s", *sockPath)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	_ = eng.Stop()
}
