package lansync

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/diskspace"
	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// GET /api/lan/sync/manifest?preset=<id|active>
//
// Resolves a sync_preset into the flat payload the on-Xbox JSON reader consumes
// (SPEC §4.3). THIS is the contract the norcal-og-xbox client depends on. The
// shape matches SPEC v1.0 and reuses the /api/lan/saves/manifest field
// vocabulary (title_id / fatx_dir / footprint_bytes / format / path).
//
// Resolution (real):
//   - profiles: checkins for preset.event → gamertag → owning user → the user's
//     ce_profiles / h2_profiles that have a generated bundle. path = the
//     existing /api/lan/saves/file/{ce,h2}-profile/{id} serve route.
//   - games: preset.games (isos) → /api/lan/sync/dl/game/{id} (tar of extracted tree).
//   - apps:  preset.apps (apps)  → /api/lan/sync/dl/app/{id} (stored zip).
//
// Games + apps are ordered by preset.priority (id→int, higher first). Dangling
// relation ids (nullify-on-delete) are tolerated (skipped).
const specVersion = "1.0"

func init() {
	register(func() {
		Group.GET("/manifest", handleManifest)
	})
}

// syncItem is one manifest entry (SPEC §4.3). One flat struct covers all three
// categories; category-specific fields use omitempty.
type syncItem struct {
	ID             string `json:"id"`
	Category       string `json:"category"` // "profile" | "game" | "app"
	Player         string `json:"player,omitempty"`
	Title          string `json:"title,omitempty"`
	TitleID        string `json:"title_id,omitempty"`
	Name           string `json:"name"`
	FatxDir        string `json:"fatx_dir,omitempty"` // profiles
	DestDir        string `json:"dest_dir,omitempty"` // games, apps
	FootprintBytes uint64 `json:"footprint_bytes"`
	Format         string `json:"format"` // "tar" | "zip"
	Path           string `json:"path"`
	Priority       int    `json:"priority,omitempty"`
	Conflict       string `json:"conflict"`
}

type catPolicy struct {
	Conflict string `json:"conflict"`
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

// profileInfo is the subset of a profile record's save_info JSON the manifest
// needs (mirrors saveartifact.Info).
type profileInfo struct {
	TitleID string `json:"title_id"`
	DirName string `json:"dir_name"`
	FatxDir string `json:"fatx_dir"`
	Files   []struct {
		Size int `json:"size"`
	} `json:"files"`
}

func handleManifest(e *core.RequestEvent) error {
	sel := strings.TrimSpace(e.Request.URL.Query().Get("preset"))

	preset, err := resolvePreset(e.App, sel)
	if err != nil || preset == nil {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error": "no matching sync preset (pass ?preset=<id> or ?preset=active)",
		})
	}

	policy := resolvePolicy(preset)
	priority := resolvePriority(preset)
	eventID := preset.GetString("event")

	man := syncManifest{
		SpecVersion: specVersion,
		Hub:         "norcal-halo-lan",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Preset: presetHeader{
			ID:     preset.Id,
			Name:   preset.GetString("name"),
			Active: preset.GetBool("active"),
		},
		Event:    resolveEventHeader(e.App, eventID),
		Policy:   policy,
		Profiles: resolveProfiles(e.App, eventID, policy.Profiles.Conflict),
		Games:    resolveGames(e.App, preset.GetStringSlice("games"), priority, policy.Games.Conflict),
		Apps:     resolveApps(e.App, preset.GetStringSlice("apps"), priority, policy.Apps.Conflict),
	}

	return e.JSON(http.StatusOK, man)
}

// resolveProfiles walks the checked-in gamertags of eventID and emits each
// checked-in player's generated CE + H2 profiles.
func resolveProfiles(app core.App, eventID, conflict string) []syncItem {
	out := []syncItem{}
	if eventID == "" {
		return out
	}
	checkins, err := app.FindRecordsByFilter("checkins",
		"event = {:e}", "", 0, 0, dbx.Params{"e": eventID})
	if err != nil {
		return out
	}
	for _, ci := range checkins {
		gt, err := app.FindRecordById("gamertags", ci.GetString("gamertag"))
		if err != nil || gt == nil {
			continue // dangling gamertag ref — skip
		}
		player := gt.GetString("tag")
		userID := gt.GetString("user")
		if item, ok := profileItem(app, "ce_profiles", "ce", "ce-profile", userID, player, conflict); ok {
			out = append(out, item)
		}
		if item, ok := profileItem(app, "h2_profiles", "h2", "h2-profile", userID, player, conflict); ok {
			out = append(out, item)
		}
	}
	// Stable order for a deterministic payload (player, then title).
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Player != out[j].Player {
			return out[i].Player < out[j].Player
		}
		return out[i].Title < out[j].Title
	})
	return out
}

// profileItem builds the manifest item for a user's profile in the given
// collection, or ok=false when the user has none / it has no generated bundle.
func profileItem(app core.App, collection, title, kind, userID, player, conflict string) (syncItem, bool) {
	if userID == "" {
		return syncItem{}, false
	}
	rec := firstRecordByUser(app, collection, userID)
	if rec == nil || rec.GetString("save_bundle") == "" {
		return syncItem{}, false
	}
	info := parseProfileInfo(rec.GetString("save_info"))
	// name is the player-facing display name (the gamertag / save name, SPEC
	// example "New001"); the hex FATX dir stays authoritative in fatx_dir.
	name := player
	sizes := make([]int, len(info.Files))
	for i, f := range info.Files {
		sizes[i] = f.Size
	}
	return syncItem{
		ID:             rec.Id,
		Category:       "profile",
		Player:         player,
		Title:          title,
		TitleID:        info.TitleID,
		Name:           name,
		FatxDir:        info.FatxDir,
		FootprintBytes: diskspace.FATXFootprint(sizes, cfg.FATXCluster),
		Format:         "tar",
		Path:           "/api/lan/saves/file/" + kind + "/" + rec.Id,
		Conflict:       conflict,
	}, true
}

// resolveGames expands the preset's iso ids into game items (tar of extracted
// tree), ordered by priority (higher first).
func resolveGames(app core.App, isoIDs []string, priority map[string]int, conflict string) []syncItem {
	out := []syncItem{}
	for _, id := range isoIDs {
		rec, err := app.FindRecordById("isos", id)
		if err != nil || rec == nil {
			continue // dangling ref — skip
		}
		if rec.GetBool("drift_detected") {
			continue // failed integrity check — never serve bad bytes to a console
		}
		out = append(out, syncItem{
			ID:             rec.Id,
			Category:       "game",
			Title:          titleFromID(rec.GetString("title_id")),
			TitleID:        rec.GetString("title_id"),
			Name:           rec.GetString("name"),
			DestDir:        joinDest(cfg.HaloDir, destName(rec)),
			FootprintBytes: uint64(rec.GetInt("footprint_bytes")),
			Format:         "tar",
			Path:           "/api/lan/sync/dl/game/" + rec.Id,
			Priority:       priority[rec.Id],
			Conflict:       conflict,
		})
	}
	sortByPriorityDesc(out)
	return out
}

// resolveApps expands the preset's app ids into app items (stored zip), ordered
// by priority (higher first).
func resolveApps(app core.App, appIDs []string, priority map[string]int, conflict string) []syncItem {
	out := []syncItem{}
	for _, id := range appIDs {
		rec, err := app.FindRecordById("apps", id)
		if err != nil || rec == nil {
			continue
		}
		out = append(out, syncItem{
			ID:             rec.Id,
			Category:       "app",
			Name:           rec.GetString("name"),
			DestDir:        joinDest(cfg.AppsDir, destName(rec)),
			FootprintBytes: uint64(rec.GetInt("footprint_bytes")),
			Format:         "zip",
			Path:           "/api/lan/sync/dl/app/" + rec.Id,
			Priority:       priority[rec.Id],
			Conflict:       conflict,
		})
	}
	sortByPriorityDesc(out)
	return out
}

// ---- small resolution helpers ----

// resolvePreset returns the preset for the selector (bare id, or "active"/"" →
// the active preset).
func resolvePreset(app core.App, sel string) (*core.Record, error) {
	if sel != "" && sel != "active" {
		return app.FindRecordById("sync_presets", sel)
	}
	return app.FindFirstRecordByFilter("sync_presets", "active = true", dbx.Params{})
}

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

// resolvePolicy reads the preset's `policy` JSON verbatim, defaulting each
// category to the safest non-destructive policy (conflict=skip, prune=false).
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
		return def
	}
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

// resolvePriority parses the preset's `priority` JSON (id→int map). Missing/bad
// → empty map (all priorities 0).
func resolvePriority(preset *core.Record) map[string]int {
	out := map[string]int{}
	raw := strings.TrimSpace(preset.GetString("priority"))
	if raw == "" || raw == "null" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// firstRecordByUser returns the one-per-user profile record for userID, or nil.
func firstRecordByUser(app core.App, collection, userID string) *core.Record {
	rec, err := app.FindFirstRecordByFilter(collection, "user = {:u}", dbx.Params{"u": userID})
	if err != nil {
		return nil
	}
	return rec
}

func parseProfileInfo(raw string) profileInfo {
	var info profileInfo
	if s := strings.TrimSpace(raw); s != "" && s != "null" {
		_ = json.Unmarshal([]byte(s), &info)
	}
	return info
}

// titleFromID maps a known Xbox title-id dir to the Halo title short name.
func titleFromID(titleID string) string {
	switch strings.ToLower(titleID) {
	case strings.ToLower(halosave.TitleIDHaloCE):
		return halosave.TitleCE
	case strings.ToLower(halosave.TitleIDHalo2):
		return halosave.TitleH2
	}
	return ""
}

// destName is the destination folder name: the explicit dest_name, else a
// FATX-safe slug of the record's name.
func destName(rec *core.Record) string {
	if d := strings.TrimSpace(rec.GetString("dest_name")); d != "" {
		return d
	}
	return slugFolder(rec.GetString("name"))
}

// joinDest joins a client dir root (e.g. `\Halo`) with a folder using a
// backslash separator (Xbox FATX paths).
func joinDest(root, folder string) string {
	root = strings.TrimRight(root, `\`)
	return root + `\` + folder
}

// slugFolder reduces a name to a FATX-safe folder component (alphanumerics
// kept, other runs collapsed out). Empty → "item".
func slugFolder(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "item"
	}
	return b.String()
}

func sortByPriorityDesc(items []syncItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority > items[j].Priority
		}
		return items[i].Name < items[j].Name
	})
}
