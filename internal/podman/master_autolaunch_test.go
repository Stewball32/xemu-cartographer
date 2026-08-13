package podman

import (
	"strings"
	"testing"
)

func TestPatchDVDAutoLaunch(t *testing.T) {
	// A minimal slice of the real UnleashX config.xml Preference block, with
	// sibling AutoLaunch attributes that must NOT be touched.
	base := `<Preference>
		<Games AutoLaunch="No"></Games>
		<DVD AutoLaunch="No">E:\Apps\XBMC\default.xbe</DVD>
		<AudioCD AutoLaunch="No">C:\xboxdash.xbe</AudioCD>
		<Data AutoLaunch="No"></Data>
	</Preference>`

	out, changed, err := patchDVDAutoLaunch([]byte(base))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on a No->Yes flip")
	}
	got := string(out)
	if !strings.Contains(got, `<DVD AutoLaunch="Yes">E:\Apps\XBMC\default.xbe</DVD>`) {
		t.Errorf("DVD AutoLaunch not set to Yes:\n%s", got)
	}
	// Siblings untouched.
	for _, sib := range []string{
		`<Games AutoLaunch="No">`,
		`<AudioCD AutoLaunch="No">`,
		`<Data AutoLaunch="No">`,
	} {
		if !strings.Contains(got, sib) {
			t.Errorf("sibling %q was modified:\n%s", sib, got)
		}
	}
	// The DVD element's handler path must be preserved verbatim.
	if !strings.Contains(got, `E:\Apps\XBMC\default.xbe`) {
		t.Errorf("DVD handler path lost:\n%s", got)
	}

	// Idempotent: re-running on the patched output is a no-op.
	out2, changed2, err := patchDVDAutoLaunch(out)
	if err != nil {
		t.Fatalf("unexpected error on second pass: %v", err)
	}
	if changed2 {
		t.Error("expected changed=false when already Yes")
	}
	if string(out2) != string(out) {
		t.Error("idempotent pass should return identical bytes")
	}
}

func TestPatchDVDAutoLaunchAlreadyYes(t *testing.T) {
	in := `<DVD AutoLaunch="Yes">E:\Apps\XBMC\default.xbe</DVD>`
	out, changed, err := patchDVDAutoLaunch([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("already Yes should report no change")
	}
	if string(out) != in {
		t.Error("bytes should be unchanged")
	}
}

func TestPatchDVDAutoLaunchMissing(t *testing.T) {
	if _, _, err := patchDVDAutoLaunch([]byte(`<Preference><Games AutoLaunch="No"/></Preference>`)); err == nil {
		t.Error("expected an error when no <DVD AutoLaunch> element is present")
	}
}
