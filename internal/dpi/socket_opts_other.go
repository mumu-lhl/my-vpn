//go:build !linux

package dpi

import (
	"errors"
	"net"
)

func HasRawSocketCapability() bool { return false }

func GetSocketTTL(_ net.Conn) (int, error) { return 64, nil }

func SetSocketTTL(_ net.Conn, _ int) error {
	return errors.New("SetSocketTTL unsupported on this platform")
}

func SendOOBByte(_ net.Conn, _ byte) error {
	return errors.New("MSG_OOB unsupported on this platform")
}
