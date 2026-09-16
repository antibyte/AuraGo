"""Metric authoring sources for the fixed 2:1 pixel-world projection."""
from math import sin,cos,pi,tau
from geometry import Mesh
from mathutils import Matrix
from architecture import architecture
from environment import vegetation
from characters import humanoid,human_pose
from animals import animal
from props import props
from vehicles import vehicle,space
from maritime import ship

def terrain(e):
    g=Mesh(e['id']);d=e['design'];material,_,shape=d.partition('-')
    paint={'grass':'leaf','soil':'soil','sand':'sand','rock':'stone','snow':'snow','metal':'steel','water':'water','river':'water','canal':'water','lava':'orange','path':'soil','road':'road','cobble':'stone','bridge':'wood','waterfall':'water'}[material]
    height=1 if shape in ('block','edge','outer-corner','inner-corner','pillar') else .125
    if shape=='slope' or shape=='ramp':g.prism([(0,-1),(1,1),(0,1)],2,paint)
    elif shape=='stairs':
        for i in range(4):g.box((0,(i+1)*.125,-.75+i*.5),(2,(i+1)*.25,.5),paint,0)
    elif shape=='pillar':g.cylinder((0,0,0),(0,2,0),.4,paint,sides=8)
    elif shape=='outer-corner':g.prism([(0,-1),(1,-1),(0,1)],2,paint)
    elif shape=='inner-corner':
        g.box((-.5,.5,0),(1,1,2),paint,0);g.box((.5,.5,.5),(1,1,1),paint,0)
    else:
        g.box((0,-height/2,0),(2,height,2),paint,0)
        if shape=='edge':g.box((0,.25,.875),(2,.5,.25),paint,0)
    # Narrow pillars have their own top cap. Full-tile surface decoration would
    # float outside their actual footprint.
    if shape=='pillar':
        g.cylinder((0,1.92,0),(0,2.06,0),.46,paint,sides=8)
        if material=='metal':
            for y in (.12,1.85):g.ring((0,y,0),.43,.05,'ink')
        return g,None,None
    if material=='grass':
        for i in range(5):g.add([(-.8+i*.35,0,.4),(-.73+i*.35,.12,.43),(-.68+i*.35,0,.46)],[(0,1,2)],'leaf_light')
    elif material=='soil':
        for i in range(4):g.ellipsoid((-.7+i*.4,.01,.35),(.08,.04,.05),'bark',6,3)
    elif material=='sand':
        for i in range(3):g.beam((-.8,.015,-.7+i*.5),(.8,.015,-.8+i*.5),.017,'cream',.01,0)
    elif material=='rock':
        for i in range(3):g.beam((-.8+i*.4,.01,-.9),(-.4+i*.4,.01,.8),.027,'soil',.015,0)
    elif material=='snow':
        for i in range(3):g.ellipsoid((-.6+i*.6,.02,.2),(.3,.06,.28),'white',8,4)
    elif material=='metal':
        for x in (-.8,.8):
            for z in (-.8,.8):g.cylinder((x,0,z),(x,.03,z),.04,'ink',sides=6)
        g.box((0,.01,0),(.035,.02,2),'ink',0)
    elif material in ('water','river','canal','lava','waterfall'):
        for i in range(4):g.beam((-.7,.01,-.75+i*.45),(.3,.01,-.7+i*.45),.025,'cyan' if material!='lava' else 'yellow',.015,0)
        if shape in ('shore','corner','inlet','lock'):
            g.box((-.85,.15,0),(.3,.3,2),'stone' if material=='canal' else 'sand',0)
        if shape in ('corner','bend','inlet'):g.box((0,.15,.85),(2,.3,.3),'sand',0)
        if shape=='fork':g.box((.75,.15,.75),(.5,.3,.5),'sand',0)
        if material=='waterfall':g.box((0,1,0),(1.5,2,.12),'water',0)
        if shape=='shallow':
            for x,z in ((-.6,.35),(.45,-.4),(.65,.55)):g.ellipsoid((x,.025,z),(.18,.035,.11),'sand',8,4)
        if material=='canal':
            for x in (-.93,.93):g.box((x,.12,0),(.14,.24,2),'stone',0)
            if shape=='lock':
                for x in (-.42,.42):g.box((x,.4,0),(.83,.8,.14),'wood',0)
    elif material in ('path','road','cobble'):
        if shape not in ('corner','end'):g.box((0,.012,0),(.07,.024,2),'yellow' if material=='road' else 'sand',0)
        if shape in ('corner','t-junction','cross'):g.box((.5,.012,0),(1,.024,.07),'yellow' if material=='road' else 'sand',0)
        if shape=='end':g.box((0,.012,.55),(1.5,.024,.08),'white',0)
        if shape=='cross':g.box((-.5,.012,0),(1,.024,.07),'white',0)
        if material=='cobble':
            for i in range(8):g.box((-.75+(i%4)*.5,.02,-.45+(i//4)*.9),(.42,.04,.7),'soil',.04)
    elif material=='bridge':
        for side in (-1,1):g.box((side*.85,.45,0),(.08,.9,2),'steel' if shape=='metal' else 'stone' if shape=='stone' else 'bark',0)
        if shape=='wood':
            for z in (-.8,-.4,0,.4,.8):g.box((0,.025,z),(1.6,.05,.04),'bark',0)
        elif shape=='stone':
            for z in (-.75,0,.75):g.box((0,.025,z),(1.6,.05,.025),'stone',0)
        elif shape=='metal':
            for x in (-.6,.6):g.beam((x,.03,-.8),(-x,.03,.8),.04,'steel',.03,0)
    return g,None,None

def building(e):
    style,_,d=e['design'].partition('-')
    names={'window-wall':'wall-window','door-wall':'wall-door','outer-corner':'corner-outer','inner-corner':'corner-inner','corner':'corner-outer','door':'door-hinged','rail':'railing','rail-corner':'railing-corner','roof':'roof-slope','roof-corner':'roof-gable','ridge':'roof-ridge','gable':'roof-gable'}
    if d in ('arch','chimney','awning','fence'):
        g=Mesh(e['id'])
        if d=='arch':
            for s in (-1,1):g.box((s*.8,1,0),(.4,2,.35),'stone');g.box((0,2,0),(2,.4,.35),'stone')
        elif d=='chimney':g.box((0,1,0),(.6,2,.6),'bark');g.box((0,2,0),(.8,.14,.8),'stone')
        elif d=='awning':g.box((0,1.8,0),(2,.1,1.5),'cream');g.box((0,1.65,.7),(2,.3,.07),'red')
        else:
            for i in range(5):g.box((-.8+i*.4,.5,0),(.12,1,.1),'wood')
            for y in (.3,.75):g.box((0,y,0),(2,.1,.12),'bark')
    elif d=='roof-corner':
        g=Mesh(e['id']);g.add([(-1,0,-1),(1,0,-1),(1,0,1),(-1,0,1),(-1,1,1)],[(0,1,4),(1,2,4)],'red')
    else:g=architecture(dict(e,design=names.get(d,d)))
    # Structural differences apply to every style, including floors, ceilings
    # and connectors; palette variants alone must not fill catalog slots.
    if d in ('floor','ceiling'):
        g=Mesh(e['id']);g.box((0,-.08,0),(2,.16,2),'wood' if style=='village' else 'stone' if style=='city' else 'navy',0)
        if d=='ceiling':
            for x in (-.65,.65):g.box((x,-.16,0),(.18,.18,2),'bark' if style=='village' else 'steel',0)
        elif style=='village':
            for z in (-.8,-.4,0,.4,.8):g.box((0,.025,z),(2,.05,.035),'bark',0)
        elif style=='city':
            for x in (-.5,.5):
                for z in (-.5,.5):g.box((x,.025,z),(.94,.05,.94),'ivory',0)
        else:
            g.box((0,.025,0),(1.7,.05,1.7),'steel',0)
            for x in (-.9,.9):g.box((x,.035,0),(.045,.02,1.6),'cyan',0,material=3)
    if style=='village':
        # Exposed timber framing is modeled, not merely recoloured city masonry.
        if 'wall' in d or 'corner' in d:
            for x in (-.87,.87):g.box((x,1.5,.14),(.14,3,.12),'bark')
            g.beam((-.87,.2,.14),(.87,2.8,.14),.1,'bark')
        elif d not in ('floor','ceiling','roof-corner'):
            # Wooden joinery preserves the common snap positions.
            if d in ('pillar','beam'):g.box((0,.18,0),(.5,.3,.5),'bark')
            elif d in ('rail','rail-corner','balcony','fence'):g.box((0,.65,.9),(2,.16,.12),'bark')
            elif d in ('stairs','ramp'):g.box((0,.08,-1.8),(1.8,.12,.2),'wood')
            elif d in ('roof','ridge','gable'):g.beam((-1,0,-.95),(1,0,-.95),.12,'bark')
            elif d=='window':g.box((0,.8,.18),(1.5,.16,.3),'wood')
            elif d=='door':g.beam((-.5,.2,.08),(.5,2.1,.08),.06,'bark')
            elif d=='arch':g.beam((-.8,1.7,.22),(.8,2.15,.22),.12,'bark')
            elif d=='chimney':g.box((0,2.15,0),(1,.12,1),'red')
            elif d=='awning':
                for x in (-.9,.9):g.box((x,.9,.6),(.08,1.8,.08),'wood')
    elif style=='city':
        if 'wall' in d:
            for y in (.15,1,2,2.9):g.box((0,y,.12),(2,.065,.08),'stone')
        if d=='door':g.box((.3,1.5,.1),(.25,.5,.04),'glass',material=2)
    else:
        if 'wall' in d or d=='door':
            for x in (-.8,.8):g.box((x,1.5,.17),(.05,2.6,.04),'cyan',material=3)
            g.box((.5,1.2,.22),(.3,.5,.08),'ink')
    return g,None,None

def create(e):
    c=e['category'];d=e['design']
    if c=='terrain':return terrain(e)
    if c=='architecture':return building(e)
    if c=='people':
        roles=['explorer-a','explorer-b','civilian-a','civilian-b','security-a','explorer-a','civilian-b','mechanic-a','civilian-a','mechanic-b','pilot-a','security-b','explorer-b','mechanic-a','trooper-a','trooper-b']
        g,bones=humanoid(dict(e,design=roles[e['index']]))
        if d in ('knight','archer','villager','blacksmith'):
            g.bone='chest';g.box((0,1.3,.15),(.3,.3,.07),'steel' if d=='knight' else 'bark')
            g.bone='head';g.cylinder((0,1.76,0),(0,1.83,0),.15,'steel' if d=='knight' else 'wood')
        if d=='scholar':g.bone='chest';g.box((.16,1.17,.22),(.16,.24,.06),'red')
        if d=='medic':g.bone='chest';g.box((0,1.35,.15),(.04,.15,.02),'red');g.box((0,1.35,.15),(.12,.04,.03),'red')
        g.bone=None
        return g,bones,human_pose,['idle','walk','run','interact','carry_idle','carry_walk','hit','death','punch','rifle_fire']
    if c=='animals':
        g,bones,actions,pose=animal(e);return g,bones,pose,actions
    if c=='vehicles':
        if d in ('rowboat','steamship'):return ship(dict(e,design='rowboat' if d=='rowboat' else 'paddle-steamer'))
        if d in ('car','truck'):return vehicle(dict(e,design='sedan' if d=='car' else 'box-truck')),None,None
        if d in ('shuttle','hovercraft'):return space(dict(e,design='shuttle' if d=='shuttle' else 'lander')),None,None
        g=Mesh(e['id']);g.box((0,.6,0),(1.3,.12,1.7),'wood')
        for s in (-1,1):g.box((s*.65,.9,0),(.1,.6,1.7),'wood');g.beam((s*.5,.6,.7),(s*.5,.4,2),.09,'bark')
        for s in (-1,1):
            for z in ((-.55,.55) if d=='wagon' else (0,)):g.cylinder((s*.65,.4,z),(s*.8,.4,z),.4,'bark',sides=12)
        return g,None,None
    if c=='nature':
        mapping={'round-bush':'bush-round','thorn-bush':'bush-tall','grass':'grass-short','moss':'grass-short','willow':'oak','dead-tree':'stump'}
        if d=='moss':
            g=Mesh(e['id'])
            for i in range(7):g.ellipsoid((sin(i*3)*.5,.035,cos(i*3)*.5),(.3,.05,.22),'leaf_dark',7,4)
            return g,None,None
        if d=='dead-tree':
            g=Mesh(e['id']);g.cylinder((0,0,0),(.1,3,0),.23,'bark',.09,sides=8)
            for s in (-1,1):
                g.beam((0,1.5,0),(s*.8,2.2,.2),.12,'bark');g.beam((s*.8,2.2,.2),(s*1.2,2.8,.15),.08,'bark')
            return g,None,None
        if d in ('boulder','rock-pile','stalagmite','crystal','mushroom','cluster-mushroom'):
            if d=='crystal':return props(dict(e,design='crystal')),None,None
            g=Mesh(e['id'])
            for i in range(4 if d in ('rock-pile','cluster-mushroom') else 1):
                x=sin(i*3)*.45;z=cos(i*3)*.4
                if 'mushroom' in d:g.cylinder((x,0,z),(x,.5,z),.08,'cream');g.ellipsoid((x,.5,z),(.28,.12,.28),'red',10,6)
                elif d=='stalagmite':g.cylinder((x,0,z),(x,1.5,z),.4,'stone',0,7)
                else:g.ellipsoid((x,.35,z),(.55,.4,.45),'stone',7,5)
            return g,None,None
        g=vegetation(dict(e,design=mapping.get(d,d)))
        if d in ('grass','fern'):
            g.parts['fronds']=g.parts.pop('body');g.moving_part('fronds',(0,0,0),'sway',(0,0,1))
        if d=='willow':
            for i in range(14):
                a=i*tau/14;x=cos(a)*2;z=sin(a)*2
                g.cylinder((x,3.2,z),(x*1.07,.7,z*1.07),.075,'leaf',.025,sides=5)
        return g,None,None
    mapping={'wardrobe':'locker','desk':'computer-desk','cabinet':'kitchen','crate':'crate-wood','barrel':'barrel-wood','console':'console','chest':'chest','workbench':'table','streetlamp':'street-lamp'}
    supported={'table','chair','sofa','bed','shelf','generator','pipe','valve','antenna'}
    if d in supported or d in mapping:return props(dict(e,design=mapping.get(d,d))),None,None
    if d in ('door-wood','door-metal'):return building(dict(e,design=('village' if d=='door-wood' else 'scifi')+'-door'))
    g=Mesh(e['id'])
    if d in ('stool','basket','jar','sack','cartwheel'):
        if d=='cartwheel':g.ring((0,.5,0),.45,.07,'wood','Z')
        elif d=='stool':g.cylinder((0,0,0),(0,.5,0),.12,'bark');g.cylinder((0,.5,0),(0,.58,0),.3,'wood')
        else:g.ellipsoid((0,.35,0),(.3,.4,.25),'cream' if d=='sack' else 'wood',10,6);g.ring((0,.65,0),.16,.03,'bark')
    elif d in ('sink','oven','forge','anvil'):
        g.box((0,.5,0),(1,1,.8),'stone');g.box((0,1,0),(1.2,.1,.9),'steel')
        if d=='sink':g.ring((0,1.07,0),.25,.06,'silver')
        elif d=='anvil':g.prism([(1,0),(1.3,-.6),(1.3,.6),(1,.3)],.3,'ink')
        else:g.box((0,.6,.42),(.6,.4,.06),'orange' if d=='forge' else 'ink',material=3 if d=='forge' else 0)
    elif d in ('torch','lantern','lamp','brazier'):
        g.cylinder((0,0,0),(0,1.2,0),.05,'ink');g.ellipsoid((0,1.3,0),(.18,.25,.18),'yellow',8,5,3)
        if d=='lantern':g.box((0,1.52,0),(.4,.08,.4),'ink')
        if d=='lamp':g.cylinder((0,1.3,0),(0,1.6,0),.3,'cream',.15)
    elif d=='market-stall':g.box((0,.7,0),(2,.14,1),'wood');g.box((0,2,0),(2.2,.1,1.5),'red');g.box((0,1.85,.7),(2.2,.3,.1),'cream')
    elif d=='rope':
        for i in range(5):g.ring((0,.025*i,0),.3,.025,'cream')
    elif d=='sign':g.box((0,.5,0),(.08,1,.08),'bark');g.box((0,1,0),(.7,.4,.06),'wood')
    elif d=='tools':g.beam((-.3,.03,-.2),(.3,.03,.3),.05,'wood');g.box((.3,.03,.3),(.2,.08,.1),'steel')
    elif d=='lever':g.box((0,.1,0),(.4,.2,.4),'stone');g.moving_part('lever',(0,.2,0),'hinge',(1,0,0));g.beam((0,.2,0),(0,.65,.15),.04,'steel');g.ellipsoid((0,.65,.15),(.08,)*3,'red',8,5)
    elif d=='pressure-plate':g.box((0,.03,0),(.9,.06,.9),'steel');g.box((0,.065,0),(.65,.02,.65),'yellow')
    elif d=='coin':g.cylinder((0,.25,-.04),(0,.25,.04),.24,'yellow',sides=12,material=1)
    else:raise ValueError(d)
    return g,None,None
