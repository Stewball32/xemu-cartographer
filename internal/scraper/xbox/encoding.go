package xbox

import (
	"encoding/binary"
	"unicode/utf16"
)

// DecodeUTF16LE decodes a null-terminated UTF-16LE byte slice into a Go
// string. Stops at the first 0x0000 code unit; if none is present, decodes
// the whole buffer.
func DecodeUTF16LE(b []byte) string {
	u16s := make([]uint16, len(b)/2)
	for i := range u16s {
		u16s[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	for i, c := range u16s {
		if c == 0 {
			u16s = u16s[:i]
			break
		}
	}
	return string(utf16.Decode(u16s))
}

// DecodeUTF16LEBounded reads up to maxChars UTF-16LE code units from data,
// stopping at the first null terminator, and returns the decoded string.
func DecodeUTF16LEBounded(data []byte, maxChars int) string {
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

// EncodeUTF16LE encodes a Go string into a UTF-16LE byte slice. No BOM, no
// trailing null. Surrogate pairs from utf16.Encode are emitted as two code
// units each, matching how the xbox kernel lays out wide strings in heap.
func EncodeUTF16LE(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 2*len(units))
	for i, u := range units {
		out[2*i] = byte(u)
		out[2*i+1] = byte(u >> 8)
	}
	return out
}
