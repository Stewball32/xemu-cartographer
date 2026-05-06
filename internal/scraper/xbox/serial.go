package xbox

import (
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// ReadSerialNumber reads the 12-character ASCII Xbox console serial number
// from the kernel's EEPROM cache. Returns "" on read error or when the field
// is unmapped/blank (uncommon — populated at boot before any XBE runs).
//
// The field is fixed-width 12 bytes, padded with spaces or NULs when the
// underlying serial is shorter; trailing whitespace and NULs are stripped
// before return.
func ReadSerialNumber(mem *xemu.Mem) string {
	if mem == nil {
		return ""
	}
	buf, err := mem.ReadBytes(RefAddrSerialNumber, LenSerialNumber)
	if err != nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(string(buf)), "\x00")
}
