package haloce

import "fmt"

// Nav candidates: UNCLASSIFIED low globals surfaced RAW on the admin diagnostics panel
// so an operator can watch which one actually tracks a screen's state.
//
// They came out of two capture-and-diff hunts on the net rig (2026-08-08), each with an
// ambient-noise baseline subtracted (a second capture with NO input, so timer/RNG churn
// is excluded):
//
//	HUNT A — Multiplayer submenu, highlighted multiplayer_type_conn_item, then A.
//	         620 bytes moved, 216 after noise subtraction, in 150 regions.
//	HUNT B — COLD main menu (18 live widget blocks, tick 0), Down x3.
//	         Only 16 bytes moved on EVERY step after noise subtraction, and NONE
//	         monotonic — so there is no low-global selection index on a cold menu.
//
// ANCHORED, NOT ABSOLUTE. The hunts produced absolute addresses on the rig's build, but
// the runner also drives a different XBE (the dedicated "server" build), where absolute
// rig addresses need not land in the same place. Each candidate is therefore stored as a
// DELTA from a base global that the per-build offset set already resolves correctly
// (menu_focus / game_connection / main_menu are known-good on every build we run). So a
// candidate follows its neighbourhood across builds instead of pointing at a fixed
// address that may be meaningless elsewhere.
type navCandidate struct {
	Label string
	Delta int32 // signed offset from Base
	Base  func(r *Reader) uint32
}

var navCandidates = []navCandidate{
	// Moved in BOTH hunts (highest interest).
	{"c1_focus-0x08", -0x08, func(r *Reader) uint32 { return r.off.AddrUiWidgetFocusPtr }},
	{"c2_conn-0x38", -0x38, func(r *Reader) uint32 { return r.off.AddrGameConnection }},
	{"c3_mm+0x3A4", +0x3A4, func(r *Reader) uint32 { return r.off.AddrMainMenuActive }}, // 0x2E440C
	// main_menu global neighbourhood.
	{"c4_mm-0x68", -0x68, func(r *Reader) uint32 { return r.off.AddrMainMenuActive }},
	{"c5_mm-0x58", -0x58, func(r *Reader) uint32 { return r.off.AddrMainMenuActive }},
	{"c6_mm+0x0C", +0x0C, func(r *Reader) uint32 { return r.off.AddrMainMenuActive }},
	// Hunt A only: head of a stride-0x208 datum array whose entries went
	// 0x800a->0x000b, 0x810a->0x010b, ... (a valid bit clearing while the index is
	// preserved and a generation counter increments) — a table being re-initialised.
	{"c7_array0", +0x480C, func(r *Reader) uint32 { return r.off.AddrMainMenuActive }},
	{"c8_array1", +0x4A14, func(r *Reader) uint32 { return r.off.AddrMainMenuActive }},
	// Cold-menu hunt only (relative to menu_focus).
	{"c9_focus+0x5364", +0x5364, func(r *Reader) uint32 { return r.off.AddrUiWidgetFocusPtr }},
	{"c10_focus+0x53E0", +0x53E0, func(r *Reader) uint32 { return r.off.AddrUiWidgetFocusPtr }},
}

// readNavCandidates reads each candidate as a u32 THROUGH THE LOW-GVA TRANSLATION.
//
// These are LOW guest VAs (0x2E…/0x2F…), which are NOT identity-mapped and NOT in the
// high 0x80000000 window. They must go through inst.LowHVA (the instance's cached
// translation) exactly like game_connection / main_menu / menu_focus do. The first cut
// used Mem.ReadU32, which internally applies Mem.HighGVA — that translated these low
// addresses as if they were high-window ones and read unrelated host memory, which is
// why the panel showed the repeating fill pattern 0x22002200 and zeros instead of live
// values.
//
// Cheap (10 small reads) and purely diagnostic — nothing routes on these. An address
// that can't be translated/read reports 0 and is labelled so on the panel.
func (r *Reader) readNavCandidates() map[string]uint32 {
	out := make(map[string]uint32, len(navCandidates))
	for _, c := range navCandidates {
		gva := uint32(int64(c.Base(r)) + int64(c.Delta))
		label := fmt.Sprintf("%s (0x%X)", c.Label, gva)
		hva, err := r.inst.LowHVA(gva)
		if err != nil {
			out[label] = 0
			continue
		}
		v, err := r.inst.Mem.ReadU32At(hva)
		if err != nil {
			v = 0
		}
		out[label] = v
	}
	return out
}
