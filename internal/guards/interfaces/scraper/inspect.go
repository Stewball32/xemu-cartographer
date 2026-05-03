package scraper

import (
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// Info is one row in the running-scraper list, returned by Lifecycle consumers
// (the /api/admin/scraper GET handler, dashboards, debug routes).
type Info struct {
	Name      string    `json:"name"`
	Sock      string    `json:"sock"`
	TitleID   uint32    `json:"title_id"`
	GameTitle string    `json:"game_title"`
	XboxName  string    `json:"xbox_name"`
	Tick      uint32    `json:"tick"`  // most recent observed game tick
	Ticks     uint64    `json:"ticks"` // total iterations executed
	StartedAt time.Time `json:"started_at"`
}

// InspectState is the per-runner deep-dive view served by the debug page's
// /api/admin/scraper/{name}/inspect endpoint. Embeds Info for the basic
// identity fields and adds whatever the runner has cached so the debug page
// can render even before the next game-state transition broadcasts a snapshot.
//
// LatestSnapshot/LatestTick are nil until the runner has observed at least
// one PreGame/InGame/PostGame transition or in-game tick respectively.
// RecentEvents is newest-first, capped at the runner's ring-buffer size.
type InspectState struct {
	Info
	Running        bool                     `json:"running"`
	CurrentState   scraper.GameState        `json:"current_state"`
	StateInputs    scraper.StateInputs      `json:"state_inputs"`
	ScoreProbe     scraper.ScoreProbe       `json:"score_probe"`
	LatestSnapshot *scraper.SnapshotPayload `json:"latest_snapshot"`
	LatestTick     *scraper.TickPayload     `json:"latest_tick"`
	RecentEvents   []scraper.Envelope       `json:"recent_events"`
}

// Inspect lets callers enumerate currently-running scrapers and fetch the
// deep-dive cached state for one named runner.
type Inspect interface {
	List() []Info
	Inspect(name string) (InspectState, bool)
}
