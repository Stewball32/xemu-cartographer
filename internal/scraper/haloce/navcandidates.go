package haloce

// NavCandidates are UNCLASSIFIED low globals surfaced RAW on the admin diagnostics
// panel so an operator can watch which one actually tracks a screen's state. They
// came out of two capture-and-diff hunts on the net rig (2026-08-08), each with an
// ambient-noise baseline subtracted (a second capture with NO input, so timer/RNG
// churn is excluded):
//
//	HUNT A — Multiplayer submenu, highlighted multiplayer_type_conn_item, then A.
//	         620 bytes moved, 216 after noise subtraction, in 150 regions.
//	HUNT B — COLD main menu (18 live widget blocks, tick 0), Down x3.
//	         Only 16 bytes moved on EVERY step after noise subtraction, and NONE
//	         monotonic — so there is still no low-global selection index on a cold
//	         menu (this is the rigorous re-confirmation of that earlier finding).
//
// The addresses that moved in BOTH hunts are listed first — they sit in the
// menu-focus / game_connection / main_menu struct neighbourhoods and are the most
// promising "does this track the screen?" candidates.
var navCandidates = []struct {
	Label string
	Addr  uint32
}{
	// Moved in BOTH hunts (highest interest).
	{"c1_focus-8 (0x2F9B30)", 0x2F9B30},   // 8 bytes below menu_focus — same struct
	{"c2_conn-0x38 (0x2E364C)", 0x2E364C}, // 0x38 below game_connection
	{"c3_0x2E440C", 0x2E440C},
	// Main-menu global neighbourhood (hunt A + cold-menu churn).
	{"c4_mm-0x68 (0x2E4000)", 0x2E4000},
	{"c5_mm-0x58 (0x2E4010)", 0x2E4010},
	{"c6_mm+0x0C (0x2E4074)", 0x2E4074},
	// Hunt A only: head of a stride-0x208 datum array whose entries went
	// 0x800a->0x000b, 0x810a->0x010b, ... (a valid-bit clearing while an index is
	// preserved and a generation counter increments) — i.e. a table being re-inited.
	{"c7_array0 (0x2E8874)", 0x2E8874},
	{"c8_array1 (0x2E8A7C)", 0x2E8A7C},
	// Cold-menu hunt only.
	{"c9_0x2FEE9D", 0x2FEE9C},
	{"c10_0x2FEF1A", 0x2FEF18},
}

// readNavCandidates reads each candidate as a u32. Cheap (10 small reads) and purely
// diagnostic — nothing routes on these. Unreadable addresses report 0.
func (r *Reader) readNavCandidates() map[string]uint32 {
	out := make(map[string]uint32, len(navCandidates))
	for _, c := range navCandidates {
		v, err := r.inst.Mem.ReadU32(c.Addr)
		if err != nil {
			v = 0
		}
		out[c.Label] = v
	}
	return out
}
