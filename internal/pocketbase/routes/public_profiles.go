package routes

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// GET /api/public/profiles?gamertags=a,b,c
//
// Public, read-only resolver from a live in-game player name → that player's
// broadcast identity: the display name to put on air, their avatar, and their
// profile APPEARANCE (CE armor colour + H2 emblem/appearance byte map). It exists
// so an anonymous OBS overlay (which holds no PB session) can show who is actually
// playing, keyed by the live roster's scraped player name.
//
// Why a dedicated public route: `users`, `gamertags`, `ce_profiles` and
// `h2_profiles` are all owner-or-admin scoped (read rules gate on
// @request.auth.id), so none can be read anonymously. This endpoint reads them
// server-side (e.App superuser context) and returns ONLY broadcast-safe identity —
// never emails, save bundles, or anything else.
//
// RESOLUTION GOES THROUGH THE `gamertags` COLLECTION, not the `users.gamertag`
// text field. That is deliberate: `gamertags` is the moderated identity system
// (the M22 queue), and this output goes on a live broadcast. `users.gamertag` is
// free text the user sets for their in-game Halo profile with no review at all, so
// matching on it would let anyone put an unreviewed string on air by editing their
// own profile. Only statuses the moderators have affirmatively cleared resolve
// here — see profileVisibleStatuses.
//
// Response: { "profiles": { "<lower-scraped-name>": {gamertag, display?, avatar?,
// ce?, h2?} } }. A name that resolves to nothing is simply absent, and the caller
// falls back to the trimmed scraped name + a placeholder avatar.
func init() {
	register(registerPublicProfilesRoute)
}

const maxProfileLookup = 64

// profileVisibleStatuses are the `gamertags.status` values that may appear on a
// broadcast. The four-state queue is pending / approved / allowed / blocked;
// only the two affirmative review outcomes qualify. `pending` is excluded on
// purpose — an unreviewed name reaching the stream is exactly what the queue
// exists to prevent — and such a player simply renders under their trimmed
// in-game name instead. Note this is STRICTER than the `status != "blocked"`
// convention used elsewhere (internal/gamertags, the /u/ profile page), because
// those gate matching and authorization rather than publication.
var profileVisibleStatuses = map[string]bool{"approved": true, "allowed": true}

type publicCEAppearance struct {
	Color int `json:"color"`
}

type publicH2Appearance struct {
	Appearance map[string]int `json:"appearance"`
}

type publicProfile struct {
	// Gamertag echoes the requested name in its original casing.
	Gamertag string `json:"gamertag"`
	// Display is the player's DEFAULT gamertag — the canonical handle to put on
	// air in place of whatever the console happened to be logged in as. Omitted
	// when the user has no usable default, in which case the caller keeps the
	// trimmed scraped name.
	Display string `json:"display,omitempty"`
	// Avatar is the user's PocketBase avatar file (the built-in users.avatar
	// upload) as a same-origin thumb URL, '' when the user has none. The file is
	// world-readable by PB's own rules (the field is not Protected), so handing
	// the URL to an anonymous overlay leaks nothing the file server wouldn't.
	Avatar string `json:"avatar,omitempty"`
	// Motto is the plate's second line (users.motto, ≤40 chars, big plates
	// only) and Plate the 600×100 banner art URL of the ORGANIZER-CURATED
	// nameplate the user picked (users.nameplate → nameplates.art). Both are
	// deliberate broadcast surfaces from the settings Stream tab.
	Motto string              `json:"motto,omitempty"`
	Plate string              `json:"plate,omitempty"`
	CE    *publicCEAppearance `json:"ce"`
	H2    *publicH2Appearance `json:"h2"`
}

// userAvatarPath builds the public file URL for a users.avatar upload — PB
// serves record files at /api/files/{collection}/{recordId}/{filename}; the
// thumb query keeps the broadcast payload small (cards render ~40-90 px).
// ” in → ” out.
func userAvatarPath(userID, filename string) string {
	if strings.TrimSpace(filename) == "" {
		return ""
	}
	return "/api/files/users/" + userID + "/" + filename + "?thumb=100x100"
}

// nameplateArtPath resolves a users.nameplate relation to its banner-art file
// URL ("" for no pick, a dangling row, or a plate with no art yet). Hidden
// (unselectable) banners still serve — current wearers keep them; only the
// settings picker shrinks. 600×100 source art is already broadcast-sized, so
// no thumb query.
func nameplateArtPath(app core.App, nameplateID string) string {
	if nameplateID == "" {
		return ""
	}
	plate, err := app.FindRecordById("nameplates", nameplateID)
	if err != nil || plate == nil {
		return ""
	}
	art := plate.GetString("art")
	if art == "" {
		return ""
	}
	return "/api/files/nameplates/" + plate.Id + "/" + art
}

func registerPublicProfilesRoute(se *core.ServeEvent) {
	se.Router.GET("/api/public/profiles", func(e *core.RequestEvent) error {
		out := map[string]publicProfile{}
		tags := parseGamertagList(e.Request.URL.Query().Get("gamertags"), maxProfileLookup)
		if len(tags) == 0 {
			return e.JSON(http.StatusOK, map[string]any{"profiles": out})
		}

		// Load the gamertag queue once and index the broadcast-visible rows by
		// their `sanitized` column — the same lowercased+trimmed form the scraper
		// matching path uses everywhere else (internal/gamertags,
		// manager.Membership), so a scraped name lines up without per-call
		// normalisation guesswork. Modest counts on a tournament hub, and an
		// in-memory scan avoids SQLite collation surprises.
		gamertags, err := e.App.FindAllRecords("gamertags")
		if err != nil {
			return e.JSON(http.StatusOK, map[string]any{"profiles": out})
		}
		type tagRow struct {
			userID string
			tag    string
		}
		bySanitized := map[string]tagRow{}
		tagByID := map[string]*core.Record{} // for the default_gamertag lookup below
		for _, g := range gamertags {
			tagByID[g.Id] = g
			if !profileVisibleStatuses[g.GetString("status")] {
				continue
			}
			key := sanitizedKey(g.GetString("sanitized"), g.GetString("tag"))
			if key == "" {
				continue
			}
			// The unique index on gamertags is (user, sanitized), so two DIFFERENT
			// users may legitimately hold the same tag. Nothing in the data can
			// break that tie, so first-wins — and the roster is the wrong place to
			// guess. Rare in practice on a LAN roster.
			if _, seen := bySanitized[key]; !seen {
				bySanitized[key] = tagRow{userID: g.GetString("user"), tag: g.GetString("tag")}
			}
		}

		// Which requested names resolve to a user? Seed the response + track the
		// user→key mapping so we can attach profiles in one pass each.
		keyForUser := map[string]string{} // userID -> lower scraped name (response key)
		for _, t := range tags {
			lt := strings.ToLower(t)
			row, ok := bySanitized[lt]
			if !ok {
				continue
			}
			u, err := e.App.FindRecordById("users", row.userID)
			if err != nil || u == nil {
				continue
			}
			// A deleted or banned account keeps its rows but must not go on air.
			if u.GetBool("is_deleted") || u.GetBool("is_banned") {
				continue
			}
			var defTag, defStatus string
			if def := tagByID[u.GetString("default_gamertag")]; def != nil {
				defTag, defStatus = def.GetString("tag"), def.GetString("status")
			}
			keyForUser[u.Id] = lt
			out[lt] = publicProfile{
				Gamertag: t,
				Display:  displayNameFor(defTag, defStatus, row.tag),
				Avatar:   userAvatarPath(u.Id, u.GetString("avatar")),
				Motto:    strings.TrimSpace(u.GetString("motto")),
				Plate:    nameplateArtPath(e.App, u.GetString("nameplate")),
			}
		}
		if len(keyForUser) == 0 {
			return e.JSON(http.StatusOK, map[string]any{"profiles": out})
		}
		tagForUser := keyForUser

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

		// Drop entries that resolved to a user but carry nothing worth sending —
		// no display name, no avatar, no plate identity, no game profile. The
		// caller treats "absent" as "use the trimmed scraped name + placeholder
		// avatar". A user with only a display name is KEPT: the name swap is
		// useful on its own, even before they upload an avatar.
		for k, v := range out {
			if v.Display == "" && v.Avatar == "" && v.Motto == "" && v.Plate == "" && v.CE == nil && v.H2 == nil {
				delete(out, k)
			}
		}
		return e.JSON(http.StatusOK, map[string]any{"profiles": out})
	})
}

// sanitizedKey is a gamertag row's match key: the `sanitized` column when the
// row carries one, else the same normalisation applied to `tag`. Mirrors
// internal/gamertags.SanitizedForUser so both sides of the match agree on older
// rows written before `sanitized` was populated.
func sanitizedKey(sanitized, tag string) string {
	if s := strings.ToLower(strings.TrimSpace(sanitized)); s != "" {
		return s
	}
	return strings.ToLower(strings.TrimSpace(tag))
}

// displayNameFor picks the handle to put on air: the user's DEFAULT gamertag
// when it is itself broadcast-visible, otherwise the tag we matched on (already
// known visible, since that is how we got here).
//
// Gating the default on its own status matters — a user can point
// default_gamertag at a row that is pending or blocked, and without this check
// the swap would launder an unreviewed name onto the stream via a different,
// approved tag.
func displayNameFor(defaultTag, defaultStatus, matchedTag string) string {
	if profileVisibleStatuses[defaultStatus] {
		if t := strings.TrimSpace(defaultTag); t != "" {
			return t
		}
	}
	return strings.TrimSpace(matchedTag)
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
