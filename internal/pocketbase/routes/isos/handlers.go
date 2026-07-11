package isos

import (
	"net/http"
	"sort"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerList)
	register(registerLibrary)
	register(registerCreate)
	register(registerGet)
	register(registerUpdate)
	register(registerDelete)
}

const collectionName = "isos"

// isoView is the JSON projection of an `isos` record for admin responses.
func isoView(r *core.Record) map[string]any {
	return map[string]any{
		"id":          r.Id,
		"name":        r.GetString("name"),
		"filename":    r.GetString("filename"),
		"title_id":    r.GetString("title_id"),
		"description": r.GetString("description"),
		"available":   r.GetBool("available"),
		"created":     r.GetString("created"),
		"updated":     r.GetString("updated"),
	}
}

// validFilename mirrors the containers `game_iso` guard: the catalog may only
// reference a bare filename inside the shared ISO library, never an arbitrary
// host path. Rejects separators, "..", and dotfiles.
func validFilename(name string) bool {
	if name == "" {
		return false
	}
	if strings.ContainsAny(name, `/\`) || name == ".." || strings.HasPrefix(name, ".") {
		return false
	}
	return true
}

// GET /api/admin/isos — list every catalog entry (available or not), by name.
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

// GET /api/admin/isos/library — the actual disc images present in the shared ISO
// library dir, each flagged with whether it's already catalogued. Lets the admin
// see what filenames are registerable. 503 when the podman manager isn't wired
// (CONTAINERS_ENABLED=false) — there's no host library to scan.
func registerLibrary() {
	Group.GET("/library", func(e *core.RequestEvent) error {
		if Manager == nil {
			return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "container subsystem not enabled"})
		}
		files, err := Manager.ISOLibrary()
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		// Map filename → catalog record id so the UI can show registered state.
		registered := map[string]string{}
		if records, err := e.App.FindAllRecords(collectionName); err == nil {
			for _, r := range records {
				registered[r.GetString("filename")] = r.Id
			}
		}
		entries := make([]map[string]any, 0, len(files))
		for _, f := range files {
			id, ok := registered[f]
			entries = append(entries, map[string]any{
				"filename":   f,
				"registered": ok,
				"iso_id":     id,
			})
		}
		return e.JSON(http.StatusOK, map[string]any{
			"dir":   Manager.ISODir(),
			"files": entries,
		})
	})
}

// createBody is the POST payload. available defaults to true (nil → visible).
type createBody struct {
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	TitleID     string `json:"title_id"`
	Description string `json:"description"`
	Available   *bool  `json:"available"`
}

// POST /api/admin/isos — register a library file as a pickable ISO.
func registerCreate() {
	Group.POST("", func(e *core.RequestEvent) error {
		var body createBody
		if err := e.BindBody(&body); err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		body.Name = strings.TrimSpace(body.Name)
		body.Filename = strings.TrimSpace(body.Filename)
		if body.Name == "" {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
		}
		if !validFilename(body.Filename) {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "filename must be a bare filename in the ISO library"})
		}
		// If the library is scannable, reject a dead entry up front.
		if Manager != nil && !Manager.ISOExists(body.Filename) {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "no such file in the ISO library: " + body.Filename})
		}

		col, err := e.App.FindCollectionByNameOrId(collectionName)
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		rec := core.NewRecord(col)
		rec.Set("name", body.Name)
		rec.Set("filename", body.Filename)
		rec.Set("title_id", strings.TrimSpace(body.TitleID))
		rec.Set("description", body.Description)
		available := true
		if body.Available != nil {
			available = *body.Available
		}
		rec.Set("available", available)
		if err := e.App.Save(rec); err != nil {
			// Most likely the unique-filename index — this file is already catalogued.
			return e.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		}
		return e.JSON(http.StatusCreated, isoView(rec))
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
type updateBody struct {
	Name        *string `json:"name"`
	Filename    *string `json:"filename"`
	TitleID     *string `json:"title_id"`
	Description *string `json:"description"`
	Available   *bool   `json:"available"`
}

// PATCH /api/admin/isos/{id} — partial update. Only provided fields change.
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
		if body.Filename != nil {
			filename := strings.TrimSpace(*body.Filename)
			if !validFilename(filename) {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "filename must be a bare filename in the ISO library"})
			}
			if Manager != nil && !Manager.ISOExists(filename) {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "no such file in the ISO library: " + filename})
			}
			rec.Set("filename", filename)
		}
		if body.TitleID != nil {
			rec.Set("title_id", strings.TrimSpace(*body.TitleID))
		}
		if body.Description != nil {
			rec.Set("description", *body.Description)
		}
		if body.Available != nil {
			rec.Set("available", *body.Available)
		}
		if err := e.App.Save(rec); err != nil {
			return e.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		}
		return e.JSON(http.StatusOK, isoView(rec))
	})
}

// DELETE /api/admin/isos/{id} — remove a catalog entry (the library file on disk
// is untouched).
func registerDelete() {
	Group.DELETE("/{id}", func(e *core.RequestEvent) error {
		rec, err := e.App.FindRecordById(collectionName, e.Request.PathValue("id"))
		if err != nil {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "iso not found"})
		}
		if err := e.App.Delete(rec); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return e.NoContent(http.StatusNoContent)
	})
}
