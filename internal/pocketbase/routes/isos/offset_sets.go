// Offset-set library routes (organizer redesign, Offsets page). Sets come from
// two worlds merged into one listing: EMBEDDED baselines compiled into the
// binary (internal/scraper/offsets/sets/), and IMPORTED sets — offsetmap JSON
// exports from the hunting rig, uploaded here and stored byte-identical in the
// offset_sets collection. Discs bind either kind by id (isos.offset_set); the
// scraper resolves imported ids through offsets.SetDynamicSource (wired in
// main.go). Baselines can't be imported over or deleted.
package isos

import (
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/offsets"
)

func init() {
	register(registerOffsetSetList)
	register(registerOffsetSetImport)
	register(registerOffsetSetDownload)
	register(registerOffsetSetDelete)
}

const offsetSetsCollection = "offset_sets"

// offsetSetView is one listing entry — the shape the Offsets page renders.
// Embedded baselines carry baseline:true and no imported date; imported sets
// carry their upload provenance + version counter.
type offsetSetView struct {
	Game        string `json:"game"`
	ID          string `json:"id"`
	Description string `json:"description"`
	Count       int    `json:"count"`
	Baseline    bool   `json:"baseline"`
	BoundDiscs  int    `json:"bound_discs"`
	Imported    string `json:"imported,omitempty"`    // "" for embedded
	SourceName  string `json:"source_name,omitempty"` // original upload filename
	Version     int    `json:"version,omitempty"`
}

// boundCounts maps offset-set id → number of catalog discs binding it.
func boundCounts(app core.App) map[string]int {
	out := map[string]int{}
	records, err := app.FindAllRecords(collectionName)
	if err != nil {
		return out
	}
	for _, r := range records {
		if id := r.GetString("offset_set"); id != "" {
			out[id]++
		}
	}
	return out
}

// GET /api/admin/isos/offset-sets — embedded + imported sets, sorted by game
// then id, each with its dependent-disc count.
func registerOffsetSetList() {
	Group.GET("/offset-sets", func(e *core.RequestEvent) error {
		bound := boundCounts(e.App)
		out := []offsetSetView{}
		for _, s := range offsets.All() {
			_, src, _ := offsets.Raw(s.ID)
			out = append(out, offsetSetView{
				Game: s.Game, ID: s.ID, Description: s.Description,
				Count: s.Count, Baseline: s.Baseline,
				BoundDiscs: bound[s.ID], SourceName: src,
			})
		}
		records, err := e.App.FindAllRecords(offsetSetsCollection)
		if err == nil {
			for _, r := range records {
				out = append(out, offsetSetView{
					Game:        r.GetString("game"),
					ID:          r.GetString("set_id"),
					Description: r.GetString("description"),
					Count:       r.GetInt("count"),
					BoundDiscs:  bound[r.GetString("set_id")],
					Imported:    r.GetString("updated"),
					SourceName:  r.GetString("source_name"),
					Version:     r.GetInt("version"),
				})
			}
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Game != out[j].Game {
				return out[i].Game < out[j].Game
			}
			return out[i].ID < out[j].ID
		})
		return e.JSON(http.StatusOK, out)
	})
}

// POST /api/admin/isos/offset-sets — import an offsetmap JSON export.
// Multipart form: `file` (the export) + optional `save_as` (rename the id
// before it lands — discs will reference THIS id). Re-importing an existing
// imported id replaces its file and bumps `version`; embedded ids are refused
// (the compiled-in set would always win at bind time).
func registerOffsetSetImport() {
	Group.POST("/offset-sets", func(e *core.RequestEvent) error {
		file, header, err := e.Request.FormFile("file")
		if err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "an offsetmap JSON file is required"})
		}
		defer file.Close()
		raw, err := io.ReadAll(io.LimitReader(file, 5<<20+1))
		if err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "read upload: " + err.Error()})
		}
		if len(raw) > 5<<20 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "offsetmap file too large (5 MiB cap)"})
		}
		parsed, err := offsets.ParseSet(raw)
		if err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "not a valid offsetmap export: " + err.Error()})
		}

		id := strings.TrimSpace(e.Request.FormValue("save_as"))
		if id == "" {
			id = parsed.ID
		}
		if len(id) > 64 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "set id too long (64 chars max)"})
		}
		if _, _, embedded := offsets.Raw(id); embedded {
			return e.JSON(http.StatusConflict, map[string]string{"error": "\"" + id + "\" is a built-in baseline — import under a different id"})
		}

		f, err := filesystem.NewFileFromBytes(raw, header.Filename)
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		rec, _ := e.App.FindFirstRecordByData(offsetSetsCollection, "set_id", id)
		if rec == nil {
			col, err := e.App.FindCollectionByNameOrId(offsetSetsCollection)
			if err != nil {
				return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			rec = core.NewRecord(col)
			rec.Set("set_id", id)
			rec.Set("version", 1)
		} else {
			rec.Set("version", rec.GetInt("version")+1)
		}
		rec.Set("game", parsed.Game)
		rec.Set("description", parsed.Description)
		rec.Set("count", parsed.Len())
		rec.Set("source_name", header.Filename)
		rec.Set("file", f)
		if err := e.App.Save(rec); err != nil {
			return e.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		}
		return e.JSON(http.StatusCreated, offsetSetView{
			Game: parsed.Game, ID: id, Description: parsed.Description,
			Count: parsed.Len(), BoundDiscs: boundCounts(e.App)[id],
			Imported: rec.GetString("updated"), SourceName: header.Filename,
			Version: rec.GetInt("version"),
		})
	})
}

// offsetSetRaw returns an id's offsetmap bytes — embedded registry first, then
// the stored upload. Byte-identical to what the rig exported.
func offsetSetRaw(app core.App, id string) (raw []byte, name string, err error) {
	if raw, src, ok := offsets.Raw(id); ok {
		return raw, src, nil
	}
	rec, err := app.FindFirstRecordByData(offsetSetsCollection, "set_id", id)
	if err != nil {
		return nil, "", err
	}
	fsys, err := app.NewFilesystem()
	if err != nil {
		return nil, "", err
	}
	defer fsys.Close()
	r, err := fsys.GetReader(rec.BaseFilesPath() + "/" + rec.GetString("file"))
	if err != nil {
		return nil, "", err
	}
	defer r.Close()
	raw, err = io.ReadAll(r)
	name = rec.GetString("source_name")
	if name == "" {
		name = id + ".offsetmap.json"
	}
	return raw, name, err
}

// GET /api/admin/isos/offset-sets/{id}/raw — the set's offsetmap JSON, for
// Download and for the detail table (the client parses the same bytes it can
// save, so what you read is exactly what you'd re-import).
func registerOffsetSetDownload() {
	Group.GET("/offset-sets/{id}/raw", func(e *core.RequestEvent) error {
		id := e.Request.PathValue("id")
		raw, name, err := offsetSetRaw(e.App, id)
		if err != nil {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "unknown offset set: " + id})
		}
		e.Response.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
		e.Response.Header().Set("Content-Type", "application/json")
		_, err = e.Response.Write(raw)
		return err
	})
}

// deleteOffsetSetBody carries the migration target: every disc bound to the
// deleted set re-binds to migrate_to ("" = unbound — those discs ride their
// game's baseline and modded stats go dark, the mock's explicit last resort).
type deleteOffsetSetBody struct {
	MigrateTo string `json:"migrate_to"`
}

// DELETE /api/admin/isos/offset-sets/{id} — delete an imported set, migrating
// its dependents. Baselines are refused; a non-empty migration target must be
// a known set (embedded or imported) and not the set being deleted.
func registerOffsetSetDelete() {
	Group.DELETE("/offset-sets/{id}", func(e *core.RequestEvent) error {
		id := e.Request.PathValue("id")
		if _, _, embedded := offsets.Raw(id); embedded {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "built-in baselines can't be deleted"})
		}
		rec, err := e.App.FindFirstRecordByData(offsetSetsCollection, "set_id", id)
		if err != nil {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "unknown offset set: " + id})
		}
		var body deleteOffsetSetBody
		if err := e.BindBody(&body); err != nil && err != io.EOF {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		target := strings.TrimSpace(body.MigrateTo)
		if target == id {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "can't migrate discs to the set being deleted"})
		}
		if target != "" && !offsetSetExists(e.App, target) {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "unknown migration target: " + target})
		}
		dependents, err := e.App.FindRecordsByFilter(collectionName,
			"offset_set = {:id}", "", 0, 0, dbx.Params{"id": id})
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		for _, d := range dependents {
			d.Set("offset_set", target)
			if err := e.App.Save(d); err != nil {
				return e.JSON(http.StatusInternalServerError, map[string]string{"error": "re-bind " + d.GetString("name") + ": " + err.Error()})
			}
		}
		if err := e.App.Delete(rec); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return e.JSON(http.StatusOK, map[string]any{"migrated": len(dependents), "migrate_to": target})
	})
}
