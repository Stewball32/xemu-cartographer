package podman

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveGameISO covers the precedence + validation of the per-instance disc
// resolution: per-instance name (ISODir-relative), absolute path, global
// DVDPath fallback, empty (no disc), and the fail-fast on a missing file.
func TestResolveGameISO(t *testing.T) {
	dir := t.TempDir()
	isoDir := filepath.Join(dir, "isos")
	if err := os.MkdirAll(isoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	libISO := filepath.Join(isoDir, "halo-ce.iso")
	if err := os.WriteFile(libISO, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	absISO := filepath.Join(dir, "elsewhere.iso")
	if err := os.WriteFile(absISO, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	globalISO := filepath.Join(dir, "global.iso")
	if err := os.WriteFile(globalISO, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		cfg       Config
		requested string
		want      string
		wantErr   bool
	}{
		{"no disc", Config{ISODir: isoDir}, "", "", false},
		{"per-instance name resolves against ISODir", Config{ISODir: isoDir}, "halo-ce.iso", libISO, false},
		{"absolute path used as-is", Config{ISODir: isoDir}, absISO, absISO, false},
		{"falls back to global DVDPath", Config{ISODir: isoDir, DVDPath: globalISO}, "", globalISO, false},
		{"per-instance overrides global", Config{ISODir: isoDir, DVDPath: globalISO}, "halo-ce.iso", libISO, false},
		{"missing named ISO is an error", Config{ISODir: isoDir}, "nope.iso", "", true},
		{"missing absolute ISO is an error", Config{ISODir: isoDir}, filepath.Join(dir, "gone.iso"), "", true},
		{"directory is not a valid ISO", Config{ISODir: isoDir}, isoDir, "", true},
		{"default ISODir under SharedDir", Config{SharedDir: dir}, "halo-ce.iso", libISO, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &Manager{cfg: c.cfg}
			got, err := m.resolveGameISO(c.requested)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got path %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("resolveGameISO(%q) = %q, want %q", c.requested, got, c.want)
			}
		})
	}
}
