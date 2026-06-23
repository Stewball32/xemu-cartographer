# Game-icon extraction (visualizer minimap markers)

Decodes Halo's in-game **HUD/UI icon sprites** from the user's own game files
into a local, git-ignored cache that the 2D top-down visualizer
(`/visualizer/[instance]/`) serves as map markers — real weapon / grenade /
objective glyphs instead of generic diamonds.

This is the icon analogue of [halo-offset-mapper]'s map-manifest extractor and
**reuses its decoder**: the swizzle-aware DXT / Xbox bitmap decoder
(`bitmaps.py`) and `.map` cache parser (`halomap.py`). We don't reimplement
bitmap decoding here — only the icon-specific cropping + tag→icon mapping.

## Legal boundary

The decoded PNGs are **game-derived art** and are **never committed** — same
stance as the map-preview PNGs. `extract_icons.py` writes them into
`sveltekit/static/game-icons/<game>/`, which is git-ignored. The committed
product is this script + the frontend consumer (`$lib/utils/game-icons.ts`),
which **degrades to generic markers** when the cache was never regenerated.
Regenerate locally from your own legally-owned game files.

## Prerequisites

- **[halo-offset-mapper] cloned beside this repo** (`../halo-offset-mapper`) —
  the decoder lives there. Override with `--mapper-dir` if it's elsewhere.
- **Python deps:** `numpy`, `pillow` (already required by the decoder).
- **Unpacked stock Halo CE `.map` files.** Any stock multiplayer map carries the
  shared `ui` HUD atlases; `bloodgulch.map` is the default. Point `--maps-dir`
  (or `$HALO_CE_MAPS_DIR`) at your maps directory.

## Run

```sh
# defaults: --game haloce, decoder at ../halo-offset-mapper, maps at
# ../halo-offset-mapper/xbe-dropzone/Halo CE/maps, out at sveltekit/static/game-icons
python3 scripts/game-icons/extract_icons.py

# explicit
python3 scripts/game-icons/extract_icons.py \
  --maps-dir "/path/to/Halo CE/maps" --map bloodgulch.map
```

Output: `sveltekit/static/game-icons/haloce/<key>.png` + `manifest.json`. The
SvelteKit static adapter copies `static/` into `pb_public/` on `pnpm build`, so
no extra serving wiring is needed (dev: vite serves `static/`).

## What gets extracted (Halo: CE)

All sources are derived from the map's **own tag data** — nothing about the
stock weapon set is hardcoded:

| Icons                                                                                                 | Source                | Mapping                                                                                                                                                                                  |
| ----------------------------------------------------------------------------------------------------- | --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Weapons** (pistol, AR, shotgun, sniper, rocket, plasma pistol, plasma rifle, needler, flamethrower) | `hud_msg_icons` atlas | **Authoritative** — each `weapon_hud_interface` (wphi) tag's messaging-info sequence index (s16 @ `0x13C`), keyed by the weapon's own tag folder. Adapts to any build's weapon set.      |
| **Grenade**                                                                                           | `hud_msg_icons`       | Single generic glyph. **VERIFIED frag + plasma share it** in stock CE (both grenade_hud_interface tags use the same pickup-message AND the same HUD-counter glyph; only a tint differs). |
| **Objective markers** (CTF flag, oddball/skull, KotH/crown)                                           | `hud_waypoints` atlas | Curated index map (`GAMES["haloce"]["waypoints"]`) — these sequences are unnamed in the tag, so the index→meaning is identified by eye and documented in the script.                     |
| **Generic markers** (arrow, double-chevron, ring, diamond)                                            | `hud_waypoints` atlas | The rest of the waypoint palette — the "unused waypoint slots" the modded build repurposes as power-weapon / powerup waypoints (see below).                                              |

Each sprite is normalized to a **crisp white silhouette with real alpha** (the
two atlases encode the glyph in different channels — line-art in alpha for
`hud_msg_icons`, luminance for `hud_waypoints`), so they read uniformly on the
dark minimap and are tinted by a category/team-colored ring in the view layer.

### Powerups → waypoint markers (no dedicated CE powerup sprite)

Stock CE has **no dedicated 2D HUD pickup icon** for powerups (the
`powerups\*\bitmaps\*` tags are 3D model textures, not icons; the equipment tags
carry no per-type message-icon index). The full `hud_waypoints` atlas is exactly
**7 glyphs**, fully used, with no extra/hidden art. So the markers the modded
build shows for power weapons + powerups are **the existing generic waypoint
glyphs** (ring / diamond / chevron / arrow) that stock gameplay barely uses —
the "unused waypoint slots" on disk.

`GAMES["haloce"]["powerups"]` maps each powerup tag folder → a waypoint-marker
key (over shield → `marker_diamond`, active camo → `marker_ring`, …), written
into the manifest's `tag_map.powerup` so the visualizer renders a real waypoint
marker instead of the generic diamond. **This is a sensible DEFAULT assignment**
— the exact glyph each powerup/power-weapon gets in Stewart's mod needs his
modded files (the dropzone is stock-only now). Adjust the one `powerups` table
(and, to override a power weapon's silhouette with a marker, add a `marker_*`
entry under the weapon's slug) and re-run; the mapping flows through the
manifest with no frontend change. Power weapons keep their **weapon silhouette**
by default (more informative on a minimap than a generic waypoint).

Per-vehicle silhouettes are still not in CE HUD data → vehicles keep the generic
fallback rectangle.

## Adding Halo 2 (or another build)

Add a `GAMES["halo2"]` entry with that engine's atlas tag paths + waypoint /
powerup maps; the crop / normalize / manifest pipeline is shared. The visualizer
resolves icons from the manifest's `tag_map` (curated) first, then an optimistic
derived key (`weapon_<folder>`, `vehicle_<folder>`, …), so an H2 manifest that
ships `vehicle_*` sprites or its own `tag_map` entries lights those up with **no
frontend change**.

## How the visualizer consumes it

`$lib/utils/game-icons.ts` fetches `<game>/manifest.json` (best-effort — any
failure → empty set → generic markers) and resolves each object's icon URL: the
manifest's curated `tag_map` first (authoritative weapon keys + the powerup →
waypoint-marker mapping that can't be derived from the tag), then an optimistic
derived key gated on what the manifest ships. `TopDownMap.svelte` renders the
icon, else the prior generic shape. Resolution is pure + unit-tested
(`game-icons.test.ts`); `slugifyTagSegment` must stay in sync with this script's
`slugify`. Weapon icons are **per-object from the live feed** (each item's own
tag), so a weapon only renders where it actually spawns on the loaded map —
there is no fixed per-map weapon set.

[halo-offset-mapper]: ../../halo-offset-mapper
