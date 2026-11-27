//go:build linux

package dpi

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"syscall"
	"time"
)

// DialAndCaptureTCPSeq dials targetIP:port using dialTCP and, when CAP_NET_RAW is available
// for an IPv4 target, concurrently listens on an IPPROTO_TCP raw socket to extract the exact
// live TCP sequence and ack numbers from the server's SYN-ACK handshake packet.
func DialAndCaptureTCPSeq(
	ctx context.Context,
	targetIP string,
	port int,
	dialTCP func(ctx context.Context, addr string) (net.Conn, error),
) (conn net.Conn, seq uint32, ack uint32, hasSeq bool, err error) {
	addr := net.JoinHostPort(targetIP, strconv.Itoa(port))
	dstIP, parseErr := netip.ParseAddr(targetIP)
	if parseErr != nil || !dstIP.Is4() || !HasRawSocketCapability() {
		c, derr := dialTCP(ctx, addr)
		return c, 0, 0, false, derr
	}

	fd, serr := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_TCP)
	if serr != nil {
		c, derr := dialTCP(ctx, addr)
		return c, 0, 0, false, derr
	}
	defer syscall.Close(fd)

	// Cap read blocking so handshake sniffer can never stall Dial.
	tv := syscall.Timeval{Sec: 0, Usec: 150000} // 150ms
	_ = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)

	type pktSnap struct {
		data []byte
	}
	pktsCh := make(chan pktSnap, 32)
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		buf := make([]byte, 2048)
		for {
			select {
			case <-stopCh:
				return
			default:
			}
			n, _, rerr := syscall.Recvfrom(fd, buf, 0)
			if rerr != nil {
				if rerr == syscall.EAGAIN || rerr == syscall.EWOULDBLOCK || rerr == syscall.EINTR {
					continue
				}
				return
			}
			if n >= 40 {
				cp := make([]byte, n)
				copy(cp, buf[:n])
				select {
				case pktsCh <- pktSnap{data: cp}:
				default:
				}
			}
		}
	}()

	c, derr := dialTCP(ctx, addr)
	if derr != nil {
		close(stopCh)
		<-doneCh
		return nil, 0, 0, false, derr
	}

	// Give the sniffer goroutine a brief moment to enqueue the SYN-ACK that already arrived.
	time.Sleep(5 * time.Millisecond)
	close(stopCh)
	<-doneCh

	la, okL := c.LocalAddr().(*net.TCPAddr)
	if !okL || la == nil {
		return c, 0, 0, false, nil
	}
	localPort := uint16(la.Port)
	remotePort := uint16(port)

	for {
		select {
		case snap := <-pktsCh:
			if s, a, ok := ExtractSeqAckFromSYNACK(snap.data, dstIP, remotePort, localPort); ok {
				return c, s, a, true, nil
			}
		default:
			return c, 0, 0, false, nil
		}
	}
}
