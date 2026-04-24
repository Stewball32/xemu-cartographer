package scraper

import "encoding/json"

// GameState describes the current state of the game engine.
type GameState string

const (
	GameStateMenu     GameState = "menu"
	GameStatePreGame  GameState = "pregame"
	GameStateInGame   GameState = "in_game"
	GameStatePostGame GameState = "postgame"
)

// Event type constants.
const (
	EventKill           = "kill"
	EventDeath          = "death"
	EventSpawn          = "spawn"
	EventDamage         = "damage"
	EventMelee          = "melee"
	EventTeamKill       = "team_kill"
	EventItemPickedUp   = "item_picked_up"
	EventItemDropped    = "item_dropped"
	EventItemSpawned    = "item_spawned"
	EventItemDepleted   = "item_depleted"
	EventGrenadeThrown  = "grenade_thrown"
	EventPowerupPickup  = "powerup_picked_up"
	EventPowerupExpired = "powerup_expired"
	EventMultikill      = "multikill"
	EventKillStreak     = "kill_streak"
	EventScore          = "score"
	EventVehicleEntered = "vehicle_entered"
	EventVehicleExited  = "vehicle_exited"
	EventPlayerQuit     = "player_quit"
	EventGameStart      = "game_start"
	EventGameEnd        = "game_end"
)

// Envelope is the top-level wrapper for every WebSocket message.
type Envelope struct {
	Type     string          `json:"type"`
	Instance string          `json:"instance"`
	Tick     uint32          `json:"tick"`
	Payload  json.RawMessage `json:"payload"`
}

// MakeEnvelope serialises a payload into an Envelope. Ignores marshal errors
// (caller controls the payload type).
func MakeEnvelope(msgType, instance string, tick uint32, payload any) Envelope {
	b, _ := json.Marshal(payload)
	return Envelope{Type: msgType, Instance: instance, Tick: tick, Payload: b}
}

// -------------------------------------------------------------------
// Snapshot types
// -------------------------------------------------------------------

// SnapshotPayload is sent on connect and on every game-state transition.
type SnapshotPayload struct {
	GameState       GameState        `json:"game_state"`
	Map             string           `json:"map"`
	Gametype        string           `json:"gametype"`
	IsTeamGame      bool             `json:"is_team_game"`
	ScoreLimit      int32            `json:"score_limit"`
	TimeLimitTicks  int32            `json:"time_limit_ticks"`
	TeamScores      []TeamScore      `json:"team_scores"`
	Players         []SnapshotPlayer `json:"players"`
	PowerItemSpawns []PowerItemSpawn `json:"power_item_spawns"`
}

// SnapshotPlayer is the static/score portion of a player.
//
// IsLocal/LocalIndex report whether a player is local to this xemu instance
// (vs remote via system-link) and, for locals, the splitscreen slot (0–3).
// Pointer types so games without local detection serialise them as null.
type SnapshotPlayer struct {
	Index      int    `json:"index"`
	Name       string `json:"name"`
	Team       uint32 `json:"team"`
	Kills      int16  `json:"kills"`
	Deaths     int16  `json:"deaths"`
	Assists    int16  `json:"assists"`
	CTFScore   int16  `json:"ctf_score"`
	TeamKills  int16  `json:"team_kills"`
	Suicides   int16  `json:"suicides"`
	KillStreak uint16 `json:"kill_streak"`
	Multikill  uint16 `json:"multikill"`
	ShotsFired int32  `json:"shots_fired"`
	ShotsHit   int16  `json:"shots_hit"`
	IsLocal    *bool  `json:"is_local"`
	LocalIndex *int   `json:"local_index"`
}

// TeamScore is one team's current score.
type TeamScore struct {
	Team  uint32 `json:"team"`
	Score int32  `json:"score"`
}

// PowerItemSpawn describes one scenario power item spawn point.
// InitialObjectID is used internally and excluded from JSON.
type PowerItemSpawn struct {
	SpawnID            int     `json:"spawn_id"`
	Tag                string  `json:"tag"`
	SpawnIntervalTicks int16   `json:"spawn_interval_ticks"`
	X                  float32 `json:"x"`
	Y                  float32 `json:"y"`
	Z                  float32 `json:"z"`
	InitialObjectID    uint32  `json:"-"` // objectID at game start; 0xFFFF if not found
}

// -------------------------------------------------------------------
// Tick types
// -------------------------------------------------------------------

// TickPayload is the 30Hz broadcast payload.
type TickPayload struct {
	Players    []TickPlayer      `json:"players"`
	PowerItems []PowerItemStatus `json:"power_items"`
}

// TickPlayer is the full per-player dynamic state for one tick.
type TickPlayer struct {
	Index              int          `json:"index"`
	Alive              bool         `json:"alive"`
	RespawnInTicks     *uint32      `json:"respawn_in_ticks"`
	X                  float32      `json:"x"`
	Y                  float32      `json:"y"`
	Z                  float32      `json:"z"`
	VX                 float32      `json:"vx"`
	VY                 float32      `json:"vy"`
	VZ                 float32      `json:"vz"`
	AimX               float32      `json:"aim_x"`
	AimY               float32      `json:"aim_y"`
	AimZ               float32      `json:"aim_z"`
	ZoomLevel          int8         `json:"zoom_level"`
	CrouchScale        float32      `json:"crouchscale"`
	Health             float32      `json:"health"`
	Shields            float32      `json:"shields"`
	HasCamo            bool         `json:"has_camo"`
	HasOvershield      bool         `json:"has_overshield"`
	Frags              uint8        `json:"frags"`
	Plasmas            uint8        `json:"plasmas"`
	SelectedWeaponSlot int16        `json:"selected_weapon_slot"`
	IsCrouching        bool         `json:"is_crouching"`
	IsJumping          bool         `json:"is_jumping"`
	IsFiring           bool         `json:"is_firing"`
	IsShooting         bool         `json:"is_shooting"`
	IsFlashlightOn     bool         `json:"is_flashlight_on"`
	IsThrowingGrenade  bool         `json:"is_throwing_grenade"`
	IsMeleeing         bool         `json:"is_meleeing"`
	IsPressingAction   bool         `json:"is_pressing_action"`
	IsHoldingAction    bool         `json:"is_holding_action"`
	Weapons            []WeaponInfo `json:"weapons"`
}

// WeaponInfo is one weapon slot in a player's inventory.
type WeaponInfo struct {
	Slot     int      `json:"slot"`
	ObjectID uint32   `json:"object_id"` // handle & 0xFFFF
	Tag      string   `json:"tag"`
	AmmoPack *int16   `json:"ammo_pack"` // null for energy weapons
	AmmoMag  *int16   `json:"ammo_mag"`  // null for energy weapons
	Charge   *float32 `json:"charge,omitempty"`
	IsEnergy bool     `json:"is_energy"`
}

// PowerItemStatus describes the live state of one power item spawn.
type PowerItemStatus struct {
	SpawnID        int    `json:"spawn_id"`
	Status         string `json:"status"` // "held" | "world" | "respawning"
	HeldBy         *int   `json:"held_by"`
	WorldPos       *XYZ   `json:"world_pos"`
	RespawnInTicks *int32 `json:"respawn_in_ticks"`
}

// XYZ is a 3D position.
type XYZ struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

// -------------------------------------------------------------------
// Internal (non-broadcast) types
// -------------------------------------------------------------------

// DamageTableSlots is the number of damage table entries per biped.
const DamageTableSlots = 4

// InternalPlayerState holds per-player data needed for event detection.
// It is read alongside TickPlayer but not broadcast.
type InternalPlayerState struct {
	Index           int
	WeaponSlots     [4]uint32 // raw handles; 0xFFFFFFFF = empty
	ParentObject    uint32
	MeleeRemaining  uint8
	MeleeDamageTick uint8
	ObjDataAddr     uint32 // biped object_data_addr (0 if dead)
	PrevObjDataAddr uint32 // prev biped addr (from prev_object_handle when dead)
	DamageTable     [DamageTableSlots]DamageEntry
	// Static player fields
	Kills        int16
	Deaths       int16
	Assists      int16
	TeamKills    int16
	Suicides     int16
	KillStreak   uint16
	Multikill    uint16
	ShotsFired   int32
	ShotsHit     int16
	QuitFlag     uint8
	RespawnTimer uint32
}

// DamageEntry is one slot from the biped damage table.
type DamageEntry struct {
	DamageTime      uint32 // 0xFFFFFFFF = empty
	Amount          float32
	DealerObjHandle uint32
	DealerPlrHandle uint32 // &0xFFFF = player index
}

// TickResult bundles the broadcast payload and internal state for one tick.
type TickResult struct {
	Payload         TickPayload
	InternalPlayers []InternalPlayerState
	// Cached for power item event detection
	PrevPowerItems []PowerItemStatus
}
