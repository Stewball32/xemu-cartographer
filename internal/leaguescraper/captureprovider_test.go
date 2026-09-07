package leaguescraper

import (
	"reflect"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/xemu-cartographer/xc-scraper/capture"
	"github.com/xemu-cartographer/xc-scraper/runner"
)

func newPolicyApp(t *testing.T) core.App {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	return app
}

func ensurePolicyCollection(t *testing.T, app core.App) *core.Collection {
	t.Helper()
	c := core.NewBaseCollection(CapturePoliciesCollection)
	c.Fields.Add(
		&core.TextField{Name: "instance"},
		&core.TextField{Name: "class"},
		&core.TextField{Name: "mode"},
		&core.TextField{Name: "cadence"},
		&core.TextField{Name: "sink"},
		&core.TextField{Name: "description"},
	)
	if err := app.Save(c); err != nil {
		t.Fatalf("save %s collection: %v", CapturePoliciesCollection, err)
	}
	return c
}

func savePolicy(t *testing.T, app core.App, col *core.Collection, p capture.Policy) *core.Record {
	t.Helper()
	r := core.NewRecord(col)
	r.Set("instance", p.Instance)
	r.Set("class", p.Class)
	r.Set("mode", string(p.Mode))
	r.Set("cadence", string(p.Cadence))
	r.Set("sink", p.Sink)
	r.Set("description", p.Description)
	if err := app.Save(r); err != nil {
		t.Fatalf("save policy: %v", err)
	}
	return r
}

// TestReloadCapturePolicies_LoadsRows: every capture_policies row lands on
// the manager as a capture.Policy with all six columns mapped.
func TestReloadCapturePolicies_LoadsRows(t *testing.T) {
	app := newPolicyApp(t)
	col := ensurePolicyCollection(t, app)
	mgr := runner.New(runner.Options{})
	defer mgr.Close()

	want := []capture.Policy{
		{Instance: "alpha", Class: "event", Mode: capture.Mode("always"), Cadence: capture.Cadence("tick"), Sink: "pb:game_events", Description: "all deaths"},
		{Instance: "*", Class: "tick", Mode: capture.Mode("off"), Cadence: capture.Cadence("never"), Sink: "", Description: ""},
	}
	for _, p := range want {
		savePolicy(t, app, col, p)
	}

	if err := ReloadCapturePolicies(app, mgr); err != nil {
		t.Fatalf("ReloadCapturePolicies: %v", err)
	}
	got := mgr.CapturePolicies()
	if len(got) != len(want) {
		t.Fatalf("policies = %+v, want %+v", got, want)
	}
	// FindAllRecords order is not contractual — compare as sets.
	for _, w := range want {
		found := false
		for _, g := range got {
			if reflect.DeepEqual(g, w) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("policy %+v missing from %+v", w, got)
		}
	}
}

// TestReloadCapturePolicies_MissingCollection: a fresh DB without the
// collection is "no policies", not a hook failure.
func TestReloadCapturePolicies_MissingCollection(t *testing.T) {
	app := newPolicyApp(t)
	mgr := runner.New(runner.Options{})
	defer mgr.Close()
	mgr.SetCapturePolicies([]capture.Policy{{Instance: "stale"}})

	if err := ReloadCapturePolicies(app, mgr); err != nil {
		t.Fatalf("ReloadCapturePolicies on a missing collection: %v", err)
	}
	if got := mgr.CapturePolicies(); len(got) != 0 {
		t.Fatalf("stale policies survived a failed load: %+v", got)
	}
}

// TestReloadCapturePolicies_NilSafe: nil app / manager are no-ops.
func TestReloadCapturePolicies_NilSafe(t *testing.T) {
	mgr := runner.New(runner.Options{})
	defer mgr.Close()
	if err := ReloadCapturePolicies(nil, mgr); err != nil {
		t.Fatalf("nil app: %v", err)
	}
	if err := ReloadCapturePolicies(newPolicyApp(t), nil); err != nil {
		t.Fatalf("nil manager: %v", err)
	}
	RegisterCapturePolicyHooks(nil, mgr) // must not panic
}

// TestRegisterCapturePolicyHooks_HotReload: create / update / delete on the
// collection re-syncs the manager without an explicit reload call.
func TestRegisterCapturePolicyHooks_HotReload(t *testing.T) {
	app := newPolicyApp(t)
	col := ensurePolicyCollection(t, app)
	mgr := runner.New(runner.Options{})
	defer mgr.Close()
	RegisterCapturePolicyHooks(app, mgr)

	rec := savePolicy(t, app, col, capture.Policy{Instance: "alpha", Class: "event", Mode: "always"})
	if got := mgr.CapturePolicies(); len(got) != 1 || got[0].Instance != "alpha" {
		t.Fatalf("after create: %+v", got)
	}

	rec.Set("instance", "beta")
	if err := app.Save(rec); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := mgr.CapturePolicies(); len(got) != 1 || got[0].Instance != "beta" {
		t.Fatalf("after update: %+v", got)
	}

	if err := app.Delete(rec); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := mgr.CapturePolicies(); len(got) != 0 {
		t.Fatalf("after delete: %+v", got)
	}
}
