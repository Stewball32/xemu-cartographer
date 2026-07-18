# Halo CE Mark V Spartan — native Linux mesh export + pose/render pipeline

Extracts Halo: Combat Evolved's **Master Chief** (the `characters\cyborg\cyborg`
**Mark V** body) straight out of a stock **Xbox** CE cache map, retargets MCC's
global customization poses onto its skeleton, and renders the appearance-studio
passes with the CE single-armor-colour tint — **entirely on Linux**, no Windows
tools, no Wine, no .NET, no xemu.

This is the H1 counterpart to `../h2-model/` (the H2 Mark VI). Same native-decoder
approach; the format is different (see below).

## Mark V vs Mark VI

CE's Master Chief is **Mark V** — an H1 `mode` (gbxmodel-family) tag, 19 bones,
~2.0k tris, single-armour-colour tinting (no emblem). H2's is **Mark VI** — a
`render_model` tag, 33 bones, ~3.1k tris, primary+secondary tint + chest emblem.
Both are the same underlying **Halo bip01 skeleton**, so MCC's cross-game
customization poses retarget onto either rig.

## The Xbox H1 `mode` format (reverse-engineered here)

CE on Xbox uses `mode` (model), **not** the PC `mod2`/gbxmodel. Layout was
derived field-for-field from the stock cache (cross-checked vs the Invader / c20
`gbxmodel` spec). Tag-index + cache parsing is reused from `halomap.py`
(vendored from `halo-offset-mapper`).

- **Header** `0xE8` bytes; reflexives (12B `[count i32][vaddr u32][pad]`):
  markers `0xAC`, **nodes `0xB8`**, **regions `0xC4`**, **geometries `0xD0`**,
  shaders `0xDC`.
- **Nodes** (156B): name[32], sibling/child/parent i16, translation (3×f32),
  rotation quat `(i,j,k,w)`, distance. **The quat is the INVERSE of H2's
  `render_model` convention** — the same Halo pelvis is `(.5,.5,.5,+.5)` here but
  `(.5,.5,.5,-.5)` in H2. We **conjugate** it on load so FK comes out Z-up and
  matches the mesh (head joint → Z=0.609, identical to H2).
- **Region → permutation** (88B): per-LOD geometry indices (i16) at `+0x40`
  (superlow…**superhigh**); we take super-high.
- **Geometry** (48B): parts reflexive at `+0x24`.
- **Part** (104B): `+0x58` vertex_count, `+0x4C` index-buffer vaddr (uint16
  triangle strips, degenerate-join doublets, local indices), `+0x64` → 12B
  vertex descriptor whose `+0x04` is the vertex-data vaddr.
- **Vertex** (32B): `+0x00` float x,y,z · `+0x0C` normal (11/11/10) · `+0x18`
  int16 u,v (/32767) · `+0x1C` node0×3 · `+0x1D` node1×3 (0xFD=none) · `+0x1E`
  int16 node0_weight (/32767, node1 = 1−node0). Skin weights are already clean
  (sum to 1.0).

## Pipeline (all Linux)

1. **`ce_render_model.py`** — decodes the `mode` tag → `cyborg.obj`,
   `cyborg.npz` (mesh + 19-bone skeleton + skin), `cyborg.model.json`.
2. **`halomap.py`** (vendored) + the H1 **`bitmaps.py`** (from `halo-offset-mapper`)
   extract `cyborg` (diffuse) and `cyborg multipurpose`; the multipurpose
   **blue channel** is the change-colour mask, baked to `cyborg_cc.png`.
3. **`blender_pose_render_ce.py`** — normalizes CE bone names (`bip01 l thigh`
   → `l_thigh`), retargets MCC's `poses.json` onto the 19-bone rig (same NumPy
   skinning as H2), renders diffuse + change-mask passes.
4. **`ce_composite.py`** — tints by a single CE armour colour (`blam.sav` u32
   @ 0x18 enum; palette in `sveltekit/src/lib/data/halo-armor-palettes.json`)
   over the change-mask. No emblem on CE.

## Run

```bash
CE="$HOME/repos/halo-offset-mapper/xbe-dropzone/Halo CE/maps"
python3 ce_render_model.py --map "$CE/bloodgulch.map" --out out --debug
# textures via halo-offset-mapper bitmaps.py (cyborg + cyborg multipurpose),
# then bake cyborg_cc.png = multipurpose blue channel.
blender -b -P blender_pose_render_ce.py -- --npz out/cyborg.npz --tex out/tex \
        --poses ../h2-model/out/mcc/poses.json --out out/mcc/render --res 768
python3 ce_composite.py --render out/mcc/render --out out/mcc/composite \
        --pose Salute --colors Green,Red,Cobalt,Gray,Orange
```

## Output (verified)

- `cyborg.obj` / `cyborg.npz` — 2256 verts, 2016 tris, 19-bone skin, weights
  sum to 1.0; bind pose renders as a clean Mark V Master Chief.
- All **11 MCC poses** retarget spike-free (the rigid-skin guardrail catches
  0–6 verts/pose; 144–169 before the quaternion-convention fix).
- Single-colour composites across the 18-colour CE palette.

## IP

Real Halo CE asset; for Stewart's personal / LAN use with his own copy of the
game. Do not redistribute the extracted assets. `halomap.py` / `bitmaps.py` are
vendored from the sibling `halo-offset-mapper` project (same author).
