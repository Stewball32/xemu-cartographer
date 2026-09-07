package manager

import (
	"time"

	"github.com/xemu-cartographer/xc-scraper/scraper"
)

// The view types the Manager returns to its consumers (routes, the WS join
// guard, the authz adapter, the play API). Moved here from
// internal/guards/interfaces/scraper in step 7 part 3c so the manager is
// league-free; that package now aliases every one of them (type identity is
// preserved — scraperiface.Info IS manager.Info).

// Info is one row in the running-scraper list, returned by Lifecycle consumers
// (the /api/admin/scraper GET handler, dashboards, debug routes).
type Info struct {
	Name     string `json:"name"`
	Sock     string `json:"sock"`
	TitleID  uint32 `json:"title_id"`
	Title    string `json:"title"`
	XboxName string `json:"xbox_name"`

	// EEPROM-derived system info — populated by the runner's system-snapshot
	// pass (xbox.Read* readers in xc-scraper/xbox). Title-agnostic; stable
	// for the lifetime of the runner once first populated. Fields are empty
	// strings / 0 until the first successful read.
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
// match), "live" (active match).
type InspectState struct {
	Info
	Running      bool                 `json:"running"`
	Phase        string               `json:"phase"`
	LastReadAt   time.Time            `json:"last_read_at"`
	StateInputs  scraper.StateInputs  `json:"state_inputs"`
	ScoreProbe   scraper.ScoreProbe   `json:"score_probe"`
	GameData     *scraper.GameData    `json:"game_data"`
	LatestTick   *scraper.TickPayload `json:"latest_tick"`
	RecentEvents []scraper.Envelope   `json:"recent_events"`
	PreviousGame *PreviousGameInfo    `json:"previous_game,omitempty"`
	// PlayerAccum is the per-player accumulated match stats + activity latch
	// snapshot (xc-scraper/scraper accum.go). Read-only — replaced wholesale
	// by the runner, never mutated in place. Nil before the first live tick.
	PlayerAccum map[int]scraper.PlayerAccum `json:"player_accum,omitempty"`
}

// InstanceState is the per-instance scraper view surfaced to the container
// detail page (game title + Xbox console name + running flag). Empty/zero
// values are normal — they mean the scraper is not yet attached or the
// game-specific reader hasn't resolved that field yet.
type InstanceState struct {
	Name     string `json:"name"`
	TitleID  uint32 `json:"title_id"`
	Title    string `json:"title"`
	XboxName string `json:"xbox_name"`
	Running  bool   `json:"running"`
}

// MapOption is one selectable map (or gametype) enumerated LIVE from a specific
// instance — the actual game/disc on that box, so modded discs' custom maps show
// up. Steps is the option's ABSOLUTE 0-based index (position) in the live carousel,
// NOT a D-pad press count: the carousel start is non-deterministic, so navigating to
// a pick is cursor-relative (presses = (Steps − liveCursorIndex) mod count) and must
// be computed at nav time. See xc-scraper/haloce EnumerateLobby.
type MapOption struct {
	Name  string `json:"name"`
	Steps int    `json:"steps"`
}

// MapList is the per-instance available maps + gametypes the player API serves.
// Available is false when the instance can't be enumerated yet (no live carousel
// read / disc parse for it) — in which case Maps/Gametypes are empty and the
// caller must NOT substitute a fixed/stock table (modded discs make any hardcoded
// set wrong). Selection then falls back to free-form names.
type MapList struct {
	Available bool        `json:"available"`
	Maps      []MapOption `json:"maps"`
	Gametypes []MapOption `json:"gametypes"`
	// GametypesPending is true while the box is enumerable (built-ins readable) but
	// the async host-side custom-variant read hasn't finished — i.e. the gametype
	// list is still incomplete. The picker shows a "reading gametypes…" waiting
	// state instead of the built-ins-only intermediate until this clears.
	GametypesPending bool `json:"gametypes_pending"`
}

// IndexOf returns the Steps for a named map/gametype option (case-insensitive on
// the caller's side is not assumed — names are compared as-is), and whether it
// was found. Used by the play API to translate a chosen name into the runner's
// D-pad navigation count.
func (l MapList) IndexOf(options []MapOption, name string) (int, bool) {
	for _, o := range options {
		if o.Name == name {
			return o.Steps, true
		}
	}
	return 0, false
}
