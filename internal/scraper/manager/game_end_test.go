package manager

import (
	"errors"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// scriptedReader drives runLive without a live xemu: ReadGameState returns
// in_game for liveTicks reads (tick = read count), then either the exit
// state or — with failAfter — a persistent read error (the heartbeat /
// xemu-death path). DetectEvents emits one death per tick so the per-match
// log accumulates. Embeds fakeReader (hostrunner_test.go) for the rest of
// the GameReader surface.
type scriptedReader struct {
	fakeReader
	liveTicks uint32
	exitState scraper.GameState
	failAfter bool
	reads     uint32
}

func (s *scriptedReader) ReadGameState() (scraper.GameState, uint32, error) {
	s.reads++
	if s.reads <= s.liveTicks {
		return scraper.GameStateInGame, s.reads, nil
	}
	if s.failAfter {
		return "", 0, errors.New("scripted read failure")
	}
	return s.exitState, s.reads, nil
}

func (s *scriptedReader) DetectEvents(tick uint32, _ string, _ scraper.GameData, _ scraper.TickResult, _ *scraper.TickState) []scraper.Envelope {
	return []scraper.Envelope{makeEvent(tick, scraper.EventTypeDeath)}
}

// newLiveRunner wires a runner ready to enter runLive with the scripted
// reader bound and current game data in the cache, mirroring the state the
// Ready→Live transition leaves behind.
func newLiveRunner(t *testing.T, sr *scriptedReader) *runner {
	t.Helper()
	r := newTestRunner("alpha")
	t.Cleanup(r.cancel)
	r.reader = sr
	r.gameData = scraper.GameData{Map: "bloodgulch", Gametype: "slayer"}
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseLive
		c.GameData = &scraper.GameData{Map: "bloodgulch", Gametype: "slayer"}
	})
	return r
}

// TestRunLiveEndReasonByExitState: the Live→Ready edge derives end_reason
// from the observed exit state — only postgame earns "postgame"; menu and
// pregame (no postgame observed) are honestly "left_match". The capture also
// carries the per-match event log oldest-first.
func TestRunLiveEndReasonByExitState(t *testing.T) {
	cases := []struct {
		name       string
		exitState  scraper.GameState
		wantReason string
	}{
		{"postgame", scraper.GameStatePostGame, endReasonPostgame},
		{"quit to menu", scraper.GameStateMenu, endReasonLeftMatch},
		{"straight to next pregame", scraper.GameStatePreGame, endReasonLeftMatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newLiveRunner(t, &scriptedReader{liveTicks: 2, exitState: tc.exitState})

			if next := r.runLive(nil); next != PhaseReady {
				t.Fatalf("runLive returned %q, want %q", next, PhaseReady)
			}

			pg := r.readCache().PreviousGame
			if pg == nil {
				t.Fatal("no PreviousGame captured on Live→Ready")
			}
			if pg.EndReason != tc.wantReason {
				t.Fatalf("end_reason = %q, want %q", pg.EndReason, tc.wantReason)
			}
			if pg.GameUID == "" {
				t.Fatal("game_uid not minted at capture")
			}
			if len(pg.Events) != 2 || pg.Events[0].Tick != 1 || pg.Events[1].Tick != 2 {
				t.Fatalf("captured events: want oldest-first [1 2], got %v", tickList(pg.Events))
			}
		})
	}
}

// TestRunLiveShutdownEndReason: a ctx-cancel mid-match still captures the
// game (the deferred captureLiveAsPrevious) with end_reason "shutdown".
func TestRunLiveShutdownEndReason(t *testing.T) {
	r := newLiveRunner(t, &scriptedReader{})
	r.cancel() // cancelled before entry — the loop's first select exits

	if next := r.runLive(nil); next != PhaseLive {
		t.Fatalf("runLive returned %q, want %q on ctx-cancel", next, PhaseLive)
	}

	pg := r.readCache().PreviousGame
	if pg == nil {
		t.Fatal("ctx-cancel: no PreviousGame captured")
	}
	if pg.EndReason != endReasonShutdown {
		t.Fatalf("end_reason = %q, want %q", pg.EndReason, endReasonShutdown)
	}
	if pg.GameUID == "" {
		t.Fatal("ctx-cancel: game_uid not minted")
	}
}

// TestRunLiveHeartbeatFailureNoArtifact: the xemu-death path (persistent
// read failures → releaseReader → Idle) must NOT produce a previous_game —
// releaseReader clears the cache before the deferred capture runs, and that
// includes the new per-match log. Guards the documented "a match is recorded
// only if the scraper observes its end" semantics.
func TestRunLiveHeartbeatFailureNoArtifact(t *testing.T) {
	r := newLiveRunner(t, &scriptedReader{liveTicks: 2, failAfter: true})

	if next := r.runLive(nil); next != PhaseIdle {
		t.Fatalf("runLive returned %q, want %q after heartbeat failure", next, PhaseIdle)
	}

	c := r.readCache()
	if c.PreviousGame != nil {
		t.Fatalf("heartbeat failure: phantom PreviousGame captured: %+v", c.PreviousGame)
	}
	if len(c.Events) != 0 || len(c.MatchEvents) != 0 {
		t.Fatalf("heartbeat failure: event logs not cleared (ring=%d match=%d)", len(c.Events), len(c.MatchEvents))
	}
}

// TestRunLiveMatchLogResetBetweenMatches: each runLive entry starts a fresh
// per-match log, so back-to-back games capture disjoint event sets and
// distinct game_uids.
func TestRunLiveMatchLogResetBetweenMatches(t *testing.T) {
	r := newLiveRunner(t, &scriptedReader{liveTicks: 2, exitState: scraper.GameStatePostGame})
	if next := r.runLive(nil); next != PhaseReady {
		t.Fatalf("first match: runLive returned %q, want %q", next, PhaseReady)
	}
	first := r.readCache().PreviousGame
	if first == nil || len(first.Events) != 2 {
		t.Fatalf("first match: capture wrong: %+v", first)
	}

	r.reader = &scriptedReader{liveTicks: 3, exitState: scraper.GameStatePostGame}
	if next := r.runLive(nil); next != PhaseReady {
		t.Fatalf("second match: runLive returned %q, want %q", next, PhaseReady)
	}
	second := r.readCache().PreviousGame
	if second == nil || len(second.Events) != 3 {
		t.Fatalf("second match: want 3 events (fresh log), got %+v", second)
	}
	if second.GameUID == first.GameUID {
		t.Fatalf("second match reused game_uid %q", first.GameUID)
	}
}
