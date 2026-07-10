package hostrunner

// ActionKind is what the runner decided to do this tick. The integration layer
// executes it against the input channel (internal/vncinput).
type ActionKind int

const (
	// ActionWait: gating not satisfied yet (waiting for a press to land, a
	// screen to appear, or a native precondition) — do nothing this tick.
	ActionWait ActionKind = iota
	// ActionTap: press+release a single labelled key (the common case).
	ActionTap
	// ActionChord: press several keys together (e.g. Control_L+r reset).
	ActionChord
	// ActionDone: the sequence reached its target (lobby / armed / started).
	ActionDone
	// ActionBlocked: on an unexpected screen with no timed fallback — the
	// runner refuses to press blindly and surfaces this for the operator.
	ActionBlocked
)

func (k ActionKind) String() string {
	switch k {
	case ActionTap:
		return "tap"
	case ActionChord:
		return "chord"
	case ActionDone:
		return "done"
	case ActionBlocked:
		return "blocked"
	default:
		return "wait"
	}
}

// Action is the runner's per-tick decision. Keys carries the vncinput label(s)
// for Tap/Chord. Intent is the high-level player intent ("select map", "clear
// carnage") and Reason is a short gating explanation — both surface on the
// observable input stream.
type Action struct {
	Kind   ActionKind
	Keys   []string
	Intent string
	Reason string
}

func wait(reason string) Action    { return Action{Kind: ActionWait, Reason: reason} }
func blocked(reason string) Action { return Action{Kind: ActionBlocked, Reason: reason} }
func done(reason string) Action    { return Action{Kind: ActionDone, Reason: reason} }

func tap(key, intent, reason string) Action {
	return Action{Kind: ActionTap, Keys: []string{key}, Intent: intent, Reason: reason}
}

func chord(keys []string, intent, reason string) Action {
	return Action{Kind: ActionChord, Keys: keys, Intent: intent, Reason: reason}
}

// Key returns the first key (Tap) or "" — convenience for events/tests.
func (a Action) Key() string {
	if len(a.Keys) > 0 {
		return a.Keys[0]
	}
	return ""
}
