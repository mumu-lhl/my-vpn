package dpi

import (
	"encoding/binary"
	"net/netip"
)

// ExtractSeqAckFromSYNACK parses a raw IPv4+TCP packet and, if it is a SYN-ACK from
// (expectedSrcIP:expectedSrcPort) addressed to local port expectedDstPort, returns
// the client's next outbound TCP sequence number (synAck.AckNum) and ACK number
// (synAck.SeqNum + 1) needed for in-window raw fake packet injection.
func ExtractSeqAckFromSYNACK(
	ipPacket []byte,
	expectedSrcIP netip.Addr,
	expectedSrcPort uint16,
	expectedDstPort uint16,
) (clientNextSeq uint32, clientNextAck uint32, ok bool) {
	if !expectedSrcIP.Is4() || len(ipPacket) < 40 {
		return 0, 0, false
	}
	if ipPacket[0]>>4 != 4 || ipPacket[9] != 6 { // IPv4 + TCP
		return 0, 0, false
	}
	ihl := int(ipPacket[0]&0x0F) * 4
	if ihl < 20 || len(ipPacket) < ihl+20 {
		return 0, 0, false
	}
	src4 := expectedSrcIP.As4()
	if ipPacket[12] != src4[0] || ipPacket[13] != src4[1] || ipPacket[14] != src4[2] || ipPacket[15] != src4[3] {
		return 0, 0, false
	}
	tcp := ipPacket[ihl:]
	srcPort := binary.BigEndian.Uint16(tcp[0:2])
	dstPort := binary.BigEndian.Uint16(tcp[2:4])
	if srcPort != expectedSrcPort || dstPort != expectedDstPort {
		return 0, 0, false
	}
	flags := tcp[13]
	const synAckMask = 0x12 // SYN (0x02) | ACK (0x10)
	if (flags & synAckMask) != synAckMask {
		return 0, 0, false
	}
	serverSeq := binary.BigEndian.Uint32(tcp[4:8])
	serverAck := binary.BigEndian.Uint32(tcp[8:12])
	return serverAck, serverSeq + 1, true
}
