package halosave

import "fmt"

// Halo 2 player profile — `profile`, fixed 500 bytes (see FORMATS.md §5).
//
//	0x00..0x07            header (zero)
//	0x08..0xEB            UTF-16LE player name, NUL-terminated (0xE4-byte field)
//	0xEC..0x117          per-profile sub-block: a fixed 16-byte 0xFF field
//	                      (0xEC..0xFB, identical in every real sample) plus
//	                      bytes at 0xFC/0x101/0x102 that vary per profile. NOT
//	                      part of the name — preserved verbatim from the template.
//	0x118..0x11F          appearance: armor primary/secondary + emblem primary/
//	                      secondary colour, character model, emblem foreground/
//	                      background, flags (see H2ProfileFields)
//	0x120..0x12F          controller block (per-profile bytes)
//	0x1E0..0x1F3          20-byte trailing digest (HMAC signature; see digest.go)
//
// The name field ends at 0xEC, NOT 0x118. An earlier map treated the whole
// 0x08..0x118 region as the name buffer; writing a name then zero-filled
// 0xEC..0x117 and clobbered the fixed 0xFF field + the per-profile bytes there.
// Cross-checked against 4 real profiles: bytes 0xEC..0xFB are 0xFF in all of
// them and 0xFC/0x101/0x102 carry real data — so h2pNameBuf stops at 0xEC.
//
// The emblem/appearance bytes (0x118..0x11F) are CONFIRMED against the in-game
// s_player_profile struct and two real profiles; the controller bytes
// (0x12B/0x12F) remain provisional. The block is exposed raw so it can be copied
// from a template or patched byte-wise; named single-byte accessors are a
// convenience layered on top.
const (
	h2pSize      = 0x1F4 // 500
	h2pOffName   = 0x08
	h2pNameBuf   = 0xE4 // name field 0x08..0xEB; stops before the 0xEC sub-block
	h2pOffAppctl = 0x118
	h2pAppctlLen = 0x18
	h2pOffDigest = 0x1E0
	h2pDigestLen = 20
)

// H2ProfileField labels the provisional single-byte appearance/controller
// fields within the 0x118 block. Offsets are absolute file offsets.
type H2ProfileField struct {
	Offset int    `json:"offset"`
	Key    string `json:"key"`
	Label  string `json:"label"`
}

// H2ProfileFields maps the appearance/emblem/controller bytes in the 0x118
// block. The emblem/colour bytes (0x118..0x11F) are CONFIRMED: they decode
// cleanly against the in-game `s_player_profile` struct — primary/secondary
// armor colour, tertiary/quaternary emblem colour, character model, emblem
// foreground symbol, emblem background shape, flags — and were verified
// byte-for-byte against two real captured profiles (587C…/E4CADA…). Colours are
// e_player_color (0..17); foreground 0..63; background 0..31; character 0..3.
// The controller bytes (0x12B/0x12F) remain provisional. See
// docs/gamertag-system/H2-EMBLEM-FORMAT.md.
var H2ProfileFields = []H2ProfileField{
	{0x118, "armor_primary", "Armor primary color (0-17)"},
	{0x119, "armor_secondary", "Armor secondary color (0-17)"},
	{0x11A, "emblem_primary", "Emblem primary color (0-17)"},
	{0x11B, "emblem_secondary", "Emblem secondary color (0-17)"},
	{0x11C, "character_type", "Character model (0=MasterChief 1=Dervish 2=Spartan 3=Elite)"},
	{0x11D, "emblem_foreground", "Emblem foreground symbol (0-63)"},
	{0x11E, "emblem_background", "Emblem background shape (0-31)"},
	{0x11F, "emblem_flags", "Emblem flags"},
	{0x12B, "ctrl_a", "Controller setting A?"},
	{0x12F, "ctrl_b", "Controller setting B?"},
}

// H2Profile is the parsed view of a `profile` file.
type H2Profile struct {
	Raw    []byte         `json:"-"`
	Name   string         `json:"name"`
	Appctl []byte         `json:"appctl"`        // raw 0x118 block
	Fields map[string]int `json:"appctl_fields"` // provisional named bytes
	Digest []byte         `json:"digest"`
}

// H2ProfileParse decodes a 500-byte H2 profile.
func H2ProfileParse(b []byte) (*H2Profile, error) {
	if len(b) != h2pSize {
		return nil, fmt.Errorf("halosave: H2 profile must be %d bytes, got %d", h2pSize, len(b))
	}
	p := &H2Profile{
		Raw:    append([]byte(nil), b...),
		Name:   readUTF16z(b, h2pOffName, h2pNameBuf),
		Appctl: append([]byte(nil), b[h2pOffAppctl:h2pOffAppctl+h2pAppctlLen]...),
		Fields: map[string]int{},
		Digest: append([]byte(nil), b[h2pOffDigest:h2pOffDigest+h2pDigestLen]...),
	}
	for _, f := range H2ProfileFields {
		p.Fields[f.Key] = int(b[f.Offset])
	}
	return p, nil
}

// H2ProfilePatch holds the fields to overwrite on an H2 profile template.
type H2ProfilePatch struct {
	Name *string // new player name
	// Appctl, if non-nil, replaces the whole 0x118 block (must be h2pAppctlLen).
	Appctl []byte
	// AppctlPatch holds single-byte patches keyed by ABSOLUTE file offset
	// (e.g. {0x118: 7} to set the primary armor color). Applied after Appctl.
	AppctlPatch map[int]byte
}

// H2ProfileBuild patches a real H2 profile template. The digest and all other
// bytes are preserved verbatim. See CEBuild for the recompute semantics.
func H2ProfileBuild(template []byte, p H2ProfilePatch, recompute bool) ([]byte, error) {
	if len(template) != h2pSize {
		return nil, fmt.Errorf("halosave: H2 profile template must be %d bytes, got %d", h2pSize, len(template))
	}
	out := append([]byte(nil), template...)
	if p.Name != nil {
		if err := writeUTF16z(out, h2pOffName, h2pNameBuf, *p.Name); err != nil {
			return nil, err
		}
	}
	if p.Appctl != nil {
		if len(p.Appctl) != h2pAppctlLen {
			return nil, fmt.Errorf("halosave: H2 appctl block must be %d bytes, got %d", h2pAppctlLen, len(p.Appctl))
		}
		copy(out[h2pOffAppctl:h2pOffAppctl+h2pAppctlLen], p.Appctl)
	}
	for off, val := range p.AppctlPatch {
		if off < 0 || off >= h2pSize {
			return nil, fmt.Errorf("halosave: appctl patch offset %#x out of range", off)
		}
		out[off] = val
	}
	if recompute {
		dg, err := RecomputeDigest(out, h2pOffDigest, h2pDigestLen, "h2")
		if err != nil {
			return nil, err
		}
		copy(out[h2pOffDigest:h2pOffDigest+h2pDigestLen], dg)
	}
	return out, nil
}
