package manager

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/capture"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// recentEventsCap caps the per-runner event log surfaced via the inspect
// endpoint and replayed to mid-match overlay joiners. Newest-first; older
// entries drop off the back.
const recentEventsCap = 50

// instanceCache is the per-runner authoritative source of truth introduced
// in M5 stage 5a. Holds phase, identity, freshness, current match data,
// recent event log, and the just-ended `previous_game` slot in one struct.
//
// All access goes through runner.cacheMu. Pointer / slice / map fields are
// replaced wholesale rather than mutated in place so consumers that copy the
// cache out under the lock can read their copy without further coordination.
//
// M5 stage 5c builds the wire envelopes (`current_state`, `state_update`)
// from this struct on demand via runner.buildCurrentStateEnvelope and
// runner.buildStateUpdateEnvelope — there is no pre-marshaled bytes cache.
type instanceCache struct {
	// Lifecycle.
	Phase     Phase
	StartedAt time.Time

	// Identity. TitleID is the most recently observed XBE title ID (0 if
	// no successful read yet). Title comes from the bound GameReader and
	// stays empty when no plugin is bound. XboxName is supplied by the
	// xbox/* system-snapshot pass (see runSystemSnapshot in loop.go) — it's
	// title-agnostic and survives Ready→Idle so the dashboard / between-
	// games view keeps showing the console name. The snapshot is throttled
	// at the runner level (see runner.lastSystemSnapshotAt) so renames pick
	// up automatically without per-call gating here.
	TitleID  uint32
	Title    string
	XboxName string

	// EEPROM-derived system info. Populated by runSystemSnapshot from kernel
	// globals at fixed GVAs (see internal/scraper/xbox/offsets.go), so all
	// fields are title-agnostic and stable across XBE swaps. Values are read
	// once per system-snapshot tick and survive Ready→Idle the same way
	// XboxName does.
	SerialNumber    string
	MACAddress      string
	VideoStandard   string
	TimeZoneBias    int32
	TimeZoneStdName string
	TimeZoneDltName string

	// XBE-certificate-derived fields. Populated by runSystemSnapshot from the
	// running XBE's on-disk certificate (always present even when no plugin
	// is bound — the kernel's XBE header is mapped from the moment an XBE
	// loads). Empty until the first successful read; refreshed on the same
	// 2.5s cadence as other system fields, so XBE swaps land in cache within
	// one snapshot.
	XBETitleName    string
	XBEVersion      uint32
	XBEGameRegion   uint32
	XBEDiskNumber   uint32
	XBEAllowedMedia uint32

	// Kernel-clock fields. Populated by runSystemSnapshot from the kernel's
	// KeSystemTime / KeInterruptTime globals. SystemTime is wall-clock UTC;
	// BootTime is when the guest booted; Uptime is the elapsed time since
	// boot. All three are read together so they don't skew against each
	// other.
	KernelSystemTime time.Time
	KernelBootTime   time.Time
	KernelUptime     time.Duration

	// Freshness. LastReadAt advances on every successful memory read of
	// any kind (title-ID poll, ReadGameState, ReadGameData, ReadTick).
	// EngineTick is the most recent engine tick; Iterations counts loop
	// iterations since Start.
	LastReadAt time.Time
	EngineTick uint32
	Iterations uint64

	// Match data — populated in Ready and Live, dropped on Ready→Idle.
	GameState  scraper.GameState
	GameData   *scraper.GameData
	LatestTick *scraper.TickPayload
	Events     []scraper.Envelope // newest-first; bounded by recentEventsCap

	// Just-ended match. Populated on Live→Ready transition (deferred so a
	// panic / ctx-cancel mid-match still moves the data); dropped on
	// Ready→Idle.
	PreviousGame *previousGame
}

// previousGame is the just-ended match captured on Live→Ready. Serialised as
// part of the current_state envelope's payload, so json tags determine the
// wire shape.
type previousGame struct {
	GameData *scraper.GameData  `json:"game_data,omitempty"`
	Events   []scraper.Envelope `json:"events,omitempty"`
	EndedAt  time.Time          `json:"ended_at"`
}

// runner owns one xemu instance for its lifetime: from Manager.Start (which
// always succeeds, regardless of whether the running XBE is registered)
// through Manager.Stop. The runner hot-swaps its GameReader as the running
// XBE's title-ID becomes recognised and unrecognised — see phase.go for the
// state machine.
//
// Fields outside cacheMu are accessed only from the loop goroutine. The
// reader's internal caches (tagNameCache, weaponTagDataCache, etc.) are
// not concurrent-safe, so anything that needs to look at reader state from
// another goroutine must go through the cache.
type runner struct {
	name string
	sock string
	inst *xemu.Instance

	// hostRoom is the per-instance WebSocket room name ("host:<name>")
	// scraper broadcasts target. Pre-validated at Manager.Start by the
	// rooms.RoomForInstance chokepoint and passed in here; the broadcast
	// helpers read it directly so loop.go doesn't re-derive it per tick.
	hostRoom string

	// agg is the Manager's host:all aggregator. Runners post hostSummary
	// updates via agg.post(...) on phase / game-data changes; the
	// aggregator coalesces and broadcasts to host:all on its own cadence.
	// May be nil in tests that inject a runner directly without a Manager
	// (publishSummary is a no-op in that case).
	agg *aggregator

	// reader and state are bound when the runner enters Ready (a title-ID
	// match is found in the registry) and cleared when it returns to Idle.
	// Both are accessed only from the loop goroutine.
	reader scraper.GameReader
	state  *scraper.TickState

	// gameData is the loop's working copy of the current game data.
	// Mirrored into cache.GameData so the loop avoids round-tripping
	// through cacheMu on every tick read of e.g. PowerItemSpawns.
	gameData scraper.GameData

	// powerItemsInitialised gates state.InitPowerItems so it only fires
	// once per match — and only after the game data's PowerItemSpawns
	// carry real InitialObjectIDs (matchCache.InitialObjIDsFilled = true).
	// Reset on every Ready → Live transition so the next match re-seeds.
	powerItemsInitialised bool

	// liveReadFailures counts consecutive ReadGameState errors during
	// Live; a heartbeat fallback for the Live → Idle transition (M5 OQ6).
	// Reset on every successful ReadGameState.
	liveReadFailures int

	// lastSummaryPushAt throttles host:all heartbeat pushes — see
	// maybeHeartbeatSummary. Loop-goroutine only; no mutex needed.
	lastSummaryPushAt time.Time

	// lastSystemSnapshotAt throttles xbox/* system reads (console name +
	// future EEPROM / kernel-clock readers). Re-scans on every Idle tick
	// (3s spacing) plus best-effort during Ready / Live so renames in the
	// dashboard pick up live. Loop-goroutine only; no mutex needed.
	lastSystemSnapshotAt time.Time

	// lastReadyBroadcastAt throttles inclusion of GameData in Live
	// state_update envelopes (cumulative scoring updates at 1Hz, separate
	// from the 30Hz Tick stream). Reset to zero on Ready→Live so the first
	// Live broadcast after a transition carries fresh GameData immediately.
	// Loop-goroutine only; no mutex needed.
	lastReadyBroadcastAt time.Time

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	cacheMu sync.Mutex
	cache   instanceCache

	// seqMu guards seqByClass. Each envelope class has its own
	// monotonically-increasing per-(instance, class) sequence number so
	// clients can detect drops / out-of-order delivery / retransmits.
	// Map starts empty on newRunner — no reset on Manager.Start because
	// the runner is recreated each time.
	seqMu      sync.Mutex
	seqByClass map[string]uint64

	// probeReqCh serialises probe requests through the loop goroutine so
	// reader methods (LastStateInputs, BuildScoreProbe) stay loop-only.
	// Buffered cap=4 to absorb small bursts; over-cap requests block
	// briefly on the sender (the request_probe WS handler, which has its
	// own 2s timeout). drainProbeRequests services this queue at the
	// end of each tick — see loop.go.
	probeReqCh chan probeRequest

	// policyMu guards policies. Replaced wholesale by setPolicies so
	// readers under policies() get a stable snapshot without holding
	// the lock for the resolve loop. nil until the Manager wires in
	// the capture-policy loader (PR 16); shouldRead treats nil as
	// "no capture rows" and falls back to WS-subscriber-only gating.
	policyMu sync.RWMutex
	policies []capture.Policy

	// sinks holds per-class destinations (file:, pb:) for envelopes the
	// runner emits. Driven by the same capture-policy slice that gates
	// reads — setPolicies reconciles by calling sinks.applyPolicies.
	// Owned for the runner's lifetime; closed in the loop's shutdown
	// defer.
	sinks *sinkManager
}

func newRunner(name, sock, hostRoom string, agg *aggregator, inst *xemu.Instance) *runner {
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now()
	return &runner{
		name:     name,
		sock:     sock,
		hostRoom: hostRoom,
		agg:      agg,
		inst:     inst,
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
		cache: instanceCache{
			Phase:     PhaseIdle,
			StartedAt: now,
		},
		seqByClass: map[string]uint64{},
		probeReqCh: make(chan probeRequest, 4),
		sinks:      newSinkManager(name),
	}
}

// nextSeq returns and bumps the per-class sequence counter for this
// runner. Goroutine-safe via seqMu. Counters start at 0; the first
// envelope of a given class ships seq=0, the second seq=1, etc.
func (r *runner) nextSeq(class string) uint64 {
	r.seqMu.Lock()
	defer r.seqMu.Unlock()
	seq := r.seqByClass[class]
	r.seqByClass[class] = seq + 1
	return seq
}

// getPolicies returns a stable snapshot of the runner's current capture
// policies. Safe to call from any goroutine; the returned slice must not
// be mutated. Returns nil when the Manager hasn't pushed policies yet,
// which the demand aggregator treats the same as "no rows".
func (r *runner) getPolicies() []capture.Policy {
	r.policyMu.RLock()
	defer r.policyMu.RUnlock()
	return r.policies
}

// setPolicies replaces the runner's capture-policy snapshot. Called by
// the Manager's loader on startup and on capture_policies record-change
// hooks (PR 16). Wholesale replacement keeps getPolicies lock-free at
// the read site. Also reconciles the per-class sinks so any spec
// changes take effect immediately — adds open new sinks, dropped rows
// close the matching active sink.
func (r *runner) setPolicies(p []capture.Policy) {
	r.policyMu.Lock()
	r.policies = p
	r.policyMu.Unlock()
	if r.sinks != nil {
		r.sinks.applyPolicies(p)
	}
}

func (r *runner) info() scraperiface.Info {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	return scraperiface.Info{
		Name:             r.name,
		Sock:             r.sock,
		TitleID:          r.cache.TitleID,
		Title:            r.cache.Title,
		XboxName:         r.cache.XboxName,
		SerialNumber:     r.cache.SerialNumber,
		MACAddress:       r.cache.MACAddress,
		VideoStandard:    r.cache.VideoStandard,
		TimeZoneBias:     r.cache.TimeZoneBias,
		TimeZoneStdName:  r.cache.TimeZoneStdName,
		TimeZoneDltName:  r.cache.TimeZoneDltName,
		XBETitleName:     r.cache.XBETitleName,
		XBEVersion:       r.cache.XBEVersion,
		XBEGameRegion:    r.cache.XBEGameRegion,
		XBEDiskNumber:    r.cache.XBEDiskNumber,
		XBEAllowedMedia:  r.cache.XBEAllowedMedia,
		KernelSystemTime: r.cache.KernelSystemTime,
		KernelBootTime:   r.cache.KernelBootTime,
		KernelUptime:     r.cache.KernelUptime,
		Tick:             r.cache.EngineTick,
		Ticks:            r.cache.Iterations,
		StartedAt:        r.cache.StartedAt,
	}
}

// recordIteration advances the per-iteration progress counters and freshness
// timestamp. Called once per successful loop iteration in any phase. Also
// fires a throttled heartbeat to the host:all aggregator so freshness
// updates land even when no other field changed.
func (r *runner) recordIteration(tick uint32) {
	r.cacheMu.Lock()
	r.cache.EngineTick = tick
	r.cache.Iterations++
	r.cache.LastReadAt = time.Now()
	r.cacheMu.Unlock()
	r.maybeHeartbeatSummary()
}

// withCache runs fn under cacheMu so multi-field updates are atomic from
// a consumer's point of view.
func (r *runner) withCache(fn func(c *instanceCache)) {
	r.cacheMu.Lock()
	fn(&r.cache)
	r.cacheMu.Unlock()
}

// readCache copies the cache out under cacheMu. Pointer / slice / map fields
// in the returned struct share backing storage with the runner's working
// copy; callers must treat the returned struct as read-only. The runner's
// publication discipline (always replace, never mutate) keeps that safe.
func (r *runner) readCache() instanceCache {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	return r.cache
}

// pushEvent appends an event to the cache's newest-first event log and
// prunes to capacity. Always allocates a new backing array so existing
// consumer copies of cache.Events keep pointing at their old slice.
func (r *runner) pushEvent(env scraper.Envelope) {
	r.cacheMu.Lock()
	next := append([]scraper.Envelope{env}, r.cache.Events...)
	if len(next) > recentEventsCap {
		next = next[:recentEventsCap]
	}
	r.cache.Events = next
	r.cacheMu.Unlock()
}

// publishGameState mirrors the loop's most recent ReadGameState result
// into the cache (used by both Ready and Live phases). state_inputs and
// score_probe used to be cached here for the per-tick debug envelope;
// they're now fetched on-demand via Manager.ProbeReply (see probe.go).
func (r *runner) publishGameState(gs scraper.GameState) {
	r.cacheMu.Lock()
	r.cache.GameState = gs
	r.cacheMu.Unlock()
}

// publishGameData stores a freshly-read GameData as the current match
// data. Always allocates a new pointer so any prior copy held by a
// consumer remains stable. Console name is supplied by the system-snapshot
// pass (xbox.ReadConsoleName), not by the plugin — see runSystemSnapshot.
func (r *runner) publishGameData(md scraper.GameData) {
	cp := md
	r.cacheMu.Lock()
	r.cache.GameData = &cp
	r.cacheMu.Unlock()
}

// publishTick stores a freshly-read TickPayload.
func (r *runner) publishTick(tp scraper.TickPayload) {
	cp := tp
	r.cacheMu.Lock()
	r.cache.LatestTick = &cp
	r.cacheMu.Unlock()
}

// summaryHeartbeatInterval bounds how often a runner pushes a fresh
// hostSummary to the aggregator just to refresh LastSuccessfulReadAt. Real
// state changes (phase / map / gametype / score) push immediately; this
// timer covers the steady-state Live case where nothing in the summary
// changed but the freshness timestamp is creeping forward at 30Hz.
//
// The aggregator's own coalesce ticker (250ms) further bounds the
// host:all broadcast cadence — this only governs how often a runner
// occupies the aggregator's input channel.
const summaryHeartbeatInterval = time.Second

// publishSummary derives a hostSummary from the current cache and posts it
// to the aggregator. Loop-goroutine only (lastSummaryPushAt is unsynchronised).
// No-op when r.agg is nil (tests with injected runners).
func (r *runner) publishSummary() {
	if r.agg == nil {
		return
	}
	c := r.readCache()
	s := summaryFromCache(r.name, &c)
	r.agg.post(summaryUpdate{Instance: r.name, Snapshot: &s})
	r.lastSummaryPushAt = time.Now()
}

// maybeHeartbeatSummary calls publishSummary if it's been at least
// summaryHeartbeatInterval since the last push. Wired into recordIteration
// so freshness updates ride along with normal cache writes.
func (r *runner) maybeHeartbeatSummary() {
	if r.agg == nil {
		return
	}
	if time.Since(r.lastSummaryPushAt) < summaryHeartbeatInterval {
		return
	}
	r.publishSummary()
}

// wrapRoomMessage wraps envBytes (already-marshaled scraper.Envelope)
// in websocket.Message{Type:"scraper", Room:room} and returns the wire
// bytes. Logged-and-dropped on marshal error.
func wrapRoomMessage(name, room string, envBytes []byte) ([]byte, bool) {
	msg := websocket.Message{
		Type:    "scraper",
		Room:    room,
		Payload: envBytes,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("scraper[%s]: marshal message: %v", name, err)
		return nil, false
	}
	return msgBytes, true
}

// marshalRoomMessage takes a scraper.Envelope, serialises it, and
// wraps the result in the websocket.Message envelope. Used by callers
// (events.go's EventsReply) that don't need the inner bytes separately.
func marshalRoomMessage(name, room string, env scraper.Envelope) ([]byte, bool) {
	envBytes, err := json.Marshal(env)
	if err != nil {
		log.Printf("scraper[%s]: marshal envelope (%s): %v", name, env.Type, err)
		return nil, false
	}
	return wrapRoomMessage(name, room, envBytes)
}

// marshalClassEnvelope builds a v2 class envelope and serialises both
// the inner envelope and the WS-wrapped message. Returns:
//
//	envBytes  — marshaled scraper.Envelope (one NDJSON line for sinks)
//	msgBytes  — websocket.Message{Type:"scraper", Room:room, Payload:envBytes}
//	room      — per-class room name (host:<inst>:<class>)
//
// (nil, nil, "", false) on validation or marshal failure (logged).
func (r *runner) marshalClassEnvelope(class string, tick uint32, payload any) ([]byte, []byte, string, bool) {
	room, err := rooms.RoomForInstanceClass(r.name, class)
	if err != nil {
		log.Printf("scraper[%s]: cannot resolve room for class %q: %v", r.name, class, err)
		return nil, nil, "", false
	}
	env := scraper.MakeEnvelope(class, r.name, r.nextSeq(class), tick, payload)
	envBytes, err := json.Marshal(env)
	if err != nil {
		log.Printf("scraper[%s]: marshal envelope (%s): %v", r.name, class, err)
		return nil, nil, "", false
	}
	msgBytes, ok := wrapRoomMessage(r.name, room, envBytes)
	if !ok {
		return nil, nil, "", false
	}
	return envBytes, msgBytes, room, ok
}

// emitClass marshals a v2 class envelope, broadcasts the WS-wrapped
// message to the per-class room, and tees the inner envelope to the
// configured sink (if any). No-op on nil svc/WS (test harnesses) or
// marshal failure.
func (r *runner) emitClass(svc *guards.Services, class string, tick uint32, payload any) {
	if svc == nil || svc.WS == nil {
		return
	}
	envBytes, msgBytes, room, ok := r.marshalClassEnvelope(class, tick, payload)
	if !ok {
		return
	}
	svc.WS.SendToRoomRaw(room, msgBytes)
	r.sinks.write(class, envBytes)
}

// classEnvelopeMessages returns one marshaled-message-bytes per class that
// has data to send, in dependency order (xbox → scenario → game →
// previous_game → tick → objects → debug). Used both by the snapshot
// broadcast and by per-class join replay.
//
// Returns (class-name, bytes) pairs so callers can route or filter.
func (r *runner) classEnvelopeMessages() []classMessage {
	c := r.readCache()
	out := make([]classMessage, 0, 7)
	add := func(class string, payload any) {
		if payload == nil {
			return
		}
		_, msgBytes, _, ok := r.marshalClassEnvelope(class, c.EngineTick, payload)
		if !ok {
			return
		}
		out = append(out, classMessage{Class: class, Bytes: msgBytes})
	}
	add("xbox", buildXboxPayload(&c))
	if sp := buildScenarioPayload(&c); sp != nil {
		add("scenario", *sp)
	}
	add("game", buildGamePayload(&c))
	if pg := buildPreviousGamePayload(&c); pg != nil {
		add("previous_game", *pg)
	}
	if tp := buildTickPayload(&c); tp != nil {
		add("tick", *tp)
	}
	if op := buildObjectsPayload(&c); op != nil {
		add("objects", *op)
	}
	if dp := buildDebugPayload(&c); dp != nil {
		add("debug", *dp)
	}
	return out
}

// classMessage pairs a class name with its marshaled wire bytes. Used by
// classEnvelopeMessages so callers can filter to a specific class (per-
// class join replay) or fan everything out (snapshot broadcast).
type classMessage struct {
	Class string
	Bytes []byte
}

// broadcastSnapshot emits every applicable per-class envelope for the
// current cache state. Called on phase transitions so a transitioning
// (or just-joined) client gets a complete view across all classes
// before per-poll updates resume.
//
// Replaces v1 broadcastCurrentState — instead of one monolithic
// current_state envelope to host:<name>, this fans out 1–7 envelopes
// to per-class rooms (host:<name>:xbox, :scenario, :game, ...).
func (r *runner) broadcastSnapshot(svc *guards.Services) {
	if svc == nil || svc.WS == nil {
		return
	}
	for _, m := range r.classEnvelopeMessages() {
		room, err := rooms.RoomForInstanceClass(r.name, m.Class)
		if err != nil {
			continue
		}
		svc.WS.SendToRoomRaw(room, m.Bytes)
	}
}

// broadcastPoll emits the volatile / per-poll classes — game (heartbeat
// + change-driven) and, when live tick data exists, tick / objects /
// debug. Called once per successful poll iteration in each phase
// function. No phase argument needed in v2: the cache state itself
// determines which classes have data to send.
//
// Replaces v1 broadcastStateUpdate. The 1 Hz GameData piggyback dance
// is gone — game is always emitted on every poll, and the per-tick
// stream (tick/objects/debug) only fires when there's tick data.
func (r *runner) broadcastPoll(svc *guards.Services) {
	if svc == nil || svc.WS == nil {
		return
	}
	c := r.readCache()
	pol := r.getPolicies()
	if shouldRead(r.name, "game", pol, svc.WS) {
		r.emitClass(svc, "game", c.EngineTick, buildGamePayload(&c))
	}
	if c.LatestTick == nil {
		return
	}
	if shouldRead(r.name, "tick", pol, svc.WS) {
		if tp := buildTickPayload(&c); tp != nil {
			r.emitClass(svc, "tick", c.EngineTick, *tp)
		}
	}
	if shouldRead(r.name, "objects", pol, svc.WS) {
		if op := buildObjectsPayload(&c); op != nil {
			r.emitClass(svc, "objects", c.EngineTick, *op)
		}
	}
	if shouldRead(r.name, "debug", pol, svc.WS) {
		if dp := buildDebugPayload(&c); dp != nil {
			r.emitClass(svc, "debug", c.EngineTick, *dp)
		}
	}
}
