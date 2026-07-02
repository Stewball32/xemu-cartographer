#!/usr/bin/env python3
"""Add MCC's 10 ARMED (rifle) H2 Spartan customization stances to poses.json.

The base pipeline (mcc_pose_export.py) covers the 11 UNARMED stances. MCC's
armory also offers 10 rifle stances where the Spartan holds a weapon; those are
the ones not yet wired. This extracts each Rifle_* AnimSequence straight from the
pak (ue4_pak), decodes the settled pose (ue4_anim), applies the same Y-mirror
convention as mcc_pose_export, and MERGES the result into the existing
out/mcc/poses.json without touching the 11 unarmed poses already there.

The weapon bone's settled local transform is carried through (it's one of the 35
UE bones), so the Blender side can attach the battle-rifle prop to the posed hand.

Run:
  python3 mcc_rifle_poses.py \
      --pak "$MCC/MCC/Content/Paks/MCC-WindowsNoEditor.pak" \
      --h2 out/mcc/h2 --poses out/mcc/poses.json
"""
import json, os, sys, argparse
sys.path.insert(0, os.path.dirname(__file__) or ".")
import ue4_pak, ue4_anim

# label (PascalCase, matches poses.json + render filenames) -> pak anim asset stem
RIFLE = {
    "RifleAtSide":    "A_H2_Rifle_At_side",
    "RifleBack":      "A_H2_Rifle_Back",
    "RifleBuff":      "A_H2_Rifle_Buff",
    "RifleBuffBack":  "A_H2_Rifle_Buff_Back",
    "RifleHeroPose":  "A_H2_Rifle_Hero_Pose",
    "RifleKneel":     "A_H2_Rifle_Kneel",
    "RifleOnShoulder":"A_H2_Rifle_On_Shoulder",
    "RifleReady":     "A_H2_Rifle_Ready",
    "RifleSalute":    "A_H2_Rifle_Salute",
    "RifleSit":       "A_H2_Rifle_Sit",
}
ANIM_DIR = "MCC/Content/UI/Features/Customization/Characters/H2/Spartan/Animations"

# same handedness flip mcc_pose_export applies (UE rig Y-mirrored vs our Blam rig)
def refl_q(q): x, y, z, w = q; return [-x, y, -z, w]
def refl_t(t): x, y, z = t; return [x, -y, z]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--pak", required=True)
    ap.add_argument("--h2", default="out/mcc/h2", help="dir to drop extracted .uasset/.uexp")
    ap.add_argument("--poses", default="out/mcc/poses.json")
    a = ap.parse_args()

    pk = ue4_pak.Pak(a.pak)
    os.makedirs(a.h2, exist_ok=True)
    data = json.load(open(a.poses))
    n = len(data["ue_bones"])
    rest_q = [b["rest_q"] for b in data["ue_bones"]]
    rest_t = [b["rest_t"] for b in data["ue_bones"]]

    for label, stem in RIFLE.items():
        for ext in ("uasset", "uexp"):
            name = f"{ANIM_DIR}/{stem}.{ext}"
            open(os.path.join(a.h2, f"{label}.{ext}"), "wb").write(pk.read(name))
        d = ue4_anim.decode(os.path.join(a.h2, label))
        local_q = [list(q) for q in rest_q]
        local_t = [list(t) for t in rest_t]
        for t in d["tracks"]:
            bi = t["bone"]
            if 0 <= bi < n:
                local_q[bi] = refl_q(t["quat"]); local_t[bi] = refl_t(t["pos"])
        data["poses"][label] = {"num_frames": d["num_frames"],
                                "local_q": local_q, "local_t": local_t}
        print(f"  {label}: {len(d['tracks'])} tracks, {d['num_frames']} frames, badQuats={d['bad_quats']}")

    json.dump(data, open(a.poses, "w"), indent=1)
    print(f"merged {len(RIFLE)} rifle poses -> {a.poses}  (now {len(data['poses'])} poses)")


if __name__ == "__main__":
    main()
