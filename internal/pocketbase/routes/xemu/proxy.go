package xemu

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"

	scraperroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/xcclient"
)

// Wire mode (DESIGN-STEP8 §6.2 / §9): the four probe tools moved into the
// daemon as POST /api/ctl/xemu/{probe,probe_title,sample_deltas,scan_string}
// (JSON bodies, attached instances only). Behind the unchanged admin gate
// each GET here re-encodes its query string as that body and relays the
// daemon's status + body verbatim. In-process (XC_SCRAPER_URL unset) the
// handlers keep opening the QMP socket locally.

// wireCtl returns the daemon control client when the scraper feed runs in
// wire mode, nil in-process. It reads the scraper route group's injected
// source so no extra main.go wiring is needed.
func wireCtl() *xcclient.Ctl {
	return scraperroutes.Wire(scraperroutes.Manager)
}

// addrOf turns the league's ?sock=<path> into the daemon.Addr grammar
// ("unix:<abs path>"); an already-qualified unix:/tcp: value passes through.
func addrOf(sock string) string {
	if strings.HasPrefix(sock, "unix:") || strings.HasPrefix(sock, "tcp:") {
		return sock
	}
	return "unix:" + sock
}

// intKnob copies an integer query knob into body when it parses; the daemon
// applies the same clamp / fallback rules the local handlers use (0 = unset).
func intKnob(body map[string]any, q url.Values, key string) {
	if v, err := strconv.Atoi(q.Get(key)); err == nil {
		body[key] = v
	}
}

// strKnob copies a non-empty string query knob (hex ranges travel as strings).
func strKnob(body map[string]any, q url.Values, key string) {
	if v := q.Get(key); v != "" {
		body[key] = v
	}
}

// probeBudgetBase is the relay timeout floor for one probe call. The daemon
// runs each tool synchronously, so the 2 s control-call timeout
// (xcclient.DefaultCtlTimeout, §8.5) fits probe / scan_string but not
// probe_title (samples × interval_ms, up to 600 × 5 s) or sample_deltas (two
// reads interval_ms apart, up to 10 s): probeBudget adds the sampling window
// the knobs ask for on top of this floor, clamped to the daemon's own knob
// bounds so a wild query cannot pick an unbounded timeout.
const probeBudgetBase = 60 * time.Second

func probeBudget(body map[string]any) time.Duration {
	samples, _ := body["samples"].(int)
	interval, _ := body["interval_ms"].(int)
	samples = min(max(samples, 1), 600)
	interval = min(max(interval, 0), 10000)
	return probeBudgetBase + time.Duration(samples)*time.Duration(interval)*time.Millisecond
}

// proxy forwards body to POST /api/ctl/xemu/<tool> with the control token and
// writes the daemon's status + JSON body unchanged (the daemon keeps the
// league field names plus addr). 503 while the daemon is away, 502 when the
// request itself fails. The call runs on its own probe budget (probeBudget),
// not the shared control timeout, so long samplings are not cut at 2 s.
func proxy(e *core.RequestEvent, ctl *xcclient.Ctl, tool string, body map[string]any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	probe := *ctl // per-call copy: the shared Ctl keeps its 2 s control timeout
	probe.Timeout = probeBudget(body)
	status, out, err := probe.Do(e.Request.Context(), http.MethodPost, "/api/ctl/xemu/"+tool, raw, true)
	if err != nil {
		if scraperroutes.IsUpstreamDown(err) {
			return scraperroutes.Unavailable(e)
		}
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	if len(out) == 0 {
		return e.NoContent(status)
	}
	return e.Blob(status, "application/json", out)
}
