package leaguescraper

import (
	"os/exec"
	"strings"
	"testing"
)

// managerPkg is the one flagship package the scraper manager's dependency
// closure may contain: itself.
const managerPkg = "github.com/Stewball32/xemu-cartographer/internal/scraper/manager"

// TestManagerIsLeagueFree pins the step 7 part 3c exit criterion: the
// scraper manager's (non-test) import closure contains no PocketBase and no
// other flagship package. Everything league-shaped — the WS hub, authz,
// guards.Services, the games persistence chain — reaches the manager only
// through its ports (manager.Options) or wraps it (WireAdapter), never the
// other way round. If this fails, a league import crept back into
// internal/scraper/manager (or something it imports).
func TestManagerIsLeagueFree(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not available:", err)
	}
	out, err := exec.Command(goBin, "list", "-deps", managerPkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", managerPkg, err, out)
	}
	var bad []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		dep := strings.TrimSpace(line)
		if dep == "" || dep == managerPkg {
			continue
		}
		if strings.HasPrefix(dep, "github.com/pocketbase/") ||
			strings.HasPrefix(dep, "github.com/Stewball32/xemu-cartographer/") {
			bad = append(bad, dep)
		}
	}
	if len(bad) != 0 {
		t.Fatalf("scraper manager depends on league packages:\n  %s", strings.Join(bad, "\n  "))
	}
}
