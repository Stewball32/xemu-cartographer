# MCC Spartan customization poses → H2 Mark VI rig

Replaces the hand-set arm angles in `blender_build_render.py` with MCC's **exact**
post-game Spartan customization stances, retargeted onto our natively-decoded H2
Mark VI rig.

## Where the poses come from (the source)

MCC's "Pose" customization (the post-game / Spartan-ID display stance) is a
**global, cross-game set of 11 stances**, identical across H1/H2/H2A/H3/H4/HR
(`Data/UI/unlockdb.xml`):

> Default, At Ease, Salute, Crossed Arms, Hands on Hips, Double Flex, Kneel,
> Look Back, Peace Sign, Meditation, Yoga

They are **not** in any per-game Blam map. The customization Spartan is rendered
by **MCC's UE4 shell**, and the stances live in
`MCC/Content/Paks/MCC-WindowsNoEditor.pak` as UE4 `AnimSequence` assets under
`…/Customization/Characters/<game>/Spartan/Animations/`. We use the **H2** set
(`A_H2_AtEase`, `A_H2_Salute`, …, `A_H2_Spartan_Idle`) + `SK_H2_Spartan_Skeleton`.
The UE skeleton uses the same Halo bone names (`b_l_upperarm`, …) — it's the Halo
skeleton imported into UE, Y-mirrored (handedness flip).

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
   (UE skeleton + per-bone local pose for each of the 11 stances).
4. **`blender_pose_render.py`** — retargets onto the Mark VI 33-bone rig and
   renders the 3 appearance-studio passes (diffuse / primary / secondary).

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
# extract skeleton + 11 pose anims  (done once into out/mcc/h2/)
python3 mcc_pose_export.py --mcc out/mcc/h2 --out out/mcc/poses.json
blender -b -P blender_pose_render.py -- --npz out/masterchief.npz --tex out/tex \
        --poses out/mcc/poses.json --out out/mcc/render --skip hand,index,ring,thumb
python3 spartan_composite.py --render out/mcc/render --emblems ../../sveltekit/static/emblems \
        --out out/mcc/composite --pose Salute
```

## Wired into the studio

`sveltekit/src/lib/utils/spartan-art.ts` → `SPARTAN_POSES` is now the MCC 11
(`default, at_ease, salute, crossed_arms, hands_on_hips, double_flex, kneel,
look_back, peace_sign, meditation, yoga`) + `SPARTAN_POSE_LABELS`. Each pose's
`<pose>{,_p,_s}.png` is in `sveltekit/static/spartan/`.

## Known limitation

The forearm/arm armor spikes on **extreme arm folds** (Salute, Crossed Arms,
Double Flex, Peace Sign) — an **inherited** artifact of the H2 mesh-skinning
decode (`h2_render_model.py`), **not** the pose data: the original hand-set
`t_pose`/`salute`/`crouch` show the identical spikes, and a NumPy rigid-skin
simulation of the same data is clean. Pose-data decode is exact. Fixing the
spikes belongs to the mesh/skin decoder (re-derive the bind / weld armor seams),
tracked separately. Poses that don't raise the arms (Default-ish, Hands on Hips,
Kneel, Look Back, Meditation, Yoga, At Ease) render cleanly.
