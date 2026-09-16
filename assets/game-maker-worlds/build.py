"""Blender authoring/export. --only produces an explicitly partial review build."""
from pathlib import Path
import sys, json, hashlib, argparse, importlib.util, math
import bpy
from mathutils import Matrix, Vector

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
spec=importlib.util.spec_from_file_location('worlds_catalog',HERE/'catalog.py')
catalog=importlib.util.module_from_spec(spec);spec.loader.exec_module(catalog)
sys.path.insert(0,str(HERE));sys.path.insert(0,str(HERE.parent/'game-maker-low-poly'))
import build as base
from geometry import xyz
from characters import armature, animate
from maritime import ship, person, fish, sea_animal
spec=importlib.util.spec_from_file_location('maritime_scenery',HERE/'environment.py')
scenery=importlib.util.module_from_spec(spec);spec.loader.exec_module(scenery)

OUT=ROOT/'internal/gamemaker/asset_packs/aurago-pirates-3d'
PRODUCTION=HERE/'production'
base.OUT=OUT

def write_json(path,data):
    path.parent.mkdir(parents=True,exist_ok=True)
    path.write_text(json.dumps(data,separators=(',',':'),ensure_ascii=False)+'\n',encoding='utf-8')

def parts_animation(g,objects):
    clips={}
    for part in g.moving:
        obj=next(o for o in objects.values() if o.name==part['node'])
        kind=part['kind'];rest=obj.location.copy()
        actions={'hinge':['open','close'],'sway':['sway' if g.id.startswith('nature-') else 'sailing'],'recoil':['fire'],'damage':['damaged','repair'],'rotate':['operate'] if entry_mechanism(g.id) else []}.get(kind,[])
        for action in actions:
            frames=31 if action in ('open','close') else 61 if action in ('sailing','sway') else 16
            obj.animation_data_create();clip=bpy.data.actions.new(g.id+'__'+action);obj.animation_data.action=clip
            for f in range(1,frames+1,3):
                t=(f-1)/(frames-1);axis=xyz(part['axis'])
                if kind=='damage':
                    value=min(1,t*4) if action=='damaged' else max(0,1-t*4)
                    obj.scale=(value,)*3;obj.keyframe_insert('scale',frame=f)
                elif kind=='recoil':
                    obj.location=rest-axis*.3*math.sin(min(1,t*3)*math.pi);obj.keyframe_insert('location',frame=f)
                else:
                    value=t*math.tau if kind=='rotate' else .08*math.sin(t*math.tau) if kind=='sway' else (t if action=='open' else 1-t)*math.pi*.55
                    obj.rotation_mode='QUATERNION';obj.rotation_quaternion=axis.rotation_difference(axis) if value==0 else Matrix.Rotation(value,4,axis).to_quaternion();obj.keyframe_insert('rotation_quaternion',frame=f)
            track=obj.animation_data.nla_tracks.new();track.name=action;track.strips.new(action,1,clip);track.mute=True
            obj.animation_data.action=None;obj.location=rest;obj.rotation_quaternion=(1,0,0,0)
            if kind=='damage':obj.scale=(0,0,0)
            clips[action]=dict(id=action,duration=(frames-1)/30,loop=action in ('sailing','sway','operate'),speed=0,events=[dict(time=.033,name='shot')] if action=='fire' else [])
    return list(clips.values())

def entry_mechanism(id):
    return id.startswith('equipment-') or id.startswith('objects-')

def create(entry):
    category=entry['category']
    if category=='ships':return ship(entry)
    if category=='people':return person(entry)
    if category=='submarines':return scenery.submarine(entry)
    if category=='nature':return scenery.nature(entry)
    if category=='coast':return scenery.coast(entry)
    if category=='harbor':return scenery.harbor(entry)
    if category=='equipment':return scenery.equipment(entry)
    if category=='animals' and entry['design'] in ('reef-fish','schooling-fish','tuna','dolphin','reef-shark','hammerhead'):return fish(entry)
    if category=='animals':return sea_animal(entry)
    raise ValueError('Not yet authored: '+entry['id'])

def build_one(entry):
    bpy.ops.object.select_all(action='SELECT');bpy.ops.object.delete(use_global=False)
    scene=bpy.context.scene;scene.render.fps=30
    created=create(entry);g,bones,pose=created[:3]
    collection=bpy.data.collections.new(entry['id']);scene.collection.children.link(collection)
    rig=armature(entry['id']+'__rig',bones,collection) if bones else None
    root,objects=g.objects(collection,rig)
    root['asset_id']=entry['id'];root['license']='MIT'
    points=[v for part in g.parts.values() for v in part['vertices']]
    low=[min(v[i] for v in points) for i in range(3)];high=[max(v[i] for v in points) for i in range(3)]
    human=entry['category']=='people'
    actions=['idle','walk','run','crouch','jump','land','climb','swim','tread_water','interact','hit','death','saber_attack','block','pistol_fire','pistol_reload','cannon_operate','dive'] if human else ['idle','swim','turn','hit','death','attack'] if bones else []
    if len(created)>3:actions=created[3]
    clips=animate(rig,bones,actions,pose,entry['id']) if bones else parts_animation(g,objects)
    for clip in clips:
        if clip['id'] in ('swim','tread_water'):clip['loop']=True
        if clip['id'] in ('walk','run'):clip['events']=[dict(time=clip['duration']*.15,name='foot_left'),dict(time=clip['duration']*.65,name='foot_right')]
    desc={k:entry[k] for k in ('id','name','category','description','tags')}
    desc.update(kind='model3d',view='3d',entity=entry['id'],up='+Y',forward='+Z',units='metres',origin='waterline' if entry['category']=='ships' else 'center' if entry['category'] in ('submarines','animals') or entry['design'] in ('torpedo','depth-charge','spyglass') else 'base',
        bounds=dict(min=low,max=high),collider=dict(type='compound',shapes=g.colliders) if g.colliders else dict(type='box',center=[(a+b)/2 for a,b in zip(low,high)],size=[b-a for a,b in zip(low,high)]),
        sockets=[dict(id=n,node=objects['socket_'+n].name) for n in g.sockets],moving_parts=g.moving,rig='humanoid-v1' if human else entry['id']+'-v1' if bones else '',animations=clips,lods=[],connections=g.connections,
        preview='previews/'+entry['id']+'.webp')
    if entry['category']=='ships':desc['waterline']=0
    desc['layers']=[dict(id=name,node=objects[name].name) for name in ('roof','front-wall') if name in objects]
    if human:
        # Same rest skeleton, a single marine library per pack, no geometry copy.
        lib=OUT/'animations/humanoid-marine.glb'
        if not lib.exists():
            record,_=base.export({'root':rig},lib,True);record['level']=0
            write_json(OUT/'animations/humanoid-marine.json',dict(file=record,clips=clips))
        shared=json.loads((OUT/'animations/humanoid-marine.json').read_text());desc['animation_library']=shared['file'];desc['animations']=shared['clips']
    meshes=[o for o in objects.values() if o.type=='MESH'];original={o:o.data for o in meshes}
    for level,ratio in enumerate((1,.5,.2) if entry['category'] in ('ships','submarines','coast','harbor') else (1,)):
        if level:
            for obj in meshes:
                obj.data=original[obj].copy();bpy.context.view_layer.objects.active=obj
                mod=obj.modifiers.new('LOD','DECIMATE');mod.ratio=ratio
                if len(obj.modifiers)>1:bpy.ops.object.modifier_move_up(modifier=mod.name)
                bpy.ops.object.modifier_apply(modifier=mod.name)
        record,doc=base.export(objects,OUT/'models'/f"{entry['id']}.lod{level}.glb",bool(clips) and not human)
        if not human:assert {c['id'] for c in clips}<={c['name'] for c in doc.get('animations',[])},entry['id']
        record['level']=level;desc['lods'].append(record)
        for obj in meshes:
            if obj.data!=original[obj]:reduced=obj.data;obj.data=original[obj];bpy.data.meshes.remove(reduced)
    source=PRODUCTION/entry['id'];source.mkdir(parents=True,exist_ok=True)
    bpy.ops.wm.save_as_mainfile(filepath=str(source/'source.blend'),compress=True)
    write_json(source/'metadata.json',desc)
    return desc

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--only')
    options=parser.parse_args(sys.argv[sys.argv.index('--')+1:] if '--' in sys.argv else [])
    ids=set(options.only.split(',')) if options.only else {e['id'] for e in catalog.entries()};entries=[e for e in catalog.entries() if e['id'] in ids]
    assert len(entries)==len(ids),'Unknown IDs'
    OUT.mkdir(parents=True,exist_ok=True);records=[]
    for e in entries:
        records.append(build_one(e));print('MARITIME_EXPORTED',e['id'],flush=True)
    path=OUT/'manifest.json'
    if path.exists():records=[a for a in json.loads(path.read_text())['assets'] if a['id'] not in ids]+records
    write_json(path,dict(id='aurago-pirates-3d',version='1.0.0',schema_version=1,kind='model3d',license='MIT',name='AuraGo Pirates 3D',description='Original maritime low-poly assets.',tags=['pirates','steampunk','maritime'],assets=sorted(records,key=lambda a:a['id']),production_status='review'))
    (OUT/'LICENSE.txt').write_text((ROOT/'LICENSE').read_text(encoding='utf-8'),encoding='utf-8')
