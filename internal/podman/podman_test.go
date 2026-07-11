package podman

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSeedEeprom guards the fix-#2 provisioning step: the root HDD's paired
// eeprom must land at the toml's eeprom_path, resolve relative to SharedDir,
// no-op when unset, and never clobber an existing per-instance eeprom.
func TestSeedEeprom(t *testing.T) {
	dst := func(cfgDir string) string {
		return filepath.Join(cfgDir, ".local", "share", "xemu", "xemu", "eeprom.bin")
	}

	t.Run("unset is a no-op", func(t *testing.T) {
		cfgDir := t.TempDir()
		m := &Manager{cfg: Config{}}
		if err := m.seedEeprom(cfgDir); err != nil {
			t.Fatalf("seedEeprom: %v", err)
		}
		if _, err := os.Stat(dst(cfgDir)); !os.IsNotExist(err) {
			t.Fatalf("expected no eeprom written, got err=%v", err)
		}
	})

	t.Run("absolute path is seeded", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "eeprom-ceprof.bin")
		want := []byte("PAIRED-EEPROM-256B-STUB")
		if err := os.WriteFile(src, want, 0o644); err != nil {
			t.Fatal(err)
		}
		cfgDir := t.TempDir()
		m := &Manager{cfg: Config{RootEeprom: src}}
		if err := m.seedEeprom(cfgDir); err != nil {
			t.Fatalf("seedEeprom: %v", err)
		}
		got, err := os.ReadFile(dst(cfgDir))
		if err != nil {
			t.Fatalf("read seeded eeprom: %v", err)
		}
		if string(got) != string(want) {
			t.Fatalf("seeded eeprom = %q, want %q", got, want)
		}
	})

	t.Run("relative path resolves against SharedDir", func(t *testing.T) {
		shared := t.TempDir()
		if err := os.WriteFile(filepath.Join(shared, "root.eeprom"), []byte("REL"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfgDir := t.TempDir()
		m := &Manager{cfg: Config{SharedDir: shared, RootEeprom: "root.eeprom"}}
		if err := m.seedEeprom(cfgDir); err != nil {
			t.Fatalf("seedEeprom: %v", err)
		}
		got, _ := os.ReadFile(dst(cfgDir))
		if string(got) != "REL" {
			t.Fatalf("relative seed = %q, want REL", got)
		}
	})

	t.Run("does not clobber an existing eeprom", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "paired.bin")
		if err := os.WriteFile(src, []byte("PAIRED"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfgDir := t.TempDir()
		existing := dst(cfgDir)
		if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(existing, []byte("INSTANCE-CUSTOMISED"), 0o644); err != nil {
			t.Fatal(err)
		}
		m := &Manager{cfg: Config{RootEeprom: src}}
		if err := m.seedEeprom(cfgDir); err != nil {
			t.Fatalf("seedEeprom: %v", err)
		}
		got, _ := os.ReadFile(existing)
		if string(got) != "INSTANCE-CUSTOMISED" {
			t.Fatalf("existing eeprom was clobbered: %q", got)
		}
	})
}

func TestSudoPrefix(t *testing.T) {
	cases := []struct {
		podmanCmd string
		want      string // space-joined prefix, "" for none
	}{
		// The default .env value — the regression: "podman" isn't last, so the
		// prefix must still be just "sudo -n" (not "sudo -n podman --runtime=crun").
		{"sudo -n podman --runtime=crun", "sudo -n"},
		{"sudo -n podman", "sudo -n"},
		{"sudo podman", "sudo"},
		{"podman", ""},          // rootless: no prefix, run directly
		{"/usr/bin/podman", ""}, // absolute path still matches basename
		{"sudo -n /usr/bin/podman --log-level=error", "sudo -n"},
		{"", ""},
	}
	for _, c := range cases {
		got := strings.Join(sudoPrefix(c.podmanCmd), " ")
		if got != c.want {
			t.Errorf("sudoPrefix(%q) = %q, want %q", c.podmanCmd, got, c.want)
		}
	}
}

// TestXemuAutostartCarriesQMP guards the critical regression: both WM variants
// must launch xemu with the -qmp socket the scraper needs, or the container's
// memory is unreadable.
func TestXemuAutostartCarriesQMP(t *testing.T) {
	wantQMP := "-qmp unix:/qmp/pod1.sock,server,nowait"
	openbox := xemuAutostartScript("pod1", false)
	labwc := xemuAutostartScript("pod1", true)

	for name, s := range map[string]string{"openbox": openbox, "labwc": labwc} {
		if !strings.Contains(s, wantQMP) {
			t.Errorf("%s autostart missing %q:\n%s", name, wantQMP, s)
		}
		if !strings.Contains(s, "/opt/xemu/AppRun") || !strings.Contains(s, "-full-screen") {
			t.Errorf("%s autostart missing the xemu launch line:\n%s", name, s)
		}
	}
	// Wayland (labwc) wraps in foot; X11 (openbox) does not.
	if !strings.Contains(labwc, "foot -e /opt/xemu/AppRun") {
		t.Errorf("labwc autostart should launch via foot:\n%s", labwc)
	}
	if strings.Contains(openbox, "foot") {
		t.Errorf("openbox autostart should NOT use foot:\n%s", openbox)
	}
}

// TestSudoPrefixDoesNotHandCommandToPodman guards the specific orphan-files bug:
// runSudo("rm","-rf",...) must never produce a `podman ... rm -rf` invocation.
func TestSudoPrefixDoesNotHandCommandToPodman(t *testing.T) {
	for _, cmd := range []string{"sudo -n podman --runtime=crun", "sudo podman", "podman"} {
		full := append(sudoPrefix(cmd), "rm", "-rf", "/some/path")
		for _, tok := range full {
			if tok == "podman" {
				t.Errorf("PodmanCmd %q: rm command routed through podman: %v", cmd, full)
			}
		}
	}
}
