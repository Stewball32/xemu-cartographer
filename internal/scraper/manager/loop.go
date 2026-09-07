package manager

import (
	"encoding/json"
	"log"
	"runtime/debug"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/xbox"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

// Per-phase poll cadences (M5 stage 5a). Each phase trades freshness for read
// cost: Idle just polls a single u32 every few seconds, Ready re-reads the
// full game-data field set every ~500ms (cheap once scenario caches are
// warm), and Live polls fast enough to catch every fresh engine tick.
const (
	idlePollInterval  = 3 * time.Second
	readyPollInterval = 500 * time.Millisecond
	// livePollInterval gives ~3 polls per 30Hz engine tick — enough to catch
	// every tick advance without busy-spinning. Matches the legacy loop's
	// in-game poll cadence so the M2 30Hz envelope cadence stays unchanged.
	livePollInterval = 10 * time.Millisecond
)

// readyTitleCheckInterval is the number of Ready iterations between XBE
// title-ID re-checks. At readyPollInterval=500ms × 10 = ~5s. Matches the
// legacy idle re-check cadence — a transition to Idle here stops the runner's
// game-specific reads while keeping the runner alive for the next title.
const readyTitleCheckInterval = 10

// liveReadFailureLimit is the OQ6 heartbeat-fallback threshold. If
// ReadGameState errors this many times in a row during Live, the runner
// gives up on the current match and transitions back to Idle so it can
// re-detect the title via ReadTitleID. Calibrated for ~300ms of failure at
// the 10ms Live poll cadence — long enough to ride out single-tick reads
// the engine missed, short enough to react to a clean xemu exit.
//
// This is the production fallback while the title-ID-at-0x00010000 probe
// (see internal/pocketbase/routes/xemu/probe.go and ROADMAP.md M5 OQ6) is
// still being validated against real Halo CE → dashboard transitions.
const liveReadFailureLimit = 30

// readyReadRefreshAt is the consecutive-Ready-failure count at which the loop
// re-translates the low GVAs (RefreshLowHVA over the reader's LowGVAs). Ready
// polls at 500ms, so 3 ≈ 1.5s of solid failure before a refresh — long enough
// to ignore a one-off missed read, short enough that a box that auto-attached
// mid-boot self-heals within a couple of seconds instead of sitting blind
// forever. Reset after each refresh attempt so it retries on the next window.
const readyReadRefreshAt = 3

// readyStaleRefreshAt / readyStaleRefreshMax gate the SILENT-staleness refresh
// (see lowReadsLookStale). More conservative than the error gate — all-zero low
// reads are legitimately transient while the guest is still bringing the
// front-end up — and it BACKS OFF, because unlike the error path this condition
// can persist indefinitely and each refresh costs one QMP round trip per low
// GVA (~65 of them). 6 ≈ 3s at the 500ms Ready cadence, backing off to ~60s.
const (
	readyStaleRefreshAt  = 6
	readyStaleRefreshMax = 120
)

// consecutiveFailureGate counts consecutive failures and reports when a
// recovery action is due. fail() returns true on the `at`-th consecutive
// failure and resets the count so the action re-arms for the next run; ok()
// clears the count on a success. Pure + synchronous — the Ready loop uses it to
// decide when to re-translate stale low mappings, and it's unit-tested apart
// from the loop. Zero value never fires; construct with a positive `at`.
//
// When built via newBackoffGate, the threshold DOUBLES after each firing up to
// max, so a condition that never clears degrades to an occasional retry instead
// of a hot loop; ok() restores the original threshold.
type consecutiveFailureGate struct {
	n    int
	at   int
	base int // 0 = no backoff restore (plain literal construction)
	max  int // 0 = no backoff
}

func newBackoffGate(at, max int) consecutiveFailureGate {
	return consecutiveFailureGate{at: at, base: at, max: max}
}

func (g *consecutiveFailureGate) fail() bool {
	g.n++
	if g.at > 0 && g.n >= g.at {
		g.n = 0
		if g.max > 0 && g.at < g.max {
			if g.at *= 2; g.at > g.max {
				g.at = g.max
			}
		}
		return true
	}
	return false
}

func (g *consecutiveFailureGate) ok() {
	g.n = 0
	if g.base > 0 {
		g.at = g.base
	}
}

// loop is the per-runner tick goroutine. Started by Manager.Start, exits when
// ctx is cancelled (Manager.Stop). Always closes the xemu instance and
// signals done on exit, even on panic, so Manager.Stop's <-r.done unblocks.
//
// The loop is phase-driven (M5 stage 5a): Idle polls for title-ID
// recognition, Ready scrapes game-data on a slow cadence, Live runs the
// 30Hz tick goroutine. Phase transitions update r.cache.Phase under cacheMu
// so consumers (Inspect endpoint, future M5 5c emission layer) can observe
// the runner's state independently of GameState.
func (r *runner) loop(svc *guards.Services) {
	defer close(r.done)
	defer r.inst.Close()
	defer func() {
		if r.sinks != nil {
			r.sinks.closeAll()
		}
	}()
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("scraper[%s]: panic in tick loop: %v\n%s", r.name, rec, debug.Stack())
		}
	}()

	r.publishPhase(PhaseIdle)
	phase := PhaseIdle
	var prevPhase Phase // empty sentinel — the first iteration always emits

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
		}

		// Phase transition: emit current_state before the new phase function
		// starts streaming state_updates. The runner is the single goroutine
		// writer for its host:<name> room, so this synchronous ordering
		// satisfies the M5 brief's invariant — "current_state for a
		// transition reaches clients before any state_update tagged with the
		// new phase" — without extra synchronisation. The empty-string
		// sentinel makes the very first iteration (entering Idle at startup)
		// emit too.
		if phase != prevPhase {
			r.broadcastSnapshot(svc)
			prevPhase = phase
		}

		switch phase {
		case PhaseIdle:
			phase = r.runIdle(svc)
		case PhaseReady:
			phase = r.runReady(svc)
		case PhaseLive:
			phase = r.runLive(svc)
		default:
			log.Printf("scraper[%s]: unknown phase %q — defaulting to idle", r.name, phase)
			phase = PhaseIdle
		}
		r.publishPhase(phase)
		// Phase changes are first-class summary events — push immediately so
		// host:all subscribers see Idle → Ready → Live transitions without
		// waiting for the next heartbeat tick.
		r.publishSummary()
	}
}

// runIdle polls the XBE title ID and binds a GameReader as soon as the
// title becomes recognised in the scraper registry. Returns the next phase
// (Idle if no match, Ready if a reader was bound).
func (r *runner) runIdle(svc *guards.Services) Phase {
	titleID, err := scraper.ReadTitleID(r.inst)
	if err != nil {
		// Common during xemu boot or when the kernel hasn't mapped the XBE
		// header yet. Stay idle and retry. Don't update LastReadAt — a
		// failing read is not progress.
		log.Printf("scraper[%s]: idle title-ID read: %v", r.name, err)
		r.broadcastPoll(svc)
		r.sleepOrCancel(idlePollInterval)
		return PhaseIdle
	}

	r.recordIteration(0)
	r.withCache(func(c *instanceCache) {
		c.TitleID = titleID
	})

	// Run xbox/* system reads (console name, future EEPROM/clock reads). This
	// fires whether or not a plugin matches the title ID, so UnleashX and
	// other unrecognised titles still surface xbox_name in the snapshot.
	r.runSystemSnapshot()

	if scraper.Lookup(titleID) == nil {
		// Unknown title — stay idle and re-poll. The TitleID is already
		// surfaced in the cache so the debug page can show "phase=idle,
		// title_id=0x...".
		r.broadcastPoll(svc)
		r.sleepOrCancel(idlePollInterval)
		return PhaseIdle
	}

	// Title recognised — bind a reader through the version-level offset
	// layer: the instance's assigned offset set when the catalog names one,
	// else the game's baseline (identical values to the old hardcoded
	// constants — see internal/scraper/offsets). Then re-init the xemu
	// instance with the reader's required low GVAs (xemu.Instance.Init is
	// idempotent for already-translated addresses, so the detection-only
	// init done at Start time is preserved).
	setID := ""
	if r.offsetSetFor != nil {
		setID = r.offsetSetFor()
	}
	reader, err := scraper.NewReaderForTitle(titleID, r.inst, r.name, setID)
	if err != nil {
		log.Printf("scraper[%s]: bind reader: %v — staying idle", r.name, err)
		r.broadcastPoll(svc)
		r.sleepOrCancel(idlePollInterval)
		return PhaseIdle
	}
	allGVAs := append(scraper.DetectionGVAs(), reader.LowGVAs()...)
	if err := r.inst.Init(allGVAs); err != nil {
		log.Printf("scraper[%s]: bind reader (init low GVAs): %v — staying idle", r.name, err)
		r.broadcastPoll(svc)
		r.sleepOrCancel(idlePollInterval)
		return PhaseIdle
	}

	r.reader = reader
	r.state = reader.NewTickState()
	r.gameData = scraper.GameData{}
	r.powerItemsInitialised = false
	r.liveReadFailures = 0
	r.withCache(func(c *instanceCache) {
		c.Title = reader.Title()
		// Idle drops PreviousGame; entering Ready inherits that empty slot.
		c.PreviousGame = nil
	})

	log.Printf("scraper[%s]: title 0x%08X recognised (%s) — idle → ready", r.name, titleID, reader.Title())
	return PhaseReady
}

// runReady runs the Ready phase loop: lobby / pregame / postgame / between-
// match menu. Reads ReadGameState every iteration; on in_game observation
// transitions to Live. Periodically re-checks the title ID; on change or
// read failure releases the reader and transitions back to Idle.
//
// M5 stage 5c: every iteration emits one state_update envelope before
// sleeping, carrying the volatile portion of the cache (Ready-phase game
// data). GameState transitions within Ready (menu→pregame, postgame→menu,
// etc.) trigger a thorough ReadGameData refresh so the cache stays current,
// but no separate envelope fires — the next state_update carries the new
// data. Phase transitions (Ready → Live, Ready → Idle) emit a fresh
// current_state from the loop dispatcher before the new phase starts.
func (r *runner) runReady(svc *guards.Services) Phase {
	prevState := scraper.GameState("")
	titleCheckCount := 0
	readGate := consecutiveFailureGate{at: readyReadRefreshAt}
	staleGate := newBackoffGate(readyStaleRefreshAt, readyStaleRefreshMax)

	for {
		select {
		case <-r.ctx.Done():
			return PhaseReady
		default:
		}

		// Title-ID re-check fires on every iteration's pre-flight, BEFORE
		// any reader call, so a stuck-on-errors loop (typical XBE-swapped-
		// to-dashboard symptom: Halo reader's reads succeed but return
		// nonsense, or fail outright) still escapes back to Idle within
		// readyTitleCheckInterval × readyPollInterval.
		titleCheckCount++
		if titleCheckCount >= readyTitleCheckInterval {
			titleCheckCount = 0
			if titleID, err := scraper.ReadTitleID(r.inst); err != nil {
				log.Printf("scraper[%s]: ready title-ID re-check: %v — ready → idle", r.name, err)
				r.releaseReader()
				return PhaseIdle
			} else if titleID != r.cachedTitleID() {
				log.Printf("scraper[%s]: title 0x%08X → 0x%08X — ready → idle", r.name, r.cachedTitleID(), titleID)
				r.releaseReader()
				r.withCache(func(c *instanceCache) { c.TitleID = titleID })
				return PhaseIdle
			}
		}

		gs, tick, err := r.reader.ReadGameState()
		if err != nil {
			log.Printf("scraper[%s]: ready ReadGameState: %v", r.name, err)
			// A run of failures on the low game-.data reads means the cached
			// host-VA translations are stale (e.g. attached seconds into boot
			// before the guest's page tables settled). Re-translate against the
			// current layout so the box self-heals instead of sitting blind.
			if readGate.fail() {
				r.refreshLowTranslations()
			}
			r.broadcastPoll(svc)
			r.sleepOrCancel(readyPollInterval)
			continue
		}
		readGate.ok()

		// The read SUCCEEDED — but success isn't proof the translation is
		// pointing at real memory. If every low global reads back zero, the
		// cached mappings are stale (see lowReadsLookStale) and no error will
		// ever surface to trigger the refresh above, so check explicitly.
		if r.lowReadsLookStale() {
			if staleGate.fail() {
				log.Printf("scraper[%s]: all low globals read zero — cached translations look stale", r.name)
				r.refreshLowTranslations()
			}
		} else {
			staleGate.ok()
		}

		r.recordIteration(tick)
		r.publishGameState(gs)

		// Refresh xbox/* system values (console name, etc.) — throttled so
		// the cost is bounded regardless of phase poll cadence. Picks up
		// renames the user did via the dashboard between matches and any
		// late-arriving heap state from a freshly-loaded XBE.
		r.runSystemSnapshot()

		// Service any pending probe requests from request_probe handlers
		// before the next sleep — probe readers (BuildScoreProbe) must
		// run on the loop goroutine for reader-cache thread safety.
		r.drainProbeRequests()

		// State-transition handling within Ready — refresh the cache via the
		// thorough ReadGameData path so static + cached scenario data is
		// current when the next state_update fires. A summary push here
		// keeps host:all in sync with map / gametype changes immediately.
		if gs != prevState {
			if err := r.reader.OnStateChange(prevState, gs); err != nil {
				log.Printf("scraper[%s]: OnStateChange %s → %s: %v", r.name, prevState, gs, err)
			}
			if prevState != "" {
				log.Printf("scraper[%s]: state %s → %s tick=%d", r.name, prevState, gs, tick)
			} else {
				log.Printf("scraper[%s]: initial state %s tick=%d", r.name, gs, tick)
			}
			if gs == scraper.GameStateMenu {
				r.powerItemsInitialised = false
			}
			if snap, err := r.reader.ReadGameData(); err != nil {
				log.Printf("scraper[%s]: ReadGameData: %v", r.name, err)
			} else {
				snap.GameState = gs
				r.gameData = snap
				r.publishGameData(snap)
				r.maybeEmitScenario(svc)
				r.publishSummary()
				prevState = gs
			}
		}

		// Ready → Live transition: in_game observed. The loop dispatcher
		// will emit current_state for Live before runLive starts emitting
		// state_updates, so no need to fire one here.
		if gs == scraper.GameStateInGame {
			r.state = r.reader.NewTickState()
			r.powerItemsInitialised = false
			r.liveReadFailures = 0
			r.lastReadyBroadcastAt = time.Time{}
			return PhaseLive
		}

		// Cheap game-data refresh so the cache (and the next state_update)
		// reflects current scoreboard / roster data without waiting for the
		// next state transition.
		if snap, err := r.reader.ReadReadyState(); err == nil {
			snap.GameState = gs
			r.gameData = snap
			r.publishGameData(snap)
			r.maybeEmitScenario(svc)
		}

		// Refresh the create-game map/gametype carousel enumeration (throttled)
		// so /api/play/options serves the real per-instance lists. Runs in Ready
		// because the source UI tags are only loaded while the front-end is up;
		// keeps the last-known set when unavailable (mid-match).
		r.enumerateLobby()

		// Drive the player-hosting runner: navigate the CE menu → system-link
		// host lobby, gated on the just-read state. The lobby flow lives entirely
		// in Ready (menu / pregame / postgame), so this is where auto-hosting
		// happens. Throttled internally; a no-op when host-running is disabled.
		r.tickHost(gs, tick)

		r.broadcastPoll(svc)
		// Sleep in host-tick slices: with a host runner attached the box
		// re-reads state + ticks the runner every hostTickMinInterval (~100ms)
		// so the nav's closed-loop confirmations land promptly; without one
		// this is a plain readyPollInterval sleep.
		r.hostSubTicks(readyPollInterval)
	}
}

// refreshLowTranslations re-resolves every low guest VA the reader depends on
// against the instance's current page tables, in place, so a stale boot-time
// mapping recovers without tearing down the runner. RefreshLowHVA is atomic
// per address — a translate that still fails (page not yet mapped) leaves that
// entry's prior value intact and simply retries on the next refresh window —
// so a partial success never leaves the reader with holes. Best-effort: the
// count is logged; failures are expected while the guest is still settling.
// lowStaleKeys are the ReadGameState values that come from LOW guest VAs — the
// ones resolved through the cached gva2gpa translations. High-GVA reads (heap /
// menu widgets) don't use that cache and are unaffected.
var lowStaleKeys = []string{
	"main_menu", "game_connection",
	"game_engine_globals_ptr", "game_time_globals_ptr", "game_can_score",
}

// lowReadsLookStale reports the SILENT failure mode: every low-GVA global reads
// back as zero while the title is up and high-GVA reads are returning real data.
//
// This is what a stale translation actually looks like in practice — not an
// error. The scraper can attach seconds into container boot, translate the low
// GVAs before the guest's page tables are settled, and cache host addresses that
// point at unbacked zero pages. Every later read then SUCCEEDS and returns 0, so
// the error-driven refresh never fires, main_menu is forever 0, Classify can
// never reach ScreenMainMenu, and the host runner sits blocked at "unknown".
//
// An all-zero set is implausible for a running game: at a real front-end menu
// main_menu is 1, and in a game the engine pointers are non-nil. It IS legitimate
// transiently during boot, which is why the caller gates + backs off rather than
// refreshing on the first sighting. A plugin that doesn't report these keys opts
// out entirely (no guessing).
func (r *runner) lowReadsLookStale() bool {
	if r.reader == nil {
		return false
	}
	si := r.reader.LastStateInputs()
	if len(si) == 0 {
		return false
	}
	for _, k := range lowStaleKeys {
		if _, ok := si[k]; !ok {
			return false
		}
		if stateInputInt(si, k) != 0 {
			return false
		}
	}
	return true
}

func (r *runner) refreshLowTranslations() {
	gvas := r.reader.LowGVAs()
	refreshed := 0
	for _, gva := range gvas {
		if _, err := r.inst.RefreshLowHVA(gva); err == nil {
			refreshed++
		}
	}
	log.Printf("scraper[%s]: ready reads failing — re-translated %d/%d low GVAs against current layout",
		r.name, refreshed, len(gvas))
}

// runLive runs the Live phase loop: 30Hz tick reads, event detection, and
// per-tick broadcasts. Returns the next phase (Ready when ReadGameState
// reports the match has ended, Idle when ReadGameState fails the heartbeat).
//
// A defer captures the just-ended match into cache.PreviousGame *before*
// returning, so a panic / ctx-cancel / xemu-vanishes scenario still moves
// the data rather than dropping it. Ready inherits the populated
// PreviousGame slot; Idle clears it (handled in releaseReader).
func (r *runner) runLive(svc *guards.Services) (next Phase) {
	// endReason is the observed exit condition captured into PreviousGame.
	// The state-observed exits below overwrite it; a ctx-cancel (or panic)
	// unwind leaves the shutdown default. Read by the deferred capture via
	// closure because defer arguments are evaluated at defer time.
	endReason := endReasonShutdown

	// LIFO: persistFinishedGame runs after captureLiveAsPrevious, so
	// cache.PreviousGame is populated when it reads it (M13 game-end trigger).
	defer r.persistFinishedGame(svc)
	defer func() { r.captureLiveAsPrevious(endReason) }()

	// Fresh match → fresh stat accumulator + activity latches + per-match
	// event log. The cache's previous-match snapshot is cleared here (not on
	// Live→Ready) so the postgame graphic keeps the finished match's stats
	// until a new one starts.
	r.accum = scraper.NewMatchAccum()
	r.publishAccum(nil)
	r.withCache(func(c *instanceCache) {
		c.MatchEvents = nil
		c.MatchEventsTruncated = false
	})

	var lastBroadcastTick uint32

	for {
		select {
		case <-r.ctx.Done():
			return PhaseLive
		default:
		}

		gs, tick, err := r.reader.ReadGameState()
		if err != nil {
			r.liveReadFailures++
			log.Printf("scraper[%s]: live ReadGameState (failure %d/%d): %v", r.name, r.liveReadFailures, liveReadFailureLimit, err)
			if r.liveReadFailures >= liveReadFailureLimit {
				log.Printf("scraper[%s]: live read heartbeat failed — live → idle", r.name)
				r.releaseReader()
				return PhaseIdle
			}
			r.sleepOrCancel(livePollInterval)
			continue
		}
		r.liveReadFailures = 0
		r.recordIteration(tick)
		r.publishGameState(gs)

		// Refresh xbox/* system values during long matches (throttled, ~3s
		// minimum spacing). Renames are unreachable from in-match, but a
		// late-binding kernel record on a host that booted straight into
		// gameplay still gets a chance to land.
		r.runSystemSnapshot()

		// Service any pending probe requests — see runReady for rationale.
		r.drainProbeRequests()

		if gs != scraper.GameStateInGame {
			log.Printf("scraper[%s]: state in_game → %s tick=%d — live → ready", r.name, gs, tick)
			if err := r.reader.OnStateChange(scraper.GameStateInGame, gs); err != nil {
				log.Printf("scraper[%s]: OnStateChange live→ready: %v", r.name, err)
			}
			// Exit states observed in practice: postgame (the match ran to
			// its carousel) or menu / pregame (players bailed to the front
			// end, or the lobby rolled straight into the next match's setup
			// without this box seeing a postgame). Only an observed postgame
			// earns endReasonPostgame; anything else is honestly a
			// left-before-the-end.
			if gs == scraper.GameStatePostGame {
				endReason = endReasonPostgame
			} else {
				endReason = endReasonLeftMatch
			}
			return PhaseReady
		}

		// Initialise power-item trackers once per match — gated on the
		// game data carrying real InitialObjectIDs (matchCache fills them
		// asynchronously after pregame → in_game).
		if !r.powerItemsInitialised && hasResolvedSpawnIDs(r.gameData.PowerItemSpawns) && r.state != nil {
			r.state.InitPowerItems(r.gameData.PowerItemSpawns)
			r.powerItemsInitialised = true
		}

		// Skip duplicate-tick polls — engine ticks at 30Hz, we poll at ~100Hz.
		if tick == lastBroadcastTick {
			r.sleepOrCancel(livePollInterval)
			continue
		}

		tickResult, err := r.reader.ReadTick(r.gameData.PowerItemSpawns, r.state)
		if err != nil {
			log.Printf("scraper[%s]: ReadTick: %v", r.name, err)
			r.sleepOrCancel(livePollInterval)
			continue
		}
		r.publishTick(tickResult.Payload)

		// Accumulate match stats + activity latches off this tick (HaloCaster
		// extract_events port) and publish the snapshot for the wire/filter.
		r.accum.Observe(tickResult)
		r.publishAccum(r.accum.Snapshot())

		// Refresh cached game data so the next state_update / current_state
		// carries current scoreboard / roster data. Not broadcast separately
		// — state_update below carries the tick payload, current_state on
		// phase transitions carries the cache snapshot.
		if snap, err := r.reader.ReadReadyState(); err == nil {
			snap.GameState = gs
			r.gameData = snap
			r.publishGameData(snap)
			r.maybeEmitScenario(svc)
		}

		// Tick the host runner during a live match too (throttled). In-game it
		// just reports "match live" on the observable stream — the auto-host loop
		// re-engages once the match ends and the loop returns to Ready.
		r.tickHost(gs, tick)

		// One state_update per fresh engine tick (~30Hz).
		r.broadcastPoll(svc)

		events := r.reader.DetectEvents(tick, r.name, r.gameData, tickResult, r.state)
		for _, ev := range events {
			r.pushEvent(ev)
			r.broadcast(svc, ev)
		}
		// Viewer-facing death stream. Separate pass rather than a branch
		// inside the loop above: broadcast routes on env.Type, and the
		// filtered path re-types its envelopes, so the two can't share a
		// send. Demand-gated internally — no subscriber, no work.
		r.broadcastEventsFiltered(svc, events)

		lastBroadcastTick = tick
		r.sleepOrCancel(livePollInterval)
	}
}

// captureLiveAsPrevious moves the just-ended match's game data + per-match
// event log (oldest-first, see instanceCache.MatchEvents) into
// cache.PreviousGame and clears the live slots. Deferred from runLive so the
// data survives a panic / ctx-cancel / heartbeat fallout. endReason is the
// exit condition runLive observed (see the endReason* constants).
//
// The game_uid is minted HERE, once per captured game — never in a payload
// builder — so every downstream consumer of this capture sees one stable
// idempotency key.
func (r *runner) captureLiveAsPrevious(endReason string) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	if r.cache.GameData == nil && len(r.cache.MatchEvents) == 0 && len(r.cache.Events) == 0 {
		return
	}
	r.cache.PreviousGame = &previousGame{
		GameData:        r.cache.GameData,
		Events:          r.cache.MatchEvents,
		EventsTruncated: r.cache.MatchEventsTruncated,
		EndedAt:         time.Now(),
		GameUID:         newGameUID(),
		EndReason:       endReason,
	}
	r.cache.LatestTick = nil
	r.cache.Events = nil
	r.cache.MatchEvents = nil
	r.cache.MatchEventsTruncated = false
}

// releaseReader clears the bound GameReader and resets the cache fields
// that are only meaningful while a reader is in place. Called on
// Ready→Idle and Live→Idle transitions.
//
// XboxName / SystemScannedFor are intentionally NOT cleared — console name
// is an Xbox-the-machine fact independent of plugin lifecycle, supplied by
// runSystemSnapshot. The next runIdle iteration re-runs the snapshot only
// if the title actually changed (cache.TitleID != cache.SystemScannedFor).
func (r *runner) releaseReader() {
	r.reader = nil
	r.state = nil
	r.gameData = scraper.GameData{}
	r.powerItemsInitialised = false
	r.liveReadFailures = 0
	r.lastScenarioFingerprint = 0
	r.withCache(func(c *instanceCache) {
		c.Title = ""
		c.GameState = ""
		c.GameData = nil
		c.LatestTick = nil
		c.Events = nil
		// MatchEvents must go with GameData: the deferred capture keys off
		// both, and leaving a stale match log here would turn the xemu-death
		// path (heartbeat → Idle, deliberately no artifact) into a phantom
		// previous_game.
		c.MatchEvents = nil
		c.MatchEventsTruncated = false
		c.PreviousGame = nil
	})
}

// systemSnapshotInterval bounds how often a runner walks guest RAM for
// xbox/* system values. Set just below Idle's 3s tick so consecutive Idle
// iterations always re-scan (catches dashboard renames without title
// changes); Ready / Live call sites are naturally throttled to this
// cadence too without each caller doing its own arithmetic.
const systemSnapshotInterval = 2500 * time.Millisecond

// runSystemSnapshot reads xbox/* global system values (console name, and
// future EEPROM / kernel-clock / dashboard-pointer reads) into the cache.
// Throttled by systemSnapshotInterval so it's safe to call every loop
// iteration in any phase. Logs the first scan attempt per runner and any
// observed value change so stdout reflects whether the path is even firing
// — silent failure was the whole reason debugging this was hard.
func (r *runner) runSystemSnapshot() {
	if time.Since(r.lastSystemSnapshotAt) < systemSnapshotInterval {
		return
	}
	titleID := r.cachedTitleID()
	if titleID == 0 {
		// "Wait for xemu" gate — title-ID == 0 means the kernel hasn't
		// mapped the XBE header yet, and the dashboard certainly hasn't
		// loaded NICKNAME.XBN. xbox.ReadConsoleName is title-agnostic but
		// has nothing to find this early.
		return
	}
	firstAttempt := r.lastSystemSnapshotAt.IsZero()
	r.lastSystemSnapshotAt = time.Now()

	name := xbox.ReadConsoleName(r.inst.Mem)
	serial := xbox.ReadSerialNumber(r.inst.Mem)
	mac := xbox.ReadMACAddress(r.inst.Mem)
	video := xbox.ReadVideoStandard(r.inst.Mem)
	tz, tzOK := xbox.ReadTimeZone(r.inst.Mem)
	cert, certOK := xbox.ReadXBECertificate(r.inst)
	clock, clockOK := xbox.ReadSystemClock(r.inst.Mem)

	r.cacheMu.Lock()
	prev := r.cache.XboxName
	prevXBETitle := r.cache.XBETitleName
	if name != "" {
		r.cache.XboxName = name
	}
	if serial != "" {
		r.cache.SerialNumber = serial
	}
	if mac != "" {
		r.cache.MACAddress = mac
	}
	if video != "" {
		r.cache.VideoStandard = video
	}
	if tzOK {
		r.cache.TimeZoneBias = tz.BiasMinutes
		r.cache.TimeZoneStdName = tz.StdName
		r.cache.TimeZoneDltName = tz.DltName
	}
	if certOK {
		r.cache.XBETitleName = cert.TitleName
		r.cache.XBEVersion = cert.Version
		r.cache.XBEGameRegion = cert.GameRegion
		r.cache.XBEDiskNumber = cert.DiskNumber
		r.cache.XBEAllowedMedia = cert.AllowedMedia
	}
	if clockOK {
		r.cache.KernelSystemTime = clock.SystemTime
		r.cache.KernelBootTime = clock.BootTime
		r.cache.KernelUptime = clock.Uptime
	}
	r.cacheMu.Unlock()

	switch {
	case firstAttempt && name != "":
		log.Printf("scraper[%s]: console name = %q serial=%q mac=%s video=%s tz=%s/%s bias=%d xbe=%q ver=0x%08X region=%s (title 0x%08X)",
			r.name, name, serial, mac, video, tz.StdName, tz.DltName, tz.BiasMinutes,
			cert.TitleName, cert.Version, xbox.FormatGameRegion(cert.GameRegion), titleID)
	case firstAttempt:
		log.Printf("scraper[%s]: console-name scan: no match in window (title 0x%08X)", r.name, titleID)
	case name != "" && name != prev:
		log.Printf("scraper[%s]: console name changed: %q → %q (title 0x%08X)", r.name, prev, name, titleID)
	case certOK && cert.TitleName != "" && cert.TitleName != prevXBETitle:
		log.Printf("scraper[%s]: xbe title changed: %q → %q (title 0x%08X)", r.name, prevXBETitle, cert.TitleName, titleID)
	}
}

// publishPhase updates cache.Phase under cacheMu.
func (r *runner) publishPhase(p Phase) {
	r.cacheMu.Lock()
	r.cache.Phase = p
	r.cacheMu.Unlock()
}

// cachedTitleID returns the most recently observed title ID under cacheMu.
func (r *runner) cachedTitleID() uint32 {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	return r.cache.TitleID
}

// hasResolvedSpawnIDs reports whether at least one of spawns has its
// InitialObjectID resolved (non-0xFFFF). Used as the gate for state-tracker
// initialisation: the matchStaticCache fills these asynchronously after
// world objects exist, so the first few in-game ticks may still carry
// placeholders.
func hasResolvedSpawnIDs(spawns []scraper.PowerItemSpawn) bool {
	for _, s := range spawns {
		if s.InitialObjectID != 0xFFFF {
			return true
		}
	}
	return false
}

// sleepOrCancel waits for d, returning early if the context is cancelled.
// Using a timer + select instead of time.Sleep keeps Stop responsive.
func (r *runner) sleepOrCancel(d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-r.ctx.Done():
	case <-t.C:
	}
}

// broadcast wraps a scraper.Envelope inside a websocket.Message and pushes
// the serialised bytes to the per-class room for this runner and the
// envelope's type (host:<inst>:<class>). v2: routing is per-class — events
// land on host:<inst>:event, state-class envelopes (if ever broadcast via
// this generic path) land on host:<inst>:<class>.
func (r *runner) broadcast(svc *guards.Services, env scraper.Envelope) {
	if svc == nil || svc.WS == nil {
		return
	}
	room, err := rooms.RoomForInstanceClass(r.name, env.Type)
	if err != nil {
		log.Printf("scraper[%s]: cannot route envelope type %q: %v", r.name, env.Type, err)
		return
	}
	envBytes, err := json.Marshal(env)
	if err != nil {
		log.Printf("scraper[%s]: marshal envelope (%s): %v", r.name, env.Type, err)
		return
	}
	msgBytes, ok := wrapRoomMessage(r.name, room, envBytes)
	if !ok {
		return
	}
	svc.WS.SendToRoomRaw(room, msgBytes)
	if r.sinks != nil {
		r.sinks.write(env.Type, envBytes)
	}
}
