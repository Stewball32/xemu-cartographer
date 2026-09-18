package xcclient

import (
	"os/exec"
	"strings"
	"testing"
)

const thisPkg = "github.com/xemu-cartographer/xemu-cartographer/internal/xcclient"

// TestXcclientIsPocketBaseFree pins DESIGN-STEP8 §8's layering: the upstream
// client reaches the flagship hub only through HubPort, so its (non-test)
// import closure must never pull PocketBase or the league packages that do.
func TestXcclientIsPocketBaseFree(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not available:", err)
	}
	out, err := exec.Command(goBin, "list", "-deps", thisPkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", thisPkg, err, out)
	}
	var bad []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		dep := strings.TrimSpace(line)
		switch {
		case dep == "" || dep == thisPkg:
		case strings.HasPrefix(dep, "github.com/pocketbase/"),
			strings.HasPrefix(dep, "github.com/xemu-cartographer/xemu-cartographer/internal/websocket"),
			strings.HasPrefix(dep, "github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase"),
			strings.HasPrefix(dep, "github.com/xemu-cartographer/xemu-cartographer/internal/leaguescraper"):
			bad = append(bad, dep)
		}
	}
	if len(bad) != 0 {
		t.Fatalf("xcclient depends on league/PocketBase packages:\n  %s", strings.Join(bad, "\n  "))
	}
}
