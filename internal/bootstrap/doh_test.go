package bootstrap

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestBootstrapDoHStaticIPFallbackAndCachingAC16(t *testing.T) {
	var dohCalls atomic.Int32

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "" && r.Header.Get("X-Test") == "ok" {
			_, _ = w.Write([]byte("BOOTSTRAP_OK:" + r.Host))
			return
		}
		_, _ = w.Write([]byte("BOOTSTRAP_OK"))
	}))
	defer target.Close()
	targetURL, _ := url.Parse(target.URL)
	targetPort := targetURL.Port()

	dohSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dohCalls.Add(1)
		w.Header().Set("Content-Type", "application/dns-json")
		_, _ = w.Write([]byte(`{"Status":0,"Answer":[{"type":1,"TTL":60,"data":"127.0.0.1"}]}`))
	}))
	defer dohSrv.Close()
	dohURL, _ := url.Parse(dohSrv.URL)

	// Using an unresolvable synthetic hostname ("sub.invalid.test") guarantees system DNS cannot resolve it.
	cli := NewHTTPClient(Options{
		Timeout: 2 * time.Second,
		Endpoints: []DoHEndpoint{
			{URL: "http://dead-doh.invalid:1/dns-query", StaticIPs: []string{"127.0.0.1"}},
			{URL: fmt.Sprintf("http://mock-doh.invalid:%s/dns-query", dohURL.Port()), StaticIPs: []string{"127.0.0.1"}},
		},
	})

	reqURL := fmt.Sprintf("http://sub.invalid.test:%s/", targetPort)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, reqURL, nil)
	req.Header.Set("X-Test", "ok")
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("bootstrap HTTP request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != fmt.Sprintf("BOOTSTRAP_OK:sub.invalid.test:%s", targetPort) {
		t.Fatalf("unexpected response body: %q", string(body))
	}

	resp2, err := cli.Get(reqURL)
	if err != nil {
		t.Fatalf("second bootstrap HTTP request failed: %v", err)
	}
	_ = resp2.Body.Close()
	if dohCalls.Load() != 1 {
		t.Fatalf("expected 1 cached DoH call, got %d", dohCalls.Load())
	}
}
