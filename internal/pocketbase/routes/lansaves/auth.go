package lansaves

import (
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
)

// Access policy for the LAN-saves endpoints.
//
// Two client classes hit this group: the admin browser editors (which carry a
// PocketBase JWT, loaded into e.Auth by PB's global loadAuthToken middleware)
// and the nxdk LAN client on the Xbox (which carries a machine key in
// X-LAN-Token / ?token= / Authorization: Bearer). Both are resolved to one
// authz principal by pb.AuthorizeLAN (design A.5): no credential or a bad one
// is a 401, and each route's verb below names the action + resource that
// principal must be allowed — an admin passes on level, a machine key on its
// scopes. There is no open mode: an unset key is a closed door.
//
// lanVerb is the route → verb table. /file/{kind}/{id} returns an empty
// action on purpose: its resource needs the served record's owner, so
// handleServeFile performs that check itself after the load (PD-8).

// groupPrefix is the mount point lanVerb strips before matching.
const groupPrefix = "/api/lan/saves"

// lanActionUnmapped is what a path outside the table yields: an action the
// rule table does not know, which authz.Can denies for every principal.
const lanActionUnmapped authz.Action = "lan.saves.unmapped"

// lanVerb adapts lanVerbFor to the request (pb.AuthorizeLAN's verb shape).
func lanVerb(e *core.RequestEvent) (authz.Action, authz.Resource) {
	return lanVerbFor(strings.TrimPrefix(e.Request.URL.Path, groupPrefix))
}

// lanVerbFor is the pure route → (action, resource) map for a path relative
// to the group (unit-tested without a request). Unknown paths fail closed.
func lanVerbFor(path string) (authz.Action, authz.Resource) {
	path = "/" + strings.Trim(path, "/")
	switch {
	case path == "/identity" || strings.HasPrefix(path, "/identity/"):
		return authz.ActionLANSavesIdentity, authz.Global()
	case strings.HasPrefix(path, "/file/"):
		return "", authz.Resource{}
	case path == "/build":
		return authz.ActionLANSavesBuild, authz.Global()
	case path == "/download":
		return authz.ActionLANSavesDownload, authz.Global()
	case path == "/manifest":
		return authz.ActionLANSavesManifest, authz.Global()
	case path == "/meta":
		return authz.ActionLANSavesMeta, authz.Global()
	}
	return lanActionUnmapped, authz.Global()
}
