package haloce

import (
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/xbox"
)

// Create-game carousel source tags. The SELECT MAP and SELECT GAMETYPE screens
// render their cards from these 'ustr' (unicode_string_list) tags, loaded from
// ui.map while the front-end is up. Matching by path keeps the enumeration
// build-agnostic: a modded disc's own mp_map_list / gametype names flow through
// unchanged (nothing about the map/gametype SET is hardcoded).
const (
	// TagPathMPMapList holds the multiplayer map display names, in SELECT MAP
	// carousel order. May carry trailing non-selectable placeholder entries
	// ("Unknown Level", " ") past the real maps — trimmed via TagPathMPMapDescriptions.
	TagPathMPMapList = `ui\shell\main_menu\mp_map_list`
	// TagPathMPMapDescriptions holds one description per SELECTABLE map, so its
	// element count is the authoritative selectable-map count.
	TagPathMPMapDescriptions = `ui\shell\main_menu\multiplayer_type_select\mp_map_select\map_data`
	// TagPathGameSettingNames holds the built-in multiplayer gametype variant
	// names, in SELECT GAMETYPE carousel order.
	TagPathGameSettingNames = `ui\default_multiplayer_game_setting_names`

	// tagGroupUstr is the 'ustr' tag group as stored in the tag entry (the
	// 4-char group code is written byte-REVERSED). Used to skip non-ustr tags
	// during the tag-array walk before dereferencing their data as a string list.
	tagGroupUstr = "ustr"
)

// EnumerateLobby reads the create-game MAP and GAMETYPE carousels from the loaded
// UI tags and returns them as ordered options. Implements scraper.LobbyEnumerator.
//
// Sources (all reached via the cache tag header AddrTagHeaderPtr → tag-array walk):
//   - maps: TagPathMPMapList, truncated to the TagPathMPMapDescriptions count so
//     trailing placeholder entries aren't offered.
//   - gametypes: TagPathGameSettingNames (the built-in variant set).
//
// The returned options are in carousel order and Steps carries each card's ABSOLUTE
// 0-based carousel index (position), NOT a press count. Navigation is CURSOR-RELATIVE
// and must be computed at nav time — presses = (targetIndex − liveCursorIndex) mod
// count — because the carousel does NOT start on a fixed card (RUNTIME-VERIFIED
// 2026-07-10: a fresh SELECT MAP entry landed on Chill Out, not the earlier Blood
// Gulch nor the last-committed map; the start is non-deterministic / remembers the
// last-used). See docs/ce-mapselect-2026-07-10.md.
//
// The live cursor read differs per carousel:
//   - GAMETYPE: readable via the card-render pool (0x2FAF84) — walk-until-target
//     (halo-offset-mapper scripts/runtime/ce_menu.py goto) navigates correctly from
//     any start (RUNTIME-VERIFIED 2026-07-10). This path also surfaces user-SAVED
//     custom variants (which prepend to the built-ins and are NOT in the ustr tag).
//   - MAP: NO memory-readable cursor and NO navigation feedback exist (the card pool
//     never updates for maps; the widget focus is a relinked heap ring with no clean
//     index; render buffers are unanchored). Cursor-relative map navigation from an
//     arbitrary start is therefore NOT achievable from memory — an open blocker.
//
// So this method delivers the LIST (the player-picker's real per-instance options);
// wiring the cursor-relative navigation is the remaining work (gametype: card-pool
// walk; map: blocked on the CE menu-cursor wall).
//
// Returns Available=false (empty lists) when the tags aren't loaded — i.e. the
// game isn't at/past the front-end (mid-match), so the caller keeps its last
// successful enumeration rather than clobbering it with an empty set.
func (r *Reader) EnumerateLobby() scraper.LobbyOptions {
	metas := r.findUstrTagData(r.off.TagPathMPMapList, r.off.TagPathMPMapDescriptions, r.off.TagPathGameSettingNames)

	mapNames := r.parseUnicodeStringList(metas[r.off.TagPathMPMapList])
	mapDescs := r.parseUnicodeStringList(metas[r.off.TagPathMPMapDescriptions])
	gtNames := r.parseUnicodeStringList(metas[r.off.TagPathGameSettingNames])

	maps := buildMapOptions(mapNames, len(mapDescs))
	gametypes := buildGametypeOptions(gtNames)

	return scraper.LobbyOptions{
		Available: len(maps) > 0 || len(gametypes) > 0,
		Maps:      maps,
		Gametypes: gametypes,
	}
}

// findUstrTagData walks the cache tag array ONCE and returns, for each requested
// path, the tag's data (meta) pointer — or 0 if not found / not a 'ustr' tag. One
// pass keeps the ~1000-entry walk cheap: the group code is checked from a single
// bulk read of the array before any per-entry name string is dereferenced.
func (r *Reader) findUstrTagData(paths ...string) map[string]uint32 {
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

	// Bulk-read the whole tag array (stride ConstTagEntrySize) so the group filter
	// and pointer fields come from one pread rather than count*3 small reads.
	blob, err := mem.ReadBytes(tagArray, int(count*ConstTagEntrySize))
	if err != nil {
		return out
	}

	remaining := len(paths)
	for i := uint32(0); i < count && remaining > 0; i++ {
		base := i * ConstTagEntrySize
		// Group code is stored byte-reversed; reverse the 4 bytes to compare.
		g := []byte{blob[base+3], blob[base+2], blob[base+1], blob[base+0]}
		if string(g) != tagGroupUstr {
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
		out[name] = leU32(blob, base+OffTagDataPtr)
		remaining--
	}
	return out
}

// parseUnicodeStringList reads a 'ustr' tag's strings block into an ordered slice
// of display names. meta is the tag data pointer (OffTagDataPtr). Returns nil on
// any malformed / out-of-range link so a mis-resolved tag never produces garbage.
func (r *Reader) parseUnicodeStringList(meta uint32) []string {
	if meta < HighGVAThreshold {
		return nil
	}
	mem := r.inst.Mem
	count, err := mem.ReadU32(meta + OffUstrCount)
	if err != nil || count == 0 || count > MaxUstrElemCount {
		return nil
	}
	arr, err := mem.ReadU32(meta + OffUstrArrayPtr)
	if err != nil || arr < HighGVAThreshold {
		return nil
	}
	// One bulk read of the element array, then per-element inline-text reads.
	elems, err := mem.ReadBytes(arr, int(count*ConstUstrElemSize))
	if err != nil {
		return nil
	}
	out := make([]string, 0, count)
	for k := uint32(0); k < count; k++ {
		base := k * ConstUstrElemSize
		size := leU32(elems, base+OffUstrElemSize)
		textPtr := leU32(elems, base+OffUstrElemTextPtr)
		name := ""
		if size > 0 && size <= MaxUstrTextBytes && textPtr >= HighGVAThreshold {
			if b, err := mem.ReadBytes(textPtr, int(size)); err == nil {
				name = xbox.DecodeUTF16LE(b)
			}
		}
		out = append(out, name)
	}
	return out
}

// buildMapOptions turns the raw mp_map_list names into carousel-ordered options.
// selectableCount (the map_data description count) drops trailing non-selectable
// placeholder entries; a zero/over-count value means "descriptions unavailable"
// and we fall back to dropping trailing blank names only.
func buildMapOptions(names []string, selectableCount int) []scraper.LobbyOption {
	if selectableCount > 0 && selectableCount < len(names) {
		names = names[:selectableCount]
	}
	names = trimTrailingBlank(names)
	return buildOptions(names)
}

// buildGametypeOptions turns the built-in gametype variant names into carousel-
// ordered options.
func buildGametypeOptions(names []string) []scraper.LobbyOption {
	return buildOptions(trimTrailingBlank(names))
}

// buildOptions assigns each name its ABSOLUTE 0-based carousel index (position) as
// Steps. This is NOT a press count — the carousel start is non-deterministic, so the
// runner must compute presses cursor-relative at nav time (see EnumerateLobby).
func buildOptions(names []string) []scraper.LobbyOption {
	if len(names) == 0 {
		return nil
	}
	out := make([]scraper.LobbyOption, 0, len(names))
	for i, nm := range names {
		out = append(out, scraper.LobbyOption{Name: nm, Steps: i})
	}
	return out
}

// trimTrailingBlank drops trailing empty/whitespace-only names (the mp_map_list
// pads its tail with a " " entry). Interior entries are preserved so Steps stay
// aligned to the live carousel.
func trimTrailingBlank(names []string) []string {
	end := len(names)
	for end > 0 && strings.TrimSpace(names[end-1]) == "" {
		end--
	}
	return names[:end]
}

// leU32 reads a little-endian uint32 from b at off, returning 0 if out of range.
func leU32(b []byte, off uint32) uint32 {
	if int(off)+4 > len(b) {
		return 0
	}
	return uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
}
