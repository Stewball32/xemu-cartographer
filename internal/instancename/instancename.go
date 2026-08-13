// Package instancename derives the two DECOUPLED identities of a container
// instance from one user-supplied name:
//
//   - the CANONICAL / display name (Display): the pretty name, printable ASCII,
//     capped at 15 chars — the exact rule the Xbox console-nickname writer
//     enforces (internal/consolename.Sanitize). Stored as canonical and written
//     as the console nickname (and a future H2 profile name).
//   - the podman CONTAINER name fragment (Slug): the display name slugified into
//     a valid podman name (lowercase, spaces→'-', [a-z0-9._-] only, separator
//     runs collapsed, trimmed).
//
// Callers prepend their deployment prefix (e.g. "beta-") and/or a "play-<uid>"
// uniqueness suffix around Slug; Display is persisted verbatim so the pretty
// name survives independently of the mangled container name.
package instancename

import (
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/consolename"
)

// MaxDisplay is the display-name cap in characters (the CE player buffer only
// surfaces 12, so longer names truncate in CE — accepted; the cap is 15, the
// console-nickname buffer's limit).
const MaxDisplay = consolename.NameMax // 15

// Display normalizes a raw user name into the canonical pretty display name:
// printable ASCII (0x20–0x7E), trimmed, capped at MaxDisplay. This is exactly
// consolename.Sanitize — the same rule the console-nickname writer applies —
// so what validates here is byte-for-byte what gets written to the Xbox
// nickname. Returns "" when nothing usable remains.
func Display(raw string) string { return consolename.Sanitize(raw) }

// Slug derives a podman-safe container-name fragment from a (typically already
// Display-normalized) name:
//
//   - lowercase, trimmed;
//   - spaces become '-';
//   - keep [a-z0-9._-], drop everything else;
//   - collapse any run of 2+ separators ('-', '_', '.') into a single '-';
//   - trim leading/trailing separators.
//
// The result therefore starts and ends with an alphanumeric (or is ""), so
// prefix+Slug is always a valid podman name ([a-zA-Z0-9][a-zA-Z0-9_.-]*).
// Returns "" when nothing usable remains — the caller then falls back to its
// uid-based uniqueness scheme.
func Slug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))

	// Keep valid chars; map spaces to '-'; drop the rest.
	kept := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			kept = append(kept, r)
		case r == ' ':
			kept = append(kept, '-')
		}
	}

	// Collapse separator runs: a lone separator is kept as-is; a run of 2+ is
	// normalized to a single '-'.
	out := make([]rune, 0, len(kept))
	for i := 0; i < len(kept); {
		if isSep(kept[i]) {
			j := i
			for j < len(kept) && isSep(kept[j]) {
				j++
			}
			if j-i == 1 {
				out = append(out, kept[i])
			} else {
				out = append(out, '-')
			}
			i = j
			continue
		}
		out = append(out, kept[i])
		i++
	}

	return strings.Trim(string(out), "-_.")
}

func isSep(r rune) bool { return r == '-' || r == '_' || r == '.' }
