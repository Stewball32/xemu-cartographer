#!/usr/bin/env python3
"""Native, Linux-only decoder for the Halo 2 **Xbox** `render_model` (`mode`) tag.

Reads the compressed, node-skinned geometry straight out of the stock Xbox cache
maps (`shared.map` + `mainmenu.map`, cache version 8, uncompressed) and emits a
standard mesh plus the node (bone) skeleton and per-vertex skin weights — no
Windows tools, no Wine, no .NET.

This is the model-side complement to `tools/h2-emblems/extract_emblems.py`: the
emblems proved we can parse the Xbox cache + resolve the top-2-bits "raw pointer"
map selector ourselves; the geometry uses the *same* pointer scheme, so the only
new work is the `render_model` tag layout + the packed vertex formats.

Format authority: Gravemind2401's Reclaimer (open source, MIT) — its H2 reader
(`Reclaimer.Blam/Blam/Halo2/render_model.cs` + `Halo2Common.cs`) is the reference
this port follows field-for-field. Mirror copies live in `tools/reclaimer-ref/`.

Outputs (into --out):
  masterchief.obj        quick universal sanity mesh (pos/uv/normal, triangulated)
  masterchief.npz        numpy arrays for the Blender assembler (skin + skeleton)
  masterchief.model.json human-readable manifest (bones, sections, counts)

IP: personal / LAN use with your own copy of the game. Do not redistribute the
extracted assets.
"""
import argparse, json, os, struct, sys
import numpy as np

# ---------------------------------------------------------------------------
# Halo 2 Xbox cache header field offsets (CacheType.Halo2Xbox, fixed 2048-byte
# header). From Reclaimer Reclaimer.Blam/Blam/Halo2/Config/CacheFile.config.cs.
# ---------------------------------------------------------------------------
H_INDEX_ADDR        = 16
H_META_OFFSET       = 20
H_META_SIZE         = 24
H_BUILD             = 288     # null-terminated(32)
H_STRING_COUNT      = 356
H_STRING_TBL_IDX    = 364     # int[] of offsets into the string table
H_STRING_TBL        = 368
H_FILE_TBL          = 708     # base of tag-path table
H_FILE_TBL_IDX      = 716     # int[] of offsets into the tag-path table

TAG_INDEX_HEADER_SIZE = 32

# render_model field offsets (tag meta), all BlockCollection (tag_block) fields
# are 8 bytes on Xbox: [count int32][pointer int32].
RM_BOUNDING_BOXES = 20
RM_REGIONS        = 28
RM_SECTIONS       = 36
RM_NODES          = 72
RM_MARKER_GROUPS  = 88
RM_SHADERS        = 96

# Xbox section resource Type0 selectors (Halo2Common.ReadMesh, Xbox branch).
RTYPE_INDEX   = 32
RTYPE_VERTEX  = 56      # Type1: 0=pos/skin, 1=uv, 2=normals
RTYPE_NODEMAP = 100

SUBMESH_SIZE = 72       # SubmeshDataBlock, MinVersion Halo2Beta


def cstr(buf, off):
    end = buf.index(b"\x00", off)
    return buf[off:end].decode("latin1")


class Cache:
    """A loaded H2 Xbox cache map (its own header + tag index + string tables)."""

    def __init__(self, path, mainmenu_path=None):
        self.path = path
        self.b = open(path, "rb").read()
        b = self.b
        u32 = lambda o: struct.unpack_from("<I", b, o)[0]
        i32 = lambda o: struct.unpack_from("<i", b, o)[0]
        self.u32, self.i32 = u32, i32

        assert b[0:4] == b"daeh", f"not an H2 cache (head magic={b[0:4]!r})"
        self.index_addr = u32(H_INDEX_ADDR)
        self.meta_offset = u32(H_META_OFFSET)
        self.build = cstr(b, H_BUILD)
        self.string_count = u32(H_STRING_COUNT)
        self.string_tbl_idx = u32(H_STRING_TBL_IDX)
        self.string_tbl = u32(H_STRING_TBL)
        self.file_tbl = u32(H_FILE_TBL)
        self.file_tbl_idx = u32(H_FILE_TBL_IDX)

        # tag index header (read at index_addr)
        ia = self.index_addr
        self.tag_class_off = u32(ia + 0)
        self.tag_class_count = u32(ia + 4)
        self.tag_data_off = u32(ia + 8)
        self.tag_count = u32(ia + 24)

        # HeaderAddressTranslator magic: tag class list sits right after the
        # 32-byte index header.
        self.header_magic = self.tag_class_off - (ia + TAG_INDEX_HEADER_SIZE)
        self.tag_data_file = self.tag_data_off - self.header_magic

        # tag-path (file) table -> names keyed by tag index i
        self.file_idx = [i32(self.file_tbl_idx + k * 4) for k in range(self.tag_count)]

        # read tag index entries
        self.items = []  # (class, id, meta_ptr, meta_size)
        for i in range(self.tag_count):
            o = self.tag_data_file + i * 16
            cid = b[o:o + 4][::-1].decode("latin1")
            tid = struct.unpack_from("<h", b, o + 4)[0]
            mp = u32(o + 8)
            ms = u32(o + 12)
            self.items.append((cid, tid, mp, ms))

        # TagAddressTranslator magic (Xbox): from the first tag's meta pointer.
        first_mp = self.items[0][2]
        self.meta_magic = first_mp - (self.index_addr + self.meta_offset)

        # string table (node/region/marker names)
        self.strings = [""] * self.string_count
        for k in range(self.string_count):
            so = i32(self.string_tbl_idx + k * 4)
            if so >= 0:
                self.strings[k] = cstr(b, self.string_tbl + so)

        # external pool for DataPointer resolution
        self.mainmenu = open(mainmenu_path, "rb").read() if mainmenu_path else None
        # 0=local(this map), 1=mainmenu, 2=shared, 3=single_player_shared
        self.pools = {0: self.b, 1: self.mainmenu, 2: self.b, 3: None}

    def meta_file(self, ptr):
        return ptr - self.meta_magic

    def tag_name(self, i):
        fi = self.file_idx[i]
        return cstr(self.b, self.file_tbl + fi) if fi >= 0 else ""

    def stringid(self, sid):
        return self.strings[sid] if 0 <= sid < self.string_count else f"<{sid}>"

    def tag_block(self, file_off):
        """Read an 8-byte H2 tag_block at file_off -> (count, elements_file_off)."""
        count = self.i32(file_off)
        ptr = self.u32(file_off + 4)
        return count, self.meta_file(ptr)

    def find(self, cls, name_suffix):
        for i, (cid, tid, mp, ms) in enumerate(self.items):
            if cid == cls:
                nm = self.tag_name(i)
                if nm.endswith(name_suffix):
                    return i, tid, mp, ms, nm
        return None


# ---------------------------------------------------------------------------
# packed-vector decoders (Reclaimer.Core PackedVectorHelper)
# ---------------------------------------------------------------------------
def unpack_henn3(u):
    """HenDN3: 11/11/10-bit sign-extended normalised normal packed in a uint32."""
    def sx(v, bits):
        s = 1 << (bits - 1)
        return (v - (1 << bits)) if (v & s) else v
    x = sx(u & 0x7FF, 11) / 1023.0
    y = sx((u >> 11) & 0x7FF, 11) / 1023.0
    z = sx((u >> 22) & 0x3FF, 10) / 511.0
    return x, y, z


class RealBounds:
    __slots__ = ("min", "max")

    def __init__(self, mn, mx):
        self.min, self.max = mn, mx

    def lerp(self, t):
        return self.min + (self.max - self.min) * t


def decode_render_model(cache: Cache, tag_index: int, lod: int = 0, debug=False):
    b = cache.b
    f = struct.Struct("<f").unpack_from
    i16 = lambda o: struct.unpack_from("<h", b, o)[0]
    u16 = lambda o: struct.unpack_from("<H", b, o)[0]
    i32 = cache.i32
    u32 = cache.u32

    meta = cache.meta_file(cache.items[tag_index][2])

    # --- bounding boxes (compression bounds) ---
    n_bb, bb_off = cache.tag_block(meta + RM_BOUNDING_BOXES)
    assert n_bb >= 1, "render_model has no bounding box (need compression bounds)"
    xb = RealBounds(f(b, bb_off + 0)[0],  f(b, bb_off + 4)[0])
    yb = RealBounds(f(b, bb_off + 8)[0],  f(b, bb_off + 12)[0])
    zb = RealBounds(f(b, bb_off + 16)[0], f(b, bb_off + 20)[0])
    ub = RealBounds(f(b, bb_off + 24)[0], f(b, bb_off + 28)[0])
    vb = RealBounds(f(b, bb_off + 32)[0], f(b, bb_off + 36)[0])

    # --- nodes (skeleton) ---
    n_nodes, nd_off = cache.tag_block(meta + RM_NODES)
    nodes = []
    for k in range(n_nodes):
        o = nd_off + k * 96
        name = cache.stringid(u16(o + 0))
        parent = i16(o + 4)
        pos = (f(b, o + 12)[0], f(b, o + 16)[0], f(b, o + 20)[0])
        rot = (f(b, o + 24)[0], f(b, o + 28)[0], f(b, o + 32)[0], f(b, o + 36)[0])
        nodes.append({"name": name, "parent": parent, "pos": pos, "rot": rot})

    # --- regions / permutations (to pick LOD sections actually used) ---
    n_reg, rg_off = cache.tag_block(meta + RM_REGIONS)
    regions = []
    used_sections = set()
    LOD = [14, 12, 10, 8, 6, 4]  # SuperHigh..Potato section-index field offsets
    for r in range(n_reg):
        ro = rg_off + r * 16
        rname = cache.stringid(u16(ro + 0))
        n_perm, p_off = cache.tag_block(ro + 8)
        perms = []
        for p in range(n_perm):
            po = p_off + p * 16
            pname = cache.stringid(u16(po + 0))
            lod_secs = [i16(po + off) for off in LOD]
            sec = lod_secs[lod]
            if sec < 0:  # fall back to the highest available LOD
                sec = next((s for s in lod_secs if s >= 0), -1)
            if sec >= 0:
                used_sections.add(sec)
            perms.append({"name": pname, "section": sec})
        regions.append({"name": rname, "permutations": perms})

    # --- sections (geometry) ---
    n_sec, sc_off = cache.tag_block(meta + RM_SECTIONS)
    sections = []
    for s in range(n_sec):
        o = sc_off + s * 92
        geom_class = i16(o + 0)
        vcount = u16(o + 4)
        fcount = u16(o + 6)
        nodes_per_vertex = b[o + 20]
        data_ptr = u32(o + 56)
        data_size = i32(o + 60)
        header_size = i32(o + 68)
        n_res, res_off = cache.tag_block(o + 72)
        resources = []
        for ri in range(n_res):
            rio = res_off + ri * 16
            resources.append({
                "type0": i16(rio + 4), "type1": i16(rio + 6),
                "size": i32(rio + 8), "offset": i32(rio + 12),
            })
        sections.append({
            "index": s, "geom_class": geom_class, "vcount": vcount, "fcount": fcount,
            "nodes_per_vertex": nodes_per_vertex, "data_ptr": data_ptr,
            "data_size": data_size, "header_size": header_size, "resources": resources,
        })

    # ---------------------------------------------------------------------
    # decode each used section's mesh data
    # ---------------------------------------------------------------------
    all_pos, all_nrm, all_uv = [], [], []
    all_ji, all_jw = [], []          # joint indices (4), joint weights (4)
    all_tris = []
    vbase = 0
    section_out = []

    sec_indices = sorted(used_sections) if used_sections else range(n_sec)
    for s in sec_indices:
        sec = sections[s]
        if sec["vcount"] == 0:
            continue
        loc = (sec["data_ptr"] >> 30) & 3
        addr = sec["data_ptr"] & 0x3FFFFFFF
        pool = cache.pools.get(loc)
        if pool is None:
            print(f"  [section {s}] SKIP: data in pool {loc} (not available)")
            continue
        data = pool[addr: addr + sec["data_size"]]
        base = sec["data_size"] - sec["header_size"] - 4

        du16 = lambda o: struct.unpack_from("<H", data, o)[0]
        di16 = lambda o: struct.unpack_from("<h", data, o)[0]
        du32 = lambda o: struct.unpack_from("<I", data, o)[0]

        index_count = du16(40)       # MeshResourceDetailsBlock.IndexCount @40 (Xbox)
        nodemap_count = du16(108)    # .NodeMapCount @108 (Xbox)

        res = sec["resources"]
        def first(t0, t1=None):
            for r in res:
                if r["type0"] == t0 and (t1 is None or r["type1"] == t1):
                    return r
            return None
        submesh_res = res[0] if res else None
        index_res = first(RTYPE_INDEX)
        vertex_res = first(RTYPE_VERTEX, 0)
        uv_res = first(RTYPE_VERTEX, 1)
        normal_res = first(RTYPE_VERTEX, 2)
        nodemap_res = first(RTYPE_NODEMAP)
        if vertex_res is None or index_res is None:
            print(f"  [section {s}] SKIP: missing vertex/index resource")
            continue

        vcount = sec["vcount"]
        vstride = vertex_res["size"] // vcount

        # submeshes (segments)
        submeshes = []
        if submesh_res:
            cnt = submesh_res["size"] // SUBMESH_SIZE
            for m in range(cnt):
                mo = base + submesh_res["offset"] + m * SUBMESH_SIZE
                submeshes.append({
                    "shader": di16(mo + 4),
                    "index_start": du16(mo + 6),
                    "index_length": du16(mo + 8),
                })

        # index buffer
        ib_off = base + index_res["offset"]
        indices = np.frombuffer(data, dtype="<u2", count=index_count, offset=ib_off).astype(np.int64)
        is_strip = not (sec["fcount"] * 3 == index_count)

        # positions (3x int16 -> +32768 -> /65535 -> lerp bounds)
        pos = np.empty((vcount, 3), np.float32)
        for i in range(vcount):
            vo = base + vertex_res["offset"] + i * vstride
            px = (di16(vo + 0) + 32768) / 65535.0
            py = (di16(vo + 2) + 32768) / 65535.0
            pz = (di16(vo + 4) + 32768) / 65535.0
            pos[i] = (xb.lerp(px), yb.lerp(py), zb.lerp(pz))

        # uvs (2x int16 -> +32768 -> /65535 -> lerp uv bounds)
        uv = np.empty((vcount, 2), np.float32)
        uo0 = base + uv_res["offset"]
        for i in range(vcount):
            uo = uo0 + i * 4
            uu = (di16(uo + 0) + 32768) / 65535.0
            uvv = (di16(uo + 2) + 32768) / 65535.0
            uv[i] = (ub.lerp(uu), vb.lerp(uvv))

        # normals (HenDN3 packed in uint32, stride 12)
        nrm = np.zeros((vcount, 3), np.float32)
        if normal_res is not None:
            no0 = base + normal_res["offset"]
            for i in range(vcount):
                nrm[i] = unpack_henn3(du32(no0 + i * 12))

        # node map (local node index -> global node index)
        node_map = None
        if nodemap_res is not None:
            nmo = base + nodemap_res["offset"]
            node_map = list(data[nmo: nmo + nodemap_count])

        # skinning
        npv = sec["nodes_per_vertex"]
        ji = np.zeros((vcount, 4), np.int32)
        jw = np.zeros((vcount, 4), np.float32)
        if sec["geom_class"] == 1:  # Rigid -> whole section bound to one node
            bone = 0
            if npv == 1 and node_map:
                bone = node_map[0]
            ji[:, 0] = bone
            jw[:, 0] = 1.0
        else:
            for i in range(vcount):
                so = base + vertex_res["offset"] + i * vstride + 6
                if sec["geom_class"] == 2:  # RigidBoned
                    idx = data[so]
                    nodes_i = [idx, 0, 0, 0]
                    wts = [1.0, 0, 0, 0]
                else:                        # Skinned
                    # Xbox skinned-vertex blend block (Halo2Common.cs,
                    # ReadXboxRenderModelMeshData): optional int16 pad for
                    # npv 2/4, then `npv` node bytes IMMEDIATELY followed by
                    # `npv` weight bytes — i.e. the reader only consumes npv
                    # bytes per field, so weights start at p + npv, NOT a
                    # fixed p + 4. The old hard-coded +4 was only correct for
                    # npv == 4; for npv 2/3 it read the *next* vertex's bytes
                    # as weights (garbage secondary weights → forearm/limb
                    # skinning spikes once a bone rotates away from bind).
                    p = so
                    if npv in (2, 4):
                        p += 2  # skip int16
                    nodes_i = [data[p + k] if npv > k else 0 for k in range(4)]
                    wts = [data[p + npv + k] / 255.0 if npv > k else 0.0 for k in range(4)]
                    if npv == 1 and sum(wts) == 0:
                        wts[0] = 1.0
                if node_map:
                    nodes_i = [node_map[nodes_i[k]] if npv > k else 0 for k in range(4)]
                ji[i] = nodes_i
                jw[i] = wts

        # de-stripify each submesh segment into a triangle list
        tris = []
        segs = submeshes if submeshes else [{"index_start": 0, "index_length": index_count}]
        for seg in segs:
            a0 = seg["index_start"]
            a1 = a0 + seg["index_length"]
            seg_idx = indices[a0:a1]
            if is_strip:
                for k in range(len(seg_idx) - 2):
                    x, y, z = int(seg_idx[k]), int(seg_idx[k + 1]), int(seg_idx[k + 2])
                    if x == y or y == z or x == z:
                        continue
                    tris.append((x, z, y) if (k & 1) else (x, y, z))
            else:
                for k in range(0, len(seg_idx) - 2, 3):
                    tris.append((int(seg_idx[k]), int(seg_idx[k + 1]), int(seg_idx[k + 2])))
        tris = np.asarray(tris, np.int64) + vbase

        if debug:
            print(f"  [section {s}] gc={sec['geom_class']} npv={npv} v={vcount} "
                  f"idx={index_count} strip={is_strip} vstride={vstride} "
                  f"tris={len(tris)} base={base} datasize={sec['data_size']} hdr={sec['header_size']}")
            print(f"      pos x[{pos[:,0].min():.2f},{pos[:,0].max():.2f}] "
                  f"y[{pos[:,1].min():.2f},{pos[:,1].max():.2f}] "
                  f"z[{pos[:,2].min():.2f},{pos[:,2].max():.2f}]  "
                  f"bounds X[{xb.min:.2f},{xb.max:.2f}] Z[{zb.min:.2f},{zb.max:.2f}]")

        all_pos.append(pos); all_nrm.append(nrm); all_uv.append(uv)
        all_ji.append(ji); all_jw.append(jw); all_tris.append(tris)
        section_out.append({"index": s, "vstart": vbase, "vcount": vcount,
                            "tris": int(len(tris)), "geom_class": sec["geom_class"]})
        vbase += vcount

    if not all_pos:
        raise SystemExit("no geometry decoded")

    P = np.concatenate(all_pos); N = np.concatenate(all_nrm)
    UV = np.concatenate(all_uv); JI = np.concatenate(all_ji)
    JW = np.concatenate(all_jw); T = np.concatenate(all_tris)

    return {
        "bounds": {"x": (xb.min, xb.max), "y": (yb.min, yb.max), "z": (zb.min, zb.max),
                   "u": (ub.min, ub.max), "v": (vb.min, vb.max)},
        "nodes": nodes, "regions": regions, "sections": section_out,
        "P": P, "N": N, "UV": UV, "JI": JI, "JW": JW, "T": T,
    }


def write_obj(path, P, N, UV, T):
    with open(path, "w") as fh:
        fh.write("# Halo 2 Mark VI (mode #83) — native Linux decode\n")
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
    ap.add_argument("--maps", required=True, help="dir with Xbox shared.map + mainmenu.map")
    ap.add_argument("--out", required=True)
    ap.add_argument("--tag", default="objects\\characters\\masterchief\\masterchief")
    ap.add_argument("--debug", action="store_true")
    a = ap.parse_args()

    shared = os.path.join(a.maps, "shared.map")
    mainmenu = os.path.join(a.maps, "mainmenu.map")
    cache = Cache(shared, mainmenu)
    print(f"loaded {shared}  build={cache.build}  tags={cache.tag_count}")

    hit = cache.find("mode", a.tag.split("\\")[-1])
    if not hit:
        raise SystemExit(f"could not find mode tag for {a.tag}")
    i, tid, mp, ms, nm = hit
    print(f"mode tag: i={i} id={tid} name={nm} meta_size={ms}")

    m = decode_render_model(cache, i, debug=a.debug)
    os.makedirs(a.out, exist_ok=True)
    P, N, UV, JI, JW, T = m["P"], m["N"], m["UV"], m["JI"], m["JW"], m["T"]
    print(f"decoded: verts={len(P)} tris={len(T)} bones={len(m['nodes'])} "
          f"sections={len(m['sections'])}")

    obj = os.path.join(a.out, "masterchief.obj")
    write_obj(obj, P, N, UV, T)
    npz = os.path.join(a.out, "masterchief.npz")
    np.savez_compressed(npz, P=P, N=N, UV=UV, JI=JI, JW=JW, T=T,
                        node_names=np.array([n["name"] for n in m["nodes"]]),
                        node_parent=np.array([n["parent"] for n in m["nodes"]], np.int32),
                        node_pos=np.array([n["pos"] for n in m["nodes"]], np.float32),
                        node_rot=np.array([n["rot"] for n in m["nodes"]], np.float32))
    with open(os.path.join(a.out, "masterchief.model.json"), "w") as fh:
        json.dump({"bounds": m["bounds"], "nodes": m["nodes"],
                   "regions": m["regions"], "sections": m["sections"],
                   "counts": {"verts": int(len(P)), "tris": int(len(T)),
                              "bones": len(m["nodes"])}}, fh, indent=2)
    print(f"wrote {obj}\n      {npz}\n      {os.path.join(a.out,'masterchief.model.json')}")


if __name__ == "__main__":
    main()
