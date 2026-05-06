package xbox

import (
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// ReadVideoStandard reads the 4-byte LE u32 video standard from the EEPROM
// cache and maps it to a short label ("NTSC-M", "NTSC-J", "PAL-I", "PAL-M").
// Unrecognised values fall back to a hex string. Returns "" on read error.
func ReadVideoStandard(mem *xemu.Mem) string {
	if mem == nil {
		return ""
	}
	v, err := mem.ReadU32(RefAddrVideoStandard)
	if err != nil {
		return ""
	}
	switch v {
	case VideoStandardNTSCM:
		return "NTSC-M"
	case VideoStandardNTSCJ:
		return "NTSC-J"
	case VideoStandardPALI:
		return "PAL-I"
	case VideoStandardPALM:
		return "PAL-M"
	}
	// Unknown — surface raw value so a future kernel/region we haven't seen
	// is still observable rather than silently dropped.
	return rawVideoStandardLabel(v)
}

func rawVideoStandardLabel(v uint32) string {
	const hexdigits = "0123456789ABCDEF"
	out := []byte("0x00000000")
	for i := 0; i < 8; i++ {
		nibble := byte((v >> (4 * (7 - i))) & 0xF)
		out[2+i] = hexdigits[nibble]
	}
	return string(out)
}
