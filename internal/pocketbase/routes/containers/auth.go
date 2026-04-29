package containers

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

// kioskTokenCookie is the cookie name used to carry a JWT through the iframe's
// sub-resource requests. The iframe entry-point is fetched with ?token=…; once
// validated, this cookie is set with Path scoped to the per-container kiosk
// prefix so CSS/JS/images/websockify under that prefix authenticate without
// the parent page rewriting URLs.
const kioskTokenCookie = "kiosk_token"

// authorizeAdminQueryToken validates a PocketBase JWT pulled from either
// `?token=` (preferred — used by the parent page when constructing iframe and
// WebSocket URLs) or the kiosk_token cookie (used by sub-resource fetches
// originating inside the iframe). It then confirms the caller is a superuser
// or has isAdmin=true on their users-record.
//
// Returns true when admitted; false when missing/invalid/non-admin.
func authorizeAdminQueryToken(e *core.RequestEvent) bool {
	if Services == nil || Services.App == nil {
		return false
	}
	token := e.Request.URL.Query().Get("token")
	if token == "" {
		if c, err := e.Request.Cookie(kioskTokenCookie); err == nil {
			token = c.Value
		}
	}
	if token == "" {
		return false
	}
	record, err := Services.App.FindAuthRecordByToken(token, core.TokenTypeAuth)
	if err != nil || record == nil {
		return false
	}
	if record.IsSuperuser() {
		return true
	}
	if record.GetBool("isAdmin") {
		return true
	}
	return false
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
