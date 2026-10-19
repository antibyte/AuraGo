"""Generate AuraGo's original MIT-licensed chess piece kit in Blender 5.2 LTS.

Run from the repository root:

    blender --background --factory-startup --python assets/chess-3d/generate.py

The script writes one GLB per piece plus a manifest into ``ui/3d/chess/v1/``
and saves the editable source scene to ``assets/chess-3d/aura-chess-pieces.blend``.

Geometry is built from scratch: bodies are surfaces of revolution around the
Blender Z axis, the knight head is a swept silhouette and the rook merlons and
the king cross are extruded prisms. Everything is authored in metres with the
base of each piece centred on the origin, so the runtime can place a piece on a
board square without a per-model offset. Blender exports glTF Y-up, which turns
the Blender base plane into the glTF ``y = 0`` ground plane.
"""

import hashlib
import json
import math
import os

import bmesh
import bpy
from mathutils import Matrix, Vector

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
RUNTIME = os.path.join(ROOT, "ui", "3d", "chess", "v1")
SOURCE = os.path.join(ROOT, "assets", "chess-3d", "aura-chess-pieces.blend")
LICENSE = os.path.join(RUNTIME, "LICENSE.txt")

# A FIDE board square is 50-65 mm; 55 mm keeps the pieces and squares in
# proportion and leaves enough room between neighbouring bases.
SQUARE = 0.055
SEGMENTS = 32
MATERIAL_NAME = "chess-piece"
PIECE_NAMES = ("pawn", "knight", "bishop", "rook", "queen", "king")


# --------------------------------------------------------------------------
# mesh helpers
# --------------------------------------------------------------------------


class Profile:
    """A piece silhouette in the (radius, height) revolve plane."""

    def __init__(self):
        self.points = []
        self.hard = []

    def point(self, radius, height, hard=False):
        self.points.append((float(radius), float(height)))
        self.hard.append(bool(hard))
        return self

    def arc(self, center_r, center_z, radius, start_deg, end_deg, steps, hard=False):
        """Arc around (center_r, center_z) inside the revolve plane."""
        a0 = math.radians(start_deg)
        a1 = math.radians(end_deg)
        for index in range(steps + 1):
            angle = a0 + (a1 - a0) * (index / steps)
            self.point(
                center_r + radius * math.cos(angle),
                center_z + radius * math.sin(angle),
                hard and index in (0, steps),
            )
        return self

    def sphere(self, center_z, radius, steps=12):
        """A ball whose bottom pole sits on the current height."""
        return self.arc(0.0, center_z, radius, -90.0, 90.0, steps)


class MeshBuilder:
    """Accumulates transformed parts into a single mesh."""

    def __init__(self):
        self.verts = []
        self.faces = []
        self.smooth = []

    def add(self, verts, faces, smooth, matrix=None):
        base = len(self.verts)
        if matrix is None:
            self.verts.extend(tuple(v) for v in verts)
        else:
            self.verts.extend(tuple(matrix @ Vector(v)) for v in verts)
        for face, is_smooth in zip(faces, smooth):
            self.faces.append(tuple(index + base for index in face))
            self.smooth.append(bool(is_smooth))

    def to_object(self, name, material):
        mesh = bpy.data.meshes.new(name)
        mesh.from_pydata(self.verts, [], self.faces)
        mesh.validate(verbose=False)
        for index, polygon in enumerate(mesh.polygons):
            polygon.use_smooth = self.smooth[index] if index < len(self.smooth) else True
        mesh.update()
        obj = bpy.data.objects.new(name, mesh)
        if material is not None:
            obj.data.materials.append(material)
        bpy.context.scene.collection.objects.link(obj)
        return obj


def build_lathe(profile, segments=SEGMENTS):
    """Revolve a profile around the Z axis and return (verts, faces, smooth)."""
    verts = []
    faces = []
    smooth = []
    points = profile.points

    def make_ring(radius, height):
        base = len(verts)
        for index in range(segments):
            angle = 2.0 * math.pi * index / segments
            verts.append((radius * math.cos(angle), radius * math.sin(angle), height))
        return list(range(base, base + segments))

    def make_pole(height):
        index = len(verts)
        verts.append((0.0, 0.0, height))
        return index

    starts = []
    ends = []
    for radius, height in points:
        if radius <= 1e-7:
            pole = make_pole(height)
            starts.append([pole])
            ends.append([pole])
        else:
            ring = make_ring(radius, height)
            starts.append(ring)
            if profile.hard[len(starts) - 1]:
                ends.append(make_ring(radius, height))
            else:
                ends.append(ring)

    for index in range(len(points) - 1):
        lower = ends[index]
        upper = starts[index + 1]
        flat = abs(points[index + 1][1] - points[index][1]) < 1e-9
        if len(lower) == 1:
            pole = lower[0]
            for step in range(segments):
                nxt = (step + 1) % segments
                faces.append((pole, upper[nxt], upper[step]))
                smooth.append(not flat)
        elif len(upper) == 1:
            pole = upper[0]
            for step in range(segments):
                nxt = (step + 1) % segments
                faces.append((lower[step], lower[nxt], pole))
                smooth.append(not flat)
        else:
            for step in range(segments):
                nxt = (step + 1) % segments
                faces.append((lower[step], lower[nxt], upper[nxt], upper[step]))
                smooth.append(True)
    return verts, faces, smooth


def build_prism(polygon, z_bottom, z_top):
    """Extrude a closed 2D polygon (x, y) between two heights."""
    count = len(polygon)
    verts = [(x, y, z_bottom) for x, y in polygon] + [(x, y, z_top) for x, y in polygon]
    faces = []
    smooth = []
    faces.append(tuple(range(count - 1, -1, -1)))
    smooth.append(False)
    faces.append(tuple(range(count, count * 2)))
    smooth.append(False)
    for index in range(count):
        nxt = (index + 1) % count
        faces.append((index, nxt, count + nxt, count + index))
        smooth.append(False)
    return verts, faces, smooth


def build_knight_head(outline, half_width):
    """Carve the horse head from a side silhouette and round its edges.

    The outline is extruded along Y, the two flats are filled and a bevel
    turns the hard silhouette into a soft, carved pillow.
    """
    bm = bmesh.new()
    count = len(outline)
    front = [bm.verts.new((x, half_width, z)) for x, z in outline]
    back = [bm.verts.new((x, -half_width, z)) for x, z in outline]
    front_edges = []
    back_edges = []
    for index in range(count):
        nxt = (index + 1) % count
        front_edges.append(bm.edges.new((front[index], front[nxt])))
        back_edges.append(bm.edges.new((back[index], back[nxt])))
        bm.faces.new((front[index], front[nxt], back[nxt], back[index]))
    bmesh.ops.triangle_fill(bm, use_beauty=True, edges=front_edges)
    bmesh.ops.triangle_fill(bm, use_beauty=True, edges=back_edges)
    bmesh.ops.recalc_face_normals(bm, faces=list(bm.faces))
    mesh = bpy.data.meshes.new("knight-head")
    bm.to_mesh(mesh)
    bm.free()
    obj = bpy.data.objects.new("knight-head", mesh)
    bpy.context.scene.collection.objects.link(obj)

    bevel = obj.modifiers.new("Bevel", "BEVEL")
    bevel.width = 0.0026
    bevel.segments = 4
    bevel.limit_method = "ANGLE"
    bevel.angle_limit = math.radians(25.0)
    bevel.use_clamp_overlap = True
    bpy.context.view_layer.objects.active = obj
    bpy.ops.object.modifier_apply(modifier=bevel.name)
    for polygon in obj.data.polygons:
        polygon.use_smooth = True
    return obj


def append_object(builder, obj, matrix=None):
    mesh = obj.data
    verts = [tuple(vertex.co) for vertex in mesh.vertices]
    faces = [tuple(polygon.vertices) for polygon in mesh.polygons]
    smooth = [polygon.use_smooth for polygon in mesh.polygons]
    builder.add(verts, faces, smooth, matrix)
    bpy.data.objects.remove(obj, do_unlink=True)


# --------------------------------------------------------------------------
# piece profiles
# --------------------------------------------------------------------------


def base_profile(radius, neck_radius, height):
    """The shared low disc every piece stands on."""
    profile = Profile()
    profile.point(0.0, 0.0)
    profile.point(radius * 0.86, 0.0, True)
    profile.point(radius, 0.0013, True)
    profile.point(radius, height * 0.56, True)
    profile.point(radius * 0.86, height * 0.80, True)
    profile.point(radius * 0.60, height, True)
    profile.point(neck_radius, height * 1.30, True)
    return profile


def pawn_profile():
    profile = base_profile(0.0155, 0.0086, 0.0060)
    profile.point(0.0104, 0.0100, True)
    profile.point(0.0082, 0.0116, True)
    profile.point(0.0070, 0.0150, True)
    profile.point(0.0069, 0.0210)
    profile.point(0.0080, 0.0250, True)
    profile.point(0.0100, 0.0264, True)
    profile.point(0.0102, 0.0276, True)
    profile.sphere(0.0334, 0.0062, 12)
    return profile


def rook_profile():
    profile = base_profile(0.0175, 0.0122, 0.0062)
    profile.point(0.0132, 0.0110, True)
    profile.point(0.0138, 0.0140)
    profile.point(0.0138, 0.0480)
    profile.point(0.0146, 0.0496, True)
    profile.point(0.0164, 0.0512, True)
    profile.point(0.0164, 0.0560, True)
    profile.point(0.0152, 0.0572, True)
    profile.point(0.0140, 0.0552, True)
    profile.point(0.0140, 0.0500)
    profile.point(0.0, 0.0500, True)
    return profile


def bishop_profile():
    profile = base_profile(0.0180, 0.0110, 0.0062)
    profile.point(0.0100, 0.0092, True)
    profile.point(0.0124, 0.0128, True)
    profile.point(0.0134, 0.0180)
    profile.point(0.0120, 0.0250)
    profile.point(0.0090, 0.0320)
    profile.point(0.0072, 0.0380, True)
    profile.point(0.0078, 0.0424, True)
    profile.point(0.0050, 0.0452, True)
    profile.arc(0.0, 0.0560, 0.0126, -74.0, 62.0, 14)
    profile.point(0.0034, 0.0722, True)
    profile.sphere(0.0752, 0.0044, 10)
    return profile


def queen_profile():
    profile = base_profile(0.0190, 0.0116, 0.0064)
    profile.point(0.0126, 0.0106, True)
    profile.point(0.0150, 0.0158, True)
    profile.point(0.0172, 0.0230)
    profile.point(0.0176, 0.0290)
    profile.point(0.0160, 0.0362)
    profile.point(0.0128, 0.0430)
    profile.point(0.0104, 0.0472, True)
    profile.point(0.0116, 0.0496, True)
    profile.point(0.0094, 0.0516, True)
    profile.point(0.0110, 0.0540, True)
    profile.point(0.0132, 0.0620)
    profile.point(0.0126, 0.0652, True)
    profile.point(0.0104, 0.0646, True)
    profile.point(0.0098, 0.0560)
    profile.point(0.0, 0.0560, True)
    return profile


def king_profile():
    profile = base_profile(0.0195, 0.0112, 0.0066)
    profile.point(0.0128, 0.0110, True)
    profile.point(0.0158, 0.0176, True)
    profile.point(0.0178, 0.0250)
    profile.point(0.0176, 0.0320)
    profile.point(0.0158, 0.0392)
    profile.point(0.0126, 0.0462)
    profile.point(0.0106, 0.0518, True)
    profile.point(0.0118, 0.0546, True)
    profile.point(0.0094, 0.0570, True)
    profile.point(0.0082, 0.0600, True)
    profile.point(0.0094, 0.0624, True)
    profile.point(0.0094, 0.0690, True)
    profile.point(0.0080, 0.0708, True)
    profile.point(0.0, 0.0708, True)
    return profile


KNIGHT_OUTLINE = [
    (-0.0098, 0.0060),
    (-0.0118, 0.0180),
    (-0.0122, 0.0300),
    (-0.0100, 0.0398),
    (-0.0072, 0.0448),
    (-0.0062, 0.0530),
    (-0.0038, 0.0452),
    (-0.0004, 0.0518),
    (0.0022, 0.0440),
    (0.0074, 0.0398),
    (0.0124, 0.0330),
    (0.0152, 0.0262),
    (0.0142, 0.0206),
    (0.0098, 0.0192),
    (0.0050, 0.0216),
    (0.0000, 0.0196),
    (-0.0038, 0.0130),
    (-0.0074, 0.0064),
]


def knight_base_profile():
    profile = base_profile(0.0180, 0.0130, 0.0060)
    profile.point(0.0120, 0.0078, True)
    profile.point(0.0104, 0.0104, True)
    profile.point(0.0080, 0.0122, True)
    return profile


# --------------------------------------------------------------------------
# scene assembly
# --------------------------------------------------------------------------


def make_material():
    material = bpy.data.materials.get(MATERIAL_NAME)
    if material is None:
        material = bpy.data.materials.new(MATERIAL_NAME)
    material.use_nodes = True
    bsdf = material.node_tree.nodes.get("Principled BSDF")
    if bsdf is not None:
        bsdf.inputs["Base Color"].default_value = (0.92, 0.89, 0.83, 1.0)
        bsdf.inputs["Roughness"].default_value = 0.34
        bsdf.inputs["Metallic"].default_value = 0.0
    material.diffuse_color = (0.92, 0.89, 0.83, 1.0)
    return material


def build_piece(name, material):
    builder = MeshBuilder()
    if name == "pawn":
        verts, faces, smooth = build_lathe(pawn_profile())
        builder.add(verts, faces, smooth)
    elif name == "rook":
        verts, faces, smooth = build_lathe(rook_profile())
        builder.add(verts, faces, smooth)
        merlon = [(-0.0043, -0.0038), (0.0043, -0.0038), (0.0043, 0.0038), (-0.0043, 0.0038)]
        for index in range(4):
            angle = math.radians(45.0 + 90.0 * index)
            radius = 0.0132
            offset = (
                Matrix.Translation(Vector((radius * math.cos(angle), radius * math.sin(angle), 0.0560)))
                @ Matrix.Rotation(angle, 4, "Z")
            )
            verts, faces, smooth = build_prism(merlon, 0.0, 0.0140)
            builder.add(verts, faces, smooth, offset)
    elif name == "bishop":
        verts, faces, smooth = build_lathe(bishop_profile())
        builder.add(verts, faces, smooth)
    elif name == "queen":
        verts, faces, smooth = build_lathe(queen_profile())
        builder.add(verts, faces, smooth)
        for index in range(5):
            angle = math.radians(18.0 + 72.0 * index)
            radius = 0.0116
            ball = Profile()
            ball.sphere(0.0, 0.0038, 8)
            verts, faces, smooth = build_lathe(ball, 16)
            offset = Matrix.Translation(
                Vector((radius * math.cos(angle), radius * math.sin(angle), 0.0650))
            )
            builder.add(verts, faces, smooth, offset)
        crown = Profile()
        crown.point(0.0, 0.0)
        crown.point(0.0060, 0.0, True)
        crown.sphere(0.0060, 0.0060, 10)
        verts, faces, smooth = build_lathe(crown, 24)
        builder.add(verts, faces, smooth, Matrix.Translation(Vector((0.0, 0.0, 0.0700))))
        tip = Profile()
        tip.point(0.0, 0.0)
        tip.point(0.0034, 0.0, True)
        tip.point(0.0012, 0.0062, True)
        tip.point(0.0, 0.0074, True)
        verts, faces, smooth = build_lathe(tip, 16)
        builder.add(verts, faces, smooth, Matrix.Translation(Vector((0.0, 0.0, 0.0818))))
    elif name == "king":
        verts, faces, smooth = build_lathe(king_profile())
        builder.add(verts, faces, smooth)
        stem = [(-0.0030, -0.0028), (0.0030, -0.0028), (0.0026, 0.0028), (-0.0026, 0.0028)]
        verts, faces, smooth = build_prism(stem, 0.0690, 0.1000)
        builder.add(verts, faces, smooth)
        arm = [(-0.0092, -0.0028), (0.0092, -0.0028), (0.0092, 0.0028), (-0.0092, 0.0028)]
        verts, faces, smooth = build_prism(arm, 0.0800, 0.0856)
        builder.add(verts, faces, smooth)
    elif name == "knight":
        verts, faces, smooth = build_lathe(knight_base_profile())
        builder.add(verts, faces, smooth)
        head = build_knight_head(KNIGHT_OUTLINE, 0.0080)
        scale = Matrix.Diagonal((1.18, 1.18, 1.28, 1.0))
        turn = Matrix.Rotation(math.radians(-90.0), 4, "Z")
        append_object(builder, head, turn @ scale)
    return builder.to_object(name, material)


def clear_scene():
    for obj in list(bpy.data.objects):
        bpy.data.objects.remove(obj, do_unlink=True)
    for mesh in list(bpy.data.meshes):
        bpy.data.meshes.remove(mesh)


def export_glb(obj, path):
    bpy.ops.object.select_all(action="DESELECT")
    obj.select_set(True)
    bpy.context.view_layer.objects.active = obj
    options = {
        "filepath": path,
        "export_format": "GLB",
        "use_selection": True,
        "export_apply": True,
        "export_yup": True,
        "export_materials": "EXPORT",
        "export_normals": True,
        "export_texcoords": False,
        "export_colors": False,
        "export_cameras": False,
        "export_lights": False,
        "export_animations": False,
        "export_skins": False,
        "export_morph": False,
    }
    supported = set(bpy.ops.export_scene.gltf.get_rna_type().properties.keys())
    filtered = {key: value for key, value in options.items() if key in supported}
    bpy.ops.export_scene.gltf(**filtered)


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


LICENSE_TEXT = """AuraGo chess piece kit

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


def main():
    os.makedirs(RUNTIME, exist_ok=True)
    clear_scene()
    material = make_material()

    entries = {}
    total = 0
    for name in PIECE_NAMES:
        obj = build_piece(name, material)
        path = os.path.join(RUNTIME, name + ".glb")
        export_glb(obj, path)
        size = os.path.getsize(path)
        total += size
        height = 0.0
        radius = 0.0
        for vertex in obj.data.vertices:
            height = max(height, vertex.co.z)
            radius = max(radius, math.hypot(vertex.co.x, vertex.co.y))
        entries[name] = {
            "file": name + ".glb",
            "bytes": size,
            "sha256": sha256(path),
            "height": round(height, 5),
            "radius": round(radius, 5),
        }
        print("exported %s (%d bytes, h=%.4f, r=%.4f)" % (name, size, height, radius))

    with open(LICENSE, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(LICENSE_TEXT)

    manifest = {
        "generator": "Blender %s / assets/chess-3d/generate.py" % bpy.app.version_string,
        "units": "metres",
        "up_axis": "Y",
        "square_size": SQUARE,
        "license": "MIT",
        "pieces": entries,
        "budget_bytes": 1024 * 1024,
    }
    with open(
        os.path.join(RUNTIME, "manifest.json"), "w", encoding="utf-8", newline="\n"
    ) as handle:
        handle.write(json.dumps(manifest, indent=2, sort_keys=True) + "\n")

    os.makedirs(os.path.dirname(SOURCE), exist_ok=True)
    bpy.ops.wm.save_as_mainfile(filepath=SOURCE)
    print("wrote %s (%d bytes total)" % (RUNTIME, total))


if __name__ == "__main__":
    main()
