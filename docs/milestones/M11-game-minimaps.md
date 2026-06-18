# Milestone 11 — Game minimaps

> Browser-rendered minimap as another overlay surface. Show floor outline, player positions + view direction, power-weapon / power-up spawns, height differential cues, animated event flares, and (if feasible) projectile traces. Extends M10's overlay infrastructure but warrants its own milestone because of the rendering complexity.

## 11a. Map geometry feasibility

Audit what the Halo: CE scraper actually exposes for level geometry. Today the reader carries `power_item_spawns` and `map` identity but probably not BSP geometry. Decide between: (i) baking per-map static SVG/PNG tracings into the frontend keyed by map name + scenario tag, (ii) extracting BSP at runtime from the scraper, (iii) hybrid (static floor, dynamic markers). Almost certainly (i) for v1 — floor outlines as committed assets in `sveltekit/static/maps/<scenario>.svg`. For multi-floor maps, commit per-floor variants and use Z-coordinate ranges (see 11c) to switch between them.

## 11b. Player position + view cone

The reader exposes player world coordinates and aim vectors per the M5 tick payload. Project onto the 2D minimap via a per-map transform (committed alongside the SVG asset in 11a). Render with HTML5 canvas or SVG primitives. Filter the roster through M10d's dummy-player filter so the host's out-of-map dummy doesn't show up.

## 11c. Height differential cues

Map a player's Z (vertical) world coordinate to a visual cue on the 2D minimap so viewers can tell who's elevated vs underneath. Two complementary approaches, ship both and let style toggle pick:

- **Icon size scaling** — closer to the camera (higher Z, or whatever convention fits the map) = larger icon. Subtle range, e.g. 0.7×–1.3×.
- **Color tint / Z-banded layer** — segment Z into N bands and tint the icon (e.g. blue tint = below floor, red tint = above floor). Good for sharp multi-floor maps where size scaling reads ambiguously.

## 11d. Power weapons + power-ups

`power_item_spawns` already in tick data; render as fixed icons. Add held-or-available state if the reader exposes it; otherwise that's a follow-up offset to add (file under M19).

## 11e. Event flares + animations

When certain events fire on a tick, animate a flare on the minimap at the relevant position. Initial event list (each gets its own animation):

- **Death + respawn** — fade-out at death position, brief flash at respawn.
- **Player teleporting** — line/streak between source and destination teleporter exits.
- **Active overshield / camo** — persistent halo or shimmer around the icon while the powerup is active.
- **Power weapon held** — small badge on the icon (rocket, sniper, etc.).
- **Multi-kill / kill streak** — burst flare at the killer's position.

Animation library decision (see 11g) gates how rich these can be; v1 can ship CSS-keyframe-based animations and upgrade later.

## 11f. Projectile rendering (stretch)

Investigate whether the projectile data the reader currently exposes (visible in the debug page Tick → Projectiles tab) is rich enough for tracer rendering. If not, spec the offset additions and file as M19 follow-ups; ship 11a-e without projectiles.

## 11g. Library choice decision

Raw canvas vs. animation library (PixiJS, two.js, Konva, motion-one). Decide during 11b/11c based on perf — 30Hz tick updates × 16 players (with size + tint deltas) × N projectiles + N flares may justify a real renderer. SVG with Svelte transitions might be enough for 11a-d; flares (11e) and projectiles (11f) probably push toward canvas.

Smoke test: load `/overlays/minimap/<machine>/` for a Halo: CE match on Blood Gulch (or whatever map's been traced first). Player positions track correctly, view cones rotate with aim, height cues swap correctly when a player jumps a cliff, power weapons appear at spawn positions, kill flares fire on every kill captured by the M5 event stream, neutral-host dummy player is absent. Composite over OBS scene.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-18: First increment — the **projection + height math** (11b/11c core), implemented during the autonomous overnight run. This is the part that has to be numerically correct; the SVG assets, canvas rendering, flares, and live OBS verification are deferred (can't run here).
  - New `sveltekit/src/lib/utils/minimap.ts` (pure, unit-tested via `minimap.test.ts`): `projectToMinimap(pos, MapTransform)` (translate → rotate → scale → center, with Y-flip for SVG's downward axis), `aimToScreenAngle(facing, t)` for view cones (11b), `heightBand(z, …)` (11c Z-banded colour cue) + `iconScale(z, …)` (11c 0.7×–1.3× size cue), and a `MAP_TRANSFORMS` registry + `transformForMap()` lookup that returns null for un-traced maps so the overlay can render "minimap unavailable" instead of projecting onto the wrong asset.
  - **Deferred (need traced map assets + live data + OBS):** 11a's committed per-map SVG floor tracings (the registry currently holds a single placeholder Blood Gulch transform — values are uncalibrated until a real tracing lands); the `/overlays/minimap/<machine>/` route + canvas/SVG renderer (11d power-item icons, 11e flares); 11f projectile traces; the 11g canvas-vs-library perf decision. The minimap consumes M10d's `FilterRoster` for the dummy player — that wiring rides along with the deferred overlay-data path.
