package manager

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/xemu-cartographer/xc-scraper/scraper"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// TestFinishedGameFromPrevious: the capture-minted identity (game_uid,
// end_reason) and the capture-time context (game key, final tick, events
// truncation) ride the GameData→FinishedGame projection into the wire
// artifact; nil / empty captures stay non-recordable.
func TestFinishedGameFromPrevious(t *testing.T) {
	endedAt := time.Date(2026, 8, 30, 20, 0, 0, 0, time.UTC)
	full := &previousGame{
		GameData: &scraper.GameData{
			Map:      "bloodgulch",
			Gametype: "slayer",
			Players:  []scraper.GamePlayer{{Index: 0, Name: "STEW", Kills: 5}},
		},
		EndedAt:         endedAt,
		GameUID:         "0198f00dcafe00112233445566778899",
		EndReason:       endReasonPostgame,
		EventsTruncated: true,
		Instance:        "ignored-by-projection",
		GameKey:         "haloce",
		FinalTick:       54_000,
	}

	cases := []struct {
		name   string
		pg     *previousGame
		wantOK bool
	}{
		{"nil capture", nil, false},
		{"capture without game data", &previousGame{GameUID: "abc"}, false},
		{"full capture", full, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fg, ok := finishedGameFromPrevious("alpha", tc.pg)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if fg.Schema != wire.SchemaFinishedGame {
				t.Fatalf("schema = %q, want %q", fg.Schema, wire.SchemaFinishedGame)
			}
			if fg.GameUID != full.GameUID || fg.EndReason != full.EndReason {
				t.Fatalf("identity not threaded: uid=%q reason=%q", fg.GameUID, fg.EndReason)
			}
			if fg.Instance != "alpha" || fg.Game != "haloce" || fg.Map != "bloodgulch" || !fg.EndedAt.Equal(endedAt) {
				t.Fatalf("projection wrong: %+v", fg)
			}
			if fg.DurationTicks != 54_000 || fg.TickRateHz != defaultTickRateHz || !fg.EventsTruncated {
				t.Fatalf("capture context not threaded: %+v", fg)
			}
			if fg.StartedAt != nil || fg.Origin != "" || fg.Ext != nil {
				t.Fatalf("gap fields must stay unset: %+v", fg)
			}
			if len(fg.Players) != 1 || fg.Players[0].Name != "STEW" || fg.Players[0].Kills != 5 {
				t.Fatalf("players wrong: %+v", fg.Players)
			}
		})
	}
}

// TestFinishedGameFromGameData_TeamsAndHost: winner is the UNIQUE
// highest-scoring team (tie → nil, FFA → nil), team_scores are mapped
// verbatim, and host_machine_name is the local machine of the roster.
func TestFinishedGameFromGameData_TeamsAndHost(t *testing.T) {
	yes, no := true, false
	base := func() *scraper.GameData {
		return &scraper.GameData{
			Map:        "sidewinder",
			Gametype:   "ctf",
			IsTeamGame: true,
			ScoreLimit: 3,
			TeamScores: []scraper.TeamScore{{Team: 0, Score: 3}, {Team: 1, Score: 1}},
			Machines: []scraper.GameMachine{
				{Index: 0, Name: "away-box", IsLocal: &no},
				{Index: 1, Name: "home-box", IsLocal: &yes},
			},
		}
	}

	t.Run("unique winner", func(t *testing.T) {
		fg, ok := finishedGameFromGameData("alpha", base(), time.Now(), nil)
		if !ok {
			t.Fatal("ok = false")
		}
		if fg.WinnerTeam == nil || *fg.WinnerTeam != 0 {
			t.Fatalf("winner = %v, want 0", fg.WinnerTeam)
		}
		if fg.HostMachineName != "home-box" {
			t.Fatalf("host = %q, want home-box", fg.HostMachineName)
		}
		if !fg.IsTeamGame || fg.ScoreLimit != 3 {
			t.Fatalf("match-static fields wrong: %+v", fg)
		}
		want := []wire.GameTeamScore{{Team: 0, Score: 3}, {Team: 1, Score: 1}}
		if len(fg.TeamScores) != 2 || fg.TeamScores[0] != want[0] || fg.TeamScores[1] != want[1] {
			t.Fatalf("team_scores = %+v, want %+v", fg.TeamScores, want)
		}
	})
	t.Run("tie has no winner", func(t *testing.T) {
		gd := base()
		gd.TeamScores[1].Score = 3
		fg, _ := finishedGameFromGameData("alpha", gd, time.Now(), nil)
		if fg.WinnerTeam != nil {
			t.Fatalf("tie: winner = %d, want nil", *fg.WinnerTeam)
		}
	})
	t.Run("ffa has no winner", func(t *testing.T) {
		gd := base()
		gd.IsTeamGame = false
		fg, _ := finishedGameFromGameData("alpha", gd, time.Now(), nil)
		if fg.WinnerTeam != nil {
			t.Fatalf("ffa: winner = %d, want nil", *fg.WinnerTeam)
		}
	})
	t.Run("no local machine", func(t *testing.T) {
		gd := base()
		gd.Machines = nil
		fg, _ := finishedGameFromGameData("alpha", gd, time.Now(), nil)
		if fg.HostMachineName != "" {
			t.Fatalf("host = %q, want empty", fg.HostMachineName)
		}
	})
	t.Run("empty roster is still ok but has no players", func(t *testing.T) {
		fg, ok := finishedGameFromGameData("alpha", base(), time.Now(), nil)
		if !ok || len(fg.Players) != 0 || fg.Players == nil || fg.TeamScores == nil {
			t.Fatalf("empty roster: ok=%v players=%v (slices must marshal as [] not null)", ok, fg.Players)
		}
	})
}

// TestFinishedGameFromGameData_Players: every per-player core counter is
// carried, the pointer-typed engine fields pass through, and the Halo
// accumulator optionals are present only for players the accumulator saw.
func TestFinishedGameFromGameData_Players(t *testing.T) {
	yes := true
	mi, ci := 1, 0
	gd := &scraper.GameData{
		Map: "prisoner", Gametype: "slayer",
		Players: []scraper.GamePlayer{
			{Index: 0, Name: "STEW", Team: 1, Score: 25, Kills: 25, Deaths: 7, Assists: 3, Suicides: 1, TeamKills: 2,
				CTFScore: 4, Multikill: 3, IsLocal: &yes, MachineIndex: &mi, ControllerIndex: &ci},
			{Index: 2, Name: "GUEST", Team: 0, Score: 9, Kills: 9},
		},
	}
	accum := map[int]scraper.PlayerAccum{
		0: {ShotsFired: 400, Melees: 12, DamageDealt: 1234.5, DamageReceived: 987.25, BestKillStreak: 6},
	}
	fg, ok := finishedGameFromGameData("alpha", gd, time.Now(), accum)
	if !ok || len(fg.Players) != 2 {
		t.Fatalf("ok=%v players=%d", ok, len(fg.Players))
	}

	p := fg.Players[0]
	if p.Index != 0 || p.Name != "STEW" || p.Team != 1 || p.Score != 25 || p.Kills != 25 || p.Deaths != 7 ||
		p.Assists != 3 || p.Suicides != 1 || p.TeamKills != 2 {
		t.Fatalf("core counters wrong: %+v", p)
	}
	if p.IsLocal != &yes || p.MachineIndex != &mi || p.ControllerIndex != &ci {
		t.Fatalf("engine pointer fields must pass through: %+v", p)
	}
	if p.CTFScore == nil || *p.CTFScore != 4 || p.Multikill == nil || *p.Multikill != 3 {
		t.Fatalf("GamePlayer optionals wrong: ctf=%v multikill=%v", p.CTFScore, p.Multikill)
	}
	if p.BestKillStreak == nil || *p.BestKillStreak != 6 || p.AccShotsFired == nil || *p.AccShotsFired != 400 ||
		p.AccMelees == nil || *p.AccMelees != 12 || p.AccDamageDealt == nil || *p.AccDamageDealt != 1234.5 ||
		p.AccDamageReceived == nil || *p.AccDamageReceived != 987.25 {
		t.Fatalf("accumulator optionals wrong: %+v", p)
	}
	if p.TimeAliveMs != nil {
		t.Fatalf("time_alive_ms is reserved, got %v", *p.TimeAliveMs)
	}

	q := fg.Players[1]
	if q.Index != 2 || q.Name != "GUEST" || q.IsLocal != nil || q.MachineIndex != nil || q.ControllerIndex != nil {
		t.Fatalf("second player wrong: %+v", q)
	}
	if q.BestKillStreak != nil || q.AccShotsFired != nil || q.AccMelees != nil || q.AccDamageDealt != nil || q.AccDamageReceived != nil {
		t.Fatalf("accumulator optionals must be nil without an accumulator row: %+v", q)
	}
	if q.CTFScore == nil || *q.CTFScore != 0 || q.Multikill == nil || *q.Multikill != 0 {
		t.Fatalf("GamePlayer optionals are always carried (zero, not nil): %+v", q)
	}
}

// TestPreviousGamePayloadEmbedsFinishedGame: the previous_game payload
// carries the artifact distilled from the same capture — same game_uid,
// same end_reason, the runner's name as instance — and omits it (nil)
// when there is no game data to distil from.
func TestPreviousGamePayloadEmbedsFinishedGame(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.EngineTick = 9_000
		c.GameData = &scraper.GameData{
			Map: "damnation", Gametype: "slayer",
			Players: []scraper.GamePlayer{{Index: 0, Name: "STEW", Kills: 1}},
		}
	})
	r.captureLiveAsPrevious(endReasonPostgame)

	c := r.readCache()
	p := buildPreviousGamePayload(&c)
	if p == nil || p.FinishedGame == nil {
		t.Fatalf("previous_game payload must embed finished_game: %+v", p)
	}
	fg := p.FinishedGame
	if fg.GameUID != p.GameUID || fg.EndReason != p.EndReason || fg.Instance != "alpha" {
		t.Fatalf("embedded artifact identity drifted: %+v vs payload uid=%q reason=%q", fg, p.GameUID, p.EndReason)
	}
	if fg.DurationTicks != 9_000 || fg.Map != "damnation" || len(fg.Players) != 1 {
		t.Fatalf("embedded artifact content wrong: %+v", fg)
	}
	// A fresh copy per build: mutating one payload's artifact must not leak
	// into the next join replay.
	fg.Map = "mutated"
	if again := buildPreviousGamePayload(&c); again.FinishedGame.Map != "damnation" {
		t.Fatal("embedded artifact is shared between payload builds")
	}

	// Events-only capture (no game data) → no artifact, field omitted.
	r2 := newTestRunner("beta")
	defer r2.cancel()
	r2.pushEvent(makeEvent(4, scraper.EventTypeDeath))
	r2.captureLiveAsPrevious(endReasonLeftMatch)
	c2 := r2.readCache()
	if p2 := buildPreviousGamePayload(&c2); p2 == nil || p2.FinishedGame != nil {
		t.Fatalf("events-only capture must not embed finished_game: %+v", p2)
	}
}

// TestFireGameEndHook: the Live→Ready edge delivers the artifact to the
// GameEnd hook exactly once, off the loop goroutine, with the same identity
// the previous_game capture carries; no hook → nothing happens; an empty
// roster is not delivered.
func TestFireGameEndHook(t *testing.T) {
	t.Run("delivered once with capture identity", func(t *testing.T) {
		got := make(chan wire.FinishedGame, 4)
		roster := &scraper.GameData{
			Map: "bloodgulch", Gametype: "slayer",
			Players: []scraper.GamePlayer{{Index: 0, Name: "STEW"}},
		}
		r := newLiveRunner(t, &scriptedReader{liveTicks: 2, exitState: scraper.GameStatePostGame, readyState: roster})
		r.onGameEnd = func(fg wire.FinishedGame) { got <- fg }

		if next := r.runLive(); next != PhaseReady {
			t.Fatalf("runLive returned %q, want %q", next, PhaseReady)
		}
		if !r.awaitPersists(time.Second) {
			t.Fatal("hook goroutine did not finish")
		}
		pg := r.readCache().PreviousGame
		select {
		case fg := <-got:
			if fg.GameUID != pg.GameUID || fg.EndReason != endReasonPostgame || fg.Instance != "alpha" {
				t.Fatalf("hook artifact = %+v, want uid %q postgame alpha", fg, pg.GameUID)
			}
			if fg.Schema != wire.SchemaFinishedGame || len(fg.Players) != 1 {
				t.Fatalf("hook artifact shape wrong: %+v", fg)
			}
		default:
			t.Fatal("hook not invoked")
		}
		select {
		case fg := <-got:
			t.Fatalf("hook invoked twice: %+v", fg)
		default:
		}
	})

	t.Run("empty roster is skipped", func(t *testing.T) {
		var calls atomic.Int32
		r := newLiveRunner(t, &scriptedReader{liveTicks: 1, exitState: scraper.GameStatePostGame})
		r.onGameEnd = func(wire.FinishedGame) { calls.Add(1) }
		r.runLive()
		r.awaitPersists(time.Second)
		if calls.Load() != 0 {
			t.Fatalf("hook invoked %d times for an empty roster", calls.Load())
		}
	})

	t.Run("nil hook is a no-op", func(t *testing.T) {
		roster := &scraper.GameData{Players: []scraper.GamePlayer{{Index: 0, Name: "STEW"}}}
		r := newLiveRunner(t, &scriptedReader{liveTicks: 1, exitState: scraper.GameStatePostGame, readyState: roster})
		r.runLive()
		if !r.awaitPersists(time.Second) {
			t.Fatal("nil hook must not leave a WaitGroup entry")
		}
	})
}

// TestStopWaitsForInFlightPersist: Manager.Stop must not return while a
// game-end hook goroutine is still running — the shutdown flush that
// stops a Ctrl-C at match end from racing the write to process exit.
func TestStopWaitsForInFlightPersist(t *testing.T) {
	m := New(Options{})
	defer m.Close()

	r := newTestRunner("alpha")
	close(r.done) // no loop goroutine in this test; Stop's <-r.done must not block
	m.runners["alpha"] = r

	var flushed atomic.Bool
	r.persistWG.Add(1)
	go func() {
		time.Sleep(50 * time.Millisecond) // simulated in-flight PB chain
		flushed.Store(true)
		r.persistWG.Done()
	}()

	if err := m.Stop("alpha"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !flushed.Load() {
		t.Fatal("Stop returned before the in-flight persist finished")
	}
}

// TestAwaitPersistsTimesOut: a hook that never completes releases the
// waiter after the deadline (reporting false) so a wedged DB can't hang
// shutdown.
func TestAwaitPersistsTimesOut(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	r.persistWG.Add(1)
	defer r.persistWG.Done() // unblock the leaked waiter goroutine

	start := time.Now()
	if r.awaitPersists(30 * time.Millisecond) {
		t.Fatal("awaitPersists reported completion for a hung persist")
	}
	if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
		t.Fatalf("awaitPersists returned after %v, want >= 30ms", elapsed)
	}
}

// TestAwaitPersistsImmediateWhenIdle: no in-flight hooks → no delay.
func TestAwaitPersistsImmediateWhenIdle(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	if !r.awaitPersists(time.Second) {
		t.Fatal("awaitPersists with an idle WaitGroup reported timeout")
	}
}
