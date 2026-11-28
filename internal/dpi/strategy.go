package dpi

import (
	"runtime"

	"github.com/mumu-lhl/looma/internal/config"
)

type DesyncMethod string

const (
	MethodMultisplit DesyncMethod = "multisplit"
	MethodTLSRec     DesyncMethod = "tlsrec"
	MethodDisorder   DesyncMethod = "disorder"
	MethodOOB        DesyncMethod = "oob"
	MethodFakeTTL    DesyncMethod = "fake_ttl"
	MethodRawBadSum  DesyncMethod = "raw_badsum"
	MethodRawBadSeq  DesyncMethod = "raw_badseq"
	MethodIPFrag     DesyncMethod = "ip_frag"
)

// Strategy defines a complete TCP/TLS/HTTP/QUIC anti-DPI desync profile.
type Strategy struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Methods         []DesyncMethod `json:"methods"`
	SplitAtSNI      bool           `json:"split_at_sni"`
	MixSNICase      bool           `json:"mix_sni_case"`
	SplitCount      int            `json:"split_count"`
	FragmentSize    string         `json:"fragment_size"`
	FragmentSleep   string         `json:"fragment_sleep"`
	TTL             int            `json:"ttl"`
	OOBByte         byte           `json:"oob_byte"`
	FakeSNI         string         `json:"fake_sni"`
	HTTPMixHostCase bool           `json:"http_mix_host_case"`
	HTTPHeaderSpace bool           `json:"http_header_space"`
	QUICMode        string         `json:"quic_mode"` // "fake", "block", "pass"
	RequiresRaw     bool           `json:"requires_raw"`
}

// CapabilityStatus reports whether kernel/Raw Socket fake-packet injection is active
// or whether the engine seamlessly degraded to the rootless userspace engine.
type CapabilityStatus struct {
	OS              string `json:"os"`
	RawCapAvailable bool   `json:"raw_cap_available"`
	Degraded        bool   `json:"degraded"`
	Notice          string `json:"notice,omitempty"`
}

const (
	PresetStandardSplit        = "standard_split"
	PresetDisorderOOB          = "disorder_oob"
	PresetLowTTLFake           = "low_ttl_fake"
	PresetRawFakeMultidisorder = "raw_fake_multidisorder"
	PresetCompatMinimal        = "compat_minimal"
)

// BuiltinPresets returns the 5 curated anti-DPI presets ranging from lightweight
// compatibility to full Linux CAP_NET_RAW fake-packet + multidisorder.
func BuiltinPresets() []Strategy {
	return []Strategy{
		{
			ID:              PresetStandardSplit,
			Name:            "标准 SNI 精准分片",
			Description:     "定位 ClientHello SNI 偏移并在域名正中多段切分，叠加 TLS Record 拆分与 HTTP Host 混淆（免 Root，全平台适用）",
			Methods:         []DesyncMethod{MethodMultisplit, MethodTLSRec},
			SplitAtSNI:      true,
			MixSNICase:      true,
			SplitCount:      3,
			FragmentSize:    "10-30",
			FragmentSleep:   "2-8ms",
			TTL:             0,
			OOBByte:         'a',
			FakeSNI:         "www.bing.com",
			HTTPMixHostCase: true,
			HTTPHeaderSpace: false,
			QUICMode:        "fake",
			RequiresRaw:     false,
		},
		{
			ID:              PresetDisorderOOB,
			Name:            "乱序 + OOB 带外注入（免 Root 强力）",
			Description:     "在 SNI 中点注入带外字节（MSG_OOB）并用低 TTL 制造首片重传乱序，针对会做基础流重组的 DPI（免 Root，Linux/Android 通用）",
			Methods:         []DesyncMethod{MethodDisorder, MethodOOB, MethodMultisplit},
			SplitAtSNI:      true,
			MixSNICase:      true,
			SplitCount:      3,
			FragmentSize:    "8-24",
			FragmentSleep:   "2-6ms",
			TTL:             4,
			OOBByte:         'a',
			FakeSNI:         "www.bing.com",
			HTTPMixHostCase: true,
			HTTPHeaderSpace: true,
			QUICMode:        "fake",
			RequiresRaw:     false,
		},
		{
			ID:              PresetLowTTLFake,
			Name:            "低 TTL 诱饵 + TLS Record 拆分",
			Description:     "先以低 TTL 发送伪造 ClientHello 诱饵包毒化 DPI 状态机，再以多段 TLS Record 发送真实握手（免 Root）",
			Methods:         []DesyncMethod{MethodFakeTTL, MethodTLSRec, MethodMultisplit},
			SplitAtSNI:      true,
			MixSNICase:      true,
			SplitCount:      4,
			FragmentSize:    "10-25",
			FragmentSleep:   "2-8ms",
			TTL:             5,
			OOBByte:         'a',
			FakeSNI:         "www.microsoft.com",
			HTTPMixHostCase: true,
			HTTPHeaderSpace: true,
			QUICMode:        "fake",
			RequiresRaw:     false,
		},
		{
			ID:              PresetRawFakeMultidisorder,
			Name:            "强力内核 Fake + 多段乱序（Linux Raw）",
			Description:     "借助 Linux looma-helper (CAP_NET_RAW) 注入坏 TCP 校验和与错序列号诱饵包 + 多段乱序；无权限时自动无感降级为「乱序 + OOB」",
			Methods:         []DesyncMethod{MethodRawBadSum, MethodRawBadSeq, MethodDisorder, MethodMultisplit, MethodTLSRec},
			SplitAtSNI:      true,
			MixSNICase:      true,
			SplitCount:      4,
			FragmentSize:    "8-20",
			FragmentSleep:   "1-5ms",
			TTL:             4,
			OOBByte:         'a',
			FakeSNI:         "www.microsoft.com",
			HTTPMixHostCase: true,
			HTTPHeaderSpace: true,
			QUICMode:        "fake",
			RequiresRaw:     true,
		},
		{
			ID:              PresetCompatMinimal,
			Name:            "极简兼容（回落 TCP）",
			Description:     "仅做温和 SNI 切分并拦截 UDP 443 强制浏览器回落 TCP/TLS，适合对特殊包敏感的严格路由器或校园网",
			Methods:         []DesyncMethod{MethodMultisplit},
			SplitAtSNI:      true,
			MixSNICase:      false,
			SplitCount:      2,
			FragmentSize:    "16-40",
			FragmentSleep:   "1-4ms",
			TTL:             0,
			OOBByte:         'a',
			FakeSNI:         "www.bing.com",
			HTTPMixHostCase: true,
			HTTPHeaderSpace: false,
			QUICMode:        "block",
			RequiresRaw:     false,
		},
	}
}

func FindPreset(id string) (Strategy, bool) {
	for _, s := range BuiltinPresets() {
		if s.ID == id {
			return s, true
		}
	}
	return Strategy{}, false
}

// ResolveEffectiveStrategy builds the runtime Strategy from DirectBypassConfig
// and applies automatic seamless degradation when CAP_NET_RAW is unavailable.
func ResolveEffectiveStrategy(cfg config.DirectBypassConfig, rawCap bool) (Strategy, CapabilityStatus) {
	base, ok := FindPreset(cfg.PresetID)
	if !ok {
		base, _ = FindPreset(PresetStandardSplit)
		if cfg.PresetID != "" {
			base.ID = cfg.PresetID
		}
	}

	// Overlay explicit user overrides from DirectBypassConfig.
	if cfg.TLSFragmentSize != "" {
		base.FragmentSize = cfg.TLSFragmentSize
	}
	if cfg.TLSFragmentSleep != "" {
		base.FragmentSleep = cfg.TLSFragmentSleep
	}
	if cfg.SplitCount > 0 {
		base.SplitCount = cfg.SplitCount
	}
	if cfg.FakeTTL > 0 {
		base.TTL = cfg.FakeTTL
	}
	if cfg.QUICMode != "" {
		base.QUICMode = cfg.QUICMode
	}

	// Build explicit methods when user toggles custom flags.
	var methods []DesyncMethod
	wantsRaw := cfg.EnableRawFake || base.RequiresRaw
	if wantsRaw {
		if cfg.RawBadSum || !cfg.RawBadSeq {
			methods = append(methods, MethodRawBadSum)
		}
		if cfg.RawBadSeq {
			methods = append(methods, MethodRawBadSeq)
		}
	}
	if cfg.EnableFakeTTL || base.HasMethod(MethodFakeTTL) {
		methods = append(methods, MethodFakeTTL)
	}
	if cfg.EnableDisorder || base.HasMethod(MethodDisorder) {
		methods = append(methods, MethodDisorder)
	}
	if cfg.EnableOOB || base.HasMethod(MethodOOB) {
		methods = append(methods, MethodOOB)
	}
	if cfg.TLSRecordFragment {
		methods = append(methods, MethodTLSRec)
	}
	methods = append(methods, MethodMultisplit)
	base.Methods = dedupeMethods(methods)
	base.SplitAtSNI = cfg.SplitAtSNI || base.SplitAtSNI
	base.MixSNICase = cfg.MixSNICase
	base.HTTPMixHostCase = cfg.HTTPMixHostCase
	base.RequiresRaw = wantsRaw

	status := CapabilityStatus{
		OS:              runtime.GOOS,
		RawCapAvailable: rawCap,
	}
	if wantsRaw && !rawCap {
		// Automatically degrade to rootless userspace disorder + OOB without failing the connection.
		status.Degraded = true
		if runtime.GOOS == "linux" {
			status.Notice = "当前进程无 CAP_NET_RAW 权限，已自动降级为免 Root 用户态强力模式（乱序 + OOB）；如需内核级 Fake 诱饵包，可为 looma-helper 授予 cap_net_raw 权限。"
		} else {
			status.Notice = "当前平台使用免 Root 用户态强力反 DPI 模式（SNI 精准切分 + 乱序 + OOB）。"
		}
		base.Methods = stripRawMethods(base.Methods)
		if !base.HasMethod(MethodDisorder) {
			base.Methods = append([]DesyncMethod{MethodDisorder, MethodOOB}, base.Methods...)
		}
		base.RequiresRaw = false
		if base.TTL <= 0 {
			base.TTL = 4
		}
	}
	return base, status
}

func (s Strategy) HasMethod(m DesyncMethod) bool {
	for _, x := range s.Methods {
		if x == m {
			return true
		}
	}
	return false
}

func dedupeMethods(in []DesyncMethod) []DesyncMethod {
	seen := make(map[DesyncMethod]bool, len(in))
	out := make([]DesyncMethod, 0, len(in))
	for _, m := range in {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

func stripRawMethods(in []DesyncMethod) []DesyncMethod {
	out := make([]DesyncMethod, 0, len(in))
	for _, m := range in {
		if m == MethodRawBadSum || m == MethodRawBadSeq || m == MethodIPFrag {
			continue
		}
		out = append(out, m)
	}
	return out
}
