#!/usr/bin/env python3
"""Build the natively-decoded H2 Mark VI, RETARGET MCC's exact customization
stances (poses.json) onto its skeleton, skin in NumPy (native Halo bone frames),
and render the 3 appearance-studio passes (diffuse / primary / secondary) per pose.

Why NumPy skinning: Blender's armature modifier re-orients bones (+Y down-limb
roll) for the bind, which shoots spike artifacts on big limb rotations for this
mesh (the inherited hand-set poses show the same). Linear-blend skinning in the
native Halo node frames is clean, so we pose the verts ourselves and render a
static mesh — no armature.

Retarget (per bone, name-matched; both are the same Halo skeleton, MCC's UE rig
Y-mirrored which poses.json already corrects):
  C   = global convention (UE pelvis world * our pelvis world^-1)
  limb bones (upper/fore-arm, thigh, calf): AIM at the posed child joint (twist-free)
  others: D = C^-1 (Up Ur^-1) C  applied to our rest world orientation
Then forward-kinematics positions, LBS the verts + normals.

Run: blender -b -P blender_pose_render.py -- --npz out/masterchief.npz \
       --tex out/tex --poses out/mcc/poses.json --out out/mcc/render [--only Default,Salute]
"""
import bpy, sys, os, json, math
import numpy as np
from mathutils import Vector, Quaternion, Matrix, Euler

argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
def arg(n, d=None): return argv[argv.index(n) + 1] if n in argv else d
NPZ = arg("--npz"); TEX = arg("--tex"); OUT = arg("--out"); POSES = arg("--poses")
RES = int(arg("--res", "768")); ONLY = arg("--only")
NOGUARD = "--noguard" in argv   # disable the rigid-skin guardrail (verify decode fix alone)
SKIP = [s for s in (arg("--skip", "") or "").split(",") if s]   # bones -> rigid-follow parent
# Optional battle-rifle prop for the ARMED ("Rifle*") stances: a glTF static mesh
# attached to the MCC 'weapon' bone (posed via the l_hand frame it parents under).
RIFLE = arg("--rifle")
RSCALE = float(arg("--rscale", "1.0"))                                  # rifle mesh scale
ROFF = [float(x) for x in (arg("--roff", "0,0,0")).split(",")]         # metres, weapon-bone frame
RROT = [float(x) for x in (arg("--rrot", "0,0,0")).split(",")]         # degrees, weapon-bone frame
os.makedirs(OUT, exist_ok=True)

# ---------- quaternion / matrix helpers (x,y,z,w) ----------
def qmul(a, b):
    ax, ay, az, aw = a; bx, by, bz, bw = b
    return np.array([aw*bx+ax*bw+ay*bz-az*by, aw*by-ax*bz+ay*bw+az*bx,
                     aw*bz+ax*by-ay*bx+az*bw, aw*bw-ax*bx-ay*by-az*bz])
def qconj(q): return np.array([-q[0], -q[1], -q[2], q[3]])
def qnorm(q):
    n = np.linalg.norm(q); return q/n if n else np.array([0, 0, 0, 1.0])
def qrot(q, v):
    x, y, z, w = q; vx, vy, vz = v
    tx = 2*(y*vz-z*vy); ty = 2*(z*vx-x*vz); tz = 2*(x*vy-y*vx)
    return np.array([vx+w*tx+(y*tz-z*ty), vy+w*ty+(z*tx-x*tz), vz+w*tz+(x*ty-y*tx)])
def q2R(q):
    x, y, z, w = qnorm(q)
    return np.array([[1-2*(y*y+z*z), 2*(x*y-z*w), 2*(x*z+y*w)],
                     [2*(x*y+z*w), 1-2*(x*x+z*z), 2*(y*z-x*w)],
                     [2*(x*z-y*w), 2*(y*z+x*w), 1-2*(x*x+y*y)]])
def q_from_to(a, b):                       # shortest quaternion a->b (unit vecs)
    a = a/np.linalg.norm(a); b = b/np.linalg.norm(b); d = np.dot(a, b)
    if d > 0.999999: return np.array([0, 0, 0, 1.0])
    if d < -0.999999:
        ax = np.cross([1, 0, 0], a)
        if np.linalg.norm(ax) < 1e-6: ax = np.cross([0, 1, 0], a)
        ax = ax/np.linalg.norm(ax); return np.array([ax[0], ax[1], ax[2], 0.0])
    c = np.cross(a, b); s = math.sqrt((1+d)*2); return qnorm(np.array([c[0]/s, c[1]/s, c[2]/s, s/2]))

# ---------- load decode ----------
d = np.load(NPZ, allow_pickle=False)
P = d["P"].astype(np.float64); N = d["N"].astype(np.float64); UV = d["UV"]
JI = d["JI"]; JW = d["JW"].astype(np.float64); T = d["T"]
names = [str(x) for x in d["node_names"]]; parent = d["node_parent"]
npos = d["node_pos"].astype(np.float64); nrot = d["node_rot"].astype(np.float64)
nidx = {nm: i for i, nm in enumerate(names)}
pose_data = json.load(open(POSES))

# normalize skin weights (raw decode leaves many != 1)
wsum = JW.sum(axis=1, keepdims=True); zero = wsum[:, 0] <= 1e-6; wsum[zero] = 1.0
JWn = JW / wsum
for vi in np.where(zero)[0]: JWn[vi] = [1, 0, 0, 0]

# our rig rest world (Halo node frames): quaternion + position per bone
Owq = [None]*len(names); Owp = [None]*len(names)
for i in range(len(names)):
    lq = qnorm(nrot[i]); lt = npos[i]; p = int(parent[i])
    if 0 <= p < len(names):
        Owq[i] = qmul(Owq[p], lq); Owp[i] = Owp[p] + qrot(Owq[p], lt)
    else:
        Owq[i] = lq; Owp[i] = lt

# ---------- UE skeleton FK ----------
ue = pose_data["ue_bones"]; ue_par = [b["parent"] for b in ue]; ue_idx = {b["name"]: i for i, b in enumerate(ue)}
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

def pose_world(label):
    pd = pose_data["poses"][label]
    # These customization anims turntable the Spartan, so the captured frame ends
    # at an arbitrary facing — neutralize the root (bip01) rotation so every pose
    # faces forward consistently for the studio.
    lq = [list(q) for q in pd["local_q"]]
    if "bip01" in ue_idx:
        lq[ue_idx["bip01"]] = list(ue[ue_idx["bip01"]]["rest_q"])
    Up_q, Up_p = ue_fk(lq, pd["local_t"])
    C = qmul(Ur_q[ue_idx["pelvis"]], qconj(Owq[nidx["pelvis"]])); Cinv = qconj(C)
    # posed orientation (Halo frame) per bone
    Wq = [None]*len(names)
    for i, nm in enumerate(names):
        ui = ue_idx.get(nm)
        if ui is None or any(s in nm for s in SKIP):
            Wq[i] = None; continue
        if any(a in nm for a in AIM) and kids[i]:
            cnm = names[kids[i][0]]; cui = ue_idx.get(cnm)
            if cui is not None:
                pdir = qrot(Cinv, Up_p[cui]-Up_p[ui])
                rest_dir = Owp[nidx[cnm]] - Owp[i]
                if np.linalg.norm(pdir) > 1e-6 and np.linalg.norm(rest_dir) > 1e-6:
                    Wq[i] = qmul(q_from_to(rest_dir, pdir), Owq[i]); continue
        D = qmul(qmul(Cinv, qmul(Up_q[ui], qconj(Ur_q[ui]))), C)
        Wq[i] = qmul(D, Owq[i])
    # FK positions with posed rotations; skipped bones rigid-follow parent
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
    # weapon-bone transform in render space: it parents under l_hand in the UE rig,
    # so map its posed UE-world through the posed l_hand's UE->Halo frame alignment.
    weap = None
    wi = ue_idx.get("weapon"); lhu = ue_idx.get("l_hand"); lhn = nidx.get("l_hand")
    if wi is not None and lhu is not None and lhn is not None:
        A = qmul(Pq[lhn], qconj(Up_q[lhu]))                 # UE l_hand world -> Halo l_hand world
        wq = qmul(A, Up_q[wi])
        wp = Pp[lhn] + qrot(A, (Up_p[wi] - Up_p[lhu]) * 0.01)   # UE cm -> Halo units
        weap = (wq, wp)
    return Pq, Pp, weap

_dc = {}
def _depth(i):
    if i in _dc: return _dc[i]
    p = int(parent[i]); _dc[i] = 0 if not (0 <= p < len(names)) else _depth(p)+1
    return _dc[i]

def skin(Pq, Pp):
    """Linear-blend skin verts + normals (Halo frames), with a rigid-skin
    guardrail: where LBS diverges far from the single-dominant-bone result
    (candy-wrapper spike), fall back to rigid. Keeps smooth blends elsewhere."""
    out = np.zeros_like(P); outN = np.zeros_like(N); rig = np.zeros_like(P)
    Rs = [q2R(qmul(Pq[b], qconj(Owq[b]))) for b in range(len(names))]
    ts = [Pp[b] - Rs[b] @ Owp[b] for b in range(len(names))]
    dom = JI[np.arange(len(P)), JWn.argmax(axis=1)]
    for b in np.unique(dom):
        sel = dom == b
        rig[sel] = P[sel] @ Rs[b].T + ts[b]
    for k in range(4):
        bi = JI[:, k]; w = JWn[:, k]; m = w > 0
        for b in np.unique(bi[m]):
            sel = m & (bi == b)
            out[sel] += (w[sel, None]) * (P[sel] @ Rs[b].T + ts[b])
            outN[sel] += (w[sel, None]) * (N[sel] @ Rs[b].T)
    # guardrail: blend verts that flew far from their rigid position snap to
    # rigid. With the corrected node-skin weight decode this should catch ~0
    # verts (it was masking the buggy-weight spike); kept as a cheap safety
    # net and disablable via --noguard to prove the decode fix stands alone.
    div = np.linalg.norm(out - rig, axis=1)
    bad = div > 0.04
    if NOGUARD:
        print(f"    [skin] guardrail OFF — would have caught {int(bad.sum())} vert(s)")
    else:
        if bad.sum():
            print(f"    [skin] guardrail caught {int(bad.sum())} vert(s)")
        out[bad] = rig[bad]
    nn = np.linalg.norm(outN, axis=1, keepdims=True); nn[nn == 0] = 1
    return out, outN/nn

# ---------- Blender scene (static mesh, no armature) ----------
bpy.ops.wm.read_factory_settings(use_empty=True)
scene = bpy.context.scene
mesh = bpy.data.meshes.new("MarkVI"); obj = bpy.data.objects.new("MarkVI", mesh)
scene.collection.objects.link(obj)
mesh.from_pydata([tuple(p) for p in P.tolist()], [], [tuple(t) for t in T.tolist()]); mesh.update()
uvl = mesh.uv_layers.new(name="UVMap")
for poly in mesh.polygons:
    for li in poly.loop_indices:
        u, v = UV[mesh.loops[li].vertex_index]; uvl.data[li].uv = (float(u), 1.0-float(v))

def img(path): return bpy.data.images.load(os.path.abspath(path)) if path and os.path.exists(path) else None
diff_img = img(os.path.join(TEX, "masterchief.png"))
def mat_diffuse():
    m = bpy.data.materials.new("diffuse"); m.use_nodes = True; nt = m.node_tree; nt.nodes.clear()
    o = nt.nodes.new("ShaderNodeOutputMaterial"); b = nt.nodes.new("ShaderNodeBsdfPrincipled")
    b.inputs["Roughness"].default_value = 0.55
    if "Metallic" in b.inputs: b.inputs["Metallic"].default_value = 0.15
    if diff_img:
        t = nt.nodes.new("ShaderNodeTexImage"); t.image = diff_img; nt.links.new(t.outputs["Color"], b.inputs["Base Color"])
    else: b.inputs["Base Color"].default_value = (0.6, 0.62, 0.65, 1)
    nt.links.new(b.outputs["BSDF"], o.inputs["Surface"]); return m
def mat_mask(mp):
    m = bpy.data.materials.new("mask"); m.use_nodes = True; nt = m.node_tree; nt.nodes.clear()
    o = nt.nodes.new("ShaderNodeOutputMaterial"); e = nt.nodes.new("ShaderNodeEmission")
    mi = img(mp)
    if mi:
        mi.colorspace_settings.name = "Non-Color"
        t = nt.nodes.new("ShaderNodeTexImage"); t.image = mi; t.interpolation = "Closest"; nt.links.new(t.outputs["Color"], e.inputs["Color"])
    nt.links.new(e.outputs["Emission"], o.inputs["Surface"]); return m
M_DIFF = mat_diffuse(); M_P = mat_mask(os.path.join(TEX, "masterchief_cc_p.png")); M_S = mat_mask(os.path.join(TEX, "masterchief_cc_s.png"))
mesh.materials.append(M_DIFF)

# ---------- optional rifle prop (armed stances) ----------
rifle_obj = None
if RIFLE and os.path.exists(RIFLE):
    before = set(bpy.data.objects)
    bpy.ops.import_scene.gltf(filepath=os.path.abspath(RIFLE))
    imported = [o for o in bpy.data.objects if o not in before and o.type == "MESH"]
    rifle_obj = imported[0] if imported else None
    if rifle_obj:
        # bake the glTF import transform into the mesh so matrix_world drives it cleanly
        rifle_obj.select_set(True); bpy.context.view_layer.objects.active = rifle_obj
        bpy.ops.object.transform_apply(location=True, rotation=True, scale=True)
        # rifle is not tintable armor: its own gunmetal in the diffuse pass, solid
        # black in the mask passes so it occludes armor behind it (0 tint coverage).
        rm_d = bpy.data.materials.new("rifle_d"); rm_d.use_nodes = True
        nt = rm_d.node_tree; nt.nodes.clear()
        o = nt.nodes.new("ShaderNodeOutputMaterial"); b = nt.nodes.new("ShaderNodeBsdfPrincipled")
        b.inputs["Base Color"].default_value = (0.05, 0.05, 0.06, 1)
        b.inputs["Roughness"].default_value = 0.4
        if "Metallic" in b.inputs: b.inputs["Metallic"].default_value = 0.8
        nt.links.new(b.outputs["BSDF"], o.inputs["Surface"])
        rm_k = bpy.data.materials.new("rifle_k"); rm_k.use_nodes = True
        nt = rm_k.node_tree; nt.nodes.clear()
        o = nt.nodes.new("ShaderNodeOutputMaterial"); e = nt.nodes.new("ShaderNodeEmission")
        e.inputs["Color"].default_value = (0, 0, 0, 1)
        nt.links.new(e.outputs["Emission"], o.inputs["Surface"])
        rifle_obj.data.materials.clear(); rifle_obj.data.materials.append(rm_d)
        R_DIFF, R_K = rm_d, rm_k

def place_rifle(weap):
    """Position the rifle from the weapon-bone transform + fixed calibration."""
    if not (rifle_obj and weap):
        if rifle_obj: rifle_obj.hide_render = True
        return
    wq, wp = weap
    Rw = Quaternion((wq[3], wq[0], wq[1], wq[2])).to_matrix().to_4x4()
    Tw = Matrix.Translation(Vector((float(wp[0]), float(wp[1]), float(wp[2]))))
    cal = (Matrix.Translation(Vector(ROFF)) @
           Euler([math.radians(a) for a in RROT]).to_matrix().to_4x4() @
           Matrix.Scale(RSCALE, 4))
    rifle_obj.matrix_world = Tw @ Rw @ cal
    rifle_obj.hide_render = False

wd = bpy.data.worlds.new("w"); scene.world = wd; wd.use_nodes = True
wd.node_tree.nodes["Background"].inputs["Strength"].default_value = 0.6
wd.node_tree.nodes["Background"].inputs["Color"].default_value = (0.5, 0.55, 0.6, 1)
key = bpy.data.lights.new("key", "SUN"); key.energy = 4.0
ko = bpy.data.objects.new("key", key); scene.collection.objects.link(ko); ko.rotation_euler = (math.radians(55), math.radians(10), math.radians(60))
rim = bpy.data.lights.new("rim", "SUN"); rim.energy = 2.0
ro = bpy.data.objects.new("rim", rim); scene.collection.objects.link(ro); ro.rotation_euler = (math.radians(120), 0, math.radians(-120))
scene.render.engine = "BLENDER_EEVEE"; scene.render.resolution_x = RES; scene.render.resolution_y = RES
scene.render.film_transparent = True; scene.render.image_settings.file_format = "PNG"; scene.render.image_settings.color_mode = "RGBA"
try: scene.view_settings.view_transform = "Standard"
except Exception: pass
cam_set = False
def setup_cam():
    coords = [Vector(v.co) for v in mesh.vertices]
    mn = Vector((min(c.x for c in coords), min(c.y for c in coords), min(c.z for c in coords)))
    mx = Vector((max(c.x for c in coords), max(c.y for c in coords), max(c.z for c in coords)))
    ctr = (mn+mx)/2; hh = mx.z-mn.z; ww = mx.y-mn.y
    cd = bpy.data.cameras.new("cam"); cd.type = "ORTHO"; cd.ortho_scale = max(hh, ww)*1.15
    cam = bpy.data.objects.new("cam", cd); scene.collection.objects.link(cam)
    cam.location = ctr + Vector((max(hh, ww)*3, 0, 0))
    cam.rotation_euler = (ctr-cam.location).to_track_quat("-Z", "Y").to_euler()  # Z-up model
    scene.camera = cam

def apply_coords(pcoords, pnorm):
    for i, co in enumerate(pcoords): mesh.vertices[i].co = co
    mesh.update()
    try: mesh.normals_split_custom_set_from_vertices([tuple(n) for n in pnorm.tolist()])
    except Exception:
        for poly in mesh.polygons: poly.use_smooth = True

def render_to(path): scene.render.filepath = os.path.abspath(path); bpy.ops.render.render(write_still=True)

want = ONLY.split(",") if ONLY else list(pose_data["poses"].keys())
for label in want:
    Pq, Pp, weap = pose_world(label); pc, pn = skin(Pq, Pp)
    apply_coords(pc, pn)
    armed = label.startswith("Rifle")
    place_rifle(weap if armed else None)
    if not cam_set: setup_cam(); cam_set = True
    mesh.materials[0] = M_DIFF
    if rifle_obj and armed: rifle_obj.data.materials[0] = R_DIFF
    render_to(os.path.join(OUT, f"{label}.png"))
    mesh.materials[0] = M_P
    if rifle_obj and armed: rifle_obj.data.materials[0] = R_K
    render_to(os.path.join(OUT, f"{label}_p.png"))
    mesh.materials[0] = M_S
    render_to(os.path.join(OUT, f"{label}_s.png"))
    print(f"[render] {label}")
print("[done]")
