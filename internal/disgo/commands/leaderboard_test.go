package commands

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/rating"
)

func ensureRatings(t *testing.T, app core.App) {
	t.Helper()
	if _, err := app.FindCollectionByNameOrId("ratings"); err == nil {
		return
	}
	c := core.NewBaseCollection("ratings")
	c.Fields.Add(
		&core.TextField{Name: "gamertag", Required: true},
		&core.TextField{Name: "gametype", Required: true},
		&core.NumberField{Name: "rating"},
		&core.NumberField{Name: "games", OnlyInt: true},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	if err := app.Save(c); err != nil {
		t.Fatalf("save ratings: %v", err)
	}
}

func TestLeaderboardEntriesAndRank(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureRatings(t, app)

	add := func(gamertag, gametype string, r float64, games int) {
		c, _ := app.FindCollectionByNameOrId("ratings")
		rec := core.NewRecord(c)
		rec.Set("gamertag", gamertag)
		rec.Set("gametype", gametype)
		rec.Set("rating", r)
		rec.Set("games", games)
		if err := app.Save(rec); err != nil {
			t.Fatalf("save rating: %v", err)
		}
	}
	add("Stew", "slayer", 1600, 10)
	add("Fox", "slayer", 1500, 8)
	add("Red", "slayer", 1700, 5)
	add("Stew", "ctf", 1400, 3) // different gametype — excluded from slayer board

	entries := leaderboardEntries(app, "slayer")
	if len(entries) != 3 {
		t.Fatalf("slayer entries = %d, want 3 (ctf excluded)", len(entries))
	}

	ranked := rating.TopN(entries, 10)
	if ranked[0].Gamertag != "Red" || ranked[2].Gamertag != "Fox" {
		t.Errorf("ranked order = %v, want Red ... Fox", []string{ranked[0].Gamertag, ranked[1].Gamertag, ranked[2].Gamertag})
	}

	if r := rating.RankOf(entries, "Stew"); r != 2 {
		t.Errorf("RankOf(Stew) = %d, want 2", r)
	}
	if r := rating.RankOf(entries, "ghost"); r != 0 {
		t.Errorf("RankOf(unknown) = %d, want 0", r)
	}
}
