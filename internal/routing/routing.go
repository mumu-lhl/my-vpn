package routing

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mumu-lhl/my-vpn/internal/bootstrap"
	"github.com/mumu-lhl/my-vpn/internal/config"
)

const (
	DefaultGeoIPCNURL = "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs"
	DefaultGeoSiteCN  = "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs"
	DefaultGeoSiteAds = "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-category-ads-all.srs"
)

// DefaultDirectBypassDomains lists sites typically affected by DNS/SNI filtering while remaining IP-reachable.
var DefaultDirectBypassDomains = []string{
	"pixiv.net",
	"pximg.net",
	"e-hentai.org",
	"exhentai.org",
	"ehgt.org",
	"github.com",
	"githubusercontent.com",
	"wikipedia.org",
}

// CompileRoute builds sing-box route.rules and route.rule_set from RoutingConfig and anti-leak DNS settings.
func CompileRoute(rc config.RoutingConfig, dns config.DNSConfig, allowLAN bool, finalTag string, chainSupportsUDP, chainSupportsIPv6 bool) map[string]any {
	var rules []map[string]any
	var ruleSets []map[string]any

	if dns.HijackDNS {
		rules = append(rules, map[string]any{
			"protocol": "dns",
			"action":   "hijack-dns",
		})
	}

	// Prevent Linux TUN routing loop when geph5-client sidecar runs as Hop 1/Hop 2.
	rules = append(rules, map[string]any{
		"process_name": []string{"geph5-client", "libgeph5.so"},
		"outbound":     "direct",
	})

	// Required for LAN proxy sharing and local network reachability.
	if rc.BypassPrivate || allowLAN {
		rules = append(rules, map[string]any{
			"ip_is_private": true,
			"outbound":      "direct",
		})
	}
	if rc.BlockAds {
		ruleSets = append(ruleSets, map[string]any{
			"tag":             "geosite-ads",
			"type":            "remote",
			"format":          "binary",
			"url":             DefaultGeoSiteAds,
			"download_detour": finalTag,
		})
		rules = append(rules, map[string]any{
			"rule_set": []string{"geosite-ads"},
			"action":   "reject",
		})
	}
	// Custom rules take precedence over the default CN direct preset.
	for _, r := range rc.CustomRules {
		if !r.Enabled {
			continue
		}
		rule := map[string]any{}
		if len(r.Domain) > 0 {
			rule["domain"] = r.Domain
		}
		if len(r.DomainSuffix) > 0 {
			rule["domain_suffix"] = r.DomainSuffix
		}
		if len(r.DomainKeyword) > 0 {
			rule["domain_keyword"] = r.DomainKeyword
		}
		if len(r.DomainRegex) > 0 {
			rule["domain_regex"] = r.DomainRegex
		}
		if len(r.IPCIDR) > 0 {
			rule["ip_cidr"] = r.IPCIDR
		}
		if len(r.Port) > 0 {
			rule["port"] = r.Port
		}
		if len(r.Protocol) > 0 {
			rule["protocol"] = r.Protocol
		}
		if len(r.ProcessName) > 0 {
			rule["process_name"] = r.ProcessName
		}
		if len(r.PackageName) > 0 {
			rule["package_name"] = r.PackageName
		}
		if len(r.RuleSet) > 0 {
			rule["rule_set"] = r.RuleSet
		}
		switch r.Action {
		case "reject":
			rule["action"] = "reject"
		case "direct":
			rule["outbound"] = "direct"
		case "direct-bypass":
			rule["outbound"] = "direct-bypass"
		case "proxy", "":
			rule["outbound"] = finalTag
		default:
			rule["outbound"] = r.Action
		}
		rules = append(rules, rule)
	}
	if rc.DirectBypass.EnableDefaultPreset {
		rules = append(rules, map[string]any{
			"domain_suffix": DefaultDirectBypassDomains,
			"outbound":      "direct-bypass",
		})
	}
	if rc.BypassCN {
		ruleSets = append(ruleSets,
			map[string]any{
				"tag":             "geosite-cn",
				"type":            "remote",
				"format":          "binary",
				"url":             DefaultGeoSiteCN,
				"download_detour": finalTag,
			},
			map[string]any{
				"tag":             "geoip-cn",
				"type":            "remote",
				"format":          "binary",
				"url":             DefaultGeoIPCNURL,
				"download_detour": finalTag,
			},
		)
		rules = append(rules, map[string]any{
			"rule_set": []string{"geosite-cn", "geoip-cn"},
			"outbound": "direct",
		})
	}
	for _, rs := range rc.RemoteRuleSets {
		if !rs.Enabled || rs.Tag == "" || rs.URL == "" {
			continue
		}
		ruleSets = append(ruleSets, map[string]any{
			"tag":             rs.Tag,
			"type":            "remote",
			"format":          "binary",
			"url":             rs.URL,
			"download_detour": finalTag,
		})
	}
	// Reject non-direct UDP on TCP-only chains to prevent WebRTC/STUN direct leaks.
	if dns.RejectNonDirectUDPWhenTCPOnly && !chainSupportsUDP {
		rules = append(rules, map[string]any{
			"network": "udp",
			"action":  "reject",
		})
	}
	// Reject non-direct IPv6 when exit lacks IPv6 to prevent bypass leaks.
	if dns.BlockNonDirectIPv6WhenUnsupported && !chainSupportsIPv6 {
		rules = append(rules, map[string]any{
			"ip_version": 6,
			"action":     "reject",
		})
	}
	return map[string]any{
		"auto_detect_interface": true,
		"find_process":          true,
		"final":                 finalTag,
		"rules":                 rules,
		"rule_set":              ruleSets,
	}
}

type MatchQuery struct {
	Domain      string
	IP          string
	Port        int
	Network     string // tcp / udp
	ProcessName string
	PackageName string
}

// Evaluate returns the route action ("direct", "direct-bypass", "reject", "proxy", or custom outbound) matching CompileRoute precedence.
func Evaluate(rc config.RoutingConfig, dns config.DNSConfig, q MatchQuery, chainSupportsUDP, chainSupportsIPv6 bool) string {
	if strings.EqualFold(q.ProcessName, "geph5-client") || strings.EqualFold(q.ProcessName, "libgeph5.so") {
		return "direct"
	}
	if rc.BypassPrivate && q.IP != "" {
		if addr, err := netip.ParseAddr(q.IP); err == nil {
			if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
				return "direct"
			}
		}
	}
	if rc.BlockAds && isCommonAdDomain(q.Domain) {
		return "reject"
	}
	for _, r := range rc.CustomRules {
		if !r.Enabled {
			continue
		}
		if matchesCustomRule(r, q) {
			if r.Action == "" {
				return "proxy"
			}
			return r.Action
		}
	}
	if rc.DirectBypass.EnableDefaultPreset && matchesDomainSuffix(q.Domain, DefaultDirectBypassDomains) {
		return "direct-bypass"
	}
	if rc.BypassCN && (strings.HasSuffix(strings.ToLower(q.Domain), ".cn") || strings.EqualFold(q.Domain, "baidu.com") || strings.HasSuffix(strings.ToLower(q.Domain), ".baidu.com")) {
		return "direct"
	}
	if dns.RejectNonDirectUDPWhenTCPOnly && !chainSupportsUDP && strings.EqualFold(q.Network, "udp") {
		return "reject"
	}
	if dns.BlockNonDirectIPv6WhenUnsupported && !chainSupportsIPv6 && q.IP != "" {
		if addr, err := netip.ParseAddr(q.IP); err == nil && addr.Is6() {
			return "reject"
		}
	}
	return "proxy"
}

func isCommonAdDomain(d string) bool {
	ld := strings.ToLower(d)
	for _, s := range []string{"doubleclick.net", "googlesyndication.com", "adservice.google.com"} {
		if ld == s || strings.HasSuffix(ld, "."+s) {
			return true
		}
	}
	return false
}

func matchesDomainSuffix(domain string, suffixes []string) bool {
	ld := strings.ToLower(domain)
	for _, s := range suffixes {
		ls := strings.ToLower(s)
		if ld == ls || strings.HasSuffix(ld, "."+ls) {
			return true
		}
	}
	return false
}

func matchesCustomRule(r config.CustomRule, q MatchQuery) bool {
	matchedAny := false
	if len(r.Domain) > 0 {
		matchedAny = true
		ok := false
		for _, d := range r.Domain {
			if strings.EqualFold(q.Domain, d) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(r.DomainSuffix) > 0 {
		matchedAny = true
		ok := false
		ld := strings.ToLower(q.Domain)
		for _, s := range r.DomainSuffix {
			ls := strings.ToLower(s)
			if ld == ls || strings.HasSuffix(ld, "."+ls) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(r.DomainKeyword) > 0 {
		matchedAny = true
		ok := false
		ld := strings.ToLower(q.Domain)
		for _, kw := range r.DomainKeyword {
			if strings.Contains(ld, strings.ToLower(kw)) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(r.DomainRegex) > 0 {
		matchedAny = true
		ok := false
		for _, expr := range r.DomainRegex {
			if re, err := regexp.Compile(expr); err == nil && re.MatchString(q.Domain) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(r.IPCIDR) > 0 {
		matchedAny = true
		addr, err := netip.ParseAddr(q.IP)
		if err != nil {
			return false
		}
		ok := false
		for _, c := range r.IPCIDR {
			if p, err := netip.ParsePrefix(c); err == nil && p.Contains(addr) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(r.Port) > 0 {
		matchedAny = true
		ok := false
		for _, p := range r.Port {
			if q.Port == p {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(r.ProcessName) > 0 {
		matchedAny = true
		ok := false
		for _, p := range r.ProcessName {
			if strings.EqualFold(q.ProcessName, p) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(r.PackageName) > 0 {
		matchedAny = true
		ok := false
		for _, p := range r.PackageName {
			if strings.EqualFold(q.PackageName, p) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return matchedAny
}

// DownloadRuleSet fetches a .srs rule-set atomically to cacheDir using static-IP DoH bootstrap by default.
func DownloadRuleSet(ctx context.Context, client *http.Client, tag, url, cacheDir string) (string, error) {
	if client == nil {
		client = bootstrap.NewHTTPClient(bootstrap.Options{Timeout: 30 * time.Second})
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download rule-set %s HTTP %d", tag, resp.StatusCode)
	}
	dest := filepath.Join(cacheDir, tag+".srs")
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 50<<20)); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	return dest, nil
}
