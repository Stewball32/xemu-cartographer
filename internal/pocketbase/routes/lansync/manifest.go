package lansync

import (
	"net/http"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// GET /api/lan/sync/manifest?preset=<id|active>
//
// Resolves a sync_preset into the concrete set the client installs. This is THE
// contract the norcal-og-xbox client consumes — the shape below is a STUB to be
// reconciled against the client session's exact expected JSON.
//
// SCAFFOLD: the preset lookup + conflict-policy read are wired; the actual
// resolution of players→profiles, games→isos, apps→apps into item arrays is
// stubbed (empty arrays + TODO). Items ship in priority order once resolved.
func init() {
	register(func() {
		Group.GET("/manifest", handleManifest)
	})
}

// syncProfileRef is one checked-in player's CE+H2 profiles to install.
type syncProfileRef struct {
	Gamertag string       `json:"gamertag"`
	Priority int          `json:"priority"`
	CE       *syncFileRef `json:"ce,omitempty"`
	H2       *syncFileRef `json:"h2,omitempty"`
}

// syncFileRef is a single downloadable artifact reference.
type syncFileRef struct {
	ID          string `json:"id"`
	HasFile     bool   `json:"has_file"`
	DownloadURL string `json:"download_url,omitempty"`
}

// syncGameRef is one game (ISO) to install, pulled as an extracted tree.
type syncGameRef struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	TitleID        string `json:"title_id,omitempty"`
	Priority       int    `json:"priority"`
	ExtractedReady bool   `json:"extracted_ready"`
	DownloadURL    string `json:"download_url"`
}

// syncAppRef is one app to install into the Xbox "Apps" folder.
type syncAppRef struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Priority       int    `json:"priority"`
	ExtractedReady bool   `json:"extracted_ready"`
	DownloadURL    string `json:"download_url"`
}

// conflictPolicy is the per-category skip/overwrite/prune the client applies.
type conflictPolicy struct {
	Profiles string `json:"profiles"`
	Games    string `json:"games"`
	Apps     string `json:"apps"`
}

// presetHeader identifies the resolved preset.
type presetHeader struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// syncManifest is the top-level response — the contract the client consumes.
type syncManifest struct {
	Hub            string           `json:"hub"`
	Version        int              `json:"version"`
	Preset         presetHeader     `json:"preset"`
	ConflictPolicy conflictPolicy   `json:"conflict_policy"`
	Profiles       []syncProfileRef `json:"profiles"`
	Games          []syncGameRef    `json:"games"`
	Apps           []syncAppRef     `json:"apps"`
	Counts         manifestCounts   `json:"counts"`
}

type manifestCounts struct {
	Profiles int `json:"profiles"`
	Games    int `json:"games"`
	Apps     int `json:"apps"`
}

func handleManifest(e *core.RequestEvent) error {
	sel := strings.TrimSpace(e.Request.URL.Query().Get("preset"))

	preset, err := resolvePreset(e.App, sel)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error": "no matching sync preset (pass ?preset=<id> or ?preset=active)",
		})
	}

	man := syncManifest{
		Hub:     "xemu-cartographer",
		Version: 1,
		Preset: presetHeader{
			ID:     preset.Id,
			Name:   preset.GetString("name"),
			Active: preset.GetBool("active"),
		},
		ConflictPolicy: conflictPolicy{
			Profiles: policyOrDefault(preset.GetString("profiles_conflict")),
			Games:    policyOrDefault(preset.GetString("games_conflict")),
			Apps:     policyOrDefault(preset.GetString("apps_conflict")),
		},
		// TODO(lan-sync): resolve the concrete sets, ordered by preset.priority:
		//   Profiles — for each checked-in player (preset.players, or the
		//     checked_in set of preset.event when players is empty), look up the
		//     user's gamertag → ce_profiles / h2_profiles (mirror
		//     routes/lansaves identity), emitting /api/lan/saves/file/<kind>/<id>
		//     download URLs.
		//   Games    — expand preset.games (isos), emit /api/lan/sync/games/<id>/download,
		//     carry extracted_ready.
		//   Apps     — expand preset.apps, emit /api/lan/sync/apps/<id>/download,
		//     carry extracted_ready.
		// Tolerate dangling relation ids (nullify-on-delete leaves gaps).
		Profiles: []syncProfileRef{},
		Games:    []syncGameRef{},
		Apps:     []syncAppRef{},
	}
	man.Counts = manifestCounts{
		Profiles: len(man.Profiles),
		Games:    len(man.Games),
		Apps:     len(man.Apps),
	}

	return e.JSON(http.StatusOK, man)
}

// resolvePreset returns the preset for the selector: a bare id, "active"/"" for
// the active preset. TODO(lan-sync): if several rows have active=true (not yet
// enforced single-active), this returns the first — reconcile with the
// single-active hook.
func resolvePreset(app core.App, sel string) (*core.Record, error) {
	if sel != "" && sel != "active" {
		return app.FindRecordById("sync_presets", sel)
	}
	return app.FindFirstRecordByFilter("sync_presets", "active = true", dbx.Params{})
}

// policyOrDefault falls back to "skip" (the safest non-destructive default) when
// a preset leaves a category's conflict policy unset.
func policyOrDefault(v string) string {
	if v == "" {
		return "skip"
	}
	return v
}
