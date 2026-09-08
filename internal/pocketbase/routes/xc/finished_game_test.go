package xc

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
)

// captureRestamp swaps the restamp scheduler for one that hands the
// scheduled func back to the test; restored on cleanup.
func captureRestamp(t *testing.T) *func() {
	t.Helper()
	orig := restampAfter
	var fire func()
	restampAfter = func(d time.Duration, fn func()) {
		if d != RestampDelay {
			t.Errorf("restamp delay = %v, want %v", d, RestampDelay)
		}
		fire = fn
	}
	t.Cleanup(func() { restampAfter = orig })
	return &fire
}

func TestFinishedGameAuth(t *testing.T) {
	app, d := pbtest.NewApp(t)
	ensureGameCollections(t, app)
	mux := newMux(t, app)
	body := marshal(t, fixture(t))
	captureRestamp(t)

	// 401: no credential at all.
	if code, _ := post(t, mux, body, nil); code != http.StatusUnauthorized {
		t.Errorf("no key: status = %d, want 401", code)
	}
	// 401: an opaque key nobody minted.
	if code, _ := post(t, mux, body, map[string]string{"Authorization": "Bearer mk_nobody.secret"}); code != http.StatusUnauthorized {
		t.Errorf("unknown key: status = %d, want 401", code)
	}
	// 403: a machine key without the scope (scraper.state is a sibling
	// action, not the family wildcard).
	_, narrow := pbtest.MintToken(t, app, d, "machine", []string{"scraper.state:*", "lan.*"})
	if code, _ := post(t, mux, body, map[string]string{"Authorization": "Bearer " + narrow}); code != http.StatusForbidden {
		t.Errorf("wrong scope: status = %d, want 403", code)
	}
	// 200: a minted scraper.ingest machine key (the documented way).
	_, ingest := pbtest.MintToken(t, app, d, "machine", []string{"scraper.ingest"})
	code, out := post(t, mux, body, map[string]string{"Authorization": "Bearer " + ingest})
	if code != http.StatusOK || out["game_id"] == "" || out["series_id"] == "" || out["deduped"] != false {
		t.Fatalf("ingest key: status = %d body = %v", code, out)
	}
	// 200 deduped: the same uid again under the admin family wildcard.
	_, admin := pbtest.MintToken(t, app, d, "machine", []string{"scraper.*"})
	code, again := post(t, mux, body, map[string]string{"Authorization": "Bearer " + admin, "Idempotency-Key": "01J6Y2B5R8ZK3Q7W9XV0M4N2PA"})
	if code != http.StatusOK || again["deduped"] != true || again["game_id"] != out["game_id"] || again["series_id"] != out["series_id"] {
		t.Fatalf("dedupe: status = %d body = %v (first %v)", code, again, out)
	}
	if n, err := app.CountRecords("games"); err != nil || n != 1 {
		t.Errorf("games rows = %d (%v), want 1", n, err)
	}
}

func TestFinishedGameEnvKey(t *testing.T) {
	app, d := pbtest.NewApp(t)
	ensureGameCollections(t, app)
	mux := newMux(t, app)
	body := marshal(t, fixture(t))
	captureRestamp(t)

	if !pb.ImportWebhookEnv(d, func(k string) string {
		if k == pb.WebhookEnvVar {
			return "hook-s3cret"
		}
		return ""
	}) {
		t.Fatalf("import failed")
	}
	// The daemon presents the env value verbatim as a Bearer.
	code, out := post(t, mux, body, map[string]string{"Authorization": "Bearer hook-s3cret"})
	if code != http.StatusOK || out["deduped"] != false {
		t.Fatalf("env key: status = %d body = %v", code, out)
	}
	if code, _ := post(t, mux, body, map[string]string{"Authorization": "Bearer hook-wrong"}); code != http.StatusUnauthorized {
		t.Errorf("wrong env secret: status = %d, want 401", code)
	}
	// Unset ⇒ the raw value is no credential again.
	pb.ImportWebhookEnv(d, func(string) string { return "" })
	if code, _ := post(t, mux, body, map[string]string{"Authorization": "Bearer hook-s3cret"}); code != http.StatusUnauthorized {
		t.Errorf("after unset: status = %d, want 401", code)
	}
}

func TestFinishedGameValidation(t *testing.T) {
	app, d := pbtest.NewApp(t)
	ensureGameCollections(t, app)
	mux := newMux(t, app)
	_, key := pbtest.MintToken(t, app, d, "machine", []string{"scraper.ingest"})
	auth := func(extra map[string]string) map[string]string {
		h := map[string]string{"Authorization": "Bearer " + key}
		for k, v := range extra {
			h[k] = v
		}
		return h
	}
	fire := captureRestamp(t)

	cases := []struct {
		name   string
		mutate func(m map[string]any)
		hdr    map[string]string
		status int
		code   string
	}{
		{"bad_schema", func(m map[string]any) { m["schema"] = "xc.finished_game/2" }, nil, 400, "bad_schema"},
		{"missing_schema", func(m map[string]any) { delete(m, "schema") }, nil, 400, "bad_schema"},
		{"bad_uid", func(m map[string]any) { m["game_uid"] = "  " }, nil, 400, "bad_uid"},
		{"idempotency_mismatch", nil, map[string]string{"Idempotency-Key": "other-uid"}, 400, "idempotency_mismatch"},
	}
	for _, tc := range cases {
		m := fixture(t)
		if tc.mutate != nil {
			tc.mutate(m)
		}
		code, out := post(t, mux, marshal(t, m), auth(tc.hdr))
		if code != tc.status || out["code"] != tc.code {
			t.Errorf("%s: status = %d body = %v, want %d/%s", tc.name, code, out, tc.status, tc.code)
		}
	}
	if code, out := post(t, mux, []byte(`{"schema":`), auth(nil)); code != 400 || out["code"] != "bad_json" {
		t.Errorf("malformed: status = %d body = %v", code, out)
	}
	// 413: a body over MaxBody is rejected by the route's own reader (the
	// router-wide limit is far larger) and parks on the daemon side.
	m := fixture(t)
	m["pad"] = strings.Repeat("x", MaxBody)
	if code, out := post(t, mux, marshal(t, m), auth(nil)); code != http.StatusRequestEntityTooLarge || out["code"] != "too_large" {
		t.Errorf("too large: status = %d body = %v", code, out)
	}
	// None of the rejections persisted or scheduled anything.
	if n, _ := app.CountRecords("games"); n != 0 {
		t.Errorf("games rows after rejections = %d, want 0", n)
	}
	if *fire != nil {
		t.Errorf("a rejected body scheduled a restamp")
	}
}

func TestFinishedGameRestampsLateEvents(t *testing.T) {
	app, d := pbtest.NewApp(t)
	ensureGameCollections(t, app)
	mux := newMux(t, app)
	_, key := pbtest.MintToken(t, app, d, "machine", []string{"scraper.ingest"})
	hdr := map[string]string{"Authorization": "Bearer " + key}
	fire := captureRestamp(t)

	m := fixture(t)
	ended, err := time.Parse(time.RFC3339, m["ended_at"].(string))
	if err != nil {
		t.Fatalf("ended_at: %v", err)
	}
	early := insertEvent(t, app, "smoke1", ended.Add(-time.Minute))

	code, out := post(t, mux, marshal(t, m), hdr)
	if code != http.StatusOK {
		t.Fatalf("status = %d body = %v", code, out)
	}
	gameID, _ := out["game_id"].(string)
	if g := eventGame(t, app, early.Id); g != gameID {
		t.Fatalf("persist stamped early row with %q, want %q", g, gameID)
	}
	if *fire == nil {
		t.Fatalf("fresh persist did not schedule a restamp")
	}

	// The sink delivers a late in-window row and the next game's row after
	// the POST; the scheduled restamp stamps only the in-window one.
	late := insertEvent(t, app, "smoke1", ended.Add(-time.Second))
	next := insertEvent(t, app, "smoke1", ended.Add(time.Minute))
	other := insertEvent(t, app, "smoke2", ended.Add(-time.Second))
	(*fire)()
	if g := eventGame(t, app, late.Id); g != gameID {
		t.Errorf("late in-window row game = %q, want %q", g, gameID)
	}
	for name, id := range map[string]string{"next-game": next.Id, "other-instance": other.Id} {
		if g := eventGame(t, app, id); g != "" {
			t.Errorf("%s row stamped: %q", name, g)
		}
	}

	// A deduped 200 schedules nothing.
	*fire = nil
	if code, out := post(t, mux, marshal(t, m), hdr); code != http.StatusOK || out["deduped"] != true {
		t.Fatalf("dedupe: status = %d body = %v", code, out)
	}
	if *fire != nil {
		t.Errorf("deduped persist scheduled a restamp")
	}
}
