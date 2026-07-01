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
	color := uint32(2) // red
	thumb := byte(1)   // southpaw
	button := byte(0)  // default
	name := "CARTOG"
	payload, err := CEProfileBuild(CEProfilePatch{
		Name:       &name,
		Color:      &color,
		Thumbstick: &thumb,
		Button:     &button,
	}, true)
	if err != nil {
		t.Fatalf("CEProfileBuild: %v", err)
	}
	if len(payload) != cepSize {
		t.Fatalf("payload %d bytes, want %d", len(payload), cepSize)
	}
	p, err := CEProfileParse(payload)
	if err != nil {
		t.Fatalf("CEProfileParse: %v", err)
	}
	if p.Name != name {
		t.Errorf("name = %q, want %q", p.Name, name)
	}
	if p.Color != color {
		t.Errorf("color = %d, want %d", p.Color, color)
	}
	if p.Thumbstick != thumb || p.Button != button {
		t.Errorf("presets = %d/%d, want %d/%d", p.Thumbstick, p.Button, thumb, button)
	}
	if payload[cepOffAdv2A] != cepDefaultAdv2A {
		t.Errorf("0x2A = %#x, want fresh-MP default %#x", payload[cepOffAdv2A], cepDefaultAdv2A)
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

func TestBuildCEProfileViaDispatch(t *testing.T) {
	set, err := Build(BuildRequest{
		Title: TitleCE, Kind: KindProfile, Name: "CARTOG",
		Appearance: map[string]int{"color": 3, "thumbstick": 1, "button": 1},
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
