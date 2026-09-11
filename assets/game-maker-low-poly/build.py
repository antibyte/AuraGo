"""Reproducible Blender authoring/export. Run with --background --python ... --.

--only accepts comma-separated catalog IDs for a focused production iteration.
Full builds write ten retained .blend scenes and the complete runtime catalog.
"""
from pathlib import Path
import argparse
import hashlib
import json
import struct
import sys
from math import pi
import bpy
from mathutils import Vector, Matrix

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
from catalog import entries, VERSION, PACK_ID, HUMAN_ACTIONS, CATEGORY_NAMES
from geometry import Mesh, xyz
from vehicles import vehicle, aircraft, space
from characters import humanoid, armature, animate, human_pose

ROOT=HERE.parents[1]
OUT=ROOT/'internal/gamemaker/asset_packs'/PACK_ID
SOURCE=HERE/'production'


def json_write(path,data):
    path.parent.mkdir(parents=True,exist_ok=True)
    path.write_text(json.dumps(data,indent=2,ensure_ascii=False)+'\n',encoding='utf-8',newline='\n')


def glb_document(path):
    data=path.read_bytes(); length=struct.unpack_from('<I',data,12)[0]
    return data,json.loads(data[20:20+length]),length


def canonicalize(path):
    data,doc,length=glb_document(path)
    for i,mesh in enumerate(doc.get('meshes',[])):mesh['name']='mesh'+str(i)
    for scene in doc.get('scenes',[]):scene['name']='AuraGoLowPoly'
    for animation in doc.get('animations',[]):animation['name']=animation.get('name','').split('__')[-1]
    payload=json.dumps(doc,separators=(',',':'),sort_keys=True).encode()
    payload+=b' '*(-len(payload)%4)
    tail=data[20+length:]
    data=struct.pack('<III',0x46546C67,2,20+len(payload)+len(tail))+struct.pack('<II',len(payload),0x4E4F534A)+payload+tail
    path.write_bytes(data)
    return doc


def export(objects,path,animations=False):
    bpy.ops.object.select_all(action='DESELECT')
    for obj in objects.values():obj.select_set(True)
    bpy.context.view_layer.objects.active=objects['root']
    bpy.context.view_layer.update()
    path.parent.mkdir(parents=True,exist_ok=True)
    bpy.ops.export_scene.gltf(filepath=str(path),export_format='GLB',use_selection=True,
        use_active_scene=True,export_animations=animations,export_animation_mode='NLA_TRACKS',
        export_frame_range=False,export_cameras=False,export_lights=False,export_extras=True,
        export_texcoords=False,export_normals=True,export_tangents=False,export_yup=True,
        export_apply=False,export_optimize_animation_size=True,export_anim_slide_to_zero=True)
    doc=canonicalize(path)
    triangles=sum(doc['accessors'][p['indices']]['count']//3 for m in doc.get('meshes',[]) for p in m['primitives'] if 'indices' in p)
    return dict(file=path.relative_to(OUT).as_posix(),bytes=path.stat().st_size,
                sha256=hashlib.sha256(path.read_bytes()).hexdigest(),triangles=triangles),doc


def build_asset(entry,scene):
    category=entry['category'];bones=None;actions=[];pose_fn=None
    if category=='road':g=vehicle(entry)
    elif category=='aircraft':g=aircraft(entry)
    elif category=='space':g=space(entry)
    elif category=='humans':g,bones=humanoid(entry)
    elif category=='animals':
        from animals import animal
        g,bones,actions,pose_fn=animal(entry)
    elif category=='fps':
        from fps import fps
        g,bones,actions,pose_fn=fps(entry)
    else:
        from environment import environment
        g=environment(entry)
    collection=bpy.data.collections.new(entry['id']);scene.collection.children.link(collection)
    rig=armature(entry['id']+'__rig',bones,collection) if bones else None
    root,objects=g.objects(collection,rig)
    root['asset_id']=entry['id'];root['license']='MIT'
    objects['root']=root
    pts=[Vector(v) for part in g.parts.values() for v in part['vertices']]
    low=[min(v[i] for v in pts) for i in range(3)];high=[max(v[i] for v in pts) for i in range(3)]
    centered=category in ('aircraft','space','fps')
    # Generated geometry already uses the ground plane; trim tiny numerical drift.
    if not centered and abs(low[1])>.00001:
        root.location.z=-low[1];high[1]-=low[1];low[1]=0
    desc={k:entry[k] for k in ('id','name','category','description','tags')}
    desc.update(kind='model3d',view='3d',entity=entry['id'],forward='+Z',up='+Y',units='metres',
                bounds=dict(min=[round(x,5) for x in low],max=[round(x,5) for x in high]),
                origin=('camera' if entry['design'].startswith('arms-') else 'grip') if category=='fps' else 'center' if centered else 'base',
                collider=dict(type='compound',shapes=g.colliders) if g.colliders else dict(type='capsule' if category in ('humans','animals') else 'box',center=[round((a+b)/2,5) for a,b in zip(low,high)],size=[round(b-a,5) for a,b in zip(low,high)]),
                sockets=[dict(id=n,node=objects['socket_'+n].name) for n in g.sockets],
                connections=g.connections,moving_parts=g.moving,
                rig=('humanoid-v1' if category=='humans' else 'fps-arms-v1' if entry['design'].startswith('arms-') else entry['id']+'-v1') if bones else '',animations=[],lods=[])
    if actions:
        desc['animations']=animate(rig,bones,actions,pose_fn,entry['id'])
    elif g.moving:
        desc['animations']=animate_parts(g,objects)
    if category=='fps':
        desc['views']=['first-person'] if entry['design'].startswith('arms-') else ['world','first-person']
        if entry['design'].startswith('arms-') or entry['design'] in ('pistol','smg','rifle','shotgun','sniper','energy-pistol','plasma-rifle','heavy-blaster'):
            desc['fps_binding']=dict(rig='fps-arms-v1',parent='shared-view',weapon_translation=[.12,.01,.145],
                arm_action_prefix='pistol_' if entry['design'] in ('pistol','energy-pistol') else '',
                timing='Start matching weapon and arm actions together; update with the same delta. Recoil is already baked.')
    meshes=[obj for obj in objects.values() if obj.type=='MESH']
    original={obj:obj.data for obj in meshes}
    for level,ratio in enumerate((1,.5,.2)):
        if level and desc['lods'][0]['triangles']<1000:break
        if level:
            for obj in meshes:
                obj.data=original[obj].copy()
                # Bake only geometry reduction; keep skin modifiers and bone weights.
                bpy.context.view_layer.objects.active=obj
                modifier=obj.modifiers.new('LOD','DECIMATE');modifier.ratio=ratio
                bpy.ops.object.modifier_move_up(modifier=modifier.name) if len(obj.modifiers)>1 else None
                bpy.ops.object.modifier_apply(modifier=modifier.name)
                obj.data.validate(verbose=False)
        result,doc=export(objects,OUT/'models'/f"{entry['id']}.lod{level}.glb",bool(desc['animations']) and level==0)
        result['level']=level;desc['lods'].append(result)
        if level==0:assert {a['name'] for a in doc.get('animations',[])}=={a['id'] for a in desc['animations']},entry['id']
        if level:
            for obj in meshes:
                reduced=obj.data;obj.data=original[obj];bpy.data.meshes.remove(reduced)
    if category=='humans':
        library=OUT/'animations/humanoid-v1.glb'
        if not library.exists() or entry['design']=='civilian-a':
            clips=animate(rig,bones,HUMAN_ACTIONS,human_pose,'humanoid')
            result,doc=export(objects,library,True)
            exported={a['name'] for a in doc.get('animations',[])}
            assert exported==set(HUMAN_ACTIONS),(entry['id'],exported)
            json_write(OUT/'animations/humanoid-v1.json',dict(file=result,clips=clips))
        shared=json.loads((OUT/'animations/humanoid-v1.json').read_text())
        desc['animation_library']=shared['file']
        desc['animations']=shared['clips']
    desc['preview']='previews/'+entry['id']+'.webp'
    preview=OUT/desc['preview']
    if preview.exists():
        data=preview.read_bytes()
        desc['preview_file']=dict(file=desc['preview'],bytes=len(data),sha256=hashlib.sha256(data).hexdigest())
    return desc,root


def animate_parts(g,objects):
    """Bake open/close once; continuous wheels and propellers stay game-driven."""
    parts=[m for m in g.moving if m['kind'] in ('hinge','slide','lift')]
    if not parts:return []
    for m in parts:
        obj=next(o for o in objects.values() if o.name==m['node'])
        rest=obj.location.copy();obj.rotation_mode='AXIS_ANGLE'
        for action in ('open','close'):
            obj.animation_data_create();clip=bpy.data.actions.new(g.id+'__'+action)
            obj.animation_data.action=clip
            for frame,fraction in ((1,0),(10,.25),(20,.75),(31,1)):
                if action=='close':fraction=1-fraction
                if m['kind']=='hinge':
                    key=next(k for k,v in objects.items() if v==obj)
                    sign=-1 if g.pivots[key][0][0]>0 else 1
                    axis=xyz(m['axis']);obj.rotation_axis_angle=(sign*fraction*pi*.52,*axis)
                    obj.keyframe_insert('rotation_axis_angle',frame=frame)
                else:
                    obj.location=rest+xyz(m['axis'])*fraction*(2.4 if m['kind']=='lift' else 1.25)
                    obj.keyframe_insert('location',frame=frame)
            track=obj.animation_data.nla_tracks.new();track.name=action
            strip=track.strips.new(action,1,clip);track.mute=True
        obj.animation_data.action=None;obj.location=rest;obj.rotation_axis_angle=(0,0,0,1)
    return [dict(id=n,duration=1,loop=False,speed=0) for n in ('open','close')]


def build(only=None):
    bpy.ops.object.select_all(action='SELECT');bpy.ops.object.delete(use_global=False)
    SOURCE.mkdir(parents=True,exist_ok=True);OUT.mkdir(parents=True,exist_ok=True)
    selected=[e for e in entries() if not only or e['id'] in only]
    assert selected,'No matching catalog IDs'
    all_metadata=[]
    for category in dict.fromkeys(e['category'] for e in selected):
        scene=bpy.data.scenes.new('AuraGo '+CATEGORY_NAMES[category])
        bpy.context.window.scene=scene;scene.render.fps=30
        roots=[]
        for entry in (e for e in selected if e['category']==category):
            meta,root=build_asset(entry,scene)
            all_metadata.append(meta);roots.append(root)
            print('POLY_EXPORTED',entry['id'],sum(l['bytes'] for l in meta['lods']),flush=True)
        for i,root in enumerate(roots):root.location.x+=(i%6)*24;root.location.y+=(i//6)*24
        bpy.data.libraries.write(str(SOURCE/(category+'.blend')),{scene},compress=True)
        # Release production meshes before the next category; .blend retains editable sources.
        for obj in list(scene.objects):bpy.data.objects.remove(obj,do_unlink=True)
        bpy.data.scenes.remove(scene)
        bpy.ops.outliner.orphans_purge(do_recursive=True)
    metadata_path=OUT/'manifest.json'
    if only and metadata_path.exists():
        current=json.loads(metadata_path.read_text())['assets']
        all_metadata=sorted([e for e in current if e['id'] not in only]+all_metadata,key=lambda e:e['id'])
    manifest=dict(id=PACK_ID,version=VERSION,schema_version=1,kind='model3d',name='AuraGo Low Poly',
        description='220 original low-poly models: vehicles, worlds, modular interiors, animated people, animals and FPS equipment.',
        tags=['3d','low-poly','vehicles','space','buildings','nature','characters','fps'],
        license='MIT',generator='Blender 5.2.1 LTS / assets/game-maker-low-poly/build.py',
        units='metres',up='+Y',forward='+Z',budget_bytes=100*1024*1024,
        categories=[dict(id=k,name=v) for k,v in CATEGORY_NAMES.items()],assets=all_metadata)
    json_write(metadata_path,manifest)
    (OUT/'LICENSE.txt').write_text((ROOT/'LICENSE').read_text(encoding='utf-8'),encoding='utf-8',newline='\n')
    total=sum(p.stat().st_size for p in OUT.rglob('*') if p.is_file())
    assert total<=manifest['budget_bytes'],total
    print('POLY_COMPLETE',len(all_metadata),'models',total,'bytes',flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--only')
    options=parser.parse_args(sys.argv[sys.argv.index('--')+1:] if '--' in sys.argv else [])
    build(set(options.only.split(',')) if options.only else None)
