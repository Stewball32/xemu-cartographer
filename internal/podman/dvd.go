package podman

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// isISOFile reports whether a directory entry name looks like a game disc image
// the library should surface. Case-insensitive on the common Xbox disc
// extensions; used only to keep the library listing tidy — resolveGameISO still
// accepts any existing file, so an unusual extension can still be catalogued by
// bare filename.
func isISOFile(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".iso", ".xiso", ".cci", ".cso"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// ISOLibrary lists the bare filenames of the disc images present in the shared
// ISO library directory, sorted. A missing directory is not an error — it
// returns an empty list (the library simply hasn't been populated yet). These
// are the names that may be catalogued in the `isos` collection and passed as
// CreateOptions.GameISO.
func (m *Manager) ISOLibrary() ([]string, error) {
	dir := m.isoDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read ISO library %q: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if isISOFile(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// ISOExists reports whether a bare filename resolves to an existing regular file
// in the shared ISO library. Used by the admin catalog route to reject entries
// that point at a missing disc before they're saved. A name with a path
// separator is rejected (never allowed to escape the library dir).
func (m *Manager) ISOExists(filename string) bool {
	if filename == "" || strings.ContainsAny(filename, `/\`) {
		return false
	}
	fi, err := os.Stat(filepath.Join(m.isoDir(), filename))
	return err == nil && !fi.IsDir()
}

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
