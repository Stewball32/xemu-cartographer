package halosave

import (
	"fmt"
	"math"
)

// Halo: CE gametype — blam.lst, fixed 512 bytes, little-endian. Field offsets
// verified by diffing 23 real variants (see FORMATS.md §2). The leading bytes
// are the variant name, so two variants differing in one setting differ at
// exactly that field's offset and the digest — which is how the map was
// derived.
const (
	ceSize = 0x200 // 512

	ceOffName        = 0x00 // UTF-16LE name, NUL-terminated
	ceNameBuf        = 0x18 // 24-byte name buffer (<=11 chars)
	ceOffEngine      = 0x18 // u32: 1=ctf 2=slayer 3=oddball 4=king 5=race
	ceOffTeams       = 0x1C // u32: 1=team game, 0=free-for-all
	ceOffOptions     = 0x20 // u32 bitfield: base 0x22; bit0(+1)="R", 0x10=snipers, 0x04=practice
	ceOffScoringSub  = 0x24 // u32: slayer=2, ctf/oddball/king=1, R/wizard=0
	ceOffTimeLimit   = 0x30 // u32: raw, unit=2s (minutes*30); 0=none
	ceOffTimeLimit2  = 0x34 // u32: secondary timer
	ceOffSpeed       = 0x3C // f32: 1.0 on all samples
	ceOffScoreLimit  = 0x40 // u32: captures / kills / etc.
	ceOffOption2     = 0x44 // u32: wizard=1, snipers=4
	ceOffRespawn     = 0x48 // u32: 2=normal, 0=practice/training, 3=on-off
	ceOffEngineUnion = 0x4C // u32: engine-specific (CTF=0x01000000, slayer=0x0101, king=1)
	ceOffDigest      = 0x68 // 20-byte content digest (0x68..0x7B)
	ceDigestLen      = 20
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
// unknown. Used by the template selector and the build dispatcher.
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

// CEGametype is the parsed view of a blam.lst. Raw holds the original bytes so
// the unmapped regions (speed, padding, digest) can be inspected or preserved.
type CEGametype struct {
	Raw            []byte `json:"-"`
	Name           string `json:"name"`
	Engine         uint32 `json:"engine"`
	EngineName     string `json:"engine_name"`
	Teams          uint32 `json:"teams"`
	Options        uint32 `json:"options"`
	ScoringSubtype uint32 `json:"scoring_subtype"`
	TimeLimit      uint32 `json:"time_limit"`
	TimeLimit2     uint32 `json:"time_limit2"`
	ScoreLimit     uint32 `json:"score_limit"`
	Option2        uint32 `json:"option2"`
	Respawn        uint32 `json:"respawn"`
	EngineUnion    uint32 `json:"engine_union"`
	Digest         []byte `json:"digest"`
}

// CEParse decodes a 512-byte blam.lst.
func CEParse(b []byte) (*CEGametype, error) {
	if len(b) != ceSize {
		return nil, fmt.Errorf("halosave: CE blam.lst must be %d bytes, got %d", ceSize, len(b))
	}
	g := &CEGametype{
		Raw:            append([]byte(nil), b...),
		Name:           readUTF16z(b, ceOffName, ceNameBuf),
		Engine:         getU32(b, ceOffEngine),
		Teams:          getU32(b, ceOffTeams),
		Options:        getU32(b, ceOffOptions),
		ScoringSubtype: getU32(b, ceOffScoringSub),
		TimeLimit:      getU32(b, ceOffTimeLimit),
		TimeLimit2:     getU32(b, ceOffTimeLimit2),
		ScoreLimit:     getU32(b, ceOffScoreLimit),
		Option2:        getU32(b, ceOffOption2),
		Respawn:        getU32(b, ceOffRespawn),
		EngineUnion:    getU32(b, ceOffEngineUnion),
		Digest:         append([]byte(nil), b[ceOffDigest:ceOffDigest+ceDigestLen]...),
	}
	g.EngineName = CEEngineName(g.Engine)
	return g, nil
}

// CEPatch holds the fields to overwrite on a CE template. Nil pointers are left
// untouched (mirrors the Python kwargs=None default). Every field maps directly
// to a byte range in the template; nothing else is changed.
type CEPatch struct {
	Name           *string
	Engine         *uint32
	Teams          *uint32
	Options        *uint32
	ScoringSubtype *uint32
	TimeLimit      *uint32
	TimeLimit2     *uint32
	ScoreLimit     *uint32
	Option2        *uint32
	Respawn        *uint32
	EngineUnion    *uint32
}

// CEBuild patches a real CE blam.lst template and returns the new 512-byte file.
// The digest and all unmapped bytes are preserved verbatim from the template.
//
// recompute selects the (currently unimplemented) digest hook; pass false for
// template-patch. If recompute is true and the hook is still a stub, CEBuild
// returns ErrDigestUnresolved.
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
	if p.Engine != nil {
		putU32(out, ceOffEngine, *p.Engine)
	}
	if p.Teams != nil {
		putU32(out, ceOffTeams, *p.Teams)
	}
	if p.Options != nil {
		putU32(out, ceOffOptions, *p.Options)
	}
	if p.ScoringSubtype != nil {
		putU32(out, ceOffScoringSub, *p.ScoringSubtype)
	}
	if p.TimeLimit != nil {
		putU32(out, ceOffTimeLimit, *p.TimeLimit)
	}
	if p.TimeLimit2 != nil {
		putU32(out, ceOffTimeLimit2, *p.TimeLimit2)
	}
	if p.ScoreLimit != nil {
		putU32(out, ceOffScoreLimit, *p.ScoreLimit)
	}
	if p.Option2 != nil {
		putU32(out, ceOffOption2, *p.Option2)
	}
	if p.Respawn != nil {
		putU32(out, ceOffRespawn, *p.Respawn)
	}
	if p.EngineUnion != nil {
		putU32(out, ceOffEngineUnion, *p.EngineUnion)
	}
	if recompute {
		dg, err := RecomputeDigest(out, ceOffDigest, ceDigestLen, "ce")
		if err != nil {
			return nil, err
		}
		copy(out[ceOffDigest:ceOffDigest+ceDigestLen], dg)
	}
	return out, nil
}

// CEMinutesToRaw converts a friendly minute value to the raw time-limit field.
// Observed: raw 300 labelled "10 min", 225 "7.5", 150 "5" => unit = 2 s.
//
// NOTE: the de-risk flagged this scale as inferred from value patterns, not
// confirmed in-game. The generator stores raw values, so cloning is unaffected;
// only the human-friendly label depends on this.
func CEMinutesToRaw(minutes float64) uint32 {
	return uint32(math.Round(minutes * 30))
}

// CERawToMinutes is the inverse of CEMinutesToRaw.
func CERawToMinutes(raw uint32) float64 {
	return float64(raw) / 30.0
}

// CE options bitfield helpers (see FORMATS.md §2). Base value is 0x22.
const (
	ceOptBase     = 0x22
	ceOptRadarBit = 0x01 // the "R" suffix (radar/respawn variants)
	ceOptSnipers  = 0x10 // snipers variants set this bit
	ceOptPractice = 0x04 // practice/race variants set this bit
)
