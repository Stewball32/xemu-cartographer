#!/usr/bin/env python3
"""Extract the real Halo 2 multiplayer emblem art (64 foregrounds + 32
backgrounds) from the stock **Xbox** Halo 2 cache maps and emit per-emblem
tint masks for the appearance studio.

Why this approach
-----------------
H2 emblems live in the `ui\\global_bitmaps\\emblems\\{foreground,background}`
`bitm` tags. On the original Xbox build (cache version 8) the tag index +
metadata are plaintext and uncompressed, but:

  * bitmap pixel data is stored in a *separate* pool and referenced by a
    "raw pointer" whose top two bits select the owning map
    ([local, mainmenu, shared, single_player_shared]) and whose low 30 bits
    are the byte offset — which is why a naive grep over a single .map fails;
  * pixels are Xbox-swizzled (Morton order), not zlib-compressed (that is an
    MCC/Vista-era change);
  * Sigmmma's `reclaimer` loads the cache + tag index fine, but its H2 *Xbox*
    bitmap-meta parser mis-reads the body, so we parse the (well-documented,
    fixed-size) `bitm` record ourselves and only borrow `reclaimer` for the
    tag index during development. This script needs neither at runtime.

Encoding (decoded empirically + confirmed against the in-game enum):
  Each emblem texel is a two-tone alpha blend between a PRIMARY marker
  (yellow, R=G channel) and a SECONDARY marker (blue, B channel). So:
      primary coverage  = Red channel
      secondary coverage = Blue channel
  "solid" background = 100% red (all-primary); splits = 50/50; gradients
  interpolate. Foregrounds are transparent (R=B=0) outside the symbol.

Output: <out>/{fg,bg}/<NN>_p.png  (primary coverage mask, grayscale)
        <out>/{fg,bg}/<NN>_s.png  (secondary coverage mask, grayscale)
        <out>/{fg,bg}/<NN>_ref.png (full-colour reference, not shipped)

The studio tints these: background plate = secondary base + primary overlay
(armor colours); foreground symbol = primary + secondary (emblem colours).

Usage:
    python extract_emblems.py --maps DIR --out DIR
where DIR holds the Xbox originals shared.map + mainmenu.map (e.g. pulled
from the disc via xdvdfs).

IP: for personal / LAN use with your own copy of the game. Do not redistribute
the extracted assets.
"""
import argparse, struct, os, sys
from PIL import Image

# bitm sub-record (cache, 116 bytes). Offsets from the start of the record.
W_OFF, H_OFF, TYPE_OFF, FMT_OFF = 4, 6, 0x0A, 0x0C
LOD1_OFF, LOD1_SIZE_OFF, STRIDE = 0x1C, 0x34, 0x74
BPP = {6: 2, 8: 2, 9: 2}                 # r5g6b5, a1r5g5b5, a4r4g4b4
VALID_DIMS = {8, 16, 32, 64, 128, 256, 512, 1024}
MAP_SELECTOR = {0: "local", 1: "mainmenu", 2: "shared", 3: "single_player_shared"}


def u16(b, o): return struct.unpack_from("<H", b, o)[0]
def u32(b, o): return struct.unpack_from("<I", b, o)[0]


def _part1by1(n):
    n &= 0xFFFF
    n = (n | (n << 8)) & 0x00FF00FF
    n = (n | (n << 4)) & 0x0F0F0F0F
    n = (n | (n << 2)) & 0x33333333
    n = (n | (n << 1)) & 0x55555555
    return n


def deswizzle(data, w, h, bpp):
    """Undo Xbox Morton/Z-order swizzle for a power-of-two texture."""
    out = bytearray(len(data))
    for y in range(h):
        base = _part1by1(y) << 1
        for x in range(w):
            m = base | _part1by1(x)
            out[(y * w + x) * bpp:(y * w + x) * bpp + bpp] = data[m * bpp:m * bpp + bpp]
    return out


def decode_16bpp(px, w, h, fmt):
    img = Image.new("RGBA", (w, h))
    p = img.load()
    for i in range(w * h):
        v = px[i * 2] | (px[i * 2 + 1] << 8)
        x, y = i % w, i // w
        if fmt == 9:    # a4r4g4b4
            a = ((v >> 12) & 0xF) * 17; r = ((v >> 8) & 0xF) * 17
            g = ((v >> 4) & 0xF) * 17;  b = (v & 0xF) * 17
        elif fmt == 8:  # a1r5g5b5
            a = 255 if (v >> 15) & 1 else 0
            r = round(((v >> 10) & 0x1F) * 8.226); g = round(((v >> 5) & 0x1F) * 8.226); b = round((v & 0x1F) * 8.226)
        elif fmt == 6:  # r5g6b5
            a = 255
            r = round(((v >> 11) & 0x1F) * 8.226); g = round(((v >> 5) & 0x3F) * 4.047); b = round((v & 0x1F) * 8.226)
        else:
            a = r = g = b = 0
        p[x, y] = (r, g, b, a)
    return img


def record_dims(b, o):
    """Return (w, h, fmt) if a valid bitm record sits at offset o, else None."""
    if o + 0x50 > len(b):
        return None
    if b[o:o + 4] != b"mtib":  # 'bitm' little-endian
        return None
    w, h = u16(b, o + W_OFF), u16(b, o + H_OFF)
    t, f = u16(b, o + TYPE_OFF), u16(b, o + FMT_OFF)
    if not (w in VALID_DIMS and h in VALID_DIMS and t <= 3 and f in BPP):
        return None
    if u32(b, o + LOD1_SIZE_OFF) != int(w * h * BPP[f]):  # uncompressed, no mips
        return None
    return w, h, f


def find_clusters(b):
    """Maximal runs of consecutive (stride-116) valid records in the meta region."""
    idxoff = u32(b, 0x10)
    runs, o = [], idxoff
    while o + 0x50 <= len(b):
        if record_dims(b, o):
            start, recs = o, []
            while record_dims(b, o):
                recs.append((o, *record_dims(b, o)))
                o += STRIDE
            if len(recs) >= 8:
                runs.append((start, recs))
        else:
            o += 4
    return runs


def pick_cluster(runs, count, dim, label):
    cands = [r for r in runs if len(r[1]) == count and r[1][0][1] == dim]
    if not cands:
        sys.exit(f"could not locate the {label} cluster (count={count}, {dim}x{dim})")
    # the highest-resolution / first match is the main (non mip-set) tag
    return cands[0]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--maps", required=True, help="dir with Xbox shared.map + mainmenu.map")
    ap.add_argument("--out", required=True, help="output dir (e.g. sveltekit/static/emblems)")
    a = ap.parse_args()
    shared = open(os.path.join(a.maps, "shared.map"), "rb").read()
    mainmenu = open(os.path.join(a.maps, "mainmenu.map"), "rb").read()
    pools = {0: shared, 1: mainmenu, 2: shared, 3: None}

    runs = find_clusters(shared)
    fg = pick_cluster(runs, 64, 64, "foreground")   # 64 symbols @ 64x64
    bg = pick_cluster(runs, 32, 128, "background")  # 32 shapes  @ 128x128
    print(f"foreground cluster @ {fg[0]:#x} ({len(fg[1])} records)")
    print(f"background cluster @ {bg[0]:#x} ({len(bg[1])} records)")

    def emit(cluster, kind):
        d = os.path.join(a.out, kind)
        os.makedirs(d, exist_ok=True)
        for idx, (o, w, h, f) in enumerate(cluster[1]):
            lod1 = u32(shared, o + LOD1_OFF)
            sel, off = (lod1 >> 30) & 3, lod1 & 0x3FFFFFFF
            raw = pools[sel][off:off + w * h * BPP[f]]
            img = decode_16bpp(deswizzle(raw, w, h, BPP[f]), w, h, f)
            img.getchannel("R").save(os.path.join(d, f"{idx:02d}_p.png"))   # primary
            img.getchannel("B").save(os.path.join(d, f"{idx:02d}_s.png"))   # secondary
            img.save(os.path.join(d, f"{idx:02d}_ref.png"))
        print(f"wrote {len(cluster[1])} {kind} emblems -> {d}")

    emit(fg, "fg")
    emit(bg, "bg")


if __name__ == "__main__":
    main()
