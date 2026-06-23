#!/usr/bin/env python3
"""
Extract the Halo structure-BSP RENDERED GEOMETRY (vertices + triangles + per-
material color) from a stock `.map` cache file into the visualizer's served
geometry cache: a JSON mesh (for the 3D `/visualizer3d`) AND a top-down PNG
projection (background for the 2D minimap) + a manifest. ONE shared extraction
feeds both surfaces.

Reuses halo-offset-mapper's `.map` cache parser (`halomap.py`) to resolve the
tag space and its swizzle-aware bitmap decoder (`bitmaps.py`) to sample texture
color, then walks scenario -> structure-BSP -> lightmaps -> materials,
accumulating each material's vertex buffer + triangle list into one mesh in Halo
world coordinates (X/Y ground, Z up — the SAME frame the live marker positions
use, so the frontend maps both through one transform and they line up).

COLOR (per Stewart's tier: real-texture-sampled > flat). Each BSP material
references a shader -> base/diffuse bitmap; we decode that bitmap and average its
RGB so grass reads grass-green, sand tan, metal gray. That per-material color is
emitted as per-vertex colors on the 3D mesh AND painted into the 2D top-down
PNG. (Full UV texturing is the documented stretch goal; missing/undecodable
textures fall back to a neutral material color.)

HALO 1 (Xbox) STRUCTURE-BSP LAYOUT (verified empirically against stock
bloodgulch.map): ScenarioBSP reference (file offset/size/magic) via the
scenario's structure_bsps reflexive; BSP chunk header dword[0] -> sbsp meta;
sbsp meta world_bounds_x/y/z @ 0xC8/0xD0/0xD8, surfaces reflexive (3xu16 tris) @
0xF8, lightmaps reflexive @ 0x104; lightmap = 0x20 bytes, materials reflexive @
0x14; material = 0x100 bytes, shader dependency @ 0x00 (tag_id @ +0x0C),
surfaces(s32) @ 0x14, surface_count(s32) @ 0x18, rendered-vertex count(u32) @
0xB4; surface indices are material-local. Xbox rendered vertices are 32-byte
records (position = leading 3 floats) whose buffers sit contiguously in BSP
order but are addressed by D3D offsets, so each material's block is located by a
world_bounds-validated forward scan.

LEGAL: the decoded mesh + PNG are game-derived assets — a LOCAL, git-ignored
cache regenerated from the user's own legally-owned game files, never committed.

Usage:
  python3 scripts/game-geometry/extract_bsp.py \
      [--game haloce] [--mapper-dir ...] [--maps-dir ...] \
      [--map bloodgulch.map] [--out sveltekit/static/game-geometry] [--verbose]
"""

from __future__ import annotations

import argparse
import datetime
import json
import os
import re
import struct
import sys

SCHEMA_VERSION = 2  # bumped: adds per-vertex colors + top-down PNG
GENERATOR = "xemu-cartographer/game-geometry"

# --- verified Halo 1 (Xbox) structure_bsp offsets ---------------------------
SBSP_WORLD_BOUNDS_X = 0xC8  # (min f32, max f32); y at +8, z at +0x10
SBSP_SURFACES_REFLEXIVE = 0xF8  # render triangles, 3xu16 each
SBSP_LIGHTMAPS_REFLEXIVE = 0x104

LIGHTMAP_SIZE = 0x20
LM_MATERIALS_REFLEXIVE = 0x14

MATERIAL_SIZE = 0x100
MAT_SHADER_TAG_ID = 0x0C  # tag_id within the shader TagDependency at material+0x00
MAT_SURFACES = 0x14  # s32 first-surface index (local to this material)
MAT_SURFACE_COUNT = 0x18  # s32
MAT_VERTEX_COUNT = 0xB4  # u32 rendered-vertex count

SURFACE_SIZE = 6  # 3 x u16 vertex indices
VERTEX_STRIDE = 32  # Xbox compressed BSP vertex; position = leading 3 floats
BOUNDS_SLACK = 3.0

NEUTRAL_COLOR = (0.5, 0.5, 0.52)  # fallback when a texture can't be sampled

# 'sbsp' in both byte orders (Halo stores dependency fourccs byte-reversed).
_SBSP_BYTES = (b"sbsp", b"psbs")


def slugify(s: str) -> str:
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


# --- per-material color sampled from the real map textures ------------------
def build_tag_by_id(m):
    return {t.tag_id: t for t in m.tags}


def _avg_bitmap_color(m, B, bitmap_tag):
    import numpy as np

    bds = B.all_bitmap_data(m, bitmap_tag)
    if not bds:
        return None
    img = B.decode_to_image(m.image, bds[0]).convert("RGBA")
    arr = np.asarray(img, dtype=np.float32)
    if arr.size == 0:
        return None
    rgb = arr[..., :3].reshape(-1, 3)
    alpha = arr[..., 3].reshape(-1)
    mask = alpha > 16
    px = rgb[mask] if int(mask.sum()) >= 8 else rgb
    mean = px.mean(axis=0) / 255.0
    return (float(mean[0]), float(mean[1]), float(mean[2]))


def _first_bitmap_of_shader(m, shader_tag, tag_by_id):
    """The base/diffuse map is the first bitmap dependency in the shader meta —
    works across shader types (senv/soso/schi/...) without per-type offsets."""
    if not m.in_image(shader_tag.meta_vaddr):
        return None
    o = m.off(shader_tag.meta_vaddr)
    end = min(len(m.image), o + 0x400)
    p = o
    while p + 16 <= end:
        if m.image[p : p + 4] == b"mtib":  # 'bitm' reversed (dependency tag class)
            tid = _u32(m.image, p + 0x0C)
            t = tag_by_id.get(tid)
            if t is not None and t.group == "bitm":
                return t
        p += 4
    return None


def material_color(m, B, mat_off, tag_by_id, cache, log):
    shader_tid = _u32(m.image, mat_off + MAT_SHADER_TAG_ID)
    if shader_tid in cache:
        return cache[shader_tid]
    col = None
    sh = tag_by_id.get(shader_tid)
    if sh is not None:
        bt = _first_bitmap_of_shader(m, sh, tag_by_id)
        if bt is not None:
            try:
                col = _avg_bitmap_color(m, B, bt)
            except Exception as e:  # noqa: BLE001 — texture decode is best-effort
                log(f"    color decode failed ({sh.path!r}): {e}")
    if col is None:
        col = NEUTRAL_COLOR
    cache[shader_tid] = col
    return col


def rasterize_top(positions, indices, colors, bounds, out_path, px_per_unit=9.0):
    """Top-down 2D projection PNG: triangles filled with their material color,
    painted low-Z first so higher terrain occludes lower (elevation reads). World
    X/Y -> pixels over the mesh bounds (Y flipped = north up), transparent
    background. Aligns to live coordinates because the frontend places it by
    projecting these SAME bounds through the live map projection."""
    from PIL import Image, ImageDraw

    minx, maxx = bounds["minX"], bounds["maxX"]
    miny, maxy = bounds["minY"], bounds["maxY"]
    spanx = max(1e-3, maxx - minx)
    spany = max(1e-3, maxy - miny)
    W = max(64, min(2048, int(round(spanx * px_per_unit))))
    H = max(64, min(2048, int(round(spany * px_per_unit))))
    img = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    dr = ImageDraw.Draw(img, "RGBA")
    tris = []
    for i in range(0, len(indices), 3):
        a, b, c = indices[i], indices[i + 1], indices[i + 2]
        zavg = (positions[a * 3 + 2] + positions[b * 3 + 2] + positions[c * 3 + 2]) / 3.0
        tris.append((zavg, a, b, c))
    tris.sort(key=lambda t: t[0])
    for _, a, b, c in tris:
        pts = []
        for vi in (a, b, c):
            x = positions[vi * 3]
            y = positions[vi * 3 + 1]
            pts.append(((x - minx) / spanx * W, (maxy - y) / spany * H))
        r = int(max(0.0, min(1.0, colors[a * 3])) * 255)
        g = int(max(0.0, min(1.0, colors[a * 3 + 1])) * 255)
        bl = int(max(0.0, min(1.0, colors[a * 3 + 2])) * 255)
        dr.polygon(pts, fill=(r, g, bl, 255))
    img.save(out_path)
    return W, H


def extract_mesh(m, B, ba, meta_off, tag_by_id, log):
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
    colors = []
    indices = []
    color_cache = {}
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
            col = material_color(m, B, mat_off, tag_by_id, color_cache, log)
            base = len(positions)
            for k in range(vc):
                p = block + k * VERTEX_STRIDE
                positions.append((_f32(img, p), _f32(img, p + 4), _f32(img, p + 8)))
                colors.append(col)
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
    sampled = sum(1 for v in color_cache.values() if v != NEUTRAL_COLOR)
    log(f"  materials={materials} skipped={skipped} dropped_tris={dropped_tris} "
        f"shaders={len(color_cache)} texture-sampled={sampled}")
    log(f"  mesh: {len(positions)} verts, {len(indices)} tris, "
        f"bounds x[{min(xs):.1f},{max(xs):.1f}] y[{min(ys):.1f},{max(ys):.1f}] z[{min(zs):.1f},{max(zs):.1f}]")

    flat_pos = []
    for x, y, z in positions:
        flat_pos.extend((round(x, 4), round(y, 4), round(z, 4)))
    flat_col = []
    for r, g, b in colors:
        flat_col.extend((round(r, 4), round(g, 4), round(b, 4)))
    flat_idx = []
    for tri in indices:
        flat_idx.extend(tri)
    return {
        "positions": flat_pos,
        "colors": flat_col,
        "indices": flat_idx,
        "bounds": {"minX": min(xs), "maxX": max(xs), "minY": min(ys),
                   "maxY": max(ys), "minZ": min(zs), "maxZ": max(zs)},
        "vertex_count": len(positions),
        "triangle_count": len(indices),
    }


def extract(game, mapper_dir, maps_dir, map_file, out_root, verbose):
    sys.path.insert(0, mapper_dir)
    import halomap as H  # noqa: E402 (reused cache parser — see module docstring)
    import bitmaps as B  # noqa: E402 (reused swizzle-aware bitmap decoder)

    def log(msg):
        if verbose:
            print(msg, file=sys.stderr)

    map_path = os.path.join(maps_dir, map_file)
    if not os.path.isfile(map_path):
        raise SystemExit(f"map not found: {map_path}\n(set --maps-dir to your Halo CE maps dir)")
    m = H.parse_map(map_path)
    tag_by_id = build_tag_by_id(m)
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
        mesh = extract_mesh(m, B, ba, meta_off, tag_by_id, log)
        if mesh is not None:
            break
    if mesh is None:
        raise SystemExit(f"{map_file}: BSP geometry extraction failed (run with --verbose)")

    out_dir = os.path.join(out_root, game)
    os.makedirs(out_dir, exist_ok=True)

    # 2D top-down projection PNG (minimap background).
    top_file = f"{key}_top.png"
    try:
        W, Hh = rasterize_top(mesh["positions"], mesh["indices"], mesh["colors"],
                              mesh["bounds"], os.path.join(out_dir, top_file))
        log(f"  top projection: {W}x{Hh} -> {top_file}")
    except Exception as e:  # noqa: BLE001 — PNG is best-effort
        log(f"  top projection failed: {e}")
        top_file = None

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
        "colors": mesh["colors"],
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
        "top_image": top_file,
        "scenario": scenario_path,
        "source_map": map_file,
        "vertex_count": mesh["vertex_count"],
        "triangle_count": mesh["triangle_count"],
        "bounds": mesh["bounds"],
    }
    manifest["generated_at"] = payload["generated_at"]
    with open(man_path, "w") as fh:
        json.dump(manifest, fh, indent=2)

    return key, payload, top_file, out_dir


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    repo_root = os.path.abspath(os.path.join(here, "..", ".."))
    default_mapper = os.path.join(repo_root, "..", "halo-offset-mapper", "scripts", "mapmanifest")
    default_maps = os.environ.get("HALO_CE_MAPS_DIR") or os.path.join(
        repo_root, "..", "halo-offset-mapper", "xbe-dropzone", "Halo CE", "maps")
    default_out = os.path.join(repo_root, "sveltekit", "static", "game-geometry")

    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--game", default="haloce")
    ap.add_argument("--mapper-dir", default=default_mapper, help="halo-offset-mapper mapmanifest dir (parser/bitmap reuse)")
    ap.add_argument("--maps-dir", default=default_maps, help="directory of unpacked stock .map files")
    ap.add_argument("--map", default="bloodgulch.map", help="specific .map file to read")
    ap.add_argument("--out", default=default_out, help="served geometry cache root (per-game subdir created)")
    ap.add_argument("--verbose", action="store_true", help="log the BSP walk + validation to stderr")
    args = ap.parse_args()

    if not os.path.isdir(args.mapper_dir):
        raise SystemExit(f"mapper-dir not found: {args.mapper_dir}\n"
                         f"(clone halo-offset-mapper beside xemu-cartographer, or pass --mapper-dir)")

    key, payload, top_file, out_dir = extract(
        args.game, args.mapper_dir, args.maps_dir, args.map, args.out, args.verbose)
    print(json.dumps({
        "game": args.game, "scenario": payload["scenario"], "source_map": payload["source_map"],
        "key": key, "vertex_count": payload["vertex_count"], "triangle_count": payload["triangle_count"],
        "top_image": top_file, "bounds": payload["bounds"],
    }, indent=2))
    print(f"  -> {os.path.join(out_dir, key)}.json  (+ {top_file})")


if __name__ == "__main__":
    main()
