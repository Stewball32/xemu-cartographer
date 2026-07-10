package hostrunner

import "sync"

// Status is the arbitration + last-activity view returned by the control
// endpoint (routes/scraper). It's built from the last observable event, so it
// needs no extra locking on the single-goroutine runner state.
type Status struct {
	Instance        string   `json:"instance"`
	Present         bool     `json:"present"`
	Authority       string   `json:"authority"`
	Screen          string   `json:"screen,omitempty"`
	LastKind        string   `json:"last_kind,omitempty"`
	LastIntent      string   `json:"last_intent,omitempty"`
	LastKeys        []string `json:"last_keys,omitempty"`
	Tick            uint32   `json:"tick"`
	MachineCount    int      `json:"machine_count"`
	TeamCount       int      `json:"team_count"`
	CountdownActive bool     `json:"countdown_active"`
	ReadyToStart    bool     `json:"ready_to_start"`
}

// Registry owns the per-instance runners and is itself the runners' EventSink:
// it captures the last event per instance (for Status) and fans out to a
// downstream sink (the WS admin-room broadcaster). Concurrency-safe so control
// endpoints (a request goroutine) and the scraper loop (the tick goroutine)
// share it. This is the seam the control endpoints call (routes/scraper).
type Registry struct {
	mu         sync.RWMutex
	runners    map[string]*Runner
	last       map[string]RunnerEvent
	downstream EventSink
}

// NewRegistry creates a registry that fans observable events out to downstream
// (nil = drop after capturing for Status).
func NewRegistry(downstream EventSink) *Registry {
	return &Registry{
		runners:    map[string]*Runner{},
		last:       map[string]RunnerEvent{},
		downstream: downstream,
	}
}

// Register adds (or replaces) an instance's runner. Create the runner with this
// Registry as its EventSink so its events flow through Emit.
func (reg *Registry) Register(instance string, r *Runner) {
	reg.mu.Lock()
	reg.runners[instance] = r
	reg.mu.Unlock()
}

// Remove drops an instance's runner + last event (on scraper Stop).
func (reg *Registry) Remove(instance string) {
	reg.mu.Lock()
	delete(reg.runners, instance)
	delete(reg.last, instance)
	reg.mu.Unlock()
}

// Get returns an instance's runner.
func (reg *Registry) Get(instance string) (*Runner, bool) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	r, ok := reg.runners[instance]
	return r, ok
}

// Emit is the EventSink: capture the last event, then fan out downstream.
func (reg *Registry) Emit(ev RunnerEvent) {
	reg.mu.Lock()
	reg.last[ev.Instance] = ev
	down := reg.downstream
	reg.mu.Unlock()
	if down != nil {
		down.Emit(ev)
	}
}

// Status returns the control view for an instance.
func (reg *Registry) Status(instance string) Status {
	reg.mu.RLock()
	r, present := reg.runners[instance]
	ev, hasEv := reg.last[instance]
	reg.mu.RUnlock()

	st := Status{Instance: instance, Present: present}
	if present {
		st.Authority = r.Arbiter().Authority().String()
	}
	if hasEv {
		st.Screen = ev.Screen
		st.LastKind = ev.Kind
		st.LastIntent = ev.Intent
		st.LastKeys = ev.Keys
		st.Tick = ev.Tick
		st.MachineCount = ev.MachineCount
		st.TeamCount = ev.TeamCount
		st.CountdownActive = ev.CountdownActive
		st.ReadyToStart = ev.ReadyToStart
		if !present {
			st.Authority = ev.Authority
		}
	}
	return st
}

// SetAuthority sets an instance's arbitration state. Returns false if there's no
// runner for the instance.
func (reg *Registry) SetAuthority(instance string, auth Authority) bool {
	r, ok := reg.Get(instance)
	if !ok {
		return false
	}
	r.Arbiter().Set(auth)
	return true
}

// SinkFunc adapts a plain function to an EventSink (the WS broadcaster wiring).
type SinkFunc func(RunnerEvent)

func (f SinkFunc) Emit(e RunnerEvent) { f(e) }

// ParseAuthority maps the control endpoint's string to an Authority.
func ParseAuthority(s string) (Authority, bool) {
	switch s {
	case "runner":
		return AuthRunner, true
	case "admin":
		return AuthAdmin, true
	case "disabled":
		return AuthDisabled, true
	default:
		return AuthRunner, false
	}
}
