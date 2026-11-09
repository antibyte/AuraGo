"""AuraGo System World kit v3: shared Blender authoring core (Blender 5.2 LTS).

Imported by build_world3.py and the asset family modules. Geometry is collected as
plain vertex/face lists per named part and then turned into meshes with welded seams,
angle-based sharp edges, weighted normals and a deterministic ray-cast ambient
occlusion bake. The bake lives in the active colour attribute, is exported as COLOR_0
and multiplied with the albedo by the renderer. Nothing here uses image textures.
"""
from functools import lru_cache
from math import sin, cos, pi, radians, sqrt, atan2
from pathlib import Path
import hashlib
import json
import random
import struct

import bpy
import bmesh
from mathutils import Vector, Matrix
from mathutils.bvhtree import BVHTree

ROOT = Path(__file__).resolve().parents[2]
RAW = ROOT / 'disposable/system-world/v3-raw'
SOURCE = ROOT / 'assets/system-world/production'
REPORT = ROOT / 'reports/system-world-assets/v3'
COLLECTION = 'AURAGO_WORLD_3'
SHARP_ANGLE = 35

# Linear base colour, metallic, roughness, emission strength, alpha. Names shared with the
# v1/v2 kits keep their values so the renderer's surface kinds apply unchanged.
PALETTE = {
    'graphite': ((.032, .048, .062, 1), .72, .28, 0, 1),
    'titanium': ((.28, .36, .4, 1), .78, .26, 0, 1),
    'ceramic': ((.57, .63, .59, 1), .25, .36, 0, 1),
    'bronze': ((.39, .20, .073, 1), .82, .25, 0, 1),
    'glass': ((.025, .10, .14, 1), .65, .15, 0, 1),
    'warm': ((.65, .27, .06, 1), .1, .32, 1.3, 1),
    'ivory': ((.8, .62, .32, 1), .1, .3, .8, 1),
    'cyan': ((.09, .63, .8, 1), .15, .32, 2.1, 1),
    'jade': ((.12, .48, .29, 1), .22, .38, 1.3, 1),
    'leaf': ((.052, .135, .086, 1), .05, .66, 0, 1),
    'road': ((.035, .045, .053, 1), .12, .55, 0, 1),
    'stone': ((.16, .20, .21, 1), .12, .67, 0, 1),
    # v3 street materials.
    'kerb': ((.30, .33, .34, 1), .05, .6, 0, 1),
    'setts': ((.085, .10, .11, 1), .08, .7, 0, 1),
    'iron': ((.022, .026, .03, 1), .85, .48, 0, 1),
    'marking': ((.62, .63, .58, 1), .02, .62, 0, 1),
    'tactile': ((.50, .47, .36, 1), .04, .64, 0, 1),
    # Window panes: the renderer replaces this material with the interior shader. UV0 holds
    # room coordinates (one unit per bay along the facade and per floor upwards).
    'window': ((.018, .032, .045, 1), .0, .08, 0, 1),
}


def materials():
    result = {}
    for key, (color, metal, rough, emission, alpha) in PALETTE.items():
        name = 'city.' + key
        mat = bpy.data.materials.get(name) or bpy.data.materials.new(name)
        mat.use_nodes = True
        mat.diffuse_color = color
        bsdf = mat.node_tree.nodes.get('Principled BSDF')
        bsdf.inputs['Base Color'].default_value = color
        bsdf.inputs['Metallic'].default_value = metal
        bsdf.inputs['Roughness'].default_value = rough
        bsdf.inputs['Emission Color'].default_value = color
        bsdf.inputs['Emission Strength'].default_value = emission
        bsdf.inputs['Alpha'].default_value = alpha
        result[key] = mat
    return result


@lru_cache(maxsize=1024)
def box_template(size, bevel, segments):
    bm = bmesh.new()
    bmesh.ops.create_cube(bm, size=1)
    for v in bm.verts:
        v.co.x *= size[0]
        v.co.y *= size[1]
        v.co.z *= size[2]
    if bevel:
        bmesh.ops.bevel(bm, geom=list(bm.edges), offset=bevel, segments=segments,
                        profile=.5, affect='EDGES', clamp_overlap=True)
    bm.verts.ensure_lookup_table()
    bm.verts.index_update()
    result = ([tuple(v.co) for v in bm.verts],
              [tuple(v.index for v in f.verts) for f in bm.faces])
    bm.free()
    return result


def circle(radius, n, z=0, phase=0):
    return [(radius*cos(phase+2*pi*i/n), radius*sin(phase+2*pi*i/n), z) for i in range(n)]


class Kit:
    """Collects the geometry of one asset at one level of detail."""

    def __init__(self, lod, seed=43021):
        self.lod = lod
        self.parts = {}
        self.part = 'structure'
        self.rng = random.Random(seed)

    # -- raw access ---------------------------------------------------------------------
    def add(self, vertices, faces, mat, at=(0, 0, 0), rot=None, tint=1.0, up=False, uvs=None):
        """Add faces. `mat` and `tint` are one value or one value per face.

        With up=True every face is wound so that its normal points to +Z (ground surfaces).
        `uvs` optionally gives one (u, v) per vertex; faces without it export (0, 0).
        """
        part = self.parts.setdefault(self.part, {'verts': [], 'faces': [], 'mats': [], 'tints': [], 'uvs': []})
        start = len(part['verts'])
        offset = Vector(at)
        placed = [tuple((rot @ Vector(v) if rot else Vector(v)) + offset) for v in vertices]
        faces = [tuple(f) for f in faces]
        if up:
            def facing(face):
                z = sum((placed[a][0] - placed[b][0]) * (placed[a][1] + placed[b][1])
                        for a, b in zip(face, face[1:] + face[:1]))
                return face if z >= 0 else tuple(reversed(face))
            faces = [facing(f) for f in faces]
        part['verts'].extend(placed)
        part['uvs'].extend([tuple(uvs[i] for i in f) for f in faces] if uvs else [None] * len(faces))
        part['faces'].extend(tuple(start + i for i in f) for f in faces)
        part['mats'].extend(mat if isinstance(mat, (list, tuple)) else [mat] * len(faces))
        part['tints'].extend(tint if isinstance(tint, (list, tuple)) else [tint] * len(faces))

    def quad(self, a, b, c, d, mat, tint=1.0, up=False, uvs=None):
        self.add([a, b, c, d], [(0, 1, 2, 3)], mat, tint=tint, up=up, uvs=uvs)

    # -- solids -------------------------------------------------------------------------
    def box(self, center, size, mat='graphite', bevel=.03, segments=2, angle=0, rot=None, tint=1.0):
        if self.lod == 0:
            bevel, segments = min(bevel, min(size) * .3), segments
        elif self.lod == 1 and bevel >= .05:
            bevel, segments = min(bevel, min(size) * .3), 1
        else:
            bevel, segments = 0, 1
        vertices, faces = box_template(tuple(round(s, 6) for s in size), round(bevel, 6), segments)
        if angle:
            rot = Matrix.Rotation(angle, 3, 'Z')
        self.add(vertices, faces, mat, center, rot, tint)

    def cyl(self, center, radius, height, mat='graphite', top=None, n=None, rot=None, caps=(True, True), tint=1.0):
        n = n or (32, 18, 10)[self.lod]
        top = radius if top is None else top
        vertices = circle(radius, n, -height/2) + circle(top, n, height/2)
        self.add(vertices, [(i, (i+1) % n, (i+1) % n+n, i+n) for i in range(n)], mat, center, rot, tint)
        if caps[0]:
            self.add(vertices[:n], [tuple(reversed(range(n)))], mat, center, rot, tint)
        if caps[1] and top > 1e-6:
            self.add(vertices[n:], [tuple(range(n))], mat, center, rot, tint)

    def lathe(self, center, profile, mat='graphite', n=None, rot=None, tint=1.0, up=False):
        """Revolve [(radius, z), ...] around Z. Zero radii close the shape with a fan."""
        n = n or (32, 18, 10)[self.lod]
        vertices, rings = [], []
        for radius, z in profile:
            if radius <= 1e-6:
                rings.append([len(vertices)])
                vertices.append((0, 0, z))
            else:
                rings.append(list(range(len(vertices), len(vertices) + n)))
                vertices.extend(circle(radius, n, z))
        faces = []
        for lower, upper in zip(rings, rings[1:]):
            for i in range(n):
                j = (i + 1) % n
                if len(lower) == 1 and len(upper) == 1:
                    continue
                if len(lower) == 1:
                    faces.append((lower[0], upper[j], upper[i]))
                elif len(upper) == 1:
                    faces.append((lower[i], lower[j], upper[0]))
                else:
                    faces.append((lower[i], lower[j], upper[j], upper[i]))
        self.add(vertices, faces, mat, center, rot, tint, up)

    def beam(self, a, b, width, mat='titanium', depth=None, bevel=.02, tint=1.0):
        a, b = Vector(a), Vector(b)
        vector = b - a
        self.box((a + b) / 2, (width, depth or width, vector.length), mat, bevel, 1,
                 rot=vector.to_track_quat('Z', 'Y').to_matrix(), tint=tint)

    def pipe(self, a, b, radius=.08, mat='titanium', n=None, tint=1.0):
        a, b = Vector(a), Vector(b)
        self.cyl((a + b) / 2, radius, (b - a).length, mat, n=n or (12, 8, 5)[self.lod],
                 rot=(b - a).to_track_quat('Z', 'Y').to_matrix(), tint=tint)

    def ring(self, center, radius, tube=.1, mat='bronze', rot=None, arc=2*pi, tint=1.0):
        n, sides = (48, 28, 14)[self.lod], (8, 6, 4)[self.lod]
        closed = abs(arc - 2*pi) < 1e-6
        count = n if closed else n + 1
        vertices = [((radius+tube*cos(j*2*pi/sides))*cos(i*arc/n),
                     (radius+tube*cos(j*2*pi/sides))*sin(i*arc/n), tube*sin(j*2*pi/sides))
                    for i in range(count) for j in range(sides)]
        faces = [(i*sides+j, ((i+1) % count)*sides+j, ((i+1) % count)*sides+(j+1) % sides, i*sides+(j+1) % sides)
                 for i in range(n) for j in range(sides)]
        self.add(vertices, faces, mat, center, rot, tint)

    def prism(self, outline, z0, z1, mat, top=True, bottom=False, side=None, tint=1.0):
        """Extrude a counter-clockwise XY outline from z0 to z1 (convex, or star-shaped caps)."""
        n = len(outline)
        vertices = [(x, y, z0) for x, y in outline] + [(x, y, z1) for x, y in outline]
        self.add(vertices, [(i, (i+1) % n, (i+1) % n+n, i+n) for i in range(n)], side or mat, tint=tint)
        if top:
            self.add(vertices[n:], [tuple(range(n))], mat, tint=tint)
        if bottom:
            self.add(vertices[:n], [tuple(reversed(range(n)))], mat, tint=tint)

    # -- surfaces -----------------------------------------------------------------------
    def loft(self, sections, mats, tints=None, flip=False, up=False):
        """Join successive point rows with quads.

        `sections` are rows with equal point counts; `mats[k]` is the material of the band
        between point k and k+1 (None skips it), or a callable (row_index, k) -> material.
        `tints` works the same way. With flip=False the surface faces +Z when rows advance
        along +X and points along +Y.
        """
        vertices = [p for row in sections for p in row]
        count = len(sections[0])
        faces, face_mats, face_tints = [], [], []
        for r in range(len(sections) - 1):
            for k in range(count - 1):
                mat = mats(r, k) if callable(mats) else mats[k]
                if mat is None:
                    continue
                a, b, c, d = r*count+k, (r+1)*count+k, (r+1)*count+k+1, r*count+k+1
                faces.append((d, c, b, a) if flip else (a, b, c, d))
                face_mats.append(mat)
                tint = 1.0 if tints is None else tints(r, k) if callable(tints) else tints[k]
                face_tints.append(tint)
        self.add(vertices, faces, face_mats, tint=face_tints, up=up)

    def grid(self, xs, ys, z, mat, tint=1.0, flip=False):
        """Flat cells between the break lists; `mat`/`tint` may be callables (i, j)."""
        for i in range(len(xs) - 1):
            for j in range(len(ys) - 1):
                m = mat(i, j) if callable(mat) else mat
                if m is None:
                    continue
                t = tint(i, j) if callable(tint) else tint
                a, b, c, d = (xs[i], ys[j], z), (xs[i+1], ys[j], z), (xs[i+1], ys[j+1], z), (xs[i], ys[j+1], z)
                self.quad(*((d, c, b, a) if flip else (a, b, c, d)), m, t)

    def fan(self, hub, rim, mat, tint=1.0, flip=False, up=False, closed=False):
        """Triangle fan from `hub` over the point list `rim` (closed=True joins last to first)."""
        vertices = [hub] + list(rim)
        count = len(rim)
        pairs = [(i, i+1) for i in range(1, count)] + ([(count, 1)] if closed else [])
        faces = [(0, b, a) if flip else (0, a, b) for a, b in pairs]
        self.add(vertices, faces, mat, tint=tint, up=up)


# -- mesh creation ----------------------------------------------------------------------
def create_asset(asset_id, factory, lod, collection, mats):
    kit = Kit(lod)
    info = factory(kit) or {}
    root = bpy.data.objects.new(f'{asset_id}.lod{lod}', None)
    collection.objects.link(root)
    root['asset_id'], root['lod'], root['license'] = asset_id, lod, 'MIT'
    for part, data in kit.parts.items():
        mesh = bpy.data.meshes.new(asset_id + '.' + part)
        used = list(dict.fromkeys(data['mats']))
        for key in used:
            mesh.materials.append(mats[key])
        bm = bmesh.new()
        tint_layer = bm.faces.layers.float.new('tint')
        uv_layer = bm.loops.layers.uv.new('UVMap') if any(data['uvs']) else None
        verts = [bm.verts.new(v) for v in data['verts']]
        for face, key, tint, uvs in zip(data['faces'], data['mats'], data['tints'], data['uvs']):
            corners = [verts[i] for i in face]
            if len(set(corners)) < 3:
                continue
            try:
                f = bm.faces.new(corners)
            except ValueError:
                continue
            f.material_index = used.index(key)
            f[tint_layer] = tint
            f.smooth = True
            if uvs:
                for loop, uv in zip(f.loops, uvs):
                    loop[uv_layer].uv = uv
        bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)
        dead = [f for f in bm.faces if f.calc_area() < 1e-9]
        if dead:
            bmesh.ops.delete(bm, geom=dead, context='FACES')
        loose = [v for v in bm.verts if not v.link_faces]
        if loose:
            bmesh.ops.delete(bm, geom=loose, context='VERTS')
        bm.normal_update()
        bm.to_mesh(mesh)
        bm.free()
        mesh.set_sharp_from_angle(angle=radians(SHARP_ANGLE))
        mesh.update()
        obj = bpy.data.objects.new(part, mesh)
        obj.parent = root
        collection.objects.link(obj)
        pivot = (info.get('pivots') or {}).get(part)
        if pivot:
            mesh.transform(Matrix.Translation(-Vector(pivot)))
            obj.location = pivot
        obj['component'] = part
        weighted = obj.modifiers.new('weighted', 'WEIGHTED_NORMAL')
        weighted.keep_sharp = True
        weighted.weight = 50
    bpy.context.view_layer.update()
    bake_occlusion([o for o in root.children if o.type == 'MESH'], **(info.get('occlusion') or {}))
    return root, info


def hemisphere(samples):
    """Cosine-weighted directions around +Z from a fixed Hammersley set."""
    result = []
    for i in range(samples):
        u = (i + .5) / samples
        v, f, k = 0.0, .5, i
        while k:
            v += f * (k & 1)
            k >>= 1
            f *= .5
        r, phi = sqrt(u), 2*pi*v
        result.append(Vector((r*cos(phi), r*sin(phi), sqrt(max(0.0, 1-u)))))
    return result


def bake_occlusion(objects, ground=None, distance=1.6, samples=(28, 20, 12), floor=.3, power=1.25, lod=None):
    """Write tint * ambient occlusion into the active corner colour attribute.

    Rays are cast against every part of the asset (and an optional ground plane at height
    `ground`). The sample set is fixed and rotated per position, so repeated builds give
    identical bytes.
    """
    vertices, triangles = [], []
    for obj in objects:
        base = len(vertices)
        vertices.extend(obj.matrix_local @ v.co for v in obj.data.vertices)
        obj.data.calc_loop_triangles()
        triangles.extend(tuple(base + i for i in t.vertices) for t in obj.data.loop_triangles)
    if ground is not None:
        base = len(vertices)
        vertices.extend(Vector(p) for p in ((-500, -500, ground), (500, -500, ground), (500, 500, ground), (-500, 500, ground)))
        triangles.extend(((base, base+1, base+2), (base, base+2, base+3)))
    tree = BVHTree.FromPolygons(vertices, triangles, all_triangles=True, epsilon=0.0)
    for obj in objects:
        mesh = obj.data
        level = obj.parent.get('lod', 0) if lod is None else lod
        dirs = hemisphere(samples[level] if isinstance(samples, (tuple, list)) else samples)
        tints = [0.0] * len(mesh.polygons)
        mesh.attributes['tint'].data.foreach_get('value', tints)
        normals = mesh.corner_normals
        rotation = obj.matrix_local.to_3x3()
        cache, colors = {}, [0.0] * (len(mesh.loops) * 4)
        for polygon in mesh.polygons:
            tint = tints[polygon.index]
            for loop_index in polygon.loop_indices:
                vertex = mesh.loops[loop_index].vertex_index
                normal = (rotation @ normals[loop_index].vector).normalized()
                key = (vertex, round(normal.x, 3), round(normal.y, 3), round(normal.z, 3))
                value = cache.get(key)
                if value is None:
                    position = obj.matrix_local @ mesh.vertices[vertex].co
                    helper = Vector((1, 0, 0)) if abs(normal.x) < .8 else Vector((0, 1, 0))
                    tangent = normal.cross(helper).normalized()
                    bitangent = normal.cross(tangent)
                    spin = (sin(position.x*12.9898 + position.y*78.233 + position.z*37.719) * 43758.5453) % 1.0 * 2*pi
                    cs, sn = cos(spin), sin(spin)
                    origin = position + normal * .006
                    blocked = 0.0
                    for d in dirs:
                        x, y = d.x*cs - d.y*sn, d.x*sn + d.y*cs
                        hit = tree.ray_cast(origin, tangent*x + bitangent*y + normal*d.z, distance)
                        if hit[0] is not None:
                            blocked += (1 - hit[3]/distance) ** 1.5
                    value = floor + (1 - floor) * (1 - blocked/len(dirs)) ** power
                    cache[key] = value
                shade = max(0.0, min(1.0, value * tint))
                colors[loop_index*4:loop_index*4+4] = (shade, shade, shade, 1.0)
        attribute = mesh.color_attributes.new('Color', 'BYTE_COLOR', 'CORNER')
        attribute.data.foreach_set('color', colors)
        mesh.color_attributes.active_color = attribute
        mesh.color_attributes.render_color_index = 0
        mesh.attributes.remove(mesh.attributes['tint'])


# -- export -----------------------------------------------------------------------------
def bounds_and_triangles(root):
    depsgraph = bpy.context.evaluated_depsgraph_get()
    coords, triangles = [], 0
    for obj in root.children:
        if obj.type != 'MESH':
            continue
        evaluated = obj.evaluated_get(depsgraph).data
        evaluated.calc_loop_triangles()
        triangles += len(evaluated.loop_triangles)
        coords.extend(obj.matrix_local @ Vector(p) for p in obj.bound_box)
    low = [min(v[i] for v in coords) for i in range(3)]
    high = [max(v[i] for v in coords) for i in range(3)]
    # Blender Z-up -> glTF Y-up; metres, base-centred pivots.
    bounds = {'min': [round(low[0], 5), round(low[2], 5), round(-high[1], 5)],
              'max': [round(high[0], 5), round(high[2], 5), round(-low[1], 5)]}
    return bounds, triangles


def export_asset(root, output, animations=False):
    bpy.ops.object.select_all(action='DESELECT')
    root.select_set(True)
    for child in root.children_recursive:
        child.select_set(True)
    bpy.context.view_layer.objects.active = root
    bpy.context.view_layer.update()
    bounds, triangles = bounds_and_triangles(root)
    bpy.ops.export_scene.gltf(filepath=str(output), export_format='GLB', use_selection=True,
                             use_active_scene=True, export_animations=animations, export_cameras=False,
                             export_lights=False, export_extras=True, export_texcoords=True,
                             export_normals=True, export_tangents=False, export_yup=True, export_apply=True,
                             export_vertex_color='ACTIVE', export_all_vertex_colors=False,
                             export_active_vertex_color_when_no_material=True)
    # Stable names remove Blender's session-dependent .001 suffixes.
    data = output.read_bytes()
    json_length = struct.unpack_from('<I', data, 12)[0]
    document = json.loads(data[20:20+json_length])
    for index, node in enumerate(document.get('nodes', [])):
        extras = node.get('extras', {})
        node['name'] = extras.get('component', extras.get('asset_id', f'node{index}'))
    for index, mesh in enumerate(document.get('meshes', [])):
        mesh['name'] = f'mesh{index}'
    for scene in document.get('scenes', []):
        scene['name'] = 'CityAsset'
    payload = json.dumps(document, separators=(',', ':'), sort_keys=True).encode()
    payload += b' ' * (-len(payload) % 4)
    tail = data[20+json_length:]
    data = struct.pack('<III', 0x46546C67, 2, 20+len(payload)+len(tail))
    data += struct.pack('<II', len(payload), 0x4E4F534A) + payload + tail
    output.write_bytes(data)
    return {'file': output.name, 'triangles': triangles, 'bounds': bounds}


def source_hash(path):
    """SHA-256 over LF-normalised bytes, so a CRLF checkout hashes the same."""
    return hashlib.sha256(Path(path).read_bytes().replace(b'\r\n', b'\n')).hexdigest()


def reset_collection(scene):
    old = bpy.data.collections.get(COLLECTION)
    if old:
        for obj in list(old.all_objects):
            mesh = obj.data if obj.type == 'MESH' else None
            bpy.data.objects.remove(obj, do_unlink=True)
            if mesh and mesh.users == 0:
                bpy.data.meshes.remove(mesh)
        bpy.data.collections.remove(old)
    collection = bpy.data.collections.new(COLLECTION)
    scene.collection.children.link(collection)
    return collection


def remove_tree(root):
    for child in list(root.children_recursive):
        mesh = child.data if child.type == 'MESH' else None
        bpy.data.objects.remove(child, do_unlink=True)
        if mesh and mesh.users == 0:
            bpy.data.meshes.remove(mesh)
    bpy.data.objects.remove(root, do_unlink=True)
