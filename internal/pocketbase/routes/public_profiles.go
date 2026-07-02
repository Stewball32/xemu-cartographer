package routes

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// GET /api/public/profiles?gamertags=a,b,c
//
// Public, read-only, cosmetic-only resolver from in-game gamertag → that user's
// profile APPEARANCE (CE armor colour + H2 emblem/appearance byte map). It exists
// so an anonymous OBS overlay (which holds only a scoped read-only WS token, not a
// PB session) can render each player's PROFILE avatar — their chosen Spartan +
// emblem — on the broadcast cards, keyed by the live roster's player name.
//
// Why a dedicated public route: `ce_profiles` / `h2_profiles` and `users.gamertag`
// are all owner-scoped (read rules gate on @request.auth.id), so the collections
// can't be read anonymously. This endpoint reads them server-side (e.App superuser
// context) and returns ONLY the cosmetic appearance — never the signed save
// bundles, emails, or anything else — which is safe to expose: a gamertag and its
// emblem are already public-facing stream identity.
//
// Response: { "profiles": { "<lower-gamertag>": {gamertag, ce?, h2?} } }. Only
// gamertags that resolve to a user WITH at least one profile are included; the
// caller falls back to a generic Spartan for the rest.
func init() {
	register(registerPublicProfilesRoute)
}

const maxProfileLookup = 64

type publicCEAppearance struct {
	Color int `json:"color"`
}

type publicH2Appearance struct {
	Appearance map[string]int `json:"appearance"`
}

type publicProfile struct {
	Gamertag string              `json:"gamertag"`
	CE       *publicCEAppearance `json:"ce"`
	H2       *publicH2Appearance `json:"h2"`
}

func registerPublicProfilesRoute(se *core.ServeEvent) {
	se.Router.GET("/api/public/profiles", func(e *core.RequestEvent) error {
		out := map[string]publicProfile{}
		tags := parseGamertagList(e.Request.URL.Query().Get("gamertags"), maxProfileLookup)
		if len(tags) == 0 {
			return e.JSON(http.StatusOK, map[string]any{"profiles": out})
		}

		// Load users once and index by lowercased gamertag (modest counts on a
		// tournament hub; a scan avoids SQLite collation surprises — mirrors
		// lansaves.findUserByGamertag).
		users, err := e.App.FindAllRecords("users")
		if err != nil {
			return e.JSON(http.StatusOK, map[string]any{"profiles": out})
		}
		userByTag := map[string]*core.Record{}
		for _, u := range users {
			gt := strings.ToLower(strings.TrimSpace(u.GetString("gamertag")))
			if gt == "" {
				continue
			}
			if _, seen := userByTag[gt]; !seen {
				userByTag[gt] = u // first match wins (gamertag uniqueness isn't enforced)
			}
		}

		// Which requested tags resolve to a user? Seed the response + track the
		// user→tag mapping so we can attach profiles in one pass each.
		tagForUser := map[string]string{} // userID -> lower gamertag (response key)
		for _, t := range tags {
			lt := strings.ToLower(t)
			if u, ok := userByTag[lt]; ok {
				tagForUser[u.Id] = lt
				out[lt] = publicProfile{Gamertag: t}
			}
		}
		if len(tagForUser) == 0 {
			return e.JSON(http.StatusOK, map[string]any{"profiles": out})
		}

		if ces, err := e.App.FindAllRecords("ce_profiles"); err == nil {
			for _, r := range ces {
				lt, ok := tagForUser[r.GetString("user")]
				if !ok {
					continue
				}
				p := out[lt]
				p.CE = &publicCEAppearance{Color: ceColorFromSettings(r.GetString("settings"))}
				out[lt] = p
			}
		}
		if h2s, err := e.App.FindAllRecords("h2_profiles"); err == nil {
			for _, r := range h2s {
				lt, ok := tagForUser[r.GetString("user")]
				if !ok {
					continue
				}
				if appr := appearanceFromJSON(r.GetString("appearance")); appr != nil {
					p := out[lt]
					p.H2 = &publicH2Appearance{Appearance: appr}
					out[lt] = p
				}
			}
		}

		// Drop tags that resolved to a user but carry no profile at all — nothing
		// to render, and the caller treats "absent" as "fall back to generic".
		for k, v := range out {
			if v.CE == nil && v.H2 == nil {
				delete(out, k)
			}
		}
		return e.JSON(http.StatusOK, map[string]any{"profiles": out})
	})
}

// parseGamertagList splits the comma-separated `gamertags` query into a trimmed,
// non-empty, case-insensitively-deduped list, capped at `max` (defensive bound
// against an oversized query). Order is preserved (first occurrence wins).
func parseGamertagList(raw string, max int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		t := strings.TrimSpace(part)
		if t == "" {
			continue
		}
		lt := strings.ToLower(t)
		if seen[lt] {
			continue
		}
		seen[lt] = true
		out = append(out, t)
		if len(out) >= max {
			break
		}
	}
	return out
}

// ceColorFromSettings pulls the CE armor `color` index out of a ce_profiles
// `settings` JSON blob. Returns 0 (White) when absent/unparseable — the same
// default the editor seeds.
func ceColorFromSettings(settingsJSON string) int {
	s := strings.TrimSpace(settingsJSON)
	if s == "" || s == "null" {
		return 0
	}
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(s), &m) != nil {
		return 0
	}
	raw, ok := m["color"]
	if !ok {
		return 0
	}
	var n float64
	if json.Unmarshal(raw, &n) != nil {
		return 0
	}
	return int(n)
}

// appearanceFromJSON decodes an h2_profiles `appearance` byte map (key → 0..255)
// into map[string]int, keeping only numeric byte-range values. Returns nil for an
// empty/missing/`{}` map (meaning "use the template default" — nothing to render).
func appearanceFromJSON(appearanceJSON string) map[string]int {
	s := strings.TrimSpace(appearanceJSON)
	if s == "" || s == "null" || s == "{}" {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(s), &m) != nil {
		return nil
	}
	out := map[string]int{}
	for k, raw := range m {
		var n float64
		if json.Unmarshal(raw, &n) != nil {
			continue
		}
		if n < 0 || n > 255 {
			continue
		}
		out[k] = int(n)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
