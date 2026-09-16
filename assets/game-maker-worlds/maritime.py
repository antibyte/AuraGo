"""Original maritime silhouettes in metric author coordinates (+Y up, +Z bow)."""
from math import sin, cos, pi, tau
from mathutils import Vector, Matrix
from geometry import Mesh
from characters import humanoid, human_pose, two_bone

SHIP_DIMENSIONS = {
 'rowboat':(1.6,4,0),'dinghy':(2,5,1),'cutter':(3.4,10,1),'sloop':(4,14,1),
 'schooner':(4.6,19,2),'brig':(5.4,22,2),'frigate':(7,30,3),'galleon':(8,32,3),
 'carrack':(8.5,29,3),'junk':(6,22,3),'fishing-boat':(3.4,10,1),
 'paddle-steamer':(6.5,25,0),'ocean-steamer':(8,38,0),'ironclad':(7,27,0),
}

def ship(entry):
    g=Mesh(entry['id']); d=entry['design']; w,l,masts=SHIP_DIMENSIONS[d]
    steam=d in ('paddle-steamer','ocean-steamer','ironclad'); wood='steel' if steam else 'bark'
    # Watertight faceted hull, broad transom, tapered keel and pointed bow.
    stations=[(-.5,.55),(-.36,.9),(0,1),(.3,.85),(.5,.03)]
    vertices=[]
    for z,width in stations:
        vertices.extend([(-w*width*.5,.55,z*l),(w*width*.5,.55,z*l),
            (w*width*.32,-.65,z*l),(0,-1.05,z*l),(-w*width*.32,-.65,z*l)])
    faces=[]
    for i in range(len(stations)-1):
        for j in range(5):faces.append((i*5+j,i*5+(j+1)%5,(i+1)*5+(j+1)%5,(i+1)*5+j))
    faces.extend([(4,3,2,1,0),(20,21,22,23,24)])
    g.add(vertices,faces,wood)
    # Top is a usable deck; planks do not conceal a solid collision obstruction.
    for i in range(len(stations)-1):
        z0,a=stations[i];z1,b=stations[i+1]
        g.add([(-w*a*.47,.58,z0*l),(w*a*.47,.58,z0*l),(w*b*.47,.58,z1*l),(-w*b*.47,.58,z1*l)],[(0,1,2,3)],'wood')
        for side in (-1,1):
            g.beam((side*w*a*.49,.85,z0*l),(side*w*b*.49,.85,z1*l),.07,'yellow' if steam else 'wood')
    for z in range(int(-l*.42),int(l*.38),2):
        g.box((0,.595,z),(w*.84,.022,.025),'bark',0)
    g.colliders.append(dict(type='box',center=[0,.48,0],size=[w*.76,.2,l*.7]))
    g.moving_part('rudder',(0,-.3,-l*.47),'rotate',(0,1,0))
    g.box((0,-.52,-l*.52),(.12,.9,l*.12),'steel' if steam else 'bark')
    g.part='body'
    g.socket('bow',(0,.3,l*.5));g.socket('stern',(0,0,-l*.5));g.socket('wake',(0,-.04,-l*.53))
    g.socket('helm',(0,.65,-l*.31));g.socket('crew-port',(-w*.25,.65,-l*.1));g.socket('crew-starboard',(w*.25,.65,-l*.1))
    for index in range(masts):
        z=(index-(masts-1)/2)*l*.26;h=w*(2.3 if d!='junk' else 1.8)
        g.cylinder((0,.6,z),(0,h,z),.13,'bark',.055)
        g.beam((-w*.54,h*.72,z),(w*.54,h*.72,z),.075,'bark')
        g.moving_part('sail-'+str(index),(0,h*.72,z),'sway',(0,0,1))
        # Curved cloth panels retain a clearly faceted low-poly silhouette.
        points=[(x*w*.52,h*(.38+y*.34),z+sin((x+1)*pi/2)*.28*w) for y in (0,1) for x in (-1,-.5,0,.5,1)]
        g.add(points,[(j,j+1,j+6,j+5) for j in range(4)],'cream' if d!='junk' else 'red')
        g.part='body'
        for side in (-1,1):g.beam((side*w*.43,.68,z-l*.08),(0,h*.94,z),.025,'soil')
    if d=='rowboat':
        for z in (-l*.24,0,l*.24):g.box((0,.64,z),(w*.87,.08,.3),'wood')
        for side in (-1,1):
            g.moving_part('oar-'+str(side),(side*w*.43,.7,0),'rotate',(0,1,0))
            g.beam((side*.4,.7,0),(side*2.1,.55,-.65),.075,'wood');g.box((side*2.1,.55,-.65),(.48,.07,.2),'wood')
        g.part='body'
    if steam:
        g.box((0,1.2,-l*.18),(w*.55,1.3,l*.24),'navy')
        for side in (-1,1):
            for z in (-l*.23,-l*.15):g.box((side*w*.28,1.42,z),(.05,.36,.45),'window')
        for z in ([0,l*.16] if d=='ocean-steamer' else [l*.12]):
            g.cylinder((0,.6,z),(0,3.7,z),w*.13,'ink');g.cylinder((0,3.3,z),(0,3.6,z),w*.14,'red')
            g.socket('smoke-'+str(z),(0,3.7,z))
        if d=='paddle-steamer':
            for side in (-1,1):
                g.moving_part('paddle-'+str(side),(side*w*.55,.3,0),'rotate',(1,0,0))
                for k in range(10):
                    a=k*tau/10;g.box((side*w*.55,.3+sin(a)*1.35,cos(a)*1.35),(.65,.18,.55),'wood',rotation=Matrix.Rotation(a,3,'X'))
                g.ring((side*w*.55,.3,0),1.35,.1,'steel','X')
            g.part='body'
        else:
            g.moving_part('propeller',(0,-.6,-l*.52),'rotate',(0,0,1))
            for k in range(3):
                a=k*tau/3;g.box((sin(a)*.36,-.6+cos(a)*.36,-l*.52),(.25,.75,.12),'yellow',rotation=Matrix.Rotation(-a,3,'Z'))
            g.part='body'
    if d=='fishing-boat':
        g.box((0,1,-l*.3),(w*.65,.8,l*.23),'ivory');g.box((0,1.46,-l*.3),(w*.78,.15,l*.27),'teal')
        for s in (-1,1):g.beam((s*w*.35,.7,0),(s*w*.8,2.3,0),.07,'wood');g.beam((s*w*.8,2.3,0),(s*w*.8,.9,l*.2),.028,'cream')
    if d in ('cutter','sloop','schooner'):
        g.beam((0,.7,l*.38),(0,1.1,l*.65),.07,'bark')
        g.add([(0,1.1,l*.65),(0,w*1.7,l*.06),(0,1.1,l*.1)],[(0,1,2)],'ivory')
    if d=='galleon':
        for s in (-1,1):
            for z in (-.36,-.28):g.box((s*w*.37,2.9,z*l),(.3,1.6,.4),'yellow')
        g.box((0,3.7,-l*.33),(w*.85,.18,l*.25),'bark')
    if d=='carrack':
        g.box((0,1.15,l*.27),(w*.54,1.1,l*.18),'bark');g.box((0,1.75,l*.27),(w*.6,.12,l*.18),'wood')
        g.box((0,2.9,-l*.33),(w*.65,1.65,l*.2),'wood')
    if d in ('galleon','carrack','frigate','brig','junk'):
        g.box((0,1.3,-l*.33),(w*.72,1.4,l*.24),'bark')
        g.box((0,2.05,-l*.33),(w*.82,.15,l*.26),'wood')
        for x in (-.27,0,.27):g.box((x*w,1.42,-l*.455),(.7,.45,.06),'window')
    if l>10:
        for side in (-1,1):
            for i in range(2 if l<24 else 4):
                z=(i-1.5)*l*.12
                g.moving_part('gun-'+str(side)+'-'+str(i),(side*w*.35,.9,z),'recoil',(1,0,0))
                g.cylinder((side*w*.25,.95,z),(side*w*.57,.95,z),.16,'ink',.12,material=1)
                g.socket('muzzle-'+str(side)+'-'+str(i),(side*w*.58,.95,z),g.part)
                g.part='body'
    g.moving_part('hatch',(w*.12,.65,-l*.13),'hinge',(1,0,0));g.box((w*.12,.66,-l*.08),(w*.24,.12,l*.1),'bark');g.part='body'
    flag_y=w*(1.8 if d=='junk' else 2.3) if masts else 4.5 if steam else 1.8
    flag_z=-(masts-1)/2*l*.26 if masts else -l*.32
    if not masts:g.cylinder((0,.6,flag_z),(0,flag_y,flag_z),.055,'steel' if steam else 'bark',.035)
    g.moving_part('flag',(0,flag_y,flag_z),'sway',(0,1,0))
    g.add([(0,flag_y,flag_z),(.7,flag_y-.1,flag_z+.1),(.65,flag_y-.5,flag_z),(0,flag_y-.4,flag_z)],[(0,1,2,3)],'navy')
    # Damage is explicit geometry revealed by the damaged/repair clips. It
    # does not change collision or health; those remain game rules.
    g.moving_part('damage',(0,0,0),'damage')
    for side in (-1,1):
        for z in (-l*.2,l*.15):
            x=side*w*.498
            g.add([(x,.49,z-l*.035),(x,.1,z-l*.02),(x,.24,z+l*.015),(x,-.3,z+l*.035),(x,.46,z+l*.04)],[(0,1,2,3,4)],'ink')
    for z in (-l*.15,l*.12):g.box((w*.18,.63,z),(w*.16,.022,l*.05),'ink',0)
    return g,None,None

def person(entry):
    role=entry['design']; source=dict(entry,design=('security-a' if role.startswith('naval') else 'pilot-b' if role.startswith('diver') else 'civilian-a'))
    g,bones=humanoid(source);g.bone='head'
    if role=='diver-brass':
        g.ellipsoid((0,1.68,0),(.2,.22,.2),'yellow',16,10,1)
        g.cylinder((0,1.68,.18),(0,1.68,.23),.125,'steel',sides=12,material=1)
        g.cylinder((0,1.68,.23),(0,1.68,.24),.102,'glass',sides=12,material=2)
        for side in (-1,1):g.ellipsoid((side*.195,1.68,0),(.018,.09,.09),'steel',8,6,1)
    elif role.startswith('pirate'):
        if role=='pirate-captain':
            g.add([(-.24,1.79,0),(0,1.99,0),(.24,1.79,0),(0,1.77,.17)],[(0,1,3),(1,2,3),(2,0,3),(0,2,1)],'ink')
        else:g.ellipsoid((0,1.77,0),(.12,.045,.11),'red',12,5)
        g.box((-.047,1.687,.125),(.045,.025,.012),'ink')
        if role=='pirate-scout':
            g.ellipsoid((0,1.64,-.14),(.08,.08,.16),'red',8,5)
        g.bone='chest'
        if role=='pirate-gunner':
            for i in range(5):g.ellipsoid((-.15+i*.07,1.39-i*.07,.14),(.035,)*3,'ink',8,4,1)
        if role=='pirate-swashbuckler':
            g.beam((-.2,1.44,.14),(.18,1.09,.13),.055,'bark')
            g.bone='pelvis';g.beam((-.2,1.05,-.03),(-.28,.46,-.14),.045,'steel')
        if role=='pirate-captain':
            g.bone='chest'
            for side in (-1,1):g.ellipsoid((side*.25,1.44,0),(.1,.045,.14),'yellow',8,5)
    elif role.startswith('naval'):
        if role=='naval-officer':
            g.box((0,1.8,0),(.29,.07,.24),'navy');g.box((0,1.78,.15),(.24,.018,.12),'ink')
        elif role=='naval-marine':g.ellipsoid((0,1.77,0),(.125,.1,.12),'steel',12,6,1)
        elif role=='naval-sailor':g.cylinder((0,1.76,0),(0,1.81,0),.145,'white',sides=12)
        else:g.box((0,1.79,0),(.25,.045,.21),'red')
        g.bone='chest'
        for i in range(3):g.ellipsoid((.06,1.2+i*.065,.15),(.014,)*3,'yellow',6,4,1)
        if role=='naval-gunner':g.box((.25,1.12,.1),(.13,.15,.1),'bark')
    elif role=='fisherman':
        g.cylinder((0,1.76,0),(0,1.79,0),.22,'sand',sides=12)
        g.bone='chest';g.box((0,1.23,.15),(.22,.26,.06),'wood')
    elif role=='merchant':
        g.cylinder((0,1.77,0),(0,1.95,0),.12,'ink',sides=12)
        g.bone='chest';g.beam((-.2,1.45,.14),(.2,1.08,.12),.055,'bark');g.box((.23,1.03,.02),(.18,.25,.16),'bark')
    if role.startswith('diver'):
        if role=='diver-light':
            g.bone='head';g.box((0,1.68,.13),(.21,.06,.04),'ink');g.box((0,1.68,.156),(.18,.043,.008),'glass',material=2)
        g.bone='chest'
        for side in (-1,1):g.cylinder((side*.1,1.08,-.19),(side*.1,1.46,-.19),.075,'yellow' if role=='diver-brass' else 'steel',material=1)
        for side in ('L','R'):
            g.bone='foot_'+side;s=-1 if side=='L' else 1
            g.box((s*.115,.07,.22),(.16,.045,.39),'ink')
    g.bone=None
    return g,bones,marine_pose

def marine_pose(bones,action,t):
    aliases={'crouch':'crouch_idle','jump':'jump_start','land':'jump_land','saber_attack':'punch','block':'rifle_idle','pistol_fire':'rifle_fire','pistol_reload':'rifle_reload','cannon_operate':'push'}
    points=human_pose(bones,aliases.get(action,action),t)
    if action not in ('swim','tread_water','dive'):return points
    for side,s in [('L',-1),('R',1)]:
        phase=t*tau+(pi if s==1 else 0)
        shoulder=Vector((s*.255,1.415,0));hand=Vector((s*(.3+.14*sin(phase)),1.35+.3*cos(phase),.22*sin(phase)))
        elbow,hand=two_bone(shoulder,hand,.271,.222,(s,0,-1))
        points['upper_arm_'+side]=(shoulder,elbow);points['forearm_'+side]=(elbow,hand);points['hand_'+side]=(hand,hand+Vector((0,-.09,0)))
        hip=Vector((s*.108,.98,0));ankle=Vector((s*.12,.18,.18*sin(phase)))
        knee,ankle=two_bone(hip,ankle,.43,.43,(0,0,1))
        points['thigh_'+side]=(hip,knee);points['shin_'+side]=(knee,ankle);points['foot_'+side]=(ankle,ankle+Vector((0,-.05,.17)))
    if action in ('swim','dive'):
        rotation=Matrix.Rotation(pi/2 if action=='swim' else pi*.72,3,'X');origin=Vector((0,.95,0))
        points={n:(rotation@(a-origin)+origin,rotation@(b-origin)+origin) for n,(a,b) in points.items()}
    return points

def fish(entry):
    g=Mesh(entry['id']);d=entry['design'];shark='shark' in d or d=='hammerhead'
    length=3 if shark else 2.3 if d=='dolphin' else 1.4 if d=='tuna' else .32 if d=='schooling-fish' else .55
    bones=[('body',(0,0,-length*.1),(0,0,length*.32),None),('tail',(0,0,-length*.1),(0,0,-length*.4),'body'),('fluke',(0,0,-length*.4),(0,0,-length*.64),'tail')]
    g.bone='body';paint='steel' if shark or d=='schooling-fish' else 'blue' if d in ('tuna','dolphin') else 'orange'
    slender=d in ('dolphin','tuna','schooling-fish')
    width=.115 if slender else .13 if shark else .13
    height=.125 if slender else .14 if shark else .24
    g.ellipsoid((0,0,.02),(length*width,length*height,length*.46),paint,16,10)
    g.ellipsoid((0,-length*height*.45,length*.02),(length*width*.88,length*height*.6,length*.42),'ivory',12,7)
    if d=='dolphin':
        g.ellipsoid((0,-length*.02,length*.43),(length*.055,length*.047,length*.18),paint,12,6)
        g.ellipsoid((0,length*.07,length*.28),(length*.11,length*.095,length*.16),paint,12,7)
    if d=='reef-fish':
        for z in (-.2,0,.2):
            for side in (-1,1):g.ellipsoid((side*length*.123,0,length*z),(length*.012,length*.2,length*.035),'cream',8,6)
    if d=='tuna':
        for z in (-.3,-.38):g.add([(0,length*.1,length*z),(0,length*.17,length*(z-.03)),(0,length*.07,length*(z-.065))],[(0,1,2)],'yellow')
    for side in (-1,1):
        g.ellipsoid((side*length*width*.75,length*.04,length*.32),(length*.018,)*3,'ink',8,5)
        g.add([(side*length*.1,0,0),(side*length*.42,-length*.11,-length*.14),(side*length*.15,-length*.06,-length*.23)],[(0,1,2)],paint)
        if shark:
            for z in (.08,.12,.16):g.beam((side*length*.163,-length*.03,z*length),(side*length*.15,length*.06,z*length),length*.006,'ink')
    fin=.27 if shark else .23 if d=='dolphin' else .2 if d=='tuna' else .28 if d=='reef-fish' else .16
    g.add([(0,length*height*.7,-length*.07),(0,length*fin,-length*.15),(0,length*height*.7,-length*.32)],[(0,1,2)],paint)
    if d=='hammerhead':
        g.ellipsoid((0,.025,length*.37),(length*.35,length*.06,length*.085),paint,12,6)
        for side in (-1,1):g.ellipsoid((side*length*.32,.04,length*.4),(length*.02,)*3,'ink',8,5)
    g.bone='tail';g.cylinder((0,0,-length*.1),(0,0,-length*.44),length*.13,paint,length*.04)
    g.bone='fluke'
    if d=='dolphin':g.add([(0,0,-length*.41),(length*.29,0,-length*.66),(0,0,-length*.59),(-length*.29,0,-length*.64)],[(0,1,2),(0,2,3)],paint)
    else:g.add([(0,0,-length*.41),(0,length*.29,-length*.66),(0,0,-length*.59),(0,-length*.23,-length*.64)],[(0,1,2),(0,2,3)],paint)
    g.bone=None
    return g,bones,dolphin_pose if d=='dolphin' else fish_pose


def dolphin_pose(bones,action,t):
    points=fish_pose(bones,action,t)
    if action in ('death','hit'):return points
    # Mammal flukes beat vertically, unlike a fish's sideways tail stroke.
    for n,a,b,parent in bones:
        factor=0 if n=='body' else 1 if n=='tail' else 1.8
        rot=Matrix.Rotation(sin(t*tau)*(.05 if action=='idle' else .2)*factor,3,'X')
        points[n]=(rot@Vector(a),rot@Vector(b))
    return points

def fish_pose(bones,action,t):
    points={n:(Vector(a),Vector(b)) for n,a,b,p in bones}
    amp=.12 if action=='idle' else .35 if action in ('swim','turn') else .6
    for n,(a,b) in list(points.items()):
        factor=0 if n=='body' else 1 if n=='tail' else 1.8
        rot=Matrix.Rotation(sin(t*tau)*amp*factor,3,'Y');points[n]=(rot@a,rot@b)
    if action=='death':
        rot=Matrix.Rotation(pi*t,3,'Z');points={n:(rot@a,rot@b) for n,(a,b) in points.items()}
    if action=='hit':
        shift=Vector((sin(t*pi)*.1,0,0));points={n:(a+shift,b+shift) for n,(a,b) in points.items()}
    return points

def sea_animal(entry):
    d=entry['design']
    if d=='seagull':
        from animals import animal, SPECIES
        old=SPECIES['bird'];SPECIES['bird']=(.28,.42,.16,.07,.12,'white')
        _,bones,actions,pose=animal(dict(entry,design='bird'));SPECIES['bird']=old
        g=Mesh(entry['id']);g.bone='spine'
        g.ellipsoid((0,.28,0),(.15,.12,.3),'white',12,8)
        g.bone='head';g.ellipsoid((0,.36,.23),(.09,.09,.115),'white',12,7)
        g.cylinder((0,.35,.31),(0,.33,.48),.045,'yellow',.004,7)
        for s in (-1,1):g.ellipsoid((s*.077,.385,.27),(.013,)*3,'ink',8,4)
        g.bone='tail';g.add([(-.11,.27,-.16),(.11,.27,-.16),(.15,.28,-.43),(-.15,.28,-.43)],[(0,1,2,3)],'white')
        for side,s in [('L',-1),('R',1)]:
            g.bone='wing_'+side
            g.add([(s*.1,.28,.1),(s*.43,.28,.025),(s*.45,.28,-.22),(s*.11,.28,-.16)],[(0,1,2,3)],'ivory')
            g.bone='wing_tip_'+side
            g.add([(s*.4,.28,.03),(s*.87,.28,-.08),(s*.53,.28,-.28),(s*.4,.28,-.19)],[(0,1,2,3)],'white')
            g.add([(s*.72,.281,-.045),(s*.88,.281,-.08),(s*.53,.281,-.28)],[(0,1,2)],'ink')
            g.bone='back_'+side+'_lower';g.cylinder((s*.07,.13,-.04),(s*.07,.03,-.04),.014,'orange',.011,6)
            g.bone='back_'+side+'_foot';g.add([(s*.07,.025,-.06),(s*.07-.04,.018,.035),(s*.07+.04,.018,.035)],[(0,1,2)],'orange')
        g.bone=None
        return g,bones,pose,actions
    g=Mesh(entry['id']);bones=[('body',(0,0,-.1),(0,0,.4),None)]
    if d=='octopus':
        g.bone='body';g.ellipsoid((0,.28,0),(.28,.42,.3),'purple',14,9)
        for side in (-1,1):g.ellipsoid((side*.22,.2,.15),(.06,.055,.045),'cream',8,5);g.ellipsoid((side*.25,.2,.17),(.025,)*3,'ink',8,4)
        for i in range(8):
            a=i*tau/8;p=(cos(a)*.17,0,sin(a)*.17);q=(cos(a)*.6,-.15,sin(a)*.6);r=(cos(a)*1,-.08,sin(a)*1)
            name='arm'+str(i);bones.extend([(name,p,q,'body'),(name+'tip',q,r,name)])
            g.bone=name;g.cylinder(p,q,.075,'purple',.04,sides=8);g.bone=name+'tip';g.cylinder(q,r,.04,'pink',.012,sides=7)
            for j in range(3):
                v=Vector(q).lerp(Vector(r),j/3);g.ellipsoid((v.x,v.y-.035,v.z),(.03,.015,.03),'cream',6,4)
    elif d=='ray':
        g.bone='body';g.ellipsoid((0,0,0),(.33,.08,.6),'navy',12,6)
        for s in (-1,1):
            name='wing'+str(s);bones.append((name,(s*.2,0,0),(s*1.1,0,-.15),'body'));g.bone=name
            g.add([(s*.2,0,.5),(s*1.2,-.02,-.05),(s*.35,0,-.55)],[(0,1,2)],'blue')
        g.bone='body';g.cylinder((0,0,-.4),(0,0,-1.6),.045,'ink',.005,sides=7)
    else:
        g.bone='body';g.ellipsoid((0,.05,0),(.4,.23,.58),'leaf_dark',14,8);g.ellipsoid((0,-.055,0),(.38,.09,.55),'sand',12,6)
        for i in range(6):
            a=i*tau/6;g.ellipsoid((cos(a)*.24,.24,sin(a)*.3),(.13,.018,.16),'leaf',6,4)
        g.ellipsoid((0,0,.65),(.14,.12,.22),'green',12,6)
        for s in (-1,1):
            g.ellipsoid((s*.1,.045,.76),(.025,)*3,'ink',8,4)
            for front in (True,False):
                name=('front' if front else 'rear')+str(s);z=.3 if front else -.36
                bones.append((name,(s*.25,0,z),(s*.7,0,z-.18),'body'));g.bone=name
                g.ellipsoid((s*.5,0,z-.06),(.31,.035,.13),'green',10,5)
    g.bone=None
    return g,bones,sea_pose,['idle','swim','turn','hit','death']

def sea_pose(bones,action,t):
    points={n:(Vector(a),Vector(b)) for n,a,b,p in bones}
    if action=='death':
        rot=Matrix.Rotation(pi*t,3,'Z');return {n:(rot@a,rot@b) for n,(a,b) in points.items()}
    for index,(n,a,b,parent) in enumerate(bones):
        if n=='body':continue
        p,q=points[n];angle=sin(t*tau+index*.55)*(.06 if action=='idle' else .32)
        rot=Matrix.Rotation(angle,3,'Z');points[n]=(p,p+rot@(q-p))
    return points
