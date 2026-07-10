package hostrunner

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
