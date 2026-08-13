// Command podman-harness provisions a throwaway xemu+browser container pair
// using the repo's OWN provisioning (internal/podman Manager) — the same code
// path the server uses — without needing the full server or admin auth. It's
// for live confirmations (vncinput RFB, Firefox cert-fix boot, the host runner)
// and for tearing the instance down cleanly.
//
// Rootful podman is invoked via `sudo -n podman` inside the Manager's
// exec.Command calls (passwordless sudoers). A small JSON file store persists
// the ContainerInfo across invocations so up/down are separate commands.
//
// Subcommands:
//
//	raw <args...>       passthrough to `sudo -n podman <args>` (ps, images, logs, inspect, pull)
//	up   <name>         create (overlay+certs+containers) + start both containers
//	start <name>        start both containers
//	stop  <name>        stop both containers
//	down <name>         stop + remove both containers + delete overlay/config files
//	info <name>         print ports (BrowserWeb/websockify) + the host QMP socket path
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Stewball32/xemu-cartographer/internal/podman"
)

const (
	repo       = "/home/stew/repos/xemu-cartographer"
	podmanCmd  = "sudo -n podman --runtime=crun"
	statePath  = "/tmp/podman-harness-state.json"
	portBase   = 3600
	portStride = 10
)

func cfg() podman.Config {
	return podman.Config{
		Enabled:   true,
		SocketDir: repo + "/containers/xemu/qmp",
		SharedDir: repo + "/containers/xemu/shared",
		// Overridable so a worktree can mount ITS init scripts (e.g. testing the
		// dvd_path patch on a feature branch) instead of the main checkout's.
		InitDir: envOr("HARNESS_INIT_DIR", repo+"/containers/xemu/init"),
		// The default configs dir is root-owned (prior server-under-sudo runs)
		// and passwordless sudo here is scoped to podman only (no chown), so the
		// uid-1000 harness writes per-instance configs into a dir it owns. All
		// bind mounts + teardown derive from this, so it stays self-consistent.
		ConfigsDir:     repo + "/containers/xemu/configs-live",
		BrowserDir:     repo + "/containers/browser",
		BrowserInitDir: repo + "/containers/browser/init",
		PortBase:       portBase,
		PortStride:     portStride,
		HostIP:         "localhost",
		PodmanCmd:      podmanCmd,
		// Overridable for live tests via env: the app default root (_default.qcow2)
		// may be unbootable; point at a known-good disk + its DVD when needed.
		RootHDD:        envOr("HARNESS_ROOT_HDD", "_default.qcow2"),
		DVDPath:        os.Getenv("HARNESS_DVD_PATH"),
		ISODir:         envOr("HARNESS_ISO_DIR", repo+"/containers/xemu/shared/isos"),
		QemuImgCmd:     "qemu-img",
		SetConsoleName: false, // cosmetic; skip the FUSE/pyfatx dependency for the throwaway
		Encoder:        "x264enc",
		Framerate:      60,
		CRF:            20,
		Width:          960,
		Height:         720,
		DRINode:        "/dev/dri/renderD128",
		ShmSize:        "1g",
		BrowserShmSize: "2gb",
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: podman-harness raw|up|start|stop|down|info [args]")
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	if cmd == "raw" {
		raw(args)
		return
	}

	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: podman-harness %s <name>\n", cmd)
		os.Exit(2)
	}
	name := args[0]
	mgr, err := podman.NewManager(cfg(), &fileStore{statePath})
	if err != nil {
		fatal("new manager", err)
	}

	switch cmd {
	case "up":
		// HARNESS_GAME_ISO exercises the per-instance ISO path (CreateWithOptions);
		// absolute or a name resolved against ISODir. Empty falls back to the
		// global HARNESS_DVD_PATH.
		info, err := mgr.CreateWithOptions(name, podman.CreateOptions{GameISO: os.Getenv("HARNESS_GAME_ISO")})
		if err != nil {
			fatal("create", err)
		}
		printInfo(info)
		if info.GameISO != "" {
			fmt.Printf("game ISO      = %s (attached as DVD)\n", info.GameISO)
		}
		if err := mgr.Start(name); err != nil {
			fatal("start", err)
		}
		fmt.Println("started both containers")
	case "start":
		if err := mgr.Start(name); err != nil {
			fatal("start", err)
		}
		fmt.Println("started")
	case "stop":
		if err := mgr.Stop(name); err != nil {
			fatal("stop", err)
		}
		fmt.Println("stopped")
	case "down":
		if err := mgr.Remove(name); err != nil {
			fatal("remove", err)
		}
		fmt.Println("removed containers + files")
	case "rmfiles":
		// Direct os.RemoveAll of the per-instance files (overlay + config dir +
		// browser dir). Needed because Manager.removeContainerFiles routes
		// `rm -rf` through the podman command (a bug), and these files are
		// uid-owned so no sudo is required anyway.
		c := cfg()
		for _, p := range []string{
			c.SharedDir + "/hdds/" + name + ".qcow2",
			c.ConfigsDir + "/" + name,
			c.BrowserDir + "/config-" + name,
		} {
			if err := os.RemoveAll(p); err != nil {
				fmt.Fprintf(os.Stderr, "rm %s: %v\n", p, err)
			} else {
				fmt.Printf("removed %s\n", p)
			}
		}
	case "info":
		i, ok := mgr.Get(name)
		if !ok {
			fatal("info", fmt.Errorf("no such instance %q", name))
		}
		printInfo(&i)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", cmd)
		os.Exit(2)
	}
}

func printInfo(i *podman.ContainerInfo) {
	fmt.Printf("instance      = %s\n", i.Name)
	fmt.Printf("index         = %d\n", i.Index)
	fmt.Printf("BrowserWeb    = %d   (websockify: ws://127.0.0.1:%d/websockify)\n", i.Ports.BrowserWeb, i.Ports.BrowserWeb)
	fmt.Printf("BrowserVNC    = %d\n", i.Ports.BrowserVNC)
	fmt.Printf("XemuHTTP/S/WS = %d/%d/%d\n", i.Ports.XemuHTTP, i.Ports.XemuHTTPS, i.Ports.XemuWS)
	fmt.Printf("QMP socket    = %s/containers/xemu/qmp/%s.sock\n", repo, i.Name)
}

func raw(args []string) {
	// podmanCmd is "sudo -n podman --runtime=crun"; split it into argv.
	base := []string{"-n", "podman", "--runtime=crun"}
	c := exec.Command("sudo", append(base, args...)...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fatal(what string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
	os.Exit(1)
}

// fileStore is a trivial JSON-file ContainerInfo store so up/down persist across
// process invocations (the real Store is PocketBase-backed).
type fileStore struct{ path string }

func (s *fileStore) LoadAll() (map[string]*podman.ContainerInfo, error) {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return map[string]*podman.ContainerInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]*podman.ContainerInfo{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *fileStore) Upsert(info *podman.ContainerInfo) error {
	m, _ := s.LoadAll()
	if m == nil {
		m = map[string]*podman.ContainerInfo{}
	}
	m[info.Name] = info
	return s.save(m)
}

func (s *fileStore) Delete(name string) error {
	m, _ := s.LoadAll()
	delete(m, name)
	return s.save(m)
}

func (s *fileStore) save(m map[string]*podman.ContainerInfo) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(s.path), b, 0o644)
}
