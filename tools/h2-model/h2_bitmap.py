#!/usr/bin/env python3
"""Native H2 **Xbox** bitmap decoder — pulls a `bitm` tag's top mip out of the
cache and decodes it to RGBA PNG. Linux-only; no Windows tooling.

Handles the formats the Mark VI uses: DXT1/DXT3/DXT5 (block compressed),
A8R8G8B8/X8R8G8B8, the 16-bpp set, A8/Y8/A8Y8, and P8_bump (Halo-2 bump palette).
Xbox-swizzled textures (BitmapFlags & 8) are de-swizzled first.

Reference: Reclaimer Reclaimer.Blam/Blam/Halo2/bitmap.cs (BitmapDataBlock layout,
format enum, swizzle handling). Pixel pools resolved with the same top-2-bits
"raw pointer" scheme as the emblems / the model geometry.
"""
import argparse, os, struct
import numpy as np
from PIL import Image
from h2_render_model import Cache

FMT = {0: "A8", 1: "Y8", 2: "AY8", 3: "A8Y8", 6: "R5G6B5", 8: "A1R5G5B5",
       9: "A4R4G4B4", 10: "X8R8G8B8", 11: "A8R8G8B8", 14: "DXT1", 15: "DXT3",
       16: "DXT5", 17: "P8_bump", 18: "P8", 22: "U8V8", 23: "G8B8"}

BPP = {0: 8, 1: 8, 2: 8, 3: 16, 6: 16, 8: 16, 9: 16, 10: 32, 11: 32,
       14: 4, 15: 8, 16: 8, 17: 8, 18: 8, 22: 16, 23: 16}  # bits per pixel (read unit)


def _part1by1(n):
    n &= 0xFFFF
    n = (n | (n << 8)) & 0x00FF00FF
    n = (n | (n << 4)) & 0x0F0F0F0F
    n = (n | (n << 2)) & 0x33333333
    n = (n | (n << 1)) & 0x55555555
    return n


def unswizzle(data, w, h, unit):
    """Undo Xbox Morton swizzle. `unit` = bytes per element (1,2,4)."""
    out = bytearray(len(data))
    for y in range(h):
        by = _part1by1(y) << 1
        row = y * w
        for x in range(w):
            m = by | _part1by1(x)
            out[(row + x) * unit:(row + x) * unit + unit] = data[m * unit:m * unit + unit]
    return bytes(out)


def _rgb565(c):
    r = (c >> 11) & 0x1F; g = (c >> 5) & 0x3F; b = c & 0x1F
    return (r << 3) | (r >> 2), (g << 2) | (g >> 4), (b << 3) | (b >> 2)


def decode_dxt(data, w, h, fmt):
    """DXT1(14)/DXT3(15)/DXT5(16) -> RGBA ndarray (h,w,4)."""
    img = np.zeros((h, w, 4), np.uint8)
    img[..., 3] = 255
    bw, bh = (w + 3) // 4, (h + 3) // 4
    block = 8 if fmt == 14 else 16
    pos = 0
    for by in range(bh):
        for bx in range(bw):
            blk = data[pos:pos + block]; pos += block
            alpha = None
            if fmt == 15:  # DXT3 explicit 4-bit alpha
                a = int.from_bytes(blk[0:8], "little")
                alpha = [((a >> (4 * i)) & 0xF) * 17 for i in range(16)]
                cblk = blk[8:]
            elif fmt == 16:  # DXT5 interpolated alpha
                a0, a1 = blk[0], blk[1]
                abits = int.from_bytes(blk[2:8], "little")
                al = [a0, a1]
                if a0 > a1:
                    al += [((6 - i) * a0 + (1 + i) * a1) // 7 for i in range(6)]
                else:
                    al += [((4 - i) * a0 + (1 + i) * a1) // 5 for i in range(4)] + [0, 255]
                alpha = [al[(abits >> (3 * i)) & 7] for i in range(16)]
                cblk = blk[8:]
            else:
                cblk = blk
            c0, c1 = struct.unpack_from("<HH", cblk, 0)
            bits = struct.unpack_from("<I", cblk, 4)[0]
            col = [_rgb565(c0), _rgb565(c1)]
            if fmt == 14 and c0 <= c1:
                col.append(tuple((col[0][k] + col[1][k]) // 2 for k in range(3)))
                col.append((0, 0, 0))  # transparent in DXT1 1-bit alpha mode
                onebit = True
            else:
                col.append(tuple((2 * col[0][k] + col[1][k]) // 3 for k in range(3)))
                col.append(tuple((col[0][k] + 2 * col[1][k]) // 3 for k in range(3)))
                onebit = False
            for i in range(16):
                px, py = bx * 4 + (i % 4), by * 4 + (i // 4)
                if px >= w or py >= h:
                    continue
                ci = (bits >> (2 * i)) & 3
                r, g, b = col[ci]
                img[py, px, 0:3] = (r, g, b)
                if alpha is not None:
                    img[py, px, 3] = alpha[i]
                elif onebit and ci == 3:
                    img[py, px, 3] = 0
    return img


def decode_16(data, w, h, fmt):
    out = np.zeros((h, w, 4), np.uint8)
    v = np.frombuffer(data[:w * h * 2], "<u2").reshape(h, w).astype(np.uint32)
    if fmt == 9:    # a4r4g4b4
        out[..., 3] = ((v >> 12) & 0xF) * 17; out[..., 0] = ((v >> 8) & 0xF) * 17
        out[..., 1] = ((v >> 4) & 0xF) * 17;  out[..., 2] = (v & 0xF) * 17
    elif fmt == 8:  # a1r5g5b5
        out[..., 3] = np.where((v >> 15) & 1, 255, 0)
        out[..., 0] = (((v >> 10) & 0x1F) * 527 + 23) >> 6
        out[..., 1] = (((v >> 5) & 0x1F) * 527 + 23) >> 6
        out[..., 2] = ((v & 0x1F) * 527 + 23) >> 6
    elif fmt == 6:  # r5g6b5
        out[..., 3] = 255
        out[..., 0] = (((v >> 11) & 0x1F) * 527 + 23) >> 6
        out[..., 1] = (((v >> 5) & 0x3F) * 259 + 33) >> 6
        out[..., 2] = ((v & 0x1F) * 527 + 23) >> 6
    return out


def decode_bitmap(cache: Cache, name, want_mip0_only=True):
    hit = None
    for i, (cid, tid, mp, ms) in enumerate(cache.items):
        if cid == "bitm" and cache.tag_name(i).split("\\")[-1] == name:
            hit = (i, mp); break
    if not hit:
        raise SystemExit(f"bitm '{name}' not found")
    i, mp = hit
    meta = cache.meta_file(mp)
    n_bmp, bmp_off = cache.tag_block(meta + 68)
    o = bmp_off
    i16 = lambda x: struct.unpack_from("<h", cache.b, x)[0]
    W = i16(o + 4); H = i16(o + 6); bf = i16(o + 12); fl = i16(o + 14)
    lod0 = cache.u32(o + 28); lod0sz = cache.i32(o + 52)
    loc = (lod0 >> 30) & 3; addr = lod0 & 0x3FFFFFFF
    pool = cache.pools[loc]
    raw = pool[addr: addr + lod0sz]

    bpp = BPP[bf]
    mip0 = W * H * bpp // 8
    data = raw[:mip0] if want_mip0_only else raw

    if fl & 8:  # swizzled
        unit = max(1, bpp // 8)
        data = unswizzle(data, W, H, unit)

    if bf in (14, 15, 16):
        arr = decode_dxt(data, W, H, bf)
    elif bf in (6, 8, 9):
        arr = decode_16(data, W, H, bf)
    elif bf in (10, 11):  # x8r8g8b8 / a8r8g8b8  (BGRA byte order)
        px = np.frombuffer(data[:W * H * 4], np.uint8).reshape(H, W, 4)
        arr = np.zeros((H, W, 4), np.uint8)
        arr[..., 0] = px[..., 2]; arr[..., 1] = px[..., 1]; arr[..., 2] = px[..., 0]
        arr[..., 3] = 255 if bf == 10 else px[..., 3]
    elif bf == 0:   # A8
        a = np.frombuffer(data[:W * H], np.uint8).reshape(H, W)
        arr = np.zeros((H, W, 4), np.uint8); arr[..., 3] = a; arr[..., 0:3] = 255
    elif bf in (1,):  # Y8 luminance
        y = np.frombuffer(data[:W * H], np.uint8).reshape(H, W)
        arr = np.zeros((H, W, 4), np.uint8); arr[..., 3] = 255
        for k in range(3): arr[..., k] = y
    elif bf == 3:   # A8Y8
        d = np.frombuffer(data[:W * H * 2], np.uint8).reshape(H, W, 2)
        arr = np.zeros((H, W, 4), np.uint8)
        for k in range(3): arr[..., k] = d[..., 0]
        arr[..., 3] = d[..., 1]
    else:
        raise SystemExit(f"format {FMT.get(bf, bf)} not implemented for {name}")
    return Image.fromarray(arr, "RGBA"), {"w": W, "h": H, "fmt": FMT.get(bf, bf), "flags": fl}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--maps", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--names", nargs="+",
                    default=["masterchief", "masterchief_cc", "masterchief_bump"])
    a = ap.parse_args()
    cache = Cache(os.path.join(a.maps, "shared.map"), os.path.join(a.maps, "mainmenu.map"))
    os.makedirs(a.out, exist_ok=True)
    for nm in a.names:
        img, info = decode_bitmap(cache, nm)
        p = os.path.join(a.out, f"{nm}.png")
        img.save(p)
        print(f"{nm}: {info['w']}x{info['h']} {info['fmt']} flags={info['flags']} -> {p}")


if __name__ == "__main__":
    main()
