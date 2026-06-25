package lansaves

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
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

	rec, err := e.App.FindRecordById(target.collection, id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "record not found"})
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
