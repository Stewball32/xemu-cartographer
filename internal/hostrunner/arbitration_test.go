package hostrunner

import "testing"

func TestArbiterTransitions(t *testing.T) {
	a := NewArbiter()
	if a.Authority() != AuthRunner || !a.CanEmit() {
		t.Fatal("new arbiter should be runner-driven and able to emit")
	}

	a.TakeOver()
	if a.Authority() != AuthAdmin || a.CanEmit() {
		t.Fatal("after TakeOver: admin, cannot emit")
	}

	a.Release()
	if a.Authority() != AuthRunner || !a.CanEmit() {
		t.Fatal("after Release: runner, can emit")
	}

	a.Disable()
	if a.Authority() != AuthDisabled || a.CanEmit() {
		t.Fatal("after Disable: disabled, cannot emit")
	}

	// TakeOver is a no-op while disabled (stays disabled).
	a.TakeOver()
	if a.Authority() != AuthDisabled {
		t.Fatal("TakeOver must not un-disable")
	}
	// Release is a no-op from disabled.
	a.Release()
	if a.Authority() != AuthDisabled {
		t.Fatal("Release must not un-disable")
	}

	a.Enable()
	if a.Authority() != AuthRunner || !a.CanEmit() {
		t.Fatal("after Enable: runner, can emit")
	}
}

func TestArbiterSet(t *testing.T) {
	a := NewArbiter()
	a.Set(AuthDisabled)
	if a.Authority() != AuthDisabled {
		t.Fatal("Set(disabled) failed")
	}
	a.Set(AuthRunner)
	if !a.CanEmit() {
		t.Fatal("Set(runner) failed")
	}
}
