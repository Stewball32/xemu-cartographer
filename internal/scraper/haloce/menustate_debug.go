package haloce

import (
	"fmt"
	"log"
	"os"
	"time"
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

// navFPHeartbeat re-logs an UNCHANGED fingerprint at this interval so a capture
// still proves liveness. On-change + heartbeat replaced the old per-tick dump:
// at the 100ms host sub-tick cadence an every-tick navfp was ~10 lines/sec of
// identical spam that buried the runner's decision log (beta.log 2026-08-10).
const navFPHeartbeat = 2 * time.Second

// logNavFingerprint dumps the raw front-end screen identity — on CHANGE of the
// classification-relevant fields, plus a slow heartbeat. Called from
// ReadGameState only while at a menu, and only when navDebug is on.
func (r *Reader) logNavFingerprint(conn uint16, mainMenu uint8, menuItem int, focus uint32, uiScreen string, uiBackRec uint32) {
	// The raw FRESH-scan highlighted widget path + the resolved MenuItem, so one
	// capture while the box is HELD on System Link Games shows exactly what the
	// fresh scan returns there. menu_item is what ReadMenuItem classified this
	// same fresh scan to.
	heap, err := r.inst.Mem.ReadBytes(r.off.ConstUiWidgetHeapGVALo,
		int(r.off.ConstUiWidgetHeapGVAHi-r.off.ConstUiWidgetHeapGVALo))
	if err != nil {
		log.Printf("navfp[%s]: heap read err: %v", r.name, err)
		return
	}
	path, tick := r.rawHighlightPathFromHeap(heap, r.lastRecStamp, r.lastRecStampOK)
	serverlist := r.systemLinkGamesActive(heap)
	// Change key: everything EXCEPT focus and the activation tick — focus relinks
	// ambiently on the cold menu (beta.log 2026-08-10: a 0x800465x8 ring cycling
	// with no input), so keying on it would defeat the on-change gate. The logged
	// line still carries the live focus value.
	fp := fmt.Sprintf("%q|%q|0x%X|%d|%v|%d|%d", path, uiScreen, uiBackRec, menuItem, serverlist, conn, mainMenu)
	now := time.Now()
	if fp == r.lastNavFP && now.Sub(r.lastNavFPAt) < navFPHeartbeat {
		return
	}
	r.lastNavFP = fp
	r.lastNavFPAt = now
	// serverlist=true means the games-list widget is PRESENT (we're on System Link
	// Games) — this is what drives menu_item=6 even when dela (the highlighted item)
	// still reads the stale entry item. screen= / back= are the SCREEN-RECORD reads
	// (screenrec.go): the resolved current-screen path and the back-screen record
	// (0 exactly at the root menu) — logged side-by-side with the heap dela so a
	// live walk verifies the two classifiers agree per screen.
	log.Printf("navfp[%s]: dela=%q screen=%q back=0x%X menu_item=%d serverlist=%v conn=%d main_menu=%d focus=0x%08X tick=0x%X",
		r.name, path, uiScreen, uiBackRec, menuItem, serverlist, conn, mainMenu, focus, tick)
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
	return r.rawHighlightPathFromHeap(heap, r.lastRecStamp, r.lastRecStampOK)
}

// widgetCand is one highlighted widget's (activation tick, kind flags) pair,
// keyed by DeLa handle in the candidate map the highlight pick runs over.
type widgetCand struct {
	Tick uint32
	Kind uint32
}

// rawHighlightPathFromHeap returns the DeLa PATH + activation tick of the LIVE
// screen's highlighted ITEM widget in an already-read UI heap. Split from
// menuFingerprint so ReadMenuItem can reuse its heap (no second 2 MiB read).
//
// Pick rules (docs/MENU-ENTRYFLOW-2026-08-11.md §7, live-verified):
//
//   - STAMP GATE: only blocks whose activation tick (+0x28) equals the current
//     screen record's stamp (rec+0x18) belong to the ACTIVE screen — the
//     invariant holds on every capture including cold boot (both 0). This is
//     what drops stale prior-screen blocks: on the pregame screen the max-tick
//     highlighted block is a DECORATION (xbox_graphic), so any max-tick scheme
//     falls through to a stale item there. Without a readable stamp the max
//     tick tier is the fallback.
//   - ITEM KIND BIT: +0x64 & 0x20000 is set on every selectable *_item widget
//     and on no decoration — it replaces the lexicographic guesswork between
//     e.g. mp_map_center_pic (permanent hi=1 decoration) and the real item.
//   - ZERO surviving items is a VALID result ("no live item" — true for the
//     whole 4way entry flow and the pregame screen): return "", never fall back
//     to a stale or decoration block. Remaining multi-item ties (old+new item
//     for a tick during a move) break lexicographically for determinism.
func (r *Reader) rawHighlightPathFromHeap(heap []byte, stamp uint32, stampOK bool) (string, uint32) {
	// One linear pass: (tick, kind) per HIGHLIGHTED (+0x60==1), non-freed
	// (+0x14!=0xFFFFFFFF), valid-header (&0xFFFF0000==0x80000000) widget block,
	// keyed by its +0x10 DeLa tag handle. Stamp-gated inline when readable.
	cands := map[uint32]widgetCand{}
	limit := len(heap) - (int(OffUiWidgetKindFlags) + 4)
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
		t := uint32(leU32Int(heap, nb+int(OffUiWidgetActivationTick)))
		if stampOK && t != stamp {
			continue // stale prior-screen block
		}
		k := uint32(leU32Int(heap, nb+int(OffUiWidgetKindFlags)))
		if c, ok := cands[h]; !ok || t > c.Tick {
			cands[h] = widgetCand{Tick: t, Kind: k}
		}
	}
	if len(cands) == 0 {
		return "", 0
	}
	want := make(map[uint32]uint32, len(cands))
	for h, c := range cands {
		want[h] = c.Tick
	}
	return pickHighlightPath(cands, r.delaPathsForHandles(want), stampOK)
}

// pickHighlightPath chooses the live screen's highlighted ITEM deterministically
// from stamp-gated candidates: item-kind blocks only (+0x64 & 0x20000), highest
// tick tier when no stamp gated the set, lexicographic among remaining ties.
// ("", 0) when no item survives — a valid state, not a failure. Pure so the
// rules are unit-testable.
func pickHighlightPath(cands map[uint32]widgetCand, paths map[uint32]string, stampOK bool) (string, uint32) {
	// Without a stamp gate, restrict to the max-tick tier first (the live
	// screen's shared activation tick).
	var maxTick uint32
	if !stampOK {
		for _, c := range cands {
			if c.Tick > maxTick {
				maxTick = c.Tick
			}
		}
	}
	best, bestTick, have := "", uint32(0), false
	for h, c := range cands {
		if !stampOK && c.Tick != maxTick {
			continue
		}
		if c.Kind&ConstUiWidgetItemKindBit == 0 {
			continue // decoration / header / text — never the routed item
		}
		p, ok := paths[h]
		if !ok {
			continue
		}
		if !have || p < best {
			best, bestTick, have = p, c.Tick, true
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
