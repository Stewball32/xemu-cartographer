package manager

import (
	"log"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/capture"
)

// CapturePoliciesCollection is the PocketBase collection name holding the
// per-(instance, class) capture rows (see internal/pocketbase/schema/
// capture_policies.go). The loader and the record-change hooks both
// reference this constant.
const CapturePoliciesCollection = "capture_policies"

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

// ReloadCapturePolicies fetches every capture_policies row from PB,
// stores the resolved slice on the Manager, and pushes it to every
// running runner. Safe to call from PB record-change hooks; serialised
// by the Manager's reloadMu so concurrent fires can't interleave a
// runner-by-runner push and leave half the fleet on stale policies.
//
// No-op (returns nil) if the Manager wasn't built with a *guards.Services
// holding a non-nil App (the test path). A FindAllRecords error is
// logged and treated as "empty policies" rather than failing the hook —
// a missing collection on a fresh DB shouldn't take down the runner.
func (m *Manager) ReloadCapturePolicies() error {
	if m.svc == nil || m.svc.App == nil {
		return nil
	}
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	records, err := m.svc.App.FindAllRecords(CapturePoliciesCollection)
	if err != nil {
		log.Printf("scraper: load capture_policies: %v", err)
		m.setCapturePolicies(nil)
		return nil
	}
	m.setCapturePolicies(recordsToPolicies(records))
	return nil
}

// setCapturePolicies stores the slice on the Manager and pushes it to
// every currently-registered runner. Runners started later (via Start)
// pick up the snapshot via getCapturePolicies + their own setPolicies
// call inside Manager.Start.
//
// Two-phase to avoid holding m.mu (runners) and m.policyMu together —
// any deadlock between Start, Stop, and a hook-driven reload becomes
// impossible because no path nests the locks.
func (m *Manager) setCapturePolicies(p []capture.Policy) {
	m.policyMu.Lock()
	m.policies = p
	m.policyMu.Unlock()

	m.mu.Lock()
	runners := make([]*runner, 0, len(m.runners))
	for _, r := range m.runners {
		runners = append(runners, r)
	}
	m.mu.Unlock()

	for _, r := range runners {
		r.setPolicies(p)
	}
}

// getCapturePolicies returns the Manager's last-loaded policy slice.
// Used by Start so freshly-spawned runners inherit the current policy
// state without having to wait for the next hook-driven reload.
func (m *Manager) getCapturePolicies() []capture.Policy {
	m.policyMu.RLock()
	defer m.policyMu.RUnlock()
	return m.policies
}

// RegisterCapturePolicyHooks wires PB record-change hooks so any
// create/update/delete on capture_policies triggers a Manager-wide
// reload. Idempotent at the binding level — calling twice would bind
// twice, so main.go must call this exactly once during OnServe.
//
// Hot-reload pattern mirrors seed.RegisterContainerSnapshotHooks.
func (m *Manager) RegisterCapturePolicyHooks() {
	if m.svc == nil || m.svc.App == nil {
		return
	}
	reload := func(e *core.RecordEvent) error {
		if err := m.ReloadCapturePolicies(); err != nil {
			log.Printf("scraper: reload capture_policies: %v", err)
		}
		return e.Next()
	}
	app := m.svc.App
	app.OnRecordAfterCreateSuccess(CapturePoliciesCollection).BindFunc(reload)
	app.OnRecordAfterUpdateSuccess(CapturePoliciesCollection).BindFunc(reload)
	app.OnRecordAfterDeleteSuccess(CapturePoliciesCollection).BindFunc(reload)
}
