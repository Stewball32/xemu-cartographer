package podman

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestSanitizeConsoleName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"test-01", "test-01"},
		{"  spaced  ", "spaced"},
		{"halo-host-03", "halo-host-03"},
		{"emoji😀name", "emojiname"},                        // non-ASCII dropped
		{"ctrl\x00\x07tab\tend", "ctrltabend"},             // control chars dropped (\t is 0x09 < 0x20)
		{"way-too-long-container-name", "way-too-long-co"}, // truncated to 15
		{"\x00\x01\x02", ""},                               // nothing usable
		{"", ""},
	}
	for _, c := range cases {
		if got := sanitizeConsoleName(c.in); got != c.want {
			t.Errorf("sanitizeConsoleName(%q) = %q, want %q", c.in, got, c.want)
		}
		if got := sanitizeConsoleName(c.in); len(got) > nicknameNameMax {
			t.Errorf("sanitizeConsoleName(%q) length %d exceeds cap %d", c.in, len(got), nicknameNameMax)
		}
	}
}

func TestBuildNicknameXBN(t *testing.T) {
	name := "test-01"
	buf := buildNicknameXBN(name)

	if len(buf) != nicknameFileSize {
		t.Fatalf("payload size = %d, want %d", len(buf), nicknameFileSize)
	}
	// Header: 04 00 'S' 'M'
	if buf[0] != 0x04 || buf[1] != 0x00 || buf[2] != 'S' || buf[3] != 'M' {
		t.Errorf("header = % x, want 04 00 53 4d", buf[:4])
	}
	// Name: UTF-16LE at +0x04, NUL-terminated.
	off := 4
	for _, r := range name {
		got := binary.LittleEndian.Uint16(buf[off:])
		if got != uint16(r) {
			t.Errorf("char at +%#x = %#x, want %#x (%q)", off, got, r, r)
		}
		off += 2
	}
	// NUL terminator after the name.
	if buf[off] != 0 || buf[off+1] != 0 {
		t.Errorf("missing UTF-16 NUL terminator at +%#x", off)
	}
	// Everything past the terminator is zero padding.
	for i := off + 2; i < len(buf); i++ {
		if buf[i] != 0 {
			t.Errorf("non-zero padding at +%#x: %#x", i, buf[i])
			break
		}
	}
}

func TestBuildNicknameXBNDecodesBack(t *testing.T) {
	name := "halo-host-9"
	buf := buildNicknameXBN(name)
	// Decode UTF-16LE up to the NUL.
	var sb strings.Builder
	for off := 4; off+1 < len(buf); off += 2 {
		u := binary.LittleEndian.Uint16(buf[off:])
		if u == 0 {
			break
		}
		sb.WriteRune(rune(u))
	}
	if sb.String() != name {
		t.Errorf("round-trip decode = %q, want %q", sb.String(), name)
	}
}
