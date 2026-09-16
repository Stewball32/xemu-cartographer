package pb_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
)

// runLAN runs the AuthorizeLAN middleware over a bare event and reports the
// status the recorder saw (200 = passed through to Next).
func runLAN(t *testing.T, d *pb.PBDeps, e *core.RequestEvent, verb func(*core.RequestEvent) (authz.Action, authz.Resource)) (int, string) {
	t.Helper()
	if err := pb.AuthorizeLAN(d, verb)(e); err != nil {
		t.Fatalf("AuthorizeLAN returned %v", err)
	}
	rec := e.Response.(*httptest.ResponseRecorder)
	return rec.Code, rec.Body.String()
}

func lanBuild(*core.RequestEvent) (authz.Action, authz.Resource) {
	return authz.ActionLANSavesBuild, authz.Global()
}

func TestImportLegacyEnv(t *testing.T) {
	app, d := pbtest.NewApp(t)
	env := map[string]string{}
	getenv := func(k string) string { return env[k] }

	if pb.ImportLegacyEnv(d, getenv) || d.LegacyImported() {
		t.Fatalf("unset env must not import")
	}
	if _, ok := d.LookupToken(authz.LegacyKid); ok {
		t.Fatalf("legacy row present before import")
	}

	env[pb.LegacyEnvVar] = "  s3cret-lan  "
	if !pb.ImportLegacyEnv(d, getenv) || !d.LegacyImported() {
		t.Fatalf("set env must import")
	}
	row, ok := d.LookupToken(authz.LegacyKid)
	if !ok {
		t.Fatalf("legacy row missing after import")
	}
	if row.Kind != "machine" || row.KeyHash != authz.HashSecret("s3cret-lan") || row.Revoked || !row.ExpiresAt.IsZero() {
		t.Fatalf("legacy row = %+v", row)
	}
	if got := strings.Join(row.Scopes, ","); got != "lan.saves.*,lan.sync.*" {
		t.Fatalf("legacy scopes = %q", got)
	}

	// The raw secret is accepted on the LAN carriers only …
	for _, mk := range []func() *core.RequestEvent{
		func() *core.RequestEvent {
			return newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"X-LAN-Token": "s3cret-lan"})
		},
		func() *core.RequestEvent {
			return newEvent(app, nil, http.MethodGet, "/api/lan/saves/build?token=s3cret-lan", nil)
		},
		func() *core.RequestEvent {
			return newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"Authorization": "Bearer legacy-env.s3cret-lan"})
		},
	} {
		e := mk()
		if code, body := runLAN(t, d, e, lanBuild); code != http.StatusOK {
			t.Fatalf("legacy carrier %s: %d %s", e.Request.URL, code, body)
		}
		p := pb.Get(e)
		if p.Kind != authz.KindMachine || p.ID != authz.LegacyKid {
			t.Fatalf("legacy principal = %+v", p)
		}
	}
	// … and a raw secret on Authorization is not.
	e := newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"Authorization": "Bearer s3cret-lan"})
	if code, _ := runLAN(t, d, e, lanBuild); code != http.StatusUnauthorized {
		t.Fatalf("raw secret on Authorization: %d", code)
	}
	// Wrong secret ⇒ 401 with the JSON error shape.
	e = newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"X-LAN-Token": "wrong"})
	code, body := runLAN(t, d, e, lanBuild)
	if code != http.StatusUnauthorized {
		t.Fatalf("wrong secret: %d %s", code, body)
	}
	var msg map[string]string
	if err := json.Unmarshal([]byte(body), &msg); err != nil || !strings.HasPrefix(msg["error"], "LAN access denied: ") {
		t.Fatalf("401 body = %s", body)
	}
	// No credential ⇒ 401.
	e = newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", nil)
	if code, _ := runLAN(t, d, e, lanBuild); code != http.StatusUnauthorized {
		t.Fatalf("no credential: %d", code)
	}
	// Legacy scopes do not reach token.*: 403.
	e = newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"X-LAN-Token": "s3cret-lan"})
	code, _ = runLAN(t, d, e, func(*core.RequestEvent) (authz.Action, authz.Resource) {
		return authz.ActionTokenList, authz.Global()
	})
	if code != http.StatusForbidden {
		t.Fatalf("legacy key on token.list: %d", code)
	}
	// An empty action defers to the handler but still requires a credential.
	e = newEvent(app, nil, http.MethodGet, "/api/lan/saves/file/x", map[string]string{"X-LAN-Token": "s3cret-lan"})
	if code, _ := runLAN(t, d, e, func(*core.RequestEvent) (authz.Action, authz.Resource) { return "", authz.Resource{} }); code != http.StatusOK {
		t.Fatalf("deferred verb: %d", code)
	}
	e = newEvent(app, nil, http.MethodGet, "/api/lan/saves/file/x", nil)
	if code, _ := runLAN(t, d, e, func(*core.RequestEvent) (authz.Action, authz.Resource) { return "", authz.Resource{} }); code != http.StatusUnauthorized {
		t.Fatalf("deferred verb without credential: %d", code)
	}
	// An admin JWT passes the LAN middleware too.
	admin := pbtest.NewUser(t, app, "admin@test.dev")
	pbtest.GrantRole(t, app, admin.Id, "admin")
	e = newEvent(app, admin, http.MethodGet, "/api/lan/saves/build", nil)
	if code, body := runLAN(t, d, e, lanBuild); code != http.StatusOK {
		t.Fatalf("admin JWT: %d %s", code, body)
	}
	// A member JWT is a credential but has no lan.* scope ⇒ 403.
	member := pbtest.NewUser(t, app, "member@test.dev")
	pbtest.GrantRole(t, app, member.Id, "member")
	e = newEvent(app, member, http.MethodGet, "/api/lan/saves/build", nil)
	if code, _ := runLAN(t, d, e, lanBuild); code != http.StatusForbidden {
		t.Fatalf("member JWT: %d", code)
	}

	// Blanking the env clears the row and the raw secret stops working.
	env[pb.LegacyEnvVar] = ""
	if pb.ImportLegacyEnv(d, getenv) || d.LegacyImported() {
		t.Fatalf("blank env must clear the import")
	}
	e = newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"X-LAN-Token": "s3cret-lan"})
	if code, _ := runLAN(t, d, e, lanBuild); code != http.StatusUnauthorized {
		t.Fatalf("cleared legacy secret still accepted: %d", code)
	}
	if pb.ImportLegacyEnv(nil, getenv) {
		t.Fatalf("nil deps must not import")
	}
}

func TestLegacyEnvWithDot(t *testing.T) {
	app, d := pbtest.NewApp(t)
	// A real key on the LAN carriers: a bad secret for a kid that exists must
	// stay an error, never fall back to the whole-value compare.
	kid, token := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"})

	// Whatever the operator put in LAN_SAVES_TOKEN was the whole value the
	// pre-authz middleware compared, dots included — even one that happens
	// to start like an opaque kid.
	for _, secret := range []string{"lan.secret.v2", "lan.v2", "mk_x.y"} {
		env := secret
		pb.ImportLegacyEnv(d, func(string) string { return env })

		for _, e := range []*core.RequestEvent{
			newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"X-LAN-Token": secret}),
			newEvent(app, nil, http.MethodGet, "/api/lan/saves/build?token="+secret, nil),
		} {
			if code, body := runLAN(t, d, e, lanBuild); code != http.StatusOK {
				t.Fatalf("legacy %q on %s: %d %s", secret, e.Request.URL, code, body)
			}
			if p := pb.Get(e); p.Kind != authz.KindMachine || p.ID != authz.LegacyKid {
				t.Fatalf("legacy %q principal = %+v", secret, p)
			}
		}
		// Wrong dotted value ⇒ 401.
		e := newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"X-LAN-Token": secret + "x"})
		if code, _ := runLAN(t, d, e, lanBuild); code != http.StatusUnauthorized {
			t.Fatalf("wrong dotted secret %q: %d", secret+"x", code)
		}
		e = newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"X-LAN-Token": "mk_x.z"})
		if code, _ := runLAN(t, d, e, lanBuild); code != http.StatusUnauthorized {
			t.Fatalf("unknown kid %q: %d", "mk_x.z", code)
		}
		// The raw value on Authorization is still refused.
		e = newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"Authorization": "Bearer " + secret})
		if code, _ := runLAN(t, d, e, lanBuild); code != http.StatusUnauthorized {
			t.Fatalf("raw dotted secret %q on Authorization: %d", secret, code)
		}
		// A known kid with a bad secret is not rescued by the fallback.
		e = newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"X-LAN-Token": kid + ".wrong"})
		if code, _ := runLAN(t, d, e, lanBuild); code != http.StatusUnauthorized {
			t.Fatalf("bad secret on a known kid: %d", code)
		}
		e = newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"X-LAN-Token": token})
		if code, _ := runLAN(t, d, e, lanBuild); code != http.StatusOK {
			t.Fatalf("real key: %d", code)
		}
		if p := pb.Get(e); p.ID != kid {
			t.Fatalf("real key principal = %+v", p)
		}
	}
	// Without an imported row the fallback does not exist.
	pb.ImportLegacyEnv(d, func(string) string { return "" })
	e := newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", map[string]string{"X-LAN-Token": "mk_x.y"})
	if code, _ := runLAN(t, d, e, lanBuild); code != http.StatusUnauthorized {
		t.Fatalf("unknown kid with no legacy row: %d", code)
	}
}
