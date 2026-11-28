package geph

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mumu-lhl/looma/internal/config"
	"gopkg.in/yaml.v3"
)

type ExitLocation struct {
	Country string  `json:"country"`
	City    string  `json:"city"`
	Load    float64 `json:"load,omitempty"`
}

type ConnectedSession struct {
	Protocol string `json:"protocol"`
	Exit     struct {
		Country string `json:"country"`
		City    string `json:"city"`
	} `json:"exit"`
	Bridge string `json:"bridge,omitempty"`
}

type ConnInfo struct {
	State    string             `json:"state"`
	Sessions []ConnectedSession `json:"sessions,omitempty"`
}

// ResolveBinaryPath locates the bundled geph5-client executable across desktop
// and Android deployments without requiring manual environment configuration.
// Precedence: explicit path > LOOMA_GEPH5_BIN > MYVPN_GEPH5_BIN > executable
// sibling > Android nativeLibraryDir (/proc/self/maps) > PATH.
func ResolveBinaryPath(explicit string) string {
	for _, candidate := range []string{
		strings.TrimSpace(explicit),
		strings.TrimSpace(os.Getenv("LOOMA_GEPH5_BIN")),
		strings.TrimSpace(os.Getenv("MYVPN_GEPH5_BIN")),
	} {
		if candidate != "" && isExecutableFile(candidate) {
			return candidate
		}
	}

	names := []string{"geph5-client"}
	if runtime.GOOS == "android" {
		names = []string{"libgeph5.so", "geph5-client"}
	}

	if exe, err := os.Executable(); err == nil && exe != "" {
		dir := filepath.Dir(exe)
		for _, name := range names {
			if p := filepath.Join(dir, name); isExecutableFile(p) {
				return p
			}
		}
	}

	if runtime.GOOS == "android" {
		if libDir := androidNativeLibraryDir(); libDir != "" {
			if p := filepath.Join(libDir, "libgeph5.so"); isExecutableFile(p) {
				return p
			}
		}
	}

	if found, err := exec.LookPath("geph5-client"); err == nil && found != "" {
		return found
	}
	return ""
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

// androidNativeLibraryDir finds the app's extracted native library directory by
// locating libmygoapp.so in /proc/self/maps.
func androidNativeLibraryDir() string {
	f, err := os.Open("/proc/self/maps")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		idx := strings.Index(line, "/")
		if idx < 0 {
			continue
		}
		p := strings.TrimSpace(line[idx:])
		if strings.HasSuffix(p, "/libmygoapp.so") {
			return filepath.Dir(p)
		}
	}
	return ""
}

// GenerateYAML produces a valid geph5-client configuration YAML matching geph5-client Config struct.
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

func (s *Supervisor) BinaryPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.binaryPath != "" {
		return s.binaryPath
	}
	return ResolveBinaryPath("")
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
	binPath := s.binaryPath
	if binPath == "" {
		binPath = ResolveBinaryPath("")
	}
	if binPath == "" {
		return errors.New("geph5-client binary not found (set LOOMA_GEPH5_BIN or bundle geph5-client alongside looma)")
	}
	cmd := exec.CommandContext(ctx, binPath, "-c", cfgPath)
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

func (s *Supervisor) callControlRPC(ctx context.Context, overrideURL, method string, out any) error {
	endpoint := s.controlURL
	if overrideURL != "" {
		endpoint = overrideURL
	}
	if endpoint == "" {
		return errors.New("geph control endpoint not set")
	}

	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  []any{},
		"id":      1,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("geph control RPC HTTP %d", resp.StatusCode)
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  any             `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("geph control RPC error: %v", rpcResp.Error)
	}
	if out != nil && len(rpcResp.Result) > 0 {
		return json.Unmarshal(rpcResp.Result, out)
	}
	return nil
}

// FetchConnInfo queries geph5-client's conn_info RPC for active bridge & session state.
func (s *Supervisor) FetchConnInfo(ctx context.Context, overrideURL string) (*ConnInfo, error) {
	var info ConnInfo
	if err := s.callControlRPC(ctx, overrideURL, "conn_info", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// FetchRecentLogs queries geph5-client's recent_logs RPC for diagnostic log lines.
func (s *Supervisor) FetchRecentLogs(ctx context.Context, overrideURL string) ([]string, error) {
	var lines []string
	if err := s.callControlRPC(ctx, overrideURL, "recent_logs", &lines); err != nil {
		return nil, err
	}
	return lines, nil
}

// FetchExits queries geph5-client's nanorpc control_listen endpoint for available exit locations.
func (s *Supervisor) FetchExits(ctx context.Context, overrideURL string) ([]ExitLocation, error) {
	var result struct {
		Exits []struct {
			Country string  `json:"country"`
			City    string  `json:"city"`
			Load    float64 `json:"load"`
		} `json:"exits"`
	}
	if err := s.callControlRPC(ctx, overrideURL, "net_status", &result); err != nil {
		return nil, err
	}

	seen := map[string]ExitLocation{}
	for _, ex := range result.Exits {
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

// IsRunning reports whether a geph5-client sidecar subprocess is currently active.
func (s *Supervisor) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cmd != nil && s.cmd.Process != nil
}

// FetchExitsOnce temporarily spawns an isolated geph5-client instance on ephemeral
// loopback ports, polls its control_listen RPC for exit locations, and terminates
// the temporary process before returning.
func (s *Supervisor) FetchExitsOnce(ctx context.Context, cfg config.GephConfig) ([]ExitLocation, error) {
	if strings.TrimSpace(cfg.Secret) == "" {
		return nil, errors.New("geph secret is required")
	}
	binPath := s.BinaryPath()
	if binPath == "" {
		return nil, errors.New("geph5-client binary not found")
	}

	socksPort, err := freeLoopbackPort()
	if err != nil {
		return nil, fmt.Errorf("allocate temporary socks port: %w", err)
	}
	ctrlPort, err := freeLoopbackPort()
	if err != nil {
		return nil, fmt.Errorf("allocate temporary control port: %w", err)
	}

	probeCfg := cfg
	probeCfg.LocalSocksPort = socksPort
	probeCfg.ControlPort = ctrlPort
	probeCfg.ExitCountry = "auto"
	probeCfg.ExitCity = ""

	raw, err := GenerateYAML(probeCfg)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "looma-geph-probe-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "geph5-client.yaml")
	if err := os.WriteFile(cfgPath, raw, 0o600); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, binPath, "-c", cfgPath)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start temporary geph5-client: %w", err)
	}

	exitCh := make(chan error, 1)
	go func() {
		exitCh <- cmd.Wait()
	}()
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-exitCh:
		case <-time.After(2 * time.Second):
		}
	}()

	ctrlURL := fmt.Sprintf("http://127.0.0.1:%d", ctrlPort)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	var emptySuccesses int
	for {
		exits, err := s.FetchExits(ctx, ctrlURL)
		if err == nil {
			if len(exits) > 0 {
				return exits, nil
			}
			emptySuccesses++
			if emptySuccesses >= 6 {
				return exits, nil
			}
		} else {
			lastErr = err
		}

		select {
		case waitErr := <-exitCh:
			if lastErr != nil {
				return nil, lastErr
			}
			if waitErr != nil {
				return nil, fmt.Errorf("geph5-client exited early: %w", waitErr)
			}
			return nil, errors.New("geph5-client exited before returning exits")
		case <-ctx.Done():
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func freeLoopbackPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}
