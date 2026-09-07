package containers

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
)

// kioskTokenCookie is the cookie name used to carry a credential through the
// iframe's sub-resource requests. The iframe entry-point is fetched with
// ?token=…; once validated, this cookie is set with Path scoped to the
// per-container kiosk prefix so CSS/JS/images/websockify under that prefix
// authenticate without the parent page rewriting URLs.
const kioskTokenCookie = "kiosk_token"

// kioskCookieMaxAge bounds the kiosk cookie's lifetime (12h, PD-11) so a
// forgotten kiosk tab does not keep a credential alive indefinitely.
const kioskCookieMaxAge = 43200

// authorizeKioskAccess admits a caller to the kiosk/VNC proxy for container
// `name` (M09 9b/9c) for action a — kiosk.view for the noVNC proxy,
// kiosk.input for the VNC relay. The credential comes from ?token= or the
// kiosk_token cookie (pb.ResolveKiosk: a PB JWT or an opaque device key) and
// the decision is the rule table's: a scoped principal, a user whose
// gamertag is in that container's live roster (with the rostergrace TTL) or
// who owns the box, or a device key bound to the instance. Re-checked on
// every request; fails closed on any lookup error or before the deps are
// installed at boot.
func authorizeKioskAccess(e *core.RequestEvent, name string, a authz.Action) bool {
	if e == nil || e.App == nil {
		return false
	}
	d := pb.Default()
	p, _ := pb.ResolveKiosk(e.App, d, e)
	return authz.Can(d, p, a, authz.Container(name))
}

// requireManage is the per-handler container.manage check every mutating
// route (create / start / stop / remove / files / cleanup) runs after the
// group's admin.containers gate: the caller's scopes must cover the named
// container (or the global selector for cleanup). Returns the apis 401/403
// error to bubble, nil when allowed.
func requireManage(e *core.RequestEvent, r authz.Resource) error {
	return pb.Check(pb.Default(), e, authz.ActionContainerManage, r)
}

// setKioskTokenCookie persists the validated ?token= as an HttpOnly cookie
// scoped to the per-container kiosk prefix, so the iframe's sub-resource
// requests authenticate without anyone rewriting URLs. Path scoping means the
// cookie isn't sent to unrelated PB endpoints; Secure is set whenever the
// request arrived over TLS (directly or via a proxy's X-Forwarded-Proto) and
// the cookie expires after kioskCookieMaxAge (PD-11).
func setKioskTokenCookie(e *core.RequestEvent, path, token string) {
	http.SetCookie(e.Response, kioskCookie(e.Request, path, token))
}

// kioskCookie builds the kiosk_token cookie for req (split out so the Secure
// / MaxAge attributes are unit-testable without a RequestEvent).
func kioskCookie(req *http.Request, path, token string) *http.Cookie {
	return &http.Cookie{
		Name:     kioskTokenCookie,
		Value:    token,
		Path:     path,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsTLS(req),
		MaxAge:   kioskCookieMaxAge,
	}
}

// requestIsTLS reports whether req arrived over HTTPS — a direct TLS
// connection or a reverse proxy declaring X-Forwarded-Proto: https.
func requestIsTLS(req *http.Request) bool {
	if req == nil {
		return false
	}
	return req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https"
}
