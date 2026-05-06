// Package xbox holds title-agnostic, kernel- and system-level memory readers
// for the original Xbox running inside xemu — values that are true regardless
// of the running XBE (Halo, UnleashX dashboard, anything else) and regardless
// of the scraper runner's phase. These primitives may be called from Idle,
// Ready, or Live without a game-specific GameReader bound.
//
// Adding a new reader: prefer free functions taking *xemu.Mem (and any
// title-agnostic args) over methods on a service type. Cache nothing here —
// the runner gates re-reads via instanceCache.SystemScannedFor.
package xbox

// IsUnmapped reports whether the leading 256 bytes of data are all 0xFF —
// xemu's marker for an unbacked guest page. Cheap chunk-skip predicate for
// scanners walking across guest physical RAM.
func IsUnmapped(data []byte) bool {
	n := 256
	if len(data) < n {
		n = len(data)
	}
	for i := 0; i < n; i++ {
		if data[i] != 0xFF {
			return false
		}
	}
	return true
}
