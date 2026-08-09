package halosave

import (
	"fmt"
	"math"
)

// Halo: CE (Xbox) player profile — `blam.sav`, fixed 512 bytes, little-endian.
//
// Field map is the 2026-08-07 LIVE-VERIFIED profile spec (bench-2, H1
// Performance Build): every setting isolated by single-setting differential
// saves in the in-game profile editor, with a boot-time LOAD-BACK. This
// SUPERSEDES the earlier spec, which (1) had 0x28/0x29 SWAPPED and (2) left the
// nine Advanced Controls unmapped. Signed via the SAME per-title roamable
// HMAC-SHA1 as the CE gametype — profile signs [0:0x30] with the digest at 0x30.
//
//	0x00..0x17   UTF-16LE name, NUL-terminated, 24-byte buffer (≤11 chars)
//	0x18         u32  MP armor color enum (white=0, red=2, blue=3, …; 18 colors)
//	0x1C..0x27   12   unused (0 on MP profiles)
//	0x28         u8   BUTTON preset  (0 Default,1 Southpaw,2 Jumpy,3 Boxer,4 Green Thumb)
//	0x29         u8   THUMBSTICK preset (0 Default,1 Southpaw,2 Legacy,3 Legacy Southpaw)
//	0x2A..0x2F   6    Advanced Controls, bit-packed (see codec below)
//	0x30..0x43   20   DIGEST — HMAC-SHA1 over [0:0x30]
//	0x44..0x1FF  444  unsigned tail (don't-care, zeroed on generate)
//
// Advanced Controls packing (0x2A..0x2F):
//
//	0x2A  HSENS  — LOAD map (used when BUILDING): 0x2A = idx+1, idx=(value-1.0)/0.25
//	0x2B  INVERT THUMBSTICK  (0 NO, 1 YES)
//	0x2C  VIBRATION          (0 YES, 1 NO)
//	0x2D  bits0-4 = RSDZ code (rsdz+1) lo5 ; bits5-7 = OUTER code (15-outer) lo3
//	0x2E  bits0-5 = LSDZ code (lsdz+1)     ; bit6 = OUTER code bit3 ; bit7 = DZTYPE (1 AXIAL, 0 RADIAL)
//	0x2F  bits0-3 = VMULT code ((v-0.5)/0.05 +1) ; bits4-6 = RESPONSE code ; bit7 = RSDZ code bit5
const (
	cepSize        = 0x200 // 512
	cepOffName     = 0x00
	cepNameBuf     = 0x18 // 24-byte name field (0x00..0x17)
	cepOffColor    = 0x18 // u32 MP armor color enum
	cepOffButton   = 0x28 // u8 button preset (FIX: was thumbstick)
	cepOffThumb    = 0x29 // u8 thumbstick preset (FIX: was button)
	cepOffAdvanced = 0x2A // 6-byte packed advanced controls (0x2A..0x2F)
	cepOffDigest   = 0x30 // 20-byte HMAC over [0:0x30]
	cepDigestLen   = 20
)

// CE response-curve codes (0x2F bits4-6).
const (
	CEResponseLinear     = 1
	CEResponseGentle     = 2
	CEResponseMild       = 3
	CEResponseDefault    = 4
	CEResponseSharp      = 5
	CEResponseAggressive = 6
	CEResponseExtreme    = 7
)

// CEAdvanced is the friendly view of the nine Advanced Controls at 0x2A..0x2F.
type CEAdvanced struct {
	HSens         float64 `json:"h_sens"`         // 1.00..10.00 step 0.25
	VMult         float64 `json:"v_mult"`         // 0.50..1.00 step 0.05
	Invert        bool    `json:"invert"`         // invert thumbstick
	Vibration     bool    `json:"vibration"`      // rumble
	RSDeadzone    uint8   `json:"rs_deadzone"`    // 0..35
	LSDeadzone    uint8   `json:"ls_deadzone"`    // 0..35
	OuterDeadzone uint8   `json:"outer_deadzone"` // 1..15
	DeadzoneType  uint8   `json:"deadzone_type"`  // 0 radial, 1 axial
	Response      uint8   `json:"response"`       // 1..7 (see CEResponse*)
}

// ceAdvancedDefault is a fresh MP profile's factory Advanced Controls (spec §3).
func ceAdvancedDefault() CEAdvanced {
	return CEAdvanced{
		HSens: 3, VMult: 0.5, Invert: false, Vibration: true,
		RSDeadzone: 27, LSDeadzone: 35, OuterDeadzone: 15,
		DeadzoneType: 1 /* axial */, Response: CEResponseDefault,
	}
}

// encodeCEAdvanced packs a CEAdvanced into the 6 bytes at 0x2A..0x2F.
func encodeCEAdvanced(a CEAdvanced) [6]byte {
	var bb [6]byte
	hidx := int(math.Round((a.HSens - 1.0) / 0.25))
	bb[0] = byte(hidx + 1) // 0x2A HSENS load map
	if a.Invert {
		bb[1] = 1 // 0x2B
	}
	if !a.Vibration {
		bb[2] = 1 // 0x2C (0=YES, 1=NO)
	}
	rs := int(a.RSDeadzone) + 1
	ls := int(a.LSDeadzone) + 1
	ot := 15 - int(a.OuterDeadzone)
	vm := int(math.Round((a.VMult-0.5)/0.05)) + 1
	rc := int(a.Response)
	dz := 0
	if a.DeadzoneType == 1 {
		dz = 1
	}
	bb[3] = byte((rs & 0x1f) | ((ot & 0x07) << 5))                          // 0x2D
	bb[4] = byte((ls & 0x3f) | (((ot >> 3) & 1) << 6) | (dz << 7))          // 0x2E
	bb[5] = byte((vm & 0x0f) | ((rc & 0x07) << 4) | (((rs >> 5) & 1) << 7)) // 0x2F
	return bb
}

// decodeCEAdvanced unpacks the 6 bytes at 0x2A..0x2F.
func decodeCEAdvanced(b []byte) CEAdvanced {
	a, inv, vib := int(b[0]), b[1], b[2]
	d, e, f := int(b[3]), int(b[4]), int(b[5])
	rs := (d & 0x1f) | (((f >> 7) & 1) << 5)
	ot := ((d >> 5) & 7) | (((e >> 6) & 1) << 3)
	ls := e & 0x3f
	vm := f & 0x0f
	rc := (f >> 4) & 7
	dz := uint8(0)
	if (e>>7)&1 == 1 {
		dz = 1
	}
	return CEAdvanced{
		HSens:         1.0 + 0.25*float64(a-1),
		VMult:         math.Round((0.5+0.05*float64(vm-1))*100) / 100,
		Invert:        inv != 0,
		Vibration:     vib == 0,
		RSDeadzone:    uint8(rs - 1),
		LSDeadzone:    uint8(ls - 1),
		OuterDeadzone: uint8(15 - ot),
		DeadzoneType:  dz,
		Response:      uint8(rc),
	}
}

// CEProfile is the parsed view of a blam.sav.
type CEProfile struct {
	Raw        []byte     `json:"-"`
	Name       string     `json:"name"`
	Color      uint32     `json:"color"`
	Button     uint8      `json:"button"`
	Thumbstick uint8      `json:"thumbstick"`
	Advanced   CEAdvanced `json:"advanced"`
	Digest     []byte     `json:"digest"`
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
		Button:     b[cepOffButton],
		Thumbstick: b[cepOffThumb],
		Advanced:   decodeCEAdvanced(b[cepOffAdvanced:cepOffDigest]),
		Digest:     append([]byte(nil), b[cepOffDigest:cepOffDigest+cepDigestLen]...),
	}, nil
}

// CEProfilePatch holds the fields to set on a generated CE profile. Nil fields
// take the fresh-MP factory default (Advanced likewise when nil).
type CEProfilePatch struct {
	Name       *string
	Color      *uint32
	Button     *uint8
	Thumbstick *uint8
	Advanced   *CEAdvanced
}

// CEProfileBuild constructs a 512-byte blam.sav. The signed region (0x00..0x2F)
// is fully specified — factory defaults, then the patch applied — and the
// unsigned tail is left zeroed. recompute=true re-signs at 0x30 (mandatory; CE
// rejects a stale signature as "damaged").
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
	if p.Button != nil {
		out[cepOffButton] = *p.Button
	}
	if p.Thumbstick != nil {
		out[cepOffThumb] = *p.Thumbstick
	}
	adv := ceAdvancedDefault()
	if p.Advanced != nil {
		adv = *p.Advanced
	}
	enc := encodeCEAdvanced(adv)
	copy(out[cepOffAdvanced:cepOffAdvanced+6], enc[:])

	if recompute {
		dg, err := RecomputeDigest(out, cepOffDigest, cepDigestLen, "ce")
		if err != nil {
			return nil, err
		}
		copy(out[cepOffDigest:cepOffDigest+cepDigestLen], dg)
	}
	return out, nil
}

// CE profile editor SCHEMA — same shape as the gametype schema (CEField), so the
// creator UI renders both from the backend. Keys match the CE-profile fields on
// BuildRequest / saveartifact.CEProfileSettings.

// CE profile editor sections.
const (
	CEPSectionAppearance = "appearance"
	CEPSectionController = "controller"
	CEPSectionAdvanced   = "advanced"
)

// CEProfileSections returns the profile editor sections in display order.
func CEProfileSections() []CESection {
	return []CESection{
		{CEPSectionAppearance, "Appearance"},
		{CEPSectionController, "Controller"},
		{CEPSectionAdvanced, "Advanced Controls"},
	}
}

// CEProfileSchema returns the editable CE player-profile field schema.
func CEProfileSchema() []CEField {
	return []CEField{
		{Key: "color", Label: "Armor color", Kind: CEFieldEnum, Section: CEPSectionAppearance,
			Options: enumOpts(0, "White", 1, "Black", 2, "Red", 3, "Blue", 4, "Gray", 5, "Yellow",
				6, "Green", 7, "Pink", 8, "Purple", 9, "Cyan", 10, "Cobalt", 11, "Orange",
				12, "Teal", 13, "Sage", 14, "Brown", 15, "Tan", 16, "Maroon", 17, "Salmon")},

		{Key: "button", Label: "Button preset", Kind: CEFieldEnum, Section: CEPSectionController,
			Options: enumOpts(0, "Default", 1, "Southpaw", 2, "Jumpy", 3, "Boxer", 4, "Green Thumb")},
		{Key: "thumbstick", Label: "Thumbstick preset", Kind: CEFieldEnum, Section: CEPSectionController,
			Options: enumOpts(0, "Default", 1, "Southpaw", 2, "Legacy", 3, "Legacy Southpaw")},

		{Key: "h_sens", Label: "Horizontal sensitivity", Kind: CEFieldFloat, Section: CEPSectionAdvanced,
			Min: fptr(1), Max: fptr(10), Step: fptr(0.25), Default: fptr(3)},
		{Key: "v_mult", Label: "Vertical multiplier", Kind: CEFieldFloat, Section: CEPSectionAdvanced,
			Min: fptr(0.5), Max: fptr(1), Step: fptr(0.05), Default: fptr(0.5)},
		{Key: "invert", Label: "Invert thumbstick", Kind: CEFieldBool, Section: CEPSectionAdvanced},
		{Key: "vibration", Label: "Vibration", Kind: CEFieldBool, Section: CEPSectionAdvanced},
		{Key: "rs_deadzone", Label: "Right stick deadzone", Kind: CEFieldInt, Section: CEPSectionAdvanced,
			Min: fptr(0), Max: fptr(35), Step: fptr(1), Default: fptr(27)},
		{Key: "ls_deadzone", Label: "Left stick deadzone", Kind: CEFieldInt, Section: CEPSectionAdvanced,
			Min: fptr(0), Max: fptr(35), Step: fptr(1), Default: fptr(35)},
		{Key: "outer_deadzone", Label: "Outer deadzone", Kind: CEFieldInt, Section: CEPSectionAdvanced,
			Min: fptr(1), Max: fptr(15), Step: fptr(1), Default: fptr(15)},
		{Key: "deadzone_type", Label: "Deadzone type", Kind: CEFieldEnum, Section: CEPSectionAdvanced,
			Options: enumOpts(0, "Radial", 1, "Axial")},
		{Key: "response", Label: "Response curve", Kind: CEFieldEnum, Section: CEPSectionAdvanced,
			Options: enumOpts(1, "Linear", 2, "Gentle", 3, "Mild", 4, "Default", 5, "Sharp",
				6, "Aggressive", 7, "Extreme")},
	}
}
