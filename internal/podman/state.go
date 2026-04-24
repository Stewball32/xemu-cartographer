package podman

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ContainerInfo describes a managed container pair (xemu + browser).
type ContainerInfo struct {
	Name    string    `json:"name"`
	Index   int       `json:"index"` // port allocation index
	Ports   Ports     `json:"ports"`
	Created time.Time `json:"created"`
}

// State persists the set of managed containers to a JSON file.
type State struct {
	Containers map[string]*ContainerInfo `json:"containers"`
	path       string
}

// LoadState reads state from path, returning an empty state if the file does
// not exist.
func LoadState(path string) (*State, error) {
	s := &State{
		Containers: make(map[string]*ContainerInfo),
		path:       path,
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Containers == nil {
		s.Containers = make(map[string]*ContainerInfo)
	}
	return s, nil
}

// Save writes state atomically (write tmp + rename).
func (s *State) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// NextIndex returns the lowest non-negative index not used by any existing
// container.
func (s *State) NextIndex() int {
	used := make(map[int]struct{}, len(s.Containers))
	for _, c := range s.Containers {
		used[c.Index] = struct{}{}
	}
	for i := 0; ; i++ {
		if _, ok := used[i]; !ok {
			return i
		}
	}
}
