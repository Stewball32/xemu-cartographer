package manager

import (
	"encoding/json"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/roster"
)

// envelopeTypeEventFiltered is the wire type for the viewer-facing event
// class. It is the event-stream counterpart to game_filtered: same dummy
// rule, same "filtering stays server-side" motivation, but a narrower
// surface — see DeathFiltered.
//
// Deliberately deaths-only. The overlay consumes exactly one event (a
// death, to paint the KILLED BY plate), and every other event type carries
// its own position / attribution fields that would each need their own
// scrub audit before they could be published to an unauthenticated viewer.
// Adding a second type here means adding a second Filtered payload type,
// not widening this one.
const envelopeTypeEventFiltered = "event_filtered"

// DeathFiltered is the viewer-facing projection of scraper.DeathEvent: the
// same identity + attribution fields, minus victim_pos / killer_pos.
//
// It is a distinct type rather than DeathEvent with `omitempty` positions
// because scraper.Vec3 is a struct — a zeroed Vec3 marshals as
// {"x":0,"y":0,"z":0}, not as an absent field, so `omitempty` would silently
// publish the origin instead of nothing. Making the positions structurally
// unrepresentable turns "no world coordinates on the public class" into a
// property the compiler enforces rather than one a reviewer has to check.
type DeathFiltered struct {
	scraper.EventCommon

	Victim scraper.PlayerRef  `json:"victim"`
	Killer *scraper.PlayerRef `json:"killer"` // nil when no visible attributed killer

	Cause          string `json:"cause"`
	Weapon         string `json:"weapon"`
	TeamKill       bool   `json:"team_kill"`
	RespawnInTicks uint32 `json:"respawn_in_ticks"`
}

// visibleIndices returns the player indices that survive the dummy filter —
// the exact roster the game_filtered class publishes for the same cache and
// config, so the two viewer-facing classes can never disagree about who
// exists.
//
// Nil cache / GameData yields a nil map, which reads as "nobody is visible"
// and drops every event. Fail-closed is the right default here: without a
// roster we cannot tell a dummy from a player.
func visibleIndices(c *instanceCache, cfg roster.Config) map[int]bool {
	if c == nil || c.GameData == nil {
		return nil
	}
	cfg.HideInactiveLocals = true
	cfg.ActiveLocals = activeLocals(c.PlayerAccum)
	visible := roster.FilterRoster(c.GameData.Players, cfg)
	out := make(map[int]bool, len(visible))
	for _, p := range visible {
		out[p.Index] = true
	}
	return out
}

// buildDeathFiltered projects one raw event envelope into its viewer-facing
// death payload. ok=false means the event must not reach the filtered class
// at all — it isn't a death, its payload didn't decode, or its victim is a
// dummy nobody is supposed to know about.
//
// A death with no attributed killer (suicide, fall, environment) still
// passes: the overlay needs it to raise the RESPAWNING pill, and it names
// nobody. A death whose killer is hidden passes too, but de-attributed —
// killer, cause, weapon and team_kill are scrubbed as a set, because any one
// of them surviving alone ("cause: betrayal, killer: null") would tell the
// viewer a hidden player exists and even which team they're on.
func buildDeathFiltered(ev scraper.Envelope, visible map[int]bool) (DeathFiltered, bool) {
	if len(ev.Data) == 0 {
		return DeathFiltered{}, false
	}
	var d scraper.DeathEvent
	if err := json.Unmarshal(ev.Data, &d); err != nil {
		return DeathFiltered{}, false
	}
	if d.EventType != scraper.EventTypeDeath {
		return DeathFiltered{}, false
	}
	if !visible[d.Victim.Index] {
		return DeathFiltered{}, false
	}
	out := DeathFiltered{
		EventCommon:    d.EventCommon,
		Victim:         d.Victim,
		Killer:         d.Killer,
		Cause:          d.Cause,
		Weapon:         d.Weapon,
		TeamKill:       d.TeamKill,
		RespawnInTicks: d.RespawnInTicks,
	}
	if d.Killer != nil && !visible[d.Killer.Index] {
		out.Killer = nil
		out.Cause = scraper.DeathCauseUnknown
		out.Weapon = ""
		out.TeamKill = false
	}
	return out, true
}

// broadcastEventsFiltered emits the viewer-facing death events for one
// batch of freshly-detected events, one envelope per surviving death on
// host:<inst>:event_filtered.
//
// Demand-gated like game_filtered, so it costs nothing when no overlay is
// subscribed and an operator can switch it off with a capture policy. The
// roster snapshot is built lazily on the first death in the batch — the
// overwhelming majority of ticks carry no death at all, so the common path
// is one cheap event_type peek per event and no cache read.
//
// There is deliberately NO join replay for this class (contrast
// classEnvelopeMessages, which replays the state classes). Replaying "the
// most recent death" to a late-joining overlay would paint a KILLED BY
// plate for a player who respawned minutes ago. The cost of omitting it is
// that an overlay connecting mid-death shows the RESPAWNING pill without a
// killer name for the rest of that one respawn window.
func (r *runner) broadcastEventsFiltered(svc *guards.Services, events []scraper.Envelope) {
	if svc == nil || svc.WS == nil || len(events) == 0 {
		return
	}
	if !shouldRead(r.name, envelopeTypeEventFiltered, r.getPolicies(), svc.WS) {
		return
	}
	var (
		visible map[int]bool
		loaded  bool
	)
	for _, ev := range events {
		if eventInnerType(ev) != scraper.EventTypeDeath {
			continue
		}
		if !loaded {
			c := r.readCache()
			visible = visibleIndices(&c, r.dummyConfig(svc.App))
			loaded = true
		}
		d, ok := buildDeathFiltered(ev, visible)
		if !ok {
			continue
		}
		r.emitClass(svc, envelopeTypeEventFiltered, ev.Tick, d)
	}
}
