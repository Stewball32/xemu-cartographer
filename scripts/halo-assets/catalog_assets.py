#!/usr/bin/env python3
"""
Catalog the assets in a set of Halo: CE (Xbox) .map files — an inventory/index
of what 3D models, level geometry, vehicles, weapons, bipeds, scenery, etc. each
map ships, across the whole campaign + multiplayer set.

This is a FACTUAL metadata inventory (tag paths + groups + counts — like the map
manifest / offset maps), NOT decoded art, so the output JSON is committable. It
exists to map "everything available" for the future 3D-model visualizer stage +
asset reuse, even before any geometry is actually ripped.

REUSES halo-offset-mapper's `.map` cache parser (`halomap.py`) — the same
decoder the icon extractor uses. Xbox Halo CE maps are MONOLITHIC (each embeds
all its tags; there is no shared bitmaps.map / sounds.map / loc.map — that's the
PC layout), so every map is inventoried standalone.

Tag groups of interest for the 3D stage (one renderable concept each):
  sbsp structure_bsp (level geometry)   mode gbxmodel (object meshes)
  vehi vehicle   bipd biped   weap weapon   eqip equipment
  scen scenery   mach device_machine   ctrl device_control   proj projectile

Usage:
  python3 scripts/halo-assets/catalog_assets.py \
      [--maps-dir "../halo-offset-mapper/xbe-dropzone/Halo CE/maps"] \
      [--mapper-dir ../halo-offset-mapper/scripts/mapmanifest] \
      [--out docs/halo-ce-asset-catalog.json]
"""

from __future__ import annotations

import argparse
import collections
import datetime
import json
import os
import struct
import sys

SCHEMA_VERSION = 1
GENERATOR = "xemu-cartographer/halo-assets"

# Detailed global index (tag_path → maps it appears in) is built for these
# "renderable entity / geometry" groups; everything else is counted only.
ENTITY_GROUPS = [
    "sbsp",  # level geometry (structure BSP)
    "mode",  # object models (gbxmodel)
    "vehi",  # vehicles
    "bipd",  # bipeds (player / AI characters)
    "weap",  # weapons
    "eqip",  # equipment (powerups, grenades, ammo)
    "scen",  # scenery
    "mach",  # device_machine (doors, lifts, ...)
    "ctrl",  # device_control (switches, ...)
    "proj",  # projectiles
]

# The shared HUD icon atlases — reported per map to document that they're
# identical across the whole set (the icon extractor's source of truth).
HUD_ATLASES = [r"combined\hud_msg_icons", r"combined\hud_waypoints"]


def seq_count(m, needle: str):
    nn = needle.replace("/", "\\").lower()
    for t in m.tags_by_group("bitm"):
        if t.path.lower().endswith(nn):
            o = m.off(t.meta_vaddr)
            return struct.unpack_from("<II", m.image, o + 0x54)[0]
    return None


def catalog(maps_dir: str, mapper_dir: str) -> dict:
    sys.path.insert(0, mapper_dir)
    import halomap as H  # noqa: E402  (reused decoder)

    files = sorted(f for f in os.listdir(maps_dir) if f.lower().endswith(".map"))
    maps_out: list[dict] = []
    # group -> tag_path -> sorted set of map basenames
    index: dict[str, dict[str, list[str]]] = {g: {} for g in ENTITY_GROUPS}
    group_totals: collections.Counter = collections.Counter()
    unique_by_group: dict[str, set] = collections.defaultdict(set)
    errors: list[dict] = []

    for fn in files:
        base = os.path.splitext(fn)[0]
        path = os.path.join(maps_dir, fn)
        try:
            m = H.parse_map(path)
        except Exception as e:  # noqa: BLE001
            errors.append({"file": fn, "error": str(e)})
            continue

        groups = collections.Counter(t.group for t in m.tags)
        for g, c in groups.items():
            group_totals[g] += c
        for t in m.tags:
            unique_by_group[t.group].add(t.path)

        for g in ENTITY_GROUPS:
            for t in m.tags_by_group(g):
                index[g].setdefault(t.path, []).append(base)

        hud = {a.split("\\")[-1]: seq_count(m, a) for a in HUD_ATLASES}
        maps_out.append({
            "file": fn,
            "name": m.name,
            "type": m.map_type_name,
            "build": m.build,
            "compressed": m.compressed,
            "decompressed_mb": round(len(m.image) / 1024 / 1024),
            "tags": len(m.tags),
            "hud_atlas_sequences": hud,
            "groups": dict(sorted(groups.items(), key=lambda kv: -kv[1])),
        })

    # sort each index entry's map list + the entries themselves
    for g in index:
        index[g] = {p: sorted(set(maps)) for p, maps in sorted(index[g].items())}

    mp_names = {mm["name"] for mm in maps_out if mm["type"] == "multiplayer"}
    camp_names = {mm["name"] for mm in maps_out if mm["type"] == "campaign"}

    def split_only(g):
        camp_only, mp_only = [], []
        for p, maps in index[g].items():
            s = set(maps)
            if s & camp_names and not (s & mp_names):
                camp_only.append(p)
            elif s & mp_names and not (s & camp_names):
                mp_only.append(p)
        return camp_only, mp_only

    summary = {}
    for g in ENTITY_GROUPS:
        camp_only, mp_only = split_only(g)
        summary[g] = {
            "unique": len(index[g]),
            "campaign_only": len(camp_only),
            "mp_only": len(mp_only),
        }

    return {
        "schema_version": SCHEMA_VERSION,
        "generated_by": GENERATOR,
        "generated_at": datetime.datetime.now(datetime.timezone.utc)
        .isoformat(timespec="seconds"),
        "source": "Xbox Halo: CE monolithic .map files (no shared "
                  "bitmaps.map/sounds.map/loc.map; each map embeds its tags)",
        "maps_dir": maps_dir,
        "errors": errors,
        "totals": {
            "maps": len(maps_out),
            "unique_tags_by_group": {
                g: len(s) for g, s in sorted(unique_by_group.items(),
                                             key=lambda kv: -len(kv[1]))
            },
            "tag_instances_by_group": dict(group_totals.most_common()),
        },
        "entity_summary": summary,
        "maps": maps_out,
        "assets": index,
    }


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    repo_root = os.path.abspath(os.path.join(here, "..", ".."))
    default_mapper = os.path.join(
        repo_root, "..", "halo-offset-mapper", "scripts", "mapmanifest")
    default_maps = os.environ.get("HALO_CE_MAPS_DIR") or os.path.join(
        repo_root, "..", "halo-offset-mapper", "xbe-dropzone", "Halo CE", "maps")
    default_out = os.path.join(repo_root, "docs", "halo-ce-asset-catalog.json")

    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--maps-dir", default=default_maps)
    ap.add_argument("--mapper-dir", default=default_mapper)
    ap.add_argument("--out", default=default_out)
    args = ap.parse_args()

    if not os.path.isdir(args.maps_dir):
        raise SystemExit(f"maps-dir not found: {args.maps_dir}")
    if not os.path.isdir(args.mapper_dir):
        raise SystemExit(f"mapper-dir not found: {args.mapper_dir}")

    cat = catalog(args.maps_dir, args.mapper_dir)
    os.makedirs(os.path.dirname(args.out), exist_ok=True)
    with open(args.out, "w") as fh:
        json.dump(cat, fh, indent=1)

    t = cat["totals"]
    print(f"cataloged {t['maps']} maps -> {args.out}")
    print(f"  unique tags by group (top): "
          + ", ".join(f"{g}:{n}" for g, n in
                      list(t['unique_tags_by_group'].items())[:10]))
    print("  entity inventory (unique / campaign-only / mp-only):")
    for g, s in cat["entity_summary"].items():
        print(f"    {g}: {s['unique']:4d}  campaign-only={s['campaign_only']:3d}"
              f"  mp-only={s['mp_only']:3d}")
    if cat["errors"]:
        print(f"  errors: {len(cat['errors'])}")


if __name__ == "__main__":
    main()
