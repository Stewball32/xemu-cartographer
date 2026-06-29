#!/usr/bin/env python3
"""Native, Linux-only decoder for the Halo: Combat Evolved **Xbox** `mode`
(model) tag — the CE **Mark V** Master Chief (`characters\\cyborg\\cyborg`).

This is the H1 complement to `../h2-model/h2_render_model.py`. Where the H2
Mark VI is a `render_model` (`mode`, compressed/bounded int16 positions,
node-skin), CE's Master Chief is the older **gbxmodel-family `mode`** tag with
float positions + an Xbox D3D-resource vertex/index indirection. Same approach:
parse the Xbox cache ourselves (reusing the H1 `halomap.py` tag-index reader),
decode the geometry field-for-field, and emit the same numpy bundle the Blender
pose/render pipeline consumes.

Format authority: reverse-engineered field-for-field from the stock Xbox cache
(see CE-MARKV-MODEL.md) and cross-checked against the Invader / c20 `gbxmodel`
spec. Everything is derived from the map's own data (counts, pointers, the
cache tag-data header) — no hardcoded offsets beyond the documented tag layout.

The decoded **Xbox `mode` vertex is 32 bytes**:
    +0x00  float  x, y, z            (uncompressed position)
    +0x0C  uint32 normal             (11/11/10 signed, /1023,/1023,/511)
    +0x10  uint32 binormal           (unused here)
    +0x14  uint32 tangent            (unused here)
    +0x18  int16  u, v               (/32767)
    +0x1C  uint8  node0_index * 3
    +0x1D  uint8  node1_index * 3    (0xFD/253 = none)
    +0x1E  int16  node0_weight       (/32767; node1_weight = 1 - node0_weight)
Indices are uint16 triangle strips (degenerate-join doublets), local to a part,
bounded by the next part's index pointer / a 0xFFFF pad.

Outputs (into --out):
  cyborg.obj          universal triangulated sanity mesh (pos/uv/normal)
  cyborg.npz          numpy arrays + 19-bone skeleton for the Blender assembler
  cyborg.model.json   human-readable manifest (bones, regions, parts, counts)

IP: real Halo CE asset; personal / LAN use with your own copy. Do not
redistribute the extracted assets.
"""
import argparse, json, os, struct, sys
import numpy as np

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from halomap import parse_map

# ---- model tag header reflexive offsets (derived from the cyborg meta; the
# gbxmodel/model header is 0xE8 bytes, reflexives = 12B [count i32][vaddr u32][pad]) ----
RM_MARKERS    = 0xAC
RM_NODES      = 0xB8
RM_REGIONS    = 0xC4
RM_GEOMETRIES = 0xD0
RM_SHADERS    = 0xDC

NODE_SIZE   = 156      # 0x9C
REGION_SIZE = None     # regions are variable (header 0x4C + inline perms); walked via reflexive
PERM_SIZE   = 88       # 0x58
GEOM_SIZE   = 48       # 0x30; parts reflexive at +0x24
PART_SIZE   = 104      # 0x68
VTX_SIZE    = 32

# permutation per-LOD geometry index fields (int16), at +0x40
PERM_LOD = {"superlow": 0x40, "low": 0x42, "medium": 0x44, "high": 0x46, "superhigh": 0x48}


def henn3(u):
    """11/11/10 signed-normalised vector packed in a uint32 (same as H2 HenDN3)."""
    def sx(v, bits):
        s = 1 << (bits - 1)
        return (v - (1 << bits)) if (v & s) else v
    x = sx(u & 0x7FF, 11) / 1023.0
    y = sx((u >> 11) & 0x7FF, 11) / 1023.0
    z = sx((u >> 22) & 0x3FF, 10) / 511.0
    return x, y, z


class Model:
    def __init__(self, mp, tag_suffix="cyborg\\cyborg"):
        self.m = parse_map(mp)
        self.img = self.m.image
        self.tag = next((t for t in self.m.tags
                         if t.group == "mode" and t.path.endswith(tag_suffix)), None)
        if self.tag is None:
            raise SystemExit(f"no 'mode' tag ending {tag_suffix!r} in {mp}")
        self.base = self.m.off(self.tag.meta_vaddr)

    # primitive reads at an image file offset
    def u8(self, o):  return self.img[o]
    def i16(self, o): return struct.unpack_from("<h", self.img, o)[0]
    def u16(self, o): return struct.unpack_from("<H", self.img, o)[0]
    def i32(self, o): return struct.unpack_from("<i", self.img, o)[0]
    def u32(self, o): return struct.unpack_from("<I", self.img, o)[0]
    def f(self, o):   return struct.unpack_from("<f", self.img, o)[0]

    def refl(self, meta_field_off):
        """A 12-byte reflexive at base+off -> (count, file_off_of_elements)."""
        o = self.base + meta_field_off
        count = self.i32(o)
        vaddr = self.u32(o + 4)
        return count, self.m.off(vaddr)

    def cstr(self, o, n=32):
        return self.img[o:o + n].split(b"\x00", 1)[0].decode("latin1", "replace")

    # ---- skeleton ----
    def nodes(self):
        n, off = self.refl(RM_NODES)
        out = []
        for k in range(n):
            o = off + k * NODE_SIZE
            out.append({
                "name": self.cstr(o),
                "next_sibling": self.i16(o + 0x20),
                "first_child": self.i16(o + 0x22),
                "parent": self.i16(o + 0x24),
                "pos": (self.f(o + 0x28), self.f(o + 0x2C), self.f(o + 0x30)),
                # Tag stores rotation as (i, j, k, w). The H1 `mode` tag uses the
                # INVERSE (world->local) convention vs H2's `render_model`
                # (local->world): the same Halo pelvis is (.5,.5,.5,+.5) here but
                # (.5,.5,.5,-.5) in H2. Conjugate so FK comes out Z-up and matches
                # the mesh (verified: head joint -> Z=0.609, identical to H2), and
                # so the shared MCC pose-retarget consumes it unchanged.
                "rot": (-self.f(o + 0x34), -self.f(o + 0x38), -self.f(o + 0x3C), self.f(o + 0x40)),
                "dist": self.f(o + 0x44),
            })
        return out

    # ---- regions -> permutations -> super-high geometry indices ----
    def superhigh_geometries(self):
        n, off = self.refl(RM_REGIONS)
        geoms = []
        # regions are laid out [region header 0x4C][its perms inline]..., but each
        # region's permutations reflexive gives an explicit pointer, so walk that.
        # Region stride isn't fixed; we re-read each region's perms reflexive at +0x40.
        # For robustness we re-derive the region element offset from its reflexive,
        # so just iterate the contiguous region headers using the perms pointer to
        # skip past inline perm data.
        ro = off
        for r in range(n):
            rname = self.cstr(ro)
            pcount = self.i32(ro + 0x40)
            ppoff = self.m.off(self.u32(ro + 0x44))
            for p in range(pcount):
                po = ppoff + p * PERM_SIZE
                gi = self.i16(po + PERM_LOD["superhigh"])
                geoms.append((rname, self.cstr(po), gi))
            # advance to the next region header: just past this region's perm block
            ro = ppoff + pcount * PERM_SIZE
        return geoms

    def geometry_parts(self, gi):
        gn, goff = self.refl(RM_GEOMETRIES)
        go = goff + gi * GEOM_SIZE
        pcount = self.i32(go + 0x24)
        poff = self.m.off(self.u32(go + 0x28))
        parts = []
        for p in range(pcount):
            o = poff + p * PART_SIZE
            shader = self.u16(o + 0x04)
            vcount = self.u32(o + 0x58)
            iptr = self.u32(o + 0x4C)           # index buffer vaddr
            vdesc = self.m.off(self.u32(o + 0x64))  # 12B vertex descriptor
            vptr = self.u32(vdesc + 0x04)       # -> vertex data vaddr
            parts.append({
                "index": p, "shader": shader, "vcount": vcount,
                "ioff": self.m.off(iptr), "voff": self.m.off(vptr),
            })
        return parts


def decode(model: Model, debug=False):
    nodes = model.nodes()
    geoms = model.superhigh_geometries()
    used = sorted({g[2] for g in geoms if g[2] >= 0})

    all_P, all_N, all_UV, all_JI, all_JW, all_T = [], [], [], [], [], []
    vbase = 0
    parts_out = []

    for gi in used:
        parts = model.geometry_parts(gi)
        # part index buffers are contiguous; sort by ioff to bound each strip
        iptrs = sorted(p["ioff"] for p in parts)
        for part in parts:
            vc = part["vcount"]
            vo = part["voff"]
            io = part["ioff"]
            # decode vertices
            P = np.empty((vc, 3), np.float32)
            N = np.zeros((vc, 3), np.float32)
            UV = np.empty((vc, 2), np.float32)
            JI = np.zeros((vc, 4), np.int32)
            JW = np.zeros((vc, 4), np.float32)
            for i in range(vc):
                o = vo + i * VTX_SIZE
                P[i] = (model.f(o), model.f(o + 4), model.f(o + 8))
                N[i] = henn3(model.u32(o + 0x0C))
                UV[i] = (model.i16(o + 0x18) / 32767.0, model.i16(o + 0x1A) / 32767.0)
                n0 = model.u8(o + 0x1C) // 3
                n1b = model.u8(o + 0x1D)
                w0 = model.u16(o + 0x1E) / 32767.0
                if n1b == 0xFD or w0 >= 0.99999:
                    JI[i] = (n0, 0, 0, 0); JW[i] = (1.0, 0, 0, 0)
                else:
                    JI[i] = (n0, n1b // 3, 0, 0); JW[i] = (w0, 1.0 - w0, 0, 0)

            # index strip: bound by the next part's ioff, trim trailing 0xFFFF
            higher = [p for p in iptrs if p > io]
            end = min(higher) if higher else None
            if end is None:
                # last buffer: read until 0xFFFF pad
                cur = io; idx = []
                while True:
                    v = model.u16(cur)
                    if v == 0xFFFF:
                        break
                    idx.append(v); cur += 2
                strip = np.asarray(idx, np.int64)
            else:
                cnt = (end - io) // 2
                strip = np.frombuffer(model.img, dtype="<u2", count=cnt, offset=io).astype(np.int64)
                # drop trailing 0xFFFF pad
                while len(strip) and strip[-1] == 0xFFFF:
                    strip = strip[:-1]

            tris = []
            for k in range(len(strip) - 2):
                a, b, c = int(strip[k]), int(strip[k + 1]), int(strip[k + 2])
                if a == b or b == c or a == c or a == 0xFFFF or b == 0xFFFF or c == 0xFFFF:
                    continue
                tris.append((a, c, b) if (k & 1) else (a, b, c))
            T = np.asarray(tris, np.int64) + vbase if tris else np.zeros((0, 3), np.int64)

            if debug:
                print(f"  geom {gi} part {part['index']} shader={part['shader']} "
                      f"v={vc} strip={len(strip)} tris={len(T)} "
                      f"z[{P[:,2].min():.3f},{P[:,2].max():.3f}]")

            all_P.append(P); all_N.append(N); all_UV.append(UV)
            all_JI.append(JI); all_JW.append(JW); all_T.append(T)
            parts_out.append({"geom": gi, "part": part["index"], "shader": part["shader"],
                              "vstart": vbase, "vcount": vc, "tris": int(len(T))})
            vbase += vc

    P = np.concatenate(all_P); N = np.concatenate(all_N); UV = np.concatenate(all_UV)
    JI = np.concatenate(all_JI); JW = np.concatenate(all_JW)
    T = np.concatenate(all_T) if any(len(t) for t in all_T) else np.zeros((0, 3), np.int64)
    return {"nodes": nodes, "regions": geoms, "parts": parts_out,
            "P": P, "N": N, "UV": UV, "JI": JI, "JW": JW, "T": T}


def write_obj(path, P, N, UV, T):
    with open(path, "w") as fh:
        fh.write("# Halo CE Mark V (cyborg mode) — native Linux decode\n")
        for p in P:
            fh.write(f"v {p[0]:.5f} {p[1]:.5f} {p[2]:.5f}\n")
        for t in UV:
            fh.write(f"vt {t[0]:.6f} {t[1]:.6f}\n")
        for n in N:
            fh.write(f"vn {n[0]:.5f} {n[1]:.5f} {n[2]:.5f}\n")
        for tri in T:
            a, b, c = tri[0] + 1, tri[1] + 1, tri[2] + 1
            fh.write(f"f {a}/{a}/{a} {b}/{b}/{b} {c}/{c}/{c}\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--map", required=True, help="an Xbox Halo CE .map containing the cyborg model")
    ap.add_argument("--out", required=True)
    ap.add_argument("--tag", default="cyborg\\cyborg")
    ap.add_argument("--debug", action="store_true")
    a = ap.parse_args()

    model = Model(a.map, a.tag)
    print(f"loaded {a.map}  build={model.m.build}  tag={model.tag.path}")
    d = decode(model, debug=a.debug)
    os.makedirs(a.out, exist_ok=True)
    P, N, UV, JI, JW, T = d["P"], d["N"], d["UV"], d["JI"], d["JW"], d["T"]
    print(f"decoded: verts={len(P)} tris={len(T)} bones={len(d['nodes'])} parts={len(d['parts'])}")

    write_obj(os.path.join(a.out, "cyborg.obj"), P, N, UV, T)
    np.savez_compressed(os.path.join(a.out, "cyborg.npz"), P=P, N=N, UV=UV, JI=JI, JW=JW, T=T,
                        node_names=np.array([n["name"] for n in d["nodes"]]),
                        node_parent=np.array([n["parent"] for n in d["nodes"]], np.int32),
                        node_pos=np.array([n["pos"] for n in d["nodes"]], np.float32),
                        node_rot=np.array([n["rot"] for n in d["nodes"]], np.float32))
    with open(os.path.join(a.out, "cyborg.model.json"), "w") as fh:
        json.dump({"nodes": d["nodes"], "regions": d["regions"], "parts": d["parts"],
                   "counts": {"verts": int(len(P)), "tris": int(len(T)), "bones": len(d["nodes"])}},
                  fh, indent=2)
    print(f"wrote {a.out}/cyborg.obj  cyborg.npz  cyborg.model.json")


if __name__ == "__main__":
    main()
