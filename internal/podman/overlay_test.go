package podman

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newOverlayTestManager builds a Manager whose hdds dir contains a small fake
// root image. Skips the test when qemu-img is unavailable (CI runs go build +
// go vet, not go test, so this only matters locally).
func newOverlayTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("qemu-img not installed; skipping overlay test")
	}
	shared := t.TempDir()
	hdds := filepath.Join(shared, "hdds")
	if err := os.MkdirAll(hdds, 0o755); err != nil {
		t.Fatal(err)
	}
	// Small self-contained root image.
	root := filepath.Join(hdds, "_default.qcow2")
	if out, err := exec.Command("qemu-img", "create", "-f", "qcow2", root, "32M").CombinedOutput(); err != nil {
		t.Fatalf("create root: %v: %s", err, out)
	}
	if err := os.Chmod(root, 0o600); err != nil { // start writable to prove freeze
		t.Fatal(err)
	}
	m := &Manager{cfg: Config{
		SharedDir:  shared,
		RootHDD:    "_default.qcow2",
		QemuImgCmd: "qemu-img",
		PodmanCmd:  "podman",
	}}
	return m, root
}

func qemuImgBackingFilename(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command("qemu-img", "info", "--output=json", path).Output()
	if err != nil {
		t.Fatalf("qemu-img info: %v", err)
	}
	var info struct {
		BackingFilename string `json:"backing-filename"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatal(err)
	}
	return info.BackingFilename
}

func TestProvisionOverlay(t *testing.T) {
	m, root := newOverlayTestManager(t)

	if err := m.provisionOverlay("inst1"); err != nil {
		t.Fatalf("provisionOverlay: %v", err)
	}

	overlay := m.overlayPath("inst1")
	if _, err := os.Stat(overlay); err != nil {
		t.Fatalf("overlay not created: %v", err)
	}

	// Backing reference must be stored RELATIVE (bare basename), so it resolves
	// both on the host and at /shared/hdds inside the container.
	if got := qemuImgBackingFilename(t, overlay); got != "_default.qcow2" {
		t.Errorf("backing filename = %q, want relative %q", got, "_default.qcow2")
	}

	// Root must be frozen read-only.
	fi, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o444 {
		t.Errorf("root perms = %o, want 0444 (frozen)", perm)
	}

	// Idempotent: a second call reuses the overlay without error.
	if err := m.provisionOverlay("inst1"); err != nil {
		t.Fatalf("second provisionOverlay (reuse): %v", err)
	}
}

func TestProvisionOverlayConcurrentShareOneRoot(t *testing.T) {
	m, root := newOverlayTestManager(t)
	for _, name := range []string{"a", "b", "c"} {
		if err := m.provisionOverlay(name); err != nil {
			t.Fatalf("provisionOverlay(%s): %v", name, err)
		}
		if got := qemuImgBackingFilename(t, m.overlayPath(name)); got != "_default.qcow2" {
			t.Errorf("%s backing = %q, want _default.qcow2", name, got)
		}
	}
	// All three overlays exist and share the one root.
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("root vanished: %v", err)
	}
}

// TestRemoveContainerFilesKeepsShared is the symmetric-teardown contract: every
// per-instance file (overlay + config dir + browser profile) is removed, while
// the shared base (root qcow2, shared/bios firmware) is left untouched.
func TestRemoveContainerFilesKeepsShared(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("qemu-img not installed; skipping")
	}
	shared := t.TempDir()
	hdds := filepath.Join(shared, "hdds")
	if err := os.MkdirAll(hdds, 0o755); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(hdds, "_default.qcow2")
	if out, err := exec.Command("qemu-img", "create", "-f", "qcow2", root, "32M").CombinedOutput(); err != nil {
		t.Fatalf("create root: %v: %s", err, out)
	}
	// shared firmware that must survive teardown
	bios := filepath.Join(shared, "bios")
	mustMkdir(t, bios)
	firmware := filepath.Join(bios, "mcpx_1.0.bin")
	mustWrite(t, firmware, "rom")

	configs := filepath.Join(t.TempDir(), "configs")
	browser := filepath.Join(t.TempDir(), "browser")
	m := &Manager{cfg: Config{
		SharedDir: shared, ConfigsDir: configs, BrowserDir: browser,
		RootHDD: "_default.qcow2", QemuImgCmd: "qemu-img", PodmanCmd: "podman",
	}}

	// per-instance files: overlay + config dir (with a per-instance eeprom) + browser profile
	if err := m.provisionOverlay("inst"); err != nil {
		t.Fatal(err)
	}
	instCfg := filepath.Join(configs, "inst")
	mustMkdir(t, instCfg)
	mustWrite(t, filepath.Join(instCfg, "eeprom.bin"), "ee") // per-instance eeprom lives here
	instBrowser := filepath.Join(browser, "config-inst")
	mustMkdir(t, instBrowser)

	if err := m.removeContainerFiles("inst"); err != nil {
		t.Fatalf("removeContainerFiles: %v", err)
	}

	for _, p := range []string{m.overlayPath("inst"), instCfg, instBrowser} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("per-instance path should be removed: %s (err=%v)", p, err)
		}
	}
	for _, p := range []string{root, firmware} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("shared base must survive teardown: %s (%v)", p, err)
		}
	}

	// Removing files for an unknown container is a quiet no-op.
	if err := m.removeContainerFiles("never-existed"); err != nil {
		t.Errorf("removeContainerFiles(unknown) should be a no-op: %v", err)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProvisionOverlayMissingRoot(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("qemu-img not installed; skipping overlay test")
	}
	shared := t.TempDir()
	if err := os.MkdirAll(filepath.Join(shared, "hdds"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := &Manager{cfg: Config{SharedDir: shared, RootHDD: "_default.qcow2", PodmanCmd: "podman"}}
	if err := m.provisionOverlay("x"); err == nil {
		t.Fatal("expected error when root image is missing, got nil")
	}
}
