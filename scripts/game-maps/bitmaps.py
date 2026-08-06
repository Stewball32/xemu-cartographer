"""
Halo: Combat Evolved (Xbox) bitmap-tag decoder -> RGBA / PNG.

Handles the pixel formats Halo 1 uses for interface bitmaps and de-swizzles
Xbox-swizzled (Morton/Z-order) textures when the bitmap-data flag says so.
Compressed (DXT1/3/5) textures are block-linear and are never swizzled.

Bitmap-data element (0x30 bytes), parsed in halomap-independent form:
  0x00 char[4] 'mtib'           signature (LE 'bitm')
  0x04 u16 width
  0x06 u16 height
  0x08 u16 depth
  0x0A u16 type                 0=2D 1=3D 2=cubemap 3=sprite
  0x0C u16 format               see FORMAT below
  0x0E u16 flags                bit0 power-of-two, bit1 compressed,
                                bit2 palettized, bit3 swizzled, bit4 linear
  0x10 u16 reg_x  0x12 u16 reg_y
  0x14 u16 mipmap_count 0x16 u16 ...
  0x18 u32 pixel_data_offset    absolute offset into the map image
  0x1C u32 pixel_data_size
  0x20 u32 ... 0x24 u32 ... 0x28 u32 ... 0x2C u32 ...
"""

from __future__ import annotations

import struct
from dataclasses import dataclass

import numpy as np
from PIL import Image

BITMAP_DATA_SIZE = 0x30
SIG = b"mtib"

# format id -> name
FORMAT = {
    0: "A8", 1: "Y8", 2: "AY8", 3: "A8Y8",
    6: "R5G6B5", 8: "A1R5G5B5", 9: "A4R4G4B4",
    10: "X8R8G8B8", 11: "A8R8G8B8",
    14: "DXT1", 15: "DXT3", 16: "DXT5", 17: "P8",
}

FLAG_POWER_OF_TWO = 0x01
FLAG_COMPRESSED = 0x02
FLAG_PALETTIZED = 0x04
FLAG_SWIZZLED = 0x08
FLAG_LINEAR = 0x10


@dataclass
class BitmapData:
    index: int
    width: int
    height: int
    depth: int
    type: int
    format: int
    flags: int
    mipmaps: int
    pixel_offset: int
    pixel_size: int

    @property
    def format_name(self) -> str:
        return FORMAT.get(self.format, f"fmt{self.format}")

    @property
    def swizzled(self) -> bool:
        return bool(self.flags & FLAG_SWIZZLED)


def parse_bitmap_data(buf: bytes, index: int = 0) -> BitmapData:
    sig = buf[0:4]
    if sig != SIG:
        raise ValueError(f"bad bitmap-data signature {sig!r}")
    w, h, depth, typ = struct.unpack_from("<HHHH", buf, 4)
    fmt, flags = struct.unpack_from("<HH", buf, 0x0C)
    mip = struct.unpack_from("<H", buf, 0x14)[0]
    pix_off, pix_size = struct.unpack_from("<II", buf, 0x18)
    return BitmapData(index, w, h, depth, typ, fmt, flags, mip, pix_off, pix_size)


# --------------------------------------------------------------------------
# DXT (BC1/2/3) decoders — vectorised with numpy.
# --------------------------------------------------------------------------
def _unpack_565(c: np.ndarray):
    r = ((c >> 11) & 0x1F).astype(np.uint16)
    g = ((c >> 5) & 0x3F).astype(np.uint16)
    b = (c & 0x1F).astype(np.uint16)
    r = (r * 255 + 15) // 31
    g = (g * 255 + 31) // 63
    b = (b * 255 + 15) // 31
    return r.astype(np.uint8), g.astype(np.uint8), b.astype(np.uint8)


def _dxt_color_block(color_bytes: np.ndarray, dxt1_alpha: bool):
    """color_bytes: (N,8) uint8. Returns (N,16,4) uint8 RGBA."""
    n = color_bytes.shape[0]
    c0 = color_bytes[:, 0].astype(np.uint16) | (color_bytes[:, 1].astype(np.uint16) << 8)
    c1 = color_bytes[:, 2].astype(np.uint16) | (color_bytes[:, 3].astype(np.uint16) << 8)
    bits = (color_bytes[:, 4].astype(np.uint32)
            | (color_bytes[:, 5].astype(np.uint32) << 8)
            | (color_bytes[:, 6].astype(np.uint32) << 16)
            | (color_bytes[:, 7].astype(np.uint32) << 24))
    r0, g0, b0 = _unpack_565(c0)
    r1, g1, b1 = _unpack_565(c1)

    pal = np.zeros((n, 4, 4), dtype=np.uint8)
    pal[:, 0, 0], pal[:, 0, 1], pal[:, 0, 2], pal[:, 0, 3] = r0, g0, b0, 255
    pal[:, 1, 0], pal[:, 1, 1], pal[:, 1, 2], pal[:, 1, 3] = r1, g1, b1, 255

    r0i, g0i, b0i = r0.astype(np.int16), g0.astype(np.int16), b0.astype(np.int16)
    r1i, g1i, b1i = r1.astype(np.int16), g1.astype(np.int16), b1.astype(np.int16)

    if dxt1_alpha:
        opaque = c0 > c1  # per-block: 4-color vs 3-color+transparent
        # 4-colour blocks
        pal[:, 2, 0] = np.where(opaque, (2 * r0i + r1i) // 3, (r0i + r1i) // 2)
        pal[:, 2, 1] = np.where(opaque, (2 * g0i + g1i) // 3, (g0i + g1i) // 2)
        pal[:, 2, 2] = np.where(opaque, (2 * b0i + b1i) // 3, (b0i + b1i) // 2)
        pal[:, 2, 3] = 255
        pal[:, 3, 0] = np.where(opaque, (r0i + 2 * r1i) // 3, 0)
        pal[:, 3, 1] = np.where(opaque, (g0i + 2 * g1i) // 3, 0)
        pal[:, 3, 2] = np.where(opaque, (b0i + 2 * b1i) // 3, 0)
        pal[:, 3, 3] = np.where(opaque, 255, 0)
    else:
        pal[:, 2, 0] = (2 * r0i + r1i) // 3
        pal[:, 2, 1] = (2 * g0i + g1i) // 3
        pal[:, 2, 2] = (2 * b0i + b1i) // 3
        pal[:, 2, 3] = 255
        pal[:, 3, 0] = (r0i + 2 * r1i) // 3
        pal[:, 3, 1] = (g0i + 2 * g1i) // 3
        pal[:, 3, 2] = (b0i + 2 * b1i) // 3
        pal[:, 3, 3] = 255

    idx = np.zeros((n, 16), dtype=np.uint8)
    for px in range(16):
        idx[:, px] = (bits >> (2 * px)) & 0x3
    out = np.take_along_axis(pal, idx[:, :, None].repeat(4, axis=2), axis=1)
    return out  # (n,16,4)


def _decode_dxt(data: bytes, w: int, h: int, variant: str) -> np.ndarray:
    bw, bh = (w + 3) // 4, (h + 3) // 4
    nblocks = bw * bh
    block_size = 8 if variant == "DXT1" else 16
    need = nblocks * block_size
    if len(data) < need:
        data = data + b"\x00" * (need - len(data))
    raw = np.frombuffer(data[:need], dtype=np.uint8).reshape(nblocks, block_size)

    color_off = 0 if variant == "DXT1" else 8
    rgba = _dxt_color_block(raw[:, color_off:color_off + 8], dxt1_alpha=(variant == "DXT1"))

    if variant == "DXT3":
        a = raw[:, 0:8]
        alpha = np.zeros((nblocks, 16), dtype=np.uint8)
        for px in range(16):
            byte = a[:, px // 2]
            nib = np.where(px % 2 == 0, byte & 0x0F, byte >> 4)
            alpha[:, px] = (nib * 255 // 15).astype(np.uint8)
        rgba[:, :, 3] = alpha
    elif variant == "DXT5":
        a = raw[:, 0:8]
        a0 = a[:, 0].astype(np.int16)
        a1 = a[:, 1].astype(np.int16)
        abits = np.zeros((nblocks,), dtype=np.uint64)
        for i in range(6):
            abits |= (a[:, 2 + i].astype(np.uint64) << (8 * i))
        atab = np.zeros((nblocks, 8), dtype=np.uint8)
        atab[:, 0] = a0
        atab[:, 1] = a1
        gt = a0 > a1
        for t in range(1, 7):
            atab[:, 1 + t] = np.where(
                gt, ((7 - t) * a0 + t * a1) // 7, ((5 - t) * a0 + t * a1) // 5 if t <= 5 else 0)
        atab[:, 6] = np.where(gt, atab[:, 6], 0)
        atab[:, 7] = np.where(gt, atab[:, 7], 255)
        alpha = np.zeros((nblocks, 16), dtype=np.uint8)
        for px in range(16):
            ai = (abits >> np.uint64(3 * px)) & np.uint64(0x7)
            alpha[:, px] = np.take_along_axis(atab, ai.astype(np.int64)[:, None], axis=1)[:, 0]
        rgba[:, :, 3] = alpha

    # reassemble 4x4 blocks into the full image
    img = np.zeros((bh * 4, bw * 4, 4), dtype=np.uint8)
    blk = rgba.reshape(bh, bw, 16, 4)
    for by in range(4):
        for bx in range(4):
            img[by::4, bx::4] = blk[:, :, by * 4 + bx, :]
    return img[:h, :w]


# --------------------------------------------------------------------------
# Uncompressed formats + Xbox unswizzle.
# --------------------------------------------------------------------------
def _unswizzle(data: bytes, w: int, h: int, bpp: int) -> bytes:
    """Undo Xbox Morton/Z-order swizzle for power-of-two textures."""
    out = bytearray(len(data))
    def log2(x):
        r = 0
        while (1 << r) < x:
            r += 1
        return r
    lw, lh = log2(w), log2(h)
    # interleave masks
    def part1by1(n, bits):
        v = 0
        for i in range(bits):
            v |= ((n >> i) & 1) << (2 * i)
        return v
    src = 0
    for y in range(h):
        for x in range(w):
            # swizzled address interleaves x and y bits
            sx = part1by1(x, lw)
            sy = part1by1(y, lh)
            s = (sx | (sy << 1)) if lw <= lh else (sx | (sy << 1))
            di = (y * w + x) * bpp
            si = s * bpp
            out[di:di + bpp] = data[si:si + bpp]
    return bytes(out)


def _decode_uncompressed(data: bytes, w: int, h: int, fmt: str, swizzled: bool) -> np.ndarray:
    bpp = {"A8": 1, "Y8": 1, "P8": 1, "AY8": 1,
           "A8Y8": 2, "R5G6B5": 2, "A1R5G5B5": 2, "A4R4G4B4": 2,
           "X8R8G8B8": 4, "A8R8G8B8": 4}[fmt]
    need = w * h * bpp
    if len(data) < need:
        data = data + b"\x00" * (need - len(data))
    data = data[:need]
    if swizzled and (w & (w - 1)) == 0 and (h & (h - 1)) == 0:
        data = _unswizzle(data, w, h, bpp)
    arr = np.frombuffer(data, dtype=np.uint8)
    out = np.zeros((h, w, 4), dtype=np.uint8)
    if fmt in ("A8R8G8B8", "X8R8G8B8"):
        px = arr.reshape(h, w, 4)  # B,G,R,A
        out[..., 0] = px[..., 2]; out[..., 1] = px[..., 1]; out[..., 2] = px[..., 0]
        out[..., 3] = 255 if fmt == "X8R8G8B8" else px[..., 3]
    elif fmt in ("R5G6B5", "A1R5G5B5", "A4R4G4B4"):
        px = arr.view("<u2").reshape(h, w)
        if fmt == "R5G6B5":
            r = ((px >> 11) & 0x1F) * 255 // 31
            g = ((px >> 5) & 0x3F) * 255 // 63
            b = (px & 0x1F) * 255 // 31
            a = np.full_like(r, 255)
        elif fmt == "A1R5G5B5":
            a = ((px >> 15) & 1) * 255
            r = ((px >> 10) & 0x1F) * 255 // 31
            g = ((px >> 5) & 0x1F) * 255 // 31
            b = (px & 0x1F) * 255 // 31
        else:  # A4R4G4B4
            a = ((px >> 12) & 0xF) * 255 // 15
            r = ((px >> 8) & 0xF) * 255 // 15
            g = ((px >> 4) & 0xF) * 255 // 15
            b = (px & 0xF) * 255 // 15
        out[..., 0] = r; out[..., 1] = g; out[..., 2] = b; out[..., 3] = a
    elif fmt in ("A8", "Y8", "AY8", "P8"):
        v = arr.reshape(h, w)
        if fmt == "A8":
            out[..., 0] = out[..., 1] = out[..., 2] = 255
            out[..., 3] = v
        else:  # Y8/AY8/P8 -> grayscale opaque
            out[..., 0] = out[..., 1] = out[..., 2] = v
            out[..., 3] = 255
    elif fmt == "A8Y8":
        px = arr.reshape(h, w, 2)
        out[..., 0] = out[..., 1] = out[..., 2] = px[..., 0]
        out[..., 3] = px[..., 1]
    return out


def decode_to_image(image_bytes: bytes, bd: BitmapData) -> Image.Image:
    """Decode one BitmapData (top mip) into a PIL RGBA Image."""
    w, h = bd.width, bd.height
    fmt = bd.format_name
    data = image_bytes[bd.pixel_offset: bd.pixel_offset + bd.pixel_size]
    if fmt in ("DXT1", "DXT3", "DXT5"):
        rgba = _decode_dxt(data, w, h, fmt)
    elif fmt in FORMAT.values():
        rgba = _decode_uncompressed(data, w, h, fmt, bd.swizzled)
    else:
        raise ValueError(f"unsupported format {fmt}")
    return Image.fromarray(rgba, "RGBA")


def first_bitmap_data(map_obj, tag) -> BitmapData | None:
    """Read the first bitmap-data element of a bitm tag (count/addr block @0x60)."""
    o = map_obj.off(tag.meta_vaddr)
    cnt, addr, _ = struct.unpack_from("<III", map_obj.image, o + 0x60)
    if not cnt or not map_obj.in_image(addr):
        return None
    eo = map_obj.off(addr)
    return parse_bitmap_data(map_obj.image[eo:eo + BITMAP_DATA_SIZE], 0)


def all_bitmap_data(map_obj, tag) -> list[BitmapData]:
    o = map_obj.off(tag.meta_vaddr)
    cnt, addr, _ = struct.unpack_from("<III", map_obj.image, o + 0x60)
    out = []
    if not cnt or not map_obj.in_image(addr):
        return out
    eo = map_obj.off(addr)
    for i in range(cnt):
        buf = map_obj.image[eo + i * BITMAP_DATA_SIZE: eo + (i + 1) * BITMAP_DATA_SIZE]
        if len(buf) < BITMAP_DATA_SIZE or buf[0:4] != SIG:
            break
        out.append(parse_bitmap_data(buf, i))
    return out
