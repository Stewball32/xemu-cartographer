package lansaves

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
	"github.com/xemu-cartographer/xemu-cartographer/internal/halosave"
)

// TestLanVerbMap pins the group-relative path → LAN verb map the
// pb.AuthorizeLAN middleware consults (R-5). /file/… defers to the handler
// (empty action: the rule needs the record's owner first); anything not in
// the table maps to a verb no rule grants, so a new route is denied until it
// is added here.
func TestLanVerbMap(t *testing.T) {
	cases := []struct {
		path string
		want authz.Action
	}{
		{"/identity", authz.ActionLANSavesIdentity},
		{"/identity/", authz.ActionLANSavesIdentity},
		{"/identity/Stew", authz.ActionLANSavesIdentity},
		{"/file/h2-profile/abc", ""},
		{"/file/game/abc", ""},
		{"/build", authz.ActionLANSavesBuild},
		{"/download", authz.ActionLANSavesDownload},
		{"/manifest", authz.ActionLANSavesManifest},
		{"/meta", authz.ActionLANSavesMeta},
		{"/meta/", authz.ActionLANSavesMeta},
		{"", lanActionUnmapped},
		{"/", lanActionUnmapped},
		{"/identityx", lanActionUnmapped},
		{"/file", lanActionUnmapped},
		{"/nope", lanActionUnmapped},
	}
	for _, c := range cases {
		got, res := lanVerbFor(c.path)
		if got != c.want {
			t.Errorf("lanVerbFor(%q) = %q, want %q", c.path, got, c.want)
		}
		if got != "" && res.Kind != authz.ResGlobal {
			t.Errorf("lanVerbFor(%q) resource kind = %q, want global", c.path, res.Kind)
		}
	}
	// The unmapped sentinel must never be a granted verb.
	if lanActionUnmapped.Known() {
		t.Fatalf("%q must not be part of the action vocabulary", lanActionUnmapped)
	}
}

func TestSpecFromQueryCE(t *testing.T) {
	q, _ := url.ParseQuery("title=ce&kind=gametype&engine=slayer&name=TS+25&score_limit=25&respawn_seconds=7&lives=5&weapon_set=4&teams=1&radar=true&options=0x23&free_bytes=33554432&format=tar")
	req, tr := specFromQuery(q)
	if req.Title != "ce" || req.Kind != "gametype" || req.Engine != "slayer" || req.Name != "TS 25" {
		t.Fatalf("req strings: %+v", req)
	}
	if req.ScoreLimit == nil || *req.ScoreLimit != 25 {
		t.Errorf("score_limit not parsed")
	}
	if req.RespawnSeconds == nil || *req.RespawnSeconds != 7 {
		t.Errorf("respawn_seconds not parsed")
	}
	if req.Lives == nil || *req.Lives != 5 {
		t.Errorf("lives not parsed")
	}
	if req.WeaponSet == nil || *req.WeaponSet != 4 {
		t.Errorf("weapon_set not parsed")
	}
	if req.Teams == nil || !*req.Teams {
		t.Errorf("teams not parsed")
	}
	if req.Radar == nil || !*req.Radar {
		t.Errorf("radar not parsed")
	}
	if req.Options == nil || *req.Options != 0x23 {
		t.Errorf("options hex not parsed: %v", req.Options)
	}
	if !tr.HasFree || tr.FreeBytes == nil || *tr.FreeBytes != 33554432 {
		t.Errorf("free_bytes not parsed")
	}
	if tr.Format != "tar" {
		t.Errorf("format = %q", tr.Format)
	}
}

func TestSpecFromQueryH2Appearance(t *testing.T) {
	q, _ := url.ParseQuery("title=h2&kind=profile&name=CARTOG&app_armor_primary=7&app_emblem_foreground=3")
	req, _ := specFromQuery(q)
	if req.Appearance["armor_primary"] != 7 || req.Appearance["emblem_foreground"] != 3 {
		t.Errorf("appearance not parsed: %+v", req.Appearance)
	}
}

func TestArchiveTarLayout(t *testing.T) {
	set, err := halosave.Build(halosave.BuildRequest{Title: "ce", Kind: "gametype", Name: "TS 25", Engine: "slayer"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := archiveTar(set)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(bytes.NewReader(data))
	got := map[string]int64{}
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		got[h.Name] = h.Size
	}
	wantBlam := "UDATA/4d530004/G-TS 25/blam.lst"
	wantMeta := "UDATA/4d530004/G-TS 25/SaveMeta.xbx"
	if got[wantBlam] != 512 {
		t.Errorf("tar missing %q (size 512); entries=%v", wantBlam, got)
	}
	if _, ok := got[wantMeta]; !ok {
		t.Errorf("tar missing %q; entries=%v", wantMeta, got)
	}
}

func TestArchiveZipRoundTrips(t *testing.T) {
	set, _ := halosave.Build(halosave.BuildRequest{Title: "h2", Kind: "profile", Name: "CARTOG"})
	b, err := archiveZip(set)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("empty zip")
	}
}

func TestSelectFormat(t *testing.T) {
	set, _ := halosave.Build(halosave.BuildRequest{Title: "ce", Kind: "gametype", Name: "TS 25", Engine: "slayer"})
	// payload
	data, ct, fn, err := selectFormat(set, transport{Format: "payload"})
	if err != nil || fn != "blam.lst" || len(data) != 512 || ct != "application/octet-stream" {
		t.Errorf("payload: fn=%q ct=%q len=%d err=%v", fn, ct, len(data), err)
	}
	// savemeta
	_, _, fn, err = selectFormat(set, transport{Format: "savemeta"})
	if err != nil || fn != "SaveMeta.xbx" {
		t.Errorf("savemeta: fn=%q err=%v", fn, err)
	}
	// default = tar
	_, ct, fn, err = selectFormat(set, transport{})
	if err != nil || ct != "application/x-tar" || fn != "G-TS 25.tar" {
		t.Errorf("default: ct=%q fn=%q err=%v", ct, fn, err)
	}
	// bad format
	if _, _, _, err := selectFormat(set, transport{Format: "bogus"}); err == nil {
		t.Errorf("expected error for bad format")
	}
	// file by name
	_, _, fn, err = selectFormat(set, transport{Format: "file", File: "blam.lst"})
	if err != nil || fn != "blam.lst" {
		t.Errorf("file: fn=%q err=%v", fn, err)
	}
	if _, _, _, err := selectFormat(set, transport{Format: "file", File: "nope"}); err == nil {
		t.Errorf("expected error for missing file")
	}
}

func TestMakeBuildResponseFootprint(t *testing.T) {
	resp, err := makeBuildResponse(halosave.BuildRequest{Title: "ce", Kind: "gametype", Name: "TS 25", Engine: "slayer"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// 2 files (each <1 cluster) + 1 dir cluster = 3 * 16384.
	if resp.FootprintBytes != 3*16384 {
		t.Errorf("footprint = %d, want %d", resp.FootprintBytes, 3*16384)
	}
	if resp.TotalBytes <= 0 || resp.FATXCluster != 16384 {
		t.Errorf("totals: total=%d cluster=%d", resp.TotalBytes, resp.FATXCluster)
	}
}

// TestFileResourceStationBinding proves the lan.saves.file resource a served
// record yields — profile kinds carry their owner + the owner's sanitized
// gamertag, library kinds are unowned — and that the PD-8 station binding
// decides on it: a station key bound to gamertags may pull only the profiles
// filed under one of them, while unbound machine keys and non-profile kinds
// pass on scope alone.
func TestFileResourceStationBinding(t *testing.T) {
	app, d := pbtest.NewApp(t)
	profiles, gametypes := serveCollections(t, app)

	stew := pbtest.NewUser(t, app, "stew@example.com")
	stewTag := pbtest.NewTag(t, app, stew.Id, "Stew Ball", "approved")
	pbtest.SetField(t, app, "users", stew.Id, "default_gamertag", stewTag.Id)
	// A second usable tag that is not the default, and a pending one that a
	// station may not be bound under.
	pbtest.NewTag(t, app, stew.Id, "Stew Alt", "allowed")
	pbtest.NewTag(t, app, stew.Id, "Stew Pending", "pending")
	// The unmoderated free-text column must not widen a user who has real
	// gamertags rows (it is only the fallback when there are none).
	pbtest.SetField(t, app, "users", stew.Id, "gamertag", "Someone Else")
	// zed has no default_gamertag row — only the legacy text column.
	zed := pbtest.NewUser(t, app, "zed@example.com")
	pbtest.SetField(t, app, "users", zed.Id, "gamertag", " ZeD ")
	// nobody has neither.
	nobody := pbtest.NewUser(t, app, "nobody@example.com")

	newProfile := func(userID string) *core.Record {
		r := core.NewRecord(profiles)
		r.Set("user", userID)
		r.Set("save_bundle", "bundle.tar")
		if err := app.Save(r); err != nil {
			t.Fatalf("save profile: %v", err)
		}
		return r
	}
	stewProfile := newProfile(stew.Id)
	zedProfile := newProfile(zed.Id)
	nobodyProfile := newProfile(nobody.Id)
	gt := core.NewRecord(gametypes)
	gt.Set("save_bundle", "gt.tar")
	if err := app.Save(gt); err != nil {
		t.Fatalf("save gametype: %v", err)
	}

	// Resource shape: every usable tag (sorted, comma-joined), never the
	// pending one.
	res := fileResource(app, "h2-profile", stewProfile.Id, stewProfile)
	if res.Kind != authz.ResLANFile || res.ID != "h2-profile/"+stewProfile.Id || res.Owner != stew.Id {
		t.Fatalf("stew profile resource = %+v", res)
	}
	if res.Extra["kind"] != "h2-profile" || res.Extra["gamertag"] != "stew alt,stew ball" {
		t.Fatalf("stew profile extra = %v, want kind=h2-profile gamertag=\"stew alt,stew ball\"", res.Extra)
	}
	if got := fileResource(app, "h2-profile", zedProfile.Id, zedProfile).Extra["gamertag"]; got != "zed" {
		t.Errorf("legacy users.gamertag fallback = %q, want \"zed\"", got)
	}
	if got := fileResource(app, "h2-profile", nobodyProfile.Id, nobodyProfile).Extra["gamertag"]; got != "" {
		t.Errorf("user with no gamertag should yield \"\", got %q", got)
	}
	gtRes := fileResource(app, "gametype", gt.Id, gt)
	if gtRes.Owner != "" || gtRes.Extra["kind"] != "gametype" {
		t.Fatalf("gametype resource = %+v, want unowned", gtRes)
	}
	if _, has := gtRes.Extra["gamertag"]; has {
		t.Errorf("gametype resource must not carry a gamertag: %v", gtRes.Extra)
	}

	machine := func(scopes []string, gamertags string) authz.Principal {
		p := authz.Principal{Kind: authz.KindMachine, ID: "kid1", Scopes: scopes, Extra: map[string]string{"station_id": "st1"}}
		if gamertags != "" {
			p.Extra["gamertags"] = gamertags
		}
		return p
	}
	scoped := []string{"lan.saves.*"}
	cases := []struct {
		name string
		p    authz.Principal
		r    authz.Resource
		want bool
	}{
		{"bound station, own profile", machine(scoped, "stew ball"), res, true},
		{"bound station, own profile among several", machine(scoped, "other,stew ball"), res, true},
		{"bound station, own profile via non-default usable tag", machine(scoped, "stew alt"), res, true},
		{"bound station, own profile via pending tag", machine(scoped, "stew pending"), res, false},
		{"bound station, someone else's profile", machine(scoped, "zed"), res, false},
		{"bound station, profile with no gamertag", machine(scoped, "zed"), fileResource(app, "h2-profile", nobodyProfile.Id, nobodyProfile), false},
		{"unbound machine key, any profile", machine(scoped, ""), res, true},
		{"bound station, gametype", machine(scoped, "zed"), gtRes, true},
		{"bound station, no scope", machine(nil, "stew ball"), res, false},
		{"unbound machine key, no scope", machine(nil, ""), gtRes, false},
	}
	for _, c := range cases {
		if got := authz.Can(d, c.p, authz.ActionLANSavesFile, c.r); got != c.want {
			t.Errorf("%s: Can = %v, want %v", c.name, got, c.want)
		}
	}
}

// serveCollections adds the columns the /file handler reads that pbtest does
// not seed: users.gamertag (legacy text) + users.default_gamertag (relation)
// as the migration snapshot has them, plus h2_profiles and gametypes with
// their save_bundle file column.
func serveCollections(t *testing.T, app core.App) (profiles, gametypes *core.Collection) {
	t.Helper()
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("users collection: %v", err)
	}
	tagsCol, err := app.FindCollectionByNameOrId("gamertags")
	if err != nil {
		t.Fatalf("gamertags collection: %v", err)
	}
	users.Fields.Add(
		&core.TextField{Name: "gamertag", Max: 32},
		&core.RelationField{Name: "default_gamertag", CollectionId: tagsCol.Id, MaxSelect: 1},
	)
	if err := app.Save(users); err != nil {
		t.Fatalf("extend users: %v", err)
	}
	profiles = core.NewBaseCollection("h2_profiles")
	profiles.Fields.Add(
		&core.RelationField{Name: "user", CollectionId: users.Id, MaxSelect: 1, Required: true},
		&core.TextField{Name: "save_bundle"},
	)
	if err := app.Save(profiles); err != nil {
		t.Fatalf("save h2_profiles: %v", err)
	}
	gametypes = core.NewBaseCollection("gametypes")
	gametypes.Fields.Add(&core.TextField{Name: "save_bundle"})
	if err := app.Save(gametypes); err != nil {
		t.Fatalf("save gametypes: %v", err)
	}
	return profiles, gametypes
}

// TestServeFileStationBinding drives handleServeFile end to end with minted
// machine keys, the way a station pulls its profile bundle: a station bound
// to the owner's default tag, or to a non-default usable tag, streams the
// file (200 + bytes); one bound to another user's tag is refused with the
// LAN {"error":"forbidden"} 403 and no bytes; an unbound machine key is not
// gamertag-filtered at all. A key whose scopes do not cover lan.saves.file
// is refused before the row is loaded — the same 403 whether or not the id
// exists — while a covering key still gets the 404 for an unknown id.
func TestServeFileStationBinding(t *testing.T) {
	app, d := pbtest.NewApp(t)
	profiles, _ := serveCollections(t, app)

	stew := pbtest.NewUser(t, app, "stew@example.com")
	stewTag := pbtest.NewTag(t, app, stew.Id, "Stew Ball", "approved")
	pbtest.SetField(t, app, "users", stew.Id, "default_gamertag", stewTag.Id)
	stewAlt := pbtest.NewTag(t, app, stew.Id, "Stew Alt", "allowed")
	other := pbtest.NewUser(t, app, "other@example.com")
	otherTag := pbtest.NewTag(t, app, other.Id, "Other One", "approved")

	profile := core.NewRecord(profiles)
	profile.Set("user", stew.Id)
	profile.Set("save_bundle", "bundle.tar")
	if err := app.Save(profile); err != nil {
		t.Fatalf("save profile: %v", err)
	}
	const payload = "stew's bundle bytes"
	fsys, err := app.NewFilesystem()
	if err != nil {
		t.Fatalf("filesystem: %v", err)
	}
	if err := fsys.Upload([]byte(payload), profile.BaseFilesPath()+"/bundle.tar"); err != nil {
		t.Fatalf("upload bundle: %v", err)
	}
	fsys.Close()

	station := func(tagIDs ...string) string {
		_, tok := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"}, func(r *pb.MintRequest) {
			r.StationID = "st1"
			r.Gamertags = tagIDs
		})
		return tok
	}
	// A station holding only the manifest scope — it reaches the handler
	// (the group middleware defers /file/… to it) but may not pull files.
	_, metaOnly := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.meta"}, func(r *pb.MintRequest) {
		r.StationID = "st2"
		r.Gamertags = []string{stewTag.Id}
	})
	serve := func(token, id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/lan/saves/file/h2-profile/"+id, nil)
		req.SetPathValue("kind", "h2-profile")
		req.SetPathValue("id", id)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e := &core.RequestEvent{App: app, Event: router.Event{Request: req, Response: rec}}
		if err := handleServeFile(e); err != nil {
			t.Fatalf("handleServeFile returned %v", err)
		}
		return rec
	}

	cases := []struct {
		name     string
		token    string
		id       string
		wantCode int
		wantErr  string
	}{
		{"bound to owner's default tag", station(stewTag.Id), profile.Id, http.StatusOK, ""},
		{"bound to owner's non-default usable tag", station(stewAlt.Id), profile.Id, http.StatusOK, ""},
		{"bound to another user's tag", station(otherTag.Id), profile.Id, http.StatusForbidden, "forbidden"},
		{"unbound machine key", station(), profile.Id, http.StatusOK, ""},
		{"scoped key, unknown id", station(stewTag.Id), "nope", http.StatusNotFound, "record not found"},
		{"meta-only key, existing profile", metaOnly, profile.Id, http.StatusForbidden, "forbidden"},
		{"meta-only key, unknown id", metaOnly, "nope", http.StatusForbidden, "forbidden"},
	}
	for _, c := range cases {
		rec := serve(c.token, c.id)
		if rec.Code != c.wantCode {
			t.Errorf("%s: status = %d, want %d (body %q)", c.name, rec.Code, c.wantCode, rec.Body.String())
			continue
		}
		if c.wantCode == http.StatusOK {
			if rec.Body.String() != payload {
				t.Errorf("%s: body = %q, want the stored bundle", c.name, rec.Body.String())
			}
			continue
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != c.wantErr {
			t.Errorf("%s: body = %q, want {\"error\":%q}", c.name, rec.Body.String(), c.wantErr)
		}
	}
}

// TestLanDeniedShape pins the /file handler's denial body to the LAN group's
// {"error": ...} shape (the one pb.AuthorizeLAN and the other /api/lan
// handlers write): a 403 is the middleware's exact {"error":"forbidden"},
// other apis statuses keep their status + (PB-sentenized) message, and a
// non-apis error falls back to a plain 403.
func TestLanDeniedShape(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{"forbidden", apis.NewForbiddenError("forbidden", nil), http.StatusForbidden, "forbidden"},
		{"unauthorized", apis.NewUnauthorizedError("authentication required", nil), http.StatusUnauthorized, "Authentication required."},
		{"opaque error", errors.New("boom"), http.StatusForbidden, "forbidden"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		e := &core.RequestEvent{Event: router.Event{
			Request:  httptest.NewRequest(http.MethodGet, "/api/lan/saves/file/h2-profile/x", nil),
			Response: rec,
		}}
		if err := lanDenied(e, c.err); err != nil {
			t.Fatalf("%s: lanDenied returned %v", c.name, err)
		}
		if rec.Code != c.wantCode {
			t.Errorf("%s: status = %d, want %d", c.name, rec.Code, c.wantCode)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: body %q: %v", c.name, rec.Body.String(), err)
		}
		if len(body) != 1 || body["error"] != c.wantMsg {
			t.Errorf("%s: body = %v, want {\"error\": %q}", c.name, body, c.wantMsg)
		}
	}
}
