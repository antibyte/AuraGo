"""Aura Drift kit core: colours, geometry accumulation, scene objects (MIT, AuraGo contributors).

Everything is authored in the glTF frame (Y up, metres, car nose on +X, right side +Z) and
converted with ``to_blender`` when a mesh is created, so the exporter's Y-up conversion
returns exactly the authored coordinates.
"""

import math
import os
import random

import bmesh
import bpy
from mathutils import Vector

TAU = math.tau
COLOR_LAYER = "Col"

KIT_DIR = os.path.dirname(os.path.abspath(__file__))
GEN_DIR = os.path.dirname(KIT_DIR)
REPO_ROOT = os.path.dirname(os.path.dirname(GEN_DIR))
DEFAULT_OUT = os.path.join(REPO_ROOT, "ui", "3d", "aura-drift", "v1")

# Material registry: name -> bpy material. Filled by kit.materials.build_all().
MATERIALS = {}


def srgb_to_linear(value):
    if value <= 0.04045:
        return value / 12.92
    return ((value + 0.055) / 1.055) ** 2.4


def hex_linear(code):
    """'#rrggbb' (sRGB) -> linear (r, g, b), which is what Blender and glTF store."""
    code = code.lstrip("#")
    return tuple(srgb_to_linear(int(code[i : i + 2], 16) / 255.0) for i in (0, 2, 4))


def grey(value):
    return (value, value, value)


def rng_for(*parts):
    """Seeded generator: the same parts always give the same stream (no time, no global state)."""
    seed = 20261106
    for part in parts:
        for char in str(part):
            seed = (seed * 131 + ord(char)) % 2147483647
    return random.Random(seed)


def to_blender(point):
    """Authored glTF frame (Y up) -> Blender Z-up frame (the exporter undoes this)."""
    return (point[0], -point[2], point[1])


class Accum:
    """Collects transformed shapes that end up in one material slot.

    ``add(shape, matrix, tint)``: ``tint`` is None (white), an (r, g, b) linear colour, or a
    per-vertex list matching the shape's vertices. ``faces`` selects a subset of a shape's
    faces, so one lofted hull can feed several materials.
    """

    def __init__(self):
        self.verts = []
        self.faces = []
        self.cols = []

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
            self.verts.append(tuple(verts[old]))
            if tint is None:
                self.cols.append((1.0, 1.0, 1.0, 1.0))
            elif isinstance(tint, list):
                c = tint[old]
                self.cols.append((c[0], c[1], c[2], 1.0))
            else:
                self.cols.append((tint[0], tint[1], tint[2], 1.0))
        for f in use_faces:
            self.faces.append(tuple(remap[i] + base for i in f))
        return self

    def tint_by(self, fn):
        """Multiply every vertex colour by fn(x, y, z) -> float (baked ambient occlusion)."""
        for index, (x, y, z) in enumerate(self.verts):
            k = fn(x, y, z)
            c = self.cols[index]
            self.cols[index] = (c[0] * k, c[1] * k, c[2] * k, c[3])

    def triangles(self):
        return sum(len(f) - 2 for f in self.faces)

    def __bool__(self):
        return bool(self.faces)


class Model:
    """The objects of one GLB. ``roots`` are exported as top-level nodes."""

    def __init__(self, name):
        self.name = name
        self.objects = []
        self.roots = []
        self.stats = {}

    def empty(self, name, location=(0.0, 0.0, 0.0), parent=None):
        obj = bpy.data.objects.new(name, None)
        obj.empty_display_type = "PLAIN_AXES"
        obj.empty_display_size = 0.1
        obj.location = to_blender(location)
        bpy.context.scene.collection.objects.link(obj)
        self._register(obj, parent)
        return obj

    def mesh(self, name, parts, shade="flat", angle=38.0, parent=None, location=None):
        """One object with one material slot per non-empty (material name, Accum) part.

        shade: 'flat' (face.smooth False, the chunky facet look), 'smooth', or 'auto'
        (smooth except edges sharper than ``angle`` degrees).
        """
        parts = [(m, acc) for m, acc in parts if acc]
        if not parts:
            return None
        verts, faces, cols, slots = [], [], [], []
        for slot, (_, acc) in enumerate(parts):
            base = len(verts)
            verts.extend(acc.verts)
            cols.extend(acc.cols)
            for f in acc.faces:
                faces.append(tuple(i + base for i in f))
                slots.append(slot)
        mesh = bpy.data.meshes.new(name)
        mesh.from_pydata([to_blender(v) for v in verts], [], faces)
        mesh.update()
        bm = bmesh.new()
        bm.from_mesh(mesh)
        bm.faces.ensure_lookup_table()
        limit = math.radians(angle)
        for face in bm.faces:
            face.smooth = shade != "flat"
            face.material_index = slots[face.index]
        if shade == "auto":
            for edge in bm.edges:
                edge.smooth = not (len(edge.link_faces) == 2 and edge.calc_face_angle(0.0) > limit)
        layer = bm.verts.layers.float_color.new(COLOR_LAYER)
        for vertex in bm.verts:
            vertex[layer] = cols[vertex.index]
        bm.to_mesh(mesh)
        bm.free()
        # the exporter writes the *active render* colour attribute as COLOR_0
        mesh.color_attributes.active_color = mesh.color_attributes[COLOR_LAYER]
        mesh.color_attributes.render_color_index = mesh.color_attributes.find(COLOR_LAYER)
        for material_name, _ in parts:
            mesh.materials.append(MATERIALS[material_name])
        obj = bpy.data.objects.new(name, mesh)
        bpy.context.scene.collection.objects.link(obj)
        if location is not None:
            obj.location = to_blender(location)
        self._register(obj, parent)
        self.stats[name] = sum(acc.triangles() for _, acc in parts)
        return obj

    def instance(self, name, source, location=(0.0, 0.0, 0.0), parent=None):
        """New object sharing ``source``'s mesh data (the GLB stores the geometry once)."""
        obj = bpy.data.objects.new(name, source.data)
        bpy.context.scene.collection.objects.link(obj)
        obj.location = to_blender(location)
        self._register(obj, parent)
        self.stats[name] = self.stats.get(source.name, 0)
        return obj

    def _register(self, obj, parent):
        if parent is not None:
            obj.parent = parent
        else:
            self.roots.append(obj)
        self.objects.append(obj)

    def triangles(self):
        return sum(self.stats.values())


def clear_scene():
    """Remove every object and datablock so Blender never numbers a name that is taken."""
    for obj in list(bpy.data.objects):
        bpy.data.objects.remove(obj, do_unlink=True)
    for datablocks in (bpy.data.meshes, bpy.data.materials, bpy.data.lights, bpy.data.cameras, bpy.data.worlds):
        for block in list(datablocks):
            datablocks.remove(block)
    MATERIALS.clear()
