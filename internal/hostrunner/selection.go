package hostrunner

import "sync"

// Pick is a chosen card-screen option. In CE the runner reaches a non-default
// card by D-pad steps then A; v1 assumes the default-highlighted card (Steps==0,
// a bare A). Name is carried purely for the observable stream / logging.
type Pick struct {
	Name  string
	Steps int // D-pad steps from the default highlight (v1 uses 0)
}

// Selector is the decision-source seam: selection is emitted separately from the
// runner that applies it. HasSelection gates the front-loaded park at map-select
// (the runner creates the lobby with Y immediately, then HOLDS at the map-select
// card screen until HasSelection reports a pick — never auto-picking a default).
type Selector interface {
	MapPick() Pick
	GametypePick() Pick
	// HasSelection reports whether a map+gametype have been actively chosen. The
	// runner parks at map-select while this is false.
	HasSelection() bool
}

// FixedSelector is a constant source: fixed map + gametype picks. HasSelection is
// true only when both names are non-empty, so a zero-value FixedSelector parks.
type FixedSelector struct {
	Map      Pick
	Gametype Pick
}

func (f FixedSelector) MapPick() Pick      { return f.Map }
func (f FixedSelector) GametypePick() Pick { return f.Gametype }
func (f FixedSelector) HasSelection() bool { return f.Map.Name != "" && f.Gametype.Name != "" }

// DefaultSelector is the non-parking fallback used when a runner is built with no
// selector: it treats the default-highlighted cards as an implicit selection
// (bare A, no D-pad nav), so the flow proceeds without waiting. Production runners
// are built with an empty AtomicSelector instead, which PARKS until a player picks
// (see the manager's attachHostRunner).
func DefaultSelector() Selector {
	return FixedSelector{
		Map:      Pick{Name: "default", Steps: 0},
		Gametype: Pick{Name: "default", Steps: 0},
	}
}

// AtomicSelector is a concurrency-safe Selector the player-scoped API mutates
// (Set) while the runner reads it on the scraper-loop goroutine. It STARTS EMPTY
// (unselected) so a production runner parks at map-select until a player picks —
// there is deliberately no default. Set marks it selected and the runner then
// applies the pick in one forward pass (D-pad Pick.Steps to the chosen card → A).
//
// Steps is supplied by the caller from the live map carousel index (see the play
// API's SetSelection), so a modded disc's custom maps navigate correctly; when
// the live carousel index is unknown, Steps is 0 (the default-highlighted card).
type AtomicSelector struct {
	mu  sync.RWMutex
	m   Pick
	g   Pick
	set bool
}

// NewAtomicSelector returns an EMPTY selector (unselected → the runner parks).
func NewAtomicSelector() *AtomicSelector { return &AtomicSelector{} }

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

// HasSelection reports whether a pick has been made (the park gate).
func (s *AtomicSelector) HasSelection() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set
}

// Set replaces both picks and marks the selector selected.
func (s *AtomicSelector) Set(mapPick, gametypePick Pick) {
	s.mu.Lock()
	s.m, s.g, s.set = mapPick, gametypePick, true
	s.mu.Unlock()
}

// Clear resets to unselected (the runner re-parks at map-select). Used to reset a
// session so the next lobby waits for a fresh pick.
func (s *AtomicSelector) Clear() {
	s.mu.Lock()
	s.m, s.g, s.set = Pick{}, Pick{}, false
	s.mu.Unlock()
}

// Get returns a snapshot of both picks.
func (s *AtomicSelector) Get() (mapPick, gametypePick Pick) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m, s.g
}
