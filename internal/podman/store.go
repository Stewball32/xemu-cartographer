package podman

// Store persists ContainerInfo records. The default implementation is backed by
// a PocketBase collection (see internal/pocketbase/resolvers/containers.go).
// The interface keeps internal/podman free of PocketBase imports.
type Store interface {
	LoadAll() (map[string]*ContainerInfo, error)
	Upsert(info *ContainerInfo) error
	Delete(name string) error
}
