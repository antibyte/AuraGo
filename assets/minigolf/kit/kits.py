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

from . import bake, export, shapes

MODULES = {"pirate": "kit.props_pirate", "temple": "kit.props_temple", "moon": "kit.props_moon"}
SPACING = 40.0
MAX_PRIMITIVES = 3
MAX_MATERIALS = 8


def _measure(obj):
    radius = max((math.hypot(v.co.x, v.co.y) for v in obj.data.vertices), default=0.0)
    height = max((v.co.z for v in obj.data.vertices), default=0.0)
    return round(radius, 3), round(height, 3)


def _strip(obj):
    for key in [k for k in obj.keys() if not k.startswith("aurago_")]:
        del obj[key]


def _check_primitives(path, look):
    """Fail when an exported prop node carries more primitives than the contract allows."""
    doc = export.glb_json(path)
    for node in doc.get("nodes", []):
        if "mesh" not in node:
            continue
        count = len(doc["meshes"][node["mesh"]]["primitives"])
        if count > MAX_PRIMITIVES:
            raise RuntimeError("%s (%s kit) exports %d primitives (max %d)" % (node.get("name"), look, count, MAX_PRIMITIVES))
    return len(doc.get("materials", []))


def build_kit(look, out_dir, samples=160):
    module = importlib.import_module(MODULES[look])
    collection = bpy.data.collections.new("kit-" + look)
    bpy.context.scene.collection.children.link(collection)
    shapes.set_collection(collection)
    objs, plates = [], []
    for index, name in enumerate(sorted(module.PROPS)):
        obj = module.PROPS[name](seed=1)
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
        if len(slots) > MAX_PRIMITIVES:
            raise RuntimeError("%s uses %d materials (max %d)" % (obj.name, len(slots), MAX_PRIMITIVES))
        materials.update(m.name for m in slots)
        info = module.PROP_META.get(name, {})
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
    exported_materials = _check_primitives(path, look)
    data = open(path, "rb").read()
    entry = {"model": os.path.basename(path), "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest(),
             "triangles": export.glb_triangles(path), "props": meta}
    print("kit %s: %d bytes, %d triangles, %d props, %d materials" % (look, len(data), entry["triangles"], len(meta), exported_materials))
    return entry, objs
