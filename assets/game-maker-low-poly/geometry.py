"""Small Blender mesh builder. Author coordinates are metres, Y up, Z forward."""
import math
from functools import lru_cache
import bpy
import bmesh
from mathutils import Matrix, Vector
from catalog import PALETTE


def xyz(p):
    return Vector((p[0], -p[2], p[1]))


def linear(value):
    return value / 12.92 if value <= .04045 else ((value + .055) / 1.055) ** 2.4


def color(name, shade=1):
    value = PALETTE.get(name, name).lstrip('#')
    return tuple(linear(int(value[i:i+2], 16) / 255) * shade for i in (0, 2, 4)) + (1,)


def materials():
    result = []
    for name, rough, metal, emit in [('surface', .8, 0, 0), ('metal', .4, .5, 0),
                                     ('glass', .22, .25, 0), ('light', .5, 0, 1.5)]:
        mat = bpy.data.materials.get('poly.' + name) or bpy.data.materials.new('poly.' + name)
        mat.use_nodes = True
        mat.use_backface_culling = False
        nodes = mat.node_tree.nodes
        bsdf = nodes.get('Principled BSDF')
        bsdf.inputs['Roughness'].default_value = rough
        bsdf.inputs['Metallic'].default_value = metal
        vertex = nodes.get('Palette') or nodes.new('ShaderNodeVertexColor')
        vertex.name, vertex.layer_name = 'Palette', 'Color'
        mat.node_tree.links.new(vertex.outputs['Color'], bsdf.inputs['Base Color'])
        if emit:
            mat.node_tree.links.new(vertex.outputs['Color'], bsdf.inputs['Emission Color'])
            bsdf.inputs['Emission Strength'].default_value = emit
        result.append(mat)
    return result


@lru_cache(maxsize=512)
def box_mesh(size, bevel):
    bm = bmesh.new()
    bmesh.ops.create_cube(bm, size=1)
    for v in bm.verts:
        v.co.x *= size[0]
        v.co.y *= size[1]
        v.co.z *= size[2]
    if bevel:
        bmesh.ops.bevel(bm, geom=list(bm.edges), offset=bevel, segments=1,
                        affect='EDGES', clamp_overlap=True)
    bm.verts.ensure_lookup_table()
    bm.verts.index_update()
    vertices = [tuple(v.co) for v in bm.verts]
    faces = [tuple(v.index for v in f.verts) for f in bm.faces]
    bm.free()
    return vertices, faces


class Mesh:
    def __init__(self, asset_id):
        self.id = asset_id
        self.parts = {}
        self.part = 'body'
        self.bone = None
        self.pivots = {}
        self.sockets = {}
        self.moving = []
        self.colliders = []
        self.connections = []

    def add(self, vertices, faces, paint, center=(0, 0, 0), rotation=None, material=0, weights=None):
        # Double-sided materials cover leaves/ears; duplicated reverse polygons
        # are invalid glTF topology and also waste triangles.
        unique = {}
        for face in faces:unique.setdefault(tuple(sorted(face)),face)
        faces=list(unique.values())
        part = self.parts.setdefault(self.part, dict(vertices=[], faces=[], colors=[], materials=[], weights=[]))
        start = len(part['vertices'])
        center = Vector(center)
        part['vertices'].extend(tuple((rotation @ Vector(v) if rotation else Vector(v)) + center) for v in vertices)
        part['faces'].extend(tuple(start + i for i in f) for f in faces)
        part['colors'].extend([color(paint)] * len(faces))
        part['materials'].extend([material] * len(faces))
        part['weights'].extend(weights or ([{self.bone: 1} if self.bone else {}] * len(vertices)))

    def box(self, at, size, paint='ivory', bevel=.025, rotation=None, material=0):
        # Repeated facade details use sharp low-poly edges, not unseen bevels.
        if self.id in ('architecture-apartment','architecture-office','architecture-townhouse'):bevel=0
        v, f = box_mesh(tuple(size), min(bevel, min(size) * .22))
        self.add(v, f, paint, at, rotation, material)

    def ellipsoid(self, at, radii, paint, segments=12, rings=8, material=0):
        vertices = [(0, -radii[1], 0)]
        for j in range(1, rings):
            phi = math.pi * j / rings
            for i in range(segments):
                theta = math.tau * i / segments
                vertices.append((radii[0]*math.sin(phi)*math.cos(theta), -radii[1]*math.cos(phi), radii[2]*math.sin(phi)*math.sin(theta)))
        top = len(vertices)
        vertices.append((0, radii[1], 0))
        faces = [(0, 1 + (i+1) % segments, 1+i) for i in range(segments)]
        for j in range(rings-2):
            a = 1 + j*segments
            b = a+segments
            for i in range(segments):
                ni = (i+1) % segments
                faces.append((a+i, a+ni, b+ni, b+i))
        faces.extend((top, top-segments+i, top-segments+(i+1) % segments) for i in range(segments))
        self.add(vertices, faces, paint, at, material=material)

    def cylinder(self, a, b, radius, paint, radius_end=None, sides=12, material=0):
        a, b = Vector(a), Vector(b)
        top = radius if radius_end is None else radius_end
        rot = (b-a).to_track_quat('Y', 'Z').to_matrix()
        length = (b-a).length
        vertices = [(r*math.cos(i*math.tau/sides), y, r*math.sin(i*math.tau/sides))
                    for y, r in ((-length/2, radius), (length/2, top)) for i in range(sides)]
        faces = [(i, (i+1) % sides, (i+1) % sides+sides, i+sides) for i in range(sides)]
        faces += [tuple(reversed(range(sides))), tuple(range(sides, 2*sides))]
        self.add(vertices, faces, paint, (a+b)/2, rot, material)

    def beam(self, a, b, width, paint='steel', depth=None, bevel=.015):
        a, b = Vector(a), Vector(b)
        self.box((a+b)/2, (width, (b-a).length, depth or width), paint, bevel,
                 (b-a).to_track_quat('Y', 'Z').to_matrix())

    def ring(self, center, radius, thickness, paint, axis='Y', segments=20, material=0):
        verts = []
        for i in range(segments):
            a = math.tau*i/segments
            for j in range(6):
                b = math.tau*j/6
                verts.append(((radius+thickness*math.cos(b))*math.cos(a), thickness*math.sin(b),
                              (radius+thickness*math.cos(b))*math.sin(a)))
        faces = [(i*6+j, ((i+1) % segments)*6+j, ((i+1) % segments)*6+(j+1) % 6, i*6+(j+1) % 6)
                 for i in range(segments) for j in range(6)]
        rotation = Matrix.Rotation(math.pi/2, 3, 'Z' if axis == 'X' else 'X') if axis != 'Y' else None
        self.add(verts, faces, paint, center, rotation, material)

    def prism(self, profile, width, paint, material=0):
        """Extrude a Y/Z profile along X, with outward winding."""
        n = len(profile)
        verts = [(x, y, z) for x in (-width/2, width/2) for y, z in profile]
        faces = [tuple(reversed(range(n))), tuple(range(n, n*2))]
        faces += [(i, (i+1) % n, (i+1) % n+n, i+n) for i in range(n)]
        self.add(verts, faces, paint, material=material)

    def moving_part(self, name, pivot, kind, axis=(0, 1, 0), parent='body'):
        self.part = name
        self.pivots[name] = (tuple(pivot), parent)
        self.moving.append(dict(node=(self.id+'__'+name).replace('.','p'), kind=kind, axis=list(axis)))

    def socket(self, name, at, parent='body'):
        self.sockets[name] = (tuple(at), parent)

    def objects(self, collection, skeleton=None):
        mats = materials()
        root = bpy.data.objects.new(self.id, None)
        collection.objects.link(root)
        objects = {'root': root}
        for name, part in self.parts.items():
            pivot, parent = self.pivots.get(name, ((0, 0, 0), 'root'))
            mesh = bpy.data.meshes.new(self.id+'__'+name)
            mesh.from_pydata([xyz(Vector(v)-Vector(pivot)) for v in part['vertices']], [], part['faces'])
            mesh.update()
            # Recalculate outside normals after procedural topology construction.
            bm = bmesh.new(); bm.from_mesh(mesh)
            bmesh.ops.recalc_face_normals(bm, faces=list(bm.faces))
            bm.to_mesh(mesh); bm.free()
            for mat in mats:
                mesh.materials.append(mat)
            colors = mesh.color_attributes.new(name='Color', type='FLOAT_COLOR', domain='CORNER')
            for i, poly in enumerate(mesh.polygons):
                poly.material_index = part['materials'][i]
                for loop in poly.loop_indices:
                    colors.data[loop].color = part['colors'][i]
            obj = bpy.data.objects.new((self.id+'__'+name).replace('.','p'), mesh)
            collection.objects.link(obj)
            obj.parent, obj.location = root, xyz(pivot)
            objects[name] = obj
            if skeleton:
                obj.parent = skeleton
                groups = {bone.name: obj.vertex_groups.new(name=bone.name) for bone in skeleton.data.bones}
                for i, weights in enumerate(part['weights']):
                    for bone, weight in weights.items():
                        groups[bone].add([i], weight, 'REPLACE')
                modifier = obj.modifiers.new('Rig', 'ARMATURE')
                modifier.object = skeleton
        for name, (pivot, parent) in self.pivots.items():
            if name in objects and parent in objects and parent != name:
                obj = objects[name]
                obj.parent = objects[parent]
                parent_pivot = self.pivots.get(parent, ((0, 0, 0), 'root'))[0]
                obj.location = xyz(Vector(pivot)-Vector(parent_pivot))
        for name, (point, parent) in self.sockets.items():
            obj = bpy.data.objects.new((self.id+'__socket_'+name).replace('.','p'), None)
            collection.objects.link(obj)
            if skeleton and parent in skeleton.data.bones:
                obj.parent = skeleton
                obj.parent_type, obj.parent_bone = 'BONE', parent
                bone = skeleton.data.bones[parent]
                tail = bone.matrix_local @ Matrix.Translation((0, bone.length, 0))
                obj.matrix_basis = tail.inverted() @ Matrix.Translation(xyz(point))
            else:
                obj.parent = objects.get(parent, root)
                parent_pivot = self.pivots.get(parent, ((0, 0, 0), 'root'))[0]
                obj.location = xyz(Vector(point)-Vector(parent_pivot))
            objects['socket_'+name] = obj
        if skeleton:
            skeleton.parent = root
            objects['rig'] = skeleton
        return root, objects
