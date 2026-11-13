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

if PET == 'hana':
    # Repair the long hair before baking the arm lift into the bind pose.
    import numpy as np
    bpy.context.view_layer.objects.active=mesh
    bpy.ops.object.mode_set(mode='EDIT');bpy.ops.mesh.select_all(action='SELECT')
    bpy.ops.mesh.remove_doubles(threshold=.00001)
    bpy.ops.object.mode_set(mode='OBJECT')
    shader=mesh.data.materials[0].node_tree.nodes.get('Principled BSDF')
    image=shader.inputs['Base Color'].links[0].from_node.image
    pixels=np.empty(len(image.pixels),dtype=np.float32);image.pixels.foreach_get(pixels)
    uv=np.empty(len(mesh.data.loops)*2,dtype=np.float32);mesh.data.uv_layers.active.data.foreach_get('uv',uv);uv=uv.reshape(-1,2)
    width,height=image.size
    rgb=pixels.reshape(height,width,4)[np.clip((uv[:,1]*height).astype(int),0,height-1),np.clip((uv[:,0]*width).astype(int),0,width-1),:3]
    indices=np.empty(len(mesh.data.loops),dtype=np.int32);mesh.data.loops.foreach_get('vertex_index',indices)
    count=len(mesh.data.vertices)
    samples=np.maximum(1,np.bincount(indices,minlength=count))
    colors=np.stack([np.bincount(indices,weights=rgb[:,channel],minlength=count)/samples for channel in range(3)],axis=1)
    points=np.empty(count*3,dtype=np.float32);mesh.data.vertices.foreach_get('co',points);points=points.reshape(-1,3)
    x,y,z=points.T;r,g,b=colors.T
    height=max(v.co.z for v in mesh.data.vertices)
    neck_z=rig.data.bones['Neck'].head_local.z
    hips_z=rig.data.bones['Hips'].head_local.z
    floor=max(neck_z-height*.45,hips_z-height*.20)
    eligible=(z>floor)&(abs(x)<height*.35)&((z>hips_z)|(abs(x)>height*.16)|(y>height*.12))
    torso=(z>hips_z)&(z<neck_z)&(abs(x)<height*.14)&(y<height*.02)
    eligible &= ~torso
    crown=z>neck_z+height*.12
    purple=(b>r*1.08)&(r>g*1.02)&(b>g*1.08)
    pink=(r>b*1.02)&(r>g*1.1)&(b>g*1.1)&(z<neck_z+height*.12)&(y<height*.08)
    cyan=(g>r*1.1)&(z<neck_z)
    head=mesh.vertex_groups['Head']
    original_head=np.array([next((group.weight for group in v.groups if group.group==head.index),0) for v in mesh.data.vertices])
    face=(z>neck_z+height*.015)&(abs(x)<height*.21)&(y<rig.data.bones['Neck'].head_local.y-height*.03)
    body_seed=((~eligible)|torso|pink|cyan)&~crown&~face
    head_seed=((purple&eligible)|((y>height*.13)&eligible)|((original_head>.5)&(z>neck_z))|crown|face)&~body_seed
    edges=np.empty(len(mesh.data.edges)*2,dtype=np.int32);mesh.data.edges.foreach_get('vertices',edges);edges=edges.reshape(-1,2)
    chroma=colors/np.maximum(.001,colors.max(axis=1,keepdims=True))
    costs=np.maximum(.00001,np.linalg.norm(points[edges[:,0]]-points[edges[:,1]],axis=1))*np.exp(np.minimum(12,30*((chroma[edges[:,0]]-chroma[edges[:,1]])**2).sum(axis=1)))
    starts=np.concatenate([edges[:,0],edges[:,1]]);ends=np.concatenate([edges[:,1],edges[:,0]])
    order=np.argsort(starts,kind='stable');ends=ends[order];costs=np.concatenate([costs,costs])[order]
    offsets=np.concatenate([[0],np.cumsum(np.bincount(starts,minlength=count))])
    # Color boundaries stop cloth labels; connected grey highlights inherit their hair region.
    import heapq
    labels=np.full(count,-1,dtype=np.int8);labels[body_seed]=0;labels[head_seed]=1
    distances=np.full(count,np.inf);seeds=np.flatnonzero(body_seed|head_seed).tolist();distances[seeds]=0
    queue=[(0,index) for index in seeds];heapq.heapify(queue)
    while queue:
        distance,index=heapq.heappop(queue)
        if distance!=distances[index]:continue
        for position in range(offsets[index],offsets[index+1]):
            neighbor=int(ends[position]);candidate=distance+float(costs[position])
            if candidate<distances[neighbor]:
                distances[neighbor]=candidate;labels[neighbor]=labels[index]
                heapq.heappush(queue,(candidate,neighbor))
    loose=(labels<0)&eligible&(b>=r*.98)&(r>=g*.98)
    selected=np.flatnonzero((labels==1)|loose).tolist()
    assert 2000<len(selected)<len(mesh.data.vertices)*.9
    selected_mask=np.zeros(count,dtype=bool);selected_mask[selected]=True
    for index in np.flatnonzero((~selected_mask)&(original_head>0)).tolist():
        head.remove([index])
        if not any(group.weight>0 for group in mesh.data.vertices[index].groups):mesh.vertex_groups['Spine2'].add([index],1,'REPLACE')
    neck_point=rig.data.bones['Neck'].head_local.copy()
    def neck_area(point):
        return abs(point.x-neck_point.x)<height*.06 and abs(point.y-neck_point.y)<height*.05 and neck_z-height*.08<point.z<neck_z+height*.12
    for index in selected:
        for group in list(mesh.data.vertices[index].groups):mesh.vertex_groups[group.group].remove([index])
        head.add([index],1,'REPLACE')
    # The generated hair is fused to sleeves; remove those bridges before the arms move.
    import bmesh
    edit=bmesh.new();edit.from_mesh(mesh.data);edit.verts.ensure_lookup_table()
    bridges=[]
    for polygon in edit.faces:
        if len({bool(selected_mask[v.index]) for v in polygon.verts})<2:continue
        center=polygon.calc_center_median()
        if neck_area(center):continue
        bridges.append(polygon)
    bmesh.ops.delete(edit,geom=bridges,context='FACES')
    edit.to_mesh(mesh.data);edit.free();mesh.data.update()
    print('HANA_REMOVED_HAIR_CLOTH_BRIDGES='+str(len(bridges)),flush=True)
    print('HANA_RIGID_HAIR_VERTICES='+str(len(selected)),flush=True)

if PET == 'vampire':
    from vampire_rig import rig_tentacles
    rig_tentacles(ROOT, rig, mesh)

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
elif PET == 'snoopy':
    # The long ears overlap the T-pose arms but must stay attached to the skull.
    head = mesh.vertex_groups['DEF-spine.006']
    neck_z = rig.data.bones['DEF-spine.004'].head_local.z
    for vertex in mesh.data.vertices:
        x,y,z=vertex.co
        if z > neck_z + .07 * factor or (z > neck_z + .015 * factor and abs(y) > .08 * factor) or (z > neck_z - .08 * factor and abs(x) > .18 * factor and y > .08 * factor):
            for group in list(vertex.groups):
                mesh.vertex_groups[group.group].remove([vertex.index])
            head.add([vertex.index], 1.0, 'REPLACE')
rig.name, mesh.name = catalog[PET]['name']+'_Rig', catalog[PET]['name']+'_Surface'
textures = export_model(ROOT, PET, SOURCE, rig, mesh)
for side, sign in [('L', 1), ('R', -1)]:
    bone = rig.pose.bones['DEF-upper_arm.' + side]
    rest = bone.bone.matrix_local.to_quaternion()
    bone.rotation_quaternion = rest.inverted() @ Quaternion((0, 1, 0), sign * 1.25) @ rest
if PET == 'vampire':
    from vampire_rig import pose_tentacles
    pose_tentacles(rig)
render_thumbnail(ROOT, PET, mesh, textures)
