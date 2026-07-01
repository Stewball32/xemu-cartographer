package scraper

// MaxLocalPlayers is the engine's hard cap on splitscreen local players.
// (Halo: CE and Halo 2 both top out at 4-way splitscreen.)
const MaxLocalPlayers = 4

// ViewportRect is the normalized on-screen region a local splitscreen player's
// view occupies on the output frame. Origin is top-left; every component is a
// fraction in [0,1] — X,Y are the top-left corner, W,H the size. It is
// resolution-independent: an overlay multiplies the rect by the actual output
// pixel width/height to place per-player elements (nameplates, per-player
// stats) over the correct viewport at any size.
type ViewportRect struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	W float32 `json:"w"`
	H float32 `json:"h"`
}

// LocalViewport returns the screen rect a local splitscreen player occupies,
// given the number of local (splitscreen) players on this machine and that
// player's engine local_player_index. It encodes the fixed splitscreen layout
// shared by Halo: CE and Halo 2:
//
//	count 1      → full screen           {0,   0,   1,   1}
//	count 2      → horizontal halves      index 0 = TOP    {0,   0,   1,   0.5}
//	                                       index 1 = BOTTOM {0,   0.5, 1,   0.5}
//	count 3 or 4 → quadrants              index 0 = top-left     {0,   0,   0.5, 0.5}
//	                                       index 1 = top-right    {0.5, 0,   0.5, 0.5}
//	                                       index 2 = bottom-left  {0,   0.5, 0.5, 0.5}
//	                                       index 3 = bottom-right {0.5, 0.5, 0.5, 0.5}
//
// The 2-player split is HORIZONTAL (top/bottom, full width), not left/right —
// this matches the retail CE/H2 behaviour. In a 3-player game indices 0..2 use
// the top-left, top-right, and bottom-left quadrants; the bottom-right quadrant
// is unused by the engine.
//
// ok is false (and the zero rect returned) when localIndex is out of range for
// the layout (localIndex < 0, count outside 1..MaxLocalPlayers, or
// localIndex >= count) — e.g. a network/non-local player or a malformed read.
func LocalViewport(count, localIndex int) (ViewportRect, bool) {
	if localIndex < 0 || count < 1 || count > MaxLocalPlayers || localIndex >= count {
		return ViewportRect{}, false
	}

	switch count {
	case 1:
		return ViewportRect{X: 0, Y: 0, W: 1, H: 1}, true
	case 2:
		// Horizontal halves, stacked top over bottom.
		if localIndex == 0 {
			return ViewportRect{X: 0, Y: 0, W: 1, H: 0.5}, true
		}
		return ViewportRect{X: 0, Y: 0.5, W: 1, H: 0.5}, true
	default: // 3 or 4 → quadrants, row-major (0=TL, 1=TR, 2=BL, 3=BR)
		col := float32(localIndex % 2) // 0 = left column, 1 = right column
		row := float32(localIndex / 2) // 0 = top row,     1 = bottom row
		return ViewportRect{X: col * 0.5, Y: row * 0.5, W: 0.5, H: 0.5}, true
	}
}

// AssignLocalViewports fills the Viewport of every local player in players
// (those carrying a non-nil LocalIndex) with its splitscreen screen rect for
// the given local player count, via LocalViewport. Network/non-local players
// (LocalIndex == nil) and any local index the layout cannot place are left with
// a nil Viewport. Safe to call with count 0 (clears/leaves all viewports nil).
//
// It is engine-agnostic: any game plugin that populates GamePlayer.LocalIndex
// and the match's local player count can reuse it.
func AssignLocalViewports(players []GamePlayer, count int) {
	for i := range players {
		if players[i].LocalIndex == nil {
			players[i].Viewport = nil
			continue
		}
		if rect, ok := LocalViewport(count, *players[i].LocalIndex); ok {
			r := rect
			players[i].Viewport = &r
		} else {
			players[i].Viewport = nil
		}
	}
}
