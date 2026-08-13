package hostrunner

// StartMode is the arm-only vs arm+start toggle (requirement 6). ArmOnly leaves
// the host sitting armed in the lobby (a human/admin presses start); ArmAndStart
// lets the runner press start once the native predicates pass.
type StartMode int

const (
	ArmOnly StartMode = iota
	ArmAndStart
)

func (m StartMode) String() string {
	if m == ArmAndStart {
		return "arm+start"
	}
	return "arm-only"
}

// StartPredicate is one READ-ONLY native precondition. OK reads live state and
// must return true for the runner to consider pressing start. Predicates NEVER
// mutate or fake state (requirement 2) — they only observe.
type StartPredicate struct {
	Name string
	OK   func(Observation) bool
}

// NativeReadyPredicate reads Halo's native countdown preconditions: 2+ connected
// boxes and 2+ teams. This is the v1 zero/one check.
func NativeReadyPredicate() StartPredicate {
	return StartPredicate{
		Name: "native-ready (2+ boxes, 2+ teams)",
		OK:   func(o Observation) bool { return o.ReadyToStart() },
	}
}

// ReadyGatePredicate is the optional ready-gate (default OFF). v1 has no reliable
// per-player "ready" read, so the supplied readyFn is the seam; wire it to a
// lobby-ready count when the reader exposes one. Disabled policies omit it.
func ReadyGatePredicate(readyFn func(Observation) bool) StartPredicate {
	return StartPredicate{Name: "ready-gate", OK: readyFn}
}

// StartPolicy is the ordered predicate list + start mode (requirement 6). v1 is
// zero or one predicate (NativeReady, plus the optional ready-gate).
type StartPolicy struct {
	Predicates []StartPredicate
	Mode       StartMode
}

// DefaultStartPolicy: arm-only with the native-ready predicate. Arm-only is the
// safe default — the runner drives selection and leaves start to the operator
// unless explicitly switched to arm+start.
func DefaultStartPolicy() StartPolicy {
	return StartPolicy{
		Predicates: []StartPredicate{NativeReadyPredicate()},
		Mode:       ArmOnly,
	}
}

// Evaluate runs the predicates in order. Returns (true, "") when all pass, or
// (false, firstFailingName) at the first failure — the failing name surfaces on
// the observable stream so operators see WHY start is held.
func (p StartPolicy) Evaluate(obs Observation) (bool, string) {
	for _, pred := range p.Predicates {
		if pred.OK == nil || !pred.OK(obs) {
			return false, pred.Name
		}
	}
	return true, ""
}
