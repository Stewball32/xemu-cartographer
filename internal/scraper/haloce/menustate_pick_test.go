package haloce

import "testing"

// pickHighlightPath must be DETERMINISTIC and item-only (mapper §7,
// docs/MENU-ENTRYFLOW-2026-08-11.md): the +0x64 item-kind bit (0x20000) is set
// on every selectable *_item widget and no decoration, and "no surviving item"
// is a VALID result — never fall back to a stale or decoration block.
func TestPickHighlightPathItemKind(t *testing.T) {
	const (
		pathCenterPic = `ui\shell\main_menu\multiplayer_type_select\mp_map_select\mp_map_center_pic`
		pathLeftItem  = `ui\shell\main_menu\multiplayer_type_select\mp_map_select\mp_map_left_item`
		pathMPItem    = `ui\shell\main_menu\main_menu_item_multiplayer`
		pathXboxPic   = `ui\shell\main_menu\multiplayer_type_select\connected\pregame\xbox_graphic`
		kindItem      = ConstUiWidgetItemKindBit
		kindPic       = uint32(0x250000)
	)

	// Item beats a same-tick permanently-highlighted decoration — every read
	// (the live mp_map_left_item ↔ mp_map_center_pic flicker).
	cands := map[uint32]widgetCand{
		1: {Tick: 100, Kind: kindPic},  // decoration
		2: {Tick: 100, Kind: kindItem}, // real item
	}
	paths := map[uint32]string{1: pathCenterPic, 2: pathLeftItem}
	for i := 0; i < 64; i++ {
		if p, tick := pickHighlightPath(cands, paths, true); p != pathLeftItem || tick != 100 {
			t.Fatalf("read %d: got %q (tick 0x%X), want the item-kind widget", i, p, tick)
		}
	}

	// ZERO items is a VALID state (whole 4way flow, pregame): decorations alone
	// yield "", never a fallback pick.
	decoOnly := map[uint32]widgetCand{1: {Tick: 100, Kind: kindPic}}
	if p, _ := pickHighlightPath(decoOnly, map[uint32]string{1: pathXboxPic}, true); p != "" {
		t.Fatalf("decorations only must report no live item, got %q", p)
	}

	// The pregame regression (no stamp available): the max-tick tier holds only a
	// DECORATION while a STALE item sits below — must report no live item, not
	// the stale item.
	pregame := map[uint32]widgetCand{
		1: {Tick: 200, Kind: kindPic},  // live screen's xbox_graphic
		2: {Tick: 100, Kind: kindItem}, // stale prior-screen item
	}
	if p, _ := pickHighlightPath(pregame, map[uint32]string{1: pathXboxPic, 2: pathMPItem}, false); p != "" {
		t.Fatalf("stale item below the live tier must not win, got %q", p)
	}

	// Two same-tick items (old+new during a move): lexicographic, stable.
	twoItems := map[uint32]widgetCand{
		1: {Tick: 100, Kind: kindItem},
		2: {Tick: 100, Kind: kindItem},
	}
	paths2 := map[uint32]string{1: pathLeftItem, 2: pathMPItem}
	want := pathMPItem // "…main_menu_item_multiplayer" < "…multiplayer_type_select…"
	for i := 0; i < 64; i++ {
		if p, _ := pickHighlightPath(twoItems, paths2, true); p != want {
			t.Fatalf("read %d: got %q, want stable lexicographic pick %q", i, p, want)
		}
	}

	// No resolvable paths → empty.
	if p, tick := pickHighlightPath(map[uint32]widgetCand{9: {Tick: 5, Kind: kindItem}}, map[uint32]string{}, true); p != "" || tick != 0 {
		t.Fatalf("unresolvable set should yield empty, got %q/0x%X", p, tick)
	}
}
