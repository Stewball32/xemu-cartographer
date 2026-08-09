package halosave

import (
	"fmt"
	"math"
)

// Halo: CE gametype — blam.lst, fixed 512 bytes, little-endian.
//
// Field map is the 2026-08-07 LIVE-VERIFIED byte map (testrig bench, H1
// Performance Build): every setting isolated by single-setting differential
// saves in the in-game "Edit Game Types" editor, each pull byte-diffed against
// the previous, all five engines walked, load-back confirmed in-title. This
// SUPERSEDES the earlier offline-derived labels (which mislabelled 0x24/0x30/
// 0x34/0x3C/0x44/0x48/0x60 and missed odd-man-out / lives / respawn-growth /
// the engine scratch region). See MODDED-GAMETYPE-BYTEMAP.md §D for the
// corrections.
//
// Layout:
//
//	0x00  24  name (UTF-16LE, NUL-term, <=11 chars)
//	0x18  4   engine        1 CTF, 2 Slayer, 3 Oddball, 4 King, 5 Race
//	0x1C  4   teams         0 FFA, 1 team
//	0x20  4   options       bitfield (see ceOpt* below); base 0x22
//	0x24  4   objectives indicator  0 motion-tracker, 1 nav-points, 2 none
//	0x28  4   odd man out   0 no, 1 yes
//	0x2C  4   respawn time growth   seconds*30 (0 = none)
//	0x30  4   respawn time  seconds*30 (0 = instant)
//	0x34  4   suicide penalty       seconds*30 (0 = none)
//	0x38  4   number of lives       0 = infinite, else count
//	0x3C  4   maximum health        f32 multiplier (1.0 = 100%)
//	0x40  4   score limit   engine unit (Slayer kills / CTF captures / King+Oddball min / Race laps)
//	0x44  4   weapon set    0 normal .. 6 rockets (see ceWeaponSet*)
//	0x48  4   NHE toggles   0 training .. 4 nhe&powerups (see ceNHE*)
//	0x4C  4   engine union  per-engine Step-2 rule bitfield (see ceEU*)
//	0x50  16  engine scratch        per-engine (CTF single-flag / Oddball traits+speed+type / Race scoring)
//	0x60  4   ball spawn count      Oddball only (1 default; 0 elsewhere)
//	0x68  20  digest (roamable HMAC over [0x00:0x68])
//	0x7C  ..  unsigned don't-care tail (preserved verbatim from template)
const (
	ceSize = 0x200 // 512

	ceOffName     = 0x00 // UTF-16LE name, NUL-terminated
	ceNameBuf     = 0x18 // 24-byte name buffer (<=11 chars)
	ceOffEngine   = 0x18 // u32 engine id
	ceOffTeams    = 0x1C // u32 0=FFA, 1=team
	ceOffOptions  = 0x20 // u32 options bitfield
	ceOffObjInd   = 0x24 // u32 objectives indicator (motion/nav/none)
	ceOffOddMan   = 0x28 // u32 odd man out
	ceOffRespGrow = 0x2C // u32 respawn time growth (sec*30)
	ceOffRespTime = 0x30 // u32 respawn time (sec*30)
	ceOffSuicide  = 0x34 // u32 suicide penalty (sec*30)
	ceOffLives    = 0x38 // u32 number of lives (0=infinite)
	ceOffMaxHP    = 0x3C // f32 maximum health multiplier
	ceOffScore    = 0x40 // u32 score limit (engine unit)
	ceOffWeapon   = 0x44 // u32 weapon set
	ceOffNHE      = 0x48 // u32 NHE toggles
	ceOffEUnion   = 0x4C // u32 engine union (per-engine bitfield)
	ceOffScratch  = 0x50 // 16-byte engine scratch region
	ceOffBallSpwn = 0x60 // u32 ball spawn count (oddball)
	ceOffDigest   = 0x68 // 20-byte roamable digest (0x68..0x7B)
	ceDigestLen   = 20

	// Engine scratch sub-offsets (0x50..0x5F, reused per engine).
	ceOffCTFSingleFlag   = 0x50 // CTF: single-flag period (0 off, 1800 = 1 min)
	ceOffOddSpeed        = 0x50 // Oddball: speed with ball (0 slow, 1 normal, 2 fast)
	ceOffOddTraitWith    = 0x54 // Oddball: trait with ball (0..3)
	ceOffOddTraitNo      = 0x58 // Oddball: trait without ball (0..3)
	ceOffOddBallType     = 0x5C // Oddball: ball type (0 normal, 1 reverse tag)
	ceOffRaceScoring     = 0x50 // Race: team scoring (0 minimum, 1 maximum)
	ceRespawnTicksPerSec = 30   // sec*30 encoding for the three timer fields
)

// CE engine ids.
const (
	CEEngineCTF     = 1
	CEEngineSlayer  = 2
	CEEngineOddball = 3
	CEEngineKing    = 4
	CEEngineRace    = 5
)

// ceEngineNames maps the engine id to its lowercase name (and back).
var ceEngineNames = map[uint32]string{
	CEEngineCTF: "ctf", CEEngineSlayer: "slayer", CEEngineOddball: "oddball",
	CEEngineKing: "king", CEEngineRace: "race",
}

// CEEngineID resolves an engine name ("slayer", "ctf"…) to its id, or 0 if
// unknown.
func CEEngineID(name string) uint32 {
	for id, n := range ceEngineNames {
		if n == name {
			return id
		}
	}
	return 0
}

// CEEngineName returns the lowercase engine name for an id, or "?".
func CEEngineName(id uint32) string {
	if n, ok := ceEngineNames[id]; ok {
		return n
	}
	return "?"
}

// CE options bitfield @0x20 (all live-confirmed). Fresh default = 0x22 (base
// 0x20 + radar 0x02? no — base is 0x22 = radar-on|friend-on per the map). The
// base bits observed constant are folded into the template; the flags below are
// the toggles the editor exposes.
const (
	ceOptRadar        = 0x01 // OTHER PLAYERS ON RADAR (the "R" suffix)
	ceOptFriendInd    = 0x02 // FRIEND INDICATORS ON SCREEN
	ceOptInfGrenades  = 0x04 // INFINITE GRENADES
	ceOptShieldsOff   = 0x08 // SHIELDS OFF (set = no shields)
	ceOptInvisible    = 0x10 // INVISIBLE PLAYERS
	ceOptGenericEquip = 0x20 // STARTING EQUIPMENT = GENERIC (clear = CUSTOM)
	ceOptBase         = 0x22 // fresh default (radar + friend indicators on)
)

// engine_union @0x4C bit masks (little-endian u32). Bit ownership differs per
// engine; a mask is only meaningful for its engine.
const (
	// Slayer
	ceEUSlayerDeathBonusOff = 0x00000001 // DEATH BONUS off
	ceEUSlayerKillPenOff    = 0x00000100 // KILL PENALTY off
	ceEUSlayerKillInOrder   = 0x00010000 // KILL IN ORDER on
	// CTF
	ceEUCTFAssault    = 0x00000001 // ASSAULT
	ceEUCTFFlagReset  = 0x00010000 // FLAG MUST RESET
	ceEUCTFFlagAtHome = 0x01000000 // FLAG AT HOME TO SCORE
	// King / Oddball / Race single-bit rules
	ceEUKingMovingHill = 0x00000001 // MOVING HILL
	ceEUOddRandomStart = 0x00000001 // RANDOM START
	ceEURaceType       = 0x00000001 // RACE TYPE (0 normal / 1 any order)
)

// CEGametype is the parsed, fully-labelled view of a blam.lst. Raw holds the
// original bytes so the unsigned tail (and any unmapped region) can be inspected
// or preserved.
type CEGametype struct {
	Raw        []byte `json:"-"`
	Name       string `json:"name"`
	Engine     uint32 `json:"engine"`
	EngineName string `json:"engine_name"`
	Teams      uint32 `json:"teams"`

	Options             uint32  `json:"options"`
	ObjectivesIndicator uint32  `json:"objectives_indicator"`
	OddManOut           uint32  `json:"odd_man_out"`
	RespawnGrowth       uint32  `json:"respawn_growth"`  // raw (sec*30)
	RespawnTime         uint32  `json:"respawn_time"`    // raw (sec*30)
	SuicidePenalty      uint32  `json:"suicide_penalty"` // raw (sec*30)
	Lives               uint32  `json:"lives"`           // 0 = infinite
	MaxHealth           float32 `json:"max_health"`      // f32 multiplier
	ScoreLimit          uint32  `json:"score_limit"`
	ScoreUnit           string  `json:"score_unit"`
	WeaponSet           uint32  `json:"weapon_set"`
	NHEToggles          uint32  `json:"nhe_toggles"`
	EngineUnion         uint32  `json:"engine_union"`

	// Engine scratch (only the fields relevant to Engine are meaningful).
	CTFSingleFlag       uint32 `json:"ctf_single_flag"`
	OddballSpeed        uint32 `json:"oddball_speed"`
	OddballTraitWith    uint32 `json:"oddball_trait_with"`
	OddballTraitWithout uint32 `json:"oddball_trait_without"`
	OddballBallType     uint32 `json:"oddball_ball_type"`
	BallSpawnCount      uint32 `json:"ball_spawn_count"`
	RaceScoring         uint32 `json:"race_scoring"`

	Digest []byte `json:"digest"`
}

// ceScoreUnits maps engine id to the human unit of the 0x40 score limit.
var ceScoreUnits = map[uint32]string{
	CEEngineCTF: "captures", CEEngineSlayer: "kills", CEEngineOddball: "minutes",
	CEEngineKing: "minutes", CEEngineRace: "laps",
}

// CEScoreUnit returns the score-limit unit word for an engine name.
func CEScoreUnit(engine string) string {
	if u, ok := ceScoreUnits[CEEngineID(engine)]; ok {
		return u
	}
	return "points"
}

// CEParse decodes a 512-byte blam.lst.
func CEParse(b []byte) (*CEGametype, error) {
	if len(b) != ceSize {
		return nil, fmt.Errorf("halosave: CE blam.lst must be %d bytes, got %d", ceSize, len(b))
	}
	eng := getU32(b, ceOffEngine)
	g := &CEGametype{
		Raw:        append([]byte(nil), b...),
		Name:       readUTF16z(b, ceOffName, ceNameBuf),
		Engine:     eng,
		EngineName: CEEngineName(eng),
		Teams:      getU32(b, ceOffTeams),

		Options:             getU32(b, ceOffOptions),
		ObjectivesIndicator: getU32(b, ceOffObjInd),
		OddManOut:           getU32(b, ceOffOddMan),
		RespawnGrowth:       getU32(b, ceOffRespGrow),
		RespawnTime:         getU32(b, ceOffRespTime),
		SuicidePenalty:      getU32(b, ceOffSuicide),
		Lives:               getU32(b, ceOffLives),
		MaxHealth:           getF32(b, ceOffMaxHP),
		ScoreLimit:          getU32(b, ceOffScore),
		ScoreUnit:           ceScoreUnits[eng],
		WeaponSet:           getU32(b, ceOffWeapon),
		NHEToggles:          getU32(b, ceOffNHE),
		EngineUnion:         getU32(b, ceOffEUnion),

		CTFSingleFlag:       getU32(b, ceOffCTFSingleFlag),
		OddballSpeed:        getU32(b, ceOffOddSpeed),
		OddballTraitWith:    getU32(b, ceOffOddTraitWith),
		OddballTraitWithout: getU32(b, ceOffOddTraitNo),
		OddballBallType:     getU32(b, ceOffOddBallType),
		BallSpawnCount:      getU32(b, ceOffBallSpwn),
		RaceScoring:         getU32(b, ceOffRaceScoring),

		Digest: append([]byte(nil), b[ceOffDigest:ceOffDigest+ceDigestLen]...),
	}
	if g.ScoreUnit == "" {
		g.ScoreUnit = "points"
	}
	return g, nil
}

// CEPatch holds the fields to overwrite on a CE template. Nil pointers are left
// untouched (so a request only carries what it changes); every field maps to a
// byte range in the template and nothing else is altered. Engine-scratch fields
// are only written when set — the caller sends only the ones relevant to the
// selected engine, so the shared 0x50 slot never conflicts.
type CEPatch struct {
	Name   *string
	Engine *uint32
	Teams  *uint32

	Options             *uint32
	ObjectivesIndicator *uint32
	OddManOut           *uint32
	RespawnGrowth       *uint32
	RespawnTime         *uint32
	SuicidePenalty      *uint32
	Lives               *uint32
	MaxHealth           *float32
	ScoreLimit          *uint32
	WeaponSet           *uint32
	NHEToggles          *uint32
	EngineUnion         *uint32

	CTFSingleFlag       *uint32
	OddballSpeed        *uint32
	OddballTraitWith    *uint32
	OddballTraitWithout *uint32
	OddballBallType     *uint32
	BallSpawnCount      *uint32
	RaceScoring         *uint32
}

// CEBuild patches a real CE blam.lst template and returns the new 512-byte file.
// The unsigned tail and all untouched bytes are preserved verbatim from the
// template. recompute re-signs the 20-byte roamable digest over [0x00:0x68]
// (mandatory for an edited file — Halo rejects a stale signature as "damaged").
func CEBuild(template []byte, p CEPatch, recompute bool) ([]byte, error) {
	if len(template) != ceSize {
		return nil, fmt.Errorf("halosave: CE template must be %d bytes, got %d", ceSize, len(template))
	}
	out := append([]byte(nil), template...)
	if p.Name != nil {
		if err := writeUTF16z(out, ceOffName, ceNameBuf, *p.Name); err != nil {
			return nil, err
		}
	}
	putU32opt(out, ceOffEngine, p.Engine)
	putU32opt(out, ceOffTeams, p.Teams)
	putU32opt(out, ceOffOptions, p.Options)
	putU32opt(out, ceOffObjInd, p.ObjectivesIndicator)
	putU32opt(out, ceOffOddMan, p.OddManOut)
	putU32opt(out, ceOffRespGrow, p.RespawnGrowth)
	putU32opt(out, ceOffRespTime, p.RespawnTime)
	putU32opt(out, ceOffSuicide, p.SuicidePenalty)
	putU32opt(out, ceOffLives, p.Lives)
	if p.MaxHealth != nil {
		putF32(out, ceOffMaxHP, *p.MaxHealth)
	}
	putU32opt(out, ceOffScore, p.ScoreLimit)
	putU32opt(out, ceOffWeapon, p.WeaponSet)
	putU32opt(out, ceOffNHE, p.NHEToggles)
	putU32opt(out, ceOffEUnion, p.EngineUnion)
	putU32opt(out, ceOffCTFSingleFlag, p.CTFSingleFlag)
	putU32opt(out, ceOffOddSpeed, p.OddballSpeed)
	putU32opt(out, ceOffOddTraitWith, p.OddballTraitWith)
	putU32opt(out, ceOffOddTraitNo, p.OddballTraitWithout)
	putU32opt(out, ceOffOddBallType, p.OddballBallType)
	putU32opt(out, ceOffBallSpwn, p.BallSpawnCount)
	putU32opt(out, ceOffRaceScoring, p.RaceScoring)

	if recompute {
		dg, err := RecomputeDigest(out, ceOffDigest, ceDigestLen, "ce")
		if err != nil {
			return nil, err
		}
		copy(out[ceOffDigest:ceOffDigest+ceDigestLen], dg)
	}
	return out, nil
}

// putU32opt writes *v at off when v is non-nil.
func putU32opt(b []byte, off int, v *uint32) {
	if v != nil {
		putU32(b, off, *v)
	}
}

// CESecondsToRaw converts a friendly seconds value (respawn time / growth /
// suicide penalty) to the raw field (unit = 1/30 s, i.e. a 2-frame tick).
// LIVE-VERIFIED: 5 s = 150, 10 s = 300, none/instant = 0.
func CESecondsToRaw(seconds float64) uint32 {
	return uint32(math.Round(seconds * ceRespawnTicksPerSec))
}

// CERawToSeconds is the inverse of CESecondsToRaw.
func CERawToSeconds(raw uint32) float64 {
	return float64(raw) / ceRespawnTicksPerSec
}
