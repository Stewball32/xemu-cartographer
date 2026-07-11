// Package play exposes the player-scoped /api/play/* endpoints the play tab and
// the Pi controller drive. Unlike /api/admin/containers/* and
// /api/admin/scraper/* (admin-only), these are RequireAuth-but-not-admin: each
// request resolves the CALLER's own container server-side from their gamertag
// against the live container rosters (the M09 scoped pattern —
// gamertags.SanitizedForUser + scraperiface.MatchContainer(Membership())), so a
// player can only see and drive the box their gamertag is currently in. Admins
// may target any container with ?container=<name>.
//
// The admin kiosk + VNC keyboard path (routes/containers) is untouched; this is
// a parallel, narrower surface onto the same host-runner arbitration the admin
// endpoints use, so an admin take-over (authority=admin) still natively suspends
// whatever a player set here.
package play

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
)

// PlayControl is the subset of *hostrunner.Registry the play endpoints call.
// Kept as an interface so this package has no compile-time dependency on the
// runner wiring — SetHostControl injects the live registry from main.go.
type PlayControl interface {
	Status(instance string) hostrunner.Status
	SetAuthority(instance string, auth hostrunner.Authority) bool
	SetReady(instance string, ready bool) bool
	SetSelection(instance, mapName, gametypeName string) bool
}

// Group is the router group for /api/play endpoints (RequireAuth, no admin).
var Group *router.RouterGroup[*core.RequestEvent]

// Scraper resolves the caller's container from live rosters (Membership). nil
// disables the package (RegisterAll becomes a no-op).
var Scraper scraperiface.Service

// HostRunners is the injected host-runner control surface. nil → endpoints that
// need it return 503 (server still bootable without the subsystem).
var HostRunners PlayControl

// SetScraper wires the scraper manager (for gamertag→container resolution).
func SetScraper(s scraperiface.Service) { Scraper = s }

// SetHostControl wires the host-runner registry. Call before RegisterAll.
func SetHostControl(c PlayControl) { HostRunners = c }

var registry []func()

func register(fn func()) { registry = append(registry, fn) }

// RegisterAll creates the /api/play group and registers all handlers. No-op when
// the scraper manager is nil (keeps the server bootable without the subsystem),
// mirroring routes/scraper.
func RegisterAll(se *core.ServeEvent) {
	if Scraper == nil {
		return
	}
	Group = se.Router.Group("/api/play")
	Group.Bind(apis.RequireAuth())
	for _, fn := range registry {
		fn()
	}
}
