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
