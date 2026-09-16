package play

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/leaguescraper"
	scraperroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/xcclient"
)

// selectionDaemon stubs PUT /api/ctl/instances/{n}/host/selection with the
// documented answers and records the body that travelled (§6.2: names only).
type selectionDaemon struct {
	srv  *httptest.Server
	last map[string]any
}

func newSelectionDaemon(t *testing.T) *selectionDaemon {
	t.Helper()
	d := &selectionDaemon{}
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/ctl/instances/{name}/host/selection", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		d.last = nil
		_ = json.Unmarshal(raw, &d.last)
		w.Header().Set("Content-Type", "application/json")
		fail := func(status int, code, msg string) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": msg}})
		}
		switch {
		case r.PathValue("name") != "smoke1":
			fail(http.StatusNotFound, "not_found", "no runner attached")
		case d.last["map"] == "Nowhere":
			fail(http.StatusConflict, "not_in_carousel", "map not available on this instance: Nowhere")
		case d.last["map"] == "Unread":
			fail(http.StatusNotFound, "no_maps", "carousel not enumerated yet")
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"instance": r.PathValue("name"), "present": true, "authority": "runner",
				"selected_map": d.last["map"], "selected_gametype": d.last["gametype"],
			})
		}
	})
	d.srv = httptest.NewServer(mux)
	t.Cleanup(d.srv.Close)
	return d
}

func newEvent() (*core.RequestEvent, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/play/selection", nil)
	return &core.RequestEvent{Event: router.Event{Request: req, Response: rec}}, rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode %q: %v", rec.Body.String(), err)
		}
	}
	return out
}

// TestWireSelectionNamesOnly pins the wire branch of POST /api/play/selection:
// the body carries exactly {map, gametype} (no steps — the daemon owns the
// carousel) and the daemon's codes map onto today's statuses.
func TestWireSelectionNamesOnly(t *testing.T) {
	d := newSelectionDaemon(t)
	c, err := xcclient.New(xcclient.Config{URL: d.srv.URL, Token: "feed"})
	if err != nil {
		t.Fatal(err)
	}
	ctl := xcclient.NewCtl(c, "ctl")
	ctl.Connected = func() bool { return true }

	e, rec := newEvent()
	if err := wireSelection(e, ctl, "smoke1", "Blood Gulch", "Slayer"); err != nil {
		t.Fatal(err)
	}
	body := decode(t, rec)
	if rec.Code != http.StatusOK || body["selected_map"] != "Blood Gulch" || body["selected_gametype"] != "Slayer" {
		t.Fatalf("ok: %d %v", rec.Code, body)
	}
	if len(d.last) != 2 || d.last["map"] != "Blood Gulch" || d.last["gametype"] != "Slayer" {
		t.Fatalf("daemon body = %v, want names only", d.last)
	}

	for _, tc := range []struct {
		name, mapName string
		want          int
		msg           string
	}{
		{"smoke1", "Nowhere", http.StatusBadRequest, "not available"},
		{"smoke1", "Unread", http.StatusConflict, "not enumerated"},
		{"ghost", "Blood Gulch", http.StatusNotFound, "no host runner attached for ghost"},
	} {
		e, rec := newEvent()
		_ = wireSelection(e, ctl, tc.name, tc.mapName, "Slayer")
		body := decode(t, rec)
		msg, _ := body["error"].(string)
		if rec.Code != tc.want || !strings.Contains(msg, tc.msg) {
			t.Errorf("%s/%s: %d %q, want %d containing %q", tc.name, tc.mapName, rec.Code, msg, tc.want, tc.msg)
		}
	}

	// Daemon away: 503 before any call.
	ctl.Connected = func() bool { return false }
	d.last = nil
	e, rec = newEvent()
	_ = wireSelection(e, ctl, "smoke1", "Blood Gulch", "Slayer")
	if rec.Code != http.StatusServiceUnavailable || decode(t, rec)["error"] != scraperroutes.UnavailableMessage || d.last != nil {
		t.Fatalf("down: %d %s (daemon body %v)", rec.Code, rec.Body.String(), d.last)
	}

	// requireHost gates every play route the same way.
	adapter := leaguescraper.NewAdapter(c, ctl)
	SetHostControl(adapter)
	t.Cleanup(func() { HostRunners = nil })
	e, rec = newEvent()
	if requireHost(e) || rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("requireHost while down: ok=%v code=%d", requireHost(e), rec.Code)
	}
	ctl.Connected = func() bool { return true }
	e, _ = newEvent()
	if !requireHost(e) {
		t.Fatal("requireHost while up must pass")
	}
}
