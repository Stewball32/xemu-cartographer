package manager

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// envelopeTypeTick is the wire type for the per-instance tick class — the
// hot path, ~30 Hz, only volatile per-frame data. Static per-tag data
// (`weapons[].tag_data`, biped tag) is referenced by string ID and lives
// in scenario.tag_defs.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`tick`).
const envelopeTypeTick = "tick"

// TickPayload is the data for a tick-class envelope.
type TickPayload struct {
	Players     []TickPlayer     `json:"players"`
	PowerItems  []TickPowerItem  `json:"power_items"`
	CTFFlags    []TickCTFFlag    `json:"ctf_flags"`
	GameGlobals *TickGameGlobals `json:"game_globals"`
	Locals      []TickLocal      `json:"locals"`
}

// TickPlayer is the per-player slice. Slimmer than the v1 TickPlayer —
// the 66-field `extended` block, `bones`, and `update_queue` moved to the
// debug class; per-weapon `tag_data` and inline `biped_tag` static data
// moved to scenario.tag_defs.
type TickPlayer struct {
	Index              int               `json:"index"`
	Alive              bool              `json:"alive"`
	RespawnInTicks     *uint32           `json:"respawn_in_ticks"`
	Pos                scraper.Vec3      `json:"pos"`
	Vel                scraper.Vec3      `json:"vel"`
	Aim                scraper.Vec3      `json:"aim"`
	ZoomLevel          int8              `json:"zoom_level"`
	CrouchScale        float32           `json:"crouch_scale"`
	Health             float32           `json:"health"`     // 0..1 engine body_vitality fraction
	Shields            float32           `json:"shields"`    // 0..1 (>1 with overshield)
	MaxHealth          float32           `json:"max_health"` // absolute ceiling (engine units, ~75)
	MaxShields         float32           `json:"max_shields"`
	HasCamo            bool              `json:"has_camo"`
	HasOvershield      bool              `json:"has_overshield"`
	Frags              uint8             `json:"frags"`
	Plasmas            uint8             `json:"plasmas"`
	SelectedWeaponSlot int16             `json:"selected_weapon_slot"`
	BipedTag           string            `json:"biped_tag"`
	Actions            TickPlayerActions `json:"actions"`
	Weapons            []TickWeapon      `json:"weapons"`
}

// TickPlayerActions collapses the v1 9-boolean soup. is_firing / is_shooting
// merged to firing; is_pressing_action / is_holding_action merged to using.
// If finer distinction is needed it lives in the debug class.
type TickPlayerActions struct {
	Crouching       bool `json:"crouching"`
	Jumping         bool `json:"jumping"`
	Firing          bool `json:"firing"`
	Flashlight      bool `json:"flashlight"`
	ThrowingGrenade bool `json:"throwing_grenade"`
	Meleeing        bool `json:"meleeing"`
	Using           bool `json:"using"`
}

// TickWeapon carries only dynamic state — static tag_data lives in
// scenario.tag_defs keyed by Tag.
type TickWeapon struct {
	Slot      int      `json:"slot"`
	Tag       string   `json:"tag"`
	ObjectID  uint32   `json:"object_id"`
	AmmoMag   *int16   `json:"ammo_mag"`
	AmmoPack  *int16   `json:"ammo_pack"`
	Charge    *float32 `json:"charge"`
	Heat      float32  `json:"heat"`
	Reloading bool     `json:"reloading"`
}

type TickPowerItem struct {
	SpawnID        int           `json:"spawn_id"`
	Status         string        `json:"status"` // "held" | "world" | "respawning"
	HeldBy         *int          `json:"held_by"`
	Pos            *scraper.Vec3 `json:"pos"`
	RespawnInTicks *int32        `json:"respawn_in_ticks"`
}

type TickCTFFlag struct {
	Team    uint32       `json:"team"`
	Pos     scraper.Vec3 `json:"pos"`
	Carrier *int         `json:"carrier"`
	Status  string       `json:"status"` // "home" | "carried" | "dropped"
}

type TickGameGlobals struct {
	MapLoaded             uint8   `json:"map_loaded"`
	Active                uint8   `json:"active"`
	GameLoadingInProgress uint8   `json:"game_loading_in_progress"`
	PrecacheMapStatus     float32 `json:"precache_map_status"`
	StoredGlobalRandom    uint32  `json:"stored_global_random"`
}

type TickLocal struct {
	LocalIndex    int                `json:"local_index"`
	FPWeapon      *TickFPWeapon      `json:"fp_weapon,omitempty"`
	ObserverCam   *TickObserverCam   `json:"observer_cam,omitempty"`
	Input         *TickLocalInput    `json:"input,omitempty"`
	PlayerControl *TickPlayerControl `json:"player_control,omitempty"`
}

type TickFPWeapon struct {
	State         int16 `json:"state"`
	AnimationID   int16 `json:"animation_id"`
	AnimationTick int16 `json:"animation_tick"`
}

type TickObserverCam struct {
	Pos scraper.Vec3 `json:"pos"`
	Aim scraper.Vec3 `json:"aim"`
	FOV float32      `json:"fov"`
}

// TickLocalInput carries the abstracted-input shape — left/right stick and
// the button bools that load-bearing UIs read. Raw gamepad state (per-
// button durations, full d-pad, etc.) lives in the debug class.
type TickLocalInput struct {
	LeftStick  scraper.Vec2          `json:"left_stick"`
	RightStick scraper.Vec2          `json:"right_stick"`
	Buttons    TickLocalInputButtons `json:"buttons"`
}

type TickLocalInputButtons struct {
	A bool `json:"a"`
	B bool `json:"b"`
}

type TickPlayerControl struct {
	DesiredYaw      float32 `json:"desired_yaw"`
	DesiredPitch    float32 `json:"desired_pitch"`
	ZoomLevel       int16   `json:"zoom_level"`
	AimAssistTarget *uint32 `json:"aim_assist_target"`
}
