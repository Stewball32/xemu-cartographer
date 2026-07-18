package manager

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestWireSplitFields documents + guards the EXACT on-the-wire fields the
// splitscreen overlay derives its layout from. Everything here is HOST-reported:
// the scraper reads only the host box's game state, so `players[].is_local`,
// `players[].local_index`, `machines[].is_local` (current_state) and `locals[]`
// (state_update) describe the HOST's local splitscreen. Remote (system-link)
// players show up in the host's roster with is_local=false and are NOT locals.
//
// The overlay must never assume per-player-box telemetry — this test pins the
// host-only fields it actually consumes so a wire change that drops them fails
// loudly.
func TestWireSplitFields(t *testing.T) {
	yes, no := true, false
	l0, l1 := 0, 1
	m0, m1 := 0, 1

	// A host box running 2-player splitscreen in a system-link team game, with
	// one remote player joined from a second box.
	game := GamePayload{
		Phase:  PhaseLive,
		Config: &GameConfig{Gametype: "slayer", IsTeamGame: true},
		Players: []GameRosterPlayer{
			{Index: 0, Name: "STEW", Team: 0, Kills: 12, Deaths: 3, Assists: 7, KillStreak: 3,
				ShotsFired: 100, ShotsHit: 54, IsLocal: &yes, LocalIndex: &l0, MachineIndex: &m0},
			{Index: 1, Name: "BLUEFOX", Team: 1, Kills: 8, Deaths: 5, KillStreak: 1,
				IsLocal: &yes, LocalIndex: &l1, MachineIndex: &m0},
			{Index: 2, Name: "REMOTE", Team: 1, Kills: 4, Deaths: 6,
				IsLocal: &no, LocalIndex: nil, MachineIndex: &m1}, // system-link remote — NOT a host local
		},
		Machines: []GameMachine{
			{Index: 0, Name: "HOST-XBOX", IsLocal: &yes},
			{Index: 1, Name: "GUEST-XBOX", IsLocal: &no},
		},
	}

	tick := TickPayload{
		Players: []TickPlayer{
			{Index: 0, Alive: true, Health: 1.0, Shields: 1.0, HasCamo: false},
			{Index: 1, Alive: false, Health: 0, Shields: 0, RespawnInTicks: u32ptr(90), HasCamo: true},
			{Index: 2, Alive: true, Health: 0.5, Shields: 1.0},
		},
		Locals: []TickLocal{{LocalIndex: 0}, {LocalIndex: 1}}, // host's 2 local viewports
	}

	gj, err := json.MarshalIndent(game, "", "  ")
	if err != nil {
		t.Fatalf("marshal game: %v", err)
	}
	tj, err := json.MarshalIndent(tick, "", "  ")
	if err != nil {
		t.Fatalf("marshal tick: %v", err)
	}
	t.Logf("current_state payload (host-reported):\n%s", gj)
	t.Logf("state_update payload (host-reported):\n%s", tj)

	// current_state must carry the host-local markers the overlay reads.
	for _, want := range []string{
		`"is_team_game": true`,
		`"is_local": true`,  // host local player
		`"is_local": false`, // system-link remote
		`"local_index": 0`,  // host viewport 0
		`"local_index": 1`,  // host viewport 1
		`"name": "STEW"`,
		`"kill_streak": 3`,
		`"shots_fired": 100`,
	} {
		if !strings.Contains(string(gj), want) {
			t.Errorf("current_state missing %s", want)
		}
	}

	// state_update must carry the host's local viewports + per-player live state.
	for _, want := range []string{
		`"locals": [`,
		`"local_index": 0`,
		`"local_index": 1`,
		`"alive": true`,
		`"health": 1`,
		`"shields": 1`,
		`"has_camo": true`,
		`"respawn_in_ticks": 90`,
	} {
		if !strings.Contains(string(tj), want) {
			t.Errorf("state_update missing %s", want)
		}
	}

	// The host derives split = 2 (two is_local players / two locals), NOT 3 —
	// the remote is excluded. This mirrors overlay-split.deriveSplitCount.
	localFromRoster := 0
	for _, p := range game.Players {
		if p.IsLocal != nil && *p.IsLocal {
			localFromRoster++
		}
	}
	if localFromRoster != 2 {
		t.Errorf("host-local count = %d, want 2 (remote excluded)", localFromRoster)
	}
	if len(tick.Locals) != 2 {
		t.Errorf("tick.locals = %d, want 2", len(tick.Locals))
	}
}

func u32ptr(v uint32) *uint32 { return &v }
