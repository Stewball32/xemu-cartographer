# Halo armor / player color palettes (game-exact)

`halo-armor-palettes.json` holds the **exact** in-game armor/player colors for
Halo: CE (Xbox) and Halo 2 (Xbox), keyed by game → color index → `{name, hex, rgb}`.
`halo-armor-palettes.ts` exposes them as typed arrays (`CE_ARMOR_COLORS`,
`H2_ARMOR_COLORS`, `PALETTE_META`).

These replace the earlier approximate swatches that were derived from the app's
`--color-armor-*` oklch tokens.

## Halo: CE — 18 colors

Index order is the **c20 palette order**, i.e. the `blam.sav` profile color enum
(`u32 @ 0x18`). Source: **c20.reclaimers.net** hard-coded multiplayer armor colors
(armor color values reverse-engineered by MosesOfEgypt; in-game testing by
Lavadeeto). <https://c20.reclaimers.net/h1/engine/hard-coded-data/>

| # | Name | Hex | RGB | MCC name |
|--:|------|-----|-----|----------|
| 0 | White | `#FFFFFF` | 255, 255, 255 | |
| 1 | Black | `#000000` | 0, 0, 0 | |
| 2 | Red | `#FE0000` | 254, 0, 0 | |
| 3 | Blue | `#0201E3` | 2, 1, 227 | |
| 4 | Gray | `#707E71` | 112, 126, 113 | Grey |
| 5 | Yellow | `#FFFF01` | 255, 255, 1 | |
| 6 | Green | `#00FF01` | 0, 255, 1 | |
| 7 | Pink | `#FF56B9` | 255, 86, 185 | |
| 8 | Purple | `#AB10F4` | 171, 16, 244 | |
| 9 | Cyan | `#01FFFF` | 1, 255, 255 | |
| 10 | Cobalt | `#6493ED` | 100, 147, 237 | Light Blue |
| 11 | Orange | `#FF7F00` | 255, 127, 0 | |
| 12 | Teal | `#1ECC91` | 30, 204, 145 | Lapis Lazuli |
| 13 | Sage | `#006401` | 0, 100, 1 | Forest Green |
| 14 | Brown | `#603814` | 96, 56, 20 | |
| 15 | Tan | `#C69C6C` | 198, 156, 108 | |
| 16 | Maroon | `#9D0B0E` | 157, 11, 14 | Cherry |
| 17 | Salmon | `#F5999E` | 245, 153, 158 | |

These are diffuse tint values as authored; in-game appearance is further affected
by cubemaps/specular maps.

## Halo 2 — 18 colors

Index order is `e_player_color`, matching the profile appearance bytes:
`0x118` armor primary, `0x119` armor secondary, `0x11A` emblem primary,
`0x11B` emblem secondary. **The same 18 colors are used for both armor and emblem
colors** (confirmed — single palette).

Source: **extracted from the game itself** — the Halo 2 (Xbox) `mainmenu.map`
globals player-color table, stored as float32 RGB (`value * 255`) at file offset
`0x379B63C`. Verified **byte-identical** in `shared.map` (`0xAAF2F80`), confirming
it is the canonical, game-wide palette. Build `02.09.27.09809`. Values were
authored as 8-bit RGB, so `rgb = round(float * 255)`.

| # | Name | Hex | RGB |
|--:|------|-----|-----|
| 0 | White | `#FDFEFF` | 253, 254, 255 |
| 1 | Steel | `#535353` | 83, 83, 83 |
| 2 | Red | `#BE2C2C` | 190, 44, 44 |
| 3 | Orange | `#F57A1F` | 245, 122, 31 |
| 4 | Gold | `#F5D22C` | 245, 210, 44 |
| 5 | Olive | `#9FAC59` | 159, 172, 89 |
| 6 | Green | `#21922F` | 33, 146, 47 |
| 7 | Sage | `#235644` | 35, 86, 68 |
| 8 | Cyan | `#16A0A0` | 22, 160, 160 |
| 9 | Teal | `#36747A` | 54, 116, 122 |
| 10 | Cobalt | `#416C8F` | 65, 108, 143 |
| 11 | Blue | `#28459B` | 40, 69, 155 |
| 12 | Violet | `#6A4EB6` | 106, 78, 182 |
| 13 | Purple | `#75466D` | 117, 70, 109 |
| 14 | Pink | `#FB9BC9` | 251, 155, 201 |
| 15 | Crimson | `#981244` | 152, 18, 68 |
| 16 | Brown | `#664E3E` | 102, 78, 62 |
| 17 | Tan | `#B19256` | 177, 146, 86 |

## Wiring

`src/lib/utils/emblem.ts` derives `H2_COLORS` / `CE_COLORS` (the `{name, hex}`
shape the editors bind to) from these arrays, so `AppearanceStudio.svelte`,
`CharacterPreview.svelte`, `EmblemPreview.svelte` etc. pick up the exact values
with no further changes. The CE profile editor (`CESettingsEditor.svelte`) only
enumerates White/Red/Blue today; point its color list at `CE_ARMOR_COLORS` to get
the full 18-color carousel.
