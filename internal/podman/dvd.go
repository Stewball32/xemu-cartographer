package podman

import (
	"fmt"
	"os"
	"path/filepath"
)

// isoDir returns the shared host directory holding the game ISO library
// (Config.ISODir, default SharedDir/isos). A per-instance ISO named (not an
// absolute path) in CreateOptions.GameISO resolves against this dir.
func (m *Manager) isoDir() string {
	if m.cfg.ISODir != "" {
		return abs(m.cfg.ISODir)
	}
	return abs(filepath.Join(m.cfg.SharedDir, "isos"))
}

// ISODir exposes the resolved shared ISO library directory (absolute) so
// callers (the admin catalog route) can report where library files live.
func (m *Manager) ISODir() string { return m.isoDir() }

// resolveGameISO resolves an instance's requested game ISO to an absolute host
// path, or "" for no disc. Precedence:
//
//   - requested != "": a per-instance ISO. An absolute path is used as-is; a
//     bare name / relative path resolves against ISODir (the shared library).
//   - requested == "": falls back to Config.DVDPath (the global default disc).
//
// A non-empty result that doesn't exist (or is a directory) is an error — we
// fail Create rather than provision an instance whose disc silently won't
// attach. The returned path is bind-mounted read-only at containerDVDPath; the
// game is never copied onto the per-instance overlay.
func (m *Manager) resolveGameISO(requested string) (string, error) {
	path := requested
	if path == "" {
		path = m.cfg.DVDPath
	}
	if path == "" {
		return "", nil // no disc — HDD-only boot
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.isoDir(), path)
	}
	path = abs(path)

	fi, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("game ISO %q not found: %w", path, err)
	}
	if fi.IsDir() {
		return "", fmt.Errorf("game ISO %q is a directory, not a file", path)
	}
	return path, nil
}
