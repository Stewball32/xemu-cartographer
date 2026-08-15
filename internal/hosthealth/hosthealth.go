// Package hosthealth answers one question the wire could never answer before:
// "is this host actually sustaining its engine tick rate?"
//
// A struggling xemu host is invisible in the existing telemetry. `engine_tick`
// is already broadcast per instance, but nothing compares it to wall clock, so
// a box rendering at 24Hz looks identical to one at 30Hz — the counter just
// climbs slower, and every consumer reads it as an opaque number. Diagnosing a
// laggy host therefore meant capturing the stream by hand and doing the
// division offline.
//
// Tracker does that division continuously: feed it (tick, timestamp) pairs and
// it reports OBSERVED ticks/sec against the EXPECTED engine rate over a rolling
// window. Pure logic with an injected clock — no PB, no I/O, no goroutines — so
// it's fully unit-testable apart from the scraper loop that drives it. The
// manager owns the wiring (see internal/scraper/manager/runner.go).
//
// The hard part is not the arithmetic, it's refusing to cry wolf. Three guest
// behaviours all look like "slow" to a naive rate calculation:
//
//   - The engine clock RE-INITS TO 0 at match start (live-verified 2026-08-08 —
//     see sveltekit/src/lib/utils/overlay-state.ts). A backwards jump would
//     otherwise compute a negative or absurd rate.
//   - A guest sitting at a menu, paused, or an Idle-phase runner (which reports
//     a literal tick=0) isn't slow, it's STOPPED. That must read as stalled,
//     never as degraded.
//   - Phase transitions jump the counter discontinuously (Idle's constant 0 →
//     Ready's live free-running value), implying a rate in the hundreds of kHz.
//
// So Tracker resets its window on any discontinuity, reports StatusStalled once
// the tick has genuinely stopped advancing, and withholds a verdict entirely
// (StatusUnknown, Confident=false) until it has enough span and enough samples
// to tell 30 from 28.
package hosthealth

import "time"

// ExpectedTickHz is the Halo CE engine tick rate — 30 ticks/sec. The engine's
// game-time counter advances at this rate on a healthy host regardless of
// render framerate, which is exactly what makes it a host-health signal rather
// than a graphics one.
const ExpectedTickHz = 30.0

// Status is the coarse verdict a UI can colour on without interpreting Ratio.
type Status string

const (
	// StatusUnknown means there isn't enough data to judge yet — freshly
	// started, or just reset by a discontinuity. Not a problem report.
	StatusUnknown Status = "unknown"
	// StatusStalled means the tick has stopped advancing entirely: a menu,
	// a paused guest, or an Idle-phase runner. Explicitly NOT "slow" — this
	// is the state that must never be mistaken for a struggling host.
	StatusStalled Status = "stalled"
	// StatusOK means the observed rate is at or near expected.
	StatusOK Status = "ok"
	// StatusDegraded means the host is measurably below the expected rate.
	StatusDegraded Status = "degraded"
)

// Config tunes the measurement. Zero-valued fields fall back to the package
// defaults via New, so Config{} is a valid "just give me sane behaviour".
type Config struct {
	// ExpectedHz is the rate the engine should sustain. Defaults to
	// ExpectedTickHz; a parameter rather than a constant so a future
	// non-CE plugin with a different tick rate can reuse this package.
	ExpectedHz float64
	// Window is how far back the rolling measurement reaches. Longer =
	// steadier and more precise, slower to react.
	Window time.Duration
	// MinSpan is the shortest measured span that yields a confident reading.
	// Guards against a wild rate computed from two samples milliseconds apart.
	MinSpan time.Duration
	// MinSamples is the fewest observations that yield a confident reading.
	MinSamples int
	// StallAfter is how long the tick may sit unchanged before the verdict
	// flips to StatusStalled. Must exceed one expected tick period by a
	// comfortable margin or normal inter-tick gaps would read as stalls.
	StallAfter time.Duration
	// DegradedBelow is the observed/expected ratio under which the verdict is
	// StatusDegraded.
	DegradedBelow float64
}

// Defaults. A 5s window at 30Hz spans ~150 tick advances, so the ±1-tick
// quantization error on the endpoints is ±0.2Hz — far finer than the 2Hz gap
// between a healthy 30 and a struggling 28. MinSpan of 2s still resolves
// ±0.5Hz, which distinguishes those two, so a reading appears within a couple
// of seconds of a match starting rather than making an operator wait out the
// full window. StallAfter of 1s is 30 expected ticks — long enough that no
// normal scheduling hiccup trips it, short enough that a menu reads as stalled
// before the shrinking window can make it look degraded.
const (
	DefaultWindow        = 5 * time.Second
	DefaultMinSpan       = 2 * time.Second
	DefaultMinSamples    = 8
	DefaultStallAfter    = 1 * time.Second
	DefaultDegradedBelow = 0.95
)

// maxPlausibleRateFactor bounds how far above expected an inter-sample delta
// may imply before it's treated as a discontinuity rather than a measurement.
// The engine cannot legitimately run 4x fast; a delta that says otherwise means
// the counter jumped (phase change, re-init, a different clock source), so the
// window is reset instead of poisoned.
const maxPlausibleRateFactor = 4.0

// maxPlausibleSlackTicks is added to the plausibility bound so that two samples
// taken microseconds apart — where expected*dt rounds to nearly zero — don't
// flag a perfectly normal single-tick advance as a discontinuity.
const maxPlausibleSlackTicks = 30.0

// Health is the computed reading. Value type with no pointers, so callers can
// snapshot it into a cache and hand out copies without sharing state.
type Health struct {
	// Status is the coarse verdict; see the Status constants.
	Status Status `json:"status"`
	// ObservedHz is the measured tick rate over the window, rounded to 2dp.
	// Zero when there's nothing to report (unknown/stalled).
	ObservedHz float64 `json:"observed_hz"`
	// ExpectedHz is the rate ObservedHz is being judged against.
	ExpectedHz float64 `json:"expected_hz"`
	// Ratio is ObservedHz/ExpectedHz, rounded to 3dp. 1.0 is on-rate.
	Ratio float64 `json:"ratio"`
	// WindowSeconds is the ACTUAL measured span, not the configured window —
	// a reading taken 2s after a reset says 2, not 5. Publishing the real
	// span is what lets a reader judge how much to trust ObservedHz.
	WindowSeconds float64 `json:"window_seconds"`
	// Samples is how many observations back the reading.
	Samples int `json:"samples"`
	// MeasuredAt is when the newest observation landed. Consumers compute
	// staleness from it the same way they do from the game class's
	// last_read_at — a wedged runner stops updating this while the rest of
	// the struct keeps its last-known (and now misleading) values.
	MeasuredAt time.Time `json:"measured_at"`
	// Confident reports whether span + sample count cleared the thresholds.
	// False means treat ObservedHz as indicative at best.
	Confident bool `json:"confident"`
}

// Age reports how long ago the reading was taken. Negative durations are
// clamped to zero so a slightly-behind clock can't render as a future sample.
func (h Health) Age(now time.Time) time.Duration {
	if h.MeasuredAt.IsZero() {
		return 0
	}
	if age := now.Sub(h.MeasuredAt); age > 0 {
		return age
	}
	return 0
}

// sample is one observation of the engine counter.
type sample struct {
	tick uint32
	at   time.Time
}

// Tracker accumulates observations and computes Health. Not safe for
// concurrent use — the scraper drives one per runner from its single loop
// goroutine, under the same lock that guards the rest of that runner's cache.
type Tracker struct {
	cfg Config

	// ring is a bounded deque of samples ordered oldest→newest. Bounded so a
	// Live loop polling at ~100Hz for hours can't grow it without limit; the
	// window eviction normally keeps it well under capacity.
	ring  []sample
	head  int
	count int

	// lastAdvanceAt is when the tick last actually CHANGED, as opposed to
	// when it was last observed. Stall detection keys off this: a counter
	// that's being read 100 times a second but never moving is stalled, and
	// the observation timestamps alone can't distinguish that from healthy.
	lastAdvanceAt time.Time
}

// New builds a Tracker, filling any zero-valued Config field with its default.
func New(cfg Config) *Tracker {
	if cfg.ExpectedHz <= 0 {
		cfg.ExpectedHz = ExpectedTickHz
	}
	if cfg.Window <= 0 {
		cfg.Window = DefaultWindow
	}
	if cfg.MinSpan <= 0 {
		cfg.MinSpan = DefaultMinSpan
	}
	if cfg.MinSamples <= 0 {
		cfg.MinSamples = DefaultMinSamples
	}
	if cfg.StallAfter <= 0 {
		cfg.StallAfter = DefaultStallAfter
	}
	if cfg.DegradedBelow <= 0 {
		cfg.DegradedBelow = DefaultDegradedBelow
	}
	// Capacity covers a full window of expected-rate advances with 2x headroom
	// (the loop can observe a tick more than once, and a fast guest could
	// briefly exceed rate), floored so a tiny window still holds something.
	capacity := int(cfg.ExpectedHz*cfg.Window.Seconds()*2) + 2
	if capacity < 16 {
		capacity = 16
	}
	return &Tracker{cfg: cfg, ring: make([]sample, capacity)}
}

// Observe records one reading of the engine tick counter at time `at`.
//
// Repeated observations of an UNCHANGED tick are recorded too — they're what
// proves the counter is being watched and isn't moving, which is how a stall is
// told apart from a runner that simply stopped reporting. Only the moment of an
// actual advance updates lastAdvanceAt.
//
// A discontinuity (tick moved backwards, or forward implausibly far) drops the
// accumulated window and restarts from this sample, because no rate computed
// across such a jump means anything.
func (t *Tracker) Observe(tick uint32, at time.Time) {
	if prev, ok := t.newest(); ok {
		if t.isDiscontinuity(prev, tick, at) {
			t.Reset()
		} else if tick != prev.tick {
			t.lastAdvanceAt = at
		}
	}
	if t.lastAdvanceAt.IsZero() {
		// First sample of a fresh window: the counter hasn't been seen to
		// advance yet, so start the stall clock here rather than leaving it
		// zero (which would read as "stalled since the epoch").
		t.lastAdvanceAt = at
	}
	t.push(sample{tick: tick, at: at})
	t.evictBefore(at.Add(-t.cfg.Window))
}

// isDiscontinuity reports whether the step from prev to tick is a counter jump
// rather than elapsed engine time. Backwards is unambiguous (the engine
// re-inits to 0 at match start). Forwards is judged against what the expected
// rate could plausibly have produced in the elapsed wall time.
func (t *Tracker) isDiscontinuity(prev sample, tick uint32, at time.Time) bool {
	if tick < prev.tick {
		return true
	}
	dt := at.Sub(prev.at).Seconds()
	if dt < 0 {
		// Time went backwards (clock adjustment). Nothing sensible to measure
		// across that boundary.
		return true
	}
	limit := t.cfg.ExpectedHz*maxPlausibleRateFactor*dt + maxPlausibleSlackTicks
	return float64(tick-prev.tick) > limit
}

// Reset drops all accumulated samples. The next reading is StatusUnknown until
// the window refills.
func (t *Tracker) Reset() {
	t.head = 0
	t.count = 0
	t.lastAdvanceAt = time.Time{}
}

// Health computes the current reading as of `now`.
//
// `now` is separate from the newest sample's timestamp on purpose: a runner
// that has WEDGED stops calling Observe, and only a caller-supplied clock can
// notice that the last advance was a long time ago. Deriving staleness from the
// samples alone would let a frozen tracker keep reporting a healthy 30.0.
func (t *Tracker) Health(now time.Time) Health {
	h := Health{
		Status:     StatusUnknown,
		ExpectedHz: t.cfg.ExpectedHz,
		Samples:    t.count,
	}
	newest, ok := t.newest()
	if !ok {
		return h
	}
	h.MeasuredAt = newest.at

	oldest, _ := t.oldest()
	span := newest.at.Sub(oldest.at)
	h.WindowSeconds = round(span.Seconds(), 2)

	// Stall check first, and against `now` rather than the newest sample, so a
	// guest that stopped ticking mid-window reports stalled instead of the
	// halved rate a plain span division would produce.
	if !t.lastAdvanceAt.IsZero() && now.Sub(t.lastAdvanceAt) >= t.cfg.StallAfter {
		h.Status = StatusStalled
		return h
	}

	if span <= 0 {
		return h
	}

	// Endpoint-to-endpoint over the window. Intermediate jitter cancels out,
	// leaving a quantization error of at most one tick across the whole span.
	h.ObservedHz = round(float64(newest.tick-oldest.tick)/span.Seconds(), 2)
	h.Ratio = round(h.ObservedHz/t.cfg.ExpectedHz, 3)
	h.Confident = span >= t.cfg.MinSpan && t.count >= t.cfg.MinSamples
	if !h.Confident {
		// A reading exists and is published for transparency, but the verdict
		// stays Unknown — calling a 0.4s sample "degraded" is how false alarms
		// get made.
		return h
	}
	if h.Ratio < t.cfg.DegradedBelow {
		h.Status = StatusDegraded
	} else {
		h.Status = StatusOK
	}
	return h
}

// --- bounded deque ---

func (t *Tracker) push(s sample) {
	if t.count == len(t.ring) {
		// Full: drop the oldest to make room. Losing the far end of the window
		// only shortens the measured span, which Health reports honestly.
		t.head = (t.head + 1) % len(t.ring)
		t.count--
	}
	t.ring[(t.head+t.count)%len(t.ring)] = s
	t.count++
}

// evictBefore drops samples older than cutoff from the front. Samples are
// appended in time order, so the expired ones are always a prefix.
func (t *Tracker) evictBefore(cutoff time.Time) {
	// Keep at least one sample: an entirely-empty window would lose the
	// stall evidence for a guest whose tick froze longer than the window.
	for t.count > 1 && t.ring[t.head].at.Before(cutoff) {
		t.head = (t.head + 1) % len(t.ring)
		t.count--
	}
}

func (t *Tracker) oldest() (sample, bool) {
	if t.count == 0 {
		return sample{}, false
	}
	return t.ring[t.head], true
}

func (t *Tracker) newest() (sample, bool) {
	if t.count == 0 {
		return sample{}, false
	}
	return t.ring[(t.head+t.count-1)%len(t.ring)], true
}

// round to n decimal places. Keeps the wire readable and, more usefully, makes
// the unit tests assert exact values instead of chasing float epsilons.
func round(v float64, places int) float64 {
	pow := 1.0
	for range places {
		pow *= 10
	}
	r := v * pow
	if r < 0 {
		return float64(int64(r-0.5)) / pow
	}
	return float64(int64(r+0.5)) / pow
}
