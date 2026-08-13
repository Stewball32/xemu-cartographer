package scraper

import "strings"

// ContainerMembership is the per-container identity view M09 uses to route a
// logged-in player to the kiosk for the match they're in. Identities is the
// deduped, lowercased+trimmed set of names that identify this container's
// occupants — the local Xbox console name, the system-link machine names, and
// the in-game player roster names — so a caller can test a user's gamertag
// against it case-insensitively without caring which surface the name came
// from.
type ContainerMembership struct {
	Container  string   `json:"container"`
	Identities []string `json:"identities"`
}

// Membership exposes the cross-instance roster identity view. The per-user
// play ownership/roster resolver, the WS room guard, and the kiosk/VNC
// proxy all read it to decide whether a given user belongs to a given
// container.
type Membership interface {
	Membership() []ContainerMembership
}

// SanitizeIdentity normalizes a name (console / machine / player / gamertag)
// for case-insensitive matching. Mirrors the gamertags_sanitize hook so the
// container-side identities line up with the gamertags.sanitized column.
func SanitizeIdentity(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// MatchContainer returns the first container (deterministic order — callers
// pass a sorted view) whose identity set contains any of the caller's
// gamertags. ok is false when no container matches (idle / not in a match).
func MatchContainer(view []ContainerMembership, sanitizedTags []string) (string, bool) {
	for _, m := range view {
		if membershipContains(m, sanitizedTags) {
			return m.Container, true
		}
	}
	return "", false
}

// ContainerHasGamertag reports whether the named container's identity set
// contains any of the caller's gamertags. Used by the WS room guard and the
// kiosk/VNC proxy to authorize access to one specific container.
func ContainerHasGamertag(view []ContainerMembership, container string, sanitizedTags []string) bool {
	for _, m := range view {
		if m.Container == container {
			return membershipContains(m, sanitizedTags)
		}
	}
	return false
}

// membershipContains tests one container's identity set against the caller's
// tags. Both sides are sanitized defensively so the helper is correct even if
// a caller forgets to pre-normalize.
func membershipContains(m ContainerMembership, sanitizedTags []string) bool {
	if len(sanitizedTags) == 0 || len(m.Identities) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(m.Identities))
	for _, id := range m.Identities {
		if id = SanitizeIdentity(id); id != "" {
			set[id] = struct{}{}
		}
	}
	for _, t := range sanitizedTags {
		if t = SanitizeIdentity(t); t == "" {
			continue
		}
		if _, ok := set[t]; ok {
			return true
		}
	}
	return false
}
