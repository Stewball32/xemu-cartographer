package isoingest

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/routine"

	"github.com/Stewball32/xemu-cartographer/internal/lansync"
)

// Map-list + map-thumbnail extraction for an ingested build.
//
// The map LIST is a pure Go header parse of the extracted tree's maps/*.map
// (name + type) — always runs, no external deps. Map THUMBNAILS are the
// high-value part: CE ships no per-map preview art, so we render a top-down
// image of each multiplayer map's structure-BSP geometry (the same imagery the
// live visualizer produces). That render shells the vendored Python extractor
// (numpy/PIL + the halo-offset-mapper cache parser) — best-effort and async, so
// a build with no Python toolchain still gets the map list.
//
// H2 differs (its cache format isn't handled by the parser, and it DOES ship
// real per-map preview bitmaps) — a documented follow-on; this path is CE-only.

const mapsCollection = "iso_maps"

// Halo 1 (Xbox) .map header — enough for an inventory row (first 0x800 bytes).
const (
	mapHeadMagic  uint32 = 0x68656164 // "head" (LE u32 @ 0x00)
	mapFootMagic  uint32 = 0x666F6F74 // "foot" (LE u32 @ 0x7FC)
	mapHeaderSize        = 0x800
	mapOffName           = 0x20 // 32-byte NUL-terminated internal name
	mapOffType           = 0x60 // u32: 0 campaign, 1 multiplayer, 2 ui
)

var mapTypeNames = map[uint32]string{0: "campaign", 1: "multiplayer", 2: "ui"}

// MapInfo is one parsed cache from the tree.
type MapInfo struct {
	Filename string // e.g. "bloodgulch.map"
	Name     string // internal_name from the header
	Type     string // campaign / multiplayer / ui
}

// parseMapHeader reads a .map's header row. Errors (bad magic/foot, short file)
// mean "not a parseable cache" — the caller skips it, never fails the ingest.
func parseMapHeader(path string) (MapInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return MapInfo{}, err
	}
	defer f.Close()
	buf := make([]byte, mapHeaderSize)
	if _, err := f.ReadAt(buf, 0); err != nil {
		return MapInfo{}, fmt.Errorf("short map file: %w", err)
	}
	if binary.LittleEndian.Uint32(buf[0:]) != mapHeadMagic {
		return MapInfo{}, fmt.Errorf("bad head magic")
	}
	if binary.LittleEndian.Uint32(buf[0x7FC:]) != mapFootMagic {
		return MapInfo{}, fmt.Errorf("bad foot magic")
	}
	name := string(buf[mapOffName : mapOffName+0x20])
	if i := strings.IndexByte(name, 0); i >= 0 {
		name = name[:i]
	}
	mt := binary.LittleEndian.Uint32(buf[mapOffType:])
	typ := mapTypeNames[mt]
	if typ == "" {
		typ = fmt.Sprintf("type%d", mt)
	}
	return MapInfo{Filename: filepath.Base(path), Name: strings.TrimSpace(name), Type: typ}, nil
}

// ParseMapList enumerates maps/*.map under an extracted tree and parses each
// header. Skips unparseable files; returns the rest sorted by filename.
func ParseMapList(treeDir string) []MapInfo {
	dir := filepath.Join(treeDir, "maps")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []MapInfo
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".map") {
			continue
		}
		info, err := parseMapHeader(filepath.Join(dir, e.Name()))
		if err != nil {
			log.Printf("isoingest: map header %s: %v (skipped)", e.Name(), err)
			continue
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Filename < out[j].Filename })
	return out
}

// SyncMaps rebuilds the iso_maps rows for a build from its extracted tree, then
// kicks async best-effort thumbnail renders for each MULTIPLAYER map. Idempotent
// on re-ingest: existing rows for the iso are dropped first. Called from the
// extract pass (tree already on disk). No-op if the iso_maps collection is
// absent (older schema) or the tree has no maps/.
func SyncMaps(app core.App, cfg lansync.Config, isoID, treeDir string) {
	col, err := app.FindCollectionByNameOrId(mapsCollection)
	if err != nil {
		return // collection not migrated in — feature dormant
	}
	list := ParseMapList(treeDir)
	if len(list) == 0 {
		return
	}

	// Drop prior rows for this iso (their thumb files go with them).
	if old, err := app.FindRecordsByFilter(mapsCollection, "iso = {:id}", "", 0, 0, dbx.Params{"id": isoID}); err == nil {
		for _, r := range old {
			_ = app.Delete(r)
		}
	}

	tc := loadThumbConfig()
	var thumbable []string // map filenames to render
	for _, mi := range list {
		rec := core.NewRecord(col)
		rec.Set("iso", isoID)
		rec.Set("filename", mi.Filename)
		rec.Set("name", mi.Name)
		rec.Set("map_type", mi.Type)
		// Only multiplayer maps get a top-down render (campaign/ui aren't
		// pick-a-map surfaces); mark them pending so the UI shows a spinner.
		if mi.Type == "multiplayer" && tc.Enabled {
			rec.Set("thumb_status", "pending")
			thumbable = append(thumbable, mi.Filename)
		} else {
			rec.Set("thumb_status", "skipped")
		}
		if err := app.Save(rec); err != nil {
			log.Printf("isoingest: save map row %s/%s: %v", isoID, mi.Filename, err)
		}
	}
	if len(thumbable) == 0 {
		return
	}
	routine.FireAndForget(func() {
		for _, fn := range thumbable {
			renderAndAttachThumb(app, tc, isoID, treeDir, fn)
		}
	})
}

// renderAndAttachThumb renders one map's top-down PNG and attaches it to the
// map's row. Best-effort: any failure flips the row to thumb_status="failed"
// (the map still lists, just without an image).
func renderAndAttachThumb(app core.App, tc thumbConfig, isoID, treeDir, mapFilename string) {
	rec, err := findMapRow(app, isoID, mapFilename)
	if err != nil {
		return
	}
	pngPath, cleanup, err := renderThumb(tc, treeDir, mapFilename)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		log.Printf("isoingest: thumb %s/%s: %v", isoID, mapFilename, err)
		rec.Set("thumb_status", "failed")
		_ = app.Save(rec)
		return
	}
	file, err := filesystem.NewFileFromPath(pngPath)
	if err != nil {
		log.Printf("isoingest: thumb file %s/%s: %v", isoID, mapFilename, err)
		rec.Set("thumb_status", "failed")
		_ = app.Save(rec)
		return
	}
	rec.Set("thumb", file)
	rec.Set("thumb_status", "ready")
	if err := app.Save(rec); err != nil {
		log.Printf("isoingest: save thumb %s/%s: %v", isoID, mapFilename, err)
	}
}

func findMapRow(app core.App, isoID, mapFilename string) (*core.Record, error) {
	recs, err := app.FindRecordsByFilter(mapsCollection,
		"iso = {:id} && filename = {:f}", "", 1, 0,
		dbx.Params{"id": isoID, "f": mapFilename})
	if err != nil || len(recs) == 0 {
		return nil, fmt.Errorf("map row not found")
	}
	return recs[0], nil
}

// renderThumb shells the vendored BSP extractor to produce a top-down PNG for
// one map, into a temp dir. Returns the PNG path + a cleanup func. The extractor
// writes <out>/haloce/<mapname>_top.png (mapname = filename sans .map).
func renderThumb(tc thumbConfig, treeDir, mapFilename string) (string, func(), error) {
	tmp, err := os.MkdirTemp("", "mapthumb-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	cmd := exec.Command(tc.Python, tc.Script,
		"--game", "haloce",
		"--mapper-dir", tc.MapperDir,
		"--maps-dir", filepath.Join(treeDir, "maps"),
		"--map", mapFilename,
		"--out", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", cleanup, fmt.Errorf("render: %w: %s", err, tail(string(out), 200))
	}
	// The extractor names the PNG by the map's INTERNAL scenario name, not the
	// filename (e.g. hangman.map → hangem_au_top.png), so glob the single
	// *_top.png the fresh temp dir contains rather than assuming the basename.
	matches, _ := filepath.Glob(filepath.Join(tmp, "haloce", "*_top.png"))
	if len(matches) == 0 {
		return "", cleanup, fmt.Errorf("no *_top.png produced under %s", tmp)
	}
	return matches[0], cleanup, nil
}

// thumbConfig is the (env-overridable) toolchain for the top-down render.
type thumbConfig struct {
	Enabled   bool
	Python    string
	Script    string
	MapperDir string
}

func loadThumbConfig() thumbConfig {
	return thumbConfig{
		Enabled:   envDefault("MAPS_THUMBS_ENABLED", "true") != "false",
		Python:    envDefault("MAPS_THUMBS_PYTHON", "python3"),
		Script:    envDefault("MAPS_THUMBS_SCRIPT", "./tools/game-maps/extract_bsp.py"),
		MapperDir: envDefault("MAPS_THUMBS_MAPPER_DIR", "./tools/game-maps"),
	}
}

func envDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}

// mapView projects an iso_maps record for API responses.
func mapView(r *core.Record) map[string]any {
	thumbURL := ""
	if fn := r.GetString("thumb"); fn != "" {
		thumbURL = fmt.Sprintf("/api/files/%s/%s/%s", mapsCollection, r.Id, fn)
	}
	return map[string]any{
		"id":           r.Id,
		"filename":     r.GetString("filename"),
		"name":         r.GetString("name"),
		"map_type":     r.GetString("map_type"),
		"thumb_url":    thumbURL,
		"thumb_status": r.GetString("thumb_status"),
	}
}

// MapsForISO returns a build's map rows (multiplayer first, then by name) as API
// views. Shared by the organizer + play endpoints.
func MapsForISO(app core.App, isoID string) []map[string]any {
	recs, err := app.FindRecordsByFilter(mapsCollection, "iso = {:id}", "", 0, 0, dbx.Params{"id": isoID})
	if err != nil {
		return []map[string]any{}
	}
	sort.SliceStable(recs, func(i, j int) bool {
		ti, tj := recs[i].GetString("map_type"), recs[j].GetString("map_type")
		if (ti == "multiplayer") != (tj == "multiplayer") {
			return ti == "multiplayer" // multiplayer first
		}
		return recs[i].GetString("name") < recs[j].GetString("name")
	})
	out := make([]map[string]any, 0, len(recs))
	for _, r := range recs {
		out = append(out, mapView(r))
	}
	return out
}
