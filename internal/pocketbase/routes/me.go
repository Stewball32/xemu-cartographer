package routes

import (
	"log"
	"net/http"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerMeRoute)
}

// meResponse is the shape returned by GET /api/me. Fields added in M7:
//   - default_gamertag: caller's "show me as" pick, or null
//   - gamertags:        every tag the caller owns (every status included so
//     the settings UI can render a badge for blocked/pending rows)
//   - teams:            currently-active team memberships with the per-team
//     captain/manager flags and joined_at date
//
// M22b: gamertagInfo.Blocked → Status (4-state enum from the gamertags
// SelectField). Callers that previously checked `blocked === true` should
// switch to `status === "blocked"`.
type meResponse struct {
	ID              string         `json:"id"`
	Email           string         `json:"email"`
	IsAdmin         bool           `json:"isAdmin"`
	IsSuperuser     bool           `json:"isSuperuser"`
	DefaultGamertag *gamertagInfo  `json:"default_gamertag"`
	Gamertags       []gamertagInfo `json:"gamertags"`
	Teams           []teamInfo     `json:"teams"`
}

type gamertagInfo struct {
	ID     string `json:"id"`
	Tag    string `json:"tag"`
	Status string `json:"status"`
}

type teamInfo struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Slug       string             `json:"slug"`
	Membership teamMembershipInfo `json:"membership"`
}

type teamMembershipInfo struct {
	GamertagID string  `json:"gamertag_id"`
	IsOwner    bool    `json:"is_owner"`
	IsManager  bool    `json:"is_manager"`
	JoinedAt   string  `json:"joined_at"`
	LeftAt     *string `json:"left_at"`
}

func registerMeRoute(se *core.ServeEvent) {
	se.Router.GET("/api/me", func(e *core.RequestEvent) error {
		resp := meResponse{
			ID:          e.Auth.Id,
			Email:       e.Auth.Email(),
			IsAdmin:     e.Auth.GetBool("isAdmin"),
			IsSuperuser: e.Auth.IsSuperuser(),
			Gamertags:   []gamertagInfo{},
			Teams:       []teamInfo{},
		}

		// Superusers live in _superusers, not users — they have no
		// gamertags/teams to render. Return the basic identity payload.
		if resp.IsSuperuser {
			return e.JSON(http.StatusOK, resp)
		}

		userID := e.Auth.Id
		tags, err := e.App.FindRecordsByFilter(
			"gamertags",
			"user = {:userID}",
			"tag",
			0,
			0,
			dbx.Params{"userID": userID},
		)
		if err != nil {
			log.Printf("/api/me: gamertags lookup for %s: %v", userID, err)
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": "identity lookup failed"})
		}
		for _, t := range tags {
			resp.Gamertags = append(resp.Gamertags, gamertagInfo{
				ID:     t.Id,
				Tag:    t.GetString("tag"),
				Status: t.GetString("status"),
			})
		}

		// default_gamertag may be empty (OAuth signup before username set)
		// or point at a now-deleted row (race with admin delete). Both are
		// non-fatal — the caller renders a "no default selected" UI.
		if defID := e.Auth.GetString("default_gamertag"); defID != "" {
			for i := range resp.Gamertags {
				if resp.Gamertags[i].ID == defID {
					resp.DefaultGamertag = &resp.Gamertags[i]
					break
				}
			}
		}

		rosters, err := e.App.FindRecordsByFilter(
			"rosters",
			"gamertag.user = {:userID} && left_at = null",
			"team.name",
			0,
			0,
			dbx.Params{"userID": userID},
		)
		if err != nil {
			log.Printf("/api/me: rosters lookup for %s: %v", userID, err)
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": "identity lookup failed"})
		}
		if len(rosters) > 0 {
			for _, expandErr := range e.App.ExpandRecords(rosters, []string{"team"}, nil) {
				if expandErr != nil {
					log.Printf("/api/me: team expand for %s: %v", userID, expandErr)
					return e.JSON(http.StatusInternalServerError, map[string]string{"error": "identity lookup failed"})
				}
			}
			for _, r := range rosters {
				team := r.ExpandedOne("team")
				if team == nil {
					// Roster row references a since-deleted team — skip
					// silently. (Shouldn't happen with the cascade settings
					// we have, but defensive.)
					continue
				}
				resp.Teams = append(resp.Teams, teamInfo{
					ID:   team.Id,
					Name: team.GetString("name"),
					Slug: team.GetString("slug"),
					Membership: teamMembershipInfo{
						GamertagID: r.GetString("gamertag"),
						IsOwner:    r.GetBool("is_owner"),
						IsManager:  r.GetBool("is_manager"),
						JoinedAt:   r.GetDateTime("joined_at").String(),
						LeftAt:     nil,
					},
				})
			}
		}

		return e.JSON(http.StatusOK, resp)
	}).Bind(apis.RequireAuth())
}
