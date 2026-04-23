//go:build !dev

package seed

import "github.com/pocketbase/pocketbase"

// Run is a no-op in production builds.
func Run(_ *pocketbase.PocketBase) error { return nil }