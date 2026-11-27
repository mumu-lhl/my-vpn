//go:build linux

package dpi

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// HasRawSocketCapability reports whether the current process holds CAP_NET_RAW (bit 13)
// in its effective capability set (/proc/self/status CapEff).
func HasRawSocketCapability() bool {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "CapEff:") {
			hexStr := strings.TrimSpace(strings.TrimPrefix(line, "CapEff:"))
			mask, err := strconv.ParseUint(hexStr, 16, 64)
			if err != nil {
				return false
			}
			const capNetRawBit = 13
			return (mask & (1 << capNetRawBit)) != 0
		}
	}
	return false
}

// GetSocketTTL reads IP_TTL (or IPV6_UNICAST_HOPS) from a TCP/UDP connection.
func GetSocketTTL(c net.Conn) (int, error) {
	sc, ok := c.(syscall.Conn)
	if !ok {
		return 64, errors.New("connection does not expose syscall.RawConn")
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return 64, err
	}
	isV6 := isIPv6Conn(c)
	var ttl int
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		if isV6 {
			ttl, sockErr = syscall.GetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_UNICAST_HOPS)
		} else {
			ttl, sockErr = syscall.GetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL)
		}
	}); err != nil {
		return 64, err
	}
	if sockErr != nil {
		return 64, sockErr
	}
	if ttl <= 0 {
		return 64, nil
	}
	return ttl, nil
}

// SetSocketTTL updates IP_TTL (or IPV6_UNICAST_HOPS) on a TCP/UDP connection.
func SetSocketTTL(c net.Conn, ttl int) error {
	if ttl <= 0 || ttl > 255 {
		return errors.New("invalid TTL")
	}
	sc, ok := c.(syscall.Conn)
	if !ok {
		return errors.New("connection does not expose syscall.RawConn")
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return err
	}
	isV6 := isIPv6Conn(c)
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		if isV6 {
			sockErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_UNICAST_HOPS, ttl)
		} else {
			sockErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
		}
	}); err != nil {
		return err
	}
	return sockErr
}

// SendOOBByte sends a single TCP urgent/out-of-band byte (MSG_OOB) to disrupt DPI stream offsets.
func SendOOBByte(c net.Conn, b byte) error {
	sc, ok := c.(syscall.Conn)
	if !ok {
		return errors.New("connection does not expose syscall.RawConn")
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return err
	}
	var sendErr error
	if err := raw.Write(func(fd uintptr) bool {
		sendErr = syscall.Sendto(int(fd), []byte{b}, syscall.MSG_OOB, nil)
		return !errors.Is(sendErr, syscall.EAGAIN)
	}); err != nil {
		return err
	}
	return sendErr
}

func isIPv6Conn(c net.Conn) bool {
	if ra, ok := c.RemoteAddr().(*net.TCPAddr); ok && ra.IP != nil {
		return ra.IP.To4() == nil
	}
	if ra, ok := c.RemoteAddr().(*net.UDPAddr); ok && ra.IP != nil {
		return ra.IP.To4() == nil
	}
	return false
}
