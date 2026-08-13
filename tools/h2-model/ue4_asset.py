#!/usr/bin/env python3
"""Minimal UE4 (4.22 / FileVersionUE4=517) package reader — enough to pull the
MCC customization Spartan **skeleton** (bones + reference pose) and **AnimSequence**
pose tracks out of the cooked .uasset/.uexp pairs extracted from the MCC pak.

Scope: FPackageFileSummary (name/import/export maps) + FPropertyTag walk +
USkeleton FReferenceSkeleton + UAnimSequence cooked compressed-data block.
Self-contained (no UE), in the spirit of the rest of tools/h2-model.

IP: personal / LAN use with your own copy of MCC.
"""
import struct, sys, math

# UE4 object-version gates relevant at 517 (UE 4.22)
VER_NAME_HASHES = 504
VER_TEMPLATE_INDEX = 508          # FObjectExport.TemplateIndex
VER_PRELOAD_DEPS = 507            # FObjectExport preload-dependency ints
VER_LOC_ID = 516                 # summary LocalizationId FString
VER_SEARCHABLE_NAMES = 510
VER_STRUCT_GUID_TAG = 441


class Pkg:
    def __init__(self, uasset_path, uexp_path=None):
        self.b = open(uasset_path, "rb").read()
        self.uexp = open(uexp_path, "rb").read() if uexp_path else b""
        self.o = 0
        self._summary()
        self._names()
        self._imports()
        self._exports()

    # --- primitive readers (operate on self.b at self.o) ---
    def i32(self): v = struct.unpack_from("<i", self.b, self.o)[0]; self.o += 4; return v
    def u32(self): v = struct.unpack_from("<I", self.b, self.o)[0]; self.o += 4; return v
    def i64(self): v = struct.unpack_from("<q", self.b, self.o)[0]; self.o += 8; return v
    def u16(self): v = struct.unpack_from("<H", self.b, self.o)[0]; self.o += 2; return v
    def u8(self):  v = self.b[self.o]; self.o += 1; return v

    def fstr(self):
        n = self.i32()
        if n == 0: return ""
        if n < 0:
            n = -n; s = self.b[self.o:self.o + n*2].decode("utf-16-le", "replace"); self.o += n*2
        else:
            s = self.b[self.o:self.o + n].decode("latin1", "replace"); self.o += n
        return s.split("\x00", 1)[0]

    def fname(self):
        idx = self.i32(); num = self.i32()
        nm = self.names[idx] if 0 <= idx < len(self.names) else f"<{idx}>"
        return nm if num == 0 else f"{nm}_{num-1}"

    # --- summary ---
    def _summary(self):
        self.tag = self.u32(); self.legacy = self.i32(); self.ue3 = self.i32()
        self.ue4 = self.i32(); self.lic = self.i32()
        assert self.tag == 0x9E2A83C1
        if self.legacy <= -2:                      # custom versions (optimized: {Guid,ver})
            cvc = self.i32(); self.o += cvc * (16 + 4)
        self.total_header = self.i32()
        self.folder = self.fstr()
        self.pkg_flags = self.u32()
        self.name_count = self.i32(); self.name_off = self.i32()
        # NOTE: LocalizationId FString is NOT present at ue4=517 (added later, 518+).
        # gatherable text (VER_UE4_SERIALIZE_TEXT_IN_PACKAGES=459, always at 517)
        self.gather_count = self.i32(); self.gather_off = self.i32()
        self.export_count = self.i32(); self.export_off = self.i32()
        self.import_count = self.i32(); self.import_off = self.i32()
        self.depends_off = self.i32()

    def _names(self):
        self.names = []
        o = self.name_off
        for _ in range(self.name_count):
            self.o = o
            s = self.fstr()
            if self.ue4 >= VER_NAME_HASHES: self.o += 4   # 2x uint16 hashes
            o = self.o
            self.names.append(s)

    def _imports(self):
        self.imports = []
        self.o = self.import_off
        for _ in range(self.import_count):
            cls_pkg = self.fname(); cls = self.fname()
            outer = self.i32(); name = self.fname()
            self.imports.append(dict(cls_pkg=cls_pkg, cls=cls, outer=outer, name=name))

    def _exports(self):
        self.exports = []
        self.o = self.export_off
        for _ in range(self.export_count):
            cls_idx = self.i32(); super_idx = self.i32()
            tmpl = self.i32() if self.ue4 >= VER_TEMPLATE_INDEX else 0
            outer = self.i32(); name = self.fname(); flags = self.u32()
            ssize = self.i64(); soff = self.i64()
            self.o += 4*3                       # bForcedExport, bNotForClient, bNotForServer
            self.o += 16                        # PackageGuid
            self.o += 4                         # PackageFlags
            self.o += 4                         # bNotAlwaysLoadedForEditorGame
            self.o += 4                         # bIsAsset
            if self.ue4 >= VER_PRELOAD_DEPS: self.o += 4*5
            self.exports.append(dict(cls_idx=cls_idx, name=name, ssize=ssize, soff=soff,
                                     uexp_off=soff - self.total_header))

    def export_class(self, e):
        ci = e["cls_idx"]
        if ci < 0:  # import
            return self.imports[-ci - 1]["name"]
        if ci > 0:
            return self.exports[ci - 1]["name"]
        return "Class"


# --- tagged-property walker over a bytes buffer ---
class Props:
    def __init__(self, pkg, buf, o):
        self.pkg = pkg; self.b = buf; self.o = o; self.tags = []
        self._walk()

    def i32(self): v = struct.unpack_from("<i", self.b, self.o)[0]; self.o += 4; return v
    def i64(self): v = struct.unpack_from("<q", self.b, self.o)[0]; self.o += 8; return v
    def u8(self): v = self.b[self.o]; self.o += 1; return v
    def f32(self): v = struct.unpack_from("<f", self.b, self.o)[0]; self.o += 4; return v
    def fname(self):
        idx = struct.unpack_from("<i", self.b, self.o)[0]; num = struct.unpack_from("<i", self.b, self.o+4)[0]
        self.o += 8
        nm = self.pkg.names[idx] if 0 <= idx < len(self.pkg.names) else f"<{idx}>"
        return nm

    def _walk(self):
        while True:
            name = self.fname()
            if name == "None":
                break
            typ = self.fname()
            size = self.i32(); arr_idx = self.i32()
            inner = None
            if typ == "StructProperty":
                inner = self.fname(); self.o += 16  # struct guid
            elif typ in ("ArrayProperty", "SetProperty"):
                inner = self.fname()
            elif typ in ("ByteProperty", "EnumProperty"):
                inner = self.fname()
            elif typ == "MapProperty":
                inner = (self.fname(), self.fname())
            elif typ == "BoolProperty":
                inner = self.u8()
            has_guid = self.u8()
            if has_guid: self.o += 16
            val_off = self.o
            self.tags.append(dict(name=name, type=typ, size=size, inner=inner, off=val_off))
            if typ == "BoolProperty":
                continue                      # value already consumed (in tag header)
            self.o = val_off + size           # skip value payload
        self.end = self.o


if __name__ == "__main__":
    import argparse, os
    ap = argparse.ArgumentParser()
    ap.add_argument("stem", help="path stem (without extension)")
    a = ap.parse_args()
    pkg = Pkg(a.stem + ".uasset", a.stem + ".uexp")
    print(f"ue4={pkg.ue4} names={pkg.name_count} imports={pkg.import_count} "
          f"exports={pkg.export_count} total_header={pkg.total_header} uasset_len={len(pkg.b)}")
    for e in pkg.exports:
        print(f"  export {e['name']:24s} class={pkg.export_class(e)} "
              f"ssize={e['ssize']} soff={e['soff']} uexp_off={e['uexp_off']}")
    # walk first export's props
    e = pkg.exports[0]
    p = Props(pkg, pkg.uexp, e["uexp_off"])
    print(f"  props (end @uexp {p.end}, export data ends @ {e['uexp_off']+e['ssize']}):")
    for t in p.tags:
        print(f"    {t['name']:28s} {t['type']:16s} size={t['size']} inner={t['inner']} off={t['off']}")
