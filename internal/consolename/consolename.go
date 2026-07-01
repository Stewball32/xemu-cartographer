// Package consolename builds the Original-Xbox console-name file
// E:\UDATA\NICKNAME.XBN — plaintext, no checksum (see the offset-mapper
// SYSTEM-INFO.md §7). A 4-byte header (04 00 'S' 'M') then the name as UTF-16LE,
// NUL-terminated, zero-padded to a fixed 3400-byte file.
//
// This is a pure leaf package (stdlib only) so both the podman provisioner
// (which writes it into a container's HDD overlay) and the gamertag identity
// system (which packs it into a downloadable save bundle) share one source of
// truth for the format. On Xbox, Halo: CE has no MP player profile — the
// console name IS the in-game multiplayer name, which is why generating it is
// the whole of the "CE profile".
package consolename

import (
	"encoding/binary"
	"strings"
)

const (
	// FileSize is the total fixed NICKNAME.XBN size in bytes.
	FileSize = 3400
	// NameMax is the console-name length cap (chars). The Xbox console name
	// allows up to this; note the per-user gamertag is capped tighter (≤11, to
	// fit Halo: CE's in-memory MP name buffer) — a gamertag always fits here.
	NameMax = 15
)

// Sanitize reduces an arbitrary name to a safe Xbox console name: printable
// ASCII only (UTF-16-encodable without surrogates, system-link friendly),
// trimmed, truncated to NameMax. Returns "" when nothing usable remains.
func Sanitize(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 0x20 && r < 0x7f { // printable ASCII
			b.WriteRune(r)
		}
	}
	s := strings.TrimSpace(b.String())
	if len(s) > NameMax {
		s = strings.TrimSpace(s[:NameMax])
	}
	return s
}

// BuildXBN renders the full fixed-size NICKNAME.XBN payload for an
// already-sanitized (printable-ASCII) name. Header 04 00 'S' 'M'; the name at
// +0x04 as UTF-16LE; the NUL terminator + trailing padding are the zeroed
// remainder of the buffer.
func BuildXBN(name string) []byte {
	buf := make([]byte, FileSize)
	buf[0], buf[1], buf[2], buf[3] = 0x04, 0x00, 'S', 'M'
	off := 4
	for _, r := range name { // sanitized => each rune is < 0x80
		binary.LittleEndian.PutUint16(buf[off:], uint16(r))
		off += 2
	}
	return buf
}
