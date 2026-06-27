# H2 emblem extraction (real in-game art)

Extracts the real Halo 2 multiplayer emblem set — **64 foreground symbols +
32 background shapes** — from the stock **Xbox** Halo 2 cache maps and emits the
tint masks the appearance studio uses (`sveltekit/static/emblems/{fg,bg}/<NN>_{p,s}.png`).

For personal / LAN use with your own copy of the game. Don't redistribute the
extracted art.

## What worked (and what didn't)

| Tool | Result |
| --- | --- |
| **Pytolith** | ✗ for this job — reads *loose* H2EK tags only; we have packed maps, no editing kit installed. |
| **Sigmmma `reclaimer` + `arbytmap`** (Python, Linux) | ◐ loads the Xbox cache, reads the tag index/paths perfectly — but its H2-**Xbox** *bitmap meta* parser returns empty (Vista-shaped defs). Used only to locate the emblem tags during development. Needs `audioop-lts` on Python 3.13+ and a `setup_defs` shim (`h2lib.py`). |
| **Reclaimer (.NET) / H2EK `tool` under Wine** | not needed — no Mono/.NET here, and the route below works. |
| **Direct parse (this script)** | ✓ The emblem `bitm` records are a fixed 116-byte struct; the pixels are an uncompressed, Xbox-swizzled pool referenced by a raw pointer. Parse the record + deswizzle + decode → done. `arbytmap` not required for these 16-bit formats. |

## The format, briefly

- Maps: original Xbox `shared.map` (183 MB) + `mainmenu.map`, pulled from the
  disc via xdvdfs (`~/scratch/h2extract/out/`). Cache version 8, **uncompressed**
  (MCC's version-13 maps are zlib + `textures.dat` — that's why a naive grep over
  MCC fails; the Xbox maps have plaintext tag paths).
- Emblem tags: `ui\global_bitmaps\emblems\{foreground,background}` (`bitm`).
- Foreground = 64× 64×64 `a4r4g4b4`; background = 32× 128×128
  (`r5g6b5`/`a1r5g5b5`/`a4r4g4b4`). All Xbox-swizzled (Morton). Pixel data lives
  in the `mainmenu` pool, referenced by the record's `lod1_offset` raw pointer
  (top 2 bits select the map: `[local, mainmenu, shared, single_player_shared]`).
- **Two-tone encoding**: each texel blends a PRIMARY marker (yellow, R channel)
  and a SECONDARY marker (blue, B channel). `_p.png` = R coverage, `_s.png` = B
  coverage. The studio fills the primary mask with one colour and the secondary
  mask with the other — background plate in the two armor colours, foreground
  symbol in the two emblem colours.

## Run

```bash
python -m venv venv && . venv/bin/activate
pip install pillow                     # only Pillow is needed for the final pass
python extract_emblems.py \
  --maps /path/to/xbox/maps \          # dir with shared.map + mainmenu.map
  --out  ../../sveltekit/static/emblems
```

The enum order of the output matches `e_emblem_foreground` /
`e_emblem_background` (index 54–63 are the digits 0–9 — a handy alignment check),
so file `<NN>` is emblem index `NN` and lines up with the `0x11D`/`0x11E` profile
bytes documented in `docs/gamertag-system/H2-EMBLEM-FORMAT.md`.

## Files

- `extract_emblems.py` — the extractor (Pillow only).
- `h2lib.py` — the `reclaimer` Python-3.14 shim used while locating tags.
- `SPARTAN-MODEL.md` + `spartan_model_inventory.json` — Mark VI model status
  (located; mesh export blocked on Linux) and the pose/render plan.
