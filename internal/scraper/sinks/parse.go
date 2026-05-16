package sinks

import (
	"fmt"
	"strings"
)

// Parse interprets a sink spec string (as stored in
// capture_policies.sink) and returns the matching Sink.
//
// Recognised schemes:
//
//	file:<path>   — append-mode NDJSON file (FileSink)
//
// Empty spec → (nil, nil); the runner treats that as "no sink, just
// broadcast". Unknown scheme → error, so a typo in a policy row fails
// loudly at load time instead of silently dropping captures.
//
// Path templates may reference the vars passed in via the vars map —
// the recognised variable for PR 17 is {instance}. `{game_id}` is in
// the design doc but requires game-boundary tracking and lands later.
// Unknown placeholders are left as-is rather than erroring so a future
// release can introduce new vars without breaking older policies.
//
// PR 18 adds the `pb:<collection>` scheme.
func Parse(spec string, vars map[string]string) (Sink, error) {
	if spec == "" {
		return nil, nil
	}
	idx := strings.IndexByte(spec, ':')
	if idx <= 0 {
		return nil, fmt.Errorf("sink: spec %q missing scheme prefix (file: or pb:)", spec)
	}
	scheme, rest := spec[:idx], spec[idx+1:]
	switch scheme {
	case "file":
		return NewFileSink(expandVars(rest, vars)), nil
	default:
		return nil, fmt.Errorf("sink: unknown scheme %q in %q", scheme, spec)
	}
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
