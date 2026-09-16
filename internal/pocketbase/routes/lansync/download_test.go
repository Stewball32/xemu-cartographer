package lansync

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
)

// TestGameDownloadStationFilter drives handleGameDownload with minted machine
// keys and proves /dl/game/{id} applies the same PD-9 predicate as the
// manifest: a station (key bound to a station_id) gets the cleared disc but
// a 404 — indistinguishable from an unknown id — for a server-role disc it
// could only reach by guessing the id; an unbound peer key downloads both;
// a drifted disc is a 404 for everyone.
func TestGameDownloadStationFilter(t *testing.T) {
	app, d := pbtest.NewApp(t)
	isos := isosTestCollection(t, app)

	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, "default.xbe"), []byte("xbe"), 0o644); err != nil {
		t.Fatalf("write tree: %v", err)
	}
	newISO := func(name, role string, allowOnXbox, drift bool) string {
		r := core.NewRecord(isos)
		r.Set("name", name)
		r.Set("dest_name", name)
		r.Set("role", role)
		r.Set("allow_on_xbox", allowOnXbox)
		r.Set("drift_detected", drift)
		r.Set("extracted_ready", true)
		r.Set("extracted_path", tree)
		if err := app.Save(r); err != nil {
			t.Fatalf("save iso %q: %v", name, err)
		}
		return r.Id
	}
	cleared := newISO("Cleared", "play", true, false)
	serverBuild := newISO("ServerBuild", "server", true, false)
	drifted := newISO("Drifted", "play", true, true)

	scopes := []string{"lan.sync.*"}
	_, peer := pbtest.MintToken(t, app, d, "machine", scopes)
	_, station := pbtest.MintToken(t, app, d, "machine", scopes, func(r *pb.MintRequest) {
		r.StationID = "st1"
	})

	download := func(token, id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/lan/sync/dl/game/"+id, nil)
		req.SetPathValue("id", id)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e := &core.RequestEvent{App: app, Event: router.Event{Request: req, Response: rec}}
		if err := handleGameDownload(e); err != nil {
			t.Fatalf("handleGameDownload returned %v", err)
		}
		return rec
	}

	cases := []struct {
		name     string
		token    string
		id       string
		wantCode int
	}{
		{"peer, cleared disc", peer, cleared, http.StatusOK},
		{"peer, server build", peer, serverBuild, http.StatusOK},
		{"peer, drifted", peer, drifted, http.StatusNotFound},
		{"station, cleared disc", station, cleared, http.StatusOK},
		{"station, server build", station, serverBuild, http.StatusNotFound},
		{"station, drifted", station, drifted, http.StatusNotFound},
		{"station, unknown id", station, "nope", http.StatusNotFound},
	}
	for _, c := range cases {
		rec := download(c.token, c.id)
		if rec.Code != c.wantCode {
			t.Errorf("%s: status = %d, want %d (body %q)", c.name, rec.Code, c.wantCode, rec.Body.String())
			continue
		}
		if c.wantCode == http.StatusOK {
			if ct := rec.Header().Get("Content-Type"); ct != "application/x-tar" {
				t.Errorf("%s: content-type = %q, want application/x-tar", c.name, ct)
			}
			continue
		}
		if !strings.Contains(rec.Body.String(), "game record not found") {
			t.Errorf("%s: body = %q, want the not-found shape", c.name, rec.Body.String())
		}
	}
}
