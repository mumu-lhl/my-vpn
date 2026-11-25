package platform

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strings"
)

const RequiredHelperCaps = "cap_net_admin,cap_net_raw+ep"

// HasTUNCapabilities checks if core-helper already holds cap_net_admin and cap_net_raw
// via getcap, enabling passwordless TUN toggling (Route A, AC-10).
func HasTUNCapabilities(helperPath string) (bool, string, error) {
	out, err := exec.Command("getcap", helperPath).CombinedOutput()
	if err != nil {
		return false, string(out), err
	}
	return ParseGetcapOutput(string(out)), strings.TrimSpace(string(out)), nil
}

func ParseGetcapOutput(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "cap_net_admin") && strings.Contains(lower, "cap_net_raw")
}

// GrantTUNCapabilitiesOnce runs pkexec setcap once on the non-GUI core-helper binary.
func GrantTUNCapabilitiesOnce(helperPath string) error {
	cmd := exec.Command("pkexec", "setcap", RequiredHelperCaps, helperPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pkexec setcap failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// SetGNOMESystemProxy configures gsettings proxy keys (used by GTK/GNOME apps on niri/Wayland).
func SetGNOMESystemProxy(enabled bool, httpPort, socksPort int) error {
	if _, err := exec.LookPath("gsettings"); err != nil {
		return nil // non-GNOME Wayland session without gsettings
	}
	if !enabled {
		return exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "none").Run()
	}
	cmds := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "manual"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "host", "127.0.0.1"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "port", fmt.Sprintf("%d", httpPort)},
		{"gsettings", "set", "org.gnome.system.proxy.https", "host", "127.0.0.1"},
		{"gsettings", "set", "org.gnome.system.proxy.https", "port", fmt.Sprintf("%d", httpPort)},
		{"gsettings", "set", "org.gnome.system.proxy.socks", "host", "127.0.0.1"},
		{"gsettings", "set", "org.gnome.system.proxy.socks", "port", fmt.Sprintf("%d", socksPort)},
	}
	for _, args := range cmds {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			return err
		}
	}
	return nil
}

// LocalLANIP returns the primary non-loopback IPv4 address for LAN proxy sharing display.
func LocalLANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if v4 := ipnet.IP.To4(); v4 != nil {
				return v4.String()
			}
		}
	}
	return "127.0.0.1"
}
