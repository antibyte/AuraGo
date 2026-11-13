"""Anatomical landmarks and skin masks for the non-humanoid desktop companions."""
from math import cos, sin
from mathutils import Vector

def build_profile(mesh, kind, pet):
    low=Vector([min(v.co[i] for v in mesh.data.vertices) for i in range(3)])
    high=Vector([max(v.co[i] for v in mesh.data.vertices) for i in range(3)])
    height=high.z-low.z
    assert height>.05
    origin=Vector(((low.x+high.x)/2,(low.y+high.y)/2,low.z))
    for vertex in mesh.data.vertices:
        vertex.co=(vertex.co-origin)/height
    width=(high.x-low.x)/height;depth=(high.y-low.y)/height
    specs=[]
    def add(name,head,tail,parent=None):
        assert (Vector(tail)-Vector(head)).length>.001,name
        specs.append((name,Vector(head),Vector(tail),parent))
    names=['DEF-spine','DEF-spine.001','DEF-spine.002','DEF-spine.003','DEF-spine.004','DEF-spine.006']
    if kind=='spider':
        centers=[(0,.12*depth,.28),(0,.18*depth,.48),(0,0,.45),(0,-.12*depth,.50),(0,-.22*depth,.55),(0,-.32*depth,.62)]
    elif kind=='tracked':
        centers=[(0,0,.16),(0,0,.28),(0,0,.43),(0,0,.54),(0,0,.68),(0,0,.84)]
    elif kind=='clip':
        centers=[(0,0,.03),(0,0,.20),(0,0,.36),(0,0,.51),(0,0,.64),(0,0,.75)]
    else:
        centers=[(0,0,.10),(0,0,.25),(0,0,.39),(0,0,.51),(0,-.02,.62),(0,-.04,.74)]
    for i,name in enumerate(names):
        add(name,centers[i],centers[i+1] if i<5 else Vector(centers[i])+Vector((0,0,.15)),names[i-1] if i else None)
    def chain(prefix,side,points,parent):
        for i,part in enumerate(prefix):
            add(part+'.'+side,points[i],points[i+1],prefix[i-1]+'.'+side if i else parent)
    arms=['DEF-upper_arm','DEF-forearm','DEF-hand']
    legs=['DEF-thigh','DEF-shin','DEF-foot']
    if kind=='spider':
        for side,sign in [('L',1),('R',-1)]:
            for leg in range(4):
                y=depth*(-.30+leg*.20)
                parts=arms if leg==0 else [f'Leg.{leg+1:02}.hip',f'Leg.{leg+1:02}.knee',f'Leg.{leg+1:02}.tip']
                points=[(sign*width*.15,y,.38),(sign*width*(.28+leg*.025),y*1.25,.42),(sign*width*(.35+leg*.035),y*1.5,.17),(sign*width*(.34+leg*.04),y*1.5,.02)]
                chain(parts,side,points,'DEF-spine.002')
    elif kind!='clip':
        for side,sign in [('L',1),('R',-1)]:
            if kind=='blob':
                points=[(sign*width*.29,0,.39),(sign*width*.43,-depth*.08,.29),(sign*width*.47,-depth*.12,.25)]
                chain(arms[:2],side,points,'DEF-spine.002')
            else:
                points=[(sign*width*.29,0,.52),(sign*width*.42,-depth*.05,.40),(sign*width*.45,-depth*.10,.30),(sign*width*.42,-depth*.14,.26)]
                chain(arms,side,points,'DEF-spine.003')
            if kind in ('bird','bird-tail'):
                points=[(sign*width*.17,0,.23),(sign*width*.18,0,.12),(sign*width*.18,-depth*.12,.04),(sign*width*.18,-depth*.34,.025)]
                chain(legs,side,points,'DEF-spine')
            elif kind=='tracked':
                add('Tread.'+side,(sign*width*.32,0,.13),(sign*width*.32,-depth*.20,.13),'DEF-spine')
        if kind in ('bird','bird-tail'):
            tail=[(0,depth*.13,.27),(0,depth*.27,.32),(0,depth*.40,.35),(0,depth*.49,.38)]
            for i in range(3):
                add(f'Tail.{i+1:02}',tail[i],tail[i+1],f'Tail.{i:02}' if i else 'DEF-spine')
            add('Jaw',(0,-depth*.19,.68),(0,-depth*.38,.65),'DEF-spine.006')
            if kind=='bird-tail':
                for i in range(7):
                    angle=(i-3)*.36
                    add(f'Fan.{i+1:02}',(0,depth*.16,.28),(sin(angle)*width*.49,depth*.32,.28+cos(angle)*.49),'Tail.01')
        elif kind=='blob':
            add('TentacleTip',(0,0,.79),(width*.10,0,.96),'DEF-spine.006')
    segments={name:(head,tail) for name,head,tail,_ in specs}
    def smooth(a,b,value):
        t=max(0,min(1,(value-a)/(b-a)));return t*t*(3-2*t)
    def blend(a,b,t):
        result={name:value*(1-t) for name,value in a.items()}
        for name,value in b.items():result[name]=result.get(name,0)+value*t
        return result
    def nearest(point,choices):
        distances=[]
        for name in choices:
            a,b=segments[name];v=b-a;t=max(0,min(1,(point-a).dot(v)/v.length_squared))
            distances.append((name,(point-a-v*t).length))
        values={name:1/max(.018,distance)**4 for name,distance in sorted(distances,key=lambda v:v[1])[:2]}
        total=sum(values.values());return {name:value/total for name,value in values.items()}
    def vertical(z):
        for i in range(5):
            if z<centers[i+1][2]:
                t=smooth(centers[i][2],centers[i+1][2],z)
                return {names[i]:1-t,names[i+1]:t}
        return {names[-1]:1}
    def weights(point):
        x,y,z=point;side='L' if x>=0 else 'R'
        body=vertical(z)
        if kind=='clip':
            return blend({'DEF-spine.003':1},{'DEF-spine.006':1},smooth(.40,.52,z))
        if kind=='spider':
            body=blend({'DEF-spine.001':1},{'DEF-spine.006':1},1-smooth(-depth*.03,depth*.12,y))
            limb=[n for n in segments if (n.startswith('Leg.') or n.startswith('DEF-upper_arm') or n.startswith('DEF-forearm') or n.startswith('DEF-hand')) and n.endswith('.'+side)]
            return blend(body,nearest(point,limb),smooth(width*.16,width*.28,abs(x))*(1-smooth(.37,.46,z)))
        if kind=='tracked':
            body=blend({'DEF-spine.002':1},{'DEF-spine.004':1},smooth(.50,.60,z))
            body=blend(body,{'DEF-spine.006':1},smooth(.60,.66,z))
            if z<.27:
                return blend({'DEF-spine':1},{'Tread.'+side:1},smooth(width*.20,width*.28,abs(x)))
        elif kind=='blob':
            # Keep the face and the surrounding trunk together while the base flexes.
            body=blend({'DEF-spine.001':1},{'DEF-spine.006':1},smooth(.12,.35,z))
            body=blend(body,{'TentacleTip':1},smooth(.80,.93,z))
        else:
            head_start,head_end={'openpets-default':(.40,.46),'tux':(.38,.44),'pippin':(.30,.38),'pavlo':(.50,.56)}[pet]
            body=blend(body,{'DEF-spine.006':1},smooth(head_start,head_end,z))
            if z<.25:
                body=blend(body,nearest(point,[n+'.'+side for n in legs]),smooth(width*.05,width*.13,abs(x))*(1-smooth(.17,.26,z)))
            if kind=='bird-tail':
                fan=smooth(depth*.02,depth*.16,y)*smooth(.28,.40,z)
                body=blend(body,nearest(point,[f'Fan.{i:02}' for i in range(1,8)]),fan)
            else:
                body=blend(body,nearest(point,[f'Tail.{i:02}' for i in range(1,4)]),smooth(depth*.23,depth*.39,y)*(1-smooth(.35,.45,z)))
        arm_parts=arms[:2] if kind=='blob' else arms
        arm=smooth(width*.27,width*.38,abs(x))*smooth(.22,.30,z)*(1-smooth(.51,.60,z))
        body=blend(body,nearest(point,[n+'.'+side for n in arm_parts]),arm)
        if pet=='openpets-default' and y<-depth*.13 and .20<z<.50 and abs(x)<width*.35:
            body={'DEF-spine.003':1}
        return body
    assert len({name for name,_,_,_ in specs})==len(specs)
    assert sum(parent is None for _,_,_,parent in specs)==1
    return specs,weights,{'original_bounds':[list(low),list(high)],'normalization_origin':list(origin),'normalization_height':height}
