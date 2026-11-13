"""Prepare the retained VARCO finger rig for the shared offline pet runtime."""
import bpy
import json
import math
import argparse
import sys
from pathlib import Path
from mathutils import Quaternion

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'assets/desktop-pets'))
from model_export import export_model, render_thumbnail
catalog = json.loads((ROOT / 'assets/desktop-pets/catalog.json').read_text(encoding='utf-8'))
parser = argparse.ArgumentParser()
parser.add_argument('--pet', choices=[key for key, value in catalog.items() if value['kind'] == 'humanoid' and key != 'punk'], default='zombert')
args = parser.parse_args(sys.argv[sys.argv.index('--')+1:] if '--' in sys.argv else [])
PET = args.pet
SOURCE = ROOT / f'assets/desktop-pets/{PET}/varco-rigged.glb'
bpy.ops.wm.read_factory_settings(use_empty=True)
bpy.ops.import_scene.gltf(filepath=str(SOURCE))
rig = next(o for o in bpy.context.scene.objects if o.type == 'ARMATURE')
mesh = next(o for o in bpy.context.scene.objects if o.type == 'MESH' and any(m.type == 'ARMATURE' for m in o.modifiers))
assert len(rig.data.bones) == 53
for obj in list(bpy.context.scene.objects):
    if obj not in (mesh, rig):
        bpy.data.objects.remove(obj, do_unlink=True)

# Bake the slightly lowered arms into the T-bind expected by Mesh2Motion.
for side, sign in [('Left', 1), ('Right', -1)]:
    for part, child in [('Arm', 'ForeArm'), ('ForeArm', 'Hand')]:
        bone = rig.pose.bones[side + part]
        direction = rig.pose.bones[side + child].head - bone.head
        angle = sign * math.atan2(direction.z, abs(direction.x))
        rest = bone.bone.matrix_local.to_quaternion()
        bone.rotation_mode = 'QUATERNION'
        bone.rotation_quaternion = rest.inverted() @ Quaternion((0, 1, 0), angle) @ rest
        bpy.context.view_layer.update()
bpy.context.view_layer.objects.active = mesh
for modifier in list(mesh.modifiers):
    if modifier.type == 'ARMATURE':
        bpy.ops.object.modifier_apply(modifier=modifier.name)
bpy.context.view_layer.objects.active = rig
bpy.ops.object.mode_set(mode='POSE')
bpy.ops.pose.armature_apply(selected=False)
bpy.ops.object.mode_set(mode='OBJECT')
mesh.modifiers.new('Armature', 'ARMATURE').object = rig

names = {'Hips': 'DEF-spine', 'Spine': 'DEF-spine.001', 'Spine1': 'DEF-spine.002', 'Spine2': 'DEF-spine.003', 'Neck': 'DEF-spine.004', 'Head': 'DEF-spine.006'}
for side, suffix in [('Left', 'L'), ('Right', 'R')]:
    for source, target in [('Shoulder', 'shoulder'), ('Arm', 'upper_arm'), ('ForeArm', 'forearm'), ('Hand', 'hand'), ('UpLeg', 'thigh'), ('Leg', 'shin'), ('Foot', 'foot'), ('ToeBase', 'toe')]:
        names[side + source] = 'DEF-' + target + '.' + suffix
    for finger, target in enumerate(['thumb', 'f_index', 'f_middle', 'f_ring', 'f_pinky']):
        for segment in range(3):
            names[f'HandLoRA_Bip01_{suffix}_Finger{finger}' + (str(segment) if segment else '')] = f'DEF-{target}.{segment+1:02}.{suffix}'
for old, new in names.items():
    rig.data.bones[old].name = new
    assert mesh.vertex_groups.get(new), new

factor = 1.75 / max(v.co.z for v in mesh.data.vertices)
for vertex in mesh.data.vertices:
    vertex.co *= factor
bpy.context.view_layer.objects.active = rig
bpy.ops.object.mode_set(mode='EDIT')
for bone in rig.data.edit_bones:
    bone.head *= factor
    bone.tail *= factor
bpy.ops.object.mode_set(mode='OBJECT')
# Keep the oversized skull, eyes and teeth rigid while the neck follows the body.
if PET == 'zombert':
    head = mesh.vertex_groups['DEF-spine.006']
    for vertex in mesh.data.vertices:
        if vertex.co.z > .67 * factor and abs(vertex.co.x) < .29 * factor:
            for group in list(vertex.groups):
                mesh.vertex_groups[group.group].remove([vertex.index])
            head.add([vertex.index], 1.0, 'REPLACE')
rig.name, mesh.name = catalog[PET]['name']+'_Rig', catalog[PET]['name']+'_Surface'
textures = export_model(ROOT, PET, SOURCE, rig, mesh)
for side, sign in [('L', 1), ('R', -1)]:
    bone = rig.pose.bones['DEF-upper_arm.' + side]
    rest = bone.bone.matrix_local.to_quaternion()
    bone.rotation_quaternion = rest.inverted() @ Quaternion((0, 1, 0), sign * 1.25) @ rest
render_thumbnail(ROOT, PET, mesh, textures)
