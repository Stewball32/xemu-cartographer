package xbox

// ConsoleNameRecordMagic is the 4-byte sentinel the OG Xbox kernel/dashboard
// writes immediately before each record in a loaded copy of E:\UDATA\NICKNAME.XBN.
// The on-disk file format keeps its layout intact in heap, so any record (slot 0
// = console name, slots 1..N = per-controller nicknames) can be located by
// scanning for this magic.
//
// The 4 bytes preceding the magic are always zero in genuine records — both
// in NICKNAME.XBN's file format (last 4 bytes of the 8-byte reserved header
// before the first record, and zero-padding between successive records) and
// in the dashboard's clean kernel-cache copies (the buffer is zero-initialised
// before parsing the file in). This lets us discriminate against the same
// 4-byte magic value appearing as an immediate or displacement constant in
// kernel/dashboard image code, where preceding bytes are never four zeros.
const ConsoleNameRecordMagic uint32 = 0x9E115330

// ConsoleNameMaxBytes caps each record's UTF-16LE name field at 32 bytes
// (16 wchars), matching the OG Xbox console-name length limit.
const ConsoleNameMaxBytes int = 32

// EEPROM-derived globals — kernel-decoded fields living at fixed GVAs in the
// OG Xbox kernel image. Verified consistent across xemu containers; values
// are populated once at boot from the decrypted EEPROM blob and don't move.
//
// These are NOT a 1:1 dump of the on-disk EEPROM format — the kernel decodes
// the encrypted user section into individual fields and lays them out in its
// own static struct. The offsets below reference that internal layout.
//
// Most fields are per-machine (different across xemu containers because each
// gets a fresh randomly-generated EEPROM); a few (TimeZoneStdName etc.) are
// per-locale and identical across containers configured with the same region.
const (
	// RefAddrSerialNumber: 12 ASCII bytes, no null terminator. Padded with
	// trailing spaces / zeros if the actual serial is shorter.
	RefAddrSerialNumber uint32 = 0x80056F74
	LenSerialNumber     int    = 12

	// RefAddrMACAddress: 6 bytes in network byte order (OUI first). xemu's
	// default OUI is 00:50:F2 (Microsoft retail).
	RefAddrMACAddress uint32 = 0x80056F80
	LenMACAddress     int    = 6

	// RefAddrOnlineKey: 16 bytes, per-machine Xbox Live online key. Sensitive —
	// readers exist for completeness but should never be surfaced to clients.
	RefAddrOnlineKey uint32 = 0x80056F88
	LenOnlineKey     int    = 16

	// RefAddrVideoStandard: 4-byte LE u32. Compare against XC_VIDEO_STANDARD_*
	// constants below.
	RefAddrVideoStandard uint32 = 0x80056F98

	// RefAddrTimeZoneBias: 4-byte LE int32, UTC offset in minutes (positive
	// for west of UTC, negative for east, per Microsoft's TIME_ZONE_INFORMATION
	// convention).
	RefAddrTimeZoneBias uint32 = 0x80056F9C

	// RefAddrTimeZoneStdName / RefAddrTimeZoneDltName: 4 ASCII bytes each
	// (null-padded), e.g. "GMT", "BST", "EST", "EDT", "PST", "PDT".
	RefAddrTimeZoneStdName uint32 = 0x80056FA8
	RefAddrTimeZoneDltName uint32 = 0x80056FAC
	LenTimeZoneName        int    = 4
)

// XC_VIDEO_STANDARD_* are the documented kernel constants for the
// VideoStandard field at RefAddrVideoStandard. Mirror the public
// OpenXbox / Cxbx-Reloaded names so cross-referencing is trivial.
const (
	VideoStandardNTSCM uint32 = 0x00400100
	VideoStandardNTSCJ uint32 = 0x00400200
	VideoStandardPALI  uint32 = 0x00800300
	VideoStandardPALM  uint32 = 0x00400400
)

// XBE certificate constants — every XBE binary has a certificate at a fixed
// offset within its loaded image. The OG Xbox kernel maps the running XBE
// header at GVA 0x00010000 (low GVA, requires QMP translation; already
// translated for any runner via DetectionGVAs() in internal/scraper).
//
// The certificate pointer at xbeOffCertPtr is interpreted relative to either
// the header (if low) or as a direct high GVA — see ReadXBECertificate.
const (
	// XBEHeaderGVA is the kernel-fixed load address for the running XBE.
	XBEHeaderGVA uint32 = 0x00010000

	// XBEOffCertPtr: u32 within the XBE header pointing at the certificate.
	XBEOffCertPtr uint32 = 0x0118

	// XBE certificate field offsets — relative to the certificate base.
	XBECertOffSize         uint32 = 0x000
	XBECertOffTimeDate     uint32 = 0x004
	XBECertOffTitleID      uint32 = 0x008
	XBECertOffTitleName    uint32 = 0x00C // UTF-16LE, 40 wchars / 80 bytes
	XBECertOffAllowedMedia uint32 = 0x09C
	XBECertOffGameRegion   uint32 = 0x0A0
	XBECertOffGameRatings  uint32 = 0x0A4
	XBECertOffDiskNumber   uint32 = 0x0A8
	XBECertOffVersion      uint32 = 0x0AC

	// LenXBETitleName: the certificate reserves 80 bytes for TitleName but
	// values are usually shorter and null-terminated.
	LenXBETitleName int = 80
)

// XBE_GAME_REGION_* — game-region bitfield values for the GameRegion field.
// Multiple bits may be set on multi-region releases.
const (
	XBEGameRegionNTSCUS    uint32 = 0x00000001
	XBEGameRegionNTSCJ     uint32 = 0x00000002
	XBEGameRegionPAL       uint32 = 0x00000004
	XBEGameRegionDevkit    uint32 = 0x80000000
	XBEGameRegionAnyRegion uint32 = 0x7FFFFFFF
)

// Kernel clock globals — verified by delta-sampling all 4 running containers
// and observing ~10,000,000 / sec on these GVAs (FILETIME 100ns tick rate).
//
// Both are 8-byte little-endian u64s. KeSystemTime is the wall-clock UTC time
// since the FILETIME epoch (1601-01-01 00:00:00 UTC); cross-container low-u32
// values land within seconds of each other (RTT skew). KeInterruptTime is
// the elapsed time since guest boot; cross-container low-u32 values vary by
// minutes (each xemu booted at a slightly different wall time).
const (
	RefAddrKeSystemTime    uint32 = 0x800463F0
	RefAddrKeInterruptTime uint32 = 0x800463A0
)

// FileTimeEpochOffsetTicks is the number of 100ns ticks between the FILETIME
// epoch (1601-01-01 UTC) and the Unix epoch (1970-01-01 UTC). Used to convert
// a kernel FILETIME u64 to a Go time.Time.
const FileTimeEpochOffsetTicks uint64 = 116444736000000000
