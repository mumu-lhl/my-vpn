package speedtest

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/config"
)

type Result struct {
	NodeID    string `json:"node_id"`
	LatencyMs int    `json:"latency_ms"`
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
		probe = defaultHTTPProbe
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

func defaultHTTPProbe(ctx context.Context, _ config.ExternalNode, testURL string) (time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return time.Since(start), nil
}
