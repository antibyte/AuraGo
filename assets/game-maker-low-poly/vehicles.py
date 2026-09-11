"""Authored vehicle families, articulated parts and attachment points (MIT)."""
from math import pi, sin, cos, tau
from mathutils import Matrix, Vector
import random
from geometry import Mesh


def wheels(g, width, axles, radius=.35, dual=False):
    for axle, z in enumerate(axles):
        for side in (-1, 1):
            x = side*(width/2+.035)
            steer = 'steer_'+str(axle)+('_l' if side < 0 else '_r')
            # A tiny axle mesh keeps the pivot as a real exported node.
            g.moving_part(steer, (x, radius, z), 'steer' if axle == 0 else 'axle')
            g.box((x, radius, z), (.12, .12, .12), 'steel')
            g.moving_part('wheel_'+steer, (x, radius, z), 'wheel', (1, 0, 0), steer)
            g.cylinder((x-.15, radius, z), (x+.15, radius, z), radius, 'rubber', sides=20)
            for outer in (-1, 1):
                g.cylinder((x+outer*.151, radius, z), (x+outer*.168, radius, z), radius*.57, 'silver', sides=12, material=1)
                g.cylinder((x+outer*.17, radius, z), (x+outer*.18, radius, z), radius*.22, 'ink', sides=8)
                for j in range(5):
                    a = tau*j/5
                    g.box((x+outer*.185, radius+sin(a)*radius*.35, z+cos(a)*radius*.35), (.012, .035, .035), 'steel', .005, material=1)
    g.part = 'body'


def vehicle(entry):
    g = Mesh(entry['id']); n = entry['design']; index = entry['index']
    paint = ['blue', 'teal', 'red', 'yellow', 'ivory', 'orange'][index % 6]
    truck = n in ('box-truck','tractor-truck','tanker','tipper','fire-engine','trailer')
    long = n in ('bus','minibus','van','ambulance')
    w = 2.4 if truck or n == 'bus' else 1.8
    length = {'compact':3.5, 'sports':4.5, 'rally':3.9, 'suv':4.5, 'pickup':5.1,
              'van':5.2, 'minibus':6.2, 'bus':9.2, 'trailer':8, 'tractor-truck':5.6,
              'tanker':8, 'tipper':6.5, 'box-truck':7.3, 'fire-engine':7,
              'ambulance':5.4, 'forklift':2.7}.get(n, 4.4)
    if n == 'police': paint = 'white'
    if n == 'ambulance': paint = 'ivory'
    if n == 'fire-engine': paint = 'red'
    if n == 'forklift': paint = 'yellow'
    radius = .48 if truck or n == 'bus' else .34
    g.box((0,.63,0),(w,.45,length),paint,.09)
    g.box((0,.4,0),(w*.77,.2,length*.91),'ink',.03)
    for side in (-1,1):
        g.box((side*(w/2+.015),.59,0),(.025,.1,length*.85),'steel',.008,material=1)
    cabin_z = length/2-1.4 if truck else -.05
    cabin_h = 2.25 if truck or long else (1.29 if n == 'sports' else 1.55)
    cabin_l = length-.55 if long else (2.1 if truck else 2.35)
    if n != 'trailer':
        if truck or long:
            g.box((0,(cabin_h+.86)/2,cabin_z),(w*.95,cabin_h-.86,cabin_l),paint,.09)
            g.box((0,cabin_h-.36,cabin_z+cabin_l/2+.01),(w*.82,.6,.035),'glass',.04,material=2)
            if long:
                count = max(2, int(cabin_l/1.15))
                for side in (-1,1):
                    for j in range(count):
                        z = cabin_z+cabin_l/2-.5-j*(cabin_l-.5)/count
                        g.box((side*w*.479,cabin_h-.38,z),(.025,.56,.85),'glass',.035,material=2)
            else:
                for side in (-1,1):
                    g.box((side*w*.48,cabin_h-.36,cabin_z+.14),(.035,.55,1.35),'glass',.04,material=2)
        else:
            # Tapered greenhouse, low bonnet and rear deck distinguish the silhouette.
            roof_front = .28 if n != 'sports' else .05
            g.prism([(.88,-1.27),(.88,1.05),(cabin_h,roof_front),(cabin_h,-.68)], w*.88, paint)
            g.prism([(.99,.94),(cabin_h-.09,roof_front+.035),(cabin_h-.13,roof_front+.075),(1.0,.98)],w*.76,'glass',2)
            g.prism([(.99,-1.25),(cabin_h-.12,-.74),(cabin_h-.09,-.72),(.97,-1.28)],w*.76,'glass',2)
            for side in (-1,1):
                g.moving_part('door_'+str(side), (side*w*.455,.93,.78), 'hinge', (0,1,0))
                g.box((side*w*.461,.82,-.02),(.055,.37,1.51),paint,.025)
                g.add([(side*w*.447,1.02,.7),(side*w*.447,cabin_h-.13,.23),
                       (side*w*.447,cabin_h-.13,-.6),(side*w*.447,1.02,-1.03)],
                      [(0,1,2,3) if side<0 else (3,2,1,0)],'glass',material=2)
                g.beam((side*w*.451,1.02,-.25),(side*w*.451,cabin_h-.12,-.25),.045,paint)
                g.box((side*w*.493,.96,-.52),(.03,.035,.16),'silver',.009,material=1)
                g.part='body'
                g.box((side*(w/2+.11),1.08,.64),(.22,.13,.22),paint,.035)
        g.box((0,.71,length/2+.025),(w*.86,.18,.11),'ink',.025)
        g.box((0,.97,length/2-.25),(w*.65,.035,.38),paint,.025)
        for side in (-1,1):
            g.box((side*w*.32,.86,length/2+.016),(.34,.15,.035),'cream',.025,material=3)
            g.box((side*w*.36,.83,-length/2-.014),(.21,.17,.035),'red',.025,material=3)
        g.socket('driver',(-.42,.94,cabin_z+.05))
        g.socket('passenger',(.42,.94,cabin_z+.05))
        g.socket('exhaust',(-.62,.36,-length/2))
    if n in ('box-truck','ambulance'):
        cargo_l = length-2.45 if truck else 2.45
        cargo_z = -length/2+cargo_l/2+.1
        g.box((0,1.67,cargo_z),(w*.97,1.85,cargo_l),'ivory',.06)
        for side in (-1,1):
            g.box((side*w*.49,1.48,cargo_z),(.018,.21,cargo_l*.9),paint,.005)
        g.moving_part('cargo_door',(w/2,1.0,-length/2+.07),'hinge')
        g.box((0,1.65,-length/2+.055),(w*.9,1.7,.08),'white',.025)
        g.box((.65,1.5,-length/2),(.045,.45,.03),'steel',.01,material=1)
        g.part='body'
    if n == 'tanker':
        g.cylinder((0,1.65,-length/2+.25),(0,1.65,.7),1.0,'silver',sides=16,material=1)
        for z in (-2.8,-1.3,.2):
            g.ring((0,1.65,z),1.01,.035,'ink',axis='Z',segments=16)
        g.box((0,2.68,-1.4),(.5,.12,2.8),'steel',.015,material=1)
    if n in ('pickup','tipper','trailer'):
        bed_l = 2.3 if n == 'pickup' else length-(2.65 if n == 'tipper' else .2)
        bed_z = -length/2+bed_l/2+.05
        bed_y = .95 if n == 'pickup' else 1.15
        g.moving_part('bed',(0,.83,-length/2+.1),'tip',(1,0,0))
        g.box((0,bed_y,bed_z),(w*.98,.18,bed_l),'ink',.035)
        for side in (-1,1):
            g.box((side*w*.46,bed_y+.3,bed_z),(.15,.6,bed_l),paint,.035)
        g.box((0,bed_y+.3,bed_z+bed_l/2-.1),(w*.95,.6,.16),paint,.04)
        g.moving_part('tailgate',(0,bed_y,-length/2+.05),'hinge',(1,0,0),'bed')
        g.box((0,bed_y+.3,-length/2+.05),(w*.94,.55,.1),paint,.03)
        g.part='body'
    if n == 'tractor-truck':
        g.cylinder((0,.85,-1.5),(0,1.05,-1.5),.7,'steel',sides=12,material=1)
        g.socket('hitch',(0,1.05,-1.5))
    if n == 'trailer': g.socket('hitch',(0,.6,length/2-.6))
    if n in ('police','ambulance','fire-engine'):
        g.box((0,cabin_h+.1,cabin_z),(.95,.12,.26),'steel',.025,material=1)
        for side in (-1,1):
            g.box((side*.3,cabin_h+.18,cabin_z),(.32,.15,.24),'blue' if side<0 else 'red',.035,material=3)
    if n == 'fire-engine':
        g.box((0,1.55,-1.2),(w*.98,1.65,3.8),'red',.06)
        for side in (-1,1):
            for z in (-2.5,-1.3,-.1):
                g.box((side*w*.5,1.45,z),(.05,1.0,1.0),'steel',.02,material=1)
        for x in (-.35,.35): g.beam((x,2.53,-3),(x,2.53,.55),.08,'silver')
        for z in range(12): g.beam((-.35,2.53,-3+z*.3),(.35,2.53,-3+z*.3),.045,'silver')
    if n == 'rally':
        for x in (-.5,-.18,.18,.5): g.cylinder((x,.98,length/2+.04),(x,.98,length/2+.12),.13,'white',sides=12,material=3)
    if n in ('sports','rally'):
        g.box((0,1.22,-length/2+.15),(w*1.02,.08,.32),'ink',.02)
        for x in (-.58,.58): g.box((x,1.02,-length/2+.16),(.08,.38,.1),'steel',.01)
    if n == 'suv':
        for x in (-.6,.6): g.beam((x,cabin_h+.08,-.7),(x,cabin_h+.08,.25),.065,'ink')
    if n == 'forklift':
        g.box((0,1.12,-.6),(1.45,1.05,1.0),'yellow',.08)
        for x in (-.68,.68):
            g.beam((x,.7,-.25),(x,2.3,-.25),.1,'ink')
            g.beam((x,.7,.85),(x,2.3,.85),.1,'ink')
        g.box((0,2.34,.25),(1.6,.14,1.5),'ink',.03)
        g.box((0,1.21,-.2),(.65,.2,.6),'rubber',.05)
        for x in (-.45,.45): g.beam((x,.22,1.2),(x,2.7,1.2),.14,'steel')
        g.moving_part('forks',(0,.2,1.2),'lift')
        for x in (-.48,.48): g.box((x,.16,1.9),(.18,.12,1.5),'steel',.015,material=1)
        g.part='body'
    axles = [length*.32,-length*.31]
    if truck and n not in ('tractor-truck',): axles += [-length*.31+.95]
    wheels(g,w,axles,radius)
    return g


def aircraft(entry):
    g=Mesh(entry['id']); n=entry['design']; paint=['ivory','red','yellow','navy','white','green','orange','teal'][entry['index']]
    length={'airliner':20,'cargo-plane':16,'fighter-jet':10,'helicopter':7,'quadcopter':1.6}.get(n,7)
    radius = .9 if n in ('airliner','cargo-plane') else .55
    if n == 'quadcopter':
        g.box((0,0,0),(.8,.3,.75),'ivory',.12)
        for x in (-.8,.8):
            for z in (-.8,.8):
                g.beam((0,0,0),(x,0,z),.13,'ink')
                key='rotor_'+str(x)+'_'+str(z)
                g.moving_part(key,(x,.18,z),'rotor')
                g.box((x,.2,z),(.9,.04,.09),'rubber',.02)
                g.cylinder((x,0,z),(x,.23,z),.12,'steel')
                g.part='body'
        g.ellipsoid((0,-.2,.34),(.15,.15,.15),'glass',material=2)
    else:
        g.ellipsoid((0,0,0),(radius,radius,length/2),paint,segments=16,rings=12)
        g.ellipsoid((0,radius*.58,length*.28),(radius*.85,radius*.7,length*.14),'glass',segments=12,rings=6,material=2)
        if n == 'helicopter':
            g.ellipsoid((0,-.1,1.4),(.85,.8,1.65),paint)
            g.cylinder((0,.2,-1),(0,.65,-length*.62),.4,paint,.08,sides=10)
            g.moving_part('main_rotor',(0,1.2,.55),'rotor')
            for angle in (0,pi/2): g.box((0,1.22,.55),(8,.07,.22),'ink',.02,Matrix.Rotation(angle,3,'Y'))
            g.cylinder((0,.6,.55),(0,1.25,.55),.1,'steel')
            g.part='body'
            g.moving_part('tail_rotor',(.16,.8,-length*.55),'rotor',(1,0,0))
            g.box((.18,.8,-length*.55),(.055,1.8,.14),'ink',.025)
            g.part='body'
            for x in (-.95,.95):
                g.beam((x,-1.1,-1.5),(x,-1.1,2.4),.09,'steel')
                for z in (-.8,1.3):g.beam((x*.65,-.5,z),(x,-1.1,z),.075,'steel')
        else:
            span=length*(.95 if n not in ('fighter-jet','biplane') else .75)
            swept=n in ('fighter-jet','airliner','cargo-plane')
            for side in (-1,1):
                verts=[(side*.3,-.05,1),(side*span/2,-.05,-1.6 if swept else .3),
                       (side*span/2,.02,-2.25 if swept else -.65),(side*.35,.13,-1.0)]
                g.add(verts,[(0,1,2,3)],paint)
                g.add([(x,y-.12,z) for x,y,z in verts],[(3,2,1,0)],'steel')
                g.beam(verts[0],verts[1],.13,paint)
                g.beam(verts[2],verts[3],.1,paint)
                g.moving_part('aileron_'+str(side),(side*span*.3,-.03,-1.0),'control',(1,0,0))
                g.box((side*span*.33,-.03,-1.15),(span*.28,.08,.32),paint,.015)
                g.part='body'
                g.box((side*1.15,.35,-length*.37),(2.3,.1,.9),paint,.03)
            g.add([(0,.3,-length*.36),(0,2,-length*.44),(0,.4,-length*.49)],[(0,1,2)],paint)
            g.beam((0,.4,-length*.49),(0,2,-length*.44),.12,paint)
            if n=='biplane':
                g.box((0,1.35,.05),(span,.12,1.05),paint,.025)
                for x in (-span*.34,span*.34):
                    for z in (-.35,.35):g.beam((x,0,z),(x,1.35,z),.055,'steel')
            if n in ('prop-plane','biplane','seaplane','cargo-plane'):
                locations=[(0,0,length/2)] if n!='cargo-plane' else [(-2.8,0,0),(2.8,0,0)]
                for i,(x,y,z) in enumerate(locations):
                    g.moving_part('propeller_'+str(i),(x,y,z),'propeller',(0,0,1))
                    for angle in (0,pi/2):g.box((x,y,z),(2.6,.12,.08),'ink',.02,Matrix.Rotation(angle,3,'Z'))
                    g.ellipsoid((x,y,z+.12),(.17,.17,.3),'silver',segments=10,rings=6)
                    g.part='body'
            if n in ('airliner','fighter-jet'):
                for x in (-1.7,1.7) if n=='airliner' else (-.4,.4):
                    g.cylinder((x,-.7,-.5),(x,-.7,1.6),.48,'steel',.35,material=1)
                    g.cylinder((x,-.7,-.53),(x,-.7,-.55),.34,'ink')
                    g.socket('engine_'+str(x),(x,-.7,-.6))
            if n=='airliner':
                for side in (-1,1):
                    for j in range(17):g.box((side*.887,.26,6-j*.73),(.035,.26,.24),'glass',.05,material=2)
            if n=='seaplane':
                for x in (-1.1,1.1):
                    g.ellipsoid((x,-1.25,.25),(.28,.3,2.2),'silver')
                    for z in (-.7,.8):g.beam((x*.5,-.4,z),(x,-1.15,z),.07,'steel')
            else:
                for x,z in ((0,length*.23),(-.85,-length*.2),(.85,-length*.2)):
                    g.moving_part('gear_'+str(x),(x,-.35,z),'landing_gear',(1,0,0))
                    g.beam((x,-.35,z),(x,-1.2,z),.065,'steel')
                    g.cylinder((x-.12,-1.2,z),(x+.12,-1.2,z),.28,'rubber')
                    g.part='body'
        g.socket('pilot',(0,.25,length*.23))
    return g


def space(entry):
    g=Mesh(entry['id']); n=entry['design']; rng=random.Random(301+entry['index'])
    if n.startswith('planet-') or n=='moon' or n.startswith('asteroid-'):
        planet=n.startswith('planet-') or n=='moon'
        radius=2 if planet else .7+entry['index']*.025
        paint={'planet-earth':'blue','planet-desert':'sand','planet-ice':'snow','planet-lava':'ink',
               'planet-gas':'cream','moon':'stone','asteroid-ice':'snow','asteroid-metal':'steel'}.get(n,'stone')
        g.ellipsoid((0,0,0),(radius,radius*(1 if planet else .72),radius*(1 if planet else .86)),paint,24 if planet else 12,16 if planet else 8)
        part=g.parts['body']
        if not planet:
            for i,p in enumerate(part['vertices']):
                factor=.7+rng.random()*.6
                if n=='asteroid-spire': factor*=1+abs(p[1])/radius*.8
                part['vertices'][i]=tuple(v*factor for v in p)
        from geometry import color
        for i,f in enumerate(part['faces']):
            center=sum((Vector(part['vertices'][j]) for j in f),Vector())/len(f)
            x,y,z=center
            wave=sin(x*3+y*1.7)+cos(z*2.9-x*2.1)+sin(y*4-z)
            if n=='planet-earth': c='leaf' if wave>.38 else ('snow' if abs(y)>radius*.83 else 'blue')
            elif n=='planet-lava': c='orange' if wave>1.5 else 'ink'
            elif n=='planet-gas':c=['cream','orange','sand','ivory'][int((y/radius+1)*8)%4]
            elif n=='planet-ice': c='snow' if wave>-.1 else 'teal'
            else:c=paint
            part['colors'][i]=color(c,.87+rng.random()*.18)
        if n=='planet-gas':
            for i in range(4):g.ring((0,0,0),2.7+i*.12,.045,['sand','cream','ivory','orange'][i],segments=56)
        return g
    if n.startswith('station-'):
        if n=='station-hub':
            g.cylinder((0,-.8,0),(0,.8,0),2.3,'ivory',sides=8)
            g.ring((0,.85,0),1.75,.07,'cyan',segments=32,material=3)
            for a in range(4):
                x,z=cos(a*pi/2)*3,sin(a*pi/2)*3
                g.beam((0,0,0),(x,0,z),.9,'steel',depth=1)
                g.socket('port_'+str(a),(x,0,z))
        elif n=='station-habitat':
            g.cylinder((0,0,-3),(0,0,3),1.3,'ivory',sides=12)
            for z in (-2.8,0,2.8):g.ring((0,0,z),1.32,.075,'steel','Z')
            for side in (-1,1):g.box((side*2.7,0,0),(3,.08,4.2),'blue',.035,material=1)
        else:
            g.box((0,-1,0),(6,.25,6),'steel')
            for side in (-1,1):g.box((side*2.9,.4,0),(.25,2.8,6),'ivory')
            g.box((0,1.7,0),(6,.25,6),'ivory')
            g.moving_part('hangar_door',(0,1.6,3),'slide')
            g.box((0,.25,3),(5.5,2.55,.15),'navy');g.part='body'
        g.socket('dock',(0,0,3))
        return g
    if n.startswith('satellite-'):
        g.box((0,0,0),(1.2,1.6,1.2),'cream',.12,material=1)
        for side in (-1,1):
            g.beam((0,0,0),(side*3.3,0,0),.1,'steel')
            g.moving_part('solar_'+str(side),(side*.8,0,0),'solar',(1,0,0))
            g.box((side*2.1,0,0),(2.4,.06,2.1),'navy',.03,material=1)
            for j in range(5):g.box((side*2.1, .038,-.84+j*.42),(2.3,.012,.015),'window',.001)
            g.part='body'
        if n=='satellite-comms':
            g.cylinder((0,.75,0),(0,1.3,0),.2,'silver',.75,sides=16,material=1)
            g.beam((0,1.2,0),(0,1.9,0),.04,'steel')
        else:g.cylinder((0,0,.5),(0,0,2.2),.42,'ink',.65,sides=12)
        return g
    dimensions={'scout':(1.2,.55,3.6),'interceptor':(1.5,.5,4.4),'heavy-fighter':(2.4,.8,5),
                'shuttle':(2.1,1.2,5),'freighter':(3.2,1.1,7),'lander':(2.4,1.3,3.3)}
    w,h,l=dimensions[n]; paint=['teal','ivory','red','orange','navy','cream'][entry['index']%6]
    g.prism([(-h*.4,-l/2),(-h*.4,l*.3),(0,l/2),(h*.4,l*.18),(h*.6,-l*.12),(h*.25,-l/2)],w,paint)
    g.ellipsoid((0,h*.45,l*.17),(w*.33,h*.35,l*.14),'glass',material=2)
    for side in (-1,1):
        span=w*(1.1 if n!='freighter' else .75)
        g.add([(side*w*.35,0,l*.25),(side*span,-.15,-l*.15),(side*span,.04,-l*.39),(side*w*.35,.25,-l*.4)],[(0,1,2,3)],paint)
        g.beam((side*w*.4,-.06,l*.2),(side*span,-.1,-l*.2),.17,paint)
        g.cylinder((side*w*.34,0,-l*.25),(side*w*.34,0,-l*.54),w*.16,'ink',w*.21,sides=12)
        g.cylinder((side*w*.34,0,-l*.545),(side*w*.34,0,-l*.56),w*.15,'cyan',sides=12,material=3)
        g.socket('engine_'+str(side),(side*w*.34,0,-l*.57))
        if n in ('interceptor','heavy-fighter'):
            g.cylinder((side*span,-.05,-.3),(side*span,-.05,l*.45),.085,'steel',.055,material=1)
            g.socket('muzzle_'+str(side),(side*span,-.05,l*.45))
    if n=='freighter':
        for x in (-1.9,1.9):
            for z in (-1.4,.4,2.2):g.box((x,.2,z),(1.0,1.25,1.5),'orange',.12)
    if n=='lander':
        for x in (-1.7,1.7):
            for z in (-1.3,1.3):
                g.moving_part('leg_'+str(x)+'_'+str(z),(x*.6,0,z*.6),'landing_gear',(1,0,0))
                g.beam((x*.6,0,z*.6),(x,-1.6,z),.14,'steel')
                g.box((x,-1.65,z),(.8,.12,.7),'ink',.05)
                g.part='body'
    g.socket('pilot',(0,h*.25,l*.14))
    return g
