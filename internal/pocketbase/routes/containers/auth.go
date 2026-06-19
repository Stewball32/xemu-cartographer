package containers

import (
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/gamertags"
	"github.com/Stewball32/xemu-cartographer/internal/roles"
	"github.com/Stewball32/xemu-cartographer/internal/rostergrace"
)

// kioskTokenCookie is the cookie name used to carry a JWT through the iframe's
// sub-resource requests. The iframe entry-point is fetched with ?token=…; once
// validated, this cookie is set with Path scoped to the per-container kiosk
// prefix so CSS/JS/images/websockify under that prefix authenticate without
// the parent page rewriting URLs.
const kioskTokenCookie = "kiosk_token"

// kioskTokenRecord resolves the caller's auth record from a PocketBase JWT
// pulled from either `?token=` (preferred — used by the parent page when
// constructing iframe and WebSocket URLs) or the kiosk_token cookie (used by
// sub-resource fetches originating inside the iframe). Returns nil when no
// token is present or it doesn't validate.
func kioskTokenRecord(e *core.RequestEvent) *core.Record {
	if Services == nil || Services.App == nil {
		return nil
	}
	token := e.Request.URL.Query().Get("token")
	if token == "" {
		if c, err := e.Request.Cookie(kioskTokenCookie); err == nil {
			token = c.Value
		}
	}
	if token == "" {
		return nil
	}
	record, err := Services.App.FindAuthRecordByToken(token, core.TokenTypeAuth)
	if err != nil || record == nil {
		return nil
	}
	return record
}

// authorizeKioskAccess admits a caller to the kiosk/VNC proxy for container
// `name` (M09 9b/9c). Admins (superuser or admin role) get in for any
// container; a non-admin is admitted when one of their gamertags is in that
// specific container's live roster OR was seen there within the rostergrace
// TTL — so a transient roster drop (e.g. editing the gametype) doesn't kick
// them off mid-edit. Re-checked on every request; fails closed once the grace
// window lapses or on any lookup error.
func authorizeKioskAccess(e *core.RequestEvent, name string) bool {
	record := kioskTokenRecord(e)
	if record == nil {
		return false
	}
	if roles.IsAdminAuth(Services.App, record) {
		return true
	}
	if Services.Scraper == nil {
		return false
	}
	tags, err := gamertags.SanitizedForUser(Services.App, record.Id)
	if err != nil || len(tags) == 0 {
		return false
	}
	return rostergrace.Default.Allow(Services.Scraper.Membership(), name, tags, time.Now())
}

// setKioskTokenCookie persists the validated ?token= as an HttpOnly cookie
// scoped to the per-container kiosk prefix, so the iframe's sub-resource
// requests authenticate without anyone rewriting URLs. Path scoping means the
// cookie isn't sent to unrelated PB endpoints.
func setKioskTokenCookie(e *core.RequestEvent, path, token string) {
	http.SetCookie(e.Response, &http.Cookie{
		Name:     kioskTokenCookie,
		Value:    token,
		Path:     path,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
