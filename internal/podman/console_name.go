package podman

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Console-name (E:\UDATA\NICKNAME.XBN) layout — see the offset-mapper
// SYSTEM-INFO.md §7. Plaintext, no checksum: a 4-byte header (04 00 'S' 'M')
// then the name as UTF-16LE, NUL-terminated, zero-padded to the fixed file size.
const (
	nicknameFileSize = 3400 // total NICKNAME.XBN size in bytes
	nicknameNameMax  = 15   // console-name length cap (chars); conservative
)

// sanitizeConsoleName reduces an arbitrary container name to a safe Xbox console
// name: printable ASCII only (UTF-16-encodable without surrogates, system-link
// friendly), trimmed, truncated to nicknameNameMax. Returns "" when nothing
// usable remains — the caller then leaves the Xbox's random-name default.
func sanitizeConsoleName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 0x20 && r < 0x7f { // printable ASCII
			b.WriteRune(r)
		}
	}
	s := strings.TrimSpace(b.String())
	if len(s) > nicknameNameMax {
		s = strings.TrimSpace(s[:nicknameNameMax])
	}
	return s
}

// buildNicknameXBN renders the full fixed-size NICKNAME.XBN payload for an
// already-sanitized (printable-ASCII) name. The header is 04 00 'S' 'M'; the
// name follows at +0x04 as UTF-16LE; the NUL terminator + trailing padding are
// the zeroed remainder of the buffer.
func buildNicknameXBN(name string) []byte {
	buf := make([]byte, nicknameFileSize)
	buf[0], buf[1], buf[2], buf[3] = 0x04, 0x00, 'S', 'M'
	off := 4
	for _, r := range name { // sanitized => each rune is < 0x80
		binary.LittleEndian.PutUint16(buf[off:], uint16(r))
		off += 2
	}
	return buf
}

func (m *Manager) consoleNamingEnabled() bool { return m.cfg.SetConsoleName }

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
