package halosave

import "fmt"

// Halo 2 gametype — payload named after the mode (e.g. `slayer`), variable size
// (the one sample, "Slayer: team brs", is 324 bytes). See FORMATS.md §4.
//
//	0x00  u32           version (00 00 00 01)
//	0x04  UTF-16LE       name, NUL-terminated
//	0x50  u32            score limit (0x32 = 50 in the slayer sample)
//	last 20 bytes        trailing digest
//
// Only one H2 gametype sample was available, so the field map is partial:
// name + score limit are mapped; other settings (time, mode flags) are not yet
// diff-localised. The file is otherwise preserved from the template.
const (
	h2gtOffVersion    = 0x00
	h2gtOffName       = 0x04
	h2gtNameBuf       = 0x40
	h2gtOffScoreLimit = 0x50
	h2gtDigestLen     = 20
)

// H2Gametype is the parsed view of an H2 gametype payload.
type H2Gametype struct {
	Raw        []byte `json:"-"`
	Version    uint32 `json:"version"`
	Name       string `json:"name"`
	ScoreLimit uint32 `json:"score_limit"`
	Digest     []byte `json:"digest"`
}

// H2GametypeParse decodes an H2 gametype payload. The digest is the trailing 20
// bytes, so the slice must be at least that long plus the score-limit field.
func H2GametypeParse(b []byte) (*H2Gametype, error) {
	if len(b) < h2gtOffScoreLimit+4+h2gtDigestLen {
		return nil, fmt.Errorf("halosave: H2 gametype too short (%d bytes)", len(b))
	}
	g := &H2Gametype{
		Raw:        append([]byte(nil), b...),
		Version:    getU32(b, h2gtOffVersion),
		Name:       readUTF16z(b, h2gtOffName, h2gtNameBuf),
		ScoreLimit: getU32(b, h2gtOffScoreLimit),
		Digest:     append([]byte(nil), b[len(b)-h2gtDigestLen:]...),
	}
	return g, nil
}

// H2GametypePatch holds the fields to overwrite on an H2 gametype template.
type H2GametypePatch struct {
	Name       *string
	ScoreLimit *uint32
}

// H2GametypeBuild patches a real H2 gametype template, preserving the digest and
// all unmapped bytes. See CEBuild for recompute semantics.
func H2GametypeBuild(template []byte, p H2GametypePatch, recompute bool) ([]byte, error) {
	if len(template) < h2gtOffScoreLimit+4+h2gtDigestLen {
		return nil, fmt.Errorf("halosave: H2 gametype template too short (%d bytes)", len(template))
	}
	out := append([]byte(nil), template...)
	if p.Name != nil {
		if err := writeUTF16z(out, h2gtOffName, h2gtNameBuf, *p.Name); err != nil {
			return nil, err
		}
	}
	if p.ScoreLimit != nil {
		putU32(out, h2gtOffScoreLimit, *p.ScoreLimit)
	}
	if recompute {
		off := len(out) - h2gtDigestLen
		dg, err := RecomputeDigest(out, off, h2gtDigestLen, "h2")
		if err != nil {
			return nil, err
		}
		copy(out[off:off+h2gtDigestLen], dg)
	}
	return out, nil
}
