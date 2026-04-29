//go:build !dev

package seed

import "github.com/pocketbase/pocketbase"

// ensureContainersFromSnapshot is a no-op in production builds.
func ensureContainersFromSnapshot(_ *pocketbase.PocketBase) error { return nil }

// RegisterContainerSnapshotHooks is a no-op in production builds.
func RegisterContainerSnapshotHooks(_ *pocketbase.PocketBase) {}
