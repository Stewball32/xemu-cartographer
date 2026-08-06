package manager

import "testing"

// The ready loop re-translates its low mappings after readyReadRefreshAt
// consecutive ReadGameState failures (self-healing a stale boot-time mapping),
// then re-arms. This pins that trigger logic apart from the loop goroutine.
func TestConsecutiveFailureGate(t *testing.T) {
	g := consecutiveFailureGate{at: 3}

	// Below the threshold: never fires.
	if g.fail() || g.fail() {
		t.Fatal("gate fired before reaching the threshold")
	}
	// The at-th consecutive failure fires exactly once...
	if !g.fail() {
		t.Fatal("gate did not fire on the 3rd consecutive failure")
	}
	// ...and re-arms, so the next run of failures fires again at the threshold.
	if g.fail() || g.fail() {
		t.Fatal("gate fired again before re-reaching the threshold after firing")
	}
	if !g.fail() {
		t.Fatal("gate did not re-arm and fire on the next threshold run")
	}
}

// A success resets the count, so a fresh run of failures must reach the full
// threshold again — a one-off failure between good reads never triggers a
// refresh.
func TestConsecutiveFailureGateResetOnOK(t *testing.T) {
	g := consecutiveFailureGate{at: 3}
	g.fail()
	g.fail() // 2 in a row
	g.ok()   // a good read clears it
	if g.fail() || g.fail() {
		t.Fatal("gate fired without a full fresh run of failures after ok()")
	}
	if !g.fail() {
		t.Fatal("gate did not fire after a full fresh run of 3 failures")
	}
}

// The zero value (at=0) must never fire — a guard against an unconfigured gate
// refreshing on every single failure.
func TestConsecutiveFailureGateZeroValueNeverFires(t *testing.T) {
	var g consecutiveFailureGate
	for i := 0; i < 100; i++ {
		if g.fail() {
			t.Fatal("zero-value gate fired")
		}
	}
}
