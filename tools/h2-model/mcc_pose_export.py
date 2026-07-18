#!/usr/bin/env python3
"""Export MCC's 11 H2 Spartan customization stances (decoded from the UE4 pak)
into a single poses.json the Blender retargeter consumes.

For each pose we emit, per UE skeleton bone, the settled LOCAL transform
(quaternion + translation). The Blender side does FK on the UE skeleton (rest +
pose) and on our Mark VI rig to transfer each bone's reorientation — see
blender_pose_render.py. Bone names are de-prefixed (b_l_upperarm -> l_upperarm)
to match our rig; UE-only bones (bip01, weapon) are kept for correct UE FK.
"""
import json, os, sys, argparse
sys.path.insert(0, os.path.dirname(__file__) or ".")
import ue4_skeleton, ue4_anim

POSES = {
    "Default": "Default", "AtEase": "AtEase", "Salute": "Salute",
    "CrossedArms": "CrossedArms", "HandsOnHips": "HandsOnHips",
    "DoubleFlex": "DoubleFlex", "Kneel": "Kneel", "LookBack": "LookBack",
    "PeaceSign": "PeaceSign", "Meditation": "Meditation", "YogaPose": "YogaPose",
}


def deprefix(n):
    return n[2:] if n.startswith("b_") else n

# MCC's UE rig is Y-mirrored vs our Blam rig (handedness flip): convert every
# transform into our convention up front so retargeting needs no reflection.
def refl_q(q): x, y, z, w = q; return [-x, y, -z, w]   # Yr-conjugation of a rotation
def refl_t(t): x, y, z = t; return [x, -y, z]          # negate Y coordinate


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--mcc", default="out/mcc/h2", help="dir with Skeleton.* + pose .uasset/.uexp")
    ap.add_argument("--out", default="out/mcc/poses.json")
    a = ap.parse_args()
    os.chdir(a.mcc) if False else None
    here = a.mcc

    sk = ue4_skeleton.decode(os.path.join(here, "Skeleton"))
    bones = [{"name": deprefix(sk["names"][i]), "raw": sk["names"][i],
              "parent": int(sk["parents"][i]),
              "rest_q": refl_q(sk["rest"][i]["quat"]),     # x,y,z,w (Y-mirrored)
              "rest_t": refl_t(sk["rest"][i]["pos"])}      # cm (Y-mirrored)
             for i in range(sk["n"])]
    name_to_idx = {b["raw"]: i for i, b in enumerate(bones)}

    out = {"ue_bones": bones, "poses": {}}
    for label, stem in POSES.items():
        d = ue4_anim.decode(os.path.join(here, stem))
        # track -> bone index (t2s); fill per-bone local pose (default = rest)
        local_q = {i: refl_q(sk["rest"][i]["quat"]) for i in range(sk["n"])}
        local_t = {i: refl_t(sk["rest"][i]["pos"]) for i in range(sk["n"])}
        for t in d["tracks"]:
            bi = t["bone"]
            if 0 <= bi < sk["n"]:
                local_q[bi] = refl_q(t["quat"]); local_t[bi] = refl_t(t["pos"])
        out["poses"][label] = {
            "num_frames": d["num_frames"],
            "local_q": [local_q[i] for i in range(sk["n"])],
            "local_t": [local_t[i] for i in range(sk["n"])],
        }
        print(f"  {label}: {len(d['tracks'])} tracks, {d['num_frames']} frames")

    op = os.path.join(here, "..", "poses.json") if False else a.out
    os.makedirs(os.path.dirname(op), exist_ok=True)
    json.dump(out, open(op, "w"), indent=1)
    print(f"wrote {op}  ({sk['n']} UE bones, {len(out['poses'])} poses)")


if __name__ == "__main__":
    main()
