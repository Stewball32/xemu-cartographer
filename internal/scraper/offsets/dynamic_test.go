package offsets

import (
	"strings"
	"testing"
)

// A minimal, valid offsetmap export for the dynamic-source tests. Game matches
// the embedded haloce baseline so game-checking is exercised both ways.
const dynRaw = `{
  "game": "haloce",
  "id": "nhe-test-1.0",
  "description": "dynamic test set",
  "offsets": {
    "PlayerBlock": {"value": "0x0032A480", "type": "address"},
    "UIPath":      {"value": "ui\\shell\\thing", "type": "string"}
  }
}`

func withDynamic(t *testing.T, fn DynamicSource) {
	t.Helper()
	SetDynamicSource(fn)
	t.Cleanup(func() { SetDynamicSource(nil) })
}

// TestLookupDynamic: an id the embedded registry doesn't know resolves through
// the dynamic source, and the RECORD id wins over the file's internal id (the
// "Save as" rename path).
func TestLookupDynamic(t *testing.T) {
	withDynamic(t, func(id string) ([]byte, bool) {
		if id == "renamed-set" || id == "nhe-test-1.0" {
			return []byte(dynRaw), true
		}
		return nil, false
	})

	s, err := Lookup("haloce", "renamed-set")
	if err != nil {
		t.Fatalf("dynamic lookup: %v", err)
	}
	if s.ID != "renamed-set" {
		t.Errorf("record id should win over the file's internal id; got %q", s.ID)
	}
	if v, err := s.Addr("PlayerBlock"); err != nil || v != 0x0032A480 {
		t.Errorf("Addr(PlayerBlock) = (%#x, %v)", v, err)
	}
	if v, err := s.Str("UIPath"); err != nil || v != `ui\shell\thing` {
		t.Errorf("Str(UIPath) = (%q, %v)", v, err)
	}
}

// TestLookupDynamicGameMismatch: a dynamic set for the wrong game is rejected,
// and Resolve degrades to the baseline with a warning (fail-soft).
func TestLookupDynamicGameMismatch(t *testing.T) {
	withDynamic(t, func(id string) ([]byte, bool) { return []byte(dynRaw), true })

	if _, err := Lookup("halo2", "some-ce-set"); err == nil {
		t.Fatal("expected game-mismatch error")
	}
	s, warn := Resolve("halo2", "some-ce-set")
	if warn == nil {
		t.Error("expected fail-soft warning")
	}
	if s.ID != BaselineID("halo2") {
		t.Errorf("should degrade to baseline, got %q", s.ID)
	}
}

// TestLookupDynamicParseError: unparseable stored bytes surface as an error
// (never a silent baseline — the set exists but is genuinely unusable).
func TestLookupDynamicParseError(t *testing.T) {
	withDynamic(t, func(id string) ([]byte, bool) { return []byte("{not json"), true })
	_, err := Lookup("haloce", "corrupt-set")
	if err == nil || !strings.Contains(err.Error(), "imported set") {
		t.Fatalf("expected parse error mentioning the imported set, got %v", err)
	}
}

// TestLookupEmbeddedWins: an embedded id never consults the dynamic source.
func TestLookupEmbeddedWins(t *testing.T) {
	called := false
	withDynamic(t, func(id string) ([]byte, bool) { called = true; return nil, false })
	if _, err := Lookup("haloce", "ce-baseline"); err != nil {
		t.Fatalf("embedded lookup: %v", err)
	}
	if called {
		t.Error("dynamic source consulted for an embedded id")
	}
}

// TestRaw: embedded sets re-export their exact file bytes + source filename;
// unknown ids report ok=false.
func TestRaw(t *testing.T) {
	raw, name, ok := Raw("ce-baseline")
	if !ok || len(raw) == 0 || name != "ce-baseline.json" {
		t.Fatalf("Raw(ce-baseline) = (%d bytes, %q, %v)", len(raw), name, ok)
	}
	if s, err := ParseSet(raw); err != nil || s.ID != "ce-baseline" {
		t.Errorf("re-parse of raw bytes: (%v, %v)", s, err)
	}
	if _, _, ok := Raw("no-such-set"); ok {
		t.Error("Raw of unknown id should report ok=false")
	}
}
