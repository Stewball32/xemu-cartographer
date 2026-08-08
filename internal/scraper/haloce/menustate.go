package haloce

import (
	"bytes"
	"encoding/binary"
	"strings"
)

// Menu-item classification for the host-runner's STATE-AWARE front-end
// navigation. The runner used to drive the CE main menu → System Link with a
// wrap-prone blind key COUNT that mis-landed (A on Single Player → Campaign).
// Instead we read WHICH menu item is currently highlighted, from the CE UI
// widget heap, and navigate by item IDENTITY.
//
// How (RE'd + verified live on H1 Perf, 2026-08-07): every front-end menu item
// is a ui_widget_definition ('DeLa') widget block in the UI heap
// (ConstUiWidgetHeapGVALo..Hi). The HIGHLIGHTED item's block has +0x60 == 1
// (OffUiWidgetHighlightFlag); its +0x10 tag handle resolves (via the cache tag
// array) to a DeLa PATH like `…\main_menu_item_multiplayer`. We classify that
// path into a small enum the planner routes on.
//
// STALE-BLOCK disambiguation via the activation tick: CE does NOT clear a prior
// screen's +0x60 flags when you move to another screen, so on the main menu a
// leftover multiplayer-submenu block can still read +0x60 == 1 alongside the
// real main_menu_item_multiplayer highlight. A deepest-first PRIORITY order was
// tried and proved WRONG (it let a stale Coop block outrank the live
// Multiplayer). The fix is OffUiWidgetActivationTick (+0x28): every widget of a
// screen shares one tick stamped when that screen was (re)activated, and the
// ACTIVE screen's tick is the HIGHEST. So among all highlighted item blocks we
// pick the one with the MAX tick — the live screen's — and classify THAT. A mod
// that renames the tags degrades to MenuItemUnknown → the planner Back-normalises.
const (
	MenuItemUnknown      = 0 // no recognised front-end item highlighted
	MenuItemMainOther    = 1 // main menu, a non-Multiplayer item (Single Player / Settings / …)
	MenuItemMultiplayer  = 2 // main menu, MULTIPLAYER highlighted
	MenuItemSubmenuOther = 3 // Multiplayer submenu, a non-System-Link item (Coop / Split / Gametypes)
	MenuItemSystemLink   = 4 // Multiplayer submenu, SYSTEM LINK (conn) highlighted
	MenuItemProfile      = 5 // SELECT PROFILE screen (join / pick / all-ready)
	// MenuItemSystemLinkGames is the System Link GAMES BROWSER — the screen where
	// the host presses Y to CREATE. Its highlighted widget is a per-server list item
	// (…\connected\server_list\server_list_itemN, RE'd from a networked box), so it's
	// matched by PATH SUBSTRING, not the fixed menuItemPaths. Recognising it from the
	// RELIABLE high-GVA widget heap lets the runner create off the widget instead of
	// the stale-prone game_connection low global (which reads 0 here in the fast loop).
	MenuItemSystemLinkGames = 6
)

// sysLinkGamesPathMark identifies the System Link games-browser screen by a stable
// substring of its highlighted widget's DeLa path. The trailing item index varies
// (server_list_item0/1/…), so we match the parent path, not an exact tag.
const sysLinkGamesPathMark = `\connected\server_list`

// menuItemPaths maps the front-end item DeLa tag paths to their enum. These are
// the standard CE `ui\shell\main_menu` widget tags (verified present on H1 Perf);
// a mod that renames them degrades to MenuItemUnknown → the planner falls back to
// Back-normalisation, never a wrong press.
var menuItemPaths = map[string]int{
	`ui\shell\main_menu\main_menu_item_multiplayer`:                              MenuItemMultiplayer,
	`ui\shell\main_menu\main_menu_item_load_camp`:                                MenuItemMainOther,
	`ui\shell\main_menu\main_menu_item_settings`:                                 MenuItemMainOther,
	`ui\shell\main_menu\main_menu_item_game_demos`:                               MenuItemMainOther,
	`ui\shell\main_menu\multiplayer_type_select\multiplayer_type_conn_item`:      MenuItemSystemLink,
	`ui\shell\main_menu\multiplayer_type_select\multiplayer_type_coop_item`:      MenuItemSubmenuOther,
	`ui\shell\main_menu\multiplayer_type_select\multiplayer_type_split_item`:     MenuItemSubmenuOther,
	`ui\shell\main_menu\multiplayer_type_select\multiplayer_type_gametypes_item`: MenuItemSubmenuOther,
	`ui\shell\main_menu\player_profiles_select\player_profile_left_item`:         MenuItemProfile,
	`ui\shell\main_menu\player_profiles_select\player_profile_right_item`:        MenuItemProfile,
}

func menuItemPathList() []string {
	out := make([]string, 0, len(menuItemPaths))
	for p := range menuItemPaths {
		out = append(out, p)
	}
	return out
}

// ReadMenuItem classifies the currently-highlighted front-end menu item into a
// MenuItem* enum by reading the UI widget heap. Among ALL highlighted item
// blocks (there may be stale ones from prior screens), it returns the enum of
// the block with the highest OffUiWidgetActivationTick — the live screen's item.
// MenuItemUnknown when nothing recognised is highlighted (off-route screen,
// non-menu state, or a mod that renamed the tags). Cheap-ish: one bulk heap read
// + a cached tag-handle map.
func (r *Reader) ReadMenuItem() int {
	handles := r.menuItemDelaHandles()
	if len(handles) == 0 {
		return MenuItemUnknown
	}
	heap, err := r.inst.Mem.ReadBytes(r.off.ConstUiWidgetHeapGVALo,
		int(r.off.ConstUiWidgetHeapGVAHi-r.off.ConstUiWidgetHeapGVALo))
	if err != nil {
		return MenuItemUnknown
	}
	best := MenuItemUnknown
	var bestTick uint32
	haveBest := false
	for path, handle := range handles {
		if handle == 0 {
			continue
		}
		tick, ok := widgetHighlightTick(heap, handle)
		if !ok {
			continue
		}
		if !haveBest || tick > bestTick {
			haveBest = true
			bestTick = tick
			best = menuItemPaths[path]
		}
	}
	// None of the FIXED front-end items is highlighted — but this may be the System
	// Link games browser, whose highlighted widget (…\connected\server_list\…) isn't
	// in menuItemPaths. Reuse the heap we already read to grab the raw highlighted
	// path and recognise it, so the runner can CREATE (Y) off this reliable high-GVA
	// widget rather than the stale-prone game_connection low global.
	if best == MenuItemUnknown {
		if path, _ := r.rawHighlightPathFromHeap(heap); strings.Contains(path, sysLinkGamesPathMark) {
			return MenuItemSystemLinkGames
		}
	}
	return best
}

// menuItemDelaHandles resolves + caches the front-end item DeLa tag handles. The
// cache shares the lobby-cursor cache's lifecycle (invalidated on menu entry,
// where the UI cache reloads) — see reader_cache.go.
func (r *Reader) menuItemDelaHandles() map[string]uint32 {
	if r.menuItemHandles != nil {
		return r.menuItemHandles
	}
	h := r.resolveDelaHandles(menuItemPathList()...)
	for _, v := range h {
		if v != 0 {
			r.menuItemHandles = h // cache only once at least one resolved
			break
		}
	}
	return h
}

// widgetHighlightTick finds the (non-freed) UI widget block carrying this DeLa
// tag handle at +0x10 whose highlight flag (+0x60) is set, and returns its
// activation tick (+0x28). ok is false when no highlighted block for this handle
// exists. Byte-granular handle scan + header validation, mirroring
// findWidgetSelection; skips freed blocks (+0x14 == 0xFFFFFFFF) so a stale flag
// can't false-positive. When more than one block for the SAME handle is
// highlighted (a duplicate stale copy), the highest tick wins — same rule
// ReadMenuItem applies across handles.
func widgetHighlightTick(heap []byte, handle uint32) (uint32, bool) {
	var hb [4]byte
	binary.LittleEndian.PutUint32(hb[:], handle)
	hoff := int(OffUiWidgetDefTagHandle)
	var bestTick uint32
	found := false
	for o := 0; ; {
		i := bytes.Index(heap[o:], hb[:])
		if i < 0 {
			break
		}
		off := o + i
		o = off + 1
		nb := off - hoff
		if nb < 0 || nb+int(OffUiWidgetHighlightFlag)+4 > len(heap) {
			continue
		}
		if leU32Int(heap, nb)&ConstUiWidgetHeaderMask != ConstUiWidgetHeaderFlag {
			continue
		}
		if leU32Int(heap, nb+int(OffUiWidgetDefDataPtr)) == 0xFFFFFFFF {
			continue // freed block retaining a stale handle
		}
		if leU32Int(heap, nb+int(OffUiWidgetHighlightFlag)) != 1 {
			continue
		}
		tick := uint32(leU32Int(heap, nb+int(OffUiWidgetActivationTick)))
		if !found || tick > bestTick {
			bestTick = tick
			found = true
		}
	}
	return bestTick, found
}
