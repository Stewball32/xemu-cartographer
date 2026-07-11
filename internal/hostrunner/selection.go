package hostrunner

import "sync"

// Pick is a chosen card-screen option. In CE the runner reaches a non-default
// card by D-pad steps then A; v1 assumes the default-highlighted card (Steps==0,
// a bare A). Name is carried purely for the observable stream / logging.
type Pick struct {
	Name  string
	Steps int // D-pad steps from the default highlight (v1 uses 0)
}

// Selector is the v1 seam requirement: selection is a decision-source that emits
// "the pick is X", kept SEPARATE from the runner that applies it. v1 ships a
// single static source (FixedSelector); later sources (rotation, vote, admin
// queue) implement the same interface without touching the runner.
type Selector interface {
	MapPick() Pick
	GametypePick() Pick
}

// FixedSelector is the v1 single source: constant map + gametype picks.
type FixedSelector struct {
	Map      Pick
	Gametype Pick
}

func (f FixedSelector) MapPick() Pick      { return f.Map }
func (f FixedSelector) GametypePick() Pick { return f.Gametype }

// DefaultSelector picks whatever card is highlighted by default (bare A on each
// card screen) — the minimal v1 that needs no D-pad navigation.
func DefaultSelector() Selector {
	return FixedSelector{
		Map:      Pick{Name: "default", Steps: 0},
		Gametype: Pick{Name: "default", Steps: 0},
	}
}

// AtomicSelector is a concurrency-safe Selector the player-scoped API mutates
// (SetSelection) while the runner reads it on the scraper-loop goroutine. It is
// the decision-source seam made live: a player picks a map/gametype in the play
// tab, the /api/play route calls Set, and the pick surfaces on the observable
// stream + status as the runner's intent.
//
// v1 note: the CE host sequence still presses the default-highlighted card (see
// DefaultHostSequence), so Steps is recorded but not yet walked — mapping a map
// NAME to its card's D-pad position is follow-up work. The pick is honest intent
// until then, not a guaranteed on-screen selection.
type AtomicSelector struct {
	mu sync.RWMutex
	m  Pick
	g  Pick
}

// NewAtomicSelector seeds the initial picks (default-highlighted when zero).
func NewAtomicSelector(mapPick, gametypePick Pick) *AtomicSelector {
	if mapPick == (Pick{}) {
		mapPick = Pick{Name: "default"}
	}
	if gametypePick == (Pick{}) {
		gametypePick = Pick{Name: "default"}
	}
	return &AtomicSelector{m: mapPick, g: gametypePick}
}

func (s *AtomicSelector) MapPick() Pick {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m
}

func (s *AtomicSelector) GametypePick() Pick {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.g
}

// Set replaces both picks.
func (s *AtomicSelector) Set(mapPick, gametypePick Pick) {
	s.mu.Lock()
	s.m, s.g = mapPick, gametypePick
	s.mu.Unlock()
}

// Get returns a snapshot of both picks.
func (s *AtomicSelector) Get() (mapPick, gametypePick Pick) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m, s.g
}
