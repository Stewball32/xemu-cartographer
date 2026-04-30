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

		record := core.NewRecord(collection)
		record.Set("name", c.Name)
		record.Set("index", c.Index)
		record.Set("xemu_http", portFromMap(c.Ports, "xemu_http"))
		record.Set("xemu_https", portFromMap(c.Ports, "xemu_https"))
		record.Set("xemu_ws", portFromMap(c.Ports, "xemu_ws"))
		record.Set("browser_web", portFromMap(c.Ports, "browser_web"))
		record.Set("browser_vnc", portFromMap(c.Ports, "browser_vnc"))
		record.Set("created", c.Created)

		if err := app.Save(record); err != nil {
			return fmt.Errorf("seed container %s: %w", c.Name, err)
		}
		log.Printf("  container %s: created", c.Name)
	}
	return nil
}

// portFromMap reads an integer port from the seed file's permissive
// map[string]any. JSON-decoded numbers arrive as float64.
func portFromMap(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
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
		name := r.GetString("name")
		out.Containers[name] = &seedContainer{
			Name:  name,
			Index: r.GetInt("index"),
			Ports: map[string]any{
				"xemu_http":   r.GetInt("xemu_http"),
				"xemu_https":  r.GetInt("xemu_https"),
				"xemu_ws":     r.GetInt("xemu_ws"),
				"browser_web": r.GetInt("browser_web"),
				"browser_vnc": r.GetInt("browser_vnc"),
			},
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
