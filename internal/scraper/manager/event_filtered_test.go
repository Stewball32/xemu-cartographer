package manager

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/capture"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/roster"
)

// deathEnv wraps a DeathEvent in the "event" envelope the runner loop hands
// to broadcastEventsFiltered, with the event_type discriminator set (the
// haloce emitter fills EventCommon; here we do it explicitly).
func deathEnv(tick uint32, d scraper.DeathEvent) scraper.Envelope {
	d.EventType = scraper.EventTypeDeath
	d.Tick = tick
	return scraper.MakeEnvelope("event", "alpha", 0, tick, d)
}

func playerRef(index int, name string, team uint32) scraper.PlayerRef {
	return scraper.PlayerRef{Index: index, Name: name, Team: team}
}

// visibleAll is the roster view for a match where every listed index is a
// real player.
func visibleAll(indices ...int) map[int]bool {
	out := make(map[int]bool, len(indices))
	for _, i := range indices {
		out[i] = true
	}
	return out
}

// TestBuildDeathFilteredKeepsAttributedKill: the ordinary case — both
// players visible, so identity and attribution pass through untouched.
func TestBuildDeathFilteredKeepsAttributedKill(t *testing.T) {
	killer := playerRef(1, "Stewball", 0)
	env := deathEnv(120, scraper.DeathEvent{
		Victim:         playerRef(0, "gravemind", 1),
		VictimPos:      scraper.Vec3{X: 1, Y: 2, Z: 3},
		Killer:         &killer,
		KillerPos:      &scraper.Vec3{X: 4, Y: 5, Z: 6},
		Cause:          scraper.DeathCauseKill,
		Weapon:         "pistol",
		RespawnInTicks: 90,
	})

	got, ok := buildDeathFiltered(env, visibleAll(0, 1))
	if !ok {
		t.Fatal("attributed kill between two visible players was dropped")
	}
	if got.Killer == nil || got.Killer.Name != "Stewball" {
		t.Fatalf("killer = %+v, want Stewball", got.Killer)
	}
	if got.Cause != scraper.DeathCauseKill || got.Weapon != "pistol" {
		t.Fatalf("attribution altered: cause=%q weapon=%q", got.Cause, got.Weapon)
	}
	if got.Victim.Name != "gravemind" || got.RespawnInTicks != 90 || got.Tick != 120 {
		t.Fatalf("victim/respawn/tick mangled: %+v", got)
	}
}

// TestDeathFilteredJSONHasNoPositions asserts the property the DeathFiltered
// type exists to guarantee, against the actual wire bytes rather than the
// struct — JSON is the leak surface, and a struct-level assertion would pass
// even if an embedded field reintroduced the coordinates.
func TestDeathFilteredJSONHasNoPositions(t *testing.T) {
	killer := playerRef(1, "Stewball", 0)
	env := deathEnv(7, scraper.DeathEvent{
		Victim:    playerRef(0, "gravemind", 1),
		VictimPos: scraper.Vec3{X: 111, Y: 222, Z: 333},
		Killer:    &killer,
		KillerPos: &scraper.Vec3{X: 444, Y: 555, Z: 666},
		Cause:     scraper.DeathCauseKill,
	})

	got, ok := buildDeathFiltered(env, visibleAll(0, 1))
	if !ok {
		t.Fatal("event dropped")
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wire := string(b)
	for _, banned := range []string{"victim_pos", "killer_pos", "111", "444"} {
		if strings.Contains(wire, banned) {
			t.Fatalf("filtered death leaked %q on the wire: %s", banned, wire)
		}
	}
}

// TestBuildDeathFilteredKeepsUnattributedDeaths: suicide / fall /
// environment name nobody, and the overlay needs them to raise the
// RESPAWNING pill. Dropping them would leave a dead player's card stuck
// with no timer, so they must survive with Killer nil.
func TestBuildDeathFilteredKeepsUnattributedDeaths(t *testing.T) {
	for _, cause := range []string{
		scraper.DeathCauseSuicide,
		scraper.DeathCauseFall,
		scraper.DeathCauseEnvironment,
	} {
		t.Run(cause, func(t *testing.T) {
			env := deathEnv(50, scraper.DeathEvent{
				Victim:         playerRef(0, "gravemind", 1),
				Cause:          cause,
				RespawnInTicks: 90,
			})
			got, ok := buildDeathFiltered(env, visibleAll(0))
			if !ok {
				t.Fatalf("%s dropped; the RESPAWNING pill depends on it", cause)
			}
			if got.Killer != nil {
				t.Fatalf("%s gained a killer: %+v", cause, got.Killer)
			}
			if got.Cause != cause {
				t.Fatalf("cause = %q, want %q", got.Cause, cause)
			}
		})
	}
}

// TestBuildDeathFilteredKeepsBetrayal: a betrayal between two visible
// players keeps both its cause and its team_kill flag — the overlay
// distinguishes it from a normal kill.
func TestBuildDeathFilteredKeepsBetrayal(t *testing.T) {
	killer := playerRef(1, "Stewball", 1)
	env := deathEnv(60, scraper.DeathEvent{
		Victim:   playerRef(0, "gravemind", 1),
		Killer:   &killer,
		Cause:    scraper.DeathCauseBetrayal,
		TeamKill: true,
	})
	got, ok := buildDeathFiltered(env, visibleAll(0, 1))
	if !ok {
		t.Fatal("betrayal dropped")
	}
	if got.Cause != scraper.DeathCauseBetrayal || !got.TeamKill {
		t.Fatalf("betrayal flattened: cause=%q team_kill=%v", got.Cause, got.TeamKill)
	}
}

// TestBuildDeathFilteredDropsDummyVictim: the dummy seat isn't on the
// viewer's roster, so a death for it must not reach the filtered class at
// all — an overlay that received it would have nowhere to put it and the
// event itself would prove the hidden seat exists.
func TestBuildDeathFilteredDropsDummyVictim(t *testing.T) {
	env := deathEnv(30, scraper.DeathEvent{
		Victim: playerRef(3, "DummyHost", 0),
		Cause:  scraper.DeathCauseFall,
	})
	if _, ok := buildDeathFiltered(env, visibleAll(0, 1)); ok {
		t.Fatal("death of a hidden dummy victim was published")
	}
}

// TestBuildDeathFilteredScrubsDummyKiller: the victim is real so the event
// must survive (they need their respawn timer), but every field that would
// betray the hidden killer is cleared as a set. Leaving cause="betrayal"
// with killer=null would still tell the viewer a hidden teammate exists.
func TestBuildDeathFilteredScrubsDummyKiller(t *testing.T) {
	killer := playerRef(3, "DummyHost", 1)
	env := deathEnv(40, scraper.DeathEvent{
		Victim:         playerRef(0, "gravemind", 1),
		Killer:         &killer,
		Cause:          scraper.DeathCauseBetrayal,
		Weapon:         "sniper",
		TeamKill:       true,
		RespawnInTicks: 90,
	})

	got, ok := buildDeathFiltered(env, visibleAll(0, 1))
	if !ok {
		t.Fatal("real victim's death was dropped because the killer was hidden")
	}
	if got.Killer != nil {
		t.Fatalf("hidden killer published: %+v", got.Killer)
	}
	if got.Cause != scraper.DeathCauseUnknown {
		t.Fatalf("cause = %q, want %q — betrayal implies a teammate", got.Cause, scraper.DeathCauseUnknown)
	}
	if got.Weapon != "" || got.TeamKill {
		t.Fatalf("residual attribution: weapon=%q team_kill=%v", got.Weapon, got.TeamKill)
	}
	if got.RespawnInTicks != 90 {
		t.Fatalf("respawn lost in the scrub: %d", got.RespawnInTicks)
	}
}

// TestBuildDeathFilteredRejectsNonDeathAndMalformed: the class is
// deaths-only, and anything that fails to decode fails closed.
func TestBuildDeathFilteredRejectsNonDeathAndMalformed(t *testing.T) {
	medal := scraper.MakeEnvelope("event", "alpha", 0, 10, scraper.MedalEvent{
		EventCommon: scraper.EventCommon{EventType: scraper.EventTypeMedal, Tick: 10},
		Kind:        scraper.MedalKindMultikill,
		Player:      playerRef(0, "gravemind", 1),
		Count:       2,
	})
	if _, ok := buildDeathFiltered(medal, visibleAll(0)); ok {
		t.Fatal("medal event published on the deaths-only class")
	}

	empty := scraper.Envelope{Type: "event", Tick: 10}
	if _, ok := buildDeathFiltered(empty, visibleAll(0)); ok {
		t.Fatal("envelope with no payload was published")
	}

	garbage := scraper.Envelope{Type: "event", Tick: 10, Data: json.RawMessage(`{"victim":`)}
	if _, ok := buildDeathFiltered(garbage, visibleAll(0)); ok {
		t.Fatal("undecodable payload was published")
	}
}

// TestVisibleIndicesMatchesGameFiltered: the two viewer-facing classes must
// agree on who exists, so visibleIndices is asserted against the roster
// buildGameFilteredPayload actually publishes rather than against a
// hand-written expectation.
func TestVisibleIndicesMatchesGameFiltered(t *testing.T) {
	c := &instanceCache{
		GameData: &scraper.GameData{
			Players: []scraper.GamePlayer{
				{Index: 0, Name: "IdleLocal", IsLocal: boolp(true)},
				{Index: 1, Name: "ActiveLocal", IsLocal: boolp(true)},
				{Index: 2, Name: "Remote", IsLocal: boolp(false)},
			},
		},
		PlayerAccum: map[int]scraper.PlayerAccum{1: {Active: true}},
	}

	got := visibleIndices(c, roster.Config{})
	want := map[int]bool{}
	for _, p := range buildGameFilteredPayload(c, roster.Config{}).Players {
		want[p.Index] = true
	}
	if len(got) != len(want) {
		t.Fatalf("visibleIndices = %v, game_filtered roster = %v", got, want)
	}
	for idx := range want {
		if !got[idx] {
			t.Fatalf("index %d visible in game_filtered but not in event_filtered", idx)
		}
	}
}

// TestVisibleIndicesFailsClosed: with no roster we cannot tell a dummy from
// a player, so nobody is visible and every death is dropped.
func TestVisibleIndicesFailsClosed(t *testing.T) {
	if v := visibleIndices(nil, roster.Config{}); len(v) != 0 {
		t.Fatalf("nil cache: %v, want nobody visible", v)
	}
	if v := visibleIndices(&instanceCache{}, roster.Config{}); len(v) != 0 {
		t.Fatalf("nil GameData: %v, want nobody visible", v)
	}
}

// TestBroadcastEventsFilteredEmitsOnlyDeaths: one batch carrying a death
// and a medal produces exactly one envelope, on the event_filtered room,
// typed event_filtered (not "event" — that would double-broadcast the raw
// class the generic broadcast already handled).
func TestBroadcastEventsFilteredEmitsOnlyDeaths(t *testing.T) {
	ws := &stubWS{occupied: map[string]bool{"host:alpha:event_filtered": true}}
	svc := &guards.Services{WS: ws}
	r := newTestRunner("alpha")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseLive
		c.GameData = &scraper.GameData{
			Players: []scraper.GamePlayer{
				{Index: 0, Name: "gravemind", IsLocal: boolp(false)},
				{Index: 1, Name: "Stewball", IsLocal: boolp(false)},
			},
		}
	})

	killer := playerRef(1, "Stewball", 0)
	r.broadcastEventsFiltered(svc, []scraper.Envelope{
		scraper.MakeEnvelope("event", "alpha", 0, 10, scraper.MedalEvent{
			EventCommon: scraper.EventCommon{EventType: scraper.EventTypeMedal, Tick: 10},
			Kind:        scraper.MedalKindMultikill,
			Player:      killer,
		}),
		deathEnv(11, scraper.DeathEvent{
			Victim: playerRef(0, "gravemind", 1),
			Killer: &killer,
			Cause:  scraper.DeathCauseKill,
		}),
	})

	sends := ws.snapshot()
	if len(sends) != 1 {
		t.Fatalf("want one send, got %d: %+v", len(sends), sends)
	}
	if sends[0].Room != "host:alpha:event_filtered" {
		t.Fatalf("room = %q", sends[0].Room)
	}
	_, env := decodeClassEnvelope(t, sends[0].Data)
	if env.Type != envelopeTypeEventFiltered {
		t.Fatalf("envelope type = %q, want %q", env.Type, envelopeTypeEventFiltered)
	}
	if env.Tick != 11 {
		t.Fatalf("envelope tick = %d, want the event's own tick 11", env.Tick)
	}
	var d DeathFiltered
	if err := json.Unmarshal(env.Data, &d); err != nil {
		t.Fatalf("unmarshal DeathFiltered: %v", err)
	}
	if d.Victim.Name != "gravemind" || d.Killer == nil || d.Killer.Name != "Stewball" {
		t.Fatalf("payload = %+v", d)
	}
}

// TestBroadcastEventsFilteredRespectsDemand: with nobody in the room and no
// capture policy the class costs nothing; a `never` policy hard-caps it even
// when an overlay is subscribed.
func TestBroadcastEventsFilteredRespectsDemand(t *testing.T) {
	events := []scraper.Envelope{
		deathEnv(11, scraper.DeathEvent{
			Victim: playerRef(0, "gravemind", 1),
			Cause:  scraper.DeathCauseFall,
		}),
	}
	withRoster := func(r *runner) {
		r.withCache(func(c *instanceCache) {
			c.Phase = PhaseLive
			c.GameData = &scraper.GameData{
				Players: []scraper.GamePlayer{{Index: 0, Name: "gravemind", IsLocal: boolp(false)}},
			}
		})
	}

	t.Run("no subscriber", func(t *testing.T) {
		ws := &stubWS{occupied: map[string]bool{}}
		r := newTestRunner("alpha")
		defer r.cancel()
		withRoster(r)
		r.broadcastEventsFiltered(&guards.Services{WS: ws}, events)
		if sends := ws.snapshot(); len(sends) != 0 {
			t.Fatalf("emitted with no subscriber: %+v", sends)
		}
	})

	t.Run("never policy hard-caps a subscribed room", func(t *testing.T) {
		ws := &stubWS{occupied: map[string]bool{"host:alpha:event_filtered": true}}
		r := newTestRunner("alpha")
		defer r.cancel()
		withRoster(r)
		r.setPolicies([]capture.Policy{
			{Instance: "alpha", Class: envelopeTypeEventFiltered, Mode: capture.ModeNever},
		})
		r.broadcastEventsFiltered(&guards.Services{WS: ws}, events)
		if sends := ws.snapshot(); len(sends) != 0 {
			t.Fatalf("never policy ignored: %+v", sends)
		}
	})
}
