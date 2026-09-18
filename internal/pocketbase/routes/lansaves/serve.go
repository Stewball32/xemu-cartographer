package lansaves

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb"
	"github.com/xemu-cartographer/xemu-cartographer/internal/gamertags"
)

// File serve — stream a stored save bundle / game upload to the LAN client.
//
//	GET /api/lan/saves/file/{kind}/{id}
//
// kind selects the collection + file field. Profiles and gametypes serve their
// generated `save_bundle` tar (unpack relative to the Xbox E:\ root); game
// serves the raw uploaded `file`. Streaming goes through PocketBase's
// filesystem.Serve, so Range requests work for the large game uploads.
//
// This complements the on-the-fly /download endpoint: /download regenerates
// from a spec, whereas /file serves the already-generated, record-backed
// artifact referenced by the identity manifest.
func init() {
	register(func() {
		Group.GET("/file/{kind}/{id}", handleServeFile)
	})
}

type serveTarget struct {
	collection string
	field      string
}

var serveKinds = map[string]serveTarget{
	"h2-profile": {"h2_profiles", "save_bundle"},
	"ce-profile": {"ce_profiles", "save_bundle"},
	"gametype":   {"gametypes", "save_bundle"},
	"game":       {"game_titles", "file"},
}

func handleServeFile(e *core.RequestEvent) error {
	kind := e.Request.PathValue("kind")
	id := e.Request.PathValue("id")

	target, ok := serveKinds[kind]
	if !ok {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error": "unknown file kind: " + kind + " (want h2-profile|ce-profile|gametype|game)",
		})
	}

	// lan.saves.file is the one LAN verb the group middleware defers to the
	// handler (lanVerb returns an empty action for /file/…): the rule needs
	// the record's owner + gamertag, which only exist once the row is loaded.
	// Before touching the row, probe the caller's scope on the URL-derived
	// resource so a principal whose scopes do not cover lan.saves.file for
	// this kind/id is refused without learning whether the id exists (no
	// 404-vs-403 oracle). The probe carries the caller's own gamertag binding
	// as the owner tags, so PD-8 holds by construction and only the scope
	// decides here.
	d := pb.Default()
	probe := authz.LANFile(kind, id, "")
	if tags := pb.Get(e).Extra["gamertags"]; tags != "" {
		probe.Extra["gamertag"] = tags
	}
	if err := pb.Check(d, e, authz.ActionLANSavesFile, probe); err != nil {
		return lanDenied(e, err)
	}

	rec, err := e.App.FindRecordById(target.collection, id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "record not found"})
	}

	// The real check on the loaded record: a station key bound to gamertags
	// may only pull profile files owned by one of them (PD-8); gametype /
	// game artifacts carry no owner.
	if err := pb.Check(d, e, authz.ActionLANSavesFile, fileResource(e.App, kind, id, rec)); err != nil {
		return lanDenied(e, err)
	}

	filename := rec.GetString(target.field)
	if filename == "" {
		// CE profiles legitimately have no file yet (generation deferred).
		return e.JSON(http.StatusNotFound, map[string]string{
			"error": "no file generated for this record yet",
		})
	}

	fsys, err := e.App.NewFilesystem()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "filesystem unavailable"})
	}
	defer fsys.Close()

	key := rec.BaseFilesPath() + "/" + filename
	if err := fsys.Serve(e.Response, e.Request, key, filename); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "serve failed: " + err.Error()})
	}
	return nil
}

// lanDenied renders a pb.Check denial in the LAN group's JSON shape
// ({"error": ...}, the same body pb.AuthorizeLAN and every other /api/lan
// handler write) instead of PocketBase's {"status","message","data"} apis
// envelope, so the nxdk client sees one error format across the group. A
// 403 is exactly the middleware's {"error":"forbidden"}; any other apis
// status (a 401 is unreachable here — AuthorizeLAN already rejected the
// unresolved caller) keeps its status + message; a non-apis error falls
// back to a plain 403.
func lanDenied(e *core.RequestEvent, err error) error {
	var apiErr *router.ApiError
	if errors.As(err, &apiErr) && apiErr.Status > 0 && apiErr.Status != http.StatusForbidden {
		return e.JSON(apiErr.Status, map[string]string{"error": apiErr.Message})
	}
	return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
}

// fileResource builds the lan.saves.file resource for a loaded record: the
// profile kinds (h2-profile / ce-profile) are owned by their `user` relation
// and carry every gamertag that user may be bound under in Extra["gamertag"]
// (comma-separated, the list form predStationFile intersects with the key's
// binding) for the PD-8 station binding; gametype / game rows are unowned
// library artifacts.
func fileResource(app core.App, kind, id string, rec *core.Record) authz.Resource {
	owner := ""
	if strings.HasSuffix(kind, "-profile") {
		owner = rec.GetString("user")
	}
	res := authz.LANFile(kind, id, owner)
	if owner != "" {
		res.Extra["gamertag"] = ownerGamertags(app, owner)
	}
	return res
}

// ownerGamertags resolves every sanitized (lower/trim) gamertag a profile file
// may be filed under, comma-joined and sorted: all of the owner's usable
// gamertags rows (approved / allowed — the same set a station key's binding
// is minted from, so a station bound to a non-default tag still reaches its
// own profiles) plus the owner's default_gamertag row. Only when those yield
// nothing does the legacy free-text users.gamertag (what the identity
// manifest keys on) stand in — it is user-editable and unmoderated, so it
// must not widen a user who already has real gamertags rows. "" when the
// user has none — a bound station then fails closed.
func ownerGamertags(app core.App, userID string) string {
	user, err := app.FindRecordById("users", userID)
	if err != nil {
		return ""
	}
	seen := map[string]bool{}
	add := func(s string) {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			seen[s] = true
		}
	}
	if tags, err := gamertags.UsableForUser(app, userID); err == nil {
		for _, s := range tags {
			add(s)
		}
	}
	if tagID := user.GetString("default_gamertag"); tagID != "" {
		if tag, err := app.FindRecordById("gamertags", tagID); err == nil {
			if s := tag.GetString("sanitized"); strings.TrimSpace(s) != "" {
				add(s)
			} else {
				add(tag.GetString("tag"))
			}
		}
	}
	if len(seen) == 0 {
		add(user.GetString("gamertag"))
	}

	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}
