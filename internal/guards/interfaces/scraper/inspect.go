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
	Title string    `json:"title"`
	XboxName  string    `json:"xbox_name"`
	Tick      uint32    `json:"tick"`  // most recent observed game tick
	Ticks     uint64    `json:"ticks"` // total iterations executed
	StartedAt time.Time `json:"started_at"`
}

// PreviousGameInfo is the just-ended match captured on Live → Ready
// transitions. Surfaced by Inspect so the debug page can render the
// previous match's roster / scores while the runner is back in Ready.
// Dropped on Ready → Idle.
type PreviousGameInfo struct {
	GameData *scraper.GameData `json:"game_data,omitempty"`
	Events    []scraper.Envelope `json:"events,omitempty"`
	EndedAt   time.Time          `json:"ended_at"`
}

// InspectState is the per-runner deep-dive view served by the debug page's
// /api/admin/scraper/{name}/inspect endpoint. Embeds Info for the basic
// identity fields and adds whatever the runner has cached so the debug page
// can render even before the next game-state transition broadcasts a fresh
// game-data envelope.
//
// GameData / LatestTick are nil until the runner has observed at least
// one PreGame/InGame/PostGame transition or in-game tick respectively.
// RecentEvents is newest-first, capped at the runner's ring-buffer size.
//
// Phase is the runner's lifecycle state introduced in M5 stage 5a:
// "idle" (no recognised title yet), "ready" (title detected, no live
// match), "live" (active match). Renders independently of CurrentState
// so the debug page can show "phase=idle" even when the bound reader
// hasn't observed any game state at all.
type InspectState struct {
	Info
	Running      bool                 `json:"running"`
	Phase        string               `json:"phase"`
	LastReadAt   time.Time            `json:"last_read_at"`
	CurrentState scraper.GameState    `json:"current_state"`
	StateInputs  scraper.StateInputs  `json:"state_inputs"`
	ScoreProbe   scraper.ScoreProbe   `json:"score_probe"`
	GameData    *scraper.GameData   `json:"game_data"`
	LatestTick   *scraper.TickPayload `json:"latest_tick"`
	RecentEvents []scraper.Envelope   `json:"recent_events"`
	PreviousGame *PreviousGameInfo    `json:"previous_game,omitempty"`
}

// Inspect lets callers enumerate currently-running scrapers and fetch the
// deep-dive cached state for one named runner.
type Inspect interface {
	List() []Info
	Inspect(name string) (InspectState, bool)
}
