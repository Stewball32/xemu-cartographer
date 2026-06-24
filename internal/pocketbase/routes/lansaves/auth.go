package lansaves

import (
	"crypto/subtle"
	"net/http"
	"os"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

// Access policy for the LAN-saves endpoints.
//
// Two client classes hit this group: the admin browser editors (which carry a
// PocketBase JWT, loaded into e.Auth by PB's global loadAuthToken middleware)
// and the nxdk LAN client on the Xbox (which carries nothing, or a static
// shared token). The policy reconciles both:
//
//   - An authenticated admin is always allowed.
//   - If LAN_SAVES_TOKEN is set, a request presenting it (via the X-LAN-Token
//     header or ?token= query) is allowed.
//   - If LAN_SAVES_TOKEN is unset, the endpoints are OPEN — the default for a
//     trusted LAN appliance. Set the env var to lock them down.
//
// Authorization is reserved for the PB JWT; the LAN token deliberately uses a
// separate header/query so the two never collide.
const lanTokenEnv = "LAN_SAVES_TOKEN"

func lanToken() string { return os.Getenv(lanTokenEnv) }

// lanTokenFromRequest pulls the presented LAN token from X-LAN-Token or ?token=.
func lanTokenFromRequest(e *core.RequestEvent) string {
	if t := e.Request.Header.Get("X-LAN-Token"); t != "" {
		return t
	}
	return e.Request.URL.Query().Get("token")
}

// lanAccessAllowed is the pure access decision (unit-tested without a request).
// Returns whether access is granted and a short reason for logging.
func lanAccessAllowed(envToken, provided string, isAdmin bool) (bool, string) {
	if isAdmin {
		return true, "admin"
	}
	if envToken == "" {
		return true, "open" // LAN-trusted default
	}
	if provided != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(envToken)) == 1 {
		return true, "token"
	}
	return false, "denied"
}

// authorizeLAN is the group middleware enforcing the policy above.
func authorizeLAN() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		isAdmin := e.Auth != nil && roles.IsAdminAuth(e.App, e.Auth)
		ok, _ := lanAccessAllowed(lanToken(), lanTokenFromRequest(e), isAdmin)
		if !ok {
			return e.JSON(http.StatusUnauthorized, map[string]string{
				"error": "LAN saves access denied — present the LAN token (X-LAN-Token or ?token=) or authenticate as an admin",
			})
		}
		return e.Next()
	}
}
