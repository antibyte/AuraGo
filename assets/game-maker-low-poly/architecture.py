"""Original 2 m architecture kit. Openings and stairs remain traversable."""
from math import pi
from mathutils import Vector
from geometry import Mesh


def solid(g,at,size,paint='ivory',bevel=.02):
    g.box(at,size,paint,bevel)
    g.colliders.append(dict(type='box',center=list(at),size=list(size)))


def connection(g,name,at,normal,width=2):
    g.socket(name,at)
    g.connections.append(dict(id=name,position=list(at),normal=list(normal),width=width))


def panel(g,at,width=2,height=3,opening=None,paint='ivory',axis='X'):
    x,y,z=at
    def block(dx,dy,w,h,color):
        center=(x+dx,y+dy,z) if axis=='X' else (x,y+dy,z+dx)
        size=(w,h,.18) if axis=='X' else (.18,h,w)
        solid(g,center,size,color,.015)
    if opening:
        ow=min(1.2,width-.4);bottom=0 if opening=='door' else .95
        top=2.4 if opening=='door' else 2.35
        block(-(width+ow)/4,height/2,(width-ow)/2,height,paint)
        block((width+ow)/4,height/2,(width-ow)/2,height,paint)
        block(0,(top+height)/2,ow,height-top,paint)
        if bottom:block(0,bottom/2,ow,bottom,paint)
        if opening=='window':frame(g,(x,y+1.65,z),ow,1.4,axis)
    else:block(0,height/2,width,height,paint)
    if opening!='door':block(0,.08,width,.12,'stone')
    block(0,height-.08,width,.12,'white')


def frame(g,at,w,h,axis='X'):
    x,y,z=at
    def b(dx,dy,sx,sy,paint):
        g.box((x+dx,y+dy,z) if axis=='X' else (x,y+dy,z+dx),
              (sx,sy,.23) if axis=='X' else (.23,sy,sx),paint,.01)
    for s in (-1,1):
        b(s*(w/2-.045),0,.09,h,'white')
        b(0,s*(h/2-.04),w,.08,'white')
    if g.id!='architecture-apartment':b(0,0,.035,h,'steel')
    g.box(at,(w-.15,h-.13,.012) if axis=='X' else (.012,h-.13,w-.15),'window',.002,material=2)


def railing(g,a,b,h=1):
    a,b=Vector(a),Vector(b);count=max(1,round((b-a).length/.5))
    for i in range(count+1):
        p=a+(b-a)*i/count
        g.beam(p,p+Vector((0,h,0)),.045,'steel')
    for y in (.12,h):g.beam(a+Vector((0,y,0)),b+Vector((0,y,0)),.06,'steel')


def door(g,at=(0,0,0),sliding=False,scifi=False,w=1.2,h=2.4):
    x,y,z=at
    for s in (-1,1):g.box((x+s*(w/2+.055),y+h/2,z),(.11,h+.1,.26),'steel',.025)
    g.box((x,y+h+.04,z),(w+.22,.12,.26),'steel',.02)
    g.moving_part('door',(x if sliding else x-w/2,y,z),'slide' if sliding else 'hinge',(1,0,0) if sliding else (0,1,0))
    g.box((x,y+h/2,z),(w-.035,h-.025,.09),'navy' if scifi else 'wood',.022)
    g.box((x,y+h*.64,z+.053),(w*.62,h*.4,.015),'glass',.015,material=2)
    g.box((x+w*.33,y+h*.44,z+.087),(.07,.2,.07),'silver',.015,material=1)
    if scifi:
        for s in (-1,1):g.box((x+s*w*.34,y+h*.27,z+.057),(.035,.6,.015),'cyan',.007,material=3)
    g.part='body';g.socket('doorway',(x,y,z))


def roof(g,w,d,y,paint='red',flat=False):
    if flat:
        solid(g,(0,y+.1,0),(w+.25,.2,d+.25),'stone')
        for s in (-1,1):
            g.box((s*w/2,y+.24,0),(.16,.28,d),'ivory')
            g.box((0,y+.24,s*d/2),(w,.28,.16),'ivory')
    else:
        rise=min(2,d*.28)
        for s in (-1,1):
            a=Vector((0,y+rise,0));b=Vector((0,y,s*d/2))
            rot=(b-a).to_track_quat('Z','Y').to_matrix()
            g.box((a+b)/2,(w+.3,.14,(b-a).length+.22),paint,.015,rot)
            for j in range(1,7):
                p=a+(b-a)*j/7;g.box(p,(w+.31,.028,.025),'bark',.003,rot)
        g.beam((-w/2-.15,y+rise+.07,0),(w/2+.15,y+rise+.07,0),.15,paint)
        for s in (-1,1):
            g.add([(s*w/2,y,-d/2),(s*w/2,y,d/2),(s*w/2,y+rise,0)],[(0,1,2)],'ivory')


def staircase(g,x,z,base):
    for j in range(15):
        solid(g,(x,base+(j+1)*.1,z+j*.27),(2,(j+1)*.2,.27),'stone',.008)
        g.box((x,base+(j+1)*.2+.012,z-.1+j*.27),(1.95,.02,.06),'yellow',.003)
    for side in (-.94,.94):railing(g,(x+side,base+.2,z-.13),(x+side,base+3,z+3.9))


def architecture(entry):
    g=Mesh(entry['id']);n=entry['design']
    buildings={'house':(6,8,1),'townhouse':(6,6,2),'shop':(8,6,1),'office':(8,8,3),
               'apartment':(10,8,4),'warehouse':(12,10,1),'hangar':(16,16,1),
               'garage':(6,6,1),'workshop':(10,8,1),'barn':(10,12,1),'outpost':(8,8,1)}
    if n in buildings:
        w,d,floors=buildings[n];industrial=n in ('warehouse','hangar','garage','workshop','barn')
        level=6 if n=='hangar' else 3
        paint={'barn':'red','outpost':'navy','shop':'sand','office':'white','warehouse':'steel'}.get(n,'ivory')
        solid(g,(0,.1,0),(w,.2,d),'stone')
        for floor in range(floors):
            base=floor*3+.2
            if floor:
                # A real 2 x 4 stairwell: no solid floor crosses its opening.
                solid(g,(1,base-.1,0),(w-2,.2,d),'stone')
                if d>4:solid(g,(-w/2+1,base-.1,(d-4)/2),(2,.2,d-4),'stone')
                staircase(g,-w/2+1,-d/2+.14,base-3)
                connection(g,'level-'+str(floor),(-w/2+1,base,-d/2+4.1),(0,0,1))
            for z in (-d/2,d/2):
                for k in range(int(w/2)):
                    x=-w/2+1+k*2;front=z>0;opening='window'
                    if industrial and front and abs(x)<w*.32:continue
                    if floor==0 and front and k==int(w/4):opening='door'
                    panel(g,(x,base,z),2,level,opening,paint)
            for x in (-w/2,w/2):
                for k in range(int(d/2)):
                    panel(g,(x,base,-d/2+1+k*2),2,level,'window' if not industrial else None,paint,'Z')
        if industrial:
            opening_w=round(w*.64/2)*2
            g.box((0,level-.25,d/2),(opening_w,.9,.25),'steel')
            for x in (-opening_w/2,opening_w/2):g.box((x,level/2,d/2),(.18,level,.25),'steel')
            g.moving_part('loading_door',(0,level,d/2),'lift',(0,1,0))
            for j in range(6):g.box((0,.35+j*(level-.65)/6,d/2),(opening_w-.12,(level-.7)/6,.1),paint,.01)
            g.part='body'
        roof(g,w,d,level*floors+.2,'bark' if n=='barn' else 'red',n in ('office','apartment','outpost','warehouse','shop'))
        if n in ('house','townhouse'):
            g.box((-w*.26,3*floors+1.2,-d*.18),(.65,1.6,.65),'stone')
            g.box((-w*.26,3*floors+2,-d*.18),(.8,.15,.8),'ink')
        if n=='shop':
            g.box((0,2.7,d/2+.3),(w-.4,.55,.12),'teal')
            for i in range(int(w)):g.box((-w/2+.5+i,2.35,d/2+.7),(.97,.12,1.15),'white' if i%2 else 'red',.01)
        if n=='outpost':
            for x in (-w/2,w/2):
                g.box((x,1.65,d/2),(.4,3.2,.5),'steel')
                g.box((x,2.3,d/2+.26),(.08,.9,.025),'cyan',.015,material=3)
            g.cylinder((0,3.4,-2),(0,5,-2),.07,'steel')
            g.ellipsoid((0,5,-2),(.5,.18,.5),'white',12,6)
        if n=='workshop':g.box((-3,3.8,-2),(1,1.2,1),'steel')
        entrance_x=0 if industrial else -w/2+1+int(w/4)*2
        connection(g,'entrance',(entrance_x,.2,d/2),(0,0,1),4 if industrial else 1.2)
        return g
    if n=='water-tower':
        for x in (-1.5,1.5):
            for z in (-1.5,1.5):g.beam((x,0,z),(x,6,z),.22,'steel')
        for s in (-1,1):
            g.beam((-1.5,1,s*1.5),(1.5,5,s*1.5),.12,'steel')
            g.beam((s*1.5,1,-1.5),(s*1.5,5,1.5),.12,'steel')
        g.cylinder((0,5.6,0),(0,8.5,0),2.1,'teal',sides=16)
        g.cylinder((0,8.5,0),(0,9.4,0),2.2,'steel',.2,sides=16)
        for y in (5.7,8.4):g.ring((0,y,0),2.12,.07,'steel',segments=16)
        for j in range(24):g.beam((-.28,j*.3,2.1),(.28,j*.3,2.1),.055,'silver')
        for x in (-.3,.3):g.beam((x,0,2.1),(x,7.2,2.1),.07,'steel')
        return g
    if n in ('floor','ceiling','roof-flat'):
        solid(g,(0,.1,0),(2,.2,2),'stone')
        for x in (-.5,.5):
            for z in (-.5,.5):g.box((x,.207,z),(.97,.014,.97),'ivory',.006)
    elif n in ('wall','wall-window','wall-door'):panel(g,(0,0,0),opening={'wall-window':'window','wall-door':'door'}.get(n))
    elif n in ('corner-inner','corner-outer'):
        panel(g,(0,0,-.91));panel(g,(-.91,0,0),axis='Z')
        if n=='corner-outer':g.box((-.98,1.5,-.98),(.22,3,.22),'stone')
    elif n in ('pillar','beam'):
        solid(g,(0,1.5,0) if n=='pillar' else (0,.15,0),(.3,3,.3) if n=='pillar' else (2,.3,.3),'steel')
        for y in (0,2.9) if n=='pillar' else (0,):g.box((0,y+.05,0),(.44,.1,.44),'silver')
    elif n in ('door-hinged','door-sliding','bulkhead'):door(g,sliding=n!='door-hinged',scifi=n=='bulkhead')
    elif n=='window':frame(g,(0,.7,0),1.2,1.4)
    elif n in ('stairs','ramp'):
        if n=='stairs':staircase(g,0,-1.87,0)
        else:
            g.prism([(0,-2),(0,2),(3,2)],2,'stone')
            g.colliders.append(dict(type='ramp',size=[2,3,4],center=[0,1.5,0]))
            for x in (-.94,.94):railing(g,(x,.2,-2),(x,3,2))
        connection(g,'bottom',(0,0,-2),(0,0,-1));connection(g,'top',(0,3,2),(0,0,1))
    elif n in ('railing','railing-corner','balcony'):
        railing(g,(-1,0,.9),(1,0,.9))
        if n!='railing':railing(g,(-.9,0,-1),(-.9,0,.9))
        if n=='balcony':
            solid(g,(0,.08,0),(2,.16,2),'stone');railing(g,(.9,0,-1),(.9,0,.9))
    elif n in ('roof-slope','roof-ridge','roof-gable'):
        if n=='roof-slope':g.prism([(0,-1),(.15,-1),(1.15,1),(1,1)],2,'red')
        elif n=='roof-ridge':roof(g,2,2,0)
        else:g.prism([(0,-1),(0,1),(1,0)],.18,'ivory')
    elif n.startswith('corridor') or n in ('room','airlock'):
        size=4 if n=='room' else 2
        solid(g,(0,.1,0),(size,.2,size),'stone');solid(g,(0,3.1,0),(size,.2,size))
        openings={'corridor':{'north','south'},'corridor-corner':{'north','east'},
                  'corridor-tee':{'north','east','west'},'corridor-cross':{'north','south','east','west'},
                  'room':{'south'},'airlock':{'north','south'}}[n]
        for name,at,axis,normal in [('north',(0,.2,size/2),'X',(0,0,1)),('south',(0,.2,-size/2),'X',(0,0,-1)),
                                   ('east',(size/2,.2,0),'Z',(1,0,0)),('west',(-size/2,.2,0),'Z',(-1,0,0))]:
            if name not in openings:panel(g,at,size,2.8,paint='navy' if n=='airlock' else 'ivory',axis=axis)
            else:connection(g,name,(at[0],.2,at[2]),normal,size)
        if n=='airlock':door(g,(0,.2,.95),True,True);g.box((.72,1.6,.93),(.22,.38,.09),'ink')
        for x in (-size*.4,size*.4):g.box((x,2.95,0),(.05,.025,size*.65),'cyan',.006,material=3)
    elif n=='ladder':
        for x in (-.35,.35):g.beam((x,0,0),(x,3,0),.075,'steel')
        for j in range(11):g.beam((-.35,.15+j*.27,.04),(.35,.15+j*.27,.04),.06,'silver')
    elif n=='lift':
        for x in (-.95,.95):g.box((x,1.5,-.95),(.13,3,.18),'steel')
        g.moving_part('platform',(0,0,0),'lift',(0,1,0));g.box((0,.12,0),(2,.24,2),'navy')
        for x in (-.94,.94):railing(g,(x,.24,-.9),(x,.24,.9))
        g.part='body';g.socket('floor',(0,.24,0),'platform')
    else:raise ValueError(n)
    if not g.connections:
        for name,p,normal in [('north',(0,0,1),(0,0,1)),('south',(0,0,-1),(0,0,-1)),
                              ('east',(1,0,0),(1,0,0)),('west',(-1,0,0),(-1,0,0))]:connection(g,name,p,normal)
    return g
