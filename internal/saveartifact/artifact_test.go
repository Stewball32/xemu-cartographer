package saveartifact

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// tarEntries reads a tar blob into a name->bytes map for assertions.
func tarEntries(t *testing.T, blob []byte) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(blob))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("tar entry read: %v", err)
		}
		out[hdr.Name] = data
	}
	return out
}

func TestBuildH2Profile(t *testing.T) {
	b, err := Build(H2ProfileRequest("CARTOG", map[string]int{"armor_primary": 13}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if b.Set.Title != halosave.TitleH2 || b.Set.Kind != halosave.KindProfile {
		t.Fatalf("unexpected title/kind: %s/%s", b.Set.Title, b.Set.Kind)
	}
	entries := tarEntries(t, b.Tar)
	wantProfile := "UDATA/" + halosave.TitleIDHalo2 + "/" + b.Set.DirName + "/profile"
	wantMeta := "UDATA/" + halosave.TitleIDHalo2 + "/" + b.Set.DirName + "/SaveMeta.xbx"
	if _, ok := entries[wantProfile]; !ok {
		t.Fatalf("tar missing %q; have %v", wantProfile, keys(entries))
	}
	if _, ok := entries[wantMeta]; !ok {
		t.Fatalf("tar missing %q; have %v", wantMeta, keys(entries))
	}
	if len(entries[wantProfile]) != 500 {
		t.Fatalf("h2 profile payload = %d bytes, want 500", len(entries[wantProfile]))
	}
}

func TestBuildCEGametype(t *testing.T) {
	score := uint32(50)
	teams := true
	b, err := Build(GametypeRequest("ce", "slayer", "TS 50", GametypeSettings{
		ScoreLimit: &score,
		Teams:      &teams,
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	entries := tarEntries(t, b.Tar)
	wantBlam := "UDATA/" + halosave.TitleIDHaloCE + "/" + b.Set.DirName + "/blam.lst"
	if _, ok := entries[wantBlam]; !ok {
		t.Fatalf("tar missing %q; have %v", wantBlam, keys(entries))
	}
	if len(entries[wantBlam]) != 512 {
		t.Fatalf("ce gametype payload = %d bytes, want 512", len(entries[wantBlam]))
	}
}

// TestBundlesAreSigned guards the dependency on the halosave digest fix: every
// generated bundle must carry a RECOMPUTED (correct) signature, not the
// template's stale one — otherwise Halo 2 rejects edited profiles as "damaged".
func TestBundlesAreSigned(t *testing.T) {
	cases := []struct {
		name string
		req  halosave.BuildRequest
	}{
		{"h2-profile", H2ProfileRequest("CARTOG", map[string]int{"armor_primary": 13})},
		{"ce-gametype", GametypeRequest("ce", "slayer", "TS 50", GametypeSettings{})},
	}
	for _, c := range cases {
		b, err := Build(c.req)
		if err != nil {
			t.Fatalf("%s: Build: %v", c.name, err)
		}
		if !b.Set.Digest.Resolved {
			t.Errorf("%s: digest not resolved — generated file is not correctly signed", c.name)
		}
		if string(b.Set.Digest.Mode) != "recomputed" {
			t.Errorf("%s: digest mode = %q, want \"recomputed\"", c.name, b.Set.Digest.Mode)
		}
	}
}

func TestBuildRejectsBadAppearance(t *testing.T) {
	_, err := Build(H2ProfileRequest("X", map[string]int{"armor_primary": 999}))
	if err == nil {
		t.Fatal("expected out-of-range appearance byte to error")
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
