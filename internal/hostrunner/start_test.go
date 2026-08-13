package hostrunner

import "testing"

func TestStartPolicyEvaluate(t *testing.T) {
	p := DefaultStartPolicy() // native-ready only, arm-only
	if p.Mode != ArmOnly {
		t.Fatal("default should be arm-only")
	}
	if ok, why := p.Evaluate(Observation{MachineCount: 1, TeamCount: 2}); ok || why == "" {
		t.Fatalf("1 box should fail native-ready, got ok=%v why=%q", ok, why)
	}
	if ok, _ := p.Evaluate(Observation{MachineCount: 2, TeamCount: 2}); !ok {
		t.Fatal("2 boxes + 2 teams should pass")
	}
}

func TestStartPolicyOrderedFailure(t *testing.T) {
	// native-ready passes, ready-gate fails → reports the ready-gate name.
	p := StartPolicy{
		Predicates: []StartPredicate{
			NativeReadyPredicate(),
			ReadyGatePredicate(func(Observation) bool { return false }),
		},
		Mode: ArmAndStart,
	}
	ok, why := p.Evaluate(Observation{MachineCount: 2, TeamCount: 2})
	if ok {
		t.Fatal("ready-gate should hold start")
	}
	if why != "ready-gate" {
		t.Fatalf("expected ready-gate to be the failing predicate, got %q", why)
	}
}

func TestStartModeString(t *testing.T) {
	if ArmOnly.String() != "arm-only" || ArmAndStart.String() != "arm+start" {
		t.Fatal("StartMode strings wrong")
	}
}
