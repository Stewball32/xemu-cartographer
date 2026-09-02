package manager

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// TestFinishedGameFromPrevious: the capture-minted identity (game_uid,
// end_reason) rides the GameData→FinishedGame projection into the
// persistence input; nil / empty captures stay non-persistable.
func TestFinishedGameFromPrevious(t *testing.T) {
	endedAt := time.Date(2026, 8, 30, 20, 0, 0, 0, time.UTC)
	full := &previousGame{
		GameData: &scraper.GameData{
			Map:      "bloodgulch",
			Gametype: "slayer",
			Players:  []scraper.GamePlayer{{Index: 0, Name: "STEW", Kills: 5}},
		},
		EndedAt:   endedAt,
		GameUID:   "0198f00dcafe00112233445566778899",
		EndReason: endReasonPostgame,
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
			if fg.GameUID != full.GameUID || fg.EndReason != full.EndReason {
				t.Fatalf("identity not threaded: uid=%q reason=%q", fg.GameUID, fg.EndReason)
			}
			if fg.Container != "alpha" || fg.Map != "bloodgulch" || !fg.EndedAt.Equal(endedAt) {
				t.Fatalf("projection wrong: %+v", fg)
			}
			if len(fg.Players) != 1 || fg.Players[0].Gamertag != "STEW" {
				t.Fatalf("players wrong: %+v", fg.Players)
			}
		})
	}
}

// TestStopWaitsForInFlightPersist: Manager.Stop must not return while a
// game-end persist goroutine is still running — the shutdown flush that
// stops a Ctrl-C at match end from racing the write to process exit.
func TestStopWaitsForInFlightPersist(t *testing.T) {
	m := New(nil)
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

// TestAwaitPersistsTimesOut: a persist that never completes releases the
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

// TestAwaitPersistsImmediateWhenIdle: no in-flight persists → no delay.
func TestAwaitPersistsImmediateWhenIdle(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()

	if !r.awaitPersists(time.Second) {
		t.Fatal("awaitPersists with an idle WaitGroup reported timeout")
	}
}
