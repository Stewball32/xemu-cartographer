package routes

import (
	"log"
	"net/http"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/gamertags"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

func init() {
	register(registerMeMatchRoute)
}

// meMatchResponse is the shape returned by GET /api/me/match (M09 9b). Container
// is the name of the container whose live roster contains one of the caller's
// gamertags, or null when the caller isn't in any active match. The /play/ page
// polls this (initial load + window focus) and renders the kiosk for Container
// when set, idle otherwise.
type meMatchResponse struct {
	Container *string `json:"container"`
}

func registerMeMatchRoute(se *core.ServeEvent) {
	se.Router.GET("/api/me/match", func(e *core.RequestEvent) error {
		resp := meMatchResponse{}

		// No scraper subsystem (or containers disabled) → never a match.
		if scraperSvc == nil {
			return e.JSON(http.StatusOK, resp)
		}

		// Superusers live in _superusers and own no gamertags — they always
		// resolve to "no match", which is the correct idle state for /play/.
		tags, err := gamertags.SanitizedForUser(e.App, e.Auth.Id)
		if err != nil {
			log.Printf("/api/me/match: gamertags for %s: %v", e.Auth.Id, err)
			return e.JSON(http.StatusOK, resp) // fail soft → idle
		}

		if name, ok := scraperiface.MatchContainer(scraperSvc.Membership(), tags); ok {
			resp.Container = &name
		}
		return e.JSON(http.StatusOK, resp)
	}).Bind(apis.RequireAuth())
}
