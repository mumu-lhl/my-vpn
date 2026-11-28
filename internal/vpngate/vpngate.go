package vpngate

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const DefaultAPIURL = "https://www.vpngate.net/api/iphone/"

type Node struct {
	ID             string `json:"id"`
	HostName       string `json:"host_name"`
	IP             string `json:"ip"`
	Score          int64  `json:"score"`
	Ping           int    `json:"ping"`
	Speed          int64  `json:"speed"`
	CountryLong    string `json:"country_long"`
	CountryShort   string `json:"country_short"`
	NumVpnSessions int    `json:"num_vpn_sessions"`
	Uptime         int64  `json:"uptime"`
	TotalUsers     int64  `json:"total_users"`
	TotalTraffic   int64  `json:"total_traffic"`
	Operator       string `json:"operator,omitempty"`
	Server         string `json:"server"`
	ServerPort     int    `json:"server_port"`
	Network        string `json:"network"`
	SupportsUDP    bool   `json:"supports_udp"`
	CACert         string `json:"ca_cert,omitempty"`
	ClientCert     string `json:"client_cert,omitempty"`
	ClientKey      string `json:"client_key,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
}

type ovpnParsed struct {
	Server     string
	ServerPort int
	Network    string
	CACert     string
	ClientCert string
	ClientKey  string
	HasAuth    bool
}

// ParseCSV parses the VPNGate iPhone API CSV response into structured OpenVPN nodes.
func ParseCSV(r io.Reader) ([]Node, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var headerLine string
	var rows []string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "*") {
			continue
		}
		if headerLine == "" && strings.HasPrefix(line, "#") {
			headerLine = strings.TrimPrefix(line, "#")
			continue
		}
		rows = append(rows, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if headerLine == "" {
		return nil, errors.New("vpngate CSV header not found")
	}

	csvReader := csv.NewReader(strings.NewReader(headerLine + "\n" + strings.Join(rows, "\n")))
	csvReader.FieldsPerRecord = -1
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse vpngate CSV: %w", err)
	}
	if len(records) < 2 {
		return []Node{}, nil
	}

	colIdx := make(map[string]int, len(records[0]))
	for i, name := range records[0] {
		colIdx[strings.TrimSpace(name)] = i
	}

	getField := func(rec []string, key string) string {
		idx, ok := colIdx[key]
		if !ok || idx < 0 || idx >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[idx])
	}

	out := make([]Node, 0, len(records)-1)
	for _, rec := range records[1:] {
		b64 := getField(rec, "OpenVPN_ConfigData_Base64")
		if b64 == "" {
			continue
		}
		rawOvpn, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			continue
		}
		ovpn := parseOVPNConfig(string(rawOvpn))
		ip := getField(rec, "IP")
		if ovpn.Server == "" {
			ovpn.Server = ip
		}
		if ovpn.Server == "" || ovpn.ServerPort <= 0 {
			continue
		}
		hostName := getField(rec, "HostName")
		countryShort := strings.ToUpper(getField(rec, "CountryShort"))
		countryLong := getField(rec, "CountryLong")

		score, _ := strconv.ParseInt(getField(rec, "Score"), 10, 64)
		ping, _ := strconv.Atoi(getField(rec, "Ping"))
		speed, _ := strconv.ParseInt(getField(rec, "Speed"), 10, 64)
		sessions, _ := strconv.Atoi(getField(rec, "NumVpnSessions"))
		uptime, _ := strconv.ParseInt(getField(rec, "Uptime"), 10, 64)
		totalUsers, _ := strconv.ParseInt(getField(rec, "TotalUsers"), 10, 64)
		totalTraffic, _ := strconv.ParseInt(getField(rec, "TotalTraffic"), 10, 64)

		netType := ovpn.Network
		if netType == "" {
			netType = "tcp"
		}
		nodeID := fmt.Sprintf("%s-%s-%s-%d", hostName, ip, netType, ovpn.ServerPort)
		if hostName == "" {
			nodeID = fmt.Sprintf("%s-%s-%d", ip, netType, ovpn.ServerPort)
		}

		var username, password string
		if ovpn.HasAuth {
			username = "vpn"
			password = "vpn"
		}

		out = append(out, Node{
			ID:             nodeID,
			HostName:       hostName,
			IP:             ip,
			Score:          score,
			Ping:           ping,
			Speed:          speed,
			CountryLong:    countryLong,
			CountryShort:   countryShort,
			NumVpnSessions: sessions,
			Uptime:         uptime,
			TotalUsers:     totalUsers,
			TotalTraffic:   totalTraffic,
			Operator:       getField(rec, "Operator"),
			Server:         ovpn.Server,
			ServerPort:     ovpn.ServerPort,
			Network:        netType,
			SupportsUDP:    netType == "udp",
			CACert:         ovpn.CACert,
			ClientCert:     ovpn.ClientCert,
			ClientKey:      ovpn.ClientKey,
			Username:       username,
			Password:       password,
		})
	}
	return out, nil
}

func parseOVPNConfig(text string) ovpnParsed {
	var res ovpnParsed
	res.CACert = extractXMLBlock(text, "ca")
	res.ClientCert = extractXMLBlock(text, "cert")
	res.ClientKey = extractXMLBlock(text, "key")

	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "<") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "proto":
			if len(fields) >= 2 {
				p := strings.ToLower(fields[1])
				if strings.HasPrefix(p, "udp") {
					res.Network = "udp"
				} else if strings.HasPrefix(p, "tcp") {
					res.Network = "tcp"
				}
			}
		case "remote":
			if len(fields) >= 2 && res.Server == "" {
				res.Server = fields[1]
				if len(fields) >= 3 {
					if port, err := strconv.Atoi(fields[2]); err == nil && port > 0 {
						res.ServerPort = port
					}
				}
			}
		case "port":
			if len(fields) >= 2 && res.ServerPort <= 0 {
				if port, err := strconv.Atoi(fields[1]); err == nil && port > 0 {
					res.ServerPort = port
				}
			}
		case "auth-user-pass":
			res.HasAuth = true
		}
	}
	return res
}

func extractXMLBlock(text, tag string) string {
	startTag := "<" + tag + ">"
	endTag := "</" + tag + ">"
	startIdx := strings.Index(text, startTag)
	if startIdx < 0 {
		return ""
	}
	rest := text[startIdx+len(startTag):]
	endIdx := strings.Index(rest, endTag)
	if endIdx < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:endIdx])
}

// FetchFromAPI fetches and parses VPNGate nodes from the given API URL, optionally
// routing the request through proxyURL (e.g. "http://127.0.0.1:10809").
func FetchFromAPI(ctx context.Context, apiURL, proxyURL string) ([]Node, error) {
	if strings.TrimSpace(apiURL) == "" {
		apiURL = DefaultAPIURL
	}
	transport := &http.Transport{}
	if strings.TrimSpace(proxyURL) != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyURL, err)
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	client := &http.Client{
		Timeout:   25 * time.Second,
		Transport: transport,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "looma/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vpngate API returned HTTP %d", resp.StatusCode)
	}
	return ParseCSV(resp.Body)
}

func LoadCache(path string) ([]Node, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Node{}, nil
		}
		return nil, err
	}
	var nodes []Node
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, err
	}
	if nodes == nil {
		return []Node{}, nil
	}
	return nodes, nil
}

func SaveCache(path string, nodes []Node) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
