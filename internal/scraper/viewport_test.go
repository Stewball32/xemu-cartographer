package scraper

import "testing"

func ptr(i int) *int { return &i }

func TestLocalViewport_Layouts(t *testing.T) {
	tests := []struct {
		name       string
		count      int
		localIndex int
		want       ViewportRect
	}{
		// 1 player → full screen.
		{"1p full", 1, 0, ViewportRect{0, 0, 1, 1}},

		// 2 players → HORIZONTAL halves (top/bottom, full width).
		{"2p index0 top", 2, 0, ViewportRect{0, 0, 1, 0.5}},
		{"2p index1 bottom", 2, 1, ViewportRect{0, 0.5, 1, 0.5}},

		// 3 players → 3 quadrants, bottom-right unused.
		{"3p index0 TL", 3, 0, ViewportRect{0, 0, 0.5, 0.5}},
		{"3p index1 TR", 3, 1, ViewportRect{0.5, 0, 0.5, 0.5}},
		{"3p index2 BL", 3, 2, ViewportRect{0, 0.5, 0.5, 0.5}},

		// 4 players → full quadrants.
		{"4p index0 TL", 4, 0, ViewportRect{0, 0, 0.5, 0.5}},
		{"4p index1 TR", 4, 1, ViewportRect{0.5, 0, 0.5, 0.5}},
		{"4p index2 BL", 4, 2, ViewportRect{0, 0.5, 0.5, 0.5}},
		{"4p index3 BR", 4, 3, ViewportRect{0.5, 0.5, 0.5, 0.5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := LocalViewport(tt.count, tt.localIndex)
			if !ok {
				t.Fatalf("LocalViewport(%d,%d) ok=false, want true", tt.count, tt.localIndex)
			}
			if got != tt.want {
				t.Errorf("LocalViewport(%d,%d) = %+v, want %+v", tt.count, tt.localIndex, got, tt.want)
			}
		})
	}
}

// The 2-player split must be top/bottom (full-width horizontal halves), NOT
// left/right. Guards against a regression that flips it to a vertical split.
func TestLocalViewport_2pIsHorizontal(t *testing.T) {
	top, _ := LocalViewport(2, 0)
	bottom, _ := LocalViewport(2, 1)

	for name, r := range map[string]ViewportRect{"top": top, "bottom": bottom} {
		if r.W != 1 {
			t.Errorf("2p %s half W = %v, want 1 (full width → horizontal split)", name, r.W)
		}
		if r.H != 0.5 {
			t.Errorf("2p %s half H = %v, want 0.5", name, r.H)
		}
	}
	if top.Y != 0 {
		t.Errorf("2p top Y = %v, want 0", top.Y)
	}
	if bottom.Y != 0.5 {
		t.Errorf("2p bottom Y = %v, want 0.5", bottom.Y)
	}
}

func TestLocalViewport_OutOfRange(t *testing.T) {
	bad := []struct {
		count, localIndex int
	}{
		{0, 0},  // no local players
		{1, 1},  // index past count
		{2, 2},  // index past count
		{2, -1}, // negative index (network/non-local marker)
		{5, 0},  // count above the cap
		{4, 4},  // index past count
		{-1, 0}, // negative count
	}
	for _, b := range bad {
		if rect, ok := LocalViewport(b.count, b.localIndex); ok {
			t.Errorf("LocalViewport(%d,%d) ok=true (%+v), want false", b.count, b.localIndex, rect)
		}
	}
}

// Each layout must tile its screen region without overlap: areas sum to the
// full screen for 1/2/4 players, and to 3/4 for the 3-player layout (the
// bottom-right quadrant is intentionally empty).
func TestLocalViewport_TilesScreen(t *testing.T) {
	wantArea := map[int]float32{1: 1, 2: 1, 3: 0.75, 4: 1}
	for count := 1; count <= MaxLocalPlayers; count++ {
		var area float32
		for idx := 0; idx < count; idx++ {
			r, ok := LocalViewport(count, idx)
			if !ok {
				t.Fatalf("LocalViewport(%d,%d) unexpectedly ok=false", count, idx)
			}
			area += r.W * r.H
		}
		if area != wantArea[count] {
			t.Errorf("count=%d total viewport area = %v, want %v", count, area, wantArea[count])
		}
	}
}

func TestAssignLocalViewports(t *testing.T) {
	players := []GamePlayer{
		{Index: 0, LocalIndex: ptr(0)}, // local, top
		{Index: 1, LocalIndex: ptr(1)}, // local, bottom
		{Index: 2, LocalIndex: nil},    // network/non-local
	}

	AssignLocalViewports(players, 2)

	if players[0].Viewport == nil || *players[0].Viewport != (ViewportRect{0, 0, 1, 0.5}) {
		t.Errorf("player 0 viewport = %+v, want top half", players[0].Viewport)
	}
	if players[1].Viewport == nil || *players[1].Viewport != (ViewportRect{0, 0.5, 1, 0.5}) {
		t.Errorf("player 1 viewport = %+v, want bottom half", players[1].Viewport)
	}
	if players[2].Viewport != nil {
		t.Errorf("network player viewport = %+v, want nil", players[2].Viewport)
	}
}

// A local index the layout can't place (e.g. a stale count) clears the
// viewport rather than leaving a stale rect.
func TestAssignLocalViewports_ClearsUnplaceable(t *testing.T) {
	stale := ViewportRect{0.5, 0.5, 0.5, 0.5}
	players := []GamePlayer{{Index: 0, LocalIndex: ptr(3), Viewport: &stale}}

	AssignLocalViewports(players, 2) // index 3 invalid for a 2-player layout

	if players[0].Viewport != nil {
		t.Errorf("viewport = %+v, want nil (index out of range for count)", players[0].Viewport)
	}
}

func TestAssignLocalViewports_ZeroCount(t *testing.T) {
	players := []GamePlayer{{Index: 0, LocalIndex: ptr(0)}}
	AssignLocalViewports(players, 0)
	if players[0].Viewport != nil {
		t.Errorf("viewport = %+v, want nil when count=0", players[0].Viewport)
	}
}
