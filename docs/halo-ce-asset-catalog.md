# Halo: CE (Xbox) asset catalog

A factual inventory of the 3D models, level geometry, vehicles, weapons,
bipeds, scenery, and HUD assets that exist across the stock Halo: CE **Xbox**
`.map` set — a map of "everything available" for the future 3D-model visualizer
stage and asset reuse, plus the source-of-truth check for the minimap-icon hunt.

- **Full machine-readable index:** [`halo-ce-asset-catalog.json`](halo-ce-asset-catalog.json)
  (tag paths + per-map presence + group counts — factual metadata, like the map
  manifest / offset maps, so it's committed; decoded art is never committed).
- **Regenerate:** `python3 scripts/halo-assets/catalog_assets.py` (reuses
  halo-offset-mapper's `.map` parser; reads the same `Halo CE/maps` dropzone the
  icon extractor uses).

## Key structural finding (and why the icon hunt ends here)

**Xbox Halo: CE is monolithic.** There is **no `bitmaps.map` / `sounds.map` /
`loc.map`** — those shared resource files are the _PC_ layout. Every Xbox `.map`
(MP and campaign) **embeds its own copy** of all tags, including the `ui\hud`
bitmap atlases. (The PC `bitmaps.map` does exist in a Steam/MCC install, but
that's a different build than what the cartographer scrapes through xemu.)

Consequently, the HUD icon atlases are **identical across all 24 maps**:

| atlas           | sequences                                                    | every map |
| --------------- | ------------------------------------------------------------ | --------- |
| `hud_msg_icons` | 24 (weapons + grenade + button glyphs)                       | ✅ same   |
| `hud_waypoints` | 7 (flag / oddball / koth + arrow / chevron / ring / diamond) | ✅ same   |

Campaign maps add **no richer waypoint/marker icon set** — their only
campaign-unique "marker"-named bitmaps are **level decals** (wall symbols like
`decal symbol airlock/bridge/cryo`, `hologram markers`, `holo halo ring
markers`) and the `marine_helmet_hud`, none of which are reusable HUD marker
icons.

So **the complete minimap marker palette is the 7 `hud_waypoints` glyphs plus
the weapon glyphs in `hud_msg_icons`** — which the icon extractor already pulls
(17 icons). The "unused asset slots" the modded build repurposes for
power-weapon / powerup waypoints are the generic `hud_waypoints` glyphs (ring /
diamond / chevron / arrow), already extracted. There is nothing new on disk to
pull for the minimap; a marker that looks different from those 7 glyphs would be
**art the mod added** (would need the modded files). See
[`scripts/game-icons/README.md`](../scripts/game-icons/README.md).

## What the catalog covers

24 maps: 13 multiplayer + 10 campaign (`a10`–`d40`) + `ui`. Campaign maps are
**6–9× richer** than MP (e.g. 100 models / 18 bipeds in `c10` vs ~50 / 2 in an
MP map) — they are the real asset source for the 3D stage.

### Per-map inventory (selected groups)

| map      | type        |  tags | bitm | mode | sbsp | vehi | bipd | scen |  MB |
| -------- | ----------- | ----: | ---: | ---: | ---: | ---: | ---: | ---: | --: |
| a10      | campaign    |  3357 |  485 |   76 |    9 |   13 |   14 |   33 | 262 |
| a30      | campaign    |  3215 |  445 |   56 |    2 |    6 |   14 |   15 | 203 |
| a50      | campaign    |  3311 |  458 |   75 |    4 |    4 |   15 |   25 | 239 |
| b30      | campaign    |  3325 |  467 |   86 |    2 |    7 |   11 |   27 | 214 |
| b40      | campaign    |  3647 |  517 |   82 |   13 |    8 |   12 |   26 | 267 |
| c10      | campaign    |  3369 |  476 |  100 |    6 |    2 |   18 |   64 | 241 |
| c20      | campaign    |  1860 |  334 |   63 |    4 |    0 |    8 |   10 | 135 |
| c40      | campaign    |  3208 |  510 |  102 |   13 |    6 |   17 |   27 | 244 |
| d20      | campaign    |  2743 |  472 |   89 |    5 |    5 |   15 |   30 | 196 |
| d40      | campaign    |  2978 |  525 |   98 |   10 |    6 |   19 |   28 | 262 |
| _13× MP_ | multiplayer | ~1.7k | ~340 |  ~50 |    1 |    3 |    2 | 1–19 | ~42 |
| ui       | ui          |   983 |  195 |    5 |    1 |    0 |    0 |    1 |  32 |

### Global unique-asset counts (renderable / geometry groups)

| group  | meaning                                | unique | campaign-only | mp-only |
| ------ | -------------------------------------- | -----: | ------------: | ------: |
| `sbsp` | level geometry (structure BSP)         |     82 |            68 |      13 |
| `mode` | object models (gbxmodel)               |    336 |           237 |      24 |
| `vehi` | vehicles                               |     25 |            22 |       2 |
| `bipd` | bipeds (characters)                    |     43 |            41 |       1 |
| `weap` | weapons                                |     23 |             8 |       3 |
| `eqip` | equipment (powerups / grenades / ammo) |     14 |             0 |       1 |
| `scen` | scenery                                |    200 |           154 |      12 |
| `mach` | device_machine (doors / lifts)         |     82 |            80 |       1 |
| `ctrl` | device_control (switches)              |      9 |             9 |       0 |
| `proj` | projectiles                            |     28 |            12 |       0 |

(Largest groups overall, for scale: `snd!` 2879, `bitm` 1325, `mode` 336,
`scen` 200 — full per-group counts in the JSON `totals`.)

### Campaign-only highlights (the richer set the 3D stage unlocks)

- **Vehicles (22 campaign-only):** banshee, wraith, ghost, scorpion, **pelican**,
  **covenant dropship** (`c_dropship`) + its gun turret, **fighter-bomber**,
  lifepods (entry / docked / atmosphere), cryotube, pilot/pod chairs, and
  `scenery\halo\halo` (the ring itself). (`warthog` is the only MP+campaign
  vehicle; `ghost_mp` / `scorpion_mp` are the MP variants.)
- **Bipeds (41 campaign-only):** the full cast — **elite** (+ special), **grunt**
  (+ specops), **jackal** (+ major), **hunter**, **flood** (infection /
  carrier / combat elite / combat human / captain), **sentinel**, **monitor**
  (343 Guilty Spark), **marines** (armored / sniper / wounded / suicidal),
  **captain Keyes**, **Cortana**, **Johnson**, engineer, pilot, crewman.
- **Weapons (8 campaign-only):** energy sword, fuel rod gun (+ hunter variant),
  sentinel beam, and the covenant vehicle guns (banshee gun, wraith mortar,
  dropship gun).

The MP-only entities are the small expected set: `ball` + `flag` + `gravity
rifle` weapons, `ghost_mp` / `scorpion_mp` vehicles, and the MP `cyborg_mp`
biped.

## Use for the 3D stage

The renderable-geometry path is: `sbsp` (level mesh) for the map, plus `mode`
(gbxmodel) referenced by each placed `vehi` / `bipd` / `scen` / `mach` object.
The JSON's `assets` index gives, per group, every tag path and which maps ship
it — so a 3D extractor knows exactly which models exist, where to read them, and
which are shared vs level-specific. Geometry decoding (the `mode` / `sbsp`
vertex+index buffers) is a separate, larger effort; this catalog is the index it
will consume.
