package lansync

import (
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// Kiosk-scoped download endpoints — headless stations pull the derived
// EXTRACTED trees for a game (iso) / app.
//
//	GET /api/lan/sync/games/{id}/download  → extracted disc tree for isos/{id}
//	GET /api/lan/sync/apps/{id}/download   → extracted app tree for apps/{id}
//
// (Profiles + gametypes already download via /api/lan/saves/file/*.)
//
// SCAFFOLD: both are stubs — they validate the row + that its extracted cache is
// ready, then 501 with a TODO. The transport (tar stream of the extracted tree,
// like lansaves) is deferred.
func init() {
	register(func() {
		Group.GET("/games/{id}/download", handleGameDownload)
		Group.GET("/apps/{id}/download", handleAppDownload)
	})
}

func handleGameDownload(e *core.RequestEvent) error {
	return handleExtractedDownload(e, "isos")
}

func handleAppDownload(e *core.RequestEvent) error {
	return handleExtractedDownload(e, "apps")
}

// handleExtractedDownload is the shared stub for the game/app extracted-tree
// pulls: resolve the row, require the extracted cache to be ready, then 501.
func handleExtractedDownload(e *core.RequestEvent, collection string) error {
	id := strings.TrimSpace(e.Request.PathValue("id"))
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	rec, err := e.App.FindRecordById(collection, id)
	if err != nil || rec == nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": collection + " record not found"})
	}
	if !rec.GetBool("extracted_ready") {
		// The extraction hook hasn't produced (or has evicted) the cache.
		return e.JSON(http.StatusConflict, map[string]string{
			"error": "extracted cache not ready — the extraction hook has not produced this tree yet",
		})
	}

	// TODO(lan-sync): stream the extracted tree at rec.GetString("extracted_path")
	// as a tar (mirror routes/lansaves/download.go's archive path), with a
	// disk-space / path-safety pre-flight. For now, advertise the resolved cache
	// location so the client contract can be exercised end-to-end.
	return e.JSON(http.StatusNotImplemented, map[string]any{
		"error":          "extracted-tree download not implemented (scaffold)",
		"collection":     collection,
		"id":             id,
		"extracted_path": rec.GetString("extracted_path"),
	})
}
