"""Original street-level machinery and garden architecture for the living city."""
from math import sin, cos, pi
from mathutils import Matrix

ASSETS = ('repair-bay', 'parcel-sorter', 'relay-mast', 'kinetic-fountain', 'glass-garden', 'meeting-charge')


def fasteners(g, x, y, z, w, d):
    if g.lod > 0:
        return
    for sx in (-1, 1):
        for sy in (-1, 1):
            g.cyl((x+sx*w/2, y+sy*d/2, z), .045, .025, 'bronze', n=8)


def cabinet(g, x, y, w=1.1, h=1.5):
    g.box((x, y, h/2), (w, .7, h), 'titanium', .09)
    g.box((x, y-.37, h*.68), (w*.72, .045, .3), 'glass')
    g.box((x, y-.401, h*.7), (w*.5, .015, .035), 'jade', 0)
    if g.lod < 2:
        for z in (.22, .31, .4, .49):
            g.box((x, y-.365, z), (w*.62, .025, .027), 'graphite', 0)
        for side in (-1, 1):
            g.pipe((x+side*w*.36,y-.38,h*.32),(x+side*w*.36,y-.38,h*.56),.025,'bronze')


def build_living(g, kind):
    if kind == 'repair-bay':
        g.box((0,0,.13),(5,3.8,.26),'graphite',.1)
        for x in (-2.2,2.2):
            g.box((x,0,.29),(.12,3.3,.05),'ivory',0)
        cabinet(g,-1.8,.9,1,1.8)
        g.cyl((.9,.7,.6),.55,1.2,'titanium')
        g.ring((.9,.7,1.13),.53,.06,'bronze')
        fasteners(g,.9,.7,.3,.85,.85)
        g.part='manipulator'
        g.cyl((.9,.7,1.3),.28,.3,'graphite')
        g.beam((.9,.7,1.35),(.15,.7,2.8),.28,'ceramic')
        g.pipe((1.03,.56,1.42),(.3,.56,2.62),.06,'bronze')
        g.cyl((.15,.7,2.8),.25,.4,'titanium',rotation=Matrix.Rotation(pi/2,3,'X'))
        g.beam((.15,.7,2.8),(-.5,-.5,2.2),.22,'ceramic')
        g.pipe((.2,.7,2.99),(-.5,-.5,2.39),.045,'graphite')
        for x in (-.68,-.32):
            g.beam((x,-.5,2.2),(x,-.5,1.82),.075,'bronze')
        g.cyl((-.5,-.5,2.15),.16,.12,'cyan')
    elif kind == 'parcel-sorter':
        for x in (-2.7,2.7):
            for y in (-.7,.7):
                g.box((x,y,.55),(.22,.22,1.1),'titanium')
        g.box((0,0,1.05),(6,1.9,.25),'graphite')
        for y in (-.88,.88):
            g.box((0,y,1.27),(6.3,.14,.25),'bronze')
        for i in range((-12 if g.lod==0 else -6),(13 if g.lod==0 else 7)):
            x=i*(.23 if g.lod==0 else .46)
            g.cyl((x,0,1.22),.09,1.65,'titanium',rotation=Matrix.Rotation(pi/2,3,'X'),n=8)
        for y in (-1,1):
            g.box((.5,y,2.15),(.23,.25,2.2),'ceramic')
        g.box((.5,0,3.2),(.32,2.25,.28),'ceramic')
        g.box((.5,0,3.04),(.08,1.8,.05),'cyan',0)
        cabinet(g,2,1.45,.8,1.65)
        g.part='parcel'
        g.box((-1.6,0,1.65),(.7,.82,.64),'bronze',.06)
        for y in (-.25,.25):
            g.box((-1.6,y,1.99),(.71,.045,.025),'ivory',0)
    elif kind == 'relay-mast':
        g.cyl((0,0,.2),1.6,.4,'stone',n=24)
        for a in range(4):
            t=a*pi/2
            g.beam((cos(t)*1.2,sin(t)*1.2,.35),(cos(t)*.2,sin(t)*.2,4.8),.14,'bronze')
        g.cyl((0,0,3),.24,6,'titanium')
        cabinet(g,0,.9,1.2,1.6)
        for z in (2.2,3.6,5):
            g.ring((0,0,z),.4,.065,'cyan')
        g.part='antenna'
        # A shallow dish with concentric ribs and an offset feed horn.
        for i in range(1,6 if g.lod<2 else 4):
            r=i*.24
            g.ring((0,-.45+r*r*.16,6.1),r,.045,'ceramic',rotation=Matrix.Rotation(pi/2,3,'X'))
        for a in range(8 if g.lod<2 else 4):
            t=a*pi/(4 if g.lod<2 else 2)
            g.beam((0,-.45,6.1),(1.2*cos(t),-.22,6.1+1.2*sin(t)),.05,'titanium')
        g.pipe((0,-.4,6.1),(0,-1.5,6.1),.055,'bronze')
        g.cyl((0,-1.55,6.1),.13,.2,'cyan',rotation=Matrix.Rotation(pi/2,3,'X'))
    elif kind == 'kinetic-fountain':
        g.cyl((0,0,.18),3,.36,'stone')
        g.ring((0,0,.46),2.85,.22,'titanium')
        g.ring((0,0,.56),2.63,.035,'cyan')
        g.cyl((0,0,.35),2.62,.04,'glass')
        g.cyl((0,0,.85),.55,1,'bronze')
        g.part='sculpture'
        for z,r in ((1.3,1.15),(2,.8),(2.6,.4)):
            g.ring((0,0,z),r,.08,'ceramic',rotation=Matrix.Rotation(.3,3,'X'))
        g.part='structure'
        for a in range(8):
            t=a*pi/4
            g.cyl((2.3*cos(t),2.3*sin(t),.6),.12,.25,'bronze')
    elif kind == 'glass-garden':
        g.box((0,0,.15),(5.4,4.4,.3),'stone')
        for x in (-2.5,2.5):
            for y in (-2,2):
                g.box((x,y,1.9),(.14,.14,3.8),'bronze')
        for y in (-2,0,2):
            g.beam((-2.5,y,3.8),(0,y,5),.12,'ceramic')
            g.beam((0,y,5),(2.5,y,3.8),.12,'ceramic')
        for x in (-2.5,0,2.5):
            g.beam((x,-2,5 if x==0 else 3.8),(x,2,5 if x==0 else 3.8),.1,'bronze')
        # Open front and translucent roof panes keep the planting visible.
        for side in (-1,1):
            g.add([(0,-2,5),(side*2.5,-2,3.8),(side*2.5,2,3.8),(0,2,5)],[(0,1,2,3)],'garden-glass')
        for x in (-1.4,1.4):
            g.box((x,0,.48),(1.45,3.1,.6),'graphite')
            for j in range(3):
                y=j-1;h=1.4+(j%2)*.8
                g.pipe((x,y,.75),(x+.15,y,h+1),.055,'bronze')
                for k in range(6 if g.lod<2 else 3):
                    t=k*2.4;z=.95+k*h/(6 if g.lod<2 else 3)
                    g.add([(x,y,z),(x+cos(t)*.8,y+sin(t)*.65,z+.25),(x+cos(t+.4)*.9,y+sin(t+.4)*.7,z+.45),(x,y,z+.3)],[(0,1,2,3)],'leaf')
        g.part='vent'
        g.box((0,0,5.05),(1,1,.07),'titanium')
    elif kind == 'meeting-charge':
        g.box((0,0,.12),(6.5,4,.24),'stone')
        for x in (-3,3):
            g.box((x,1.5,2.4),(.24,.3,4.8),'bronze')
        g.box((0,0,4.7),(6.6,4.2,.23),'titanium',.1)
        for x in (-2,0,2):
            g.box((x,0,4.55),(1.2,2.7,.04),'ivory',0)
        for x in (-2,2):
            cabinet(g,x,1.3,.8,2)
            g.ring((x,0,.27),.85,.035,'jade')
            if g.lod<2:
                g.pipe((x+.45,1.3,.8),(x+.65,.5,.5),.035,'graphite')
        g.box((0,1.3,.75),(2.2,.75,.18),'bronze')
        for x in (-.85,.85):
            g.box((x,1.3,.37),(.12,.5,.74),'graphite')
    g.part='structure'


def living_navigation(kind):
    boxes={
        'repair-bay':[[-2.5,0,-1.9,2.5,3.15,1.9]],
        'parcel-sorter':[[-3.2,0,-1.1,3.2,3.35,1.1],[1.5,0,-1.9,2.5,1.8,-1.05]],
        'relay-mast':[[-1.65,0,-1.65,1.65,7.5,1.65]],
        'kinetic-fountain':[[-3.1,0,-3.1,3.1,3.1,3.1]],
        'glass-garden':[[-2.7,3.8,-2.2,2.7,5.2,2.2],[-2.13,.3,-1.55,-.67,3.1,1.55],[.67,.3,-1.55,2.13,3.1,1.55]]+
                       [[x-.1,.3,z-.1,x+.1,3.8,z+.1] for x in (-2.5,2.5) for z in (-2,2)],
        'meeting-charge':[[-3.3,4.45,-2.1,3.3,4.9,2.1],[-3.2,0,-1.7,-2.7,4.5,-1.3],[2.7,0,-1.7,3.2,4.5,-1.3],[-2.5,0,-1.8,2.5,2.2,-.85]],
    }
    z=4.5 if kind in ('kinetic-fountain','glass-garden') else 3.4
    floors={'repair-bay':([-2.5,-1.9,2.5,1.9],.18),'glass-garden':([-2.7,-2.2,2.7,2.2],.3),'meeting-charge':([-3.25,-2,3.25,2],.24)}
    surfaces=[{'rect':floors[kind][0],'height':floors[kind][1]}] if kind in floors else []
    return {'colliders':boxes[kind],'surfaces':surfaces, 'portals':[],
            'workpoints':[{'id':'left','position':[-1.3,0,z]},{'id':'right','position':[1.3,0,z]}],
            'interaction':[{'position':[0,1.5,z],'radius':4}],
            'emitter':[{'position':[0,2.2,0]}]}
