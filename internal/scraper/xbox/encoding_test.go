package xbox

import (
	"testing"
)

// le builds a UTF-16LE byte slice from raw uint16 code units (no terminator).
func le(units ...uint16) []byte {
	out := make([]byte, 2*len(units))
	for i, u := range units {
		out[2*i] = byte(u)
		out[2*i+1] = byte(u >> 8)
	}
	return out
}

func TestDecodeUTF16LE(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"empty", nil, ""},
		{"ascii no terminator", le('H', 'i'), "Hi"},
		{"stops at null", le('H', 'i', 0, 'X'), "Hi"},
		{"leading null", le(0, 'X'), ""},
		{"latin1 accent", le('c', 'a', 'f', 0x00E9), "café"},
		// Odd trailing byte is dropped by len(b)/2 truncation.
		{"odd length drops last byte", append(le('O', 'k'), 0x41), "Ok"},
		// Surrogate pair (U+1F600 GRINNING FACE) → two code units.
		{"surrogate pair", le(0xD83D, 0xDE00), "\U0001F600"},
		{"whole buffer when no null", le('a', 'b', 'c'), "abc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DecodeUTF16LE(c.in); got != c.want {
				t.Errorf("DecodeUTF16LE(% x) = %q; want %q", c.in, got, c.want)
			}
		})
	}
}

func TestDecodeUTF16LEBounded(t *testing.T) {
	cases := []struct {
		name     string
		in       []byte
		maxChars int
		want     string
	}{
		{"empty", nil, 8, ""},
		{"zero max", le('H', 'i'), 0, ""},
		{"under max no null", le('H', 'i'), 8, "Hi"},
		{"stops at max", le('H', 'e', 'l', 'l', 'o'), 3, "Hel"},
		{"stops at null before max", le('H', 'i', 0, 'X'), 8, "Hi"},
		// Truncated final code unit (odd tail) is not read.
		{"truncated trailing unit", append(le('O', 'k'), 0x41), 8, "Ok"},
		{"exact max equals length", le('a', 'b'), 2, "ab"},
		{"surrogate within bound", le(0xD83D, 0xDE00), 2, "\U0001F600"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DecodeUTF16LEBounded(c.in, c.maxChars); got != c.want {
				t.Errorf("DecodeUTF16LEBounded(% x, %d) = %q; want %q", c.in, c.maxChars, got, c.want)
			}
		})
	}
}

func TestEncodeUTF16LE(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []byte
	}{
		{"empty", "", []byte{}},
		{"ascii", "Hi", le('H', 'i')},
		{"accent", "café", le('c', 'a', 'f', 0x00E9)},
		{"surrogate pair", "\U0001F600", le(0xD83D, 0xDE00)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EncodeUTF16LE(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("EncodeUTF16LE(%q) len = %d; want %d (% x)", c.in, len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("EncodeUTF16LE(%q) = % x; want % x", c.in, got, c.want)
				}
			}
		})
	}
}

// Round-trip: encode then decode returns the original for BMP + astral input.
func TestUTF16LERoundTrip(t *testing.T) {
	for _, s := range []string{"", "Halo", "café über", "player_\U0001F600", "console-name"} {
		if got := DecodeUTF16LE(EncodeUTF16LE(s)); got != s {
			t.Errorf("round-trip %q -> %q", s, got)
		}
	}
}
