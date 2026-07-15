package lansync

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// GET /api/lan/sync/manifest?preset=<id|active>
//
// Resolves a sync_preset into the flat payload the on-Xbox JSON reader consumes
// (SPEC §4.3). THIS is the contract the norcal-og-xbox client depends on — the
// shape below matches SPEC v1.0 exactly and reuses the /api/lan/saves/manifest
// field vocabulary (title_id / fatx_dir / footprint_bytes / format / path).
//
// SCAFFOLD: the preset lookup, event header, and policy passthrough are wired;
// the actual resolution of checkins→profiles, preset.games→isos, preset.apps→
// apps into the item arrays is stubbed (empty arrays + TODO). Items ship in
// preset.priority order once resolved.
const specVersion = "1.0"

func init() {
	register(func() {
		Group.GET("/manifest", handleManifest)
	})
}

// syncItem is one manifest entry. A single flat struct covers all three
// categories (SPEC §4.3 field reference); category-specific fields use omitempty
// so a profile omits dest_dir/priority and a game/app omits player/fatx_dir.
type syncItem struct {
	ID             string `json:"id"`
	Category       string `json:"category"` // "profile" | "game" | "app"
	Player         string `json:"player,omitempty"`
	Title          string `json:"title,omitempty"`
	TitleID        string `json:"title_id,omitempty"`
	Name           string `json:"name"`
	FatxDir        string `json:"fatx_dir,omitempty"`  // profiles
	DestDir        string `json:"dest_dir,omitempty"`  // games, apps (relative to data drive)
	FootprintBytes uint64 `json:"footprint_bytes"`
	Format         string `json:"format"` // "tar" | "zip"
	Path           string `json:"path"`   // ready-to-GET relative URL
	Priority       int    `json:"priority,omitempty"`
	Conflict       string `json:"conflict"` // per-item override of policy default
}

type catPolicy struct {
	Conflict string `json:"conflict"` // "skip" | "overwrite"
	Prune    bool   `json:"prune"`
}

type manifestPolicy struct {
	Profiles catPolicy `json:"profiles"`
	Games    catPolicy `json:"games"`
	Apps     catPolicy `json:"apps"`
}

type presetHeader struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type eventHeader struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// syncManifest is the top-level response (SPEC §4.3).
type syncManifest struct {
	SpecVersion string         `json:"spec_version"`
	Hub         string         `json:"hub"`
	GeneratedAt string         `json:"generated_at"`
	Preset      presetHeader   `json:"preset"`
	Event       eventHeader    `json:"event"`
	Policy      manifestPolicy `json:"policy"`
	Profiles    []syncItem     `json:"profiles"`
	Games       []syncItem     `json:"games"`
	Apps        []syncItem     `json:"apps"`
}

func handleManifest(e *core.RequestEvent) error {
	sel := strings.TrimSpace(e.Request.URL.Query().Get("preset"))

	preset, err := resolvePreset(e.App, sel)
	if err != nil || preset == nil {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error": "no matching sync preset (pass ?preset=<id> or ?preset=active)",
		})
	}

	man := syncManifest{
		SpecVersion: specVersion,
		Hub:         "norcal-halo-lan",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Preset: presetHeader{
			ID:     preset.Id,
			Name:   preset.GetString("name"),
			Active: preset.GetBool("active"),
		},
		Event:  resolveEventHeader(e.App, preset.GetString("event")),
		Policy: resolvePolicy(preset),
		// TODO(lan-sync): resolve the concrete sets, ordered by preset.priority
		// (id→int, higher first). Tolerate dangling relation ids (nullify-on-delete).
		//   Profiles — for each checked-in gamertag of preset.event (checkins
		//     where event == preset.event), resolve gamertag → owning user →
		//     ce_profiles / h2_profiles (mirror routes/lansaves identity), emit
		//     category="profile", format="tar", path=/api/lan/saves/download?...,
		//     fatx_dir/title_id/footprint_bytes from the generated bundle.
		//   Games — expand preset.games (isos), category="game", format="tar",
		//     path=/api/lan/sync/dl/game/<id>, dest_dir=<halo_dir>\<name>,
		//     footprint from the extracted tree.
		//   Apps  — expand preset.apps, category="app", format="zip",
		//     path=/api/lan/sync/dl/app/<id>, dest_dir=<apps_dir>\<name>.
		// Per-item `conflict` defaults to the category policy conflict.
		Profiles: []syncItem{},
		Games:    []syncItem{},
		Apps:     []syncItem{},
	}

	return e.JSON(http.StatusOK, man)
}

// resolvePreset returns the preset for the selector: a bare id, or "active"/""
// for the active preset. TODO(lan-sync): if several rows have active=true (not
// yet enforced single-active), this returns the first.
func resolvePreset(app core.App, sel string) (*core.Record, error) {
	if sel != "" && sel != "active" {
		return app.FindRecordById("sync_presets", sel)
	}
	return app.FindFirstRecordByFilter("sync_presets", "active = true", dbx.Params{})
}

// resolveEventHeader projects the preset's lan_event into {id, label}. Empty on
// a missing/dangling event id (tolerated).
func resolveEventHeader(app core.App, eventID string) eventHeader {
	if eventID == "" {
		return eventHeader{}
	}
	rec, err := app.FindRecordById("lan_events", eventID)
	if err != nil || rec == nil {
		return eventHeader{ID: eventID}
	}
	return eventHeader{ID: rec.Id, Label: rec.GetString("label")}
}

// resolvePolicy reads the preset's `policy` JSON verbatim into the manifest
// shape, defaulting each category to the safest non-destructive policy
// (conflict=skip, prune=false) when a field is absent.
func resolvePolicy(preset *core.Record) manifestPolicy {
	def := manifestPolicy{
		Profiles: catPolicy{Conflict: "skip"},
		Games:    catPolicy{Conflict: "skip"},
		Apps:     catPolicy{Conflict: "skip"},
	}
	raw := strings.TrimSpace(preset.GetString("policy"))
	if raw == "" || raw == "null" {
		return def
	}
	var parsed manifestPolicy
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return def // malformed policy → safe defaults (TODO: surface a warning)
	}
	// Fill any category the organizer left unset with the safe default.
	if parsed.Profiles.Conflict == "" {
		parsed.Profiles.Conflict = "skip"
	}
	if parsed.Games.Conflict == "" {
		parsed.Games.Conflict = "skip"
	}
	if parsed.Apps.Conflict == "" {
		parsed.Apps.Conflict = "skip"
	}
	return parsed
}
