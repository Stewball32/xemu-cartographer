package isoingest

import (
	"log"
	"os"
	"path/filepath"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/lansync"
)

// Canonical map catalog sync (organizer redesign, Maps page).
//
// iso_maps rows are per-DISC facts (this disc ships this cache); the `maps`
// collection is per-BUILD identity — one row per (game, filename, content_hash)
// that the organizer names, groups into variants, and decorates. This file
// keeps the second derived from the first: MINT missing catalog rows, never
// touch existing ones (display_name / variant_of / graphic / power_items are
// organizer-owned), and never delete (a build whose last disc is removed keeps
// its curated identity for when the disc returns).

const catalogCollection = "maps"

// GameForTitleID maps an isos.title_id to the catalog's game key. Unknown
// title ids (mods with rewritten certs, other games) return "" — their maps
// stay out of the catalog rather than polluting it with a wrong game bucket.
// Shared with the maps-catalog view route.
func GameForTitleID(titleID string) string {
	switch titleID {
	case "4D530004": // Halo: Combat Evolved (NTSC)
		return "ce"
	case "4D530064": // Halo 2
		return "h2"
	}
	return ""
}

// SyncCatalog mints catalog rows for every hashed map of one disc that isn't
// cataloged yet. Requires the disc's title_id (extracted before SyncMaps runs);
// a disc with an unknown game is skipped quietly. Safe to call repeatedly.
func SyncCatalog(app core.App, isoID string) {
	col, err := app.FindCollectionByNameOrId(catalogCollection)
	if err != nil {
		return // collection not migrated in — feature dormant
	}
	iso, err := app.FindRecordById(collectionName, isoID)
	if err != nil {
		return
	}
	game := GameForTitleID(iso.GetString("title_id"))
	if game == "" {
		return
	}
	rows, err := app.FindRecordsByFilter(mapsCollection, "iso = {:id}", "", 0, 0, dbx.Params{"id": isoID})
	if err != nil {
		return
	}
	for _, r := range rows {
		// Only multiplayer caches are catalog material — campaign/ui aren't
		// pick-a-map surfaces and would triple the shelf with noise.
		if r.GetString("map_type") != "multiplayer" {
			continue
		}
		hash := r.GetString("content_hash")
		if hash == "" {
			continue
		}
		filename := r.GetString("filename")
		existing, _ := app.FindFirstRecordByFilter(catalogCollection,
			"game = {:g} && filename = {:f} && content_hash = {:h}",
			dbx.Params{"g": game, "f": filename, "h": hash})
		if existing != nil {
			continue
		}
		rec := core.NewRecord(col)
		rec.Set("game", game)
		rec.Set("filename", filename)
		rec.Set("content_hash", hash)
		if err := app.Save(rec); err != nil {
			log.Printf("isoingest: catalog row %s %s@%.12s: %v", game, filename, hash, err)
		}
	}
}

// BackfillCatalog hashes any pre-catalog iso_maps rows (rows minted before the
// content_hash field existed) from their extracted trees, then syncs the
// catalog for every extracted disc. Boot-time, idempotent, best-effort — run it
// FireAndForget from an OnServe hook.
func BackfillCatalog(app core.App) {
	rows, err := app.FindRecordsByFilter(mapsCollection, "content_hash = ''", "", 0, 0, nil)
	if err == nil && len(rows) > 0 {
		treeByISO := map[string]string{}
		hashed := 0
		for _, r := range rows {
			isoID := r.GetString("iso")
			tree, ok := treeByISO[isoID]
			if !ok {
				tree = ""
				if iso, err := app.FindRecordById(collectionName, isoID); err == nil && iso.GetBool("extracted_ready") {
					tree = iso.GetString("extracted_path")
				}
				treeByISO[isoID] = tree
			}
			if tree == "" {
				continue
			}
			path := filepath.Join(tree, "maps", r.GetString("filename"))
			if _, err := os.Stat(path); err != nil {
				continue
			}
			h, err := lansync.HashFile(path)
			if err != nil {
				continue
			}
			r.Set("content_hash", h)
			if err := app.Save(r); err == nil {
				hashed++
			}
		}
		if hashed > 0 {
			log.Printf("isoingest: catalog backfill hashed %d map row(s)", hashed)
		}
	}

	isos, err := app.FindAllRecords(collectionName)
	if err != nil {
		return
	}
	for _, iso := range isos {
		if iso.GetBool("extracted_ready") {
			SyncCatalog(app, iso.Id)
		}
	}
}
