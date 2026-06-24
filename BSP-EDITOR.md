# BSP spectator-mesh editor

A purpose-built dev tool for **culling a Halo map's BSP geometry down to a clean
"spectator mesh"** — drop the roof / ceilings / out-of-bounds clutter, keep the
walkable interior — that both the 2D floorplan and 3D visualizer then render.

It does **not** edit the shipping `.map`/BSP. It bakes a **derived** asset from
the already-extracted geometry; the playable map stays untouched. The baked asset
is a culled mesh in the exact same JSON schema as the raw one, so both visualizer
views pick it up through the existing loader with no view changes.

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

## The loop: edit → export → shows up in the visualizer

1. **Load** a map (e.g. Chill Out). It opens in the orbit view with material
   colours, walkable-floor reference dots, and the cull-height plane pre-set just
   under the ceiling (highest walkable floor + headroom).
2. **Cull the roof in one drag:** with the plane shown, drag the **cull-height
   slider** (or drag the plane itself in Orbit view). Everything above the plane
   previews in **orange**. Hit **Remove above plane**. The roof/ceiling is gone.
3. **Clean stragglers:** pick a select tool —
   - **Connected piece** — click any triangle, the whole connected ceiling/wall
     selects.
   - **Same material** — click, every triangle sharing that texture selects.
   - **Single triangle** — one at a time.
   - **Box select** — drag a rectangle over loose bits.

   then **Delete / mark inaccessible**. Shift-click adds to the selection,
   Alt-click removes from it. `Delete` key also deletes; `Esc` clears.

4. **Made a mistake?** Nothing is destructive until you export. **Undo/Redo**
   (`Ctrl+Z` / `Ctrl+Shift+Z`), toggle **Ghost removed** to see what you cut, and
   **Restore** any re-selected ghost triangles.
5. **Bake it:** click **Save to cartographer (dev)**. It writes
   `static/game-geometry/<game>/<key>.spectator.json` and patches `manifest.json`
   with a `spectator_file` pointer.
6. **See it:** open a visualizer for that map — e.g.
   - 3D: `http://localhost:5173/visualizer3d/demo/?map=chillout`
   - 2D: `http://localhost:5173/visualizer/demo/?map=chillout`

   `loadBspMesh` now serves the **baked** mesh automatically (it prefers
   `spectator_file`), so both views render your cleaned geometry. Hard-refresh if
   it was already open.

To **keep iterating**: the dropdown shows a `· baked` badge once a map has a
spectator mesh; click **Load baked** to re-open the culled asset and cut further,
or use **Import .json (re-edit)** to load any exported file.

---

## Exports

| Button                   | Output                                    | Who consumes it                                                                                           |
| ------------------------ | ----------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| **Save to cartographer** | writes into the geometry cache + manifest | the 2D + 3D visualizers (canonical)                                                                       |
| **Download JSON**        | `<key>.spectator.json`                    | same schema — drop it in `static/game-geometry/<game>/` manually if the dev save endpoint isn't available |
| **Download GLB**         | `<key>.spectator.glb` (Y-up)              | external 3D tools (Blender, etc.) — not loaded by cartographer                                            |

The **JSON is canonical**: it's a culled `BspMesh` in the identical schema as the
raw mesh, so the 2D view is just an orthographic projection of the same geometry
the 3D view renders — the two always agree by construction. The GLB is a
take-it-elsewhere convenience.

### Manual persistence (no dev server)

`Download JSON`, then:

```sh
mv ~/Downloads/chillout.spectator.json sveltekit/static/game-geometry/haloce/
# add  "spectator_file": "chillout.spectator.json"  to manifest.json's chillout entry
```

The **Save to cartographer** button does both of these for you in `pnpm dev` /
`task dev`.

---

## Baked asset format (`*.spectator.json`)

A strict **superset** of the raw mesh schema (`internal` field names match the
extractor's, so `normalizeMesh` / `loadBspMesh` read it unchanged):

```jsonc
{
  "schema_version": 1,
  "kind": "spectator-mesh",
  "generated_by": "bsp-editor",
  "game": "haloce",
  "scenario": "levels\\test\\chillout\\chillout",
  "source_map": "chillout.map",
  "source_mesh": "chillout.json",   // provenance: raw mesh it was baked from
  "cull_z": 7.2,                     // provenance: cull height used
  "bounds": { "minX": …, "maxZ": … },// recomputed from the kept geometry
  "positions": [ … ],                // re-indexed; only kept vertices
  "colors":    [ … ],                // per-vertex material colours (if present)
  "indices":   [ … ],                // kept triangles, densely re-indexed
  "vertex_count": …,
  "triangle_count": …,
  "bands": [ { "index": 0, "minZ": …, "maxZ": …, "midZ": … } ]  // optional baked elevation bands
}
```

---

## How the integration is additive

- `loadBspMesh(game, scenario)` now prefers a manifest entry's `spectator_file`
  over its raw `file` — so the visualizer pages need **zero changes** to consume
  the baked asset, and fall back to the raw mesh wherever none exists. Pass
  `{ raw: true }` to force the original (the editor edits the raw one).
- The existing live-geometry path (`buildFloorplan` 2D cull, `classifyShellTriangles`
  - room clustering 3D) is untouched. On a baked mesh those heuristics simply have
    little left to cut — which is the point.
- The dev save endpoint is a Vite `apply: 'serve'` middleware
  ([`sveltekit/vite-bsp-save.ts`](sveltekit/vite-bsp-save.ts)) — it has **no effect
  on `pnpm build`** (adapter-static).

## Where the code lives

| File                                                         | Role                                                                    |
| ------------------------------------------------------------ | ----------------------------------------------------------------------- |
| `sveltekit/src/lib/utils/bsp-edit.ts`                        | Pure editing core (selection, connectivity, export, undo) — unit-tested |
| `sveltekit/src/lib/utils/bsp-edit.test.ts`                   | Unit tests for the core                                                 |
| `sveltekit/src/lib/components/bsp-editor/EditorScene.svelte` | Three.js canvas (cameras, picking, plane drag, box select)              |
| `sveltekit/src/routes/bsp-editor/+page.svelte`               | Editor page (tools, controls, export)                                   |
| `sveltekit/vite-bsp-save.ts`                                 | Dev-only `POST /__bsp-save` write endpoint                              |
| `sveltekit/src/lib/utils/game-geometry.ts`                   | `loadBspMesh` spectator-preference + `loadGeometryManifest`             |
