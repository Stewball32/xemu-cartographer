package manager

import (
	"testing"
	"time"

	"github.com/xemu-cartographer/xc-scraper/scraper"
)

// helloNames flattens a payload's instance list for comparison.
func helloNames(p HelloPayload) []string {
	out := make([]string, 0, len(p.Instances))
	for _, inst := range p.Instances {
		out = append(out, inst.Name)
	}
	return out
}

// TestHelloPayloadFilteredKeepsSubset is the mechanics behind the W-1 hello
// narrowing (the principal → keep mapping itself is the league adapter's,
// tested in internal/leaguescraper wireadapter_test.go): keep receives the
// full name-sorted list, only the names it returns survive, unknown names it
// returns are ignored, the instance list is always a JSON array (never
// null), and the protocol fields are not identity-dependent.
func TestHelloPayloadFilteredKeepsSubset(t *testing.T) {
	m := New(Options{})
	defer m.Close()

	started := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	for _, name := range []string{"pod-b", "pod-a"} {
		r := newRunner(name, "/tmp/"+name, "host:"+name, nil, nil, nil)
		defer r.cancel()
		r.cache.StartedAt = started
		m.runners[name] = r
	}

	tests := []struct {
		name string
		keep func(names []string) []string
		want []string
	}{
		{"nil keep is unfiltered", nil, []string{"pod-a", "pod-b"}},
		{"identity", func(n []string) []string { return n }, []string{"pod-a", "pod-b"}},
		{"one of two", func([]string) []string { return []string{"pod-b"} }, []string{"pod-b"}},
		{"unknown name ignored", func([]string) []string { return []string{"pod-z"} }, []string{}},
		{"none", func([]string) []string { return nil }, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var seen []string
			keep := tc.keep
			if keep != nil {
				inner := keep
				keep = func(names []string) []string {
					seen = append([]string(nil), names...)
					return inner(names)
				}
			}
			p := m.HelloPayloadFiltered(keep)
			if p.Instances == nil {
				t.Fatal("instances = nil, want a non-nil slice so JSON marshals as []")
			}
			if keep != nil && (len(seen) != 2 || seen[0] != "pod-a" || seen[1] != "pod-b") {
				t.Fatalf("keep saw %v, want the full sorted list [pod-a pod-b]", seen)
			}
			got := helloNames(p)
			if len(got) != len(tc.want) {
				t.Fatalf("instances = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("instances = %v, want %v", got, tc.want)
				}
			}
			for _, inst := range p.Instances {
				if !inst.StartedAt.Equal(started) {
					t.Fatalf("%s started_at = %v, want %v", inst.Name, inst.StartedAt, started)
				}
			}
			if p.ProtocolVersion != scraper.ProtocolVersion || len(p.Classes) != len(allClasses) {
				t.Fatalf("protocol fields changed by the filter: version %d classes %v", p.ProtocolVersion, p.Classes)
			}
		})
	}
}
