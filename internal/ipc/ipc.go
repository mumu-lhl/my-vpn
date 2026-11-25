package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/config"
	"github.com/mumu-lhl/my-vpn/internal/core"
)

type StartRequest struct {
	Config *config.AppConfig `json:"config"`
	TunFD  int               `json:"tun_fd,omitempty"`
}

type StartResponse struct {
	OK       bool                  `json:"ok"`
	Warnings []core.CompileWarning `json:"warnings,omitempty"`
	Error    string                `json:"error,omitempty"`
}

type Server struct {
	sockPath string
	engine   *core.Engine
	srv      *http.Server
	ln       net.Listener
	mu       sync.Mutex
}

func NewServer(sockPath string, eng *core.Engine) *Server {
	return &Server{sockPath: sockPath, engine: eng}
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.sockPath), 0o700); err != nil {
		return err
	}
	_ = os.Remove(s.sockPath)
	ln, err := net.Listen("unix", s.sockPath)
	if err != nil {
		return err
	}
	_ = os.Chmod(s.sockPath, 0o600)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(s.engine.Stats())
	})
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		var req StartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Config == nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(StartResponse{Error: "invalid start request"})
			return
		}
		res, err := s.engine.Start(context.Background(), req.Config, req.TunFD)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(StartResponse{Error: err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(StartResponse{OK: true, Warnings: res.Warnings})
	})
	mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		if err := s.engine.Stop(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	s.ln = ln
	s.srv = &http.Server{Handler: mux}
	go func() { _ = s.srv.Serve(ln) }()
	return nil
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = s.srv.Shutdown(ctx)
		cancel()
	}
	if s.ln != nil {
		_ = s.ln.Close()
	}
	_ = os.Remove(s.sockPath)
	return nil
}

type Client struct {
	httpClient *http.Client
}

func NewClient(sockPath string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", sockPath)
				},
			},
		},
	}
}

func (c *Client) Start(ctx context.Context, cfg *config.AppConfig, tunFD int) (*StartResponse, error) {
	b, _ := json.Marshal(StartRequest{Config: cfg, TunFD: tunFD})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/start", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out StartResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.OK {
		return &out, errors.New(out.Error)
	}
	return &out, nil
}

func (c *Client) Stop(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/stop", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stop failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Stats(ctx context.Context) (*core.TrafficStats, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/stats", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var st core.TrafficStats
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return nil, err
	}
	return &st, nil
}
