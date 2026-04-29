package resolvers

import (
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/podman"
)

const containersCollection = "containers"

// ContainersStore is a podman.Store backed by the "containers" PB collection.
type ContainersStore struct {
	app core.App
}

// NewContainersStore returns a podman.Store backed by PocketBase.
func NewContainersStore(app core.App) *ContainersStore {
	return &ContainersStore{app: app}
}

// LoadAll returns every container record keyed by name.
func (s *ContainersStore) LoadAll() (map[string]*podman.ContainerInfo, error) {
	records, err := s.app.FindAllRecords(containersCollection)
	if err != nil {
		return nil, fmt.Errorf("load containers: %w", err)
	}

	out := make(map[string]*podman.ContainerInfo, len(records))
	for _, r := range records {
		info, err := recordToContainer(r)
		if err != nil {
			return nil, fmt.Errorf("load containers: record %s: %w", r.Id, err)
		}
		out[info.Name] = info
	}
	return out, nil
}

// Upsert creates or updates the record matching info.Name.
func (s *ContainersStore) Upsert(info *podman.ContainerInfo) error {
	collection, err := s.app.FindCollectionByNameOrId(containersCollection)
	if err != nil {
		return fmt.Errorf("find containers collection: %w", err)
	}

	record, _ := s.app.FindFirstRecordByFilter(containersCollection, "name = {:name}", map[string]any{"name": info.Name})
	if record == nil {
		record = core.NewRecord(collection)
	}

	portsJSON, err := json.Marshal(info.Ports)
	if err != nil {
		return fmt.Errorf("marshal ports: %w", err)
	}

	record.Set("name", info.Name)
	record.Set("index", info.Index)
	record.Set("ports", string(portsJSON))
	record.Set("created", info.Created)

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("save container %s: %w", info.Name, err)
	}
	return nil
}

// Delete removes the record matching name. Missing records are not an error.
func (s *ContainersStore) Delete(name string) error {
	record, _ := s.app.FindFirstRecordByFilter(containersCollection, "name = {:name}", map[string]any{"name": name})
	if record == nil {
		return nil
	}
	if err := s.app.Delete(record); err != nil {
		return fmt.Errorf("delete container %s: %w", name, err)
	}
	return nil
}

func recordToContainer(r *core.Record) (*podman.ContainerInfo, error) {
	var ports podman.Ports
	raw := r.GetString("ports")
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &ports); err != nil {
			return nil, fmt.Errorf("unmarshal ports: %w", err)
		}
	}

	return &podman.ContainerInfo{
		Name:    r.GetString("name"),
		Index:   r.GetInt("index"),
		Ports:   ports,
		Created: r.GetDateTime("created").Time(),
	}, nil
}
