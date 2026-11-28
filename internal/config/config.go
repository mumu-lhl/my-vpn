package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"runtime"
	"strings"
)

type HopKind string

const (
	HopGeph         HopKind = "geph"
	HopPsiphon      HopKind = "psiphon"
	HopWarp         HopKind = "warp"
	HopVPNGate      HopKind = "vpngate"
	HopExternal     HopKind = "external"
	HopDirectBypass HopKind = "direct_bypass"
)

const (
	DirectBypassScopeGlobal = "global"
	DirectBypassScopeList   = "list"
)

type InboundMode string

const (
	InboundProxy InboundMode = "proxy"
	InboundTUN   InboundMode = "tun"
)

type PerAppMode string

const (
	PerAppAll       PerAppMode = "all"
	PerAppWhitelist PerAppMode = "whitelist"
	PerAppBlacklist PerAppMode = "blacklist"
)

type AppConfig struct {
	Inbound       InboundConfig  `json:"inbound"`
	Chain         ChainConfig    `json:"chain"`
	Geph          GephConfig     `json:"geph"`
	Psiphon       PsiphonConfig  `json:"psiphon"`
	Warp          WarpConfig     `json:"warp"`
	VPNGate       VPNGateConfig  `json:"vpngate"`
	Subscriptions []Subscription `json:"subscriptions"`
	ExternalNodes []ExternalNode `json:"external_nodes"`
	Routing       RoutingConfig  `json:"routing"`
	DNS           DNSConfig      `json:"dns"`
	Probe         ProbeConfig    `json:"probe"`
	Logs          LogsConfig     `json:"logs"`
	Linux         LinuxConfig    `json:"linux"`
	Android       AndroidConfig  `json:"android"`
}

type ProbeConfig struct {
	Mode          string `json:"mode"` // "builtin" or "custom"
	TestURL       string `json:"test_url"`
	CustomURL     string `json:"custom_url,omitempty"`
	TimeoutMs     int    `json:"timeout_ms"`
	Hop1ProbePort int    `json:"hop1_probe_port"`
}

type LogsConfig struct {
	Enabled bool `json:"enabled"`
}

type InboundConfig struct {
	Mode        InboundMode `json:"mode"`
	SocksPort   int         `json:"socks_port"`
	HTTPPort    int         `json:"http_port"`
	AllowLAN    bool        `json:"allow_lan"`
	TUNName     string      `json:"tun_name"`
	TUNAddress  string      `json:"tun_address"`
	AutoRoute   bool        `json:"auto_route"`
	StrictRoute bool        `json:"strict_route"`
	KillSwitch  bool        `json:"kill_switch"`
}

type HopRef struct {
	Kind   HopKind `json:"kind"`
	NodeID string  `json:"node_id,omitempty"`
}

type ChainConfig struct {
	TwoHop bool   `json:"two_hop"`
	Hop1   HopRef `json:"hop1"`
	Hop2   HopRef `json:"hop2"`
}

type GephConfig struct {
	Secret         string `json:"secret"`
	ExitCountry    string `json:"exit_country"` // "" or "auto" means auto
	ExitCity       string `json:"exit_city,omitempty"`
	BrokerURL      string `json:"broker_url,omitempty"`
	LocalSocksPort int    `json:"local_socks_port"`
	ControlPort    int    `json:"control_port"`
}

type PsiphonConfig struct {
	EgressRegion              string `json:"egress_region"` // "" means auto
	EstablishTunnelTimeoutSec int    `json:"establish_tunnel_timeout_sec"`
	DataDirectory             string `json:"data_directory,omitempty"`
}

type VPNGateSelectedNode struct {
	ID           string `json:"id"`
	HostName     string `json:"host_name"`
	IP           string `json:"ip"`
	CountryLong  string `json:"country_long"`
	CountryShort string `json:"country_short"`
	Server       string `json:"server"`
	ServerPort   int    `json:"server_port"`
	Network      string `json:"network"`
	SupportsUDP  bool   `json:"supports_udp"`
	CACert       string `json:"ca_cert,omitempty"`
	ClientCert   string `json:"client_cert,omitempty"`
	ClientKey    string `json:"client_key,omitempty"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
}

type VPNGateConfig struct {
	APIURL       string               `json:"api_url,omitempty"`
	SelectedNode *VPNGateSelectedNode `json:"selected_node,omitempty"`
}

type WarpConfig struct {
	Mode            string         `json:"mode"` // auto, wg, awg, masque
	LicenseKey      string         `json:"license_key,omitempty"`
	DeviceID        string         `json:"device_id,omitempty"`
	PrivateKey      string         `json:"private_key,omitempty"`
	ForceRegister   bool           `json:"force_register"`
	MTU             int            `json:"mtu"`           // 0 means auto
	EndpointMode    string         `json:"endpoint_mode"` // cloudflare, scan, custom
	CleanIP         string         `json:"clean_ip,omitempty"`
	Port            int            `json:"port,omitempty"`
	NoiseCount      string         `json:"noise_count,omitempty"`
	NoiseMode       string         `json:"noise_mode,omitempty"` // standard, random, quic, dns, sip, stun
	NoiseSize       string         `json:"noise_size,omitempty"`
	NoiseDelayMs    string         `json:"noise_delay_ms,omitempty"`
	ScannerMaxRTTMs int            `json:"scanner_max_rtt_ms,omitempty"`
	ScannerTimeoutS int            `json:"scanner_timeout_s,omitempty"`
	ScannerCount    int            `json:"scanner_count,omitempty"`
	AWG             WarpAWGConfig  `json:"awg"`
	MASQUE          WarpMASQUEConf `json:"masque"`
}

type WarpAWGConfig struct {
	JunkCount   int `json:"junk_count,omitempty"`
	MinJunkSize int `json:"min_junk_size,omitempty"`
	MaxJunkSize int `json:"max_junk_size,omitempty"`
}

type WarpMASQUEConf struct {
	VHTTP        string `json:"vhttp"` // auto, h3, h2
	ServerName   string `json:"server_name,omitempty"`
	IPv6         bool   `json:"ipv6"`
	KeepaliveSec int    `json:"keepalive_sec,omitempty"`
	TLSFragment  bool   `json:"tls_fragment"`
	ECH          bool   `json:"ech"`
}

type Subscription struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	ProxyMode          string `json:"proxy_mode,omitempty"` // "auto" (default), "proxy", "direct"
	UpdateIntervalHour int    `json:"update_interval_hour"`
	LastUpdatedUnix    int64  `json:"last_updated_unix"`
	Enabled            bool   `json:"enabled"`
	UploadBytes        int64  `json:"upload_bytes,omitempty"`
	DownloadBytes      int64  `json:"download_bytes,omitempty"`
	TotalBytes         int64  `json:"total_bytes,omitempty"`
	ExpireUnix         int64  `json:"expire_unix,omitempty"`
	LastError          string `json:"last_error,omitempty"`
}

type ExternalNode struct {
	ID             string         `json:"id"`
	SubscriptionID string         `json:"subscription_id,omitempty"`
	Name           string         `json:"name"`
	Protocol       string         `json:"protocol"` // vless, vmess, trojan, shadowsocks, hysteria2, tuic, wireguard, socks, http
	Server         string         `json:"server"`
	Port           int            `json:"port"`
	SupportsUDP    bool           `json:"supports_udp"`
	SupportsIPv6   bool           `json:"supports_ipv6"`
	LatencyMs      int            `json:"latency_ms,omitempty"`
	RawOutbound    map[string]any `json:"raw_outbound"`
}

type RemoteRuleSet struct {
	Tag             string `json:"tag"`
	URL             string `json:"url"`
	UpdateIntervalH int    `json:"update_interval_h"`
	Enabled         bool   `json:"enabled"`
}

type CustomRule struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	Domain        []string `json:"domain,omitempty"`
	DomainSuffix  []string `json:"domain_suffix,omitempty"`
	DomainKeyword []string `json:"domain_keyword,omitempty"`
	DomainRegex   []string `json:"domain_regex,omitempty"`
	IPCIDR        []string `json:"ip_cidr,omitempty"`
	Port          []int    `json:"port,omitempty"`
	Protocol      []string `json:"protocol,omitempty"`
	ProcessName   []string `json:"process_name,omitempty"`
	PackageName   []string `json:"package_name,omitempty"`
	RuleSet       []string `json:"rule_set,omitempty"`
	Action        string   `json:"action"` // proxy, direct, direct-bypass, reject, or node:<id>
}

type DirectBypassConfig struct {
	EnableDefaultPreset bool     `json:"enable_default_preset"`
	Scope               string   `json:"scope"` // "global" (all non-CN/private) or "list" (preset + custom rules only)
	PresetID            string   `json:"preset_id,omitempty"`
	SplitAtSNI          bool     `json:"split_at_sni"`
	MixSNICase          bool     `json:"mix_sni_case"`
	SplitCount          int      `json:"split_count,omitempty"`
	TLSFragmentSize     string   `json:"tls_fragment_size"`
	TLSFragmentSleep    string   `json:"tls_fragment_sleep"`
	TLSRecordFragment   bool     `json:"tls_record_fragment"`
	EnableDisorder      bool     `json:"enable_disorder"`
	EnableOOB           bool     `json:"enable_oob"`
	EnableFakeTTL       bool     `json:"enable_fake_ttl"`
	FakeTTL             int      `json:"fake_ttl,omitempty"`
	EnableRawFake       bool     `json:"enable_raw_fake"`
	RawBadSum           bool     `json:"raw_bad_sum"`
	RawBadSeq           bool     `json:"raw_bad_seq"`
	HTTPMixHostCase     bool     `json:"http_mix_host_case"`
	QUICMode            string   `json:"quic_mode,omitempty"` // "fake", "block", "pass"
	EnableECH           bool     `json:"enable_ech"`
	AutoLearnPerDomain  bool     `json:"auto_learn_per_domain"`
	DoHURL              string   `json:"doh_url,omitempty"`
	DoHStaticIPs        []string `json:"doh_static_ips,omitempty"`
	DoHDisableSNI       bool     `json:"doh_disable_sni"`
}

type RoutingConfig struct {
	BypassCN           bool               `json:"bypass_cn"`
	BlockAds           bool               `json:"block_ads"`
	BypassPrivate      bool               `json:"bypass_private"`
	DirectBypass       DirectBypassConfig `json:"direct_bypass"`
	RuleSetUpdateHours int                `json:"rule_set_update_hours"`
	RemoteRuleSets     []RemoteRuleSet    `json:"remote_rule_sets,omitempty"`
	CustomRules        []CustomRule       `json:"custom_rules,omitempty"`
}

type DNSConfig struct {
	DirectDNS                         string   `json:"direct_dns"`
	ProxyDNS                          string   `json:"proxy_dns"`
	FallbackDNS                       string   `json:"fallback_dns"`
	FakeIP                            bool     `json:"fake_ip"`
	FakeIPExclude                     []string `json:"fake_ip_exclude,omitempty"`
	HijackDNS                         bool     `json:"hijack_dns"`
	BlockNonDirectIPv6WhenUnsupported bool     `json:"block_non_direct_ipv6_when_unsupported"`
	RejectNonDirectUDPWhenTCPOnly     bool     `json:"reject_non_direct_udp_when_tcp_only"`
}

type LinuxConfig struct {
	SetSystemProxy bool   `json:"set_system_proxy"`
	MinimizeToTray bool   `json:"minimize_to_tray"`
	CoreHelperPath string `json:"core_helper_path,omitempty"`
}

type AndroidConfig struct {
	PerAppMode     PerAppMode `json:"per_app_mode"`
	PerAppPackages []string   `json:"per_app_packages,omitempty"`
}

func DefaultConfig() *AppConfig {
	defaultInboundMode := InboundProxy
	if runtime.GOOS == "android" {
		defaultInboundMode = InboundTUN
	}
	return &AppConfig{
		Inbound: InboundConfig{
			Mode:        defaultInboundMode,
			SocksPort:   10808,
			HTTPPort:    10809,
			AllowLAN:    false,
			TUNName:     "looma0",
			TUNAddress:  "172.19.0.1/30",
			AutoRoute:   true,
			StrictRoute: true,
		},
		Chain: ChainConfig{
			TwoHop: true,
			Hop1:   HopRef{Kind: HopGeph},
			Hop2:   HopRef{Kind: HopWarp},
		},
		Geph: GephConfig{
			ExitCountry:    "auto",
			LocalSocksPort: 19909,
			ControlPort:    19910,
		},
		Psiphon: PsiphonConfig{
			EgressRegion:              "",
			EstablishTunnelTimeoutSec: 30,
		},
		Warp: WarpConfig{
			Mode:            "auto",
			EndpointMode:    "scan",
			NoiseMode:       "quic",
			NoiseCount:      "5-10",
			NoiseSize:       "50-100",
			NoiseDelayMs:    "10-20",
			ScannerMaxRTTMs: 1200,
			ScannerTimeoutS: 10,
			ScannerCount:    8,
			MASQUE: WarpMASQUEConf{
				VHTTP:        "auto",
				KeepaliveSec: 30,
				TLSFragment:  true,
				ECH:          true,
			},
		},
		Routing: RoutingConfig{
			BypassCN:      true,
			BlockAds:      true,
			BypassPrivate: true,
			DirectBypass: DirectBypassConfig{
				EnableDefaultPreset: true,
				Scope:               DirectBypassScopeGlobal,
				PresetID:            "disorder_oob",
				SplitAtSNI:          true,
				MixSNICase:          true,
				SplitCount:          3,
				TLSFragmentSize:     "10-30",
				TLSFragmentSleep:    "2-8ms",
				TLSRecordFragment:   true,
				EnableDisorder:      true,
				EnableOOB:           true,
				FakeTTL:             4,
				RawBadSum:           true,
				RawBadSeq:           true,
				HTTPMixHostCase:     true,
				QUICMode:            "fake",
				EnableECH:           true,
				AutoLearnPerDomain:  true,
				DoHDisableSNI:       true,
			},
			RuleSetUpdateHours: 24,
		},
		DNS: DNSConfig{
			DirectDNS:                         "https://dns.alidns.com/dns-query",
			ProxyDNS:                          "https://1.1.1.1/dns-query",
			FallbackDNS:                       "https://dns.google/dns-query",
			FakeIP:                            true,
			HijackDNS:                         true,
			BlockNonDirectIPv6WhenUnsupported: true,
			RejectNonDirectUDPWhenTCPOnly:     true,
		},
		Probe: ProbeConfig{
			Mode:          "builtin",
			TestURL:       "http://connectivitycheck.gstatic.com/generate_204",
			TimeoutMs:     5000,
			Hop1ProbePort: 10810,
		},
		Linux: LinuxConfig{
			SetSystemProxy: true,
			MinimizeToTray: true,
		},
		Android: AndroidConfig{
			PerAppMode: PerAppAll,
		},
	}
}

// EffectiveProbeURL returns the active connectivity test URL according to Probe.Mode.
func (c *AppConfig) EffectiveProbeURL() string {
	if c.Probe.Mode == "custom" && strings.TrimSpace(c.Probe.CustomURL) != "" {
		return strings.TrimSpace(c.Probe.CustomURL)
	}
	if strings.TrimSpace(c.Probe.TestURL) != "" {
		return strings.TrimSpace(c.Probe.TestURL)
	}
	return "http://connectivitycheck.gstatic.com/generate_204"
}

func (c *AppConfig) Validate() error {
	if c.Inbound.SocksPort < 1 || c.Inbound.SocksPort > 65535 {
		return fmt.Errorf("invalid socks_port: %d", c.Inbound.SocksPort)
	}
	if c.Inbound.HTTPPort < 1 || c.Inbound.HTTPPort > 65535 {
		return fmt.Errorf("invalid http_port: %d", c.Inbound.HTTPPort)
	}
	if c.Inbound.SocksPort == c.Inbound.HTTPPort {
		return fmt.Errorf("socks_port and http_port must differ")
	}
	for _, raw := range []string{c.Probe.TestURL, c.Probe.CustomURL} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("invalid probe URL: %s", raw)
		}
	}
	if qm := strings.TrimSpace(c.Routing.DirectBypass.QUICMode); qm != "" && qm != "fake" && qm != "block" && qm != "pass" {
		return fmt.Errorf("invalid direct_bypass.quic_mode: %s", qm)
	}
	if scope := strings.TrimSpace(c.Routing.DirectBypass.Scope); scope != "" && scope != DirectBypassScopeGlobal && scope != DirectBypassScopeList {
		return fmt.Errorf("invalid direct_bypass.scope: %s", scope)
	}
	if rawDoH := strings.TrimSpace(c.Routing.DirectBypass.DoHURL); rawDoH != "" {
		u, err := url.Parse(rawDoH)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("invalid direct_bypass.doh_url: %s", rawDoH)
		}
	}
	for _, rawIP := range c.Routing.DirectBypass.DoHStaticIPs {
		rawIP = strings.TrimSpace(rawIP)
		if rawIP == "" {
			continue
		}
		if _, err := netip.ParseAddr(rawIP); err != nil {
			return fmt.Errorf("invalid direct_bypass.doh_static_ips entry: %s", rawIP)
		}
	}
	return nil
}

func (c *AppConfig) FindExternalNode(id string) *ExternalNode {
	if id == "auto" || strings.HasPrefix(id, "auto:") {
		subFilter := ""
		if strings.HasPrefix(id, "auto:") {
			subFilter = strings.TrimPrefix(id, "auto:")
		}
		var best *ExternalNode
		var fallback *ExternalNode
		for i := range c.ExternalNodes {
			n := &c.ExternalNodes[i]
			if subFilter != "" && n.SubscriptionID != subFilter {
				continue
			}
			if fallback == nil {
				fallback = n
			}
			if n.LatencyMs > 0 && (best == nil || n.LatencyMs < best.LatencyMs) {
				best = n
			}
		}
		if best != nil {
			return best
		}
		return fallback
	}
	for i := range c.ExternalNodes {
		if c.ExternalNodes[i].ID == id {
			return &c.ExternalNodes[i]
		}
	}
	return nil
}
