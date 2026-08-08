package scraper

// MapOption is one selectable map (or gametype) enumerated LIVE from a specific
// instance — the actual game/disc on that box, so modded discs' custom maps show
// up. Steps is the option's ABSOLUTE 0-based index (position) in the live carousel,
// NOT a D-pad press count: the carousel start is non-deterministic, so navigating to
// a pick is cursor-relative (presses = (Steps − liveCursorIndex) mod count) and must
// be computed at nav time. See internal/scraper/haloce EnumerateLobby.
type MapOption struct {
	Name  string `json:"name"`
	Steps int    `json:"steps"`
}

// MapList is the per-instance available maps + gametypes the player API serves.
// Available is false when the instance can't be enumerated yet (no live carousel
// read / disc parse for it) — in which case Maps/Gametypes are empty and the
// caller must NOT substitute a fixed/stock table (modded discs make any hardcoded
// set wrong). Selection then falls back to free-form names.
type MapList struct {
	Available bool        `json:"available"`
	Maps      []MapOption `json:"maps"`
	Gametypes []MapOption `json:"gametypes"`
	// GametypesPending is true while the box is enumerable (built-ins readable) but
	// the async host-side custom-variant read hasn't finished — i.e. the gametype
	// list is still incomplete. The picker shows a "reading gametypes…" waiting
	// state instead of the built-ins-only intermediate until this clears.
	GametypesPending bool `json:"gametypes_pending"`
}

// IndexOf returns the Steps for a named map/gametype option (case-insensitive on
// the caller's side is not assumed — names are compared as-is), and whether it
// was found. Used by the play API to translate a chosen name into the runner's
// D-pad navigation count.
func (l MapList) IndexOf(options []MapOption, name string) (int, bool) {
	for _, o := range options {
		if o.Name == name {
			return o.Steps, true
		}
	}
	return 0, false
}
