"""Prepare the retained VARCO finger rig for the shared offline pet runtime."""
import bpy
import hashlib
import json
import math
from pathlib import Path
from mathutils import Quaternion, Vector

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / 'assets/desktop-pets/zombert/varco-rigged.glb'
OUT = ROOT / 'ui/3d/pets/zombert'
SCRATCH = ROOT / 'disposable/_zombert-pet/textures'
OUT.mkdir(parents=True, exist_ok=True)
SCRATCH.mkdir(parents=True, exist_ok=True)
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
head = mesh.vertex_groups['DEF-spine.006']
for vertex in mesh.data.vertices:
    if vertex.co.z > .67 * factor and abs(vertex.co.x) < .29 * factor:
        for group in list(vertex.groups):
            mesh.vertex_groups[group.group].remove([vertex.index])
        head.add([vertex.index], 1.0, 'REPLACE')
rig.name, mesh.name = 'Zombert_Rig', 'Zombert_Surface'
for image in bpy.data.images:
    if image.type == 'IMAGE':
        image.pack()
bpy.context.preferences.filepaths.save_version = 0
bpy.ops.wm.save_as_mainfile(filepath=str(ROOT / 'assets/desktop-pets/zombert/source.blend'))

bpy.ops.object.select_all(action='DESELECT')
mesh.select_set(True)
bpy.context.view_layer.objects.active = mesh
modifier = mesh.modifiers.new('Desktop LOD', 'DECIMATE')
modifier.ratio = 50000 / sum(len(p.vertices)-2 for p in mesh.data.polygons)
bpy.ops.object.modifier_move_up(modifier=modifier.name)
bpy.ops.object.modifier_apply(modifier=modifier.name)
triangles = sum(len(p.vertices)-2 for p in mesh.data.polygons)
assert 40000 < triangles <= 60000, triangles
for vertex in mesh.data.vertices:
    total = sum(g.weight for g in vertex.groups)
    assert total > 0, vertex.index
    for group in list(vertex.groups):
        mesh.vertex_groups[group.group].add([vertex.index], group.weight / total, 'REPLACE')
textures = []
for image in bpy.data.images:
    if image.type != 'IMAGE':
        continue
    if not image.has_data:
        image.pixels[0]
    image.unpack(method='REMOVE')
    if max(image.size) > 1024:
        image.scale(1024, 1024)
    target = SCRATCH / f'{len(textures):02}.png'
    image.filepath_raw, image.file_format = str(target), 'PNG'
    image.save()
    textures.append(target)
rig.select_set(True)
assert sum(b.parent is None for b in rig.data.bones) == 1
assert rig.data.bones['DEF-thumb.01.L'].parent.name == 'DEF-hand.L'
bpy.ops.export_scene.gltf(filepath=str(OUT/'model.glb'), export_format='GLB', use_selection=True, export_skins=True, export_animations=False, export_cameras=False, export_lights=False)
assert (OUT/'model.glb').stat().st_size < 8*1024*1024
report = {'source_sha256': hashlib.sha256(SOURCE.read_bytes()).hexdigest(), 'triangles': triangles, 'vertices': len(mesh.data.vertices), 'deform_joints': len(rig.data.bones), 'texture_max_size': 1024, 'model_sha256': hashlib.sha256((OUT/'model.glb').read_bytes()).hexdigest()}
(OUT/'model.json').write_text(json.dumps(report, indent=2)+'\n', encoding='utf-8', newline='\n')
print('ZOMBERT_MODEL_READY='+json.dumps(report), flush=True)

for side, sign in [('L', 1), ('R', -1)]:
    bone = rig.pose.bones['DEF-upper_arm.' + side]
    rest = bone.bone.matrix_local.to_quaternion()
    bone.rotation_quaternion = rest.inverted() @ Quaternion((0, 1, 0), sign * 1.25) @ rest
scene = bpy.context.scene
scene.render.engine = 'CYCLES'
scene.cycles.samples = 24
scene.world = bpy.data.worlds.new('Studio')
scene.world.use_nodes = True
scene.world.node_tree.nodes['Background'].inputs['Color'].default_value = (.12, .13, .16, 1)
scene.world.node_tree.nodes['Background'].inputs['Strength'].default_value = .7
center = Vector((0, 0, .875))
for name, location, power in [('Key', (-2,-3,4),400), ('Fill',(2,-2,2),180), ('Rim',(1,2,3),300)]:
    data = bpy.data.lights.new(name, 'AREA')
    data.energy, data.size = power, 3
    obj = bpy.data.objects.new(name, data)
    scene.collection.objects.link(obj)
    obj.location = location
    obj.rotation_euler = (center-obj.location).to_track_quat('-Z','Y').to_euler()
data = bpy.data.cameras.new('Preview')
camera = bpy.data.objects.new('Preview', data)
scene.collection.objects.link(camera)
scene.camera = camera
data.type, data.ortho_scale = 'ORTHO', 2.0
camera.location = center + Vector((.25,-4,0))
camera.rotation_euler = (center-camera.location).to_track_quat('-Z','Y').to_euler()
scene.render.film_transparent = True
scene.render.image_settings.color_mode = 'RGBA'
scene.render.resolution_x, scene.render.resolution_y = 384, 416
scene.render.resolution_percentage = 100
scene.render.filepath = str(OUT/'thumbnail.png')
bpy.ops.render.render(write_still=True)
for texture in textures:
    texture.unlink()
