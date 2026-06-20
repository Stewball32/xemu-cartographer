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

func TestRemoveOverlayFileKeepsRoot(t *testing.T) {
	m, root := newOverlayTestManager(t)
	if err := m.provisionOverlay("gone"); err != nil {
		t.Fatal(err)
	}
	overlay := m.overlayPath("gone")

	m.removeOverlayFile("gone")
	if _, err := os.Stat(overlay); !os.IsNotExist(err) {
		t.Errorf("overlay still present after removeOverlayFile: err=%v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Errorf("root must survive overlay removal: %v", err)
	}

	// Removing a non-existent overlay is a quiet no-op.
	m.removeOverlayFile("never-existed")
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
