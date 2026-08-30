// Package isoingest is the ISO ingest + drift-detection orchestration over the
// managed library (design locked with Stewart).
//
// Flow (per file dropped in cfg.InboxDir): hash it → dedupe by hash → create the
// catalog row (get the ID) → atomically rename it into the library as
// <id>.iso → store hash/size/mtime → freeze it read-only → kick extraction. The
// managed file is ID-anchored, decoupled from the freely editable display name.
//
// Drift: the managed bytes are re-verified on boot and before serving/booting a
// disc — a cheap size+mtime pre-check, full re-hash only if those changed; on
// mismatch the row is forced unavailable + flagged so bad bytes never boot/sync.
//
// PB orchestration lives here; the pure fs/exec mechanics are in
// internal/lansync (ManagedISOPath / HashFile / StatSig / AtomicMove /
// FreezeFile / UnfreezeFile / ExtractISO).
package isoingest

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/routine"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/Stewball32/xemu-cartographer/internal/lansync"
)

const collectionName = "isos"

// InboxFile is a pending disc image staged in the inbox.
type InboxFile struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// IngestedItem / SkippedItem / IngestResult report a scan+ingest pass.
type IngestedItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Filename  string `json:"filename"`
	Hash      string `json:"hash"`
	Immutable bool   `json:"immutable"`
}
type SkippedItem struct {
	Filename string `json:"filename"`
	Reason   string `json:"reason"`
	DupOf    string `json:"dup_of,omitempty"`
}
type IngestResult struct {
	Ingested []IngestedItem `json:"ingested"`
	Skipped  []SkippedItem  `json:"skipped"`
	Errors   []string       `json:"errors"`
}

// InboxPending lists the regular files currently staged in the inbox.
func InboxPending(cfg lansync.Config) ([]InboxFile, error) {
	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(cfg.InboxDir)
	if err != nil {
		return nil, err
	}
	out := []InboxFile{}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, InboxFile{Filename: e.Name(), Size: info.Size()})
	}
	return out, nil
}

// IngestInbox scans the inbox and ingests every pending file. Hashing + the
// atomic move + freeze happen synchronously (so the caller gets per-file results
// including dedupe); the slow extraction is kicked async per row.
func IngestInbox(app core.App, cfg lansync.Config) (IngestResult, error) {
	res := IngestResult{Ingested: []IngestedItem{}, Skipped: []SkippedItem{}, Errors: []string{}}
	pending, err := InboxPending(cfg)
	if err != nil {
		return res, err
	}
	for _, f := range pending {
		item, skip, err := ingestOne(app, cfg, f.Filename)
		switch {
		case err != nil:
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", f.Filename, err))
		case skip != nil:
			res.Skipped = append(res.Skipped, *skip)
		case item != nil:
			res.Ingested = append(res.Ingested, *item)
		}
	}
	return res, nil
}

// ingestOne ingests a single inbox file. Returns exactly one of (item, skip, err).
func ingestOne(app core.App, cfg lansync.Config, filename string) (*IngestedItem, *SkippedItem, error) {
	if strings.ContainsAny(filename, `/\`) || strings.HasPrefix(filename, ".") {
		return nil, nil, fmt.Errorf("unsafe inbox filename")
	}
	inboxPath := filepath.Join(cfg.InboxDir, filename)
	if _, err := os.Stat(inboxPath); err != nil {
		return nil, nil, err
	}

	// Hash first, so dedupe rejects cleanly before we create a row or move bytes.
	hash, err := lansync.HashFile(inboxPath)
	if err != nil {
		return nil, nil, fmt.Errorf("hash: %w", err)
	}
	if dup, _ := app.FindFirstRecordByFilter(collectionName,
		"content_hash = {:h}", dbx.Params{"h": hash}); dup != nil {
		return nil, &SkippedItem{
			Filename: filename,
			Reason:   "duplicate content",
			DupOf:    dup.GetString("name"),
		}, nil
	}

	col, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		return nil, nil, err
	}
	rec := core.NewRecord(col)
	rec.Set("name", displayName(filename))
	rec.Set("filename", filename) // original inbox name — provenance only
	rec.Set("content_hash", hash)
	// New discs land shelved with baseline offsets — the organizer sets role +
	// bindings in the Discs detail, then saves.
	rec.Set("role", "shelved")
	rec.Set("allow_on_xbox", false)
	rec.Set("drift_detected", false)
	if err := app.Save(rec); err != nil {
		return nil, nil, fmt.Errorf("create row: %w", err)
	}

	// Atomically move the disc into the managed library as <id>.iso. Same
	// filesystem (inbox ↔ library) → the file appears whole or not at all.
	managed := cfg.ManagedISOPath(rec.Id)
	if err := lansync.AtomicMove(inboxPath, managed); err != nil {
		_ = app.Delete(rec) // roll back the orphaned row
		return nil, nil, fmt.Errorf("move into library: %w", err)
	}

	// Record the managed file's post-move size+mtime (the drift anchor), then
	// freeze it read-only.
	size, mtime, statErr := lansync.StatSig(managed)
	if statErr == nil {
		rec.Set("file_size", size)
		rec.Set("file_mtime", mtime)
	}
	immutable, freezeErr := lansync.FreezeFile(managed)
	if freezeErr != nil {
		log.Printf("isoingest: freeze %s: %v", managed, freezeErr)
	} else if !immutable {
		log.Printf("isoingest: %s chmod 0444 (immutable bit not set — needs root)", filepath.Base(managed))
	}
	if err := app.Save(rec); err != nil {
		return nil, nil, fmt.Errorf("save managed meta: %w", err)
	}

	// Extraction reads the whole disc again (extract-xiso); slow + multi-GiB, so
	// kick it async. extracted_ready flips true when the tree lands.
	id := rec.Id
	routine.FireAndForget(func() {
		if err := Extract(app, cfg, id); err != nil {
			log.Printf("isoingest: extract %s: %v", id, err)
		}
	})

	return &IngestedItem{
		ID: rec.Id, Name: rec.GetString("name"), Filename: filename, Hash: hash, Immutable: immutable,
	}, nil, nil
}

// Extract builds (or rebuilds) the extracted tree for a managed disc and writes
// the cache fields back. Idempotent — a ready + present tree is a no-op.
func Extract(app core.App, cfg lansync.Config, id string) error {
	rec, err := app.FindRecordById(collectionName, id)
	if err != nil {
		return err
	}
	if rec.GetBool("extracted_ready") {
		if p := rec.GetString("extracted_path"); p != "" {
			if _, statErr := os.Stat(p); statErr == nil {
				return nil
			}
		}
	}
	treeDir, footprint, err := lansync.ExtractISO(cfg, id)
	if err != nil {
		return err
	}
	rec.Set("extracted_path", treeDir)
	rec.Set("extracted_ready", true)
	rec.Set("extracted_at", types.NowDateTime())
	rec.Set("footprint_bytes", footprint)
	// Auto-extract the title id from the disc's boot XBE (the tree is already
	// on disk — this is a header read, not another disc pass). Server-owned:
	// always refreshed from the disc, never hand-entered. Best-effort — a tree
	// with no parseable default.xbe just leaves the field as-is.
	if titleID, terr := TitleIDFromTree(treeDir); terr == nil {
		rec.Set("title_id", titleID)
	} else {
		log.Printf("isoingest: title id for %s: %v", id, terr)
	}
	if err := app.Save(rec); err != nil {
		return err
	}
	// Map list + async best-effort top-down thumbnails (the map-graphics
	// feature). Non-fatal to extraction — a missing iso_maps collection or
	// Python toolchain just means no maps/thumbs, never a failed ingest.
	SyncMaps(app, cfg, id, treeDir)
	return nil
}

// VerifyManaged checks a row's managed bytes against its recorded anchor. Cheap
// path: matching size+mtime ⇒ OK without re-hashing. Otherwise re-hash and
// compare to content_hash. A row with no content_hash yet is treated as OK
// (nothing to verify — mid-ingest / legacy).
func VerifyManaged(cfg lansync.Config, rec *core.Record) (ok bool, reason string) {
	want := rec.GetString("content_hash")
	if want == "" {
		return true, ""
	}
	path := cfg.ManagedISOPath(rec.Id)
	size, mtime, err := lansync.StatSig(path)
	if err != nil {
		return false, "managed file missing"
	}
	if size == int64(rec.GetInt("file_size")) && mtime == int64(rec.GetInt("file_mtime")) {
		return true, "" // size+mtime unchanged — trust the anchor
	}
	got, err := lansync.HashFile(path)
	if err != nil {
		return false, "re-hash failed"
	}
	if got != want {
		return false, "content hash mismatch"
	}
	return true, "" // benign mtime touch; bytes still match
}

// VerifyAndFlag verifies a row and, on drift, forces it unavailable + flagged
// (persisted) so bad bytes never boot or sync. Returns whether the bytes are
// good. The shared gate for the boot scan, the provision path, and lansync.
func VerifyAndFlag(app core.App, cfg lansync.Config, rec *core.Record) bool {
	ok, reason := VerifyManaged(cfg, rec)
	if ok {
		return true
	}
	log.Printf("isoingest: DRIFT %s (%s): %s — shelving", rec.Id, rec.GetString("name"), reason)
	rec.Set("role", "shelved")
	rec.Set("drift_detected", true)
	if err := app.Save(rec); err != nil {
		log.Printf("isoingest: flag drift %s: %v", rec.Id, err)
	}
	return false
}

// ScanDrift verifies every catalog row's managed bytes and flags the bad ones.
// Called on boot. Returns the ids flagged this pass.
func ScanDrift(app core.App, cfg lansync.Config) []string {
	rows, err := app.FindAllRecords(collectionName)
	if err != nil {
		log.Printf("isoingest: drift scan: %v", err)
		return nil
	}
	var flagged []string
	for _, rec := range rows {
		if !VerifyAndFlag(app, cfg, rec) {
			flagged = append(flagged, rec.Id)
		}
	}
	return flagged
}

// DeleteManaged tears down a catalog row: unfreeze + remove the managed disc,
// drop the extracted tree, then delete the record. The disk file goes with the
// row now (it's ID-owned), unlike the old bare-filename registry.
func DeleteManaged(app core.App, cfg lansync.Config, rec *core.Record) error {
	managed := cfg.ManagedISOPath(rec.Id)
	lansync.UnfreezeFile(managed)
	if err := os.Remove(managed); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove managed file: %w", err)
	}
	if tree := rec.GetString("extracted_path"); tree != "" {
		_ = os.RemoveAll(tree)
	} else {
		_ = os.RemoveAll(filepath.Join(cfg.ExtractDir, "isos", rec.Id))
	}
	return app.Delete(rec)
}

// displayName derives a human default from an inbox filename (drop the
// extension). The admin can rename freely afterward.
func displayName(filename string) string {
	base := filepath.Base(filename)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return filename
	}
	return base
}
