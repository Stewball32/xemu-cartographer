// Package xc is the league-side HTTP surface the xc-scraper daemon calls
// (step 8 §7.2): `/api/xc/*`. Today that is the finished_game webhook.
//
// Every route in the group is gated by the `scraper.ingest` action through
// the authz adapter, read per request via pb.Default() (repo convention —
// pbtest swaps the default in tests, and a nil default fails closed). The
// credential is a machine key: an api_tokens row minted with the
// `scraper.ingest` scope (or the seeded admin `scraper.*`), or the
// XC_SCRAPER_WEBHOOK_TOKEN env import (authzpb.ImportWebhookEnv), presented
// as `Authorization: Bearer <key>` — which is exactly what the daemon's
// --webhook-token becomes.
package xc

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
)

// Group is the /api/xc route group. All routes inherit requireIngest.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

func register(fn func()) { registry = append(registry, fn) }

// requireIngest admits superusers and any principal whose scopes cover
// `scraper.ingest` on the global resource. The deps are read at request
// time through pb.Default() and a nil default fails closed.
func requireIngest() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		return pb.Require(pb.Default(), authz.ActionScraperIngest, nil)(e)
	}
}

// RegisterAll creates the xc group and registers all handlers. Always
// active — the ingest path needs only PocketBase.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/xc")
	Group.BindFunc(requireIngest())

	for _, fn := range registry {
		fn()
	}
}
