package halosave

import (
	"bytes"
	"path/filepath"
	"testing"
)

// The ce_livemap fixtures are the 2026-08-07 single-setting differential saves
// produced in-game on the testrig bench (H1 Performance Build). Each isolates
// one setting; together they pin every field of the live-verified byte map. This
// test is the offline proof that ce.go's offsets/labels are correct.

// TestCELiveMapFieldValues asserts, per fixture, ONLY the field(s) that fixture
// isolates (the differential saves are cumulative, so unrelated fields carry
// prior settings; and 0x50 aliases across engines, so each engine's scratch
// field is asserted only for that engine). Values are ground truth read off the
// bench saves.
func TestCELiveMapFieldValues(t *testing.T) {
	// an assertion picks one field out of the parsed gametype and expects a value.
	type check struct {
		label string
		get   func(*CEGametype) uint32
		want  uint32
	}
	engine := func(v uint32) check { return check{"engine", func(g *CEGametype) uint32 { return g.Engine }, v} }
	cases := map[string][]check{
		"S0_baseline": {engine(2), {"options", func(g *CEGametype) uint32 { return g.Options }, 0x03}},
		"E2_lives5":   {engine(2), {"lives", func(g *CEGametype) uint32 { return g.Lives }, 5}},
		"E5_respawn5": {engine(2), {"respawnTime", func(g *CEGametype) uint32 { return g.RespawnTime }, 150}},
		"E6_growth5":  {engine(2), {"respawnGrowth", func(g *CEGametype) uint32 { return g.RespawnGrowth }, 150}},
		"E4_shieldsNO": {engine(2),
			{"shields-off bit", func(g *CEGametype) uint32 { return g.Options & ceOptShieldsOff }, ceOptShieldsOff}},
		"E7_oddman":  {engine(2), {"oddManOut", func(g *CEGametype) uint32 { return g.OddManOut }, 1}},
		"A1_objNONE": {engine(2), {"objectivesIndicator", func(g *CEGametype) uint32 { return g.ObjectivesIndicator }, 2}},
		"E14_startGeneric": {engine(2),
			{"generic-equip bit", func(g *CEGametype) uint32 { return g.Options & ceOptGenericEquip }, ceOptGenericEquip}},
		"E15_deathbonusNO": {engine(2),
			{"death-bonus-off bit", func(g *CEGametype) uint32 { return g.EngineUnion & ceEUSlayerDeathBonusOff }, ceEUSlayerDeathBonusOff}},
		"O1_traitwith3":    {engine(3), {"oddTraitWith", func(g *CEGametype) uint32 { return g.OddballTraitWith }, 3}},
		"O2_traitwithout2": {engine(3), {"oddTraitWithout", func(g *CEGametype) uint32 { return g.OddballTraitWithout }, 2}},
		"O3_speedFAST":     {engine(3), {"oddSpeed", func(g *CEGametype) uint32 { return g.OddballSpeed }, 2}},
		"O4_balltypeRT":    {engine(3), {"oddBallType", func(g *CEGametype) uint32 { return g.OddballBallType }, 1}},
		"O6_spawn3":        {engine(3), {"ballSpawn", func(g *CEGametype) uint32 { return g.BallSpawnCount }, 3}},
		"C2_singleflag1m":  {engine(1), {"ctfSingleFlag", func(g *CEGametype) uint32 { return g.CTFSingleFlag }, 1800}},
		"C4_flaghome": {engine(1),
			{"flag-at-home bit", func(g *CEGametype) uint32 { return g.EngineUnion & ceEUCTFFlagAtHome }, ceEUCTFFlagAtHome}},
		"P1_teamscoreMAX": {engine(5), {"raceScoring", func(g *CEGametype) uint32 { return g.RaceScoring }, 1}},
	}
	for name, checks := range cases {
		name, checks := name, checks
		t.Run(name, func(t *testing.T) {
			b := readFile(t, filepath.Join("testdata/ce_livemap", name+".blam.lst"))
			g, err := CEParse(b)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			for _, c := range checks {
				if got := c.get(g); got != c.want {
					t.Errorf("%s = %d (%#x), want %d (%#x)", c.label, got, got, c.want, c.want)
				}
			}
		})
	}

	// float-encoded field checked separately.
	t.Run("E3_health150", func(t *testing.T) {
		g, err := CEParse(readFile(t, "testdata/ce_livemap/E3_health150.blam.lst"))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if g.MaxHealth != 1.5 {
			t.Errorf("maxHealth = %v, want 1.5", g.MaxHealth)
		}
	})
}

// TestCELiveMapSignatureAndRebuild proves, over ALL differential fixtures, that
// (a) each carries a valid roamable signature and (b) a template-patch rebuild
// (empty patch, preserved digest) is byte-identical — i.e. parse+build fully
// round-trip on genuine editor-produced saves across all five engines.
func TestCELiveMapSignatureAndRebuild(t *testing.T) {
	files, err := filepath.Glob("testdata/ce_livemap/*.blam.lst")
	if err != nil || len(files) == 0 {
		t.Fatalf("no ce_livemap fixtures: %v", err)
	}
	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			b := readFile(t, f)
			// signature valid
			dg, err := RecomputeDigest(b, ceOffDigest, ceDigestLen, "ce")
			if err != nil {
				t.Fatalf("digest: %v", err)
			}
			if !bytes.Equal(dg, b[ceOffDigest:ceOffDigest+ceDigestLen]) {
				t.Fatalf("stored signature invalid")
			}
			// rebuild byte-identical (empty patch, preserve digest)
			rebuilt, err := CEBuild(b, CEPatch{}, false)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if !bytes.Equal(rebuilt, b) {
				t.Fatalf("rebuild not byte-identical, first diff @%d", firstDiff(rebuilt, b))
			}
			// re-sign path must reproduce the same (valid) file too
			resigned, err := CEBuild(b, CEPatch{}, true)
			if err != nil {
				t.Fatalf("re-sign build: %v", err)
			}
			if !bytes.Equal(resigned, b) {
				t.Fatalf("re-signed empty-patch differs from original @%d", firstDiff(resigned, b))
			}
		})
	}
}

// TestCEFriendlyBuild proves the friendly BuildRequest → raw-byte conversion:
// seconds×30 timers, option/engine_union bit toggles, and CTF single-flag
// minutes×1800 all land at the right offsets with the right encoding, and the
// result is correctly signed + round-trips.
func TestCEFriendlyBuild(t *testing.T) {
	tr := true
	respawn := 5.0
	singleFlag := 1.0
	set, err := Build(BuildRequest{
		Title: TitleCE, Kind: KindGametype, Name: "PROOF", Engine: "ctf",
		RespawnSeconds:       &respawn,    // -> 0x30 = 150
		ShieldsOff:           &tr,         // -> options |= 0x08
		Assault:              &tr,         // -> engine_union |= 0x01
		FlagAtHome:           &tr,         // -> engine_union |= 0x01000000
		CTFSingleFlagMinutes: &singleFlag, // -> 0x50 = 1800
	})
	if err != nil {
		t.Fatal(err)
	}
	g := set.Parsed.(*CEGametype)
	if g.RespawnTime != 150 {
		t.Errorf("respawnTime = %d, want 150", g.RespawnTime)
	}
	if g.Options&ceOptShieldsOff == 0 {
		t.Errorf("shields-off bit not set (options=%#x)", g.Options)
	}
	if g.EngineUnion&ceEUCTFAssault == 0 || g.EngineUnion&ceEUCTFFlagAtHome == 0 {
		t.Errorf("engine_union bits wrong (%#x)", g.EngineUnion)
	}
	if g.CTFSingleFlag != 1800 {
		t.Errorf("ctf single flag = %d, want 1800", g.CTFSingleFlag)
	}
	// signed + round-trips
	if !set.Digest.Resolved {
		t.Errorf("generated CTF gametype not signed")
	}
	dg, _ := RecomputeDigest(g.Raw, ceOffDigest, ceDigestLen, "ce")
	if !bytes.Equal(dg, g.Digest) {
		t.Errorf("generated file not correctly self-signed")
	}
}
