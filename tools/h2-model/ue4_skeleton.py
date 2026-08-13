#!/usr/bin/env python3
"""Decode a cooked UE4.22 USkeleton's FReferenceSkeleton: bone names, parent
indices, and reference (rest) local transforms. Needed as the source rig for
retargeting MCC's pose AnimSequences onto our H2 Mark VI skeleton.

Spec: umodel USkeleton::Serialize (ReferenceSkeleton = RefBoneInfo[] +
RefBonePose[] + NameToIndexMap).  FMeshBoneInfo cooked = FName + int32 parent.
FTransform = FQuat(16) + FVector trans(12) + FVector scale(12).
"""
import struct, sys, json
sys.path.insert(0, ".")
from ue4_asset import Pkg, Props


def decode(stem, debug=False):
    pkg = Pkg(stem + ".uasset", stem + ".uexp")
    e = [x for x in pkg.exports if pkg.export_class(x) == "Skeleton"][0]
    p = Props(pkg, pkg.uexp, e["uexp_off"])
    b = pkg.uexp
    def nm_at(off):
        idx = struct.unpack_from('<i', b, off)[0]
        return pkg.names[idx] if 0 <= idx < len(pkg.names) else None
    # FReferenceSkeleton.RefBoneInfo: locate by signature — count in [16,256],
    # bone0 name resolves and bone0 ParentIndex == -1 (root).
    base = None
    for cand in range(p.end - 8, p.end + 24):
        c = struct.unpack_from('<i', b, cand)[0]
        if not (16 <= c <= 256):
            continue
        if nm_at(cand+4) and struct.unpack_from('<i', b, cand+12)[0] == -1:
            base = cand; break
    assert base is not None, "RefBoneInfo not located"
    o = base
    def i32():
        nonlocal o; v = struct.unpack_from('<i', b, o)[0]; o += 4; return v
    def fname():
        nonlocal o
        idx = struct.unpack_from('<i', b, o)[0]; num = struct.unpack_from('<i', b, o+4)[0]; o += 8
        nm = pkg.names[idx] if 0 <= idx < len(pkg.names) else None
        return nm

    n = i32()                       # RefBoneInfo count
    assert 0 < n < 512, f"bone count {n}"
    names = []; parents = []
    for i in range(n):
        nm = fname(); par = i32()
        names.append(nm); parents.append(par)
    # RefBonePose
    npose = i32()
    assert npose == n, f"pose count {npose} != bones {n}"
    rest = []
    for i in range(n):
        qx, qy, qz, qw = struct.unpack_from('<4f', b, o); o += 16
        tx, ty, tz = struct.unpack_from('<3f', b, o); o += 12
        sx, sy, sz = struct.unpack_from('<3f', b, o); o += 12
        rest.append(dict(quat=(qx, qy, qz, qw), pos=(tx, ty, tz), scale=(sx, sy, sz)))
    if debug:
        print(f"{stem}: bones={n}")
        for i in range(n):
            r = rest[i]; q = r["quat"]; t = r["pos"]
            print(f"  [{i:2d}] parent={parents[i]:3d} {names[i]:16s} "
                  f"t=({t[0]:+7.2f},{t[1]:+7.2f},{t[2]:+7.2f}) q=({q[0]:+.3f},{q[1]:+.3f},{q[2]:+.3f},{q[3]:+.3f})")
    return dict(names=names, parents=parents, rest=rest, n=n)


if __name__ == "__main__":
    import argparse
    ap = argparse.ArgumentParser(); ap.add_argument("stem"); a = ap.parse_args()
    decode(a.stem, debug=True)
