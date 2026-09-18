package tokens

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
)

// call runs a handler over a synthetic event and returns the status + JSON
// body. apis.ApiError returns are folded into their status code.
func call(t *testing.T, app core.App, auth *core.Record, method, target, body string, hdr map[string]string, h func(*core.RequestEvent) error) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e := &core.RequestEvent{
		App:   app,
		Auth:  auth,
		Event: router.Event{Request: req, Response: rec},
	}
	err := h(e)
	if err != nil {
		var apiErr *router.ApiError
		if errors.As(err, &apiErr) {
			return apiErr.Status, map[string]any{"message": apiErr.Message}
		}
		t.Fatalf("handler returned a non-API error: %v", err)
	}
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("body %q: %v", rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

func TestMintSpectatorRequiresOverlayMint(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	body := `{"kind":"spectator","label":"obs","scopes":["overlay.read_state:xc-1"]}`

	// No credential ⇒ 401.
	if code, _ := call(t, app, nil, http.MethodPost, "/api/admin/tokens", body, nil, handleMint); code != http.StatusUnauthorized {
		t.Fatalf("anonymous mint: %d", code)
	}
	// A member (no overlay.mint) ⇒ 403.
	member := pbtest.NewUser(t, app, "m@test.dev")
	pbtest.GrantRole(t, app, member.Id, "member")
	if code, _ := call(t, app, member, http.MethodPost, "/api/admin/tokens", body, nil, handleMint); code != http.StatusForbidden {
		t.Fatalf("member mint: %d", code)
	}
	// An overlay_manager holds overlay.mint ⇒ 201, token shown once.
	om := pbtest.NewUser(t, app, "om@test.dev")
	pbtest.GrantRole(t, app, om.Id, "overlay_manager")
	code, out := call(t, app, om, http.MethodPost, "/api/admin/tokens", body, nil, handleMint)
	if code != http.StatusCreated {
		t.Fatalf("overlay_manager mint: %d %v", code, out)
	}
	kid, _ := out["kid"].(string)
	token, _ := out["token"].(string)
	if !strings.HasPrefix(kid, "sp_") || !strings.HasPrefix(token, kid+".") || out["kind"] != "spectator" || out["label"] != "obs" {
		t.Fatalf("mint response = %v", out)
	}
	if exp, _ := out["expires_at"].(string); exp == "" {
		t.Fatalf("spectator key must carry a default expiry: %v", out)
	}
	if _, leaked := out["key_hash"]; leaked {
		t.Fatalf("response leaks key_hash")
	}
	// The exact body Studio sends (tokens-api.ts spectatorScopesFor): the
	// seed overlay_manager holds no room.join scope, overlay.mint covers the
	// four overlay rooms of the instance (PD-5).
	studio := `{"kind":"spectator","label":"obs xc-1","scopes":["overlay.read_state:xc-1","room.join:host:xc-1:game_filtered","room.join:host:xc-1:tick","room.join:host:xc-1:scenario","room.join:host:xc-1:event_filtered"]}`
	if code, out := call(t, app, om, http.MethodPost, "/api/admin/tokens", studio, nil, handleMint); code != http.StatusCreated {
		t.Fatalf("overlay_manager Studio mint: %d %v", code, out)
	}
	// ... and no other class: the delegation ceiling still applies.
	if code, out := call(t, app, om, http.MethodPost, "/api/admin/tokens", `{"kind":"spectator","scopes":["overlay.read_state:xc-1","room.join:host:xc-1:game"]}`, nil, handleMint); code != http.StatusForbidden {
		t.Fatalf("overlay_manager unfiltered class mint: %d %v", code, out)
	}
	// overlay.mint does not extend to machine keys.
	if code, _ := call(t, app, om, http.MethodPost, "/api/admin/tokens", `{"kind":"machine","scopes":["lan.*"]}`, nil, handleMint); code != http.StatusForbidden {
		t.Fatalf("overlay_manager machine mint: %d", code)
	}
	// Validation failures are 400 with {"error"}.
	code, out = call(t, app, om, http.MethodPost, "/api/admin/tokens", `{"kind":"spectator","scopes":["overlay.*"]}`, nil, handleMint)
	if code != http.StatusBadRequest || out["error"] == "" {
		t.Fatalf("wildcard spectator: %d %v", code, out)
	}
	code, out = call(t, app, om, http.MethodPost, "/api/admin/tokens", `{"kind":"spectator","scopes":["overlay.read_state:xc-1"],"expires_at":"2000-01-01 00:00:00.000Z"}`, nil, handleMint)
	if code != http.StatusBadRequest || out["error"] == "" {
		t.Fatalf("past expiry: %d %v", code, out)
	}
	code, out = call(t, app, om, http.MethodPost, "/api/admin/tokens", `{"kind":"spectator","scopes":["overlay.read_state:xc-1"],"expires_at":"not a date"}`, nil, handleMint)
	if code != http.StatusBadRequest || out["error"] == "" {
		t.Fatalf("bad expiry: %d %v", code, out)
	}
	if code, _ := call(t, app, om, http.MethodPost, "/api/admin/tokens", `{"kind":"robot","scopes":["x"]}`, nil, handleMint); code != http.StatusBadRequest {
		t.Fatalf("unknown kind: %d", code)
	}
}

func TestMintMachineRequiresTokenMint(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	body := `{"kind":"machine","label":"station","scopes":["lan.saves.*","lan.sync.*"],"station_id":"s1"}`

	member := pbtest.NewUser(t, app, "m@test.dev")
	pbtest.GrantRole(t, app, member.Id, "member")
	if code, _ := call(t, app, member, http.MethodPost, "/api/admin/tokens", body, nil, handleMint); code != http.StatusForbidden {
		t.Fatalf("member machine mint: %d", code)
	}
	om := pbtest.NewUser(t, app, "om@test.dev")
	pbtest.GrantRole(t, app, om.Id, "overlay_manager")
	if code, _ := call(t, app, om, http.MethodPost, "/api/admin/tokens", body, nil, handleMint); code != http.StatusForbidden {
		t.Fatalf("overlay_manager machine mint: %d", code)
	}

	admin := pbtest.NewUser(t, app, "a@test.dev")
	pbtest.GrantRole(t, app, admin.Id, "admin")
	code, out := call(t, app, admin, http.MethodPost, "/api/admin/tokens", body, nil, handleMint)
	if code != http.StatusCreated {
		t.Fatalf("admin machine mint: %d %v", code, out)
	}
	kid, _ := out["kid"].(string)
	token, _ := out["token"].(string)
	if !strings.HasPrefix(kid, "mk_") || out["expires_at"] != "" {
		t.Fatalf("machine mint response = %v", out)
	}
	if got := pbtest.TokenRecord(t, app, kid); got.GetString("minted_by") != admin.Id || got.GetString("station_id") != "s1" {
		t.Fatalf("stored row minted_by=%q station=%q", got.GetString("minted_by"), got.GetString("station_id"))
	}

	// A machine key carrying token.* may mint through the same route — the
	// scopes it holds itself. Anything wider is 403 {error} (the delegation
	// ceiling): token.mint:machine alone cannot mint "*" and then list every
	// key through the child.
	_, adminKey := pbtest.MintToken(t, app, pb.Default(), "machine", []string{"token.*", "lan.*"})
	code, out = call(t, app, nil, http.MethodPost, "/api/admin/tokens", body, map[string]string{"Authorization": "Bearer " + adminKey}, handleMint)
	if code != http.StatusCreated {
		t.Fatalf("machine key mint: %d %v", code, out)
	}
	_, narrowKey := pbtest.MintToken(t, app, pb.Default(), "machine", []string{"token.mint:machine"})
	for _, scopes := range []string{`["*"]`, `["lan.saves.*","lan.sync.*"]`, `["token.list"]`} {
		code, out = call(t, app, nil, http.MethodPost, "/api/admin/tokens", `{"kind":"machine","scopes":`+scopes+`}`, map[string]string{"Authorization": "Bearer " + narrowKey}, handleMint)
		if code != http.StatusForbidden || !strings.HasPrefix(out["error"].(string), "scope exceeds the caller's own") {
			t.Fatalf("narrow machine key minting %s: %d %v", scopes, code, out)
		}
	}
	if code, out := call(t, app, nil, http.MethodPost, "/api/admin/tokens", `{"kind":"machine","scopes":["token.mint:machine"]}`, map[string]string{"Authorization": "Bearer " + narrowKey}, handleMint); code != http.StatusCreated {
		t.Fatalf("narrow machine key minting its own scope: %d %v", code, out)
	}

	// Superuser ⇒ 201.
	su := pbtest.NewSuperuser(t, app, "root@test.dev")
	if code, out := call(t, app, su, http.MethodPost, "/api/admin/tokens", body, nil, handleMint); code != http.StatusCreated {
		t.Fatalf("superuser mint: %d %v", code, out)
	}

	// List: token.list required; never the hash.
	if code, _ := call(t, app, member, http.MethodGet, "/api/admin/tokens", "", nil, handleList); code != http.StatusForbidden {
		t.Fatalf("member list: %d", code)
	}
	code, out = call(t, app, admin, http.MethodGet, "/api/admin/tokens?kind=machine", "", nil, handleList)
	if code != http.StatusOK {
		t.Fatalf("admin list: %d %v", code, out)
	}
	list, _ := out["tokens"].([]any)
	if len(list) != 6 {
		t.Fatalf("listed %d machine keys, want 6", len(list))
	}
	for _, item := range list {
		row := item.(map[string]any)
		if _, leaked := row["key_hash"]; leaked {
			t.Fatalf("list leaks key_hash: %v", row)
		}
		if _, leaked := row["token"]; leaked {
			t.Fatalf("list leaks token: %v", row)
		}
	}
	if code, _ := call(t, app, admin, http.MethodGet, "/api/admin/tokens?kind=robot", "", nil, handleList); code != http.StatusBadRequest {
		t.Fatalf("bad kind filter: %d", code)
	}

	// Revoke: token.revoke on Token(kid); 404 unknown; 409 legacy; the key
	// stops working.
	revoke := func(auth *core.Record, kidPath, body string, hdr map[string]string) (int, map[string]any) {
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/tokens/"+kidPath, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		req.SetPathValue("kid", kidPath)
		rec := httptest.NewRecorder()
		e := &core.RequestEvent{App: app, Auth: auth, Event: router.Event{Request: req, Response: rec}}
		if err := handleRevoke(e); err != nil {
			var apiErr *router.ApiError
			if errors.As(err, &apiErr) {
				return apiErr.Status, nil
			}
			t.Fatalf("revoke: %v", err)
		}
		out := map[string]any{}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	if code, _ := revoke(member, kid, "", nil); code != http.StatusForbidden {
		t.Fatalf("member revoke: %d", code)
	}
	if code, _ := revoke(nil, kid, "", nil); code != http.StatusUnauthorized {
		t.Fatalf("anonymous revoke: %d", code)
	}
	if code, out := revoke(admin, "mk_missing", "", nil); code != http.StatusNotFound || out["error"] == "" {
		t.Fatalf("unknown kid: %d %v", code, out)
	}
	pb.ImportLegacyEnv(pb.Default(), func(string) string { return "legacy" })
	if code, out := revoke(admin, "legacy-env", "", nil); code != http.StatusConflict || out["error"] != "unset LAN_SAVES_TOKEN instead" {
		t.Fatalf("legacy revoke: %d %v", code, out)
	}
	code, out = revoke(admin, kid, `{"reason":"rotated"}`, nil)
	if code != http.StatusOK || out["kid"] != kid || out["revoked"] != true {
		t.Fatalf("revoke: %d %v", code, out)
	}
	if code, _ := call(t, app, nil, http.MethodPost, "/api/admin/tokens", body, map[string]string{"Authorization": "Bearer " + token}, handleMint); code != http.StatusUnauthorized {
		t.Fatalf("revoked key still mints: %d", code)
	}
	// Idempotent.
	if code, _ := revoke(admin, kid, "", nil); code != http.StatusOK {
		t.Fatalf("second revoke: %d", code)
	}
	code, out = call(t, app, admin, http.MethodGet, "/api/admin/tokens?kind=machine", "", nil, handleList)
	if code != http.StatusOK {
		t.Fatalf("list after revoke: %d", code)
	}
	found := false
	for _, item := range out["tokens"].([]any) {
		row := item.(map[string]any)
		if row["kid"] == kid {
			found = true
			if row["revoked"] != true || row["revoked_at"] == "" || row["minted_by"] != admin.Id {
				t.Fatalf("revoked row = %v", row)
			}
		}
	}
	if !found {
		t.Fatalf("revoked kid missing from the listing")
	}
}

// TestMintCredentialCheckedBeforeBody pins the fail-closed order of
// handleMint: an anonymous or badly-credentialed caller gets 401 before the
// body is validated, so the "kind must be one of ..." / "invalid body" 400s
// are only ever reachable by a resolved principal.
func TestMintCredentialCheckedBeforeBody(t *testing.T) {
	app, _ := pbtest.NewApp(t)

	for name, body := range map[string]string{
		"empty object": `{}`,
		"unknown kind": `{"kind":"robot","scopes":["x"]}`,
		"malformed":    `{"kind":`,
	} {
		if code, _ := call(t, app, nil, http.MethodPost, "/api/admin/tokens", body, nil, handleMint); code != http.StatusUnauthorized {
			t.Fatalf("anonymous %s: %d, want 401", name, code)
		}
		code, out := call(t, app, nil, http.MethodPost, "/api/admin/tokens", body, map[string]string{"Authorization": "Bearer mk_bogus.secret"}, handleMint)
		if code != http.StatusUnauthorized {
			t.Fatalf("bogus key %s: %d, want 401", name, code)
		}
		if msg, _ := out["message"].(string); msg == "" || strings.HasPrefix(msg, "authz: ") {
			t.Fatalf("bogus key %s: message %q should be the resolver's text without the authz: prefix", name, msg)
		}
	}

	// A resolved principal still gets the body validation 400 (and the
	// per-kind 403 when it lacks the scope).
	member := pbtest.NewUser(t, app, "m@test.dev")
	pbtest.GrantRole(t, app, member.Id, "member")
	if code, _ := call(t, app, member, http.MethodPost, "/api/admin/tokens", `{}`, nil, handleMint); code != http.StatusBadRequest {
		t.Fatalf("member empty body: %d, want 400", code)
	}
	if code, _ := call(t, app, member, http.MethodPost, "/api/admin/tokens", `{"kind":"machine","scopes":["lan.*"]}`, nil, handleMint); code != http.StatusForbidden {
		t.Fatalf("member machine mint: %d, want 403", code)
	}
}
