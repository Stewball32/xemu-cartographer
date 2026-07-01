package halosave

import "fmt"

// Halo: CE (Xbox) player profile — `blam.sav`, fixed 512 bytes, little-endian.
// See docs/gamertag-system/CE-PROFILE-FORMAT.md. Only the first 48 bytes
// (0x00–0x2F) are signed/meaningful; the rest is uninitialised heap padding
// (don't-care, zeroed on generate). Signed via the SAME per-title roamable
// HMAC-SHA1 as the CE gametype — only the offset/length differ (profile signs
// [0:0x30] with the digest at 0x30; gametype signs [0:0x68] at 0x68).
//
//	0x00..0x17   UTF-16LE name, NUL-terminated, 24-byte buffer (≤11 chars)
//	0x18         u32  MP armor color enum (white=0, red=2, blue=3, …)
//	0x1C..0x27   12   advanced block (0 on fresh MP profiles)
//	0x28         u8   thumbstick layout preset (0=Default, 1=Southpaw)
//	0x29         u8   button layout preset     (0=Default, 1=Southpaw)
//	0x2A         u8   advanced (unmapped; 0x03 on fresh MP profiles)
//	0x2B..0x2F   5    reserved (0)
//	0x30..0x43   20   DIGEST — HMAC-SHA1 over [0:0x30]
//	0x44..0x1FF  444  unsigned tail (don't-care)
const (
	cepSize      = 0x200 // 512
	cepOffName   = 0x00
	cepNameBuf   = 0x18 // 24-byte name field (0x00..0x17)
	cepOffColor  = 0x18 // u32 MP armor color enum
	cepOffAdvLo  = 0x1C // advanced block start (signed, 0x1C..0x2F)
	cepOffThumb  = 0x28 // u8 thumbstick preset
	cepOffButton = 0x29 // u8 button preset
	cepOffAdv2A  = 0x2A // u8 advanced (unmapped)
	cepOffDigest = 0x30 // 20-byte HMAC over [0:0x30]
	cepDigestLen = 20

	// cepDefaultAdv2A is byte 0x2A on a freshly-created MP profile (observed
	// identical on the Stew + New001 samples). Otherwise unmapped.
	cepDefaultAdv2A = 0x03
)

// CEProfileField labels an editable CE profile field for the editor / meta.
type CEProfileField struct {
	Offset int    `json:"offset"`
	Size   int    `json:"size"`
	Key    string `json:"key"`
	Label  string `json:"label"`
}

// CEProfileFields is the mapped, editable surface of a CE profile (the rest of
// the signed region is advanced settings not yet individually mapped).
var CEProfileFields = []CEProfileField{
	{cepOffColor, 4, "color", "Armor color"},
	{cepOffThumb, 1, "thumbstick", "Thumbstick preset"},
	{cepOffButton, 1, "button", "Button preset"},
}

// CEProfile is the parsed view of a blam.sav.
type CEProfile struct {
	Raw        []byte `json:"-"`
	Name       string `json:"name"`
	Color      uint32 `json:"color"`
	Thumbstick uint8  `json:"thumbstick"`
	Button     uint8  `json:"button"`
	Advanced   []byte `json:"advanced"` // raw 0x1C..0x2F (signed; mostly unmapped)
	Digest     []byte `json:"digest"`
}

// CEProfileParse decodes a 512-byte blam.sav.
func CEProfileParse(b []byte) (*CEProfile, error) {
	if len(b) != cepSize {
		return nil, fmt.Errorf("halosave: CE profile must be %d bytes, got %d", cepSize, len(b))
	}
	return &CEProfile{
		Raw:        append([]byte(nil), b...),
		Name:       readUTF16z(b, cepOffName, cepNameBuf),
		Color:      getU32(b, cepOffColor),
		Thumbstick: b[cepOffThumb],
		Button:     b[cepOffButton],
		Advanced:   append([]byte(nil), b[cepOffAdvLo:cepOffDigest]...),
		Digest:     append([]byte(nil), b[cepOffDigest:cepOffDigest+cepDigestLen]...),
	}, nil
}

// CEProfilePatch holds the fields to set on a generated CE profile.
type CEProfilePatch struct {
	Name       *string
	Color      *uint32
	Thumbstick *uint8
	Button     *uint8
	// AdvPatch optionally overrides bytes in the advanced block 0x1C..0x2F (the
	// pluggable, not-yet-individually-mapped region) by absolute offset.
	AdvPatch map[int]byte
}

// CEProfileBuild constructs a 512-byte blam.sav. The signed region (0x00..0x2F)
// is fully specified — built from scratch with fresh-MP defaults, then the patch
// fields applied — and the unsigned tail is left zeroed (don't-care per the
// format spec). recompute=true always re-signs at 0x30 (a valid digest is
// mandatory; CE rejects an edited file with a stale one).
func CEProfileBuild(p CEProfilePatch, recompute bool) ([]byte, error) {
	out := make([]byte, cepSize)

	name := ""
	if p.Name != nil {
		name = *p.Name
	}
	if err := writeUTF16z(out, cepOffName, cepNameBuf, name); err != nil {
		return nil, err
	}
	if p.Color != nil {
		putU32(out, cepOffColor, *p.Color)
	}
	// Fresh-MP advanced default (0x1C..0x27 + 0x2B..0x2F stay 0).
	out[cepOffAdv2A] = cepDefaultAdv2A
	if p.Thumbstick != nil {
		out[cepOffThumb] = *p.Thumbstick
	}
	if p.Button != nil {
		out[cepOffButton] = *p.Button
	}
	for off, v := range p.AdvPatch {
		if off < cepOffAdvLo || off >= cepOffDigest {
			return nil, fmt.Errorf("halosave: CE advanced patch offset %#x out of 0x1C..0x2F range", off)
		}
		out[off] = v
	}

	if recompute {
		dg, err := RecomputeDigest(out, cepOffDigest, cepDigestLen, "ce")
		if err != nil {
			return nil, err
		}
		copy(out[cepOffDigest:cepOffDigest+cepDigestLen], dg)
	}
	return out, nil
}
