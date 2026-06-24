# BSP spectator-mesh editor

A purpose-built dev tool for **culling a Halo map's BSP geometry down to a clean
"spectator mesh"** — drop the roof / ceilings / out-of-bounds clutter, keep the
walkable interior — that both the 2D floorplan and 3D visualizer then render.

The primary mechanism is **surface tagging**: every face is auto-classified
(floor / ramp / wall / ceiling), the operator fixes mistags, and per-tag render
rules decide what survives the bake. The cull-height plane + manual select/delete
stay as scalpels for stragglers.

It does **not** edit the shipping `.map`/BSP. It bakes a **derived** asset from
the already-extracted geometry; the playable map stays untouched. The baked asset
is a culled mesh in the exact same JSON schema as the raw one (plus a tag layer),
so both visualizer views pick it up through the existing loader with no view
changes.

> Lives on branch `feat/bsp-editor`, added **additively** alongside the live
> 2D/3D heuristic-culling path (which the `feat/visualizer-spectator-readability`
> branch is concurrently editing). Nothing in the existing live-geometry path was
> removed — the baked asset simply feeds cleaner input into the same pipeline.

---

## Launch

```sh
# Frontend only is enough (the editor reads the static geometry cache and writes
# through a dev-only Vite middleware — no PocketBase backend required):
cd sveltekit
pnpm dev

# …or the whole stack:
task dev
```

Then open:

```
http://localhost:5173/bsp-editor/
```

(Port is Vite's dev port — 5173 by default; `task dev` prints the actual URL.)

It auto-loads **Chill Out** if present, else the first extracted map. Pick any
map from the dropdown in the top bar.

### Prerequisite: the geometry cache

The editor consumes the same cache the visualizers do:
`sveltekit/static/game-geometry/<game>/` (`manifest.json` + per-map `*.json`).
If it's empty, the editor says so — generate it first:

```sh
python scripts/game-geometry/extract_bsp.py   # against your own Halo .map files
```

Chill Out (`chillout.json`), Blood Gulch, and Prisoner ship in the local cache by
default (git-ignored, machine-local).

---

## The loop: load → auto-tag → fix → bake → shows up in the visualizer

1. **Load** a map (e.g. Chill Out). It opens in Orbit view, **tag-coloured**
   (floors teal, ramps yellow-green, walls slate-blue), with floor reference dots.
   Auto-classify has already run: the **Surfaces (tags)** panel shows the per-tag
   counts, and the default render rules **drop the ceiling/roof automatically** —
   you're already looking into a clean interior, no manual culling yet. (On Chill
   Out: ~547 floor / 122 ramp / 1843 wall kept, ~596 ceiling dropped.)
2. **Fix any mistags.** Pick a select tool —
   - **Connected piece** — click a triangle → the whole welded piece selects.
   - **Same material** — click → every triangle sharing that texture selects.
   - **Single triangle** / **Box select** (drag a rectangle).

   then assign a tag: click **⤺** on a tag row, or press **1–6** (floor … clutter).
   Click a tag's **name** to select everything carrying it, and **◉** to isolate
   (show only that tag) for a fast fix-up sweep. Shift-click adds to a selection,
   Alt-click removes.

3. **Decide what renders.** Each tag row has a **render checkbox** — off = that
   tag is dropped from the bake. Defaults: floor/ramp/wall **on**, ceiling +
   inaccessible + clutter **off**. Tag a junk surface **inaccessible** to drop it.
4. **Scalpels for stragglers.** Drag the **cull-height plane** (slider, or grab
   the plane in Orbit view) → orange preview → **Remove above plane**; or
   select + **Delete**. Nothing is destructive: **Undo/Redo** (`Ctrl+Z` /
   `Ctrl+Shift+Z`), **Ghost dropped** to see what's excluded, **Restore** to bring
   it back. `Delete` key deletes the selection; `Esc` clears.
5. **Bake it:** click **Save to cartographer (dev)**. It writes
   `static/game-geometry/<game>/<key>.spectator.json` (mesh + embedded tags) and a
   `<key>.tags.json` sidecar, and patches `manifest.json` with `spectator_file` +
   `tags_file`.
6. **See it:** open a visualizer for that map — e.g.
   - 3D: `http://localhost:5173/visualizer3d/demo/?map=chillout`
   - 2D: `http://localhost:5173/visualizer/demo/?map=chillout`

   `loadBspMesh` now serves the **baked** mesh automatically (it prefers
   `spectator_file`), so both views render your cleaned geometry. Hard-refresh if
   it was already open.

To **keep iterating**: the dropdown shows a `· baked` badge once a map has a
spectator mesh; **Load baked** re-opens the culled asset, or **Import .json
(re-edit)** loads any exported file (its embedded tags are restored). Either way,
the `*.tags.json` sidecar's manual overrides are re-applied on load.

---

## Surface tags

| Tag                | Auto-classify (by face normal) | Renders by default | Role in the views                         |
| ------------------ | ------------------------------ | ------------------ | ----------------------------------------- |
| **floor**          | up-facing, ≤30° from flat      | ✅                 | filled + material colour + elevation band |
| **ramp / stairs**  | up-facing, 30–60° slope        | ✅                 | filled, banded with floors                |
| **wall**           | near-vertical                  | ✅                 | outline (2D) / solid (3D)                 |
| **ceiling / roof** | down-facing                    | ❌ (dropped)       | hidden                                    |
| **inaccessible**   | manual bin (never auto)        | ❌ (dropped)       | dropped                                   |
| **clutter**        | manual bin (small props)       | ❌ (dropped)       | dropped                                   |

Tags drive the **elevation banding** too — only floor/ramp surfaces are banded.

Today the per-tag render decision is applied **at bake** (off-render tags are
dropped from the baked geometry), so both existing visualizer views render the
tag-driven result with no changes to their (concurrently-edited) rendering code.
The asset still carries per-triangle tags + the render map, so once the branches
reconcile the views can read tags directly for live in-view per-tag styling /
toggling — the data is already there.

---

## Exports

| Button                   | Output                                        | Who consumes it                                                                        |
| ------------------------ | --------------------------------------------- | -------------------------------------------------------------------------------------- |
| **Save to cartographer** | mesh + tag sidecar into the cache + manifest  | the 2D + 3D visualizers (canonical)                                                    |
| **Download JSON**        | `<key>.spectator.json` (mesh + embedded tags) | same schema — drop it in `static/game-geometry/<game>/` if the dev save is unavailable |
| **Download GLB**         | `<key>.spectator.glb` (Y-up)                  | external 3D tools (Blender, etc.) — not loaded by cartographer                         |

The **JSON is canonical**: a culled `BspMesh` in the identical schema as the raw
mesh, so the 2D view is just an orthographic projection of the same geometry the
3D view renders — the two always agree by construction. The GLB is a
take-it-elsewhere convenience.

### Manual persistence (no dev server)

`Download JSON`, then `mv ~/Downloads/chillout.spectator.json
sveltekit/static/game-geometry/haloce/` and add `"spectator_file":
"chillout.spectator.json"` to that map's `manifest.json` entry. **Save to
cartographer** does this for you in `pnpm dev` / `task dev`.

---

## Asset format

### `*.spectator.json` — the baked mesh (+ tag layer)

A **superset** of the raw mesh schema (field names match the extractor, so
`normalizeMesh` / `loadBspMesh` read it unchanged):

```jsonc
{
  "schema_version": 2,
  "kind": "spectator-mesh",
  "generated_by": "bsp-editor",
  "game": "haloce",
  "scenario": "levels\\test\\chillout\\chillout",
  "source_map": "chillout.map",
  "source_mesh": "chillout.json", // provenance: raw mesh it was baked from
  "cull_z": 7.2, // provenance: cull-plane height used
  "bounds": { "minX": …, "maxZ": … }, // recomputed from the kept geometry
  "positions": [ … ], // re-indexed; only kept vertices
  "colors": [ … ], // per-vertex material colours (if present)
  "indices": [ … ], // kept triangles, densely re-indexed
  "vertex_count": …,
  "triangle_count": …,
  "tag_legend": ["floor","ramp","wall","ceiling","inaccessible","clutter"],
  "tags": [ 0, 0, 2, … ], // one tag index per KEPT triangle
  "tag_render": { "floor": true, "ceiling": false, … }, // bake render rules
  "tag_overrides": [ { "k": "12,−3,4,pz", "tag": "inaccessible" } ], // stable-keyed manual edits
  "bands": [ { "index": 0, "minZ": …, "maxZ": …, "midZ": … } ] // from tagged floors
}
```

### `*.tags.json` — the re-applicable tag sidecar

Tags **can't** live in the BSP/.map, so the hand-editable, geometry-independent
slice is written separately so manual tags **survive a geometry re-extract**:

```jsonc
{
  "schema_version": 2,
  "kind": "spectator-tags",
  "legend": ["floor","ramp","wall","ceiling","inaccessible","clutter"],
  "render": { "floor": true, "ceiling": false, … },
  "overrides": [ { "k": "12,−3,4,pz", "tag": "inaccessible" } ]
}
```

`overrides` are keyed by a **stable spatial signature** (quantised centroid +
dominant normal axis), so on a future re-extract/re-bake the editor re-applies the
operator's hand-edits to the freshly-extracted triangles instead of wiping them —
only the surfaces you changed from the auto-classification are stored.

---

## How the integration is additive

- `loadBspMesh(game, scenario)` now prefers a manifest entry's `spectator_file`
  over its raw `file` — so the visualizer pages need **zero changes** to consume
  the baked asset, and fall back to the raw mesh wherever none exists. Pass
  `{ raw: true }` to force the original (the editor edits the raw one).
- The existing live-geometry path (`buildFloorplan` 2D cull,
  `classifyShellTriangles` + room clustering 3D) is untouched. On a baked mesh
  those heuristics simply have little left to cut — which is the point.
- The dev save endpoint is a Vite `apply: 'serve'` middleware
  ([`sveltekit/vite-bsp-save.ts`](sveltekit/vite-bsp-save.ts)) — **no effect on
  `pnpm build`** (adapter-static).

## Where the code lives

| File                                                         | Role                                                                                             |
| ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------ |
| `sveltekit/src/lib/utils/bsp-edit.ts`                        | Pure core: tags, auto-classify, selection, connectivity, stable keys, export, undo — unit-tested |
| `sveltekit/src/lib/utils/bsp-edit.test.ts`                   | Unit tests for the core                                                                          |
| `sveltekit/src/lib/components/bsp-editor/EditorScene.svelte` | Three.js canvas (cameras, tag colours, isolate, picking, plane drag, box select)                 |
| `sveltekit/src/routes/bsp-editor/+page.svelte`               | Editor page (tag UI, tools, controls, export)                                                    |
| `sveltekit/vite-bsp-save.ts`                                 | Dev-only `POST /__bsp-save` (writes mesh + tag sidecar)                                          |
| `sveltekit/src/lib/utils/game-geometry.ts`                   | `loadBspMesh` spectator-preference + `loadGeometryManifest`                                      |
