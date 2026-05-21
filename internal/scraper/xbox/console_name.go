package xbox

import (
	"bytes"
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

const (
	consoleNameScanStartGVA uint32 = 0x80000000
	consoleNameScanEndGVA   uint32 = 0x90000000
	consoleNameScanChunk    int    = 1 << 20 // 1 MiB
	consoleNameMaxChars     int    = 16      // 16-wchar Xbox console-name cap
)

// nickHeaderMagic is the 12-byte prefix that consistently precedes every
// console-name record found in xemu guest RAM across both observed
// container shapes (containers in-game with Halo loaded AND containers
// idling on the UnleashX dashboard): "NICK" (`4B 43 49 4E`) followed by
// `01 00 00 00` (record version=1) and four zero bytes (padding to a
// u32 boundary before the record tag at +12). The pattern is distinct
// enough that we haven't seen image-code false positives — the 12-byte
// anchor is far less ambiguous than the original 8-byte
// `00 00 00 00 30 53 11 9E` scan.
var nickHeaderMagic = []byte{
	0x4B, 0x43, 0x49, 0x4E, // "NICK"
	0x01, 0x00, 0x00, 0x00, // record version u32 = 1
	0x00, 0x00, 0x00, 0x00, // 4 bytes of zero padding
}

// After the 16-byte NICK header, two record-body shapes have been
// observed in the wild. Both encode the console name as a UTF-16LE
// string up to consoleNameMaxChars wchars; only the body header differs.
var (
	// nickRecordTypeA is the "compact" body: a 4-byte magic followed
	// immediately by the name bytes. Decodes as LE u32 0x4D530004 —
	// almost certainly a Microsoft / Xbox SDK record-type marker
	// (`53 4D` reads as "SM"). Observed across all three of host,
	// alpha, and bravo regardless of their loaded XBE.
	nickRecordTypeA = []byte{0x04, 0x00, 0x53, 0x4D}

	// nickRecordTypeB is the canonical NICKNAME.XBN record: the
	// 4-byte ConsoleNameRecordMagic (0x9E115330) followed by a u16
	// length (`20 00` = 0x0020 = 32 bytes / 16 wchars max). Observed
	// on charlie (UnleashX dashboard fully booted, NICKNAME.XBN
	// mapped into RAM). The first 4 bytes match the legacy 8-byte
	// anchor the previous scanner used (`00 00 00 00 30 53 11 9E`).
	nickRecordTypeB = []byte{0x30, 0x53, 0x11, 0x9E}
)

// ReadConsoleName scans guest RAM for a console-name record loaded from
// E:\UDATA\NICKNAME.XBN and returns the decoded name. The 12-byte NICK
// anchor + record-type dispatch handles both the type A (compact, seen
// in host/alpha/bravo) and type B (canonical NICKNAME.XBN, seen in
// charlie) layouts, so it works whether the container is running Halo,
// idling on the dashboard, or anywhere in between.
//
// Returns the FIRST decoded record in the scan window. Per-controller
// nickname slots have the same NICK header and would match too, but
// the scan walks low-to-high GVA and the console-name record is
// historically slot 0 — so the controller slots are skipped naturally
// by returning on the first non-empty decode.
//
// Title-agnostic. Returns "" if no anchor is found or every match
// decodes to an empty / whitespace string.
func ReadConsoleName(mem *xemu.Mem) string {
	if mem == nil {
		return ""
	}

	// Width of the longest legal record after the NICK header. Type B
	// is 4-byte magic + 2-byte length + name bytes; type A is 4-byte
	// magic + name bytes. Reserve the longer.
	const maxRecordTail = 4 + 2 + ConsoleNameMaxBytes
	overlap := len(nickHeaderMagic) + maxRecordTail - 1
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

		// Stitch a small tail from the previous chunk so a header
		// straddling the boundary still hits.
		search := buf
		if len(prevTail) > 0 {
			stitched := make([]byte, len(prevTail)+len(buf))
			copy(stitched, prevTail)
			copy(stitched[len(prevTail):], buf)
			search = stitched
		}

		offset := 0
		for offset < len(search) {
			i := bytes.Index(search[offset:], nickHeaderMagic)
			if i < 0 {
				break
			}
			hit := offset + i
			recordStart := hit + len(nickHeaderMagic)
			if recordStart+4 > len(search) {
				break
			}

			recordTag := search[recordStart : recordStart+4]
			var nameStart int
			switch {
			case bytes.Equal(recordTag, nickRecordTypeA):
				nameStart = recordStart + 4
			case bytes.Equal(recordTag, nickRecordTypeB):
				// Skip the 4-byte magic AND the 2-byte length field
				// (0x0020 = 32 bytes / 16 wchars max).
				nameStart = recordStart + 4 + 2
			default:
				// Unknown record type — keep walking. Could be a stray
				// "NICK" in image code; the post-header bytes will
				// nearly always fail this check.
				offset = hit + len(nickHeaderMagic)
				continue
			}

			if nameStart+ConsoleNameMaxBytes > len(search) {
				break
			}
			name := strings.TrimSpace(DecodeUTF16LEBounded(search[nameStart:], consoleNameMaxChars))
			if name != "" {
				return name
			}
			offset = hit + len(nickHeaderMagic)
		}

		if overlap > 0 && len(buf) >= overlap {
			prevTail = append([]byte(nil), buf[len(buf)-overlap:]...)
		} else {
			prevTail = nil
		}
	}

	return ""
}
