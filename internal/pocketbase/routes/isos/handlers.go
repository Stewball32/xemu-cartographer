package isos

import (
	"net/http"
	"sort"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/isoingest"
	"github.com/Stewball32/xemu-cartographer/internal/lansync"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/offsets"
)

func init() {
	register(registerList)
	register(registerInbox)
	register(registerIngest)
	register(registerOffsetSets)
	register(registerGet)
	register(registerUpdate)
	register(registerDelete)
}

const collectionName = "isos"

// isoView is the JSON projection of an `isos` record for admin responses.
//
// Under the ingest model the managed disc is <id>.iso; `filename` holds the
// ORIGINAL inbox filename (provenance only). content_hash is the drift anchor;
// drift_detected flags a row whose managed bytes no longer match it (forced
// unavailable). extracted_* / footprint_bytes are the derived cache status.
// server_iso optionally links another catalog entry as this game's server build.
func isoView(r *core.Record) map[string]any {
	return map[string]any{
		"id":              r.Id,
		"name":            r.GetString("name"),
		"filename":        r.GetString("filename"),
		"title_id":        r.GetString("title_id"),
		"description":     r.GetString("description"),
		"available":       r.GetBool("available"),
		"server_iso":      r.GetString("server_iso"),
		"offset_set":      r.GetString("offset_set"),
		"content_hash":    r.GetString("content_hash"),
		"drift_detected":  r.GetBool("drift_detected"),
		"file_size":       r.GetInt("file_size"),
		"extracted_ready": r.GetBool("extracted_ready"),
		"extracted_at":    r.GetString("extracted_at"),
		"footprint_bytes": r.GetInt("footprint_bytes"),
		"created":         r.GetString("created"),
		"updated":         r.GetString("updated"),
	}
}

// resolveServerISO validates an optional server_iso relation target for the game
// record identified by selfID. "" clears the link; a non-empty value must
// reference an EXISTING catalog entry and may not be the game itself. Returns the
// id to store (or "") and a human message when it's rejected (400).
func resolveServerISO(app core.App, raw, selfID string) (string, string) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", ""
	}
	if id == selfID {
		return "", "a game cannot be its own server ISO"
	}
	if _, err := app.FindRecordById(collectionName, id); err != nil {
		return "", "server_iso must reference an existing ISO catalog entry"
	}
	return id, ""
}

// GET /api/admin/isos — every catalog entry, by name.
func registerList() {
	Group.GET("", func(e *core.RequestEvent) error {
		records, err := e.App.FindAllRecords(collectionName)
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		sort.SliceStable(records, func(i, j int) bool {
			return strings.ToLower(records[i].GetString("name")) < strings.ToLower(records[j].GetString("name"))
		})
		out := make([]map[string]any, 0, len(records))
		for _, r := range records {
			out = append(out, isoView(r))
		}
		return e.JSON(http.StatusOK, out)
	})
}

// GET /api/admin/isos/inbox — disc images staged in the ingest drop-zone,
// pending ingest into the managed library.
func registerInbox() {
	Group.GET("/inbox", func(e *core.RequestEvent) error {
		files, err := isoingest.InboxPending(lansync.Load())
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return e.JSON(http.StatusOK, map[string]any{"files": files})
	})
}

// POST /api/admin/isos/ingest — scan the inbox and ingest every pending file:
// hash → dedupe → create row → atomic move to <id>.iso → freeze → kick extract.
// Returns per-file ingested / skipped(duplicate) / errors.
func registerIngest() {
	Group.POST("/ingest", func(e *core.RequestEvent) error {
		res, err := isoingest.IngestInbox(e.App, lansync.Load())
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return e.JSON(http.StatusOK, res)
	})
}

// offsetSetExists reports whether an offset-set id is registered (any game —
// the game an ISO boots isn't knowable from the record alone; a game-mismatched
// assignment degrades to the baseline at bind time with a logged warning).
func offsetSetExists(id string) bool {
	for _, s := range offsets.All() {
		if s.ID == id {
			return true
		}
	}
	return false
}

// GET /api/admin/isos/offset-sets — the registered offset sets (per game, with
// the baseline flagged), for the assignment picker. An ISO with offset_set ""
// rides the detected game's baseline.
func registerOffsetSets() {
	Group.GET("/offset-sets", func(e *core.RequestEvent) error {
		return e.JSON(http.StatusOK, offsets.All())
	})
}

// GET /api/admin/isos/{id} — one catalog entry.
func registerGet() {
	Group.GET("/{id}", func(e *core.RequestEvent) error {
		rec, err := e.App.FindRecordById(collectionName, e.Request.PathValue("id"))
		if err != nil {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "iso not found"})
		}
		return e.JSON(http.StatusOK, isoView(rec))
	})
}

// updateBody is the PATCH payload; every field is optional (pointer = provided).
// The managed file is ID-anchored, so the display name (and metadata) is freely
// editable. title_id is deliberately ABSENT — it's server-owned, auto-extracted
// from the disc's boot XBE at ingest/extract time. A provided server_iso of ""
// clears the link.
type updateBody struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Available   *bool   `json:"available"`
	ServerISO   *string `json:"server_iso"`
	OffsetSet   *string `json:"offset_set"`
}

// PATCH /api/admin/isos/{id} — partial metadata update.
func registerUpdate() {
	Group.PATCH("/{id}", func(e *core.RequestEvent) error {
		rec, err := e.App.FindRecordById(collectionName, e.Request.PathValue("id"))
		if err != nil {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "iso not found"})
		}
		var body updateBody
		if err := e.BindBody(&body); err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if body.Name != nil {
			name := strings.TrimSpace(*body.Name)
			if name == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "name cannot be empty"})
			}
			rec.Set("name", name)
		}
		if body.Description != nil {
			rec.Set("description", *body.Description)
		}
		if body.Available != nil {
			rec.Set("available", *body.Available)
		}
		if body.ServerISO != nil {
			serverID, msg := resolveServerISO(e.App, *body.ServerISO, rec.Id)
			if msg != "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": msg})
			}
			rec.Set("server_iso", serverID)
		}
		if body.OffsetSet != nil {
			id := strings.TrimSpace(*body.OffsetSet)
			if id != "" && !offsetSetExists(id) {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "unknown offset set: " + id})
			}
			rec.Set("offset_set", id)
		}
		if err := e.App.Save(rec); err != nil {
			return e.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		}
		return e.JSON(http.StatusOK, isoView(rec))
	})
}

// DELETE /api/admin/isos/{id} — remove the catalog entry AND its managed disc +
// extracted tree (the file is ID-owned now; delete-to-replace is the flow).
func registerDelete() {
	Group.DELETE("/{id}", func(e *core.RequestEvent) error {
		rec, err := e.App.FindRecordById(collectionName, e.Request.PathValue("id"))
		if err != nil {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "iso not found"})
		}
		if err := isoingest.DeleteManaged(e.App, lansync.Load(), rec); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return e.NoContent(http.StatusNoContent)
	})
}
