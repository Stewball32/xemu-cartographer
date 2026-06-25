package lansaves

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Identity manifest — what a single gamertag's box should pull. The nxdk LAN
// client GETs this to discover the player's generated CE + H2 profiles plus the
// shared gametype library and game (XBE) uploads, each with a ready-to-fetch
// download URL.
//
//	GET /api/lan/saves/identity            → list of gamertags with a profile
//	GET /api/lan/saves/identity/{gamertag} → the manifest for one gamertag
//
// Profiles/gametypes/games are read through e.App (superuser context), so the
// per-collection owner rules don't gate the device client — access to these
// endpoints is governed by authorizeLAN (admin JWT or the shared LAN token).
func init() {
	register(func() {
		Group.GET("/identity", handleIdentityList)
		Group.GET("/identity/{gamertag}", handleIdentity)
	})
}

type fileRef struct {
	ID          string          `json:"id"`
	HasFile     bool            `json:"has_file"`
	DownloadURL string          `json:"download_url,omitempty"`
	Info        json.RawMessage `json:"info,omitempty"`
}

type gametypeRef struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Engine      string          `json:"engine"`
	Name        string          `json:"name"`
	HasFile     bool            `json:"has_file"`
	DownloadURL string          `json:"download_url,omitempty"`
	Info        json.RawMessage `json:"info,omitempty"`
}

type gameRef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
}

type identityManifest struct {
	Gamertag  string        `json:"gamertag"`
	CEProfile *fileRef      `json:"ce_profile"`
	H2Profile *fileRef      `json:"h2_profile"`
	Gametypes []gametypeRef `json:"gametypes"`
	Games     []gameRef     `json:"games"`
}

func handleIdentity(e *core.RequestEvent) error {
	gamertag := strings.TrimSpace(e.Request.PathValue("gamertag"))
	if gamertag == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "gamertag is required"})
	}

	man := identityManifest{
		Gamertag:  gamertag,
		Gametypes: []gametypeRef{},
		Games:     []gameRef{},
	}

	if h2 := findProfileByGamertag(e.App, "h2_profiles", gamertag); h2 != nil {
		man.H2Profile = profileRef(h2, "h2-profile")
	}
	if ce := findProfileByGamertag(e.App, "ce_profiles", gamertag); ce != nil {
		man.CEProfile = profileRef(ce, "ce-profile")
	}

	gts, err := e.App.FindAllRecords("gametypes")
	if err == nil {
		for _, r := range gts {
			has := r.GetString("save_bundle") != ""
			ref := gametypeRef{
				ID:      r.Id,
				Title:   r.GetString("title"),
				Engine:  r.GetString("engine"),
				Name:    r.GetString("name"),
				HasFile: has,
				Info:    rawJSON(r, "save_info"),
			}
			if has {
				ref.DownloadURL = "/api/lan/saves/file/gametype/" + r.Id
			}
			man.Gametypes = append(man.Gametypes, ref)
		}
		sort.Slice(man.Gametypes, func(i, j int) bool {
			if man.Gametypes[i].Title != man.Gametypes[j].Title {
				return man.Gametypes[i].Title < man.Gametypes[j].Title
			}
			return man.Gametypes[i].Name < man.Gametypes[j].Name
		})
	}

	games, err := e.App.FindAllRecords("game_titles")
	if err == nil {
		for _, r := range games {
			man.Games = append(man.Games, gameRef{
				ID:          r.Id,
				Name:        r.GetString("name"),
				Description: r.GetString("description"),
				Filename:    r.GetString("file"),
				DownloadURL: "/api/lan/saves/file/game/" + r.Id,
			})
		}
		sort.Slice(man.Games, func(i, j int) bool { return man.Games[i].Name < man.Games[j].Name })
	}

	return e.JSON(http.StatusOK, man)
}

// handleIdentityList returns the distinct user gamertags (the in-game names),
// so a client (or operator) can discover who is set up.
func handleIdentityList(e *core.RequestEvent) error {
	seen := map[string]string{} // lower -> display
	users, err := e.App.FindAllRecords("users")
	if err == nil {
		for _, u := range users {
			gt := strings.TrimSpace(u.GetString("gamertag"))
			if gt == "" {
				continue
			}
			seen[strings.ToLower(gt)] = gt
		}
	}
	out := make([]string, 0, len(seen))
	for _, gt := range seen {
		out = append(out, gt)
	}
	sort.Strings(out)
	return e.JSON(http.StatusOK, map[string]any{"gamertags": out})
}

func profileRef(r *core.Record, kind string) *fileRef {
	has := r.GetString("save_bundle") != ""
	ref := &fileRef{ID: r.Id, HasFile: has, Info: rawJSON(r, "save_info")}
	if has {
		ref.DownloadURL = "/api/lan/saves/file/" + kind + "/" + r.Id
	}
	return ref
}

// findProfileByGamertag resolves the user who owns the gamertag (users.gamertag
// is the single source of truth), then returns that user's profile in the given
// collection (one-per-user via the unique `user` index). Returns nil if no user
// has that gamertag or they have no such profile yet.
func findProfileByGamertag(app core.App, collection, gamertag string) *core.Record {
	user := findUserByGamertag(app, gamertag)
	if user == nil {
		return nil
	}
	rec, err := app.FindFirstRecordByFilter(collection, "user = {:u}", dbx.Params{"u": user.Id})
	if err != nil {
		return nil
	}
	return rec
}

// findUserByGamertag does a case-insensitive match against users.gamertag. User
// counts are modest on a LAN hub, so a full scan + EqualFold is both correct (no
// SQLite collation surprises) and cheap. First match wins if two users somehow
// share a gamertag (uniqueness isn't enforced).
func findUserByGamertag(app core.App, gamertag string) *core.Record {
	want := strings.TrimSpace(gamertag)
	if want == "" {
		return nil
	}
	users, err := app.FindAllRecords("users")
	if err != nil {
		return nil
	}
	for _, u := range users {
		if strings.EqualFold(strings.TrimSpace(u.GetString("gamertag")), want) {
			return u
		}
	}
	return nil
}

func rawJSON(r *core.Record, field string) json.RawMessage {
	s := strings.TrimSpace(r.GetString(field))
	if s == "" || s == "null" {
		return nil
	}
	return json.RawMessage(s)
}
