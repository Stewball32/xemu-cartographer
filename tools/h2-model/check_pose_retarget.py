#!/usr/bin/env python3
"""Verify the pose retarget against the decoded UE ground truth, per pose.

For every pose in poses.json this FKs (a) the UE skeleton with the anim's local
transforms — the ground truth of what MCC displays — and (b) our Halo rig through
the SAME retarget math as blender_pose_render.py (kept in sync by hand; no bpy so
it runs standalone). It then compares matched-joint world positions (pelvis-
aligned, UE cm -> Halo units) and reports per-pose mean/max error + worst bones,
and renders side-by-side stick figures (UE truth vs retarget) for eyeballing.

Run:  python3 check_pose_retarget.py --npz out/masterchief.npz \
        --poses out/mcc/poses.json --out /tmp/pose_check [--skip hand,index,ring,thumb]
"""
import json, math, os, sys, argparse
import numpy as np

ap = argparse.ArgumentParser()
ap.add_argument("--npz", default="out/masterchief.npz")
ap.add_argument("--poses", default="out/mcc/poses.json")
ap.add_argument("--out", default="/tmp/pose_check")
ap.add_argument("--skip", default="")
a = ap.parse_args()
SKIP = [s for s in a.skip.split(",") if s]
os.makedirs(a.out, exist_ok=True)

# ---- quaternion helpers (x,y,z,w) — identical math to blender_pose_render.py ----
def qmul(q1, q2):
    ax, ay, az, aw = q1; bx, by, bz, bw = q2
    return np.array([aw*bx+ax*bw+ay*bz-az*by, aw*by-ax*bz+ay*bw+az*bx,
                     aw*bz+ax*by-ay*bx+az*bw, aw*bw-ax*bx-ay*by-az*bz])
def qconj(q): return np.array([-q[0], -q[1], -q[2], q[3]])
def qnorm(q):
    n = np.linalg.norm(q); return q/n if n else np.array([0, 0, 0, 1.0])
def qrot(q, v):
    x, y, z, w = q; vx, vy, vz = v
    tx = 2*(y*vz-z*vy); ty = 2*(z*vx-x*vz); tz = 2*(x*vy-y*vx)
    return np.array([vx+w*tx+(y*tz-z*ty), vy+w*ty+(z*tx-x*tz), vz+w*tz+(x*ty-y*tx)])
def q_from_to(u, v):
    u = u/np.linalg.norm(u); v = v/np.linalg.norm(v); d = np.dot(u, v)
    if d > 0.999999: return np.array([0, 0, 0, 1.0])
    if d < -0.999999:
        ax = np.cross([1, 0, 0], u)
        if np.linalg.norm(ax) < 1e-6: ax = np.cross([0, 1, 0], u)
        ax = ax/np.linalg.norm(ax); return np.array([ax[0], ax[1], ax[2], 0.0])
    c = np.cross(u, v); s = math.sqrt((1+d)*2); return qnorm(np.array([c[0]/s, c[1]/s, c[2]/s, s/2]))

# ---- load our rig ----
d = np.load(a.npz, allow_pickle=False)
names = [str(x) for x in d["node_names"]]; parent = d["node_parent"]
npos = d["node_pos"].astype(np.float64); nrot = d["node_rot"].astype(np.float64)
nidx = {nm: i for i, nm in enumerate(names)}
pose_data = json.load(open(a.poses))

Owq = [None]*len(names); Owp = [None]*len(names)
for i in range(len(names)):
    lq = qnorm(nrot[i]); lt = npos[i]; p = int(parent[i])
    if 0 <= p < len(names):
        Owq[i] = qmul(Owq[p], lq); Owp[i] = Owp[p] + qrot(Owq[p], lt)
    else:
        Owq[i] = lq; Owp[i] = lt

# ---- UE skeleton ----
ue = pose_data["ue_bones"]; ue_par = [b["parent"] for b in ue]
ue_idx = {b["name"]: i for i, b in enumerate(ue)}
def ue_fk(lq, lt):
    Wq = [None]*len(ue); Wp = [None]*len(ue)
    for i in range(len(ue)):
        q = qnorm(np.array(lq[i])); t = np.array(lt[i]); p = ue_par[i]
        if 0 <= p < len(ue) and Wq[p] is not None:
            Wq[i] = qmul(Wq[p], q); Wp[i] = Wp[p] + qrot(Wq[p], t)
        else:
            Wq[i] = q; Wp[i] = t
    return Wq, Wp
Ur_q, Ur_p = ue_fk([b["rest_q"] for b in ue], [b["rest_t"] for b in ue])

AIM = ("upperarm", "forearm", "thigh", "calf")
kids = {i: [j for j in range(len(names)) if int(parent[j]) == i] for i in range(len(names))}
_dc = {}
def _depth(i):
    if i in _dc: return _dc[i]
    p = int(parent[i]); _dc[i] = 0 if not (0 <= p < len(names)) else _depth(p)+1
    return _dc[i]

def pose_world(label):
    """Mirror of blender_pose_render.py pose_world (kept in sync)."""
    pd = pose_data["poses"][label]
    lq = [list(q) for q in pd["local_q"]]
    if "bip01" in ue_idx:
        lq[ue_idx["bip01"]] = list(ue[ue_idx["bip01"]]["rest_q"])
    Up_q, Up_p = ue_fk(lq, pd["local_t"])
    # facing correction (gaze rule) — keep in sync with blender_pose_render.py
    def _yaw(ui):
        dq = qmul(Up_q[ui], qconj(Ur_q[ui])); f = qrot(dq, (1, 0, 0))
        return math.atan2(f[1], f[0])
    if "pelvis" in ue_idx and "head" in ue_idx:
        yp, yh = _yaw(ue_idx["pelvis"]), _yaw(ue_idx["head"])
        if abs(yp) > math.radians(15) and abs(yh - yp) < math.radians(15):
            rz = np.array([0, 0, math.sin(-yp/2), math.cos(-yp/2)])
            piv = np.array(Up_p[ue_idx["pelvis"]])
            for i in range(len(ue)):
                Up_q[i] = qmul(rz, Up_q[i])
                Up_p[i] = piv + qrot(rz, np.array(Up_p[i]) - piv)
    C = qmul(Ur_q[ue_idx["pelvis"]], qconj(Owq[nidx["pelvis"]])); Cinv = qconj(C)
    Wq = [None]*len(names)
    for i, nm in enumerate(names):
        ui = ue_idx.get(nm)
        if ui is None or any(s in nm for s in SKIP):
            Wq[i] = None; continue
        if any(x in nm for x in AIM) and kids[i]:
            cnm = names[kids[i][0]]; cui = ue_idx.get(cnm)
            if cui is not None:
                pdir = qrot(Cinv, Up_p[cui]-Up_p[ui])
                rest_dir = Owp[nidx[cnm]] - Owp[i]
                if np.linalg.norm(pdir) > 1e-6 and np.linalg.norm(rest_dir) > 1e-6:
                    Wq[i] = qmul(q_from_to(rest_dir, pdir), Owq[i]); continue
        D = qmul(qmul(Cinv, qmul(Up_q[ui], qconj(Ur_q[ui]))), C)
        Wq[i] = qmul(D, Owq[i])
    Pq = [None]*len(names); Pp = [None]*len(names)
    root_shift = qrot(Cinv, (Up_p[ue_idx["pelvis"]]-Ur_p[ue_idx["pelvis"]])*0.01)
    order = sorted(range(len(names)), key=lambda i: _depth(i))
    for i in order:
        p = int(parent[i]); rq = Wq[i]
        if not (0 <= p < len(names)):
            Pq[i] = rq if rq is not None else Owq[i]; Pp[i] = Owp[i] + root_shift
        else:
            local_off = qrot(qconj(Owq[p]), Owp[i]-Owp[p])
            Pp[i] = Pp[p] + qrot(Pq[p], local_off)
            Pq[i] = rq if rq is not None else qmul(Pq[p], qmul(qconj(Owq[p]), Owq[i]))
    return (Up_q, Up_p), (Pq, Pp), Cinv

# ---- UE truth -> our frame: rotate by Cinv, scale cm->units, align pelvis ----
def ue_truth_positions(Up_p, Cinv, Pp):
    lhs = {}
    up_pelvis = np.array(Up_p[ue_idx["pelvis"]])
    for nm, ui in ue_idx.items():
        v = (np.array(Up_p[ui]) - up_pelvis) * 0.01
        lhs[nm] = qrot(Cinv, v) + Pp[nidx["pelvis"]] if nm in nidx else qrot(Cinv, v)
    return lhs

# ---- stick-figure rendering (orthographic front view: screen x=-Y, y=Z) ----
from PIL import Image, ImageDraw
def draw_skel(dr, joints, bones, colour, off, scale, ctr, W, H):
    def scr(p):
        return (off[0] + (-(p[1]-ctr[1]))*scale + W/2, off[1] + (-(p[2]-ctr[2]))*scale + H/2)
    for a_, b_ in bones:
        if a_ in joints and b_ in joints:
            dr.line([scr(joints[a_]), scr(joints[b_])], fill=colour, width=3)
    for nm, p in joints.items():
        x, y = scr(p); dr.ellipse([x-3, y-3, x+3, y+3], fill=colour)

UE_BONES = [(ue[i]["name"], ue[ue_par[i]]["name"]) for i in range(len(ue)) if 0 <= ue_par[i] < len(ue)]
OUR_BONES = [(names[i], names[int(parent[i])]) for i in range(len(names)) if 0 <= int(parent[i]) < len(names)]

report = []
cellW, cellH = 340, 420
labels = list(pose_data["poses"].keys())
sheet = Image.new("RGB", (cellW*7, cellH*3 + 24), (18, 18, 24))
sd = ImageDraw.Draw(sheet)
sd.text((6, 4), "green = UE ground truth (decoded MCC anim)   red = our retargeted rig  (front view)", fill=(230, 230, 235))
for idx, label in enumerate(labels):
    (Up_q, Up_p), (Pq, Pp), Cinv = pose_world(label)
    truth = ue_truth_positions(Up_p, Cinv, Pp)
    ours = {names[i]: Pp[i] for i in range(len(names))}
    errs = []
    for nm in ours:
        if nm in truth and nm in ue_idx:
            errs.append((np.linalg.norm(ours[nm]-truth[nm]), nm))
    errs.sort(reverse=True)
    mean_e = float(np.mean([e for e, _ in errs])); max_e, max_b = errs[0]
    report.append((label, mean_e, max_e, max_b, [f"{b}:{e:.3f}" for e, b in errs[:4]]))
    x0, y0 = (idx % 7)*cellW, (idx // 7)*cellH + 24
    tile = Image.new("RGB", (cellW, cellH), (28, 28, 34)); td = ImageDraw.Draw(tile)
    allp = list(truth.values()) + list(ours.values())
    ctr = np.mean(allp, axis=0)
    span = max(max(abs(p[1]-ctr[1]) for p in allp), max(abs(p[2]-ctr[2]) for p in allp))
    scale = (min(cellW, cellH)*0.42)/max(span, 1e-6)
    draw_skel(td, truth, UE_BONES, (80, 220, 120), (0, 0), scale, ctr, cellW, cellH)
    draw_skel(td, ours, OUR_BONES, (235, 90, 90), (0, 0), scale, ctr, cellW, cellH)
    td.text((6, 4), label, fill=(255, 255, 130))
    td.text((6, cellH-18), f"mean {mean_e:.3f}  max {max_e:.3f} ({max_b})", fill=(200, 200, 210))
    sheet.paste(tile, (x0, y0))
sheet.save(os.path.join(a.out, "retarget_check.png"))

print(f"{'pose':16s} {'meanErr':>8s} {'maxErr':>8s}  worst bones")
for label, mean_e, max_e, max_b, worst in sorted(report, key=lambda r: -r[2]):
    flag = "  <-- CHECK" if max_e > 0.08 else ""
    print(f"{label:16s} {mean_e:8.3f} {max_e:8.3f}  {', '.join(worst)}{flag}")
print(f"\nwrote {a.out}/retarget_check.png")
