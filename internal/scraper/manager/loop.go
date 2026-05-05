package manager

import (
	"encoding/json"
	"log"
	"runtime/debug"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
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

// inGameDataBroadcastEvery throttles in-game game-data rebroadcasts.
// GameData carries scoreboard / roster / score-limit fields the overlay
// wants live, but most of it (map, spawns, fog) is scenario-static —
// rebroadcasting every 30Hz tick wastes bandwidth. Every 5 ticks is ~167ms
// update latency, which feels live to a viewer.
const inGameDataBroadcastEvery = 5

// Legacy wire envelope type strings. M5 stage 5a deliberately retired the
// "snapshot" term internally (the cache is instanceCache.GameData, the
// reader method is ReadGameData, the join-replay helper is
// JoinReplayMessages), but the wire envelope `Type:"snapshot"` stays in
// place until M5 stage 5c replaces it with `current_state` /
// `state_update`. These constants are the only sanctioned source of those
// strings — any other reference is a leak.
const (
	envelopeTypeGameData = "snapshot"
	envelopeTypeTick     = "tick"
	envelopeTypeEvent    = "event"
)

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
		if rec := recover(); rec != nil {
			log.Printf("scraper[%s]: panic in tick loop: %v\n%s", r.name, rec, debug.Stack())
		}
	}()

	r.publishPhase(PhaseIdle)
	phase := PhaseIdle

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
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
		r.sleepOrCancel(idlePollInterval)
		return PhaseIdle
	}

	r.recordIteration(0)
	r.withCache(func(c *instanceCache) {
		c.TitleID = titleID
	})

	factory := scraper.Lookup(titleID)
	if factory == nil {
		// Unknown title — stay idle and re-poll. The TitleID is already
		// surfaced in the cache so the debug page can show "phase=idle,
		// title_id=0x...".
		r.sleepOrCancel(idlePollInterval)
		return PhaseIdle
	}

	// Title recognised — bind a reader. Re-init the xemu instance with the
	// reader's required low GVAs (xemu.Instance.Init is idempotent for
	// already-translated addresses, so the detection-only init done at
	// Start time is preserved).
	reader := factory(r.inst, r.name)
	allGVAs := append(scraper.DetectionGVAs(), reader.LowGVAs()...)
	if err := r.inst.Init(allGVAs); err != nil {
		log.Printf("scraper[%s]: bind reader (init low GVAs): %v — staying idle", r.name, err)
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
		c.XboxName = reader.XboxName()
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
// Within Ready, every observed GameState transition broadcasts a fresh
// game-data envelope (legacy wire type "snapshot" until M5 5c replaces
// it with current_state) so existing overlay clients see lobby joins /
// team swaps / match start without waiting for Live to begin.
func (r *runner) runReady(svc *guards.Services) Phase {
	prevState := scraper.GameState("")
	titleCheckCount := 0

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
			r.sleepOrCancel(readyPollInterval)
			continue
		}
		r.recordIteration(tick)
		r.publishGameState(gs, r.reader.LastStateInputs(), r.reader.BuildScoreProbe())

		// State-transition handling within Ready — broadcast a fresh
		// game-data envelope so the overlay sees the new lobby state.
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
				r.broadcast(svc, scraper.MakeEnvelope(envelopeTypeGameData, r.name, tick, snap))
				// Push a summary on the state-transition path so host:all
				// reflects map / gametype / score changes right when they
				// happen rather than on the next heartbeat tick.
				r.publishSummary()
				prevState = gs
			}
		}

		// Ready → Live transition: in_game observed.
		if gs == scraper.GameStateInGame {
			r.state = r.reader.NewTickState()
			r.powerItemsInitialised = false
			r.liveReadFailures = 0
			return PhaseLive
		}

		// Cheap game-data refresh so the inspect endpoint sees current
		// scoreboard / roster data without waiting for the next state
		// transition. Not broadcast — debug page polls HTTP at 3s.
		if snap, err := r.reader.ReadReadyState(); err == nil {
			snap.GameState = gs
			r.gameData = snap
			r.publishGameData(snap)
		}

		r.sleepOrCancel(readyPollInterval)
	}
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
	defer r.captureLiveAsPrevious()

	var lastBroadcastTick uint32
	gameDataBroadcastCount := 0

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
		r.publishGameState(gs, r.reader.LastStateInputs(), r.reader.BuildScoreProbe())

		if gs != scraper.GameStateInGame {
			log.Printf("scraper[%s]: state in_game → %s tick=%d — live → ready", r.name, gs, tick)
			if err := r.reader.OnStateChange(scraper.GameStateInGame, gs); err != nil {
				log.Printf("scraper[%s]: OnStateChange live→ready: %v", r.name, err)
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
		r.broadcast(svc, scraper.MakeEnvelope(envelopeTypeTick, r.name, tick, tickResult.Payload))

		// Refresh the cached game data via ReadReadyState so the inspect
		// endpoint sees current scoreboard / roster data. Broadcast every
		// inGameDataBroadcastEvery ticks (~167ms at 30Hz) so the
		// overlay sees roster / score updates without flooding.
		if snap, err := r.reader.ReadReadyState(); err == nil {
			snap.GameState = gs
			r.gameData = snap
			r.publishGameData(snap)
			gameDataBroadcastCount++
			if gameDataBroadcastCount >= inGameDataBroadcastEvery {
				gameDataBroadcastCount = 0
				r.broadcast(svc, scraper.MakeEnvelope(envelopeTypeGameData, r.name, tick, snap))
			}
		}

		events := r.reader.DetectEvents(tick, r.name, r.gameData, tickResult, r.state)
		for _, ev := range events {
			r.pushEvent(ev)
			r.broadcast(svc, ev)
		}

		lastBroadcastTick = tick
		r.sleepOrCancel(livePollInterval)
	}
}

// captureLiveAsPrevious moves the just-ended match's game data + event log
// into cache.PreviousGame and clears the live slots. Deferred from runLive
// so the data survives a panic / ctx-cancel / heartbeat fallout.
func (r *runner) captureLiveAsPrevious() {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	if r.cache.GameData == nil && len(r.cache.Events) == 0 {
		return
	}
	r.cache.PreviousGame = &previousGame{
		GameData: r.cache.GameData,
		Events:    r.cache.Events,
		EndedAt:   time.Now(),
	}
	r.cache.LatestTick = nil
	r.cache.Events = nil
}

// releaseReader clears the bound GameReader and resets the cache fields
// that are only meaningful while a reader is in place. Called on
// Ready→Idle and Live→Idle transitions.
func (r *runner) releaseReader() {
	r.reader = nil
	r.state = nil
	r.gameData = scraper.GameData{}
	r.powerItemsInitialised = false
	r.liveReadFailures = 0
	r.withCache(func(c *instanceCache) {
		c.Title = ""
		c.XboxName = ""
		c.GameState = ""
		c.StateInputs = nil
		c.ScoreProbe = nil
		c.GameData = nil
		c.LatestTick = nil
		c.Events = nil
		c.PreviousGame = nil
	})
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
// the serialised bytes to this runner's per-instance host room. M5 stage 5b:
// the room is host:<name> (computed once at Manager.Start via the
// rooms.RoomForInstance chokepoint and cached in r.hostRoom), replacing
// the single shared "overlay" room from earlier stages.
func (r *runner) broadcast(svc *guards.Services, env scraper.Envelope) {
	if svc == nil || svc.WS == nil {
		return
	}
	envBytes, err := json.Marshal(env)
	if err != nil {
		log.Printf("scraper[%s]: marshal envelope (%s): %v", r.name, env.Type, err)
		return
	}
	msg := websocket.Message{
		Type:    "scraper",
		Room:    r.hostRoom,
		Payload: envBytes,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("scraper[%s]: marshal message: %v", r.name, err)
		return
	}
	svc.WS.SendToRoomRaw(r.hostRoom, msgBytes)
}
