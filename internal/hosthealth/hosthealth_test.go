package hosthealth

import (
	"math"
	"testing"
	"time"
)

// base is an arbitrary fixed origin — the tracker only ever looks at
// differences, so the absolute value is irrelevant beyond being non-zero.
var base = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

// feed drives the tracker at `hz` engine ticks per second for `dur`, sampling
// at `pollEvery` (mimicking the scraper loop, which polls faster than the
// engine ticks and therefore observes the same tick value repeatedly).
// Returns the wall-clock time after the last observation.
func feed(t *Tracker, start time.Time, dur time.Duration, hz float64, pollEvery time.Duration, startTick uint32) time.Time {
	now := start
	end := start.Add(dur)
	for !now.After(end) {
		elapsed := now.Sub(start).Seconds()
		t.Observe(startTick+uint32(elapsed*hz), now)
		now = now.Add(pollEvery)
	}
	return now.Add(-pollEvery)
}

func TestHealthyHostReadsExpectedRate(t *testing.T) {
	tr := New(Config{})
	now := feed(tr, base, 6*time.Second, 30, 10*time.Millisecond, 1000)

	h := tr.Health(now)
	if h.Status != StatusOK {
		t.Fatalf("status = %q, want %q (observed %.2f)", h.Status, StatusOK, h.ObservedHz)
	}
	if !h.Confident {
		t.Errorf("Confident = false, want true (span %.2fs, %d samples)", h.WindowSeconds, h.Samples)
	}
	if math.Abs(h.ObservedHz-30) > 0.3 {
		t.Errorf("ObservedHz = %.2f, want ~30.0", h.ObservedHz)
	}
	if math.Abs(h.Ratio-1) > 0.01 {
		t.Errorf("Ratio = %.3f, want ~1.0", h.Ratio)
	}
	if h.ExpectedHz != ExpectedTickHz {
		t.Errorf("ExpectedHz = %v, want %v", h.ExpectedHz, ExpectedTickHz)
	}
}

// The acceptance bar: a host running at 28Hz must be distinguishable from one
// at 30Hz, not merely "a bit under". Both the numeric readout and the coarse
// status have to separate them.
func TestDistinguishes30From28(t *testing.T) {
	healthy := New(Config{})
	nowHealthy := feed(healthy, base, 6*time.Second, 30, 10*time.Millisecond, 0)
	slow := New(Config{})
	nowSlow := feed(slow, base, 6*time.Second, 28, 10*time.Millisecond, 0)

	hHealthy := healthy.Health(nowHealthy)
	hSlow := slow.Health(nowSlow)

	if math.Abs(hSlow.ObservedHz-28) > 0.3 {
		t.Errorf("slow host ObservedHz = %.2f, want ~28.0", hSlow.ObservedHz)
	}
	if hHealthy.ObservedHz-hSlow.ObservedHz < 1.5 {
		t.Errorf("30Hz (%.2f) and 28Hz (%.2f) are not separable", hHealthy.ObservedHz, hSlow.ObservedHz)
	}
	if hHealthy.Status != StatusOK {
		t.Errorf("healthy status = %q, want %q", hHealthy.Status, StatusOK)
	}
	if hSlow.Status != StatusDegraded {
		t.Errorf("slow status = %q, want %q", hSlow.Status, StatusDegraded)
	}
}

// A guest at a menu, paused, or an Idle-phase runner reports a tick that never
// moves. That must read as stalled — the whole point of the status field is
// that it never gets mistaken for a slow host.
func TestStalledTickIsNotDegraded(t *testing.T) {
	tr := New(Config{})
	// Establish a healthy baseline first, so the failure mode under test is
	// "was ticking, then stopped" rather than "never had data".
	now := feed(tr, base, 4*time.Second, 30, 10*time.Millisecond, 500)
	if got := tr.Health(now); got.Status != StatusOK {
		t.Fatalf("precondition: status = %q, want %q", got.Status, StatusOK)
	}

	// Tick freezes; the scraper keeps polling and keeps reading the same value.
	frozen, _ := tr.newest()
	for i := range 300 {
		tr.Observe(frozen.tick, now.Add(time.Duration(i)*10*time.Millisecond))
	}
	now = now.Add(3 * time.Second)

	h := tr.Health(now)
	if h.Status != StatusStalled {
		t.Fatalf("status = %q, want %q (observed %.2f)", h.Status, StatusStalled, h.ObservedHz)
	}
	if h.Status == StatusDegraded {
		t.Error("a stopped guest must never report as a degraded host")
	}
}

// An Idle-phase runner calls Observe(0, ...) every 3s forever. That is a
// stalled counter, not a host at 0Hz.
func TestIdlePhaseConstantZeroReadsStalled(t *testing.T) {
	tr := New(Config{})
	now := base
	for range 10 {
		tr.Observe(0, now)
		now = now.Add(3 * time.Second)
	}
	if h := tr.Health(now); h.Status != StatusStalled {
		t.Fatalf("status = %q, want %q", h.Status, StatusStalled)
	}
}

// The engine clock re-inits to 0 at match start (live-verified 2026-08-08). A
// backwards jump must discard the window rather than compute a nonsense rate.
func TestMatchStartResetDiscardsWindow(t *testing.T) {
	tr := New(Config{})
	now := feed(tr, base, 5*time.Second, 30, 10*time.Millisecond, 90000)

	// Match starts: counter re-inits to 0.
	now = now.Add(10 * time.Millisecond)
	tr.Observe(0, now)

	h := tr.Health(now)
	if h.Status != StatusUnknown {
		t.Fatalf("status after reset = %q, want %q", h.Status, StatusUnknown)
	}
	if h.Confident {
		t.Error("Confident = true immediately after a reset, want false")
	}
	if h.ObservedHz != 0 {
		t.Errorf("ObservedHz = %.2f after reset, want 0", h.ObservedHz)
	}

	// It recovers on the new window without any manual intervention.
	now = feed(tr, now, 5*time.Second, 30, 10*time.Millisecond, 0)
	if h := tr.Health(now); h.Status != StatusOK {
		t.Fatalf("status after refill = %q, want %q (observed %.2f)", h.Status, StatusOK, h.ObservedHz)
	}
}

// Idle→Ready jumps the counter from a constant 0 to a live free-running value.
// Measuring across that step would report hundreds of kHz.
func TestImplausibleForwardJumpIsTreatedAsDiscontinuity(t *testing.T) {
	tr := New(Config{})
	now := base
	for range 5 {
		tr.Observe(0, now)
		now = now.Add(3 * time.Second)
	}
	// Phase change: the real engine counter appears.
	tr.Observe(1234567, now)

	h := tr.Health(now)
	if h.ObservedHz != 0 {
		t.Fatalf("ObservedHz = %.2f across a phase-change jump, want 0 (window reset)", h.ObservedHz)
	}
	if h.Status != StatusUnknown {
		t.Errorf("status = %q, want %q", h.Status, StatusUnknown)
	}
}

// A normal advance must NOT be mistaken for a discontinuity, including the
// coarse 500ms Ready cadence (15 ticks/sample) and back-to-back samples.
func TestNormalAdvancesAreNotDiscontinuities(t *testing.T) {
	for _, tc := range []struct {
		name  string
		poll  time.Duration
		hz    float64
		wantS Status
	}{
		{"live cadence 10ms", 10 * time.Millisecond, 30, StatusOK},
		{"ready cadence 500ms", 500 * time.Millisecond, 30, StatusOK},
		{"slightly fast guest", 10 * time.Millisecond, 31, StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := New(Config{})
			now := feed(tr, base, 6*time.Second, tc.hz, tc.poll, 100)
			if h := tr.Health(now); h.Status != tc.wantS {
				t.Fatalf("status = %q, want %q (observed %.2f, samples %d)",
					h.Status, tc.wantS, h.ObservedHz, h.Samples)
			}
		})
	}
}

// A verdict must not be rendered off a sliver of data — that is how false
// "your host is slow" alarms get made.
func TestShortSpanIsNotConfident(t *testing.T) {
	tr := New(Config{})
	now := feed(tr, base, 400*time.Millisecond, 30, 10*time.Millisecond, 0)

	h := tr.Health(now)
	if h.Confident {
		t.Errorf("Confident = true at %.2fs span, want false", h.WindowSeconds)
	}
	if h.Status != StatusUnknown {
		t.Errorf("status = %q at %.2fs span, want %q", h.Status, h.WindowSeconds, StatusUnknown)
	}
	// The number is still published so an operator can see something is moving.
	if h.ObservedHz <= 0 {
		t.Errorf("ObservedHz = %.2f, want a provisional reading", h.ObservedHz)
	}
}

func TestTooFewSamplesIsNotConfident(t *testing.T) {
	tr := New(Config{MinSamples: 50})
	// Long span, but sampled sparsely: 4s at 1 sample/sec is only 5 samples.
	now := base
	for i := range 5 {
		tr.Observe(uint32(i*30), now)
		now = now.Add(time.Second)
	}
	now = now.Add(-time.Second)

	h := tr.Health(now)
	if h.Confident {
		t.Errorf("Confident = true with %d samples, want false", h.Samples)
	}
	if h.Status != StatusUnknown {
		t.Errorf("status = %q, want %q", h.Status, StatusUnknown)
	}
}

// WindowSeconds must report the ACTUAL measured span, not the configured
// window — that is what tells a reader how much to trust the number.
func TestWindowSecondsReportsActualSpan(t *testing.T) {
	tr := New(Config{Window: 5 * time.Second})
	now := feed(tr, base, 3*time.Second, 30, 100*time.Millisecond, 0)
	if h := tr.Health(now); math.Abs(h.WindowSeconds-3) > 0.2 {
		t.Errorf("WindowSeconds = %.2f, want ~3.0 (actual span, not the 5s config)", h.WindowSeconds)
	}

	// Past the window, the span saturates at the configured length.
	now = feed(tr, now, 10*time.Second, 30, 100*time.Millisecond, 90)
	if h := tr.Health(now); h.WindowSeconds > 5.2 {
		t.Errorf("WindowSeconds = %.2f, want <= ~5.0 (window bound)", h.WindowSeconds)
	}
}

// The reading is a snapshot; a runner that wedges stops refreshing it. Age is
// how a consumer notices, so it must be measured against the caller's clock.
func TestAgeTracksNewestSample(t *testing.T) {
	tr := New(Config{})
	now := feed(tr, base, 3*time.Second, 30, 10*time.Millisecond, 0)
	h := tr.Health(now)

	if got := h.Age(now); got > 20*time.Millisecond {
		t.Errorf("Age at measurement time = %v, want ~0", got)
	}
	if got := h.Age(now.Add(45 * time.Second)); got != 45*time.Second {
		t.Errorf("Age 45s later = %v, want 45s", got)
	}
	// A clock that slipped backwards must not produce a negative age.
	if got := h.Age(now.Add(-time.Minute)); got != 0 {
		t.Errorf("Age with a backwards clock = %v, want 0", got)
	}
	if got := (Health{}).Age(now); got != 0 {
		t.Errorf("zero-value Age = %v, want 0", got)
	}
}

func TestEmptyTrackerIsUnknown(t *testing.T) {
	tr := New(Config{})
	h := tr.Health(base)
	if h.Status != StatusUnknown {
		t.Errorf("status = %q, want %q", h.Status, StatusUnknown)
	}
	if h.Samples != 0 || h.Confident || h.ObservedHz != 0 {
		t.Errorf("unexpected non-zero reading: %+v", h)
	}
	if !h.MeasuredAt.IsZero() {
		t.Errorf("MeasuredAt = %v, want zero", h.MeasuredAt)
	}
	if h.ExpectedHz != ExpectedTickHz {
		t.Errorf("ExpectedHz = %v, want %v even with no data", h.ExpectedHz, ExpectedTickHz)
	}
}

func TestResetClearsState(t *testing.T) {
	tr := New(Config{})
	now := feed(tr, base, 5*time.Second, 30, 10*time.Millisecond, 0)
	if tr.Health(now).Status != StatusOK {
		t.Fatal("precondition: expected a healthy reading before Reset")
	}

	tr.Reset()
	h := tr.Health(now)
	if h.Status != StatusUnknown || h.Samples != 0 {
		t.Errorf("after Reset: status %q samples %d, want %q / 0", h.Status, h.Samples, StatusUnknown)
	}
}

// The ring is bounded; a Live loop polling at ~100Hz for a long match must not
// grow it, and the reading must stay correct once it saturates.
func TestRingStaysBoundedAndAccurate(t *testing.T) {
	tr := New(Config{})
	capacity := len(tr.ring)
	now := feed(tr, base, 60*time.Second, 30, 10*time.Millisecond, 0)

	if len(tr.ring) != capacity {
		t.Errorf("ring grew from %d to %d", capacity, len(tr.ring))
	}
	if tr.count > capacity {
		t.Fatalf("count %d exceeds capacity %d", tr.count, capacity)
	}
	h := tr.Health(now)
	if h.Status != StatusOK || math.Abs(h.ObservedHz-30) > 0.3 {
		t.Errorf("after 60s: status %q observed %.2f, want ok / ~30.0", h.Status, h.ObservedHz)
	}
}

func TestConfigDefaultsFillZeroFields(t *testing.T) {
	tr := New(Config{})
	if tr.cfg.ExpectedHz != ExpectedTickHz {
		t.Errorf("ExpectedHz = %v, want %v", tr.cfg.ExpectedHz, ExpectedTickHz)
	}
	if tr.cfg.Window != DefaultWindow || tr.cfg.MinSpan != DefaultMinSpan {
		t.Errorf("Window/MinSpan = %v/%v, want %v/%v", tr.cfg.Window, tr.cfg.MinSpan, DefaultWindow, DefaultMinSpan)
	}
	if tr.cfg.MinSamples != DefaultMinSamples || tr.cfg.StallAfter != DefaultStallAfter {
		t.Errorf("MinSamples/StallAfter = %v/%v, want %v/%v",
			tr.cfg.MinSamples, tr.cfg.StallAfter, DefaultMinSamples, DefaultStallAfter)
	}
	if tr.cfg.DegradedBelow != DefaultDegradedBelow {
		t.Errorf("DegradedBelow = %v, want %v", tr.cfg.DegradedBelow, DefaultDegradedBelow)
	}
}

// A non-CE plugin with a different tick rate reuses this package by config.
func TestExpectedHzIsConfigurable(t *testing.T) {
	tr := New(Config{ExpectedHz: 60})
	now := feed(tr, base, 6*time.Second, 60, 5*time.Millisecond, 0)

	h := tr.Health(now)
	if h.Status != StatusOK {
		t.Fatalf("status = %q, want %q (observed %.2f)", h.Status, StatusOK, h.ObservedHz)
	}
	if math.Abs(h.ObservedHz-60) > 0.5 {
		t.Errorf("ObservedHz = %.2f, want ~60.0", h.ObservedHz)
	}
	if h.ExpectedHz != 60 {
		t.Errorf("ExpectedHz = %v, want 60", h.ExpectedHz)
	}
}

// Wall-clock going backwards (NTP step) must reset rather than produce a
// negative span or a bogus rate.
func TestBackwardsClockResets(t *testing.T) {
	tr := New(Config{})
	now := feed(tr, base, 5*time.Second, 30, 10*time.Millisecond, 0)
	tr.Observe(9999, now.Add(-2*time.Second))

	if h := tr.Health(now); h.ObservedHz != 0 {
		t.Errorf("ObservedHz = %.2f across a backwards clock step, want 0", h.ObservedHz)
	}
}

func TestRoundHalfAwayFromZero(t *testing.T) {
	for _, tc := range []struct {
		in     float64
		places int
		want   float64
	}{
		{30.005, 2, 30.01},
		{29.994, 2, 29.99},
		{0.9666, 3, 0.967},
		// Exactly representable in float64, so this asserts the negative
		// half-away-from-zero branch rather than a binary-representation quirk.
		{-1.25, 1, -1.3},
		{30, 2, 30},
		{1.5, 0, 2},
	} {
		if got := round(tc.in, tc.places); got != tc.want {
			t.Errorf("round(%v, %d) = %v, want %v", tc.in, tc.places, got, tc.want)
		}
	}
}
