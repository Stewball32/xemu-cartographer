# MCC Spartan customization poses → H2 Mark VI rig

Replaces the hand-set arm angles in `blender_build_render.py` with MCC's **exact**
post-game Spartan customization stances, retargeted onto our natively-decoded H2
Mark VI rig.

## Where the poses come from (the source)

MCC's Spartan customization stances live in the UE4 shell's
`…/Customization/Characters/<game>/Spartan/Animations/` as `AnimSequence` assets.
Enumerating the H2 Spartan set gives the **complete set of 21 stances**:

- **11 UNARMED** — Default, At Ease, Salute, Crossed Arms, Hands on Hips, Double
  Flex, Kneel, Look Back, Peace Sign, Meditation, Yoga.
- **10 ARMED** (hold a battle rifle) — Rifle Ready, Rifle at Side, Rifle on
  Shoulder, Rifle on Back, Rifle Kneel, Rifle Sit, Rifle Salute, Rifle Hero,
  Rifle Buff, Rifle Buff (Back).

(A 12th unarmed asset, `A_H2_Pose_Finger`, is a weapon trigger-finger helper used
by the `BP_*_Pose_Weapon` blueprints, **not** a selectable stance — excluded.)

The customization Spartan is rendered by **MCC's UE4 shell**, and the stances
live in `MCC/Content/Paks/MCC-WindowsNoEditor.pak` as UE4 `AnimSequence` assets.
We use the **H2** set (`A_H2_AtEase`, …, `A_H2_Rifle_Ready`, …) +
`SK_H2_Spartan_Skeleton`. The UE skeleton uses the same Halo bone names
(`b_l_upperarm`, …) — it's the Halo skeleton imported into UE, Y-mirrored
(handedness flip) — plus a `weapon` bone the armed anims drive.

IP: real Halo/MCC asset, for Stewart's personal / LAN use with his own copy.
Don't redistribute the extracted assets.

## Pipeline (all Linux, no UE, no Windows tools)

1. **`ue4_pak.py`** — pure-Python UE4 pak v7 reader (index is unencrypted).
   Lists + extracts the pose `AnimSequence` `.uasset/.uexp` + the skeleton.
2. **`ue4_asset.py`** — UE4.22 package reader (name/import/export maps + tagged
   properties). `ue4_skeleton.py` decodes `FReferenceSkeleton` (35 bones + rest).
   `ue4_anim.py` decodes the cooked `UAnimSequence` compressed bone tracks
   (umodel's `UnAnim4.cpp` as the format spec) → the settled (final-frame) local
   pose per bone. All 11 validate (every rotation a unit quaternion).
3. **`mcc_pose_export.py`** — Y-mirrors UE → our convention, writes `poses.json`
   (UE skeleton + per-bone local pose for each of the 11 **unarmed** stances).
4. **`mcc_rifle_poses.py`** — the same decode for the **10 armed** stances,
   extracting each `A_H2_Rifle_*` straight from the pak and **merging** them into
   `poses.json` (carrying the `weapon` bone's settled transform through). Leaves
   the 11 unarmed poses untouched.
5. **`blender_pose_render.py`** — retargets onto the Mark VI 33-bone rig and
   renders the 3 appearance-studio passes (diffuse / primary / secondary). For
   the armed stances (`--rifle`), it attaches the `SM_H2_BattleRifle` static mesh
   at the anim's `weapon` bone, anchored via the **forearm frame of the hand
   nearest the weapon** (forearms are AIM-retargeted and reliable; the hands
   rigid-follow under `--skip`, and hand-frame anchoring left the rifle 7–26 cm
   off UE truth — the forearm anchor is within ~2.5 cm on every armed pose). The
   rifle renders as its own gunmetal in the diffuse pass and **solid black in the
   mask passes**, so the armor tint never colours it.
6. **Facing correction (gaze rule)** in `pose_world`: some source anims bake the
   whole character yawed off-camera — `A_H2_Spartan_Idle` ('Default') is authored
   −45°. When pelvis AND head are co-rotated (gaze follows the body), the pose is
   squared by the pelvis yaw. When the head compensates back to camera (Crossed
   Arms: body +32°, head ≈0°), it's a designed ¾ display stance and is left alone.

The `SM_H2_BattleRifle` prop geometry is a UE `StaticMesh` (the native decoder
does anims/skeletons, not UE meshes), exported once to glTF with **umodel**
(`-game=ue4.21`). Only the final render PNGs are committed — not the raw mesh.

### Retarget (per bone; both are the same Halo skeleton)

- The UE rig is **Y-mirrored** vs our Blam rig — `poses.json` already corrects
  it (`q→(−x,y,−z,w)`, `pos→(x,−y,z)`), after which torso/legs/head rest poses
  match to 0°.
- Global convention `C` from the pelvis; limb bones (upper/fore-arm, thigh,
  calf) are **aimed** at the posed child joint (twist-free); other bones take the
  UE world deformation onto our rest.
- The customization anims **turntable** the Spartan, so the captured frame ends
  at an arbitrary facing — the root (`bip01`) rotation is **neutralized** so all
  poses face forward.
- **Skinning is done in NumPy** in the native Halo node frames (Blender's
  armature re-orients bones for the bind, which shoots spikes), with a
  rigid-skin guardrail that snaps candy-wrapper outliers.

## Run

```bash
PAK="$HOME/.local/share/Steam/steamapps/common/Halo The Master Chief Collection/MCC/Content/Paks/MCC-WindowsNoEditor.pak"
# unarmed: extract skeleton + 11 pose anims  (done once into out/mcc/h2/)
python3 mcc_pose_export.py --mcc out/mcc/h2 --out out/mcc/poses.json
# armed: extract + decode the 10 rifle anims, merge into poses.json
python3 mcc_rifle_poses.py --pak "$PAK" --h2 out/mcc/h2 --poses out/mcc/poses.json
# rifle prop mesh (once), via umodel:
#   umodel -path="<Paks dir>" -game=ue4.21 -export -gltf \
#     MCC/Content/UI/Features/Customization/Characters/H2/Weapons/SM_H2_BattleRifle
RIFLE=path/to/SM_H2_BattleRifle.gltf
blender -b -P blender_pose_render.py -- --npz out/masterchief.npz --tex out/tex \
        --poses out/mcc/poses.json --out out/mcc/render --skip hand,index,ring,thumb \
        --rifle "$RIFLE" --rscale 1.3
python3 spartan_composite.py --render out/mcc/render --emblems ../../sveltekit/static/emblems \
        --out out/mcc/composite --pose Salute
```

Publish: copy each `out/mcc/render/<Pascal>{,_p,_s}.png` to
`sveltekit/static/spartan/<snake>{,_p,_s}.png` (e.g. `RifleReady` → `rifle_ready`).

## Wired into the studio

`sveltekit/src/lib/utils/spartan-art.ts` → `SPARTAN_POSES` is now the complete
**21** MCC stances (11 unarmed + 10 armed) with `SPARTAN_POSE_LABELS`. The armed
subset is `SPARTAN_ARMED_POSES` / `isArmedPose()` so the selector can group or
badge them. Each pose's `<pose>{,_p,_s}.png` is in `sveltekit/static/spartan/`.

## Verification (all 21 poses, 2026-07-01)

Two standalone checkers gate the pipeline (no bpy; keep their retarget math in
sync with `blender_pose_render.py` by hand):

- **`check_pose_retarget.py`** — FKs the UE ground truth AND our retarget per
  pose, compares matched-joint world positions, renders green/red stick-figure
  overlays. Result: max error ≤ 8 cm on all 21 poses, and every worst bone is a
  fingertip (`--skip`ped in the render anyway) — body bones match to mm.
- **`check_frame_choice.py`** — sweeps every anim's keys at 0/¼/½/¾/1 to confirm
  the baked LAST key is the held stance (max body drift ≤ 5 cm across every
  anim — these are idle-in-stance loops, so frame choice is a non-issue; Double
  Flex's asymmetric flex is genuinely the stance), and measures pelvis/chest/head
  yaw (the input to the facing gaze rule above).

Historical note: the first committed batch of the 11 unarmed renders predated the
`86d8e56` node-skin weight fix (the fix landed code-only) and showed severe
candy-wrapper spikes on folded arms; re-rendering with the fixed decode cleared
them. If renders ever look spiky again, check they were produced by the current
`h2_render_model.py` decode before suspecting the pose data.
