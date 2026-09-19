"""Original MIT System World 2 assets, authored and animated in Blender.

Run: blender --background --factory-startup --python assets/system-world/build_expansion.py
Coordinates in this authoring file are Blender Z-up; exports use metres, Y-up, +Z forward.
"""
import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))
import bpy
import json
import hashlib
import struct
from math import sin, pi
from mathutils import Matrix, Vector
from build_city import Geometry, create_asset, materials, bounds_and_triangles, ROOT

OUT = ROOT / 'ui/3d/system-world/v2'
PIVOTS = {}


def robot(g, kind):
    accent = {'courier': 'bronze', 'technician': 'jade', 'archivist': 'cyan'}[kind]
    g.box((0, 0, 1.25), (.65, .4, .72), 'ceramic', .12)
    g.box((0, -.22, 1.3), (.42, .04, .28), accent)
    g.cyl((0, 0, .91), .2, .12, 'graphite')
    g.part = 'head'
    g.box((0, 0, 1.91), (.58, .43, .42), 'ceramic', .13)
    g.box((0, -.235, 1.93), (.45, .06, .17), 'glass')
    for x in (-.12, .12):
        g.box((x, -.273, 1.94), (.065, .018, .055), accent, 0)
    for side in (-1, 1):
        g.part = 'arm_l' if side < 0 else 'arm_r'
        g.cyl((side*.47, 0, 1.58), .13, .18, 'graphite')
        g.box((side*.47, 0, 1.3), (.22, .25, .62), 'ceramic', .09)
        g.box((side*.47, -.015, .96), (.21, .24, .16), 'titanium')
        g.part = 'leg_l' if side < 0 else 'leg_r'
        g.box((side*.2, 0, .5), (.25, .3, .76), 'titanium', .06)
        g.box((side*.2, -.12, .1), (.3, .5, .2), 'graphite')
    g.part = 'structure'
    if kind == 'courier':
        g.box((0, .32, 1.3), (.72, .4, .64), 'bronze', .1)
        g.box((0, .535, 1.3), (.5, .025, .05), 'ivory')
    elif kind == 'technician':
        g.box((.4, .04, 1), (.15, .2, .2), 'bronze')
        g.pipe((-.2, .26, 1.1), (.2, .26, 1.52), .04, 'jade')
    else:
        g.ring((0, 0, 2.18), .3, .035, 'bronze')


def module(g, kind):
    if kind in ('floor', 'ceiling'):
        g.box((0, 0, -.1 if kind == 'floor' else .1), (4, 4, .2), 'stone' if kind == 'floor' else 'graphite')
        if kind == 'floor':
            for x in (-1.8, 1.8): g.box((x, 0, .012), (.035, 3.6, .02), 'bronze', 0)
        else:
            g.box((0, 0, -.025), (2.8, .14, .04), 'ivory')
    elif kind in ('wall', 'window'):
        if kind == 'wall': g.box((0, 0, 2), (4, .25, 4), 'titanium')
        else:
            g.box((0, 0, .5), (4, .25, 1), 'titanium')
            g.box((0, 0, 3.75), (4, .25, .5), 'titanium')
            for x in (-1.9, 0, 1.9): g.box((x, 0, 2.3), (.14, .3, 3), 'bronze')
    elif kind == 'door':
        for x in (-1.8, 1.8): g.box((x, 0, 2), (.4, .4, 4), 'titanium')
        g.box((0, 0, 3.8), (4, .4, .4), 'bronze')
        for side in (-1, 1):
            g.part = 'door_l' if side < 0 else 'door_r'
            g.box((side*.8, 0, 1.8), (1.6, .16, 3.6), 'graphite')
            g.box((side*.8, -.09, 2.3), (1.1, .02, .75), 'glass')
            g.box((side*.09, -.095, 1.8), (.035, .02, 2.8), 'cyan', 0)
    elif kind in ('stairs', 'ramp'):
        if kind == 'stairs':
            for i in range(12): g.box((0, 2.75-i*.5, (i+1)/12), (3, .5, (i+1)/6), 'stone')
        else:
            g.add([(-1.5,-3,0),(1.5,-3,0),(-1.5,3,0),(1.5,3,0),(-1.5,-3,2),(1.5,-3,2)],
                  [(0,1,3,2),(0,4,5,1),(0,2,4),(1,5,3),(4,2,3,5)], 'stone')
        for side in (-1, 1): g.beam((side*1.5,3,1),(side*1.5,-3,3), .08, 'bronze')
    elif kind == 'lift':
        for x in (-1.7, 1.7):
            g.box((x, 0, 2.4), (.14, 3.6, 4.8), 'bronze')
        g.part = 'platform'
        g.box((0, 0, .12), (3.2, 3.2, .24), 'titanium')
        for x in (-1.5, 1.5): g.box((x, 0, 1), (.06, 3, 1.4), 'glass')
    elif kind == 'arcade':
        for x in (-3, 3): g.box((x, 0, 2.4), (.5, 2, 4.8), 'titanium')
        g.box((0, 0, 4.7), (6.5, 2.4, .45), 'bronze')
        g.box((0, 0, 4.44), (5.5, .1, .04), 'ivory')
    elif kind == 'bridge':
        g.box((0, 0, -.12), (4, 12, .24), 'titanium')
        for x in (-2, 2):
            g.box((x, 0, .8), (.12, 12, 1.6), 'glass')
            g.box((x, 0, 1.6), (.12, 12, .08), 'bronze')
    elif kind == 'quay':
        g.box((0, 0, -.25), (8, 6, .5), 'stone')
        for x in (-3.5, 0, 3.5): g.cyl((x, -2.8, .5), .13, 1, 'bronze')
        g.beam((-3.5,-2.8,.9),(3.5,-2.8,.9), .09, 'bronze')
    elif kind == 'garden':
        g.box((0, 0, .2), (4, 4, .4), 'stone')
        for x,y,h in [(-1,-1,2),(.8,-.6,2.8),(.3,1,1.6)]:
            g.cyl((x,y,h/2), .09, h, 'bronze')
            for z in (.55,.85,1): g.cyl((x,y,h*z), .65, h*.5, 'leaf', top=.15, n=8)


def furnishing(g, kind):
    if kind == 'archive-shelf':
        for x in (-1.2,1.2): g.box((x,0,1.75),(.14,1,3.5),'bronze')
        for z in (.2,1.25,2.3,3.4):
            g.box((0,0,z),(2.5,1,.12),'graphite')
            if z<3:
                for x in (-.85,-.28,.28,.85):
                    g.box((x,0,z+.4),(.42,.72,.62),'titanium')
                    g.box((x,-.37,z+.43),(.22,.025,.035),'cyan',0)
    elif kind == 'console':
        g.box((0,0,.6), (1.7,.85,1.2), 'graphite')
        g.box((0,-.2,1.23), (1.5,.7,.07), 'cyan')
        for x in (-.5,0,.5): g.box((x,-.45,1.3), (.15,.15,.08), 'ivory')
    elif kind == 'hologram':
        g.cyl((0,0,.5), 1.3, 1, 'graphite')
        g.ring((0,0,1.02), 1.1, .05, 'cyan')
        g.part = 'display'
        for z,r in ((1.6,.6),(2,.45),(2.4,.2)): g.ring((0,0,z),r,.035,'jade')
    elif kind == 'charger':
        g.box((0,0,1.2), (.7,.5,2.4), 'titanium')
        g.box((0,-.26,1.7), (.45,.03,.5), 'jade')
        g.pipe((.38,0,.5),(.6,-.1,1.7),.05,'graphite')
    elif kind == 'cargo':
        g.box((0,0,.6), (1.8,1.3,1.2), 'bronze')
        for x in (-.7,.7): g.box((x,0,.6), (.09,1.34,1.24), 'graphite')
    elif kind == 'cooler':
        g.box((0,0,1.5), (3,2.5,3), 'titanium')
        g.part = 'fan'
        for angle in (0,pi/2): g.box((0,0,3.1), (2,.2,.1), 'graphite', angle=angle)
        g.part = 'structure'; g.ring((0,0,3.1),1.15,.12,'bronze')
    elif kind == 'bench':
        g.box((0,0,.7),(3,.8,.2),'bronze')
        g.box((0,.35,1.15),(3,.14,.8),'titanium')
        for x in (-1.1,1.1): g.box((x,0,.35),(.15,.6,.7),'graphite')
    elif kind == 'station':
        g.box((0,0,.15),(9,4,.3),'stone')
        for x in (-4,4): g.box((x,1.5,1.9),(.15,.15,3.8),'bronze')
        g.box((0,0,3.8),(9,4,.22),'titanium')
        g.box((0,1.5,2.7),(3,.1,.6),'cyan')
    elif kind == 'pad':
        g.cyl((0,0,.15),4,.3,'stone',n=32)
        g.ring((0,0,.33),3.2,.08,'cyan')
        g.box((0,0,.34),(3,.25,.04),'ivory')
    elif kind in ('tram', 'service-cart'):
        length = 8 if kind == 'tram' else 3
        g.box((0,0,.4),(2.8,length,.8),'graphite')
        g.box((0,0,.84),(2.8,length,.12),'stone')
        for y in (-length/2+.2,length/2-.2):
            g.box((0,y,2),(2.8,.18,2.5),'ceramic')
            g.box((0,y-.1,2.4),(2.4,.04,.8),'glass')
        for x in (-1.35,1.35):
            for y in (-length*.36,length*.36):
                g.box((x,y,2),(.12,length*.23,2.5),'ceramic')
                g.box((x,y,2.4),(.14,length*.19,.8),'glass')
            g.part='door_l' if x<0 else 'door_r'
            g.box((x,0,2),(.12,1.6,2.3),'glass'); g.part='structure'
            g.box((x*.65,0,1.25),(.6,length*.65,.2),'bronze')
        g.box((0,0,3.3),(3,length,.18),'graphite')
        for x in (-.8,.8): g.box((x,-length/2-.1,1.1),(.4,.05,.15),'ivory')


def animate(root, asset):
    clips = ['idle','walk','turn','greet','work','carry'] if asset.startswith('robot-') else \
        ['open'] if asset in ('door','tram','service-cart') else ['operate'] if asset in ('lift','cooler','hologram') else []
    parts = {o.get('component'): o for o in root.children}
    pivots = {'head':(0,0,1.7),'arm_l':(-.47,0,1.62),'arm_r':(.47,0,1.62),
              'leg_l':(-.2,0,.9),'leg_r':(.2,0,.9),'fan':(0,0,3.1)}
    for part, pos in pivots.items():
        if part in parts:
            obj=parts[part];obj.data.transform(Matrix.Translation(-Vector(pos)));obj.location=pos
    for part,obj in parts.items():
        if part=='structure': continue
        base=obj.location.copy();obj.animation_data_clear()
        for clip in clips:
            obj.rotation_euler=(0,0,0);obj.location=base
            for frame in range(1,50,4):
                t=(frame-1)/48;wave=sin(t*pi*2);side=-1 if part.endswith('_l') else 1
                obj.rotation_euler=(0,0,0);obj.location=base
                if asset.startswith('robot-'):
                    if clip in ('walk','carry'):
                        if part.startswith('leg'): obj.rotation_euler.x=wave*side*.55
                        if part.startswith('arm'): obj.rotation_euler.x=(-.8 if clip=='carry' else -wave*side*.45)
                    if clip=='idle' and part=='head': obj.rotation_euler.z=wave*.08
                    if clip=='turn' and part=='head': obj.rotation_euler.z=wave*.5
                    if clip=='greet' and part=='arm_r': obj.rotation_euler.y=-1.8+wave*.4
                    if clip=='work' and part.startswith('arm'): obj.rotation_euler.x=-.8+wave*.3
                elif clip=='open':
                    if asset=='door': obj.location.x=base.x+side*1.6*t
                    else: obj.location.y=base.y+1.7*t
                elif asset=='lift': obj.location.z=base.z+4*t
                elif part in ('fan','display'): obj.rotation_euler.z=2*pi*t
                obj.keyframe_insert(data_path='rotation_euler',frame=frame)
                obj.keyframe_insert(data_path='location',frame=frame)
            action=obj.animation_data.action;action.name=asset+'.'+part+'.'+clip
            track=obj.animation_data.nla_tracks.new();track.name=clip
            track.strips.new(clip,1,action);track.mute=True
            obj.animation_data.action=None;obj.rotation_euler=(0,0,0);obj.location=base
    return clips


def navigation(asset):
    # Coordinates use exported glTF metres (Y up). These helpers are also kept as
    # named Blender empties so authors can inspect movement separately from art.
    data={'colliders':[], 'surfaces':[], 'portals':[]}
    if asset in ('floor','ceiling'):
        if asset=='floor': data['surfaces']=[{'rect':[-2,-2,2,2], 'height':0}]
    elif asset=='door':
        data['colliders']=[[-2,0,-.2,-1.6,4,.2],[1.6,0,-.2,2,4,.2]]
        data['portals']=[{'name':'entrance','width':3.2,'height':3.6,'animation':'open'}]
    elif asset=='lift':
        data['surfaces']=[{'rect':[-1.6,-1.6,1.6,1.6],'height':0,'component':'platform','travel':4}]
    elif asset in ('ramp','stairs'):
        data['surfaces']=[{'rect':[-1.5,-3,1.5,3],'from_height':0,'to_height':2,'axis':'+Z'}]
    elif asset=='bridge': data['surfaces']=[{'rect':[-1.8,-6,1.8,6],'height':0}]
    elif asset=='tram':
        data['surfaces']=[{'rect':[-.55,-3.6,.55,3.6],'height':.9}]
        data['portals']=[{'name':'boarding-left','position':[-1.4,.9,0]},{'name':'boarding-right','position':[1.4,.9,0]}]
    elif asset in ('wall','window'): data['colliders']=[[-2,0,-.15,2,4,.15]]
    elif asset in ('console','cargo','hologram','bench','charger','cooler','archive-shelf'):
        size={'console':(.85,.6,.5),'cargo':(.9,.6,.65),'hologram':(1.3,1.25,1.3),'bench':(1.5,.75,.4),'charger':(.4,1.2,.3),'cooler':(1.5,1.5,1.25),'archive-shelf':(1.25,1.75,.5)}[asset]
        x,y,z=size;data['colliders']=[[-x,0,-z,x,y*2,z]]
    return data


def build():
    OUT.mkdir(parents=True, exist_ok=True)
    bpy.ops.object.select_all(action='SELECT');bpy.ops.object.delete(use_global=False)
    scene=bpy.context.scene;scene.render.fps=24;scene.frame_start=1;scene.frame_end=49
    collection=bpy.data.collections.new('AURAGO_WORLD_2');scene.collection.children.link(collection)
    mats=materials(); entries=[]; roots=[]
    factories={k:(lambda g,k=k:module(g,k)) for k in ['arcade','bridge','stairs','ramp','garden','quay','floor','wall','ceiling','window','door','lift']}
    factories.update({k:(lambda g,k=k:furnishing(g,k)) for k in ['tram','station','service-cart','pad','console','hologram','charger','cargo','cooler','bench','archive-shelf']})
    factories.update({'robot-'+k:(lambda g,k=k:robot(g,k)) for k in ['courier','technician','archivist']})
    for asset,factory in factories.items():
        entry={'id':asset,'lods':[], 'navigation':navigation(asset)}
        for lod in range(3):
            root=create_asset(asset,factory,lod,collection,mats);clips=animate(root,asset)
            for kind,items in entry['navigation'].items():
                for i,item in enumerate(items):
                    marker=bpy.data.objects.new(f'nav_{kind}_{i}',None);collection.objects.link(marker);marker.parent=root
                    marker['component']=f'nav_{kind}_{i}';marker['navigation']=json.dumps(item,sort_keys=True);marker.empty_display_type='CUBE';marker.empty_display_size=.4
            bpy.ops.object.select_all(action='DESELECT');root.select_set(True)
            for obj in root.children: obj.select_set(True)
            bpy.context.view_layer.objects.active=root;bpy.context.view_layer.update()
            bounds,triangles=bounds_and_triangles(root)
            output=OUT/f'{asset}.lod{lod}.glb'
            bpy.ops.export_scene.gltf(filepath=str(output),export_format='GLB',use_selection=True,
                export_animations=bool(clips),export_animation_mode='NLA_TRACKS',export_nla_strips_merged_animation_name='Animation',
                export_extras=True,export_yup=True,export_cameras=False,export_lights=False)
            data=output.read_bytes();jlen=struct.unpack_from('<I',data,12)[0];doc=json.loads(data[20:20+jlen])
            for n in doc.get('nodes',[]): n['name']=n.get('extras',{}).get('component',n.get('extras',{}).get('asset_id',n.get('name')))
            payload=json.dumps(doc,separators=(',',':'),sort_keys=True).encode();payload+=b' '*(-len(payload)%4);tail=data[20+jlen:]
            data=struct.pack('<III',0x46546c67,2,20+len(payload)+len(tail))+struct.pack('<II',len(payload),0x4e4f534a)+payload+tail
            output.write_bytes(data)
            actual=sorted(a['name'] for a in doc.get('animations',[]))
            assert set(clips)==set(actual),(asset,clips,actual)
            entry['lods'].append({'level':lod,'file':output.name,'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest(),'bounds':bounds,'triangles':triangles})
            entry['animations']=actual;entry['components']=[o.get('component') for o in root.children]
            if lod==0: roots.append(root)
            else:
                for o in list(root.children): bpy.data.objects.remove(o,do_unlink=True)
                bpy.data.objects.remove(root,do_unlink=True)
        entries.append(entry);print('WORLD2',asset)
    manifest={'schema_version':2,'version':'2.0.0','license':'MIT','units':'metres','up_axis':'Y','front_axis':'Z',
              'generator':'Blender '+bpy.app.version_string,'assets':entries,'budget_bytes':48*1024*1024}
    (OUT/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n',encoding='utf8')
    (OUT/'LICENSE.txt').write_text((ROOT/'LICENSE').read_text(encoding='utf8'),encoding='utf8')
    for i,root in enumerate(roots): root.location=((i%5)*16,(i//5)*16,0)
    source=ROOT/'assets/system-world/production';source.mkdir(exist_ok=True)
    bpy.data.libraries.write(str(source/'aurago-world-2.blend'),{scene},compress=True)
    total=sum(p.stat().st_size for p in (ROOT/'ui/3d/system-world').rglob('*') if p.is_file())
    assert total<=48*1024*1024,total
    print('WORLD2_COMPLETE',len(entries),total)


if __name__=='__main__': build()
