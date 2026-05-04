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

// StateInputs are the raw values sampled by a plugin's ReadGameState that drive
// its GameState classification. Surfaced via the inspect endpoint as a debug
// aid — lets the debug page show why the scraper thinks the game is in the
// state it's in (e.g. "main_menu=1, initialized=0 → menu"). Field names are
// plugin-specific; consumers treat the map as opaque JSON.
type StateInputs map[string]any

// ScoreProbe is a free-form bag of every candidate address a plugin knows
// about for gametype detection, team scores, score limits, and per-player
// scores. Surfaced via the inspect endpoint and rendered as the debug page's
// "Probe" tab so a human can spot which raw value matches what they see in-
// game when authoritative offsets are still being worked out.
type ScoreProbe map[string]any

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

	// Roster + team-score change events (Part D of the caching refactor).
	// Emitted only when the relevant SnapshotPayload field diff vs the
	// previous tick is non-zero.
	EventTeamScore         = "team_score"
	EventPlayerJoined      = "player_joined"
	EventPlayerLeft        = "player_left"
	EventPlayerTeamChanged = "player_team_changed"
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
	VariantName     string           `json:"variant_name,omitempty"`
	IsTeamGame      bool             `json:"is_team_game"`
	ScoreLimit      int32            `json:"score_limit"`
	TimeLimitTicks  int32            `json:"time_limit_ticks"`
	TeamScores      []TeamScore      `json:"team_scores"`
	Players         []SnapshotPlayer `json:"players"`
	PowerItemSpawns []PowerItemSpawn `json:"power_item_spawns"`

	// Machines is the connected-machine roster for system-link / splitscreen
	// lobbies. Empty when no network game is active. Each entry's Index
	// matches SnapshotPlayer.MachineIndex.
	Machines []SnapshotMachine `json:"machines,omitempty"`

	// Static map / scenario data scraped at match-start.
	GameDifficulty uint8               `json:"game_difficulty"`
	PlayerSpawns   []StaticPlayerSpawn `json:"player_spawns,omitempty"`
	Fog            *StaticFog          `json:"fog,omitempty"`
	ObjectTypes    []StaticObjectType  `json:"object_types,omitempty"`
	TagCache       *StaticCachePtrs    `json:"tag_cache,omitempty"`
}

// StaticPlayerSpawn describes one scenario player-spawn point. Static for the
// lifetime of the loaded scenario. Source: OffPlayerSpawn* constants.
type StaticPlayerSpawn struct {
	Index     int     `json:"index"`
	X         float32 `json:"x"`
	Y         float32 `json:"y"`
	Z         float32 `json:"z"`
	Facing    float32 `json:"facing"`
	TeamIndex uint8   `json:"team_index"`
	BspIndex  uint8   `json:"bsp_index"`
	Unk0      uint16  `json:"unk_0"`
	Gametype0 uint8   `json:"gametype_0"`
	Gametype1 uint8   `json:"gametype_1"`
	Gametype2 uint8   `json:"gametype_2"`
	Gametype3 uint8   `json:"gametype_3"`
}

// StaticFog mirrors the per-map fog parameters. Source: OffFog* constants.
type StaticFog struct {
	ColorR      float32 `json:"color_r"`
	ColorG      float32 `json:"color_g"`
	ColorB      float32 `json:"color_b"`
	MaxDensity  float32 `json:"max_density"`
	AtmoMinDist float32 `json:"atmo_min_dist"`
	AtmoMaxDist float32 `json:"atmo_max_dist"`
}

// StaticObjectType is one entry from the engine's object-type-definition
// array, walked once per session. Source: OffObjTypeDef* constants.
type StaticObjectType struct {
	TypeIndex int    `json:"type_index"`
	Name      string `json:"name"`
	DatumSize uint16 `json:"datum_size"`
}

// StaticCachePtrs is the diagnostic cache-pointer triplet read once at init.
// Useful for memory-layout introspection in the debug page.
type StaticCachePtrs struct {
	GameStateBase    uint32 `json:"game_state_base"`
	GameStateSize    uint32 `json:"game_state_size"`
	TagCacheBase     uint32 `json:"tag_cache_base"`
	TagCacheSize     uint32 `json:"tag_cache_size"`
	TextureCacheBase uint32 `json:"texture_cache_base"`
	TextureCacheSize uint32 `json:"texture_cache_size"`
	SoundCacheBase   uint32 `json:"sound_cache_base"`
	SoundCacheSize   uint32 `json:"sound_cache_size"`
}

// SnapshotPlayer is the static/score portion of a player.
//
// IsLocal/LocalIndex report whether a player is local to this xemu instance
// (vs remote via system-link) and, for locals, the splitscreen slot (0–3).
// Pointer types so games without local detection serialise them as null.
type SnapshotPlayer struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Team  uint32 `json:"team"`
	// Score is the per-player gametype score: ctf_score for CTF, the per-
	// player slot of the matching score table for Slayer/Oddball/King/Race
	// (these all live in distinct memory bases — see haloce/offsets.go
	// AddrScore*). Populated by the plugin from the active gametype.
	Score      int32  `json:"score"`
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
	// MachineIndex is the network-machine index this player is hosted on.
	// Populated from the lobby roster (system-link / splitscreen) and lines
	// up with SnapshotPayload.Machines[].Index. Nil when machine attribution
	// isn't available (e.g. in-engine PlayerDatumArray reads pre-network).
	MachineIndex *int `json:"machine_index"`
}

// SnapshotMachine is one connected machine in a system-link lobby. Index is
// the per-machine slot the network stack uses to address the host.
type SnapshotMachine struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
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

// TickPayload is the per-game-tick broadcast (30Hz when in_game). Carries
// only high-frequency volatile data; cumulative counters and roster identity
// live on SnapshotPayload and are emitted on change via events.
type TickPayload struct {
	Players     []TickPlayer      `json:"players"`
	PowerItems  []PowerItemStatus `json:"power_items"`
	GameGlobals *TickGameGlobals  `json:"game_globals,omitempty"`
	PlayerCount int16             `json:"player_count"`
	LocalCount  uint16            `json:"local_count"`
	Locals      []TickLocal       `json:"locals,omitempty"`
	Network     *TickNetwork      `json:"network,omitempty"`
	DataQueue   *TickDataQueue    `json:"data_queue,omitempty"`
	CTFFlags    []TickCTFFlag     `json:"ctf_flags,omitempty"`
	Objects     []TickObject      `json:"objects,omitempty"`
	Projectiles []TickProjectile  `json:"projectiles,omitempty"`
}

// TickGameGlobals mirrors the per-tick game_globals struct
// (at *AddrGameGlobalsPtr). Source: OffGG* constants.
type TickGameGlobals struct {
	MapLoaded             uint8   `json:"map_loaded"`
	Active                uint8   `json:"active"`
	PlayersAreDoubleSpeed uint8   `json:"players_are_double_speed"`
	GameLoadingInProgress uint8   `json:"game_loading_in_progress"`
	PrecacheMapStatus     float32 `json:"precache_map_status"`
	GameDifficultyLevel   uint8   `json:"game_difficulty_level"`
	StoredGlobalRandom    uint32  `json:"stored_global_random"`
}

// TickLocal aggregates the per-local-player subsystems (FPW, observer cam,
// input abstraction, UI globals, player control, look rates).
// Indexed independently from TickPlayer.LocalIndex. UpdateQueue lives on
// TickPlayer because the engine's update_queue tracks every player slot
// (locals AND remotes), not just locals.
type TickLocal struct {
	LocalIndex    int                `json:"local_index"`
	FPWeapon      *TickFPWeapon      `json:"fp_weapon,omitempty"`
	ObserverCam   *TickObserverCam   `json:"observer_cam,omitempty"`
	IAS           *TickInputAbstract `json:"ias,omitempty"`
	Gamepad       *TickGamepad       `json:"gamepad,omitempty"`
	UI            *TickUIGlobals     `json:"ui,omitempty"`
	PlayerControl *TickPlayerControl `json:"player_control,omitempty"`
	LookYawRate   float32            `json:"look_yaw_rate"`
	LookPitchRate float32            `json:"look_pitch_rate"`
}

// TickFPWeapon mirrors the first-person weapon record
// (at *RefAddrFPWeaponPtr + 7840*local). Source: OffFPW* constants.
type TickFPWeapon struct {
	WeaponRendered         uint32 `json:"weapon_rendered"`
	PlayerObject           uint32 `json:"player_object"`
	WeaponObject           uint32 `json:"weapon_object"`
	State                  int16  `json:"state"`
	IdleAnimationThreshold int16  `json:"idle_animation_threshold"`
	IdleAnimationCounter   int16  `json:"idle_animation_counter"`
	AnimationID            int16  `json:"animation_id"`
	AnimationTick          int16  `json:"animation_tick"`
}

// TickObserverCam mirrors the observer-camera record
// (at RefAddrObserverCameraBase + 668*local). Source: OffObsCam* constants.
type TickObserverCam struct {
	X    float32 `json:"x"`
	Y    float32 `json:"y"`
	Z    float32 `json:"z"`
	VelX float32 `json:"vel_x"`
	VelY float32 `json:"vel_y"`
	VelZ float32 `json:"vel_z"`
	AimX float32 `json:"aim_x"`
	AimY float32 `json:"aim_y"`
	AimZ float32 `json:"aim_z"`
	FOV  float32 `json:"fov"`
}

// TickInputAbstract mirrors the post-button-config input state
// (at RefAddrInputAbstractInputState + 0x1C*local). Source: OffIAS* constants.
type TickInputAbstract struct {
	BtnA                 uint8   `json:"btn_a"`
	BtnBlack             uint8   `json:"btn_black"`
	BtnX                 uint8   `json:"btn_x"`
	BtnY                 uint8   `json:"btn_y"`
	BtnB                 uint8   `json:"btn_b"`
	BtnWhite             uint8   `json:"btn_white"`
	LeftTrigger          uint8   `json:"left_trigger"`
	RightTrigger         uint8   `json:"right_trigger"`
	BtnStart             uint8   `json:"btn_start"`
	BtnBack              uint8   `json:"btn_back"`
	LeftStickButton      uint8   `json:"left_stick_button"`
	RightStickButton     uint8   `json:"right_stick_button"`
	LeftStickVertical    float32 `json:"left_stick_vertical"`
	LeftStickHorizontal  float32 `json:"left_stick_horizontal"`
	RightStickHorizontal float32 `json:"right_stick_horizontal"`
	RightStickVertical   float32 `json:"right_stick_vertical"`
}

// TickGamepad mirrors the raw-controller gamepad record
// (at RefAddrGamepadState + 0x28*player). Source: OffGP* constants.
type TickGamepad struct {
	BtnA                 uint8 `json:"btn_a"`
	BtnB                 uint8 `json:"btn_b"`
	BtnX                 uint8 `json:"btn_x"`
	BtnY                 uint8 `json:"btn_y"`
	BtnBlack             uint8 `json:"btn_black"`
	BtnWhite             uint8 `json:"btn_white"`
	LeftTrigger          uint8 `json:"left_trigger"`
	RightTrigger         uint8 `json:"right_trigger"`
	BtnADuration         uint8 `json:"btn_a_duration"`
	BtnBDuration         uint8 `json:"btn_b_duration"`
	BtnXDuration         uint8 `json:"btn_x_duration"`
	BtnYDuration         uint8 `json:"btn_y_duration"`
	BlackDuration        uint8 `json:"black_duration"`
	WhiteDuration        uint8 `json:"white_duration"`
	LTDuration           uint8 `json:"lt_duration"`
	RTDuration           uint8 `json:"rt_duration"`
	DpadUpDuration       uint8 `json:"dpad_up_duration"`
	DpadDownDuration     uint8 `json:"dpad_down_duration"`
	DpadLeftDuration     uint8 `json:"dpad_left_duration"`
	DpadRightDuration    uint8 `json:"dpad_right_duration"`
	LeftStickDuration    uint8 `json:"left_stick_duration"`
	RightStickDuration   uint8 `json:"right_stick_duration"`
	LeftStickHorizontal  int16 `json:"left_stick_horizontal"`
	LeftStickVertical    int16 `json:"left_stick_vertical"`
	RightStickHorizontal int16 `json:"right_stick_horizontal"`
	RightStickVertical   int16 `json:"right_stick_vertical"`
}

// TickUIGlobals mirrors per-local-player UI/profile config
// (at RefAddrPerLocalUIGlobals + 56*local). Source: OffUI* constants.
type TickUIGlobals struct {
	Color                    uint8  `json:"color"`
	ButtonConfig             uint8  `json:"button_config"`
	JoystickConfig           uint8  `json:"joystick_config"`
	Sensitivity              uint8  `json:"sensitivity"`
	JoystickInverted         uint8  `json:"joystick_inverted"`
	RumbleEnabled            uint8  `json:"rumble_enabled"`
	FlightInverted           uint8  `json:"flight_inverted"`
	AutocenterEnabled        uint8  `json:"autocenter_enabled"`
	ActivePlayerProfileIndex uint32 `json:"active_player_profile_index"`
	JoinedMultiplayerGame    uint8  `json:"joined_multiplayer_game"`
}

// TickPlayerControl mirrors the player_control struct
// (at *RefAddrPlayerControlPtr + (local << 6)). Source: OffPC* constants.
type TickPlayerControl struct {
	DesiredYaw      float32 `json:"desired_yaw"`
	DesiredPitch    float32 `json:"desired_pitch"`
	ZoomLevel       int16   `json:"zoom_level"`
	AimAssistTarget uint32  `json:"aim_assist_target"`
	AimAssistNear   float32 `json:"aim_assist_near"`
	AimAssistFar    float32 `json:"aim_assist_far"`
}

// TickUpdateQueue mirrors a per-player slot in the update-queue array
// (within update_client_player + 0x28*player). Source: OffUQ* constants.
type TickUpdateQueue struct {
	UnitRef          uint16             `json:"unit_ref"`
	ButtonField      uint8              `json:"button_field"`
	ActionField      uint8              `json:"action_field"`
	Buttons          UpdateQueueButtons `json:"buttons"`
	Actions          UpdateQueueActions `json:"actions"`
	DesiredYaw       float32            `json:"desired_yaw"`
	DesiredPitch     float32            `json:"desired_pitch"`
	Forward          float32            `json:"forward"`
	Left             float32            `json:"left"`
	RightTriggerHeld float32            `json:"right_trigger_held"`
	DesiredWeapon    uint16             `json:"desired_weapon"`
	DesiredGrenades  uint16             `json:"desired_grenades"`
	ZoomLevel        int16              `json:"zoom_level"`
}

// UpdateQueueButtons decodes the OffUQButtonField bitfield.
type UpdateQueueButtons struct {
	Crouch     bool `json:"crouch"`
	Jump       bool `json:"jump"`
	Fire       bool `json:"fire"`
	Flashlight bool `json:"flashlight"`
	Reload     bool `json:"reload"`
	Melee      bool `json:"melee"`
}

// UpdateQueueActions decodes the OffUQActionField bitfield.
type UpdateQueueActions struct {
	ThrowGrenade bool `json:"throw_grenade"`
	UseAction    bool `json:"use_action"`
}

// TickNetwork bundles per-tick network-game state.
type TickNetwork struct {
	Client         *TickNetworkClient   `json:"client,omitempty"`
	Server         *TickNetworkServer   `json:"server,omitempty"`
	GameData       *TickNetworkGameData `json:"game_data,omitempty"`
	Machines       []TickNetMachine     `json:"machines,omitempty"`
	NetworkPlayers []TickNetPlayer      `json:"network_players,omitempty"`
}

// TickNetworkClient mirrors network_game_client (at RefAddrNetworkGameClient).
// Source: OffNGC* constants.
type TickNetworkClient struct {
	MachineIndex       uint16 `json:"machine_index"`
	PingTargetIP       int32  `json:"ping_target_ip"`
	PacketsSent        int16  `json:"packets_sent"`
	PacketsReceived    int16  `json:"packets_received"`
	AveragePing        int16  `json:"average_ping"`
	PingActive         uint8  `json:"ping_active"`
	SecondsToGameStart int16  `json:"seconds_to_game_start"`
}

// TickNetworkServer mirrors network_game_server (at RefAddrNetworkGameServer).
// Source: OffNGS* constants.
type TickNetworkServer struct {
	CountdownActive       uint8 `json:"countdown_active"`
	CountdownPaused       uint8 `json:"countdown_paused"`
	CountdownAdjustedTime uint8 `json:"countdown_adjusted_time"`
}

// TickNetworkGameData mirrors the inline network_game_data sub-struct
// (at network_game_client + 2140). Source: OffNGD* constants.
type TickNetworkGameData struct {
	MaximumPlayerCount uint8 `json:"maximum_player_count"`
	MachineCount       int16 `json:"machine_count"`
	PlayerCount        int16 `json:"player_count"`
}

// TickNetMachine mirrors one entry in network_game_data.NetworkMachines.
// Source: OffNetMachine* constants.
type TickNetMachine struct {
	Index uint8  `json:"index"`
	Name  string `json:"name"`
}

// TickNetPlayer mirrors one entry in network_game_data.NetworkPlayers.
// Source: OffNetPlayer* constants.
type TickNetPlayer struct {
	Name            string `json:"name"`
	Color           int16  `json:"color"`
	Unused          int16  `json:"unused"`
	MachineIndex    uint8  `json:"machine_index"`
	ControllerIndex uint8  `json:"controller_index"`
	Team            uint8  `json:"team"`
	ListIndex       uint8  `json:"list_index"`
}

// TickDataQueue mirrors the data-queue header
// (at RefAddrUpdateQueueCounterLo dereferenced). Source: OffDataQueue* constants.
type TickDataQueue struct {
	Tick         int32  `json:"tick"`
	GlobalRandom uint32 `json:"global_random"`
	Tick2        int32  `json:"tick2"`
	Unk1         uint16 `json:"unk1"`
	PlayerCount  int16  `json:"player_count"`
}

// TickCTFFlag describes one CTF flag's live position and carrier (if any).
// Status semantics: "home" (on flagpole), "carried" (in a player's slot),
// "dropped" (in world but not held).
type TickCTFFlag struct {
	Team    uint32  `json:"team"`
	X       float32 `json:"x"`
	Y       float32 `json:"y"`
	Z       float32 `json:"z"`
	Carrier *int    `json:"carrier_index"`
	Status  string  `json:"status"`
}

// TickObject is the per-object generic-data view emitted from the world-object
// scan. One entry per non-garbage object in the OHD array each tick. Source:
// OffObj* constants.
type TickObject struct {
	ObjectID       uint32  `json:"object_id"`
	Tag            string  `json:"tag"`
	Type           uint8   `json:"type"`
	Flags          uint32  `json:"flags"`
	X              float32 `json:"x"`
	Y              float32 `json:"y"`
	Z              float32 `json:"z"`
	AngVelX        float32 `json:"ang_vel_x"`
	AngVelY        float32 `json:"ang_vel_y"`
	AngVelZ        float32 `json:"ang_vel_z"`
	UnkDamage1     int16   `json:"unk_damage_1"`
	TimeExisting   int16   `json:"time_existing"`
	OwnerUnitRef   uint32  `json:"owner_unit_ref"`
	OwnerObjectRef uint32  `json:"owner_object_ref"`
	UltimateParent uint32  `json:"ultimate_parent"`
}

// TickProjectile mirrors the projectile sub-struct
// (at object_address + RefAddrItemDatumSize). Source: OffProj* constants.
type TickProjectile struct {
	ObjectID               uint32  `json:"object_id"`
	Tag                    string  `json:"tag"`
	X                      float32 `json:"x"`
	Y                      float32 `json:"y"`
	Z                      float32 `json:"z"`
	Flags                  uint32  `json:"flags"`
	Action                 int16   `json:"action"`
	HitMaterialType        int16   `json:"hit_material_type"`
	IgnoreObjectIndex      int32   `json:"ignore_object_index"`
	DetonationTimer        float32 `json:"detonation_timer"`
	DetonationTimerDelta   float32 `json:"detonation_timer_delta"`
	TargetObjectIndex      int32   `json:"target_object_index"` // OR arming_time f32; HC bug
	ArmingTimeDelta        float32 `json:"arming_time_delta"`
	DistanceTraveled       float32 `json:"distance_traveled"`
	DecelerationTimer      float32 `json:"deceleration_timer"`
	DecelerationTimerDelta float32 `json:"deceleration_timer_delta"`
	Deceleration           float32 `json:"deceleration"`
	MaximumDamageDistance  float32 `json:"maximum_damage_distance"`
	RotationAxisX          float32 `json:"rotation_axis_x"`
	RotationAxisY          float32 `json:"rotation_axis_y"`
	RotationAxisZ          float32 `json:"rotation_axis_z"`
	RotationSine           float32 `json:"rotation_sine"`
	RotationCosine         float32 `json:"rotation_cosine"`
}

// TickPlayer is the per-player slice of one TickPayload — only high-frequency
// volatile data. Roster identity (name, team, splitscreen index) and
// cumulative counters (kills/deaths/assists/etc.) live on SnapshotPlayer; they
// are emitted on change via events rather than streamed every tick.
type TickPlayer struct {
	Index int `json:"index"`

	// Dynamic per-tick state.
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

	// Extended scraped state (populated when readDynPlayerExtended runs).
	Extended *TickPlayerExtended `json:"extended,omitempty"`

	// 19 model-node bone positions (see ModelNodeBoneOffsets). Populated when
	// the bones subsystem is enabled.
	Bones []TickBone `json:"bones,omitempty"`

	// Update-queue slot. Tracks input replication state per player index.
	UpdateQueue *TickUpdateQueue `json:"update_queue,omitempty"`

	// Static biped tag data (cached per tag index; populated when alive).
	BipedTag *StaticBipedTagData `json:"biped_tag,omitempty"`
}

// TickBone is one model-node bone position read from the dynamic biped.
// Index aligns with ModelNodeBoneOffsets ordering (0..18).
type TickBone struct {
	Index int     `json:"index"`
	X     float32 `json:"x"`
	Y     float32 `json:"y"`
	Z     float32 `json:"z"`
}

// TickPlayerExtended captures the ~50 dynamic-biped diagnostic fields (legs,
// aim vectors, animation, damage countdowns, airborne state, etc.) that aren't
// load-bearing for the existing overlay but are scraped for completeness.
// Source: dynamic biped extended OffDyn* constants in offsets.go.
type TickPlayerExtended struct {
	// Leg / facing rotations
	LegsPitch float32 `json:"legs_pitch"`
	LegsYaw   float32 `json:"legs_yaw"`
	LegsRoll  float32 `json:"legs_roll"`
	Pitch1    float32 `json:"pitch_1"`
	Yaw1      float32 `json:"yaw_1"`
	Roll1     float32 `json:"roll_1"`

	// Angular velocity
	AngVelX float32 `json:"ang_vel_x"`
	AngVelY float32 `json:"ang_vel_y"`
	AngVelZ float32 `json:"ang_vel_z"`

	// Aim-assist sphere
	AimAssistSphereX      float32 `json:"aim_assist_sphere_x"`
	AimAssistSphereY      float32 `json:"aim_assist_sphere_y"`
	AimAssistSphereZ      float32 `json:"aim_assist_sphere_z"`
	AimAssistSphereRadius float32 `json:"aim_assist_sphere_radius"`

	// Object scale + sub-type
	Scale           float32 `json:"scale"`
	TypeU16         uint16  `json:"type_u16"`
	RenderFlags     uint16  `json:"render_flags"`
	WeaponOwnerTeam int16   `json:"weapon_owner_team"`
	PowerupUnk2     int16   `json:"powerup_unk_2"`
	IdleTicks       int16   `json:"idle_ticks"`

	// Animation handle / id / tick
	AnimationUnk1 uint32 `json:"animation_unk_1"`
	AnimationUnk2 int16  `json:"animation_unk_2"`
	AnimationUnk3 int16  `json:"animation_unk_3"`

	// Damage countdowns
	DmgCountdown98 float32 `json:"dmg_countdown_98"`
	DmgCountdown9C float32 `json:"dmg_countdown_9c"`
	DmgCountdownA4 float32 `json:"dmg_countdown_a4"`
	DmgCountdownA8 float32 `json:"dmg_countdown_a8"`
	DmgCounterAC   int32   `json:"dmg_counter_ac"`
	DmgCounterB0   int32   `json:"dmg_counter_b0"`

	// Shields (extended)
	ShieldsStatus2     uint16 `json:"shields_status_2"`
	ShieldsChargeDelay uint16 `json:"shields_charge_delay"`

	// Object-table linkage
	NextObject  int32  `json:"next_object"`
	NextObject2 uint32 `json:"next_object_2"`

	// State / flashlight / stunned
	StateFlags uint8   `json:"state_flags"`
	Flashlight uint8   `json:"flashlight"`
	Stunned    float32 `json:"stunned"`

	// Aim / look unit vectors
	Xunk0          float32 `json:"x_unk_0"`
	Yunk0          float32 `json:"y_unk_0"`
	Zunk0          float32 `json:"z_unk_0"`
	XAimA          float32 `json:"x_aim_a"`
	YAimA          float32 `json:"y_aim_a"`
	ZAimA          float32 `json:"z_aim_a"`
	XAim0          float32 `json:"x_aim_0"`
	YAim0          float32 `json:"y_aim_0"`
	ZAim0          float32 `json:"z_aim_0"`
	XAim1          float32 `json:"x_aim_1"`
	YAim1          float32 `json:"y_aim_1"`
	ZAim1          float32 `json:"z_aim_1"`
	LookingVectorX float32 `json:"looking_vector_x"`
	LookingVectorY float32 `json:"looking_vector_y"`
	LookingVectorZ float32 `json:"looking_vector_z"`

	// Movement throttles
	MoveForward float32 `json:"move_forward"`
	MoveLeft    float32 `json:"move_left"`
	MoveUp      float32 `json:"move_up"`

	// Melee + animation tags
	MeleeDamageType uint8 `json:"melee_damage_type"`
	Animation1      uint8 `json:"animation_1"`
	Animation2      uint8 `json:"animation_2"`

	// Equipment / camo extended
	CurrentEquipment uint32 `json:"current_equipment"`
	CamoSelfRevealed uint16 `json:"camo_self_revealed"`

	// Facing vectors
	Facing1 float32 `json:"facing_1"`
	Facing2 float32 `json:"facing_2"`
	Facing3 float32 `json:"facing_3"`

	// Air / landing
	Airborne                   uint8  `json:"airborne"`
	LandingStunCurrentDuration uint8  `json:"landing_stun_current_duration"`
	LandingStunTargetDuration  uint8  `json:"landing_stun_target_duration"`
	AirborneTicks              uint8  `json:"airborne_ticks"`
	SlippingTicks              uint8  `json:"slipping_ticks"`
	StopTicks                  uint8  `json:"stop_ticks"`
	JumpRecoveryTimer          uint8  `json:"jump_recovery_timer"`
	Landing                    uint16 `json:"landing"`
	AirState460                int16  `json:"air_state_460"`
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

	// Live weapon-object state (heat, reload, world position when dropped).
	Extended *TickWeaponObjectExtended `json:"extended,omitempty"`

	// Static weapon-tag-data (zoom levels, autoaim/magnetism params).
	// Populated once per tag and reused across ticks.
	TagData *StaticWeaponTagData `json:"tag_data,omitempty"`
}

// TickWeaponObjectExtended is the per-tick weapon-object live state. World
// position is only meaningful when the weapon is dropped (no owner).
// Source: OffWep* constants.
type TickWeaponObjectExtended struct {
	HeatMeter   float32 `json:"heat_meter"`
	UsedEnergy  float32 `json:"used_energy"`
	OwnerHandle uint32  `json:"owner_handle"`
	IsReloading uint8   `json:"is_reloading"`
	ReloadTime  int16   `json:"reload_time"`
	CanFire     uint8   `json:"can_fire"`
	World       *XYZ    `json:"world,omitempty"`
}

// StaticWeaponTagData is the static weapon-tag metadata cached per tag.
// Source: OffWepTag* + OffAnim* constants.
type StaticWeaponTagData struct {
	ZoomLevels     int16   `json:"zoom_levels"`
	ZoomMin        float32 `json:"zoom_min"`
	ZoomMax        float32 `json:"zoom_max"`
	AutoaimAngle   float32 `json:"autoaim_angle"`
	AutoaimRange   float32 `json:"autoaim_range"`
	MagnetismAngle float32 `json:"magnetism_angle"`
	MagnetismRange float32 `json:"magnetism_range"`
	DeviationAngle float32 `json:"deviation_angle"`

	// Animation entries walked from the tag's animation array. Capped at
	// AnimEntriesScanCap entries; stops early on invalid/empty entries.
	Animations []AnimEntry `json:"animations,omitempty"`
}

// AnimEntry mirrors one animation tag-data entry (stride 180 bytes).
// Source: OffAnim* constants. Index is the entry's position in the array.
type AnimEntry struct {
	Index  int   `json:"index"`
	Length int16 `json:"length"`
	Unk46  int16 `json:"unk_46"`
	Unk52  int16 `json:"unk_52"`
	Unk54  int16 `json:"unk_54"`
}

// StaticBipedTagData is the static biped-tag metadata cached per tag.
// Source: OffBipedTag* constants.
type StaticBipedTagData struct {
	TagIndex          int16   `json:"tag_index"`
	Flags             uint32  `json:"flags"`
	AutoaimPillRadius float32 `json:"autoaim_pill_radius"`
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
