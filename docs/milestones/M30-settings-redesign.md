# M30 — Player settings redesign (unified tabbed page)

> **Status:** In progress
> **Started:** 2026-08-29
> **Completed:** —
> **Depends on:** M29 (nameplates collection), the CL-01…18 overlay pack (NamePlate), M07/M22 (gamertag moderation)

## Goal

One page owns the whole player identity (designer handoff
`Player Settings Redesign.zip`, 2026-08-29): account profile, per-game Halo: CE
/ Halo 2 profile config, the stream nameplate + gamertags, and OAuth
connections — consolidating and RETIRING the old `/gamertag/` page and the
previous `/settings/` layout. The save logic stays the pre-redesign machinery
(users.gamertag single source of truth → server-side signed-save regen,
`h2_profiles` / `ce_profiles` upserts) behind the new UI.

## Scope

- **Five tabs**: General (profile / security / danger strip), Halo: CE (WIP —
  armor + schema-driven presets/advanced cyclers), Halo 2 (WIP — armor/emblem
  studio; Controls panel as a DISABLED preview until the six in-game options'
  byte offsets are live-verified), Stream (nameplate + motto + curated banner
  picker + gamertag list), Accounts (OAuth link/unlink, no action bar).
- **Stream identity backend**: `users.motto` (≤40) + `users.nameplate`
  (relation → nameplates); a hook guards NEW picks to Selectable banners
  (worn-hidden banners stay); `/api/public/profiles` now serves `motto` +
  `plate`; the overlay identity path (`OverlayIdentity` → `applyIdentities` →
  `player.motto`/`player.plateBg`) feeds NamePlate.
- **Default-gamertag = the profile name**: a users hook syncs `users.gamertag`
  from `default_gamertag` changes (blocked defaults refused), which cascades
  the existing profile-regen hook — "changing your default regenerates both
  saves."
- **Decisions** (Stewart, 2026-08-29): old Teams/membership settings content is
  **PARKED** (components stay in-tree; no player-facing home until the Teams
  tab design lands — self-serve team creation + invite accept UI are offline);
  banner picker is **curated-only** (no player uploads — the settings mock's
  upload tile lost to the organizer handoff's organizer-only rule); H2 controls
  ship as a **disabled preview**.
- Deviation from the mock, deliberate: the plate preview renders the **default
  gamertag** (the moderated on-air handle), not the free-text display name the
  mock showed — putting unreviewed `users.name` on the broadcast is exactly
  what the PR #30 moderation model forbids. The avatar well shows the real
  site avatar (star fallback), matching what OBS draws.
- Out of scope: the Teams tab design, H2 control byte mapping, player banner
  uploads/moderation.

## Actions

- [x] Migration: users.motto + users.nameplate
- [x] Hooks: default-gamertag → gamertag sync (+regen cascade), nameplate
      selectable guard — both integration-tested
- [x] /api/public/profiles motto + plate; overlay store/state/mock wiring
      (+ applyIdentities test)
- [x] Five tabs + shared components (ActionBar, CyclerRow, SwatchGrid,
      WipChip); sticky per-tab action bars; <900px sticky mini-preview
- [x] /gamertag/ → /settings/?tab=h2 redirect; Gamertag nav entry removed
- [ ] H2 control byte offsets live-verified → enable the Controls panel
- [ ] Teams tab design pass (un-park the M23 membership surface)
- [ ] True the WIP tabs against validated flows, then drop the WIP chips

## Verification

- All gates green: `go build`/`vet`/`test` (new hook + wire tests),
  `pnpm lint`/`check`/`test` (196)/`build`.
- Fresh-DB live smoke (2026-08-29): five tabs walked as a real user with zero
  post-login console errors; `/gamertag/` lands on the H2 tab; H2 Save &
  regenerate produced a signed save (telemetry in the action bar); Stream save
  put motto + banner on `/api/public/profiles` (verified on the wire with the
  picked armor color riding along); the selectable guard refused a fresh
  hidden-banner pick while a worn-hidden banner survived; `?mock=1` POV bar
  renders the motto line. Renders: `/tmp/settings-redesign/renders/`.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-08-29: created; full page + backend landed on `update/settings`
  (stacked on `update/organizer` → `update/overlays`); smoke-verified.
