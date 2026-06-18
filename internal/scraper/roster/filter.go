// Package roster holds the M10d dummy-player / neutral-host filter — the
// single data-layer cleaning step shared by overlays (M10), minimaps (M11),
// and stats (M15) so a neutral host's out-of-bounds dummy player never leaks
// into a viewer-facing surface. The raw debug page (M06) keeps the unfiltered
// roster for diagnostics, so the filter lives here rather than mutating the
// scraper's cache in place.
package roster

import (
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// Config controls which players FilterRoster drops. The zero value is a no-op
// (no neutral host, empty allowlist), so an unconfigured container's roster
// passes through unchanged.
type Config struct {
	// IsNeutralHost drops this container's local player(s). In a modded
	// neutral-host match the host machine fields a dummy player that spawns
	// out of bounds and never participates; it is the local player on the
	// host seat, so dropping IsLocal players removes it.
	IsNeutralHost bool

	// DummyGamertags is the global always-dummy allowlist. Keys must already
	// be sanitized (see SanitizeName); any player whose name sanitizes into
	// the set is dropped regardless of the host flag.
	DummyGamertags map[string]struct{}
}

// FilterRoster returns the players with neutral-host dummies and globally
// listed dummy gamertags removed. Pure: it never mutates the input slice and
// preserves nil-vs-empty (a nil input returns nil).
func FilterRoster(players []scraper.GamePlayer, cfg Config) []scraper.GamePlayer {
	if players == nil {
		return nil
	}
	out := make([]scraper.GamePlayer, 0, len(players))
	for _, p := range players {
		if cfg.IsNeutralHost && p.IsLocal != nil && *p.IsLocal {
			continue
		}
		if _, dummy := cfg.DummyGamertags[SanitizeName(p.Name)]; dummy {
			continue
		}
		out = append(out, p)
	}
	return out
}

// SanitizeName normalizes a player name for allowlist matching — lowercased +
// trimmed, mirroring the gamertags_sanitize hook. Use it to build the
// Config.DummyGamertags set from the dummy_gamertags collection's raw rows.
func SanitizeName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// BuildDummySet turns the raw dummy-gamertag strings (e.g. the `gamertag`
// column of every dummy_gamertags row) into a sanitized lookup set ready for
// Config.DummyGamertags.
func BuildDummySet(raw []string) map[string]struct{} {
	if len(raw) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(raw))
	for _, r := range raw {
		if s := SanitizeName(r); s != "" {
			set[s] = struct{}{}
		}
	}
	return set
}
