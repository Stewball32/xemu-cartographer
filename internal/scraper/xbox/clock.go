package xbox

import (
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// SystemClock bundles the three time values derivable from the kernel's clock
// globals — wall-clock UTC, time since guest boot, and the wall-clock instant
// at which the guest booted. Returning all three from a single read avoids
// snapshot-time skew between SystemTime and InterruptTime that would otherwise
// produce a slightly-wrong BootTime when callers read them separately.
type SystemClock struct {
	SystemTime time.Time     // wall-clock UTC, derived from KeSystemTime
	Uptime     time.Duration // since guest boot, derived from KeInterruptTime
	BootTime   time.Time     // SystemTime - Uptime
}

// ReadSystemClock reads both kernel-clock globals back-to-back and returns
// the decoded triplet. ok=false on any read error or when the FILETIME is
// implausibly small (before the FILETIME→Unix epoch offset, which would mean
// the kernel hasn't yet populated the field — common during the first few
// hundred milliseconds of boot).
func ReadSystemClock(mem *xemu.Mem) (SystemClock, bool) {
	if mem == nil {
		return SystemClock{}, false
	}
	systemFT, err := mem.ReadU64(RefAddrKeSystemTime)
	if err != nil {
		return SystemClock{}, false
	}
	if systemFT < FileTimeEpochOffsetTicks {
		// Pre-1970 = uninitialised. Avoid producing a time.Time that points
		// to the year 1601 (correct but useless to a human reader).
		return SystemClock{}, false
	}
	interruptTicks, err := mem.ReadU64(RefAddrKeInterruptTime)
	if err != nil {
		return SystemClock{}, false
	}

	systemTime := fileTimeToGoTime(systemFT)
	uptime := time.Duration(interruptTicks * 100) // 100ns ticks → ns
	return SystemClock{
		SystemTime: systemTime,
		Uptime:     uptime,
		BootTime:   systemTime.Add(-uptime),
	}, true
}

// fileTimeToGoTime converts a Windows FILETIME (100ns ticks since 1601-01-01
// UTC) to a Go time.Time. Caller is responsible for filtering out values
// before the Unix epoch — they convert correctly but are almost certainly
// uninitialised kernel state rather than legitimate pre-1970 timestamps.
func fileTimeToGoTime(ft uint64) time.Time {
	// time.Unix takes seconds + nanoseconds; we have a single 100ns count.
	// Subtract the 1601→1970 offset, then multiply remaining ticks by 100
	// to get nanoseconds since the Unix epoch.
	unixTicks := ft - FileTimeEpochOffsetTicks
	return time.Unix(0, int64(unixTicks*100)).UTC()
}
