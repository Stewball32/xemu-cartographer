package podman

import "time"

// ContainerInfo describes a managed container pair (xemu + browser).
type ContainerInfo struct {
	Name  string `json:"name"`
	Index int    `json:"index"` // port allocation index
	Ports Ports  `json:"ports"`
	// GameISO is the absolute host path of the game ISO attached as this
	// instance's DVD (resolved at create time from CreateOptions.GameISO or the
	// global Config.DVDPath), or "" when no disc is attached. Bind-mounted
	// read-only into the container; the master image auto-launches it.
	GameISO string    `json:"game_iso,omitempty"`
	Created time.Time `json:"created"`
}

// nextIndex returns the lowest non-negative index not used by any container in
// the given map.
func nextIndex(containers map[string]*ContainerInfo) int {
	used := make(map[int]struct{}, len(containers))
	for _, c := range containers {
		used[c.Index] = struct{}{}
	}
	for i := 0; ; i++ {
		if _, ok := used[i]; !ok {
			return i
		}
	}
}
