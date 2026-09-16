package lansync

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// isosTestCollection builds a minimal `isos` collection carrying the columns
// resolveGames reads (no PB rules).
func isosTestCollection(t *testing.T, app core.App) *core.Collection {
	t.Helper()
	isos := core.NewBaseCollection("isos")
	isos.Fields.Add(
		&core.TextField{Name: "name"},
		&core.TextField{Name: "title_id"},
		&core.TextField{Name: "dest_name"},
		&core.NumberField{Name: "footprint_bytes", OnlyInt: true},
		&core.SelectField{Name: "role", Values: []string{"play", "server", "shelved"}, MaxSelect: 1},
		&core.BoolField{Name: "allow_on_xbox"},
		&core.BoolField{Name: "drift_detected"},
		&core.BoolField{Name: "extracted_ready"},
		&core.TextField{Name: "extracted_path"},
	)
	if err := app.Save(isos); err != nil {
		t.Fatalf("save isos: %v", err)
	}
	return isos
}

// TestGameServable pins the shared PD-9 predicate: drifted is out for
// everyone; a station only gets play + allow_on_xbox; a peer gets the rest.
func TestGameServable(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	isos := isosTestCollection(t, app)

	iso := func(role string, allowOnXbox, drift bool) *core.Record {
		r := core.NewRecord(isos)
		r.Set("role", role)
		r.Set("allow_on_xbox", allowOnXbox)
		r.Set("drift_detected", drift)
		return r
	}
	cases := []struct {
		name        string
		rec         *core.Record
		peer, stati bool
	}{
		{"play + allow_on_xbox", iso("play", true, false), true, true},
		{"play, not allowed on xbox", iso("play", false, false), true, false},
		{"server build", iso("server", true, false), true, false},
		{"shelved", iso("shelved", true, false), true, false},
		{"drifted", iso("play", true, true), false, false},
	}
	for _, c := range cases {
		if got := gameServable(c.rec, false); got != c.peer {
			t.Errorf("%s: peer servable = %v, want %v", c.name, got, c.peer)
		}
		if got := gameServable(c.rec, true); got != c.stati {
			t.Errorf("%s: station servable = %v, want %v", c.name, got, c.stati)
		}
	}
}

// TestResolveGamesStationFilter proves the PD-9 split: a station (machine key
// bound to a station_id) only sees play-role discs flagged allow_on_xbox,
// while a peer keeps the whole preset. A drifted disc is hidden from both.
func TestResolveGamesStationFilter(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	isos := isosTestCollection(t, app)

	newISO := func(name, role string, allowOnXbox, drift bool) string {
		r := core.NewRecord(isos)
		r.Set("name", name)
		r.Set("role", role)
		r.Set("allow_on_xbox", allowOnXbox)
		r.Set("drift_detected", drift)
		if err := app.Save(r); err != nil {
			t.Fatalf("save iso %q: %v", name, err)
		}
		return r.Id
	}

	cleared := newISO("Cleared", "play", true, false)
	playNoXbox := newISO("PlayNoXbox", "play", false, false)
	serverBuild := newISO("ServerBuild", "server", true, false)
	shelved := newISO("Shelved", "shelved", true, false)
	drifted := newISO("Drifted", "play", true, true)
	ids := []string{cleared, playNoXbox, serverBuild, shelved, drifted, "dangling-id"}

	names := func(items []syncItem) []string {
		out := make([]string, 0, len(items))
		for _, it := range items {
			out = append(out, it.Name)
		}
		return out
	}
	equal := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	// Peer: everything but the drifted disc (and the dangling ref), sorted by
	// name within equal priority.
	peer := resolveGames(app, ids, nil, "skip", false)
	if got, want := names(peer), []string{"Cleared", "PlayNoXbox", "ServerBuild", "Shelved"}; !equal(got, want) {
		t.Errorf("peer games = %v, want %v", got, want)
	}

	// Station: only play + allow_on_xbox, never drifted.
	station := resolveGames(app, ids, nil, "skip", true)
	if got, want := names(station), []string{"Cleared"}; !equal(got, want) {
		t.Errorf("station games = %v, want %v", got, want)
	}
	if len(station) == 1 && station[0].ID != cleared {
		t.Errorf("station game id = %q, want %q", station[0].ID, cleared)
	}
}
