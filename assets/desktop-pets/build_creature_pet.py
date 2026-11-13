"""Author anatomical skinning for retained non-humanoid desktop pet meshes."""
import argparse
import bpy
import hashlib
import json
import sys
from pathlib import Path
from mathutils import Vector

ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'assets/desktop-pets'))
from model_export import export_model, render_thumbnail
from shape_rigs import build_profile
catalog=json.loads((ROOT/'assets/desktop-pets/catalog.json').read_text(encoding='utf-8'))
parser=argparse.ArgumentParser()
parser.add_argument('--pet',required=True,choices=[key for key,value in catalog.items() if value['kind'] not in ('humanoid','quadruped')])
args=parser.parse_args(sys.argv[sys.argv.index('--')+1:])
PET=args.pet
SOURCE=ROOT/f'assets/desktop-pets/{PET}/varco-source.glb'
bpy.ops.wm.read_factory_settings(use_empty=True)
bpy.ops.import_scene.gltf(filepath=str(SOURCE))
mesh=next(o for o in bpy.context.scene.objects if o.type=='MESH')
assert mesh.matrix_world.is_identity
if PET=='wall-e':
    from tracked_surface import build_surface
    bpy.data.objects.remove(mesh,do_unlink=True)
    mesh=build_surface()
specs=[]
shape_weights=None
normalization=None
def add(name,head,tail,parent=None):
    assert (Vector(tail)-Vector(head)).length>.001,name
    specs.append((name,Vector(head),Vector(tail),parent))

if PET=='rexi':
    centers=[(0,0,.30),(0,0,.38),(0,-.02,.44),(0,-.04,.50),(0,-.10,.54),(0,-.17,.61)]
    names=['DEF-spine','DEF-spine.001','DEF-spine.002','DEF-spine.003','DEF-spine.004','DEF-spine.006']
    for i,name in enumerate(names):
        add(name,centers[i],centers[i+1] if i<5 else (0,-.20,.73),names[i-1] if i else None)
    add('Jaw',(0,-.12,.565),(0,-.43,.51),'DEF-spine.006')
    tail=[(0,.10,.31),(-.04,.24,.29),(-.10,.36,.32),(-.18,.45,.41),(-.20,.49,.50)]
    for i in range(4):
        add(f'Tail.{i+1:02}',tail[i],tail[i+1],f'Tail.{i:02}' if i else 'DEF-spine')
    for side,sign in [('L',1),('R',-1)]:
        leg=[(.14*sign,.0,.31),(.18*sign,-.13,.18),(.18*sign,-.19,.07),(.18*sign,-.42,.04)]
        for i,part in enumerate(['thigh','shin','foot']):
            add(f'DEF-{part}.{side}',leg[i],leg[i+1],f'DEF-{["thigh","shin"][i-1]}.{side}' if i else 'DEF-spine')
        arm=[(.13*sign,-.18,.43),(.17*sign,-.28,.38),(.15*sign,-.36,.34)]
        add(f'DEF-upper_arm.{side}',arm[0],arm[1],'DEF-spine.003')
        add(f'DEF-forearm.{side}',arm[1],arm[2],f'DEF-upper_arm.{side}')
elif PET=='purple-tentacle':
    centers=[(0,0,.06),(0,0,.18),(0,0,.40),(0,0,.62),(.015,0,.79),(.05,0,.90)]
    names=['DEF-spine','DEF-spine.001','DEF-spine.002','DEF-spine.003','DEF-spine.004','DEF-spine.006']
    for i,name in enumerate(names):
        add(name,centers[i],centers[i+1] if i<5 else (.15,0,.92),names[i-1] if i else None)
    add('TentacleTip',(.15,0,.92),(.24,0,.86),'DEF-spine.006')
    for side,sign in [('L',1),('R',-1)]:
        arm=[(.19*sign,-.02,.48),(.31*sign,-.03,.39),(.35*sign,-.05,.29),(.28*sign,-.12,.27)]
        for i,part in enumerate(['upper_arm','forearm','hand']):
            add(f'DEF-{part}.{side}',arm[i],arm[i+1],f'DEF-{["upper_arm","forearm"][i-1]}.{side}' if i else 'DEF-spine.002')
else:
    specs,shape_weights,normalization=build_profile(mesh,catalog[PET]['kind'],PET)

armature=bpy.data.armatures.new(PET+'_Skeleton')
rig=bpy.data.objects.new(PET+'_Rig',armature);bpy.context.scene.collection.objects.link(rig)
bpy.context.view_layer.objects.active=rig;rig.select_set(True)
bpy.ops.object.mode_set(mode='EDIT')
for name,head,tail,parent in specs:
    bone=armature.edit_bones.new(name);bone.head=head;bone.tail=tail
    if parent:
        bone.parent=armature.edit_bones[parent]
bpy.ops.object.mode_set(mode='OBJECT')
groups={name:mesh.vertex_groups.get(name) or mesh.vertex_groups.new(name=name) for name,_,_,_ in specs}
segments={name:(head,tail) for name,head,tail,_ in specs}
def smooth(a,b,value):
    t=max(0,min(1,(value-a)/(b-a)))
    return t*t*(3-2*t)
def distance(point,head,tail):
    segment=tail-head
    along=max(0,min(1,(point-head).dot(segment)/segment.length_squared))
    return (point-(head+segment*along)).length
def closest(point,choices):
    nearest=sorted([(name,distance(point,*segments[name])) for name in choices],key=lambda pair:pair[1])[:2]
    values={name:1/max(.018,d)**4 for name,d in nearest}
    total=sum(values.values())
    return {name:value/total for name,value in values.items()}
def blend(a,b,amount):
    result={name:weight*(1-amount) for name,weight in a.items()}
    for name,weight in b.items():
        result[name]=result.get(name,0)+weight*amount
    return result
def body_weights(z):
    if z<=centers[0][2]:
        return {names[0]:1}
    for i in range(len(centers)-1):
        if z<=centers[i+1][2]:
            t=smooth(centers[i][2],centers[i+1][2],z)
            return {names[i]:1-t,names[i+1]:t}
    return {names[-1]:1}

for vertex in mesh.data.vertices:
    point=vertex.co;x,y,z=point
    side='L' if x>=0 else 'R'
    weights=({mesh.vertex_groups[g.group].name:g.weight for g in vertex.groups} if PET=='wall-e'
             else shape_weights(point) if shape_weights else body_weights(z))
    if PET=='rexi':
        if y>.08:
            weights=blend(weights,closest(point,[f'Tail.{i:02}' for i in range(1,5)]),smooth(.08,.20,y))
        else:
            head=smooth(.48,.58,z)
            weights=blend(weights,{'DEF-spine.006':1},head)
            jaw=smooth(.23,.31,-y)*(1-smooth(.525,.555,z))*smooth(.42,.47,z)
            weights=blend(weights,{'Jaw':1},jaw)
            leg=(1-smooth(.25,.35,z))*smooth(.075,.135,abs(x))
            weights=blend(weights,closest(point,[f'DEF-{part}.{side}' for part in ['thigh','shin','foot']]),leg)
            arm_names=[f'DEF-upper_arm.{side}',f'DEF-forearm.{side}']
            arm_distance=min(distance(point,*segments[name]) for name in arm_names)
            arm=(1-smooth(.055,.10,arm_distance))*smooth(.28,.34,z)*(1-smooth(.46,.50,z))*smooth(.12,.22,-y)
            weights=blend(weights,closest(point,arm_names),arm)
    elif PET=='purple-tentacle':
        # Keep the cups and their surrounding trunk together when the base bends.
        weights=blend({'DEF-spine.001':1},{'DEF-spine.002':1},smooth(.08,.24,z))
        weights=blend(weights,{'DEF-spine.006':1},smooth(.80,.93,z))
        tip=smooth(.09,.18,x)*smooth(.83,.91,z)
        weights=blend(weights,{'TentacleTip':1},tip)
        arm=smooth(.31-.26*z,.39-.26*z,abs(x))*(1-smooth(.08,.18,y))*smooth(.21,.28,z)*(1-smooth(.47,.54,z))
        weights=blend(weights,closest(point,[f'DEF-{part}.{side}' for part in ['upper_arm','forearm','hand']]),arm)
    weights=sorted([(name,value) for name,value in weights.items() if value>.00001],key=lambda pair:pair[1],reverse=True)[:4]
    total=sum(weight for _,weight in weights)
    assert total>0
    for name,weight in weights:
        groups[name].add([vertex.index],weight/total,'REPLACE')

low=Vector([min(v.co[i] for v in mesh.data.vertices) for i in range(3)])
high=Vector([max(v.co[i] for v in mesh.data.vertices) for i in range(3)])
assert abs(low.z)<.001
factor=min(1.75/high.z,1.8/max(high.x-low.x,high.y-low.y))
for vertex in mesh.data.vertices:
    vertex.co*=factor
bpy.context.view_layer.objects.active=rig;bpy.ops.object.mode_set(mode='EDIT')
for bone in armature.edit_bones:
    bone.head*=factor;bone.tail*=factor
bpy.ops.object.mode_set(mode='OBJECT')
mesh.parent=rig;mesh.name=PET+'_Surface'
mesh.modifiers.new('Armature','ARMATURE').object=rig
rig_report={'source_sha256':hashlib.sha256(SOURCE.read_bytes()).hexdigest(),'scale':factor,'source_bounds':[list(low),list(high)],'bones':[{'name':name,'head':list(head),'tail':list(tail),'parent':parent} for name,head,tail,parent in specs]}
if normalization:
    rig_report['source_normalization']=normalization
(ROOT/f'assets/desktop-pets/{PET}/rig.json').write_text(json.dumps(rig_report,indent=2)+'\n',encoding='utf-8',newline='\n')
textures=export_model(ROOT,PET,SOURCE,rig,mesh)
render_thumbnail(ROOT,PET,mesh,textures)
