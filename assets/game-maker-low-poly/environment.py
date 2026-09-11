"""Original landscape and vegetation. Deterministic facets, metric connectors."""
from math import sin, cos, pi, tau, sqrt
from random import Random
from mathutils import Matrix, Vector
from geometry import Mesh
from architecture import architecture, connection, railing


def landscape(entry):
    g=Mesh(entry['id']);n=entry['design'];rng=Random(n)
    if n.startswith('road'):
        if n=='road-roundabout':
            for i in range(48):
                a,b=i*tau/48,(i+1)*tau/48
                g.add([(r*cos(t),.12,r*sin(t)) for r,t in ((5,a),(13,a),(13,b),(5,b))],[(0,1,2,3)],'road')
                if i%2:g.beam((9*cos(a),.14,9*sin(a)),(9*cos(b),.14,9*sin(b)),.12,'white',.025,.002)
            g.cylinder((0,0,0),(0,.15,0),5,'leaf_dark',sides=32)
            for j in range(4):
                a=j*pi/2;connection(g,'lane-'+str(j),(13*cos(a),.12,13*sin(a)),(cos(a),0,sin(a)),8)
        elif n=='road-curve':
            for i in range(20):
                a,b=i*pi/40,(i+1)*pi/40
                g.add([(-8+r*cos(t),.12,-8+r*sin(t)) for r,t in ((4,a),(12,a),(12,b),(4,b))],[(0,1,2,3)],'road')
                if i%2:g.beam((-8+8*cos(a),.14,-8+8*sin(a)),(-8+8*cos(b),.14,-8+8*sin(b)),.12,'white',.025,.002)
            connection(g,'entry',(0,.12,-8),(0,0,-1),8);connection(g,'exit',(-8,.12,0),(-1,0,0),8)
        else:
            ramp=n=='road-ramp';bridge=n=='road-bridge'
            height=lambda z:3 if bridge else (z+4)*.375 if ramp else .12
            g.add([(-4,height(-4),-4),(4,height(-4),-4),(4,height(4),4),(-4,height(4),4)],[(0,1,2,3)],'road')
            if n in ('road-tee','road-cross'):
                for x in (-4,4):connection(g,'side-'+str(x),(x,.12,0),(x/4,0,0),8)
            for z in (-3,-1,1,3):
                if n not in ('road-tee','road-cross'):g.beam((0,height(z)+.015,z-.5),(0,height(z)+.015,z+.5),.12,'white',.018,.002)
                for x in (-3.75,3.75):
                    if n not in ('road-tee','road-cross'):g.beam((x,height(z)+.015,z-1),(x,height(z)+.015,z+1),.09,'white',.015,.002)
            if n=='road-tee':g.box((0,.14,3.8),(8,.02,.13),'white',.002)
            if bridge:
                for x in (-4.15,4.15):
                    railing(g,(x,3,-4),(x,3,4),1.2)
                    for z in (-3.5,3.5):g.box((x,1.5,z),(.55,3,.8),'stone')
            if n=='road-end':
                for x in (-3,-1,1,3):g.box((x,.7,3.8),(1.5,.35,.2),'yellow')
            connection(g,'entry',(0,height(-4),-4),(0,0,-1),8)
            if n not in ('road-end','road-tee'):connection(g,'exit',(0,height(4),4),(0,0,1),8)
            g.colliders.append(dict(type='ramp' if ramp else 'box',center=[0,(height(4)+height(-4))/2-.1,0],size=[8,3 if ramp else .2,8]))
        if not g.colliders:g.colliders.append(dict(type='mesh',node=g.id+'__body'))
    elif n.startswith('sidewalk') or n.startswith('path'):
        paved=n.startswith('sidewalk');curve=n.endswith('corner') or n.endswith('curve')
        for i in range(8):
            a=i*pi/14 if curve else 0
            x=(3*cos(a)-3) if curve else 0;z=(3*sin(a)-1.5) if curve else i*.5-1.75
            g.box((x,.09,z),(2,.18,.48),'stone' if paved else 'soil',.015,Matrix.Rotation(-a,3,'Y') if curve else None)
            if paved:g.box((x-.93,.17,z),(.1,.12,.48),'ivory',.01)
    elif n.startswith('river') or n=='lake':
        if n=='lake':
            for i in range(24):
                a,b=i*tau/24,(i+1)*tau/24
                g.add([(0,.05,0),(5*cos(a),.05,4*sin(a)),(5*cos(b),.05,4*sin(b))],[(0,1,2)],'water',material=2)
            for i in range(20):
                a=i*tau/20;g.ellipsoid((5*cos(a),.02,4*sin(a)),(.6,.15,.5),'sand',6,4)
        else:
            for i in range(12):
                a=i*pi/22 if n=='river-curve' else 0
                c=Vector((3*cos(a)-3,0,3*sin(a)-1.5)) if n=='river-curve' else Vector((0,0,i*.5-2.75))
                g.box(c+Vector((0,.05,0)),(3,.06,.53),'water',.01,Matrix.Rotation(-a,3,'Y'),2)
                for s in (-1,1):
                    if n!='river-tee' or s<0 or abs(c.z)>1.5:g.ellipsoid(c+Vector((s*1.7,.02,0)),(.42,.25,.34),'soil',7,4)
            if n=='river-tee':g.box((2,.05,0),(4,.06,3),'water',.01,material=2)
    elif n in ('rock-arch','cave'):
        for s in (-1,1):g.ellipsoid((s*2,1.6,0),(1,2,1.5),'stone',7,5)
        for i in range(7):
            a=pi*i/6;g.ellipsoid((2*cos(a),2+1.9*sin(a),0),(.8,.65,1.2),'stone',7,5)
        if n=='cave':
            for s in (-1,1):g.ellipsoid((s*3,1.2,-1.7),(1.8,1.9,2),'soil',7,5)
        g.colliders.append(dict(type='mesh',node=g.id+'__body'))
    elif n=='landing-pad':
        g.cylinder((0,0,0),(0,.25,0),5,'navy',sides=12)
        for x in (-1,1):g.box((x,.26,0),(.28,.02,2.8),'white',.002)
        g.box((0,.26,0),(2.2,.02,.28),'white',.002)
        for i in range(8):
            a=i*pi/4;g.box((4.2*cos(a),.3,4.2*sin(a)),(.22,.13,.22),'cyan',.02,material=3)
    else:
        res=8;verts=[];faces=[]
        for iz in range(res+1):
            for ix in range(res+1):
                x=(ix/res-.5)*8;z=(iz/res-.5)*8;r=sqrt(x*x+z*z)
                noise=rng.uniform(-.22,.22) if ix not in (0,res) and iz not in (0,res) else 0
                y={'ground':0,'hill':2.8*max(0,1-r/5)**1.3,'mountain':6*max(0,1-r/5),
                   'dune':1.8*max(0,1-r/5)*(1+.3*sin(x)),'slope':(z+4)*.4,
                   'cliff':3 if z<0 else .1,'cliff-corner':3 if x<0 and z<0 else .1,
                   'island':2*max(0,1-r/5),'crag':3*max(0,1-r/4)*(1+.3*sin(x*2))}[n]
                verts.append((x,max(0,y+noise),z))
        for z in range(res):
            for x in range(res):
                a=z*(res+1)+x;b=a+res+1;faces.extend([(a,b,a+1),(a+1,b,b+1)])
        for f in faces:
            y=sum(verts[i][1] for i in f)/3
            paint='snow' if n=='mountain' and y>3.6 else 'sand' if n in ('dune','island') and y<.5 else 'stone' if n in ('cliff','cliff-corner','crag','mountain') else 'sand' if n=='dune' else rng.choice(['leaf','leaf_dark','green'])
            g.add([verts[i] for i in f],[(0,1,2)],paint)
        g.colliders.append(dict(type='mesh',node=g.id+'__body'))
    return g


def leaf(g,a,b,width,paint):
    a,b=Vector(a),Vector(b);v=b-a;across=v.cross(Vector((0,1,0)))
    if across.length<.01:across=Vector((1,0,0))
    across.normalize();mid=a+v*.48
    verts=[a,mid+across*width,mid+Vector((0,width*.22,0)),mid-across*width,b]
    g.add(verts,[(0,1,2),(0,2,3),(1,4,2),(2,4,3)],paint)
    g.add(verts,[(2,1,0),(3,2,0),(2,4,1),(3,4,2)],paint)


def vegetation(entry):
    g=Mesh(entry['id']);n=entry['design'];rng=Random(n)
    if n in ('oak','birch','pine','spruce','palm','acacia','baobab','alien-tree'):
        h={'oak':5,'birch':6,'pine':7,'spruce':6,'palm':6,'acacia':4.5,'baobab':6,'alien-tree':5}[n]
        trunk='white' if n=='birch' else 'purple' if n=='alien-tree' else 'bark'
        radius=.65 if n=='baobab' else .18 if n=='birch' else .3
        points=[(0,0,0),(.08,h*.3,0),(-.1,h*.65,.12),(.15,h*.92,0)]
        for i in range(3):g.cylinder(points[i],points[i+1],radius*(1-i*.22),trunk,radius*(.78-i*.18),sides=7)
        if n=='birch':
            for j in range(12):g.box((.08*sin(j),j*.35+.2,.15),(.2,.06,.035),'ink',.005)
        if n in ('pine','spruce'):
            for j in range(5 if n=='pine' else 7):
                y=1.2+j*.7;r=(h-y)*.34
                g.cylinder((0,y,0),(0,y+1.9,0),r,'leaf_dark' if j%2 else 'green',0,sides=9)
        elif n=='palm':
            for i in range(10):
                a=i*tau/10;tip=Vector((2.4*cos(a),h-.8,2.4*sin(a)));base=Vector(points[-1])
                mid=base*.4+tip*.6+Vector((0,.7,0));g.beam(base,mid,.045,'leaf_dark')
                for j in range(7):
                    p=base.lerp(mid,j/7)
                    for s in (-1,1):
                        end=p+Vector((cos(a+s*pi/2)*.55,-.15,sin(a+s*pi/2)*.55))+(tip-base)*.12
                        leaf(g,p,end,.12,'leaf' if j%2 else 'leaf_light')
                leaf(g,mid,tip,.28,'leaf')
            for i in range(3):g.ellipsoid((.2*cos(i*2),h*.87,.2*sin(i*2)),(.14,.18,.14),'bark',7,5)
        else:
            count=10 if n=='oak' else 7 if n=='baobab' else 8
            for i in range(count):
                a=i*2.399;spread=2.5 if n in ('acacia','baobab','oak') else 1.5
                y=h*(.68+.24*rng.random()) if n!='acacia' else h*.88
                end=(cos(a)*spread*(.4+.6*rng.random()),y,sin(a)*spread*(.4+.6*rng.random()))
                g.cylinder((0,h*.5,0),end,.11,trunk,.04,7)
                r=1.35 if n=='acacia' else 1
                g.ellipsoid(end,(r,.48 if n=='acacia' else .95,r),('purple' if i%2 else 'cyan') if n=='alien-tree' else rng.choice(['leaf','leaf_light','leaf_dark']),7,5)
        g.colliders.append(dict(type='capsule',center=[0,h/2,0],size=[radius*2,h,radius*2]))
    elif n.startswith('cactus'):
        if n=='cactus-paddle':
            for i in range(6):
                x=.32*sin(i*2);y=.3+i*.28
                g.ellipsoid((x,y,0),(.32,.48,.12),'green',8,6)
                for j in range(4):g.box((x+.15*sin(j*2),y+.2*cos(j*2),.12),(.015,.04,.015),'ivory',.002)
        else:
            g.cylinder((0,.15,0),(0,2.3,0),.25,'green',.2,10)
            g.ellipsoid((0,2.3,0),(.2,.22,.2),'leaf',10,5)
            if n=='cactus-branch':
                for s,y in [(-1,1.1),(1,.8)]:
                    g.cylinder((0,y,0),(s*.65,y,0),.15,'green',sides=8)
                    g.cylinder((s*.65,y,0),(s*.65,y+.9,0),.15,'green',.12,8)
                    g.ellipsoid((s*.65,y+.9,0),(.12,.14,.12),'leaf',8,5)
            for i in range(8):
                a=i*tau/8
                for j in range(6):g.beam((.25*cos(a),.3+j*.3,.25*sin(a)),(.29*cos(a),.34+j*.3,.29*sin(a)),.013,'ivory',bevel=.001)
    elif n in ('stump','log'):
        a,b=((0,.05,0),(0,.75,0)) if n=='stump' else ((-1.4,.3,0),(1.4,.3,0))
        g.cylinder(a,b,.35,'bark',.29,10)
        direction=(Vector(b)-Vector(a)).normalized()
        g.cylinder(Vector(b),Vector(b)+direction*.015,.255,'wood',sides=10)
        for r in (.08,.16,.23):g.ring(Vector(b)+direction*.02,r,.007,'bark','Y' if n=='stump' else 'X',12)
        if n=='stump':
            for i in range(5):g.cylinder((0,.25,0),(.7*cos(i*tau/5),.02,.7*sin(i*tau/5)),.16,'bark',.035,6)
    elif n in ('bush-round','bush-tall','hedge','alien-bush'):
        for i in range(7):
            x=(i-3)*.26 if n=='hedge' else rng.uniform(-.4,.4);z=rng.uniform(-.25,.25)
            y=.45+(i%3)*.3 if n=='bush-tall' else .4
            g.cylinder((x,0,z),(x,y,z),.04,'bark',sides=6)
            g.ellipsoid((x,y,z),(.42,.7 if n=='bush-tall' else .45,.42),'purple' if n=='alien-bush' else rng.choice(['leaf','leaf_light','leaf_dark']),7,5)
    else:
        for i in range(18 if n!='fern' else 10):
            a=i*2.4;x=rng.uniform(-.55,.55);z=rng.uniform(-.55,.55)
            h=(.25 if n=='grass-short' else 1.2 if n=='reeds' else .55)*rng.uniform(.65,1.2)
            tip=(x+.2*cos(a),h,z+.2*sin(a))
            if n in ('flowers','reeds'):
                g.cylinder((x,0,z),tip,.013,'leaf_dark',sides=5)
                if n=='reeds':g.cylinder(tip,Vector(tip)+Vector((0,.2,0)),.04,'bark',sides=7)
                else:
                    for j in range(5):leaf(g,tip,Vector(tip)+Vector((.11*cos(j*tau/5),.015,.11*sin(j*tau/5))),.04,'pink' if i%2 else 'cream')
                    g.ellipsoid(tip,(.035,.025,.035),'yellow',6,4)
            elif n=='fern':
                end=Vector((.9*cos(a),.5,.9*sin(a)))
                for j in range(7):
                    p=end*j/8
                    for s in (-1,1):leaf(g,p,p+Vector((.22*cos(a+s*pi/2),.04,.22*sin(a+s*pi/2))),.07,'leaf')
            else:leaf(g,(x,0,z),tip,.045,'leaf' if i%2 else 'leaf_light')
    return g


def environment(entry):
    if entry['category']=='props':
        from props import props
        return props(entry)
    return {'architecture':architecture,'landscape':landscape,'vegetation':vegetation}[entry['category']](entry)
