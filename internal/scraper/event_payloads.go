package scraper

import "time"

// v2 event taxonomy — 5 event_types with kind / cause discriminators.
// Replaces the 24-event-type v1 map[string]any payloads. See
// atlas/new_json/04-ground-up-rebuild.md §6 (event stream) for the full
// shape spec and the kind catalogs.
//
// All five event payload structs embed EventCommon; constructors on the
// haloce events.Context populate it (seq=0 placeholder until per-class
// seq tracking lands).

// Event-type constants — top-level discriminator in every event payload.
// The envelope's outer Type stays "event" for live broadcasts and "events"
// for the events_reply backfill batch; these are the semantic types inside.
const (
	EventTypeDeath        = "death"
	EventTypeDamage       = "damage"
	EventTypeMedal        = "medal"
	EventTypePlayerUpdate = "player_update"
	EventTypeGameUpdate   = "game_update"
)

// Death causes (DeathEvent.Cause).
const (
	DeathCauseKill        = "kill"
	DeathCauseSuicide     = "suicide"
	DeathCauseFall        = "fall"
	DeathCauseBetrayal    = "betrayal"
	DeathCauseEnvironment = "environment"
	DeathCauseUnknown     = "unknown"
)

// Damage kinds (DamageEvent.Kind).
const (
	DamageKindHit   = "hit"
	DamageKindMelee = "melee"
)

// Medal kinds (MedalEvent.Kind).
const (
	MedalKindMultikill  = "multikill"
	MedalKindKillStreak = "kill_streak"
)

// PlayerUpdate kinds (PlayerUpdateEvent.Kind).
const (
	PlayerUpdateKindSpawn           = "spawn"
	PlayerUpdateKindScore           = "score"
	PlayerUpdateKindPowerupPickedUp = "powerup_picked_up"
	PlayerUpdateKindPowerupExpired  = "powerup_expired"
	PlayerUpdateKindVehicleEntered  = "vehicle_entered"
	PlayerUpdateKindVehicleExited   = "vehicle_exited"
	PlayerUpdateKindItemPickedUp    = "item_picked_up"
	PlayerUpdateKindItemDropped     = "item_dropped"
	PlayerUpdateKindItemDepleted    = "item_depleted"
	PlayerUpdateKindGrenadeThrown   = "grenade_thrown"
	PlayerUpdateKindPlayerQuit      = "player_quit"
)

// GameUpdate kinds (GameUpdateEvent.Kind).
const (
	GameUpdateKindPlayerJoined      = "player_joined"
	GameUpdateKindPlayerLeft        = "player_left"
	GameUpdateKindPlayerTeamChanged = "player_team_changed"
	GameUpdateKindTeamScore         = "team_score"
	GameUpdateKindGameStart         = "game_start"
	GameUpdateKindGameEnd           = "game_end"
	GameUpdateKindItemSpawned       = "item_spawned"
)

// Powerup kinds (used in PlayerUpdateEvent.Powerup for kind=powerup_*).
const (
	PowerupActiveCamouflage = "active_camouflage"
	PowerupOvershield       = "overshield"
)

// Grenade kinds (used in PlayerUpdateEvent.GrenadeType for kind=grenade_thrown).
const (
	GrenadeFrag   = "frag"
	GrenadePlasma = "plasma"
)

// Item-depletion causes (used in PlayerUpdateEvent.Cause for kind=item_depleted).
const (
	ItemDepletionAmmo   = "ammo"
	ItemDepletionEnergy = "energy"
)

// Vehicle seats (used in PlayerUpdateEvent.Seat for kind=vehicle_entered).
const (
	VehicleSeatDriver    = "driver"
	VehicleSeatPassenger = "passenger"
	VehicleSeatGunner    = "gunner"
)

// EventCommon is the metadata embedded into every v2 event payload — the
// envelope already carries seq/tick/ts but events also carry them inside
// the payload so an event is self-contained when archived in a
// previous_game.events log or an events_reply batch.
//
// Today seq is always 0 (placeholder); real per-(instance, type) counter
// lands when runner emission is rewritten.
type EventCommon struct {
	Seq       uint64    `json:"seq"`
	Tick      uint32    `json:"tick"`
	At        time.Time `json:"at"`
	EventType string    `json:"event_type"`
}

// DeathEvent is emitted on every player death. Cause discriminates:
//
//	"kill"        another player got the kill (Killer set, may be team_kill)
//	"betrayal"    same as "kill" but Killer is on the same team as Victim
//	"suicide"     self-inflicted (Killer nil; Weapon may be set for grenade-
//	              at-feet / sticky / rocket-into-wall)
//	"fall"        environmental fall damage (Killer nil, Weapon empty)
//	"environment" lava / water / kill volume (Killer nil, Weapon empty)
//	"unknown"     fallback when attribution failed
//
// Today's v1 separate `kill` and `team_kill` events fold into this — see
// the `team_kill` boolean (or `cause: "betrayal"`).
type DeathEvent struct {
	EventCommon

	Victim    PlayerRef `json:"victim"`
	VictimPos Vec3      `json:"victim_pos"`

	Killer    *PlayerRef `json:"killer"`     // nil when no attributed killer
	KillerPos *Vec3      `json:"killer_pos"` // nil iff Killer is nil

	Cause          string `json:"cause"`
	Weapon         string `json:"weapon"` // empty when unknown
	TeamKill       bool   `json:"team_kill"`
	RespawnInTicks uint32 `json:"respawn_in_ticks"`
}

// DamageEvent is emitted on HP loss. Kind discriminates "hit" (gunfire,
// splash, fall ticks, etc.) from "melee" (melee strike with damage).
// Dealer is nil for environmental damage (fall ticks, kill volumes that
// damage before killing).
type DamageEvent struct {
	EventCommon

	Kind string `json:"kind"`

	Dealer    *PlayerRef `json:"dealer"`     // nil for env damage
	DealerPos *Vec3      `json:"dealer_pos"` // nil iff Dealer is nil

	Victim    PlayerRef `json:"victim"`
	VictimPos Vec3      `json:"victim_pos"`

	Amount float32 `json:"amount"`
	Weapon string  `json:"weapon"` // empty when unknown
}

// MedalEvent is a milestone notification (multikill, kill streak, future
// medals). No position — milestones are achievements, not spatial events;
// the component DeathEvents carry positions.
type MedalEvent struct {
	EventCommon

	Kind   string    `json:"kind"`
	Player PlayerRef `json:"player"`
	Count  uint16    `json:"count"`
}

// PlayerUpdateEvent is the union for all per-player state changes that
// aren't combat or medals. Kind discriminates which optional fields are
// meaningful; the rest are zero / nil and omitted on the wire.
//
// Field summary by kind:
//
//	spawn               player, pos
//	score               player, kills, deaths, assists, kill_streak, multikill
//	powerup_picked_up   player, pos, powerup
//	powerup_expired     player, pos, powerup
//	vehicle_entered     player, pos, vehicle, seat
//	vehicle_exited      player, pos, vehicle
//	item_picked_up      player, pos, item
//	item_dropped        player, item, pos (where it landed)
//	item_depleted       player, item, cause ("ammo" | "energy")
//	grenade_thrown      player, pos, grenade_type, remaining
//	player_quit         player
type PlayerUpdateEvent struct {
	EventCommon

	Kind   string    `json:"kind"`
	Player PlayerRef `json:"player"`
	Pos    *Vec3     `json:"pos,omitempty"`

	// kind=score
	Kills      *int16  `json:"kills,omitempty"`
	Deaths     *int16  `json:"deaths,omitempty"`
	Assists    *int16  `json:"assists,omitempty"`
	KillStreak *uint16 `json:"kill_streak,omitempty"`
	Multikill  *uint16 `json:"multikill,omitempty"`

	// kind=powerup_*
	Powerup string `json:"powerup,omitempty"`

	// kind=vehicle_*
	Vehicle *VehicleRef `json:"vehicle,omitempty"`
	Seat    string      `json:"seat,omitempty"`

	// kind=item_* (item.spawn_id is omitempty in ItemRef itself)
	Item *ItemRef `json:"item,omitempty"`

	// kind=grenade_thrown
	GrenadeType string `json:"grenade_type,omitempty"`
	Remaining   *uint8 `json:"remaining,omitempty"`

	// kind=item_depleted ("ammo" | "energy")
	Cause string `json:"cause,omitempty"`
}

// GameUpdateEvent is the union for match-level / roster / world state
// changes. Player is set for player-driven roster events (joined / left /
// team_changed); nil for match-level (team_score / game_start / game_end)
// and world (item_spawned).
type GameUpdateEvent struct {
	EventCommon

	Kind   string     `json:"kind"`
	Player *PlayerRef `json:"player,omitempty"`

	// kind=player_team_changed (Player.Team is the new team)
	PreviousTeam *uint32 `json:"previous_team,omitempty"`

	// kind=team_score
	Team  *uint32 `json:"team,omitempty"`
	Score *int32  `json:"score,omitempty"`

	// kind=game_start, kind=game_end
	Map      string `json:"map,omitempty"`
	Gametype string `json:"gametype,omitempty"`

	// kind=item_spawned
	Item *ItemRef `json:"item,omitempty"`
	Pos  *Vec3    `json:"pos,omitempty"`
}
