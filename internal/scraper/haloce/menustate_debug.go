package haloce

import (
	"log"
	"os"
)

// navDebug gates the nav-fingerprint diagnostic (PHASE 1). Set HOSTRUNNER_NAV_DEBUG=1
// on the box, walk Multiplayer → System Link Play → Select Profile → System Link
// Games, and beta.log gets one `navfp[...]` line per DISTINCT screen — the RAW DeLa
// widget path the classifier keys on, INCLUDING screens not in menuItemPaths (so we
// can finally tell Select Profile from System Link Games and wire the flow).
var navDebug = func() bool {
	v := os.Getenv("HOSTRUNNER_NAV_DEBUG")
	return v == "1" || v == "true"
}()

// logNavFingerprint dumps the raw front-end screen identity when it changes. Called
// from ReadGameState only while at a menu, and only when navDebug is on.
func (r *Reader) logNavFingerprint(conn uint16, mainMenu uint8, menuItem int, focus uint32) {
	// EVERY tick (not on-change): the raw FRESH-scan highlighted widget path + the
	// resolved MenuItem, so one capture while the box is HELD on System Link Games
	// shows exactly what the fresh scan returns there — confirming it now reads
	// \connected\server_list (→ menu_item=6, fixed) or still a stale entry item
	// (→ the scan itself is stale, needs a deeper fix). menu_item is what ReadMenuItem
	// classified this same fresh scan to.
	path, tick := r.menuFingerprint()
	log.Printf("navfp[%s]: dela=%q menu_item=%d conn=%d main_menu=%d focus=0x%08X tick=0x%X",
		r.name, path, menuItem, conn, mainMenu, focus, tick)
}

// menuFingerprint returns the DeLa tag PATH of the highest-activation-tick
// HIGHLIGHTED front-end widget — the raw screen identity, INCLUDING screens the
// classifier doesn't map (Select Profile vs System Link Games). "" when nothing
// recognisable is highlighted. Diagnostic only (one heap pass + one tag-array walk).
func (r *Reader) menuFingerprint() (string, uint32) {
	heap, err := r.inst.Mem.ReadBytes(r.off.ConstUiWidgetHeapGVALo,
		int(r.off.ConstUiWidgetHeapGVAHi-r.off.ConstUiWidgetHeapGVALo))
	if err != nil {
		return "", 0
	}
	return r.rawHighlightPathFromHeap(heap)
}

// rawHighlightPathFromHeap returns the DeLa PATH + activation tick of the highest-
// tick HIGHLIGHTED widget in an already-read UI heap. Split from menuFingerprint so
// ReadMenuItem can reuse its heap (no second 2 MiB read) to recognise screens the
// fixed menuItemPaths don't cover — the System Link games browser in particular.
func (r *Reader) rawHighlightPathFromHeap(heap []byte) (string, uint32) {
	// One linear pass: the max activation tick per HIGHLIGHTED (+0x60==1), non-freed
	// (+0x14!=0xFFFFFFFF), valid-header (&0xFFFF0000==0x80000000) widget block, keyed
	// by its +0x10 DeLa tag handle.
	ticks := map[uint32]uint32{}
	limit := len(heap) - (int(OffUiWidgetHighlightFlag) + 4)
	for nb := 0; nb <= limit; nb += 4 {
		if leU32Int(heap, nb)&ConstUiWidgetHeaderMask != ConstUiWidgetHeaderFlag {
			continue
		}
		if leU32Int(heap, nb+int(OffUiWidgetDefDataPtr)) == 0xFFFFFFFF {
			continue
		}
		if leU32Int(heap, nb+int(OffUiWidgetHighlightFlag)) != 1 {
			continue
		}
		h := leU32Int(heap, nb+int(OffUiWidgetDefTagHandle))
		if h == 0 {
			continue
		}
		if t := uint32(leU32Int(heap, nb+int(OffUiWidgetActivationTick))); t > ticks[h] {
			ticks[h] = t
		}
	}
	if len(ticks) == 0 {
		return "", 0
	}
	paths := r.delaPathsForHandles(ticks)
	best, bestTick, have := "", uint32(0), false
	for h, t := range ticks {
		if p, ok := paths[h]; ok && (!have || t > bestTick) {
			best, bestTick, have = p, t, true
		}
	}
	return best, bestTick
}

// delaPathsForHandles walks the cache tag array ONCE and returns the DeLa tag PATH
// for each requested handle (reverse of resolveDelaHandles' path→handle). Reads a
// name string only for entries whose handle is wanted, so it's cheap.
func (r *Reader) delaPathsForHandles(want map[uint32]uint32) map[uint32]string {
	out := make(map[uint32]string, len(want))
	th, err := r.inst.DerefLowPtr(r.off.AddrTagHeaderPtr)
	if err != nil || th < HighGVAThreshold {
		return out
	}
	mem := r.inst.Mem
	tagArray, err := mem.ReadU32(th + OffTagHeaderTagArray)
	if err != nil || tagArray < HighGVAThreshold {
		return out
	}
	count, err := mem.ReadU32(th + OffTagHeaderTagCount)
	if err != nil || count == 0 || count > 65535 {
		return out
	}
	blob, err := mem.ReadBytes(tagArray, int(count*ConstTagEntrySize))
	if err != nil {
		return out
	}
	for i := uint32(0); i < count && len(out) < len(want); i++ {
		base := i * ConstTagEntrySize
		grp := []byte{blob[base+3], blob[base+2], blob[base+1], blob[base+0]}
		if string(grp) != tagGroupDela {
			continue
		}
		h := leU32(blob, base+OffTagHandle)
		if _, ok := want[h]; !ok {
			continue
		}
		namePtr := leU32(blob, base+OffTagNamePtr)
		if namePtr < HighGVAThreshold {
			continue
		}
		out[h] = r.readHighString(namePtr)
	}
	return out
}
