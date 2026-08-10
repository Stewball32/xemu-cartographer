package manager

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/roster"
)

func boolp(b bool) *bool { return &b }

// buildGameFilteredPayload applies the unified dummy rule: local seats hidden
// until latched Active by the accumulator; is_neutral_host = hard override;
// remotes untouched. The raw buildGamePayload (debug) stays unfiltered.
func TestBuildGameFilteredPayload_ActivityRule(t *testing.T) {
	cache := func(accum map[int]scraper.PlayerAccum) *instanceCache {
		return &instanceCache{
			GameData: &scraper.GameData{
				Players: []scraper.GamePlayer{
					{Index: 0, Name: "IdleLocal", IsLocal: boolp(true)},
					{Index: 1, Name: "Remote", IsLocal: boolp(false)},
				},
			},
			PlayerAccum: accum,
		}
	}

	t.Run("pre-activity: local hidden, remote kept", func(t *testing.T) {
		p := buildGameFilteredPayload(cache(nil), roster.Config{})
		if len(p.Players) != 1 || p.Players[0].Name != "Remote" {
			t.Fatalf("players = %+v, want only Remote", p.Players)
		}
	})

	t.Run("latched Active local becomes visible", func(t *testing.T) {
		p := buildGameFilteredPayload(cache(map[int]scraper.PlayerAccum{0: {Active: true}}), roster.Config{})
		if len(p.Players) != 2 {
			t.Fatalf("players = %+v, want both", p.Players)
		}
	})

	t.Run("neutral-host override hides an Active local", func(t *testing.T) {
		p := buildGameFilteredPayload(
			cache(map[int]scraper.PlayerAccum{0: {Active: true}}),
			roster.Config{IsNeutralHost: true},
		)
		if len(p.Players) != 1 || p.Players[0].Name != "Remote" {
			t.Fatalf("players = %+v, want only Remote", p.Players)
		}
	})

	t.Run("raw game payload stays unfiltered and carries acc stats", func(t *testing.T) {
		c := cache(map[int]scraper.PlayerAccum{0: {ShotsFired: 7, BestKillStreak: 3}})
		p := buildGamePayload(c)
		if len(p.Players) != 2 {
			t.Fatalf("raw payload filtered: %+v", p.Players)
		}
		if p.Players[0].AccShotsFired != 7 || p.Players[0].BestKillStreak != 3 {
			t.Fatalf("acc merge missing: %+v", p.Players[0])
		}
	})
}
