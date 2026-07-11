// Command master-autolaunch performs the ONE-TIME master-image edit that makes
// the shared root qcow2 (_default.qcow2) auto-launch an attached game DVD
// instead of sitting on the UnleashX dashboard. It flips UnleashX's
// <DVD AutoLaunch="No"> to "Yes" in E:\Dashboard\config.xml inside the root,
// using the rootless qemu-storage-daemon FUSE + pyfatx mechanism.
//
// Run it ONCE against a fresh master, BEFORE any instances are created (editing
// the root invalidates overlays already backed by it). It is idempotent: a
// second run is a no-op.
//
// Config comes from CONTAINERS_* env (podman.LoadFromEnv), with flag overrides
// for the few values that matter here. The FATX write needs a pyfatx-capable
// python — the stock python3 usually lacks it, so pass -python (or set
// CONTAINERS_PYTHON_CMD) to a venv that has `pip install pyfatx`.
//
// Example:
//
//	go run ./cmd/master-autolaunch \
//	  -shared ./containers/xemu/shared \
//	  -init   ./containers/xemu/init \
//	  -python /home/stew/xemu-hdd-extract/venv/bin/python3
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Stewball32/xemu-cartographer/internal/podman"
)

func main() {
	cfg := podman.LoadFromEnv()

	shared := flag.String("shared", cfg.SharedDir, "shared dir holding hdds/<root>")
	initDir := flag.String("init", cfg.InitDir, "xemu init dir (its ../tools holds fatx_file.py)")
	root := flag.String("root", cfg.RootHDD, "root qcow2 basename under <shared>/hdds")
	python := flag.String("python", cfg.PythonCmd, "python with pyfatx importable")
	qsd := flag.String("qsd", cfg.QemuStorageDaemonCmd, "qemu-storage-daemon command")
	podmanCmd := flag.String("podman", cfg.PodmanCmd, "podman command (for the sudo chmod fallback if the root is root-owned)")
	flag.Parse()

	cfg.SharedDir = *shared
	cfg.InitDir = *initDir
	cfg.RootHDD = *root
	cfg.PythonCmd = *python
	cfg.QemuStorageDaemonCmd = *qsd
	cfg.PodmanCmd = *podmanCmd

	mgr, err := podman.NewManager(cfg, noopStore{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "new manager: %v\n", err)
		os.Exit(1)
	}

	changed, err := mgr.SetMasterDVDAutoLaunch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "set master DVD auto-launch: %v\n", err)
		os.Exit(1)
	}
	if changed {
		fmt.Println("master DVD auto-launch ENABLED (UnleashX <DVD AutoLaunch=\"Yes\">)")
	} else {
		fmt.Println("master DVD auto-launch already enabled — no change")
	}
}

// noopStore satisfies podman.Store; SetMasterDVDAutoLaunch never touches it.
type noopStore struct{}

func (noopStore) LoadAll() (map[string]*podman.ContainerInfo, error) {
	return map[string]*podman.ContainerInfo{}, nil
}
func (noopStore) Upsert(*podman.ContainerInfo) error { return nil }
func (noopStore) Delete(string) error                { return nil }
