#!/usr/bin/env python3
"""Blender (headless, Linux) assembler + poser + renderer for the natively
decoded Halo 2 Mark VI.

Consumes `masterchief.npz` (from h2_render_model.py) + the decoded textures,
builds a real mesh + node armature + skin weights, exports a standard `.glb`,
then renders — orthographic, transparent background — one set of passes per pose:

    <pose>.png     diffuse-lit base
    <pose>_p.png   primary-armor coverage   (cc.R, screen space)
    <pose>_s.png   secondary-armor coverage (cc.G, screen space)

These feed the appearance-studio compositor exactly like the emblem _p/_s masks:
fill primary coverage with armor colour 1, secondary with colour 2, over the
base, then drop the emblem decal on the chest.

Run:  blender -b -P blender_build_render.py -- --npz out/masterchief.npz \
        --tex out/tex --out out/render --glb out/masterchief.glb
"""
import bpy, sys, os, json, math
import numpy as np
from mathutils import Vector, Quaternion, Matrix

# ---- args after "--" ----------------------------------------------------
argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
def arg(name, default=None):
    return argv[argv.index(name) + 1] if name in argv else default
NPZ = arg("--npz"); TEX = arg("--tex"); OUT = arg("--out"); GLB = arg("--glb")
RES = int(arg("--res", "768"))
os.makedirs(OUT, exist_ok=True)

# ---- load decoded data --------------------------------------------------
d = np.load(NPZ, allow_pickle=False)
P, N, UV, JI, JW, T = d["P"], d["N"], d["UV"], d["JI"], d["JW"], d["T"]
names = [str(x) for x in d["node_names"]]
parent = d["node_parent"]; npos = d["node_pos"]; nrot = d["node_rot"]
print(f"[build] verts={len(P)} tris={len(T)} bones={len(names)}")

# ---- clean scene --------------------------------------------------------
bpy.ops.wm.read_factory_settings(use_empty=True)
scene = bpy.context.scene

# ---- mesh ---------------------------------------------------------------
mesh = bpy.data.meshes.new("MarkVI")
obj = bpy.data.objects.new("MarkVI", mesh)
scene.collection.objects.link(obj)
mesh.from_pydata([tuple(p) for p in P.tolist()], [], [tuple(t) for t in T.tolist()])
mesh.update()

# UVs (flip V for image convention)
uvl = mesh.uv_layers.new(name="UVMap")
for poly in mesh.polygons:
    for li in poly.loop_indices:
        vi = mesh.loops[li].vertex_index
        u, v = UV[vi]
        uvl.data[li].uv = (float(u), 1.0 - float(v))

# normals
try:
    mesh.normals_split_custom_set_from_vertices([tuple(n) for n in N.tolist()])
except Exception as e:
    print("[build] custom normals failed, smoothing:", e)
    for poly in mesh.polygons:
        poly.use_smooth = True

# ---- armature -----------------------------------------------------------
arm = bpy.data.armatures.new("rig")
arm_obj = bpy.data.objects.new("rig", arm)
scene.collection.objects.link(arm_obj)

# compose world matrices from local (translation + quaternion)
world = [None] * len(names)
def local_mat(i):
    x, y, z, w = nrot[i]
    q = Quaternion((w, x, y, z))
    return Matrix.Translation(Vector(npos[i])) @ q.to_matrix().to_4x4()
order = sorted(range(len(names)), key=lambda i: 0)  # parents always precede children in H2
for i in range(len(names)):
    L = local_mat(i)
    p = parent[i]
    world[i] = (world[p] @ L) if (0 <= p < len(names) and world[p] is not None) else L

bpy.context.view_layer.objects.active = arm_obj
bpy.ops.object.mode_set(mode="EDIT")
# heads sit exactly on the joints (so the bind matches the bind-pose mesh);
# tails point at the first child so each bone's +Y runs down the limb -> poses
# are intuitive (bend a joint by rotating about its local X).
heads = [world[i].translation.copy() for i in range(len(names))]
kids = {i: [j for j in range(len(names)) if parent[j] == i] for i in range(len(names))}
ebs = {}
for i, nm in enumerate(names):
    eb = arm.edit_bones.new(nm)
    h = heads[i]
    if kids[i]:
        t = heads[kids[i][0]]
        if (t - h).length < 1e-5:
            t = h + Vector((0, 0, 0.04))
    else:
        p = parent[i]
        dv = (h - heads[p]) if 0 <= p < len(names) else Vector((0, 0, 1))
        dv = dv.normalized() if dv.length > 1e-5 else Vector((0, 0, 1))
        t = h + dv * 0.04
    eb.head = h; eb.tail = t
    ebs[i] = eb
for i in range(len(names)):
    p = parent[i]
    if 0 <= p < len(names):
        ebs[i].parent = ebs[p]
bpy.ops.object.mode_set(mode="OBJECT")

# ---- skin: vertex groups + armature modifier ----------------------------
vgs = {nm: obj.vertex_groups.new(name=nm) for nm in names}
for vi in range(len(P)):
    for k in range(4):
        w = float(JW[vi, k])
        if w > 0.0:
            vgs[names[int(JI[vi, k])]].add([vi], w, "REPLACE")
mod = obj.modifiers.new("Armature", "ARMATURE")
mod.object = arm_obj
obj.parent = arm_obj

# ---- materials ----------------------------------------------------------
def img(path):
    return bpy.data.images.load(os.path.abspath(path)) if path and os.path.exists(path) else None
diff_img = img(os.path.join(TEX, "masterchief.png"))
cc_img = img(os.path.join(TEX, "masterchief_cc.png"))

def mat_diffuse():
    m = bpy.data.materials.new("diffuse"); m.use_nodes = True
    nt = m.node_tree; nt.nodes.clear()
    out = nt.nodes.new("ShaderNodeOutputMaterial")
    bsdf = nt.nodes.new("ShaderNodeBsdfPrincipled")
    bsdf.inputs["Roughness"].default_value = 0.55
    if "Metallic" in bsdf.inputs: bsdf.inputs["Metallic"].default_value = 0.15
    if diff_img:
        tex = nt.nodes.new("ShaderNodeTexImage"); tex.image = diff_img
        nt.links.new(tex.outputs["Color"], bsdf.inputs["Base Color"])
    else:
        bsdf.inputs["Base Color"].default_value = (0.6, 0.62, 0.65, 1)
    nt.links.new(bsdf.outputs["BSDF"], out.inputs["Surface"])
    return m

def mat_mask(mask_path):
    """Shadeless emission of a pre-baked opaque coverage mask (white=coverage)."""
    m = bpy.data.materials.new("mask"); m.use_nodes = True
    nt = m.node_tree; nt.nodes.clear()
    out = nt.nodes.new("ShaderNodeOutputMaterial")
    emit = nt.nodes.new("ShaderNodeEmission")
    mi = img(mask_path)
    if mi:
        mi.colorspace_settings.name = "Non-Color"
        tex = nt.nodes.new("ShaderNodeTexImage"); tex.image = mi; tex.interpolation = "Closest"
        nt.links.new(tex.outputs["Color"], emit.inputs["Color"])
    nt.links.new(emit.outputs["Emission"], out.inputs["Surface"])
    return m

M_DIFF = mat_diffuse()
M_P = mat_mask(os.path.join(TEX, "masterchief_cc_p.png"))
M_S = mat_mask(os.path.join(TEX, "masterchief_cc_s.png"))
mesh.materials.append(M_DIFF)

# ---- camera (orthographic front view) -----------------------------------
# world-space bbox of the mesh
deps = bpy.context.evaluated_depsgraph_get()
coords = [obj.matrix_world @ Vector(v.co) for v in mesh.vertices]
mn = Vector((min(c.x for c in coords), min(c.y for c in coords), min(c.z for c in coords)))
mx = Vector((max(c.x for c in coords), max(c.y for c in coords), max(c.z for c in coords)))
center = (mn + mx) / 2
height = (mx.z - mn.z); width = (mx.y - mn.y)
cam_data = bpy.data.cameras.new("cam"); cam_data.type = "ORTHO"
cam_data.ortho_scale = max(height, width) * 1.15
cam = bpy.data.objects.new("cam", cam_data); scene.collection.objects.link(cam)
cam.location = center + Vector((max(height, width) * 3, 0, 0))  # +X side (front)
dirv = center - cam.location
cam.rotation_euler = dirv.to_track_quat("-Z", "Y").to_euler()
scene.camera = cam

# ---- lighting -----------------------------------------------------------
world_d = bpy.data.worlds.new("w"); scene.world = world_d
world_d.use_nodes = True
world_d.node_tree.nodes["Background"].inputs["Strength"].default_value = 0.6
world_d.node_tree.nodes["Background"].inputs["Color"].default_value = (0.5, 0.55, 0.6, 1)
key = bpy.data.lights.new("key", "SUN"); key.energy = 4.0
ko = bpy.data.objects.new("key", key); scene.collection.objects.link(ko)
ko.rotation_euler = (math.radians(55), math.radians(10), math.radians(60))
rim = bpy.data.lights.new("rim", "SUN"); rim.energy = 2.0
ro = bpy.data.objects.new("rim", rim); scene.collection.objects.link(ro)
ro.rotation_euler = (math.radians(120), 0, math.radians(-120))

# ---- render settings ----------------------------------------------------
scene.render.engine = "BLENDER_EEVEE"
scene.render.resolution_x = RES; scene.render.resolution_y = RES
scene.render.film_transparent = True
scene.render.image_settings.file_format = "PNG"
scene.render.image_settings.color_mode = "RGBA"
try: scene.view_settings.view_transform = "Standard"
except Exception: pass

# ---- poses --------------------------------------------------------------
# rotations applied in each bone's local space (degrees, XYZ euler).
POSES = {
    "idle": {},
    "t_pose": {  # arms straight out to the sides (shoulder abduction = local Z)
        "l_upperarm": (0, 0, 88), "r_upperarm": (0, 0, -88),
    },
    "salute": {  # right arm out then elbow folded toward the brow
        "r_upperarm": (0, 0, -95), "r_forearm": (-118, 0, 0), "r_hand": (-15, 0, 0),
    },
    "crouch": {  # knees + hips bent
        "l_thigh": (62, 0, 0), "r_thigh": (62, 0, 0),
        "l_calf": (-118, 0, 0), "r_calf": (-118, 0, 0), "spine": (16, 0, 0),
    },
}

def apply_pose(pose):
    bpy.context.view_layer.objects.active = arm_obj
    bpy.ops.object.mode_set(mode="POSE")
    for pb in arm_obj.pose.bones:
        pb.rotation_mode = "XYZ"
        pb.rotation_euler = (0, 0, 0)
        pb.location = (0, 0, 0)
    for bone, (rx, ry, rz) in pose.items():
        if bone in arm_obj.pose.bones:
            pb = arm_obj.pose.bones[bone]
            pb.rotation_euler = (math.radians(rx), math.radians(ry), math.radians(rz))
    bpy.ops.object.mode_set(mode="OBJECT")
    bpy.context.view_layer.update()

def render_to(path):
    scene.render.filepath = os.path.abspath(path)
    bpy.ops.render.render(write_still=True)

def render_pose(name, pose):
    apply_pose(pose)
    mesh.materials[0] = M_DIFF; render_to(os.path.join(OUT, f"{name}.png"))
    mesh.materials[0] = M_P;    render_to(os.path.join(OUT, f"{name}_p.png"))
    mesh.materials[0] = M_S;    render_to(os.path.join(OUT, f"{name}_s.png"))
    print(f"[render] {name}")

# ---- export glb (bind pose) ---------------------------------------------
if GLB:
    apply_pose({})
    print("[glb] scene objects:", [(o.name, o.type) for o in scene.objects])
    # remove anything that isn't the mesh / armature so the glb is clean
    for o in list(bpy.data.objects):
        if o not in (obj, arm_obj) and o.type in ("MESH",):
            bpy.data.objects.remove(o, do_unlink=True)
    for o in scene.objects:
        o.select_set(o in (obj, arm_obj))
    bpy.context.view_layer.objects.active = obj
    bpy.ops.export_scene.gltf(filepath=os.path.abspath(GLB), export_format="GLB",
                              use_selection=True, export_skins=True,
                              export_apply=False)
    print(f"[glb] {GLB}")

for nm, pose in POSES.items():
    render_pose(nm, pose)

print("[done]")
