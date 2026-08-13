package halosave

import (
	"bytes"
	"os"
	"testing"
)

// TestCEProfileSigningMatchesRealSample is the load-bearing check: a real,
// captured CE profile (P-LD50 II) must reproduce its stored 20-byte digest from
// HMAC-SHA1 over [0:0x30] under the CE per-title key. This proves the profile
// offset (0x30) + key are right — the same crack as the CE gametype, different
// offset.
func TestCEProfileSigningMatchesRealSample(t *testing.T) {
	b, err := os.ReadFile("testdata/ce/P-LD50 II/blam.sav")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	if len(b) != cepSize {
		t.Fatalf("sample is %d bytes, want %d", len(b), cepSize)
	}
	got, err := RecomputeDigest(b, cepOffDigest, cepDigestLen, "ce")
	if err != nil {
		t.Fatalf("RecomputeDigest: %v", err)
	}
	want := b[cepOffDigest : cepOffDigest+cepDigestLen]
	if !bytes.Equal(got, want) {
		t.Fatalf("CE profile digest mismatch:\n got  %x\n want %x", got, want)
	}
}

func TestCEProfileBuildAndParse(t *testing.T) {
	color := uint32(8) // purple
	button := byte(3)  // Boxer
	thumb := byte(2)   // Legacy
	name := "CARTOG"
	adv := CEAdvanced{
		HSens: 7, VMult: 0.75, Invert: true, Vibration: false,
		RSDeadzone: 10, LSDeadzone: 20, OuterDeadzone: 8,
		DeadzoneType: 0 /* radial */, Response: CEResponseSharp,
	}
	payload, err := CEProfileBuild(CEProfilePatch{
		Name:       &name,
		Color:      &color,
		Button:     &button,
		Thumbstick: &thumb,
		Advanced:   &adv,
	}, true)
	if err != nil {
		t.Fatalf("CEProfileBuild: %v", err)
	}
	if len(payload) != cepSize {
		t.Fatalf("payload %d bytes, want %d", len(payload), cepSize)
	}
	// Lock the 0x28=button / 0x29=thumbstick fix at the byte level.
	if payload[0x28] != button {
		t.Errorf("0x28 (button) = %#x, want %#x", payload[0x28], button)
	}
	if payload[0x29] != thumb {
		t.Errorf("0x29 (thumbstick) = %#x, want %#x", payload[0x29], thumb)
	}
	p, err := CEProfileParse(payload)
	if err != nil {
		t.Fatalf("CEProfileParse: %v", err)
	}
	if p.Name != name || p.Color != color {
		t.Errorf("name/color = %q/%d, want %q/%d", p.Name, p.Color, name, color)
	}
	if p.Button != button || p.Thumbstick != thumb {
		t.Errorf("button/thumb = %d/%d, want %d/%d", p.Button, p.Thumbstick, button, thumb)
	}
	// Advanced controls must round-trip exactly.
	if p.Advanced != adv {
		t.Errorf("advanced round-trip:\n got  %+v\n want %+v", p.Advanced, adv)
	}
	// The generated file must self-verify: digest == HMAC over [0:0x30].
	want, _ := RecomputeDigest(payload, cepOffDigest, cepDigestLen, "ce")
	if !bytes.Equal(p.Digest, want) {
		t.Errorf("generated profile is not correctly signed")
	}
	// Unsigned tail is zeroed (don't-care).
	for i := cepOffDigest + cepDigestLen; i < cepSize; i++ {
		if payload[i] != 0 {
			t.Errorf("non-zero tail byte at %#x", i)
			break
		}
	}
}

// TestCEProfileFactoryDefaults verifies a no-advanced build encodes the
// documented fresh-MP factory Advanced Controls and they decode back.
func TestCEProfileFactoryDefaults(t *testing.T) {
	name := "FRESH"
	payload, err := CEProfileBuild(CEProfilePatch{Name: &name}, true)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, _ := CEProfileParse(payload)
	def := ceAdvancedDefault()
	if p.Advanced != def {
		t.Errorf("factory advanced:\n got  %+v\n want %+v", p.Advanced, def)
	}
}

func TestBuildCEProfileViaDispatch(t *testing.T) {
	color, button, thumb := uint32(3), uint32(1), uint32(1)
	hsens := 5.0
	set, err := Build(BuildRequest{
		Title: TitleCE, Kind: KindProfile, Name: "CARTOG",
		Color: &color, Button: &button, Thumbstick: &thumb, HSens: &hsens,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if set.TitleID != TitleIDHaloCE {
		t.Errorf("title id = %q, want %q", set.TitleID, TitleIDHaloCE)
	}
	var hasBlam, hasMeta bool
	for _, f := range set.Files {
		if f.Name == "blam.sav" && f.Size == cepSize {
			hasBlam = true
		}
		if f.Name == "SaveMeta.xbx" {
			hasMeta = true
		}
	}
	if !hasBlam || !hasMeta {
		t.Fatalf("CE profile set missing blam.sav/SaveMeta.xbx: %+v", set.Files)
	}
	if !set.Digest.Resolved {
		t.Error("CE profile should be signed (digest resolved)")
	}
}
