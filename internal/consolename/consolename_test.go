package consolename

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"test-01", "test-01"},
		{"  spaced  ", "spaced"},
		{"emoji😀name", "emojiname"},                        // non-ASCII dropped
		{"ctrl\x00\x07tab\tend", "ctrltabend"},             // control chars dropped
		{"way-too-long-container-name", "way-too-long-co"}, // truncated to NameMax
		{"\x00\x01\x02", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := Sanitize(c.in); got != c.want {
			t.Errorf("Sanitize(%q) = %q, want %q", c.in, got, c.want)
		}
		if got := Sanitize(c.in); len(got) > NameMax {
			t.Errorf("Sanitize(%q) length %d exceeds cap %d", c.in, len(got), NameMax)
		}
	}
}

func TestBuildXBN(t *testing.T) {
	name := "CARTOG"
	buf := BuildXBN(name)
	if len(buf) != FileSize {
		t.Fatalf("payload size = %d, want %d", len(buf), FileSize)
	}
	if buf[0] != 0x04 || buf[1] != 0x00 || buf[2] != 'S' || buf[3] != 'M' {
		t.Errorf("header = % x, want 04 00 53 4d", buf[:4])
	}
	off := 4
	for _, r := range name {
		if got := binary.LittleEndian.Uint16(buf[off:]); got != uint16(r) {
			t.Errorf("char at +%#x = %#x, want %#x", off, got, r)
		}
		off += 2
	}
	if buf[off] != 0 || buf[off+1] != 0 {
		t.Errorf("missing UTF-16 NUL terminator at +%#x", off)
	}
	for i := off + 2; i < len(buf); i++ {
		if buf[i] != 0 {
			t.Errorf("non-zero padding at +%#x", i)
			break
		}
	}
}

func TestBuildXBNRoundTrip(t *testing.T) {
	name := "halo-host-9"
	buf := BuildXBN(name)
	var sb strings.Builder
	for off := 4; off+1 < len(buf); off += 2 {
		u := binary.LittleEndian.Uint16(buf[off:])
		if u == 0 {
			break
		}
		sb.WriteRune(rune(u))
	}
	if sb.String() != name {
		t.Errorf("round-trip = %q, want %q", sb.String(), name)
	}
}
