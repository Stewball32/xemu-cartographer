package sinks

import (
	"fmt"
	"strings"
	"sync"
)

// Constructor builds a Sink for one parsed spec ("rest" is the part
// after the "scheme:" prefix). vars is the per-runner substitution map
// the caller passes through Parse — at present that's {instance}; PR
// follow-ups may add {game_id}.
type Constructor func(rest string, vars map[string]string) (Sink, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Constructor{
		"file": parseFile,
	}
)

// Register installs a Constructor for the named scheme. Called once
// at startup — file: registers in init() in this package; pb: registers
// from a separate file (pb.go) that holds the PocketBase dependency,
// so the sinks package itself stays free of PB imports for callers
// that only need file:.
//
// Re-registering a scheme replaces the existing constructor. There's
// no Unregister: schemes are append-only in practice.
func Register(scheme string, c Constructor) {
	registryMu.Lock()
	registry[scheme] = c
	registryMu.Unlock()
}

// Parse interprets a sink spec string (as stored in
// capture_policies.sink) and returns the matching Sink.
//
// Recognised schemes are looked up in the registry — file: ships with
// the package; pb: is registered from internal/scraper/sinks/pb.go
// when its RegisterPBSink helper is called by main.go.
//
// Empty spec → (nil, nil); the runner treats that as "no sink, just
// broadcast". Unknown scheme → error, so a typo or a missing
// RegisterPBSink call fails loudly at policy-load time instead of
// silently dropping captures.
//
// Path templates may reference vars by name as "{key}" — known keys
// for PR 17 are {instance}. Unknown placeholders are left as-is so a
// future release can introduce new vars without breaking older policies.
func Parse(spec string, vars map[string]string) (Sink, error) {
	if spec == "" {
		return nil, nil
	}
	idx := strings.IndexByte(spec, ':')
	if idx <= 0 {
		return nil, fmt.Errorf("sink: spec %q missing scheme prefix (file: or pb:)", spec)
	}
	scheme, rest := spec[:idx], spec[idx+1:]

	registryMu.RLock()
	ctor, ok := registry[scheme]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("sink: unknown scheme %q in %q", scheme, spec)
	}
	return ctor(rest, vars)
}

// expandVars replaces every "{key}" in s with vars[key]. Missing keys
// are left intact so a future variable can be introduced without
// invalidating older policy rows.
func expandVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// parseFile is the built-in constructor for the file: scheme.
func parseFile(rest string, vars map[string]string) (Sink, error) {
	return NewFileSink(expandVars(rest, vars)), nil
}
