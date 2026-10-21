"""GLB export helpers for the AuraGolf generator (MIT License, see ../LICENSE).

Canonical node names, the exporter call, the scenery merge and the read-back of the collision meshes from the
exact exported triangles (``glb_collision``: every node with the extra ``aurago_collider``, welded per surface,
plus the digest of contract V1).
"""

import hashlib
import json
import struct

import bmesh
import bpy

FUNCTIONAL_NAMES = ("flag", "ball-template", "putter-template", "water-shore")
FUNCTIONAL_PREFIXES = ("floor-", "rail-", "obstacle-", "mechanic-")
# Functional nodes that must keep their exact name (Blender numbers a taken name): the track kit's fixed names and
# prefixes and its three collision rail surfaces.
CANONICAL_NAMES = ("flag", "ball-template", "putter-template", "rail-trim", "water-shore", "rail-wood", "rail-stone", "rail-metal")
CANONICAL_PREFIXES = ("floor-", "obstacle-", "mechanic-")
COLLIDER_SURFACES = ("grass", "sand", "wood", "stone", "metal")


def canonical_export_names(objects):
    current=[]; targets={}
    for obj in objects:
        base=obj.name.split(".")[0]
        target=base if base in CANONICAL_NAMES or base.startswith(CANONICAL_PREFIXES) else None
        if target:
            current.append((obj,obj.name,target))
            targets[target]=obj
    displaced=[]
    for target,obj in targets.items():
        previous=bpy.data.objects.get(target)
        if previous and previous is not obj:
            old=previous.name; previous.name=f"__aura_export_restore_{len(displaced):02d}"
            displaced.append((previous,old))
    for obj,old,target in current:
        obj.name=target
    return current,displaced


def restore_export_names(names):
    current,displaced=names
    for obj,old,_ in current:
        obj.name=old
    for obj,old in displaced:
        obj.name=old


def glb_triangles(path):
    data=open(path,"rb").read()
    if data[:4]!=b"glTF": raise RuntimeError("Blender did not produce a GLB: "+path)
    offset=12; doc=None
    while offset < len(data):
        size,kind=struct.unpack_from("<II",data,offset); offset+=8
        chunk=data[offset:offset+size]; offset+=size
        if kind==0x4E4F534A: doc=json.loads(chunk.decode("utf-8").rstrip(" \0"))
    if doc is None: raise RuntimeError("GLB has no JSON chunk: "+path)
    return sum((doc["accessors"][p["indices"]]["count"]//3 if "indices" in p else doc["accessors"][p["attributes"]["POSITION"]]["count"]//3) for m in doc.get("meshes",[]) for p in m.get("primitives",[]) if p.get("mode",4)==4)


def _read_glb(path):
    with open(path, "rb") as f:
        data = f.read()
    if data[:4] != b"glTF":
        raise RuntimeError("Not a GLB: " + path)
    offset, doc, binary = 12, None, None
    while offset < len(data):
        size, kind = struct.unpack_from("<II", data, offset)
        offset += 8
        chunk = data[offset:offset + size]
        offset += size
        if kind == 0x4E4F534A and doc is None:
            doc = json.loads(chunk.decode("utf-8").rstrip(" \0"))
        elif kind == 0x004E4942 and binary is None:
            binary = chunk
    if doc is None or binary is None:
        raise RuntimeError("GLB is missing embedded chunks: " + path)
    return doc, binary


def _accessor_rows(doc, binary, index):
    acc = doc["accessors"][index]
    view = doc["bufferViews"][acc["bufferView"]]
    components = {"SCALAR": 1, "VEC3": 3}[acc["type"]]
    fmt = {5121: "B", 5123: "H", 5125: "I", 5126: "f"}[acc["componentType"]]
    size = struct.calcsize("<" + fmt)
    stride = view.get("byteStride", components * size)
    start = view.get("byteOffset", 0) + acc.get("byteOffset", 0)
    return [struct.unpack_from("<" + fmt * components, binary, start + i * stride) for i in range(acc["count"])]


def _apply_local(node, point):
    """One node transform with the term order of ``minigolf-collision.js`` (Float64)."""
    if "matrix" in node:
        m = node["matrix"]
        return (m[0] * point[0] + m[4] * point[1] + m[8] * point[2] + m[12],
                m[1] * point[0] + m[5] * point[1] + m[9] * point[2] + m[13],
                m[2] * point[0] + m[6] * point[1] + m[10] * point[2] + m[14])
    s, qv, t = node.get("scale", (1, 1, 1)), node.get("rotation", (0, 0, 0, 1)), node.get("translation", (0, 0, 0))
    x, y, z = (point[i] * s[i] for i in range(3))
    qx, qy, qz, qw = qv
    tx, ty, tz = 2 * (qy * z - qz * y), 2 * (qz * x - qx * z), 2 * (qx * y - qy * x)
    return (x + qw * tx + qy * tz - qz * ty + t[0], y + qw * ty + qz * tx - qx * tz + t[1], z + qw * tz + qx * ty - qy * tx + t[2])


def glb_collision(path):
    """Read the collision meshes of a track-kit GLB, weld them and compute the digest (contract V1).

    Every node with the extra ``aurago_collider`` (``grass``, ``sand``, ``wood``, ``stone``, ``metal``) is read
    with its parent chain applied. Per surface all triangles of all nodes (node order, then primitives) are
    collected; a vertex is identified by the bit patterns of its three Float32 world coordinates and numbered
    in order of first use; triangles with two equal indices after welding are dropped. The digest is the
    SHA-256 over the surfaces in the order of ``COLLIDER_SURFACES`` (empty ones left out): UTF-8 name, ``0x00``,
    Uint32 LE vertex count, Uint32 LE index count, Float32 LE vertices, Uint32 LE indices.

    Returns ``{"groups": [{surface, nodes, vertices, indices}], "nodes": n, "triangles": t, "digest": hex}``;
    ``nodes`` counts the collision nodes, ``triangles`` the welded triangles (the manifest ``collision`` entry).
    """
    doc, binary = _read_glb(path)
    nodes = doc.get("nodes", [])
    parents = {child: parent for parent, node in enumerate(nodes) for child in node.get("children", [])}
    groups = {name: {"surface": name, "nodes": 0, "keys": {}, "vertices": [], "indices": []} for name in COLLIDER_SURFACES}
    for node_index, node in enumerate(nodes):
        surface = (node.get("extras") or {}).get("aurago_collider")
        if surface is None:
            continue
        if surface not in groups:
            raise RuntimeError("Unknown collider surface %r in %s" % (surface, path))
        if "mesh" not in node:
            continue
        group = groups[surface]
        group["nodes"] += 1
        chain, current = [], node_index
        while current is not None:
            chain.append(nodes[current])
            current = parents.get(current)
        for primitive in doc["meshes"][node["mesh"]].get("primitives", []):
            if primitive.get("mode", 4) != 4:
                continue
            positions = _accessor_rows(doc, binary, primitive["attributes"]["POSITION"])
            indices = [row[0] for row in _accessor_rows(doc, binary, primitive["indices"])] if "indices" in primitive else list(range(len(positions)))
            if len(indices) % 3:
                raise RuntimeError("Exported collider primitive is not triangulated: " + str(node.get("name")))
            world = []
            for point in positions:
                for transform in chain:
                    point = _apply_local(transform, point)
                packed = struct.pack("<fff", *point)
                world.append((packed, struct.unpack("<fff", packed)))
            welded = {}

            def weld(i):
                if i not in welded:
                    key, values = world[i]
                    number = group["keys"].get(key)
                    if number is None:
                        number = group["keys"][key] = len(group["keys"])
                        group["vertices"].extend(values)
                    welded[i] = number
                return welded[i]
            for k in range(0, len(indices), 3):
                a, b, c = weld(indices[k]), weld(indices[k + 1]), weld(indices[k + 2])
                if a != b and b != c and a != c:
                    group["indices"].extend((a, b, c))
    used = [groups[name] for name in COLLIDER_SURFACES if groups[name]["indices"]]
    digest = hashlib.sha256()
    for group in used:
        digest.update(group["surface"].encode("utf-8") + b"\x00")
        digest.update(struct.pack("<II", len(group["vertices"]) // 3, len(group["indices"])))
        digest.update(struct.pack("<%df" % len(group["vertices"]), *group["vertices"]))
        digest.update(struct.pack("<%dI" % len(group["indices"]), *group["indices"]))
    return {"groups": [{k: g[k] for k in ("surface", "nodes", "vertices", "indices")} for g in used],
            "nodes": sum(g["nodes"] for g in groups.values()), "triangles": sum(len(g["indices"]) // 3 for g in used),
            "digest": digest.hexdigest()}


def export_glb(path, objects):
    """Export ``objects`` (already selected in the active scene) as one GLB."""
    names = canonical_export_names(objects)
    try:
        bpy.ops.export_scene.gltf(filepath=path, export_format="GLB", use_selection=True, use_active_scene=True,
                                  export_yup=True, export_extras=True, export_cameras=False, export_lights=False,
                                  export_materials="EXPORT", export_current_frame=True)
    finally:
        restore_export_names(names)


def glb_json(path):
    """Return the parsed JSON chunk of the GLB at ``path``."""
    with open(path, "rb") as f:
        data = f.read()
    if data[:4] != b"glTF":
        raise RuntimeError("Blender did not produce a GLB: " + path)
    offset = 12
    while offset < len(data):
        size, kind = struct.unpack_from("<II", data, offset)
        offset += 8
        if kind == 0x4E4F534A:
            return json.loads(data[offset:offset + size].decode("utf-8").rstrip(" \0"))
        offset += size
    raise RuntimeError("GLB has no JSON chunk: " + path)


def is_functional(obj):
    """Objects the runtime or the physics look up by name stay single nodes."""
    base = obj.name.split(".")[0]
    return obj.type != "MESH" or base in FUNCTIONAL_NAMES or base.startswith(FUNCTIONAL_PREFIXES)


def _split_materials(obj):
    """Split a multi-material mesh object into one object per material slot (in slot order)."""
    slots = [slot.material for slot in obj.material_slots]
    if len(slots) < 2:
        return [obj]
    parts = []
    for index, mat in enumerate(slots):
        bm = bmesh.new()
        bm.from_mesh(obj.data)
        bmesh.ops.delete(bm, geom=[f for f in bm.faces if f.material_index != index], context="FACES")
        if not bm.faces:
            bm.free()
            continue
        mesh = bpy.data.meshes.new("%s-%d" % (obj.data.name, index))
        bm.to_mesh(mesh)
        bm.free()
        mesh.materials.append(mat)
        part = bpy.data.objects.new("%s-m%d" % (obj.name, index), mesh)
        part.matrix_world = obj.matrix_world
        for coll in obj.users_collection:
            coll.objects.link(part)
        parts.append(part)
    data = obj.data
    bpy.data.objects.remove(obj, do_unlink=True)
    if data.users == 0:
        bpy.data.meshes.remove(data)
    return parts


def join_scenery(model):
    """Join every non-functional mesh of ``model`` per material into ``scenery-<material>`` nodes; returns them.

    Functional nodes (floors, rails, trims, obstacles, mechanics, templates, flag, shore) stay as
    they are. Builder bookkeeping properties are dropped.
    """
    loose = [obj for obj in model.objects if not is_functional(obj)]
    singles = []
    for obj in loose:
        singles.extend(_split_materials(obj))
    groups = {}
    for obj in singles:
        mat = obj.material_slots[0].material.name if obj.material_slots and obj.material_slots[0].material else "none"
        groups.setdefault(mat, []).append(obj)
    joined = []
    for mat, objs in sorted(groups.items()):
        target = objs[0]
        if len(objs) > 1:
            with bpy.context.temp_override(active_object=target, object=target, selected_objects=objs, selected_editable_objects=objs):
                bpy.ops.object.join()
        target.name = "scenery-" + mat.replace("kit-", "").replace(".", "_")
        target.data.name = target.name + "-mesh"
        for key in list(target.keys()):
            del target[key]
        joined.append(target)
    return joined
