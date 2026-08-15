package manager

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/hosthealth"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/roster"
)

// The pure rate maths is covered in internal/hosthealth. These tests cover the
// WIRING: that recordIteration actually drives the tracker, that the snapshot
// lands in the cache, and that it survives onto the game-class envelope — the
// three places this feature can silently become a no-op.

// recordIteration is the single call every phase makes on every successful
// read, so it's what has to feed the tracker. If it doesn't, host health stays
// StatusUnknown forever and the whole feature is dead on arrival.
func TestRecordIterationDrivesHostHealth(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	if got := r.readCache().HostHealth.Status; got != hosthealth.StatusUnknown {
		t.Fatalf("fresh runner status = %q, want %q", got, hosthealth.StatusUnknown)
	}

	// recordIteration stamps time.Now() internally, so drive it in real time
	// rather than trying to inject a clock through the loop.
	start := time.Now()
	tick := uint32(0)
	for time.Since(start) < 300*time.Millisecond {
		tick++
		r.recordIteration(tick)
		time.Sleep(2 * time.Millisecond)
	}

	h := r.readCache().HostHealth
	if h.Samples < 2 {
		t.Fatalf("Samples = %d, want the tracker to have been fed", h.Samples)
	}
	if h.MeasuredAt.IsZero() {
		t.Error("MeasuredAt is zero — the snapshot never reached the cache")
	}
	if h.ExpectedHz != hosthealth.ExpectedTickHz {
		t.Errorf("ExpectedHz = %v, want %v", h.ExpectedHz, hosthealth.ExpectedTickHz)
	}
	if h.ObservedHz <= 0 {
		t.Errorf("ObservedHz = %v, want a positive reading", h.ObservedHz)
	}
}

// An Idle-phase runner calls recordIteration(0) forever. That must read as
// stalled, never as a host running at 0Hz — the menu-vs-slow distinction is
// the entire reason the status field exists.
func TestIdleRunnerReportsStalledNotDegraded(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	// StallAfter is 1s of wall clock; use a tracker tuned short so the test
	// doesn't sleep for a second. Same code path, faster threshold.
	r.health = hosthealth.New(hosthealth.Config{StallAfter: 50 * time.Millisecond})
	for range 5 {
		r.recordIteration(0)
		time.Sleep(20 * time.Millisecond)
	}
	r.recordIteration(0)

	if got := r.readCache().HostHealth.Status; got != hosthealth.StatusStalled {
		t.Fatalf("idle runner status = %q, want %q", got, hosthealth.StatusStalled)
	}
}

// The reading has to reach consumers, which means surviving buildGamePayload
// and JSON marshalling with its field names intact.
func TestGameEnvelopeCarriesHostHealth(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	want := hosthealth.Health{
		Status:        hosthealth.StatusDegraded,
		ObservedHz:    27.4,
		ExpectedHz:    30,
		Ratio:         0.913,
		WindowSeconds: 5,
		Samples:       142,
		MeasuredAt:    time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC),
		Confident:     true,
	}
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseLive
		c.HostHealth = want
	})

	snap := r.readCache()
	got := buildGamePayload(&snap)
	if got.HostHealth != want {
		t.Fatalf("buildGamePayload host health = %+v, want %+v", got.HostHealth, want)
	}

	// Marshal through the real envelope path so a renamed/dropped JSON tag
	// fails here rather than silently in a browser.
	msgs := r.classEnvelopeMessages(roster.Config{})
	var gameMsg []byte
	for _, m := range msgs {
		if m.Class == "game" {
			gameMsg = m.Bytes
		}
	}
	if gameMsg == nil {
		t.Fatal("no game-class envelope emitted")
	}
	_, env := decodeClassEnvelope(t, gameMsg)

	var payload struct {
		HostHealth struct {
			Status        string  `json:"status"`
			ObservedHz    float64 `json:"observed_hz"`
			ExpectedHz    float64 `json:"expected_hz"`
			Ratio         float64 `json:"ratio"`
			WindowSeconds float64 `json:"window_seconds"`
			Samples       int     `json:"samples"`
			Confident     bool    `json:"confident"`
			MeasuredAt    string  `json:"measured_at"`
		} `json:"host_health"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal game payload: %v", err)
	}
	hh := payload.HostHealth
	if hh.Status != string(hosthealth.StatusDegraded) {
		t.Errorf("wire status = %q, want %q", hh.Status, hosthealth.StatusDegraded)
	}
	if hh.ObservedHz != 27.4 || hh.ExpectedHz != 30 || hh.Ratio != 0.913 {
		t.Errorf("wire rates = %.2f/%.2f ratio %.3f, want 27.40/30 ratio 0.913",
			hh.ObservedHz, hh.ExpectedHz, hh.Ratio)
	}
	if hh.WindowSeconds != 5 || hh.Samples != 142 || !hh.Confident {
		t.Errorf("wire confidence = window %.2f samples %d confident %v, want 5/142/true",
			hh.WindowSeconds, hh.Samples, hh.Confident)
	}
	if hh.MeasuredAt == "" {
		t.Error("wire measured_at is empty — consumers can't judge staleness")
	}
}
