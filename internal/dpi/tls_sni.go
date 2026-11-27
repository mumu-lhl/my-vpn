package dpi

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"sort"
	"unicode"
)

// SNISpan locates the TLS server_name (0x0000) extension and hostname slice inside a ClientHello record.
type SNISpan struct {
	ExtStart  int
	ExtEnd    int
	HostStart int
	HostEnd   int
	Host      string
}

// IsTLSClientHello reports whether p begins with a TLS Handshake (0x16) ClientHello (0x01) record.
func IsTLSClientHello(p []byte) bool {
	if len(p) < 6 {
		return false
	}
	if p[0] != 0x16 || p[1] != 0x03 {
		return false
	}
	recLen := int(binary.BigEndian.Uint16(p[3:5]))
	if recLen <= 0 || len(p) < 5+recLen {
		return false
	}
	return p[5] == 0x01
}

// FindSNIExtension parses a TLS ClientHello record (RFC 5246 / RFC 8446) and returns
// the exact byte offsets of the SNI extension and hostname.
func FindSNIExtension(p []byte) (SNISpan, bool) {
	if !IsTLSClientHello(p) {
		return SNISpan{}, false
	}
	recEnd := 5 + int(binary.BigEndian.Uint16(p[3:5]))
	if recEnd > len(p) {
		recEnd = len(p)
	}
	// Handshake header: type(1) + length(3) + client_version(2) + random(32) = 38 bytes after record header.
	off := 5 + 1 + 3 + 2 + 32
	if off >= recEnd {
		return SNISpan{}, false
	}
	// Session ID
	sessLen := int(p[off])
	off += 1 + sessLen
	if off+2 > recEnd {
		return SNISpan{}, false
	}
	// Cipher Suites
	csLen := int(binary.BigEndian.Uint16(p[off : off+2]))
	off += 2 + csLen
	if off+1 > recEnd {
		return SNISpan{}, false
	}
	// Compression Methods
	compLen := int(p[off])
	off += 1 + compLen
	if off+2 > recEnd {
		return SNISpan{}, false
	}
	// Extensions total length
	extTotal := int(binary.BigEndian.Uint16(p[off : off+2]))
	off += 2
	extLimit := off + extTotal
	if extLimit > recEnd {
		extLimit = recEnd
	}

	for off+4 <= extLimit {
		extType := binary.BigEndian.Uint16(p[off : off+2])
		extLen := int(binary.BigEndian.Uint16(p[off+2 : off+4]))
		extStart := off
		extData := off + 4
		extEnd := extData + extLen
		if extEnd > extLimit {
			break
		}
		if extType == 0x0000 && extLen >= 5 {
			// server_name_list length (2) + name_type (1, 0=host_name) + host_name length (2)
			listLen := int(binary.BigEndian.Uint16(p[extData : extData+2]))
			pos := extData + 2
			listEnd := pos + listLen
			if listEnd > extEnd {
				listEnd = extEnd
			}
			for pos+3 <= listEnd {
				nameType := p[pos]
				nameLen := int(binary.BigEndian.Uint16(p[pos+1 : pos+3]))
				nameStart := pos + 3
				nameEnd := nameStart + nameLen
				if nameEnd > listEnd {
					break
				}
				if nameType == 0 && nameLen > 0 {
					return SNISpan{
						ExtStart:  extStart,
						ExtEnd:    extEnd,
						HostStart: nameStart,
						HostEnd:   nameEnd,
						Host:      string(p[nameStart:nameEnd]),
					}, true
				}
				pos = nameEnd
			}
		}
		off = extEnd
	}
	return SNISpan{}, false
}

// ComputeSNISplitPoints calculates deterministic+guaranteed split offsets inside p.
// When an SNI extension is found, at least one cut is guaranteed to land strictly inside
// [HostStart+1, HostEnd-1] so DPI cannot match the contiguous hostname in a single packet.
func ComputeSNISplitPoints(p []byte, splitCount int) []int {
	if len(p) <= 2 {
		return nil
	}
	if splitCount < 2 {
		splitCount = 2
	}
	var points []int
	if span, ok := FindSNIExtension(p); ok && span.HostEnd-span.HostStart >= 2 {
		mid := span.HostStart + (span.HostEnd-span.HostStart)/2
		if mid <= span.HostStart {
			mid = span.HostStart + 1
		}
		points = append(points, mid)
		if splitCount >= 3 && span.HostStart > 6 {
			// Cut shortly after TLS record header before SNI.
			points = append(points, span.HostStart-1)
		}
		if splitCount >= 4 && span.HostEnd-span.HostStart >= 4 {
			q1 := span.HostStart + (span.HostEnd-span.HostStart)/4
			q3 := span.HostStart + 3*(span.HostEnd-span.HostStart)/4
			if q1 > span.HostStart && q1 < mid {
				points = append(points, q1)
			}
			if q3 > mid && q3 < span.HostEnd {
				points = append(points, q3)
			}
		}
	} else {
		// Fallback even split when ClientHello has no plaintext SNI (e.g. ECH).
		step := len(p) / splitCount
		if step < 1 {
			step = 1
		}
		for i := 1; i < splitCount; i++ {
			off := i * step
			if off > 0 && off < len(p) {
				points = append(points, off)
			}
		}
	}

	sort.Ints(points)
	uniq := points[:0]
	for _, pt := range points {
		if pt > 0 && pt < len(p) && (len(uniq) == 0 || uniq[len(uniq)-1] != pt) {
			uniq = append(uniq, pt)
		}
	}
	return uniq
}

// SliceAtOffsets splits p at sorted byte offsets into contiguous non-empty slices.
func SliceAtOffsets(p []byte, offsets []int) [][]byte {
	if len(p) == 0 {
		return nil
	}
	var out [][]byte
	prev := 0
	for _, off := range offsets {
		if off <= prev || off >= len(p) {
			continue
		}
		cp := make([]byte, off-prev)
		copy(cp, p[prev:off])
		out = append(out, cp)
		prev = off
	}
	if prev < len(p) {
		cp := make([]byte, len(p)-prev)
		copy(cp, p[prev:])
		out = append(out, cp)
	}
	return out
}

// MutateClientHelloSNICase returns a copy of p with alternating uppercase/lowercase
// ASCII letters in the SNI hostname (RFC 4343 DNS names are case-insensitive).
func MutateClientHelloSNICase(p []byte) []byte {
	span, ok := FindSNIExtension(p)
	if !ok {
		return p
	}
	out := make([]byte, len(p))
	copy(out, p)
	upper := true
	for i := span.HostStart; i < span.HostEnd; i++ {
		r := rune(out[i])
		if unicode.IsLetter(r) {
			if upper {
				out[i] = byte(unicode.ToUpper(r))
			} else {
				out[i] = byte(unicode.ToLower(r))
			}
			upper = !upper
		}
	}
	return out
}

// SplitTLSRecordsAtOffsets splits a single TLS Handshake record into multiple valid
// TLS Handshake records whose boundaries align with the given record-relative offsets.
func SplitTLSRecordsAtOffsets(p []byte, offsets []int) [][]byte {
	if !IsTLSClientHello(p) {
		return [][]byte{p}
	}
	recLen := int(binary.BigEndian.Uint16(p[3:5]))
	verMajor, verMinor := p[1], p[2]
	payload := p[5 : 5+recLen]
	trailing := p[5+recLen:]

	// Convert packet offsets into payload-relative offsets (subtract 5-byte TLS record header).
	var rel []int
	for _, off := range offsets {
		if off > 5 && off < 5+recLen {
			rel = append(rel, off-5)
		}
	}
	if len(rel) == 0 && len(payload) > 2 {
		rel = []int{len(payload) / 2}
	}

	pieces := SliceAtOffsets(payload, rel)
	out := make([][]byte, 0, len(pieces)+1)
	for _, piece := range pieces {
		rec := make([]byte, 5+len(piece))
		rec[0] = 0x16
		rec[1] = verMajor
		rec[2] = verMinor
		binary.BigEndian.PutUint16(rec[3:5], uint16(len(piece)))
		copy(rec[5:], piece)
		out = append(out, rec)
	}
	if len(trailing) > 0 {
		out = append(out, trailing)
	}
	return out
}

// BuildFakeClientHello constructs a syntactically valid TLS 1.2 ClientHello carrying
// a benign decoy SNI (used by low-TTL fake and Raw Socket badsum/badseq injectors).
func BuildFakeClientHello(fakeSNI string) []byte {
	if fakeSNI == "" {
		fakeSNI = "www.microsoft.com"
	}
	sniBytes := []byte(fakeSNI)
	// SNI extension body: list_len(2) + type(1)=0 + name_len(2) + name
	sniExtDataLen := 2 + 1 + 2 + len(sniBytes)
	extBlockLen := 4 + sniExtDataLen

	// ClientHello body: version(2) + random(32) + sess_len(1) + ciphers(4) + comp(2) + ext_len(2) + ext
	var body bytes.Buffer
	body.Write([]byte{0x03, 0x03}) // TLS 1.2
	rnd := make([]byte, 32)
	_, _ = rand.Read(rnd)
	body.Write(rnd)
	body.WriteByte(0x00)                       // session id len = 0
	body.Write([]byte{0x00, 0x02, 0x13, 0x01}) // cipher suite TLS_AES_128_GCM_SHA256
	body.Write([]byte{0x01, 0x00})             // compression methods: null
	_ = binary.Write(&body, binary.BigEndian, uint16(extBlockLen))
	_ = binary.Write(&body, binary.BigEndian, uint16(0x0000)) // server_name extension
	_ = binary.Write(&body, binary.BigEndian, uint16(sniExtDataLen))
	_ = binary.Write(&body, binary.BigEndian, uint16(1+2+len(sniBytes)))
	body.WriteByte(0x00) // host_name
	_ = binary.Write(&body, binary.BigEndian, uint16(len(sniBytes)))
	body.Write(sniBytes)

	hsLen := body.Len()
	recLen := 4 + hsLen
	out := make([]byte, 5+recLen)
	out[0] = 0x16
	out[1] = 0x03
	out[2] = 0x01
	binary.BigEndian.PutUint16(out[3:5], uint16(recLen))
	out[5] = 0x01 // ClientHello
	out[6] = byte(hsLen >> 16)
	out[7] = byte(hsLen >> 8)
	out[8] = byte(hsLen)
	copy(out[9:], body.Bytes())
	return out
}

// IsPlainHTTPRequest reports whether p starts with a standard HTTP/1.x request method.
func IsPlainHTTPRequest(p []byte) bool {
	for _, method := range [][]byte{
		[]byte("GET "), []byte("POST "), []byte("HEAD "),
		[]byte("PUT "), []byte("DELETE "), []byte("OPTIONS "), []byte("PATCH "),
	} {
		if bytes.HasPrefix(p, method) {
			return true
		}
	}
	return false
}

// ObfuscateHTTPRequest applies RFC 9110-compliant Host header obfuscation (mixed-case
// "HoSt:" and optional extra whitespace after the colon) to evade HTTP DPI signatures.
func ObfuscateHTTPRequest(p []byte, mixHostCase, headerSpace bool) []byte {
	hdrEnd := bytes.Index(p, []byte("\r\n\r\n"))
	if hdrEnd < 0 {
		hdrEnd = len(p)
	}
	out := make([]byte, len(p))
	copy(out, p)

	lower := bytes.ToLower(out[:hdrEnd])
	idx := bytes.Index(lower, []byte("\r\nhost:"))
	if idx < 0 {
		return out
	}
	if mixHostCase {
		copy(out[idx+2:idx+7], []byte("HoSt:"))
	}
	if headerSpace {
		// Insert an additional OWS tab after "HoSt:" (valid per RFC 9110 §5.5).
		pos := idx + 7
		expanded := make([]byte, 0, len(out)+1)
		expanded = append(expanded, out[:pos]...)
		expanded = append(expanded, '\t')
		expanded = append(expanded, out[pos:]...)
		out = expanded
	}
	return out
}
