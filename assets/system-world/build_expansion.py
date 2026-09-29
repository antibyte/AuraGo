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
from math import sin, cos, pi
from mathutils import Matrix, Vector
from build_city import Geometry, create_asset, materials, bounds_and_triangles, ROOT
from living_assets import ASSETS as LIVING_ASSETS, build_living, living_navigation

OUT = ROOT / 'ui/3d/system-world/v2'
PIVOTS = {}
# Agent tower sky deck: the crown terrace sits at 72.2 m, the east lift starts on the
# 0.88 m podium. Mirrored by `skyDeck` in ui/js/desktop/apps/sysworld-layout.js.
SKY_TRAVEL = 71.32


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
        g.part = 'parcel'
        g.box((0, -.52, 1.16), (.72, .4, .52), 'bronze', .1)
        g.box((0, -.735, 1.16), (.5, .025, .05), 'ivory')
        g.part = 'structure'
    elif kind == 'technician':
        g.box((.4, .04, 1), (.15, .2, .2), 'bronze')
        g.pipe((-.2, .26, 1.1), (.2, .26, 1.52), .04, 'jade')
    else:
        g.ring((0, 0, 2.18), .3, .035, 'bronze')


def module(g, kind):
    if kind in ('floor', 'ceiling'):
        g.box((0, 0, -.1 if kind == 'floor' else .1), (4, 4, .2), 'stone' if kind == 'floor' else 'graphite', 0)
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
            for y in (-1.7, 1.7): g.box((x, y, 2.4), (.14, .14, 4.8), 'bronze')
        g.part = 'platform'
        # The top is the declared walk surface at zero; the platform opens to
        # the gallery on its left, with three guards travelling with the car.
        g.box((0, 0, -.12), (3.2, 3.2, .24), 'titanium', 0)
        g.box((1.5, 0, .7), (.06, 3, 1.4), 'glass')
        for y in (-1.5, 1.5): g.box((0, y, .7), (3, .06, 1.4), 'glass')
    elif kind == 'railing':
        for x in (-1.94, 1.94): g.box((x, 0, .55), (.1, .12, 1.1), 'bronze')
        g.box((0, 0, 1.1), (4, .12, .1), 'bronze')
        g.box((0, 0, .55), (3.8, .06, .95), 'glass')
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
            if kind == 'tram':
                # Real sight line from the passenger camera through the end
                # window; an opaque "glass" plate is not a usable windscreen.
                g.box((0,y,1.35),(2.8,.18,1.3),'ceramic')
                for x in (-1.28,1.28): g.box((x,y,2.55),(.24,.18,1.2),'ceramic')
                g.box((0,y,3.16),(2.8,.18,.2),'ceramic')
            else:
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


def glass_band(g, radius, height, mat, start, arc, n):
    # Open, double-sided band (a cylinder without caps) for see-through balustrades.
    verts = [(radius*cos(start+arc*i/n), radius*sin(start+arc*i/n), z) for i in range(n+1) for z in (0, height)]
    faces = [f for i in range(n) for f in ((2*i, 2*i+2, 2*i+3, 2*i+1), (2*i+1, 2*i+3, 2*i+2, 2*i))]
    g.add(verts, faces, mat)


def lookout(g, kind):
    if kind == 'telescope':
        # Public viewer; the tube pivots on its yoke and looks along -Y (glTF +Z) at rest.
        g.cyl((0, 0, .08), .42, .16, 'graphite', n=(24, 16, 10)[g.lod])
        g.cyl((0, 0, .95), .11, 1.6, 'titanium', top=.08)
        g.box((0, 0, 1.78), (.56, .3, .08), 'bronze')
        for x in (-.25, .25): g.box((x, 0, 1.95), (.06, .16, .36), 'bronze')
        g.part = 'tube'
        g.box((0, -.05, 2.05), (.4, .9, .34), 'titanium', .08)
        for x in (-.1, .1):
            g.pipe((x, -.48, 2.06), (x, -.66, 2.06), .12, 'bronze')
            g.pipe((x, -.66, 2.06), (x, -.69, 2.06), .1, 'cyan')
            g.pipe((x, .38, 2.1), (x, .52, 2.1), .055, 'graphite')
    elif kind == 'sky-deck':
        # Crown terrace around the agent tower: paved disc, tapered soffit with a light
        # ring, glass balustrade and a bridge onto the lift landing (+X).
        n, gap = (64, 40, 24)[g.lod], .141
        g.cyl((0, 0, -.25), 8.4, .5, 'stone', n=n)
        g.cyl((0, 0, -1.1), 3.4, 1.2, 'titanium', top=8.2, n=n)
        g.ring((0, 0, -.62), 8.25, .06, 'cyan')
        glass_band(g, 8.2, 1.05, 'garden-glass', gap, 2*pi-2*gap, n)
        g.ring((0, 0, 1.1), 8.2, .05, 'bronze', rotation=Matrix.Rotation(gap, 3, 'Z'), arc=2*pi-2*gap)
        posts = (24, 16, 8)[g.lod]
        for i in range(posts):
            a = gap+(2*pi-2*gap)*i/(posts-1)
            g.box((8.2*cos(a), 8.2*sin(a), .55), (.07, .07, 1.1), 'bronze', 0, angle=a)
        g.ring((0, 0, .12), 3.95, .12, 'graphite')
        g.ring((0, 0, .27), 3.95, .025, 'ivory')
        g.box((8.25, 0, -.15), (.5, 2.3, .3), 'stone', 0)
        for y in (-1.15, 1.15):
            g.box((8.25, y, .55), (.5, .05, 1.05), 'garden-glass', 0)
            g.box((8.25, y, 1.1), (.5, .08, .08), 'bronze', 0)
    elif kind == 'sky-lift':
        # Open lattice shaft (nothing blocks the view) with ties to the tower facade.
        # The boarding side (-Y) opens at the podium, the tower side (-X) at the deck.
        top = SKY_TRAVEL+3.5
        for x in (-1.45, 1.45):
            for y in (-1.45, 1.45): g.box((x, y, top/2), (.16, .16, top), 'bronze', 0)
        step = (4.8, 4.8, 9.6)[g.lod]
        corners = [(-1.45, -1.45), (1.45, -1.45), (1.45, 1.45), (-1.45, 1.45)]
        # Axis-aligned members are unbevelled boxes; only the diagonals need rotated beams.
        for i in range(1, int(top/step)+1):
            z = i*step
            for (ax, ay), (bx, by) in zip(corners, corners[1:]+corners[:1]):
                if ax == bx == -1.45 and SKY_TRAVEL-.3 < z < SKY_TRAVEL+3.4: continue
                g.box(((ax+bx)/2, (ay+by)/2, z), (abs(bx-ax)+.09, abs(by-ay)+.09, .09), 'titanium', 0)
            if g.lod < 2 and z+step <= top:
                g.beam((1.45, -1.45, z), (1.45, 1.45, z+step), .06, 'titanium')
                g.beam((-1.45, 1.45, z), (1.45, 1.45, z+step), .06, 'titanium')
        for z, reach in ((10, 2.45), (25, 3.55), (50, 4.45), (65, 5.35)):
            for y in (-1.45, 1.45): g.box((-1.45-reach/2, y, z), (reach, .2, .2), 'titanium', 0)
        g.box((0, 0, top+.3), (3.3, 3.3, .6), 'graphite')
        g.box((0, 0, top+.64), (1, 1, .08), 'cyan', 0)
        g.box((1.62, -1.62, 1.3), (.3, .3, .5), 'cyan', 0)
        g.part = 'cab'
        # Floor top 3 cm above the podium, so the parked cab never z-fights with it.
        g.box((0, 0, -.08), (2.6, 2.6, .22), 'titanium', 0)
        for x, y, w, d in ((1.28, 0, .05, 2.56), (0, 1.28, 2.56, .05)):
            g.box((x, y, .6), (w, d, 1.1), 'garden-glass', 0)
            g.box((x, y, 1.15), (max(w, .08), max(d, .08), .06), 'bronze', 0)
        for x in (-1.25, 1.25):
            for y in (-1.25, 1.25): g.box((x, y, 1.7), (.07, .07, 3.3), 'bronze', 0)
        g.box((0, 0, 3.42), (2.7, 2.7, .16), 'graphite')
        g.box((0, 0, 3.32), (2.2, 2.2, .03), 'ivory', 0)
        g.box((1.29, -.9, 1.3), (.04, .3, .4), 'cyan', 0)


def animate(root, asset):
    clips = ['idle','walk','turn','greet','work','carry'] if asset.startswith('robot-') else \
        ['open'] if asset in ('door','tram','service-cart') else ['operate'] if asset in ('lift','cooler','hologram',*LIVING_ASSETS[:-1]) else []
    parts = {o.get('component'): o for o in root.children}
    pivots = {'head':(0,0,1.7),'arm_l':(-.47,0,1.62),'arm_r':(.47,0,1.62),
              'leg_l':(-.2,0,.9),'leg_r':(.2,0,.9),'fan':(0,0,3.1),
              'manipulator':(.9,.7,1.3),'antenna':(0,0,6.1),'sculpture':(0,0,1.3),'vent':(0,0,5.05),'tube':(0,0,2.05)}
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
                elif part=='manipulator': obj.rotation_euler.z=wave*.55
                elif part=='antenna': obj.rotation_euler.z=wave*.7
                elif part=='sculpture': obj.rotation_euler.z=2*pi*t
                elif part=='parcel': obj.location.x=base.x+3.2*t
                elif part=='vent': obj.rotation_euler.x=.12+wave*.1
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
    if asset in LIVING_ASSETS: return living_navigation(asset)
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
    elif asset=='railing': data['colliders']=[[-2,0,-.06,2,1.15,.06]]
    elif asset=='telescope': data['colliders']=[[-.42,0,-.42,.42,2.3,.42]]
    elif asset=='sky-deck':
        # Walking is an annulus around the crown plus the lift bridge; the disc itself is a solid below.
        data['colliders']=[[-8.3,-1.7,-8.3,8.3,0,8.3]]
        data['surfaces']=[{'rect':[-8,-8,8,8],'height':0,'inner':4.2,'outer':8,'bridge':[8,-1,8.5,1]}]
    elif asset=='sky-lift':
        data['colliders']=[[x-.1,0,z-.1,x+.1,SKY_TRAVEL+4.1,z+.1] for x in (-1.45,1.45) for z in (-1.45,1.45)]
        data['surfaces']=[{'rect':[-1.3,-1.3,1.3,1.3],'height':.03,'component':'cab','travel':SKY_TRAVEL}]
    elif asset=='station':
        data['colliders']=[[-4.5,3.69,-2,4.5,3.91,2]]+[[x-.075,0,-1.575,x+.075,3.8,-1.425] for x in (-4,4)]
        data['surfaces']=[{'rect':[-4.5,-2,4.5,2],'height':.3}]
    elif asset=='garden': data['colliders']=[[-2,0,-2,2,3.5,2]]
    elif asset=='arcade': data['colliders']=[[-3.25,0,-1,-2.75,4.8,1],[2.75,0,-1,3.25,4.8,1],[-3.25,4.44,-1.2,3.25,4.925,1.2]]
    elif asset=='service-cart': data['colliders']=[[-1.5,0,-1.65,1.5,3.4,1.65]]
    elif asset in ('console','cargo','hologram','bench','charger','cooler','archive-shelf'):
        size={'console':(.85,.6,.5),'cargo':(.9,.6,.65),'hologram':(1.3,1.25,1.3),'bench':(1.5,.75,.4),'charger':(.4,1.2,.3),'cooler':(1.5,1.5,1.25),'archive-shelf':(1.25,1.75,.5)}[asset]
        x,y,z=size;data['colliders']=[[-x,0,-z,x,y*2,z]]
    return data


def motion_bounds(root, clips):
    bounds,_=bounds_and_triangles(root)
    scene=bpy.context.scene
    moving=[o for o in root.children if o.animation_data]
    bases=[(o,o.location.copy(),o.rotation_euler.copy()) for o in moving]
    for clip in clips:
        for o in moving:
            track=next((t for t in o.animation_data.nla_tracks if t.name==clip),None)
            o.animation_data.action=track.strips[0].action if track else None
        for frame in range(1,50):
            scene.frame_set(frame);bpy.context.view_layer.update()
            current,_=bounds_and_triangles(root)
            for i in range(3):
                bounds['min'][i]=min(bounds['min'][i],current['min'][i])
                bounds['max'][i]=max(bounds['max'][i],current['max'][i])
    for o,location,rotation in bases:
        o.animation_data.action=None;o.location=location;o.rotation_euler=rotation
    scene.frame_set(1);bpy.context.view_layer.update()
    # Small margin covers interpolation between the sampled export frames.
    return {'min':[v-.025 for v in bounds['min']], 'max':[v+.025 for v in bounds['max']]}


def build():
    OUT.mkdir(parents=True, exist_ok=True)
    bpy.ops.object.select_all(action='SELECT');bpy.ops.object.delete(use_global=False)
    scene=bpy.context.scene;scene.render.fps=24;scene.frame_start=1;scene.frame_end=49
    collection=bpy.data.collections.new('AURAGO_WORLD_2');scene.collection.children.link(collection)
    mats=materials(); entries=[]; roots=[]
    glass=mats['glass'].copy();glass.name='city.garden-glass';glass.diffuse_color=(.18,.42,.4,.2)
    glass.node_tree.nodes['Principled BSDF'].inputs['Base Color'].default_value=(.18,.42,.4,.2)
    glass.node_tree.nodes['Principled BSDF'].inputs['Alpha'].default_value=.2
    glass.surface_render_method='DITHERED';mats['garden-glass']=glass
    factories={k:(lambda g,k=k:module(g,k)) for k in ['arcade','bridge','stairs','ramp','garden','quay','floor','wall','ceiling','window','door','lift','railing']}
    factories.update({k:(lambda g,k=k:furnishing(g,k)) for k in ['tram','station','service-cart','pad','console','hologram','charger','cargo','cooler','bench','archive-shelf']})
    factories.update({'robot-'+k:(lambda g,k=k:robot(g,k)) for k in ['courier','technician','archivist']})
    factories.update({k:(lambda g,k=k:build_living(g,k)) for k in LIVING_ASSETS})
    factories.update({k:(lambda g,k=k:lookout(g,k)) for k in ['telescope','sky-deck','sky-lift']})
    for asset,factory in factories.items():
        entry={'id':asset,'lods':[], 'navigation':navigation(asset)}
        for lod in range(3):
            root=create_asset(asset,factory,lod,collection,mats);clips=animate(root,asset)
            if lod==0 and asset.startswith('robot-'): entry['motion_bounds']=motion_bounds(root,clips)
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
    manifest={'schema_version':2,'version':'2.2.0','license':'MIT','units':'metres','up_axis':'Y','front_axis':'Z',
              'generator':'Blender '+bpy.app.version_string,'assets':entries,'budget_bytes':48*1024*1024,'model_budget_bytes':12*1024*1024,
              'sources':{name:hashlib.sha256((ROOT/'assets/system-world'/name).read_bytes()).hexdigest() for name in ['build_city.py','build_expansion.py','living_assets.py']}}
    (OUT/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n',encoding='utf8',newline='\n')
    (OUT/'LICENSE.txt').write_text((ROOT/'LICENSE').read_text(encoding='utf8'),encoding='utf8')
    for i,root in enumerate(roots): root.location=((i%5)*16,(i//5)*16,0)
    source=ROOT/'assets/system-world/production';source.mkdir(exist_ok=True)
    bpy.data.libraries.write(str(source/'aurago-world-2.blend'),{scene},compress=True)
    total=sum(p.stat().st_size for p in (ROOT/'ui/3d/system-world').rglob('*') if p.is_file())
    assert total<=12*1024*1024,total
    print('WORLD2_COMPLETE',len(entries),total)


if __name__=='__main__': build()
