package podman

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// hddsDir is the absolute directory holding the canonical root qcow2 and every
// per-instance overlay (SharedDir/hdds). It is bind-mounted into each container
// at /shared/hdds.
func (m *Manager) hddsDir() string {
	return filepath.Join(abs(m.cfg.SharedDir), "hdds")
}

// rootHDDName returns the configured canonical root image basename.
func (m *Manager) rootHDDName() string {
	if m.cfg.RootHDD != "" {
		return m.cfg.RootHDD
	}
	return "_default.qcow2"
}

// overlayPath returns the absolute path of a per-instance overlay qcow2.
func (m *Manager) overlayPath(name string) string {
	return filepath.Join(m.hddsDir(), name+".qcow2")
}

// qemuImgCmd returns the configured qemu-img binary (default "qemu-img").
func (m *Manager) qemuImgCmd() string {
	if m.cfg.QemuImgCmd != "" {
		return m.cfg.QemuImgCmd
	}
	return "qemu-img"
}

// provisionOverlay creates a thin copy-on-write overlay for `name`, backed by
// the canonical read-only root image, and freezes the root read-only.
//
// Why overlays instead of a full copy: the previous flow `cp`'d the entire
// multi-GiB root per instance (slow, disk-heavy — N instances meant N×8 GiB). An
// overlay records ONLY that instance's deltas; the shared root is read at open
// time through qcow2's backing chain. xemu is a qemu fork using qemu's stock
// qcow2 block driver, so it honors the chain and opens the backing file
// read-only — N instances safely share one root (verified: concurrent overlays
// over one root, root stays byte-identical).
//
// Backing path is stored RELATIVE (the bare root basename) on purpose. The root
// and every overlay live in the same hdds directory, which is bind-mounted at a
// DIFFERENT absolute path inside the container (/shared/hdds) than on the host
// (./containers/xemu/shared/hdds). A relative backing resolves correctly in both
// contexts and survives the instances dir being relocated, as long as the root
// stays alongside its overlays. An absolute host path would break inside the
// container and break if the dir moved.
//
// CAVEAT (enforced by freezeRoot): the root must never be modified while any
// overlay depends on it — editing a backing file corrupts every overlay above
// it. If you need to update the root, delete every overlay first, replace the
// root, then recreate the instances.
func (m *Manager) provisionOverlay(name string) error {
	hdds := m.hddsDir()
	rootName := m.rootHDDName()
	root := filepath.Join(hdds, rootName)
	overlay := m.overlayPath(name)

	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("canonical root image %s not found — drop the Halo-installed baseline there before creating instances: %w", root, err)
	}

	// Freeze the root read-only so no host-side edit can corrupt the overlays
	// that depend on it. qemu already opens backing files read-only; this makes
	// accidental edits fail loudly AND ensures every overlay reader (whatever uid
	// xemu runs as) can read the shared backing.
	m.freezeRoot(root)

	// Idempotent: reuse an existing overlay (e.g. a re-Create after a crash)
	// instead of discarding its deltas.
	if _, err := os.Stat(overlay); err == nil {
		log.Printf("podman: hdd overlay %s already exists, reusing", overlay)
		return nil
	}

	// qemu-img create -f qcow2 -b <root-basename> -F qcow2 <name>.qcow2, run with
	// cwd = hdds so the stored backing string is the bare relative basename.
	cmd := exec.Command(m.qemuImgCmd(), "create", "-f", "qcow2",
		"-b", rootName, "-F", "qcow2", name+".qcow2")
	cmd.Dir = hdds
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img create overlay for %q: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	log.Printf("podman: created CoW hdd overlay %s (backing %s)", overlay, rootName)
	return nil
}

// freezeRoot chmods the canonical root image read-only (0444). Best-effort: a
// failure (e.g. the root is owned by another uid) is logged but does not abort
// provisioning, since qemu opens backing files read-only regardless.
func (m *Manager) freezeRoot(root string) {
	fi, err := os.Stat(root)
	if err != nil {
		return
	}
	if fi.Mode().Perm() == 0o444 {
		return // already frozen
	}
	if err := os.Chmod(root, 0o444); err != nil {
		// rootful podman may have left the root owned by root; retry via the
		// same privilege prefix used for podman (no-op prefix when rootless).
		if out, serr := m.runSudo("chmod", "0444", root); serr != nil {
			log.Printf("podman: warning: could not freeze root image %s read-only: %v (%s); it MUST stay unmodified while overlays depend on it", root, err, strings.TrimSpace(string(out)))
			return
		}
	}
	log.Printf("podman: froze root image %s read-only (0444)", root)
}

// removeOverlayFile deletes a per-instance overlay qcow2 during teardown.
// Best-effort + sudo-aware (rootful podman stamps the file root-owned). Never
// touches the shared root.
func (m *Manager) removeOverlayFile(name string) {
	overlay := m.overlayPath(name)
	if _, err := os.Stat(overlay); err != nil {
		return // nothing to remove
	}
	if err := os.Remove(overlay); err != nil {
		if out, serr := m.runSudo("rm", "-f", overlay); serr != nil {
			log.Printf("podman: warning: failed to remove overlay %s: %v (%s)", overlay, err, strings.TrimSpace(string(out)))
			return
		}
	}
	log.Printf("podman: removed hdd overlay %s", overlay)
}
