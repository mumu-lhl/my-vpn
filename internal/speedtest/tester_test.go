package speedtest

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/mumu-lhl/looma/internal/config"
)

func TestBatchURLTestAndSortingAC03(t *testing.T) {
	nodes := []config.ExternalNode{
		{ID: "slow", Name: "Slow"},
		{ID: "dead", Name: "Dead"},
		{ID: "fast", Name: "Fast"},
	}
	tester := NewTester()
	tester.Probe = func(ctx context.Context, node config.ExternalNode, _ string) (time.Duration, error) {
		switch node.ID {
		case "fast":
			return 25 * time.Millisecond, nil
		case "slow":
			return 180 * time.Millisecond, nil
		default:
			return 0, errors.New("timeout")
		}
	}
	res := tester.TestAll(context.Background(), nodes)
	if len(res) != 3 {
		t.Fatalf("expected 3 results, got %d", len(res))
	}
	if res[0].NodeID != "fast" || res[0].LatencyMs != 25 {
		t.Fatalf("expected fast first, got %+v", res[0])
	}
	if res[1].NodeID != "slow" || res[1].LatencyMs != 180 {
		t.Fatalf("expected slow second, got %+v", res[1])
	}
	if res[2].NodeID != "dead" || res[2].LatencyMs != -1 {
		t.Fatalf("expected dead last with -1, got %+v", res[2])
	}
}

func TestDefaultNodeProbeRealTCPConnectAndTimeoutACSUB04(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	alivePort := ln.Addr().(*net.TCPAddr).Port

	deadLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen dead: %v", err)
	}
	deadPort := deadLn.Addr().(*net.TCPAddr).Port
	_ = deadLn.Close()

	tester := NewTester()
	tester.Timeout = time.Second
	res := tester.TestAll(context.Background(), []config.ExternalNode{
		{ID: "dead", Server: "127.0.0.1", Port: deadPort},
		{ID: "alive", Server: "127.0.0.1", Port: alivePort},
	})
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	if res[0].NodeID != "alive" || res[0].LatencyMs <= 0 {
		t.Fatalf("expected alive node first with positive latency, got %+v", res[0])
	}
	if res[1].NodeID != "dead" || res[1].LatencyMs != -1 {
		t.Fatalf("expected dead node with -1 latency, got %+v", res[1])
	}
}
