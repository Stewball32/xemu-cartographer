package pb

import (
	"errors"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
)

// Require is the route-group middleware: resolve the principal (stored for
// Get), then Can(a, res(e)). An unresolvable credential or an unbound
// anonymous caller gets 401; every other denial 403.
func Require(d *PBDeps, a authz.Action, res func(e *core.RequestEvent) authz.Resource) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if _, err := ResolveRequest(e.App, d, e); err != nil {
			return unauthorized(err)
		}
		r := authz.Global()
		if res != nil {
			r = res(e)
		}
		if err := Check(d, e, a, r); err != nil {
			return err
		}
		return e.Next()
	}
}

// Get returns the principal a Require / ResolveRequest / AuthorizeLAN
// middleware stored on the event. When none ran yet it resolves the request
// once through Default() (fail-closed: a nil Default still yields at most
// what the carriers prove) and stores the result; Nobody() when nothing
// resolves.
func Get(e *core.RequestEvent) authz.Principal {
	if e == nil {
		return authz.Nobody()
	}
	if p, ok := e.Get(principalKey).(authz.Principal); ok {
		return p
	}
	p, _ := ResolveRequest(e.App, Default(), e)
	return p
}

// Check is the handler-level check: Can(d, Get(e), a, r) mapped to apis
// errors — 401 for an unresolvable credential or an unbound anonymous
// principal, 403 otherwise, nil when allowed.
func Check(d *PBDeps, e *core.RequestEvent, a authz.Action, r authz.Resource) error {
	if e == nil {
		return apis.NewUnauthorizedError("authentication required", nil)
	}
	p, ok := e.Get(principalKey).(authz.Principal)
	if !ok {
		var err error
		p, err = ResolveRequest(e.App, d, e)
		if err != nil {
			return unauthorized(err)
		}
	}
	if authz.Can(d, p, a, r) {
		return nil
	}
	if p.Kind == authz.KindAnonymous && p.BoundInstance() == "" {
		return apis.NewUnauthorizedError("authentication required", nil)
	}
	return apis.NewForbiddenError("forbidden", nil)
}

// AuthorizeLAN is the one middleware for /api/lan/saves and /api/lan/sync
// (A.5): the credential comes from the PB JWT, an opaque key in
// X-LAN-Token / ?token= / Authorization: Bearer / X-Api-Key, or the raw
// legacy secret (no dot) in X-LAN-Token / ?token=. No credential or a bad
// one ⇒ 401 JSON {"error": ...}; verb(e) names the action + resource and a
// denial ⇒ 403 JSON. A verb returning an empty action defers the check to
// the handler (the /file route needs the record's owner first) — the caller
// is still required to be a resolved, non-anonymous principal.
func AuthorizeLAN(d *PBDeps, verb func(e *core.RequestEvent) (authz.Action, authz.Resource)) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		p, err := resolveEvent(e.App, d, e, lanCandidates(e))
		if err == nil && p.Kind == authz.KindAnonymous {
			err = ErrNoCredential
		}
		if err != nil {
			e.Set(principalKey, authz.Nobody())
			return e.JSON(http.StatusUnauthorized, map[string]string{
				"error": "LAN access denied: " + errText(err) + " — present a machine key (X-LAN-Token, ?token= or Authorization: Bearer) or authenticate as an admin",
			})
		}
		e.Set(principalKey, p)
		if verb == nil {
			return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		a, r := verb(e)
		if a == "" {
			return e.Next()
		}
		if !authz.Can(d, p, a, r) {
			return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		return e.Next()
	}
}

// RejectBannedAuth is the router-wide middleware (bound once in OnServe
// before any route group) that drops a verified users JWT whose row is
// banned or soft-deleted — the same usableUser re-read ResolveRequest
// applies — so a route guarded only by apis.RequireAuth() answers 401 the
// way an authz-guarded one does. Superusers and other auth collections pass
// untouched; the request itself continues (as a guest).
func RejectBannedAuth(e *core.RequestEvent) error {
	if e.Auth != nil && !e.Auth.IsSuperuser() &&
		e.Auth.Collection() != nil && e.Auth.Collection().Name == "users" {
		if _, ok := usableUser(e.App, e.Auth); !ok {
			e.Auth = nil
		}
	}
	return e.Next()
}

// unauthorized maps a resolver error to the 401 apis error.
func unauthorized(err error) error {
	return apis.NewUnauthorizedError(errText(err), nil)
}

// errText strips the "authz: " prefix for client-facing messages.
func errText(err error) string {
	if err == nil {
		return "unauthorized"
	}
	if errors.Is(err, ErrBanned) {
		return "account banned or deleted"
	}
	return strings.TrimPrefix(err.Error(), "authz: ")
}
