package manager

import (
	"context"
	"sync"
	"time"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// recentEventsCap caps the per-runner event log surfaced via the inspect
// endpoint and replayed to mid-match overlay joiners. Newest-first; older
// entries drop off the back.
const recentEventsCap = 50

// instanceCache is the per-runner authoritative source of truth introduced
// in M5 stage 5a. Replaces the loose cluster of cache fields the runner
// used to hold (one pre-marshaled-bytes cache for the legacy "snapshot"
// envelope plus several per-field caches) with a single struct, and adds
// Phase, LastReadAt, and PreviousGame.
//
// All access goes through runner.cacheMu. Pointer / slice / map fields are
// replaced wholesale rather than mutated in place so consumers that copy the
// cache out under the lock can read their copy without further coordination.
//
// Wire format is unchanged in 5b — broadcast envelopes are still built from
// this cache and sent as Message{Type:"scraper", Room:"host:<name>", ...}
// with the legacy "snapshot" / "tick" / "event" envelope types. 5c will
// introduce the new current_state / state_update / event protocol that
// reads from the same struct.
type instanceCache struct {
	// Lifecycle.
	Phase     Phase
	StartedAt time.Time

	// Identity. TitleID is the most recently observed XBE title ID (0 if
	// no successful read yet). Title and XboxName come from the bound
	// GameReader and stay empty in Idle.
	TitleID   uint32
	Title string
	XboxName  string

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
	GameData   *scraper.GameData
	LatestTick  *scraper.TickPayload
	Events      []scraper.Envelope // newest-first; bounded by recentEventsCap

	// Just-ended match. Populated on Live→Ready transition (deferred so a
	// panic / ctx-cancel mid-match still moves the data); dropped on
	// Ready→Idle.
	PreviousGame *previousGame
}

// previousGame is the just-ended match captured on Live→Ready.
type previousGame struct {
	GameData *scraper.GameData
	Events    []scraper.Envelope
	EndedAt   time.Time
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
		Name:      r.name,
		Sock:      r.sock,
		TitleID:   r.cache.TitleID,
		Title: r.cache.Title,
		XboxName:  r.cache.XboxName,
		Tick:      r.cache.EngineTick,
		Ticks:     r.cache.Iterations,
		StartedAt: r.cache.StartedAt,
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
// consumer remains stable.
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
