// Package lansync exposes /api/lan/sync/* — the LAN box-provisioning contract
// the norcal-og-xbox client consumes to push player profiles + games + apps onto
// LAN Xboxes from cartographer.
//
// It is the SERVER half of a client↔server system (SCAFFOLD — heavy resolution
// is stubbed with TODOs):
//
//   - GET /api/lan/sync/manifest?preset=<id|active>
//     resolves the active (or a named) sync_preset into the concrete set of
//     profiles + games + apps, in priority order, with per-category
//     conflict/prune flags + ready-to-GET download URLs. THIS is the contract
//     the client consumes (shape reconciled against the client session).
//   - GET /api/lan/sync/games/{id}/download
//   - GET /api/lan/sync/apps/{id}/download
//     kiosk-scoped pulls of the derived EXTRACTED trees for a game (iso) / app.
//     (Profiles + gametypes already have /api/lan/saves — this group adds only
//     games + apps.)
//
// Access mirrors /api/lan/saves (authorizeLAN): admin JWT or the shared LAN
// token, OPEN when the token env is unset — the trusted-LAN-appliance default.
// TODO(lan-sync): kiosk stations are unauthenticated on the LAN today; tighten
// (per-station token / mTLS) before exposing beyond a trusted segment.
package lansync

import (
	"crypto/subtle"
	"net/http"
	"os"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

// Group is the router group for /api/lan/sync. Access is governed by
// authorizeLAN (admin JWT or the optional LAN token), NOT RequireAdmin, because
// the on-Xbox client cannot present a browser JWT.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

func register(fn func()) { registry = append(registry, fn) }

// RegisterAll creates the group + registers all handlers.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/lan/sync")
	Group.BindFunc(authorizeLAN())
	for _, fn := range registry {
		fn()
	}
}

// lanTokenEnv is shared with /api/lan/saves so a single LAN token unlocks both
// surfaces. TODO(lan-sync): unify this + lansaves.authorizeLAN into one shared
// helper instead of two parallel copies.
const lanTokenEnv = "LAN_SAVES_TOKEN"

func lanToken() string { return os.Getenv(lanTokenEnv) }

func lanTokenFromRequest(e *core.RequestEvent) string {
	if t := e.Request.Header.Get("X-LAN-Token"); t != "" {
		return t
	}
	return e.Request.URL.Query().Get("token")
}

// lanAccessAllowed is the pure access decision (unit-testable without a request).
func lanAccessAllowed(envToken, provided string, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	if envToken == "" {
		return true // LAN-trusted default (see package doc TODO)
	}
	return provided != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(envToken)) == 1
}

func authorizeLAN() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		isAdmin := e.Auth != nil && roles.IsAdminAuth(e.App, e.Auth)
		if !lanAccessAllowed(lanToken(), lanTokenFromRequest(e), isAdmin) {
			return e.JSON(http.StatusUnauthorized, map[string]string{
				"error": "LAN sync access denied — present the LAN token (X-LAN-Token or ?token=) or authenticate as an admin",
			})
		}
		return e.Next()
	}
}
