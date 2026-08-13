package halosave

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

// getU32 reads a little-endian uint32 at off.
func getU32(b []byte, off int) uint32 {
	return binary.LittleEndian.Uint32(b[off : off+4])
}

// putU32 writes a little-endian uint32 at off (mirrors the Python pu32, which
// masks to 32 bits).
func putU32(b []byte, off int, v uint32) {
	binary.LittleEndian.PutUint32(b[off:off+4], v)
}

// getF32 reads a little-endian IEEE-754 float32 at off (e.g. CE max-health
// multiplier @0x3C).
func getF32(b []byte, off int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(b[off : off+4]))
}

// putF32 writes a little-endian float32 at off.
func putF32(b []byte, off int, v float32) {
	binary.LittleEndian.PutUint32(b[off:off+4], math.Float32bits(v))
}

// readUTF16z reads a NUL-terminated UTF-16LE string from b[off:off+maxBytes].
// Reading stops at the first 0x0000 code unit or the end of the window.
func readUTF16z(b []byte, off, maxBytes int) string {
	raw := b[off : off+maxBytes]
	units := make([]uint16, 0, maxBytes/2)
	for i := 0; i+1 < len(raw); i += 2 {
		ch := uint16(raw[i]) | uint16(raw[i+1])<<8
		if ch == 0 {
			break
		}
		units = append(units, ch)
	}
	return string(utf16.Decode(units))
}

// writeUTF16z writes name as UTF-16LE into a fixed bufBytes-wide buffer at off,
// zero-filling the whole buffer first (so the trailing NUL terminator and any
// stale bytes are cleared). It mirrors the Python _utf16z_write contract: it
// errors if the encoded name plus its 2-byte NUL terminator would not fit.
func writeUTF16z(b []byte, off, bufBytes int, name string) error {
	enc := utf16.Encode([]rune(name))
	encBytes := len(enc) * 2
	if encBytes+2 > bufBytes {
		return fmt.Errorf("halosave: name %q too long for %d-byte buffer (max %d chars)",
			name, bufBytes, (bufBytes-2)/2)
	}
	// Zero the whole buffer, then lay down the encoded name; remaining bytes
	// stay zero, which provides the NUL terminator.
	for i := off; i < off+bufBytes; i++ {
		b[i] = 0
	}
	for i, u := range enc {
		binary.LittleEndian.PutUint16(b[off+i*2:off+i*2+2], u)
	}
	return nil
}

// utf16CharLen returns how many UTF-16 code units name encodes to. Useful for
// validating name length against a buffer before writing.
func utf16CharLen(name string) int {
	return len(utf16.Encode([]rune(name)))
}
