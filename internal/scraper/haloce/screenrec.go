package haloce

import "time"

// screenrec.go — the CE front-end SCREEN-RECORD classifier (halo-offset-mapper
// docs/MENU-NAV-PACING-2026-08-10.md, commit b991fd0; verified live on ce-h1perf,
// addresses identical family-wide for the in-place-patched stock-CE builds).
//
// AddrUiCurrentScreenRec points into a fixed .data pool of screen records;
// rec+0x00 is the widget_definition ('DeLa') tag id of the CURRENT screen.
// Resolving that id through the cache tag table yields the screen's canonical
// path (`ui\shell\main_menu\main_menu`, `…\4way_start2join_screen`, …) — a
// 2-fixed-read + cached-resolve classification that replaces the ~2 MiB UI-heap
// DeLa-fingerprint scan for "which screen am I on", distinguishes visually
// identical screens (campaign ENTER NAME vs profiles ENTER NAME carry different
// tags), and is what lets the host runner tick at ~100ms.
//
// RULES (from the hunt):
//   - Slots are REUSED across visits (one address held different screens at
//     different times, one screen appeared under two addresses) — ALWAYS resolve
//     the tag id, never compare or cache by record address.
//   - AddrUiBackScreenRec is the record of the screen B returns to; it reads 0
//     EXACTLY at the root main menu (going non-zero is the cold-prime's "a
//     submenu opened" confirm; with the resolved path it derives the panel's
//     at_root_menu diagnostic).
const (
	// AddrUiCurrentScreenRec → u32 ptr to the CURRENT screen's record (0 when no
	// shell screen is up / very early boot).
	AddrUiCurrentScreenRec uint32 = 0x2E4000
	// AddrUiBackScreenRec → u32 ptr to the record B would return to; == 0 exactly
	// at the root main menu.
	AddrUiBackScreenRec uint32 = 0x2E4010
	// OffUiScreenRecTagID: rec+0x00 = widget_definition tag id of the screen.
	// (rec+0x04 = runtime widget instance in the UI heap, rec+0x18 = an
	// AddrUiMsClock stamp — neither is read here.)
	OffUiScreenRecTagID uint32 = 0x00

	// Support offsets (diagnostic surfaces on the admin panel; the runner routes
	// on none of them yet except as noted):
	//   AddrUiOskActive — u8, 1 while the on-screen keyboard is CAPTURING input
	//     (key presses land in the text buffer, not menu nav).
	//   AddrUiMsClock — free-running ms-scale UI clock; the correct "UI alive"
	//     heartbeat (hashing static RAM false-negatives on an idle menu).
	//   AddrUiFadeState — byte pair D5/49 at the root menu ↔ D4/48 on a
	//     sub-screen, one atomic flip per transition (read as a LE u16).
	AddrUiOskActive uint32 = 0x2E3DA8
	AddrUiMsClock   uint32 = 0x2E4020
	AddrUiFadeState uint32 = 0x2D37D4

	// OffUiScreenRecStamp: rec+0x18 = the AddrUiMsClock value stamped when the
	// screen was (re)activated. The ACTIVE screen's widget blocks carry this
	// exact value in their +0x28 activation tick — the invariant (verified on
	// every capture incl. cold, where both read 0) that lets the highlight pick
	// drop stale prior-screen blocks (docs/MENU-ENTRYFLOW-2026-08-11.md §7).
	OffUiScreenRecStamp uint32 = 0x18

	// System Link ENTRY-FLOW per-press effect fields (port-0 controller slot;
	// mapper §1, live-verified through five staged rigs). These are what confirm
	// each A of the join → select → commit ladder — menu_focus relinks on
	// DELIVERY even when the flow ignores the press, and the screen record stays
	// on 4way_start2join for the whole flow, so these fields are the ONLY
	// per-step truth:
	//   claim A  → AddrUiSlotClaimed 0->1 (~173ms)
	//   select A → AddrUiSlotProfile -1 -> profile handle (~163ms)
	//   commit A → record + back-rec + game_connection flip in the same frame
	// PERSISTENCE CAUTION: neither field resets on flow exit / B-out — consumers
	// must gate on the record being the 4way screen first. Ports 1–3 are
	// hypothesized at +0x208 strides, unverified — port 0 only here.
	AddrUiSlotClaimed uint32 = 0x2E4104 // u8: this slot pressed "A to Join"
	AddrUiSlotProfile uint32 = 0x2E4100 // u32: selected profile handle; 0xFFFFFFFF = none
)

// Screen-record pointer sanity bounds: the port-0 widget-instance arena,
// [0x4B2B70, 0x4C2B80) — 64 KiB, mapped by the offset hunt
// (docs/MENU-ENTRYFLOW-2026-08-11.md §6; the end bound is held in the global at
// 0x2E46D8, deepest observed excursion 0x4B7C4C). Record pointers are PAYLOAD
// addresses inside this arena (block header at rec-0x10); anything outside it is
// a garbage read we refuse to chase with a QMP page translation. The root
// record sits at 0x4B2B80 but every other slot is reused — always resolve tags,
// never compare addresses.
const (
	screenRecPtrMin uint32 = 0x4B2B70
	screenRecPtrMax uint32 = 0x4C2B80
)

// dynPageFailRetry throttles re-attempted QMP page translations for a page that
// failed to translate, so a persistently-unmapped page can't turn into a QMP
// hammer from the read loop (rapid QMP reconnects can wedge some xemu builds).
const dynPageFailRetry = 10 * time.Second

// readUiScreen reads the current/back screen records, resolves the current
// screen's widget_definition tag to its DeLa path, and reads the record's
// activation STAMP (rec+0x18 — the value the active screen's widget blocks carry
// in their +0x28 tick; the highlight pick gates on it). Returns ("", cur, back,
// 0, false) when anything along the way is unreadable — callers treat an empty
// path as "no screen-record signal" and fall back to the heap classification.
//
// Loop-goroutine only (mutates the reader's resolve caches).
func (r *Reader) readUiScreen() (path string, cur, back, stamp uint32, stampOK bool) {
	cur = r.readLowU32(r.off.AddrUiCurrentScreenRec)
	back = r.readLowU32(r.off.AddrUiBackScreenRec)
	if cur == 0 {
		return "", cur, back, 0, false
	}
	// The record lives inside the widget-instance arena at an address the Init
	// list can't know in advance (slots are reused). Bound it, keep the reads
	// (tag id at +0x00, stamp at +0x18) inside one page, then translate the page
	// on demand (cached per reader bind).
	if cur < screenRecPtrMin || cur >= screenRecPtrMax || cur&0xFFF > 0xFE4 {
		return "", cur, back, 0, false
	}
	hva, ok := r.lowHVADynamic(cur + OffUiScreenRecTagID)
	if !ok {
		return "", cur, back, 0, false
	}
	tagID, err := r.inst.Mem.ReadU32At(hva)
	if err != nil || tagID == 0 || tagID == 0xFFFFFFFF {
		return "", cur, back, 0, false
	}
	if v, err := r.inst.Mem.ReadU32At(hva + int64(OffUiScreenRecStamp)); err == nil {
		stamp, stampOK = v, true
	}
	return r.delaPathForTagID(tagID), cur, back, stamp, stampOK
}

// delaPathForTagID resolves a widget_definition tag id to its tag path by
// DIRECT INDEX into the cache tag array (entry = tag_array + (id&0xFFFF)*0x20,
// name ptr at entry+0x10) — no full-array walk. The entry must round-trip: its
// stored handle (+0x0C) must equal the id and its group must be 'DeLa', so a
// stale/garbage id resolves to "" instead of a wrong screen. Results are cached
// per tag id (the pool reuses record SLOTS, so the address is never the key —
// the tag id is); the cache clears with the other UI-session caches on menu
// entry (OnStateChange).
func (r *Reader) delaPathForTagID(tagID uint32) string {
	if p, ok := r.screenTagPaths[tagID]; ok {
		return p
	}
	th, err := r.inst.DerefLowPtr(r.off.AddrTagHeaderPtr)
	if err != nil || th < HighGVAThreshold {
		return ""
	}
	mem := r.inst.Mem
	tagArray, err := mem.ReadU32(th + OffTagHeaderTagArray)
	if err != nil || tagArray < HighGVAThreshold {
		return ""
	}
	count, err := mem.ReadU32(th + OffTagHeaderTagCount)
	if err != nil || count == 0 || count > 65535 {
		return ""
	}
	idx := tagID & 0xFFFF
	if idx >= count {
		return ""
	}
	entry, err := mem.ReadBytes(tagArray+idx*ConstTagEntrySize, int(ConstTagEntrySize))
	if err != nil {
		return ""
	}
	grp := []byte{entry[3], entry[2], entry[1], entry[0]}
	if string(grp) != tagGroupDela {
		return ""
	}
	if leU32(entry, OffTagHandle) != tagID {
		return "" // torn/stale id — the entry doesn't round-trip
	}
	namePtr := leU32(entry, OffTagNamePtr)
	if namePtr < HighGVAThreshold {
		return ""
	}
	path := r.readHighString(namePtr)
	if path == "" {
		return ""
	}
	if r.screenTagPaths == nil {
		r.screenTagPaths = make(map[uint32]string, 8)
	}
	r.screenTagPaths[tagID] = path
	return path
}

// readLowU32 reads a u32 at a PRE-TRANSLATED low GVA, best-effort (0 on any
// failure). For the fixed screen-record globals, which are in AllLowGVAs.
func (r *Reader) readLowU32(gva uint32) uint32 {
	hva, err := r.inst.LowHVA(gva)
	if err != nil {
		return 0
	}
	v, err := r.inst.Mem.ReadU32At(hva)
	if err != nil {
		return 0
	}
	return v
}

// readLowU16 is readLowU32's u16 sibling (the fade-state byte pair), routed
// through the DYNAMIC page translation because its page carries no other
// pre-translated global — a failed translate must degrade to 0, never block
// instance attach.
func (r *Reader) readLowU16Dynamic(gva uint32) uint16 {
	hva, ok := r.lowHVADynamic(gva)
	if !ok {
		return 0
	}
	v, err := r.inst.Mem.ReadU16At(hva)
	if err != nil {
		return 0
	}
	return v
}

// readLowU32Dynamic reads a u32 through the DYNAMIC page translation, reporting
// readability so callers can distinguish "reads 0" from "unreadable" (the
// game-over flag's exact-sentinel check needs that distinction).
func (r *Reader) readLowU32Dynamic(gva uint32) (uint32, bool) {
	hva, ok := r.lowHVADynamic(gva)
	if !ok {
		return 0, false
	}
	v, err := r.inst.Mem.ReadU32At(hva)
	if err != nil {
		return 0, false
	}
	return v, true
}

// lowHVADynamic translates an ARBITRARY low GVA to a host VA, for reads whose
// target address is only known at runtime (screen-record slots) or whose page
// carries no Init-time global (the fade pair). Page-granular with a per-reader
// cache so the QMP cost is one round-trip per page per reader bind:
//
//  1. the reader's page cache (fresh per bind — a stale post-XBE-swap mapping
//     dies with the reader instead of lingering in the instance);
//  2. the instance's Init-time table, exact then page-base (what unit tests
//     pre-register — no QMP in tests);
//  3. one RefreshLowHVA(page) QMP translation, negative-cached with a retry
//     window so an unmapped page can't become a per-tick QMP hammer.
func (r *Reader) lowHVADynamic(gva uint32) (int64, bool) {
	page := gva &^ 0xFFF
	off := int64(gva & 0xFFF)
	if base, ok := r.lowPageHVAs[page]; ok {
		return base + off, true
	}
	if hva, err := r.inst.LowHVA(gva); err == nil {
		return hva, true
	}
	if base, err := r.inst.LowHVA(page); err == nil {
		return base + off, true
	}
	if last, ok := r.lowPageFails[page]; ok && time.Since(last) < dynPageFailRetry {
		return 0, false
	}
	base, err := r.inst.RefreshLowHVA(page)
	if err != nil {
		if r.lowPageFails == nil {
			r.lowPageFails = make(map[uint32]time.Time, 2)
		}
		r.lowPageFails[page] = time.Now()
		return 0, false
	}
	if r.lowPageHVAs == nil {
		r.lowPageHVAs = make(map[uint32]int64, 2)
	}
	r.lowPageHVAs[page] = base
	delete(r.lowPageFails, page)
	return base + off, true
}
