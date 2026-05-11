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
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
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
	GameState   scraper.GameState
	StateInputs scraper.StateInputs
	ScoreProbe  scraper.ScoreProbe
	GameData    *scraper.GameData
	LatestTick  *scraper.TickPayload
	Events      []scraper.Envelope // newest-first; bounded by recentEventsCap

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
// into the cache (used by both Ready and Live phases).
func (r *runner) publishGameState(gs scraper.GameState, si scraper.StateInputs, sp scraper.ScoreProbe) {
	r.cacheMu.Lock()
	r.cache.GameState = gs
	if si != nil {
		cp := make(scraper.StateInputs, len(si))
		for k, v := range si {
			cp[k] = v
		}
		r.cache.StateInputs = cp
	}
	if sp != nil {
		cp := make(scraper.ScoreProbe, len(sp))
		for k, v := range sp {
			cp[k] = v
		}
		r.cache.ScoreProbe = cp
	}
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

// readyBroadcastInterval bounds how often Live state_update envelopes
// include the full GameData payload. The 30Hz tick stream carries volatile
// per-frame state in TickPayload; cumulative scoring (kills/deaths/assists,
// team scores) lives on GameData and is refreshed by the runner every tick
// but only changes on score events. 1Hz keeps overlay rosters in sync
// without inflating the high-frequency stream.
const readyBroadcastInterval = time.Second

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

// CurrentStatePayload is the per-instance "current_state" envelope payload —
// a full atomic-cache-read snapshot of the runner's instanceCache. Sent on
// join and on every phase transition. Consumers reconstruct the entire
// instance view from a single payload of this shape without needing prior
// history. State_update envelopes then carry the volatile-fields portion at
// phase-appropriate cadence.
type CurrentStatePayload struct {
	Phase            Phase         `json:"phase"`
	StartedAt        time.Time     `json:"started_at"`
	TitleID          uint32        `json:"title_id"`
	Title            string        `json:"title"`
	XboxName         string        `json:"xbox_name"`
	SerialNumber     string        `json:"serial_number,omitempty"`
	MACAddress       string        `json:"mac_address,omitempty"`
	VideoStandard    string        `json:"video_standard,omitempty"`
	TimeZoneBias     int32         `json:"time_zone_bias,omitempty"`
	TimeZoneStdName  string        `json:"time_zone_std_name,omitempty"`
	TimeZoneDltName  string        `json:"time_zone_dlt_name,omitempty"`
	XBETitleName     string        `json:"xbe_title_name,omitempty"`
	XBEVersion       uint32        `json:"xbe_version,omitempty"`
	XBEGameRegion    uint32        `json:"xbe_game_region,omitempty"`
	XBEDiskNumber    uint32        `json:"xbe_disk_number,omitempty"`
	XBEAllowedMedia  uint32        `json:"xbe_allowed_media,omitempty"`
	KernelSystemTime time.Time     `json:"kernel_system_time,omitempty"`
	KernelBootTime   time.Time     `json:"kernel_boot_time,omitempty"`
	KernelUptime     time.Duration `json:"kernel_uptime_ns,omitempty"`

	LastReadAt   time.Time            `json:"last_read_at"`
	EngineTick   uint32               `json:"engine_tick"`
	Iterations   uint64               `json:"iterations"`
	GameData     *scraper.GameData    `json:"game_data,omitempty"`
	LatestTick   *scraper.TickPayload `json:"latest_tick,omitempty"`
	Events       []scraper.Envelope   `json:"events,omitempty"`
	PreviousGame *previousGame        `json:"previous_game,omitempty"`
}

// StateUpdatePayload is the per-poll "state_update" envelope payload — the
// volatile / tick-fields portion of the cache. Sent every successful poll
// at phase-appropriate cadence: Idle ~3s (no per-phase data, just freshness),
// Ready ~500ms (volatile lobby/menu game data), Live ~30Hz (tick payload +
// engine tick). The top-level envelope's tick field is the engine tick in
// Live and 0 outside Live (per the M5 brief's "Decisions made" section).
type StateUpdatePayload struct {
	Phase      Phase                `json:"phase"`
	LastReadAt time.Time            `json:"last_read_at"`
	Iterations uint64               `json:"iterations"`
	EngineTick uint32               `json:"engine_tick,omitempty"`
	Tick       *scraper.TickPayload `json:"tick,omitempty"`
	Ready      *scraper.GameData    `json:"ready,omitempty"`
}

// buildCurrentStateEnvelope reads the instanceCache atomically and builds the
// marshaled websocket.Message bytes for a current_state envelope addressed
// to this runner's host:<name> room. Returns (nil, false) on marshal error
// (logged). The caller is responsible for fanning the bytes out via
// SendToRoomRaw or SendRaw.
func (r *runner) buildCurrentStateEnvelope() ([]byte, bool) {
	c := r.readCache()
	payload := CurrentStatePayload{
		Phase:            c.Phase,
		StartedAt:        c.StartedAt,
		TitleID:          c.TitleID,
		Title:            c.Title,
		XboxName:         c.XboxName,
		SerialNumber:     c.SerialNumber,
		MACAddress:       c.MACAddress,
		VideoStandard:    c.VideoStandard,
		TimeZoneBias:     c.TimeZoneBias,
		TimeZoneStdName:  c.TimeZoneStdName,
		TimeZoneDltName:  c.TimeZoneDltName,
		XBETitleName:     c.XBETitleName,
		XBEVersion:       c.XBEVersion,
		XBEGameRegion:    c.XBEGameRegion,
		XBEDiskNumber:    c.XBEDiskNumber,
		XBEAllowedMedia:  c.XBEAllowedMedia,
		KernelSystemTime: c.KernelSystemTime,
		KernelBootTime:   c.KernelBootTime,
		KernelUptime:     c.KernelUptime,
		LastReadAt:       c.LastReadAt,
		EngineTick:       c.EngineTick,
		Iterations:       c.Iterations,
		GameData:         c.GameData,
		LatestTick:       c.LatestTick,
		Events:           c.Events,
		PreviousGame:     c.PreviousGame,
	}
	env := scraper.MakeEnvelope(envelopeTypeCurrentState, r.name, c.EngineTick, payload)
	return marshalRoomMessage(r.name, r.hostRoom, env)
}

// buildStateUpdateEnvelope reads the instanceCache atomically and builds the
// marshaled websocket.Message bytes for a state_update envelope. The
// envelope's top-level tick is c.EngineTick when phase==PhaseLive, otherwise
// 0 (the brief's "Decisions made" rule). Phase-specific fields are
// populated only in their phase: Tick in Live, Ready in PhaseReady; Idle
// carries only the always-present freshness fields.
//
// includeReady piggybacks the cached GameData onto the Live payload so
// cumulative scoring stays fresh on the WS stream. Ignored outside Live —
// PhaseReady always includes Ready, PhaseIdle has no GameData to send.
func (r *runner) buildStateUpdateEnvelope(phase Phase, includeReady bool) ([]byte, bool) {
	c := r.readCache()
	payload := StateUpdatePayload{
		Phase:      phase,
		LastReadAt: c.LastReadAt,
		Iterations: c.Iterations,
	}
	var envTick uint32
	switch phase {
	case PhaseLive:
		payload.EngineTick = c.EngineTick
		payload.Tick = c.LatestTick
		envTick = c.EngineTick
		if includeReady {
			payload.Ready = c.GameData
		}
	case PhaseReady:
		payload.Ready = c.GameData
	}
	env := scraper.MakeEnvelope(envelopeTypeStateUpdate, r.name, envTick, payload)
	return marshalRoomMessage(r.name, r.hostRoom, env)
}

// marshalRoomMessage wraps env in websocket.Message{Type:"scraper", Room:room}
// and returns the marshaled wire bytes. Logged-and-dropped on marshal error
// (caller-visible via the (nil, false) return).
func marshalRoomMessage(name, room string, env scraper.Envelope) ([]byte, bool) {
	envBytes, err := json.Marshal(env)
	if err != nil {
		log.Printf("scraper[%s]: marshal envelope (%s): %v", name, env.Type, err)
		return nil, false
	}
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

// broadcastCurrentState builds a current_state envelope and pushes it to
// this runner's host:<name> room. Called on phase transitions (the loop
// dispatcher) so the brief's ordering invariant — "current_state for a
// transition reaches clients before any state_update tagged with the new
// phase" — is satisfied by the runner being the single goroutine writer
// for its room.
func (r *runner) broadcastCurrentState(svc *guards.Services) {
	if svc == nil || svc.WS == nil {
		return
	}
	msgBytes, ok := r.buildCurrentStateEnvelope()
	if !ok {
		return
	}
	svc.WS.SendToRoomRaw(r.hostRoom, msgBytes)
}

// broadcastStateUpdate builds a state_update envelope and pushes it to
// this runner's host:<name> room. Called once per successful poll iteration
// in each phase function. Phase determines which optional payload fields
// are populated (see StateUpdatePayload).
//
// During Live, GameData is piggybacked at readyBroadcastInterval cadence
// (~1Hz) so cumulative scoring stays fresh on the WS stream without
// inflating the 30Hz tick payload.
func (r *runner) broadcastStateUpdate(svc *guards.Services, phase Phase) {
	if svc == nil || svc.WS == nil {
		return
	}
	includeReady := false
	if phase == PhaseLive && time.Since(r.lastReadyBroadcastAt) >= readyBroadcastInterval {
		includeReady = true
		r.lastReadyBroadcastAt = time.Now()
	}
	msgBytes, ok := r.buildStateUpdateEnvelope(phase, includeReady)
	if !ok {
		return
	}
	svc.WS.SendToRoomRaw(r.hostRoom, msgBytes)
}
