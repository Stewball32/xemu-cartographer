package play

import (
	"log"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/gamertags"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

// resolveContainer is the pure scoping decision: an admin may target any
// container via the override; everyone else resolves to the single container
// whose live roster contains one of their gamertags. Split from the request
// plumbing so it's unit-testable with no live container.
//
// ok is false when a non-admin isn't in any container's roster (idle / not in a
// match) or an admin passed no override — the caller renders that as idle
// (current/options) or refuses the action (control POSTs).
func resolveContainer(isAdmin bool, override string, tags []string, view []scraperiface.ContainerMembership) (string, bool) {
	if isAdmin && override != "" {
		return override, true
	}
	return scraperiface.MatchContainer(view, tags)
}

// resolveCaller resolves the container for the current request. ok=false means
// "no active instance" and is NOT an error — a gamertag-resolution failure is
// logged and folded into the same idle result (fail-soft, matching
// /api/me/match), so the caller never has to distinguish the two.
func resolveCaller(e *core.RequestEvent) (name string, ok bool) {
	isAdmin := roles.IsAdminAuth(e.App, e.Auth)
	override := e.Request.URL.Query().Get("container")

	var tags []string
	if !isAdmin || override == "" {
		t, err := gamertags.SanitizedForUser(e.App, e.Auth.Id)
		if err != nil {
			log.Printf("/api/play: gamertags for %s: %v", e.Auth.Id, err)
			return "", false // fail soft → idle
		}
		tags = t
	}
	return resolveContainer(isAdmin, override, tags, Scraper.Membership())
}
