package core

import (
	"encoding/json"
	"fmt"

	"github.com/mumu-lhl/my-vpn/internal/config"
	"github.com/mumu-lhl/my-vpn/internal/platform"
	"github.com/mumu-lhl/my-vpn/internal/routing"
)

type CompileWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CompileResult struct {
	Config           map[string]any   `json:"config"`
	JSON             []byte           `json:"-"`
	FinalOutboundTag string           `json:"final_outbound_tag"`
	ChainSupportsUDP bool             `json:"chain_supports_udp"`
	Hop1ProbePort    int              `json:"hop1_probe_port,omitempty"`
	Warnings         []CompileWarning `json:"warnings,omitempty"`
}

// CompileSingBoxConfig translates AppConfig into a complete hiddify-sing-box v5 JSON configuration.
func CompileSingBoxConfig(cfg *config.AppConfig, tunFD int) (*CompileResult, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	var warnings []CompileWarning
	var outbounds []map[string]any
	var endpoints []map[string]any

	buildHop := func(ref config.HopRef, tag string, detour string, forceWarpMASQUEH2 bool) (supportsUDP bool, supportsIPv6 bool, err error) {
		switch ref.Kind {
		case config.HopGeph:
			port := cfg.Geph.LocalSocksPort
			if port <= 0 {
				port = 19909
			}
			ob := map[string]any{
				"type":        "socks",
				"tag":         tag,
				"server":      "127.0.0.1",
				"server_port": port,
				"version":     "5",
			}
			if detour != "" {
				ob["detour"] = detour
			}
			outbounds = append(outbounds, ob)
			return true, false, nil

		case config.HopPsiphon:
			timeout := cfg.Psiphon.EstablishTunnelTimeoutSec
			if timeout <= 0 {
				timeout = 30
			}
			ob := map[string]any{
				"type":                     "psiphon",
				"tag":                      tag,
				"establish_tunnel_timeout": fmt.Sprintf("%ds", timeout),
			}
			if cfg.Psiphon.EgressRegion != "" && cfg.Psiphon.EgressRegion != "auto" {
				ob["egress_region"] = cfg.Psiphon.EgressRegion
			}
			if cfg.Psiphon.DataDirectory != "" {
				ob["data_directory"] = cfg.Psiphon.DataDirectory
			}
			if detour != "" {
				ob["detour"] = detour
			}
			outbounds = append(outbounds, ob)
			warnings = append(warnings, CompileWarning{
				Code:    "PSIPHON_TCP_ONLY",
				Message: "Psiphon only supports TCP; non-direct UDP traffic is rejected to prevent leaks.",
			})
			return false, false, nil

		case config.HopWarp:
			w := cfg.Warp
			mode := w.Mode
			if mode == "" {
				mode = "auto"
			}
			vhttp := w.MASQUE.VHTTP
			if vhttp == "" {
				vhttp = "auto"
			}
			if forceWarpMASQUEH2 {
				mode = "masque"
				vhttp = "h2"
				warnings = append(warnings, CompileWarning{
					Code:    "WARP_AUTO_MASQUE_H2",
					Message: "Hop 1 is TCP-only; Hop 2 WARP automatically switched to MASQUE (HTTP/2 over TLS/TCP).",
				})
			}

			ep := map[string]any{
				"type": "warp",
				"tag":  tag,
				"mode": mode,
			}
			if w.LicenseKey != "" {
				ep["license_key"] = w.LicenseKey
			}
			if w.DeviceID != "" {
				ep["device_id"] = w.DeviceID
			}
			if w.PrivateKey != "" {
				ep["private_key"] = w.PrivateKey
			}
			if w.ForceRegister {
				ep["force_register"] = true
			}
			if w.MTU > 0 {
				ep["mtu"] = w.MTU
			}
			if w.EndpointMode != "" {
				ep["endpoint_mode"] = w.EndpointMode
			}
			if w.CleanIP != "" {
				ep["address"] = w.CleanIP
			}
			if w.Port > 0 {
				ep["port"] = w.Port
			}
			if w.NoiseCount != "" || w.NoiseMode != "" {
				ep["noise"] = map[string]any{
					"count": w.NoiseCount,
					"mode":  w.NoiseMode,
					"size":  w.NoiseSize,
					"delay": w.NoiseDelayMs,
				}
			}
			if w.ScannerMaxRTTMs > 0 || w.ScannerCount > 0 {
				ep["scanner"] = map[string]any{
					"max_rtt": fmt.Sprintf("%dms", w.ScannerMaxRTTMs),
					"timeout": fmt.Sprintf("%ds", w.ScannerTimeoutS),
					"count":   w.ScannerCount,
				}
			}
			if mode == "awg" || w.AWG.JunkCount > 0 {
				ep["amnezia"] = map[string]any{
					"jc":   w.AWG.JunkCount,
					"jmin": w.AWG.MinJunkSize,
					"jmax": w.AWG.MaxJunkSize,
				}
			}
			if mode == "masque" || mode == "auto" {
				masque := map[string]any{
					"vhttp":        vhttp,
					"ipv6":         w.MASQUE.IPv6,
					"tls_fragment": w.MASQUE.TLSFragment,
					"ech":          w.MASQUE.ECH,
				}
				if w.MASQUE.ServerName != "" {
					masque["server_name"] = w.MASQUE.ServerName
				}
				if w.MASQUE.KeepaliveSec > 0 {
					masque["keepalive"] = fmt.Sprintf("%ds", w.MASQUE.KeepaliveSec)
				}
				ep["masque"] = masque
			}
			if detour != "" {
				ep["detour"] = detour
			}
			endpoints = append(endpoints, ep)
			return true, w.MASQUE.IPv6, nil

		case config.HopExternal:
			node := cfg.FindExternalNode(ref.NodeID)
			if node == nil {
				return false, false, fmt.Errorf("external node not found: %s", ref.NodeID)
			}
			ob := make(map[string]any, len(node.RawOutbound)+2)
			for k, v := range node.RawOutbound {
				ob[k] = v
			}
			ob["tag"] = tag
			if detour != "" {
				ob["detour"] = detour
			}
			outbounds = append(outbounds, ob)
			return node.SupportsUDP, node.SupportsIPv6, nil

		default:
			return false, false, fmt.Errorf("unsupported hop kind: %s", ref.Kind)
		}
	}

	isServerlessBypass := cfg.Chain.Hop1.Kind == config.HopDirectBypass
	var finalTag string
	var chainUDP, chainIPv6 bool
	effectiveTwoHop := cfg.Chain.TwoHop && !isServerlessBypass

	if isServerlessBypass {
		if cfg.Chain.TwoHop {
			warnings = append(warnings, CompileWarning{
				Code:    "DIRECT_BYPASS_SINGLE_HOP",
				Message: "直连绕过（无服务器）模式不支持双跳，已自动按单跳直连绕过编译。",
			})
		}
		if cfg.Routing.DirectBypass.Scope == config.DirectBypassScopeList {
			finalTag = "direct"
		} else {
			finalTag = "direct-bypass"
		}
		chainUDP = true
		chainIPv6 = true
	} else {
		hop1UDP, hop1IPv6, err := buildHop(cfg.Chain.Hop1, "hop-1", "", false)
		if err != nil {
			return nil, fmt.Errorf("build hop1: %w", err)
		}
		finalTag = "hop-1"
		chainUDP = hop1UDP
		chainIPv6 = hop1IPv6

		if effectiveTwoHop {
			forceH2 := !hop1UDP && cfg.Chain.Hop2.Kind == config.HopWarp
			hop2UDP, hop2IPv6, err := buildHop(cfg.Chain.Hop2, "hop-2", "hop-1", forceH2)
			if err != nil {
				return nil, fmt.Errorf("build hop2: %w", err)
			}
			finalTag = "hop-2"
			// If Hop 2 is WARP over MASQUE H2 (IP-in-HTTP/2), WARP itself carries UDP inside TCP H2!
			if forceH2 {
				chainUDP = true
			} else {
				chainUDP = hop1UDP && hop2UDP
			}
			chainIPv6 = hop2IPv6
		}
	}

	fragSize := cfg.Routing.DirectBypass.TLSFragmentSize
	if fragSize == "" {
		fragSize = "10-30"
	}
	fragSleep := cfg.Routing.DirectBypass.TLSFragmentSleep
	if fragSleep == "" {
		fragSleep = "2-8ms"
	}
	outbounds = append(outbounds,
		map[string]any{
			"type": "direct",
			"tag":  "direct",
		},
		map[string]any{
			"type":                "direct",
			"tag":                 "direct-bypass",
			"tls_fragment":        true,
			"tls_record_fragment": cfg.Routing.DirectBypass.TLSRecordFragment,
			"tls_tricks": map[string]any{
				"mixedcase_sni":  true,
				"fragment_size":  fragSize,
				"fragment_sleep": fragSleep,
			},
		},
	)

	listenAddr := "127.0.0.1"
	if cfg.Inbound.AllowLAN {
		listenAddr = "0.0.0.0"
	}
	inbounds := []map[string]any{
		{
			"type":        "socks",
			"tag":         "socks-in",
			"listen":      listenAddr,
			"listen_port": cfg.Inbound.SocksPort,
		},
		{
			"type":        "http",
			"tag":         "http-in",
			"listen":      listenAddr,
			"listen_port": cfg.Inbound.HTTPPort,
		},
	}

	hop1ProbePort := 0
	if effectiveTwoHop {
		hop1ProbePort = cfg.Probe.Hop1ProbePort
		if hop1ProbePort <= 0 {
			hop1ProbePort = 10810
		}
		// Loopback-only inbound routed directly to hop-1 so per-hop connectivity & IP probes can test Hop 1 independently.
		inbounds = append(inbounds, map[string]any{
			"type":        "socks",
			"tag":         "hop1-probe-in",
			"listen":      "127.0.0.1",
			"listen_port": hop1ProbePort,
		})
	}

	if cfg.Inbound.Mode == config.InboundTUN {
		tunIn := map[string]any{
			"type":           "tun",
			"tag":            "tun-in",
			"interface_name": cfg.Inbound.TUNName,
			"address":        []string{cfg.Inbound.TUNAddress},
			"auto_route":     cfg.Inbound.AutoRoute,
			"strict_route":   cfg.Inbound.StrictRoute,
			"stack":          "mixed",
		}
		if tunFD > 0 {
			tunIn["file_descriptor"] = tunFD
		}
		inbounds = append(inbounds, tunIn)
	}

	proxyDNSAddr := cfg.DNS.ProxyDNS
	proxyDNSDetour := finalTag
	if isServerlessBypass {
		if cfg.Routing.DirectBypass.DoHURL != "" {
			proxyDNSAddr = cfg.Routing.DirectBypass.DoHURL
		}
		proxyDNSDetour = "direct-bypass"
	}
	dnsServers := []map[string]any{
		{
			"tag":     "dns-proxy",
			"address": proxyDNSAddr,
			"detour":  proxyDNSDetour,
		},
		{
			"tag":     "dns-direct",
			"address": cfg.DNS.DirectDNS,
			"detour":  "direct",
		},
	}
	if cfg.DNS.FallbackDNS != "" {
		dnsServers = append(dnsServers, map[string]any{
			"tag":     "dns-fallback",
			"address": cfg.DNS.FallbackDNS,
			"detour":  finalTag,
		})
	}
	if cfg.DNS.FakeIP {
		dnsServers = append(dnsServers, map[string]any{
			"tag":     "dns-fakeip",
			"address": "fakeip",
		})
	}

	var dnsRules []map[string]any
	// Prevent DNS bootstrap deadlock when geph5-client resolves broker/bridge hostnames in TUN mode
	dnsRules = append(dnsRules, map[string]any{
		"process_name": []string{"geph5-client", "libgeph5.so"},
		"server":       "dns-direct",
	})
	if cfg.Routing.DirectBypass.EnableDefaultPreset {
		// Resolve direct-bypass domains via encrypted proxy DNS to avoid ISP DNS poisoning.
		dnsRules = append(dnsRules, map[string]any{
			"domain_suffix": routing.DefaultDirectBypassDomains,
			"server":        "dns-proxy",
		})
	}
	if cfg.Routing.BypassCN {
		dnsRules = append(dnsRules, map[string]any{
			"rule_set": []string{"geosite-cn"},
			"server":   "dns-direct",
		})
	}
	if cfg.DNS.FakeIP {
		dnsRules = append(dnsRules, map[string]any{
			"query_type": []string{"A", "AAAA"},
			"server":     "dns-fakeip",
		})
	}

	dnsBlock := map[string]any{
		"servers":           dnsServers,
		"rules":             dnsRules,
		"final":             "dns-proxy",
		"independent_cache": true,
	}
	if cfg.DNS.FakeIP {
		dnsBlock["fakeip"] = map[string]any{
			"enabled":     true,
			"inet4_range": "198.18.0.0/15",
			"inet6_range": "fc00::/18",
		}
	}

	routeBlock := routing.CompileRoute(cfg.Routing, cfg.DNS, cfg.Inbound.AllowLAN, finalTag, chainUDP, chainIPv6)
	if effectiveTwoHop {
		if rules, ok := routeBlock["rules"].([]map[string]any); ok {
			probeRule := map[string]any{
				"inbound":  []string{"hop1-probe-in"},
				"outbound": "hop-1",
			}
			routeBlock["rules"] = append([]map[string]any{probeRule}, rules...)
		}
	}
	if cfg.Inbound.KillSwitch {
		routeBlock["default_mark"] = platform.KillSwitchRoutingMark
	}

	doc := map[string]any{
		"log": map[string]any{
			"level":     "info",
			"timestamp": true,
		},
		"dns":       dnsBlock,
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"route":     routeBlock,
	}
	if len(endpoints) > 0 {
		doc["endpoints"] = endpoints
	}

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}

	return &CompileResult{
		Config:           doc,
		JSON:             raw,
		FinalOutboundTag: finalTag,
		ChainSupportsUDP: chainUDP,
		Hop1ProbePort:    hop1ProbePort,
		Warnings:         warnings,
	}, nil
}
