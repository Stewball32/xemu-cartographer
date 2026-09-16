package leaguescraper

import (
	"log"
	"sync"

	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xc-scraper/capture"
)

// Pusher is the receiving half of the capture-policy provider: whoever
// consumes the PB capture_policies rows. *runner.Manager satisfies it in
// embedded mode (SetCapturePolicies stores + pushes runner-by-runner); in
// wire mode (DESIGN-STEP8 §10) the *EventWriter keeps the pb: rows
// league-side while the *ConfigPusher ships the rest to the daemon.
type Pusher interface {
	SetCapturePolicies(policies []capture.Policy)
}

// CapturePoliciesCollection is the PocketBase collection name holding the
// per-(instance, class) capture rows (see the collections snapshot
// migration). The loader and the record-change hooks both reference this
// constant.
const CapturePoliciesCollection = "capture_policies"

// reloadMu serialises full reloads — only one ReloadCapturePolicies pass
// runs at a time so a slow FindAllRecords can't race with a fast one and
// leave runners on stale slices (the manager's SetCapturePolicies pushes
// runner-by-runner). One server → one manager, so a package-level lock is
// the same scope as a per-manager one.
var reloadMu sync.Mutex

// recordsToPolicies converts capture_policies PB records into the pure
// capture.Policy slice the resolver consumes. Pulled out as a free
// function so the field-mapping is testable without a Manager + PB
// harness (the function side-effects nothing).
func recordsToPolicies(records []*core.Record) []capture.Policy {
	out := make([]capture.Policy, 0, len(records))
	for _, r := range records {
		out = append(out, capture.Policy{
			Instance:    r.GetString("instance"),
			Class:       r.GetString("class"),
			Mode:        capture.Mode(r.GetString("mode")),
			Cadence:     capture.Cadence(r.GetString("cadence")),
			Sink:        r.GetString("sink"),
			Description: r.GetString("description"),
		})
	}
	return out
}

// ReloadCapturePolicies fetches every capture_policies row from PB and
// hands the resolved slice to the Pusher (runner.SetCapturePolicies —
// stored on the Manager and pushed to every running runner). Safe to call
// from PB record-change hooks; serialised by reloadMu so concurrent fires
// can't interleave a runner-by-runner push and leave half the fleet on
// stale policies.
//
// No-op (returns nil) on a nil app or pusher (the test path). A
// FindAllRecords error is logged and treated as "empty policies" rather
// than failing the hook — a missing collection on a fresh DB shouldn't
// take down the runner.
func ReloadCapturePolicies(app core.App, mgr Pusher) error {
	if app == nil || mgr == nil {
		return nil
	}
	reloadMu.Lock()
	defer reloadMu.Unlock()

	records, err := app.FindAllRecords(CapturePoliciesCollection)
	if err != nil {
		log.Printf("scraper: load capture_policies: %v", err)
		mgr.SetCapturePolicies(nil)
		return nil
	}
	mgr.SetCapturePolicies(recordsToPolicies(records))
	return nil
}

// RegisterCapturePolicyHooks wires PB record-change hooks so any
// create/update/delete on capture_policies triggers a Pusher-wide
// reload. Idempotent at the binding level — calling twice would bind
// twice, so main.go must call this exactly once during OnServe.
//
// Hot-reload pattern mirrors seed.RegisterContainerSnapshotHooks.
func RegisterCapturePolicyHooks(app core.App, mgr Pusher) {
	if app == nil || mgr == nil {
		return
	}
	reload := func(e *core.RecordEvent) error {
		if err := ReloadCapturePolicies(app, mgr); err != nil {
			log.Printf("scraper: reload capture_policies: %v", err)
		}
		return e.Next()
	}
	app.OnRecordAfterCreateSuccess(CapturePoliciesCollection).BindFunc(reload)
	app.OnRecordAfterUpdateSuccess(CapturePoliciesCollection).BindFunc(reload)
	app.OnRecordAfterDeleteSuccess(CapturePoliciesCollection).BindFunc(reload)
}
