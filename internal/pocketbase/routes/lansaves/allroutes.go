// Package lansaves exposes /api/lan/saves/* — the LAN hub's profile/gametype
// save generator and download endpoint. It is the server side of the file-based
// remote-setup flow: the browser editors POST a spec to preview/validate a
// generated Halo CE/H2 save, and the de-risked nxdk LAN client GETs the
// generated file(s) with a disk-space pre-flight before writing them onto the
// Xbox FATX disk.
//
// This is the file-based complement to M24's memory-write approach: instead of
// poking the running game's lobby state, it produces the on-disk UDATA saves
// (gametype variants, H2 player profiles) the game reads at boot. Generation is
// pure template-patch over real samples (see internal/halosave); the unresolved
// 20-byte digest is surfaced, not hidden.
package lansaves

import (
	"os"
	"strconv"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/diskspace"
)

// Group is the router group for /api/lan/saves. Access is governed by
// authorizeLAN (admin JWT or the optional LAN token), NOT RequireAdmin, because
// the nxdk client cannot present a browser JWT.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

func register(fn func()) { registry = append(registry, fn) }

// Config tunes the disk-space guard. Read once in RegisterAll from env.
type Config struct {
	// StagingDir is the directory whose free space the server checks before
	// generating (a degenerate "server itself is full" guard). Default os.TempDir.
	StagingDir string
	// FATXCluster is the FATX cluster size used for the footprint estimate.
	FATXCluster int
}

var cfg = Config{FATXCluster: diskspace.DefaultFATXCluster}

// RegisterAll creates the group and registers all handlers.
func RegisterAll(se *core.ServeEvent) {
	cfg.StagingDir = envDefault("LAN_SAVES_STAGING_DIR", os.TempDir())
	if cs, err := strconv.Atoi(os.Getenv("LAN_SAVES_FATX_CLUSTER")); err == nil && cs > 0 {
		cfg.FATXCluster = cs
	}

	Group = se.Router.Group("/api/lan/saves")
	Group.BindFunc(authorizeLAN())
	for _, fn := range registry {
		fn()
	}
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
