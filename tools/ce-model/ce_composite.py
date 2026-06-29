#!/usr/bin/env python3
"""CE Mark V tint compositor — the H1 analogue of spartan_composite.py.

Halo CE tints the Spartan by a SINGLE armor colour (the profile colour enum at
`blam.sav` u32 @ 0x18) applied over the model's change-colour region — the
**multipurpose map's blue channel** (baked to cyborg_cc.png and rendered as the
`<pose>_c.png` coverage pass). There is **no** primary/secondary split and **no**
emblem on CE.

Tint model (matches the in-game colour-change shader): in the masked region the
diffuse is multiplied by the armour colour, so texture detail shows through and
dark colours darken — final = diffuse * lerp(1, colour, mask).

Run:
  python3 ce_composite.py --render out/mcc/render --out out/mcc/composite \
        --pose Salute --colors Green,Red,Cobalt,Steel
Colours are names or indices into the CE palette (../../sveltekit/src/lib/data/
halo-armor-palettes.json), or #RRGGBB.
"""
import argparse, json, os
import numpy as np
from PIL import Image

HERE = os.path.dirname(os.path.abspath(__file__))
PALETTE = os.path.join(HERE, "..", "..", "sveltekit", "src", "lib", "data", "halo-armor-palettes.json")


def load_palette():
    d = json.load(open(PALETTE))["ce"]
    by_name, by_idx = {}, {}
    for k, v in d.items():
        if not k.isdigit():
            continue
        by_idx[int(k)] = (v["name"], tuple(v["rgb"]))
        by_name[v["name"].lower()] = tuple(v["rgb"])
    return by_name, by_idx


def resolve_color(tok, by_name, by_idx):
    t = tok.strip()
    if t.startswith("#") and len(t) == 7:
        return t, (int(t[1:3], 16), int(t[3:5], 16), int(t[5:7], 16))
    if t.isdigit() and int(t) in by_idx:
        nm, rgb = by_idx[int(t)]
        return nm, rgb
    if t.lower() in by_name:
        return t, by_name[t.lower()]
    raise SystemExit(f"unknown colour {tok!r}")


def composite(diffuse_png, mask_png, rgb):
    diff = np.asarray(Image.open(diffuse_png).convert("RGBA"), np.float32) / 255.0
    mk = np.asarray(Image.open(mask_png).convert("RGBA"), np.float32) / 255.0
    mask = mk[..., 0:1]                                   # coverage (grayscale)
    col = np.array(rgb, np.float32).reshape(1, 1, 3) / 255.0
    tint = (1.0 - mask) + mask * col                      # lerp(1, colour, mask)
    out = diff.copy()
    out[..., :3] = np.clip(diff[..., :3] * tint, 0, 1)
    out[..., 3] = diff[..., 3]                            # keep silhouette alpha
    return Image.fromarray((out * 255 + 0.5).astype(np.uint8), "RGBA")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--render", required=True, help="dir with <pose>.png + <pose>_c.png")
    ap.add_argument("--out", required=True)
    ap.add_argument("--pose", default="Default")
    ap.add_argument("--colors", default="Green,Red,Blue,Gray")
    a = ap.parse_args()
    os.makedirs(a.out, exist_ok=True)
    by_name, by_idx = load_palette()
    diffuse = os.path.join(a.render, f"{a.pose}.png")
    mask = os.path.join(a.render, f"{a.pose}_c.png")
    for tok in a.colors.split(","):
        nm, rgb = resolve_color(tok, by_name, by_idx)
        img = composite(diffuse, mask, rgb)
        p = os.path.join(a.out, f"{a.pose}_{nm.lower().replace(' ', '_')}.png")
        img.save(p)
        print(f"  {a.pose} + {nm} {rgb} -> {p}")


if __name__ == "__main__":
    main()
