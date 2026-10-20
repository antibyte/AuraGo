"""The 18 legacy AuraGolf courses (MIT License, see ../LICENSE).

Code moved verbatim from the monolithic ``generate.py``: lane, rails, cup,
flag, obstacles and templates are the collision-relevant geometry that P2 must
keep bit-identical (guarded by the P1 collider fingerprint in the asset test).
Only the GLB export, the bounds and the collider read-back live in ``export.py``.
"""

import bpy
import hashlib
import math
import os
from mathutils import Vector

from . import export

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
RUNTIME = os.path.join(ROOT, "ui", "3d", "minigolf")

SAMPLES_PER_SEGMENT = 12
LANE_WIDTH = 1.72
LANE_ROWS = 6
BALL_RADIUS = 0.16
CUP_RADIUS = 0.32

# Coordinates are Blender X/Y/Z (metres, Z-up). The GLB exporter converts them
# to glTF X/Y/Z (metres, Y-up); runtime coordinates are authored in that frame.
COURSES = [
    {"name":"Straightaway", "layout":"straight", "world":"garden", "par":2, "path":[(0,-4.45,0),(0,-2.2,0),(0,0,0),(0,2.2,0),(0,4.45,0)]},
    {"name":"Flower Arch", "layout":"flower_arch", "world":"garden", "par":3, "path":[(0,-4.45,0),(-.32,-2.5,0),(.26,-.7,0),(.22,1.15,0),(-.12,3.0,0),(0,4.45,0)]},
    {"name":"Banked Bend", "layout":"bank", "world":"garden", "par":3, "bank":.26, "path":[(0,-4.45,0),(.1,-2.8,0),(.72,-1.35,0),(1.12,.1,0),(.72,1.55,0),(-.05,2.9,0),(0,4.45,0)]},
    {"name":"Gentle Hill", "layout":"hill", "world":"garden", "par":3, "hill":[.51,.46,.32], "path":[(0,-4.45,0),(-.25,-2.4,0),(0,-.35,0),(.28,1.8,0),(0,4.45,0)]},
    {"name":"Pebble Bridge", "layout":"bridge", "world":"garden", "par":3, "path":[(0,-4.45,0),(.28,-2.7,0),(.45,-.8,0),(.1,1.0,0),(-.3,2.8,0),(0,4.45,0)]},
    {"name":"Windmill Gate", "layout":"windmill", "world":"garden", "par":4, "obstacle":{"kind":"rotate","t":.55,"amplitude":1.05,"speed":.72}, "path":[(0,-4.45,0),(-.18,-2.5,0),(0,-.5,0),(.18,1.5,0),(0,3.1,0),(0,4.45,0)]},
    {"name":"Cove Curve", "layout":"beach_curve", "world":"coast", "par":3, "path":[(0,-4.45,0),(-.42,-2.8,0),(-1.1,-1.25,0),(-1.0,.25,0),(-.25,1.55,0),(.48,2.8,0),(.5,4.45,0)]},
    {"name":"Dune Bunker", "layout":"sand", "world":"coast", "par":3, "sand":[.43,.60], "path":[(0,-4.45,0),(.58,-2.8,0),(.82,-1.15,0),(.24,.35,0),(-.62,1.8,0),(-.45,3.1,0),(0,4.45,0)]},
    {"name":"Old Pier", "layout":"pier", "world":"coast", "par":3, "path":[(0,-4.45,0),(.15,-2.5,0),(.18,-.55,0),(-.18,1.2,0),(-.12,2.8,0),(0,4.45,0)]},
    {"name":"Skipper's Leap", "layout":"shortjump", "world":"coast", "par":4, "gap":[.49,.555], "path":[(0,-4.45,0),(-.12,-2.7,0),(.2,-1.0,0),(.2,.8,0),(-.1,2.7,0),(0,4.45,0)]},
    {"name":"Harbor Gate", "layout":"harbor_gate", "world":"coast", "par":4, "obstacle":{"kind":"slide","t":.55,"amplitude":.56,"speed":.85}, "path":[(0,-4.45,0),(.32,-2.6,0),(.45,-.75,0),(0,.75,0),(-.42,2.5,0),(0,4.45,0)]},
    {"name":"Lighthouse Way", "layout":"lighthouse", "world":"coast", "par":4, "path":[(0,-4.45,0),(.65,-3.05,0),(1.18,-1.75,0),(.8,-.45,0),(-.35,.45,0),(-.88,1.65,0),(-.35,2.9,0),(.72,3.55,0),(1.22,4.45,0)]},
    {"name":"Lantern Curve", "layout":"lantern_curve", "world":"night", "par":3, "path":[(0,-4.45,0),(-.55,-2.85,0),(-.95,-1.15,0),(-.35,.25,0),(.55,1.45,0),(.85,2.8,0),(0,4.45,0)]},
    {"name":"Crystal Banks", "layout":"crystals", "world":"night", "par":4, "bank":.31, "path":[(0,-4.45,0),(.7,-2.9,0),(1.0,-1.35,0),(.2,-.1,0),(-.9,1.2,0),(-.6,2.75,0),(0,4.45,0)]},
    {"name":"Moonlit Bridge", "layout":"moonbridge", "world":"night", "par":4, "hill":[.52,.62,.65], "gap":[.41,.47], "path":[(0,-4.45,0),(-.45,-2.8,0),(-.65,-1.2,0),(-.15,.5,0),(.55,2.1,0),(.35,3.35,0),(0,4.45,0)]},
    {"name":"Revolving Gate", "layout":"revolving_gate", "world":"night", "par":4, "obstacle":{"kind":"rotate","t":.53,"amplitude":1.30,"speed":.95}, "path":[(0,-4.45,0),(0,-2.6,0),(.22,-.8,0),(.2,1.0,0),(0,2.85,0),(0,4.45,0)]},
    {"name":"Lily Pad Leap", "layout":"lilyjump", "world":"night", "par":4, "gap":[.48,.535], "path":[(0,-4.45,0),(.32,-2.8,0),(.5,-1.2,0),(.08,.35,0),(-.45,1.8,0),(-.22,3.0,0),(0,4.45,0)]},
    {"name":"Grand Finale", "layout":"finale", "world":"night", "par":4, "hill":[.39,.60,.42], "path":[(0,-4.45,0),(0,-2.8,.02),(.72,-1.4,.24),(1.25,-.1,.48),(.95,1.2,.52),(.1,2.0,.48),(-.9,2.8,.30),(-.75,3.65,.10),(0,4.45,0)]},
]


def runtime(v):
    return [float(v[0]), float(v[2]), float(-v[1])]


def blender(v):
    return (float(v[0]), float(-v[2]), float(v[1]))


def q(v):
    return [round(float(x), 5) for x in v]


def theme(world):
    if world == "garden":
        return {"grass":(.23,.53,.24,1),"sand":(.72,.57,.31,1),"wood":(.34,.20,.10,1),"stone":(.40,.48,.44,1),"water":(.10,.43,.50,1),"rock":(.31,.38,.33,1),"leaf":(.20,.44,.18,1),"accent":(.97,.49,.22,1),"light":(1.0,.79,.34,1),"sky":(.46,.68,.77,1)}
    if world == "coast":
        return {"grass":(.28,.48,.28,1),"sand":(.83,.68,.43,1),"wood":(.43,.28,.15,1),"stone":(.52,.57,.54,1),"water":(.06,.39,.58,1),"rock":(.39,.48,.49,1),"leaf":(.20,.48,.34,1),"accent":(.96,.38,.18,1),"light":(1.0,.78,.35,1),"sky":(.54,.76,.87,1)}
    return {"grass":(.095,.22,.30,1),"sand":(.28,.22,.36,1),"wood":(.12,.09,.26,1),"stone":(.26,.32,.55,1),"water":(.025,.06,.22,1),"rock":(.15,.19,.40,1),"leaf":(.08,.30,.34,1),"accent":(.52,.28,.92,1),"light":(1.0,.68,.24,1),"sky":(.025,.04,.11,1)}


def material(key, color, roughness=.78, metallic=0.0, emission=0.0):
    name = "minigolf-" + key
    mat = bpy.data.materials.get(name)
    if mat is None:
        mat = bpy.data.materials.new(name)
        mat.diffuse_color = color
        mat.use_nodes = True
        bsdf = mat.node_tree.nodes.get("Principled BSDF")
        bsdf.inputs["Base Color"].default_value = color
        bsdf.inputs["Roughness"].default_value = roughness
        bsdf.inputs["Metallic"].default_value = metallic
        if emission:
            bsdf.inputs["Emission Color"].default_value = color
            bsdf.inputs["Emission Strength"].default_value = emission
    return mat


def link_collection(scene, name):
    coll = bpy.data.collections.new(name)
    scene.collection.children.link(coll)
    return coll


def mesh_object(scene, coll, name, verts, faces, mat, role="scenery", location=None):
    mesh = bpy.data.meshes.new(name + "-mesh")
    mesh.from_pydata(verts, [], faces)
    mesh.update()
    obj = bpy.data.objects.new(name, mesh)
    coll.objects.link(obj)
    if mat:
        obj.data.materials.append(mat)
    obj["aurago_role"] = role
    if location:
        obj.location = location
    return obj


def mesh_from_circle(cx, cy, z, radius, segments=12):
    return [(cx + radius*math.cos(i*2*math.pi/segments), cy + radius*math.sin(i*2*math.pi/segments), z) for i in range(segments)]


def ring(scene, coll, name, center, inner, outer, z, mat, segments=16):
    verts=[]
    for r in (inner,outer):
        verts.extend(mesh_from_circle(center[0],center[1],z,r,segments))
    faces=[]
    for i in range(segments):
        j=(i+1)%segments
        faces.append((i,j,segments+j)); faces.append((i,segments+j,segments+i))
    return mesh_object(scene,coll,name,verts,faces,mat,"decoration")


def box_mesh(scene, coll, name, center, size, mat, role="scenery", local=False):
    x,y,z=center; sx,sy,sz=(v*.5 for v in size)
    verts=[((0 if local else x)+dx*sx,(0 if local else y)+dy*sy,(0 if local else z)+dz*sz) for dx,dy,dz in [(-1,-1,-1),(1,-1,-1),(1,1,-1),(-1,1,-1),(-1,-1,1),(1,-1,1),(1,1,1),(-1,1,1)]]
    faces=[(0,3,2),(0,2,1),(4,5,6),(4,6,7),(0,1,5),(0,5,4),(1,2,6),(1,6,5),(2,3,7),(2,7,6),(3,0,4),(3,4,7)]
    if local:
        return mesh_object(scene,coll,name,verts,faces,mat,role,location=center)
    return mesh_object(scene,coll,name,verts,faces,mat,role)


def cylinder(scene, coll, name, center, radius, depth, mat, segments=8, top_radius=None, role="scenery"):
    x,y,z=center; top_radius=radius if top_radius is None else top_radius
    verts=[]
    for zz,rr in ((z-depth/2,radius),(z+depth/2,top_radius)):
        verts.extend((x+rr*math.cos(i*2*math.pi/segments),y+rr*math.sin(i*2*math.pi/segments),zz) for i in range(segments))
    faces=[]
    faces.append(tuple(reversed(range(segments)))); faces.append(tuple(range(segments,2*segments)))
    for i in range(segments):
        j=(i+1)%segments; faces.append((i,j,segments+j,segments+i))
    return mesh_object(scene,coll,name,verts,faces,mat,role)


def ico_sphere(scene, coll, name, center, radius, mat, role="scenery"):
    phi=(1+math.sqrt(5))/2
    base=[(-1,phi,0),(1,phi,0),(-1,-phi,0),(1,-phi,0),(0,-1,phi),(0,1,phi),(0,-1,-phi),(0,1,-phi),(phi,0,-1),(phi,0,1),(-phi,0,-1),(-phi,0,1)]
    norms=[math.sqrt(x*x+y*y+z*z) for x,y,z in base]
    verts=[(center[0]+radius*x/n,center[1]+radius*y/n,center[2]+radius*z/n) for (x,y,z),n in zip(base,norms)]
    faces=[(0,11,5),(0,5,1),(0,1,7),(0,7,10),(0,10,11),(1,5,9),(5,11,4),(11,10,2),(10,7,6),(7,1,8),(3,9,4),(3,4,2),(3,2,6),(3,6,8),(3,8,9),(4,9,5),(2,4,11),(6,2,10),(8,6,7),(9,8,1)]
    return mesh_object(scene,coll,name,verts,faces,mat,role)


def cone(scene, coll, name, center, radius, height, mat, segments=7):
    return cylinder(scene,coll,name,center,radius,height,mat,segments,0.0)


def catmull_path(control):
    pts=[]
    for seg in range(len(control)-1):
        p0=control[max(0,seg-1)]; p1=control[seg]; p2=control[seg+1]; p3=control[min(len(control)-1,seg+2)]
        for step in range(SAMPLES_PER_SEGMENT):
            t=step/SAMPLES_PER_SEGMENT; t2=t*t; t3=t2*t
            pts.append(tuple(.5*((2*p1[k])+(-p0[k]+p2[k])*t+(2*p0[k]-5*p1[k]+4*p2[k]-p3[k])*t2+(-p0[k]+3*p1[k]-3*p2[k]+p3[k])*t3) for k in range(3)))
    pts.append(control[-1])
    return pts


def lane_center_height(course, path, index):
    """Return the authored fairway height at the lane center for a path sample."""
    t = index / (len(path) - 1)
    height = path[index][2]
    hill = course.get("hill")
    if hill:
        center, amplitude, width = hill
        height += amplitude * max(0, 1 - abs(t - center) / width)
    return height


def rail_edge(course, path, i, sign):
    """Inner and outer rail edge at path sample i (Blender XY) and the lane height there.

    The fourth value is the unit vector pointing away from the lane on that side.
    ``make_surface`` builds the collision rails from it and the P2 trims reuse it,
    so both follow exactly the same edge.
    """
    n=len(path)
    p=path[i]; prev=path[max(0,i-1)]; nxt=path[min(n-1,i+1)]
    dx=nxt[0]-prev[0]; dy=nxt[1]-prev[1]; ln=max(.0001,math.hypot(dx,dy)); sx=dy/ln; sy=-dx/ln
    h=lane_center_height(course,path,i); t=i/(n-1)
    edge=sign*LANE_WIDTH/2; outer=edge+sign*.14
    bank=course.get("bank",0)
    if bank:
        h+=bank*edge*math.sin(math.pi*max(0,min(1,(t-.18)/.66)))
    return (p[0]+sx*edge,p[1]+sy*edge),(p[0]+sx*outer,p[1]+sy*outer),h,(sx*sign,sy*sign)


def make_surface(scene, coll, course, mats):
    path=catmull_path(course["path"]); n=len(path)
    cup_t=.91; cup_i=round(cup_t*(n-1)); cup_path=path[cup_i]
    cup=(cup_path[0],cup_path[1],lane_center_height(course,path,cup_i))
    gap=course.get("gap"); sand=course.get("sand"); bank=course.get("bank",0)
    verts=[]
    for i,p in enumerate(path):
        t=i/(n-1); prev=path[max(0,i-1)]; nxt=path[min(n-1,i+1)]
        dx=nxt[0]-prev[0]; dy=nxt[1]-prev[1]; ln=max(.0001,math.hypot(dx,dy)); sx=dy/ln; sy=-dx/ln
        h=lane_center_height(course,path,i)
        for row in range(LANE_ROWS+1):
            across=-LANE_WIDTH/2+LANE_WIDTH*row/LANE_ROWS
            bank_term=bank*across*math.sin(math.pi*max(0,min(1,(t-.18)/.66))) if bank else 0
            verts.append((p[0]+sx*across,p[1]+sy*across,h+bank_term))
    groups={"grass":[],"sand":[]}
    for i in range(n-1):
        t=(i+.5)/(n-1)
        if gap and gap[0] <= t <= gap[1]:
            continue
        surf="sand" if sand and sand[0] <= t <= sand[1] else "grass"
        for j in range(LANE_ROWS):
            a=i*(LANE_ROWS+1)+j; d=a+1; b=a+LANE_ROWS+1; c=b+1
            mid=((verts[a][0]+verts[b][0]+verts[c][0]+verts[d][0])/4,(verts[a][1]+verts[b][1]+verts[c][1]+verts[d][1])/4)
            if math.dist(mid,(cup[0],cup[1])) < CUP_RADIUS+.13:
                continue
            groups[surf].extend([(a,d,c),(a,c,b)])
    for surf,faces in groups.items():
        if not faces:
            continue
        name="surface-"+surf
        mesh_object(scene,coll,name,verts,faces,mats[surf],"play-surface")
    # Low continuous side bumpers keep ordinary shots on the fairway. These are
    # generated from the same sampled edges as the lane and exported as meshes.
    for side_name,sign in (("rail-left",-1),("rail-right",1)):
        rail_verts=[]; rail_faces=[]
        for i in range(n):
            (ix,iy),(ox,oy),h,_=rail_edge(course,path,i,sign)
            rail_verts.extend([(ix,iy,h-.045),(ox,oy,h-.045),(ix,iy,h+.24),(ox,oy,h+.24)])
        for i in range(n-1):
            a=i*4; b=(i+1)*4
            rail_faces.extend([(a,b,b+2),(a,b+2,a+2),(a+1,a+3,b+3),(a+1,b+3,b+1),(a+2,b+2,b+3),(a+2,b+3,a+3),(a,a+1,b+1),(a,b+1,b)])
        # End caps close the wood edge while leaving the putting ends open.
        rail_faces.extend([(0,2,3),(0,3,1),(4*(n-1),4*(n-1)+1,4*(n-1)+3),(4*(n-1),4*(n-1)+3,4*(n-1)+2)])
        rail_surface="stone" if course["layout"]=="crystals" else "wood"
        mesh_object(scene,coll,side_name,rail_verts,rail_faces,mats[rail_surface],"boundary")
    return path,cup


def add_island(scene, coll, mats, world):
    # Faceted earth plinth and a shallow colored water apron make every course a
    # compact diorama while leaving the cup depth visually open.
    water=mats["water"]
    water_verts=[(-6.15,-6.15,-1.15),(6.15,-6.15,-1.15),(6.15,6.15,-1.15),(-6.15,6.15,-1.15)]
    mesh_object(scene,coll,"water-plane",water_verts,[(0,1,2),(0,2,3)],water,"water")
    segments=14; verts=[]
    for ring_i,(rx,ry,z) in enumerate(((4.95,4.95,-.04),(4.62,4.62,-.70),(4.3,4.3,-1.25))):
        for i in range(segments):
            a=2*math.pi*i/segments; wobble=1+.045*math.sin(i*2.3+ring_i)
            verts.append((rx*wobble*math.cos(a),ry*wobble*math.sin(a),z+(0.08*math.sin(i*1.6) if ring_i==0 else 0)))
    faces=[]
    for r in range(2):
        for i in range(segments):
            a=r*segments+i; b=r*segments+(i+1)%segments; c=(r+1)*segments+(i+1)%segments; d=(r+1)*segments+i
            faces.extend([(a,b,c),(a,c,d)])
    faces.append(tuple(reversed(range(2*segments,3*segments))))
    earth=material("earth-"+world,(.30,.25,.17,1) if world=="garden" else ((.47,.37,.24,1) if world=="coast" else (.19,.20,.28,1)))
    mesh_object(scene,coll,"island-rock-plinth",verts,faces,earth,"scenery")
    # Mossy top inlay on the plinth.
    top=[]
    for i in range(segments):
        a=2*math.pi*i/segments; w=1+.045*math.sin(i*2.3)
        top.append((4.95*w*math.cos(a),4.95*w*math.sin(a),-.04+.08*math.sin(i*1.6)))
    mesh_object(scene,coll,"island-turf-edge",[(0,0,-.04)]+top,[(0,1+i,1+(i+1)%segments) for i in range(segments)],mats["grass"],"scenery")


def add_tree(scene, coll, x, y, z, mats, height=1.8, palm=False):
    cylinder(scene,coll,"palm-trunk" if palm else "garden-tree-trunk",(x,y,z+height*.34),.095,height*.68,mats["wood"],6,top_radius=.06)
    if palm:
        for i in range(6):
            a=i*math.tau/6
            ex=x+math.cos(a)*.78; ey=y+math.sin(a)*.78
            verts=[(x,y,z+height*.65),(x+math.cos(a+.42)*.18,y+math.sin(a+.42)*.18,z+height*.9),(ex,ey,z+height*.76),(x+math.cos(a-.42)*.18,y+math.sin(a-.42)*.18,z+height*.9)]
            mesh_object(scene,coll,"palm-frond",verts,[(0,1,2),(0,2,3)],mats["leaf"],"scenery")
    else:
        ico_sphere(scene,coll,"faceted-canopy",(x,y,z+height*.75),height*.31,mats["leaf"])
        ico_sphere(scene,coll,"faceted-canopy",(x-.19,y,z+height*.94),height*.22,mats["leaf"])
        ico_sphere(scene,coll,"faceted-canopy",(x+.2,y+.06,z+height*.91),height*.23,mats["leaf"])


def add_flower(scene, coll, x, y, z, mats, color, scale=.14):
    petal=material("flower-"+"-".join(f"{v:.2f}" for v in color[:3]),color)
    cylinder(scene,coll,"flower-stem",(x,y,z+scale*.48),.018,scale*.95,mats["leaf"],5)
    ico_sphere(scene,coll,"flower-center",(x,y,z+scale),scale*.17,mats["light"])
    for i in range(5):
        a=i*math.tau/5
        ico_sphere(scene,coll,"flower-petal",(x+math.cos(a)*scale*.37,y+math.sin(a)*scale*.37,z+scale),scale*.28,petal)


def add_lighthouse(scene, coll, x, y, z, mats, scale=1.0):
    stone=material("lighthouse-ivory",(.88,.82,.66,1))
    red=material("lighthouse-red",(.78,.20,.15,1))
    cylinder(scene,coll,"lighthouse-tower",(x,y,z+.82*scale),.34*scale,1.64*scale,stone,8,top_radius=.25*scale)
    for dz in (.42,1.22):
        cylinder(scene,coll,"lighthouse-red-band",(x,y,z+dz*scale),.335*scale,.17*scale,red,8,top_radius=.30*scale)
    for dz in (.52,1.18):
        box_mesh(scene,coll,"lighthouse-window",(x,y-.30*scale,z+dz*scale),(.12*scale,.04,.20*scale),mats["accent"])
    cylinder(scene,coll,"lighthouse-lantern-room",(x,y,z+1.73*scale),.28*scale,.20*scale,mats["light"],8)
    cone(scene,coll,"lighthouse-roof",(x,y,z+1.91*scale),.38*scale,.34*scale,red,8)
    glow=material("beacon-glow",mats["light"].diffuse_color,roughness=.32,emission=1.8)
    cylinder(scene,coll,"lighthouse-beacon",(x,y,z+1.74*scale),.20*scale,.12*scale,glow,8)


def add_arch(scene, coll, x, y, z, mats):
    # Two carved posts and a chunky, segmented curved lintel frame the lane.
    for side in (-1,1):
        cylinder(scene,coll,"flower-arch-post",(x+side*1.20,y,z+.76),.10,1.52,mats["wood"],7)
        for j in range(4):
            add_flower(scene,coll,x+side*(1.05+.12*(j%2)),y-.52+j*.34,z+.48+.25*(j%2),mats,((.96,.30,.39,1) if j%2 else (.99,.70,.22,1)),.13)
    for i in range(11):
        t=i/10; xx=x-1.2+t*2.4; zz=z+1.32+.48*math.sin(t*math.pi)
        slope=.48*math.pi/2.4*math.cos(t*math.pi)
        segment=box_mesh(scene,coll,"flower-arch-lintel",(0,0,0),(.30,.28,.25),mats["leaf"],local=True)
        segment.location=(xx,y,zz); segment.rotation_euler[1]=-math.atan(slope)
        if i in (2,5,8):
            add_flower(scene,coll,xx,y-.19,zz+.08,mats,((.98,.35,.42,1) if i==5 else (.99,.74,.25,1)),.16)


def add_bridge(scene, coll, x, y, z, mats, moon=False):
    stone=material("moon-bridge-stone",(.44,.48,.62,1) if moon else (.50,.43,.30,1))
    # A pair of side parapets and stylized support arches sit just outside the
    # lane edges; the playable track remains the sole deck collider.
    for side in (-1,1):
        sx=x+side*.98
        box_mesh(scene,coll,"bridge-parapet",(sx,y,z+.27),( .14,2.55,.46),stone)
        for k in range(4):
            box_mesh(scene,coll,"bridge-baluster",(sx,y-1.0+k*.66,z+.46),(.19,.10,.38),mats["accent"])
    for xx in (x-.72,x+.72):
        cylinder(scene,coll,"bridge-pile",(xx,y,z-.25),.12,.55,mats["wood"],6)


def add_lantern(scene, coll, x, y, z, mats):
    cylinder(scene,coll,"lantern-post",(x,y,z+.68),.055,1.36,mats["wood"],6)
    glow=material("lantern-glow",mats["light"].diffuse_color,roughness=.3,emission=2.2)
    ico_sphere(scene,coll,"lantern-glass",(x,y,z+1.34),.17,glow)
    cone(scene,coll,"lantern-cap",(x,y,z+1.53),.20,.18,mats["wood"])


def island_top_height(x, y, segments=14):
    # Barycentric height of the island turf fan triangle under (x, y). The cup
    # interior must sit above this surface or the fan hides the dark bottom.
    ring = []
    for i in range(segments):
        a = 2 * math.pi * i / segments
        w = 1 + .045 * math.sin(i * 2.3)
        ring.append((4.95 * w * math.cos(a), 4.95 * w * math.sin(a), -.04 + .08 * math.sin(i * 1.6)))
    for i in range(segments):
        j = (i + 1) % segments
        ax, ay, az = ring[i]
        bx, by, bz = ring[j]
        det = ax * by - ay * bx
        if abs(det) < 1e-9:
            continue
        u = (x * by - y * bx) / det
        v = (ax * y - ay * x) / det
        if u >= -1e-6 and v >= -1e-6 and u + v <= 1 + 1e-6:
            return -.04 * (1 - u - v) + az * u + bz * v
    return -.04


def cup_lining(scene, coll, cup, mat, bottom_z):
    # Tapered inner wall between the turf cut and the dark cup bottom. It is
    # scenery only: physics keeps the authored open cup with no floor.
    cx, cy, cz = cup
    seg = 18
    verts = []
    for zz, rr in ((bottom_z, CUP_RADIUS + .02), (cz + .004, CUP_RADIUS + .12)):
        verts.extend((cx + rr * math.cos(i * math.tau / seg), cy + rr * math.sin(i * math.tau / seg), zz) for i in range(seg))
    faces = [(i, seg + i, seg + (i + 1) % seg, (i + 1) % seg) for i in range(seg)]
    mesh_object(scene, coll, "cup-lining", verts, faces, mat, "decoration")


HOLE_DIGITS = {
    "0": "abcdef", "1": "bc", "2": "abged", "3": "abgcd", "4": "fgbc",
    "5": "afgcd", "6": "afgedc", "7": "abc", "8": "abcdefg", "9": "abcdfg"
}


def hole_number(scene, coll, cup, number, mat):
    # Raised seven-segment hole number on the flag pole, readable without text.
    digits = str(number)
    fx = cup[0] + .43
    fy = cup[1]
    z = cup[2] + .40
    dw, dh, t = .09, .15, .03
    step = dw + .05
    width = len(digits) * dw + (len(digits) - 1) * .05
    for index, char in enumerate(digits):
        cx = fx + .20 - width / 2 + dw / 2 + index * step
        for segment in HOLE_DIGITS.get(char, ""):
            if segment in "agd":
                zz = z + dh / 2 if segment == "a" else (z - dh / 2 if segment == "d" else z)
                box_mesh(scene, coll, "hole-digit-segment", (cx, fy, zz), (dw + t, .05, t), mat, "decoration")
            else:
                xx = cx + dw / 2 if segment in "bc" else cx - dw / 2
                zz = z + dh / 4 if segment in "bf" else z - dh / 4
                box_mesh(scene, coll, "hole-digit-segment", (xx, fy, zz), (t, .05, dh / 2), mat, "decoration")


def add_fence(scene, coll, x, y, z, mats, length=1.7, along="y"):
    half = length / 2
    for s in (-1, 1):
        px, py = (x, y + s * half) if along == "y" else (x + s * half, y)
        box_mesh(scene, coll, "deco-fence-post", (px, py, z + .34), (.10, .10, .68), mats["wood"])
    for lift in (.24, .52):
        size = (.055, length, .075) if along == "y" else (length, .055, .075)
        box_mesh(scene, coll, "deco-fence-rail", (x, y, z + lift), size, mats["wood"])


def add_bench(scene, coll, x, y, z, mats):
    box_mesh(scene, coll, "deco-bench-seat", (x, y, z + .30), (1.05, .34, .07), mats["wood"])
    box_mesh(scene, coll, "deco-bench-back", (x, y + .15, z + .52), (1.05, .06, .34), mats["wood"])
    for s in (-1, 1):
        box_mesh(scene, coll, "deco-bench-leg", (x + s * .44, y, z + .14), (.09, .28, .28), mats["stone"])


def add_driftwood(scene, coll, x, y, z, mats, angle=0.0):
    for i, (length, tilt) in enumerate(((1.0, .0), (.72, .35))):
        log = cylinder(scene, coll, "deco-driftwood", (0, 0, 0), .07, length, mats["wood"], 6)
        log.location = (x + i * .18, y + (i - 1) * .22, z + .09 + i * .10)
        log.rotation_euler = (0, math.pi / 2 + tilt, angle + i * .9)


def add_buoy(scene, coll, x, y, z, mats):
    ico_sphere(scene, coll, "deco-buoy", (x, y, z), .17, mats["accent"])
    cone(scene, coll, "deco-buoy-cap", (x, y, z + .24), .09, .20, mats["light"], 6)
    cylinder(scene, coll, "deco-buoy-post", (x, y, z + .40), .03, .30, mats["wood"], 5)


def add_pebble_cluster(scene, coll, x, y, z, mats, phase=0.0):
    for i in range(4):
        a = phase + i * math.tau / 4
        r = .16 + .06 * ((i + int(phase * 10)) % 3)
        ico_sphere(scene, coll, "deco-pebble", (x + math.cos(a) * r, y + math.sin(a) * r, z + .04 + .03 * (i % 2)), .10 + .04 * (i % 3), mats["rock"])


def add_shard(scene, coll, x, y, z, mat, height=.55):
    cylinder(scene, coll, "deco-shard", (x, y, z + height / 2), .13, height, mat, 5, top_radius=.02)


def add_obstacle(scene, coll, spec, path, mats):
    if not spec:
        return []
    t=spec["t"]; i=round(t*(len(path)-1)); x,y,h=path[i]; h+=.36
    name="obstacle-"+("windmill-gate" if spec["kind"]=="rotate" and spec["speed"]<.8 else ("revolving-gate" if spec["kind"]=="rotate" else "harbor-gate"))
    if spec["kind"]=="rotate":
        obj=box_mesh(scene,coll,name,(0,0,0),(1.45,.16,.18),mats["accent"],"dynamic-obstacle",local=True)
        # A second short paddle creates a readable low-poly windmill silhouette.
        extra=box_mesh(scene,coll,"obstacle-cross-paddle",(0,0,0),(.16,1.08,.16),mats["wood"],"dynamic-obstacle",local=True)
        extra.parent=obj; extra.location=(0,0,0); extra["aurago_obstacle_component"]="cross-paddle"
        obj.location=(x,y,h)
        obj["aurago_obstacle"] = True
        obj["aurago_obstacle_node"] = name
        parts=[{"position":[0,0,0],"halfExtents":[.725,.09,.08]},{"position":[0,0,0],"halfExtents":[.08,.08,.54]}]
        half_extents=[.725,.09,.54]
    else:
        obj=box_mesh(scene,coll,name,(0,0,0),(1.38,.16,.72),mats["accent"],"dynamic-obstacle",local=True)
        obj.location=(x,y,h+.08); obj["aurago_obstacle"] = True; obj["aurago_obstacle_node"] = name
        half_extents=[.69,.36,.08]
        parts=[{"position":[0,0,0],"halfExtents":half_extents.copy()}]
    axis=[0,1,0] if spec["kind"]=="rotate" else [1,0,0]
    return [{"id":name.replace("obstacle-",""),"node":name,"kind":spec["kind"],"position":q(runtime(obj.location)),"halfExtents":half_extents,"colliders":parts,"axis":axis,"amplitude":spec["amplitude"],"speed":spec["speed"],"phase":0}]


def add_special(scene, coll, course, path, mats):
    layout=course["layout"]
    at=lambda t: path[round(t*(len(path)-1))]
    if layout=="flower_arch":
        p=at(.52); add_arch(scene,coll,p[0],p[1],p[2],mats)
    elif layout=="bridge":
        p=at(.5); add_bridge(scene,coll,p[0],p[1],p[2],mats)
    elif layout=="pier":
        for i in range(10):
            p=at(.16+i*.071)
            box_mesh(scene,coll,"pier-plank",(p[0],p[1],p[2]-.035),(1.65,.16,.06),mats["wood"])
        for side in (-1,1):
            for i in range(5):
                p=at(.12+i*.17); cylinder(scene,coll,"pier-pile",(p[0]+side*.92,p[1],p[2]-.52),.08,.98,mats["wood"],6)
    elif layout=="shortjump":
        for t in (.43,.60):
            p=at(t)
            box_mesh(scene,coll,"leap-ramp",(p[0],p[1],p[2]-.04),(1.78,.42,.14),mats["wood"])
    elif layout=="lilyjump":
        for j,t in enumerate((.485,.51,.535)):
            p=at(t); off=(-.24 if j==0 else (.23 if j==2 else 0))
            cylinder(scene,coll,"lily-pad",(p[0]+off,p[1],p[2]-.12),.38,.12,mats["leaf"],9)
            ico_sphere(scene,coll,"lily-flower",(p[0]+off,p[1],p[2]-.02),.10,mats["accent"])
    elif layout in ("moonbridge",):
        p=at(.50); add_bridge(scene,coll,p[0],p[1],p[2],mats,True)
    elif layout=="lighthouse":
        add_lighthouse(scene,coll,3.35,1.85,-.04,mats,1.28)
    elif layout=="crystals":
        crystal=material("crystal",(.34,.61,.82,1),roughness=.25,metallic=.12)
        for x,y,r,h in [(-3.5,-2.6,.32,1.2),(3.45,-2.1,.38,1.5),(-3.55,.5,.30,1.1),(3.55,2.55,.42,1.55)]:
            cylinder(scene,coll,"crystal-cluster",(x,y,-.04+h/2),r,h,crystal,6,top_radius=.025)
    elif layout in ("lantern_curve","revolving_gate","finale"):
        for x,y in ((-3.75,-2.7),(3.75,-.8),(-3.65,2.0),(3.65,3.3)):
            add_lantern(scene,coll,x,y,-.04,mats)
    elif layout=="bank":
        stone=mats["stone"]
        for side in (-1,1):
            p=at(.44)
            ico_sphere(scene,coll,"banked-rock-outcrop",(p[0]+side*1.55,p[1],-.15),.40,stone)
    elif layout=="beach_curve":
        for x,y in [(-3.7,-2.8),(3.55,-1.7),(-3.8,1.2),(3.65,3.0)]:
            add_tree(scene,coll,x,y,-.45,mats,1.8,True)
    elif layout=="harbor_gate":
        for side in (-1,1):
            cylinder(scene,coll,"harbor-post",(side*1.22,0,-.02),.11,1.65,mats["wood"],8)
    elif layout=="windmill":
        p=at(.55)
        mx=p[0]+1.48; my=p[1]; mz=p[2]
        cylinder(scene,coll,"windmill-mast",(mx,my,mz+1.05),.09,2.10,mats["wood"],8,top_radius=.07)
        cylinder(scene,coll,"windmill-hub",(mx,my,mz+2.13),.19,.22,mats["light"],8)
        for i,angle in enumerate((0,math.pi/2,math.pi,math.pi*1.5)):
            center=(mx+.48*math.cos(angle),my,mz+2.13+.48*math.sin(angle))
            sail=box_mesh(scene,coll,"windmill-sail",(0,0,0),(.88,.15,.24),mats["accent"] if i%2 else mats["light"],local=True)
            sail.location=center; sail.rotation_euler[1]=angle
    elif layout=="revolving_gate":
        p=at(.53)
        cylinder(scene,coll,"gate-hub",(p[0],p[1],.30),.18,.20,mats["light"],8)
    elif layout=="finale":
        p=at(.76); add_lighthouse(scene,coll,-3.25,p[1]+.5,-.04,mats,.7)


def add_decor(scene, coll, course, mats, seed):
    world=course["world"]
    add_island(scene,coll,mats,world)
    # Twelve perimeter props are positioned well outside the fairway. A fixed
    # seed makes all authored scenes and exports byte-stable.
    props=[(-4.25,-3.45),(-3.9,-1.3),(-4.28,1.2),(-3.6,3.55),(4.15,-3.2),(4.25,-.8),(3.95,1.3),(4.28,3.55),(-2.6,-4.35),(2.55,-4.3),(-2.9,4.15),(2.9,4.18)]
    for i,(x,y) in enumerate(props):
        if world=="coast" and i%3==0:
            add_tree(scene,coll,x,y,-.04,mats,1.75,True)
        elif world=="night" and i%3==1:
            add_lantern(scene,coll,x,y,-.04,mats)
        elif i%3==0:
            add_tree(scene,coll,x,y,-.04,mats,1.55+(i%2)*.35)
        else:
            ico_sphere(scene,coll,"faceted-rock",(x,y,.12),.28+(i%2)*.11,mats["rock"])
    for i in range(14):
        a=2*math.pi*i/14; x=3.65*math.cos(a); y=3.65*math.sin(a)
        # Sparse flowers keep the green readable and frame the course edges.
        if i%2==0:
            add_flower(scene,coll,x,y,-.02,mats,((.97,.38,.34,1) if i%4==0 else (.92,.72,.26,1)),.13)
    if world=="coast":
        for x,y in ((-4.85,-.1),(4.85,2.6)):
            for j in range(3):
                ico_sphere(scene,coll,"shore-rock",(x+(j-1)*.2,y+j*.22,-.04),.26,mats["rock"])
    # Pebble clusters fill the odd slots of the proven flower ring.
    for i in (1, 5, 9, 13):
        a = i * math.tau / 14
        add_pebble_cluster(scene, coll, 3.65 * math.cos(a), 3.65 * math.sin(a), -.04, mats, a)
    if world == "garden":
        add_fence(scene, coll, -4.32, -2.35, -.04, mats, 1.7, "y")
        add_fence(scene, coll, 4.3, 2.1, -.04, mats, 1.5, "y")
        add_bench(scene, coll, -4.35, -.05, -.04, mats)
    elif world == "coast":
        add_driftwood(scene, coll, -4.5, -.05, -.04, mats, .4)
        add_driftwood(scene, coll, 4.45, -.05, -.04, mats, 2.1)
        for a in (.8, 2.4, 4.0, 5.5):
            add_buoy(scene, coll, 5.45 * math.cos(a), 5.45 * math.sin(a), -1.02, mats)
    else:
        shard_mat = material("deco-shard-glow-" + world, tuple(mats["accent"].diffuse_color), roughness=.3, emission=.9)
        for x, y, height in ((-4.4, -1.6, .48), (4.4, 1.3, .60), (-1.0, -4.5, .54)):
            add_shard(scene, coll, x, y, -.04, shard_mat, height)


def add_templates(scene, coll, mats, tee):
    white=material("golf-ball",(.96,.96,.91,1),roughness=.22)
    gold=material("putter-metal",(.64,.70,.74,1),roughness=.26,metallic=.68)
    # The renderer hides these tagged source meshes in the course instance,
    # then clones them for the live ball and putter. Keep all template geometry
    # in local coordinates around the origin so the runtime can position it.
    ball=ico_sphere(scene,coll,"ball-template",(0,0,0),BALL_RADIUS,white,"template")
    ball["aurago_template"]="ball"
    # The Blender Z axis becomes runtime Y: the shaft runs from 0 to .82 m,
    # while the short, broad head sits at its lower end.
    shaft_verts=[]
    segments=7
    for z in (0,.82):
        shaft_verts.extend((.035*math.cos(i*math.tau/segments),.035*math.sin(i*math.tau/segments),z) for i in range(segments))
    shaft_faces=[tuple(reversed(range(segments))),tuple(range(segments,segments*2))]
    for i in range(segments):
        j=(i+1)%segments
        shaft_faces.append((i,j,segments+j,segments+i))
    hx,hy,hz=.20,.065,.045; cx,cy,cz=0,0,.055
    head_verts=[(cx+dx*hx,cy+dy*hy,cz+dz*hz) for dx,dy,dz in [(-1,-1,-1),(1,-1,-1),(1,1,-1),(-1,1,-1),(-1,-1,1),(1,-1,1),(1,1,1),(-1,1,1)]]
    offset=len(shaft_verts)
    head_faces=[(0,3,2),(0,2,1),(4,5,6),(4,6,7),(0,1,5),(0,5,4),(1,2,6),(1,6,5),(2,3,7),(2,7,6),(3,0,4),(3,4,7)]
    shaft=mesh_object(scene,coll,"putter-template",shaft_verts+head_verts,shaft_faces+[tuple(offset+i for i in face) for face in head_faces],gold,"template")
    shaft["aurago_template"]="putter"


def add_studio(scene, studio, theme_data):
    camera_data=bpy.data.cameras.new("Course Review Camera"); camera_data.type="ORTHO"; camera_data.ortho_scale=13.4
    camera=bpy.data.objects.new("Course Review Camera",camera_data); studio.objects.link(camera); camera.location=(9.2,-11.4,12.3)
    direction=Vector((0,0,-.25))-camera.location; camera.rotation_euler=direction.to_track_quat("-Z","Y").to_euler(); scene.camera=camera
    for name,loc,power,size in (("Key",(1,-5,11),1550,8),("Fill",(-7,1,7),950,7),("Rim",(5,7,8),1200,6)):
        data=bpy.data.lights.new(name,"AREA"); data.energy=power; data.shape="DISK"; data.size=size
        obj=bpy.data.objects.new(name,data); studio.objects.link(obj); obj.location=loc; obj.rotation_euler=(Vector((0,0,0))-obj.location).to_track_quat("-Z","Y").to_euler()
    world=bpy.data.worlds.new("AuraGolf Studio") if not bpy.data.worlds else bpy.data.worlds[0]
    scene.world=world; world.use_nodes=True; world.node_tree.nodes["Background"].inputs["Color"].default_value=theme_data["sky"]; world.node_tree.nodes["Background"].inputs["Strength"].default_value=.65


def build_scene(number, course, dresser=None):
    """Create the course scene (geometry only, nothing exported).

    Without a ``dresser`` this is the legacy course with its island, decor and
    specials. With a ``kit.dress.Dresser`` the lane, rails, obstacles, tee, cup and
    flag are built by the very same code, while island, trims and specials come
    from the dresser. Returns a dict for ``build_course``.
    """
    course_id=f"{course['world']}-{number:02d}"
    scene=bpy.data.scenes.new(f"{number:02d} - {course['name']}")
    scene["license"]="MIT"; scene["aurago_course_id"]=course_id; scene["authoring"]="AuraGo original Blender asset"
    model=link_collection(scene,"Course Model"); studio=link_collection(scene,"Review Lights and Camera")
    palette=theme(course["world"]); mats={key:material(key+"-"+course["world"],col,roughness=.72 if key not in ("water","light") else .28,metallic=.15 if key=="water" else 0) for key,col in palette.items()}
    mats["water"]=material("water-"+course["world"],palette["water"],roughness=.24,metallic=.08)
    if dresser is None:
        add_decor(scene,model,course,mats,number)
        path,cup=make_surface(scene,model,course,mats)
    else:
        # The dressing builds modifier-finished meshes and bakes, which both need
        # this scene to be the active one.
        bpy.context.window.scene=scene
        path,cup=make_surface(scene,model,course,dresser.lane_mats(mats))
        dresser.dress(model,path,cup)
    obstacles=add_obstacle(scene,model,course.get("obstacle"),path,mats)
    if dresser is None:
        add_special(scene,model,course,path,mats)
    else:
        dresser.specials(scene,model,course,path,mats)
    tee_path=path[0]; tee=(tee_path[0],tee_path[1],lane_center_height(course,path,0)); add_templates(scene,model,mats,tee)
    # A thin inlaid tee mark and a genuine open cup: the annulus has no center
    # face, and the dark cup bottom sits below the green without a collider.
    ring(scene,model,"tee-ring",tee,.10,.18,tee[2]+.012,mats["light"],12)
    ring(scene,model,"cup-rim",cup,CUP_RADIUS,CUP_RADIUS+.075,cup[2]-.005,mats["stone"],20)
    cup_dark=material("cup-interior-"+course["world"],(.035,.045,.055,1),roughness=.22,metallic=.2)
    ground = island_top_height(cup[0], cup[1]) if dresser is None else dresser.top_height(cup[0], cup[1])
    floor_z = min(cup[2] - .02, max(cup[2] - .30, ground + .022))
    cylinder(scene,model,"cup-bottom",(cup[0],cup[1],floor_z+.014),CUP_RADIUS-.035,.025,cup_dark,16)
    cup_wall_mat=material("cup-lining-"+course["world"],(.13,.15,.17,1),roughness=.36,metallic=.1)
    cup_lining(scene,model,cup,cup_wall_mat,floor_z)
    # A slim flag stands beside, rather than inside, the opening.
    fx=cup[0]+.43; fy=cup[1]
    pole_mat=material("flag-pole",(.83,.84,.79,1),metallic=.45,roughness=.28)
    cylinder(scene,model,"flag-pole-base",(fx,fy,cup[2]+.43),.025,.86,pole_mat,7)
    flagmat=material("flag-cloth-"+course["world"],palette["accent"],roughness=.82)
    flag_verts=[(fx,fy,cup[2]+.82),(fx,fy,cup[2]+.55),(fx+.39,fy,cup[2]+.67)]
    flag=mesh_object(scene,model,"flag",flag_verts,[(0,1,2)],flagmat,"marker")
    digit_mat=material("hole-number-"+course["world"],(1.0,.87,.46,1),roughness=.42,emission=1.2)
    hole_number(scene,model,cup,number,digit_mat)
    if dresser is not None:
        dresser.restyle(model)
    add_studio(scene,studio,palette)
    return {"course_id":course_id,"scene":scene,"model":model,"studio":studio,"palette":palette,"path":path,"cup":cup,"tee":tee,"obstacles":obstacles}


def legacy_bounds(number, course):
    """Bounds of the legacy course (its island, water plane and decor), without keeping the scene.

    Dressed courses keep these bounds bit-identical: the physics uses them for the
    out-of-bounds test, so a smaller diorama must not change it.
    """
    keep=bpy.context.window.scene
    built=build_scene(number,course)
    bpy.context.window.scene=built["scene"]; bpy.context.view_layer.update()
    bounds=export.scene_bounds(list(built["model"].objects),built["obstacles"])
    bpy.context.window.scene=keep
    for coll in (built["model"],built["studio"]):
        for obj in list(coll.objects):
            data=obj.data; bpy.data.objects.remove(obj,do_unlink=True)
            if data is not None and data.users==0:
                (bpy.data.meshes if isinstance(data,bpy.types.Mesh) else bpy.data.cameras if isinstance(data,bpy.types.Camera) else bpy.data.lights).remove(data)
        bpy.data.collections.remove(coll)
    bpy.data.scenes.remove(built["scene"])
    return bounds


def build_course(number, course, dresser=None):
    bounds=legacy_bounds(number,course) if dresser is not None else None
    built=build_scene(number,course,dresser)
    course_id,scene,model,palette,path,cup,tee,obstacles=(built[k] for k in ("course_id","scene","model","palette","path","cup","tee","obstacles"))
    if dresser is not None:
        dresser.bake(model)
        export.join_scenery(model)
    bpy.context.window.scene=scene
    bpy.ops.object.select_all(action="DESELECT")
    for obj in model.objects: obj.select_set(True)
    bpy.context.view_layer.objects.active=next((obj for obj in model.objects if obj.type=="MESH"),None)
    os.makedirs(RUNTIME,exist_ok=True)
    glb=os.path.join(RUNTIME,f"course-{number:02d}.glb")
    export.export_glb(glb,model.objects)
    # Exported GLB is the canonical geometry for both rendering and fixed
    # collision. This also follows Blender's exact triangle split and float32
    # positions instead of approximating exporter output from source polygons.
    colliders=export.glb_fixed_colliders(glb,course["layout"])
    data=open(glb,"rb").read(); tee_rt=runtime(tee); cup_rt=runtime(cup)
    water=[{"min":[-6.15,-1.55,-6.15],"max":[6.15,-1.15,6.15]}]
    entry={"id":course_id,"number":number,"world":course["world"],"par":course["par"],"model":os.path.basename(glb),"tee":q([tee_rt[0],tee_rt[1]+BALL_RADIUS,tee_rt[2]]),"cup":q(cup_rt),"cupRadius":CUP_RADIUS,"bounds":export.scene_bounds(list(model.objects),obstacles) if bounds is None else bounds,"colliders":colliders,"obstacles":obstacles,"water":water,"sha256":hashlib.sha256(data).hexdigest(),"bytes":len(data),"triangles":export.glb_triangles(glb)}
    if dresser is not None:
        path_rt=[q(runtime((p[0],p[1],lane_center_height(course,path,i)))) for i,p in enumerate(path) if i%3==0 or i==len(path)-1]
        entry.update({"look":dresser.look,"path":[[round(v,3) for v in p] for p in path_rt],"props":dresser.runtime_props()})
    print(f"{course_id}: {len(data)} bytes, {entry['triangles']} triangles, {len(colliders)} colliders, {len(obstacles)} dynamic")
    return entry
