package geph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/config"
	"gopkg.in/yaml.v3"
)

type ExitLocation struct {
	Country string  `json:"country"`
	City    string  `json:"city"`
	Load    float64 `json:"load,omitempty"`
}

func GenerateYAML(cfg config.GephConfig) ([]byte, error) {
	if strings.TrimSpace(cfg.Secret) == "" {
		return nil, errors.New("geph secret is required")
	}
	socksPort := cfg.LocalSocksPort
	if socksPort <= 0 {
		socksPort = 19909
	}
	ctrlPort := cfg.ControlPort
	if ctrlPort <= 0 {
		ctrlPort = 19910
	}

	doc := map[string]any{
		"socks5_listen":  fmt.Sprintf("127.0.0.1:%d", socksPort),
		"control_listen": fmt.Sprintf("127.0.0.1:%d", ctrlPort),
		"credentials": map[string]any{
			"secret": cfg.Secret,
		},
	}

	country := strings.TrimSpace(cfg.ExitCountry)
	city := strings.TrimSpace(cfg.ExitCity)
	if country == "" || strings.EqualFold(country, "auto") {
		doc["exit_constraint"] = "auto"
	} else if city != "" {
		doc["exit_constraint"] = map[string]any{
			"country_city": []string{strings.ToUpper(country), city},
		}
	} else {
		doc["exit_constraint"] = map[string]any{
			"country": strings.ToUpper(country),
		}
	}

	if cfg.BrokerURL != "" {
		doc["broker"] = map[string]any{
			"direct": cfg.BrokerURL,
		}
	}
	return yaml.Marshal(doc)
}

type Supervisor struct {
	mu         sync.Mutex
	binaryPath string
	workDir    string
	cmd        *exec.Cmd
	httpClient *http.Client
	controlURL string
}

func NewSupervisor(binaryPath, workDir string) *Supervisor {
	return &Supervisor{
		binaryPath: binaryPath,
		workDir:    workDir,
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

func (s *Supervisor) Start(ctx context.Context, cfg config.GephConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
		s.cmd = nil
	}
	raw, err := GenerateYAML(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.workDir, 0o700); err != nil {
		return err
	}
	cfgPath := filepath.Join(s.workDir, "geph5-client.yaml")
	if err := os.WriteFile(cfgPath, raw, 0o600); err != nil {
		return err
	}
	ctrlPort := cfg.ControlPort
	if ctrlPort <= 0 {
		ctrlPort = 19910
	}
	s.controlURL = fmt.Sprintf("http://127.0.0.1:%d", ctrlPort)
	if s.binaryPath == "" {
		return errors.New("geph5-client binary path not configured")
	}
	cmd := exec.CommandContext(ctx, s.binaryPath, "-c", cfgPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start geph5-client: %w", err)
	}
	s.cmd = cmd
	return nil
}

func (s *Supervisor) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		err := s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
		s.cmd = nil
		return err
	}
	return nil
}

func (s *Supervisor) FetchExits(ctx context.Context, overrideURL string) ([]ExitLocation, error) {
	endpoint := s.controlURL
	if overrideURL != "" {
		endpoint = overrideURL
	}
	if endpoint == "" {
		return nil, errors.New("geph control endpoint not set")
	}

	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "net_status",
		"params":  []any{},
		"id":      1,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geph control RPC HTTP %d", resp.StatusCode)
	}

	var rpcResp struct {
		Result struct {
			Exits []struct {
				Country string  `json:"country"`
				City    string  `json:"city"`
				Load    float64 `json:"load"`
			} `json:"exits"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("geph control RPC error: %v", rpcResp.Error)
	}

	seen := map[string]ExitLocation{}
	for _, ex := range rpcResp.Result.Exits {
		key := strings.ToUpper(ex.Country) + ":" + ex.City
		if prev, ok := seen[key]; !ok || ex.Load < prev.Load {
			seen[key] = ExitLocation{Country: strings.ToUpper(ex.Country), City: ex.City, Load: ex.Load}
		}
	}
	out := make([]ExitLocation, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Country != out[j].Country {
			return out[i].Country < out[j].Country
		}
		return out[i].City < out[j].City
	})
	return out, nil
}
