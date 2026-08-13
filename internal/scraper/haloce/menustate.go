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
	// FRESH every tick — no cached widget handle/block/path carried between ticks.
	// Read the heap and take the GLOBAL max-activation-tick HIGHLIGHTED widget (the
	// LIVE screen's item) and classify its DeLa path — NOT a scan limited to the
	// fixed menuItemPaths handles. That limited scan was the bug: CE never clears a
	// prior screen's +0x60 highlight, so at System Link Games the stale, still-
	// highlighted mp_submenu_system_link (a KNOWN item) won the known-scan and the
	// live \connected\server_list item (not a known item) was never even considered —
	// the runner mis-read the games browser as the entry item and pressed A forever.
	// The activation tick disambiguates: the live screen's widgets carry the HIGHEST
	// tick, so the global-max scan picks server_list over the stale entry block.
	heap, err := r.inst.Mem.ReadBytes(r.off.ConstUiWidgetHeapGVALo,
		int(r.off.ConstUiWidgetHeapGVAHi-r.off.ConstUiWidgetHeapGVALo))
	if err != nil {
		r.lastMenuDela = ""
		r.lastUIStats = UIHeapStats{}
		return MenuItemUnknown
	}
	// Cheap census of the already-read heap for the admin diagnostics panel — the
	// COLD-BOOT discriminator. A freshly-booted, un-interacted front end has only a
	// handful of live widget blocks; once the tree is built it jumps by an order of
	// magnitude (18 vs 168 measured on the rig). Purely diagnostic — nothing routes
	// on it yet; Stewart watches it live to tell us which signal is reliable.
	r.lastUIStats = uiHeapStats(heap)
	// Resolve the LIVE screen's highlighted ITEM (stamp-gated, item-kind-bit —
	// see rawHighlightPathFromHeap) and stash it for the admin diagnostics panel —
	// so the raw navfp `dela=` fingerprint is available on EVERY screen, including
	// System Link Games where the presence detection below short-circuits the
	// classification. "" = no live item, a VALID state (the whole 4way entry flow
	// and the pregame screen run with no item highlighted).
	path, _ := r.rawHighlightPathFromHeap(heap, r.lastRecStamp, r.lastRecStampOK)
	r.lastMenuDela = path
	// System Link Games — detect by PRESENCE of its games-list widget in the heap
	// (the screen with the "SYSTEM LINK GAMES" title + the list of hosted games), NOT
	// by which item is (stale-)highlighted. The list widget is live the whole time
	// you're on the screen even while a leftover mp_submenu_system_link block keeps
	// the +0x60 highlight flag — which is what made the runner read the games browser
	// as the entry item and press A forever. Presence → CREATE (Y).
	if r.systemLinkGamesActive(heap) {
		return MenuItemSystemLinkGames
	}
	return classifyMenuItemPath(path)
}

// systemLinkGamesActive reports whether a System Link games-browser widget is PRESENT
// (a valid, non-freed widget block for one of the \connected\server_list DeLa handles)
// in the UI heap — i.e. we're on that screen. Presence, not highlight: robust against
// the lagging highlighted-item read. False before entering System Link (the list
// widgets aren't instantiated on the submenu), and post-create the hosting cursor
// signal outranks it in Classify, so a lingering block can't misroute the map screen.
func (r *Reader) systemLinkGamesActive(heap []byte) bool {
	handles := r.systemLinkGamesHandles()
	if len(handles) == 0 {
		return false
	}
	for h := range handles {
		var hb [4]byte
		binary.LittleEndian.PutUint32(hb[:], h)
		for o := 0; ; {
			i := bytes.Index(heap[o:], hb[:])
			if i < 0 {
				break
			}
			off := o + i
			o = off + 1
			nb := off - int(OffUiWidgetDefTagHandle)
			if nb < 0 || nb+int(OffUiWidgetDefDataPtr)+4 > len(heap) {
				continue
			}
			if leU32Int(heap, nb)&ConstUiWidgetHeaderMask != ConstUiWidgetHeaderFlag {
				continue
			}
			if leU32Int(heap, nb+int(OffUiWidgetDefDataPtr)) == 0xFFFFFFFF {
				continue // freed block retaining a stale handle
			}
			return true // a live games-list widget block is present → on the screen
		}
	}
	return false
}

// systemLinkGamesHandles resolves + caches the 'DeLa' handles of every widget whose
// tag path contains \connected\server_list (the games-list container + its per-server
// items). Resolved by SUBSTRING (one tag-array walk), so it survives the item-index
// variance (server_list_item0/1/…) — we don't need to know which slot is highlighted,
// only that the list widget exists. Cached per UI session (cleared on menu entry).
func (r *Reader) systemLinkGamesHandles() map[uint32]bool {
	if r.sysLinkGamesHandles != nil {
		return r.sysLinkGamesHandles
	}
	out := map[uint32]bool{}
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
	for i := uint32(0); i < count; i++ {
		base := i * ConstTagEntrySize
		grp := []byte{blob[base+3], blob[base+2], blob[base+1], blob[base+0]}
		if string(grp) != tagGroupDela {
			continue
		}
		namePtr := leU32(blob, base+OffTagNamePtr)
		if namePtr < HighGVAThreshold {
			continue
		}
		if strings.Contains(r.readHighString(namePtr), sysLinkGamesPathMark) {
			out[leU32(blob, base+OffTagHandle)] = true
		}
	}
	if len(out) > 0 {
		r.sysLinkGamesHandles = out // cache only once at least one resolved
	}
	return out
}

// classifyMenuItemPath maps a highlighted widget's DeLa PATH to a MenuItem enum: an
// exact front-end item (menuItemPaths), the System Link games browser
// (…\connected\server_list\…), or Unknown.
func classifyMenuItemPath(path string) int {
	if path == "" {
		return MenuItemUnknown
	}
	if v, ok := menuItemPaths[path]; ok {
		return v
	}
	if strings.Contains(path, sysLinkGamesPathMark) {
		return MenuItemSystemLinkGames
	}
	return MenuItemUnknown
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

// UIHeapStats is a cheap census of the CE UI widget heap, surfaced to the admin
// diagnostics panel as COLD-BOOT / menu-state candidate signals. Diagnostic only —
// no runner logic routes on these; they exist so an operator can watch which value
// actually distinguishes a cold, un-woken front end from a built widget tree, and
// which one moves on a screen (like SELECT PROFILE) where the dela is static.
type UIHeapStats struct {
	// Blocks is the number of LIVE (valid header, non-freed) widget blocks. The
	// headline cold-vs-woken signal: a handful on a cold menu, an order of magnitude
	// more once the tree is built.
	Blocks int
	// Highlighted is how many of those carry the +0x60 highlight flag. Zero on a cold
	// menu with no selection; CE never clears stale flags, so it grows with depth.
	Highlighted int
	// MaxTick is the highest widget activation tick (+0x28) — the "UI tick". The
	// ACTIVE screen's widgets carry the highest value, so it advances as screens
	// change; 0 on a cold front end that has never activated a screen.
	MaxTick uint32
}

// uiHeapStats walks an already-read UI heap once and counts live/highlighted blocks
// plus the max activation tick. Same block validity rules as the other heap scans
// (header &0xFFFF0000 == 0x80000000, +0x14 != 0xFFFFFFFF).
func uiHeapStats(heap []byte) UIHeapStats {
	var st UIHeapStats
	limit := len(heap) - (int(OffUiWidgetHighlightFlag) + 4)
	for nb := 0; nb <= limit; nb += 4 {
		if leU32Int(heap, nb)&ConstUiWidgetHeaderMask != ConstUiWidgetHeaderFlag {
			continue
		}
		if leU32Int(heap, nb+int(OffUiWidgetDefDataPtr)) == 0xFFFFFFFF {
			continue
		}
		st.Blocks++
		if leU32Int(heap, nb+int(OffUiWidgetHighlightFlag)) == 1 {
			st.Highlighted++
		}
		if t := uint32(leU32Int(heap, nb+int(OffUiWidgetActivationTick))); t > st.MaxTick {
			st.MaxTick = t
		}
	}
	return st
}
