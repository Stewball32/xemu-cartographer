package scraper

import (
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// Info is one row in the running-scraper list, returned by Lifecycle consumers
// (the /api/admin/scraper GET handler, dashboards, debug routes).
type Info struct {
	Name     string `json:"name"`
	Sock     string `json:"sock"`
	TitleID  uint32 `json:"title_id"`
	Title    string `json:"title"`
	XboxName string `json:"xbox_name"`

	// EEPROM-derived system info — populated by the runner's system-snapshot
	// pass (xbox.Read* readers in internal/scraper/xbox/). Title-agnostic;
	// stable for the lifetime of the runner once first populated. Fields are
	// empty strings / 0 until the first successful read.
	SerialNumber    string `json:"serial_number,omitempty"`
	MACAddress      string `json:"mac_address,omitempty"`
	VideoStandard   string `json:"video_standard,omitempty"`
	TimeZoneBias    int32  `json:"time_zone_bias,omitempty"`
	TimeZoneStdName string `json:"time_zone_std_name,omitempty"`
	TimeZoneDltName string `json:"time_zone_dlt_name,omitempty"`

	// XBE-certificate-derived fields. Populated by the runner's
	// system-snapshot pass (xbox.ReadXBECertificate). XBETitleName is the
	// canonical game name as the developer wrote it ("Halo: Combat Evolved",
	// "Microsoft Xbox Dashboard"); useful as a fallback when the title-ID
	// registry lookup misses. XBEGameRegion is the bitfield from the
	// certificate; format with xbox.FormatGameRegion if a label is needed.
	XBETitleName    string `json:"xbe_title_name,omitempty"`
	XBEVersion      uint32 `json:"xbe_version,omitempty"`
	XBEGameRegion   uint32 `json:"xbe_game_region,omitempty"`
	XBEDiskNumber   uint32 `json:"xbe_disk_number,omitempty"`
	XBEAllowedMedia uint32 `json:"xbe_allowed_media,omitempty"`

	// Kernel-clock fields. Populated by the runner's system-snapshot pass
	// from the kernel's KeSystemTime / KeInterruptTime globals.
	// kernel_system_time is wall-clock UTC; kernel_boot_time is the wall-clock
	// instant the guest booted; kernel_uptime_ns is nanoseconds since boot.
	// Useful for spot checks ("is this runner alive?", "when did this xemu
	// start?") and cross-container event correlation.
	KernelSystemTime time.Time     `json:"kernel_system_time,omitempty"`
	KernelBootTime   time.Time     `json:"kernel_boot_time,omitempty"`
	KernelUptime     time.Duration `json:"kernel_uptime_ns,omitempty"`

	Tick      uint32    `json:"tick"`  // most recent observed game tick
	Ticks     uint64    `json:"ticks"` // total iterations executed
	StartedAt time.Time `json:"started_at"`
}

// PreviousGameInfo is the just-ended match captured on Live → Ready
// transitions. Surfaced by Inspect so the debug page can render the
// previous match's roster / scores while the runner is back in Ready.
// Dropped on Ready → Idle.
type PreviousGameInfo struct {
	GameData *scraper.GameData  `json:"game_data,omitempty"`
	Events   []scraper.Envelope `json:"events,omitempty"`
	EndedAt  time.Time          `json:"ended_at"`
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
	GameData     *scraper.GameData    `json:"game_data"`
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
