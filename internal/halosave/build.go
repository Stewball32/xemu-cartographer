package halosave

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

// Title / kind discriminators used by BuildRequest.
const (
	TitleCE = "ce"
	TitleH2 = "h2"

	KindGametype = "gametype"
	KindProfile  = "profile"
)

// SaveFile is one file in a generated save set, with its bytes and integrity
// metadata. Data is omitted from JSON (the preview endpoint returns metadata
// only; the download endpoint streams Data).
type SaveFile struct {
	Name string `json:"name"` // FATX filename, e.g. "blam.lst" / "SaveMeta.xbx"
	Size int    `json:"size"`
	SHA1 string `json:"sha1"`
	Data []byte `json:"-"`
}

func newSaveFile(name string, data []byte) SaveFile {
	sum := sha1.Sum(data)
	return SaveFile{Name: name, Size: len(data), SHA1: hex.EncodeToString(sum[:]), Data: data}
}

// SaveSet is a complete, ready-to-write save directory: the payload, its
// SaveMeta sidecar, and where on the Xbox FATX disk it belongs.
type SaveSet struct {
	Title    string       `json:"title"`    // "ce" | "h2"
	Kind     string       `json:"kind"`     // "gametype" | "profile"
	TitleID  string       `json:"title_id"` // "4d530004" | "4d530064"
	DirName  string       `json:"dir_name"` // FATX save-dir name
	FatxDir  string       `json:"fatx_dir"` // "UDATA/<titleID>/<dirName>"
	Files    []SaveFile   `json:"files"`
	Digest   DigestStatus `json:"digest"`
	Parsed   any          `json:"parsed"`             // re-parsed payload (round-trip confirmation)
	Warnings []string     `json:"warnings,omitempty"` // inferred-field / partial-map caveats
}

// TotalBytes is the sum of all file sizes (raw, before FATX cluster rounding).
func (s *SaveSet) TotalBytes() int {
	n := 0
	for _, f := range s.Files {
		n += f.Size
	}
	return n
}

// BuildRequest is the flat, JSON/query-friendly spec consumed by Build. CE and
// H2 fields share this one struct; Title+Kind select which are used. Pointer
// fields are "leave at template value" when nil (so a request only carries what
// it wants to change).
type BuildRequest struct {
	Title string `json:"title"` // "ce" | "h2"
	Kind  string `json:"kind"`  // "gametype" | "profile"

	// Naming. Name is the variant/player name shown in-game and in SaveMeta.
	// InternalName optionally overrides the in-file name field (defaults to
	// Name). DirName optionally overrides the FATX save-dir name.
	Name         string `json:"name"`
	InternalName string `json:"internal_name,omitempty"`
	DirName      string `json:"dir_name,omitempty"`

	// CE gametype — friendly settings. buildCEGametype owns the offset/bit/scale
	// knowledge and converts these to raw bytes; a nil field keeps the template's
	// value. Raw escape hatches (Options / EngineUnion) are applied first, then
	// the matching bool toggles win per-bit on top.
	Engine string `json:"engine,omitempty"` // slayer/ctf/oddball/king/race
	Teams  *bool  `json:"teams,omitempty"`

	// options bitfield @0x20 (Player/Item/Indicator toggles)
	Radar            *bool   `json:"radar,omitempty"`             // other players on radar ("R")
	FriendIndicators *bool   `json:"friend_indicators,omitempty"` // friend indicators on screen
	InfiniteGrenades *bool   `json:"infinite_grenades,omitempty"`
	ShieldsOff       *bool   `json:"shields_off,omitempty"` // set = no shields
	InvisiblePlayers *bool   `json:"invisible_players,omitempty"`
	GenericEquipment *bool   `json:"generic_equipment,omitempty"` // set = generic, clear = custom
	Options          *uint32 `json:"options,omitempty"`           // raw override

	ObjectivesIndicator  *uint32  `json:"objectives_indicator,omitempty"` // 0 motion, 1 nav, 2 none
	OddManOut            *bool    `json:"odd_man_out,omitempty"`
	RespawnSeconds       *float64 `json:"respawn_seconds,omitempty"`        // 0x30 (sec)
	RespawnGrowthSeconds *float64 `json:"respawn_growth_seconds,omitempty"` // 0x2C (sec)
	SuicideSeconds       *float64 `json:"suicide_seconds,omitempty"`        // 0x34 (sec)
	Lives                *uint32  `json:"lives,omitempty"`                  // 0 = infinite
	MaxHealth            *float32 `json:"max_health,omitempty"`             // multiplier (1.0 = 100%)
	ScoreLimit           *uint32  `json:"score_limit,omitempty"`            // also H2 gametype score
	WeaponSet            *uint32  `json:"weapon_set,omitempty"`             // 0..6
	NHEToggles           *uint32  `json:"nhe_toggles,omitempty"`            // 0..4
	EngineUnion          *uint32  `json:"engine_union,omitempty"`           // raw override

	// engine_union rule toggles (only the selected engine's are meaningful)
	DeathBonusOff  *bool `json:"death_bonus_off,omitempty"`  // slayer
	KillPenaltyOff *bool `json:"kill_penalty_off,omitempty"` // slayer
	KillInOrder    *bool `json:"kill_in_order,omitempty"`    // slayer
	Assault        *bool `json:"assault,omitempty"`          // ctf
	FlagMustReset  *bool `json:"flag_must_reset,omitempty"`  // ctf
	FlagAtHome     *bool `json:"flag_at_home,omitempty"`     // ctf
	MovingHill     *bool `json:"moving_hill,omitempty"`      // king
	RandomStart    *bool `json:"random_start,omitempty"`     // oddball
	RaceAnyOrder   *bool `json:"race_any_order,omitempty"`   // race

	// engine scratch (0x50..0x60), engine-specific
	CTFSingleFlagMinutes *float64 `json:"ctf_single_flag_minutes,omitempty"` // ctf; 0 = off
	OddballSpeed         *uint32  `json:"oddball_speed,omitempty"`           // 0 slow,1 normal,2 fast
	OddballTraitWith     *uint32  `json:"oddball_trait_with,omitempty"`      // 0..3
	OddballTraitWithout  *uint32  `json:"oddball_trait_without,omitempty"`   // 0..3
	OddballBallType      *uint32  `json:"oddball_ball_type,omitempty"`       // 0 normal,1 reverse tag
	BallSpawnCount       *uint32  `json:"ball_spawn_count,omitempty"`        // oddball
	RaceScoring          *uint32  `json:"race_scoring,omitempty"`            // 0 min,1 max

	// H2 profile — appearance/controller bytes keyed by H2ProfileFields.Key.
	Appearance map[string]int `json:"appearance,omitempty"`

	// CE profile — armor color, controller presets, and the nine Advanced
	// Controls (2026-08-07 live-verified). nil = fresh-MP factory default.
	Color         *uint32  `json:"color,omitempty"`
	Button        *uint32  `json:"button,omitempty"`     // 0..4 preset
	Thumbstick    *uint32  `json:"thumbstick,omitempty"` // 0..3 preset
	HSens         *float64 `json:"h_sens,omitempty"`     // 1.00..10.00
	VMult         *float64 `json:"v_mult,omitempty"`     // 0.50..1.00
	Invert        *bool    `json:"invert,omitempty"`
	Vibration     *bool    `json:"vibration,omitempty"`
	RSDeadzone    *uint32  `json:"rs_deadzone,omitempty"`    // 0..35
	LSDeadzone    *uint32  `json:"ls_deadzone,omitempty"`    // 0..35
	OuterDeadzone *uint32  `json:"outer_deadzone,omitempty"` // 1..15
	DeadzoneType  *uint32  `json:"deadzone_type,omitempty"`  // 0 radial,1 axial
	Response      *uint32  `json:"response,omitempty"`       // 1..7 curve

	// Recompute is retained for API compatibility. The digest algorithm is now
	// resolved (see digest.go), so CE/H2 files are ALWAYS correctly re-signed
	// regardless of this flag — a generated file must carry a valid signature or
	// Halo rejects it as "damaged". The field no longer gates signing.
	Recompute bool `json:"recompute,omitempty"`
}

// Build produces a complete SaveSet from a request, dispatching on Title+Kind.
func Build(req BuildRequest) (*SaveSet, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("halosave: name is required")
	}
	switch req.Title {
	case TitleCE:
		switch req.Kind {
		case KindGametype:
			return buildCEGametype(req)
		case KindProfile:
			return buildCEProfile(req)
		}
		return nil, fmt.Errorf("halosave: unknown CE kind %q", req.Kind)
	case TitleH2:
		switch req.Kind {
		case KindProfile:
			return buildH2Profile(req)
		case KindGametype:
			return buildH2Gametype(req)
		}
		return nil, fmt.Errorf("halosave: unknown H2 kind %q", req.Kind)
	}
	return nil, fmt.Errorf("halosave: unknown title %q (want %q or %q)", req.Title, TitleCE, TitleH2)
}

func buildCEGametype(req BuildRequest) (*SaveSet, error) {
	engine := req.Engine
	if engine == "" {
		engine = "slayer"
	}
	tmpl, err := CETemplate(engine)
	if err != nil {
		return nil, err
	}
	base, err := CEParse(tmpl)
	if err != nil {
		return nil, err
	}

	internal := req.InternalName
	if internal == "" {
		internal = req.Name
	}
	p := CEPatch{Name: &internal}
	p.Engine = u32ptr(CEEngineID(engine))
	if req.Teams != nil {
		p.Teams = u32ptr(boolU32(*req.Teams))
	}

	// Options bitfield: raw override applied first, then the bool toggles win
	// per-bit on top of the resolved value. If nothing touched it, leave the
	// template's value alone.
	opts := base.Options
	optTouched := false
	if req.Options != nil {
		opts = *req.Options
		optTouched = true
	}
	optTouched = setBitOpt(&opts, ceOptRadar, req.Radar) || optTouched
	optTouched = setBitOpt(&opts, ceOptFriendInd, req.FriendIndicators) || optTouched
	optTouched = setBitOpt(&opts, ceOptInfGrenades, req.InfiniteGrenades) || optTouched
	optTouched = setBitOpt(&opts, ceOptShieldsOff, req.ShieldsOff) || optTouched
	optTouched = setBitOpt(&opts, ceOptInvisible, req.InvisiblePlayers) || optTouched
	optTouched = setBitOpt(&opts, ceOptGenericEquip, req.GenericEquipment) || optTouched
	if optTouched {
		p.Options = u32ptr(opts)
	}

	if req.ObjectivesIndicator != nil {
		p.ObjectivesIndicator = req.ObjectivesIndicator
	}
	if req.OddManOut != nil {
		p.OddManOut = u32ptr(boolU32(*req.OddManOut))
	}
	if req.RespawnSeconds != nil {
		p.RespawnTime = u32ptr(CESecondsToRaw(*req.RespawnSeconds))
	}
	if req.RespawnGrowthSeconds != nil {
		p.RespawnGrowth = u32ptr(CESecondsToRaw(*req.RespawnGrowthSeconds))
	}
	if req.SuicideSeconds != nil {
		p.SuicidePenalty = u32ptr(CESecondsToRaw(*req.SuicideSeconds))
	}
	if req.Lives != nil {
		p.Lives = req.Lives
	}
	if req.MaxHealth != nil {
		p.MaxHealth = req.MaxHealth
	}
	if req.ScoreLimit != nil {
		p.ScoreLimit = req.ScoreLimit
	}
	if req.WeaponSet != nil {
		p.WeaponSet = req.WeaponSet
	}
	if req.NHEToggles != nil {
		p.NHEToggles = req.NHEToggles
	}

	// engine_union: raw override first, then engine-specific rule toggles.
	eu := base.EngineUnion
	euTouched := false
	if req.EngineUnion != nil {
		eu = *req.EngineUnion
		euTouched = true
	}
	euTouched = setBitOpt(&eu, ceEUSlayerDeathBonusOff, req.DeathBonusOff) || euTouched
	euTouched = setBitOpt(&eu, ceEUSlayerKillPenOff, req.KillPenaltyOff) || euTouched
	euTouched = setBitOpt(&eu, ceEUSlayerKillInOrder, req.KillInOrder) || euTouched
	euTouched = setBitOpt(&eu, ceEUCTFAssault, req.Assault) || euTouched
	euTouched = setBitOpt(&eu, ceEUCTFFlagReset, req.FlagMustReset) || euTouched
	euTouched = setBitOpt(&eu, ceEUCTFFlagAtHome, req.FlagAtHome) || euTouched
	euTouched = setBitOpt(&eu, ceEUKingMovingHill, req.MovingHill) || euTouched
	euTouched = setBitOpt(&eu, ceEUOddRandomStart, req.RandomStart) || euTouched
	euTouched = setBitOpt(&eu, ceEURaceType, req.RaceAnyOrder) || euTouched
	if euTouched {
		p.EngineUnion = u32ptr(eu)
	}

	// engine scratch (0x50..0x60)
	if req.CTFSingleFlagMinutes != nil {
		p.CTFSingleFlag = u32ptr(uint32(math.Round(*req.CTFSingleFlagMinutes * 60 * 30)))
	}
	if req.OddballSpeed != nil {
		p.OddballSpeed = req.OddballSpeed
	}
	if req.OddballTraitWith != nil {
		p.OddballTraitWith = req.OddballTraitWith
	}
	if req.OddballTraitWithout != nil {
		p.OddballTraitWithout = req.OddballTraitWithout
	}
	if req.OddballBallType != nil {
		p.OddballBallType = req.OddballBallType
	}
	if req.BallSpawnCount != nil {
		p.BallSpawnCount = req.BallSpawnCount
	}
	if req.RaceScoring != nil {
		p.RaceScoring = req.RaceScoring
	}

	payload, err := CEBuild(tmpl, p, true) // always re-sign: a valid digest is mandatory
	if err != nil {
		return nil, err
	}
	parsed, err := CEParse(payload)
	if err != nil {
		return nil, err // structural round-trip: a generated file must re-parse
	}

	dir := req.DirName
	if dir == "" {
		dir = "G-" + req.Name
	}
	set := &SaveSet{
		Title:   TitleCE,
		Kind:    KindGametype,
		TitleID: TitleIDHaloCE,
		DirName: dir,
		Files: []SaveFile{
			newSaveFile("blam.lst", payload),
			newSaveFile("SaveMeta.xbx", SaveMetaBuild(req.Name)),
		},
		Digest:   recomputedDigest(!bytes.Equal(payload, tmpl)),
		Parsed:   parsed,
		Warnings: ceWarnings(!bytes.Equal(payload, tmpl)),
	}
	set.finish()
	return set, nil
}

// buildCEProfile generates a signed Halo: CE player profile (blam.sav). Editable
// surface: armor color, button/thumbstick presets, and the nine Advanced
// Controls (all 2026-08-07 live-verified). Name is the in-game MP name. Always
// re-signed at 0x30.
func buildCEProfile(req BuildRequest) (*SaveSet, error) {
	name := req.Name
	p := CEProfilePatch{Name: &name, Color: req.Color}
	if req.Button != nil {
		if *req.Button > 255 {
			return nil, fmt.Errorf("halosave: CE button preset %d out of byte range", *req.Button)
		}
		b := byte(*req.Button)
		p.Button = &b
	}
	if req.Thumbstick != nil {
		if *req.Thumbstick > 255 {
			return nil, fmt.Errorf("halosave: CE thumbstick preset %d out of byte range", *req.Thumbstick)
		}
		b := byte(*req.Thumbstick)
		p.Thumbstick = &b
	}

	// Advanced controls: start from factory default, override provided fields.
	adv := ceAdvancedDefault()
	if req.HSens != nil {
		adv.HSens = *req.HSens
	}
	if req.VMult != nil {
		adv.VMult = *req.VMult
	}
	if req.Invert != nil {
		adv.Invert = *req.Invert
	}
	if req.Vibration != nil {
		adv.Vibration = *req.Vibration
	}
	if req.RSDeadzone != nil {
		adv.RSDeadzone = uint8(min32(*req.RSDeadzone, 35))
	}
	if req.LSDeadzone != nil {
		adv.LSDeadzone = uint8(min32(*req.LSDeadzone, 35))
	}
	if req.OuterDeadzone != nil {
		adv.OuterDeadzone = uint8(clamp32(*req.OuterDeadzone, 1, 15))
	}
	if req.DeadzoneType != nil {
		adv.DeadzoneType = uint8(min32(*req.DeadzoneType, 1))
	}
	if req.Response != nil {
		adv.Response = uint8(clamp32(*req.Response, 1, 7))
	}
	p.Advanced = &adv

	payload, err := CEProfileBuild(p, true) // always re-sign
	if err != nil {
		return nil, err
	}
	parsed, err := CEProfileParse(payload)
	if err != nil {
		return nil, err // structural round-trip: a generated file must re-parse
	}

	dir := req.DirName
	if dir == "" {
		dir = deriveH2Dir(payload) // 12-hex id, same convention as H2
	}
	set := &SaveSet{
		Title:   TitleCE,
		Kind:    KindProfile,
		TitleID: TitleIDHaloCE,
		DirName: dir,
		Files: []SaveFile{
			newSaveFile("blam.sav", payload),
			newSaveFile("SaveMeta.xbx", SaveMetaBuild(req.Name)),
		},
		Digest: recomputedDigest(true),
		Parsed: parsed,
		Warnings: []string{
			"CE profile does not generate a campaign savegame.bin sibling (CE auto-creates it on first play). The HSENS byte uses the game's LOAD map, which the editor displays correctly.",
		},
	}
	set.finish()
	return set, nil
}

func min32(v, hi uint32) uint32 {
	if v > hi {
		return hi
	}
	return v
}

func clamp32(v, lo, hi uint32) uint32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func buildH2Profile(req BuildRequest) (*SaveSet, error) {
	tmpl, err := H2ProfileTemplate()
	if err != nil {
		return nil, err
	}
	name := req.Name
	p := H2ProfilePatch{Name: &name, AppctlPatch: map[int]byte{}}
	for key, val := range req.Appearance {
		off, ok := h2ProfileFieldOffset(key)
		if !ok {
			return nil, fmt.Errorf("halosave: unknown appearance field %q (have %v)", key, h2ProfileFieldKeys())
		}
		if val < 0 || val > 255 {
			return nil, fmt.Errorf("halosave: appearance %q value %d out of byte range 0..255", key, val)
		}
		p.AppctlPatch[off] = byte(val)
	}
	payload, err := H2ProfileBuild(tmpl, p, true) // always re-sign: a valid digest is mandatory
	if err != nil {
		return nil, err
	}
	parsed, err := H2ProfileParse(payload)
	if err != nil {
		return nil, err
	}
	dir := req.DirName
	if dir == "" {
		dir = deriveH2Dir(payload)
	}
	set := &SaveSet{
		Title:   TitleH2,
		Kind:    KindProfile,
		TitleID: TitleIDHalo2,
		DirName: dir,
		Files: []SaveFile{
			newSaveFile("profile", payload),
			newSaveFile("SaveMeta.xbx", SaveMetaBuild("Profile: "+req.Name)),
		},
		Digest: recomputedDigest(!bytes.Equal(payload, tmpl)),
		Parsed: parsed,
		Warnings: []string{
			"Halo 2 appearance/controller byte labels are PROVISIONAL (derived from 2 sample profiles); confirm before trusting individual labels.",
		},
	}
	set.finish()
	return set, nil
}

func buildH2Gametype(req BuildRequest) (*SaveSet, error) {
	tmpl, err := H2GametypeTemplate()
	if err != nil {
		return nil, err
	}
	name := req.Name
	p := H2GametypePatch{Name: &name}
	if req.ScoreLimit != nil {
		p.ScoreLimit = req.ScoreLimit
	}
	payload, err := H2GametypeBuild(tmpl, p, true) // always re-sign: a valid digest is mandatory
	if err != nil {
		return nil, err
	}
	parsed, err := H2GametypeParse(payload)
	if err != nil {
		return nil, err
	}
	dir := req.DirName
	if dir == "" {
		dir = deriveH2Dir(payload)
	}
	mode := H2GametypeMode
	display := strings.ToUpper(mode[:1]) + mode[1:] + ": " + req.Name
	set := &SaveSet{
		Title:   TitleH2,
		Kind:    KindGametype,
		TitleID: TitleIDHalo2,
		DirName: dir,
		Files: []SaveFile{
			newSaveFile(mode, payload),
			newSaveFile("SaveMeta.xbx", SaveMetaBuild(display)),
		},
		Digest: recomputedDigest(!bytes.Equal(payload, tmpl)),
		Parsed: parsed,
		Warnings: []string{
			"Halo 2 gametype field map is PARTIAL (only name + score limit are mapped from a single sample); other settings are preserved from the template.",
		},
	}
	set.finish()
	return set, nil
}

// finish fills the derived FATX path and appends the digest caveat when edited.
func (s *SaveSet) finish() {
	s.FatxDir = "UDATA/" + s.TitleID + "/" + s.DirName
	if s.Digest.Edited && !s.Digest.Resolved {
		s.Warnings = append(s.Warnings,
			"Edited-settings file: the 20-byte content digest is PRESERVED from the template and no longer matches the new content. Whether Halo re-verifies this digest on load is unverified (see the de-risk verdict). Confirm acceptance on xemu before relying on this file.")
	}
}

// ceWarnings returns build caveats for a CE gametype. The field map is
// 2026-08-07 LIVE-VERIFIED (single-setting differentials + in-title load-back on
// all five engines), so an edited file is trustworthy; the only residual
// caveats are two enum ranges that weren't exhaustively cycled.
func ceWarnings(edited bool) []string {
	if edited {
		return []string{
			"Oddball SPEED WITH BALL (0 slow, 2 fast) and BALL TYPE (0 normal, 1 reverse tag) were confirmed at those values but not every intermediate value was cycled in-game.",
		}
	}
	return nil
}

// setBitOpt applies an optional boolean toggle to bit(s) mask in *v and reports
// whether it changed anything (i.e. b was non-nil). Used to resolve the CE
// options and engine_union bitfields from friendly per-bit toggles.
func setBitOpt(v *uint32, mask uint32, b *bool) bool {
	if b == nil {
		return false
	}
	if *b {
		*v |= mask
	} else {
		*v &^= mask
	}
	return true
}

// deriveH2Dir produces a deterministic 12-hex-uppercase FATX directory name
// from the payload bytes (Halo's real H2 save dirs are 12-hex ids). Determinism
// keeps generation reproducible and tests stable; callers may override DirName.
func deriveH2Dir(payload []byte) string {
	sum := sha1.Sum(payload)
	return strings.ToUpper(hex.EncodeToString(sum[:6]))
}

func h2ProfileFieldOffset(key string) (int, bool) {
	for _, f := range H2ProfileFields {
		if f.Key == key {
			return f.Offset, true
		}
	}
	return 0, false
}

func h2ProfileFieldKeys() []string {
	out := make([]string, len(H2ProfileFields))
	for i, f := range H2ProfileFields {
		out[i] = f.Key
	}
	return out
}

func u32ptr(v uint32) *uint32 { return &v }

func boolU32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}
