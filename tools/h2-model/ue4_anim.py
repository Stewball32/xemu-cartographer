#!/usr/bin/env python3
"""Decode a cooked UE4.22 UAnimSequence (FileVersionUE4=517) into the settled
(final-frame) per-bone local pose, following umodel's UnAnim4.cpp
SerializeCompressedData (non-per-track path) field-for-field.

We need only the held pose, so per track we take the LAST translation + rotation
key. Output: list of {track, bone, pos(xyz), quat(xyzw)} plus frame count.

Spec authority: gildor2/UEViewer Unreal/UnrealMesh/UnAnim4.cpp.
IP: personal / LAN use with your own copy of MCC.
"""
import struct, math, sys
sys.path.insert(0, ".")
from ue4_asset import Pkg, Props

ACF_None=0; ACF_Float96NoW=1; ACF_Fixed48NoW=2; ACF_IntervalFixed32NoW=3
ACF_Fixed32NoW=4; ACF_Float32NoW=5; ACF_Identity=7
AKF_ConstantKeyLerp=0; AKF_VariableKeyLerp=1; AKF_PerTrackCompression=2


def _w(x, y, z):
    t = 1.0 - (x*x + y*y + z*z)
    return math.sqrt(t) if t > 0 else 0.0


class S:
    def __init__(s, b, o=0): s.b = b; s.o = o
    def i32(s): v = struct.unpack_from('<i', s.b, s.o)[0]; s.o += 4; return v
    def u16(s): v = struct.unpack_from('<H', s.b, s.o)[0]; s.o += 2; return v
    def u32(s): v = struct.unpack_from('<I', s.b, s.o)[0]; s.o += 4; return v
    def f32(s): v = struct.unpack_from('<f', s.b, s.o)[0]; s.o += 4; return v
    def arr(s): n = s.i32(); return [s.i32() for _ in range(n)]


def _quat(sr, fmt, mins=None, ran=None):
    if fmt in (ACF_None,):
        return (sr.f32(), sr.f32(), sr.f32(), sr.f32())
    if fmt == ACF_Float96NoW:
        x = sr.f32(); y = sr.f32(); z = sr.f32(); return (x, y, z, _w(x, y, z))
    if fmt == ACF_Fixed48NoW:
        s = 1.0/32767.0
        x = (sr.u16()-32767)*s; y = (sr.u16()-32767)*s; z = (sr.u16()-32767)*s
        return (x, y, z, _w(x, y, z))
    if fmt == ACF_Fixed32NoW:
        p = sr.u32()
        x = ((p >> 21) & 0x7FF - 0); y = ((p >> 10) & 0x7FF); z = (p & 0x3FF)
        x = (((p >> 21) & 0x7FF)-1023)/1023.0; y = (((p >> 10) & 0x7FF)-1023)/1023.0; z = ((p & 0x3FF)-511)/511.0
        return (x, y, z, _w(x, y, z))
    if fmt == ACF_IntervalFixed32NoW:
        p = sr.u32()
        x = mins[0]+ran[0]*((p & 0x7FF)/2047.0); y = mins[1]+ran[1]*(((p >> 11) & 0x7FF)/2047.0)
        z = mins[2]+ran[2]*(((p >> 22) & 0x3FF)/1023.0)
        return (x, y, z, _w(x, y, z))
    if fmt == ACF_Identity:
        return (0.0, 0.0, 0.0, 1.0)
    raise ValueError(f"rot fmt {fmt}")


def _vec(sr, fmt, mins=None, ran=None):
    if fmt in (ACF_None, ACF_Float96NoW):
        return (sr.f32(), sr.f32(), sr.f32())
    if fmt == ACF_Fixed48NoW:
        s = 1.0/128.0
        return ((sr.u16()-32767)*s, (sr.u16()-32767)*s, (sr.u16()-32767)*s)
    if fmt == ACF_IntervalFixed32NoW:
        p = sr.u32()
        return (mins[0]+ran[0]*((p & 0x7FF)/2047.0), mins[1]+ran[1]*(((p >> 11) & 0x7FF)/2047.0),
                mins[2]+ran[2]*(((p >> 22) & 0x3FF)/1023.0))
    if fmt == ACF_Identity:
        return (0.0, 0.0, 0.0)
    raise ValueError(f"trans fmt {fmt}")


def _keysize_rot(fmt):
    return {ACF_None:16, ACF_Float96NoW:12, ACF_Fixed48NoW:6, ACF_Fixed32NoW:4,
            ACF_Float32NoW:4, ACF_IntervalFixed32NoW:4, ACF_Identity:0}[fmt]

def _keysize_vec(fmt):
    return {ACF_None:12, ACF_Float96NoW:12, ACF_Fixed48NoW:6, ACF_Fixed32NoW:4,
            ACF_Float32NoW:4, ACF_IntervalFixed32NoW:4, ACF_Identity:0}[fmt]


def decode(stem, debug=False):
    pkg = Pkg(stem + ".uasset", stem + ".uexp")
    e = [x for x in pkg.exports if pkg.export_class(x) == "AnimSequence"][0]
    p = Props(pkg, pkg.uexp, e["uexp_off"])
    pr = {t["name"]: t for t in p.tags}
    num_frames = struct.unpack_from('<i', pkg.uexp, pr["NumFrames"]["off"])[0]
    seq_len = struct.unpack_from('<f', pkg.uexp, pr["SequenceLength"]["off"])[0]
    uexp = pkg.uexp

    # Locate SerializeCompressedData: 4 small format bytes + CompressedTrackOffsets,
    # validated by CompressedTrackToSkeletonMapTable count == nTracks.
    found = None
    for start in range(p.end, p.end + 512):
        try:
            r = S(uexp, start)
            ke = uexp[start]; tf = uexp[start+1]; rf = uexp[start+2]; sf = uexp[start+3]
            if ke > 2 or max(tf, rf, sf) > 7:
                continue
            r.o = start + 4
            n = struct.unpack_from('<i', uexp, r.o)[0]
            if not (0 < n < 100000):
                continue
            track_offsets = r.arr()
            opb = 2 if ke == AKF_PerTrackCompression else 4
            if len(track_offsets) % opb:
                continue
            ntr = len(track_offsets) // opb
            scale_off = r.arr(); scale_strip = r.i32()
            segs = r.arr()
            t2s = r.arr()
            if len(t2s) != ntr:
                continue
            found = dict(ke=ke, tf=tf, rf=rf, sf=sf, track_offsets=track_offsets,
                         t2s=t2s, ntr=ntr, after=r.o, opb=opb)
            break
        except Exception:
            continue
    assert found, f"{stem}: SerializeCompressedData not located"
    ke = found["ke"]; tf = found["tf"]; rf = found["rf"]
    TO = found["track_offsets"]; t2s = found["t2s"]; ntr = found["ntr"]
    has_time = (ke == AKF_VariableKeyLerp)
    assert ke != AKF_PerTrackCompression, "per-track not needed for these assets"

    # stream length L (from offsets) -> NumBytes int32 == L marks stream start
    L = 0
    for t in range(ntr):
        to, tk, ro, rk = TO[t*4:t*4+4]
        tfmt = ACF_None if tk == 1 else tf
        rfmt = ACF_Float96NoW if rk == 1 else rf
        tend = to + tk*_keysize_vec(tfmt)
        rend = ro + rk*_keysize_rot(rfmt)
        L = max(L, ((tend+3) & ~3), ((rend+3) & ~3))
    # find NumBytes==L just after the curve-name block
    sstart = None
    for X in range(found["after"], found["after"] + 600):
        if struct.unpack_from('<i', uexp, X)[0] == L:
            sstart = X + 4; break
    assert sstart, f"{stem}: stream start (NumBytes={L}) not found"
    stream = uexp[sstart:sstart + L]

    out = []
    for ti in range(ntr):
        to, tk, ro, rk = TO[ti*4:ti*4+4]
        bone = t2s[ti] if ti < len(t2s) else ti
        # translation: take last key
        pos = (0.0, 0.0, 0.0)
        if tk:
            sr = S(stream, to); fmt = ACF_None if tk == 1 else tf
            mins = ran = None
            if fmt == ACF_IntervalFixed32NoW:
                mins = (sr.f32(), sr.f32(), sr.f32()); ran = (sr.f32(), sr.f32(), sr.f32())
            for k in range(tk):
                pos = _vec(sr, fmt, mins, ran)
        # rotation: take last key
        sr = S(stream, ro); fmt = ACF_Float96NoW if rk == 1 else rf
        mins = ran = None
        if rk > 1 and fmt == ACF_IntervalFixed32NoW:
            mins = (sr.f32(), sr.f32(), sr.f32()); ran = (sr.f32(), sr.f32(), sr.f32())
        quat = (0.0, 0.0, 0.0, 1.0)
        for k in range(rk):
            quat = _quat(sr, fmt, mins, ran)
        out.append(dict(track=ti, bone=bone, pos=pos, quat=quat))

    # validate: all unit-ish quats
    bad = sum(1 for t in out if abs(t["quat"][0]**2+t["quat"][1]**2+t["quat"][2]**2+t["quat"][3]**2 - 1) > 0.05)
    if debug:
        print(f"  {stem}: frames={num_frames} seqlen={seq_len} keyEnc={ke} trans={tf} rot={rf} "
              f"tracks={ntr} L={L} streamStart={sstart} badQuats={bad}")
    return dict(num_frames=num_frames, seq_len=seq_len, tracks=out, ntracks=ntr,
                key_enc=ke, trans_fmt=tf, rot_fmt=rf, bad_quats=bad)


if __name__ == "__main__":
    import argparse
    ap = argparse.ArgumentParser(); ap.add_argument("stems", nargs="+"); a = ap.parse_args()
    for stem in a.stems:
        d = decode(stem, debug=True)
        t = d["tracks"][2]
        q = t["quat"]
        print(f"      sample bone {t['bone']}: q=({q[0]:+.3f},{q[1]:+.3f},{q[2]:+.3f},{q[3]:+.3f})")
