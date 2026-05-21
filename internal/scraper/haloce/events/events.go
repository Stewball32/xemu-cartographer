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
	"time"

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
	Snap     scraper.GameData
	Result   scraper.TickResult
	State    *scraper.TickState
}

// emit wraps a payload into an "event" envelope using the context's tick /
// instance fields. Seq is a 0 placeholder — the real per-(instance, type)
// counter lands when runner emission is rewritten in a later v2 PR.
// Ts is captured automatically by the Envelope constructor here.
//
// All five typed emit helpers below (emitDeath, emitDamage, emitMedal,
// emitPlayerUpdate, emitGameUpdate) delegate here after populating
// EventCommon — detectors should prefer the typed helpers for compile-time
// shape checking.
func (c *Context) emit(payload any) scraper.Envelope {
	b, _ := json.Marshal(payload)
	return scraper.Envelope{
		V:        scraper.ProtocolVersion,
		Type:     "event",
		Instance: c.Instance,
		Seq:      0,
		Tick:     c.Tick,
		Ts:       time.Now(),
		Data:     b,
	}
}

// common builds the EventCommon block embedded into every v2 event payload.
// Seq is a 0 placeholder until per-(instance, type) seq tracking lands.
func (c *Context) common(eventType string) scraper.EventCommon {
	return scraper.EventCommon{
		Seq:       0,
		Tick:      c.Tick,
		At:        time.Now(),
		EventType: eventType,
	}
}

// emitDeath fills the EventCommon block and emits a DeathEvent envelope.
func (c *Context) emitDeath(e scraper.DeathEvent) scraper.Envelope {
	e.EventCommon = c.common(scraper.EventTypeDeath)
	return c.emit(e)
}

// emitDamage fills the EventCommon block and emits a DamageEvent envelope.
func (c *Context) emitDamage(e scraper.DamageEvent) scraper.Envelope {
	e.EventCommon = c.common(scraper.EventTypeDamage)
	return c.emit(e)
}

// emitMedal fills the EventCommon block and emits a MedalEvent envelope.
func (c *Context) emitMedal(e scraper.MedalEvent) scraper.Envelope {
	e.EventCommon = c.common(scraper.EventTypeMedal)
	return c.emit(e)
}

// emitPlayerUpdate fills the EventCommon block and emits a PlayerUpdateEvent
// envelope. The caller sets Kind plus the kind-specific optional fields.
func (c *Context) emitPlayerUpdate(e scraper.PlayerUpdateEvent) scraper.Envelope {
	e.EventCommon = c.common(scraper.EventTypePlayerUpdate)
	return c.emit(e)
}

// emitGameUpdate fills the EventCommon block and emits a GameUpdateEvent
// envelope. The caller sets Kind plus the kind-specific optional fields.
func (c *Context) emitGameUpdate(e scraper.GameUpdateEvent) scraper.Envelope {
	e.EventCommon = c.common(scraper.EventTypeGameUpdate)
	return c.emit(e)
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
// referencing the same GameData the kill chain reads — both consume it
// read-only, so order is immaterial.
func Detect(tick uint32, instance string, snap scraper.GameData, result scraper.TickResult, state *scraper.TickState) []scraper.Envelope {
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
