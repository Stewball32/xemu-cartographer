// Package events contains the change-emit event detectors for Halo: CE.
//
// One file per detection unit (kill_chain.go, damage.go, etc.). Each file's
// init() calls Register so the coordinator's Detect function can dispatch
// without explicit knowledge of which detectors exist. UpdateTickState
// likewise iterates a separate updater registry so each module owns its
// slice of TickState.Prev*.
//
// Constants from the parent haloce package (HandleEmpty, HandleIndexMask,
// DamageEmptySentinel) are duplicated as package locals to avoid an
// import cycle: haloce.Reader.DetectEvents calls into events, so events
// cannot import haloce.
package events

import (
	"encoding/json"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// Halo: CE handle / damage-table sentinels. Mirrored from the parent haloce
// package to keep events/ free of a cyclic import.
const (
	handleEmpty         uint32 = 0xFFFFFFFF
	handleIndexMask     uint32 = 0xFFFF
	damageEmptySentinel uint32 = 0xFFFFFFFF
)

// Context bundles the data every detector needs. Built once per Detect call.
type Context struct {
	Tick     uint32
	Instance string
	Snap     scraper.SnapshotPayload
	Result   scraper.TickResult
	State    *scraper.TickState
}

// emit wraps a payload into an "event" envelope using the context's tick /
// instance fields.
func (c *Context) emit(payload any) scraper.Envelope {
	b, _ := json.Marshal(payload)
	return scraper.Envelope{
		Type:     "event",
		Instance: c.Instance,
		Tick:     c.Tick,
		Payload:  b,
	}
}

// Detector emits zero or more envelopes for one change-detection concern
// (kills, damage, roster, etc.).
type Detector func(*Context) []scraper.Envelope

// Updater copies current values from result into state.Prev* so the next
// tick's detectors can diff against them. Order across updaters does not
// matter — they update disjoint slices of TickState.
type Updater func(state *scraper.TickState, result scraper.TickResult)

var (
	detectors []Detector
	updaters  []Updater
)

// RegisterDetector adds f to the dispatch list. Called from per-file init().
func RegisterDetector(f Detector) {
	detectors = append(detectors, f)
}

// RegisterUpdater adds f to the prev-state update list. Called from per-file
// init() alongside RegisterDetector.
func RegisterUpdater(f Updater) {
	updaters = append(updaters, f)
}

// Detect runs every registered detector against the current tick and returns
// the concatenated envelope list. Then runs every updater so the next tick
// can diff against this one's values.
//
// Note on registration order: detectors run in the order they were registered.
// Today the only cross-detector ordering concern is roster-change detection
// referencing the same snapshot the kill chain reads — both consume the
// snapshot read-only, so order is immaterial.
func Detect(tick uint32, instance string, snap scraper.SnapshotPayload, result scraper.TickResult, state *scraper.TickState) []scraper.Envelope {
	ctx := &Context{
		Tick:     tick,
		Instance: instance,
		Snap:     snap,
		Result:   result,
		State:    state,
	}
	var out []scraper.Envelope
	for _, d := range detectors {
		out = append(out, d(ctx)...)
	}
	for _, u := range updaters {
		u(state, result)
	}
	return out
}
