package scraper

// LobbyOption is one selectable map or gametype enumerated LIVE from a running
// game's create-game carousel, in carousel order. Steps is the option's ABSOLUTE
// 0-based index (position) in that carousel — NOT a D-pad press count. Navigation is
// cursor-relative (presses = (targetIndex − liveCursorIndex) mod count) because the
// carousel start is non-deterministic; see the CE reader's EnumerateLobby doc.
//
// Game-agnostic wire type. The scraper manager maps it 1:1 onto the
// guards/interfaces/scraper.MapOption the player API serves (kept separate so the
// game plugins need no dependency on the guards package).
type LobbyOption struct {
	Name  string
	Steps int
}

// LobbyOptions is a per-instance enumeration of the create-game MAP and GAMETYPE
// carousels, read from live guest memory. Available is false when the running
// game can't be enumerated (title not registered, or the create-game UI tags
// aren't loaded — e.g. mid-match). When false, Maps/Gametypes are empty and the
// caller MUST NOT substitute a fixed/stock table (modded discs make any hardcoded
// set wrong); it keeps the last successful enumeration instead.
type LobbyOptions struct {
	Available bool
	Maps      []LobbyOption
	Gametypes []LobbyOption
}

// LobbyEnumerator is an OPTIONAL capability a GameReader may implement to expose
// the live create-game map/gametype carousels for the player-hosting picker. The
// scraper manager type-asserts a reader for it; readers that don't implement it
// simply never populate the available-maps cache (the picker then falls back to
// free-form names). Called from the scraper loop goroutine only — same discipline
// as the other GameReader reads.
type LobbyEnumerator interface {
	// EnumerateLobby reads the map + gametype carousels from the loaded UI data.
	// Returns Available=false (empty lists) when the game isn't at/past a state
	// where the carousels are readable, so the caller keeps the last-known set.
	EnumerateLobby() LobbyOptions
}
