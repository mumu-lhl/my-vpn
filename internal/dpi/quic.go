package dpi

import (
	"crypto/rand"
	"encoding/binary"
	"net"
)

// IsQUICInitial reports whether pkt is an RFC 9000 QUIC Long Header Initial packet.
func IsQUICInitial(pkt []byte) bool {
	if len(pkt) < 7 {
		return false
	}
	// Long header form bit (0x80) + Fixed bit (0x40), packet type bits (0x30) == 0x00 (Initial).
	if (pkt[0] & 0xF0) != 0xC0 {
		return false
	}
	version := binary.BigEndian.Uint32(pkt[1:5])
	return version != 0
}

// BuildFakeQUICInitial generates a decoy QUIC v1 Initial packet padded to at least 1200 bytes
// (RFC 9000 minimum Initial datagram size) to poison DPI QUIC classifiers.
func BuildFakeQUICInitial(size int) []byte {
	if size < 1200 {
		size = 1200
	}
	pkt := make([]byte, size)
	pkt[0] = 0xC0 | 0x03 // Long header, Initial, 4-byte packet number
	// QUIC v1 version = 0x00000001
	binary.BigEndian.PutUint32(pkt[1:5], 0x00000001)
	pkt[5] = 8 // DCID length
	_, _ = rand.Read(pkt[6:14])
	pkt[14] = 8 // SCID length
	_, _ = rand.Read(pkt[15:23])
	pkt[23] = 0 // Token length = 0
	// Remaining payload filled with random decoy bytes.
	_, _ = rand.Read(pkt[24:])
	return pkt
}

// DesyncQUICPacket applies the strategy's QUIC handling to outbound UDP datagrams:
//   - "block": drops QUIC Initial packets on UDP 443 so browsers fall back to TCP/TLS.
//   - "fake": sends a decoy QUIC Initial packet immediately before the real Initial packet.
//   - "pass": forwards unmodified.
func DesyncQUICPacket(conn net.PacketConn, dst net.Addr, pkt []byte, mode string) (dropped bool, err error) {
	if !IsQUICInitial(pkt) {
		_, err = conn.WriteTo(pkt, dst)
		return false, err
	}
	switch mode {
	case "block":
		return true, nil
	case "fake":
		fake := BuildFakeQUICInitial(len(pkt))
		_, _ = conn.WriteTo(fake, dst)
		_, err = conn.WriteTo(pkt, dst)
		return false, err
	default:
		_, err = conn.WriteTo(pkt, dst)
		return false, err
	}
}
