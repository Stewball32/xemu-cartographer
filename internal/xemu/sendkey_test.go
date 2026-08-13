package xemu

import (
	"testing"
	"time"
)

func TestQemuKeyName(t *testing.T) {
	cases := map[string]string{
		"a":         "a",
		"y":         "y",
		"Up":        "up",
		"Down":      "down",
		"Left":      "left",
		"Right":     "right",
		"Return":    "ret",
		"BackSpace": "backspace",
		"5":         "5",
		"Control_L": "ctrl",
		"r":         "r",
		"F1":        "f1",
		"F12":       "f12",
	}
	for in, want := range cases {
		got, err := qemuKeyName(in)
		if err != nil {
			t.Fatalf("qemuKeyName(%q) unexpected error: %v", in, err)
		}
		if got != want {
			t.Errorf("qemuKeyName(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := qemuKeyName("Backspace"); err == nil {
		t.Error("qemuKeyName(\"Backspace\") should error — labels are case-sensitive (want \"BackSpace\")")
	}
	if _, err := qemuKeyName("nope"); err == nil {
		t.Error("qemuKeyName(\"nope\") should error on unknown key")
	}
}

func TestQemuChord(t *testing.T) {
	got, err := qemuChord([]string{"Control_L", "r"})
	if err != nil {
		t.Fatalf("qemuChord error: %v", err)
	}
	if got != "ctrl-r" {
		t.Errorf("qemuChord(Control_L,r) = %q, want %q", got, "ctrl-r")
	}

	got, err = qemuChord([]string{"Return"})
	if err != nil {
		t.Fatalf("qemuChord error: %v", err)
	}
	if got != "ret" {
		t.Errorf("qemuChord(Return) = %q, want %q", got, "ret")
	}

	if _, err := qemuChord(nil); err == nil {
		t.Error("qemuChord(nil) should error on empty key list")
	}
	if _, err := qemuChord([]string{"a", "bogus"}); err == nil {
		t.Error("qemuChord with an unknown key should error")
	}
}

func TestHoldToMs(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want int
	}{
		{0, 0},
		{-5 * time.Millisecond, 0},
		{100 * time.Millisecond, 100},
		{900 * time.Millisecond, 900},
		{time.Second, 1000},
		{500 * time.Microsecond, 1}, // sub-ms positive clamps to 1
	}
	for _, c := range cases {
		if got := holdToMs(c.in); got != c.want {
			t.Errorf("holdToMs(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestBuildSendKeyCmd(t *testing.T) {
	if got := buildSendKeyCmd("e", 0); got != "sendkey e" {
		t.Errorf("buildSendKeyCmd(e,0) = %q, want %q", got, "sendkey e")
	}
	if got := buildSendKeyCmd("e", 900); got != "sendkey e 900" {
		t.Errorf("buildSendKeyCmd(e,900) = %q, want %q", got, "sendkey e 900")
	}
	if got := buildSendKeyCmd("ctrl-r", 200); got != "sendkey ctrl-r 200" {
		t.Errorf("buildSendKeyCmd(ctrl-r,200) = %q, want %q", got, "sendkey ctrl-r 200")
	}
}

// TestSupportedKeysMirrorsVncMap guards against the primitive silently drifting
// from the frontend VNC key vocabulary it claims to mirror.
func TestSupportedKeysMirrorsVncMap(t *testing.T) {
	// The exact label set from sveltekit/src/lib/utils/vnc-keyboard.ts KEYSYM.
	want := []string{
		"a", "b", "x", "y",
		"Up", "Down", "Left", "Right",
		"Return", "BackSpace", "5",
		"1", "2", "3", "4",
		"w", "o",
		"e", "s", "d", "f",
		"i", "j", "k", "l",
		"Control_L", "r",
		"F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12",
	}
	have := make(map[string]bool)
	for _, k := range SupportedKeys() {
		have[k] = true
	}
	for _, k := range want {
		if !have[k] {
			t.Errorf("SupportedKeys missing frontend VNC label %q", k)
		}
	}
	if len(SupportedKeys()) != len(want) {
		t.Errorf("SupportedKeys has %d entries, VNC map has %d — they should match 1:1",
			len(SupportedKeys()), len(want))
	}
}
