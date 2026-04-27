package scraper

import "time"

// Info is one row in the running-scraper list, returned by Lifecycle consumers
// (the /api/admin/scraper GET handler, dashboards, debug routes).
type Info struct {
	Name      string    `json:"name"`
	Sock      string    `json:"sock"`
	TitleID   uint32    `json:"title_id"`
	Tick      uint32    `json:"tick"`        // most recent observed game tick
	Ticks     uint64    `json:"ticks"`       // total iterations executed
	StartedAt time.Time `json:"started_at"`
}

// Inspect lets callers enumerate currently-running scrapers.
type Inspect interface {
	List() []Info
}
