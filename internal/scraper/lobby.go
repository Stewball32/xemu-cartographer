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

// LobbyCursor is the LIVE highlighted-card state of the create-game SELECT MAP /
// SELECT GAMETYPE carousels, read directly from the running game (CE: the menu
// widget-instance's +0x4C selected index + +0x54 item count). Index is the
// 0-based carousel position of the currently-highlighted card; Count is the live
// list length. Valid is true only when the corresponding list widget was found
// ACTIVE (count > 0) this read — false off the screen or when the read failed, in
// which case Index/Count are meaningless and the caller must not navigate on them.
//
// It is the missing piece that makes carousel navigation cursor-relative and
// closed-loop: presses = (targetIndex − Index) mod Count, and the runner confirms
// Index == targetIndex by re-reading before committing (A). The enumerated LIST
// (LobbyOptions) supplies targetIndex; this supplies the live cursor.
type LobbyCursor struct {
	MapIndex      int
	MapCount      int
	MapValid      bool
	GametypeIndex int
	GametypeCount int
	GametypeValid bool
}

// LobbyCursorReader is an OPTIONAL capability a GameReader may implement to expose
// the LIVE carousel cursor (LobbyCursor) for the player-hosting navigation. Paired
// with LobbyEnumerator: the enumerator gives the option LIST + absolute indices,
// this gives the live highlighted index so the runner navigates cursor-relative.
// Called from the scraper loop goroutine only — same discipline as the other reads.
type LobbyCursorReader interface {
	// ReadLobbyCursor reads the live map + gametype select-list cursors. Returns a
	// zero-Valid LobbyCursor when the widgets aren't readable (not on the create-game
	// screens, tags not loaded) so the caller holds rather than navigating blind.
	ReadLobbyCursor() LobbyCursor
}
