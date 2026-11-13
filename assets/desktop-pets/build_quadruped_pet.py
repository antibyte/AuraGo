"""Compact the original VARCO quadruped skin without replacing its learned weights."""
import argparse
import bpy
import hashlib
import json
import sys
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'assets/desktop-pets'))
from model_export import export_model,render_thumbnail
catalog=json.loads((ROOT/'assets/desktop-pets/catalog.json').read_text(encoding='utf-8'))
parser=argparse.ArgumentParser()
parser.add_argument('--pet',required=True,choices=[key for key,item in catalog.items() if item['kind']=='quadruped'])
pet=parser.parse_args(sys.argv[sys.argv.index('--')+1:]).pet
source=ROOT/f'assets/desktop-pets/{pet}/varco-rigged.glb'
bpy.ops.wm.read_factory_settings(use_empty=True)
bpy.ops.import_scene.gltf(filepath=str(source))
rig=next(o for o in bpy.context.scene.objects if o.type=='ARMATURE')
meshes=[o for o in bpy.context.scene.objects if o.type=='MESH' and any(m.type=='ARMATURE' for m in o.modifiers)]
assert len(meshes)==1
mesh=meshes[0]
for obj in list(bpy.context.scene.objects):
    if obj not in (mesh,rig):bpy.data.objects.remove(obj,do_unlink=True)
assert mesh.matrix_world.is_identity and rig.matrix_world.is_identity
assert len(rig.data.bones)==29
assert rig.data.bones['spine.006'].parent.name=='spine.005'
for bone in list(rig.data.bones):
    old=bone.name
    if old=='Root':name='DEF-root'
    elif old.startswith('tail'):name='Tail.'+str(1 if old=='tail' else int(old.split('.')[1])+1).zfill(2)
    else:name='DEF-'+old
    bone.name=name
low=[min(v.co[i] for v in mesh.data.vertices) for i in range(3)]
high=[max(v.co[i] for v in mesh.data.vertices) for i in range(3)]
assert abs(low[2])<.001
factor=min(1.75/high[2],1.8/max(high[0]-low[0],high[1]-low[1]))
for vertex in mesh.data.vertices:vertex.co*=factor
bpy.context.view_layer.objects.active=rig
bpy.ops.object.mode_set(mode='EDIT')
for bone in rig.data.edit_bones:bone.head*=factor;bone.tail*=factor
bpy.ops.object.mode_set(mode='OBJECT')
rig.name,mesh.name=catalog[pet]['name']+'_Rig',catalog[pet]['name']+'_Surface'
report={'source_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'scale':factor,'source_bounds':[low,high],'original_varco_weights_preserved':True,'root_motion_bone':'DEF-root','bones':[{'name':b.name,'head':list(b.head_local),'tail':list(b.tail_local),'parent':b.parent.name if b.parent else None} for b in rig.data.bones]}
(ROOT/f'assets/desktop-pets/{pet}/rig.json').write_text(json.dumps(report,indent=2)+'\n',encoding='utf-8',newline='\n')
textures=export_model(ROOT,pet,source,rig,mesh)
render_thumbnail(ROOT,pet,mesh,textures)
