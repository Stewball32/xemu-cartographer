// This file holds the offsets HaloCaster reads that we deliberately do NOT
// wire into the active scraper. Originally this file collected every unread
// offset; the "scrape every non-Broken offset" pass moved all of those into
// offsets.go. What remains here is the small set HaloCaster itself flagged as
// commented-out / known-broken — kept for completeness so future audits can
// see the exact provenance, but never read at runtime.
//
// Each constant is suffixed `Broken` to make the unread intent explicit at the
// call site. To revisit one of these (say, after M7 verification), move it
// into offsets.go and rename without the suffix.

package haloce

// ============================================================================
// Dynamic player / biped — HaloCaster commented-out / known-broken fields
// ============================================================================
//
// HC marked each of these as exploratory or broken; do not trust the offset
// without re-verification.
const (
	OffDynStunnedCandidateBroken          uint32 = 0x1CB // s32 — halocaster.py:1698 ("not actually stunned")
	OffDynMaybeDesiredFacingVectorXBroken uint32 = 0x1C8 // f32 — halocaster.py:1700 (HC commented-out)
	OffDynMaybeDesiredFacingVectorYBroken uint32 = 0x1CC // f32 — halocaster.py:1701 (HC FIXME "y is null")
	OffDynMaybeDesiredFacingVectorZBroken uint32 = 0x1D0 // f32 — halocaster.py:1702 (HC commented-out)
	OffDynSelectedWeaponIndex2Broken      uint32 = 0x2A4 // s16 — halocaster.py:1736 (HC commented-out)
	OffDynCamoThing2Broken                uint32 = 0x330 // f32 — halocaster.py:1756 (HC commented-out)
)

// Sentinel: ensure the broken constants compile cleanly even without consumers.
var _ = []any{
	OffDynStunnedCandidateBroken,
	OffDynMaybeDesiredFacingVectorXBroken,
	OffDynMaybeDesiredFacingVectorYBroken,
	OffDynMaybeDesiredFacingVectorZBroken,
	OffDynSelectedWeaponIndex2Broken,
	OffDynCamoThing2Broken,
}
