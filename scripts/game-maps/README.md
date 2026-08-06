# game-maps tool (map-thumbnail render)

Self-contained toolchain for the map-graphics feature: render a top-down PNG of
a Halo: CE `.map`'s structure-BSP geometry (CE ships no per-map preview art, so
the render IS the map graphic). Shelled by `internal/isoingest` at ingest time
(best-effort, async, multiplayer maps only).

Files:
- `extract_bsp.py` — the renderer (copy of `scripts/game-geometry/extract_bsp.py`;
  produces `<out>/haloce/<map>_top.png`).
- `halomap.py`, `bitmaps.py` — VENDORED from halo-offset-mapper
  (`scripts/mapmanifest/`): the `.map` cache parser + swizzle-aware bitmap
  decoder the renderer imports. Self-contained (stdlib + numpy + PIL). Kept here
  so the feature builds/runs without a sibling checkout.

Runtime deps: `python3`, `numpy`, `Pillow`.

Staged to `~/xcarto-beta/tools/game-maps/`; the Go side defaults
`MAPS_THUMBS_SCRIPT=./tools/game-maps/extract_bsp.py` and
`MAPS_THUMBS_MAPPER_DIR=./tools/game-maps` (relative to the tier cwd).
Disable with `MAPS_THUMBS_ENABLED=false`.
