package xbox

import (
	"fmt"

	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// ReadMACAddress reads the 6-byte MAC address from the kernel's EEPROM cache
// and formats it as a colon-separated lower-case hex string (e.g.
// "00:50:f2:07:8d:5c"). Returns "" on read error.
func ReadMACAddress(mem *xemu.Mem) string {
	if mem == nil {
		return ""
	}
	buf, err := mem.ReadBytes(RefAddrMACAddress, LenMACAddress)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
}
