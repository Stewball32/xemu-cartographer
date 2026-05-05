// Package manager owns the per-instance scraper lifecycle: it opens an
// xemu.Instance and runs a phase-driven goroutine that broadcasts
// game-data / tick / event envelopes to a per-instance host:<name> room,
// while a single per-Manager aggregator goroutine maintains a cross-
// instance host:all summary feed.
// (Wire envelope type strings stay "snapshot" / "tick" / "event" until
// M5 stage 5c — see envelopeType* constants in loop.go.)
//
// One Manager per server. Routes (/api/admin/scraper/*) and the discovery
// watcher (internal/discovery → onAdd) call Start/Stop/List through the
// scraperiface.Service interface.
//
// M5 stage 5a (single-runner-per-lifetime, OQ4): Start always creates a
// runner — it does NOT call scraper.Detect upfront. The runner enters the
// Idle phase and polls the XBE title ID itself; on detection it binds a
// GameReader and transitions to Ready. This means Start succeeds whenever
// xemu / QMP is reachable, even if the running XBE isn't registered with
// the scraper package (the runner sits in Idle, visible via Inspect).
//
// M5 stage 5b: instance names are validated at Start via the
// rooms.RoomForInstance chokepoint (rejects "all" and other reserved
// strings); the per-instance host room name is cached on the runner so
// loop broadcasts don't re-derive it per tick.
package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// ErrAlreadyRunning is returned from Start when name is already in use.
var ErrAlreadyRunning = errors.New("scraper already running")

// ErrInvalidName is the sentinel wrapped around input-validation failures
// from the rooms.RoomForInstance chokepoint (M5 stage 5b). Lets HTTP route
// handlers distinguish "client passed a bad name" (→ 400) from "QMP init
// failed" (→ 502) without depending on rooms-package internals.
var ErrInvalidName = errors.New("scraper: invalid instance name")

// Manager owns a name → runner map and dispatches lifecycle operations.
// Implements scraperiface.Service via structural typing.
type Manager struct {
	svc *guards.Services

	mu      sync.Mutex
	runners map[string]*runner

	agg *aggregator
}

// New constructs a Manager that broadcasts via svc.WS and starts a host:all
// aggregator goroutine. svc may be nil for tests; in that case broadcasts
// become no-ops but the aggregator still runs (it short-circuits when
// svc.WS is nil). Call Close() on shutdown to stop the aggregator.
func New(svc *guards.Services) *Manager {
	m := &Manager{
		svc:     svc,
		runners: make(map[string]*runner),
		agg:     newAggregator(svc),
	}
	go m.agg.run()
	return m
}

// Close stops the host:all aggregator goroutine. Idempotent — but does NOT
// stop running scraper runners; the caller should iterate List + Stop
// before calling Close (see cmd/server/main.go OnTerminate).
func (m *Manager) Close() {
	if m.agg != nil {
		m.agg.stop()
	}
}

// Start spins up a runner for the named instance. It opens the xemu instance
// at sock and launches the phase-driven goroutine. The runner enters Idle
// and self-detects the running XBE; no upfront scraper.Detect call is made.
//
// M5 stage 5b: name is validated through rooms.RoomForInstance — reserved
// suffixes (currently "all"), names containing ":" or whitespace, and the
// empty string are rejected before any state is mutated. Both the discovery
// watcher's auto-start path and the manual /api/admin/scraper/start route
// flow through here, so this is the single trust boundary for instance-name
// → room-name derivation.
//
// Returns ErrAlreadyRunning if name is already in use, the chokepoint error
// from rooms.RoomForInstance for invalid names, or whatever error
// xemu.Instance.Init surfaced.
func (m *Manager) Start(name, sock string) error {
	if sock == "" {
		return errors.New("scraper: sock required")
	}

	hostRoom, err := rooms.RoomForInstance(name)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidName, err)
	}

	m.mu.Lock()
	if _, exists := m.runners[name]; exists {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}
	m.mu.Unlock()

	inst := &xemu.Instance{Name: name, QMPSock: sock}
	if err := inst.Init(scraper.DetectionGVAs()); err != nil {
		return fmt.Errorf("scraper: init xemu instance: %w", err)
	}

	r := newRunner(name, sock, hostRoom, m.agg, inst)

	m.mu.Lock()
	// Re-check under lock — guards against two concurrent Start calls racing.
	if _, exists := m.runners[name]; exists {
		m.mu.Unlock()
		r.cancel()
		inst.Close()
		return ErrAlreadyRunning
	}
	m.runners[name] = r
	m.mu.Unlock()

	// Seed the aggregator immediately so host:all subscribers see a fresh
	// Idle entry without waiting for the runner's first heartbeat tick.
	m.agg.post(summaryUpdate{
		Instance: name,
		Snapshot: &hostSummary{Instance: name, Phase: PhaseIdle},
	})

	go r.loop(m.svc)
	return nil
}

// Stop cancels the named runner's context, closes its xemu.Instance, and
// removes it from the registry. Returns nil if name is unknown (idempotent).
// Posts a Removed update to the host:all aggregator so the cross-instance
// summary view evicts the entry promptly.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	r, ok := m.runners[name]
	if ok {
		delete(m.runners, name)
	}
	m.mu.Unlock()

	if !ok {
		return nil
	}
	r.cancel()
	<-r.done
	m.agg.post(summaryUpdate{Instance: name, Removed: true})
	return nil
}

// List returns one Info per currently-tracked runner. Sorted by name for
// stable output.
func (m *Manager) List() []scraperiface.Info {
	m.mu.Lock()
	infos := make([]scraperiface.Info, 0, len(m.runners))
	for _, r := range m.runners {
		infos = append(infos, r.info())
	}
	m.mu.Unlock()
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}

// InstanceState returns a per-runner view (game title, Xbox name) for the
// container detail page. Returns (zero, false) when no runner is attached.
// Reads from the runner's cache so the values stay correct across phase
// transitions (e.g. Title is empty in Idle).
func (m *Manager) InstanceState(name string) (scraperiface.InstanceState, bool) {
	m.mu.Lock()
	r, ok := m.runners[name]
	m.mu.Unlock()
	if !ok {
		return scraperiface.InstanceState{Name: name}, false
	}
	c := r.readCache()
	return scraperiface.InstanceState{
		Name:      name,
		TitleID:   c.TitleID,
		Title: c.Title,
		XboxName:  c.XboxName,
		Running:   true,
	}, true
}

// Inspect returns the runner's deep-dive cached state for the debug page.
// Reads through readCache(); never touches r.reader or r.inst, which the
// loop accesses without synchronisation. Returns (zero, false) when no
// runner is attached for name.
func (m *Manager) Inspect(name string) (scraperiface.InspectState, bool) {
	m.mu.Lock()
	r, ok := m.runners[name]
	m.mu.Unlock()
	if !ok {
		return scraperiface.InspectState{Info: scraperiface.Info{Name: name}}, false
	}

	info := r.info()
	c := r.readCache()

	var prev *scraperiface.PreviousGameInfo
	if c.PreviousGame != nil {
		prev = &scraperiface.PreviousGameInfo{
			GameData: c.PreviousGame.GameData,
			Events:    c.PreviousGame.Events,
			EndedAt:   c.PreviousGame.EndedAt,
		}
	}

	return scraperiface.InspectState{
		Info:         info,
		Running:      true,
		Phase:        string(c.Phase),
		LastReadAt:   c.LastReadAt,
		CurrentState: c.GameState,
		StateInputs:  c.StateInputs,
		ScoreProbe:   c.ScoreProbe,
		GameData:    c.GameData,
		LatestTick:   c.LatestTick,
		RecentEvents: c.Events,
		PreviousGame: prev,
	}, true
}

// JoinReplayMessages returns one game-data envelope per runner, addressed
// to that runner's host:<name> room. Retained for the request_state
// handler until M5 stage 5d narrows it to a single-room reply.
//
// M5 stage 5a: bytes are built on demand from each runner's instanceCache
// rather than pulled from a pre-marshaled bytes cache.
// M5 stage 5b: the Room field on each replay message is the per-instance
// host:<name> room (was "overlay"). Wire format is otherwise unchanged —
// the bytes still encode legacy Type:"snapshot" envelopes (M5 stage 5c
// replaces "snapshot" with "current_state").
func (m *Manager) JoinReplayMessages() [][]byte {
	m.mu.Lock()
	runners := make([]*runner, 0, len(m.runners))
	for _, r := range m.runners {
		runners = append(runners, r)
	}
	m.mu.Unlock()

	out := make([][]byte, 0, len(runners))
	for _, r := range runners {
		if msgBytes, ok := buildJoinReplayMessage(r); ok {
			out = append(out, msgBytes)
		}
	}
	return out
}

// JoinReplayForInstance returns the join-replay bytes for a single runner,
// or an empty slice if the named runner has no cached game data (or doesn't
// exist). Used by the join_room handler when a client subscribes to
// host:<name> so the overlay can render immediately rather than waiting
// for the next state-transition broadcast.
func (m *Manager) JoinReplayForInstance(name string) [][]byte {
	m.mu.Lock()
	r, ok := m.runners[name]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	if msgBytes, ok := buildJoinReplayMessage(r); ok {
		return [][]byte{msgBytes}
	}
	return nil
}

// JoinReplayForHostAll returns one envelope-bytes message representing the
// current host:all summary cache. Used by the join_room handler when a
// client subscribes to host:all so it can populate its instance list
// without waiting for the next aggregator coalesce tick.
func (m *Manager) JoinReplayForHostAll() [][]byte {
	return m.agg.joinReplay()
}

// buildJoinReplayMessage marshals one runner's cached GameData into a
// websocket.Message addressed to the runner's host:<name> room. Returns
// (nil, false) when the runner has no cached game data or marshaling fails.
func buildJoinReplayMessage(r *runner) ([]byte, bool) {
	c := r.readCache()
	if c.GameData == nil {
		return nil, false
	}
	env := scraper.MakeEnvelope(envelopeTypeGameData, r.name, c.EngineTick, *c.GameData)
	envBytes, err := json.Marshal(env)
	if err != nil {
		return nil, false
	}
	msg := websocket.Message{
		Type:    "scraper",
		Room:    r.hostRoom,
		Payload: envBytes,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, false
	}
	return msgBytes, true
}
