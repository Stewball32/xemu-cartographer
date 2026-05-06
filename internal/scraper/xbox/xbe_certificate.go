package xbox

import (
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// XBECertificate is a partial decode of the running XBE's on-disk certificate
// struct, every field of which is stable for the lifetime of that XBE in
// memory. Surfaced as a single read because the underlying QMP translation
// (XBE header is at low GVA 0x00010000) is the bulk of the cost; once the
// header is translated, all field reads land in the same already-mapped
// page.
type XBECertificate struct {
	TitleID      uint32 // raw u32 — 4-char publisher+game ID, e.g. 0x4D530102 = "MS-0102"
	TitleName    string // up to 40 wchars, decoded UTF-16LE, e.g. "Halo: Combat Evolved"
	Version      uint32 // raw kernel version dword (developer-defined, varies by title)
	GameRegion   uint32 // bitfield, see XBEGameRegion* constants
	GameRatings  uint32 // ESRB bitfield (raw — surface for future decoding)
	AllowedMedia uint32 // bitfield: HD/DVD/CD/etc. (raw)
	DiskNumber   uint32 // multi-disc disc index (usually 0 or 1)
}

// ReadXBECertificate reads the running XBE's certificate via the kernel's
// fixed XBE-header GVA. Always re-translates the low GVA before reading
// because the kernel re-pages the XBE on every XBE swap (dashboard → game,
// game → game, game → dashboard) — without the refresh we'd read stale
// bytes from the previous XBE's image.
//
// Returns ok=false on translation / read errors; the typical cause is the
// kernel hasn't mapped an XBE yet (very early boot, between XBE swaps).
// Caller is responsible for caching — nothing is cached inside.
func ReadXBECertificate(inst *xemu.Instance) (XBECertificate, bool) {
	if inst == nil || inst.Mem == nil {
		return XBECertificate{}, false
	}
	headerHVA, err := inst.RefreshLowHVA(XBEHeaderGVA)
	if err != nil {
		return XBECertificate{}, false
	}
	certPtr, err := inst.Mem.ReadU32At(headerHVA + int64(XBEOffCertPtr))
	if err != nil {
		return XBECertificate{}, false
	}

	// Certificate pointer can be expressed as either a low GVA (relative to
	// header base) or a high GVA (kernel-mapped image). Mirror the existing
	// branch in internal/scraper/ReadTitleID so we don't drift.
	var certHVA int64
	if certPtr < 0x80000000 {
		certHVA = headerHVA + int64(certPtr) - int64(XBEHeaderGVA)
	} else {
		certHVA = inst.Mem.HighGVA(certPtr)
	}

	titleID, err := inst.Mem.ReadU32At(certHVA + int64(XBECertOffTitleID))
	if err != nil {
		return XBECertificate{}, false
	}
	nameBytes, err := inst.Mem.ReadBytesAt(certHVA+int64(XBECertOffTitleName), LenXBETitleName)
	if err != nil {
		return XBECertificate{}, false
	}
	allowedMedia, err := inst.Mem.ReadU32At(certHVA + int64(XBECertOffAllowedMedia))
	if err != nil {
		return XBECertificate{}, false
	}
	gameRegion, err := inst.Mem.ReadU32At(certHVA + int64(XBECertOffGameRegion))
	if err != nil {
		return XBECertificate{}, false
	}
	gameRatings, err := inst.Mem.ReadU32At(certHVA + int64(XBECertOffGameRatings))
	if err != nil {
		return XBECertificate{}, false
	}
	diskNumber, err := inst.Mem.ReadU32At(certHVA + int64(XBECertOffDiskNumber))
	if err != nil {
		return XBECertificate{}, false
	}
	version, err := inst.Mem.ReadU32At(certHVA + int64(XBECertOffVersion))
	if err != nil {
		return XBECertificate{}, false
	}

	return XBECertificate{
		TitleID:      titleID,
		TitleName:    strings.TrimSpace(DecodeUTF16LEBounded(nameBytes, LenXBETitleName/2)),
		Version:      version,
		GameRegion:   gameRegion,
		GameRatings:  gameRatings,
		AllowedMedia: allowedMedia,
		DiskNumber:   diskNumber,
	}, true
}

// FormatGameRegion produces a short human-readable label from the GameRegion
// bitfield. Common combinations (US-only, JP-only, World) get their own
// label; anything else falls back to a comma-joined list. Returns "" for 0
// (no region set — uncommon outside specific homebrew).
func FormatGameRegion(r uint32) string {
	if r == 0 {
		return ""
	}
	if r == XBEGameRegionAnyRegion {
		return "World"
	}
	parts := make([]string, 0, 4)
	if r&XBEGameRegionNTSCUS != 0 {
		parts = append(parts, "NTSC-US")
	}
	if r&XBEGameRegionNTSCJ != 0 {
		parts = append(parts, "NTSC-J")
	}
	if r&XBEGameRegionPAL != 0 {
		parts = append(parts, "PAL")
	}
	if r&XBEGameRegionDevkit != 0 {
		parts = append(parts, "Devkit")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ",")
}
