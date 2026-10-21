"""Shared Blender helpers of the track kit (MIT License, see ../LICENSE).

Runtime/Blender coordinate swizzles, the material and mesh helpers, the hole number and the ball and
putter templates that the track kit builds on.
"""

import bpy
import math

from .track import BALL_RADIUS


def runtime(v):
    return [float(v[0]), float(v[2]), float(-v[1])]


def blender(v):
    return (float(v[0]), float(-v[2]), float(v[1]))


def q(v):
    return [round(float(x), 5) for x in v]


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
