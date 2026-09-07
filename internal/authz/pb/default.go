package pb

import "sync"

var (
	defaultMu   sync.RWMutex
	defaultDeps *PBDeps
)

// SetDefault installs the process-wide deps. main.go calls it once (B-1)
// before routes.RegisterAll; tests install their own and reset to nil.
func SetDefault(d *PBDeps) {
	defaultMu.Lock()
	defaultDeps = d
	defaultMu.Unlock()
}

// Default returns the process-wide deps, nil before SetDefault. Packages
// without a *guards.Services reach the adapter here; with a nil result
// authz.Can denies ("no_deps"), so a route bound before boot fails closed.
func Default() *PBDeps {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultDeps
}
