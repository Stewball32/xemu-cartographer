package haloce

import (
	"bytes"
	"encoding/binary"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// widget.go ports the CE UI widget-instance read (halo-offset-mapper
// scripts/runtime/ce_widget.py + docs/ce-menu-system-2026-07-11.md) into Go: the
// LIVE highlighted-card index of any menu list/spinner, resolved by scanning the
// per-screen UI heap for the widget block whose +0x10 == the list's 'DeLa' tag
// handle and reading +0x4C (selected) / +0x54 (count).
//
// This is what closes the create-game map/gametype navigation loop: with the live
// cursor readable, the host-runner drives the carousel cursor-relative
// (presses = (target − cursor) mod count) and re-reads +0x4C == target to confirm
// it landed before pressing A — no assumed default, no card-pool scroll walk, and
// gametype custom-variant offsets handled natively.
//
// Read path: the UI heap lives at high GVA 0x81E00000..0x82000000, which QMP
// memsave can't reach (absent from the page tables) but which cartographer's
// Mem.HighGVA reads directly out of /proc/<pid>/mem at base(gpa2hva 0)+(gva−
// 0x80000000) — the same physical read ce_widget.py does. So a plain
// mem.ReadBytes(ConstUiWidgetHeapGVALo, …) over the window is all that's needed.

// ReadLobbyCursor reads the LIVE SELECT MAP and SELECT GAMETYPE carousel cursors
// via the widget system. Implements scraper.LobbyCursorReader.
//
// One heap read serves both lists: the resolved 'DeLa' tag handles are cached per
// front-end session (invalidated on entry to menu by OnStateChange), and the
// ~2 MiB UI-heap window is read once per call and scanned for both handles.
//
// Returns a zero-Valid LobbyCursor when the create-game screens aren't up (widget
// not found or count 0) or the reads fail, so the caller holds rather than
// navigating on a stale/garbage index. Loop-goroutine only (mutates the handle
// cache, reads r.inst) — same discipline as the other GameReader reads.
func (r *Reader) ReadLobbyCursor() scraper.LobbyCursor {
	handles := r.lobbyCursorDelaHandles()
	mapHandle := handles[r.off.TagPathMPMapSelectList]
	gtHandle := handles[r.off.TagPathGametypeSelectList]
	if mapHandle == 0 && gtHandle == 0 {
		return scraper.LobbyCursor{}
	}

	heap, err := r.inst.Mem.ReadBytes(r.off.ConstUiWidgetHeapGVALo, int(r.off.ConstUiWidgetHeapGVAHi-r.off.ConstUiWidgetHeapGVALo))
	if err != nil {
		return scraper.LobbyCursor{}
	}

	// Two signals per list: Count>0 means the list widget is UP (we're ON the SELECT
	// MAP / SELECT GAMETYPE screen) — stable across a scroll animation and the
	// reliable "which screen" signal for Classify. Valid additionally requires the
	// selected index to be settled in range (liveIndex): only then is the cursor
	// safe to navigate on. During a scroll the +0x4C index is briefly out of range,
	// so Valid drops for a tick while Count stays put — the runner holds that tick
	// (re-reads) but never mistakes the card screen for the settled lobby.
	var cur scraper.LobbyCursor
	if mapHandle != 0 {
		if sel, count, ok := findWidgetSelection(heap, mapHandle); ok && count > 0 {
			cur.MapCount = count
			if liveIndex(sel, count) {
				cur.MapIndex, cur.MapValid = sel, true
			}
		}
	}
	if gtHandle != 0 {
		if sel, count, ok := findWidgetSelection(heap, gtHandle); ok && count > 0 {
			cur.GametypeCount = count
			if liveIndex(sel, count) {
				cur.GametypeIndex, cur.GametypeValid = sel, true
			}
		}
	}
	return cur
}

// liveIndex reports whether a widget's (sel, count) read is a settled, live
// cursor: the list is active (count > 0) AND the selected index is in range
// [0, count). During a carousel SCROLL ANIMATION the +0x4C field briefly holds an
// out-of-range intermediate value (the CE analog of H2's render-copy settle
// window); rejecting those makes ReadLobbyCursor report the cursor invalid so the
// runner holds that tick and re-reads once the scroll settles, rather than
// navigating on a garbage index.
func liveIndex(sel, count int) bool {
	return count > 0 && sel >= 0 && sel < count
}

// lobbyCursorDelaHandles returns the SELECT MAP / SELECT GAMETYPE list widgets'
// 'DeLa' tag handles keyed by tag path, resolving+caching them on first use. The
// handles are stable within a loaded UI cache (front-end session) and re-resolved
// after each return to the front-end (OnStateChange clears the cache). Returns a
// map with zero-valued (missing) entries when the tags aren't loaded yet, and does
// NOT cache a failed resolve so a transient miss retries next call.
func (r *Reader) lobbyCursorDelaHandles() map[string]uint32 {
	if r.lobbyCursorHandles != nil {
		return r.lobbyCursorHandles
	}
	resolved := r.resolveDelaHandles(r.off.TagPathMPMapSelectList, r.off.TagPathGametypeSelectList)
	// Only cache once at least one handle resolved — otherwise the tags aren't
	// loaded yet and we want to retry on the next call (cheap tag-array walk).
	if resolved[r.off.TagPathMPMapSelectList] != 0 || resolved[r.off.TagPathGametypeSelectList] != 0 {
		r.lobbyCursorHandles = resolved
	}
	return resolved
}

// resolveDelaHandles walks the cache tag array ONCE and returns, for each
// requested tag path, the 'DeLa' (ui_widget_definition) tag's HANDLE (+0x0C in
// the entry) — or 0 if not found / not a DeLa tag. Mirrors findUstrTagData's
// single-pass structure: one bulk read of the array, group filter before any
// per-entry name deref, exact path match.
func (r *Reader) resolveDelaHandles(paths ...string) map[string]uint32 {
	out := make(map[string]uint32, len(paths))
	for _, p := range paths {
		out[p] = 0
	}

	tagHeader, err := r.inst.DerefLowPtr(r.off.AddrTagHeaderPtr)
	if err != nil || tagHeader < HighGVAThreshold {
		return out
	}
	mem := r.inst.Mem
	tagArray, err := mem.ReadU32(tagHeader + OffTagHeaderTagArray)
	if err != nil || tagArray < HighGVAThreshold {
		return out
	}
	count, err := mem.ReadU32(tagHeader + OffTagHeaderTagCount)
	if err != nil || count == 0 || count > 65535 {
		return out
	}

	blob, err := mem.ReadBytes(tagArray, int(count*ConstTagEntrySize))
	if err != nil {
		return out
	}

	remaining := len(paths)
	for i := uint32(0); i < count && remaining > 0; i++ {
		base := i * ConstTagEntrySize
		// Group code is stored byte-reversed; reverse the 4 bytes to compare.
		g := []byte{blob[base+3], blob[base+2], blob[base+1], blob[base+0]}
		if string(g) != tagGroupDela {
			continue
		}
		namePtr := leU32(blob, base+OffTagNamePtr)
		if namePtr < HighGVAThreshold {
			continue
		}
		name := r.readHighString(namePtr)
		if _, wanted := out[name]; !wanted || out[name] != 0 {
			continue
		}
		out[name] = leU32(blob, base+OffTagHandle)
		remaining--
	}
	return out
}

// findWidgetSelection scans the UI-heap buffer for the widget block whose
// +0x10 == handle and returns its +0x4C selected index and +0x54 item count. When
// more than one block carries the handle (a freed block can retain a stale +0x10),
// the ACTIVE one (count > 0) wins — matching ce_widget.py's "prefer count>0". A hit
// is validated by the block header (hdr & 0xFFFF0000 == 0x80000000). heap starts at
// GVA ConstUiWidgetHeapGVALo. Returns ok=false when no valid block matches.
//
// The handle bytes are found with bytes.Index (byte-granular, exactly like
// ce_widget.py) so no allocation-alignment assumption can make it miss the live
// block during a screen transition (where a freed + a fresh block briefly coexist).
func findWidgetSelection(heap []byte, handle uint32) (sel, count int, ok bool) {
	var hb [4]byte
	binary.LittleEndian.PutUint32(hb[:], handle)
	hoff := int(OffUiWidgetDefTagHandle)
	for o := 0; ; {
		i := bytes.Index(heap[o:], hb[:])
		if i < 0 {
			break
		}
		off := o + i
		o = off + 1 // advance one byte so overlapping matches aren't skipped
		nb := off - hoff
		if nb < 0 || nb+int(OffUiWidgetItemCount)+4 > len(heap) {
			continue
		}
		if leU32Int(heap, nb)&ConstUiWidgetHeaderMask != ConstUiWidgetHeaderFlag {
			continue
		}
		s := int(int32(leU32Int(heap, nb+int(OffUiWidgetSelectedIndex))))
		c := int(int32(leU32Int(heap, nb+int(OffUiWidgetItemCount))))
		// First valid hit seeds the result; a later ACTIVE (count>0) block, or a
		// higher active count, overrides it (the live instance wins over a freed
		// block that kept a stale +0x10 handle).
		if !ok || (c > 0 && c > count) {
			sel, count, ok = s, c, true
		}
	}
	return sel, count, ok
}

// leU32Int reads a little-endian uint32 from b at off (int offset), returning 0
// when out of range. Companion to leU32 (uint32 offset) in enumerate.go.
func leU32Int(b []byte, off int) uint32 {
	if off < 0 || off+4 > len(b) {
		return 0
	}
	return uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
}
