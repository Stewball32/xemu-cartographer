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
// # Design philosophy: TEMPLATE-PATCH + RE-SIGN
//
// Every payload save carries a 20-byte content-dependent integrity DIGEST — the
// Original-Xbox roamable save signature (cracked 2026-06-25; see digest.go). The
// generator takes a real, same-engine save as a TEMPLATE, overwrites only the
// fields we understand, preserves every other byte verbatim, and then RE-SIGNS:
// it recomputes the digest over the new content so the file is valid. Without
// this, an edited file's digest no longer matches its content and Halo rejects
// it on load as "damaged" (confirmed in xemu, 2026-06-25). Consequences:
//
//   - Build with no field changes -> byte-identical to the template (the
//     recomputed signature equals the template's, verified on real samples).
//   - Build with edited settings   -> identical except the patched field bytes
//     AND a freshly recomputed, valid signature.
//
// The signature is per-title (roamable): the signing key derives from a global
// Xbox constant and the title's XBE certificate key, with no console-specific
// input, so the hub signs centrally. RecomputeDigest implements it for CE and
// H2 (DigestResolved() == true).
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
