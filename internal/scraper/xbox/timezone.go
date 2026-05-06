package xbox

import (
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// TimeZone bundles the three EEPROM time-zone fields that travel together.
type TimeZone struct {
	BiasMinutes int32  // UTC offset in minutes (positive = west of UTC)
	StdName     string // standard-time abbreviation (e.g. "GMT", "EST")
	DltName     string // daylight-saving abbreviation (e.g. "BST", "EDT")
}

// ReadTimeZone reads the three contiguous EEPROM-cache time-zone fields.
// Returns the zero TimeZone with ok=false on any read error so callers can
// distinguish "not yet populated" from "populated, all-zero".
func ReadTimeZone(mem *xemu.Mem) (TimeZone, bool) {
	if mem == nil {
		return TimeZone{}, false
	}
	bias, err := mem.ReadS32(RefAddrTimeZoneBias)
	if err != nil {
		return TimeZone{}, false
	}
	stdName, err := readTimeZoneNameAt(mem, RefAddrTimeZoneStdName)
	if err != nil {
		return TimeZone{}, false
	}
	dltName, err := readTimeZoneNameAt(mem, RefAddrTimeZoneDltName)
	if err != nil {
		return TimeZone{}, false
	}
	return TimeZone{
		BiasMinutes: bias,
		StdName:     stdName,
		DltName:     dltName,
	}, true
}

func readTimeZoneNameAt(mem *xemu.Mem, gva uint32) (string, error) {
	buf, err := mem.ReadBytes(gva, LenTimeZoneName)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(buf), "\x00 "), nil
}
