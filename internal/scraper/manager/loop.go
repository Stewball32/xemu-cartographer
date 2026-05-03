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

// Tick cadence — copied from the legacy Go scraper. Halo: CE runs at 30Hz
// (~33ms per tick), so 10ms in-game gives ~3 polls/tick which is enough to
// catch every game-tick advance without busy-spinning. 500ms idle keeps the
// manager responsive to game-state transitions without thrashing memory reads
// while the player is in menus.
const (
	inGamePollInterval = 10 * time.Millisecond
	idlePollInterval   = 500 * time.Millisecond
)

// titleIDCheckInterval is the number of idle iterations between XBE title-ID
// re-checks. At idlePollInterval=500ms × 10 = ~5s. Skipped while in_game to
// keep the hot path clean — title swaps don't happen mid-match.
const titleIDCheckInterval = 10

// inGameSnapshotBroadcastEvery throttles in-game snapshot rebroadcasts. The
// snapshot carries scoreboard / roster / score-limit data that the overlay
// and debug page want to see live, but most snapshot fields (map, spawns,
// fog, object types) are scenario-static — re-sending the whole payload
// every game tick (30Hz) wastes bandwidth without a payoff. Every 5 ticks
// gives ~167ms update latency, which feels live to a viewer.
const inGameSnapshotBroadcastEvery = 5

// OverlayRoom is the WebSocket room name scraper broadcasts target. Clients
// must send {"type":"join_room","room":"overlay"} (or "overlay:foo") to
// subscribe. RequireAuth gates membership — see internal/websocket/rooms/overlay.go.
const OverlayRoom = "overlay"

// loop is the per-runner tick goroutine. Started by Manager.Start, exits when
// ctx is cancelled (Manager.Stop) or when the running XBE swaps to a different
// title. Always closes the xemu instance and signals done on exit, even on
// panic, so Manager.Stop's <-r.done unblocks.
func (r *runner) loop(svc *guards.Services) {
	defer close(r.done)
	defer r.inst.Close()
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("scraper[%s]: panic in tick loop: %v\n%s", r.name, rec, debug.Stack())
		}
	}()

	prevState := scraper.GameState("")
	var lastBroadcastTick uint32

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
		}

		gs, tick, err := r.reader.ReadGameState()
		if err != nil {
			log.Printf("scraper[%s]: ReadGameState: %v", r.name, err)
			r.sleepOrCancel(idlePollInterval)
			continue
		}
		r.recordTick(tick)
		r.cacheState(gs)
		r.cacheStateInputs(r.reader.LastStateInputs())
		r.cacheScoreProbe(r.reader.BuildScoreProbe())

		// State-transition handling: notify the reader (cache invalidation),
		// log, and broadcast a fresh "current state" snapshot whenever we
		// land in a state where the snapshot has meaningful payload.
		if gs != prevState {
			if err := r.reader.OnStateChange(prevState, gs); err != nil {
				log.Printf("scraper[%s]: OnStateChange %s → %s: %v", r.name, prevState, gs, err)
			}
			if prevState != "" {
				log.Printf("scraper[%s]: state %s → %s tick=%d", r.name, prevState, gs, tick)
			} else {
				log.Printf("scraper[%s]: initial state %s tick=%d", r.name, gs, tick)
			}
			// Reset per-match runner gates on entry to menu so the next
			// match starts fresh.
			if gs == scraper.GameStateMenu {
				r.powerItemsInitialised = false
			}
			snapshotOK := true
			if gs == scraper.GameStateInGame || gs == scraper.GameStatePreGame || gs == scraper.GameStatePostGame {
				if snap, err := r.reader.ReadSnapshot(); err != nil {
					log.Printf("scraper[%s]: ReadSnapshot: %v", r.name, err)
					snapshotOK = false
				} else {
					snap.GameState = gs
					r.snapshot = snap
					// Reset event-detection prev-state on entering a new
					// match so kill / death diffs aren't compared against
					// the previous match's counters.
					if gs == scraper.GameStateInGame || gs == scraper.GameStatePreGame {
						r.state = r.reader.NewTickState()
					}
					r.cacheSnapshot(snap)
					r.broadcast(svc, scraper.MakeEnvelope("snapshot", r.name, tick, snap))
				}
			}
			if snapshotOK {
				prevState = gs
			}
		}

		// Initialise power-item trackers once per match — gated on the
		// snapshot carrying real InitialObjectIDs (matchCache fills them
		// asynchronously after pregame → in_game). Until then 0xFFFF
		// placeholders are present; firing InitPowerItems against them
		// seeds trackers with sentinel values that never refresh.
		if !r.powerItemsInitialised && hasResolvedSpawnIDs(r.snapshot.PowerItemSpawns) && r.state != nil {
			r.state.InitPowerItems(r.snapshot.PowerItemSpawns)
			r.powerItemsInitialised = true
		}

		switch gs {
		case scraper.GameStateInGame:
			// Skip redundant ticks — game advances at 30Hz; we poll at
			// ~100Hz so usually 2/3 polls are duplicates. Only do the heavy
			// reads on a fresh tick.
			if tick != lastBroadcastTick {
				tickResult, err := r.reader.ReadTick(r.snapshot.PowerItemSpawns, r.state)
				if err != nil {
					log.Printf("scraper[%s]: ReadTick: %v", r.name, err)
				} else {
					r.cacheTick(tickResult.Payload)
					r.broadcast(svc, scraper.MakeEnvelope("tick", r.name, tick, tickResult.Payload))

					// Refresh the cached snapshot via ReadLobby every fresh
					// tick so the inspect endpoint sees current scoreboard
					// / roster data without waiting for the next state
					// transition. Cheap (~50–100 reads) once caches are warm.
					// Not broadcast — debug page polls HTTP at 3s.
					if snap, err := r.reader.ReadLobby(); err == nil {
						snap.GameState = gs
						r.snapshot = snap
						r.cacheSnapshot(snap)
					}

					events := r.reader.DetectEvents(tick, r.name, r.snapshot, tickResult, r.state)
					for _, ev := range events {
						r.broadcast(svc, ev)
					}
					lastBroadcastTick = tick
				}
			}
			r.sleepOrCancel(inGamePollInterval)
		default:
			// menu / pregame / postgame: cheap snapshot refresh so the
			// debug page sees lobby joins / team swaps / final scores
			// within ~500ms. Suppressed on menu where snap data is empty.
			if gs == scraper.GameStatePreGame || gs == scraper.GameStatePostGame {
				if snap, err := r.reader.ReadLobby(); err == nil {
					snap.GameState = gs
					r.snapshot = snap
					r.cacheSnapshot(snap)
				}
			}
			// Periodic XBE-swap check: re-read the title ID and self-stop
			// if the running XBE has changed (e.g. user exited Halo: CE
			// back to UnleashX).
			r.idlePollCount++
			if r.idlePollCount >= titleIDCheckInterval {
				r.idlePollCount = 0
				if titleID, err := scraper.ReadTitleID(r.inst); err != nil {
					log.Printf("scraper[%s]: title ID re-check failed: %v — stopping", r.name, err)
					return
				} else if titleID != r.titleID {
					log.Printf("scraper[%s]: title ID changed (0x%08X → 0x%08X), stopping", r.name, r.titleID, titleID)
					return
				}
			}
			r.sleepOrCancel(idlePollInterval)
		}
	}
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
// Using a timer + select instead of time.Sleep keeps Stop responsive — the
// 500ms idle interval would otherwise be the worst-case shutdown latency.
func (r *runner) sleepOrCancel(d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-r.ctx.Done():
	case <-t.C:
	}
}

// broadcast wraps a scraper.Envelope inside a websocket.Message and pushes the
// serialised bytes to the overlay room. Both layers serialise to JSON; clients
// parse the outer Message first, dispatch on Message.Type=="scraper", then
// parse Message.Payload as a scraper.Envelope.
//
// Wrap-not-extend (M2c decision in ROADMAP.md): keeps the wire schema uniform
// across all WebSocket rooms — every payload arrives wrapped in Message.
//
// Snapshot envelopes are also cached on the runner so the join_room handler
// can replay them to mid-match joiners.
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
		Room:    OverlayRoom,
		Payload: envBytes,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("scraper[%s]: marshal message: %v", r.name, err)
		return
	}

	if env.Type == "snapshot" {
		r.cacheMu.Lock()
		r.latestSnapshotMsg = msgBytes
		r.cacheMu.Unlock()
	} else if env.Type == "event" {
		r.cacheEvent(env)
	}

	svc.WS.SendToRoomRaw(OverlayRoom, msgBytes)
}
