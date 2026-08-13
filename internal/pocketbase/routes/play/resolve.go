package play

import (
	"log"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/gamertags"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

// resolveContainer is the pure scoping decision, in priority order:
//
//  1. an admin may target any container via the override;
//  2. OWNERSHIP — the caller's own per-user box ("<prefix>play-<uid>", the name
//     request-instance derives) resolves the moment it exists, roster or not.
//     This is what recognises a freshly-provisioned box (still booting, nobody
//     in the roster yet) and a neutral host (creator never spawns as a player)
//     as the creator's, so /play advances out of "Starting your box";
//  3. gamertag→roster match — the M09 path for joining someone else's box.
//
// Split from the request plumbing so it's unit-testable with no live container.
// ok is false when nothing resolves (idle) or an admin passed no override and
// owns/joins nothing — the caller renders that as idle (current/options) or
// refuses the action (control POSTs).
func resolveContainer(isAdmin bool, override, ownedBox string, tags []string, view []scraperiface.ContainerMembership) (string, bool) {
	if isAdmin && override != "" {
		return override, true
	}
	if ownedBox != "" {
		for _, m := range view {
			if m.Container == ownedBox {
				return ownedBox, true
			}
		}
	}
	return scraperiface.MatchContainer(view, tags)
}

// ownedBoxName derives the caller's per-user box name — the same derivation
// request-instance uses for a non-admin (instanceName's perUser scheme) — or ""
// when the provisioner isn't wired / the uid yields nothing.
func ownedBoxName(userID string) string {
	if Provisioner == nil {
		return ""
	}
	uid := sanitizeName(userID)
	if uid == "" {
		return ""
	}
	return Provisioner.NamePrefix() + "play-" + uid
}

// resolveCaller resolves the container for the current request. ok=false means
// "no active instance" and is NOT an error — a gamertag-resolution failure is
// logged and folded into the same idle result (fail-soft), so the caller never
// has to distinguish the two.
func resolveCaller(e *core.RequestEvent) (name string, ok bool) {
	isAdmin := roles.IsAdminAuth(e.App, e.Auth)
	override := e.Request.URL.Query().Get("container")

	var tags []string
	if !isAdmin || override == "" {
		t, err := gamertags.SanitizedForUser(e.App, e.Auth.Id)
		if err != nil {
			log.Printf("/api/play: gamertags for %s: %v", e.Auth.Id, err)
			tags = nil // fail soft — ownership resolution below still applies
		} else {
			tags = t
		}
	}
	return resolveContainer(isAdmin, override, ownedBoxName(e.Auth.Id), tags, Scraper.Membership())
}
