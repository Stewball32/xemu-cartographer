package saveartifact

import "github.com/Stewball32/xemu-cartographer/internal/halosave"

// H2ProfileRequest builds the halosave request for a Halo 2 player profile from
// a gamertag (the in-game name) and the appearance/controller byte map (keyed
// by halosave.H2ProfileFields[].Key; values 0..255). An empty/nil appearance
// produces a byte-identical clone of the template profile, just renamed.
func H2ProfileRequest(gamertag string, appearance map[string]int) halosave.BuildRequest {
	return halosave.BuildRequest{
		Title:      halosave.TitleH2,
		Kind:       halosave.KindProfile,
		Name:       gamertag,
		Appearance: appearance,
	}
}

// CEProfileSettings is the editable CE player-profile field set persisted in a
// ce_profiles record's `settings` JSON: armor color, the two control presets,
// and the nine Advanced Controls (all 2026-08-07 live-verified). Pointer fields
// are "use the fresh-profile factory default" when nil.
type CEProfileSettings struct {
	Color      *uint32 `json:"color,omitempty"`      // armor color enum (white=0, red=2, blue=3, …)
	Button     *uint32 `json:"button,omitempty"`     // 0 Default,1 Southpaw,2 Jumpy,3 Boxer,4 Green Thumb
	Thumbstick *uint32 `json:"thumbstick,omitempty"` // 0 Default,1 Southpaw,2 Legacy,3 Legacy Southpaw

	// Advanced Controls (0x2A-0x2F)
	HSens         *float64 `json:"h_sens,omitempty"`
	VMult         *float64 `json:"v_mult,omitempty"`
	Invert        *bool    `json:"invert,omitempty"`
	Vibration     *bool    `json:"vibration,omitempty"`
	RSDeadzone    *uint32  `json:"rs_deadzone,omitempty"`
	LSDeadzone    *uint32  `json:"ls_deadzone,omitempty"`
	OuterDeadzone *uint32  `json:"outer_deadzone,omitempty"`
	DeadzoneType  *uint32  `json:"deadzone_type,omitempty"`
	Response      *uint32  `json:"response,omitempty"`
}

// CEProfileRequest builds the halosave request for a Halo: CE player profile
// (blam.sav) from a gamertag (the in-game name) + the editable settings. The
// generator signs it via the per-title HMAC (digest at 0x30).
func CEProfileRequest(gamertag string, s CEProfileSettings) halosave.BuildRequest {
	return halosave.BuildRequest{
		Title:         halosave.TitleCE,
		Kind:          halosave.KindProfile,
		Name:          gamertag,
		Color:         s.Color,
		Button:        s.Button,
		Thumbstick:    s.Thumbstick,
		HSens:         s.HSens,
		VMult:         s.VMult,
		Invert:        s.Invert,
		Vibration:     s.Vibration,
		RSDeadzone:    s.RSDeadzone,
		LSDeadzone:    s.LSDeadzone,
		OuterDeadzone: s.OuterDeadzone,
		DeadzoneType:  s.DeadzoneType,
		Response:      s.Response,
	}
}

// GametypeSettings is the gametype parameter set persisted in a gametypes
// record's `settings` JSON. It holds FRIENDLY values (booleans, seconds, enum
// indices) mirroring the in-game "Edit Game Types" editor; halosave owns the
// byte/offset/scale conversion. CE and H2 share it — only the fields relevant to
// the title/engine are consumed (H2 gametype currently maps name + score limit
// only). Pointer fields are "leave at the template's value" when nil, so a
// record only carries what it actually overrides.
//
// The field set is the 2026-08-07 live-verified CE map. NOTE: this replaced the
// pre-2026-08-08 shape whose `time_minutes` actually wrote RESPAWN TIME (0x30)
// — such rows regenerate correctly on next save via the generate hook.
type GametypeSettings struct {
	Teams *bool `json:"teams,omitempty"`

	// options bitfield toggles
	Radar            *bool `json:"radar,omitempty"`
	FriendIndicators *bool `json:"friend_indicators,omitempty"`
	InfiniteGrenades *bool `json:"infinite_grenades,omitempty"`
	ShieldsOff       *bool `json:"shields_off,omitempty"`
	InvisiblePlayers *bool `json:"invisible_players,omitempty"`
	GenericEquipment *bool `json:"generic_equipment,omitempty"`

	ObjectivesIndicator  *uint32  `json:"objectives_indicator,omitempty"`
	OddManOut            *bool    `json:"odd_man_out,omitempty"`
	RespawnSeconds       *float64 `json:"respawn_seconds,omitempty"`
	RespawnGrowthSeconds *float64 `json:"respawn_growth_seconds,omitempty"`
	SuicideSeconds       *float64 `json:"suicide_seconds,omitempty"`
	Lives                *uint32  `json:"lives,omitempty"`
	MaxHealth            *float32 `json:"max_health,omitempty"`
	ScoreLimit           *uint32  `json:"score_limit,omitempty"`
	WeaponSet            *uint32  `json:"weapon_set,omitempty"`
	NHEToggles           *uint32  `json:"nhe_toggles,omitempty"`

	// engine_union rule toggles (engine-specific)
	DeathBonusOff  *bool `json:"death_bonus_off,omitempty"`
	KillPenaltyOff *bool `json:"kill_penalty_off,omitempty"`
	KillInOrder    *bool `json:"kill_in_order,omitempty"`
	Assault        *bool `json:"assault,omitempty"`
	FlagMustReset  *bool `json:"flag_must_reset,omitempty"`
	FlagAtHome     *bool `json:"flag_at_home,omitempty"`
	MovingHill     *bool `json:"moving_hill,omitempty"`
	RandomStart    *bool `json:"random_start,omitempty"`
	RaceAnyOrder   *bool `json:"race_any_order,omitempty"`

	// engine scratch
	CTFSingleFlagMinutes *float64 `json:"ctf_single_flag_minutes,omitempty"`
	OddballSpeed         *uint32  `json:"oddball_speed,omitempty"`
	OddballTraitWith     *uint32  `json:"oddball_trait_with,omitempty"`
	OddballTraitWithout  *uint32  `json:"oddball_trait_without,omitempty"`
	OddballBallType      *uint32  `json:"oddball_ball_type,omitempty"`
	BallSpawnCount       *uint32  `json:"ball_spawn_count,omitempty"`
	RaceScoring          *uint32  `json:"race_scoring,omitempty"`

	// Raw escape hatches (advanced): applied before the bool toggles.
	Options     *uint32 `json:"options,omitempty"`
	EngineUnion *uint32 `json:"engine_union,omitempty"`
}

// GametypeRequest builds the halosave request for a CE or H2 gametype variant.
// title is "ce" or "h2"; engine is the CE engine (slayer/ctf/oddball/king/race)
// or the H2 mode (slayer); name is the variant name shown in-game.
func GametypeRequest(title, engine, name string, s GametypeSettings) halosave.BuildRequest {
	return halosave.BuildRequest{
		Title:  title,
		Kind:   halosave.KindGametype,
		Name:   name,
		Engine: engine,

		Teams:            s.Teams,
		Radar:            s.Radar,
		FriendIndicators: s.FriendIndicators,
		InfiniteGrenades: s.InfiniteGrenades,
		ShieldsOff:       s.ShieldsOff,
		InvisiblePlayers: s.InvisiblePlayers,
		GenericEquipment: s.GenericEquipment,
		Options:          s.Options,

		ObjectivesIndicator:  s.ObjectivesIndicator,
		OddManOut:            s.OddManOut,
		RespawnSeconds:       s.RespawnSeconds,
		RespawnGrowthSeconds: s.RespawnGrowthSeconds,
		SuicideSeconds:       s.SuicideSeconds,
		Lives:                s.Lives,
		MaxHealth:            s.MaxHealth,
		ScoreLimit:           s.ScoreLimit,
		WeaponSet:            s.WeaponSet,
		NHEToggles:           s.NHEToggles,
		EngineUnion:          s.EngineUnion,

		DeathBonusOff:  s.DeathBonusOff,
		KillPenaltyOff: s.KillPenaltyOff,
		KillInOrder:    s.KillInOrder,
		Assault:        s.Assault,
		FlagMustReset:  s.FlagMustReset,
		FlagAtHome:     s.FlagAtHome,
		MovingHill:     s.MovingHill,
		RandomStart:    s.RandomStart,
		RaceAnyOrder:   s.RaceAnyOrder,

		CTFSingleFlagMinutes: s.CTFSingleFlagMinutes,
		OddballSpeed:         s.OddballSpeed,
		OddballTraitWith:     s.OddballTraitWith,
		OddballTraitWithout:  s.OddballTraitWithout,
		OddballBallType:      s.OddballBallType,
		BallSpawnCount:       s.BallSpawnCount,
		RaceScoring:          s.RaceScoring,
	}
}
