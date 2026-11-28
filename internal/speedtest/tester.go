package speedtest

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mumu-lhl/looma/internal/config"
)

type Result struct {
	NodeID    string `json:"node_id"`
	LatencyMs int    `json:"latency_ms"` // -1 on error/timeout
	Err       string `json:"err,omitempty"`
}

type ProbeFunc func(ctx context.Context, node config.ExternalNode, testURL string) (time.Duration, error)

type Tester struct {
	TestURL     string
	Concurrency int
	Timeout     time.Duration
	Probe       ProbeFunc
}

func NewTester() *Tester {
	return &Tester{
		TestURL:     "https://www.gstatic.com/generate_204",
		Concurrency: 8,
		Timeout:     5 * time.Second,
	}
}

func (t *Tester) TestAll(ctx context.Context, nodes []config.ExternalNode) []Result {
	if len(nodes) == 0 {
		return nil
	}
	conc := t.Concurrency
	if conc <= 0 {
		conc = 4
	}
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	probe := t.Probe
	if probe == nil {
		probe = defaultNodeProbe
	}

	sem := make(chan struct{}, conc)
	results := make([]Result, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		wg.Add(1)
		go func(idx int, node config.ExternalNode) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			d, err := probe(cctx, node, t.TestURL)
			if err != nil {
				results[idx] = Result{NodeID: node.ID, LatencyMs: -1, Err: err.Error()}
				return
			}
			ms := int(d.Milliseconds())
			if ms < 1 {
				ms = 1
			}
			results[idx] = Result{NodeID: node.ID, LatencyMs: ms}
		}(i, n)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if results[i].LatencyMs < 0 {
			return false
		}
		if results[j].LatencyMs < 0 {
			return true
		}
		return results[i].LatencyMs < results[j].LatencyMs
	})
	return results
}

func defaultNodeProbe(ctx context.Context, node config.ExternalNode, _ string) (time.Duration, error) {
	if strings.TrimSpace(node.Server) == "" || node.Port <= 0 {
		return 0, errors.New("missing node server or port")
	}
	addr := net.JoinHostPort(node.Server, strconv.Itoa(node.Port))
	dialer := &net.Dialer{}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	if tlsMap, ok := node.RawOutbound["tls"].(map[string]any); ok {
		if enabled, _ := tlsMap["enabled"].(bool); enabled {
			sni, _ := tlsMap["server_name"].(string)
			if sni == "" {
				sni = node.Server
			}
			tlsConn := tls.Client(conn, &tls.Config{
				ServerName:         sni,
				InsecureSkipVerify: true,
			})
			if deadline, ok := ctx.Deadline(); ok {
				_ = tlsConn.SetDeadline(deadline)
			}
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				return 0, err
			}
		}
	}
	return time.Since(start), nil
}
