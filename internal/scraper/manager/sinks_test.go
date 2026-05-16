package manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/capture"
)

// TestSinkManagerOpenWriteClose: the basic happy path — an `always`
// policy with a file: sink opens a file on first write and persists
// the envelope bytes verbatim with a trailing newline.
func TestSinkManagerOpenWriteClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tick.ndjson")
	sm := newSinkManager("alpha")
	sm.applyPolicies([]capture.Policy{
		{Instance: "alpha", Class: "tick", Mode: capture.ModeAlways, Sink: "file:" + path},
	})

	sm.write("tick", []byte(`{"v":2,"type":"tick","seq":1}`))
	sm.write("tick", []byte(`{"v":2,"type":"tick","seq":2}`))
	sm.closeAll()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := `{"v":2,"type":"tick","seq":1}` + "\n" + `{"v":2,"type":"tick","seq":2}` + "\n"
	if string(got) != want {
		t.Fatalf("file mismatch.\n got: %q\nwant: %q", got, want)
	}
}

// TestSinkManagerSkipsClassWithoutSink: a policy with mode always but
// empty Sink must not open a sink — the class is broadcast-only.
// write() for an unconfigured class is a silent no-op so the runner's
// emit hot path doesn't need to branch on "has sink".
func TestSinkManagerSkipsClassWithoutSink(t *testing.T) {
	sm := newSinkManager("alpha")
	sm.applyPolicies([]capture.Policy{
		{Instance: "alpha", Class: "tick", Mode: capture.ModeAlways}, // no Sink
	})
	if got := len(sm.active); got != 0 {
		t.Fatalf("no-sink policy created %d sinks, want 0", got)
	}
	sm.write("tick", []byte(`{"v":2}`)) // must not panic
}

// TestSinkManagerInstanceTemplate: the {instance} placeholder is
// expanded at sink-construction time, so two runners with the same
// policy template land in distinct files.
func TestSinkManagerInstanceTemplate(t *testing.T) {
	dir := t.TempDir()
	spec := "file:" + filepath.Join(dir, "{instance}", "tick.ndjson")

	for _, inst := range []string{"alpha", "bravo"} {
		sm := newSinkManager(inst)
		sm.applyPolicies([]capture.Policy{
			{Instance: "*", Class: "tick", Mode: capture.ModeAlways, Sink: spec},
		})
		sm.write("tick", []byte(`{"v":2}`))
		sm.closeAll()
	}

	for _, inst := range []string{"alpha", "bravo"} {
		p := filepath.Join(dir, inst, "tick.ndjson")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
		if !strings.Contains(string(b), `"v":2`) {
			t.Fatalf("%s: content not written", p)
		}
	}
}

// TestSinkManagerReplaceOnSpecChange: changing a class's sink spec via
// applyPolicies must close the old sink and open a new one. The new
// file gets the next write; the old file keeps whatever was already
// written.
func TestSinkManagerReplaceOnSpecChange(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.ndjson")
	new_ := filepath.Join(dir, "new.ndjson")
	sm := newSinkManager("alpha")

	sm.applyPolicies([]capture.Policy{
		{Instance: "alpha", Class: "tick", Mode: capture.ModeAlways, Sink: "file:" + old},
	})
	sm.write("tick", []byte(`{"seq":1}`))

	sm.applyPolicies([]capture.Policy{
		{Instance: "alpha", Class: "tick", Mode: capture.ModeAlways, Sink: "file:" + new_},
	})
	sm.write("tick", []byte(`{"seq":2}`))
	sm.closeAll()

	if b, _ := os.ReadFile(old); string(b) != "{\"seq\":1}\n" {
		t.Fatalf("old file: got %q", b)
	}
	if b, _ := os.ReadFile(new_); string(b) != "{\"seq\":2}\n" {
		t.Fatalf("new file: got %q", b)
	}
}

// TestSinkManagerDropsRemovedClass: when a policy row is removed (the
// effective policy reverts to DefaultPolicy with empty Sink), the
// active sink for that class must be closed and dropped. Subsequent
// writes for the class become no-ops.
func TestSinkManagerDropsRemovedClass(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tick.ndjson")
	sm := newSinkManager("alpha")

	sm.applyPolicies([]capture.Policy{
		{Instance: "alpha", Class: "tick", Mode: capture.ModeAlways, Sink: "file:" + path},
	})
	sm.write("tick", []byte(`{"seq":1}`))

	sm.applyPolicies(nil) // removal: no rows match
	if _, ok := sm.active["tick"]; ok {
		t.Fatal("sink for removed class should be dropped")
	}
	sm.write("tick", []byte(`{"seq":2}`)) // silent no-op
	sm.closeAll()

	got, _ := os.ReadFile(path)
	if string(got) != "{\"seq\":1}\n" {
		t.Fatalf("file after drop: got %q, want only seq:1", got)
	}
}
