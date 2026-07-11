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

	// Map / Gametype are the READ (currently-loaded) values from the last
	// observation; SelectedMap / SelectedGametype are the player's picked intent
	// off the runner's selector; Ready is the player's arm+start request.
	Map              string `json:"map,omitempty"`
	Gametype         string `json:"gametype,omitempty"`
	SelectedMap      string `json:"selected_map,omitempty"`
	SelectedGametype string `json:"selected_gametype,omitempty"`
	Ready            bool   `json:"ready"`
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
		st.Ready = r.Ready()
		mp, gt := r.Selection()
		st.SelectedMap = mp.Name
		st.SelectedGametype = gt.Name
	}
	if hasEv {
		st.Screen = ev.Screen
		st.LastKind = ev.Kind
		st.LastIntent = ev.Intent
		st.LastKeys = ev.Keys
		st.Tick = ev.Tick
		st.Map = ev.Map
		st.Gametype = ev.Gametype
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

// SetReady sets an instance's player-scoped start request. Returns false when no
// runner is attached.
func (reg *Registry) SetReady(instance string, ready bool) bool {
	r, ok := reg.Get(instance)
	if !ok {
		return false
	}
	r.SetReady(ready)
	return true
}

// SetSelection records the player's map / gametype pick for an instance. Returns
// false when no runner is attached or its selector isn't mutable.
func (reg *Registry) SetSelection(instance, mapName, gametypeName string) bool {
	r, ok := reg.Get(instance)
	if !ok {
		return false
	}
	return r.SetSelection(mapName, gametypeName)
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
