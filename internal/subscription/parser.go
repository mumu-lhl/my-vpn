package subscription

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mumu-lhl/looma/internal/bootstrap"
	"github.com/mumu-lhl/looma/internal/config"
	"gopkg.in/yaml.v3"
)

// ParseAuto detects and parses sing-box JSON, Clash/Mihomo YAML, Base64 URI bundles,
// or raw newline-separated share URIs (vless, vmess, trojan, ss, hy2, tuic, wg).
func ParseAuto(raw []byte, subID string) ([]config.ExternalNode, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, errors.New("empty subscription payload")
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		if nodes, err := parseSingBoxJSON([]byte(trimmed), subID); err == nil && len(nodes) > 0 {
			return nodes, nil
		}
	}
	if strings.Contains(trimmed, "proxies:") {
		if nodes, err := parseClashYAML([]byte(trimmed), subID); err == nil && len(nodes) > 0 {
			return nodes, nil
		}
	}
	if looksLikeURIList(trimmed) {
		return parseURIList(trimmed, subID)
	}
	if decoded, err := decodeBase64Flexible(trimmed); err == nil {
		if looksLikeURIList(decoded) {
			return parseURIList(decoded, subID)
		}
	}
	return nil, errors.New("unrecognized subscription format")
}

func looksLikeURIList(s string) bool {
	for _, prefix := range []string{"vless://", "vmess://", "trojan://", "ss://", "hy2://", "hysteria2://", "tuic://", "wg://", "wireguard://"} {
		if strings.Contains(s, prefix) {
			return true
		}
	}
	return false
}

func decodeBase64Flexible(s string) (string, error) {
	clean := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), "\n", ""), "\r", "")
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(clean); err == nil {
			return string(b), nil
		}
	}
	return "", errors.New("invalid base64")
}

func parseSingBoxJSON(raw []byte, subID string) ([]config.ExternalNode, error) {
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var out []config.ExternalNode
	for _, ob := range doc.Outbounds {
		typ, _ := ob["type"].(string)
		switch typ {
		case "vless", "vmess", "trojan", "shadowsocks", "hysteria2", "tuic", "wireguard", "socks", "http":
		default:
			continue
		}
		tag, _ := ob["tag"].(string)
		srv, _ := ob["server"].(string)
		port := intFromAny(ob["server_port"])
		if tag == "" {
			tag = fmt.Sprintf("%s-%s:%d", typ, srv, port)
		}
		out = append(out, config.ExternalNode{
			ID:             stableNodeID(subID, tag, srv, port),
			SubscriptionID: subID,
			Name:           tag,
			Protocol:       typ,
			Server:         srv,
			Port:           port,
			SupportsUDP:    typ != "http",
			RawOutbound:    ob,
		})
	}
	return out, nil
}

func parseClashYAML(raw []byte, subID string) ([]config.ExternalNode, error) {
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var out []config.ExternalNode
	for _, p := range doc.Proxies {
		typ, _ := p["type"].(string)
		name, _ := p["name"].(string)
		srv, _ := p["server"].(string)
		port := intFromAny(p["port"])
		proto := normalizeProto(typ)
		if proto == "" || srv == "" || port == 0 {
			continue
		}
		udp := true
		if v, ok := p["udp"].(bool); ok {
			udp = v
		}
		ob := clashToSingBoxOutbound(proto, name, srv, port, p)
		out = append(out, config.ExternalNode{
			ID:             stableNodeID(subID, name, srv, port),
			SubscriptionID: subID,
			Name:           name,
			Protocol:       proto,
			Server:         srv,
			Port:           port,
			SupportsUDP:    udp,
			RawOutbound:    ob,
		})
	}
	return out, nil
}

func clashToSingBoxOutbound(proto, name, srv string, port int, p map[string]any) map[string]any {
	ob := map[string]any{
		"type":        proto,
		"tag":         name,
		"server":      srv,
		"server_port": port,
	}
	switch proto {
	case "vless", "vmess":
		if u, ok := p["uuid"].(string); ok {
			ob["uuid"] = u
		}
		if f, ok := p["flow"].(string); ok && f != "" {
			ob["flow"] = f
		}
	case "trojan", "hysteria2":
		if pw, ok := p["password"].(string); ok {
			ob["password"] = pw
		}
	case "shadowsocks":
		if m, ok := p["cipher"].(string); ok {
			ob["method"] = m
		}
		if pw, ok := p["password"].(string); ok {
			ob["password"] = pw
		}
	case "tuic":
		if u, ok := p["uuid"].(string); ok {
			ob["uuid"] = u
		}
		if pw, ok := p["password"].(string); ok {
			ob["password"] = pw
		}
	case "wireguard":
		if pk, ok := p["private-key"].(string); ok {
			ob["private_key"] = pk
		}
		if pub, ok := p["public-key"].(string); ok {
			ob["peer_public_key"] = pub
		}
	}
	if sni, ok := p["servername"].(string); ok && sni != "" {
		ob["tls"] = map[string]any{"enabled": true, "server_name": sni}
	} else if sni, ok := p["sni"].(string); ok && sni != "" {
		ob["tls"] = map[string]any{"enabled": true, "server_name": sni}
	}
	return ob
}

func parseURIList(text, subID string) ([]config.ExternalNode, error) {
	lines := strings.Split(text, "\n")
	var out []config.ExternalNode
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		node, err := ParseShareURI(line, subID)
		if err == nil {
			out = append(out, node)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no valid share URIs found")
	}
	return out, nil
}

func ParseShareURI(uri, subID string) (config.ExternalNode, error) {
	if strings.HasPrefix(uri, "vmess://") {
		return parseVMessURI(uri, subID)
	}
	u, err := url.Parse(uri)
	if err != nil {
		return config.ExternalNode{}, err
	}
	proto := normalizeProto(u.Scheme)
	if proto == "" {
		return config.ExternalNode{}, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	name, _ := url.QueryUnescape(u.Fragment)
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())

	ob := map[string]any{
		"type":        proto,
		"server":      host,
		"server_port": port,
	}

	switch proto {
	case "vless":
		ob["uuid"] = u.User.Username()
		q := u.Query()
		if flow := q.Get("flow"); flow != "" {
			ob["flow"] = flow
		}
		if sni := q.Get("sni"); sni != "" {
			tls := map[string]any{"enabled": true, "server_name": sni}
			if pbk := q.Get("pbk"); pbk != "" {
				tls["reality"] = map[string]any{"enabled": true, "public_key": pbk, "short_id": q.Get("sid")}
			}
			ob["tls"] = tls
		}
	case "trojan", "hysteria2":
		ob["password"] = u.User.Username()
		if sni := u.Query().Get("sni"); sni != "" {
			ob["tls"] = map[string]any{"enabled": true, "server_name": sni}
		}
	case "shadowsocks":
		method, pass := "", ""
		if u.User != nil {
			if p, ok := u.User.Password(); ok {
				method, pass = u.User.Username(), p
			} else if dec, err := decodeBase64Flexible(u.User.Username()); err == nil {
				parts := strings.SplitN(dec, ":", 2)
				if len(parts) == 2 {
					method, pass = parts[0], parts[1]
				}
			}
		}
		ob["method"] = method
		ob["password"] = pass
	case "tuic":
		ob["uuid"] = u.User.Username()
		if p, ok := u.User.Password(); ok {
			ob["password"] = p
		}
		if sni := u.Query().Get("sni"); sni != "" {
			ob["tls"] = map[string]any{"enabled": true, "server_name": sni}
		}
	case "wireguard":
		ob["private_key"] = u.User.Username()
		q := u.Query()
		if pub := q.Get("publickey"); pub != "" {
			ob["peer_public_key"] = pub
		}
		if addr := q.Get("address"); addr != "" {
			ob["local_address"] = strings.Split(addr, ",")
		}
	}

	if name == "" {
		name = fmt.Sprintf("%s-%s:%d", proto, host, port)
	}
	ob["tag"] = name
	return config.ExternalNode{
		ID:             stableNodeID(subID, name, host, port),
		SubscriptionID: subID,
		Name:           name,
		Protocol:       proto,
		Server:         host,
		Port:           port,
		SupportsUDP:    true,
		RawOutbound:    ob,
	}, nil
}

func parseVMessURI(uri, subID string) (config.ExternalNode, error) {
	dec, err := decodeBase64Flexible(strings.TrimPrefix(uri, "vmess://"))
	if err != nil {
		return config.ExternalNode{}, err
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(dec), &m); err != nil {
		return config.ExternalNode{}, err
	}
	srv, _ := m["add"].(string)
	port := intFromAny(m["port"])
	name, _ := m["ps"].(string)
	if name == "" {
		name = fmt.Sprintf("vmess-%s:%d", srv, port)
	}
	ob := map[string]any{
		"type":        "vmess",
		"tag":         name,
		"server":      srv,
		"server_port": port,
		"uuid":        m["id"],
		"alter_id":    intFromAny(m["aid"]),
	}
	if sni, _ := m["sni"].(string); sni != "" {
		ob["tls"] = map[string]any{"enabled": true, "server_name": sni}
	}
	return config.ExternalNode{
		ID:             stableNodeID(subID, name, srv, port),
		SubscriptionID: subID,
		Name:           name,
		Protocol:       "vmess",
		Server:         srv,
		Port:           port,
		SupportsUDP:    true,
		RawOutbound:    ob,
	}, nil
}

func normalizeProto(s string) string {
	switch strings.ToLower(s) {
	case "vless":
		return "vless"
	case "vmess":
		return "vmess"
	case "trojan":
		return "trojan"
	case "ss", "shadowsocks":
		return "shadowsocks"
	case "hy2", "hysteria2":
		return "hysteria2"
	case "tuic":
		return "tuic"
	case "wg", "wireguard":
		return "wireguard"
	default:
		return ""
	}
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(t)
		return n
	default:
		return 0
	}
}

func stableNodeID(subID, name, srv string, port int) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s|%s|%s|%d", subID, name, srv, port)))
	return "node-" + hex.EncodeToString(h[:6])
}

type Fetcher struct {
	Client *http.Client
}

func (f *Fetcher) Fetch(ctx context.Context, sub config.Subscription) ([]config.ExternalNode, error) {
	cli := f.Client
	if cli == nil {
		cli = bootstrap.NewHTTPClient(bootstrap.Options{Timeout: 20 * time.Second})
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sub.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sing-box/1.15 looma/0.1")
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	return ParseAuto(body, sub.ID)
}
