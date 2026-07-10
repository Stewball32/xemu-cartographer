package vpad

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func TestNameCRC16(t *testing.T) {
	// The value padpool.py hard-codes for the default name (LE bytes 81b8).
	if got := NameCRC16(deviceName); got != 0xb881 {
		t.Errorf("NameCRC16(%q) = 0x%04x, want 0xb881", deviceName, got)
	}
}

func TestPredictGUID(t *testing.T) {
	// Must reproduce the GUIDs ce-nav's TOML binds (base 0x0600 → port1 0x0601,
	// port2 0x0602) — proof the derived GUID matches what xemu actually computes.
	cases := map[int]string{
		0x0601: "030081b85e0400008e02000001060000",
		0x0602: "030081b85e0400008e02000002060000",
		0x0701: "030081b85e0400008e02000001070000",
	}
	for version, want := range cases {
		if got := PredictGUID(deviceName, version); got != want {
			t.Errorf("PredictGUID(version=0x%04x) = %s, want %s", version, got, want)
		}
		if len(want) != 32 {
			t.Errorf("GUID %s is not 32 hex chars", want)
		}
	}
}

// TestSupportedLabelsSubsetOfSendKey checks the pad speaks the sendkey
// vocabulary (minus the keyboard-only Control_L / r reset chord).
func TestSupportedLabelsSubsetOfSendKey(t *testing.T) {
	want := []string{
		"a", "b", "x", "y",
		"Up", "Down", "Left", "Right",
		"Return", "BackSpace", "5",
		"1", "2", "3", "4",
		"w", "o",
		"e", "s", "d", "f",
		"i", "j", "k", "l",
	}
	have := make(map[string]bool)
	for _, l := range SupportedLabels() {
		have[l] = true
	}
	for _, l := range want {
		if !have[l] {
			t.Errorf("SupportedLabels missing %q", l)
		}
	}
	if len(SupportedLabels()) != len(want) {
		t.Errorf("SupportedLabels has %d entries, want %d", len(SupportedLabels()), len(want))
	}
	// Keyboard-only labels must NOT be pad controls.
	for _, l := range []string{"Control_L", "r"} {
		if have[l] {
			t.Errorf("SupportedLabels unexpectedly includes keyboard-only %q", l)
		}
	}
}

// TestWireStructSizes guards the uinput/evdev struct layouts against drift.
func TestWireStructSizes(t *testing.T) {
	if got := unsafe.Sizeof(uinputUserDev{}); got != 1116 {
		t.Errorf("sizeof(uinputUserDev) = %d, want 1116", got)
	}
	if got := unsafe.Sizeof(inputEvent{}); got != 24 {
		t.Errorf("sizeof(inputEvent) = %d, want 24", got)
	}
}

// TestDeviceCreateSmoke actually creates a pad through uinput when the harness
// allows it (needs write access to /dev/uinput); otherwise it skips so CI —
// which has no uinput — stays green.
func TestDeviceCreateSmoke(t *testing.T) {
	if unix.Access(uinputPath, unix.W_OK) != nil {
		t.Skipf("%s not writable; skipping live uinput test", uinputPath)
	}
	p, err := New(Options{Version: 0x0F01})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()
	if p.GUID() != "030081b85e0400008e020000010f0000" {
		t.Errorf("GUID = %s", p.GUID())
	}
	// Exercise the emit paths — must not error against a real device.
	if err := p.Tap("a"); err != nil {
		t.Errorf("Tap(a): %v", err)
	}
	if err := p.LeftStick(-stickFull, 0); err != nil {
		t.Errorf("LeftStick: %v", err)
	}
	if err := p.Neutral(); err != nil {
		t.Errorf("Neutral: %v", err)
	}
	if err := p.Tap("Control_L"); err == nil {
		t.Error("Tap(Control_L) should error (no pad control)")
	}
}
