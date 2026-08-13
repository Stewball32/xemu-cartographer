package podman

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Master-image DVD auto-launch — OPTIONAL lever (empirically NOT required).
//
// ⚠️ Live testing (2026-07-10, ADR-0004) showed a game disc present at COLD BOOT
// is already auto-launched by the master image's Cerbios/Xbox boot path,
// REGARDLESS of UnleashX's <DVD AutoLaunch> setting: on the unmodified root,
// Halo 2.iso booted Halo 2, Halo CE.iso booted Halo, and no disc booted the
// UnleashX dashboard. So attaching an ISO (DVDPath / CreateOptions.GameISO)
// alone makes an instance boot straight into that game — this helper is NOT
// needed for the primary flow and is deliberately NOT wired into Create.
//
// It remains as a documented, tested lever for a future image that DOES sit on
// the dashboard with a disc (this project has seen image drift), or to force
// disc auto-launch for a disc INSERTED while the dashboard is up. It performs a
// one-time edit of the shared read-only ROOT qcow2 (_default.qcow2), NOT a
// per-instance change, flipping UnleashX's <DVD AutoLaunch> to "Yes".
//
// Boot chain on this master image: Cerbios (flashrom) boots DashPath1 =
// E:\Dashboard\default.xbe (UnleashX). Setting <DVD AutoLaunch="Yes"> makes
// UnleashX also auto-run the disc in the tray: for an Xbox game disc it launches
// D:\default.xbe (the game); for a non-game disc it falls back to the handler
// path in the element (harmless — we only ever attach game XISOs). The
// HDD-installed Halo + its "Play Halo" menu entry are left intact as a fallback.
//
// The write reuses the exact rootless mechanism console_name.go uses:
// qemu-storage-daemon's FUSE export of the qcow2 (writable) + pyfatx (here the
// generic fatx_file.py helper) to read/patch/write the config file on the FATX
// E: partition. All the config knowledge (which element, idempotency) lives in
// Go (patchDVDAutoLaunch, unit-tested); the python helper just moves bytes.
//
// ⚠️ Editing the root invalidates any overlay already backed by it (qcow2
// backing-chain rule). Run this ONCE on a fresh master before any instances are
// created, or delete + recreate all instances afterward.
const (
	// masterDashDrive is the FATX drive letter holding the UnleashX dashboard.
	masterDashDrive = "e"
	// masterUnleashXConfig is the UnleashX config path on masterDashDrive
	// (E:\Dashboard\config.xml — the dir Cerbios boots via DashPath1).
	masterUnleashXConfig = "/Dashboard/config.xml"
)

// dvdAutoLaunchRe matches the UnleashX <DVD AutoLaunch="..."> preference's value.
// It is specific to the DVD element — <Games AutoLaunch>, <AudioCD AutoLaunch>
// and <Data AutoLaunch> siblings do not match, so they are left untouched.
var dvdAutoLaunchRe = regexp.MustCompile(`<DVD\s+AutoLaunch="([^"]*)"`)

// patchDVDAutoLaunch flips UnleashX's <DVD AutoLaunch="No"> to "Yes" in the given
// config.xml, returning the patched bytes and whether a change was made
// (idempotent — already-"Yes" returns changed=false). Only the DVD element's
// value is touched; the rest of the file is byte-preserved. Errors if no
// <DVD AutoLaunch="..."> element is present.
func patchDVDAutoLaunch(xml []byte) (out []byte, changed bool, err error) {
	m := dvdAutoLaunchRe.FindSubmatchIndex(xml)
	if m == nil {
		return nil, false, fmt.Errorf(`no <DVD AutoLaunch="..."> element found in UnleashX config`)
	}
	valStart, valEnd := m[2], m[3]
	if strings.EqualFold(string(xml[valStart:valEnd]), "Yes") {
		return xml, false, nil
	}
	out = make([]byte, 0, len(xml)+1)
	out = append(out, xml[:valStart]...)
	out = append(out, "Yes"...)
	out = append(out, xml[valEnd:]...)
	return out, true, nil
}

// SetMasterDVDAutoLaunch performs the one-time root edit described above,
// returning whether the config was changed (false = already enabled). It
// temporarily makes the root writable, exposes it via FUSE, reads/patches/writes
// the UnleashX config, then restores the root's read-only freeze regardless of
// outcome.
func (m *Manager) SetMasterDVDAutoLaunch() (changed bool, err error) {
	root := filepath.Join(m.hddsDir(), m.rootHDDName())
	fi, err := os.Stat(root)
	if err != nil {
		return false, fmt.Errorf("root image %s not found: %w", root, err)
	}
	origMode := fi.Mode().Perm()

	// qemu-storage-daemon must open the root read-write to expose a writable
	// FUSE view, so lift the read-only freeze for the duration. Re-freeze on the
	// way out (deferred first → runs last, after the FUSE export is torn down).
	defer func() {
		_ = os.Chmod(root, origMode)
		m.freezeRoot(root)
	}()
	if err := m.ensureWritable(root); err != nil {
		return false, err
	}

	raw, cleanup, err := m.exposeOverlayFUSE(root)
	if err != nil {
		return false, fmt.Errorf("expose root FUSE: %w", err)
	}
	defer cleanup()

	cur, err := m.fatxReadFile(raw, masterDashDrive, masterUnleashXConfig)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", masterUnleashXConfig, err)
	}
	patched, changed, err := patchDVDAutoLaunch(cur)
	if err != nil {
		return false, err
	}
	if !changed {
		log.Printf("podman: master DVD auto-launch already enabled (%s)", root)
		return false, nil
	}
	if err := m.fatxWriteFile(raw, masterDashDrive, masterUnleashXConfig, patched); err != nil {
		return false, fmt.Errorf("write %s: %w", masterUnleashXConfig, err)
	}
	log.Printf("podman: enabled DVD auto-launch in master root %s (UnleashX %s)", root, masterUnleashXConfig)
	return true, nil
}

// ensureWritable makes the root writable by its owner (0644) so
// qemu-storage-daemon can open it read-write. Mirrors freezeRoot's sudo fallback
// for a root that rootful podman left root-owned.
func (m *Manager) ensureWritable(root string) error {
	if err := os.Chmod(root, 0o644); err == nil {
		return nil
	}
	if out, serr := m.runSudo("chmod", "0644", root); serr != nil {
		return fmt.Errorf("make root %s writable: %v (%s)", root, serr, strings.TrimSpace(string(out)))
	}
	return nil
}

// fatxFileToolPath returns the generic pyfatx read/write helper, a sibling of
// the console-name tool in containers/xemu/tools/.
func (m *Manager) fatxFileToolPath() string {
	return abs(filepath.Join(filepath.Dir(m.fatxToolPath()), "fatx_file.py"))
}

// fatxReadFile reads a file from a FATX drive of the raw image via the pyfatx
// helper, returning its raw bytes.
func (m *Manager) fatxReadFile(raw, drive, path string) ([]byte, error) {
	tool := m.fatxFileToolPath()
	if _, err := os.Stat(tool); err != nil {
		return nil, fmt.Errorf("fatx file tool not found at %s: %w", tool, err)
	}
	cmd := exec.Command(m.pythonCmd(), tool, "read", raw, drive, path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("fatx read: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// fatxWriteFile writes data to a file on a FATX drive of the raw image via the
// pyfatx helper (base64-encoded on argv; created if absent, truncated to len).
func (m *Manager) fatxWriteFile(raw, drive, path string, data []byte) error {
	tool := m.fatxFileToolPath()
	if _, err := os.Stat(tool); err != nil {
		return fmt.Errorf("fatx file tool not found at %s: %w", tool, err)
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	cmd := exec.Command(m.pythonCmd(), tool, "write", raw, drive, path, b64)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("fatx write: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
