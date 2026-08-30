# M29 — Organizer route redesign (six-page library suite)

> **Status:** In progress
> **Started:** 2026-08-29
> **Completed:** —
> **Depends on:** M08 (roles), M26 (ingest model), the CL-01…18 overlay pack (NamePlate)

## Goal

Reshape `/organizer/` from two tabbed pages into six first-class library pages
(designer handoff `Organizer route redesign.zip`, 2026-08-29), so every asset a
LAN night runs on — memory offsets, discs, maps, gametypes, rulesets, plate
banners — has one curated home with the same master-detail + draft-then-save
language. The in-page tabs move into the nav rail (one entry per page).

## Scope

- **Offsets** (new): offset-set library — import offsetmap exports from the
  hunting rig into the `offset_sets` collection (versioned by re-import),
  byte-identical download, delete-with-migration; embedded baselines listed
  alongside, undeletable. Scraper resolves imported ids via
  `offsets.SetDynamicSource` (PB-backed, wired in main.go).
- **Discs** (rename of Games): master-detail re-layout; `available` bool →
  three-state `role` (play / server / shelved) + `allow_on_xbox` eligibility
  switch. New ingests land shelved; drift forces shelved. Play picker + launch
  guard follow `role`.
- **Maps** (new): canonical build catalog — one card per (game, filename,
  content_hash); iso_maps gains per-cache hashes (ingest + boot backfill);
  organizer curates display name, variant-of (no chains — hook-enforced),
  description, uploaded square graphic (BSP render stands in), power-item
  rotations.
- **Gametypes** (absorbs Creator): master-detail re-layout, CE/H2 baked at
  creation, library name vs in-game `display_name` (save + lobby use the
  latter), CE fields stay server-schema-driven, live save preview kept.
  WIP per the handoff: labels/values get trued against in-game screenshots.
- **Rulesets** (new): gametypes + map pool + team size + series per ruleset;
  unsigned-save warnings bubble up; empty pool = open pool. WIP alongside
  Gametypes.
- **Nameplates** (new): 600×100 banner library for the overlay NamePlate;
  card grid renders real plates; one editor dialog with a 6:1 crop stage under
  the exact plate chrome. The handoff's deliberate text-outline deviation is
  back-ported into `NamePlate.svelte` for art-backed plates so organizer
  preview and OBS output stay identical.
- Out of scope: the player-side banner picker (settings), station sync-time
  selection UI, Series/stations referencing rulesets, H2 gametype surface
  growth.

## Actions

- [x] Migrations: isos role/allow_on_xbox (+backfill, drop available),
      offset_sets, maps + iso_maps.content_hash, gametypes.display_name,
      rulesets, nameplates
- [x] Offset-set routes (list merged, import, raw download, delete-migrate) +
      scraper dynamic source
- [x] Maps catalog sync (ingest + boot backfill) + joined view route +
      variant-guard hook
- [x] Nav rework (six rail links), redirects (games→discs, creator→gametypes)
- [x] Six pages + shared organizer components (MasterDetail, DraftBar,
      FilterChips, OrgToggle, ImageReframe, PlatePreview)
- [x] NamePlate back-port (art-scoped outline + white motto, chromeOnly mode)
- [x] Dev seeder: baseline role rows + organizer@dev.com (fresh-DB fix)
- [ ] True Gametypes/Rulesets field surfaces against in-game menu screenshots
      (handoff WIP note)
- [ ] Live pass with a real ingested disc (catalog cards from actual iso_maps
      rows, disc binding of an imported offset set)

## Verification

- `go build` / `vet` / `test ./...` green (new: offsets dynamic-source, catalog
  sync, offset-set route helpers); `pnpm lint` / `check` / `test` (195) /
  `build` green.
- Live smoke on a fresh dev DB (2026-08-29): all six migrations applied; six
  pages walked as `organizer@dev.com` (organizer-not-admin path) with zero
  console errors; offset-set import → version bump → rename → byte-identical
  download → delete-with-migration exercised over the API; baseline
  shadow/delete refused; gametype created via PB rules generates a signed save
  from `display_name`; ruleset references it; nameplate art uploaded through
  the crop dialog renders on the library card as the real plate.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-08-29: created; backend + all six pages landed on `update/organizer`
  (stacked on `update/overlays` for NamePlate); smoke-verified on a fresh dev
  DB. Field-truing + live-disc pass remain.
