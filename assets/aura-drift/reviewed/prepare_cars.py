"""Recover UV artwork and closed wheel wells from retained NC AI car originals (MIT)."""
import argparse
import gzip
import hashlib
import json
import math
from pathlib import Path
import sys
import subprocess
import tempfile

import bmesh
import bpy
import numpy as np
from mathutils import Vector
from mathutils.bvhtree import BVHTree

HERE = Path(__file__).resolve().parent
CONFIG = {
    "vektor": (.277, -.295, .108, .35), "kobalt": (.295, -.295, .107, .34),
    "lumen": (.28, -.31, .112, .35), "tundra": (.32, -.28, .124, .36),
    "maehkoenig": (.17, -.30, .125, .34), "couch": (.3075, -.3506, .14, .36),
    "bertha": (.28, -.28, .235, .43),
    "softeis": (.290167, -.308039, .1272108, .34),
}


def frame(p, car):
    return Vector((p.x, p.z, -p.y) if car in ("maehkoenig", "bertha") else (-p.y, p.z, -p.x))


def native(p):
    return (p.x, p.z, -p.y)


def material(name, image=None):
    mat = bpy.data.materials.new(name)
    mat.use_nodes = True
    shader = mat.node_tree.nodes.get("Principled BSDF")
    shader.inputs["Base Color"].default_value = (.035, .04, .05, 1)
    if image:
        node = mat.node_tree.nodes.new("ShaderNodeTexImage")
        node.image = image
        mat.node_tree.links.new(node.outputs["Color"], shader.inputs["Base Color"])
    return mat


def paint_atlas(image, car):
    """Alpha is a paint mask, never transparency; RGB keeps unpainted texture detail."""
    image.scale(1024, 1024)
    rgba = np.empty(1024 * 1024 * 4, dtype=np.float32)
    image.pixels.foreach_get(rgba)
    rgba = rgba.reshape((-1, 4))
    rgb = rgba[:, :3]
    hi, lo = rgb.max(axis=1), rgb.min(axis=1)
    delta = hi - lo
    hue = np.zeros_like(hi)
    np.divide(rgb[:, 1] - rgb[:, 2], delta, out=hue, where=delta > 0)
    green = (hi == rgb[:, 1]) & (delta > 0)
    blue = (hi == rgb[:, 2]) & (delta > 0)
    hue[green] = 2 + (rgb[green, 2] - rgb[green, 0]) / delta[green]
    hue[blue] = 4 + (rgb[blue, 0] - rgb[blue, 1]) / delta[blue]
    hue = (hue / 6) % 1
    sat = np.divide(delta, hi, out=np.zeros_like(hi), where=hi > 0)
    ranges = {"vektor": (-.04, .065), "kobalt": (.52, .70), "lumen": (.035, .15),
              "tundra": (.20, .49), "maehkoenig": (.14, .43), "couch": (.85, .99),
              "bertha": (-.04, .055), "softeis": (.78, 1.0)}
    a, b = ranges[car]
    in_hue = ((hue > a) & (hue < b)) if a >= 0 else ((hue > 1 + a) | (hue < b))
    threshold = {"tundra": .15, "softeis": .04}.get(car, .3)
    mask = in_hue & (sat > threshold)
    rgb[mask] = hi[mask, None]
    # Keep RGB decodable through browser premultiplication: 0.5 means unpainted, 1 means paint.
    rgba[:, 3] = .5 + .5 * mask
    output = bpy.data.images.new("car-" + car + "-paint", width=1024, height=1024, alpha=True)
    output.alpha_mode = "CHANNEL_PACKED"
    output.pixels.foreach_set(rgba.ravel())
    destination = HERE / ("car-" + car + ".webp")
    with tempfile.TemporaryDirectory(prefix="drift-atlas-") as scratch:
        output.filepath_raw = str(Path(scratch) / "atlas.png")
        output.file_format = "PNG"
        output.save()
        subprocess.run(["ffmpeg", "-v", "error", "-y", "-i", output.filepath_raw,
            "-c:v", "libwebp", "-quality", "88", "-compression_level", "6", str(destination)], check=True)
    assert mask.any() and not mask.all(), car
    return bpy.data.images.load(str(destination), check_existing=False)


def boolean(obj, cutter, operation="DIFFERENCE"):
    bpy.context.view_layer.objects.active = obj
    mod = obj.modifiers.new("Closed joint cut", "BOOLEAN")
    mod.operation, mod.solver, mod.object = operation, "EXACT", cutter
    bpy.ops.object.modifier_apply(modifier=mod.name)
    bpy.data.objects.remove(cutter, do_unlink=True)


def clean_fragments(obj, radius):
    """Keep the body/interior; discard axle scraps severed by wheel removal."""
    bm = bmesh.new()
    bm.from_mesh(obj.data)
    bmesh.ops.remove_doubles(bm, verts=list(bm.verts), dist=.0000001)
    unseen = set(bm.verts)
    groups = []
    while unseen:
        todo = [unseen.pop()]
        group = []
        while todo:
            v = todo.pop()
            group.append(v)
            for edge in v.link_edges:
                other = edge.other_vert(v)
                if other in unseen:
                    unseen.remove(other)
                    todo.append(other)
        groups.append(group)
    largest = max(groups, key=len)
    discard = [v for g in groups if g is not largest and (max(v.co.z for v in g) < radius * 1.85 or len(g) < 16) for v in g]
    bmesh.ops.delete(bm, geom=discard, context="VERTS")
    bmesh.ops.recalc_face_normals(bm, faces=list(bm.faces))
    bm.to_mesh(obj.data)
    bm.free()
    return len(discard)


def cut_box(obj, low, high, operation):
    center = [(a + b) / 2 for a, b in zip(low, high)]
    bpy.ops.mesh.primitive_cube_add(size=1, location=(center[0], -center[2], center[1]))
    cutter = bpy.context.object
    cutter.scale = (high[0] - low[0], high[2] - low[2], high[1] - low[1])
    for mat in obj.data.materials: cutter.data.materials.append(mat)
    for face in cutter.data.polygons: face.material_index = 1
    boolean(obj, cutter, operation)


def close_mesh(obj):
    bm = bmesh.new(); bm.from_mesh(obj.data)
    bmesh.ops.remove_doubles(bm, verts=list(bm.verts), dist=.0000001)
    bmesh.ops.dissolve_degenerate(bm, edges=list(bm.edges), dist=.00000001)
    caps = bmesh.ops.holes_fill(bm, edges=[e for e in bm.edges if e.is_boundary], sides=0).get("faces", [])
    for face in caps: face.material_index = 1
    open_edges = sum(e.is_boundary for e in bm.edges)
    bmesh.ops.recalc_face_normals(bm, faces=list(bm.faces))
    bm.to_mesh(obj.data); bm.free()
    return open_edges


def geometry(obj, length, pivot=(0, 0, 0)):
    me = obj.data
    me.calc_loop_triangles()
    parts, indices = {}, {}
    uv = me.uv_layers.active
    for tri in me.loop_triangles:
        name = "trim" if tri.material_index == 1 else "paint"
        part = parts.setdefault(name, {"verts": [], "faces": [], "cols": [], "uvs": []})
        index = indices.setdefault(name, {})
        face = []
        for vi, li in zip(tri.vertices, tri.loops):
            p = native(me.vertices[vi].co)
            pos = tuple(round(p[i] * length - pivot[i], 8) for i in range(3))
            texcoord = tuple(round(v, 7) for v in uv.data[li].uv)
            key = pos
            if key not in index:
                index[key] = len(part["verts"])
                part["verts"].append(pos)
                part["cols"].append([1, 1, 1, 1])
            face.append(index[key])
            part["uvs"].append(texcoord)
        part["faces"].append(face)
    return parts


def prepare(car, source_folder):
    source = source_folder / ("car-" + car) / "model.glb"
    sources = json.loads((HERE / "sources.json").read_text())["assets"]
    expected = next(s["model_sha256"] for s in sources if s["id"] == "car-" + car)
    assert hashlib.sha256(source.read_bytes()).hexdigest() == expected, source
    path = HERE / ("car-" + car + ".json.gz")
    if path.exists():
        old = json.loads(gzip.decompress(path.read_bytes()))
    else:
        original = json.loads(gzip.decompress((HERE.parent / "kit/softeis_mesh.json.gz").read_bytes()))
        old = {"nodes": original["nodes"], "anchors": original["anchors"], "wheels": {
            "positions": original["wheels"], "radius": original["wheels"]["wheel_fr"][1],
            "width": .56, "hub": "#ffd1e3", "spokes": 6}}
    bpy.ops.object.select_all(action="SELECT")
    bpy.ops.object.delete(use_global=False)
    bpy.ops.import_scene.gltf(filepath=str(source))
    obj = next(o for o in bpy.context.scene.objects if o.type == "MESH")
    bsdf = obj.data.materials[0].node_tree.nodes.get("Principled BSDF")
    image = bsdf.inputs["Base Color"].links[0].from_node.image
    atlas = paint_atlas(image, car)
    points = [frame(obj.matrix_world @ v.co, car) for v in obj.data.vertices]
    low = Vector([min(p[i] for p in points) for i in range(3)])
    high = Vector([max(p[i] for p in points) for i in range(3)])
    size, center = high.x - low.x, (high + low) / 2
    for v, p in zip(obj.data.vertices, points):
        p = Vector(((p.x - center.x) / size, (p.y - low.y) / size, (p.z - center.z) / size))
        v.co = (p.x, -p.z, p.y)
    obj.matrix_world.identity()
    obj.data.materials.clear()
    obj.data.materials.append(material("paint", atlas))
    obj.data.materials.append(material("trim"))
    for poly in obj.data.polygons:
        poly.material_index = 0
    bm = bmesh.new(); bm.from_mesh(obj.data)
    bmesh.ops.remove_doubles(bm, verts=list(bm.verts), dist=.0000001)
    open_before = sum(e.is_boundary for e in bm.edges)
    print("SOURCE_BOUNDARY", car, open_before, flush=True)
    bm.to_mesh(obj.data); bm.free()
    front, rear, radius, track_ratio = CONFIG[car]
    track = (high.z - low.z) / size * track_ratio
    length = old.get("source_length", old["wheels"]["radius"] / radius)
    fits = json.loads((HERE / "wheels.json").read_text()).get(car)
    if fits:
        assert fits["source_sha256"] == expected, car
        old["wheels"]["axles"] = {axle: {key: fits[axle][key] * length for key in ("radius", "width")} for axle in ("f", "r")}
        old["wheels"].update(old["wheels"]["axles"]["f"])
        old["wheels"]["positions"] = {"wheel_" + axle + side:
            [fits[axle]["x"] * length, fits[axle]["radius"] * length, sign * fits[axle]["track"] * length]
            for axle in ("f", "r") for side, sign in (("l", -1), ("r", 1))}
        old["source_length"] = length
    # The sofa has a narrower front axle; the overall bounds include its side table.
    tracks = {"f": .29, "r": .425} if car == "couch" else {"f": track, "r": track}
    if car == "couch" and not fits:
        old["wheels"]["width"] = length * .10
        old["wheels"]["positions"] = {"wheel_" + axle + side: [x * length, radius * length, sign * tracks[axle] * length]
            for axle, x in (("f", front), ("r", rear)) for side, sign in (("l", -1), ("r", 1))}
    for axle, x in (("f", front), ("r", rear)):
        fit = fits[axle] if fits else None
        x, y = (fit["x"], fit["radius"]) if fit else (x, radius)
        cut_x, cut_y = (fit["cut_x"], fit["cut_y"]) if fit else (radius * 1.045, radius * 1.045)
        for sign in (-1, 1):
            if fit:
                inset = math.hypot(fit["radius"], fit["width"] * .5) * 1.06 if axle == "f" else fit["width"] * .75
                inner = fit["track"] - inset
            else:
                inner = tracks[axle] * .50
            inner = max(.025, inner)
            outer = (high.z - low.z) / size * .5 + .1
            bpy.ops.mesh.primitive_cylinder_add(vertices=64, radius=1, depth=outer - inner,
                location=(x, -sign * (outer + inner) / 2, y), rotation=(math.pi / 2, 0, 0))
            cutter = bpy.context.object
            cutter.scale.x, cutter.scale.y = cut_x, cut_y
            for mat in obj.data.materials: cutter.data.materials.append(mat)
            for poly in cutter.data.polygons: poly.material_index = 1
            boolean(obj, cutter)
            if fit:
                # A wheel arch opens downward; a circular hole leaves the AI tyre's lower corners.
                zlow, zhigh = sorted((sign * inner, sign * outer))
                cut_box(obj, (x-cut_x, -.1, zlow), (x+cut_x, y+cut_y*.05, zhigh), "DIFFERENCE")
    if car == "maehkoenig":
        cut_box(obj, (.32, .11, -.10), (.48, .17, .10), "DIFFERENCE")
    removed = clean_fragments(obj, radius)
    joints = {
        "maehkoenig": {"gnome": ((-.25, .34, -.22), (.13, 1.5, .22))},
        "couch": {"lamp": ((-1, .65, -1), (-.24, 1.5, 1)),
                  "cushion": ((-.29, .38, .10), (-.035, .59, .32))},
        "softeis": {"cone": ((-1, .64, -1), (.06, 1.5, 1))},
    }.get(car, {})
    nodes, open_after = [], 0
    for name, (a, b) in joints.items():
        part = obj.copy(); part.data = obj.data.copy()
        bpy.context.collection.objects.link(part)
        cut_box(part, a, b, "INTERSECT")
        cut_box(obj, a, b, "DIFFERENCE")
        open_after += close_mesh(part)
        pivot = next(n["location"] for n in old["nodes"] if n["name"] == name)
        nodes.append({"name": name, "location": pivot, "parts": geometry(part, length, pivot)})
        assert nodes[-1]["parts"].get("paint", {}).get("faces"), (car, name)
    open_after += close_mesh(obj)
    assert open_after <= open_before, (car, "New open cut edges", open_before, open_after)
    body = {"name": "body", "parts": geometry(obj, length)}
    # Existing anchors retain functional lights; small closed lamps sit on the surface.
    sys.path.insert(0, str(HERE.parent))
    from kit.core import Accum
    from kit.shapes import box, strut, xf
    axles = Accum()
    for axle in ("f", "r"):
        x, y, z = old["wheels"]["positions"]["wheel_" + axle + "r"]
        axle_radius = old["wheels"].get("axles", {}).get(axle, {}).get("radius", old["wheels"]["radius"])
        axles.add(strut((x, y, -z), (x, y, z), axle_radius * .10, 8))
        for side in (-1, 1):
            a = (x, y, side * z * .45)
            b = (x, y + axle_radius * 1.2, side * z * .45)
            axles.add(strut(a, b, axle_radius * .06, 6))
    trim = body["parts"]["trim"]
    offset = len(trim["verts"])
    trim["verts"].extend(axles.verts)
    trim["faces"].extend([[i + offset for i in face] for face in axles.faces])
    trim["cols"].extend([[1, 1, 1, 1]] * len(axles.verts))
    trim["uvs"].extend([[0, 0]] * sum(map(len, axles.faces)))
    surface = BVHTree.FromObject(obj, bpy.context.evaluated_depsgraph_get())
    for role, prefix in (("head_light", "anchor_head"), ("brake_light", "anchor_tail")):
        acc = Accum()
        for name, pos in old["anchors"].items():
            if name.startswith(prefix):
                if car == "bertha":
                    sign = 1 if prefix == "anchor_head" else -1
                    start = Vector((sign, -.155 if name.endswith("_r") else .155, .47))
                    nearest = surface.ray_cast(start, Vector((-sign, 0, 0)))[0]
                else:
                    nearest = surface.find_nearest(Vector((pos[0], -pos[2], pos[1])) / length)[0]
                assert nearest is not None, name
                pos[:] = [v * length for v in native(nearest)]
                acc.add(box(length * .008, length * .018, length * .035), xf(pos))
        body["parts"][role] = {"verts": acc.verts, "faces": acc.faces, "cols": [[1, 1, 1, 1]] * len(acc.verts)}
    old["nodes"] = [body] + nodes
    old["texture"] = "car-" + car + ".webp"
    old["texture_sha256"] = hashlib.sha256((HERE / old["texture"]).read_bytes()).hexdigest()
    old["preparation"] = {"closed_wheel_cuts": 4, "discarded_vertices": removed, "texture_size": 1024,
                          "source_boundary_edges": open_before, "boundary_edges": open_after}
    temporary = path.with_name(path.name + ".tmp")
    try:
        temporary.write_bytes(gzip.compress(json.dumps(old, sort_keys=True, separators=(",", ":")).encode(), mtime=0))
        temporary.replace(path)
    finally:
        temporary.unlink(missing_ok=True)
    print("REPAIRED", car, len(obj.data.polygons), "discarded", removed, flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sources", type=Path, required=True)
    parser.add_argument("--only", default=",".join(CONFIG))
    args = parser.parse_args(sys.argv[sys.argv.index("--") + 1:])
    for car in args.only.split(","): prepare(car, args.sources)
