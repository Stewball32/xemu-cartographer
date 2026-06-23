#!/usr/bin/env python3
"""
Extract the Halo structure-BSP RENDERED GEOMETRY (vertices + triangles) from a
stock `.map` cache file into the 3D visualizer's served geometry cache (a JSON
mesh per level + a manifest).

This is the geometry analogue of the HUD-icon extractor
(scripts/game-icons/extract_icons.py): it REUSES halo-offset-mapper's `.map`
cache parser (`halomap.py`) to resolve the tag space, walks the scenario →
structure-BSP → lightmaps → materials chain, and accumulates every material's
vertex buffer + triangle list into one mesh in Halo world coordinates (X/Y
ground, Z up — the SAME frame the live marker positions use, so the frontend
maps both through one transform and they line up).

HALO 1 (Xbox) STRUCTURE-BSP LAYOUT (verified empirically against the stock
bloodgulch.map in the dropzone; offsets are constants below):

  * The scenario's `structure_bsps` reflexive points to a ScenarioBSP reference
    holding the BSP chunk's file offset / size / load magic. The chunk opens
    with a header whose first dword points at the sbsp meta (in BSP address
    space; an 'sbsp' signature sits a few dwords in).
  * sbsp meta: world_bounds_x/y/z at 0xC8/0xD0/0xD8 (two floats each); the
    `surfaces` reflexive (render triangles, 3×u16 each) at 0xF8; the `lightmaps`
    reflexive at 0x104.
  * lightmap element = 0x20 bytes, its `materials` reflexive at 0x14.
  * material element = 0x100 bytes: shader dependency at 0x00, first-surface
    index (s32) at 0x14, surface_count (s32) at 0x18, rendered-vertex count
    (u32) at 0xB4. A material's triangle indices are LOCAL to its own vertex
    buffer.
  * Xbox stores the rendered vertices as 32-byte records (position = the leading
    3 floats) whose buffers are laid out contiguously in BSP order — but the
    material's vertex POINTER is a D3D/AGP offset, not a file offset. So we
    locate each material's vertex block by walking forward and matching its
    known vertex count to a run of in-world-bounds positions (every accepted
    vertex is validated against world_bounds).

ROBUSTNESS. The well-known anchors are constants; the one thing that can't be
read as a plain file pointer (the Xbox vertex buffer location) is discovered by
the bounds-validated forward scan. A wrong sbsp-meta offset yields implausible
world_bounds and is rejected, so the extractor fails loudly rather than emitting
garbage; the frontend then degrades to the auto-fit world-bounds box.

LEGAL: the decoded mesh is game-derived geometry. Like the map-preview PNGs and
the HUD icons, it is a LOCAL, git-ignored cache regenerated from the user's own
legally-owned game files — never committed.

Usage:
  python3 scripts/game-geometry/extract_bsp.py \
      [--game haloce] \
      [--mapper-dir ../halo-offset-mapper/scripts/mapmanifest] \
      [--maps-dir "../halo-offset-mapper/xbe-dropzone/Halo CE/maps"] \
      [--map bloodgulch.map] \
      [--out sveltekit/static/game-geometry] \
      [--verbose]
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
GENERATOR = "xemu-cartographer/game-geometry"

# --- verified Halo 1 (Xbox) structure_bsp offsets ---------------------------
REFLEXIVE_SIZE = 0x0C  # count(u32) + pointer(u32) + 4 runtime-zero bytes

SBSP_WORLD_BOUNDS_X = 0xC8  # (min f32, max f32); y at +8, z at +0x10
SBSP_SURFACES_REFLEXIVE = 0xF8  # render triangles, 3×u16 each
SBSP_LIGHTMAPS_REFLEXIVE = 0x104

LIGHTMAP_SIZE = 0x20
LM_MATERIALS_REFLEXIVE = 0x14

MATERIAL_SIZE = 0x100
MAT_SURFACES = 0x14  # s32 first-surface index (local to this material)
MAT_SURFACE_COUNT = 0x18  # s32
MAT_VERTEX_COUNT = 0xB4  # u32 rendered-vertex count

SURFACE_SIZE = 6  # 3 × u16 vertex indices
VERTEX_STRIDE = 32  # Xbox compressed BSP vertex; position = leading 3 floats
BOUNDS_SLACK = 3.0  # world-bounds tolerance when validating vertices

# 'sbsp' in both byte orders (Halo stores dependency fourccs byte-reversed).
_SBSP_BYTES = (b"sbsp", b"psbs")


def slugify(s: str) -> str:
    """Match the frontend's mesh-key derivation (game-geometry.ts)."""
    return re.sub(r"[^a-z0-9]+", "_", s.strip().lower()).strip("_")


def _f32(buf, off):
    return struct.unpack_from("<f", buf, off)[0]


def _u32(buf, off):
    return struct.unpack_from("<I", buf, off)[0]


def _s32(buf, off):
    return struct.unpack_from("<i", buf, off)[0]


def _finite(*vals):
    return all(v == v and abs(v) < 1e9 for v in vals)


class BspAddr:
    """Translator for a BSP chunk's private address space: file offset `start`
    maps to virtual address `magic`."""

    def __init__(self, start, size, magic):
        self.start = start
        self.size = size
        self.magic = magic

    def off(self, vaddr):
        return vaddr - self.magic + self.start

    def valid(self, vaddr, n=1):
        o = self.off(vaddr)
        return self.start <= o and o + n <= self.start + self.size


def _has_sbsp(window):
    return any(sig in window for sig in _SBSP_BYTES)


def _looks_like_bsp_ref(m, o):
    """Does the ScenarioBSP reference element at file offset `o` frame a real BSP
    chunk (file offset / size / magic + an 'sbsp' dependency)?"""
    filesize = len(m.image)
    if o < 0 or o + 0x30 > filesize:
        return None
    start = _u32(m.image, o)
    size = _u32(m.image, o + 4)
    magic = _u32(m.image, o + 8)
    if not (0x800 <= start < filesize):
        return None
    if not (0 < size <= filesize - start):
        return None
    if magic < 0x1000:
        return None
    if not _has_sbsp(m.image[o : o + 0x40]):
        return None
    return BspAddr(start, size, magic)


def find_bsp_reference(m, scnr, log):
    """Locate the structure BSP chunk via the scenario's structure_bsps reflexive
    (its target's first element frames the chunk); falls back to an inline scan."""
    base = m.off(scnr.meta_vaddr)
    filesize = len(m.image)
    for p in range(base, base + 0x600, 4):
        if p + 8 > filesize:
            break
        count = _u32(m.image, p)
        ptr = _u32(m.image, p + 4)
        if not (1 <= count <= 16) or not m.in_image(ptr):
            continue
        ba = _looks_like_bsp_ref(m, m.off(ptr))
        if ba:
            log(f"  bsp ref via reflexive @ {p - base:#x} (count={count}): "
                f"start={ba.start:#x} size={ba.size:#x} magic={ba.magic:#x}")
            return ba
    for p in range(base, base + 0x600, 4):
        ba = _looks_like_bsp_ref(m, p)
        if ba:
            log(f"  bsp ref inline @ {p - base:#x}")
            return ba
    return None


def sbsp_meta_candidates(m, ba, log):
    """Candidate file offsets for the sbsp meta. The chunk header's first dword
    points at the meta (in BSP space); also offer fixed header sizes. The caller
    keeps the first whose world_bounds validate."""
    cands = []
    head_ptr = _u32(m.image, ba.start)
    if ba.valid(head_ptr):
        cands.append(ba.off(head_ptr))
    for hdr in (0x10, 0x14, 0x18, 0x0):
        cands.append(ba.start + hdr)
    seen, out = set(), []
    for c in cands:
        if c not in seen and ba.start <= c < ba.start + ba.size:
            seen.add(c)
            out.append(c)
    return out


def read_world_bounds(img, meta_off):
    o = meta_off + SBSP_WORLD_BOUNDS_X
    return (
        (_f32(img, o), _f32(img, o + 4)),
        (_f32(img, o + 8), _f32(img, o + 12)),
        (_f32(img, o + 16), _f32(img, o + 20)),
    )


def bounds_plausible(b):
    (xmin, xmax), (ymin, ymax), (zmin, zmax) = b
    if not _finite(xmin, xmax, ymin, ymax, zmin, zmax):
        return False
    if not (xmax > xmin and ymax > ymin and zmax >= zmin):
        return False
    # A real level spans more than a hair and less than the engine's max.
    span = max(xmax - xmin, ymax - ymin)
    return 1.0 < span < 100000.0


def read_surfaces(img, ba, meta_off, log):
    count = _u32(img, meta_off + SBSP_SURFACES_REFLEXIVE)
    ptr = _u32(img, meta_off + SBSP_SURFACES_REFLEXIVE + 4)
    if count <= 0 or count > 5_000_000 or not ba.valid(ptr, count * SURFACE_SIZE):
        log(f"  surfaces reflexive implausible: count={count} ptr={ptr:#x}")
        return []
    o = ba.off(ptr)
    return [struct.unpack_from("<HHH", img, o + i * SURFACE_SIZE) for i in range(count)]


def find_vertex_block(img, ba, cursor, count, bounds):
    """Find the next material's 32-byte-stride vertex block: the first 4-byte-
    aligned offset at/after `cursor` where `count` consecutive positions all fall
    inside world_bounds. Returns the file offset, or None."""
    (xmin, xmax), (ymin, ymax), (zmin, zmax) = bounds
    sl = BOUNDS_SLACK
    xlo, xhi = xmin - sl, xmax + sl
    ylo, yhi = ymin - sl, ymax + sl
    zlo, zhi = zmin - sl, zmax + sl
    end = ba.start + ba.size
    o = (cursor + 3) & ~3
    span = count * VERTEX_STRIDE
    last = count - 1
    while o + span <= end:
        # Cheap reject: first / mid / last must be in-bounds before the full scan.
        ok = True
        for k in (0, last >> 1, last):
            p = o + k * VERTEX_STRIDE
            x = _f32(img, p)
            if not (xlo <= x <= xhi):
                ok = False
                break
            y = _f32(img, p + 4)
            z = _f32(img, p + 8)
            if not (ylo <= y <= yhi and zlo <= z <= zhi):
                ok = False
                break
        if ok:
            for k in range(count):
                p = o + k * VERTEX_STRIDE
                x = _f32(img, p)
                y = _f32(img, p + 4)
                z = _f32(img, p + 8)
                if not (xlo <= x <= xhi and ylo <= y <= yhi and zlo <= z <= zhi):
                    ok = False
                    break
            if ok:
                return o
        o += 4
    return None


def extract_mesh(m, ba, meta_off, log):
    img = m.image
    bounds = read_world_bounds(img, meta_off)
    log(f"  world_bounds x={bounds[0]} y={bounds[1]} z={bounds[2]}")
    if not bounds_plausible(bounds):
        log("  world_bounds implausible — sbsp meta offset likely wrong")
        return None

    surfaces = read_surfaces(img, ba, meta_off, log)
    if not surfaces:
        return None
    log(f"  surfaces: {len(surfaces)}")

    lm_count = _u32(img, meta_off + SBSP_LIGHTMAPS_REFLEXIVE)
    lm_ptr = _u32(img, meta_off + SBSP_LIGHTMAPS_REFLEXIVE + 4)
    if lm_count <= 0 or lm_count > 4096 or not ba.valid(lm_ptr, lm_count * LIGHTMAP_SIZE):
        log(f"  lightmaps reflexive implausible: count={lm_count} ptr={lm_ptr:#x}")
        return None
    lm_base = ba.off(lm_ptr)
    log(f"  lightmaps: {lm_count}")

    positions = []
    indices = []
    cursor = ba.start
    materials = 0
    skipped = 0
    dropped_tris = 0

    for li in range(lm_count):
        lm_off = lm_base + li * LIGHTMAP_SIZE
        mat_count = _u32(img, lm_off + LM_MATERIALS_REFLEXIVE)
        mat_ptr = _u32(img, lm_off + LM_MATERIALS_REFLEXIVE + 4)
        if mat_count <= 0 or mat_count > 4096 or not ba.valid(mat_ptr):
            continue
        mats_off = ba.off(mat_ptr)
        for mi in range(mat_count):
            mat_off = mats_off + mi * MATERIAL_SIZE
            vc = _u32(img, mat_off + MAT_VERTEX_COUNT)
            s0 = _s32(img, mat_off + MAT_SURFACES)
            sc = _s32(img, mat_off + MAT_SURFACE_COUNT)
            materials += 1
            if vc <= 0 or vc > 200000 or sc <= 0 or s0 < 0 or s0 + sc > len(surfaces):
                skipped += 1
                continue
            block = find_vertex_block(img, ba, cursor, vc, bounds)
            if block is None:
                skipped += 1
                continue
            base = len(positions)
            for k in range(vc):
                p = block + k * VERTEX_STRIDE
                positions.append((_f32(img, p), _f32(img, p + 4), _f32(img, p + 8)))
            for a, b, c in surfaces[s0 : s0 + sc]:
                if a < vc and b < vc and c < vc:
                    indices.append((base + a, base + b, base + c))
                else:
                    dropped_tris += 1
            cursor = block + vc * VERTEX_STRIDE

    if not positions or not indices:
        log("  no geometry accumulated")
        return None

    xs = [p[0] for p in positions]
    ys = [p[1] for p in positions]
    zs = [p[2] for p in positions]
    log(f"  materials={materials} skipped={skipped} dropped_tris={dropped_tris}")
    log(f"  mesh: {len(positions)} verts, {len(indices)} tris, "
        f"bounds x[{min(xs):.1f},{max(xs):.1f}] y[{min(ys):.1f},{max(ys):.1f}] z[{min(zs):.1f},{max(zs):.1f}]")

    flat_pos = []
    for x, y, z in positions:
        flat_pos.extend((round(x, 4), round(y, 4), round(z, 4)))
    flat_idx = []
    for tri in indices:
        flat_idx.extend(tri)
    return {
        "positions": flat_pos,
        "indices": flat_idx,
        "bounds": {"minX": min(xs), "maxX": max(xs), "minY": min(ys),
                   "maxY": max(ys), "minZ": min(zs), "maxZ": max(zs)},
        "vertex_count": len(positions),
        "triangle_count": len(indices),
    }


def extract(game, mapper_dir, maps_dir, map_file, out_root, verbose):
    sys.path.insert(0, mapper_dir)
    import halomap as H  # noqa: E402 (reused cache parser — see module docstring)

    def log(msg):
        if verbose:
            print(msg, file=sys.stderr)

    map_path = os.path.join(maps_dir, map_file)
    if not os.path.isfile(map_path):
        raise SystemExit(f"map not found: {map_path}\n(set --maps-dir to your Halo CE maps dir)")
    m = H.parse_map(map_path)
    log(f"map {map_file}: name={m.name!r} type={m.map_type_name} vbase={m.vbase:#x}")

    scnr = next((t for t in m.tags if t.group == "scnr"), None)
    if scnr is None:
        raise SystemExit(f"{map_file}: no scenario (scnr) tag")
    scenario_path = scnr.path
    key = slugify(scenario_path.replace("/", "\\").split("\\")[-1]) or slugify(m.name)
    log(f"scenario {scenario_path!r} -> key {key!r}")

    ba = find_bsp_reference(m, scnr, log)
    if ba is None:
        raise SystemExit(f"{map_file}: could not locate a structure-BSP reference")

    mesh = None
    for meta_off in sbsp_meta_candidates(m, ba, log):
        log(f"  trying sbsp meta @ file {meta_off:#x}")
        mesh = extract_mesh(m, ba, meta_off, log)
        if mesh is not None:
            break
    if mesh is None:
        raise SystemExit(f"{map_file}: BSP geometry extraction failed (run with --verbose)")

    out_dir = os.path.join(out_root, game)
    os.makedirs(out_dir, exist_ok=True)
    mesh_file = f"{key}.json"
    payload = {
        "schema_version": SCHEMA_VERSION,
        "generated_by": GENERATOR,
        "generated_at": datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds"),
        "game": game,
        "scenario": scenario_path,
        "source_map": map_file,
        "bounds": mesh["bounds"],
        "vertex_count": mesh["vertex_count"],
        "triangle_count": mesh["triangle_count"],
        "positions": mesh["positions"],
        "indices": mesh["indices"],
    }
    with open(os.path.join(out_dir, mesh_file), "w") as fh:
        json.dump(payload, fh, separators=(",", ":"))

    man_path = os.path.join(out_dir, "manifest.json")
    manifest = {"schema_version": SCHEMA_VERSION, "generated_by": GENERATOR, "game": game, "meshes": {}}
    if os.path.isfile(man_path):
        try:
            with open(man_path) as fh:
                manifest = json.load(fh)
                manifest.setdefault("meshes", {})
        except (OSError, ValueError):
            pass
    manifest["meshes"][key] = {
        "file": mesh_file,
        "scenario": scenario_path,
        "source_map": map_file,
        "vertex_count": mesh["vertex_count"],
        "triangle_count": mesh["triangle_count"],
        "bounds": mesh["bounds"],
    }
    manifest["generated_at"] = payload["generated_at"]
    with open(man_path, "w") as fh:
        json.dump(manifest, fh, indent=2)

    return key, payload, out_dir


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    repo_root = os.path.abspath(os.path.join(here, "..", ".."))
    default_mapper = os.path.join(repo_root, "..", "halo-offset-mapper", "scripts", "mapmanifest")
    default_maps = os.environ.get("HALO_CE_MAPS_DIR") or os.path.join(
        repo_root, "..", "halo-offset-mapper", "xbe-dropzone", "Halo CE", "maps")
    default_out = os.path.join(repo_root, "sveltekit", "static", "game-geometry")

    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--game", default="haloce")
    ap.add_argument("--mapper-dir", default=default_mapper, help="halo-offset-mapper mapmanifest dir (parser reuse)")
    ap.add_argument("--maps-dir", default=default_maps, help="directory of unpacked stock .map files")
    ap.add_argument("--map", default="bloodgulch.map", help="specific .map file to read")
    ap.add_argument("--out", default=default_out, help="served geometry cache root (per-game subdir created)")
    ap.add_argument("--verbose", action="store_true", help="log the BSP walk + validation to stderr")
    args = ap.parse_args()

    if not os.path.isdir(args.mapper_dir):
        raise SystemExit(f"mapper-dir not found: {args.mapper_dir}\n"
                         f"(clone halo-offset-mapper beside xemu-cartographer, or pass --mapper-dir)")

    key, payload, out_dir = extract(args.game, args.mapper_dir, args.maps_dir, args.map, args.out, args.verbose)
    print(json.dumps({
        "game": args.game, "scenario": payload["scenario"], "source_map": payload["source_map"],
        "key": key, "vertex_count": payload["vertex_count"], "triangle_count": payload["triangle_count"],
        "bounds": payload["bounds"],
    }, indent=2))
    print(f"  -> {os.path.join(out_dir, key)}.json")


if __name__ == "__main__":
    main()
