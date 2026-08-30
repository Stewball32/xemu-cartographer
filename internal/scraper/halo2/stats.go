package halo2

import (
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// Pure decode helpers for the imported match-stats / lifecycle globals
// (docs/h2-kd-semantics-2026-07-11.md + docs/h2-slim-offsets-2026-08-13.md in
// halo-offset-mapper). Kept free of memory I/O so they unit-test directly.

// gametypeName maps the H2 e_game_engine index (AddrH2Gametype) onto the wire
// gametype vocabulary the CE plugin already emits. Values induced live
// 2026-07-11 by changing only the gametype on a fixed map: 1 ctf, 2 slayer,
// 3 oddball, 4 king. Anything else (0 at boot, future modes) is unknown → "".
func gametypeName(v uint32) string {
	switch v {
	case 1:
		return "ctf"
	case 2:
		return "slayer"
	case 3:
		return "oddball"
	case 4:
		return "king"
	}
	return ""
}

// phaseFromEnum maps the H2 lifecycle enum (AddrH2GamePhase — full-cycle
// validated menu→lobby→in-game→postgame→lobby on two independent rig builds)
// onto the reader's GameState. ok=false for values the enum never took in
// validation (2 was falsified single-pass noise on another global; treat any
// unknown as "no verdict" and let the array inference decide).
func phaseFromEnum(v uint32) (scraper.GameState, bool) {
	switch v {
	case 0:
		return scraper.GameStateMenu, true
	case 1:
		return scraper.GameStatePreGame, true
	case 3:
		return scraper.GameStateInGame, true
	case 4:
		return scraper.GameStatePostGame, true
	}
	return scraper.GameStateMenu, false
}

// killsSlotDelta / deathsSlotDelta are the per-slot strides into the two
// per-player stat arrays: kills u16 @ base+2*slot, deaths u32 @ base+4*slot.
// Slots 0..15 stay well inside the arrays' 4K pages, so a single translated
// base HVA + delta read is safe.
func killsSlotDelta(slot int) int64  { return int64(slot) * 2 }
func deathsSlotDelta(slot int) int64 { return int64(slot) * 4 }

// macSlotDelta is the per-machine stride into the MAC array (6-byte entries).
func macSlotDelta(machine int) int64 { return int64(machine) * 6 }

// machineEntryDelta is a machine's entry offset within the machine table
// (stride 0xB4). The caller must page-guard reads at the resulting address —
// see machineNameReadable.
func machineEntryDelta(machine int) int64 {
	return int64(machine) * int64(ConstH2NetMachineStride)
}

// scenarioBasename extracts the final path segment of a NUL-terminated ASCII
// tag path (e.g. "scenarios\\multi\\zanzibar\\zanzibar" → "zanzibar"). ok=false
// when the buffer doesn't hold a printable-ASCII path — the pool may simply
// not be populated yet, and garbage must never become a "map name".
func scenarioBasename(raw []byte) (string, bool) {
	n := 0
	for n < len(raw) && raw[n] != 0 {
		if raw[n] < 0x20 || raw[n] > 0x7E {
			return "", false
		}
		n++
	}
	if n == 0 || n == len(raw) {
		return "", false // empty, or unterminated within the read window
	}
	s := string(raw[:n])
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\\' || s[i] == '/' {
			s = s[i+1:]
			break
		}
	}
	if s == "" {
		return "", false
	}
	return s, true
}

// machineNameReadable reports whether machine i's name field can be read
// through the table base's translated page. The low-GVA cache translates the
// base's single 4K page, and guest-contiguous is NOT host-contiguous across a
// page edge — any byte outside the base's page would read the wrong host
// memory entirely. Entries whose name span leaves the page are skipped (the
// machine still lists, just unnamed). With the table at 0x54D91C, entries
// 0..9 read safely; a LAN session rarely exceeds 8 machines.
func machineNameReadable(tableGVA uint32, machine int, nameBytes int) bool {
	start := int64(tableGVA) + machineEntryDelta(machine) + int64(OffH2NetMachineName)
	end := start + int64(nameBytes)
	pageStart := int64(tableGVA) &^ 0xFFF
	return start >= pageStart && end <= pageStart+0x1000
}
