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

| Icons                                                                                                 | Source                | Mapping                                                                                                                                                                             |
| ----------------------------------------------------------------------------------------------------- | --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Weapons** (pistol, AR, shotgun, sniper, rocket, plasma pistol, plasma rifle, needler, flamethrower) | `hud_msg_icons` atlas | **Authoritative** — each `weapon_hud_interface` (wphi) tag's messaging-info sequence index (s16 @ `0x13C`), keyed by the weapon's own tag folder. Adapts to any build's weapon set. |
| **Grenade**                                                                                           | `hud_msg_icons`       | Single generic glyph (frag + plasma share it in CE).                                                                                                                                |
| **Objective markers** (CTF flag, oddball/skull, KotH/crown, generic nav)                              | `hud_waypoints` atlas | Curated index map (`GAMES["haloce"]["objectives"]`) — these sequences are unnamed in the tag, so the index→meaning is identified by eye and documented in the script.                               |

Each sprite is normalized to a **crisp white silhouette with real alpha** (the
two atlases encode the glyph in different channels — line-art in alpha for
`hud_msg_icons`, luminance for `hud_waypoints`), so they read uniformly on the
dark minimap and are tinted by a category/team-colored ring in the view layer.

**Not extractable from CE HUD data** (no dedicated sprite): powerup pickup icons
(over shield / active camo) and per-vehicle silhouettes. Those keep the
visualizer's **generic fallback markers** (colored diamond / rectangle).

## Adding Halo 2 (or another build)

Add a `GAMES["halo2"]` entry with that engine's atlas tag paths + objective
index map; the crop / normalize / manifest pipeline is shared. The frontend
derives keys optimistically (`weapon_<folder>`, `powerup_<folder>`,
`vehicle_<folder>`, …) and gates them on the manifest, so an H2 manifest that
ships `powerup_*` / `vehicle_*` sprites lights those up with **no frontend
change**.

## How the visualizer consumes it

`$lib/utils/game-icons.ts` fetches `<game>/manifest.json` (best-effort — any
failure → empty set → generic markers), derives an icon key from each object's
tag + class, and resolves it to a served URL if the manifest ships it.
`TopDownMap.svelte` renders the icon, else the prior generic shape. Key
derivation is pure + unit-tested (`game-icons.test.ts`); it must stay in sync
with this script's `slugify`.

[halo-offset-mapper]: ../../halo-offset-mapper
