package xbox

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

const (
	consoleNameScanStartGVA uint32 = 0x80000000
	consoleNameScanEndGVA   uint32 = 0x90000000
	consoleNameScanChunk    int    = 1 << 20 // 1 MiB
	consoleNameMaxChars     int    = 16      // 16-wchar Xbox console-name cap
)

// ReadConsoleName scans guest memory for the kernel/dashboard copy of the
// console name loaded from E:\UDATA\NICKNAME.XBN. The anchor is an 8-byte
// pattern: four zero bytes followed by ConsoleNameRecordMagic in little-endian.
// Every NICKNAME.XBN record in RAM (file-load buffer or clean kernel cache)
// has this exact prefix; static occurrences of the magic in kernel/dashboard
// image code do not, since their preceding bytes are not four zeros.
//
// Returns the FIRST decoded record in the scan window — that's slot 0
// (console name); slots 1..N (per-controller nicknames) follow in heap and
// are skipped naturally because the scan walks low-to-high GVA.
//
// Title-agnostic: works whether UnleashX, Halo CE, or any other XBE is
// running, and works in any runner phase. Returns "" if no anchor is found
// or every match decodes to an empty/whitespace string.
func ReadConsoleName(mem *xemu.Mem) string {
	if mem == nil {
		return ""
	}

	var anchor [8]byte
	binary.LittleEndian.PutUint32(anchor[4:], ConsoleNameRecordMagic)
	// anchor[0:4] stays zero — that's the discriminator against image-code
	// false positives.

	overlap := len(anchor) - 1
	var prevTail []byte

	for gva := consoleNameScanStartGVA; gva < consoleNameScanEndGVA; gva += uint32(consoleNameScanChunk) {
		readN := uint32(consoleNameScanChunk)
		if gva+readN > consoleNameScanEndGVA {
			readN = consoleNameScanEndGVA - gva
		}
		buf, err := mem.ReadBytes(gva, int(readN))
		if err != nil {
			prevTail = nil
			continue
		}
		if IsUnmapped(buf) {
			prevTail = nil
			continue
		}

		// Stitch a small tail from the previous chunk so an anchor straddling
		// the boundary still hits.
		search := buf
		if len(prevTail) > 0 {
			stitched := make([]byte, len(prevTail)+len(buf))
			copy(stitched, prevTail)
			copy(stitched[len(prevTail):], buf)
			search = stitched
		}

		offset := 0
		for offset < len(search) {
			i := bytes.Index(search[offset:], anchor[:])
			if i < 0 {
				break
			}
			hit := offset + i
			nameStart := hit + len(anchor)
			if nameStart+ConsoleNameMaxBytes > len(search) {
				break
			}
			name := strings.TrimSpace(DecodeUTF16LEBounded(search[nameStart:], consoleNameMaxChars))
			if name != "" {
				return name
			}
			offset = hit + len(anchor)
		}

		if overlap > 0 && len(buf) >= overlap {
			prevTail = append([]byte(nil), buf[len(buf)-overlap:]...)
		} else {
			prevTail = nil
		}
	}

	return ""
}
