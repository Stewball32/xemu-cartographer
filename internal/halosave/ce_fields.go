package halosave

// CE gametype editor SCHEMA. This describes the editable surface of a CE
// gametype in a form the UI can render itself from — one entry per setting,
// grouped into the same sections the in-game "Edit Game Types" menu uses, with
// the value ranges/enums proven by the 2026-08-07 live map. The keys match the
// JSON field names on BuildRequest / saveartifact.GametypeSettings, so the UI
// binds a form object straight to the build request.
//
// The schema is served by /api/lan/saves/meta and consumed by the creator UI;
// it is the single source of truth for "which settings exist, for which engine,
// with what values".

// CEFieldKind is the input type the UI should render for a field.
type CEFieldKind string

const (
	CEFieldBool    CEFieldKind = "bool"    // checkbox/switch -> *bool
	CEFieldEnum    CEFieldKind = "enum"    // select -> *uint32 (Options)
	CEFieldInt     CEFieldKind = "int"     // number -> *uint32
	CEFieldFloat   CEFieldKind = "float"   // number -> *float32 (e.g. max_health)
	CEFieldSeconds CEFieldKind = "seconds" // number in seconds -> *float64
	CEFieldMinutes CEFieldKind = "minutes" // number in minutes -> *float64
)

// CE editor section ids (menu-order display groups).
const (
	CESectionGame      = "game"
	CESectionPlayer    = "player"
	CESectionItem      = "item"
	CESectionIndicator = "indicator"
	CESectionRules     = "rules" // engine-specific Step-2 "Set Game Rules"
)

// CEEnumOption is one choice of an enum field.
type CEEnumOption struct {
	Value uint32 `json:"value"`
	Label string `json:"label"`
}

// CEField describes one editable gametype setting.
type CEField struct {
	Key     string         `json:"key"`               // JSON field on the build request
	Label   string         `json:"label"`             // human label (matches the in-game menu)
	Kind    CEFieldKind    `json:"kind"`              // input type
	Section string         `json:"section"`           // display group
	Engines []string       `json:"engines,omitempty"` // nil = all engines; else only these
	Options []CEEnumOption `json:"options,omitempty"` // for enum
	Min     *float64       `json:"min,omitempty"`
	Max     *float64       `json:"max,omitempty"`
	Step    *float64       `json:"step,omitempty"`
	Default *float64       `json:"default,omitempty"` // hint only; nil = keep template
	Unit    string         `json:"unit,omitempty"`
	Help    string         `json:"help,omitempty"`
}

func fptr(v float64) *float64 { return &v }

func enumOpts(pairs ...any) []CEEnumOption {
	out := make([]CEEnumOption, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, CEEnumOption{Value: uint32(pairs[i].(int)), Label: pairs[i+1].(string)})
	}
	return out
}

// CEGametypeSchema returns the full, engine-aware CE gametype field schema in
// display order. All value tables are from the live-verified byte map.
func CEGametypeSchema() []CEField {
	return []CEField{
		// ---- Game Options ----
		{Key: "teams", Label: "Team game", Kind: CEFieldBool, Section: CESectionGame,
			Help: "Team play vs free-for-all."},
		{Key: "score_limit", Label: "Score limit", Kind: CEFieldInt, Section: CESectionGame,
			Min: fptr(0), Max: fptr(999), Step: fptr(1),
			Help: "Unit depends on engine: Slayer kills, CTF captures, King/Oddball minutes, Race laps."},

		// ---- Player Options ----
		{Key: "odd_man_out", Label: "Odd man out", Kind: CEFieldBool, Section: CESectionPlayer},
		{Key: "respawn_seconds", Label: "Respawn time", Kind: CEFieldSeconds, Section: CESectionPlayer,
			Min: fptr(0), Max: fptr(30), Step: fptr(0.5), Unit: "s", Help: "0 = instant."},
		{Key: "respawn_growth_seconds", Label: "Respawn time growth", Kind: CEFieldSeconds, Section: CESectionPlayer,
			Min: fptr(0), Max: fptr(30), Step: fptr(0.5), Unit: "s"},
		{Key: "suicide_seconds", Label: "Suicide penalty", Kind: CEFieldSeconds, Section: CESectionPlayer,
			Min: fptr(0), Max: fptr(30), Step: fptr(0.5), Unit: "s"},
		{Key: "lives", Label: "Number of lives", Kind: CEFieldInt, Section: CESectionPlayer,
			Min: fptr(0), Max: fptr(99), Step: fptr(1), Help: "0 = infinite."},
		{Key: "max_health", Label: "Maximum health", Kind: CEFieldFloat, Section: CESectionPlayer,
			Min: fptr(0.5), Max: fptr(4), Step: fptr(0.5), Unit: "×", Default: fptr(1),
			Help: "Health multiplier (1.0 = 100%)."},
		{Key: "shields_off", Label: "Shields off", Kind: CEFieldBool, Section: CESectionPlayer},
		{Key: "invisible_players", Label: "Invisible players", Kind: CEFieldBool, Section: CESectionPlayer},

		// ---- Item Options ----
		{Key: "weapon_set", Label: "Weapon set", Kind: CEFieldEnum, Section: CESectionItem,
			Options: enumOpts(0, "Normal", 1, "Pistols", 2, "Rifles", 3, "Plasma Weapons",
				4, "Sniper", 5, "No Sniping", 6, "Rocket Launchers")},
		{Key: "nhe_toggles", Label: "NHE toggles", Kind: CEFieldEnum, Section: CESectionItem,
			Options: enumOpts(0, "Training", 1, "Vanilla", 2, "NHE & Timer", 3, "Timer Only", 4, "NHE & Powerups")},
		{Key: "infinite_grenades", Label: "Infinite grenades", Kind: CEFieldBool, Section: CESectionItem},
		{Key: "generic_equipment", Label: "Generic starting equipment", Kind: CEFieldBool, Section: CESectionItem,
			Help: "On = generic, off = custom."},

		// ---- Indicator Options ----
		{Key: "radar", Label: "Other players on radar", Kind: CEFieldBool, Section: CESectionIndicator,
			Help: `The "R" suffix.`},
		{Key: "friend_indicators", Label: "Friend indicators on screen", Kind: CEFieldBool, Section: CESectionIndicator},
		{Key: "objectives_indicator", Label: "Objectives indicator", Kind: CEFieldEnum, Section: CESectionIndicator,
			Options: enumOpts(0, "Motion Tracker", 1, "Nav Points", 2, "None")},

		// ---- Engine rules (Step 2) ----
		// Slayer
		{Key: "death_bonus_off", Label: "Death bonus off", Kind: CEFieldBool, Section: CESectionRules, Engines: []string{"slayer"}},
		{Key: "kill_penalty_off", Label: "Kill penalty off", Kind: CEFieldBool, Section: CESectionRules, Engines: []string{"slayer"}},
		{Key: "kill_in_order", Label: "Kill in order", Kind: CEFieldBool, Section: CESectionRules, Engines: []string{"slayer"}},
		// CTF
		{Key: "assault", Label: "Assault", Kind: CEFieldBool, Section: CESectionRules, Engines: []string{"ctf"}},
		{Key: "flag_must_reset", Label: "Flag must reset", Kind: CEFieldBool, Section: CESectionRules, Engines: []string{"ctf"}},
		{Key: "flag_at_home", Label: "Flag at home to score", Kind: CEFieldBool, Section: CESectionRules, Engines: []string{"ctf"}},
		{Key: "ctf_single_flag_minutes", Label: "Single flag", Kind: CEFieldMinutes, Section: CESectionRules, Engines: []string{"ctf"},
			Min: fptr(0), Max: fptr(10), Step: fptr(1), Unit: "min", Help: "0 = off (period timer)."},
		// King
		{Key: "moving_hill", Label: "Moving hill", Kind: CEFieldBool, Section: CESectionRules, Engines: []string{"king"}},
		// Oddball
		{Key: "random_start", Label: "Random start", Kind: CEFieldBool, Section: CESectionRules, Engines: []string{"oddball"}},
		{Key: "oddball_speed", Label: "Speed with ball", Kind: CEFieldEnum, Section: CESectionRules, Engines: []string{"oddball"},
			Options: enumOpts(0, "Slow", 1, "Normal", 2, "Fast")},
		{Key: "oddball_trait_with", Label: "Trait with ball", Kind: CEFieldEnum, Section: CESectionRules, Engines: []string{"oddball"},
			Options: enumOpts(0, "None", 1, "Invisible", 2, "Extra Damage", 3, "Damage Resistant")},
		{Key: "oddball_trait_without", Label: "Trait without ball", Kind: CEFieldEnum, Section: CESectionRules, Engines: []string{"oddball"},
			Options: enumOpts(0, "None", 1, "Invisible", 2, "Extra Damage", 3, "Damage Resistant")},
		{Key: "oddball_ball_type", Label: "Ball type", Kind: CEFieldEnum, Section: CESectionRules, Engines: []string{"oddball"},
			Options: enumOpts(0, "Normal", 1, "Reverse Tag")},
		{Key: "ball_spawn_count", Label: "Ball spawn count", Kind: CEFieldInt, Section: CESectionRules, Engines: []string{"oddball"},
			Min: fptr(1), Max: fptr(8), Step: fptr(1), Default: fptr(1)},
		// Race
		{Key: "race_any_order", Label: "Any order", Kind: CEFieldBool, Section: CESectionRules, Engines: []string{"race"}},
		{Key: "race_scoring", Label: "Team scoring", Kind: CEFieldEnum, Section: CESectionRules, Engines: []string{"race"},
			Options: enumOpts(0, "Minimum", 1, "Maximum")},
	}
}

// CESection pairs a section id with its display label.
type CESection struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// CESections returns the editor sections in display order.
func CESections() []CESection {
	return []CESection{
		{CESectionGame, "Game Options"},
		{CESectionPlayer, "Player Options"},
		{CESectionItem, "Item Options"},
		{CESectionIndicator, "Indicator Options"},
		{CESectionRules, "Game Rules"},
	}
}
