package haloce

import (
	"bytes"
	"log"
	"unicode/utf16"

	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// xboxNameHeader is a 28-byte prefix observed before every heap-allocated
// XString copy of the local xbox console name. It's made of kernel pointers
// that should be deterministic across xemu boots (XBE loads at a fixed base,
// kernel imports resolve the same way each time).
var xboxNameHeader = []byte{
	0xC4, 0x24, 0x0A, 0xD0, 0x80, 0xB1, 0x2F, 0x00,
	0x08, 0x00, 0x00, 0x00, 0x6F, 0xE1, 0x17, 0x00,
	0x78, 0x14, 0x20, 0x00, 0x6C, 0x24, 0x0A, 0xD0,
	0xDE, 0x24, 0x00, 0x00,
}

const (
	xboxNameScanStartGVA uint32 = 0x81000000
	xboxNameScanEndGVA   uint32 = 0x83000000
	xboxNameScanChunk    int    = 1 << 20
	xboxNameMaxChars     int    = 32
)

// ReadXboxName walks the XBE heap for the XString header pattern and returns
// the first UTF-16LE string that follows. Returns "" if nothing matched.
func ReadXboxName(mem *xemu.Mem) string {
	for gva := xboxNameScanStartGVA; gva < xboxNameScanEndGVA; gva += uint32(xboxNameScanChunk) {
		readSize := xboxNameScanChunk
		if gva+uint32(readSize) > xboxNameScanEndGVA {
			readSize = int(xboxNameScanEndGVA - gva)
		}
		data, err := mem.ReadBytes(gva, readSize)
		if err != nil {
			continue
		}
		if isUnmapped(data) {
			continue
		}

		for i := 0; i <= len(data)-len(xboxNameHeader); i++ {
			if data[i] != xboxNameHeader[0] {
				continue
			}
			if !bytes.Equal(data[i:i+len(xboxNameHeader)], xboxNameHeader) {
				continue
			}

			// Name starts 4 bytes after the header (flag u32, value varies).
			nameStart := i + len(xboxNameHeader) + 4
			if nameStart+2 > len(data) {
				continue
			}

			name := decodeUTF16LEUntilNull(data[nameStart:], xboxNameMaxChars)
			if name == "" {
				continue
			}
			return name
		}
	}

	log.Printf("haloce: xbox name scan found no matches in GVA 0x%08X–0x%08X",
		xboxNameScanStartGVA, xboxNameScanEndGVA)
	return ""
}

// isUnmapped returns true when the first 256 bytes of the buffer are all 0xFF,
// the signature xemu uses for unbacked guest pages.
func isUnmapped(data []byte) bool {
	n := 256
	if len(data) < n {
		n = len(data)
	}
	for i := 0; i < n; i++ {
		if data[i] != 0xFF {
			return false
		}
	}
	return true
}

// decodeUTF16LEUntilNull reads up to maxChars UTF-16LE code units from data,
// stopping at the first null terminator, and returns the decoded string.
func decodeUTF16LEUntilNull(data []byte, maxChars int) string {
	units := make([]uint16, 0, maxChars)
	for i := 0; i < maxChars; i++ {
		off := i * 2
		if off+2 > len(data) {
			break
		}
		u := uint16(data[off]) | uint16(data[off+1])<<8
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units))
}
