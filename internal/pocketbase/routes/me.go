package routes

import (
	"log"
	"net/http"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb"
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
//
// M23a: notifications_unread_count is the count of rows in the notifications
// collection addressed to the caller where read=false. The bell badge in
// the header reads this on every /api/me hit (initial load + window focus
// + post-action refreshes); a dedicated count endpoint would just create a
// second polling cadence to debug.
//
// M08: Roles is the authoritative list of slugs the caller holds via the
// user_roles join. IsAdmin is kept as a derived shorthand (`roles.includes
// ("admin") || isSuperuser`) for frontend backwards-compat; new consumers
// should branch on Roles to support future M16-style "tournament_organizer"
// gates without another schema bump.
//
// authz (design §7.1 R-15): Scopes is the union of the caller's role scopes
// (canonical, sorted — the same list authz.Can matches against), Level the
// max role level, PrincipalKind the resolver's kind ("pb_user" for a users
// JWT). Superusers report scopes ["*"], level 1000 and "superuser" — they
// hold no user_roles rows but pass every check. IsAdmin, Roles, Scopes and
// Level all come from one principal so they can't disagree.
type meResponse struct {
	ID                       string         `json:"id"`
	Email                    string         `json:"email"`
	IsAdmin                  bool           `json:"isAdmin"`
	IsSuperuser              bool           `json:"isSuperuser"`
	Roles                    []string       `json:"roles"`
	Scopes                   []string       `json:"scopes"`
	PrincipalKind            string         `json:"principal_kind"`
	Level                    int            `json:"level"`
	DefaultGamertag          *gamertagInfo  `json:"default_gamertag"`
	Gamertags                []gamertagInfo `json:"gamertags"`
	Teams                    []teamInfo     `json:"teams"`
	NotificationsUnreadCount int            `json:"notifications_unread_count"`
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
	Status     string             `json:"status"`
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
		// Resolve the caller through the authz adapter (R-15). RequireAuth
		// already verified the JWT; the resolver re-reads the row, so a
		// user banned or soft-deleted since the token was issued gets 401
		// here rather than a stale identity payload.
		p, err := pb.ResolveRequest(e.App, pb.Default(), e)
		if err != nil {
			return apis.NewUnauthorizedError("account banned or deleted", err)
		}

		resp := meResponse{
			ID:            e.Auth.Id,
			Email:         e.Auth.Email(),
			IsAdmin:       p.IsAdmin(),
			IsSuperuser:   e.Auth.IsSuperuser(),
			Roles:         append([]string{}, p.Roles...),
			Scopes:        append([]string{}, p.Scopes...),
			PrincipalKind: string(p.Kind),
			Level:         p.Level,
			Gamertags:     []gamertagInfo{},
			Teams:         []teamInfo{},
		}

		// Superusers live in _superusers, not users — they have no
		// gamertags/teams to render and no user_roles rows. Return the
		// basic identity payload with IsAdmin=true so the admin nav still
		// renders for the bootstrap operator; scopes/level are the
		// allow-all sentinels the frontend's hasScope mirrors.
		if resp.IsSuperuser {
			resp.IsAdmin = true
			resp.Scopes = []string{"*"}
			resp.Level = 1000
			resp.PrincipalKind = string(authz.KindSuperuser)
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
					ID:     team.Id,
					Name:   team.GetString("name"),
					Slug:   team.GetString("slug"),
					Status: team.GetString("status"),
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

		// Bell-badge count. CountRecords returns 0 if the notifications
		// collection isn't present (shouldn't happen on a healthy server,
		// but the schema-registration ordering inside OnServe means the
		// route shouldn't crash a deployment that's mid-bootstrap).
		count, err := e.App.CountRecords(
			"notifications",
			dbx.HashExp{"user": userID, "read": false},
		)
		if err != nil {
			log.Printf("/api/me: notifications count for %s: %v", userID, err)
		} else {
			resp.NotificationsUnreadCount = int(count)
		}

		return e.JSON(http.StatusOK, resp)
	}).Bind(apis.RequireAuth())
}
