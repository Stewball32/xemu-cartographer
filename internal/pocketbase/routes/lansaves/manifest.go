package lansaves

import (
	"net/http"
	"net/url"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/diskspace"
	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// GET /api/lan/saves/manifest — the browse catalog for the nxdk LAN client.
//
// This is the listing endpoint the on-Xbox client pages through to discover
// what it can install: the game/title grouping plus the concrete, downloadable
// save items the generator can currently produce. The catalog is GROUNDED in
// the embedded templates — every item is generated server-side here (a cheap
// in-memory template-patch) so a listed item is, by construction, one the hub
// can actually build and sign. An item that fails to generate is silently
// skipped rather than advertised and then 500-ing at download time.
//
// Each item carries a ready-to-GET download `path` (relative URL + query
// string) and the item's authoritative `fatx_dir` + `footprint_bytes`, so the
// tiny client needs no URL-building or footprint math of its own — it lists
// `name`, and on select issues `GET path` (optionally appending the disk-space
// pre-flight params). Fields are flat strings/numbers to keep the on-Xbox JSON
// reader trivial.
//
// Purely additive: this file registers a brand-new route through the existing
// package-local register() hook and introduces no change to any existing
// route, type, handler, or the access policy (it inherits authorizeLAN like
// every other /api/lan/saves/* endpoint).
func init() {
	register(func() {
		Group.GET("/manifest", func(e *core.RequestEvent) error {
			return e.JSON(http.StatusOK, buildManifest())
		})
	})
}

// manifestGame is a title/grouping entry for the client's "Games" view. Games
// are containers, not downloadable units (this hub generates saves, not game
// images), so they carry no download path — the client uses them to label and
// filter the profile/gametype lists.
type manifestGame struct {
	ID       string   `json:"id"`
	Category string   `json:"category"` // always "game"
	Title    string   `json:"title"`    // "ce" | "h2"
	TitleID  string   `json:"title_id"` // FATX title-id dir, e.g. "4d530004"
	Label    string   `json:"label"`
	Kinds    []string `json:"kinds"` // which categories exist under this title
}

// manifestItem is one concrete, downloadable save (a profile or a gametype).
type manifestItem struct {
	ID             string `json:"id"`
	Category       string `json:"category"` // "profile" | "gametype"
	Title          string `json:"title"`    // "ce" | "h2"
	TitleID        string `json:"title_id"`
	TitleLabel     string `json:"title_label"`
	Kind           string `json:"kind"`             // "gametype" | "profile"
	Engine         string `json:"engine,omitempty"` // CE gametypes only
	Name           string `json:"name"`             // display name == in-game save name
	Format         string `json:"format"`           // transport the client should request ("tar")
	FatxDir        string `json:"fatx_dir"`         // authoritative E:\ install dir, e.g. UDATA/4d530004/G-Slayer
	FootprintBytes uint64 `json:"footprint_bytes"`  // FATX cluster-rounded on-disk size
	Path           string `json:"path"`             // ready-to-GET download URL (relative)
}

// manifestResponse is the top-level browse payload.
type manifestResponse struct {
	Hub     string         `json:"hub"`
	Version int            `json:"version"`
	Games   []manifestGame `json:"games"`
	Items   []manifestItem `json:"items"`
	Count   int            `json:"count"`
}

// ceEngineDisplay gives each CE engine a friendly default save name. The name
// becomes the on-disk save dir (G-<name>), so these are deliberately short and
// FATX-safe.
var ceEngineDisplay = map[string]string{
	"slayer":  "Slayer",
	"ctf":     "CTF",
	"king":    "King",
	"oddball": "Oddball",
	"race":    "Race",
}

func ceDisplayName(engine string) string {
	if n, ok := ceEngineDisplay[engine]; ok {
		return n
	}
	return engine
}

func buildManifest() manifestResponse {
	resp := manifestResponse{
		Hub:     "xemu-cartographer",
		Version: 1,
		Games: []manifestGame{
			{
				ID:       "ce",
				Category: "game",
				Title:    halosave.TitleCE,
				TitleID:  halosave.TitleIDHaloCE,
				Label:    "Halo: Combat Evolved",
				Kinds:    []string{halosave.KindGametype},
			},
			{
				ID:       "h2",
				Category: "game",
				Title:    halosave.TitleH2,
				TitleID:  halosave.TitleIDHalo2,
				Label:    "Halo 2",
				Kinds:    []string{halosave.KindProfile, halosave.KindGametype},
			},
		},
	}

	// CE gametypes — one per engine that has an embedded template.
	for _, eng := range halosave.CEEngines() {
		req := halosave.BuildRequest{
			Title:  halosave.TitleCE,
			Kind:   halosave.KindGametype,
			Engine: eng,
			Name:   ceDisplayName(eng),
		}
		if it, ok := makeManifestItem("gametype", "Halo: Combat Evolved", req); ok {
			resp.Items = append(resp.Items, it)
		}
	}

	// H2 gametype — single embedded sample (mode == H2GametypeMode).
	if it, ok := makeManifestItem("gametype", "Halo 2", halosave.BuildRequest{
		Title: halosave.TitleH2,
		Kind:  halosave.KindGametype,
		Name:  "Slayer",
	}); ok {
		resp.Items = append(resp.Items, it)
	}

	// H2 player profile — single embedded template.
	if it, ok := makeManifestItem("profile", "Halo 2", halosave.BuildRequest{
		Title: halosave.TitleH2,
		Kind:  halosave.KindProfile,
		Name:  "CARTOG",
	}); ok {
		resp.Items = append(resp.Items, it)
	}

	resp.Count = len(resp.Items)
	return resp
}

// makeManifestItem generates the save for a spec to obtain its authoritative
// FATX dir + footprint, and assembles the download path. Returns ok=false (and
// is skipped by the caller) if the generator cannot build this spec, so the
// manifest never advertises an item the hub cannot actually serve.
func makeManifestItem(category, titleLabel string, req halosave.BuildRequest) (manifestItem, bool) {
	set, err := halosave.Build(req)
	if err != nil {
		return manifestItem{}, false
	}
	sizes := make([]int, len(set.Files))
	for i, f := range set.Files {
		sizes[i] = f.Size
	}

	q := url.Values{}
	q.Set("title", req.Title)
	q.Set("kind", req.Kind)
	q.Set("name", req.Name)
	if req.Engine != "" {
		q.Set("engine", req.Engine)
	}
	q.Set("format", "tar")

	id := req.Title + "-" + req.Kind
	if req.Engine != "" {
		id += "-" + req.Engine
	} else {
		id += "-" + req.Name
	}

	return manifestItem{
		ID:             id,
		Category:       category,
		Title:          req.Title,
		TitleID:        set.TitleID,
		TitleLabel:     titleLabel,
		Kind:           req.Kind,
		Engine:         req.Engine,
		Name:           req.Name,
		Format:         "tar",
		FatxDir:        set.FatxDir,
		FootprintBytes: diskspace.FATXFootprint(sizes, cfg.FATXCluster),
		Path:           "/api/lan/saves/download?" + q.Encode(),
	}, true
}
