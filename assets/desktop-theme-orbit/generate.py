"""Generate AuraGo's original MIT-licensed Orbit desktop theme model pack in Blender 5.2 LTS.

Run from the repository root:

    blender --background --factory-startup --python assets/desktop-theme-orbit/generate.py

Optional preview renders (EEVEE, never written into the repository):

    blender --background --factory-startup --python assets/desktop-theme-orbit/generate.py -- --preview <dir>

Development flag: ``-- --only station,shuttle`` builds and exports just those GLBs and skips
the manifest, the license and the .blend.

The script writes one GLB per model (station, shuttle, satellite, asteroids) plus
``manifest.json`` and ``LICENSE.txt`` into ``ui/3d/desktop-theme-orbit/v1/`` and saves the
editable source scene to ``assets/desktop-theme-orbit/orbit-models.blend``.

Everything is built from code, in metres, and authored in the glTF frame (Y up, +Z forward)
before the vertices are turned into Blender's Z-up space, so the exporter's Y-up conversion
returns exactly the authored frame. Geometry is accumulated as plain vertex/face lists per
material, then written as one mesh per material with angle-based sharp edges and a weighted
normal modifier, which keeps big flat panels flat and gives bevels a crisp highlight line.
There are no textures and no UVs: surface detail comes from geometry, inset faces and
vertex colours (multiplied onto the material base colour at runtime).

The output is deterministic: only seeded ``random.Random`` instances and ``mathutils.noise``
with fixed offsets are used, and the export options are spelled out explicitly.
"""

import hashlib
import json
import math
import os
import random
import struct
import sys

import bmesh
import bpy
from mathutils import Euler, Matrix, Vector, noise

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
RUNTIME = os.path.join(ROOT, "ui", "3d", "desktop-theme-orbit", "v1")
SOURCE = os.path.join(ROOT, "assets", "desktop-theme-orbit", "orbit-models.blend")
LICENSE = os.path.join(RUNTIME, "LICENSE.txt")
MANIFEST = os.path.join(RUNTIME, "manifest.json")

TAU = math.tau
COLOR_LAYER = "Col"
MODEL_FILES = ("station", "shuttle", "satellite", "asteroids")


# --------------------------------------------------------------------------
# colour helpers
# --------------------------------------------------------------------------


def srgb_to_linear(value):
    if value <= 0.04045:
        return value / 12.92
    return ((value + 0.055) / 1.055) ** 2.4


def hex_linear(code):
    """'#rrggbb' (sRGB) -> linear (r, g, b) floats, which is what Blender and glTF store."""
    code = code.lstrip("#")
    return tuple(srgb_to_linear(int(code[i : i + 2], 16) / 255.0) for i in (0, 2, 4))


def grey(value):
    return (value, value, value)


# --------------------------------------------------------------------------
# geometry kit: every function returns (verts, faces) in the authored Y-up frame
# --------------------------------------------------------------------------


def signed_volume(verts, faces):
    total = 0.0
    for face in faces:
        a = verts[face[0]]
        for index in range(1, len(face) - 1):
            b = verts[face[index]]
            c = verts[face[index + 1]]
            total += (
                a[0] * (b[1] * c[2] - b[2] * c[1])
                - a[1] * (b[0] * c[2] - b[2] * c[0])
                + a[2] * (b[0] * c[1] - b[1] * c[0])
            )
    return total / 6.0


def orient_outward(verts, faces):
    """Flip every face when a closed solid is inside out (signed volume < 0)."""
    if signed_volume(verts, faces) < 0.0:
        return [tuple(reversed(face)) for face in faces]
    return faces


def face_normal(verts, face):
    nx = ny = nz = 0.0
    for index, current in enumerate(face):
        a = verts[current]
        b = verts[face[(index + 1) % len(face)]]
        nx += (a[1] - b[1]) * (a[2] + b[2])
        ny += (a[2] - b[2]) * (a[0] + b[0])
        nz += (a[0] - b[0]) * (a[1] + b[1])
    length = math.sqrt(nx * nx + ny * ny + nz * nz) or 1.0
    return (nx / length, ny / length, nz / length)


def transform(shape, matrix):
    verts, faces = shape
    return [tuple(matrix @ Vector(v)) for v in verts], faces


def xf(pos=(0.0, 0.0, 0.0), rot=None, scale=None):
    """Translation @ rotation (degrees, XYZ euler) @ scale."""
    matrix = Matrix.Translation(Vector(pos))
    if rot is not None:
        matrix = matrix @ Euler(tuple(math.radians(a) for a in rot), "XYZ").to_matrix().to_4x4()
    if scale is not None:
        if isinstance(scale, (int, float)):
            scale = (scale, scale, scale)
        matrix = matrix @ Matrix.Diagonal((scale[0], scale[1], scale[2], 1.0))
    return matrix


def aim(origin, target, roll=0.0):
    """Matrix placing a +Y-axis primitive at ``origin`` pointing towards ``target``."""
    direction = Vector(target) - Vector(origin)
    quaternion = Vector((0.0, 1.0, 0.0)).rotation_difference(direction.normalized())
    matrix = Matrix.Translation(Vector(origin)) @ quaternion.to_matrix().to_4x4()
    if roll:
        matrix = matrix @ Matrix.Rotation(math.radians(roll), 4, "Y")
    return matrix


def ring_frame(angle, radius, y=0.0):
    """Frame on a Y-axis ring: local X radial, Y up, Z tangent (counter-clockwise)."""
    return Matrix.Translation(Vector((radius * math.cos(angle), y, radius * math.sin(angle)))) @ Matrix.Rotation(
        -angle, 4, "Y"
    )


def revolve(points, seg=32, closed=False, sweep=360.0, start=0.0):
    """Revolve a (radius, y) profile around the Y axis.

    Points on the axis (radius ~ 0) collapse into a single pole vertex. With ``closed`` the
    profile is a loop (torus-like). A partial ``sweep`` gets planar end caps.
    """
    count = len(points)
    full = abs(sweep - 360.0) < 1e-6
    steps = seg if full else seg + 1
    verts = []
    table = []
    for radius, y in points:
        if radius <= 1e-9:
            index = len(verts)
            verts.append((0.0, y, 0.0))
            table.append([index] * steps)
        else:
            ids = []
            for k in range(steps):
                angle = math.radians(start + sweep * k / seg)
                ids.append(len(verts))
                verts.append((radius * math.cos(angle), y, radius * math.sin(angle)))
            table.append(ids)
    faces = []
    pairs = count if closed else count - 1
    for i in range(pairs):
        j = (i + 1) % count
        for k in range(seg):
            k2 = (k + 1) % steps if full else k + 1
            a, b, c, d = table[i][k], table[i][k2], table[j][k2], table[j][k]
            if a == b and c == d:
                continue
            if a == b:
                faces.append((a, c, d))
            elif c == d:
                faces.append((a, b, c))
            else:
                faces.append((a, b, c, d))
    if not full:
        first = _dedupe([table[i][0] for i in range(count)])
        last = _dedupe([table[i][seg] for i in range(count - 1, -1, -1)])
        faces.append(tuple(first))
        faces.append(tuple(last))
    return verts, orient_outward(verts, faces)


def _dedupe(ids):
    out = []
    for value in ids:
        if not out or out[-1] != value:
            out.append(value)
    if len(out) > 1 and out[0] == out[-1]:
        out.pop()
    return out


def circle_profile(center_r, center_y, radius, count=12, squash=1.0):
    return [
        (
            center_r + radius * math.cos(TAU * k / count),
            center_y + radius * squash * math.sin(TAU * k / count),
        )
        for k in range(count)
    ]


def superellipse_profile(center_r, center_y, half_r, half_y, count=18, power=2.7):
    pts = []
    for k in range(count):
        t = TAU * k / count
        c = math.cos(t)
        s = math.sin(t)
        x = math.copysign(abs(c) ** (2.0 / power), c)
        y = math.copysign(abs(s) ** (2.0 / power), s)
        pts.append((center_r + half_r * x, center_y + half_y * y))
    return pts


def torus(major, minor, seg=32, minor_seg=8, y=0.0):
    return revolve(circle_profile(major, y, minor, minor_seg), seg, closed=True)


def cylinder(r0, r1=None, height=1.0, seg=16, chamfer=0.0):
    """Y-axis frustum centred on the origin, optional chamfered rims."""
    if r1 is None:
        r1 = r0
    h = height / 2.0
    c = min(chamfer, r0 * 0.9, r1 * 0.9, h * 0.9)
    pts = [(0.0, -h)]
    if c > 0.0:
        pts += [(r0 - c, -h), (r0, -h + c), (r1, h - c), (r1 - c, h)]
    else:
        pts += [(r0, -h), (r1, h)]
    pts.append((0.0, h))
    return revolve(pts, seg)


def sphere(radius, seg=16, rings=8):
    pts = [(radius * math.sin(math.pi * k / rings), -radius * math.cos(math.pi * k / rings)) for k in range(rings + 1)]
    pts[0] = (0.0, -radius)
    pts[-1] = (0.0, radius)
    return revolve(pts, seg)


def box_raw(sx, sy, sz):
    hx, hy, hz = sx / 2.0, sy / 2.0, sz / 2.0
    verts = [
        (-hx, -hy, -hz),
        (hx, -hy, -hz),
        (hx, hy, -hz),
        (-hx, hy, -hz),
        (-hx, -hy, hz),
        (hx, -hy, hz),
        (hx, hy, hz),
        (-hx, hy, hz),
    ]
    faces = [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (2, 3, 7, 6), (1, 2, 6, 5), (0, 4, 7, 3)]
    return verts, orient_outward(verts, faces)


def bevel_shape(shape, offset, segments=2, min_angle=None):
    """Bevel the edges of a closed solid with bmesh (all edges, or those sharper than min_angle)."""
    verts, faces = shape
    bm = bmesh.new()
    made = [bm.verts.new(v) for v in verts]
    for face in faces:
        try:
            bm.faces.new([made[i] for i in face])
        except ValueError:
            pass
    bm.verts.index_update()
    edges = []
    for edge in bm.edges:
        if min_angle is None:
            edges.append(edge)
        elif len(edge.link_faces) == 2 and edge.calc_face_angle(0.0) >= min_angle:
            edges.append(edge)
    bmesh.ops.bevel(
        bm,
        geom=edges,
        offset=offset,
        offset_type="OFFSET",
        profile_type="SUPERELLIPSE",
        segments=segments,
        profile=0.5,
        affect="EDGES",
        clamp_overlap=True,
        loop_slide=True,
    )
    bm.verts.index_update()
    out_verts = [tuple(v.co) for v in bm.verts]
    out_faces = [tuple(v.index for v in f.verts) for f in bm.faces]
    bm.free()
    return out_verts, orient_outward(out_verts, out_faces)


def box(sx, sy, sz, bevel=0.0, segs=2):
    shape = box_raw(sx, sy, sz)
    if bevel > 0.0:
        bevel = min(bevel, min(sx, sy, sz) * 0.45)
        return bevel_shape(shape, bevel, segs)
    return shape


def crinkled_box(sx, sy, sz, cuts, amp, rng):
    """Box whose faces are a subdivided grid with seeded outward crinkles (thermal foil)."""
    bm = bmesh.new()
    bmesh.ops.create_cube(bm, size=1.0)
    for vertex in bm.verts:
        vertex.co.x *= sx
        vertex.co.y *= sy
        vertex.co.z *= sz
    bmesh.ops.subdivide_edges(bm, edges=list(bm.edges), cuts=cuts, use_grid_fill=True)
    bm.verts.index_update()
    for vertex in sorted(bm.verts, key=lambda v: v.index):
        direction = vertex.co.normalized()
        vertex.co += direction * rng.uniform(-amp, amp)
    bm.verts.index_update()
    verts = [tuple(v.co) for v in bm.verts]
    faces = [tuple(v.index for v in f.verts) for f in bm.faces]
    bm.free()
    return verts, orient_outward(verts, faces)


def prism(polygon, y0, y1):
    """Extrude a planar polygon given as (x, z) points between two heights."""
    n = len(polygon)
    verts = [(x, y0, z) for x, z in polygon] + [(x, y1, z) for x, z in polygon]
    faces = [tuple(range(n - 1, -1, -1)), tuple(range(n, 2 * n))]
    for i in range(n):
        j = (i + 1) % n
        faces.append((i, j, n + j, n + i))
    return verts, orient_outward(verts, faces)


def strut(p, q, radius, sides=6, radius2=None):
    """Capped prism/cone between two points."""
    p = Vector(p)
    q = Vector(q)
    d = q - p
    d.normalize()
    helper = Vector((1.0, 0.0, 0.0)) if abs(d.x) < 0.9 else Vector((0.0, 0.0, 1.0))
    u = d.cross(helper)
    u.normalize()
    v = d.cross(u)
    r2 = radius if radius2 is None else radius2
    verts = []
    for centre, rad in ((p, radius), (q, r2)):
        for k in range(sides):
            angle = TAU * k / sides
            verts.append(tuple(centre + (u * math.cos(angle) + v * math.sin(angle)) * rad))
    faces = [tuple(range(sides - 1, -1, -1)), tuple(range(sides, 2 * sides))]
    for k in range(sides):
        j = (k + 1) % sides
        faces.append((k, j, sides + j, sides + k))
    return verts, orient_outward(verts, faces)


def loft(rings, cap_start=True, cap_end=True):
    """Skin a list of equally sized vertex rings (3D points)."""
    n = len(rings[0])
    verts = [tuple(p) for ring in rings for p in ring]
    faces = []
    for i in range(len(rings) - 1):
        for k in range(n):
            j = (k + 1) % n
            faces.append((i * n + k, i * n + j, (i + 1) * n + j, (i + 1) * n + k))
    if cap_start:
        faces.append(tuple(range(n - 1, -1, -1)))
    if cap_end:
        base = (len(rings) - 1) * n
        faces.append(tuple(range(base, base + n)))
    return verts, orient_outward(verts, faces)


def parabola_shell(radius, focal_divisor, thickness, seg=32, steps=10):
    """Parabolic dish shell, apex at the origin opening towards +Y (y = x^2 / focal_divisor)."""
    front = [(radius * i / steps, (radius * i / steps) ** 2 / focal_divisor) for i in range(steps + 1)]
    back = [(r, y - thickness) for r, y in reversed(front)]
    back[-1] = (0.0, back[-1][1])
    front[0] = (0.0, front[0][1])
    return revolve(front + back, seg)


def split_faces(verts, faces, predicate):
    """Partition faces by predicate(normal) -> (faces_true, faces_false)."""
    yes = []
    no = []
    for face in faces:
        (yes if predicate(face_normal(verts, face)) else no).append(face)
    return yes, no


# --------------------------------------------------------------------------
# accumulation, objects, materials
# --------------------------------------------------------------------------


class Accum:
    """Collects transformed parts that end up as one mesh with one material."""

    def __init__(self):
        self.verts = []
        self.faces = []
        self.cols = []
        self.tinted = False

    def add(self, shape, matrix=None, tint=None, faces=None):
        verts, all_faces = shape
        use_faces = all_faces if faces is None else faces
        if matrix is not None:
            verts = [tuple(matrix @ Vector(v)) for v in verts]
            if matrix.determinant() < 0.0:
                use_faces = [tuple(reversed(f)) for f in use_faces]
        used = sorted({i for f in use_faces for i in f})
        remap = {old: new for new, old in enumerate(used)}
        base = len(self.verts)
        for old in used:
            self.verts.append(verts[old])
            if tint is None:
                self.cols.append((1.0, 1.0, 1.0, 1.0))
            elif isinstance(tint, list):
                c = tint[old]
                self.cols.append((c[0], c[1], c[2], 1.0))
            else:
                self.cols.append((tint[0], tint[1], tint[2], 1.0))
        if tint is not None:
            self.tinted = True
        for f in use_faces:
            self.faces.append(tuple(remap[i] + base for i in f))

    def triangles(self):
        return sum(len(f) - 2 for f in self.faces)

    def __bool__(self):
        return bool(self.faces)


def to_blender(point):
    """Authored glTF frame (Y up, +Z forward) -> Blender Z-up frame (the exporter undoes this)."""
    return (point[0], -point[2], point[1])


class Model:
    def __init__(self, name):
        self.name = name
        self.objects = []
        self.roots = []
        self.stats = {}

    def empty(self, name, location=(0.0, 0.0, 0.0), parent=None):
        obj = bpy.data.objects.new(name, None)
        obj.empty_display_type = "PLAIN_AXES"
        obj.empty_display_size = 0.05
        obj.location = to_blender(location)
        bpy.context.scene.collection.objects.link(obj)
        self._register(obj, parent)
        return obj

    def mesh(self, name, acc, material, parent=None, smooth_angle=38.0, weighted=True, location=None):
        if not acc:
            return None
        mesh = bpy.data.meshes.new(name)
        mesh.from_pydata([to_blender(v) for v in acc.verts], [], acc.faces)
        mesh.update()
        bm = bmesh.new()
        bm.from_mesh(mesh)
        limit = math.radians(smooth_angle)
        for face in bm.faces:
            face.smooth = True
        for edge in bm.edges:
            edge.smooth = not (len(edge.link_faces) == 2 and edge.calc_face_angle(0.0) > limit)
        if acc.tinted:
            layer = bm.verts.layers.float_color.new(COLOR_LAYER)
            bm.verts.ensure_lookup_table()
            for vertex in bm.verts:
                vertex[layer] = acc.cols[vertex.index]
        bm.to_mesh(mesh)
        bm.free()
        mesh.materials.append(material)
        obj = bpy.data.objects.new(name, mesh)
        bpy.context.scene.collection.objects.link(obj)
        if weighted:
            modifier = obj.modifiers.new("WeightedNormal", "WEIGHTED_NORMAL")
            modifier.mode = "FACE_AREA"
            modifier.weight = 50
            modifier.keep_sharp = True
            modifier.thresh = 0.01
        if location is not None:
            obj.location = to_blender(location)
        self._register(obj, parent)
        self.stats[name] = acc.triangles()
        return obj

    def _register(self, obj, parent):
        if parent is not None:
            obj.parent = parent
        else:
            self.roots.append(obj)
        self.objects.append(obj)

    def triangles(self):
        return sum(self.stats.values())


MATERIALS = {}


def make_material(name, base, metallic, roughness, emission=None, strength=0.0):
    material = bpy.data.materials.new(name)
    material.use_nodes = True
    bsdf = material.node_tree.nodes["Principled BSDF"]
    rgb = hex_linear(base)
    bsdf.inputs["Base Color"].default_value = (rgb[0], rgb[1], rgb[2], 1.0)
    bsdf.inputs["Metallic"].default_value = metallic
    bsdf.inputs["Roughness"].default_value = roughness
    if emission is not None:
        e = hex_linear(emission)
        bsdf.inputs["Emission Color"].default_value = (e[0], e[1], e[2], 1.0)
        bsdf.inputs["Emission Strength"].default_value = strength
    else:
        bsdf.inputs["Emission Strength"].default_value = 0.0
    material.use_backface_culling = True
    material.diffuse_color = (rgb[0], rgb[1], rgb[2], 1.0)
    MATERIALS[name] = material
    return material


def build_materials():
    make_material("HullWhite", "#d9dde3", 0.2, 0.45)
    make_material("HullDark", "#3a404a", 0.6, 0.35)
    make_material("GoldFoil", "#c99a3b", 1.0, 0.28)
    make_material("SolarPanel", "#1b2a55", 0.5, 0.22)
    make_material("Glass", "#0d1a33", 0.1, 0.05)
    make_material("Rock", "#ffffff", 0.0, 0.92)
    make_material("WindowsEmissive", "#ffd8a0", 0.0, 0.6, "#ffd8a0", 4.0)
    make_material("DockLightsEmissive", "#5ce1ff", 0.0, 0.6, "#5ce1ff", 3.0)
    make_material("BeaconRedEmissive", "#ff3b3b", 0.0, 0.6, "#ff3b3b", 5.0)
    make_material("BeaconGreenEmissive", "#3bff7a", 0.0, 0.6, "#3bff7a", 5.0)
    make_material("EngineEmissive", "#9fe8ff", 0.0, 0.6, "#9fe8ff", 6.0)


def clear_scene():
    for obj in list(bpy.data.objects):
        bpy.data.objects.remove(obj, do_unlink=True)
    for datablocks in (bpy.data.meshes, bpy.data.materials, bpy.data.lights, bpy.data.cameras):
        for block in list(datablocks):
            datablocks.remove(block)


def tints(rng, count, low, high):
    return [grey(rng.uniform(low, high)) for _ in range(count)]


# --------------------------------------------------------------------------
# station
# --------------------------------------------------------------------------

RING_R = 1.0
RING_HALF = 0.085
RING_POWER = 2.7
WING_Y = 0.55


def ring_top(offset):
    """Height of the ring's upper surface at radial offset from the ring centreline."""
    x = min(abs(offset) / RING_HALF, 1.0)
    return RING_HALF * (1.0 - x**RING_POWER) ** (1.0 / RING_POWER)


def solar_panel(panel, frame, center, width, depth, nx, nz, rng, thickness=0.004, border=0.006):
    """Solar panel: dark frame, a dark carrier slab and a grid of raised cells (vertex-tinted)."""
    cx, cy, cz = center
    frame_t = thickness + 0.003
    frame.add(box_raw(width, frame_t, border), xf((cx, cy, cz + depth / 2.0 - border / 2.0)))
    frame.add(box_raw(width, frame_t, border), xf((cx, cy, cz - depth / 2.0 + border / 2.0)))
    frame.add(box_raw(border, frame_t, depth - 2.0 * border), xf((cx + width / 2.0 - border / 2.0, cy, cz)))
    frame.add(box_raw(border, frame_t, depth - 2.0 * border), xf((cx - width / 2.0 + border / 2.0, cy, cz)))
    iw = width - 2.0 * border
    idp = depth - 2.0 * border
    panel.add(box_raw(iw, thickness, idp), xf((cx, cy, cz)), tint=grey(0.18))
    gap = 0.0035
    cell_w = iw / nx
    cell_d = idp / nz
    lift = 0.0016
    for i in range(nx):
        for j in range(nz):
            x = cx - iw / 2.0 + cell_w * (i + 0.5)
            z = cz - idp / 2.0 + cell_d * (j + 0.5)
            panel.add(
                box_raw(cell_w - gap, lift, cell_d - gap),
                xf((x, cy + thickness / 2.0 + lift / 2.0 - 0.0002, z)),
                tint=grey(rng.uniform(0.72, 1.0)),
            )


def build_station():
    rng = random.Random(0x0B17)
    model = Model("station")
    M = MATERIALS
    rotor = model.empty("RingRotor")

    ring_w, ring_d, ring_win, ring_glass = Accum(), Accum(), Accum(), Accum()
    hull, dark, gold, solar, dock_lights = Accum(), Accum(), Accum(), Accum(), Accum()
    ring_lights, spine_win, accent = Accum(), Accum(), Accum()

    # ---- spine: one lathe silhouette with stepped sections -------------------------------
    spine = [
        (0.0, -0.836),
        (0.050, -0.836),
        (0.056, -0.850),
        (0.090, -0.850),
        (0.098, -0.842),
        (0.098, -0.806),
        (0.090, -0.798),
        (0.080, -0.798),
        (0.080, -0.652),
        (0.092, -0.644),
        (0.092, -0.616),
        (0.105, -0.606),
        (0.105, -0.142),
        (0.140, -0.108),
        (0.140, -0.092),
        (0.122, -0.084),
        (0.122, 0.084),
        (0.140, 0.092),
        (0.140, 0.282),
        (0.130, 0.292),
        (0.100, 0.312),
        (0.100, 0.496),
        (0.116, 0.504),
        (0.116, 0.596),
        (0.100, 0.604),
        (0.100, 0.690),
        (0.072, 0.708),
        (0.072, 0.826),
        (0.082, 0.832),
        (0.082, 0.842),
        (0.066, 0.850),
        (0.0, 0.850),
    ]
    hull.add(revolve(spine, 32))

    # dark ribs around the sections
    for y, r in ((-0.54, 0.105), (-0.40, 0.105), (-0.22, 0.105), (0.19, 0.14), (0.40, 0.10), (0.66, 0.10)):
        dark.add(torus(r + 0.001, 0.0075, 28, 5, y))
    # dark bearing gaps either side of the rotor collar and docking bands
    for y in (-0.80, -0.63):
        dark.add(torus(0.084 if y < -0.7 else 0.09, 0.006, 24, 4, y))

    # ---- docking module lights + port ----------------------------------------------------
    for k in range(8):
        a = TAU * k / 8.0
        dock_lights.add(box_raw(0.013, 0.006, 0.013), xf((0.074 * math.cos(a), -0.8505, 0.074 * math.sin(a)), rot=(0, -math.degrees(a), 0)))
    # docking petals/guides
    for k in range(4):
        a = TAU * (k + 0.5) / 4.0
        dark.add(box_raw(0.028, 0.026, 0.008), xf((0.094 * math.cos(a), -0.838, 0.094 * math.sin(a)), rot=(0, -math.degrees(a), 0)))
    model.empty("DockPort", (0.0, -0.85, 0.0))

    # ---- propellant tanks around the lower section ---------------------------------------
    # profile: bottom pole -> dome -> wall -> dome -> top pole
    tank_pts = [(0.0, -0.10)]
    for k in range(1, 5):
        a = math.pi / 2.0 * k / 4.0
        tank_pts.append((0.028 * math.sin(a), -0.072 - 0.028 * math.cos(a)))
    for k in range(1, 5):
        a = math.pi / 2.0 * k / 4.0
        tank_pts.append((0.028 * math.cos(a), 0.072 + 0.028 * math.sin(a)))
    tank_pts[-1] = (0.0, tank_pts[-1][1])
    tank_pts.insert(5, (0.028, -0.072))
    tank = revolve(tank_pts, 12)
    for k in range(4):
        a = math.radians(45.0 + 90.0 * k)
        tx, tz = 0.152 * math.cos(a), 0.152 * math.sin(a)
        hull.add(tank, xf((tx, -0.46, tz)))
        for dy in (-0.05, 0.05):
            dark.add(cylinder(0.0305, None, 0.012, 12), xf((tx, -0.46 + dy, tz)))
        for dy in (-0.07, 0.07):
            dark.add(strut((0.105 * math.cos(a), -0.46 + dy, 0.105 * math.sin(a)), (tx - 0.02 * math.cos(a), -0.46 + dy, tz - 0.02 * math.sin(a)), 0.004, 5))

    # ---- gold service modules ------------------------------------------------------------
    for sign, y, mod_w in ((1.0, 0.17, 0.15), (-1.0, 0.12, 0.13)):
        shape = crinkled_box(mod_w, 0.22, 0.13, 4, 0.0016, rng)
        verts, _ = shape
        gold.add(shape, xf((0.0, y, sign * 0.205)), tint=tints(rng, len(verts), 0.78, 1.0))
        dark.add(box_raw(mod_w * 0.8, 0.02, 0.012), xf((0.0, y + 0.11, sign * 0.205 + sign * 0.066)))
        for dx in (-0.045, 0.045):
            dark.add(strut((dx, y - 0.05, sign * 0.13), (dx, y - 0.05, sign * 0.15), 0.006, 6))
            dark.add(strut((dx, y + 0.05, sign * 0.13), (dx, y + 0.05, sign * 0.15), 0.006, 6))
        hull.add(cylinder(0.022, 0.016, 0.03, 12, 0.002), xf((mod_w * 0.2, y - 0.04, sign * 0.275), rot=(90, 0, 0)))

    # ---- radiator booms + louvre fins -----------------------------------------------------
    for sign in (1.0, -1.0):
        dark.add(box_raw(0.016, 0.016, 0.5), xf((0.0, -0.30, sign * 0.34)))
        for dz in (0.20, 0.35, 0.50):
            z = sign * dz
            hull.add(box_raw(0.008, 0.20, 0.12), xf((0.0, -0.30, z)))
            for dy in (-0.07, -0.02, 0.03, 0.08):
                dark.add(box_raw(0.0105, 0.005, 0.118), xf((0.0, -0.30 + dy, z)))
            dark.add(strut((0.0, -0.30, z - 0.05), (0.0, -0.30, z + 0.05), 0.006, 6))

    # ---- wing joint, truss, panels --------------------------------------------------------
    dark.add(cylinder(0.027, None, 0.24, 20, 0.003), xf((0.0, WING_Y, 0.0), rot=(0, 0, 90)))
    for sign in (1.0, -1.0):
        x_start, x_end = 0.12, 1.70
        build_truss_mirrored(dark, sign, x_start, x_end, WING_Y, 0.021, 0.1)
        # panels sit on the truss with stand-offs
        for index in range(4):
            x = sign * (0.12 + 0.045 + 0.17 + index * 0.352)
            solar_panel(solar, dark, (x, WING_Y + 0.043, 0.0), 0.34, 0.17, 4, 2, rng)
            for dz in (-0.06, 0.06):
                dark.add(strut((x, WING_Y + 0.021, dz), (x, WING_Y + 0.040, dz), 0.0035, 5))
        # end cap with beacon mast
        dark.add(box_raw(0.012, 0.06, 0.06), xf((sign * 1.705, WING_Y, 0.0)))
        dark.add(strut((sign * 1.705, WING_Y + 0.03, 0.0), (sign * 1.705, WING_Y + 0.062, 0.0), 0.004, 5))

    beacon_r = 0.02
    red = Accum()
    red.add(sphere(beacon_r, 12, 6), xf((1.705, WING_Y + 0.075, 0.0)))
    model.mesh("BeaconRed", red, M["BeaconRedEmissive"])
    green = Accum()
    green.add(sphere(beacon_r, 12, 6), xf((-1.705, WING_Y + 0.075, 0.0)))
    model.mesh("BeaconGreen", green, M["BeaconGreenEmissive"])

    # ---- high-gain dish on top ------------------------------------------------------------
    dark.add(cylinder(0.03, 0.045, 0.04, 20, 0.003), xf((0.0, 0.868, 0.0)))
    dark.add(strut((0.0, 0.888, 0.0), (0.0, 0.93, 0.0), 0.012, 8))
    dish_m = xf((0.0, 0.955, 0.0), rot=(32.0, 0, -26.0))
    hull.add(parabola_shell(0.16, 0.6, 0.008, 28, 6), dish_m)
    # ribs on the back of the dish
    for k in range(6):
        a = TAU * k / 6.0
        rib_pts = []
        for r in (0.012, 0.06, 0.11, 0.155):
            rib_pts.append((r * math.cos(a), r * r / 0.6 - 0.0115, r * math.sin(a)))
        for q0, q1 in zip(rib_pts, rib_pts[1:]):
            dark.add(strut(q0, q1, 0.0026, 3), dish_m)
    dark.add(sphere(0.017, 12, 6), dish_m @ xf((0.0, -0.016, 0.0)))
    dark.add(cylinder(0.018, None, 0.028, 12, 0.003), xf((0.0, 0.935, 0.0), rot=(0, 0, 90)))
    focal = 0.6 / 4.0
    gold.add(cylinder(0.012, 0.0045, 0.05, 14, 0.001), dish_m @ xf((0.0, focal, 0.0)), tint=grey(0.95))
    for k in range(3):
        a = TAU * k / 3.0 + 0.5
        dark.add(strut((0.155 * math.cos(a), (0.155**2) / 0.6, 0.155 * math.sin(a)), (0.0, focal - 0.02, 0.0), 0.0025, 3), dish_m)
    # whip antennas + comms mast
    dark.add(strut((0.04, 0.84, 0.03), (0.07, 1.00, 0.05), 0.0025, 4))
    dark.add(strut((-0.04, 0.84, -0.03), (-0.06, 0.96, -0.05), 0.002, 4))

    # ---- RCS thruster clusters ------------------------------------------------------------
    for y in (-0.70, 0.76):
        for k in range(4):
            a = TAU * k / 4.0 + math.pi / 4.0
            r = 0.084 if y < 0 else 0.074
            base = (r * math.cos(a), y, r * math.sin(a))
            dark.add(box_raw(0.018, 0.026, 0.018), xf(base, rot=(0, -math.degrees(a), 0)))

    # ---- ring: hull, ribs, windows --------------------------------------------------------
    ring_shape = revolve(superellipse_profile(RING_R, 0.0, RING_HALF, RING_HALF, 18, RING_POWER), 96, closed=True)
    # 32 habitat modules, each a touch lighter or darker (vertex ids run ring by ring, 96 per ring)
    module_tint = [rng.uniform(0.82, 1.0) for _ in range(32)]
    ring_w.add(ring_shape, tint=[grey(module_tint[(index % 96) // 3]) for index in range(len(ring_shape[0]))])
    ring_d.add(revolve(circle_profile(RING_R - RING_HALF + 0.006, 0.0, 0.027, 6), 64, closed=True))
    rib_profile = superellipse_profile(RING_R, 0.0, RING_HALF * 1.04, RING_HALF * 1.04, 12, RING_POWER)
    intervals = 32
    for k in range(intervals):
        ring_d.add(revolve(rib_profile, 1, closed=True, sweep=1.0, start=k * 360.0 / intervals - 0.5))
        base = k * 360.0 / intervals
        for offset in (1.0 / 3.0, 2.0 / 3.0):
            a = math.radians(base + offset * 360.0 / intervals)
            ring_win.add(
                box_raw(0.004, 0.018, 0.055),
                ring_frame(a, RING_R + RING_HALF + 0.0012, 0.0),
                tint=None,
            )

    # hub collar + spokes (all rotate with the ring)
    ring_w.add(revolve([(0.132, -0.060), (0.160, -0.060), (0.172, -0.048), (0.172, 0.048), (0.160, 0.060), (0.132, 0.060)], 40, closed=True))
    ring_d.add(torus(0.172, 0.006, 40, 6, 0.0))
    for k in range(6):
        a = TAU * k / 6.0
        frame = ring_frame(a, 0.0, 0.0)
        ring_d.add(box(0.05, 0.075, 0.062, 0.004, 1), frame @ xf((0.196, 0.0, 0.0)))
        spoke_in = 0.22
        spoke_out = RING_R - RING_HALF + 0.012
        ring_w.add(strut((spoke_in, 0, 0), (spoke_out, 0, 0), 0.017, 10), frame)
        ring_w.add(strut((spoke_out - 0.07, 0, 0), (spoke_out, 0, 0), 0.034, 10, 0.026), frame)
        ring_d.add(box(0.06, 0.05, 0.05, 0.005, 1), frame @ xf((0.52, 0.0, 0.0)))
        ring_d.add(cylinder(0.021, None, 0.016, 12, 0.002), frame @ xf((0.30, 0.0, 0.0), rot=(0, 0, 90)))
        for dz in (-0.024, 0.024):
            ring_d.add(strut((spoke_in + 0.04, 0.02, dz), (spoke_out - 0.07, 0.02, dz), 0.0035, 4), frame)

    # hab pods on top of the ring (local frame: X radial, Y up, Z tangent)
    pod_angles = [math.radians(30.0 + 120.0 * k) for k in range(3)]
    for index, a in enumerate(pod_angles):
        frame = ring_frame(a, RING_R, ring_top(0.0) - 0.006)
        ring_d.add(box_raw(0.15, 0.012, 0.15), frame @ xf((0.0, 0.004, 0.0)))
        ring_w.add(box(0.12, 0.06, 0.19, 0.008, 2), frame @ xf((0.0, 0.034, 0.0)))
        ring_glass.add(box_raw(0.05, 0.006, 0.12), frame @ xf((0.0, 0.0655, 0.0)))
        for dx in (-0.04, 0.04):
            ring_d.add(box_raw(0.016, 0.01, 0.17), frame @ xf((dx, 0.066, 0.0)))
        ring_d.add(box_raw(0.014, 0.012, 0.19), frame @ xf((0.0, 0.034, 0.0)))
        for dz in (-0.105, 0.105):
            ring_d.add(cylinder(0.025, None, 0.02, 14, 0.003), frame @ xf((0.0, 0.034, dz), rot=(90, 0, 0)))
        for w in range(5):
            ring_win.add(box_raw(0.003, 0.012, 0.018), frame @ xf((0.0605, 0.036, (w - 2) * 0.034)))
        ring_d.add(strut((-0.035, 0.06, 0.05), (-0.04, 0.15, 0.052), 0.0025, 4), frame)
        ring_d.add(sphere(0.006, 8, 4), frame @ xf((-0.04, 0.152, 0.052)))
        if index == 1:
            ring_w.add(parabola_shell(0.035, 0.12, 0.003, 16, 5), frame @ xf((0.03, 0.095, -0.03), rot=(0, 0, 25)))
            ring_d.add(strut((0.03, 0.06, -0.03), (0.03, 0.09, -0.03), 0.004, 5), frame)

    # cyan running lights: hub collar, pod roofs, ring shoulder
    for k in range(6):
        a = TAU * (k + 0.5) / 6.0
        ring_lights.add(box_raw(0.012, 0.004, 0.020), ring_frame(a, 0.150, 0.0618))
    for a in pod_angles:
        frame = ring_frame(a, RING_R, ring_top(0.0) - 0.006)
        for dx in (-0.052, 0.052):
            for dz in (-0.086, 0.086):
                ring_lights.add(box_raw(0.008, 0.004, 0.008), frame @ xf((dx, 0.0585, dz)))
    for k in range(24):
        a = math.radians(7.5 + 15.0 * k)
        ring_lights.add(box_raw(0.006, 0.003, 0.022), ring_frame(a, RING_R - 0.052, ring_top(-0.052) + 0.0006))

    # warm windows on the spine: core, wing mount and docking neck
    for y, r, n, phase in ((0.215, 0.1415, 14, 0.0), (0.64, 0.1005, 10, 0.3), (-0.725, 0.0815, 8, 0.2), (-0.20, 0.1065, 10, 0.1)):
        for k in range(n):
            a = TAU * (k + phase) / n
            spine_win.add(box_raw(0.0036, 0.016, 0.020), ring_frame(a, r, y))
    # cyan tip lights on radiator booms and wing trusses
    for sign in (1.0, -1.0):
        accent.add(box_raw(0.010, 0.010, 0.010), xf((0.0, -0.30, sign * 0.598)))
        accent.add(box_raw(0.014, 0.006, 0.014), xf((sign * 1.69, WING_Y - 0.026, 0.0)))
        accent.add(box_raw(0.008, 0.008, 0.008), xf((sign * 0.12, WING_Y + 0.030, 0.0)))
    accent.add(box_raw(0.012, 0.012, 0.012), xf((0.0, 0.862, 0.052)))

    # greebles on the ring top and bottom
    for _ in range(64):
        a = rng.uniform(0.0, TAU)
        if any(abs(math.atan2(math.sin(a - p), math.cos(a - p))) < 0.16 for p in pod_angles):
            continue
        side = 1.0 if rng.random() < 0.5 else -1.0
        offset = rng.uniform(-0.04, 0.04)
        sx = rng.uniform(0.012, 0.05)
        sz = rng.uniform(0.012, 0.03)
        sy = rng.uniform(0.004, 0.012)
        frame = ring_frame(a, RING_R + offset, side * (ring_top(offset) + sy / 2.0 - 0.002))
        (ring_d if rng.random() < 0.55 else ring_w).add(box_raw(sz, sy, sx), frame)

    # ---- emit objects --------------------------------------------------------------------
    model.mesh("RingHull", ring_w, M["HullWhite"], rotor)
    model.mesh("RingDark", ring_d, M["HullDark"], rotor, smooth_angle=70.0)
    model.mesh("RingWindows", ring_win, M["WindowsEmissive"], rotor)
    model.mesh("RingGlass", ring_glass, M["Glass"], rotor)
    model.mesh("RingLights", ring_lights, M["DockLightsEmissive"], rotor)
    model.mesh("SpineHull", hull, M["HullWhite"])
    model.mesh("SpineDark", dark, M["HullDark"], smooth_angle=70.0)
    model.mesh("SpineWindows", spine_win, M["WindowsEmissive"])
    model.mesh("AccentLights", accent, M["DockLightsEmissive"])
    model.mesh("ServiceGold", gold, M["GoldFoil"])
    model.mesh("SolarPanels", solar, M["SolarPanel"])
    model.mesh("DockLights", dock_lights, M["DockLightsEmissive"])
    return model


def build_truss_mirrored(dark, sign, x0, x1, y, half, bay):
    """Square lattice truss along X: four longerons, square battens and alternating diagonals."""
    corners = [(half, half), (half, -half), (-half, -half), (-half, half)]

    def X(x):
        return sign * x

    for cy, cz in corners:
        dark.add(strut((X(x0), y + cy, cz), (X(x1), y + cy, cz), 0.0034, 4))
    bays = max(1, int(round((x1 - x0) / bay)))
    step = (x1 - x0) / bays
    for b in range(bays + 1):
        x = x0 + step * b
        for k in range(4):
            (ay, az), (by, bz) = corners[k], corners[(k + 1) % 4]
            dark.add(strut((X(x), y + ay, az), (X(x), y + by, bz), 0.0022, 3))
    for b in range(bays):
        xa = x0 + step * b
        xb = xa + step
        for k in range(4):
            (ay, az), (by, bz) = corners[k], corners[(k + 1) % 4]
            if (b + k) % 2 == 0:
                dark.add(strut((X(xa), y + ay, az), (X(xb), y + by, bz), 0.0018, 3))
            else:
                dark.add(strut((X(xb), y + ay, az), (X(xa), y + by, bz), 0.0018, 3))



# --------------------------------------------------------------------------
# shuttle (length ~0.18, forward +Z, origin at the nose docking probe tip)
# --------------------------------------------------------------------------


def catmull(p0, p1, p2, p3, f):
    return 0.5 * (
        (2.0 * p1)
        + (-p0 + p2) * f
        + (2.0 * p0 - 5.0 * p1 + 4.0 * p2 - p3) * f * f
        + (-p0 + 3.0 * p1 - 3.0 * p2 + p3) * f * f * f
    )


def sample_keys(keys, u):
    n = len(keys)
    i = min(int(u), n - 2)
    f = u - i
    idx = [max(i - 1, 0), i, i + 1, min(i + 2, n - 1)]
    return [catmull(*(keys[j][c] for j in idx), f) for c in range(len(keys[0]))]


SHUTTLE_FUSE_LEN = 0.134
SHUTTLE_NOSE_Z = -0.006
# (t along the fuselage, half width, top height, belly depth, centre y)
SHUTTLE_KEYS = [
    (0.00, 0.0045, 0.0045, 0.0040, 0.000),
    (0.05, 0.0110, 0.0110, 0.0075, -0.001),
    (0.16, 0.0195, 0.0195, 0.0120, -0.002),
    (0.32, 0.0255, 0.0245, 0.0150, 0.000),
    (0.52, 0.0285, 0.0260, 0.0165, 0.002),
    (0.72, 0.0290, 0.0255, 0.0170, 0.003),
    (0.88, 0.0275, 0.0245, 0.0170, 0.003),
    (1.00, 0.0262, 0.0235, 0.0165, 0.003),
]


def shuttle_ring(params, count=20, top_power=2.4, belly_power=5.0, side_power=2.6):
    t, hw, ht, hb, yc = params
    z = SHUTTLE_NOSE_Z - t * SHUTTLE_FUSE_LEN
    pts = []
    for k in range(count):
        a = TAU * k / count
        c = math.cos(a)
        s = math.sin(a)
        x = hw * math.copysign(abs(c) ** (2.0 / side_power), c)
        if s >= 0.0:
            y = yc + ht * s ** (2.0 / top_power)
        else:
            y = yc - hb * (-s) ** (2.0 / belly_power)
        pts.append((x, y, z))
    return pts


def shuttle_top(u, x):
    """Height and z of the fuselage upper surface at lateral offset x for key-space position u."""
    t, hw, ht, hb, yc = sample_keys(SHUTTLE_KEYS, u)
    c = min(abs(x) / hw, 1.0) ** 1.3
    return yc + ht * max(1.0 - c * c, 0.0) ** (1.0 / 2.4), SHUTTLE_NOSE_Z - t * SHUTTLE_FUSE_LEN


def bell_nozzle(r_throat, r_mouth, length, wall=0.0012, steps=6, seg=14):
    """Bell nozzle along +Y: closed shell loop, throat at y = 0, mouth at y = length."""
    outer = []
    for i in range(steps + 1):
        f = i / steps
        outer.append((r_throat + (r_mouth - r_throat) * f**0.55, length * f))
    inner = [(r - wall, y) for r, y in reversed(outer)]
    return revolve(outer + inner, seg, closed=True)


def plate(polygon, thickness, bevel=0.0, segs=2):
    """Thin plate from a planar (x, z) polygon, centred on y = 0."""
    shape = prism(polygon, -thickness / 2.0, thickness / 2.0)
    if bevel > 0.0:
        return bevel_shape(shape, bevel, segs)
    return shape


SWAP_XY = Matrix(((0, 1, 0, 0), (1, 0, 0, 0), (0, 0, 1, 0), (0, 0, 0, 1)))


def build_shuttle():
    rng = random.Random(0x5A11)
    model = Model("shuttle")
    M = MATERIALS
    white, dark, glass, glow, lights = Accum(), Accum(), Accum(), Accum(), Accum()
    nav_r, nav_g = Accum(), Accum()

    # ---- lofted fuselage ----------------------------------------------------------------
    count = 20
    rings = []
    samples = [i * 0.5 for i in range(0, 2 * (len(SHUTTLE_KEYS) - 1) + 1)]
    for u in samples:
        rings.append(shuttle_ring(sample_keys(SHUTTLE_KEYS, u), count))
    verts, faces = loft(rings)
    body_faces = faces[: (len(rings) - 1) * count]
    cap_faces = faces[(len(rings) - 1) * count :]
    belly = []
    top = []
    for index, face in enumerate(body_faces):
        k = index % count
        mid = math.sin(TAU * (k + 0.5) / count)
        (belly if mid < -0.40 else top).append(face)
    white.add((verts, faces), faces=top + cap_faces[:1])
    # heat-shield tiles: every belly quad gets its own vertices and a seeded tint
    for face in belly:
        quad = [verts[i] for i in face]
        dark.add((quad, [(0, 1, 2, 3)]), tint=grey(rng.uniform(0.6, 1.0)))
    dark.add((verts, faces), faces=cap_faces[1:])

    # ---- canopy ------------------------------------------------------------------------
    glass.add(sphere(1.0, 20, 10), xf((0.0, 0.0190, -0.040), scale=(0.0135, 0.0095, 0.0210)))
    dark.add(box_raw(0.0012, 0.0016, 0.0165), xf((0.0, 0.0278, -0.040)))
    dark.add(box_raw(0.0200, 0.0014, 0.0012), xf((0.0, 0.0240, -0.0495)))

    # ---- delta wings with dihedral + winglets ----------------------------------------------
    planform = [(0.016, -0.030), (0.028, -0.052), (0.095, -0.113), (0.095, -0.129), (0.030, -0.135), (0.016, -0.135)]
    wing = plate(planform, 0.0048, 0.0013, 2)
    w_verts, w_faces = wing
    up_faces, down_faces = split_faces(w_verts, w_faces, lambda n: n[1] > -0.30)
    winglet = plate([(0.0, -0.100), (0.020, -0.108), (0.020, -0.125), (0.0, -0.128)], 0.0026, 0.0007, 1)
    for sign in (1.0, -1.0):
        pivot = Vector((0.028, -0.007, 0.0))
        place = (
            Matrix.Translation(pivot)
            @ Matrix.Rotation(math.radians(4.0), 4, "Z")
            @ Matrix.Translation(Vector((-0.028, 0.0, 0.0)))
        )
        full = Matrix.Diagonal((sign, 1.0, 1.0, 1.0)) @ place
        white.add(wing, full, faces=up_faces)
        dark.add(wing, full, faces=down_faces)
        # elevon hinge line
        dark.add(box_raw(0.058, 0.0010, 0.0011), full @ xf((0.062, 0.0026, -0.1235), rot=(0, 5, 0)))
        # winglet canted slightly outward at the tip
        fin_matrix = full @ xf((0.0945, 0.0, 0.0)) @ xf((0, 0, 0), rot=(0, 0, -18.0)) @ SWAP_XY
        white.add(winglet, fin_matrix)
        light_at = tuple(fin_matrix @ Vector((0.0215, 0.0, -0.1265)))
        # port (+X) red, starboard (-X) green; forward is +Z, up is +Y
        (nav_r if sign > 0 else nav_g).add(sphere(0.0030, 8, 4), xf(light_at))

    # ---- vertical fin -------------------------------------------------------------------
    fin = plate([(0.018, -0.088), (0.080, -0.117), (0.080, -0.128), (0.018, -0.136)], 0.0050, 0.0013, 2)
    white.add(fin, SWAP_XY)
    dark.add(box_raw(0.0056, 0.050, 0.0035), xf((0.0, 0.0535, -0.1265)))
    dark.add(box_raw(0.0056, 0.0035, 0.0125), xf((0.0, 0.0795, -0.1225)))

    # ---- payload bay doors and panel lines on the back ------------------------------------
    for sign in (1.0, -1.0):
        pts = []
        for u in (3.0, 3.6, 4.2, 4.8, 5.4, 6.0):
            y, z = shuttle_top(u, 0.0105)
            pts.append((sign * 0.0105, y + 0.0003, z))
        for a, b in zip(pts, pts[1:]):
            dark.add(strut(a, b, 0.0006, 3))
    for u in (3.0, 6.0):
        y, z = shuttle_top(u, 0.0)
        dark.add(box_raw(0.022, 0.0008, 0.0010), xf((0.0, y + 0.0003, z)))
    # OMS pods + RCS blocks
    for sign in (1.0, -1.0):
        dark.add(cylinder(0.0058, 0.0054, 0.026, 12, 0.0006), xf((sign * 0.0150, 0.0225, -0.1235), rot=(90, 0, 0)))
        dark.add(cylinder(0.0032, 0.0046, 0.003, 10), xf((sign * 0.0150, 0.0225, -0.1380), rot=(90, 0, 0)))
        dark.add(box_raw(0.004, 0.0024, 0.007), xf((sign * 0.0085, 0.0090, -0.0215)))

    # ---- nose docking probe ---------------------------------------------------------------
    dark.add(cylinder(0.0085, 0.0078, 0.0080, 16, 0.0011), xf((0.0, 0.0, -0.0030), rot=(90, 0, 0)))
    dark.add(torus(0.0058, 0.0009, 16, 4, 0.0), xf((0.0, 0.0, 0.0005), rot=(90, 0, 0)))
    for k in range(4):
        a = TAU * k / 4.0 + math.pi / 4.0
        lights.add(box_raw(0.0024, 0.0024, 0.0010), xf((0.0058 * math.cos(a), 0.0058 * math.sin(a), 0.0010)))

    # ---- engine block, three nozzles ----------------------------------------------------------
    tail_z = SHUTTLE_NOSE_Z - SHUTTLE_FUSE_LEN
    dark.add(box(0.054, 0.040, 0.012, 0.0018, 2), xf((0.0, 0.003, tail_z - 0.006)))
    nozzle_z = tail_z - 0.012
    nozzle = bell_nozzle(0.0055, 0.0095, 0.020)
    for nx, ny in ((0.0, 0.0092), (-0.0148, -0.0050), (0.0148, -0.0050)):
        dark.add(nozzle, xf((nx, ny, nozzle_z), rot=(-90, 0, 0)))
        glow.add(cylinder(0.0050, None, 0.0008, 14), xf((nx, ny, nozzle_z - 0.0042), rot=(90, 0, 0)))
    model.empty("Exhaust", (0.0, 0.0, nozzle_z - 0.020 - 0.020))

    model.mesh("Hull", white, M["HullWhite"])
    model.mesh("Belly", dark, M["HullDark"])
    model.mesh("Canopy", glass, M["Glass"])
    model.mesh("EngineGlow", glow, M["EngineEmissive"])
    model.mesh("NoseLights", lights, M["DockLightsEmissive"])
    model.mesh("NavLightPort", nav_r, M["BeaconRedEmissive"])
    model.mesh("NavLightStarboard", nav_g, M["BeaconGreenEmissive"])
    return model


# --------------------------------------------------------------------------
# satellite (bus 0.12, forward +Z, Dish pivots at its gimbal)
# --------------------------------------------------------------------------


def build_satellite():
    rng = random.Random(0x5A7E)
    model = Model("satellite")
    M = MATERIALS
    gold, white, dark, solar, glass = Accum(), Accum(), Accum(), Accum(), Accum()

    # ---- bus --------------------------------------------------------------------------
    bus = crinkled_box(0.120, 0.120, 0.140, 5, 0.0011, rng)
    gold.add(bus, tint=tints(rng, len(bus[0]), 0.76, 1.0))
    # radiator plate and frame on top
    white.add(box_raw(0.088, 0.003, 0.100), xf((0.0, 0.0617, -0.008)))
    for dz in (-0.062, 0.046):
        dark.add(box_raw(0.092, 0.004, 0.004), xf((0.0, 0.0617, dz)))
    for dx in (-0.046, 0.046):
        dark.add(box_raw(0.004, 0.004, 0.104), xf((dx, 0.0617, -0.008)))
    for k in range(4):
        dark.add(box_raw(0.080, 0.0016, 0.0022), xf((0.0, 0.0634, -0.04 + k * 0.026)))
    # side access panels + bus straps
    for sign in (1.0, -1.0):
        dark.add(box_raw(0.003, 0.040, 0.050), xf((sign * 0.0605, -0.012, 0.020)))
        dark.add(box_raw(0.0035, 0.122, 0.006), xf((sign * 0.0605, 0.0, -0.045)))
    # apogee motor (bell pointing -Z) and its collar
    dark.add(cylinder(0.021, 0.021, 0.010, 14), xf((0.0, 0.0, -0.075), rot=(90, 0, 0)))
    dark.add(bell_nozzle(0.011, 0.027, 0.040, 0.0016, 5, 14), xf((0.0, 0.0, -0.080), rot=(-90, 0, 0)))
    # instrument deck on the forward face
    dark.add(box_raw(0.074, 0.046, 0.028), xf((0.0, -0.016, 0.083)))
    dark.add(cylinder(0.0185, 0.0185, 0.022, 16), xf((-0.018, -0.016, 0.108), rot=(90, 0, 0)))
    glass.add(cylinder(0.0145, None, 0.002, 16), xf((-0.018, -0.016, 0.1195), rot=(90, 0, 0)))
    dark.add(cylinder(0.011, 0.011, 0.018, 12), xf((0.021, -0.016, 0.106), rot=(90, 0, 0)))
    glass.add(cylinder(0.0085, None, 0.002, 12), xf((0.021, -0.016, 0.1155), rot=(90, 0, 0)))
    for sign in (1.0, -1.0):
        # star tracker hoods
        dark.add(cylinder(0.0105, 0.0150, 0.026, 12), xf((sign * 0.040, 0.050, 0.060), rot=(78, 0, 0)))
        white.add(cylinder(0.0065, None, 0.004, 10), xf((sign * 0.040, 0.0555, 0.0712), rot=(78, 0, 0)))
        # feed horns
        white.add(cylinder(0.006, 0.014, 0.034, 12), xf((sign * 0.038, 0.020, 0.0885), rot=(90, 0, 0)))
    # thruster blocks on the corners
    for sx in (1.0, -1.0):
        for sz in (1.0, -1.0):
            base = (sx * 0.066, 0.040, sz * 0.056)
            dark.add(box_raw(0.014, 0.020, 0.020), xf(base))
            dark.add(cylinder(0.004, 0.0065, 0.014, 8), xf((base[0] + sx * 0.010, 0.040, base[2]), rot=(0, 0, -90 * sx)))

    # ---- solar wings ----------------------------------------------------------------------
    for sign in (1.0, -1.0):
        dark.add(box_raw(0.026, 0.026, 0.040), xf((sign * 0.073, 0.0, -0.010)))
        dark.add(strut((sign * 0.080, 0.0, -0.010), (sign * 0.100, 0.0, -0.010), 0.0042, 6))
        dark.add(box_raw(0.250, 0.0045, 0.0090), xf((sign * 0.2175, -0.0048, -0.010)))
        for index in range(3):
            cx = sign * (0.1355 + index * 0.081)
            solar_panel(solar, dark, (cx, 0.0, -0.010), 0.075, 0.100, 3, 3, rng, thickness=0.003, border=0.0045)
            dark.add(box_raw(0.0045, 0.0030, 0.0140), xf((cx + sign * 0.0405, -0.0030, -0.010)))
        # trim tab at the wing tip
        dark.add(box_raw(0.0065, 0.0050, 0.108), xf((sign * 0.3585, 0.0, -0.010)))

    # ---- dish on a boom (node 'Dish' pivots at the gimbal) ----------------------------------
    gimbal = (-0.040, 0.118, 0.012)
    dark.add(strut((-0.040, 0.060, -0.022), gimbal, 0.0055, 6))
    dark.add(strut((-0.052, 0.060, -0.040), (-0.042, 0.112, 0.008), 0.0026, 5))
    dark.add(box_raw(0.026, 0.012, 0.016), xf((-0.040, 0.060, -0.022)))
    dish = model.empty("Dish", gimbal)
    d_white, d_dark = Accum(), Accum()
    d_white.add(parabola_shell(0.060, 0.24, 0.0032, 24, 5), xf((0.0, 0.0, 0.012), rot=(90, 0, 0)))
    d_dark.add(cylinder(0.007, 0.010, 0.014, 10), xf((0.0, 0.0, 0.006), rot=(90, 0, 0)))
    d_dark.add(strut((0.0, -0.011, 0.0), (0.0, 0.011, 0.0), 0.0046, 6))
    focal = 0.24 / 4.0
    d_white.add(cylinder(0.0075, 0.0030, 0.020, 10), xf((0.0, 0.0, 0.012 + focal), rot=(90, 0, 0)))
    for k in range(3):
        a = TAU * k / 3.0 + math.pi / 2.0
        rim = (0.058 * math.cos(a), 0.058 * math.sin(a), 0.012 + 0.058**2 / 0.24)
        d_dark.add(strut(rim, (0.0, 0.0, 0.012 + focal - 0.006), 0.0013, 3))
    for k in range(4):
        a = TAU * k / 4.0 + math.pi / 4.0
        pts = [(r * math.cos(a), r * math.sin(a), 0.012 + r * r / 0.24 - 0.0050) for r in (0.010, 0.032, 0.056)]
        for q0, q1 in zip(pts, pts[1:]):
            d_dark.add(strut(q0, q1, 0.0016, 3))

    # ---- whip antennas and the sensor boom --------------------------------------------------
    dark.add(strut((0.046, 0.060, -0.052), (0.052, 0.215, -0.062), 0.0016, 4))
    dark.add(sphere(0.0035, 8, 4), xf((0.052, 0.215, -0.062)))
    dark.add(strut((0.030, 0.060, -0.060), (0.020, 0.150, -0.072), 0.0014, 4))
    dark.add(strut((0.0, -0.060, 0.0), (0.012, -0.150, -0.040), 0.0034, 6))
    dark.add(sphere(0.008, 10, 5), xf((0.012, -0.158, -0.043)))
    dark.add(strut((0.040, -0.060, 0.040), (0.095, -0.108, 0.082), 0.0016, 4))
    white.add(cylinder(0.006, 0.0, 0.020, 10), xf((0.0975, -0.1105, 0.0845), rot=(-35, 0, 40)))

    model.mesh("Bus", gold, M["GoldFoil"])
    model.mesh("BusWhite", white, M["HullWhite"])
    model.mesh("BusDark", dark, M["HullDark"])
    model.mesh("Wings", solar, M["SolarPanel"])
    model.mesh("Optics", glass, M["Glass"])
    model.mesh("DishReflector", d_white, M["HullWhite"], dish)
    model.mesh("DishFeed", d_dark, M["HullDark"], dish)
    return model


# --------------------------------------------------------------------------
# asteroids (five vertex-coloured, cratered rocks, x = 0, 3, 6, 9, 12)
#
# Each rock is a radius function over the unit sphere (layered mathutils.noise fractal and
# creased noise, 2-4 craters with floor/wall/rim, planar fracture cuts) scaled non-uniformly.
# Instead of collapsing a dense icosphere, the ~376 vertices are placed on purpose: rings
# inside every crater (floor, wall, rim) plus an even Fibonacci background. A convex hull of
# those directions gives the triangulation (2V - 4 triangles, always under the budget), and
# the vertices are then displaced radially, so crater rims survive at 750 triangles.
# --------------------------------------------------------------------------

ROCK_TRIANGLE_LIMIT = 760
ROCK_COLOR_LIGHT = hex_linear("#6e655c")
ROCK_COLOR_DARK = hex_linear("#3b3631")

# seed, per-axis scale, crater count, planar cuts, roughness
ROCK_SPECS = (
    (101, (1.18, 0.86, 1.00), 3, 1, 1.00),
    (202, (0.92, 1.22, 0.84), 4, 2, 0.85),
    (303, (1.28, 0.95, 0.88), 2, 3, 1.10),
    (404, (0.84, 0.88, 1.26), 3, 2, 0.95),
    (505, (1.08, 1.04, 0.82), 4, 1, 1.20),
)


def unit(vec):
    length = math.sqrt(vec[0] ** 2 + vec[1] ** 2 + vec[2] ** 2) or 1.0
    return (vec[0] / length, vec[1] / length, vec[2] / length)


class RockShape:
    """Radius function of a potato-shaped rock: layered fractal noise, craters and planar cuts."""

    def __init__(self, spec):
        seed, self.scale, craters, cuts, self.rough = spec
        r = random.Random(seed)
        self.offsets = [Vector((r.uniform(-50, 50), r.uniform(-50, 50), r.uniform(-50, 50))) for _ in range(4)]
        self.craters = []
        for _ in range(craters):
            axis = unit((r.gauss(0, 1), r.gauss(0, 1), r.gauss(0, 1)))
            self.craters.append((axis, r.uniform(0.34, 0.56), r.uniform(0.12, 0.19)))
        self.cuts = []
        for _ in range(cuts):
            axis = unit((r.gauss(0, 1), r.gauss(0, 1), r.gauss(0, 1)))
            self.cuts.append((axis, r.uniform(0.80, 0.90)))

    def base(self, d):
        p = Vector(d)
        low = noise.fractal(p * 0.85 + self.offsets[0], 1.0, 2.0, 3)
        crease = 1.0 - 2.0 * abs(noise.noise(p * 2.6 + self.offsets[1]))
        fine = noise.fractal(p * 6.5 + self.offsets[2], 1.0, 2.0, 2)
        radius = 1.0 + 0.24 * low + 0.080 * self.rough * crease + 0.028 * self.rough * fine
        for axis, offset in self.cuts:
            dot = d[0] * axis[0] + d[1] * axis[1] + d[2] * axis[2]
            if dot > 0.05:
                plane = offset / dot
                k = 0.10
                h = max(k - abs(radius - plane), 0.0) / k
                radius = min(radius, plane) - h * h * k * 0.25
        return radius

    def crater(self, d):
        total = 0.0
        for axis, size, depth in self.craters:
            dot = max(-1.0, min(1.0, d[0] * axis[0] + d[1] * axis[1] + d[2] * axis[2]))
            s = math.acos(dot) / size
            if s < 1.0:
                # flat-ish floor, steep wall
                wall = min(1.0, max(0.0, (s - 0.45) / 0.50))
                total -= depth * (1.0 - wall * wall * (3.0 - 2.0 * wall))
            rim = math.exp(-(((s - 1.04) / 0.16) ** 2))
            total += 0.42 * depth * rim
        return total

    def radius(self, d):
        return self.base(d) + self.crater(d)


def fibonacci_points(count):
    golden = math.pi * (3.0 - math.sqrt(5.0))
    points = []
    for i in range(count):
        y = 1.0 - 2.0 * (i + 0.5) / count
        r = math.sqrt(max(0.0, 1.0 - y * y))
        points.append((r * math.cos(golden * i), y, r * math.sin(golden * i)))
    return points


def angle_between(a, b):
    return math.acos(max(-1.0, min(1.0, a[0] * b[0] + a[1] * b[1] + a[2] * b[2])))


def rock_directions(shape, limit):
    """Vertex directions: designed rings inside every crater plus an even background."""
    points = []

    def add(direction):
        d = unit(direction)
        if all(angle_between(d, q) > 0.05 for q in points):
            points.append(d)

    for axis, size, depth in shape.craters:
        helper = (1.0, 0.0, 0.0) if abs(axis[0]) < 0.9 else (0.0, 1.0, 0.0)
        u = Vector(unit(tuple(Vector(axis).cross(Vector(helper)))))
        v = Vector(axis).cross(u)
        add(axis)
        # floor, wall and rim rings (angles relative to the crater radius)
        for ratio, count, twist in ((0.5, 7, 0.0), (0.85, 11, 0.3), (1.12, 15, 0.1)):
            a = ratio * size
            for k in range(count):
                phi = TAU * (k + twist) / count
                ring = Vector(axis) * math.cos(a) + (u * math.cos(phi) + v * math.sin(phi)) * math.sin(a)
                add(tuple(ring))
    fixed = len(points)
    for count in range(limit * 2, 60, -1):
        background = [
            p
            for p in fibonacci_points(count)
            if all(angle_between(p, axis) > 1.32 * size for axis, size, _ in shape.craters)
        ]
        merged = list(points)
        for p in background:
            if all(angle_between(p, q) > 0.09 for q in merged):
                merged.append(p)
        if len(merged) <= limit:
            return merged
    return points[:fixed]


def make_rock(index):
    shape = RockShape(ROCK_SPECS[index])
    directions = rock_directions(shape, (ROCK_TRIANGLE_LIMIT + 4) // 2 - 4)
    bm = bmesh.new()
    for d in directions:
        bm.verts.new(d)
    bm.verts.ensure_lookup_table()
    bmesh.ops.convex_hull(bm, input=list(bm.verts), use_existing_faces=False)
    bm.verts.index_update()
    verts = []
    for vertex in sorted(bm.verts, key=lambda v: v.index):
        d = unit(tuple(vertex.co))
        r = shape.radius(d)
        verts.append((d[0] * r * shape.scale[0], d[1] * r * shape.scale[1], d[2] * r * shape.scale[2]))
    faces = [tuple(v.index for v in f.verts) for f in bm.faces]
    bm.free()
    triangles = sum(len(f) - 2 for f in faces)
    assert triangles <= ROCK_TRIANGLE_LIMIT, "rock %d has %d triangles" % (index, triangles)
    # recentre on the bounding box so each rock tumbles about its middle
    lo = [min(v[a] for v in verts) for a in range(3)]
    hi = [max(v[a] for v in verts) for a in range(3)]
    mid = [(lo[a] + hi[a]) / 2.0 for a in range(3)]
    verts = [(v[0] - mid[0], v[1] - mid[1], v[2] - mid[2]) for v in verts]
    # vertex colours: grey-brown to dark by cavity and noise
    raw = []
    for v in verts:
        d = unit((v[0] / shape.scale[0], v[1] / shape.scale[1], v[2] / shape.scale[2]))
        actual = math.sqrt(v[0] ** 2 + v[1] ** 2 + v[2] ** 2)
        smooth_r = shape.base(d) * math.sqrt(
            (d[0] * shape.scale[0]) ** 2 + (d[1] * shape.scale[1]) ** 2 + (d[2] * shape.scale[2]) ** 2
        )
        cavity = max(0.0, min(1.0, (smooth_r - actual) / 0.18))
        patch = noise.fractal(Vector(v) * 2.2 + shape.offsets[3], 1.0, 2.0, 3)
        speck = noise.noise(Vector(v) * 9.0 + shape.offsets[2])
        raw.append(0.95 * patch + 0.30 * speck - 0.85 * cavity)
    ordered = sorted(raw)
    lo_f = ordered[int(len(ordered) * 0.03)]
    hi_f = ordered[int(len(ordered) * 0.97)]
    colors = []
    for value in raw:
        f = max(0.0, min(1.0, (value - lo_f) / max(hi_f - lo_f, 1e-6)))
        f = f * f * (3.0 - 2.0 * f) * 0.85 + 0.15 * f
        colors.append(tuple(ROCK_COLOR_DARK[c] + (ROCK_COLOR_LIGHT[c] - ROCK_COLOR_DARK[c]) * f for c in range(3)))
    return verts, faces, colors


def build_asteroids():
    model = Model("asteroids")
    M = MATERIALS
    for index in range(5):
        verts, faces, colors = make_rock(index)
        acc = Accum()
        acc.add((verts, orient_outward(verts, faces)), tint=colors)
        model.mesh(
            "Rock%d" % index,
            acc,
            M["Rock"],
            smooth_angle=34.0,
            weighted=False,
            location=(3.0 * index, 0.0, 0.0),
        )
    return model




def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def export_glb(model, path):
    bpy.ops.object.select_all(action="DESELECT")
    for obj in model.objects:
        obj.select_set(True)
    bpy.context.view_layer.objects.active = model.objects[0]
    options = {
        "filepath": path,
        "export_format": "GLB",
        "use_selection": True,
        "export_apply": True,
        "export_yup": True,
        "export_materials": "EXPORT",
        "export_image_format": "NONE",
        "export_normals": True,
        "export_texcoords": False,
        "export_tangents": False,
        "export_vertex_color": "ACTIVE",
        "export_attributes": False,
        "export_extras": False,
        "export_cameras": False,
        "export_lights": False,
        "export_animations": False,
        "export_skins": False,
        "export_morph": False,
    }
    supported = set(bpy.ops.export_scene.gltf.get_rna_type().properties.keys())
    bpy.ops.export_scene.gltf(**{key: value for key, value in options.items() if key in supported})


def glb_triangles(path):
    """Triangle count of a GLB, summed over primitives (index count / 3)."""
    with open(path, "rb") as handle:
        data = handle.read()
    length = struct.unpack("<I", data[12:16])[0]
    doc = json.loads(data[20 : 20 + length].decode("utf8").rstrip("\x00 "))
    total = 0
    for node in doc.get("nodes", []):
        if "mesh" not in node:
            continue
        for primitive in doc["meshes"][node["mesh"]]["primitives"]:
            if "indices" in primitive:
                total += doc["accessors"][primitive["indices"]]["count"] // 3
            else:
                total += doc["accessors"][primitive["attributes"]["POSITION"]]["count"] // 3
    return total


LICENSE_TEXT = """AuraGo Orbit desktop theme model pack

Original work for AuraGo; no third-party content (no scans, textures, meshes or
fonts were used).

Copyright (c) 2026 AuraGo contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
"""


# --------------------------------------------------------------------------
# preview renders (EEVEE, outside the repository)
# --------------------------------------------------------------------------


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    options = {"preview": None, "only": None}
    index = 0
    while index < len(argv):
        if argv[index] == "--preview" and index + 1 < len(argv):
            options["preview"] = argv[index + 1]
            index += 2
        elif argv[index] == "--only" and index + 1 < len(argv):
            options["only"] = argv[index + 1].split(",")
            index += 2
        else:
            index += 1
    return options


def world_bounds(objects):
    low = Vector((1e9, 1e9, 1e9))
    high = Vector((-1e9, -1e9, -1e9))
    found = False
    for obj in objects:
        if obj.type != "MESH":
            continue
        for corner in obj.bound_box:
            point = obj.matrix_world @ Vector(corner)
            for axis in range(3):
                low[axis] = min(low[axis], point[axis])
                high[axis] = max(high[axis], point[axis])
            found = True
    if not found:
        return Vector((0, 0, 0)), Vector((0, 0, 0))
    return low, high


def enable_preview_vertex_colors():
    """Preview-only: route the exported COLOR_0 layer into the base colour like glTF viewers do."""
    for obj in bpy.data.objects:
        if obj.type == "MESH" and COLOR_LAYER not in obj.data.color_attributes:
            layer = obj.data.color_attributes.new(COLOR_LAYER, "FLOAT_COLOR", "POINT")
            for item in layer.data:
                item.color = (1.0, 1.0, 1.0, 1.0)
    for material in bpy.data.materials:
        tree = material.node_tree
        bsdf = tree.nodes.get("Principled BSDF")
        if bsdf is None or tree.nodes.get("PreviewCol"):
            continue
        attribute = tree.nodes.new("ShaderNodeAttribute")
        attribute.name = "PreviewCol"
        attribute.attribute_name = COLOR_LAYER
        mix = tree.nodes.new("ShaderNodeMix")
        mix.data_type = "RGBA"
        mix.blend_type = "MULTIPLY"
        mix.inputs[0].default_value = 1.0
        mix.inputs[6].default_value = bsdf.inputs["Base Color"].default_value
        tree.links.new(attribute.outputs["Color"], mix.inputs[7])
        tree.links.new(mix.outputs[2], bsdf.inputs["Base Color"])


SHUTTLE_VIEWS = [
    ("", 34.0, 22.0, 1.25),
    ("-top", 20.0, 70.0, 1.25),
    ("-rear", 160.0, 14.0, 1.25),
    ("-nose", 28.0, 12.0, 0.45, (0.0, 0.0, -0.02)),
]
SATELLITE_VIEWS = [
    ("", 36.0, 24.0, 1.25),
    ("-top", 18.0, 68.0, 1.25),
    ("-front", 12.0, 10.0, 1.25),
]
ASTEROID_VIEWS = [
    ("", 0.0, 10.0, 1.05),
    ("-close", 25.0, 20.0, 0.30, (3.0, 0.0, 0.0)),
    ("-close2", 200.0, 30.0, 0.30, (9.0, 0.0, 0.0)),
]


def render_previews(models, outdir, views):
    os.makedirs(outdir, exist_ok=True)
    scene = bpy.context.scene
    scene.render.engine = "BLENDER_EEVEE"
    scene.render.resolution_x = 1024
    scene.render.resolution_y = 768
    scene.render.film_transparent = False
    scene.render.image_settings.file_format = "PNG"
    try:
        scene.eevee.taa_render_samples = 64
    except AttributeError:
        pass
    world = bpy.data.worlds.new("OrbitPreview")
    world.use_nodes = True
    tree = world.node_tree
    for node in list(tree.nodes):
        tree.nodes.remove(node)
    path = tree.nodes.new("ShaderNodeLightPath")
    sky = tree.nodes.new("ShaderNodeBackground")
    sky.inputs[0].default_value = (0.20, 0.27, 0.45, 1.0)
    sky.inputs[1].default_value = 0.55
    void = tree.nodes.new("ShaderNodeBackground")
    void.inputs[0].default_value = (0.012, 0.016, 0.034, 1.0)
    void.inputs[1].default_value = 1.0
    mix = tree.nodes.new("ShaderNodeMixShader")
    out = tree.nodes.new("ShaderNodeOutputWorld")
    tree.links.new(path.outputs["Is Camera Ray"], mix.inputs[0])
    tree.links.new(sky.outputs[0], mix.inputs[1])
    tree.links.new(void.outputs[0], mix.inputs[2])
    tree.links.new(mix.outputs[0], out.inputs[0])
    scene.world = world
    try:
        scene.view_settings.view_transform = "Standard"
    except TypeError:
        pass

    def sun(name, rotation, energy, color):
        data = bpy.data.lights.new(name, "SUN")
        data.energy = energy
        data.color = color
        obj = bpy.data.objects.new(name, data)
        obj.rotation_euler = tuple(math.radians(a) for a in rotation)
        scene.collection.objects.link(obj)
        return obj

    sun("Key", (52, 8, -38), 4.5, (1.0, 0.93, 0.82))
    sun("Fill", (70, 0, 140), 1.1, (0.55, 0.68, 1.0))
    sun("Rim", (115, 0, 205), 3.2, (0.7, 0.9, 1.0))

    camera_data = bpy.data.cameras.new("PreviewCam")
    camera_data.lens = 50
    camera_data.clip_start = 0.001
    camera_data.clip_end = 100.0
    camera = bpy.data.objects.new("PreviewCam", camera_data)
    scene.collection.objects.link(camera)
    scene.camera = camera
    constraint = camera.constraints.new("TRACK_TO")
    constraint.track_axis = "TRACK_NEGATIVE_Z"
    constraint.up_axis = "UP_Y"
    target = bpy.data.objects.new("PreviewTarget", None)
    scene.collection.objects.link(target)
    constraint.target = target

    enable_preview_vertex_colors()
    all_objects = [obj for model in models for obj in model.objects]
    for model in models:
        for obj in all_objects:
            obj.hide_render = obj not in model.objects
        low, high = world_bounds(model.objects)
        center = (low + high) / 2.0
        radius = (high - low).length / 2.0
        for view in views.get(model.name, [("", 32.0, 20.0, 1.0)]):
            suffix, azimuth, elevation, fill = view[:4]
            focus = Vector(to_blender(view[4])) if len(view) > 4 else center
            distance = radius * 2.2 * fill
            a = math.radians(azimuth)
            e = math.radians(elevation)
            camera.location = center + Vector(
                (
                    distance * math.cos(e) * math.sin(a),
                    -distance * math.cos(e) * math.cos(a),
                    distance * math.sin(e),
                )
            )
            camera.location += focus - center
            target.location = focus
            scene.render.filepath = os.path.join(outdir, "%s%s.png" % (model.name, suffix))
            bpy.ops.render.render(write_still=True)
            print("preview %s" % scene.render.filepath)


STATION_VIEWS = [
    ("", 34.0, 20.0, 1.05),
    ("-side", 8.0, 2.0, 1.05),
    ("-top", 30.0, 62.0, 1.05),
    ("-dock", 40.0, -22.0, 0.7),
    ("-dockclose", 35.0, -35.0, 0.22, (0.0, -0.78, 0.0)),
    ("-hub", 30.0, 25.0, 0.28, (0.0, 0.15, 0.0)),
]


def write_text(path, text):
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(text)


def layout_for_blend(models, direction=1.0):
    """Lay the four models side by side in the saved source scene (after the exports)."""
    offsets = {"station": 0.0, "shuttle": 4.0, "satellite": 6.5, "asteroids": 9.0}
    for model in models:
        for obj in model.roots:
            obj.location.x += offsets[model.name] * direction


def main():
    options = parse_args()
    os.makedirs(RUNTIME, exist_ok=True)
    clear_scene()
    build_materials()

    names = options["only"] or list(MODEL_FILES)
    models = [BUILDERS[name]() for name in names]
    entries = []
    for model in models:
        path = os.path.join(RUNTIME, model.name + ".glb")
        export_glb(model, path)
        triangles = glb_triangles(path)
        entries.append(
            {
                "name": model.name + ".glb",
                "bytes": os.path.getsize(path),
                "sha256": sha256(path),
                "triangles": triangles,
            }
        )
        print("exported %s: %d bytes, %d triangles" % (model.name, entries[-1]["bytes"], triangles))
        print("  " + ", ".join("%s=%d" % item for item in sorted(model.stats.items(), key=lambda kv: -kv[1])))

    if options["only"] is None:
        write_text(LICENSE, LICENSE_TEXT)
        manifest = {
            "version": 1,
            "generator": "Blender %s / assets/desktop-theme-orbit/generate.py" % bpy.app.version_string,
            "units": "metres",
            "up_axis": "Y",
            "license": "MIT",
            "files": entries,
        }
        write_text(MANIFEST, json.dumps(manifest, indent=2) + "\n")
        layout_for_blend(models)
        os.makedirs(os.path.dirname(SOURCE), exist_ok=True)
        bpy.ops.wm.save_as_mainfile(filepath=SOURCE, compress=True)
        layout_for_blend(models, -1.0)
        print("wrote %s (%d bytes total)" % (RUNTIME, sum(entry["bytes"] for entry in entries)))

    if options["preview"]:
        render_previews(
            models,
            options["preview"],
            {
                "station": STATION_VIEWS,
                "shuttle": SHUTTLE_VIEWS,
                "satellite": SATELLITE_VIEWS,
                "asteroids": ASTEROID_VIEWS,
            },
        )


BUILDERS = {"station": build_station, "shuttle": build_shuttle, "satellite": build_satellite, "asteroids": build_asteroids}

if __name__ == "__main__":
    main()
