// Package manager owns the per-instance scraper lifecycle: it opens an
// xemu.Instance and runs a phase-driven goroutine that broadcasts
// current_state / state_update / event envelopes (M5 stage 5c) to a
// per-instance host:<name> room, while a single per-Manager aggregator
// goroutine maintains a cross-instance host:all summary feed.
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
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/capture"
	"github.com/Stewball32/xemu-cartographer/internal/vncinput"
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

	// offsetSetResolver maps an instance name to its assigned offset-set id
	// ("" = the detected game's baseline). Injected via SetOffsetSetResolver
	// from main.go, where the instance→ISO-record linkage lives (podman
	// GameISO basename = the isos record id under the managed ingest model).
	// Read at reader bind time — a box provisioned from a catalog row with an
	// explicit offset_set rides that set; everything else rides the baseline.
	offsetSetResolver func(instance string) string

	// policyMu guards policies. Held separately from mu so the loader
	// can update the Manager-level snapshot without contending with
	// Start/Stop on the runners map. reloadMu serialises full reloads
	// — only one ReloadCapturePolicies pass runs at a time so a slow
	// FindAllRecords can't race with a fast one and leave runners on
	// stale slices.
	policyMu sync.RWMutex
	policies []capture.Policy
	reloadMu sync.Mutex

	// Host-runner wiring (player-hosting, ADR-0003). Set once at boot via
	// SetHostRunner before any Start. When hostEnabled, each Start attaches a
	// state-aware hostrunner.Runner (ticked from the scraper loop goroutine) and
	// dials a per-instance vncinput.Pump as its input channel, resolving the
	// container's websockify URL via hostURL. All three are read-only after boot
	// so no lock is needed. hostReg may be non-nil while hostEnabled is false —
	// the control/play endpoints then just report "no runner attached".
	hostReg     *hostrunner.Registry
	hostURL     func(name string) (wsURL string, ok bool)
	hostEnabled bool
	// hostDrive is the host/client scoping gate: it decides whether a freshly
	// attached runner AUTO-DRIVES its box (AuthRunner) or attaches observe-only
	// (AuthDisabled). nil drives every box (pre-scoping behavior). Read-only
	// after boot (set via SetHostDrivePolicy before any Start), so no lock.
	hostDrive func(name string) bool
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

// AvailableMaps returns the LIVE per-instance map/gametype carousel for the
// player-hosting map picker (refinement 2). It reads the runner's cache — which
// is filled by the carousel enumeration (SetAvailableMaps) — and NEVER a stock
// table: an instance with no enumeration yet reports Available=false with empty
// lists, so a modded disc's custom maps are never masked by a hardcoded set.
// Returns a zero (unavailable) MapList for an unknown instance.
func (m *Manager) AvailableMaps(name string) scraperiface.MapList {
	m.mu.Lock()
	r, ok := m.runners[name]
	m.mu.Unlock()
	if !ok {
		return scraperiface.MapList{}
	}
	c := r.readCache()
	return scraperiface.MapList{
		Available: len(c.AvailableMaps) > 0 || len(c.AvailableGametypes) > 0,
		Maps:      c.AvailableMaps,
		Gametypes: c.AvailableGametypes,
	}
}

// SetAvailableMaps records the live-enumerated map/gametype carousel for an
// instance. The enumeration itself — a guest-memory read of the CE map-select
// carousel while the runner is parked there, and/or a parse of the instance's
// disc / HDD `.map` files — is the remaining per-instance live read (it needs
// offset/disc work + a live box to verify); this is the seam it writes into, so
// nothing downstream assumes a fixed map set. No-op for an unknown instance.
func (m *Manager) SetAvailableMaps(name string, maps, gametypes []scraperiface.MapOption) {
	m.mu.Lock()
	r, ok := m.runners[name]
	m.mu.Unlock()
	if !ok {
		return
	}
	r.withCache(func(c *instanceCache) {
		c.AvailableMaps = maps
		c.AvailableGametypes = gametypes
	})
}

// SetHostRunner wires the player-hosting subsystem (ADR-0003). reg owns the
// per-instance runners + fans the observable stream to the admin WS room;
// urlForInstance resolves a container name to its websockify URL (nil / not-ok →
// the runner attaches with no input channel, ticking + emitting state but
// pressing nothing); enabled gates whether runners are attached at all. Call
// once before any Start (i.e. before the discovery watcher runs). Passing a
// non-nil reg with enabled=false keeps the control/play endpoints functional
// (they report "no runner attached") without auto-driving any instance.
func (m *Manager) SetHostRunner(reg *hostrunner.Registry, urlForInstance func(name string) (string, bool), enabled bool) {
	m.hostReg = reg
	m.hostURL = urlForInstance
	m.hostEnabled = enabled
}

// SetHostDrivePolicy installs the host/client scoping predicate. drive(name)
// reports whether a box should be AUTO-DRIVEN by its runner: true → the runner
// attaches AuthRunner and navigates the host create-game sequence (today's
// every-box behavior); false → it attaches observe-only (AuthDisabled), so it
// still ticks, enumerates its lobby for /api/play/options, and emits state to
// the admin stream, but CanEmit()==false and it presses NOTHING.
//
// This is the fix for the pod-hijack: without it, every discovered box with a
// detected game gets driven, so an admin-created client pod meant to JOIN a
// System Link lobby is yanked into hosting its own create-game flow. With a
// policy that drives only player-provisioned "play-<uid>" boxes, client/admin
// boxes are left alone until an admin promotes one via the host control
// endpoint (Registry.SetAuthority). nil (the default) drives every box.
// Read-only after boot — call once before any Start.
func (m *Manager) SetHostDrivePolicy(drive func(name string) bool) {
	m.hostDrive = drive
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

	// InitWait, not Init: discovery fires the moment the QMP socket appears,
	// which is seconds before the guest has page tables. Init would either fail
	// outright (no scraper for the life of the box) or bind translations against
	// unsettled memory. The gate waits for the guest instead — see InitWait.
	inst := &xemu.Instance{Name: name, QMPSock: sock}
	if err := inst.InitWait(context.Background(), scraper.DetectionGVAs()); err != nil {
		return fmt.Errorf("scraper: init xemu instance: %w", err)
	}

	r := newRunner(name, sock, hostRoom, m.agg, inst, func() string {
		if m.offsetSetResolver == nil {
			return ""
		}
		return m.offsetSetResolver(name)
	})

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

	// Hand the runner the Manager's current capture-policy snapshot so
	// its first tick already has the right demand evaluation. A reload
	// firing concurrently re-pushes (last write wins); the worst case
	// is a single tick on a slightly stale slice.
	r.setPolicies(m.getCapturePolicies())

	// Seed the aggregator immediately so host:all subscribers see a fresh
	// Idle entry without waiting for the runner's first heartbeat tick.
	m.agg.post(summaryUpdate{
		Instance: name,
		Snapshot: &hostSummary{Instance: name, Phase: PhaseIdle},
	})

	// Attach the player-hosting runner (ADR-0003). Created before the loop
	// goroutine starts so r.host / r.hostPump are visible to it without a race
	// (goroutine start is a happens-before edge). The runner is TICKED from the
	// scraper loop goroutine (see tickHost in loop.go) so it never touches a
	// GameReader from another goroutine; its input pump does the blocking RFB
	// writes off the loop goroutine.
	m.attachHostRunner(r)

	go r.loop(m.svc)
	return nil
}

// attachHostRunner builds and registers the per-instance host runner + its
// vncinput input pump when host-running is enabled. No-op when disabled or
// unwired. Called from Start before the loop goroutine launches.
func (m *Manager) attachHostRunner(r *runner) {
	if !m.hostEnabled || m.hostReg == nil {
		return
	}
	var input hostrunner.Input
	if m.hostURL != nil {
		if url, ok := m.hostURL(r.name); ok && url != "" {
			// Pump lifetime is bound to the runner's context (cancelled on Stop);
			// Stop also Closes it explicitly so teardown blocks on the goroutine.
			pump := vncinput.NewPump(r.ctx, url)
			r.hostPump = pump
			input = pump // *vncinput.Pump satisfies hostrunner.Input
		}
	}
	hr := hostrunner.New(hostrunner.Config{
		Instance: r.name,
		// Empty selector → the runner parks at map-select until a player picks
		// (refinement 1); it never auto-hosts a default map.
		Selector: hostrunner.NewAtomicSelector(),
	}, input, m.hostReg)
	// Host/client scoping (pod-hijack fix). Only a DESIGNATED host box is
	// auto-driven. A non-designated box — e.g. an admin-created client pod that
	// will JOIN the host's System Link lobby — attaches observe-only: its arbiter
	// starts AuthDisabled so CanEmit()==false and the runner never presses a key,
	// yet it still ticks, enumerates its lobby (for /api/play/options), and emits
	// state. An admin can promote it at runtime via the host control endpoint
	// (Registry.SetAuthority). nil policy = drive every box (pre-scoping default).
	if m.hostDrive != nil && !m.hostDrive(r.name) {
		hr.Arbiter().Set(hostrunner.AuthDisabled)
	}
	r.host = hr
	m.hostReg.Register(r.name, hr)
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

	// Tear down the host runner + input pump after the loop goroutine has
	// exited (so no in-flight tickHost touches r.host). Registry.Remove drops the
	// runner + its last event; pump.Close blocks until its goroutine stops.
	if m.hostReg != nil {
		m.hostReg.Remove(name)
	}
	if r.hostPump != nil {
		_ = r.hostPump.Close()
	}

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
		Name:     name,
		TitleID:  c.TitleID,
		Title:    c.Title,
		XboxName: c.XboxName,
		Running:  true,
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
			Events:   c.PreviousGame.Events,
			EndedAt:  c.PreviousGame.EndedAt,
		}
	}

	return scraperiface.InspectState{
		Info:       info,
		Running:    true,
		Phase:      string(c.Phase),
		LastReadAt: c.LastReadAt,
		// state_inputs / score_probe moved off the per-tick cache when
		// probe split out as its own on-demand envelope class. The
		// fields stay on InspectState for v1 wire-shape stability but
		// always render as empty maps — clients that want live probe
		// data now use the request_probe WS handler (see probe.go).
		StateInputs:  scraper.StateInputs{},
		ScoreProbe:   scraper.ScoreProbe{},
		GameData:     c.GameData,
		LatestTick:   c.LatestTick,
		RecentEvents: c.Events,
		PreviousGame: prev,
	}, true
}

// JoinReplayMessages returns one current_state envelope per runner, each
// addressed to that runner's host:<name> room. Retained for the
// request_state handler until M5 stage 5d narrows it to a single-room
// reply.
//
// M5 stage 5a: bytes are built on demand from each runner's instanceCache
// rather than pulled from a pre-marshaled bytes cache.
// M5 stage 5b: the Room field on each replay message is the per-instance
// host:<name> room (was "overlay").
// M5 stage 5c: payload is the new current_state envelope shape — full
// instanceCache snapshot, not just GameData — so a late-joining client
// gets phase, identity, freshness, current game data, recent events, and
// previous_game in a single message.
func (m *Manager) JoinReplayMessages() [][]byte {
	m.mu.Lock()
	runners := make([]*runner, 0, len(m.runners))
	for _, r := range m.runners {
		runners = append(runners, r)
	}
	m.mu.Unlock()

	out := make([][]byte, 0, len(runners)*4)
	for _, r := range runners {
		for _, m := range r.classEnvelopeMessages() {
			out = append(out, m.Bytes)
		}
	}
	return out
}

// JoinReplayForInstance returns one envelope per applicable v2 class for
// a single runner, or nil if the named runner doesn't exist. Used by the
// join_room handler when a client subscribes to host:<name> (no class
// suffix — returns all classes) so a late joiner can render immediately
// rather than waiting for the next per-class broadcast.
func (m *Manager) JoinReplayForInstance(name string) [][]byte {
	m.mu.Lock()
	r, ok := m.runners[name]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	msgs := r.classEnvelopeMessages()
	out := make([][]byte, 0, len(msgs))
	for _, mm := range msgs {
		out = append(out, mm.Bytes)
	}
	return out
}

// JoinReplayForInstanceClass returns the envelope for a single v2 class
// of a single runner, or nil if the runner or class has no current data.
// Used by the join_room handler when a client subscribes to
// host:<name>:<class> so they get exactly the cached envelope for that
// class — no extra classes they didn't ask for.
func (m *Manager) JoinReplayForInstanceClass(name, class string) [][]byte {
	m.mu.Lock()
	r, ok := m.runners[name]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	for _, mm := range r.classEnvelopeMessages() {
		if mm.Class == class {
			return [][]byte{mm.Bytes}
		}
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

// SetOffsetSetResolver injects the instance→offset-set-id resolver consulted
// when a runner binds its GameReader (see Manager.offsetSetResolver). Call
// before Start; nil (or never calling) means every instance uses its game's
// baseline offsets.
func (m *Manager) SetOffsetSetResolver(fn func(instance string) string) {
	m.offsetSetResolver = fn
}
