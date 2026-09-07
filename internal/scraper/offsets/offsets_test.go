package offsets

import (
	"strings"
	"testing"
)

// TestBaselinesRegistered proves every shipped game has its baseline set
// embedded and non-empty — the invariant the whole selection layer rests on.
func TestBaselinesRegistered(t *testing.T) {
	for _, game := range []string{"haloce", "halo2"} {
		s, err := Baseline(game)
		if err != nil {
			t.Fatalf("Baseline(%q): %v", game, err)
		}
		if s.Game != game {
			t.Errorf("Baseline(%q).Game = %q", game, s.Game)
		}
		if s.Len() == 0 {
			t.Errorf("Baseline(%q) is empty", game)
		}
	}
}

// TestBaselineUnknownGame: a game without a baseline is an error, never a
// panic — a plugin misregistration must not kill the runner goroutine.
func TestBaselineUnknownGame(t *testing.T) {
	if s, err := Baseline("nosuchgame"); err == nil || s != nil {
		t.Errorf("Baseline(nosuchgame) = %v, %v; want nil, error", s, err)
	}
}

// TestResolve covers the selection rules: empty → baseline; valid explicit id →
// that set; unknown id or wrong-game id → baseline + warning (fail-soft); no
// baseline at all → nil set + hard error (never a panic).
func TestResolve(t *testing.T) {
	if s, warn := Resolve("haloce", ""); warn != nil || s.ID != "ce-baseline" {
		t.Errorf(`Resolve("haloce","") = %v, %v`, s.ID, warn)
	}
	if s, warn := Resolve("haloce", "ce-baseline"); warn != nil || s.ID != "ce-baseline" {
		t.Errorf(`Resolve explicit baseline = %v, %v`, s.ID, warn)
	}
	if s, warn := Resolve("haloce", "no-such-set"); warn == nil || s.ID != "ce-baseline" {
		t.Errorf("unknown id should warn + fall back, got %v, %v", s.ID, warn)
	}
	if s, warn := Resolve("haloce", "h2-baseline"); warn == nil || s.ID != "ce-baseline" {
		t.Errorf("wrong-game id should warn + fall back, got %v, %v", s.ID, warn)
	}
	if s, err := Resolve("nosuchgame", ""); err == nil || s != nil {
		t.Errorf("no-baseline game should hard-error, got %v, %v", s, err)
	}
	if s, err := Resolve("nosuchgame", "some-set"); err == nil || s != nil {
		t.Errorf("explicit id with no baseline should hard-error, got %v, %v", s, err)
	}
}

// TestParseSet covers the file format: hex values, string entries, mapper-export
// extra fields tolerated, and malformed input rejected.
func TestParseSet(t *testing.T) {
	s, err := parseSet([]byte(`{
		"game": "haloce", "id": "test-set",
		"offsets": {
			"AddrFoo": {"value": "0x2E3684", "type": "uint32", "confidence": "runtime-verified", "notes": "extra fields ignored"},
			"TagPathBar": {"value": "ui\\shell\\thing", "type": "string"}
		}
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v, err := s.Addr("AddrFoo"); err != nil || v != 0x2E3684 {
		t.Errorf("Addr = 0x%X, %v", v, err)
	}
	if v, err := s.Str("TagPathBar"); err != nil || v != `ui\shell\thing` {
		t.Errorf("Str = %q, %v", v, err)
	}
	if _, err := s.Addr("Missing"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("missing key should error, got %v", err)
	}
	if _, err := parseSet([]byte(`{"game":"g","id":"x","offsets":{"A":{"value":"zzz"}}}`)); err == nil {
		t.Error("bad hex should error")
	}
	if _, err := parseSet([]byte(`{"offsets":{}}`)); err == nil {
		t.Error("missing game/id should error")
	}
}

// TestAll lists both baselines with the baseline flag set.
func TestAll(t *testing.T) {
	byID := map[string]SetInfo{}
	for _, s := range All() {
		byID[s.ID] = s
	}
	for _, id := range []string{"ce-baseline", "h2-baseline"} {
		s, ok := byID[id]
		if !ok || !s.Baseline || s.Count == 0 {
			t.Errorf("All() missing/odd %s: %+v", id, s)
		}
	}
}
