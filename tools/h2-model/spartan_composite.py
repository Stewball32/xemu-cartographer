#!/usr/bin/env python3
"""Compositor proof — the appearance-studio 'render-on-change' step, run on Linux.

Takes the pre-rendered pose passes (<pose>.png diffuse, _p primary coverage,
_s secondary coverage) + the real extracted emblem masks and produces the final
tinted, emblemed Mark VI, exactly like the SvelteKit compositor will in-app:

    armor tint:  diffuse * ( primary_colour where cc-primary,
                             secondary_colour where cc-secondary,
                             untinted elsewhere )            # multiply keeps shading
    emblem decal: real H2 emblem (two-tone fg over two-tone bg plate) on the chest

This mirrors emblem-art.ts (two()/plate() two-tone mask fills) in raster form so
the pipeline can be verified headless; the shipping app does the same with SVG
masks over the same PNG assets.
"""
import argparse, os, json
import numpy as np
from PIL import Image


def hex_rgb(h):
    h = h.lstrip("#")
    return np.array([int(h[i:i + 2], 16) for i in (0, 2, 4)], np.float32)


def tint(diffuse, pmask, smask, primary, secondary):
    base = np.asarray(diffuse.convert("RGBA"), np.float32)
    p = (np.asarray(pmask.convert("L"), np.float32) / 255.0)[..., None]
    s = (np.asarray(smask.convert("L"), np.float32) / 255.0)[..., None]
    white = np.ones(3, np.float32)
    tintmap = white * (1 - p - s).clip(0, 1) + primary / 255.0 * p + secondary / 255.0 * s
    out = base.copy()
    out[..., :3] = (base[..., :3] * tintmap).clip(0, 255)
    return Image.fromarray(out.astype(np.uint8), "RGBA")


def emblem(fg_dir, bg_dir, fg_i, bg_i, e_pri, e_sec, plate_pri, plate_sec, size=256):
    """Build a real two-tone emblem: bg plate (plate colours) + fg symbol (emblem colours)."""
    N = 128  # common working size (bg masks are 128, fg are 64)
    def load(d, i, ch):
        im = Image.open(os.path.join(d, f"{i:02d}_{ch}.png")).convert("L").resize((N, N), Image.LANCZOS)
        return np.asarray(im, np.float32) / 255.0
    bp = load(bg_dir, bg_i, "p")
    img = np.zeros((N, N, 4), np.float32)
    # bg plate: base=secondary, overlay primary by coverage (emblem-art plate())
    img[..., :3] = plate_sec / 255.0
    img[..., :3] = img[..., :3] * (1 - bp[..., None]) + plate_pri / 255.0 * bp[..., None]
    img[..., 3] = 1.0
    # fg symbol two-tone over the plate (emblem-art two())
    fp = load(fg_dir, fg_i, "p"); fs = load(fg_dir, fg_i, "s")
    cov = (fp + fs).clip(0, 1)[..., None]
    sym = e_pri / 255.0 * fp[..., None] + e_sec / 255.0 * fs[..., None]
    img[..., :3] = img[..., :3] * (1 - cov) + sym * cov
    out = (img * 255).astype(np.uint8)
    return Image.fromarray(out, "RGBA").resize((size, size), Image.NEAREST)


def chest_box(diffuse):
    a = np.asarray(diffuse.convert("RGBA"))[..., 3]
    ys, xs = np.where(a > 16)
    y0, y1, x0, x1 = ys.min(), ys.max(), xs.min(), xs.max()
    h = y1 - y0; w = x1 - x0
    cx = (x0 + x1) // 2
    cy = int(y0 + 0.34 * h)          # chest height on a full-body figure
    ew = int(0.16 * w)               # emblem ~16% of figure width
    return cx, cy, ew


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--render", required=True, help="dir with <pose>{,_p,_s}.png")
    ap.add_argument("--emblems", required=True, help="sveltekit/static/emblems dir")
    ap.add_argument("--out", required=True)
    ap.add_argument("--pose", default="idle")
    a = ap.parse_args()
    os.makedirs(a.out, exist_ok=True)

    diffuse = Image.open(os.path.join(a.render, f"{a.pose}.png"))
    pmask = Image.open(os.path.join(a.render, f"{a.pose}_p.png"))
    smask = Image.open(os.path.join(a.render, f"{a.pose}_s.png"))
    fg = os.path.join(a.emblems, "fg"); bg = os.path.join(a.emblems, "bg")

    # a few armor combos (primary, secondary) + emblem to show "render-on-change"
    combos = [
        ("red_white",   "#BE2C2C", "#FDFEFF"),
        ("cobalt_blue", "#416C8F", "#28459B"),
        ("green_tan",   "#21922F", "#B19256"),
    ]
    em = emblem(fg, bg, fg_i=7, bg_i=5,
                e_pri=hex_rgb("#FDFEFF"), e_sec=hex_rgb("#BE2C2C"),
                plate_pri=hex_rgb("#416C8F"), plate_sec=hex_rgb("#28459B"))
    cx, cy, ew = chest_box(diffuse)
    em_r = em.resize((ew, ew), Image.LANCZOS)

    outs = []
    for name, pri, sec in combos:
        comp = tint(diffuse, pmask, smask, hex_rgb(pri), hex_rgb(sec))
        comp.alpha_composite(em_r, (cx - ew // 2, cy - ew // 2))
        p = os.path.join(a.out, f"{a.pose}_{name}.png")
        comp.save(p); outs.append(p); print("wrote", p)

    # contact sheet
    cell = 300
    sheet = Image.new("RGB", (cell * len(combos), cell), (24, 24, 28))
    for i, p in enumerate(outs):
        im = Image.open(p).convert("RGBA")
        bgc = Image.new("RGBA", im.size, (18, 18, 22, 255)); bgc.alpha_composite(im)
        sheet.paste(bgc.convert("RGB").resize((cell, cell)), (i * cell, 0))
    sheet.save(os.path.join(a.out, f"_composite_{a.pose}.png"))
    print("sheet:", os.path.join(a.out, f"_composite_{a.pose}.png"))


if __name__ == "__main__":
    main()
