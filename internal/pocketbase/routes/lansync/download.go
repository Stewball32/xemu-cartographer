package lansync

import (
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// Kiosk-scoped download endpoints (SPEC §4.4) — headless stations pull the
// server-extracted artifacts:
//
//	GET /api/lan/sync/dl/game/{id}  → tar of the extracted disc tree for isos/{id}
//	GET /api/lan/sync/dl/app/{id}   → app zip for apps/{id}
//
// (Profiles reuse the existing GET /api/lan/saves/download?... — already
// authorizeLAN-gated.) The manifest hands the client the exact `path`, so the
// client builds no URLs.
//
// SCAFFOLD: both validate the row + that its extracted cache is ready, then 501
// with a TODO. The transport (tar of the extracted tree for games; the stored
// zip for apps) is deferred.
func init() {
	register(func() {
		Group.GET("/dl/game/{id}", handleGameDownload)
		Group.GET("/dl/app/{id}", handleAppDownload)
	})
}

func handleGameDownload(e *core.RequestEvent) error {
	return handleExtractedDownload(e, "isos", "game", "tar")
}

func handleAppDownload(e *core.RequestEvent) error {
	return handleExtractedDownload(e, "apps", "app", "zip")
}

// handleExtractedDownload is the shared stub for the game/app pulls: resolve the
// row, require the extracted cache to be ready, then 501.
func handleExtractedDownload(e *core.RequestEvent, collection, category, format string) error {
	id := strings.TrimSpace(e.Request.PathValue("id"))
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	rec, err := e.App.FindRecordById(collection, id)
	if err != nil || rec == nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": category + " record not found"})
	}
	if !rec.GetBool("extracted_ready") {
		// The extraction hook hasn't produced (or has evicted) the cache.
		return e.JSON(http.StatusConflict, map[string]string{
			"error": "extracted cache not ready — the extraction hook has not produced this artifact yet",
		})
	}

	// TODO(lan-sync): stream the artifact.
	//   game → tar the extracted tree at rec.GetString("extracted_path")
	//          (mirror routes/lansaves/download.go's archive path).
	//   app  → serve the stored zip (extracted_path or the PB file blob).
	// Add a disk-space / path-safety (zip-slip) pre-flight. For now, advertise
	// the resolved cache location so the client contract can be exercised.
	return e.JSON(http.StatusNotImplemented, map[string]any{
		"error":          "artifact download not implemented (scaffold)",
		"category":       category,
		"format":         format,
		"id":             id,
		"extracted_path": rec.GetString("extracted_path"),
	})
}
