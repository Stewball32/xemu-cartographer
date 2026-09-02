package offsets

import (
	"strings"
	"testing"
)

// A minimal valid baseline export for a game with no embedded sets — the
// out-of-tree plugin registration path. The id is deliberately unconventional
// to prove BaselineID follows the registration, not the naming fallback.
const pluginRaw = `{
  "game": "quantumredshift",
  "id": "qrs-stock-1.0",
  "description": "plugin-registered baseline",
  "offsets": {
    "AddrFoo": {"value": "0x00123456", "type": "address"},
    "TagBar":  {"value": "ui\\shell\\bar", "type": "string"}
  }
}`

func registerPluginBaseline(t *testing.T) {
	t.Helper()
	if err := RegisterBaseline("quantumredshift", []byte(pluginRaw)); err != nil {
		t.Fatalf("RegisterBaseline: %v", err)
	}
	t.Cleanup(func() {
		delete(registry, "qrs-stock-1.0")
		delete(rawRegistry, "qrs-stock-1.0")
		delete(rawNames, "qrs-stock-1.0")
		delete(baselineIDs, "quantumredshift")
	})
}

// TestRegisterBaseline: a plugin-registered baseline resolves exactly like an
// embedded one — Baseline, BaselineID, Resolve's empty + fail-soft paths, the
// All() listing, and Raw re-export.
func TestRegisterBaseline(t *testing.T) {
	registerPluginBaseline(t)

	if got := BaselineID("quantumredshift"); got != "qrs-stock-1.0" {
		t.Errorf("BaselineID = %q, want the registered id", got)
	}
	s, err := Baseline("quantumredshift")
	if err != nil || s.ID != "qrs-stock-1.0" || s.Len() != 2 {
		t.Fatalf("Baseline = %+v, %v", s, err)
	}
	if s, warn := Resolve("quantumredshift", ""); warn != nil || s.ID != "qrs-stock-1.0" {
		t.Errorf(`Resolve("") = %v, %v`, s, warn)
	}
	if s, warn := Resolve("quantumredshift", "no-such-set"); warn == nil || s == nil || s.ID != "qrs-stock-1.0" {
		t.Errorf("invalid explicit id should fall back with warning, got %v, %v", s, warn)
	}
	found := false
	for _, info := range All() {
		if info.ID == "qrs-stock-1.0" {
			found = info.Baseline && info.Game == "quantumredshift"
		}
	}
	if !found {
		t.Error("All() should list the registered set with the baseline flag")
	}
	if raw, name, ok := Raw("qrs-stock-1.0"); !ok || len(raw) == 0 || name != "qrs-stock-1.0.json" {
		t.Errorf("Raw = (%d bytes, %q, %v)", len(raw), name, ok)
	}
}

// TestRegisterBaselineRejects: every invalid registration errors without
// mutating the registry.
func TestRegisterBaselineRejects(t *testing.T) {
	registerPluginBaseline(t)

	cases := []struct {
		name, game, raw, wantErr string
	}{
		{"malformed json", "g2", `{not json`, "register baseline"},
		{"game mismatch", "g2", `{"game":"other","id":"g2-x","offsets":{}}`, "belongs to game"},
		{"duplicate set id", "g2", `{"game":"g2","id":"qrs-stock-1.0","offsets":{}}`, "duplicate set id"},
		{"second baseline for game", "quantumredshift", `{"game":"quantumredshift","id":"qrs-2","offsets":{}}`, "already has baseline"},
		{"embedded game already covered", "haloce", `{"game":"haloce","id":"ce-alt","offsets":{}}`, "already has baseline"},
	}
	for _, tc := range cases {
		err := RegisterBaseline(tc.game, []byte(tc.raw))
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: err = %v, want containing %q", tc.name, err, tc.wantErr)
		}
	}
	for _, id := range []string{"g2-x", "qrs-2", "ce-alt"} {
		if _, ok := registry[id]; ok {
			t.Errorf("rejected registration %q leaked into registry", id)
		}
		if _, ok := rawRegistry[id]; ok {
			t.Errorf("rejected registration %q leaked into rawRegistry", id)
		}
		if _, ok := rawNames[id]; ok {
			t.Errorf("rejected registration %q leaked into rawNames", id)
		}
	}
	if _, ok := baselineIDs["g2"]; ok {
		t.Error("rejected game g2 gained a baseline id")
	}
	if got := baselineIDs["quantumredshift"]; got != "qrs-stock-1.0" {
		t.Errorf("quantumredshift baseline id = %q, want the original qrs-stock-1.0", got)
	}
	if got := BaselineID("haloce"); got != "ce-baseline" {
		t.Errorf("haloce baseline id = %q, want the embedded ce-baseline untouched", got)
	}
}
