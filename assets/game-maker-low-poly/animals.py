"""Twelve original species with articulated limbs, jaws, tails and wings."""
from math import sin, cos, pi, tau
from mathutils import Vector, Matrix
from geometry import Mesh
from characters import limb, two_bone
from catalog import ANIMAL_ACTIONS

# Shoulder height, torso length/width, neck height, head length, coat.
SPECIES={
    'dog':(.65,.85,.25,.16,.28,'cream'), 'wolf':(.85,1.12,.31,.2,.34,'stone'),
    'cat':(.3,.48,.13,.08,.16,'ink'), 'fox':(.42,.65,.17,.13,.23,'orange'),
    'deer':(1.2,1.15,.31,.55,.31,'bark'), 'boar':(.74,1.03,.39,.02,.35,'bark'),
    'bear':(1.12,1.45,.56,.12,.4,'hair'), 'horse':(1.6,1.65,.44,.67,.47,'bark'),
    'cow':(1.4,1.65,.51,.16,.43,'ivory'), 'sheep':(.75,.93,.35,.18,.28,'cream'),
    'chicken':(.33,.34,.17,.16,.1,'white'), 'bird':(.16,.23,.09,.04,.075,'teal'),
}


def animal(entry):
    n=entry['design'];h,length,w,neck,head,coat=SPECIES[n];bird=n in ('bird','chicken')
    g=Mesh(entry['id']);front=length*.36;back=-length*.37
    head_y=h+neck;head_z=front+head*.3
    bones=[('pelvis',(0,h*.91,back),(0,h*.97,0),None),
           ('spine',(0,h*.97,0),(0,h,front),'pelvis'),
           ('neck',(0,h,front),(0,head_y,head_z+.01),'spine'),
           ('head',(0,head_y,head_z+.01),(0,head_y,head_z+head),'neck'),
           ('jaw',(0,head_y-head*.18,head_z+.02),(0,head_y-head*.2,head_z+head*.85),'head'),
           ('tail',(0,h*.93,back),(0,h*.87,back-length*.35),'pelvis'),
           ('tail_tip',(0,h*.87,back-length*.35),(0,h*.9,back-length*.65),'tail')]
    leg_specs=[]
    for pair,z in ([('back',back*.25)] if bird else [('front',front),('back',back)]):
        for side,s in [('L',-1),('R',1)]:
            key=pair+'_'+side;x=s*w*.65
            hip=(x,h*.88,z)
            knee=(x,h*.5,z+(-.08 if pair=='front' else .13)*length)
            ankle=(x,.08*h,z)
            toe=(x,.035*h,z+.14*h)
            parent='spine' if pair=='front' else 'pelvis'
            bones += [(key+'_upper',hip,knee,parent),(key+'_lower',knee,ankle,key+'_upper'),
                      (key+'_foot',ankle,toe,key+'_lower')]
            leg_specs.append((key,hip,knee,ankle,toe))
    if bird:
        for side,s in [('L',-1),('R',1)]:
            bones.extend([('wing_'+side,(s*w*.7,h,front*.3),(s*w*2.5,h,0),'spine'),
                          ('wing_tip_'+side,(s*w*2.5,h,0),(s*w*5,h,-length*.15),'wing_'+side)])
    lookup={name:(a,b) for name,a,b,parent in bones}
    g.bone='pelvis';g.ellipsoid((0,h*.9,back*.5),(w,h*.28,length*.37),coat,12,8)
    g.bone='spine';g.ellipsoid((0,h*.94,front*.38),(w*1.05,h*.3,length*.4),coat,12,8)
    if n=='cow':
        for s in (-1,1):
            for i in range(3):g.ellipsoid((s*w*.88,h*.94,-length*.24+i*length*.25),(.055,h*.18,length*.13),'ink',7,5)
        g.bone='pelvis';g.ellipsoid((0,h*.63,back*.65),(w*.48,.17,.22),'pink',10,6)
        for s in (-1,1):g.cylinder((s*.09,h*.58,back*.65),(s*.09,h*.47,back*.65),.025,'pink',sides=6)
    if n=='sheep':
        for i in range(18):
            a=i*2.399;z=-length*.3+(i%5)*length*.14
            g.ellipsoid((w*.85*cos(a),h*.96+h*.2*sin(a),z),(.15,.17,.18),'ivory' if i%2 else 'cream',7,5)
    g.bone='neck';g.cylinder((0,h*.98,front*.7),(0,head_y,head_z),w*.66,coat,w*.43,10)
    if n in ('horse','deer'):
        g.ellipsoid((0,h+neck*.4,front),(.16,neck*.65,.25),coat,10,7)
    g.bone='head'
    g.ellipsoid((0,head_y,head_z+head*.26),(w*.58,head*.49,head*.58),coat,12,8)
    muzzle='pink' if n in ('cow','boar') else 'cream' if n in ('dog','fox','cat') else coat
    g.ellipsoid((0,head_y-head*.1,head_z+head*.7),(w*.4,head*.25,head*.43),muzzle,10,6)
    if not bird:g.ellipsoid((0,head_y-head*.08,head_z+head*1.02),(w*.29,head*.16,head*.065),'ink',8,5)
    for s in (-1,1):
        g.ellipsoid((s*w*.49,head_y+head*.12,head_z+head*.49),(.025 if h>.5 else .014,)*3,'ink',8,5)
        g.ellipsoid((s*w*.5,head_y+head*.15,head_z+head*.51),(.007 if h>.5 else .004,)*3,'white',6,4)
        if not bird:
            ear_y=head_y+head*.36
            if n in ('dog','bear','cow','sheep'):
                g.ellipsoid((s*w*.58,ear_y,head_z),(w*.23,head*.21,head*.12),coat,8,6)
            else:
                g.add([(s*w*.26,ear_y,head_z-.04),(s*w*.69,ear_y,head_z+.03),(s*w*.53,ear_y+head*.6,head_z)],
                      [(0,1,2),(2,1,0)],coat)
                g.add([(s*w*.34,ear_y+.015,head_z+.012),(s*w*.6,ear_y+.015,head_z+.028),(s*w*.51,ear_y+head*.42,head_z+.016)],
                      [(0,1,2),(2,1,0)],'pink')
    if n=='deer':
        for s in (-1,1):
            a=Vector((s*.1,head_y+head*.3,head_z))
            for j in range(4):
                b=a+Vector((s*.08,.18,-.08))
                g.cylinder(a,b,.027,'wood',.02,6)
                if j: g.cylinder(a,a+Vector((-s*.1,.16,.06)),.017,'wood',.003,6)
                a=b
    if n=='cow':
        for s in (-1,1):g.cylinder((s*.2,head_y+.17,head_z),(s*.33,head_y+.3,head_z-.09),.055,'cream',.006,8)
    if n=='boar':
        for s in (-1,1):g.cylinder((s*.15,head_y-.07,head_z+.24),(s*.22,head_y+.04,head_z+.33),.045,'ivory',.005,7)
        for j in range(10):g.cylinder((0,h*1.2,-length*.3+j*length*.07),(0,h*1.29,-length*.3+j*length*.07),.035,'ink',0,5)
    if n=='horse':
        for j in range(12):g.box((0,h+neck*j/12,front-.12+j*.02),(.055,.14,.13),'hair',.02)
    g.bone='jaw';g.ellipsoid((0,head_y-head*.28,head_z+head*.56),(w*.36,head*.12,head*.36),muzzle,10,5)
    for key,hip,knee,ankle,toe in leg_specs:
        radius=w*.33 if n in ('bear','boar') else w*.22
        limb(g,hip,knee,radius*1.45,radius*.7,coat,key+'_upper',10)
        limb(g,knee,ankle,radius*.75,radius*.42,coat if n not in ('sheep','chicken','bird') else 'ink' if n=='sheep' else 'yellow',key+'_lower',8)
        g.bone=key+'_foot'
        if bird:
            for j in (-1,0,1):g.cylinder(ankle,(toe[0]+j*.045,toe[1],toe[2]+.02),.012 if n=='chicken' else .006,'yellow',.004,5)
        else:
            hoof=n in ('horse','cow','deer','sheep','boar')
            g.box((ankle[0],.055*h,ankle[2]+.065*h),(radius*1.8,.11*h,.2*h),'ink' if hoof else coat,.02)
            if hoof:g.box((ankle[0],.055*h,ankle[2]+.166*h),(.009,.085*h,.005),'stone',.001)
            else:
                for j in (-1,0,1):g.ellipsoid((ankle[0]+j*radius*.48,.045*h,ankle[2]+.17*h),(radius*.3,.03*h,.055*h),coat,6,4)
    for name in ('tail','tail_tip'):
        a,b=lookup[name];r=w*.32 if n in ('fox','cat','wolf') else w*.14
        if n=='bear':r*=.5;b=Vector(a)+(Vector(b)-Vector(a))*.25
        limb(g,a,b,r,r*.55,'white' if n=='fox' and name=='tail_tip' else 'hair' if n=='horse' else coat,name,8)
    if bird:
        g.bone='head'
        g.cylinder((0,head_y,head_z+head*.65),(0,head_y-.025,head_z+head*1.45),head*.23,'yellow',0,6)
        if n=='chicken':
            for j in range(4):g.ellipsoid((0,head_y+head*.4,head_z+head*j*.2),(.025,.06,.035),'red',7,5)
            g.ellipsoid((0,head_y-.05,head_z+head*.6),(.025,.065,.03),'red',7,5)
        for side,s in [('L',-1),('R',1)]:
            for name in ('wing_'+side,'wing_tip_'+side):
                g.bone=name;a,b=map(Vector,lookup[name])
                g.add([a,b,b+Vector((0,-.01,-length*.8)),a+Vector((0,-.01,-length*.65))],[(0,1,2,3),(3,2,1,0)],'blue' if n=='bird' else 'ivory')
                for j in range(6):
                    p=a.lerp(b,j/6)
                    g.ellipsoid(p+Vector((0,0,-length*.55)),(w*.18,.013,length*.45),'navy' if n=='bird' else 'cream',6,4)
    g.socket('mouth',(0,head_y-head*.1,head_z+head),'head')
    g.socket('back',(0,h*1.24,0),'spine');g.bone=None
    actions=list(ANIMAL_ACTIONS)
    if n in ('dog','wolf','cat','fox','deer','horse','bear'):actions+=['jump']
    if n in ('dog','wolf','cat','fox','boar','bear'):actions+=['attack']
    if n in ('horse','deer','cow'):actions+=['kick']
    if bird:actions+=['flap']
    if n=='bird':actions+=['takeoff','fly','glide','land']
    return g,bones,actions,lambda bones,action,t:animal_pose(n,bones,action,t)


def animal_pose(species,bones,action,t):
    h,length,w,neck,head,coat=SPECIES[species];bird=species in ('bird','chicken')
    rest={n:(Vector(a),Vector(b)) for n,a,b,p in bones};points=dict(rest)
    moving=action in ('walk','run');phase=t*tau
    resting=1 if action=='rest_idle' else t if action=='rest_down' else 1-t if action=='rest_up' else 0
    airborne=action in ('takeoff','fly','glide','land','jump')
    up=(.45 if action in ('fly','glide') else .45*t if action=='takeoff' else .45*(1-t) if action=='land' else .32*sin(pi*t) if action=='jump' else 0)*h
    offset=Vector((0,-h*(.08 if moving else 0)-h*.6*resting+up,.01*sin(phase) if action=='idle' else 0))
    for n,(a,b) in rest.items():points[n]=(a+offset,b+offset)
    for key in [n[:-6] for n in rest if n.endswith('_upper')]:
        a,knee=rest[key+'_upper'];_,ankle=rest[key+'_lower'];_,toe=rest[key+'_foot']
        side=key.endswith('R');front=key.startswith('front')
        # Four-beat walk, diagonal trot, suspended gallop for fast predators/horse.
        shift=(.5 if side else 0)+(.25 if front else 0) if action=='walk' else (.5 if side!=front else 0)
        if action=='run' and species in ('horse','deer','dog','wolf','fox'):shift=(.12 if side else 0)+(.5 if front else 0)
        q=(t+shift)%1;target=ankle.copy()
        if moving:
            stance=.62 if action=='walk' else .46
            stride=h*(.25 if action=='walk' else .39)
            target.z+=stride*(1-2*q/stance) if q<stance else -stride+2*stride*(q-stance)/(1-stance)
            target.y+=0 if q<stance else h*.15*sin(pi*(q-stance)/(1-stance))
        if airborne:target+=Vector((0,up+h*.12,0))
        if resting:target.z+=h*.32*resting;target.y+=h*.05*resting
        if action=='kick' and not front:target+=Vector((0,h*.5*sin(pi*t)**2,-h*.55*sin(pi*t)**2))
        hip=a+offset
        middle,target=two_bone(hip,target,(knee-a).length,(ankle-knee).length,(0,0,-1 if front else 1))
        points[key+'_upper']=(hip,middle);points[key+'_lower']=(middle,target)
        points[key+'_foot']=(target,target+(toe-ankle))
    if action=='eat':
        pivot=rest['neck'][0]+offset;rot=Matrix.Rotation(.7+.08*sin(phase*2),3,'X')
        for name in ('neck','head','jaw'):
            a,b=points[name];points[name]=(pivot+rot@(a-pivot),pivot+rot@(b-pivot))
    if action in ('attack','eat'):
        a,b=points['jaw'];points['jaw']=(a,a+Matrix.Rotation(.35*sin(pi*t)**2,3,'X')@(b-a))
    for name in ('tail','tail_tip'):
        a,b=points[name];angle=.15*sin(phase+(0 if name=='tail' else .5))
        if name=='tail_tip':a=points['tail'][1]
        points[name]=(a,a+Matrix.Rotation(angle,3,'Y')@(b-a))
    if bird:
        flying=action in ('fly','flap','takeoff','land')
        for side,s in [('L',-1),('R',1)]:
            a,b=rest['wing_'+side];a+=offset;b+=offset
            flap=(.6*sin(phase*2) if flying else .08 if action=='glide' else -1.18)
            rot=Matrix.Rotation(s*flap,3,'Z')
            elbow=a+rot@(b-a);points['wing_'+side]=(a,elbow)
            c,d=rest['wing_tip_'+side]
            points['wing_tip_'+side]=(elbow,elbow+rot@(d-c))
    if action in ('turn_left','turn_right','hit','death','attack'):
        yaw=(-1 if action=='turn_left' else 1)*pi/2*t if action.startswith('turn') else 0
        roll=pi*.49*min(1,t*1.4) if action=='death' else 0
        pitch=-.13*sin(pi*t) if action in ('hit','attack') else 0
        rot=Matrix.Rotation(yaw,3,'Y')@Matrix.Rotation(roll,3,'Z')@Matrix.Rotation(pitch,3,'X')
        pivot=Vector((0,h*.25,0))
        for name,(a,b) in list(points.items()):points[name]=(pivot+rot@(a-pivot),pivot+rot@(b-pivot))
    return points
