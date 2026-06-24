package lansaves

import (
	"net/url"
	"strings"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// TestBuildManifestShape checks the catalog advertises the two games and the
// expected concrete items (5 CE gametypes + 1 H2 gametype + 1 H2 profile).
func TestBuildManifestShape(t *testing.T) {
	m := buildManifest()
	if len(m.Games) != 2 {
		t.Fatalf("games = %d, want 2", len(m.Games))
	}
	if m.Count != len(m.Items) {
		t.Errorf("count %d != len(items) %d", m.Count, len(m.Items))
	}
	// 5 CE engines + H2 gametype + H2 profile.
	wantItems := len(halosave.CEEngines()) + 2
	if len(m.Items) != wantItems {
		t.Errorf("items = %d, want %d", len(m.Items), wantItems)
	}

	var gametypes, profiles int
	for _, it := range m.Items {
		switch it.Category {
		case "gametype":
			gametypes++
		case "profile":
			profiles++
		default:
			t.Errorf("unexpected category %q", it.Category)
		}
		if it.FatxDir == "" || !strings.HasPrefix(it.FatxDir, "UDATA/") {
			t.Errorf("item %s: bad fatx_dir %q", it.ID, it.FatxDir)
		}
		if it.FootprintBytes == 0 {
			t.Errorf("item %s: zero footprint", it.ID)
		}
	}
	if profiles != 1 {
		t.Errorf("profiles = %d, want 1", profiles)
	}
	if gametypes != len(halosave.CEEngines())+1 {
		t.Errorf("gametypes = %d, want %d", gametypes, len(halosave.CEEngines())+1)
	}
}

// TestManifestPathsRoundTrip is the key additive guarantee: every download
// `path` the manifest advertises must parse through the existing spec parser
// and generate successfully. This binds the new endpoint to the existing
// download contract — if a future change breaks the path/spec relationship,
// this fails rather than the Xbox.
func TestManifestPathsRoundTrip(t *testing.T) {
	for _, it := range buildManifest().Items {
		q := it.Path
		i := strings.IndexByte(q, '?')
		if i < 0 {
			t.Fatalf("item %s: path has no query: %q", it.ID, it.Path)
		}
		if !strings.HasPrefix(it.Path, "/api/lan/saves/download?") {
			t.Errorf("item %s: unexpected path base %q", it.ID, it.Path)
		}
		vals, err := url.ParseQuery(q[i+1:])
		if err != nil {
			t.Fatalf("item %s: bad query: %v", it.ID, err)
		}
		req, tr := specFromQuery(vals)
		if tr.Format != "tar" {
			t.Errorf("item %s: format = %q, want tar", it.ID, tr.Format)
		}
		set, err := halosave.Build(req)
		if err != nil {
			t.Errorf("item %s: path does not generate: %v", it.ID, err)
			continue
		}
		if set.FatxDir != it.FatxDir {
			t.Errorf("item %s: fatx_dir mismatch: manifest %q vs build %q", it.ID, it.FatxDir, set.FatxDir)
		}
	}
}

// TestManifestJSONShape logs the exact client-facing JSON and asserts the flat
// field contract the on-Xbox parser depends on.
func TestManifestJSONShape(t *testing.T) {
	b, err := jsonMarshalIndent(buildManifest())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("\n%s", b)
	for _, want := range []string{
		`"hub": "xemu-cartographer"`,
		`"category": "game"`,
		`"category": "gametype"`,
		`"category": "profile"`,
		`"path": "/api/lan/saves/download?`,
		`"fatx_dir": "UDATA/`,
	} {
		if !containsStr(string(b), want) {
			t.Errorf("manifest JSON missing %q", want)
		}
	}
}
