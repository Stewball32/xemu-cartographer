package manager

import (
	"sort"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

// Membership projects every attached runner into a ContainerMembership — the
// console / machine / player names that identify who is in that container — so
// the M09 "my match" resolver and the per-container access guards can route a
// logged-in player to the kiosk for the match their gamertag is in. Reads
// through readCache() like the other Manager surfaces, so it stays correct
// across phase transitions (the identity set empties out on Ready→Idle).
func (m *Manager) Membership() []scraperiface.ContainerMembership {
	m.mu.Lock()
	runners := make([]*runner, 0, len(m.runners))
	for _, r := range m.runners {
		runners = append(runners, r)
	}
	m.mu.Unlock()

	out := make([]scraperiface.ContainerMembership, 0, len(runners))
	for _, r := range runners {
		c := r.readCache()
		out = append(out, scraperiface.ContainerMembership{
			Container:  r.name,
			Identities: collectIdentities(&c),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Container < out[j].Container })
	return out
}

// collectIdentities returns the deduped, sanitized set of names that identify
// this container's occupants: the local Xbox console name (survives Ready→Idle
// via the system-snapshot pass), the system-link machine names, and the
// in-game player roster names. A user's gamertag is matched against any of
// these, so a player registered under either their console name or their
// in-game name still routes correctly. Sorted for deterministic output.
func collectIdentities(c *instanceCache) []string {
	set := map[string]struct{}{}
	add := func(s string) {
		if s = scraperiface.SanitizeIdentity(s); s != "" {
			set[s] = struct{}{}
		}
	}
	add(c.XboxName)
	if c.GameData != nil {
		for _, mch := range c.GameData.Machines {
			add(mch.Name)
		}
		for _, p := range c.GameData.Players {
			add(p.Name)
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
