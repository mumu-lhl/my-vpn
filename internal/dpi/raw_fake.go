package dpi

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"syscall"
)

var ErrRawCapUnavailable = errors.New("CAP_NET_RAW unavailable for raw fake packet injection")

// RawFakeOptions controls crafted IPv4+TCP fake packet construction.
type RawFakeOptions struct {
	TTL           uint8
	BadChecksum   bool
	BadSeqOffset  int32
	IPFragmentMTU int // when > 0 and < packet size, splits IPv4 datagram into fragments
	Payload       []byte
}

// BuildFakeTCPPackets constructs one or more raw IPv4+TCP packets carrying a decoy payload
// with configurable low TTL, intentionally corrupted TCP checksum (badsum), shifted sequence
// number (badseq), and optional IPv4 fragmentation.
func BuildFakeTCPPackets(
	srcIP, dstIP netip.Addr,
	srcPort, dstPort uint16,
	seq, ack uint32,
	opts RawFakeOptions,
) ([][]byte, error) {
	if !srcIP.Is4() || !dstIP.Is4() {
		return nil, errors.New("raw fake TCP builder requires IPv4 addresses")
	}
	ttl := opts.TTL
	if ttl == 0 {
		ttl = 64
	}
	payload := opts.Payload
	if len(payload) == 0 {
		payload = BuildFakeClientHello("www.microsoft.com")
	}

	s4 := srcIP.As4()
	d4 := dstIP.As4()
	tcpLen := 20 + len(payload)
	tcpSeg := make([]byte, tcpLen)

	binary.BigEndian.PutUint16(tcpSeg[0:2], srcPort)
	binary.BigEndian.PutUint16(tcpSeg[2:4], dstPort)
	adjSeq := uint32(int64(seq) + int64(opts.BadSeqOffset))
	binary.BigEndian.PutUint32(tcpSeg[4:8], adjSeq)
	binary.BigEndian.PutUint32(tcpSeg[8:12], ack)
	tcpSeg[12] = 5 << 4 // data offset = 20 bytes
	tcpSeg[13] = 0x18   // PSH | ACK
	binary.BigEndian.PutUint16(tcpSeg[14:16], 65535)
	copy(tcpSeg[20:], payload)

	csum := tcpChecksumIPv4(s4, d4, tcpSeg)
	if opts.BadChecksum {
		csum ^= 0xFFFF
		if csum == 0 {
			csum = 0x1234
		}
	}
	binary.BigEndian.PutUint16(tcpSeg[16:18], csum)

	// Optional IPv4 fragmentation (8-byte aligned payload fragments per RFC 791).
	if opts.IPFragmentMTU > 28 && opts.IPFragmentMTU < 20+len(tcpSeg) {
		maxFragPayload := ((opts.IPFragmentMTU - 20) / 8) * 8
		if maxFragPayload < 8 {
			maxFragPayload = 8
		}
		var frags [][]byte
		ipID := uint16(0x4C4F)
		for off := 0; off < len(tcpSeg); off += maxFragPayload {
			end := off + maxFragPayload
			moreFrags := true
			if end >= len(tcpSeg) {
				end = len(tcpSeg)
				moreFrags = false
			}
			fragData := tcpSeg[off:end]
			pkt := make([]byte, 20+len(fragData))
			pkt[0] = 0x45
			binary.BigEndian.PutUint16(pkt[2:4], uint16(len(pkt)))
			binary.BigEndian.PutUint16(pkt[4:6], ipID)
			fragField := uint16(off / 8)
			if moreFrags {
				fragField |= 0x2000 // MF flag
			}
			binary.BigEndian.PutUint16(pkt[6:8], fragField)
			pkt[8] = ttl
			pkt[9] = 6 // TCP
			copy(pkt[12:16], s4[:])
			copy(pkt[16:20], d4[:])
			binary.BigEndian.PutUint16(pkt[10:12], ipChecksum(pkt[:20]))
			copy(pkt[20:], fragData)
			frags = append(frags, pkt)
		}
		return frags, nil
	}

	pkt := make([]byte, 20+len(tcpSeg))
	pkt[0] = 0x45 // IPv4, IHL=5
	binary.BigEndian.PutUint16(pkt[2:4], uint16(len(pkt)))
	binary.BigEndian.PutUint16(pkt[4:6], 0x4C4F)
	binary.BigEndian.PutUint16(pkt[6:8], 0x4000) // DF
	pkt[8] = ttl
	pkt[9] = 6 // TCP
	copy(pkt[12:16], s4[:])
	copy(pkt[16:20], d4[:])
	binary.BigEndian.PutUint16(pkt[10:12], ipChecksum(pkt[:20]))
	copy(pkt[20:], tcpSeg)
	return [][]byte{pkt}, nil
}

// VerifyTCPChecksumIPv4 recomputes the RFC 793 checksum of a raw IPv4+TCP packet.
func VerifyTCPChecksumIPv4(pkt []byte) bool {
	if len(pkt) < 40 || pkt[0]>>4 != 4 || pkt[9] != 6 {
		return false
	}
	ihl := int(pkt[0]&0x0F) * 4
	if ihl < 20 || len(pkt) < ihl+20 {
		return false
	}
	var s4, d4 [4]byte
	copy(s4[:], pkt[12:16])
	copy(d4[:], pkt[16:20])
	seg := make([]byte, len(pkt)-ihl)
	copy(seg, pkt[ihl:])
	want := binary.BigEndian.Uint16(seg[16:18])
	seg[16], seg[17] = 0, 0
	return tcpChecksumIPv4(s4, d4, seg) == want
}

// InjectRawFake transmits crafted raw IPv4 fake TCP packets when CAP_NET_RAW is present.
func InjectRawFake(srcIP, dstIP netip.Addr, srcPort, dstPort uint16, seq, ack uint32, opts RawFakeOptions) error {
	if !HasRawSocketCapability() {
		return ErrRawCapUnavailable
	}
	pkts, err := BuildFakeTCPPackets(srcIP, dstIP, srcPort, dstPort, seq, ack, opts)
	if err != nil {
		return err
	}
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1); err != nil {
		return err
	}
	sa := &syscall.SockaddrInet4{Port: int(dstPort), Addr: dstIP.As4()}
	for _, pkt := range pkts {
		if err := syscall.Sendto(fd, pkt, 0, sa); err != nil {
			return err
		}
	}
	return nil
}

func ipChecksum(hdr []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(hdr); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(hdr[i : i+2]))
	}
	for sum > 0xFFFF {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return ^uint16(sum)
}

func tcpChecksumIPv4(src, dst [4]byte, tcpSeg []byte) uint16 {
	var sum uint32
	sum += uint32(binary.BigEndian.Uint16(src[0:2]))
	sum += uint32(binary.BigEndian.Uint16(src[2:4]))
	sum += uint32(binary.BigEndian.Uint16(dst[0:2]))
	sum += uint32(binary.BigEndian.Uint16(dst[2:4]))
	sum += 6 // IPPROTO_TCP
	sum += uint32(len(tcpSeg))

	for i := 0; i+1 < len(tcpSeg); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(tcpSeg[i : i+2]))
	}
	if len(tcpSeg)%2 == 1 {
		sum += uint32(tcpSeg[len(tcpSeg)-1]) << 8
	}
	for sum > 0xFFFF {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return ^uint16(sum)
}
