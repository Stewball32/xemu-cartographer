package halosave

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// ---- round-trip fidelity: every real sample re-serialises byte-identical ----

func TestCERoundTripAllSamples(t *testing.T) {
	dirs, err := filepath.Glob("testdata/ce/G-*")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no CE testdata samples found: %v", err)
	}
	for _, d := range dirs {
		d := d
		t.Run(filepath.Base(d), func(t *testing.T) {
			b := readFile(t, filepath.Join(d, "blam.lst"))
			g, err := CEParse(b)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			rebuilt, err := CEBuild(b, CEPatch{}, false)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if !bytes.Equal(rebuilt, b) {
				t.Fatalf("rebuild not byte-identical (engine=%s score=%d) first diff @%d",
					g.EngineName, g.ScoreLimit, firstDiff(rebuilt, b))
			}
		})
	}
}

func TestCESurgicalEdit(t *testing.T) {
	tmpl := readFile(t, "testdata/ce/G-TS 50/blam.lst")
	name := "TS 25"
	score := uint32(25)
	time := CEMinutesToRaw(7)
	edited, err := CEBuild(tmpl, CEPatch{Name: &name, ScoreLimit: &score, TimeLimit: &time}, false)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := CEParse(edited)
	if g.Name != "TS 25" {
		t.Errorf("name = %q, want TS 25", g.Name)
	}
	if g.ScoreLimit != 25 {
		t.Errorf("score = %d, want 25", g.ScoreLimit)
	}
	if g.TimeLimit != 210 {
		t.Errorf("time = %d, want 210 (7min×30)", g.TimeLimit)
	}
	base, _ := CEParse(tmpl)
	if !bytes.Equal(g.Digest, base.Digest) {
		t.Errorf("digest changed under template-patch (should be preserved)")
	}
	// Only name (0x00..0x17), time (0x30..0x33), score (0x40..0x43) may change.
	allowed := map[int]bool{}
	for i := 0x00; i < 0x18; i++ {
		allowed[i] = true
	}
	for i := 0x30; i < 0x34; i++ {
		allowed[i] = true
	}
	for i := 0x40; i < 0x44; i++ {
		allowed[i] = true
	}
	for i := range tmpl {
		if tmpl[i] != edited[i] && !allowed[i] {
			t.Errorf("unexpected byte change at %#x", i)
		}
	}
	// Digest region 0x68..0x7B must be untouched.
	for i := 0x68; i < 0x7C; i++ {
		if tmpl[i] != edited[i] {
			t.Errorf("digest byte %#x changed", i)
		}
	}
}

func TestH2ProfileRoundTripAndEdit(t *testing.T) {
	for _, name := range []string{"profile_E4CADA6B1E65.bin", "profile_587C76321326.bin"} {
		b := readFile(t, filepath.Join("testdata/h2", name))
		if _, err := H2ProfileParse(b); err != nil {
			t.Fatalf("%s parse: %v", name, err)
		}
		rebuilt, err := H2ProfileBuild(b, H2ProfilePatch{}, false)
		if err != nil {
			t.Fatalf("%s build: %v", name, err)
		}
		if !bytes.Equal(rebuilt, b) {
			t.Fatalf("%s rebuild not byte-identical, first diff @%d", name, firstDiff(rebuilt, b))
		}
	}
	tmpl := readFile(t, "testdata/h2/profile_E4CADA6B1E65.bin")
	newName := "CARTOG"
	edited, err := H2ProfileBuild(tmpl, H2ProfilePatch{Name: &newName, AppctlPatch: map[int]byte{0x118: 7}}, false)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := H2ProfileParse(edited)
	if p.Name != "CARTOG" {
		t.Errorf("name = %q, want CARTOG", p.Name)
	}
	if edited[0x118] != 7 {
		t.Errorf("appearance byte 0x118 = %d, want 7", edited[0x118])
	}
	base, _ := H2ProfileParse(tmpl)
	if !bytes.Equal(p.Digest, base.Digest) {
		t.Errorf("digest changed (should be preserved)")
	}
	for i := 0x1E0; i < 0x1F4; i++ {
		if tmpl[i] != edited[i] {
			t.Errorf("digest region byte %#x changed", i)
		}
	}
}

func TestH2GametypeRoundTripAndEdit(t *testing.T) {
	b := readFile(t, "testdata/h2/gametype_slayer.bin")
	g, err := H2GametypeParse(b)
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "team brs" || g.ScoreLimit != 50 {
		t.Fatalf("parsed name=%q score=%d, want 'team brs'/50", g.Name, g.ScoreLimit)
	}
	rebuilt, _ := H2GametypeBuild(b, H2GametypePatch{}, false)
	if !bytes.Equal(rebuilt, b) {
		t.Fatalf("rebuild not byte-identical, first diff @%d", firstDiff(rebuilt, b))
	}
	newName := "ball 3"
	score := uint32(3)
	edited, _ := H2GametypeBuild(b, H2GametypePatch{Name: &newName, ScoreLimit: &score}, false)
	g2, _ := H2GametypeParse(edited)
	if g2.Name != "ball 3" || g2.ScoreLimit != 3 {
		t.Errorf("edited name=%q score=%d, want 'ball 3'/3", g2.Name, g2.ScoreLimit)
	}
	if !bytes.Equal(g2.Digest, g.Digest) {
		t.Errorf("digest changed (should be preserved)")
	}
}

func TestSaveMetaRoundTrip(t *testing.T) {
	paths, _ := filepath.Glob("testdata/ce/*/SaveMeta.xbx")
	h2metas, _ := filepath.Glob("testdata/h2/*.SaveMeta.xbx")
	paths = append(paths, h2metas...)
	if len(paths) == 0 {
		t.Fatal("no SaveMeta samples")
	}
	for _, p := range paths {
		b := readFile(t, p)
		name, err := SaveMetaParse(b)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if !bytes.Equal(SaveMetaBuild(name), b) {
			t.Errorf("%s: rebuild of %q not identical", p, name)
		}
	}
}

// ---- high-level Build dispatcher ----

func TestBuildCEGametype(t *testing.T) {
	score := uint32(25)
	teams := true
	set, err := Build(BuildRequest{
		Title: TitleCE, Kind: KindGametype, Name: "TS 25",
		Engine: "slayer", Teams: &teams, ScoreLimit: &score, TimeMinutes: f64ptr(7),
	})
	if err != nil {
		t.Fatal(err)
	}
	if set.TitleID != TitleIDHaloCE || set.DirName != "G-TS 25" {
		t.Errorf("dir/title: %s %s", set.TitleID, set.DirName)
	}
	if set.FatxDir != "UDATA/4d530004/G-TS 25" {
		t.Errorf("fatx dir = %q", set.FatxDir)
	}
	if len(set.Files) != 2 || set.Files[0].Name != "blam.lst" || set.Files[1].Name != "SaveMeta.xbx" {
		t.Fatalf("files = %+v", set.Files)
	}
	g := set.Parsed.(*CEGametype)
	if g.Name != "TS 25" || g.ScoreLimit != 25 || g.TimeLimit != 210 {
		t.Errorf("parsed: name=%q score=%d time=%d", g.Name, g.ScoreLimit, g.TimeLimit)
	}
	if !set.Digest.Edited {
		t.Errorf("expected Digest.Edited=true for an edited gametype")
	}
	display, _ := SaveMetaParse(set.Files[1].Data)
	if display != "TS 25" {
		t.Errorf("savemeta display = %q", display)
	}
}

func TestBuildCEEngineSwitchUsesMatchingTemplate(t *testing.T) {
	for _, eng := range CEEngines() {
		set, err := Build(BuildRequest{Title: TitleCE, Kind: KindGametype, Name: "X", Engine: eng})
		if err != nil {
			t.Fatalf("engine %s: %v", eng, err)
		}
		g := set.Parsed.(*CEGametype)
		if g.EngineName != eng {
			t.Errorf("engine %s produced %s", eng, g.EngineName)
		}
	}
}

func TestBuildH2Profile(t *testing.T) {
	set, err := Build(BuildRequest{
		Title: TitleH2, Kind: KindProfile, Name: "CARTOG",
		Appearance: map[string]int{"armor_primary": 7},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := set.Parsed.(*H2Profile)
	if p.Name != "CARTOG" {
		t.Errorf("name = %q", p.Name)
	}
	if p.Fields["armor_primary"] != 7 {
		t.Errorf("armor_primary = %d, want 7", p.Fields["armor_primary"])
	}
	if set.Files[0].Name != "profile" {
		t.Errorf("payload file = %q", set.Files[0].Name)
	}
	display, _ := SaveMetaParse(set.Files[1].Data)
	if display != "Profile: CARTOG" {
		t.Errorf("savemeta = %q", display)
	}
}

func TestBuildH2Gametype(t *testing.T) {
	score := uint32(3)
	set, err := Build(BuildRequest{Title: TitleH2, Kind: KindGametype, Name: "ball 3", ScoreLimit: &score})
	if err != nil {
		t.Fatal(err)
	}
	g := set.Parsed.(*H2Gametype)
	if g.Name != "ball 3" || g.ScoreLimit != 3 {
		t.Errorf("parsed name=%q score=%d", g.Name, g.ScoreLimit)
	}
	if set.Files[0].Name != "slayer" {
		t.Errorf("payload file = %q, want slayer", set.Files[0].Name)
	}
	display, _ := SaveMetaParse(set.Files[1].Data)
	if display != "Slayer: ball 3" {
		t.Errorf("savemeta = %q", display)
	}
}

func TestBuildUnchangedIsByteIdenticalClone(t *testing.T) {
	tmpl, _ := CETemplate("slayer")
	base, _ := CEParse(tmpl)
	set, err := Build(BuildRequest{Title: TitleCE, Kind: KindGametype, Name: base.Name, Engine: "slayer"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(set.Files[0].Data, tmpl) {
		t.Errorf("unchanged build should clone the template byte-for-byte")
	}
	if set.Digest.Edited {
		t.Errorf("unchanged clone should report Digest.Edited=false")
	}
}

// ---- guards / error paths ----

func TestBuildRejectsCEProfile(t *testing.T) {
	_, err := Build(BuildRequest{Title: TitleCE, Kind: KindProfile, Name: "x"})
	if err == nil {
		t.Fatal("expected error: CE has no profile save")
	}
}

func TestCENameTooLong(t *testing.T) {
	tmpl, _ := CETemplate("slayer")
	long := "THIS NAME IS WAY TOO LONG FOR THE BUFFER"
	if _, err := CEBuild(tmpl, CEPatch{Name: &long}, false); err == nil {
		t.Fatal("expected name-too-long error")
	}
}

func TestDigestResolved(t *testing.T) {
	if !DigestResolved() {
		t.Fatal("DigestResolved() should be true: the signature is implemented for CE and H2")
	}
	if _, err := RecomputeDigest(make([]byte, 64), 0, 20, "nope"); err == nil {
		t.Fatal("RecomputeDigest should error for an unknown title")
	}
}

// TestRecomputeReproducesRealDigests is the offline proof that the signature is
// correct and per-title (roamable): the recomputed digest equals the digest
// stored in every real, untouched sample, using only the global key + the
// title's cert key (no console input). CE 23/23, H2 3/3 (2 testdata + template).
func TestRecomputeReproducesRealDigests(t *testing.T) {
	for _, d := range globOrFail(t, "testdata/ce/G-*") {
		b := readFile(t, filepath.Join(d, "blam.lst"))
		if len(b) != ceSize {
			continue
		}
		got, err := RecomputeDigest(b, ceOffDigest, ceDigestLen, "ce")
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		if !bytes.Equal(got, b[ceOffDigest:ceOffDigest+ceDigestLen]) {
			t.Errorf("%s: recomputed digest != stored", filepath.Base(d))
		}
	}
	h2 := []string{"testdata/h2/profile_587C76321326.bin", "testdata/h2/profile_E4CADA6B1E65.bin"}
	for _, p := range h2 {
		b := readFile(t, p)
		got, err := RecomputeDigest(b, h2pOffDigest, h2pDigestLen, "h2")
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if !bytes.Equal(got, b[h2pOffDigest:h2pOffDigest+h2pDigestLen]) {
			t.Errorf("%s: recomputed digest != stored", filepath.Base(p))
		}
	}
	// The embedded H2 template must also self-verify.
	tb, _ := H2ProfileTemplate()
	got, _ := RecomputeDigest(tb, h2pOffDigest, h2pDigestLen, "h2")
	if !bytes.Equal(got, tb[h2pOffDigest:h2pOffDigest+h2pDigestLen]) {
		t.Error("embedded H2 template digest does not self-verify")
	}
}

// TestBuildH2ProfileIsCorrectlySigned is the regression test for the "damaged"
// bug: an edited generated profile must carry a digest that is VALID for its own
// (edited) content — not the template's stale digest.
func TestBuildH2ProfileIsCorrectlySigned(t *testing.T) {
	tmpl, _ := H2ProfileTemplate()
	set, err := Build(BuildRequest{
		Title: TitleH2, Kind: KindProfile, Name: "CARTOG",
		Appearance: map[string]int{"armor_primary": 7},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := set.Files[0].Data
	stored := payload[h2pOffDigest : h2pOffDigest+h2pDigestLen]
	want, _ := RecomputeDigest(payload, h2pOffDigest, h2pDigestLen, "h2")
	if !bytes.Equal(stored, want) {
		t.Fatalf("generated profile digest is INVALID for its content (the 'damaged' bug)")
	}
	// And it must NOT be the template's stale digest (content changed).
	if bytes.Equal(stored, tmpl[h2pOffDigest:h2pOffDigest+h2pDigestLen]) {
		t.Fatalf("generated profile still carries the template's stale digest")
	}
	if set.Digest.Mode != DigestRecomputed || !set.Digest.Resolved {
		t.Errorf("DigestStatus = %+v, want recomputed/resolved", set.Digest)
	}
}

// TestH2ProfileNameWritePreservesSubBlock guards the second bug: writing a name
// must NOT clobber the per-profile sub-block at 0xEC..0x117 (the fixed 16-byte
// 0xFF field + the bytes at 0xFC/0x101/0x102).
func TestH2ProfileNameWritePreservesSubBlock(t *testing.T) {
	tmpl, _ := H2ProfileTemplate()
	newName := "X" // shorter than the template name to exercise zero-fill
	edited, err := H2ProfileBuild(tmpl, H2ProfilePatch{Name: &newName}, true)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0xEC; i < 0x118; i++ {
		if edited[i] != tmpl[i] {
			t.Errorf("sub-block byte %#x clobbered: tmpl=%#x edited=%#x", i, tmpl[i], edited[i])
		}
	}
	// Name area after the new name (up to 0xEC) must be zeroed.
	for i := 0x08 + 2*len([]rune(newName)) + 2; i < 0xEC; i++ {
		if edited[i] != 0 {
			t.Errorf("name padding byte %#x not zero: %#x", i, edited[i])
		}
	}
	// Re-signed file must be self-valid.
	want, _ := RecomputeDigest(edited, h2pOffDigest, h2pDigestLen, "h2")
	if !bytes.Equal(edited[h2pOffDigest:h2pOffDigest+h2pDigestLen], want) {
		t.Error("edited profile not correctly re-signed")
	}
}

// TestBuildUnchangedH2CloneByteIdentical: re-signing an unchanged profile
// reproduces the template byte-for-byte (the signature is deterministic and the
// template's own stored digest is valid).
func TestBuildUnchangedH2CloneByteIdentical(t *testing.T) {
	tmpl, _ := H2ProfileTemplate()
	base, _ := H2ProfileParse(tmpl)
	set, err := Build(BuildRequest{Title: TitleH2, Kind: KindProfile, Name: base.Name})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(set.Files[0].Data, tmpl) {
		t.Errorf("unchanged H2 profile build should clone the template byte-for-byte (first diff @%d)",
			firstDiff(set.Files[0].Data, tmpl))
	}
}

func TestUnknownAppearanceFieldRejected(t *testing.T) {
	_, err := Build(BuildRequest{Title: TitleH2, Kind: KindProfile, Name: "x",
		Appearance: map[string]int{"not_a_field": 1}})
	if err == nil {
		t.Fatal("expected unknown-field error")
	}
}

func TestEmbeddedTemplatesParse(t *testing.T) {
	for _, eng := range CEEngines() {
		b, err := CETemplate(eng)
		if err != nil {
			t.Fatalf("template %s: %v", eng, err)
		}
		g, err := CEParse(b)
		if err != nil {
			t.Fatalf("template %s parse: %v", eng, err)
		}
		if g.EngineName != eng {
			t.Errorf("template %s has engine %s", eng, g.EngineName)
		}
	}
	if b, err := H2ProfileTemplate(); err != nil || len(b) != h2pSize {
		t.Errorf("h2 profile template: len=%d err=%v", len(b), err)
	}
	if b, err := H2GametypeTemplate(); err != nil {
		t.Errorf("h2 gametype template: %v", err)
	} else if _, err := H2GametypeParse(b); err != nil {
		t.Errorf("h2 gametype template parse: %v", err)
	}
}

// ---- helpers ----

func readFile(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return b
}

func globOrFail(t *testing.T, pat string) []string {
	t.Helper()
	m, err := filepath.Glob(pat)
	if err != nil || len(m) == 0 {
		t.Fatalf("no matches for %q: %v", pat, err)
	}
	return m
}

func firstDiff(a, b []byte) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return min(len(a), len(b))
	}
	return -1
}

func f64ptr(v float64) *float64 { return &v }
