"""Build the compact textured pet from the retained editable Rigify source."""
import bpy
import hashlib
import json
import math
from pathlib import Path
from mathutils import Quaternion

ROOT=Path(__file__).resolve().parents[2]
SOURCE=ROOT/'assets/desktop-pets/punk/source.blend'
OUT=ROOT/'ui/3d/pets/punk'
SCRATCH=ROOT/'disposable/_punk-pet/textures'
SCRATCH.mkdir(parents=True,exist_ok=True)
bpy.ops.wm.open_mainfile(filepath=str(SOURCE))
if bpy.context.object and bpy.context.object.mode!='OBJECT':
    bpy.ops.object.mode_set(mode='OBJECT')
rig=bpy.data.objects['PunkPersona_Rig']
author_rig=rig
mesh=bpy.data.objects['PunkPersona_Surface']
rig.data.pose_position='REST'
bpy.ops.object.select_all(action='DESELECT')
mesh.hide_set(False)
rig.hide_set(False)
mesh.select_set(True)
bpy.context.view_layer.objects.active=mesh
modifier=mesh.modifiers.new('Desktop LOD','DECIMATE')
modifier.ratio=.24
bpy.ops.object.modifier_apply(modifier=modifier.name)
triangles=sum(len(p.vertices)-2 for p in mesh.data.polygons)
assert 30000<triangles<=60000,triangles
texture_files=[]
for image in bpy.data.images:
    if image.type=='IMAGE':
        # Packed images are decoded lazily when a .blend opens in background mode.
        if not image.has_data:
            image.pixels[0]
        assert image.has_data,image.name
        if image.packed_file:
            image.unpack(method='REMOVE')
        if max(image.size)>1024:
            image.scale(1024,1024)
        texture_file=SCRATCH/f'{len(texture_files):02}.png'
        image.filepath_raw=str(texture_file)
        image.file_format='PNG'
        image.save()
        assert max(image.size)<=1024,image.name
        texture_files.append(texture_file)
# The library assumes a T-pose. Bake our A-pose mesh and deform bones into that rest pose.
author_rig.data.pose_position='POSE'
author_rig.animation_data.action=None
for bone in author_rig.pose.bones:
    bone.matrix_basis.identity()
for side in ('L','R'):
    author_rig.pose.bones['upper_arm_parent.'+side]['IK_FK']=1.0
    author_rig.pose.bones['thigh_parent.'+side]['IK_FK']=1.0
angles={}
for side,sign in [('L',1),('R',-1)]:
    control=author_rig.pose.bones['upper_arm_fk.'+side]
    direction=control.bone.tail_local-control.bone.head_local
    angle=sign*math.atan2(direction.z,abs(direction.x))
    rest=control.bone.matrix_local.to_quaternion()
    control.rotation_mode='QUATERNION'
    control.rotation_quaternion=rest.inverted()@Quaternion((0,1,0),angle)@rest
    angles[side]=angle
bpy.context.view_layer.update()
for side,sign in [('L',1),('R',-1)]:
    control=author_rig.pose.bones['forearm_fk.'+side]
    direction=control.tail-control.head
    angle=sign*math.atan2(direction.z,abs(direction.x))
    rest=control.bone.matrix_local.to_quaternion()
    control.rotation_mode='QUATERNION'
    control.rotation_quaternion=rest.inverted()@Quaternion((0,1,0),angle)@rest
bpy.context.view_layer.update()
for modifier in list(mesh.modifiers):
    if modifier.type=='ARMATURE':
        bpy.ops.object.modifier_apply(modifier=modifier.name)
# Rigify drives independent limb roots through constraints; portable retargeting needs a tree.
bpy.ops.object.select_all(action='DESELECT')
armature=bpy.data.armatures.new('PunkPetDeform')
rig=bpy.data.objects.new('PunkPetDeform',armature)
bpy.context.scene.collection.objects.link(rig)
rig.select_set(True)
bpy.context.view_layer.objects.active=rig
bpy.ops.object.mode_set(mode='EDIT')
for bone in author_rig.data.bones:
    if bone.use_deform:
        target=armature.edit_bones.new(bone.name)
        posed=author_rig.pose.bones[bone.name]
        target.matrix=posed.matrix.copy()
        target.length=(posed.tail-posed.head).length
for bone in author_rig.data.bones:
    if not bone.use_deform:
        continue
    parent=bone.parent.name if bone.parent and bone.parent.name in armature.edit_bones else None
    if bone.name.startswith(('DEF-thigh.','DEF-pelvis.')) and not bone.name.endswith('.001'):
        parent='DEF-spine'
    elif bone.name.startswith('DEF-shoulder.'):
        parent='DEF-spine.003'
    elif bone.name in ('DEF-upper_arm.L','DEF-upper_arm.R'):
        parent='DEF-shoulder.'+bone.name[-1]
    elif bone.name.startswith('DEF-palm.'):
        parent='DEF-hand.'+bone.name[-1]
    elif bone.name.startswith(('DEF-f_','DEF-thumb.')):
        stem,segment,side=bone.name.rsplit('.',2)
        parent=f'{stem}.{int(segment)-1:02}.{side}' if int(segment)>1 else 'DEF-hand.'+side
    if parent:
        armature.edit_bones[bone.name].parent=armature.edit_bones[parent]
bpy.ops.object.mode_set(mode='OBJECT')
mesh.modifiers.new('Armature','ARMATURE').object=rig
mesh.parent=rig
mesh.select_set(True)
assert armature.bones['DEF-thigh.L'].parent.name=='DEF-spine'
assert armature.bones['DEF-upper_arm.L'].parent.name=='DEF-shoulder.L'
assert sum(b.parent is None for b in armature.bones)==1
rig.select_set(True)
OUT.mkdir(parents=True,exist_ok=True)
bpy.ops.export_scene.gltf(filepath=str(OUT/'model.glb'),export_format='GLB',use_selection=True,export_skins=True,export_def_bones=True,export_animations=False,export_cameras=False,export_lights=False)
report={'source_sha256':hashlib.sha256(SOURCE.read_bytes()).hexdigest(),'triangles':triangles,'vertices':len(mesh.data.vertices),'deform_joints':sum(b.use_deform for b in rig.data.bones),'texture_max_size':1024,'model_sha256':hashlib.sha256((OUT/'model.glb').read_bytes()).hexdigest()}
assert (OUT/'model.glb').stat().st_size<8*1024*1024
(OUT/'model.json').write_text(json.dumps(report,indent=2)+'\n',encoding='utf-8',newline='\n')
for side in ('L','R'):
    control=rig.pose.bones['DEF-upper_arm.'+side]
    rest=control.bone.matrix_local.to_quaternion()
    control.rotation_mode='QUATERNION'
    control.rotation_quaternion=rest.inverted()@Quaternion((0,1,0),-angles[side])@rest
scene=bpy.context.scene
ground=bpy.data.objects.get('PreviewGround')
if ground:
    ground.hide_render=True
scene.render.film_transparent=True
scene.render.resolution_x,scene.render.resolution_y=384,416
scene.render.resolution_percentage=100
scene.render.filepath=str(OUT/'thumbnail.png')
bpy.ops.render.render(write_still=True)
for texture_file in texture_files:
    texture_file.unlink(missing_ok=True)
print('PET_MODEL_READY='+json.dumps(report),flush=True)
