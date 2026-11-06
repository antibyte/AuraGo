"""Aura Drift toy parts: shared helpers for items, track pieces, animals and props (MIT, AuraGo contributors).

Everything here is pure geometry plus a few Blender-side helpers. ``Parts`` collects shapes per
material and places them either in the **world frame** (``add``) or in a **character frame**
(``place``): x = the character's right, y = up, z = forward, which ``FACE`` maps to the glTF frame
(forward +X, right +Z). Characters (gnome, duck, animals) are authored in the character frame.

Materials (registered by ``ensure_materials``; colour comes from vertex colour):
    toy      glossy plastic        matte    satin paint        fuzz   wool, plush
    chrome   polished metal        mirror   mirror tiles       face   plain card for runtime textures
    glow     warm emissive         glow_red / glow_cyan / glow_lime / glow_white   emissive accents
    light    gantry lamp (white emissive, the runtime recolours it)
"""

import math
import sys
import types

import bpy
from mathutils import Matrix, Vector

from .core import Accum, hex_linear
from .materials import make_material
from .shapes import loft, transform, xf

# name -> (base, metallic, roughness, emission, strength, alpha, vertex_color)
TOY_MATERIALS = {
    "toy": ("#ffffff", 0.0, 0.24, None, 0.0, 1.0, True),
    "matte": ("#ffffff", 0.0, 0.72, None, 0.0, 1.0, True),
    "fuzz": ("#ffffff", 0.0, 0.95, None, 0.0, 1.0, True),
    "chrome": ("#dfe4ec", 1.0, 0.16, None, 0.0, 1.0, True),
    "mirror": ("#ffffff", 0.55, 0.1, None, 0.0, 1.0, True),
    "face": ("#ffffff", 0.0, 0.55, None, 0.0, 1.0, False),
    "glow": ("#ffffff", 0.0, 0.3, "#ffc83d", 2.6, 1.0, True),
    "glow_red": ("#ffffff", 0.0, 0.3, "#ff3b2a", 3.0, 1.0, True),
    "glow_cyan": ("#ffffff", 0.0, 0.3, "#5ff0ff", 2.5, 1.0, True),
    "glow_lime": ("#ffffff", 0.0, 0.3, "#a6ff3a", 2.5, 1.0, True),
    "glow_white": ("#ffffff", 0.0, 0.3, "#ffffff", 2.2, 1.0, True),
    "light": ("#ffffff", 0.0, 0.3, "#ffffff", 1.5, 1.0, True),
}

# character frame (x right, y up, z forward) -> glTF frame (X forward, Y up, Z right)
FACE = Matrix(((0, 0, 1, 0), (0, 1, 0, 0), (1, 0, 0, 0), (0, 0, 0, 1)))
MIRROR_X = Matrix.Diagonal((-1.0, 1.0, 1.0, 1.0))


def ensure_materials():
    for name, spec in TOY_MATERIALS.items():
        make_material(name, *spec[:6], vertex_color=spec[6])


def col(code):
    return hex_linear(code) if isinstance(code, str) else code


def lp(point):
    """Character-frame point -> glTF frame."""
    return (point[2], point[1], point[0])


def L(pos=(0.0, 0.0, 0.0), rot=None, scale=None):
    """Character-frame placement as a glTF-frame matrix."""
    return FACE @ xf(pos, rot, scale)


class Parts:
    """Shapes grouped by material. ``pivot`` (character frame) is subtracted for jointed parts."""

    def __init__(self, pivot=(0.0, 0.0, 0.0)):
        self.acc = {}
        self.pivot = pivot

    def add(self, mat, shape, matrix=None, tint=None, faces=None):
        """World-frame placement. ``tint`` is a hex string, a linear colour or fn(x, y, z) -> colour."""
        if callable(tint):
            verts, shape_faces = transform(shape, matrix) if matrix is not None else shape
            tint = [col(tint(*p)) for p in verts]
            shape, matrix = (verts, shape_faces), None
        self.acc.setdefault(mat, Accum()).add(shape, matrix, col(tint) if tint is not None and not isinstance(tint, list) else tint, faces)

    def place(self, mat, shape, pos=(0.0, 0.0, 0.0), rot=None, scale=None, tint=None, mirror=False):
        """Character-frame placement; ``mirror`` reflects across the centre plane (x -> -x)."""
        local = xf(pos, rot, scale)
        if mirror:
            local = MIRROR_X @ local @ MIRROR_X
        origin = Matrix.Translation(Vector(self.pivot) * -1.0)
        self.add(mat, shape, FACE @ origin @ local, tint)

    def pair(self, mat, shape, pos, rot=None, scale=None, tint=None):
        self.place(mat, shape, pos, rot, scale, tint)
        self.place(mat, shape, pos, rot, scale, tint, mirror=True)

    def include(self, other, matrix=None):
        """Copy another Parts (already in the glTF frame) with a world-frame matrix."""
        for mat, acc in other.acc.items():
            self.acc.setdefault(mat, Accum()).add((acc.verts, acc.faces), matrix, acc.cols)

    def bake(self, fn):
        for acc in self.acc.values():
            acc.tint_by(fn)

    def items(self):
        return list(self.acc.items())

    def triangles(self):
        return sum(a.triangles() for a in self.acc.values())


def ground_ao(top=0.9, low=0.66):
    """Baked ambient occlusion: darker near the ground, full brightness from ``top`` metres up."""

    def fn(x, y, z):
        t = min(max(y / top, 0.0), 1.0)
        return low + (1.0 - low) * t * t * (3.0 - 2.0 * t)

    return fn


# --------------------------------------------------------------------------
# sweeps
# --------------------------------------------------------------------------


def ribbon(path, width, depth):
    """Flat band along an XY polyline, extruded from z = 0 to ``depth`` (glyphs, chevrons, arrows).

    ``width`` is one number or a per-point list. Facing +Z.
    """
    pts = [Vector((p[0], p[1], 0.0)) for p in path]
    rings = []
    for i, p in enumerate(pts):
        a = pts[max(i - 1, 0)]
        b = pts[min(i + 1, len(pts) - 1)]
        t = (b - a).normalized()
        n = Vector((-t.y, t.x, 0.0))
        hw = (width[i] if isinstance(width, (list, tuple)) else width) / 2.0
        up = Vector((0.0, 0.0, depth))
        rings.append([tuple(p + n * hw), tuple(p - n * hw), tuple(p - n * hw + up), tuple(p + n * hw + up)])
    return loft(rings)


def tube(points, radius, sides=6, caps=True):
    """Round tube along a 3D polyline with a parallel-transport frame; ``radius`` is a number or per-point list."""
    P = [Vector(p) for p in points]
    n = len(P)
    tangents = [(P[min(i + 1, n - 1)] - P[max(i - 1, 0)]).normalized() for i in range(n)]
    t0 = tangents[0]
    helper = Vector((0.0, 1.0, 0.0)) if abs(t0.y) < 0.9 else Vector((1.0, 0.0, 0.0))
    normal = t0.cross(helper).normalized()
    rings = []
    for i in range(n):
        t = tangents[i]
        normal = (normal - t * normal.dot(t)).normalized()
        binormal = t.cross(normal)
        r = radius[i] if isinstance(radius, (list, tuple)) else radius
        ring = []
        for k in range(sides):
            a = math.tau * k / sides
            ring.append(tuple(P[i] + (normal * math.cos(a) + binormal * math.sin(a)) * r))
        rings.append(ring)
    return loft(rings, caps, caps)


def arc_points(cx, cy, r, a0, a1, count):
    return [
        (cx + r * math.cos(math.radians(a0 + (a1 - a0) * k / (count - 1))), cy + r * math.sin(math.radians(a0 + (a1 - a0) * k / (count - 1))))
        for k in range(count)
    ]


def helix(radius, length, turns, count, y=0.0, z=0.0, taper=None):
    """Spring path along X (centre line at height y, depth z)."""
    pts = []
    for k in range(count):
        t = k / (count - 1)
        a = math.tau * turns * t
        r = radius if taper is None else radius * taper(t)
        pts.append((-length / 2.0 + length * t, y + r * math.sin(a), z + r * math.cos(a)))
    return pts


def quad_facing_x(x, y0, y1, z_half):
    """Single-sided card facing +X spanning z in [-z_half, z_half] and y in [y0, y1]."""
    return [(x, y0, z_half), (x, y0, -z_half), (x, y1, -z_half), (x, y1, z_half)], [(0, 1, 2, 3)]


def add_face_uv(obj):
    """UV (0 to 1) for a card that faces +X: u runs left to right as seen from the front, v upward."""
    mesh = obj.data
    xs = [v.co.y for v in mesh.vertices]  # blender y = -gltf z
    ys = [v.co.z for v in mesh.vertices]
    y_lo, y_hi, z_lo, z_hi = min(xs), max(xs), min(ys), max(ys)
    layer = mesh.uv_layers.new(name="UVMap")
    for loop in mesh.loops:
        co = mesh.vertices[loop.vertex_index].co
        u = (co.y - y_lo) / max(y_hi - y_lo, 1e-9)  # blender +y is gltf -z: u grows toward the viewer's right
        v = (co.z - z_lo) / max(z_hi - z_lo, 1e-9)
        layer.data[loop.index].uv = (u, v)


# --------------------------------------------------------------------------
# glTF node names: the same child name (``body``, ``face``) in several pieces of one file
# --------------------------------------------------------------------------


def install_name_hook():
    """Blender object names are unique per file, glTF node names need not be.

    Objects named ``cow|body`` are written as ``body``: the glTF exporter calls this hook for
    every node (registered through a throw-away add-on entry, which factory startup allows).
    """
    if "agdrift_node_names" in sys.modules:
        return

    class glTF2ExportUserExtension:  # noqa: N801 (name is the exporter's contract)
        def gather_node_hook(self, gltf2_object, blender_object, export_settings):
            if "|" in blender_object.name:
                gltf2_object.name = blender_object.name.split("|", 1)[1]

    module = types.ModuleType("agdrift_node_names")
    module.glTF2ExportUserExtension = glTF2ExportUserExtension
    sys.modules["agdrift_node_names"] = module
    entry = bpy.context.preferences.addons.new()
    entry.module = "agdrift_node_names"
