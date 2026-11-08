"""Build all AuraPinball geometry and collision records from factory startup."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import sys
import bpy

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(Path(__file__).parent / 'kit'))
import geometry as g
import heroes

PALETTES = {
    'neon': ((.015,.015,.034),(.08,.78,1),(.95,.035,.48),(.65,.49,.25)),
    'leviathan': ((.008,.035,.055),(.05,.75,.65),(.35,.18,.48),(.62,.39,.14)),
    'temple': ((.04,.027,.012),(1,.52,.08),(1,.18,.015),(.68,.40,.10)),
}


def materials(table):
    dark, accent, secondary, gold = PALETTES[table]
    g.material('Obsidian',dark,.65,.24)
    g.material('Chrome',(.65,.73,.80),1,.16)
    g.material('Ramp',(.38,.46,.55),1,.22)
    g.material('Gold',gold,.88,.25)
    g.material('Accent',accent,.35,.23,2.1)
    g.material('Secondary',secondary,.25,.25,1.8)
    g.material('Rubber',(.065,.055,.075),0,.62)
    g.material('Stone',(.16,.19,.17) if table=='leviathan' else (.19,.16,.10),.1,.78)
    g.material('Creature',(.015,.18,.20),.62,.26)
    g.material('Ivory',(.65,.72,.66),.3,.30)
    g.material('Leaf',(.02,.16,.06),.1,.50)
    g.material('Water',(.015,.27,.30),.4,.17,.5)
    g.material('Glass',(.31,.63,.73),.1,.12,0,.8)
    g.material('TubeGlass',(.65,.80,.84),0,.08,0,.96)
    g.material('Playfield',(1,1,1),.25,.25)
    g.material('Backglass',(1,1,1),0,.6,0)


def floor():
    # A welded floor grid leaves an actual pocket opening.
    xs,zs=[-.28,-.040,.040,.274],[.075,1.060,1.14,1.20]
    rectangles=[(xs[x],xs[x+1],zs[z],zs[z+1]) for x in range(3) for z in range(3) if (x,z)!=(1,1)]
    vertices, faces=[],[]
    for x0,x1,z0,z1 in rectangles:
        n=len(vertices)
        vertices.extend([(x0,0,z0),(x1,0,z0),(x1,0,z1),(x0,0,z1)])
        faces.append((n,n+3,n+2,n+1))
    obj=g.mesh('printed-playfield',vertices,faces,'Playfield','wood')
    uv=obj.data.uv_layers.new(name='UVMap')
    for loop in obj.data.loops:
        co=obj.data.vertices[loop.vertex_index].co
        uv.data[loop.index].uv=((co.x+.28)/.554,(-co.y-.075)/1.125)
    g.box('shooter-floor',(.301,-.006,.62),(.055,.012,1.16),'Ramp','metal')
    g.box('pocket-floor',(0,-.066,1.10),(.08,.012,.08),'Obsidian','metal')
    for x in [-.043,.043]: g.box('pocket-wall',(x,-.025,1.10),(.006,.08,.09),'Gold','metal')
    for z in [1.057,1.143]: g.box('pocket-wall',(0,-.025,z),(.08,.08,.006),'Gold','metal')
    g.torus('pocket-rim',(0,.002,1.10),.044,.003,'Gold')


def cabinet(table):
    g.box('cabinet',(0,-.11,.60),(.69,.19,1.32),'Obsidian',bevel=.012)
    for x in [-.331,.331]:
        g.box('side-chrome',(x,-.007,.61),(.018,.036,1.30),'Chrome',bevel=.004)
        g.box('side-inlay',(x,.013,.61),(.004,.003,1.27),'Gold',bevel=.001)
    g.box('lockbar',(0,-.003,-.035),(.69,.05,.045),'Chrome',bevel=.009)
    g.box('backbox',(0,.255,1.295),(.69,.42,.055),'Obsidian',bevel=.009)
    for x in [-.338,.338]: g.box('backbox-trim',(x,.255,1.261),(.012,.42,.008),'Gold')
    for y in [.05,.46]: g.box('backbox-trim',(0,y,1.261),(.68,.01,.008),'Gold')
    obj=g.mesh('backglass',[(-.32,.12,1.26),(.32,.12,1.26),(.32,.45,1.26),(-.32,.45,1.26)],[(0,1,2,3)],'Backglass')
    uv=obj.data.uv_layers.new(name='UVMap')
    for loop in obj.data.loops: uv.data[loop.index].uv=[(0,0),(1,0),(1,1),(0,1)][loop.vertex_index]
    for x in [-.28,.28]:
        g.cylinder('speaker',(x,.08,1.25),.025,.008,'Chrome')
    # A closed upper border curves the shooter exit onto the main playfield.
    outer=[(-.277,0,.08),(-.277,0,1.09)]
    outer += [(.015+.318*math.cos(math.pi-i*math.pi/24),0,1.09+.11*math.sin(math.pi-i*math.pi/24)) for i in range(25)]
    outer +=[(.333,0,.08)]
    g.rail('outer-wall',g.smooth_path(outer,3),.055,.006)
    g.rail('shooter-divider',[(.271,0,.10),(.271,0,.88),(.267,0,1.03),(.245,0,1.08)],.038,.004)
    g.rail('shooter-stop',[(.273,0,.075),(.331,0,.075)],.035,.005)
    # Inner return guides stop before the flipper swing envelope.
    for side in [-1,1]:
        g.rail('return-guide',[(side*.230,0,.46),(side*.235,0,.28),(side*.105,0,.235)],.025,.004)
        g.rail('outlane-guide',[(side*.270,0,.42),(side*.264,0,.18),(side*.225,0,.08)],.024,.004)
        g.rail('sling-guide',[(side*.194,0,.385),(side*.130,0,.295),(side*.139,0,.246)],.023,.004)
        for z in [.27,.35,.43]:
            g.cylinder('post',(side*.237,.018,z),.006,.036,'Gold',vertices=16)
            g.torus('post-rubber',(side*.237,.02,z),.007,.002,'Rubber')
    for x in [-.25,.25]:
        for z in [.06,.33,.61,.89,1.17]:
            g.cylinder('rail-screw',(x,.004,z),.003,.003,'Chrome',vertices=12)


def mechanics(table):
    flippers=[]
    for name,at,side,length in [('left',[-.113,.018,.189],1,.092),('right',[.113,.018,.189],-1,.092)]+([('upper',[-.195,.148,.766],1,.075)] if table=='temple' else []):
        obj,vertices=g.flipper('flipper-'+name,at,side,length)
        flippers.append({'id':name,'at':at,'side':side,'vertices':[v for point in vertices for v in point]})
        g.cylinder('flipper-pivot',(at[0],at[1]-.011,at[2]),.016,.014,'Gold')
    bumpers=[]
    for i,at in enumerate([[-.078,0,.89],[.078,0,.89],[0,0,.995]]):
        if table=='temple': at[2]-=.08
        g.cylinder('bumper-foot',(at[0],.008,at[2]),.037,.016,'Gold')
        g.cylinder('bumper-skirt',(at[0],.021,at[2]),.031,.028,'Rubber')
        g.cylinder('bumper-band',(at[0],.040,at[2]),.035,.008,'Accent')
        cap=g.sphere('bumper-'+str(i),(at[0],.05,at[2]),(.036,.012,.036),'Chrome')
        cap['pinball_actor']='bumper-'+str(i)
        g.torus('bumper-light',(at[0],.058,at[2]),.025,.0025,'Secondary')
        bumpers.append({'id':'bumper-'+str(i),'at':[at[0],.024,at[2]],'radius':.032})
    targets=[]
    for i,x in enumerate([-.065,0,.065]):
        at=[x,.022,1.029]
        g.box('target-'+str(i),at,(.030,.042,.009),'Secondary',bevel=.002,dynamic=True)
        targets.append({'id':'target-'+str(i),'at':at})
    gate=g.box('gate',(0,.037,1.055),(.092,.070,.008),'Gold',bevel=.002,dynamic=True)
    g.box('plunger',(.301,.018,.13),(.035,.022,.015),'Chrome',bevel=.003,dynamic=True)
    for i in range(12): g.torus('plunger-spring',(.301,.014,.08-i*.005),.010,.001,'Chrome')
    spinner=g.box('spinner',(-.215,.036,.91),(.027,.044,.003),'Gold',bevel=.001,dynamic=True)
    rod=g.rod('spinner-axle',(-.236,.057,.91),(-.194,.057,.91),.0018,'Chrome')
    for side in [-1,1]:
        for i in range(3):
            z=.49+i*.075
            points=[(side*.17,.001,z),(side*.15,.001,z+.025),(side*.19,.001,z+.025)]
            g.mesh('shot-lamp-'+('left' if side<0 else 'right')+'-'+str(i),points,[(0,1,2)],'Accent',dynamic=True)
    return flippers,bumpers,targets


def ramps(table):
    if table=='neon':
        left=[(-.16,0,.57),(-.205,.045,.78),(-.15,.13,.99),(.10,.16,1.00),(.213,.12,.80),(.211,.065,.48),(.18,0,.27)]
        right=[(.15,0,.60),(.19,.065,.83),(.10,.23,1.12),(-.10,.22,.98),(-.218,.13,.75),(-.217,.06,.45),(-.18,0,.25)]
    elif table=='leviathan':
        left=[(-.17,0,.55),(-.22,.03,.72)]
        left += [(-.115+.105*math.cos(a),.04+.10*i/24,.87+.105*math.sin(a)) for i,a in enumerate([-math.pi/2+i*math.tau/24 for i in range(25)])]
        left +=[(.06,.16,1.04),(.22,.12,.88),(.23,.07,.52),(.18,0,.27)]
        right=[(.15,0,.58),(.21,.06,.77),(.13,.20,1.10),(-.12,.21,1.08),(-.237,.15,.91),(-.235,.07,.53),(-.18,0,.25)]
    else:
        left=[(-.15,0,.52),(-.23,.055,.61),(-.23,.125,.77),(-.21,.13,.89),(-.13,.13,.91)]
        right=[(.15,0,.57),(.215,.065,.77),(.15,.18,1.10),(-.05,.19,1.10),(-.25,.15,.96),(-.255,.07,.54),(-.18,0,.25)]
        g.box('upper-playfield',(-.155,.120,.83),(.19,.02,.23),'Gold','wood',bevel=0)
        g.rail('upper-left-wall',[(-.25,.13,.72),(-.25,.13,.95),(-.06,.13,.95)],.025,.004)
        g.ramp('bridge',[(-.11,.13,.80),(-.04,.14,.85),(.06,.14,.88),(.14,.12,.98),(.045,.015,1.10)],.050)
    lp=g.ramp('left-ramp',left,.054,table=='leviathan')
    rp=g.ramp('right-ramp',right,.054,table!='temple',table=='leviathan')
    sensors=[{'id':'left-ramp','type':'ramp','at':list(lp[len(lp)//2]),'half':[.035,.035,.025]}, {'id':'right-ramp','type':'ramp','at':list(rp[len(rp)//2]),'half':[.035,.035,.025]}, {'id':'orbit-left','type':'orbit','at':[-.237,.028,1.075],'half':[.029,.035,.018]}, {'id':'skill','type':'skill','at':[-.16,.028,1.16],'half':[.025,.03,.018]}, {'id':'lock','type':'lock','at':[0,-.025,1.10],'half':[.027,.025,.027]}, {'id':'spinner','type':'spinner','at':[-.215,.023,.91],'half':[.022,.028,.009]}]
    if table=='temple': sensors.append({'id':'upper-shot','type':'upper','at':[-.13,.16,.88],'half':[.035,.035,.018]})
    return sensors


def bake_maps(out, table):
    # The flat print has physical lacquer micro-normal and geometry-derived AO.
    obj=next(o for o in bpy.context.scene.objects if o.name=='printed-playfield')
    mat=g.MATERIALS['Playfield']; nodes=mat.node_tree.nodes; links=mat.node_tree.links
    bsdf=nodes.get('Principled BSDF')
    noise=nodes.new('ShaderNodeTexNoise'); noise.inputs['Scale'].default_value=900
    bump=nodes.new('ShaderNodeBump'); bump.inputs['Strength'].default_value=.10; bump.inputs['Distance'].default_value=.00008
    links.new(noise.outputs['Fac'],bump.inputs['Height']); links.new(bump.outputs['Normal'],bsdf.inputs['Normal'])
    scene=bpy.context.scene; scene.render.engine='CYCLES'; scene.cycles.samples=8
    bpy.ops.object.select_all(action='DESELECT'); obj.select_set(True); bpy.context.view_layer.objects.active=obj
    for kind in ['NORMAL','AO']:
        image=bpy.data.images.new(table+'-'+kind.lower(),width=512,height=1024,alpha=False)
        node=nodes.new('ShaderNodeTexImage'); node.image=image; nodes.active=node
        scene.render.bake.margin=4
        bpy.ops.object.bake(type=kind)
        image.filepath_raw=str(out/(table+'-'+kind.lower()+'.png')); image.file_format='PNG'; image.save()
        nodes.remove(node)
    links.remove(bsdf.inputs['Normal'].links[0])


def build(table,out,bake):
    bpy.ops.object.select_all(action='SELECT'); bpy.ops.object.delete(use_global=False)
    for collection in [bpy.data.meshes,bpy.data.curves,bpy.data.actions,bpy.data.armatures,bpy.data.cameras,bpy.data.lights]:
        for block in list(collection):
            if block.users==0: collection.remove(block)
    for mat in list(bpy.data.materials): bpy.data.materials.remove(mat)
    g.COLLIDERS.clear(); g.MATERIALS.clear()
    materials(table); floor(); cabinet(table)
    flippers,bumpers,targets=mechanics(table)
    sensors=ramps(table); heroes.build(table)
    collision=g.collision_data()
    if bake: bake_maps(out,table)
    physics={'version':1,'id':table,'meshes':collision,'flippers':flippers,'bumpers':bumpers,'targets':targets,'sensors':sensors,'shooter':[.301,.015,.155]}
    if table=='leviathan': physics['magnet']={'at':[0,.014,.76],'radius':.06,'force':.65,'hold':.9}
    text=json.dumps(physics,separators=(',',':'),sort_keys=True)+'\n'
    (out/(table+'-physics.json')).write_text(text,encoding='utf-8',newline='\n')
    g.merge_static()
    bpy.context.scene['pinball_collision_sha256']=hashlib.sha256(text.encode()).hexdigest()
    bpy.context.scene['pinball_table']=table
    bpy.ops.wm.save_as_mainfile(filepath=str(Path(__file__).parent/'build'/(table+'.blend')))
    bpy.ops.export_scene.gltf(filepath=str(out/(table+'.glb')),export_format='GLB',export_extras=True,export_animations=True,export_cameras=False,export_lights=False,export_yup=True,export_apply=True)
    from glb import canonicalize
    canonicalize(out/(table+'.glb'))
    print('PINBALL_TABLE',table,'collider_vertices',sum(len(m['vertices'])//3 for m in collision),flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser()
    parser.add_argument('--table',choices=list(PALETTES));parser.add_argument('--no-bake',action='store_true')
    parser.add_argument('--out',default=str(ROOT/'ui/3d/pinball/v1'))
    args=parser.parse_args(sys.argv[sys.argv.index('--')+1:] if '--' in sys.argv else [])
    out=Path(args.out).resolve();out.mkdir(parents=True,exist_ok=True)
    (Path(__file__).parent/'build').mkdir(exist_ok=True)
    for table in [args.table] if args.table else PALETTES: build(table,out,not args.no_bake)
