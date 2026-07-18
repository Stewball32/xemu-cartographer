#!/usr/bin/env python3
"""Is the LAST keyframe (what mcc_pose_export/mcc_rifle_poses bake) actually the
held display stance? Sweep each pose's keys at fractions 0/.25/.5/.75/1, FK the
raw UE skeleton (bip01 neutralized, same as the renderer), and report (a) how far
body joints move between mid-anim and the last frame and (b) pelvis yaw vs rest
at the last frame (turntable-on-pelvis tell). Big (a) = the anim doesn't just
hold the stance, so frame choice matters; big (b) = facing drift the bip01
neutralization can't fix.
"""
import os, sys, math
import numpy as np
sys.path.insert(0, os.path.dirname(__file__) or ".")
import ue4_skeleton, ue4_anim

HERE = "out/mcc/h2"
POSES = ["Default","AtEase","Salute","CrossedArms","HandsOnHips","DoubleFlex",
         "Kneel","LookBack","PeaceSign","Meditation","YogaPose",
         "RifleAtSide","RifleBack","RifleBuff","RifleBuffBack","RifleHeroPose",
         "RifleKneel","RifleOnShoulder","RifleReady","RifleSalute","RifleSit"]

sk = ue4_skeleton.decode(os.path.join(HERE, "Skeleton"))
N = sk["n"]; names = sk["names"]; parents = sk["parents"]
rest_q = [sk["rest"][i]["quat"] for i in range(N)]
rest_t = [sk["rest"][i]["pos"] for i in range(N)]
bip = names.index("bip01") if "bip01" in names else -1
pelvis = names.index("b_pelvis") if "b_pelvis" in names else names.index("pelvis")

def qmul(a, b):
    ax, ay, az, aw = a; bx, by, bz, bw = b
    return (aw*bx+ax*bw+ay*bz-az*by, aw*by-ax*bz+ay*bw+az*bx,
            aw*bz+ax*by-ay*bx+az*bw, aw*bw-ax*bx-ay*by-az*bz)
def qrot(q, v):
    x, y, z, w = q; vx, vy, vz = v
    tx = 2*(y*vz-z*vy); ty = 2*(z*vx-x*vz); tz = 2*(x*vy-y*vx)
    return (vx+w*tx+(y*tz-z*ty), vy+w*ty+(z*tx-x*tz), vz+w*tz+(x*ty-y*tx))
def qconj(q): return (-q[0], -q[1], -q[2], q[3])

def fk(lq, lt):
    Wq = [None]*N; Wp = [None]*N
    for i in range(N):
        p = parents[i]
        if 0 <= p < N and Wq[p] is not None:
            Wq[i] = qmul(Wq[p], lq[i]); Wp[i] = tuple(a+b for a, b in zip(Wp[p], qrot(Wq[p], lt[i])))
        else:
            Wq[i] = tuple(lq[i]); Wp[i] = tuple(lt[i])
    return Wq, Wp

def pose_at(tracks, frac):
    lq = [tuple(q) for q in rest_q]; lt = [tuple(t) for t in rest_t]
    for t in tracks:
        bi = t["bone"]
        if not (0 <= bi < N): continue
        qk = t["quat_keys"]; pk = t["pos_keys"]
        if qk: lq[bi] = qk[round(frac*(len(qk)-1))]
        if pk: lt[bi] = pk[round(frac*(len(pk)-1))]
    if bip >= 0: lq[bip] = tuple(rest_q[bip])   # same neutralization as renderer
    return fk(lq, lt)

BODY = [i for i, nm in enumerate(names)
        if not any(s in nm for s in ("index", "ring", "thumb", "weapon", "bip01"))]

print(f"{'pose':16s} {'keyEnc':>6s} {'maxKeys':>7s} | body move vs LAST frame (cm) at f=0/.25/.5/.75 | pelvisYaw(last)")
for label in POSES:
    d = ue4_anim.decode(os.path.join(HERE, label), all_keys=True)
    maxkeys = max(len(t["quat_keys"]) for t in d["tracks"])
    _, Wl = pose_at(d["tracks"], 1.0)
    deltas = []
    for f in (0.0, 0.25, 0.5, 0.75):
        _, Wf = pose_at(d["tracks"], f)
        deltas.append(max(np.linalg.norm(np.subtract(Wf[i], Wl[i])) for i in BODY))
    # pelvis yaw at last frame vs rest (rotation of the pelvis's forward axis about Z)
    Wq_l, _ = pose_at(d["tracks"], 1.0)
    Wq_r, _ = fk([tuple(q) for q in rest_q], [tuple(t) for t in rest_t])
    dq = qmul(Wq_l[pelvis], qconj(Wq_r[pelvis]))
    fwd = qrot(dq, (1, 0, 0))
    yaw = math.degrees(math.atan2(fwd[1], fwd[0]))
    flag = "  <-- moves a lot" if max(deltas) > 15 else ""
    print(f"{label:16s} {d['key_enc']:6d} {maxkeys:7d} | " +
          " ".join(f"{x:6.1f}" for x in deltas) + f" | {yaw:+7.1f} deg{flag}")
