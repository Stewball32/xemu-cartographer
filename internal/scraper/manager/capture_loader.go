package manager

import (
	"github.com/xemu-cartographer/xc-scraper/capture"
)

// SetCapturePolicies stores the slice on the Manager and pushes it to
// every currently-registered runner. Runners started later (via Start)
// pick up the snapshot via CapturePolicies + their own setPolicies call
// inside Manager.Start.
//
// This is the manager's whole capture-policy surface: WHERE the policies
// come from (the league's capture_policies collection + its record hooks)
// lives in internal/leaguescraper (ReloadCapturePolicies /
// RegisterCapturePolicyHooks), which serialises its own reloads so two
// concurrent loads can't interleave a runner-by-runner push and leave half
// the fleet on stale policies.
//
// Two-phase to avoid holding m.mu (runners) and m.policyMu together —
// any deadlock between Start, Stop, and a hook-driven reload becomes
// impossible because no path nests the locks.
func (m *Manager) SetCapturePolicies(p []capture.Policy) {
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

// CapturePolicies returns the Manager's last-set policy slice (the slice
// itself, not a copy — treat it as read-only). Used by Start so
// freshly-spawned runners inherit the current policy state without having
// to wait for the next provider reload.
func (m *Manager) CapturePolicies() []capture.Policy {
	m.policyMu.RLock()
	defer m.policyMu.RUnlock()
	return m.policies
}
