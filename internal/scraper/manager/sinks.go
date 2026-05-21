package manager

import (
	"log"
	"sync"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/capture"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/sinks"
)

// sinkManager owns the per-class sink instances for one runner. Sinks
// are constructed from the runner's effective capture policies (the
// most-specific-wins result of capture.Resolve, for each class) and
// closed when the policy spec changes or the runner shuts down.
//
// applyPolicies is the single mutator; it accepts the full policy list
// pushed in by Manager.setCapturePolicies (which already covers WS
// reload races by serialising through reloadMu). write() is called
// from the runner loop on every demand-gated emit; it's RLock-cheap so
// classes without sinks pay nothing.
type sinkManager struct {
	instance string

	mu     sync.RWMutex
	active map[string]*activeSink
}

// activeSink pairs a Sink with the expanded spec string it was opened
// for. applyPolicies compares specs to decide whether an existing sink
// can be reused (same spec → keep) or must be closed and replaced.
type activeSink struct {
	spec string
	sink sinks.Sink
}

func newSinkManager(instance string) *sinkManager {
	return &sinkManager{
		instance: instance,
		active:   make(map[string]*activeSink),
	}
}

// applyPolicies reconciles the per-class sink map with the current
// effective policies. For each class:
//
//   - resolved policy with empty Sink → close + drop any existing sink
//   - resolved policy with new spec → close existing, open replacement
//   - resolved policy with same spec → keep existing instance (no churn)
//
// Classes not represented in the resolved set keep their existing sink.
// In practice the resolved set covers every v2 class, so a row removal
// surfaces as DefaultPolicy() with no Sink → close. Specs that fail
// to parse are logged and dropped from the active map for that class.
func (sm *sinkManager) applyPolicies(policies []capture.Policy) {
	vars := map[string]string{"instance": sm.instance}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, class := range allClasses {
		resolved := capture.Resolve(policies, sm.instance, class)
		spec := resolved.Sink

		existing := sm.active[class]
		if spec == "" {
			if existing != nil {
				_ = existing.sink.Close()
				delete(sm.active, class)
			}
			continue
		}

		if existing != nil && existing.spec == spec {
			continue
		}
		if existing != nil {
			_ = existing.sink.Close()
		}
		s, err := sinks.Parse(spec, vars)
		if err != nil {
			log.Printf("scraper[%s]: sink parse %s=%q: %v", sm.instance, class, spec, err)
			delete(sm.active, class)
			continue
		}
		if s == nil {
			delete(sm.active, class)
			continue
		}
		sm.active[class] = &activeSink{spec: spec, sink: s}
	}
}

// write tees envelopeBytes to the class's sink if one is configured.
// Lock is RLock so the runner's emit hot path doesn't block on
// concurrent reloads. Sink Write errors are logged but do not stop
// further writes — a transient disk-full shouldn't take down the
// runner loop.
func (sm *sinkManager) write(class string, envelopeBytes []byte) {
	sm.mu.RLock()
	a := sm.active[class]
	sm.mu.RUnlock()
	if a == nil {
		return
	}
	if err := a.sink.Write(envelopeBytes); err != nil {
		log.Printf("scraper[%s]: sink write %s: %v", sm.instance, class, err)
	}
}

// closeAll closes every active sink and clears the map. Called from
// the runner loop's shutdown defer; idempotent.
func (sm *sinkManager) closeAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, a := range sm.active {
		_ = a.sink.Close()
	}
	sm.active = make(map[string]*activeSink)
}

// allClasses lists every v2 envelope class the runner emits — used by
// applyPolicies so a row removal (effective DefaultPolicy() with empty
// Sink) closes the formerly-active sink for that class. Keep in sync
// with hello.go's broadcast surface.
var allClasses = []string{
	"xbox",
	"scenario",
	"game",
	"tick",
	"objects",
	"debug",
	"summary",
	"previous_game",
}
