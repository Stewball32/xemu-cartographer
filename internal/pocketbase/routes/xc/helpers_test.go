package xc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// fixturePath is the vendored xc.finished_game/1 golden (byte-identical to
// ../xc-scraper/wire/testdata/finished_game.json — task sync-wire:check).
const fixturePath = "../../../../sveltekit/src/lib/types/wire-fixtures/finished_game.json"

// fixture returns the finished_game golden as a mutable map.
func fixture(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(fixturePath))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return m
}

// newMux builds the real PocketBase router (default middlewares included)
// with the xc group mounted, so the tests exercise the bind chain.
func newMux(t *testing.T, app core.App) http.Handler {
	t.Helper()
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	RegisterAll(&core.ServeEvent{App: app, Router: r})
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux: %v", err)
	}
	return mux
}

// post sends body to POST /api/xc/finished_game with hdr and returns the
// status + decoded JSON body (nil when empty / not JSON).
func post(t *testing.T, mux http.Handler, body []byte, hdr map[string]string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/xc/finished_game", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// ensureGameCollections creates the M13 chain's collections (series, games,
// game_players, game_events, ratings) on a bare test app — the same shapes
// internal/games' tests use.
func ensureGameCollections(t *testing.T, app core.App) {
	t.Helper()
	create := func(name string, fields ...core.Field) *core.Collection {
		if c, err := app.FindCollectionByNameOrId(name); err == nil {
			return c
		}
		c := core.NewBaseCollection(name)
		c.Fields.Add(fields...)
		c.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
		if name == "games" {
			c.AddIndex("idx_games_game_uid_unique", true, "game_uid", "game_uid != ''")
		}
		if err := app.Save(c); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
		return c
	}
	series := create("series",
		&core.TextField{Name: "name"},
		&core.SelectField{Name: "format", MaxSelect: 1, Values: []string{"single", "exact-n", "best-of-n", "first-to-x"}},
		&core.NumberField{Name: "target_n", OnlyInt: true},
		&core.SelectField{Name: "category", MaxSelect: 1, Values: []string{"casual", "competitive", "tournament", "custom"}},
		&core.DateField{Name: "started_at"},
		&core.DateField{Name: "ended_at"},
	)
	games := create("games",
		&core.RelationField{Name: "series", Required: true, CollectionId: series.Id, MaxSelect: 1},
		&core.TextField{Name: "container"},
		&core.TextField{Name: "host_machine_name"},
		&core.TextField{Name: "map"},
		&core.TextField{Name: "gametype"},
		&core.TextField{Name: "variant_name"},
		&core.DateField{Name: "started_at"},
		&core.DateField{Name: "ended_at"},
		&core.NumberField{Name: "winner_team", OnlyInt: true},
		&core.TextField{Name: "score_summary"},
		&core.TextField{Name: "game_uid"},
		&core.TextField{Name: "end_reason"},
	)
	create("game_players",
		&core.RelationField{Name: "game", Required: true, CollectionId: games.Id, MaxSelect: 1},
		&core.TextField{Name: "gamertag", Required: true},
		&core.NumberField{Name: "team", OnlyInt: true},
		&core.NumberField{Name: "kills", OnlyInt: true},
		&core.NumberField{Name: "deaths", OnlyInt: true},
		&core.NumberField{Name: "assists", OnlyInt: true},
		&core.NumberField{Name: "score", OnlyInt: true},
		&core.NumberField{Name: "time_alive_ms", OnlyInt: true},
	)
	create("game_events",
		&core.TextField{Name: "instance", Required: true},
		&core.TextField{Name: "type", Required: true},
		&core.NumberField{Name: "seq", OnlyInt: true},
		&core.NumberField{Name: "tick", OnlyInt: true},
		&core.DateField{Name: "ts"},
		&core.JSONField{Name: "data", MaxSize: 1 << 20},
		&core.RelationField{Name: "game", CollectionId: games.Id, MaxSelect: 1},
	)
	ratings := create("ratings",
		&core.TextField{Name: "gamertag", Required: true},
		&core.TextField{Name: "gametype", Required: true},
		&core.NumberField{Name: "rating"},
		&core.NumberField{Name: "games", OnlyInt: true},
	)
	if len(ratings.Indexes) == 0 {
		ratings.AddIndex("idx_ratings_gamertag_gametype_unique", true, "gamertag, gametype", "")
		if err := app.Save(ratings); err != nil {
			t.Fatalf("save ratings index: %v", err)
		}
	}
}

// insertEvent writes an unstamped game_events row for instance at ts.
func insertEvent(t *testing.T, app core.App, instance string, ts time.Time) *core.Record {
	t.Helper()
	col, err := app.FindCollectionByNameOrId("game_events")
	if err != nil {
		t.Fatalf("lookup game_events: %v", err)
	}
	r := core.NewRecord(col)
	r.Set("instance", instance)
	r.Set("type", "kill")
	dt, err := types.ParseDateTime(ts)
	if err != nil {
		t.Fatalf("parse ts: %v", err)
	}
	r.Set("ts", dt)
	if err := app.Save(r); err != nil {
		t.Fatalf("save event: %v", err)
	}
	return r
}

// eventGame reloads a game_events row and returns its game relation.
func eventGame(t *testing.T, app core.App, id string) string {
	t.Helper()
	r, err := app.FindRecordById("game_events", id)
	if err != nil {
		t.Fatalf("reload event %s: %v", id, err)
	}
	return r.GetString("game")
}
