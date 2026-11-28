package platform

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

const (
	KillSwitchTableName   = "looma_killswitch"
	KillSwitchRoutingMark = 0x2026
)

type KillSwitchOptions struct {
	TUNName     string
	RoutingMark int
	AllowLAN    bool
	// BypassCgroupV2 exempts a cgroupv2 path (e.g., geph5-client sidecar scope)
	// whose sockets cannot set the sing-box fwmark directly.
	BypassCgroupV2 string
	// BypassGID optionally exempts outbound sockets owned by a dedicated GID.
	BypassGID int
}

// GenerateKillSwitchNFT returns an atomic nftables script that drops all outbound traffic
// except loopback, the VPN TUN interface, core-marked sockets, DHCP, optional sidecar exemptions, and optional LAN ranges.
func GenerateKillSwitchNFT(opts KillSwitchOptions) string {
	tun := strings.TrimSpace(opts.TUNName)
	if tun == "" {
		tun = "looma0"
	}
	mark := opts.RoutingMark
	if mark <= 0 {
		mark = KillSwitchRoutingMark
	}

	var b strings.Builder
	fmt.Fprintf(&b, "add table inet %s\n", KillSwitchTableName)
	fmt.Fprintf(&b, "flush table inet %s\n", KillSwitchTableName)
	fmt.Fprintf(&b, "table inet %s {\n", KillSwitchTableName)
	b.WriteString("  chain output {\n")
	b.WriteString("    type filter hook output priority 0; policy drop;\n")
	b.WriteString("    oifname \"lo\" accept\n")
	fmt.Fprintf(&b, "    oifname %q accept\n", tun)
	fmt.Fprintf(&b, "    meta mark 0x%x accept\n", mark)
	if cg := strings.Trim(strings.TrimSpace(opts.BypassCgroupV2), "/"); cg != "" {
		level := len(strings.Split(cg, "/"))
		fmt.Fprintf(&b, "    socket cgroupv2 level %d %q accept\n", level, cg)
	}
	if opts.BypassGID > 0 {
		fmt.Fprintf(&b, "    meta skgid %d accept\n", opts.BypassGID)
	}
	b.WriteString("    udp sport 68 udp dport 67 accept\n")
	if opts.AllowLAN {
		b.WriteString("    ip daddr { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } accept\n")
		b.WriteString("    ip6 daddr { fc00::/7, fe80::/10 } accept\n")
	}
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

type NFTExecutor func(ctx context.Context, script string) error

type KillSwitchManager struct {
	mu      sync.Mutex
	active  bool
	opts    KillSwitchOptions
	execute NFTExecutor
}

func NewKillSwitchManager(execFn NFTExecutor) *KillSwitchManager {
	if execFn == nil {
		execFn = defaultNFTExecutor
	}
	return &KillSwitchManager{execute: execFn}
}

func (m *KillSwitchManager) Enable(ctx context.Context, opts KillSwitchOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	script := GenerateKillSwitchNFT(opts)
	if err := m.execute(ctx, script); err != nil {
		return fmt.Errorf("enable kill switch: %w", err)
	}
	m.active = true
	m.opts = opts
	return nil
}

func (m *KillSwitchManager) Disable(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.active {
		return nil
	}
	script := fmt.Sprintf("delete table inet %s\n", KillSwitchTableName)
	if err := m.execute(ctx, script); err != nil {
		return fmt.Errorf("disable kill switch: %w", err)
	}
	m.active = false
	return nil
}

func (m *KillSwitchManager) Active() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

func defaultNFTExecutor(ctx context.Context, script string) error {
	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nft: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
