"""Original modern/science-fiction equipment and synchronized first-person rigs."""
from math import sin, cos, pi, tau
from mathutils import Vector, Matrix
from geometry import Mesh
from characters import limb, two_bone
from catalog import FPS_ACTIONS

WEAPONS=('pistol','smg','rifle','shotgun','sniper','energy-pistol','plasma-rifle','heavy-blaster')

def view_motion(action,t):
    offset=Vector((0,0,0))
    if action in ('walk','sprint'):
        offset=Vector((.012*sin(t*tau),.015*cos(t*tau*2),0))*(1.7 if action=='sprint' else 1)
    if action in ('draw','holster'):offset.y=-.35*((1-t)**2 if action=='draw' else t*t)
    aim=1 if action=='aim_idle' else t if action=='aim_in' else 1-t if action=='aim_out' else 0
    offset+=Vector((-.12*aim,.065*aim,.025*aim))
    if action=='fire':offset.z-=.045*sin(pi*min(1,t*4))
    return offset


def magazine_motion(t):
    y=-.3*sin(pi*t/.96) if t<.48 else -.3*sin(pi*(1-t)/1.04)
    return Vector((-.04*sin(pi*t),y,.035*sin(pi*t)))


def weapon(entry):
    g=Mesh(entry['id']);n=entry['design'];energy=n in ('energy-pistol','plasma-rifle','heavy-blaster')
    pistol=n in ('pistol','energy-pistol');long=n in ('rifle','shotgun','sniper','plasma-rifle','heavy-blaster')
    length={'pistol':.22,'smg':.52,'rifle':.85,'shotgun':1.02,'sniper':1.15,
            'energy-pistol':.27,'plasma-rifle':.8,'heavy-blaster':.95}[n]
    paint='ivory' if energy else 'navy';accent='cyan' if energy else 'steel'
    bones=[('weapon',(0,0,0),(0,0,.1),None),('magazine',(0,-.13,.12),(0,-.25,.12),'weapon'),
           ('bolt',(0,.07,.03),(0,.07,.14),'weapon'),('trigger',(0,-.045,.03),(0,-.08,.03),'weapon')]
    g.bone='weapon'
    g.box((0,.04,.12 if not pistol else .055),(.075 if pistol else .115,.11,length*.4),paint,.018)
    g.box((0,-.105,-.025),(.064,.2,.088),'rubber',.014,Matrix.Rotation(-.22,3,'X'))
    # Empty trigger guard, actual mechanical handle and stock.
    for a,b in [((-.038,-.04,.055),(-.038,-.12,.09)),((-.038,-.12,.09),(-.038,-.12,.015))]:g.beam(a,b,.012,paint)
    muzzle=length*.72 if long else length*.8
    g.cylinder((0,.055,.11),(0,.055,muzzle),.025 if pistol else .038,accent,sides=12,material=1)
    g.ring((0,.055,muzzle),.029 if pistol else .045,.009,'ink','Z',12)
    if long:
        g.box((0,.015,-.2),(.105,.16,.26),paint,.025)
        g.box((0,-.015,-.34),(.13,.2,.055),'rubber',.02)
        g.box((0,.045,.32),(.125,.13,.23),paint,.018)
        for i in range(7):g.box((0,.123,.17+i*.043),(.115,.016,.017),'steel',.003)
        for s in (-1,1):
            for i in range(5):g.box((s*.064,.045,.24+i*.043),(.006,.045,.022),'ink',.002)
    if n=='smg':g.box((0,.03,-.16),(.08,.04,.25),'steel',.01)
    if n=='shotgun':
        g.cylinder((0,-.015,.2),(0,-.015,muzzle-.08),.027,'steel',sides=10)
        g.box((0,.008,.34),(.1,.11,.18),'wood',.018)
    if n=='sniper':
        g.box((0,.145,.14),(.055,.08,.25),'steel')
        g.cylinder((0,.205,-.025),(0,.205,.34),.045,'ink',sides=12)
        g.cylinder((0,.205,.34),(0,.205,.35),.039,'glass',sides=12,material=2)
        for s in (-1,1):g.beam((s*.035,.025,.53),(s*.15,-.25,.6),.025,'steel')
    if n=='heavy-blaster':
        for s in (-1,1):
            g.cylinder((s*.095,.06,.3),(s*.095,.06,muzzle),.038,'steel',sides=10)
            g.box((s*.095,.04,.13),(.08,.14,.3),'ivory')
    if energy:
        for s in (-1,1):
            g.box((s*.061,.065,.15),(.012,.035,length*.28),'cyan',.006,material=3)
            g.cylinder((s*.062,.04,.27),(s*.08,.04,.27),.04,'cyan',sides=10,material=3)
    g.bone='magazine'
    g.box((0,-.19,.11 if not pistol else -.025),(.068,.23,.095),'teal' if energy else 'steel',.014,Matrix.Rotation(.13,3,'X'))
    for j in range(4):g.box((.036,-.12-j*.04,.11),(.008,.008,.075),'ink',.002)
    g.bone='bolt'
    g.box((0,.084,.045),(.078,.045,.18 if pistol else .13),accent,.01)
    g.box((.072,.06,.01),(.06,.02,.025),'silver',.006)
    g.bone='trigger';g.box((0,-.07,.032),(.015,.045,.018),'silver',.003)
    g.socket('grip',(0,-.08,-.025),'weapon')
    g.socket('support',(0,-.02,.29 if long else .07),'weapon')
    g.socket('muzzle',(0,.055,muzzle),'weapon')
    g.socket('sight',(0,.14,.13),'weapon');g.socket('eject',(.07,.08,.05),'bolt')
    g.socket('magazine',(0,-.19,-.025 if pistol else .11),'magazine');g.bone=None
    return g,bones,list(FPS_ACTIONS),weapon_pose


def weapon_pose(bones,action,t):
    points={n:(Vector(a),Vector(b)) for n,a,b,p in bones}
    recoil=.045*sin(pi*min(1,t*4)) if action=='fire' else 0
    offset=view_motion(action,t)
    if action in ('reload','reload_empty'):
        a,b=points['magazine'];shift=magazine_motion(t)
        points['magazine']=(a+shift,b+shift)
        if action=='reload_empty':
            a,b=points['bolt'];shift=Vector((0,0,-.055*sin(pi*max(0,min(1,(t-.78)/.2)))))
            points['bolt']=(a+shift,b+shift)
    if action=='fire':
        a,b=points['bolt'];shift=Vector((0,0,-recoil*1.2));points['bolt']=(a+shift,b+shift)
        a,b=points['trigger'];points['trigger']=(a,b+Vector((0,0,-.012*sin(pi*t))))
    for n,(a,b) in list(points.items()):points[n]=(a+offset,b+offset)
    return points


def arms(entry):
    g=Mesh(entry['id']);scifi=entry['design']=='arms-scifi'
    bones=[('view_root',(0,0,0),(0,0,.1),None)]
    for side,s in [('L',-1),('R',1)]:
        shoulder=(s*.32,-.25,-.18);elbow=(s*.3,-.28,.12)
        hand=(.12,-.01,.435) if side=='L' else (.12,-.07,.12);tip=(hand[0],hand[1],hand[2]+.1)
        bones += [('arm_'+side,shoulder,elbow,'view_root'),('fore_'+side,elbow,hand,'arm_'+side),
                  ('hand_'+side,hand,tip,'fore_'+side)]
        limb(g,shoulder,elbow,.079,.057,'ivory' if scifi else 'green','arm_'+side,12)
        g.bone='arm_'+side;g.ellipsoid(shoulder,(.079,.074,.085),'ivory' if scifi else 'green',10,6)
        limb(g,elbow,hand,.057,.034,'ivory' if scifi else 'skin','fore_'+side,12)
        g.bone='fore_'+side
        if scifi:
            g.box((s*.245,-.22,.21),(.075,.055,.17),'navy',.015)
            g.box((s*.245,-.19,.22),(.045,.008,.08),'cyan',.006,material=3)
        g.bone='hand_'+side;g.box((hand[0],hand[1],hand[2]+.045),(.084,.042,.11),'ink',.015)
        for j in range(5):
            x=hand[0]+(j-2)*.017
            a=(x,hand[1]-.007,hand[2]+.08);b=(x,hand[1]-.046,hand[2]+.102);c=(x,hand[1]-.065,hand[2]+.075)
            k=f'digit_{j}_{side}';bones += [(k,a,b,'hand_'+side),(k+'_tip',b,c,k)]
            limb(g,a,b,.011,.01,'ink',k,6);limb(g,b,c,.01,.007,'ink',k+'_tip',6)
        g.socket('grip_'+side,hand,'hand_'+side)
    g.socket('camera',(0,0,0),'view_root');g.bone=None
    return g,bones,list(FPS_ACTIONS)+['pistol_'+a for a in FPS_ACTIONS]+['knife_slash','knife_stab','grenade_prime','grenade_throw'],arms_pose


def arms_pose(bones,action,t):
    rest={n:(Vector(a),Vector(b)) for n,a,b,p in bones};points={}
    pistol=action.startswith('pistol_');action=action.removeprefix('pistol_')
    offset=view_motion(action,t)
    points['view_root']=(rest['view_root'][0]+offset,rest['view_root'][1]+offset)
    for side,s in [('L',-1),('R',1)]:
        shoulder,elbow=rest['arm_'+side];_,hand=rest['fore_'+side];_,tip=rest['hand_'+side]
        target=hand+offset
        if pistol and side=='L':target=Vector((.077,-.075,.16))+offset
        if side=='L' and action in ('reload','reload_empty'):
            magazine=Vector((.12,-.18,.12 if pistol else .255))+magazine_motion(t)+offset
            blend=min(1,t/.2,(1-t)/.2)
            target=target.lerp(magazine,max(0,blend))
        if side=='R' and action in ('knife_slash','knife_stab','grenade_throw'):
            target+=Vector((-.22*sin(pi*t) if action=='knife_slash' else 0,.12*sin(pi*t),.28*sin(pi*t)))
        if side=='L' and action=='grenade_prime':target+=Vector((.18*sin(pi*t),.07*sin(pi*t),-.18*sin(pi*t)))
        middle,end=two_bone(shoulder+offset,target,(elbow-shoulder).length,(hand-elbow).length,(s,-1,0))
        points['arm_'+side]=(shoulder+offset,middle);points['fore_'+side]=(middle,end)
        points['hand_'+side]=(end,end+tip-hand)
    return points


def equipment(entry):
    n=entry['design'];g=Mesh(entry['id'])
    if n=='knife':
        g.box((0,.045,-.07),(.035,.055,.15),'ink',.012)
        g.box((0,.047,.025),(.1,.025,.03),'steel',.008)
        g.add([(-.03,.045,.03),(.03,.045,.03),(.022,.045,.18),(0,.045,.28),(-.022,.045,.18),(0,.059,.1)],
              [(0,1,5),(1,2,5),(2,3,5),(3,4,5),(4,0,5),(4,3,2,1,0)],'silver',material=1)
        g.socket('grip',(0,.045,-.07))
    elif n=='grenade':
        g.ellipsoid((0,.065,0),(.042,.065,.042),'green',10,7)
        for y in (.03,.06,.09):g.ring((0,y,0),.039,.003,'ink',segments=10)
        g.box((.03,.12,0),(.016,.035,.08),'steel',.004);g.ring((-.025,.135,0),.019,.003,'silver','Z',12)
        g.socket('grip',(0,.065,0))
    elif n in ('medkit','ammo-box','energy-cell'):
        if n=='energy-cell':
            g.cylinder((0,.02,0),(0,.22,0),.075,'steel',sides=10)
            for y in (.06,.11,.16):g.ring((0,y,0),.077,.018,'cyan',segments=10,material=3)
        else:
            g.box((0,.14,0),(.38,.28,.2),'white' if n=='medkit' else 'green',.025)
            if n=='medkit':
                for size in ((.15,.05,.008),(.05,.15,.008)):g.box((0,.14,.105),size,'red',.004)
            else:
                for x in (-.12,.12):g.box((x,.145,.105),(.045,.23,.012),'yellow',.004)
            g.box((0,.3,0),(.16,.04,.05),'ink',.006)
    elif n=='armor-plate':
        g.box((0,.2,0),(.3,.4,.045),'navy',.045)
        for x in (-.09,.09):g.box((x,.2,.028),(.05,.28,.02),'steel',.01)
    elif n=='reflex-sight':
        g.box((0,.015,0),(.065,.03,.08),'ink',.007)
        for x in (-.028,.028):g.box((x,.055,0),(.009,.07,.018),'steel',.003)
        g.box((0,.087,0),(.065,.009,.018),'steel',.003)
        g.box((0,.055,0),(.045,.047,.006),'glass',.003,material=2)
        g.ellipsoid((0,.055,.006),(.002,.002,.002),'red',6,4,3)
    elif n in ('scope','suppressor'):
        radius=.035 if n=='scope' else .025;length=.24 if n=='scope' else .19
        g.cylinder((0,radius,-length/2),(0,radius,length/2),radius,'ink',sides=12)
        for z in (-length*.4,length*.4):g.ring((0,radius,z),radius+.002,.008,'steel','Z',12)
        if n=='scope':g.cylinder((0,radius,length/2),(0,radius,length/2+.003),radius*.9,'glass',sides=12,material=2)
    elif n=='shield-generator':
        g.cylinder((0,0,0),(0,.2,0),.3,'navy',sides=12)
        g.ellipsoid((0,.28,0),(.23,.16,.23),'cyan',12,7,3)
        for i in range(4):
            a=i*pi/2;g.box((.22*sin(a),.22,.22*sin(a+pi/2)),(.09,.35,.09),'ivory',.02)
    else:raise ValueError(n)
    g.socket('mount',(0,0,0))
    return g


def fps(entry):
    if entry['design'] in WEAPONS:return weapon(entry)
    if entry['design'].startswith('arms-'):return arms(entry)
    return equipment(entry),None,[],None
