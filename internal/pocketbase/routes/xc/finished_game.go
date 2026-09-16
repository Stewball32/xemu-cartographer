package xc

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/games"
	"github.com/Stewball32/xemu-cartographer/internal/leaguescraper"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// MaxBody caps the finished_game body. A real artifact is a few KB (16
// players, a score summary); PocketBase's router-wide limit is far larger.
const MaxBody = 1 << 20

// RestampDelay is how long after a fresh persist the route re-runs the
// event stamping (§7.2): the daemon's event sink and the webhook are
// independent deliveries, so an in-window event row may land just after
// PersistFinishedGame stamped.
const RestampDelay = 3 * time.Second

// restampAfter schedules the late re-stamp; tests shorten it.
var restampAfter = func(d time.Duration, fn func()) { time.AfterFunc(d, fn) }

// Response is the 200 body of POST /api/xc/finished_game. Deduped is true
// when a games row with this game_uid already existed (still 200 — any 2xx
// acks the spool file, §7.1).
type Response struct {
	GameID   string `json:"game_id"`
	SeriesID string `json:"series_id"`
	Deduped  bool   `json:"deduped"`
}

func init() {
	register(func() {
		Group.POST("/finished_game", handleFinishedGame)
	})
}

// fail writes a JSON error with a stable code the daemon's parked-4xx log
// line shows: {"error": <message>, "code": <code>}.
func fail(e *core.RequestEvent, status int, code, msg string) error {
	return e.JSON(status, map[string]string{"error": msg, "code": code})
}

// handleFinishedGame is POST /api/xc/finished_game: the daemon's
// finished_game webhook (§7). Body = bare wire.FinishedGame
// (xc.finished_game/1). Status contract (§7.1): 2xx ⇒ the daemon deletes
// the spool file, 4xx ⇒ it parks the file in spool/failed, 5xx ⇒ it
// retries — so validation failures are 4xx and only a persist error is 500.
func handleFinishedGame(e *core.RequestEvent) error {
	e.Request.Body = http.MaxBytesReader(e.Response, e.Request.Body, MaxBody)
	var fg wire.FinishedGame
	if err := json.NewDecoder(e.Request.Body).Decode(&fg); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return fail(e, http.StatusRequestEntityTooLarge, "too_large", "body exceeds 1 MiB")
		}
		return fail(e, http.StatusBadRequest, "bad_json", "invalid JSON body: "+err.Error())
	}
	if fg.Schema != wire.SchemaFinishedGame {
		return fail(e, http.StatusBadRequest, "bad_schema", "schema must be "+wire.SchemaFinishedGame)
	}
	uid := strings.TrimSpace(fg.GameUID)
	if uid == "" {
		return fail(e, http.StatusBadRequest, "bad_uid", "game_uid is required")
	}
	if key := strings.TrimSpace(e.Request.Header.Get("Idempotency-Key")); key != "" && key != uid {
		return fail(e, http.StatusBadRequest, "idempotency_mismatch", "Idempotency-Key does not match game_uid")
	}

	lg := leaguescraper.FinishedGameFromWire(fg)
	res, err := games.PersistFinishedGame(e.App, lg)
	if err != nil {
		log.Printf("xc: persist finished game uid=%s instance=%s: %v", uid, fg.Instance, err)
		return fail(e, http.StatusInternalServerError, "persist_failed", "persist failed")
	}
	if !res.Deduped {
		// Same window the chain just used (StartedAt is zero today — the
		// artifact's known gap — so only the end bound applies).
		app, gameID, instance := e.App, res.GameID, lg.Container
		start, end := lg.StartedAt, lg.EndedAt
		restampAfter(RestampDelay, func() {
			if n, err := games.RestampEvents(app, gameID, instance, start, end); err != nil {
				log.Printf("xc: restamp events game=%s instance=%s: %v", gameID, instance, err)
			} else if n > 0 {
				log.Printf("xc: restamp events game=%s instance=%s: %d late rows stamped", gameID, instance, n)
			}
		})
	}
	return e.JSON(http.StatusOK, Response{GameID: res.GameID, SeriesID: res.SeriesID, Deduped: res.Deduped})
}
