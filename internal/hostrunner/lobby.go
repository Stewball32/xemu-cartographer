package hostrunner

import (
	"fmt"
	"time"
)

// Transition is one gated step of the host-lobby flow. The machine presses Key
// only while On(obs) holds ("am I on the right screen?"), then waits until
// Done(obs) confirms the press landed before advancing. Blind returns true for
// the map/gametype card screens that expose no distinguishing global — those
// advance on a timed fallback (Sequence.blindAdvance) instead of Done, but only
// after On confirmed we entered the blind segment.
type Transition struct {
	Name   string
	Intent string
	Key    string
	On     func(Observation) bool
	Done   func(Observation) bool
	Blind  bool
}

// StepTiming controls debounce + blind-advance behaviour. Zero value uses
// DefaultTiming.
type StepTiming struct {
	// RepressAfter re-emits a step's key if Done still isn't observed this long
	// after the last press (covers a dropped KeyEvent / a slow menu).
	RepressAfter time.Duration
	// BlindAdvanceAfter advances a Blind step this long after it became active
	// even without a Done confirmation.
	BlindAdvanceAfter time.Duration
}

// DefaultTiming is tuned for CE's ~30Hz menus over VNC.
var DefaultTiming = StepTiming{
	RepressAfter:      1200 * time.Millisecond,
	BlindAdvanceAfter: 900 * time.Millisecond,
}

func (t StepTiming) orDefault() StepTiming {
	if t.RepressAfter == 0 {
		t.RepressAfter = DefaultTiming.RepressAfter
	}
	if t.BlindAdvanceAfter == 0 {
		t.BlindAdvanceAfter = DefaultTiming.BlindAdvanceAfter
	}
	return t
}

// Sequence walks an ordered list of gated Transitions. It is the "never blind"
// core: it advances only on confirmed observations (or a timed fallback for the
// genuinely-blind card screens), never by counting presses. Not safe for
// concurrent use — the runner drives it from one goroutine.
type Sequence struct {
	steps  []Transition
	timing StepTiming

	cursor    int
	enteredAt time.Time // when the current step became active
	lastPress time.Time // last time the current step's key was emitted
	pressed   bool      // has the current step been pressed since it became active
}

// NewSequence builds a Sequence over steps with the given timing.
func NewSequence(steps []Transition, timing StepTiming) *Sequence {
	return &Sequence{steps: steps, timing: timing.orDefault()}
}

// DefaultHostSequence is the CE system-link host flow:
//
//	System Link (gc==1) --Y--> map card --A--> gametype card --A--> LOBBY
//
// The two card presses are Blind (no distinguishing global); they are bracketed
// by readable checkpoints — On requires we're hosting, and the terminal Done
// requires a readable lobby (Screen==ScreenLobby). Keys use vncinput labels:
// Y='y', A='a'.
func DefaultHostSequence(timing StepTiming) *Sequence {
	onSystemLink := func(o Observation) bool { return Classify(o) == ScreenSystemLink }
	hosting := func(o Observation) bool {
		s := Classify(o)
		return s == ScreenHosting || s == ScreenLobby
	}
	inLobby := func(o Observation) bool { return Classify(o) == ScreenLobby }

	return NewSequence([]Transition{
		{
			Name: "create-game", Intent: "create system-link game", Key: "y",
			On:   onSystemLink,
			Done: hosting, // landed in the hosting setup (map card or beyond)
		},
		{
			Name: "select-map", Intent: "select map", Key: "a",
			On:    hosting,
			Done:  inLobby, // only truly confirmable once the lobby is readable
			Blind: true,
		},
		{
			Name: "select-gametype", Intent: "select gametype", Key: "a",
			On:    hosting,
			Done:  inLobby,
			Blind: true,
		},
		{
			Name: "reach-lobby", Intent: "reach lobby", Key: "",
			On:   inLobby,
			Done: inLobby, // sentinel: confirms we're settled in the lobby
		},
	}, timing)
}

// Cursor / Current expose progress for events + tests.
func (s *Sequence) Cursor() int { return s.cursor }
func (s *Sequence) Done() bool  { return s.cursor >= len(s.steps) }
func (s *Sequence) Current() (Transition, bool) {
	if s.Done() {
		return Transition{}, false
	}
	return s.steps[s.cursor], true
}

// Reset rewinds to the first step (used by the auto-host loop before re-hosting).
func (s *Sequence) Reset(now time.Time) {
	s.cursor = 0
	s.enteredAt = now
	s.pressed = false
	s.lastPress = time.Time{}
}

func (s *Sequence) advance(now time.Time) {
	s.cursor++
	s.enteredAt = now
	s.pressed = false
	s.lastPress = time.Time{}
}

// Step decides the next Action for the current observation + clock:
//  1. catch-up: advance past any steps whose Done already holds (e.g. we landed
//     in the lobby, so every card step is retroactively satisfied);
//  2. Done when the sequence is complete;
//  3. a Blind card step (map/gametype — indistinguishable in memory) presses
//     once, then advances on a BlindAdvanceAfter timer — it CAN'T confirm the
//     sub-screen, so it never waits on On/Done between the cards;
//  4. a non-Blind step presses only while On holds (debounced) and advances on
//     Done; if On is false it Blocks rather than press blindly;
//  5. a key-less sentinel (reach-lobby) confirms the readable bracket.
func (s *Sequence) Step(obs Observation, now time.Time) Action {
	if !obs.Fresh {
		return wait("no fresh read")
	}
	// (1) catch up: advance past steps whose Done already holds. Blind card
	// steps use Done==inLobby, so reaching the readable lobby retroactively
	// satisfies all of them at once (handles presses landing faster than timed).
	for !s.Done() {
		t := s.steps[s.cursor]
		if t.Done != nil && t.Done(obs) {
			s.advance(now)
			continue
		}
		break
	}
	if s.Done() {
		return done("sequence complete")
	}

	t := s.steps[s.cursor]

	// (5) key-less sentinel: confirms we're settled on the bracket screen.
	if t.Key == "" {
		if t.On != nil && t.On(obs) {
			return done("reached " + t.Name)
		}
		return wait("awaiting " + t.Name)
	}

	// (3) blind card step: gate entry on On (we're in the hosting segment), then
	// press once and advance on a timer — the sub-screens are indistinguishable.
	if t.Blind {
		if t.On != nil && !t.On(obs) {
			return blocked(fmt.Sprintf("blind step %q: hosting segment not observed (on %s)", t.Name, screenName(obs)))
		}
		if !s.pressed {
			s.pressed = true
			s.lastPress = now
			return tap(t.Key, t.Intent, "hosting card, pressing "+t.Key)
		}
		if now.Sub(s.lastPress) >= s.timing.BlindAdvanceAfter {
			s.advance(now)
			return s.Step(obs, now) // evaluate the next step this tick
		}
		return wait("blind " + t.Name + ", awaiting timed advance")
	}

	// (4) non-blind step: press only while On holds, debounced; else Block.
	if t.On != nil && t.On(obs) {
		if !s.pressed || now.Sub(s.lastPress) >= s.timing.RepressAfter {
			s.pressed = true
			s.lastPress = now
			return tap(t.Key, t.Intent, "on "+screenName(obs)+", pressing "+t.Key)
		}
		return wait("pressed " + t.Key + ", awaiting confirm")
	}
	return blocked(fmt.Sprintf("expected screen for %q not observed (on %s)", t.Name, screenName(obs)))
}

// WalkBack presses the B face button to retreat one screen — and, if the native
// countdown is active, that same B cancels the countdown. It rewinds the cursor
// by one on a plain back-step so the machine re-presses forward from there.
func (s *Sequence) WalkBack(obs Observation, now time.Time) Action {
	if obs.CountdownActive {
		// The countdown owns the current screen; B stops it (no cursor change —
		// we stay in the lobby, just un-armed).
		return tap("b", "cancel countdown", "countdown active, pressing B to stop")
	}
	if s.cursor > 0 {
		s.cursor--
		s.enteredAt = now
		s.pressed = false
		s.lastPress = time.Time{}
	}
	return tap("b", "walk back", "pressing B to back out one screen")
}

func screenName(obs Observation) string { return Classify(obs).String() }

// vncinput labels used by the flow mirror the admin's XboxController.svelte /
// VNCKeyboard map exactly: A='a', B(face)='b', Y='y', Start='Return',
// Back='BackSpace'. In Halo menus the B FACE button is cancel/back, hence
// WalkBack emits "b".
