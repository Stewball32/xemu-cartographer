package pb

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
)

// wsOriginsEnv is the WebSocket origin allowlist websocket/handler.go reads
// (PD-15: fail-open with a warning when unset).
const wsOriginsEnv = "WS_ALLOWED_ORIGINS"

// Report is the boot-time state LogStartup prints (§8.2).
type Report struct {
	RolesOK         bool
	MissingRoles    []string
	TokenCounts     map[string]int // "machine" / "spectator" / "device" = live rows; "revoked", "expired"
	LegacyImported  bool
	ConsoleDoorOpen bool
	AnonymousScopes []string
	AdminCount      int
	WSOriginsUnset  bool
}

// Inspect gathers the Report from the roles / user_roles / api_tokens
// tables, the adapter's legacy row and the environment.
func Inspect(app core.App, d *PBDeps) Report {
	r := Report{
		TokenCounts:     map[string]int{"machine": 0, "spectator": 0, "device": 0, "revoked": 0, "expired": 0},
		AnonymousScopes: []string{},
		WSOriginsUnset:  strings.TrimSpace(os.Getenv(wsOriginsEnv)) == "",
	}
	if app == nil && d != nil {
		app = d.app
	}
	if d == nil && app != nil {
		d = newDeps(app, nil, nil)
	}

	table := loadRoles(app)
	for _, seed := range authz.SeedRoles {
		if _, ok := table[seed.Slug]; !ok {
			r.MissingRoles = append(r.MissingRoles, seed.Slug)
		}
	}
	r.RolesOK = len(r.MissingRoles) == 0

	r.AdminCount = d.AdminCount()
	r.AnonymousScopes = d.AnonymousScopes()
	r.ConsoleDoorOpen = len(r.AnonymousScopes) > 0
	r.LegacyImported = d.LegacyImported()

	if app != nil {
		now := time.Now()
		if rows, err := app.FindAllRecords("api_tokens"); err == nil {
			for _, rec := range rows {
				switch {
				case rec.GetBool("revoked"):
					r.TokenCounts["revoked"]++
				case tokenExpired(rec.GetDateTime("expires_at").Time(), now):
					r.TokenCounts["expired"]++
				default:
					r.TokenCounts[rec.GetString("kind")]++
				}
			}
		}
	}
	return r
}

// tokenExpired reports whether a non-zero expiry has passed.
func tokenExpired(expires, now time.Time) bool {
	return !expires.IsZero() && !now.Before(expires)
}

// LogStartup prints the §8.2 boot lines through log (log.Printf in main):
// roles, admins, anonymous scopes, console door, legacy key, api_tokens
// counts, /api/lan/* posture, and — only when unset — the WS origins
// warning.
func LogStartup(r Report, log func(string, ...any)) {
	if log == nil {
		return
	}
	if r.RolesOK {
		slugs := make([]string, 0, len(authz.SeedRoles))
		for _, seed := range authz.SeedRoles {
			slugs = append(slugs, seed.Slug)
		}
		log("authz: roles ok (%s)", strings.Join(slugs, ", "))
	} else {
		log("authz: roles MISSING: [%s] — run migrations", strings.Join(r.MissingRoles, ", "))
	}

	if r.AdminCount > 0 {
		log("authz: admins=%d", r.AdminCount)
	} else {
		log("authz: WARNING no admin users — grant one from /_/ (superuser) or the seed")
	}

	if len(r.AnonymousScopes) > 0 {
		log("authz: anonymous scopes=[%s]", strings.Join(r.AnonymousScopes, ", "))
	} else {
		log("authz: console door CLOSED (anonymous role has no scopes)")
	}

	if r.ConsoleDoorOpen {
		log("authz: console door OPEN (?console= accepted; PD-1 window)")
	} else {
		log("authz: console door CLOSED (?console= connects but can join nothing)")
	}

	if r.LegacyImported {
		log("authz: LAN_SAVES_TOKEN imported as kid=legacy-env scopes=[%s] — WARNING: rotate to an api_tokens machine key and unset the env var (PD-12)", strings.Join(LegacyScopes, ", "))
	} else {
		log("authz: LAN_SAVES_TOKEN unset")
	}

	counts := r.TokenCounts
	log("authz: api_tokens machine=%d spectator=%d device=%d (revoked=%d, expired=%d)",
		counts["machine"], counts["spectator"], counts["device"], counts["revoked"], counts["expired"])

	if counts["machine"] == 0 && !r.LegacyImported {
		log("authz: WARNING /api/lan/* has no valid key — all LAN clients will get 401")
	} else {
		log("authz: /api/lan/* fail-closed: %d live machine keys", counts["machine"])
	}

	if r.WSOriginsUnset {
		log("authz: %s unset — fail-open (PD-15), set it before exposing /api/ws", wsOriginsEnv)
	}
}

// String renders the report on one line (debug / tests).
func (r Report) String() string {
	return fmt.Sprintf("roles_ok=%v missing=%v admins=%d anon=%v door_open=%v legacy=%v tokens=%v ws_origins_unset=%v",
		r.RolesOK, r.MissingRoles, r.AdminCount, r.AnonymousScopes, r.ConsoleDoorOpen, r.LegacyImported, r.TokenCounts, r.WSOriginsUnset)
}
