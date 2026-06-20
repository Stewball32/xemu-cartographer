package podman

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestWriteConsoleNameIntegration exercises the real writeConsoleName path
// (qemu-storage-daemon FUSE export of the overlay + pyfatx create of
// E:\UDATA\NICKNAME.XBN) against the actual repo _default.qcow2 root, and
// confirms the root stays byte-identical.
//
// Gated: only runs when CART_FATX_IT=1, since it needs qemu-storage-daemon +
// fusermount3 + a python with pyfatx and the real (multi-GiB, FATX-formatted)
// root. Point CART_TEST_PYTHON at a python that has pyfatx. CI does not run
// `go test`, so this never runs there.
//
//	CART_FATX_IT=1 CART_TEST_PYTHON=/path/to/venv/bin/python \
//	  go test ./internal/podman/ -run TestWriteConsoleNameIntegration -v
func TestWriteConsoleNameIntegration(t *testing.T) {
	if os.Getenv("CART_FATX_IT") != "1" {
		t.Skip("set CART_FATX_IT=1 (+ CART_TEST_PYTHON) to run the FATX console-name integration test")
	}
	python := os.Getenv("CART_TEST_PYTHON")
	if python == "" {
		python = "python3"
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(repo, "containers", "xemu", "shared")
	root := filepath.Join(shared, "hdds", "_default.qcow2")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("root image %s not present: %v", root, err)
	}

	m := &Manager{cfg: Config{
		SharedDir:            shared,
		InitDir:              filepath.Join(repo, "containers", "xemu", "init"),
		RootHDD:              "_default.qcow2",
		QemuImgCmd:           "qemu-img",
		PythonCmd:            python,
		QemuStorageDaemonCmd: "qemu-storage-daemon",
		PodmanCmd:            "podman",
	}}

	const inst = "itest-console"
	overlay := m.overlayPath(inst)
	if os.Getenv("CART_KEEP_OVERLAY") != "1" { // keep it to boot-test externally
		t.Cleanup(func() { _ = os.Remove(overlay) })
	}

	rootSHA0 := fileSHA(t, root)

	if err := m.provisionOverlay(inst); err != nil {
		t.Fatalf("provisionOverlay: %v", err)
	}
	if err := m.writeConsoleName(overlay, "carto-99"); err != nil {
		t.Fatalf("writeConsoleName: %v", err)
	}

	// Read NICKNAME.XBN back through a fresh FUSE export + pyfatx and compare to
	// the expected payload.
	raw, cleanup, err := m.exposeOverlayFUSE(overlay)
	if err != nil {
		t.Fatalf("re-expose: %v", err)
	}
	defer cleanup()
	readScript := `import sys,pyfatx
fs=pyfatx.Fatx(sys.argv[1],drive='e')
sys.stdout.buffer.write(fs.read('/UDATA/NICKNAME.XBN'))`
	out, err := exec.Command(python, "-c", readScript, raw).Output()
	if err != nil {
		t.Fatalf("pyfatx readback: %v", err)
	}
	want := buildNicknameXBN("carto-99")
	if len(out) < len(want) || string(out[:len(want)]) != string(want) {
		t.Errorf("NICKNAME.XBN content mismatch:\n got %x...\nwant %x...", out[:min(16, len(out))], want[:16])
	}

	if got := fileSHA(t, root); got != rootSHA0 {
		t.Errorf("root changed during console-name write: %s -> %s", rootSHA0, got)
	}
}

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
