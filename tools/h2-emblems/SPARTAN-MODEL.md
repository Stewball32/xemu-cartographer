# Mark VI Spartan model (H2 MP body) — extraction status + pose/render plan

Status: **located + characterized; mesh export is blocked on this Linux box**
(needs a Windows H2 tool — see below). Textures are reachable with the emblem
pipeline. This is the prep for the appearance-studio "pose" feature (render the
real Mark VI once per pose → tint by armor colour → overlay the emblem decal).

## What the MP Spartan is, in the cache

Source: `shared.map` (Halo 2 **Xbox**, cache v8). Full inventory with tag ids in
`spartan_model_inventory.json`. The relevant chain:

| Tag | id | Path | Role |
| --- | -- | ---- | ---- |
| `bipd` | 3929 | `objects\characters\masterchief\masterchief_mp` | the **multiplayer** biped (Spartan player) |
| `hlmt` | 3930 | `…\masterchief_mp` | model group the biped points at |
| `mode` | **83** | `objects\characters\masterchief\masterchief` | **render_model — the Mark VI body mesh** |
| `coll` | 125 | `…\masterchief` | collision model |
| `phmo` | 200 | `…\masterchief` | physics |
| `jmad` | 126 | `…\masterchief` | **model_animation_graph — the poses/anims** |
| `bitm` | 111 | `…\bitmaps\masterchief` | diffuse / colour map |
| `bitm` | 112 | `…\bitmaps\masterchief_cc` | **change-colour map (armor primary/secondary tint mask)** |
| `bitm` | 110 | `…\bitmaps\masterchief_bump` | normal/bump |
| `bitm` | 113 | `objects\bitmaps\reflection_maps\masterchief_armor` | reflection/specular |

The character types in the profile (`0x11C`) map to: masterchief(0)+spartan(2) →
the Mark VI model above; dervish(1)+elite(3) → `objects\characters\elite\elite`
(`mode` #376, with its own `_cc` change-colour map). Both are in the inventory.

## Format of the `mode` (render_model) tag

H2 `render_model` is **not** a flat mesh: it is regions → permutations →
geometry **sections**, each section a block of **compressed vertices** (packed
position/normal/UV, 16-bit-ish) + a triangle strip index buffer, skinned to a
**node (bone) skeleton**. Materials per section reference `shad` shaders, which
reference the `bitm` textures above. Animation/pose data lives separately in the
`jmad` graph (#126), as node keyframes.

This is why the model can't be "grepped" or trivially struct-parsed the way the
2D emblems were: the geometry is compressed and node-indexed, and poses come
from a second tag.

## Why mesh export is blocked here (and what unblocks it)

The emblem extractor (`extract_emblems.py`) works because H2 *bitmaps* are a
small fixed record + a raw pixel pool. The model is not, and the Linux toolchain
can't read it:

- **Sigmmma `reclaimer` ships no H2 `mode`/`hlmt`/`bipd` definitions** — only
  `ant! bitm hsc* pphy snd! shad trak ugh!`. `get_meta(#83)` returns `None`.
- Even for classes it *does* define, reclaimer's H2-**Xbox** meta parser
  mis-reads the body (proven on the emblem bitmaps — we had to parse those
  ourselves). So extending it to models is not a small patch.
- There is no Linux-native tool that exports H2 model geometry. `xemu` is
  explicitly out of scope for this work.

**Unblock = run an existing Windows tool once** (this is the "wrap tooling, don't
reverse-engineer the mesh" path):

1. **Gravemind2401's Reclaimer** (.NET / WPF) reads the **MCC** `halo2` maps
   (already installed at
   `…/steamapps/common/Halo The Master Chief Collection/halo2/h2_maps_win64_dx11/`)
   and batch-exports `render_model` → **OBJ/FBX/AMF** + textures. Cleanest option.
   - Alternative for the Xbox maps: **H2 Editing Kit `tool`**, **Assembly**, or
     **Entity** → JMS/OBJ. Runs under Wine but needs a .NET runtime (not
     installed here; MCC + Reclaimer on a Windows box is less friction).

## Pose → tint → emblem pipeline (the studio feature)

Mirrors the emblem approach, in 3D, done **offline** so the app ships flat PNGs:

1. **Export** `masterchief` `mode` (+ skeleton) and `masterchief_cc` /
   `masterchief` / `masterchief_bump` textures via Reclaimer (step above).
2. **Import to Blender**; the mesh comes in with its node skeleton. Build a small
   set of **poses** (idle, salute, crouch, victory…) — pose the bones directly,
   or import the `jmad` clips if the exporter supports them.
3. **Render orthographic, transparent, once per pose** → `static/spartan/<pose>.png`
   (+ a matching render of the `_cc` mask so the app knows the tintable regions —
   render the cc map through the same camera/pose, or bake a primary/secondary
   coverage pass exactly like the emblem `_p`/`_s` masks).
4. **In the studio**, reuse the emblem tint compositor: the `_cc` pass gives
   primary/secondary coverage → fill with the two **armor** colours over the
   diffuse; then drop the chosen **emblem** onto the chest (CharacterPreview
   already positions an emblem decal). Result: the real posed Mark VI, tinted to
   the player's armor, wearing their real emblem — replacing the current
   hand-drawn `CharacterPreview` bust.

The change-colour map (`masterchief_cc`) uses the **same two-tone idea as the
emblems** (one region = primary armor colour, the other = secondary), so the
existing `emblem-art.ts` mask compositor drops straight in for the armor tint.

## What's saved here

- `spartan_model_inventory.json` — exact tag ids/paths for the Mark VI + Elite
  model chains (mesh, collision, physics, animations, textures), MP biped
  included. The starting point for the one-time Windows export.
