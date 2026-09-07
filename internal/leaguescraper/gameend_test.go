package leaguescraper

import (
	"bytes"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/games"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// TestFinishedGameFromWire: the wire artifact → league persistence input
// adaptation is exact — identity, match facts, the winner widening, and the
// per-player counters (engine name → Gamertag) — and leaves the league-only
// fields (SeriesID, StartedAt, TimeAliveMs) zero.
func TestFinishedGameFromWire(t *testing.T) {
	endedAt := time.Date(2026, 8, 30, 20, 0, 0, 0, time.UTC)
	winner := uint32(1)
	yes := true

	cases := []struct {
		name string
		in   wire.FinishedGame
		want games.FinishedGame
	}{
		{
			name: "team game with winner and two players",
			in: wire.FinishedGame{
				Schema:          wire.SchemaFinishedGame,
				GameUID:         "0198f00dcafe00112233445566778899",
				Instance:        "alpha",
				Game:            "haloce",
				HostMachineName: "home-box",
				Map:             "sidewinder",
				Gametype:        "ctf",
				VariantName:     "CTF Classic",
				IsTeamGame:      true,
				ScoreLimit:      3,
				EndedAt:         endedAt,
				EndReason:       wire.EndReasonPostgame,
				DurationTicks:   54_000,
				TickRateHz:      30,
				WinnerTeam:      &winner,
				TeamScores:      []wire.GameTeamScore{{Team: 0, Score: 1}, {Team: 1, Score: 3}},
				ScoreSummary:    "Red 1 – Blue 3",
				Players: []wire.FinishedGamePlayer{
					{Index: 0, Name: "STEW", Team: 1, Score: 3, Kills: 25, Deaths: 7, Assists: 3, Suicides: 1, TeamKills: 2, IsLocal: &yes},
					{Index: 1, Name: "GUEST", Team: 0, Score: 1, Kills: 9, Deaths: 20, Assists: 0},
				},
				EventsTruncated: false,
				Origin:          "xc-scraper",
			},
			want: games.FinishedGame{
				Container:       "alpha",
				HostMachineName: "home-box",
				Map:             "sidewinder",
				Gametype:        "ctf",
				VariantName:     "CTF Classic",
				EndedAt:         endedAt,
				WinnerTeam:      intPtr(1),
				ScoreSummary:    "Red 1 – Blue 3",
				GameUID:         "0198f00dcafe00112233445566778899",
				EndReason:       wire.EndReasonPostgame,
				Players: []games.PlayerStat{
					{Gamertag: "STEW", Team: 1, Kills: 25, Deaths: 7, Assists: 3, Score: 3},
					{Gamertag: "GUEST", Team: 0, Kills: 9, Deaths: 20, Assists: 0, Score: 1},
				},
			},
		},
		{
			name: "ffa without winner or players",
			in: wire.FinishedGame{
				Schema:    wire.SchemaFinishedGame,
				GameUID:   "uid-2",
				Instance:  "beta",
				Map:       "prisoner",
				Gametype:  "slayer",
				EndedAt:   endedAt,
				EndReason: wire.EndReasonLeftMatch,
			},
			want: games.FinishedGame{
				Container: "beta",
				Map:       "prisoner",
				Gametype:  "slayer",
				EndedAt:   endedAt,
				GameUID:   "uid-2",
				EndReason: wire.EndReasonLeftMatch,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FinishedGameFromWire(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("FinishedGameFromWire mismatch\n got: %+v\nwant: %+v", got, tc.want)
			}
		})
	}
}

func intPtr(v int) *int { return &v }

// TestGameEndHook_LogsPersistError: the hook never panics or surfaces an
// error — a failing persistence chain (here: a blank DB without the games
// collections) is logged with the instance name.
func TestGameEndHook_LogsPersistError(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)

	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })

	hook := GameEndHook(app)
	hook(wire.FinishedGame{
		Schema:   wire.SchemaFinishedGame,
		GameUID:  "uid-3",
		Instance: "alpha",
		Map:      "bloodgulch",
		Gametype: "slayer",
		EndedAt:  time.Now(),
		Players:  []wire.FinishedGamePlayer{{Index: 0, Name: "STEW"}},
	})

	if !strings.Contains(buf.String(), "leaguescraper: persist finished game (instance=alpha):") {
		t.Fatalf("expected a persist error log line, got %q", buf.String())
	}
}
