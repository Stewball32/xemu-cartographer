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
   to the `weapon` bone (mapped through the posed `l_hand` frame it parents
   under) — so the rifle follows MCC's own per-pose attach point (across the
   body, on the shoulder, on the back, …). The rifle renders as its own gunmetal
   in the diffuse pass and **solid black in the mask passes**, so the armor tint
   never colours it.

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

## Known limitation

The forearm/arm armor spikes on **extreme arm folds** (Salute, Crossed Arms,
Double Flex, Peace Sign) — an **inherited** artifact of the H2 mesh-skinning
decode (`h2_render_model.py`), **not** the pose data: the original hand-set
`t_pose`/`salute`/`crouch` show the identical spikes, and a NumPy rigid-skin
simulation of the same data is clean. Pose-data decode is exact. Fixing the
spikes belongs to the mesh/skin decoder (re-derive the bind / weld armor seams),
tracked separately. Poses that don't raise the arms (Default-ish, Hands on Hips,
Kneel, Look Back, Meditation, Yoga, At Ease) render cleanly.
