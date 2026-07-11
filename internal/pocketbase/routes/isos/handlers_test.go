package isos

import "testing"

// TestValidFilename covers the bare-filename guard shared by create + update:
// the catalog may only reference a plain filename inside the shared ISO library,
// never a path that could escape it.
func TestValidFilename(t *testing.T) {
	cases := map[string]bool{
		"halo-ce.iso":   true,
		"Halo 2.iso":    true,
		"":              false,
		"..":            false,
		".hidden.iso":   false,
		"sub/halo.iso":  false,
		"a\\b.iso":      false,
		"../escape.iso": false,
		"/abs/path.iso": false,
	}
	for in, want := range cases {
		if got := validFilename(in); got != want {
			t.Errorf("validFilename(%q) = %v, want %v", in, got, want)
		}
	}
}
