package podman

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/consolename"
)

// Console-name (E:\UDATA\NICKNAME.XBN) format now lives in the shared leaf
// package internal/consolename, so the gamertag identity system can reuse the
// exact same builder. These thin wrappers keep the podman call sites + tests
// stable.
const (
	nicknameFileSize = consolename.FileSize
	nicknameNameMax  = consolename.NameMax
)

func sanitizeConsoleName(name string) string { return consolename.Sanitize(name) }
func buildNicknameXBN(name string) []byte    { return consolename.BuildXBN(name) }

func (m *Manager) consoleNamingEnabled() bool { return m.cfg.SetConsoleName }

// consoleNameFor picks the name stamped as the Xbox console nickname: the pretty
// canonical DISPLAY name when set, else the (slugified) container name — the
// pre-decoupling fallback for legacy/unnamed instances. Sanitize (≤15,
// printable ASCII) is applied downstream by the writer regardless.
func consoleNameFor(container, display string) string {
	if display != "" {
		return display
	}
	return container
}

func (m *Manager) qemuStorageDaemonCmd() string {
	if m.cfg.QemuStorageDaemonCmd != "" {
		return m.cfg.QemuStorageDaemonCmd
	}
	return "qemu-storage-daemon"
}

func (m *Manager) pythonCmd() string {
	if m.cfg.PythonCmd != "" {
		return m.cfg.PythonCmd
	}
	return "python3"
}

// fatxToolPath returns the path to the pyfatx console-name helper script.
// Defaults to <InitDir>/../tools/fatx_console_name.py (i.e. containers/xemu/tools/).
func (m *Manager) fatxToolPath() string {
	if m.cfg.FatxToolPath != "" {
		return abs(m.cfg.FatxToolPath)
	}
	return abs(filepath.Join(filepath.Dir(m.cfg.InitDir), "tools", "fatx_console_name.py"))
}

// writeConsoleName writes the container name into the instance's Xbox console
// name (E:\UDATA\NICKNAME.XBN) inside its overlay, BEFORE first boot.
//
// The write goes through qemu-storage-daemon's FUSE export of the overlay's
// MERGED view (rootless; no /dev/nbd or kernel module — fits a server that may
// run unprivileged), so it lands in the overlay's copy-on-write layer and the
// shared read-only root is never modified. We deliberately do NOT round-trip the
// qcow2 through raw (qemu-img convert) — that would flatten the backing chain.
// pyfatx then creates NICKNAME.XBN in the existing E:\UDATA dir (the root leaves
// it absent on purpose, which is what makes unnamed instances get a random
// name), or overwrites it if present.
//
// Best-effort: returns an error (logged by the caller) if the tooling is
// missing or the write fails; the instance still boots with the Xbox's random
// default name.
func (m *Manager) writeConsoleName(overlay, rawName string) error {
	name := sanitizeConsoleName(rawName)
	if name == "" {
		return nil // nothing usable — leave the Xbox random-name default
	}
	payload := buildNicknameXBN(name)

	raw, cleanup, err := m.exposeOverlayFUSE(overlay)
	if err != nil {
		return fmt.Errorf("expose overlay for console-name write: %w", err)
	}
	defer cleanup()

	if err := m.runFatxConsoleName(raw, payload); err != nil {
		return err
	}
	log.Printf("podman: set console name %q in overlay %s", name, overlay)
	return nil
}

// exposeOverlayFUSE exposes the overlay's merged view as a writable raw file via
// qemu-storage-daemon's FUSE export. Returns the raw mountpoint path and a
// cleanup func that unmounts + stops the daemon + removes the temp dir. Writes
// to the raw path are committed to the overlay's CoW layer.
func (m *Manager) exposeOverlayFUSE(overlay string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "cart-fatx-")
	if err != nil {
		return "", nil, err
	}
	raw := filepath.Join(dir, "disk.raw")
	if f, err := os.Create(raw); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	} else {
		_ = f.Close()
	}

	blockdev := fmt.Sprintf("driver=qcow2,node-name=cartdisk,file.driver=file,file.filename=%s", abs(overlay))
	export := fmt.Sprintf("type=fuse,id=cartexp,node-name=cartdisk,mountpoint=%s,writable=on,allow-other=off", raw)
	cmd := exec.Command(m.qemuStorageDaemonCmd(), "--blockdev", blockdev, "--export", export)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("start %s: %w", m.qemuStorageDaemonCmd(), err)
	}

	cleanup := func() {
		_ = exec.Command("fusermount3", "-u", raw).Run()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		_ = os.RemoveAll(dir)
	}

	// Ready when the FUSE mountpoint reports the (large) virtual disk size.
	for i := 0; i < 100; i++ {
		if fi, err := os.Stat(raw); err == nil && fi.Size() > 0 {
			return raw, cleanup, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	cleanup()
	return "", nil, fmt.Errorf("qemu-storage-daemon FUSE export not ready: %s", strings.TrimSpace(stderr.String()))
}

// runFatxConsoleName invokes the pyfatx helper to write the NICKNAME.XBN payload
// (base64-encoded on argv) onto the E: partition of the raw image/device.
func (m *Manager) runFatxConsoleName(raw string, payload []byte) error {
	tool := m.fatxToolPath()
	if _, err := os.Stat(tool); err != nil {
		return fmt.Errorf("fatx console-name tool not found at %s: %w", tool, err)
	}
	b64 := base64.StdEncoding.EncodeToString(payload)
	cmd := exec.Command(m.pythonCmd(), tool, raw, b64)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("fatx console-name write: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
