"""Small deterministic builders; coordinates are game X/right, Y/up, Z/back."""
import math
import bpy
from mathutils import Vector

COLLIDERS = []
MATERIALS = {}


def bv(p):
    return (p[0], -p[2], p[1])


def gv(p):
    return (round(p[0], 6), round(p[2], 6), round(-p[1], 6))


def material(name, rgb, metal=0, rough=.3, emission=0, transmission=0):
    mat = bpy.data.materials.new(name)
    mat.use_nodes = True
    shader = mat.node_tree.nodes.get('Principled BSDF')
    shader.inputs['Base Color'].default_value = (*rgb, 1)
    shader.inputs['Metallic'].default_value = metal
    shader.inputs['Roughness'].default_value = rough
    shader.inputs['Coat Weight'].default_value = .35 if metal < .5 else .15
    shader.inputs['Coat Roughness'].default_value = .18
    shader.inputs['Emission Color'].default_value = (*rgb, 1)
    shader.inputs['Emission Strength'].default_value = emission
    shader.inputs['Transmission Weight'].default_value = transmission
    MATERIALS[name] = mat
    return mat


def mesh(name, vertices, faces, mat, collide=None, dynamic=False):
    data = bpy.data.meshes.new(name)
    data.from_pydata([bv(p) for p in vertices], [], faces)
    data.update()
    obj = bpy.data.objects.new(name, data)
    bpy.context.collection.objects.link(obj)
    obj.data.materials.append(MATERIALS[mat])
    if collide:
        obj['pinball_surface'] = collide
        COLLIDERS.append((obj, collide))
    if dynamic:
        obj['pinball_actor'] = name
    return obj


def finish(obj, mat, collide=None, bevel=0, dynamic=False):
    obj.data.materials.append(MATERIALS[mat])
    if bevel:
        mod = obj.modifiers.new('Machined edges', 'BEVEL')
        mod.width, mod.segments = bevel, 3
        bpy.context.view_layer.objects.active = obj
        bpy.ops.object.modifier_apply(modifier=mod.name)
    for face in obj.data.polygons:
        face.use_smooth = True
    if collide:
        obj['pinball_surface'] = collide
        COLLIDERS.append((obj, collide))
    if dynamic:
        obj['pinball_actor'] = obj.name
    return obj


def box(name, at, size, mat, collide=None, bevel=.001, dynamic=False):
    bpy.ops.mesh.primitive_cube_add(size=1, location=bv(at))
    obj = bpy.context.object
    obj.name = name
    obj.scale = (size[0], size[2], size[1])
    bpy.ops.object.transform_apply(location=False, rotation=False, scale=True)
    return finish(obj, mat, collide, bevel, dynamic)


def cylinder(name, at, radius, height, mat, collide=None, vertices=32):
    bpy.ops.mesh.primitive_cylinder_add(vertices=vertices, radius=radius, depth=height, location=bv(at))
    obj = bpy.context.object
    obj.name = name
    return finish(obj, mat, collide, min(.001, height / 5))


def sphere(name, at, scale, mat, segments=24):
    bpy.ops.mesh.primitive_uv_sphere_add(segments=segments, ring_count=12, radius=1, location=bv(at))
    obj = bpy.context.object
    obj.name = name
    obj.scale = (scale[0], scale[2], scale[1])
    bpy.ops.object.transform_apply(location=False, rotation=False, scale=True)
    return finish(obj, mat)


def rod(name, start, end, radius, mat):
    a, b = Vector(bv(start)), Vector(bv(end))
    bpy.ops.mesh.primitive_cylinder_add(vertices=10, radius=radius, depth=(b-a).length, location=(a+b)/2)
    obj = bpy.context.object
    obj.name = name
    obj.rotation_mode = 'QUATERNION'
    obj.rotation_quaternion = (b-a).to_track_quat('Z', 'Y')
    return finish(obj, mat)


def torus(name, at, radius, tube, mat, upright=False):
    bpy.ops.mesh.primitive_torus_add(major_segments=48, minor_segments=8, location=bv(at), major_radius=radius, minor_radius=tube)
    obj = bpy.context.object
    obj.name = name
    if upright:
        obj.rotation_euler.x = math.pi / 2
    return finish(obj, mat)


def smooth_path(points, subdivisions=10):
    out = []
    for i in range(len(points)-1):
        p0, p1 = points[max(0, i-1)], points[i]
        p2, p3 = points[i+1], points[min(len(points)-1, i+2)]
        for j in range(subdivisions):
            t = j / subdivisions
            out.append(tuple(.5*((2*p1[k])+(-p0[k]+p2[k])*t+(2*p0[k]-5*p1[k]+4*p2[k]-p3[k])*t*t+(-p0[k]+3*p1[k]-3*p2[k]+p3[k])*t*t*t) for k in range(3)))
    out.append(tuple(points[-1]))
    return out


def rail(name, points, height=.028, width=.004, mat='Chrome'):
    vertices, faces = [], []
    for i, p in enumerate(points):
        prev, nxt = points[max(0, i-1)], points[min(len(points)-1, i+1)]
        dx, dz = nxt[0]-prev[0], nxt[2]-prev[2]
        norm = max(.00001, math.hypot(dx, dz))
        nx, nz = dz/norm*width/2, -dx/norm*width/2
        vertices += [(p[0]-nx,p[1],p[2]-nz),(p[0]+nx,p[1],p[2]+nz),(p[0]+nx,p[1]+height,p[2]+nz),(p[0]-nx,p[1]+height,p[2]-nz)]
        if i:
            a, b = (i-1)*4, i*4
            faces += [(a+k,a+(k+1)%4,b+(k+1)%4,b+k) for k in range(4)]
    faces += [(3,2,1,0),(len(vertices)-4,len(vertices)-3,len(vertices)-2,len(vertices)-1)]
    obj = mesh(name, vertices, faces, mat, 'metal')
    for face in obj.data.polygons: face.use_smooth = True
    return obj


def ramp(name, controls, width=.055, glass=False, tube=False):
    points = smooth_path(controls, 12)
    vertices, faces, edges = [], [], [[], []]
    for i, p in enumerate(points):
        prev, nxt = points[max(0,i-1)], points[min(len(points)-1,i+1)]
        dx, dz = nxt[0]-prev[0], nxt[2]-prev[2]
        norm = max(.00001, math.hypot(dx,dz))
        nx, nz = dz/norm*width/2, -dx/norm*width/2
        left, right = (p[0]-nx,p[1],p[2]-nz),(p[0]+nx,p[1],p[2]+nz)
        vertices += [left,right]
        edges[0].append(left); edges[1].append(right)
        if i:
            a = 2*(i-1)
            faces.append((a,a+2,a+3,a+1))
    floor = mesh(name+'-floor', vertices, faces, 'Glass' if glass else 'Ramp', 'metal')
    for face in floor.data.polygons: face.use_smooth = True
    if tube:
        roof, roof_faces = [], []
        for i,p in enumerate(points):
            prev,nxt=points[max(0,i-1)],points[min(len(points)-1,i+1)]
            dx,dz=nxt[0]-prev[0],nxt[2]-prev[2]; norm=max(.00001,math.hypot(dx,dz))
            for j in range(13):
                angle=j*math.pi/12; offset=math.cos(angle)*width/2
                roof.append((p[0]+dz/norm*offset,p[1]+.024+math.sin(angle)*width/2,p[2]-dx/norm*offset))
                if i and j: a=(i-1)*13+j-1;b=i*13+j-1;roof_faces.append((a,b,b+1,a+1))
        roof_mesh=mesh(name+'-tube',roof,roof_faces,'TubeGlass','metal')
        for face in roof_mesh.data.polygons: face.use_smooth=True
    for side, edge in enumerate(edges):
        rail(name+'-wall-'+str(side), edge, .024, .0025, 'Glass' if glass else 'Chrome')
        for i in range(0,len(edge),4):
            a,b=edge[i],edge[min(i+4,len(edge)-1)]
            rod(name+'-rim', (a[0],a[1]+.027,a[2]),(b[0],b[1]+.027,b[2]),.0016,'Chrome')
    for i in range(10,len(points)-5,18):
        p = points[i]
        if p[1] > .04:
            cylinder(name+'-support', (p[0],p[1]/2,p[2]), .004, p[1], 'Chrome', vertices=12)
            cylinder(name+'-foot', (p[0],.004,p[2]), .01, .008, 'Gold', vertices=16)
    return points


def flipper(name, at, side=1, length=.092):
    vertices, faces = [], []
    outline = []
    for i in range(12):
        angle = math.pi/2 + math.pi*i/11
        outline.append((side*.013*math.cos(angle),.013*math.sin(angle)))
    for i in range(12):
        angle = -math.pi/2 + math.pi*i/11
        outline.append((side*(length-.010+.010*math.cos(angle)),.010*math.sin(angle)))
    for y in [-.009,.009]:
        vertices += [(x,y,z) for x,z in outline]
    n = len(outline)
    faces = [tuple(reversed(range(n))),tuple(range(n,2*n))] + [(i,(i+1)%n,(i+1)%n+n,i+n) for i in range(n)]
    obj = mesh(name, vertices, faces, 'Chrome', dynamic=True)
    obj.location = bv(at)
    obj['pinball_side'] = side
    for fraction, mat, zoff in [(1.015,'Rubber',-.002),(.86,'Gold',.003),(.78,'Obsidian',.005),(.72,'Accent',.006)]:
        child = mesh(name+'-inlay-'+mat, [(x*fraction,y*.65+zoff,z*(.22 if mat=='Accent' else fraction)) for x,y,z in vertices], faces, mat)
        child.parent = obj
    return obj, vertices


def collision_data():
    groups = {}
    welded = {}
    bpy.context.view_layer.update()
    for obj, surface in COLLIDERS:
        group = groups.setdefault(surface, {'surface':surface,'vertices':[],'indices':[]})
        data = obj.data
        data.calc_loop_triangles()
        index=welded.setdefault(surface,{})
        remap=[]
        for v in data.vertices:
            point=gv(obj.matrix_world @ v.co)
            if point not in index:
                index[point]=len(group['vertices'])//3;group['vertices'].extend(point)
            remap.append(index[point])
        for face in data.loop_triangles:
            triangle=[remap[i] for i in face.vertices]
            if len(set(triangle))==3: group['indices'].extend(triangle)
    return list(groups.values())


def merge_static():
    groups = {}
    for obj in list(bpy.context.scene.objects):
        if obj.type == 'MESH' and not obj.parent and not obj.get('pinball_actor'):
            groups.setdefault(obj.data.materials[0].name,[]).append(obj)
    for mat, objects in groups.items():
        bpy.ops.object.select_all(action='DESELECT')
        for obj in objects: obj.select_set(True)
        bpy.context.view_layer.objects.active = objects[0]
        bpy.ops.object.join()
        bpy.context.object.name = 'static-'+mat
        bpy.context.object['pinball_surface'] = ''
