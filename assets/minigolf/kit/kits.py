"""Per-world prop kits (MIT License).

Builds every registered prop of a world, bakes AO per prop on a temporary
ground plate (contact shading at the foot), exports ``props-<look>.glb`` with one
top-level node per prop and returns the manifest metadata.
"""

import hashlib
import importlib
import math
import os

import bpy
from mathutils import Matrix

from . import bake, export, shapes

MODULES = {"pirate": "kit.props_pirate", "temple": "kit.props_temple", "moon": "kit.props_moon"}
SPACING = 40.0
MAX_PRIMITIVES = 3
HERO_PRIMITIVES = 4  # a hero prop is the one place worth a fourth draw call (metal fittings next to wood, cloth and light)
MAX_MATERIALS = 8


def _measure(obj):
    radius = max((math.hypot(v.co.x, v.co.y) for v in obj.data.vertices), default=0.0)
    height = max((v.co.z for v in obj.data.vertices), default=0.0)
    return round(radius, 3), round(height, 3)


def _hull(pts):
    """Convex hull of sorted 2D points (monotone chain)."""
    if len(pts) < 3:
        return pts

    def half(seq):
        out = []
        for p in seq:
            while len(out) >= 2 and (out[-1][0] - out[-2][0]) * (p[1] - out[-2][1]) - (out[-1][1] - out[-2][1]) * (p[0] - out[-2][0]) <= 0:
                out.pop()
            out.append(p)
        return out

    lower, upper = half(pts), half(reversed(pts))
    return lower[:-1] + upper[:-1]


def _enclosing_circle(points):
    """Smallest circle around 2D ``points`` as ``(cx, cy, r)`` (Welzl's algorithm, deterministic input order)."""
    pts = _hull(sorted(set((round(x, 6), round(y, 6)) for x, y in points)))

    def circle(boundary):
        if not boundary:
            return (0.0, 0.0, -1.0)
        if len(boundary) == 1:
            return (boundary[0][0], boundary[0][1], 0.0)
        if len(boundary) == 2:
            (ax, ay), (bx, by) = boundary
            return ((ax + bx) / 2, (ay + by) / 2, math.hypot(ax - bx, ay - by) / 2)
        (ax, ay), (bx, by), (cx, cy) = boundary
        d = 2 * (ax * (by - cy) + bx * (cy - ay) + cx * (ay - by))
        if abs(d) < 1e-12:
            far = max(((p, q) for p in boundary for q in boundary), key=lambda pq: math.hypot(pq[0][0] - pq[1][0], pq[0][1] - pq[1][1]))
            return circle(list(far))
        ux = ((ax * ax + ay * ay) * (by - cy) + (bx * bx + by * by) * (cy - ay) + (cx * cx + cy * cy) * (ay - by)) / d
        uy = ((ax * ax + ay * ay) * (cx - bx) + (bx * bx + by * by) * (ax - cx) + (cx * cx + cy * cy) * (bx - ax)) / d
        return (ux, uy, math.hypot(ax - ux, ay - uy))

    def inside(c, p):
        return math.hypot(p[0] - c[0], p[1] - c[1]) <= c[2] + 1e-9

    def welzl(points, boundary):
        c = circle(boundary)
        if len(boundary) == 3:
            return c
        for i, p in enumerate(points):
            if not inside(c, p):
                c = welzl(points[:i], boundary + [p])
        return c

    return welzl(pts, [])


def centre_footprint(obj):
    """Move a prop so the centre of its smallest enclosing plan circle is the origin (the radius is then as small as possible)."""
    cx, cy, _ = _enclosing_circle([(v.co.x, v.co.y) for v in obj.data.vertices])
    obj.data.transform(Matrix.Translation((-cx, -cy, 0.0)))


def _strip(obj):
    for key in [k for k in obj.keys() if not k.startswith("aurago_")]:
        del obj[key]


def primitive_limit(tags):
    return HERO_PRIMITIVES if "hero" in tags else MAX_PRIMITIVES


def _check_primitives(path, look, meta):
    """Fail when an exported prop node carries more primitives than the contract allows."""
    doc = export.glb_json(path)
    for node in doc.get("nodes", []):
        if "mesh" not in node:
            continue
        count = len(doc["meshes"][node["mesh"]]["primitives"])
        info = meta.get(node.get("name", "")[len("prop-%s-" % look):], {})
        limit = primitive_limit(info.get("tags", []))
        if count > limit:
            raise RuntimeError("%s (%s kit) exports %d primitives (max %d)" % (node.get("name"), look, count, limit))
    return len(doc.get("materials", []))


def build_kit(look, out_dir, samples=160):
    module = importlib.import_module(MODULES[look])
    collection = bpy.data.collections.new("kit-" + look)
    bpy.context.scene.collection.children.link(collection)
    shapes.set_collection(collection)
    objs, plates = [], []
    for index, name in enumerate(sorted(module.PROPS)):
        obj = module.PROPS[name](seed=1)
        if "hero" in module.PROP_META.get(name, {}).get("tags", []):
            centre_footprint(obj)
        x, y = (index % 6) * SPACING, (index // 6) * SPACING
        obj.location = (x, y, 0.0)
        plates.append(shapes.make_object("bake-plate-" + name, [(x - 8, y - 8, 0), (x + 8, y - 8, 0), (x + 8, y + 8, 0), (x - 8, y + 8, 0)], [(0, 1, 2, 3)], smooth=False))
        objs.append(obj)
    bpy.context.view_layer.update()
    bake.process(objs, samples=samples)
    for plate in plates:
        bpy.data.objects.remove(plate, do_unlink=True)
    meta, materials = {}, set()
    for obj in objs:
        obj.location = (0.0, 0.0, 0.0)
        name = obj.name[len("prop-%s-" % look):]
        slots = [slot.material for slot in obj.material_slots if slot.material]
        info = module.PROP_META.get(name, {})
        limit = primitive_limit(info.get("tags", []))
        if len(slots) > limit:
            raise RuntimeError("%s uses %d materials (max %d)" % (obj.name, len(slots), limit))
        materials.update(m.name for m in slots)
        radius, height = _measure(obj)
        extras = {"radius": radius, "height": height, "tags": sorted(info.get("tags", []))}
        for key in ("spin", "clearance"):
            if key in info:
                extras[key] = info[key]
        _strip(obj)
        obj["aurago_prop"] = extras
        meta[name] = dict(extras, triangles=shapes.tri_count(obj))
    if len(materials) > MAX_MATERIALS:
        raise RuntimeError("%s kit uses %d materials (max %d)" % (look, len(materials), MAX_MATERIALS))
    bpy.ops.object.select_all(action="DESELECT")
    for obj in objs:
        obj.select_set(True)
    bpy.context.view_layer.objects.active = objs[0]
    path = os.path.join(out_dir, "props-%s.glb" % look)
    export.export_glb(path, objs)
    exported_materials = _check_primitives(path, look, meta)
    data = open(path, "rb").read()
    entry = {"model": os.path.basename(path), "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest(),
             "triangles": export.glb_triangles(path), "props": meta}
    print("kit %s: %d bytes, %d triangles, %d props, %d materials" % (look, len(data), entry["triangles"], len(meta), exported_materials))
    return entry, objs
