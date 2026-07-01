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
// ce_profiles record's `settings` JSON: armor color enum + the two control
// presets. (The advanced 0x1C-0x2F bytes are a pluggable follow-up.) Pointer
// fields are "use the fresh-profile default" when nil.
type CEProfileSettings struct {
	Color      *int `json:"color,omitempty"`      // armor color enum (white=0, red=2, blue=3, …)
	Thumbstick *int `json:"thumbstick,omitempty"` // 0=Default, 1=Southpaw
	Button     *int `json:"button,omitempty"`     // 0=Default, 1=Southpaw
}

// CEProfileRequest builds the halosave request for a Halo: CE player profile
// (blam.sav) from a gamertag (the in-game name) + the editable settings. The
// generator signs it via the per-title HMAC (digest at 0x30).
func CEProfileRequest(gamertag string, s CEProfileSettings) halosave.BuildRequest {
	ap := map[string]int{}
	if s.Color != nil {
		ap["color"] = *s.Color
	}
	if s.Thumbstick != nil {
		ap["thumbstick"] = *s.Thumbstick
	}
	if s.Button != nil {
		ap["button"] = *s.Button
	}
	return halosave.BuildRequest{
		Title:      halosave.TitleCE,
		Kind:       halosave.KindProfile,
		Name:       gamertag,
		Appearance: ap,
	}
}

// GametypeSettings is the flat gametype parameter set persisted in a gametypes
// record's `settings` JSON. CE and H2 share it; only the fields relevant to the
// title are consumed by the generator (H2 gametype currently maps name + score
// limit only). Pointer fields are "leave at the template's value" when nil, so
// a record only carries what it actually overrides.
type GametypeSettings struct {
	Teams          *bool    `json:"teams,omitempty"`
	Radar          *bool    `json:"radar,omitempty"`
	ScoreLimit     *uint32  `json:"score_limit,omitempty"`
	TimeMinutes    *float64 `json:"time_minutes,omitempty"`
	TimeLimit      *uint32  `json:"time_limit,omitempty"`
	TimeLimit2     *uint32  `json:"time_limit2,omitempty"`
	Options        *uint32  `json:"options,omitempty"`
	ScoringSubtype *uint32  `json:"scoring_subtype,omitempty"`
	Option2        *uint32  `json:"option2,omitempty"`
	Respawn        *uint32  `json:"respawn,omitempty"`
	EngineUnion    *uint32  `json:"engine_union,omitempty"`
}

// GametypeRequest builds the halosave request for a CE or H2 gametype variant.
// title is "ce" or "h2"; engine is the CE engine (slayer/ctf/oddball/king/race)
// or the H2 mode (slayer); name is the variant name shown in-game.
func GametypeRequest(title, engine, name string, s GametypeSettings) halosave.BuildRequest {
	return halosave.BuildRequest{
		Title:          title,
		Kind:           halosave.KindGametype,
		Name:           name,
		Engine:         engine,
		Teams:          s.Teams,
		Radar:          s.Radar,
		ScoreLimit:     s.ScoreLimit,
		TimeMinutes:    s.TimeMinutes,
		TimeLimit:      s.TimeLimit,
		TimeLimit2:     s.TimeLimit2,
		Options:        s.Options,
		ScoringSubtype: s.ScoringSubtype,
		Option2:        s.Option2,
		Respawn:        s.Respawn,
		EngineUnion:    s.EngineUnion,
	}
}
