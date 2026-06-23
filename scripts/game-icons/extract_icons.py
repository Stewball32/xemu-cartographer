#!/usr/bin/env python3
"""
Extract Halo in-game HUD/UI ICONS from stock game files into the visualizer's
served icon cache (PNG with alpha) + a manifest.

This is the icon analogue of halo-offset-mapper's map-manifest extractor: it
REUSES that project's swizzle-aware DXT/Xbox bitmap decoder (`bitmaps.py`) and
`.map` cache parser (`halomap.py`) rather than reimplementing them, then crops
the per-item sprites out of Halo's HUD atlases and writes them where the
SvelteKit visualizer can serve them.

Where each icon comes from (Halo 1 / CE), derived from the map's own tag data:

  * Weapons  : `ui\\hud\\bitmaps\\combined\\hud_msg_icons` — the pickup-message
               icon atlas. The per-weapon sprite index is read AUTHORITATIVELY
               from each `weapon_hud_interface` (wphi) tag's "messaging
               information" sequence index (s16 @ 0x13C), keyed by the weapon's
               own tag folder. So the set adapts to whatever weapons a build
               ships — nothing is hardcoded to stock CE's weapon list.
  * Grenades : `hud_msg_icons` generic grenade glyph (frag + plasma share it in
               CE — the grenade_hud_interface stores no per-type icon index).
  * Objective: `ui\\hud\\bitmaps\\combined\\hud_waypoints` — flag / oddball
               (skull) / KotH (crown) / generic nav markers. These sequences
               are unnamed in the tag, so the index→meaning map is a small
               curated, documented constant (see CE_OBJECTIVES).

NOT extractable from CE HUD data (no dedicated sprite): powerups (over shield /
active camo) and per-vehicle silhouettes. Those keep the visualizer's generic
fallback markers; H2 (which DOES carry powerup pickup icons) drops in as another
GAMES entry with its own atlas tags + index map.

LEGAL: the decoded PNGs are game-derived art. Like the map-preview PNGs, they
are a LOCAL, git-ignored cache regenerated from the user's own legally-owned
game files — never committed. Only this script + the factual manifest schema
live in git.

Usage:
  python3 scripts/game-icons/extract_icons.py \
      [--game haloce] \
      [--mapper-dir ../halo-offset-mapper/scripts/mapmanifest] \
      [--maps-dir "../halo-offset-mapper/xbe-dropzone/Halo CE/maps"] \
      [--map bloodgulch.map] \
      [--out sveltekit/static/game-icons]
"""

from __future__ import annotations

import argparse
import datetime
import json
import os
import re
import struct
import sys

SCHEMA_VERSION = 1
GENERATOR = "xemu-cartographer/game-icons"

# Offset of the weapon_hud_interface "messaging information" sequence index
# (s16) — the index into hud_msg_icons used for that weapon's pickup glyph.
# Verified across all stock CE weapon wphi tags (distinct, in-range values that
# match weapon identity: sniper=4 rocket=5 plasma_pistol=6 flamethrower=7
# assault_rifle=8 pistol=9 needler=10 plasma_rifle=11 shotgun=12).
WPHI_MSG_SEQ_OFFSET = 0x13C

# --- per-game icon-set spec ------------------------------------------------
# CE first. H2 (or any other engine) is added as another entry with its own
# atlas tag paths + objective index map; the rest of the pipeline is shared.
GAMES = {
    "haloce": {
        # Any stock multiplayer map carries the shared ui HUD atlases + wphi
        # set. bloodgulch is the canonical pick.
        "default_map": "bloodgulch.map",
        "msg_icons_tag": r"combined\hud_msg_icons",
        "waypoints_tag": r"combined\hud_waypoints",
        # generic grenade glyph in hud_msg_icons (frag + plasma share it)
        "grenade_seq": 13,
        # hud_waypoints sequence index → objective key (sequences are unnamed in
        # the tag; identified by eye from the decoded atlas and documented here)
        "objectives": {
            "flag": 2,      # CTF flag
            "oddball": 4,   # skull
            "koth": 5,      # crown (King of the Hill)
            "nav": 0,       # generic downward marker (race / unclassified)
        },
    },
}


# --------------------------------------------------------------------------
def slugify(s: str) -> str:
    """Match the frontend's key derivation: lowercase, non-alnum runs -> '_'."""
    return re.sub(r"[^a-z0-9]+", "_", s.strip().lower()).strip("_")


def find_bitm(m, needle: str):
    nn = needle.replace("/", "\\").lower()
    for t in m.tags_by_group("bitm"):
        p = t.path.lower()
        if p.endswith(nn) or nn in p:
            return t
    return None


def crop_sprite(B, m, tag, seq_index: int):
    """Crop sprite[0] of sequence `seq_index` from a sprite-atlas bitm tag.

    Returns a PIL RGBA Image, or None if the sequence / sprite is absent.
    Bitmap-tag layout (Halo 1): sequences reflexive @0x54, bitmap-data @0x60;
    each sequence is 64 bytes with a sprites reflexive @0x34; each sprite is 32
    bytes: s16 bitmap_index @0x00, then f32 left/right/top/bottom @0x08 as
    normalized texture coords into bitmap_data[bitmap_index].
    """
    o = m.off(tag.meta_vaddr)
    seq_cnt, seq_addr = struct.unpack_from("<II", m.image, o + 0x54)
    if seq_index >= seq_cnt or not m.in_image(seq_addr):
        return None
    pages = [B.decode_to_image(m.image, bd) for bd in B.all_bitmap_data(m, tag)]
    if not pages:
        return None
    base = m.off(seq_addr) + seq_index * 64
    spr_cnt, spr_addr = struct.unpack_from("<II", m.image, base + 0x34)
    if not spr_cnt or not m.in_image(spr_addr):
        return None
    sb = m.off(spr_addr)  # sprite[0]
    bm_idx = struct.unpack_from("<h", m.image, sb)[0]
    left, right, top, bottom = struct.unpack_from("<ffff", m.image, sb + 0x08)
    if bm_idx < 0 or bm_idx >= len(pages):
        return None
    pg = pages[bm_idx]
    w, h = pg.size
    box = (round(left * w), round(top * h), round(right * w), round(bottom * h))
    if box[2] <= box[0] or box[3] <= box[1]:
        return None
    return pg.crop(box)


def to_white_glyph(img):
    """Normalize a decoded HUD sprite to a crisp WHITE silhouette with real
    alpha, so every icon reads uniformly on the dark minimap (tinted by a
    category ring in the view layer) regardless of source pixel format.

    Halo's two HUD atlases encode the glyph differently:
      * hud_msg_icons (weapons / grenade) are A8Y8 — the glyph is the (semi-
        transparent) line-art in the ALPHA channel; luminance just dims it.
      * hud_waypoints (objectives) are AY8 — alpha is constant-opaque and the
        shape lives in LUMINANCE (so as-is it'd be a dark opaque box).
    We pick whichever channel carries the shape, normalize + contrast-boost it
    into a coverage mask, and paint it solid white. Line-art stays line-art;
    filled markers stay filled.
    """
    import numpy as np

    arr = np.array(img.convert("RGBA")).astype(np.float32)
    a = arr[..., 3]
    lum = arr[..., :3].mean(axis=2)
    cov = lum if a.min() >= 250 else a  # AY8 (opaque) -> luminance, else alpha
    m = float(cov.max())
    cov = cov / m if m > 0 else cov
    # drop <12% noise, saturate by ~82% so faint strokes read as solid white
    lo, hi = 0.12, 0.82
    cov = np.clip((cov - lo) / (hi - lo), 0.0, 1.0)
    out = np.zeros_like(arr)
    out[..., 0] = out[..., 1] = out[..., 2] = 255.0
    out[..., 3] = cov * 255.0
    from PIL import Image
    return Image.fromarray(out.astype(np.uint8), "RGBA")


def trim_and_square(img, margin_frac: float = 0.12):
    """Trim transparent border to the alpha bbox, then pad to a centered square
    with a small transparent margin so every icon renders uniformly."""
    from PIL import Image

    alpha = img.split()[-1]
    bbox = alpha.getbbox()
    if bbox:
        img = img.crop(bbox)
    side = max(img.size)
    pad = max(1, round(side * margin_frac))
    side += 2 * pad
    canvas = Image.new("RGBA", (side, side), (0, 0, 0, 0))
    canvas.paste(img, ((side - img.width) // 2, (side - img.height) // 2))
    return canvas


def weapon_seq_map(m) -> dict[str, int]:
    """{weapon_folder_slug: hud_msg_icons sequence index} read authoritatively
    from each `weapons\\...` weapon_hud_interface tag."""
    out: dict[str, int] = {}
    for t in m.tags_by_group("wphi"):
        parts = t.path.replace("/", "\\").split("\\")
        if len(parts) < 2 or parts[0].lower() != "weapons":
            continue  # skip vehicle-gun + ui hud wphi tags
        o = m.off(t.meta_vaddr)
        seq = struct.unpack_from("<h", m.image, o + WPHI_MSG_SEQ_OFFSET)[0]
        if 0 <= seq < 1024:
            out[slugify(parts[1])] = seq
    return out


# --------------------------------------------------------------------------
def extract(game: str, mapper_dir: str, maps_dir: str, map_file: str | None,
            out_root: str) -> dict:
    sys.path.insert(0, mapper_dir)
    import halomap as H  # noqa: E402  (reused decoder — see module docstring)
    import bitmaps as B  # noqa: E402

    spec = GAMES[game]
    map_name = map_file or spec["default_map"]
    map_path = os.path.join(maps_dir, map_name)
    if not os.path.isfile(map_path):
        raise SystemExit(f"map not found: {map_path}\n"
                         f"(set --maps-dir to your Halo CE maps directory)")
    m = H.parse_map(map_path)

    out_dir = os.path.join(out_root, game)
    os.makedirs(out_dir, exist_ok=True)

    msg_tag = find_bitm(m, spec["msg_icons_tag"])
    wp_tag = find_bitm(m, spec["waypoints_tag"])
    if msg_tag is None:
        raise SystemExit(f"{map_name}: HUD msg-icons atlas not found "
                         f"({spec['msg_icons_tag']})")

    icons: dict[str, dict] = {}
    tag_map: dict[str, dict] = {"weapon": {}, "grenade": {}, "objective": {}}
    flags: list[str] = []

    def save(key: str, img, source: str, category: str):
        sq = trim_and_square(to_white_glyph(img))
        rel = f"{key}.png"
        sq.save(os.path.join(out_dir, rel))
        icons[key] = {"file": rel, "w": sq.width, "h": sq.height,
                      "source": source, "category": category}

    # --- weapons (authoritative wphi sequence indices) ---
    wmap = weapon_seq_map(m)
    for slug, seq in sorted(wmap.items()):
        if "grenade" in slug:  # grenades classified separately, below
            continue
        img = crop_sprite(B, m, msg_tag, seq)
        if img is None:
            flags.append(f"weapon '{slug}' seq {seq} crop failed")
            continue
        key = f"weapon_{slug}"
        save(key, img, f"hud_msg_icons#{seq}", "weapon")
        tag_map["weapon"][slug] = key

    # --- grenade (single generic glyph) ---
    gimg = crop_sprite(B, m, msg_tag, spec["grenade_seq"])
    if gimg is not None:
        save("grenade", gimg, f"hud_msg_icons#{spec['grenade_seq']}", "grenade")
        tag_map["grenade"]["*"] = "grenade"
    else:
        flags.append("grenade glyph crop failed")

    # --- objective markers (curated index map over hud_waypoints) ---
    if wp_tag is None:
        flags.append(f"waypoints atlas not found ({spec['waypoints_tag']})")
    else:
        for name, seq in spec["objectives"].items():
            img = crop_sprite(B, m, wp_tag, seq)
            if img is None:
                flags.append(f"objective '{name}' seq {seq} crop failed")
                continue
            key = f"objective_{name}"
            save(key, img, f"hud_waypoints#{seq}", "objective")
            tag_map["objective"][name] = key

    manifest = {
        "schema_version": SCHEMA_VERSION,
        "generated_by": GENERATOR,
        "generated_at": datetime.datetime.now(datetime.timezone.utc)
        .isoformat(timespec="seconds"),
        "game": game,
        "source_map": map_name,
        "flags": flags,
        "icons": icons,
        "tag_map": tag_map,
    }
    with open(os.path.join(out_dir, "manifest.json"), "w") as fh:
        json.dump(manifest, fh, indent=2)
    return manifest


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    repo_root = os.path.abspath(os.path.join(here, "..", ".."))
    default_mapper = os.path.join(
        repo_root, "..", "halo-offset-mapper", "scripts", "mapmanifest")
    default_maps = os.environ.get("HALO_CE_MAPS_DIR") or os.path.join(
        repo_root, "..", "halo-offset-mapper", "xbe-dropzone", "Halo CE", "maps")
    default_out = os.path.join(repo_root, "sveltekit", "static", "game-icons")

    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--game", default="haloce", choices=sorted(GAMES))
    ap.add_argument("--mapper-dir", default=default_mapper,
                    help="halo-offset-mapper mapmanifest dir (decoder reuse)")
    ap.add_argument("--maps-dir", default=default_maps,
                    help="directory of unpacked stock .map files")
    ap.add_argument("--map", default=None, help="specific .map file to read")
    ap.add_argument("--out", default=default_out,
                    help="served icon cache root (per-game subdir created)")
    args = ap.parse_args()

    if not os.path.isdir(args.mapper_dir):
        raise SystemExit(f"mapper-dir not found: {args.mapper_dir}\n"
                         f"(clone halo-offset-mapper beside xemu-cartographer, "
                         f"or pass --mapper-dir)")

    man = extract(args.game, args.mapper_dir, args.maps_dir, args.map, args.out)
    print(json.dumps({k: man[k] for k in ("game", "source_map", "flags")},
                     indent=2))
    print(f"  icons: {len(man['icons'])} -> "
          f"{os.path.join(args.out, args.game)}/")
    for key, meta in sorted(man["icons"].items()):
        print(f"    {key:28} {meta['w']}x{meta['h']:<4} {meta['source']}")


if __name__ == "__main__":
    main()
