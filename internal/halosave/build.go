package halosave

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
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

	// CE gametype — raw mapped fields (authoritative) + friendly helpers.
	Engine         string   `json:"engine,omitempty"` // slayer/ctf/oddball/king/race
	Teams          *bool    `json:"teams,omitempty"`
	Options        *uint32  `json:"options,omitempty"`
	ScoringSubtype *uint32  `json:"scoring_subtype,omitempty"`
	TimeLimit      *uint32  `json:"time_limit,omitempty"`   // raw; wins over TimeMinutes
	TimeMinutes    *float64 `json:"time_minutes,omitempty"` // friendly (inferred ×30 scale)
	TimeLimit2     *uint32  `json:"time_limit2,omitempty"`
	ScoreLimit     *uint32  `json:"score_limit,omitempty"` // also H2 gametype score
	Option2        *uint32  `json:"option2,omitempty"`
	Respawn        *uint32  `json:"respawn,omitempty"`
	EngineUnion    *uint32  `json:"engine_union,omitempty"`
	Radar          *bool    `json:"radar,omitempty"` // helper: toggles options bit0 ("R")

	// H2 profile — appearance/controller bytes keyed by H2ProfileFields.Key.
	Appearance map[string]int `json:"appearance,omitempty"`

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
		if req.Kind != KindGametype {
			return nil, fmt.Errorf("halosave: Halo: CE has no editable %q save — only gametypes (CE has no standalone MP profile file)", req.Kind)
		}
		return buildCEGametype(req)
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
	if req.ScoringSubtype != nil {
		p.ScoringSubtype = req.ScoringSubtype
	}
	if req.TimeLimit != nil {
		p.TimeLimit = req.TimeLimit
	} else if req.TimeMinutes != nil {
		p.TimeLimit = u32ptr(CEMinutesToRaw(*req.TimeMinutes))
	}
	if req.TimeLimit2 != nil {
		p.TimeLimit2 = req.TimeLimit2
	}
	if req.ScoreLimit != nil {
		p.ScoreLimit = req.ScoreLimit
	}
	if req.Option2 != nil {
		p.Option2 = req.Option2
	}
	if req.Respawn != nil {
		p.Respawn = req.Respawn
	}
	if req.EngineUnion != nil {
		p.EngineUnion = req.EngineUnion
	}
	// Options resolution: explicit raw Options wins; otherwise start from the
	// template's options. The Radar helper then toggles bit0 on the resolved
	// value.
	opts := base.Options
	if req.Options != nil {
		opts = *req.Options
	}
	if req.Radar != nil {
		if *req.Radar {
			opts |= ceOptRadarBit
		} else {
			opts &^= ceOptRadarBit
		}
	}
	if req.Options != nil || req.Radar != nil {
		p.Options = u32ptr(opts)
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

func ceWarnings(edited bool) []string {
	if edited {
		return []string{
			"CE time-limit unit (raw = minutes×30) is inferred from value patterns, not confirmed in-game; raw values are stored verbatim.",
		}
	}
	return nil
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
