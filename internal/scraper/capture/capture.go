// Package capture defines the per-(instance, class) capture policy
// types and the most-specific-wins resolver used by the scraper manager
// to compute the capture side of demand.
//
// Pure data only — no PocketBase import. The manager owns the
// PB-record-to-Policy conversion and loads the policy list; this
// package just answers "given this list, what is the effective policy
// for (instance, class)?" and provides the enum constants for the
// fields persisted by the capture_policies collection (see
// internal/pocketbase/schema/capture_policies.go).
//
// See atlas/new_json/04-ground-up-rebuild.md §5 for the design rationale.
package capture

// Mode controls whether reads happen for a given (instance, class).
type Mode string

const (
	// ModeAuto reads iff a WS subscriber exists for the class.
	ModeAuto Mode = "auto"
	// ModeAlways reads regardless of WS subscribers; OR'd into demand.
	ModeAlways Mode = "always"
	// ModeNever hard-caps the class: never read, even when subscribers exist.
	ModeNever Mode = "never"
)

// Cadence controls poll rate for polled classes (tick, objects, debug,
// summary, game heartbeat). Event-driven classes (event, xbox, scenario,
// previous_game) ignore cadence — they fire when their thing happens.
type Cadence string

const (
	// CadenceDefault uses the runner's built-in cadence.
	CadenceDefault Cadence = "default"
	// CadenceEngine drives from the engine tick — never skip a fresh tick.
	CadenceEngine Cadence = "engine"
	Cadence30Hz   Cadence = "30hz"
	Cadence10Hz   Cadence = "10hz"
	Cadence5Hz    Cadence = "5hz"
	Cadence250ms  Cadence = "250ms"
	Cadence500ms  Cadence = "500ms"
	Cadence1s     Cadence = "1s"
)

// Wildcard is the sentinel string used in capture_policies rows to
// indicate "any instance" or "any class". Resolve treats Instance/Class
// equal to Wildcard as a lower-priority fallback than an exact match.
const Wildcard = "*"

// Policy is the resolved capture intent for one (instance, class).
// Instance/Class are the row's selectors; an empty Sink means
// broadcast-only (no persistence).
type Policy struct {
	Instance    string
	Class       string
	Mode        Mode
	Cadence     Cadence
	Sink        string
	Description string
}

// DefaultPolicy is the fallback returned by Resolve when no row matches.
// Mode auto means "respect WS subscribers"; cadence default means "use
// the runner's built-in rate"; no sink.
func DefaultPolicy() Policy {
	return Policy{Mode: ModeAuto, Cadence: CadenceDefault}
}

// Resolve returns the effective policy for (instance, class) under the
// most-specific-wins priority order documented in §5:
//
//  1. (instance, class)   — exact match
//  2. (*, class)          — class default across all instances
//  3. (instance, *)       — instance default across all classes
//  4. (*, *)              — global default
//  5. DefaultPolicy()     — built-in fallback (auto, default cadence)
//
// (*, class) ranks above (instance, *) because class-specific defaults
// usually express stronger intent than per-instance overrides; see the
// design doc for rationale. The returned Policy carries the matching
// row's Instance/Class values, not the requested ones — callers needing
// the lookup key should track it themselves.
func Resolve(policies []Policy, instance, class string) Policy {
	var exact, byClass, byInstance, global *Policy
	for i := range policies {
		p := &policies[i]
		switch {
		case p.Instance == instance && p.Class == class:
			exact = p
		case p.Instance == Wildcard && p.Class == class:
			byClass = p
		case p.Instance == instance && p.Class == Wildcard:
			byInstance = p
		case p.Instance == Wildcard && p.Class == Wildcard:
			global = p
		}
	}
	switch {
	case exact != nil:
		return *exact
	case byClass != nil:
		return *byClass
	case byInstance != nil:
		return *byInstance
	case global != nil:
		return *global
	default:
		return DefaultPolicy()
	}
}

// Demands reports whether the policy contributes to demand for reads.
// auto → no (WS subscribers decide); always → yes; never → no (and
// callers should treat it as a hard cap that overrides WS demand).
//
// IsHardCap exists for the runner-side aggregation step: when policy
// is `never`, the WS-side `RoomHasMembers` signal is ignored.
func (p Policy) Demands() bool { return p.Mode == ModeAlways }

// IsHardCap reports whether the policy forbids reads outright. When
// true, the read-gating aggregator must skip the class even if a WS
// subscriber is present.
func (p Policy) IsHardCap() bool { return p.Mode == ModeNever }
