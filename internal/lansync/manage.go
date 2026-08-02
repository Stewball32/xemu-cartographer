package lansync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Managed-library file mechanics for the ISO ingest model. The catalog row's
// disc lives at <ISODir>/<record-id>.iso — canonical, ID-anchored, decoupled
// from the display name. These are pure filesystem/exec helpers (no PocketBase);
// the orchestration (create row → move → hash → freeze → extract, dedupe, drift)
// lives in internal/isoingest.

// ManagedISOPath is the on-disk path of a catalog row's managed disc.
func (c Config) ManagedISOPath(id string) string {
	return filepath.Join(c.ISODir, id+".iso")
}

// ManagedISOName is the bare basename (<id>.iso) — what podman's resolveGameISO
// resolves against ISODir when booting an instance.
func ManagedISOName(id string) string { return id + ".iso" }

// EnsureDirs creates the inbox + library + extract dirs if missing (idempotent),
// so a fresh tier boots without a manual mkdir.
func (c Config) EnsureDirs() error {
	for _, d := range []string{c.InboxDir, c.ISODir, filepath.Join(c.ExtractDir, "isos")} {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

// HashFile streams path through SHA-256 and returns the lowercase hex digest.
// Reads the whole file once — the ingest cost paid alongside extraction.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// StatSig returns a managed file's size (bytes) + mtime (unix seconds) — the
// cheap drift pre-check compared before deciding to re-hash.
func StatSig(path string) (size int64, mtime int64, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, 0, err
	}
	return fi.Size(), fi.ModTime().Unix(), nil
}

// AtomicMove renames src → dst. Both must be on the same filesystem (inbox and
// library are, by design), so this is atomic — the managed file appears whole
// or not at all, never half-written.
func AtomicMove(src, dst string) error {
	return os.Rename(src, dst)
}

// FreezeFile makes a managed disc read-only: chmod 0444 always (works
// unprivileged), then best-effort chattr +i (needs CAP_LINUX_IMMUTABLE / root).
// Returns whether the immutable bit engaged; a false with nil err just means the
// process lacked the privilege (beta until run-as-root lands) — chmod still
// protects against accidental writes. chattr failures are non-fatal.
func FreezeFile(path string) (immutable bool, err error) {
	if err := os.Chmod(path, 0o444); err != nil {
		return false, fmt.Errorf("chmod 0444 %s: %w", path, err)
	}
	if _, lookErr := exec.LookPath("chattr"); lookErr != nil {
		return false, nil // chattr unavailable — chmod is the whole story
	}
	if out, runErr := exec.Command("chattr", "+i", path).CombinedOutput(); runErr != nil {
		// Non-root / unsupported fs → expected; caller logs, doesn't fail.
		_ = out
		return false, nil
	}
	return true, nil
}

// UnfreezeFile reverses FreezeFile so a managed disc can be deleted/replaced:
// best-effort chattr -i (ignored if it wasn't set / no privilege), then chmod
// 0644. Both are best-effort — a plain 0444 file is still unlink-able by its
// owner (unlink checks the directory, not the file mode).
func UnfreezeFile(path string) {
	if _, lookErr := exec.LookPath("chattr"); lookErr == nil {
		_ = exec.Command("chattr", "-i", path).Run()
	}
	_ = os.Chmod(path, 0o644)
}
