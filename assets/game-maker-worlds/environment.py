"""Original maritime scenery, port architecture and machinery (MIT)."""
from math import sin, cos, pi, tau
from mathutils import Matrix,Vector
from geometry import Mesh
import random

def submarine(entry):
    g=Mesh(entry['id']);i=entry['index'];d=entry['design']
    length=[2.4,5.6,4.2,13,10,15][i];width=[1.5,2.8,2.2,3.2,5,5.5][i]
    g.ellipsoid((0,0,0),(width/2,width*.43,length/2),'yellow' if i<3 else 'navy',20,10,1)
    for z in (-.28,0,.28):g.ring((0,0,z*length),width*.46,.045,'ink','Z')
    for side in (-1,1):
        for z in (-.2,.08,.3):
            g.cylinder((side*width*.44,.08,z*length),(side*width*.51,.08,z*length),.19,'yellow',material=1)
            g.cylinder((side*width*.51,.08,z*length),(side*width*.525,.08,z*length),.15,'glass',material=2)
    if i<3:
        g.ellipsoid((0,.14,length*.34),(width*.41,width*.32,length*.17),'glass',16,8,2)
        for side in (-1,1):g.beam((side*width*.4,-width*.4,-length*.35),(side*width*.4,-width*.4,length*.35),.1,'steel')
    else:
        g.box((0,width*.51,-length*.1),(width*.55,width*.7,length*.25),'steel',.12)
        g.cylinder((0,width*.8,-length*.07),(0,width*1.15,-length*.07),.09,'yellow')
        g.cylinder((0,width*1.15,-length*.07),(0,width*1.15,length*.02),.1,'steel')
        for side in (-1,1):g.box((side*width*.68,-.05,-length*.35),(width*.55,.1,length*.12),'steel')
    if i==4:
        for side in (-1,1):
            for z in (-.15,.15):g.box((side*width*.55,-.2,z*length),(1.2,1.2,1.8),'teal',.1)
    if i==5:
        for side in (-1,1):
            g.cylinder((side*width*.6,.2,-length*.3),(side*width*.6,.2,length*.23),.65,'yellow',material=1)
            for z in (-.25,0,.2):g.ring((side*width*.6,.2,z*length),.66,.05,'ink','Z')
    g.moving_part('propeller',(0,0,-length*.52),'rotate',(0,0,1))
    for i in range(4):
        a=i*pi/2;g.box((sin(a)*width*.19,cos(a)*width*.19,-length*.52),(width*.13,width*.4,.12),'yellow',rotation=Matrix.Rotation(-a,3,'Z'),material=1)
    g.part='body';g.moving_part('hatch',(0,width*.42,0),'hinge',(1,0,0));g.cylinder((0,width*.42,0),(0,width*.45,0),width*.24,'steel');g.part='body'
    g.socket('bow',(0,0,length*.5));g.socket('stern',(0,0,-length*.5));g.socket('bubbles',(0,0,-length*.55));g.socket('pilot',(0,.15,length*.1))
    for side in (-1,1):g.socket('torpedo-'+str(side),(side*width*.25,-.3,length*.4))
    return g,None,None

def nature(entry):
    g=Mesh(entry['id']);d=entry['design'];seed=random.Random(entry['index']+70)
    palm='palm' in d;tree=palm or d in ('banana','breadfruit','banyan','mangrove')
    if tree:
        height=3.5+entry['index']%4*.8;lean=.75 if d=='bent-palm' else .12
        last=(0,0,0)
        for i in range(7):
            p=(lean*(i/6)**2,height*i/6,0)
            if i:g.cylinder(last,p,.2*(1-i*.07),'bark',sides=9)
            last=p
        if palm or d=='banana':
            for i in range(8):
                a=i*tau/8;length=1.7 if d!='fan-palm' else 2.2
                points=[last,(last[0]+cos(a)*length*.6,height+.4,sin(a)*length*.6),(last[0]+cos(a)*length,height-.6,sin(a)*length)]
                for k in range(2):
                    p,q=Vector(points[k]),Vector(points[k+1]);side=Vector((-sin(a),0,cos(a)))*(.25 if k==0 else .2)
                    g.add([tuple(p),tuple((p+q)/2+side),tuple(q),tuple((p+q)/2-side)],[(0,1,2),(0,2,3)],'leaf_light' if i%2 else 'leaf')
                if d=='fan-palm':
                    tip=Vector(points[-1]);g.add([tuple(Vector(last)),tuple(tip+Vector((-sin(a)*.7,0,cos(a)*.7))),tuple(tip),tuple(tip-Vector((-sin(a)*.7,0,cos(a)*.7)))],[(0,1,2),(0,2,3)],'leaf_light')
            if d in ('coconut-palm','bent-palm'):
                for i in range(3):g.ellipsoid((lean+cos(i*tau/3)*.18,height-.18,sin(i*tau/3)*.18),(.16,.2,.16),'bark',8,5)
            if d=='royal-palm':g.cylinder((lean,height-.9,0),(lean,height,0),.24,'leaf_light',.12,sides=10)
            if d=='banana':
                for i in range(5):g.ellipsoid((lean+.2,height-.7-i*.06,.1),(.24,.045,.05),'yellow',8,5)
        else:
            for i in range(7):
                a=i*tau/7;p=(cos(a)*1.2+lean,height+.2*(i%3),sin(a)*1.2)
                g.beam((lean,height*.7,0),p,.1,'bark');g.ellipsoid(p,(1.1,.8,1),'leaf' if i%2 else 'leaf_dark',10,6)
        if d in ('mangrove','banyan'):
            for i in range(8):
                a=i*tau/8;g.beam((cos(a)*.4,1.2,sin(a)*.4),(cos(a)*1.2,0,sin(a)*1.2),.08,'bark')
    elif d in ('kelp','sea-grass','beach-grass','fern','sea-fan'):
        g.moving_part('fronds',(0,0,0),'sway',(0,0,1))
        for i in range(9):
            a=i*tau/9;h=(1.7 if d=='kelp' else .6)*(1+.2*sin(i*7))
            x,z=cos(a)*.25,sin(a)*.25
            g.add([(x,0,z),(x-.05,h*.7,z+.16),(x+.17,h,z+.3),(x+.05,h*.65,z+.16)],[(0,1,2,3)],'teal' if d in ('kelp','sea-fan') else 'leaf')
            if d=='fern':
                for j in range(4):g.ellipsoid((x,h*j/5,z+j*.06),(.22,.02,.08),'leaf',6,3)
    elif d in ('brain-coral','branching-coral','anemone'):
        if d=='brain-coral':
            g.ellipsoid((0,.28,0),(.5,.32,.42),'pink',14,8)
            for i in range(6):g.ring((0,.15+i*.05,0),.45-i*.035,.025,'purple',segments=14)
        else:
            for i in range(12):
                a=i*tau/12;h=seed.uniform(.35,.9);p=(cos(a)*.3,h,sin(a)*.3)
                g.cylinder((0,0,0),p,.045,'pink',.025,sides=7)
                if d=='branching-coral':g.cylinder((p[0]*.7,h*.65,p[2]*.7),(p[0]+.15,h,p[2]-.1),.03,'cream',.01,sides=6)
    else:
        for i in range(6):
            a=i*tau/6;g.ellipsoid((cos(a)*.3,.35,sin(a)*.3),(.4,.4,.4),'leaf',8,5)
            if d=='hibiscus':g.ellipsoid((cos(a)*.56,.5,sin(a)*.56),(.12,.05,.12),'red',6,4)
    return g,None,None

def coast(entry):
    g=Mesh(entry['id']);d=entry['design'];rng=random.Random(120+entry['index'])
    if d in ('island','atoll','rocky-island','volcano'):
        rings=3;steps=24;vertices=[]
        for ring in range(rings):
            for i in range(steps):
                a=i*tau/steps;r=(3 if d=='atoll' and ring==2 else [7,5,1][ring])*(1+.09*sin(i*2.7))
                y=[-.7,.2,1.8 if d=='rocky-island' else 4 if d=='volcano' else .45][ring]
                vertices.append((cos(a)*r,y,sin(a)*r))
        for ring in range(rings-1):
            faces=[(ring*steps+i,ring*steps+(i+1)%steps,(ring+1)*steps+(i+1)%steps,(ring+1)*steps+i) for i in range(steps)]
            g.add(vertices,faces,'stone' if d in ('volcano','rocky-island') and ring else 'sand')
        if d!='atoll':g.add(vertices,[tuple(range(steps*2,steps*3))],'soil' if d=='volcano' else 'leaf')
        if d=='volcano':g.ellipsoid((0,3.9,0),(1,.03,1),'orange',14,4,3)
    elif d=='sea-cave':
        for side in (-1,1):g.ellipsoid((side*2.5,1.5,0),(1,2,2),'stone',9,6)
        g.ellipsoid((0,3.1,0),(3,.7,2),'stone',10,5)
    elif d=='reef':
        for i in range(9):g.ellipsoid((rng.uniform(-2,2),rng.uniform(-.3,.4),rng.uniform(-2,2)),(.7,.6,.8),'stone' if i%2 else 'teal',8,5)
    else:
        height=3 if d.startswith('cliff') else .35;size=4
        g.box((0,height/2,0),(size,height,size),'stone' if height>1 else 'sand',.12)
        if d in ('beach-corner','cliff-corner'):g.ellipsoid((1,height*.5,1),(2.2,height*.52,2.2),'sand' if height<1 else 'stone',10,6)
        if d=='coast-inlet':
            # Open U-shaped shore around a navigable indentation.
            g=Mesh(entry['id']);g.box((0,.12,-1.5),(4,.25,1),'sand',0)
            for x in (-1.5,1.5):g.box((x,.12,.5),(1,.25,3),'sand',0)
        if d=='seabed':
            for i in range(5):g.ellipsoid((rng.uniform(-1.5,1.5),.25,rng.uniform(-1.5,1.5)),(.3,.22,.4),'stone',7,5)
    return g,None,None

def harbor(entry):
    g=Mesh(entry['id']);d=entry['design'];buildings=['beach-hut','stilt-house','warehouse','tavern','shipyard','watchtower','lighthouse','naval-quarters']
    def roof(w,depth,y,paint='red'):
        g.part='roof';g.prism([(y,-depth/2),(y+1.5,0),(y,depth/2)],w,paint);g.part='body'
    if d in buildings:
        w,depth=(8,10) if d in ('warehouse','shipyard','naval-quarters') else (4,5)
        base=1.8 if d=='stilt-house' else 0
        if base:
            for x in (-w/2+.2,w/2-.2):
                for z in (-depth/2+.2,depth/2-.2):g.box((x,base/2,z),(.25,base,.25),'bark')
        if d in ('watchtower','lighthouse'):
            h=9 if d=='lighthouse' else 6
            # Hollow tower with a real entrance; the lower front opening is not
            # a painted rectangle on a solid cylinder.
            paint='ivory' if d=='lighthouse' else 'bark'
            for i in range(12):
                a=(i+.5)*tau/12
                if sin(a)>.8:continue
                g.box((cos(a)*1.45,h/2,sin(a)*1.45),(.22,h,.8),paint,rotation=Matrix.Rotation(-a,3,'Y'))
            g.part='front-wall';g.box((0,h/2+1.1,1.35),(1.8,h-2.2,.2),paint);g.part='body'
            g.box((0,.1,0),(2.4,.2,2.4),'wood')
            g.cylinder((0,h,0),(0,h+.9,0),1.25,'glass',sides=12,material=2)
            g.part='roof'
            g.cylinder((0,h+.9,0),(0,h+1.8,0),1.5,'red',.04,sides=12)
            g.part='body';g.socket('entrance',(0,0,1.75));g.socket('interior',(0,.2,0))
            g.socket('light',(0,h+.4,0))
        else:
            g.box((0,base+.1,0),(w,.2,depth),'wood');g.box((-w/2,base+1.5,0),(.2,3,depth),'bark');g.box((w/2,base+1.5,0),(.2,3,depth),'bark');g.box((0,base+1.5,-depth/2),(w,3,.2),'bark')
            # Two front panels and a lintel leave a genuinely accessible doorway.
            g.part='front-wall'
            for side in (-1,1):g.box((side*(w/4+.35),base+1.5,depth/2),(w/2-.7,3,.2),'wood')
            g.box((0,base+2.7,depth/2),(1.4,.6,.2),'wood');roof(w+.5,depth+.5,base+3,'cream' if d=='beach-hut' else 'red')
            for side in (-1,1):g.box((side*(w/2+.015),base+1.8,0),(.04,1,1.2),'window')
            if d=='tavern':g.beam((w/2,2.8,2),(w/2+1,2.8,2),.08,'ink');g.box((w/2+.7,2.3,2),(.6,.6,.07),'cream')
            if d=='warehouse':
                for x in (-w*.38,w*.38):g.box((x,1.4,depth*.5+.14),(.14,2.8,.14),'ink')
                g.beam((-w*.4,2.9,depth*.5+.2),(w*.4,2.9,depth*.5+.2),.12,'steel')
                g.box((0,2.5,depth*.5+.25),(2,.35,.15),'cream')
            if d=='shipyard':
                g.beam((-w*.45,3.5,depth*.2),(-w*.45,5.5,depth*.2),.18,'bark')
                g.beam((-w*.45,5.5,depth*.2),(-w*.45,5.5,depth*.7),.16,'bark')
                g.beam((-w*.45,5.5,depth*.7),(-w*.45,2.7,depth*.7),.025,'steel')
                for z in (-3,-1,1,3):g.box((w*.5+.7,.6,z),(1.3,1.2,.15),'bark')
            if d=='naval-quarters':
                g.part='roof';g.box((0,4.7,-2),(3,.2,3),'navy');g.part='body'
                for x in (-w*.35,w*.35):
                    g.box((x,1.8,depth*.5+.15),(1.1,1.5,.1),'window');g.box((x,2.7,depth*.5+.25),(1.5,.16,.4),'ivory')
                g.cylinder((w*.5+.5,0,depth*.5),(w*.5+.5,5,depth*.5),.045,'steel')
                g.add([(w*.5+.5,5,depth*.5),(w*.5+1.7,4.8,depth*.5),(w*.5+.5,4.2,depth*.5)],[(0,1,2)],'navy')
            if base:
                for i in range(6):g.box((0,(i+1)*.15,depth*.5+1.8-i*.3),(1.4,(i+1)*.3,.3),'wood',0)
            g.socket('entrance',(0,base,depth/2+.25));g.socket('interior',(0,base+.2,0))
        return g,None,None
    if d in ('floor','flat-roof','pier','pier-corner','pier-cross','pitched-roof'):
        if d=='pitched-roof':roof(2,2,0)
        else:
            for i in range(10):g.box((0,.06,-.9+i*.2),(1.1 if d=='pier-cross' else 2,.12,.19),'wood',.008)
            if d=='flat-roof':
                g.part='roof';g.box((0,.16,0),(2.15,.1,2.15),'red',0);g.part='body'
            if d=='pier-corner':
                for i in range(5):g.box((1.5,.06,-.9+i*.2),(1,.12,.19),'wood',.008)
            if d=='pier-cross':
                for i in range(5):g.box((0,.075,-.4+i*.2),(3.1,.12,.19),'wood',.008)
            if d.startswith('pier'):
                for side in (-1,1):g.box((side*.85,-.45,0),(.16,1,.16),'bark')
        g.connections=[dict(id='north',position=[0,0,1]),dict(id='south',position=[0,0,-1])]
        if d=='pier-corner':g.connections=[dict(id='south',position=[0,0,-1]),dict(id='east',position=[2,0,-.5])]
        if d=='pier-cross':g.connections.extend([dict(id='east',position=[1.55,0,0]),dict(id='west',position=[-1.55,0,0])])
    elif d in ('wall','window-wall','doorway'):
        if d=='wall':g.box((0,1.5,0),(2,3,.15),'wood')
        else:
            for side in (-1,1):g.box((side*.8,1.5,0),(.4,3,.15),'wood')
            g.box((0,2.8,0),(1.2,.4,.15),'wood')
            if d=='window-wall':g.box((0,.55,0),(1.2,1.1,.15),'wood');g.box((0,1.9,0),(1.2,1.4,.04),'glass',material=2)
    elif d=='door':g.moving_part('door',(-.6,0,0),'hinge');g.box((0,1.2,0),(1.2,2.4,.12),'wood');g.box((.45,1.2,.09),(.08,.08,.08),'yellow',material=1)
    elif d in ('stairs','ramp'):
        if d=='ramp':g.prism([(0,-1),(3,1),(0,1)],2,'wood')
        else:
            for i in range(12):g.box((0,(i+1)*.125,-1+i/6),(2,(i+1)*.25,.18),'wood',0)
    elif 'railing' in d:
        for x in (-.9,0,.9):g.box((x,.5,0),(.1,1,.1),'wood')
        g.box((0,1,0),(2,.12,.12),'wood')
        if d=='railing-corner':g.box((.9,1,.9),(.12,.12,1.8),'wood')
    elif d=='mooring-post':g.cylinder((0,0,0),(0,1.1,0),.2,'bark');g.ring((0,.8,0),.23,.05,'steel')
    elif d=='dock-ladder':
        for x in (-.35,.35):g.box((x,1.5,0),(.08,3,.08),'wood')
        for i in range(8):g.box((0,i*.4,0),(.7,.08,.08),'wood')
    return g,None,None

def equipment(entry):
    g=Mesh(entry['id']);d=entry['design']
    if d in ('barrel','crate','treasure-chest'):
        from props import props
        g=props(dict(entry,design={'barrel':'barrel-wood','crate':'crate-wood','treasure-chest':'chest'}[d]));return g,None,None
    if d in ('cannon','swivel-gun','harpoon'):
        if d=='swivel-gun':
            g.cylinder((0,0,0),(0,.5,0),.13,'steel');g.box((0,.03,0),(.55,.06,.55),'steel')
        else:
            g.box((0,.3,0),(.65,.45,1),'bark')
            for s in (-1,1):
                for z in (-.32,.32):g.cylinder((s*.3,.22,z),(s*.48,.22,z),.22,'wood')
        g.moving_part('barrel',(0,.65,0),'recoil',(0,0,1))
        g.cylinder((0,.65,-.4),(0,.65,.8),.18 if d=='cannon' else .1,'ink',.13,material=1)
        g.socket('muzzle',(0,.65,.82),g.part)
        if d=='harpoon':
            g.beam((0,.65,.3),(0,.65,1.5),.035,'steel');g.prism([(.65,1.5),(.8,1.1),(.5,1.1)],.08,'steel')
            g.cylinder((-.4,.4,-.1),(.4,.4,-.1),.26,'cream',sides=12)
    elif d=='cannonballs':
        for x,z in ((-.2,-.2),(.2,-.2),(0,.15)):g.ellipsoid((x,.17,z),(.17,)*3,'ink',10,6,1)
        g.ellipsoid((0,.44,0),(.17,)*3,'ink',10,6,1)
    elif d in ('torpedo','depth-charge'):
        g.ellipsoid((0,0,0),(.2,.2,1 if d=='torpedo' else .35),'steel',12,8,1)
        if d=='torpedo':
            for a in (0,pi/2):g.box((0,0,-.7),(.6,.04,.35),'ink',rotation=Matrix.Rotation(a,3,'Z'))
    elif d in ('anchor','capstan','ship-wheel','gear-mechanism'):
        if d=='anchor':
            g.beam((0,0,0),(0,1.5,0),.1,'ink');g.beam((-.55,.95,0),(.55,.95,0),.1,'ink');g.ring((0,1.6,0),.14,.04,'steel','Z')
            for s in (-1,1):g.beam((0,.04,0),(s*.6,.35,0),.14,'ink');g.add([(s*.6,.35,0),(s*.5,.6,0),(s*.8,.5,0)],[(0,1,2)],'steel')
        else:
            g.cylinder((0,0,0),(0,.8,0),.17,'wood' if d=='capstan' else 'steel')
            if d=='ship-wheel':
                g.moving_part('wheel',(0,.8,0),'rotate',(0,0,1));g.ring((0,.8,0),.55,.06,'wood','Z')
                for i in range(8):
                    a=i*tau/8;g.beam((0,.8,0),(cos(a)*.7,.8+sin(a)*.7,0),.055,'wood')
                return g,None,None
            g.moving_part('wheel',(0,.8,0),'rotate',(0,1,0));g.ring((0,.8,0),.55,.06,'wood' if d=='ship-wheel' else 'yellow')
            for i in range(8):
                a=i*tau/8;g.beam((0,.8,0),(cos(a)*.7,.8,sin(a)*.7),.055,'wood' if d!='gear-mechanism' else 'yellow')
                if d=='gear-mechanism':g.box((cos(a)*.57,.8,sin(a)*.57),(.15,.15,.15),'yellow')
    elif d in ('compass','spyglass'):
        if d=='compass':g.cylinder((0,0,0),(0,.06,0),.18,'yellow',material=1);g.box((0,.065,0),(.015,.01,.27),'red')
        else:g.cylinder((0,0,-.3),(0,0,.3),.06,'yellow',.09,material=1);g.cylinder((0,0,.3),(0,0,.31),.07,'glass',material=2)
    elif d in ('rope-coil','net'):
        if d=='rope-coil':
            for i in range(6):g.ring((0,.03+i*.025,0),.3-i*.025,.027,'cream')
        else:
            for i in range(8):
                g.beam((-1,.1,-1+i*.28),(1,.1,-1+i*.28),.017,'cream');g.beam((-1+i*.28,.1,-1),(-1+i*.28,.1,1),.017,'cream')
    elif d=='lantern':
        for y in (.12,.64):g.box((0,y,0),(.35,.07,.35),'ink')
        for x in (-.15,.15):
            for z in (-.15,.15):g.beam((x,.12,z),(x,.64,z),.025,'yellow')
        g.ellipsoid((0,.4,0),(.1,.2,.1),'cream',10,6,3);g.ring((0,.8,0),.12,.025,'steel','Z')
    elif d in ('buoy','ship-bell','diving-bell'):
        scale=3 if d=='diving-bell' else 1
        g.cylinder((0,.06,0),(0,.65*scale,0),.3*scale,'yellow' if 'bell' in d else 'red',.16*scale,sides=12,material=1)
        g.ring((0,.8*scale,0),.12*scale,.025*scale,'steel','Z')
        if d=='diving-bell':g.cylinder((0,1.2,.73),(0,1.2,.8),.2,'glass',material=2)
    elif d=='flag':
        g.cylinder((0,0,0),(0,3,0),.045,'wood');g.moving_part('flag',(0,2.8,0),'sway')
        g.add([(0,2.8,0),(1,2.7,.1),(1,2.05,0),(0,2.15,0)],[(0,1,2,3)],'red')
    elif d in ('boiler','pump'):
        g.cylinder((0,.2,0),(0,1.7,0),.55,'ink' if d=='boiler' else 'teal',material=1)
        for y in (.35,1.5):g.ring((0,y,0),.56,.055,'yellow')
        g.cylinder((0,1.7,0),(0,2.2,0),.12,'steel');g.socket('steam',(0,2.2,0))
        g.moving_part('valve',(0,1,.6),'rotate',(0,0,1));g.ring((0,1,.6),.25,.03,'red','Z')
        if d=='pump':
            g.part='body';g.cylinder((-.65,.2,0),(-.65,1.2,0),.15,'steel')
            g.beam((-.65,1.2,0),(0,1.2,0),.16,'yellow')
            g.moving_part('handle',(-.65,1.2,0),'hinge',(0,0,1));g.beam((-.65,1.2,0),(-1.1,1.6,0),.05,'steel');g.part='body'
    return g,None,None
