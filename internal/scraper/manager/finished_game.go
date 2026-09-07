package manager

import (
	"time"

	"github.com/xemu-cartographer/xc-scraper/scraper"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// persistFlushTimeout bounds how long Manager.Stop waits for in-flight
// GameEnd hook goroutines (runner.awaitPersists) before abandoning them.
// Long enough for a PB transaction + hook chain on a healthy box, short
// enough that a wedged DB can't hang shutdown.
const persistFlushTimeout = 5 * time.Second

// defaultTickRateHz is the engine tick rate stamped on every finished_game
// artifact. Both registered game profiles (Halo: CE, Halo 2) run a 30 Hz
// engine tick — the same clock EngineTick counts on — and no GameReader
// exposes a per-title rate yet, so this is the one constant.
const defaultTickRateHz uint16 = 30

// fireGameEnd is the game-end trigger: on the Live→Ready edge it projects the
// just-captured game into a wire.FinishedGame and hands it to the Manager's
// GameEnd hook (Options.OnGameEnd — the league server's persistence chain).
// Deferred in runLive AFTER captureLiveAsPrevious (LIFO), so
// cache.PreviousGame is populated by the time it runs.
//
// Best-effort: the hook runs on its own goroutine — a slow or failing
// consumer must never stall the scraper loop. The goroutine is tracked on
// r.persistWG (Add happens synchronously, before this returns) so shutdown
// can flush it instead of racing process exit — see Manager.Stop /
// awaitPersists. No-op without a hook (test harnesses) or on an empty game.
//
// LIVE GAP: this path can only be verified end-to-end against a real game
// (no xemu here). The projection (finishedGameFromPrevious) is unit-tested
// against the wire shape, but the live GameData→FinishedGame mapping itself
// awaits a live Halo: CE match.
func (r *runner) fireGameEnd() {
	hook := r.onGameEnd
	if hook == nil {
		return
	}
	r.cacheMu.Lock()
	fg, ok := finishedGameFromPrevious(r.name, r.cache.PreviousGame)
	r.cacheMu.Unlock()
	if !ok || len(fg.Players) == 0 {
		return
	}
	r.persistWG.Add(1)
	go func() {
		defer r.persistWG.Done()
		hook(fg)
	}()
}

// awaitPersists blocks until every in-flight GameEnd hook goroutine has
// finished, or d elapses. Reports whether the flush completed. Called by
// Manager.Stop after the loop goroutine has exited — runLive's deferred
// fireGameEnd does its WaitGroup Add before the loop returns, so there is
// no Add/Wait race.
func (r *runner) awaitPersists(d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		r.persistWG.Wait()
		close(done)
	}()
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-done:
		return true
	case <-t.C:
		return false
	}
}

// finishedGameFromPrevious projects a captured previousGame into the
// wire.FinishedGame artifact (SchemaFinishedGame): the capture-minted
// identity (GameUID — the dedupe key across at-least-once deliveries — and
// EndReason), the capture-time context (game key, final tick, accumulator
// snapshot) and the GameData projection. ok=false on a nil capture or one
// without game data.
//
// name is the runner / instance name. StartedAt is intentionally nil (known
// gap — see wire.FinishedGame); Origin is left "" for the emitting daemon.
func finishedGameFromPrevious(name string, pg *previousGame) (wire.FinishedGame, bool) {
	if pg == nil {
		return wire.FinishedGame{}, false
	}
	fg, ok := finishedGameFromGameData(name, pg.GameData, pg.EndedAt, pg.PlayerAccum)
	if !ok {
		return fg, false
	}
	fg.GameUID = pg.GameUID
	fg.EndReason = pg.EndReason
	fg.Game = pg.GameKey
	fg.DurationTicks = pg.FinalTick
	fg.EventsTruncated = pg.EventsTruncated
	return fg, true
}

// finishedGameFromGameData projects a captured scraper.GameData (plus the
// per-player accumulator snapshot, may be nil) into the wire.FinishedGame
// core. Returns ok=false when there's nothing worth recording (nil data).
func finishedGameFromGameData(name string, gd *scraper.GameData, endedAt time.Time, accum map[int]scraper.PlayerAccum) (wire.FinishedGame, bool) {
	if gd == nil {
		return wire.FinishedGame{}, false
	}

	fg := wire.FinishedGame{
		Schema:       wire.SchemaFinishedGame,
		Instance:     name,
		Map:          gd.Map,
		Gametype:     gd.Gametype,
		VariantName:  gd.VariantName,
		IsTeamGame:   gd.IsTeamGame,
		ScoreLimit:   gd.ScoreLimit,
		EndedAt:      endedAt,
		TickRateHz:   defaultTickRateHz,
		TeamScores:   make([]wire.GameTeamScore, 0, len(gd.TeamScores)),
		ScoreSummary: renderScoreSummary(gd),
		Players:      make([]wire.FinishedGamePlayer, 0, len(gd.Players)),
	}

	// Host machine name: the local machine in the system-link roster.
	for _, m := range gd.Machines {
		if m.IsLocal != nil && *m.IsLocal {
			fg.HostMachineName = m.Name
			break
		}
	}

	for _, ts := range gd.TeamScores {
		fg.TeamScores = append(fg.TeamScores, wire.GameTeamScore{Team: ts.Team, Score: ts.Score})
	}

	// Winner: the unique highest-scoring team (team games only). A tie or a
	// non-team game leaves WinnerTeam nil (no winner recorded).
	if gd.IsTeamGame && len(gd.TeamScores) > 0 {
		best, bestScore, tie := -1, int32(0), false
		for i, ts := range gd.TeamScores {
			if i == 0 || ts.Score > bestScore {
				best, bestScore, tie = int(ts.Team), ts.Score, false
			} else if ts.Score == bestScore {
				tie = true
			}
		}
		if best >= 0 && !tie {
			w := uint32(best)
			fg.WinnerTeam = &w
		}
	}

	for _, p := range gd.Players {
		fp := wire.FinishedGamePlayer{
			Index:           p.Index,
			Name:            p.Name,
			Team:            p.Team,
			Score:           p.Score,
			Kills:           p.Kills,
			Deaths:          p.Deaths,
			Assists:         p.Assists,
			Suicides:        p.Suicides,
			TeamKills:       p.TeamKills,
			IsLocal:         p.IsLocal,
			MachineIndex:    p.MachineIndex,
			ControllerIndex: p.ControllerIndex,
			CTFScore:        ptr(p.CTFScore),
			Multikill:       ptr(p.Multikill),
		}
		// Accumulated match stats (best streak, shots, melees, damage) only
		// exist when the live accumulator ran for this player.
		if acc, ok := accum[p.Index]; ok {
			fp.BestKillStreak = ptr(acc.BestKillStreak)
			fp.AccShotsFired = ptr(acc.ShotsFired)
			fp.AccMelees = ptr(acc.Melees)
			fp.AccDamageDealt = ptr(acc.DamageDealt)
			fp.AccDamageReceived = ptr(acc.DamageReceived)
		}
		fg.Players = append(fg.Players, fp)
	}

	return fg, true
}

// ptr returns a pointer to a copy of v — for the optional (pointer-typed)
// wire.FinishedGamePlayer fields.
func ptr[T any](v T) *T { return &v }
