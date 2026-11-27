//go:build !linux

package dpi

import (
	"context"
	"net"
	"strconv"
)

// DialAndCaptureTCPSeq falls back to a standard TCP dial on non-Linux platforms.
func DialAndCaptureTCPSeq(
	ctx context.Context,
	targetIP string,
	port int,
	dialTCP func(ctx context.Context, addr string) (net.Conn, error),
) (conn net.Conn, seq uint32, ack uint32, hasSeq bool, err error) {
	c, derr := dialTCP(ctx, net.JoinHostPort(targetIP, strconv.Itoa(port)))
	return c, 0, 0, false, derr
}
