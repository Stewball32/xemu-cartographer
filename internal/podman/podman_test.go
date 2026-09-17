package podman

import (
	"strings"
	"testing"
)

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

// TestContainerUIDIsRoot pins the in-container uid/gid to 0. xemu's pcap
// netplay only works when the NET_ADMIN/NET_RAW caps land in the effective
// set, which Linux does for root only; deriving the value from the league's
// own uid (the pre-2026-09-16 behaviour, os.Getuid()) broke every tier whose
// league runs unprivileged and reaches rootful podman through sudo.
func TestContainerUIDIsRoot(t *testing.T) {
	if containerUID != 0 || containerGID != 0 {
		t.Fatalf("containerUID/GID = %d/%d, want 0/0 (xemu needs root for pcap caps)", containerUID, containerGID)
	}
}
