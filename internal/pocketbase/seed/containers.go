//go:build dev

package seed

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// snapshotPath is the location of the dev-only containers seed file. It mirrors
// the legacy state.json shape so users can rename and reuse one if they have it.
const snapshotPath = "./containers/dev-seed.json"

type seedContainer struct {
	Name    string         `json:"name"`
	Index   int            `json:"index"`
	Ports   map[string]any `json:"ports"`
	Created time.Time      `json:"created"`
}

type seedContainerFile struct {
	Containers map[string]*seedContainer `json:"containers"`
}

// ensureContainersFromSnapshot reads the dev snapshot file (if any) and upserts
// each container into the "containers" collection. Idempotent across reboots.
func ensureContainersFromSnapshot(app *pocketbase.PocketBase) error {
	data, err := os.ReadFile(snapshotPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read containers snapshot: %w", err)
	}

	var file seedContainerFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parse containers snapshot: %w", err)
	}

	collection, err := app.FindCollectionByNameOrId("containers")
	if err != nil {
		return fmt.Errorf("find containers collection: %w", err)
	}

	for _, c := range file.Containers {
		existing, _ := app.FindFirstRecordByFilter("containers", "name = {:name}", map[string]any{"name": c.Name})
		if existing != nil {
			log.Printf("  container %s: exists, skipping", c.Name)
			continue
		}

		portsJSON, err := json.Marshal(c.Ports)
		if err != nil {
			return fmt.Errorf("marshal ports for %s: %w", c.Name, err)
		}

		record := core.NewRecord(collection)
		record.Set("name", c.Name)
		record.Set("index", c.Index)
		record.Set("ports", string(portsJSON))
		record.Set("created", c.Created)

		if err := app.Save(record); err != nil {
			return fmt.Errorf("seed container %s: %w", c.Name, err)
		}
		log.Printf("  container %s: created", c.Name)
	}
	return nil
}

// RegisterContainerSnapshotHooks wires PB record hooks so that the snapshot
// file is rewritten after every create/update/delete on the containers
// collection. Dev-only (build tag) — prod builds use the stub no-op.
func RegisterContainerSnapshotHooks(app *pocketbase.PocketBase) {
	rewrite := func(e *core.RecordEvent) error {
		if err := writeContainersSnapshot(app); err != nil {
			log.Printf("seed: write containers snapshot: %v", err)
		}
		return e.Next()
	}

	app.OnRecordAfterCreateSuccess("containers").BindFunc(rewrite)
	app.OnRecordAfterUpdateSuccess("containers").BindFunc(rewrite)
	app.OnRecordAfterDeleteSuccess("containers").BindFunc(rewrite)
}

func writeContainersSnapshot(app *pocketbase.PocketBase) error {
	records, err := app.FindAllRecords("containers")
	if err != nil {
		return fmt.Errorf("load containers: %w", err)
	}

	out := seedContainerFile{Containers: make(map[string]*seedContainer, len(records))}
	for _, r := range records {
		var ports map[string]any
		if raw := r.GetString("ports"); raw != "" {
			if err := json.Unmarshal([]byte(raw), &ports); err != nil {
				return fmt.Errorf("unmarshal ports for %s: %w", r.GetString("name"), err)
			}
		}
		name := r.GetString("name")
		out.Containers[name] = &seedContainer{
			Name:    name,
			Index:   r.GetInt("index"),
			Ports:   ports,
			Created: r.GetDateTime("created").Time(),
		}
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
		return err
	}
	tmp := snapshotPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, snapshotPath)
}
