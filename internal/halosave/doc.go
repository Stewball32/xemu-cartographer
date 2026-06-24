// Package halosave reads and writes Halo: Combat Evolved and Halo 2 Xbox
// UDATA save files (multiplayer gametype variants and Halo 2 player profiles),
// for the cartographer LAN hub.
//
// It is a faithful Go port of the offline-derived `halo_save_formats.py`
// generator produced by the "profile/gametype de-risk" task (2026-06-23/24).
// The field offsets here were reverse-engineered by diffing real saves pulled
// READ-ONLY from the fleet xemu HDD images (23 CE gametypes, 2 H2 profiles,
// 1 H2 gametype). See FORMATS.md in the de-risk deliverable for the full spec
// and docs/lan-hub/README.md in this repo for the porting notes.
//
// # Design philosophy: TEMPLATE-PATCH
//
// Every payload save carries a 20-byte content-dependent integrity DIGEST whose
// algorithm has not yet been cracked (see digest.go and the M24-adjacent
// de-risk verdict). So the generator never synthesises a file from nothing: it
// takes a real, same-engine save as a TEMPLATE, overwrites only the fields we
// understand, and preserves every other byte verbatim — including the digest
// and the trailing padding. Consequences:
//
//   - Build with no field changes  -> byte-identical to the template.
//   - Build with edited settings    -> identical except the patched field bytes
//     (the digest is NOT recomputed).
//
// RecomputeDigest is a pluggable hook (see digest.go). It currently returns
// ErrDigestUnresolved; flip it on once the runtime probe in the de-risk verdict
// confirms whether the digest is even enforced on load, and the signing key /
// algorithm is recovered. Until then, callers use template-patch (preserve).
//
// # File types covered
//
//	CE gametype  : E:\UDATA\4d530004\<name>\blam.lst       (fixed 512 bytes)
//	H2 profile   : E:\UDATA\4d530064\<id>\profile          (fixed 500 bytes)
//	H2 gametype  : E:\UDATA\4d530064\<id>\<modename>       (variable; slayer=324)
//	SaveMeta.xbx : both titles, trivial UTF-16 "Name=..." sidecar (UNSIGNED)
//
// Halo: CE has no standalone multiplayer player-profile file — the editable CE
// surface is the gametype only. The "appearance/controls" half of the brief
// applies to Halo 2 (h2profile.go).
package halosave

// Xbox title IDs for the two supported titles (used to build the
// E:\UDATA\<TitleID>\ path segment a LAN client writes into).
const (
	TitleIDHaloCE = "4d530004"
	TitleIDHalo2  = "4d530064"
)
