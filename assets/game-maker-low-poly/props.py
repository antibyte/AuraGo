"""Original compact props: storage, streets, furniture, industry, gameplay."""
from math import sin, cos, pi, tau
from mathutils import Matrix
from geometry import Mesh


def props(entry):
    g=Mesh(entry['id']);n=entry['design']
    if n in ('crate-wood','crate-metal','supply-case','ammo-box'):
        metal=n!='crate-wood';paint='teal' if metal else 'wood'
        w,h,d=(1,1,1) if n.startswith('crate') else (.8,.4,.5)
        g.box((0,h/2,0),(w,h,d),paint,.04)
        for s in (-1,1):
            if metal:
                for x in (-w*.43,w*.43):g.box((x,h/2,s*(d/2+.01)),(.08,h,.055),'steel')
                g.box((0,h*.57,s*(d/2+.04)),(.3,.1,.07),'ink')
            else:
                for y in (.09,h-.09):g.box((0,y,s*(d/2+.02)),(w,.14,.065),'bark')
                g.beam((-w*.4,h*.2,s*(d/2+.02)),(w*.4,h*.8,s*(d/2+.02)),.12,'bark',.04)
        g.moving_part('lid',(0,h,-d/2),'hinge',(1,0,0))
        g.box((0,h+.04,0),(w+.04,.1,d+.04),paint);g.part='body'
    elif n in ('barrel-wood','barrel-metal'):
        wood=n.endswith('wood')
        g.cylinder((0,.08,0),(0,.9,0),.37,'wood' if wood else 'red',sides=14)
        for y in (.1,.28,.72,.91):g.ring((0,y,0),.37,.035,'steel',segments=14)
        g.cylinder((0,.91,0),(0,.94,0),.35,'bark' if wood else 'red',sides=14)
        g.cylinder((.17,.94,0),(.17,.96,0),.06,'ink',sides=8)
        if wood:
            for i in range(14):
                a=i*tau/14;g.beam((.373*cos(a),.13,.373*sin(a)),(.373*cos(a),.88,.373*sin(a)),.012,'bark',bevel=.001)
    elif n=='pallet':
        for x in (-.5,0,.5):g.box((x,.1,0),(.12,.2,1),'bark')
        for j in range(6):g.box((0,.23,-.46+j*.184),(1.2,.08,.14),'wood',.007)
    elif n=='container':
        # Open interior with separate hinged end doors.
        for x in (-1.22,1.22):
            g.box((x,1.3,0),(.08,2.6,6.06),'blue',.02)
            for j in range(18):g.box((x,1.3,-2.9+j*.34),(.15,2.38,.045),'steel',.008)
        for y in (.05,2.55):g.box((0,y,0),(2.44,.1,6.06),'blue')
        g.box((0,1.3,-3),(2.44,2.6,.08),'blue')
        for s in (-1,1):
            g.moving_part('door_'+str(s),(s*1.2,0,3.04),'hinge')
            g.box((s*.6,1.3,3.04),(1.17,2.45,.07),'blue')
            g.beam((s*.6,.1,3.1),(s*.6,2.5,3.1),.045,'silver')
        g.part='body'
        for at,size in [((-1.22,1.3,0),(.08,2.6,6.06)),((1.22,1.3,0),(.08,2.6,6.06)),((0,.05,0),(2.44,.1,6.06)),((0,2.55,0),(2.44,.1,6.06)),((0,1.3,-3),(2.44,2.6,.08))]:
            g.colliders.append(dict(type='box',center=list(at),size=list(size)))
    elif n=='fuel-tank':
        g.cylinder((0,1,-1.6),(0,1,1.6),.85,'ivory',sides=14)
        for z in (-1.4,1.4):
            g.ellipsoid((0,1,z),(.85,.85,.3),'ivory',14,7)
            g.box((0,.2,z),(1.7,.4,.4),'steel')
        g.cylinder((0,1.7,0),(0,2,0),.18,'steel');g.ring((0,2.06,0),.22,.04,'red',segments=12)
    elif n=='street-lamp':
        g.cylinder((0,0,0),(0,4,0),.08,'navy',.05,10)
        g.beam((0,4,0),(.7,4,0),.09,'navy');g.box((.75,3.96,0),(.55,.12,.28),'steel')
        g.box((.75,3.89,0),(.45,.025,.23),'cream',.01,material=3);g.box((0,.12,0),(.3,.24,.3),'stone')
        g.socket('light',(.75,3.85,0))
    elif n=='bench':
        for x in (-.65,.65):
            for z in (-.22,.22):g.beam((x,0,z),(x,.46,z),.07,'steel')
        for j in range(4):g.box((0,.46,-.23+j*.155),(1.7,.065,.13),'wood',.01)
        for j in range(3):g.box((0,.65+j*.13,-.28),(1.7,.1,.07),'wood',.01)
        for s in (-1,1):g.socket('seat_'+str(s),(s*.45,.5,0))
    elif n=='bin':
        g.cylinder((0,0,0),(0,.85,0),.28,'teal',.34,12)
        g.ring((0,.85,0),.34,.05,'steel',segments=12);g.box((0,.94,-.12),(.6,.07,.4),'steel')
    elif n=='hydrant':
        g.cylinder((0,.05,0),(0,.85,0),.16,'red',sides=10)
        g.ellipsoid((0,.83,0),(.2,.17,.2),'red',10,6);g.cylinder((-.3,.55,0),(.3,.55,0),.1,'red',sides=10)
        for s in (-1,1):g.cylinder((s*.3,.55,0),(s*.34,.55,0),.13,'silver',sides=8)
        g.ring((0,.08,0),.23,.04,'ink',segments=10)
    elif n=='cone':
        g.box((0,.04,0),(.5,.08,.5),'rubber',.025)
        g.cylinder((0,.08,0),(0,.7,0),.21,'orange',.035,12);g.cylinder((0,.34,0),(0,.46,0),.145,'white',.11,12)
    elif n=='barrier':
        for x in (-.8,.8):g.box((x,.25,0),(.15,.5,.5),'steel')
        g.box((0,.7,0),(2,.45,.16),'white')
        for x in (-.75,-.25,.25,.75):g.box((x,.7,.088),(.24,.44,.01),'orange',.002,Matrix.Rotation(-.4,3,'Z'))
    elif n=='sign':
        g.cylinder((0,0,0),(0,2.1,0),.04,'steel',sides=8);g.box((0,1.75,0),(1,.65,.08),'blue')
        for a,b in [((-.3,1.75,.05),(.25,1.75,.05)),((.1,1.9,.05),(.3,1.75,.05)),((.1,1.6,.05),(.3,1.75,.05))]:g.beam(a,b,.045,'white')
    elif n=='fence':
        for x in (-1,1):g.box((x,.7,0),(.12,1.4,.12),'bark')
        for j in range(9):g.box((-.9+j*.225,.7,0),(.14,1.15,.08),'wood',.02)
        for y in (.35,1.05):g.box((0,y,-.05),(2,.1,.08),'bark')
    elif n in ('table','computer-desk'):
        g.box((0,.76,0),(1.6,.08,.8),'wood')
        for x in (-.68,.68):
            for z in (-.3,.3):g.box((x,.37,z),(.07,.74,.07),'steel')
        if n=='computer-desk':
            g.box((0,.95,-.15),(.08,.32,.08),'ink');g.box((0,1.15,-.18),(.68,.42,.05),'ink')
            g.box((0,1.15,-.148),(.61,.35,.008),'cyan',.01,material=3);g.box((0,.82,.14),(.48,.025,.18),'navy')
    elif n=='chair':
        for x in (-.2,.2):
            for z in (-.2,.2):g.box((x,.23,z),(.055,.46,.055),'steel')
        g.box((0,.46,0),(.5,.07,.5),'wood')
        for x in (-.2,.2):g.beam((x,.45,-.21),(x,.95,-.21),.05,'steel')
        g.box((0,.83,-.21),(.5,.25,.065),'wood');g.socket('seat',(0,.5,0))
    elif n=='sofa':
        g.box((0,.36,0),(2,.45,.85),'teal',.1);g.box((0,.73,-.35),(2,.6,.22),'teal',.08)
        for x in (-.87,.87):g.box((x,.6,0),(.25,.36,.9),'teal',.07)
        for x in (-.43,.43):g.box((x,.62,.06),(.8,.18,.67),'blue',.065)
        for x in (-.8,.8):
            for z in (-.28,.28):g.box((x,.08,z),(.1,.16,.1),'bark')
    elif n=='bed':
        g.box((0,.26,0),(1.5,.3,2.2),'wood');g.box((0,.5,0),(1.44,.24,2.12),'white',.09)
        g.box((0,.64,.4),(1.45,.08,1.25),'blue',.035)
        for x in (-.36,.36):g.ellipsoid((x,.68,-.67),(.32,.12,.23),'white',8,5)
        g.box((0,.62,-1.08),(1.55,.9,.12),'wood')
    elif n in ('locker','shelf','kitchen'):
        w=.6 if n=='locker' else 1.2;h=1 if n=='kitchen' else 1.9
        for x in (-w/2,w/2):g.box((x,h/2,0),(.06,h,.5),'steel' if n=='locker' else 'wood')
        g.box((0,h/2,-.23),(w,h,.055),'navy' if n=='locker' else 'wood')
        for y in (0,h*.33,h*.66,h):g.box((0,y+.03,0),(w,.06,.5),'silver' if n=='locker' else 'wood')
        if n!='shelf':
            g.moving_part('door',(-w/2,0,.27),'hinge');g.box((0,h/2,.27),(w-.03,h-.06,.05),'blue' if n=='locker' else 'ivory')
            g.box((w*.3,h*.55,.31),(.045,.16,.05),'steel')
            if n=='locker':
                for j in range(5):g.box((0,h*.8+j*.04,.3),(w*.6,.012,.005),'ink',.001)
            g.part='body'
    elif n=='generator':
        g.box((0,.43,0),(1.6,.72,.85),'yellow',.065)
        for x in (-.65,.65):g.box((x,.06,0),(.18,.12,1),'ink')
        for j in range(8):g.box((-.805,.37,-.3+j*.085),(.025,.45,.035),'ink',.004)
        g.box((.81,.5,0),(.025,.33,.4),'navy')
        for z in (-.1,.1):g.cylinder((.83,.56,z),(.86,.56,z),.04,'cyan',sides=8,material=3)
    elif n in ('pipe','pipe-elbow'):
        points=[(0,0,0),(0,2,0)] if n=='pipe' else [(cos(i*pi/16),sin(i*pi/16),0) for i in range(9)]
        for a,b in zip(points,points[1:]):g.cylinder(a,b,.18,'steel',sides=10)
        for p in (points[0],points[-1]):g.ellipsoid(p,(.24,.13,.24),'silver',10,5)
    elif n=='valve':
        g.cylinder((-.5,.3,0),(.5,.3,0),.16,'steel',sides=10);g.cylinder((0,.3,0),(0,.7,0),.06,'steel')
        g.ring((0,.75,0),.28,.035,'red',segments=16)
        for i in range(4):g.beam((0,.75,0),(.28*cos(i*pi/2),.75,.28*sin(i*pi/2)),.035,'red')
    elif n=='console':
        g.prism([(0,-.35),(0,.35),(.85,.35),(1.1,-.35)],1.2,'navy')
        g.box((0,.995,-.03),(.9,.035,.4),'cyan',.02,Matrix.Rotation(.34,3,'X'),3)
        for x in (-.4,-.2,0,.2,.4):g.box((x,.875,.29),(.07,.035,.07),'red' if x<0 else 'yellow',.012)
    elif n=='antenna':
        g.cylinder((0,0,0),(0,3,0),.08,'steel',.035,8)
        for y in (1.5,2.2,2.8):
            g.beam((-.7,y,0),(.7,y,0),.04,'silver')
            for x in (-.5,0,.5):g.beam((x,y,-.35),(x,y,.35),.035,'silver')
        for s in (-1,1):g.beam((0,1.5,0),(s*.75,0,0),.06,'steel')
    elif n=='chest':
        g.box((0,.3,0),(1,.6,.65),'wood')
        g.moving_part('lid',(0,.6,-.325),'hinge',(1,0,0));g.ellipsoid((0,.62,0),(.5,.25,.325),'wood',12,6)
        for x in (-.35,.35):g.box((x,.78,0),(.07,.1,.67),'yellow')
        g.part='body';g.box((0,.52,.34),(.12,.18,.035),'yellow')
    elif n=='keycard':
        g.box((0,.025,0),(.09,.008,.15),'white',.008);g.box((0,.03,-.04),(.08,.002,.025),'teal',.002)
        g.box((-.022,.03,.02),(.025,.002,.025),'yellow',.002,material=1)
    elif n=='crystal':
        for i in range(5):
            a=i*2.4;h=.8+i*.13;g.cylinder((.2*cos(a),0,.2*sin(a)),(.25*cos(a),h,.25*sin(a)),.18,'purple' if i%2 else 'cyan',0,6,3)
    elif n=='checkpoint':
        for x in (-3,3):
            g.box((x,2.2,0),(.35,4.4,.45),'navy');g.box((x,2.3,.235),(.12,3.6,.025),'cyan',.015,material=3)
        g.box((0,4.3,0),(6.3,.5,.45),'navy')
        for x in range(-2,3):g.box((x,4.3,.24),(.5,.25,.02),'yellow',.01,material=3)
    elif n in ('jump-pad','teleporter'):
        g.cylinder((0,0,0),(0,.18,0),1.1,'navy',sides=12);g.ring((0,.21,0),.9,.055,'cyan',segments=24,material=3)
        if n=='teleporter':
            for s in (-1,1):
                g.box((s*1.05,1.25,0),(.15,2.5,.4),'steel');g.box((s*.96,1.3,0),(.035,2,.27),'purple',.01,material=3)
        else:
            for z in (-.3,.2):
                g.beam((-.3,.22,z-.15),(0,.22,z+.15),.1,'yellow');g.beam((.3,.22,z-.15),(0,.22,z+.15),.1,'yellow')
    else:raise ValueError(n)
    return g
