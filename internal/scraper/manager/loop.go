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

// OverlayRoom is the WebSocket room name scraper broadcasts target. Clients
// must send {"type":"join_room","room":"overlay"} (or "overlay:foo") to
// subscribe. RequireAuth gates membership — see internal/websocket/rooms/overlay.go.
const OverlayRoom = "overlay"

// loop is the per-runner tick goroutine. Started by Manager.Start, exits when
// ctx is cancelled (Manager.Stop). Always closes the xemu instance and
// signals done on exit, even on panic, so Manager.Stop's <-r.done unblocks.
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

		// Game-state transition → log + emit fresh snapshot whenever we land in
		// a state where a snapshot has meaningful payload (PreGame: lobby; InGame
		// and PostGame: match data).
		if gs != prevState {
			if prevState != "" {
				log.Printf("scraper[%s]: state %s → %s tick=%d", r.name, prevState, gs, tick)
			} else {
				log.Printf("scraper[%s]: initial state %s tick=%d", r.name, gs, tick)
			}
			if gs == scraper.GameStateInGame || gs == scraper.GameStatePreGame || gs == scraper.GameStatePostGame {
				if snap, err := r.reader.ReadSnapshot(); err != nil {
					log.Printf("scraper[%s]: ReadSnapshot: %v", r.name, err)
				} else {
					r.snapshot = snap
					// Re-init tick state on entering a new game. Keeps event detection
					// from firing spurious "kill / death" diffs against last match's data.
					if gs == scraper.GameStateInGame || gs == scraper.GameStatePreGame {
						r.state = r.reader.NewTickState()
						r.state.InitPowerItems(snap.PowerItemSpawns)
					}
					r.broadcast(svc, scraper.MakeEnvelope("snapshot", r.name, tick, snap))
				}
			}
			prevState = gs
		}

		if gs == scraper.GameStateInGame {
			// Skip redundant ticks — game advances at 30Hz; we poll at ~100Hz so
			// usually 2/3 polls are duplicates. Only do the heavy reads on a fresh tick.
			if tick != lastBroadcastTick {
				tickResult, err := r.reader.ReadTick(r.snapshot.PowerItemSpawns, r.state)
				if err != nil {
					log.Printf("scraper[%s]: ReadTick: %v", r.name, err)
				} else {
					r.broadcast(svc, scraper.MakeEnvelope("tick", r.name, tick, tickResult.Payload))

					events := r.reader.DetectEvents(tick, r.name, r.snapshot, tickResult, r.state)
					for _, ev := range events {
						r.broadcast(svc, ev)
					}
					lastBroadcastTick = tick
				}
			}
			r.sleepOrCancel(inGamePollInterval)
		} else {
			r.sleepOrCancel(idlePollInterval)
		}
	}
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
		r.snapshotMu.Lock()
		r.latestSnapshotMsg = msgBytes
		r.snapshotMu.Unlock()
	}

	svc.WS.SendToRoomRaw(OverlayRoom, msgBytes)
}
