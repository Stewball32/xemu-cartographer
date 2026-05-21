package sinks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileSinkAppendsNDJSON: writes are appended one envelope per line.
// A second Write after Close re-opens the file in append mode, so the
// existing contents are preserved — sinkManager relies on this when a
// policy reload temporarily closes and reopens a sink with the same
// path.
func TestFileSinkAppendsNDJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "tick.ndjson") // verify mkdir
	s := NewFileSink(path)

	if err := s.Write([]byte(`{"v":2,"type":"tick","seq":1}`)); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	if err := s.Write([]byte(`{"v":2,"type":"tick","seq":2}`)); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Reopen and append after close — proves append semantics.
	if err := s.Write([]byte(`{"v":2,"type":"tick","seq":3}`)); err != nil {
		t.Fatalf("write 3 after close: %v", err)
	}
	_ = s.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	got := string(data)
	want := `{"v":2,"type":"tick","seq":1}` + "\n" +
		`{"v":2,"type":"tick","seq":2}` + "\n" +
		`{"v":2,"type":"tick","seq":3}` + "\n"
	if got != want {
		t.Fatalf("file content mismatch.\n got: %q\nwant: %q", got, want)
	}
}

// TestFileSinkCloseIdempotent: sinkManager may call Close from both
// the policy-reload path and runner shutdown; the second call must
// not return an error.
func TestFileSinkCloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	s := NewFileSink(filepath.Join(dir, "a.ndjson"))
	if err := s.Write([]byte(`{}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second close should be a no-op, got %v", err)
	}
}

// TestParseEmptySpec: empty spec returns (nil, nil) — the runner reads
// this as "no sink, just broadcast". Critical because most policies
// will have an empty Sink field.
func TestParseEmptySpec(t *testing.T) {
	s, err := Parse("", nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if s != nil {
		t.Fatalf("empty spec: want nil sink, got %T", s)
	}
}

// TestParseFileSchemeExpandsVars: the {instance} placeholder must be
// substituted at parse time so different runners' policies with the
// same spec template land in different files.
func TestParseFileSchemeExpandsVars(t *testing.T) {
	s, err := Parse("file:replays/{instance}/tick.ndjson", map[string]string{"instance": "debug-host"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fs, ok := s.(*FileSink)
	if !ok {
		t.Fatalf("want *FileSink, got %T", s)
	}
	if fs.Path() != "replays/debug-host/tick.ndjson" {
		t.Fatalf("var expansion: got %q", fs.Path())
	}
}

// TestParseUnknownVarsLeftIntact: {game_id} is in the design doc but
// not yet supported. Parse must leave unknown placeholders untouched
// so the resulting path is recognisable and a future release can
// introduce the variable without invalidating older rows.
func TestParseUnknownVarsLeftIntact(t *testing.T) {
	s, err := Parse("file:replays/{game_id}.ndjson", map[string]string{"instance": "x"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fs := s.(*FileSink)
	if fs.Path() != "replays/{game_id}.ndjson" {
		t.Fatalf("unknown var: got %q", fs.Path())
	}
}

// TestParseUnknownScheme: typos in the spec ("flie:..." or omitted
// scheme) should fail loudly at parse time rather than silently
// dropping captures.
func TestParseUnknownScheme(t *testing.T) {
	cases := []string{
		"flie:bad",
		"missing-scheme",
		":no-scheme",
	}
	for _, spec := range cases {
		_, err := Parse(spec, nil)
		if err == nil {
			t.Fatalf("Parse(%q): want error, got nil", spec)
		}
		if !strings.Contains(err.Error(), "sink:") {
			t.Fatalf("Parse(%q): err should namespace with `sink:`, got %v", spec, err)
		}
	}
}
