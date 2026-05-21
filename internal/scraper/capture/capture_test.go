package capture

import "testing"

// TestResolveEmptyReturnsDefault: with no rows in the policy list, every
// (instance, class) lookup falls through to DefaultPolicy — auto mode,
// default cadence, no sink. The runner therefore behaves exactly like
// the WS-subscriber-only gate when capture_policies is empty.
func TestResolveEmptyReturnsDefault(t *testing.T) {
	got := Resolve(nil, "debug-host", "tick")
	want := DefaultPolicy()
	if got != want {
		t.Fatalf("Resolve(nil, ...) = %+v, want %+v", got, want)
	}
}

// TestResolvePrecedenceOrder: the design pins resolution to
// (instance, class) → (*, class) → (instance, *) → (*, *). This test
// builds a row at each tier and checks that removing the higher-priority
// rows promotes the next tier — locking the ordering against accidental
// reshuffles in the resolver.
func TestResolvePrecedenceOrder(t *testing.T) {
	exact := Policy{Instance: "pod-1", Class: "tick", Mode: ModeAlways, Cadence: CadenceEngine, Description: "exact"}
	byClass := Policy{Instance: "*", Class: "tick", Mode: ModeAlways, Cadence: Cadence30Hz, Description: "byClass"}
	byInstance := Policy{Instance: "pod-1", Class: "*", Mode: ModeNever, Cadence: CadenceDefault, Description: "byInstance"}
	global := Policy{Instance: "*", Class: "*", Mode: ModeAuto, Cadence: Cadence1s, Description: "global"}

	all := []Policy{global, byInstance, byClass, exact}
	if got := Resolve(all, "pod-1", "tick"); got.Description != "exact" {
		t.Fatalf("with all rows, want exact, got %+v", got)
	}
	if got := Resolve([]Policy{global, byInstance, byClass}, "pod-1", "tick"); got.Description != "byClass" {
		t.Fatalf("without exact, want byClass, got %+v", got)
	}
	if got := Resolve([]Policy{global, byInstance}, "pod-1", "tick"); got.Description != "byInstance" {
		t.Fatalf("without byClass, want byInstance, got %+v", got)
	}
	if got := Resolve([]Policy{global}, "pod-1", "tick"); got.Description != "global" {
		t.Fatalf("with only global, want global, got %+v", got)
	}
}

// TestResolveOrderIndependent: the resolver must not depend on input
// slice order — PB returns rows however the index iteration goes, which
// is not stable across edits. Three shuffles, identical answer.
func TestResolveOrderIndependent(t *testing.T) {
	exact := Policy{Instance: "pod-1", Class: "tick", Mode: ModeAlways, Description: "exact"}
	byClass := Policy{Instance: "*", Class: "tick", Mode: ModeAlways, Description: "byClass"}
	global := Policy{Instance: "*", Class: "*", Mode: ModeAuto, Description: "global"}

	orders := [][]Policy{
		{exact, byClass, global},
		{global, byClass, exact},
		{byClass, exact, global},
	}
	for i, in := range orders {
		if got := Resolve(in, "pod-1", "tick"); got.Description != "exact" {
			t.Fatalf("order %d: want exact, got %+v", i, got)
		}
	}
}

// TestResolveMissesUnrelatedRows: a row for a different (instance, class)
// must not bleed into an unrelated lookup. Guards against a refactor
// where the switch arms accidentally drop one of the equality checks.
func TestResolveMissesUnrelatedRows(t *testing.T) {
	rows := []Policy{
		{Instance: "pod-2", Class: "tick", Mode: ModeAlways},
		{Instance: "pod-1", Class: "debug", Mode: ModeNever},
	}
	got := Resolve(rows, "pod-1", "tick")
	want := DefaultPolicy()
	if got != want {
		t.Fatalf("unrelated rows leaked: got %+v want %+v", got, want)
	}
}

// TestPolicyDemandsAndHardCap: the mode helpers feed the runner's
// aggregation step. always → demands; never → hard cap; auto → neither.
func TestPolicyDemandsAndHardCap(t *testing.T) {
	cases := []struct {
		mode        Mode
		wantDemand  bool
		wantHardCap bool
	}{
		{ModeAuto, false, false},
		{ModeAlways, true, false},
		{ModeNever, false, true},
	}
	for _, tc := range cases {
		p := Policy{Mode: tc.mode}
		if p.Demands() != tc.wantDemand {
			t.Fatalf("mode=%s: Demands() = %v, want %v", tc.mode, p.Demands(), tc.wantDemand)
		}
		if p.IsHardCap() != tc.wantHardCap {
			t.Fatalf("mode=%s: IsHardCap() = %v, want %v", tc.mode, p.IsHardCap(), tc.wantHardCap)
		}
	}
}
