# M27 — 3D visualizer (live markers + real BSP geometry)

> **Status:** In progress
> **Started:** 2026-06-23
> **Completed:** —
> **Depends on:** M11 (game minimaps — VizModel + overlay feed), M25 (OBS overlays — overlay token + createOverlayFeed), game-icons extractor (regen-script + git-ignored cache stance)

## Goal

A 3D spectator/visualizer surface that flies the camera around the **actual**
Blood Gulch level with the live player / item / vehicle markers inside it —
proving the spatial data end-to-end in 3D, not just the 2D top-down. It is a
NEW route (`/visualizer3d/[instance]/`) so the 2D map stays the daily-driver,
and it is mock-able end-to-end (`?mock=1`) so the proof needs no live xemu.

## Scope

- **Stage 1 — 3D scene + live markers.** A Three.js scene fed by the SAME
  overlay/live WS feed as the 2D visualizer (reuses `createOverlayFeed` +
  the M10 overlay-token auth + `buildVizModel`). Players render as team-colored
  capsules with a name label + a heading arrow from aim; items / vehicles /
  projectiles / spawns / flags as markers; all positioned by their XYZ in a 3D
  world. Orbit camera. `?mock=1` sample-data mode.
- **Stage 2 — real map geometry.** Extract the CE structure-BSP **rendered
  geometry** (vertices + triangles) for Blood Gulch from the `.map` files
  (reuses halo-offset-mapper's `halomap.py` cache parser), convert to a mesh,
  render it as the level. Game-derived geometry → **git-ignored local cache +
  a regen script** (same stance as the icons / map PNGs); degrades gracefully
  to the auto-fit world-bounds box if the mesh cache isn't present.
- CE / Blood Gulch first; structured so H2 + real player/vehicle models
  (stage 3) drop in later with minimal changes.
- **Out:** real textured/lightmapped surfaces, real biped/vehicle models
  (stage 3+), collision-BSP extraction, per-map camera calibration.

## Actions

- [x] New route `/visualizer3d/[instance]/` (+page.ts mirrors the 2D loader;
      +page.svelte hosts the scene, header, layer toggles, recenter).
- [x] `Scene3D.svelte` — Three.js scene: orbit camera, lights, level mesh /
      fallback box, reconciled player capsules (label + heading arrow), and
      item/vehicle/projectile/spawn/flag markers.
- [x] `viz3d.ts` — pure Halo→Three coordinate remap + camera framing (unit-tested).
- [x] `game-geometry.ts` — pure scenario→mesh-key + a failure-tolerant mesh
      loader that degrades to null (→ bounds box) (unit-tested).
- [x] `scripts/game-geometry/extract_bsp.py` — structure-BSP rendered-geometry
      extractor (reuses halomap.py; self-validates vertices against world_bounds).
- [x] `.gitignore` the `sveltekit/static/game-geometry/` cache.
- [ ] Regenerate the Blood Gulch mesh locally + confirm it renders.

## Verification

- `pnpm lint` / `pnpm check` / `pnpm test` / `pnpm build` all pass.
- Headless mock render of `/visualizer3d/demo/?mock=1` shows the 4v4 markers in
  a 3D world (and the real Blood Gulch mesh when the cache is regenerated).
- With no geometry cache, the scene still renders markers inside the world-
  bounds box (graceful degrade).

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-23: created. Stage 1 (scene + live markers, mock-able) + Stage 2
  extractor + git-ignored geometry-cache loader landed off
  `feat/visualizer-halo-icons`. Coordinate frame: Halo (X/Y ground, Z up) →
  Three (x, z, −y), applied to both markers and the BSP mesh so they share one
  space. The live `GameData→positions` path is already proven by the 2D
  visualizer; this reuses the identical feed + model.
- 2026-06-23 (overnight): **Live + color + 2D background.**
  - **Live 3D confirmed** against the real `ce-viz-5` Blood Gulch CE match on the
    127.0.0.1:8093 dev backend (`/visualizer3d/5/?token=…`, host:5 overlay
    token): real players (Stew/Shadow) render + move in the real 3D map.
  - **Overlay WS join bug fixed** (`internal/websocket/handlers/join_room.go`):
    the host RoomType's `RequireAuth` guard rejected overlay-token connections
    (no user) with "authentication required" BEFORE the overlay-aware
    `authorizeHostRoom` ran — so overlay tokens could never join host rooms
    (both visualizers starved live). Overlay connections now skip the user
    guards and are gated solely by `authorizeHostRoom` (scoped to their bound
    instance, summary/other-instance/non-host denied). Unit test green.
  - **Texture-sampled per-material color** (shared extraction): each BSP
    material → shader → base bitmap is decoded (halo-offset-mapper `bitmaps.py`)
    and averaged, so terrain reads by real color (tan cliffs, olive floor, grey
    bases). 17/17 Blood Gulch shaders sampled. Emitted as per-vertex colors on
    the 3D mesh AND a top-down PNG. (Full UV texturing is the remaining stretch.)
  - **2D minimap geometry background**: the SAME extraction emits a top-down PNG
    (`<key>_top.png`); `TopDownMap` draws it placed by projecting the BSP bounds
    through the live projection (pixel-aligned with the dots), toggleable (“Map”),
    degrades to the blank grid when uncached. Dots/icons now sit on the real map.
  - **Mock relocated** into real Blood Gulch world-bounds so `?mock=1` sits on
    the geometry. `pnpm lint/check/test (150)/build` green.
  - **Known follow-ups:** the dev `OVERLAY_TOKEN_SECRET` is ephemeral, so a
    server restart rotates it and invalidates minted tokens (re-mint via the
    admin route); real UV textures + H2 are stage-3.
