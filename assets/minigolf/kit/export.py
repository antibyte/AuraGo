"""GLB export helpers for the AuraGolf generator (MIT License, see ../LICENSE).

Canonical node names, the exporter call, bounds and the read-back of fixed
collision meshes from the exact exported triangles. Moved verbatim from the
monolithic ``generate.py``.
"""

import json
import struct

import bmesh
import bpy
from mathutils import Vector

FUNCTIONAL_NAMES = ("flag", "ball-template", "putter-template", "water-shore")
FUNCTIONAL_PREFIXES = ("surface-", "rail-", "obstacle-", "windmill-sail")


def scene_bounds(objects, obstacles):
    from .courses_v1 import q, runtime  # lazy: courses_v1 imports this module
    points=[]
    for obj in objects:
        if obj.type!="MESH": continue
        for corner in obj.bound_box:
            p=obj.matrix_world @ Vector(corner); points.append(runtime(p))
    if not points:
        raise RuntimeError("course has no mesh geometry")
    lo=[min(p[i] for p in points) for i in range(3)]; hi=[max(p[i] for p in points) for i in range(3)]
    for obstacle in obstacles:
        ext=[abs(obstacle["axis"][i])*obstacle["amplitude"] for i in range(3)]
        for i in range(3): lo[i]=min(lo[i],obstacle["position"][i]-obstacle["halfExtents"][i]-ext[i]); hi[i]=max(hi[i],obstacle["position"][i]+obstacle["halfExtents"][i]+ext[i])
    return {"min":q(lo),"max":q(hi)}


def canonical_export_names(objects):
    required={"surface-grass","surface-sand","rail-left","rail-right","flag","ball-template","putter-template","rail-trim-left","rail-trim-right","water-shore"}
    current=[]; targets={}
    for obj in objects:
        base=obj.name.split(".")[0]
        target=obj.get("aurago_obstacle_node") or (base if base in required else None)
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


def glb_fixed_colliders(path, layout):
    """Read fixed collision meshes back from the exact exported GLB triangles.

    Exporter triangulation, float32 rounding, and node transforms can all differ
    slightly from source mesh data. Sourcing physics from the runtime artifact
    guarantees collider vertices and indices match what the viewer displays.
    """
    data=open(path,"rb").read(); offset=12; doc=None; binary=None
    while offset<len(data):
        size,kind=struct.unpack_from("<II",data,offset); offset+=8
        chunk=data[offset:offset+size]; offset+=size
        if kind==0x4E4F534A: doc=json.loads(chunk.decode("utf-8").rstrip(" \0"))
        elif kind==0x004E4942: binary=chunk
    if doc is None or binary is None: raise RuntimeError("GLB is missing embedded chunks: "+path)
    parents={child:parent for parent,node in enumerate(doc.get("nodes",[])) for child in node.get("children",[])}
    def accessor(index):
        acc=doc["accessors"][index]; view=doc["bufferViews"][acc["bufferView"]]
        kind=acc["type"]; components={"SCALAR":1,"VEC3":3}.get(kind)
        if components is None: raise RuntimeError("Unexpected collider accessor type: "+kind)
        component=acc["componentType"]
        fmt={5121:"B",5123:"H",5125:"I",5126:"f"}.get(component)
        if fmt is None: raise RuntimeError("Unexpected GLB accessor component type: "+str(component))
        size=struct.calcsize("<"+fmt); stride=view.get("byteStride",components*size)
        start=view.get("byteOffset",0)+acc.get("byteOffset",0)
        return [struct.unpack_from("<"+fmt*components,binary,start+i*stride) for i in range(acc["count"])]
    def local_transform(node,point):
        if "matrix" in node:
            m=node["matrix"]
            return (m[0]*point[0]+m[4]*point[1]+m[8]*point[2]+m[12],m[1]*point[0]+m[5]*point[1]+m[9]*point[2]+m[13],m[2]*point[0]+m[6]*point[1]+m[10]*point[2]+m[14])
        s=node.get("scale",(1,1,1)); qv=node.get("rotation",(0,0,0,1)); t=node.get("translation",(0,0,0))
        x,y,z=(point[i]*s[i] for i in range(3)); qx,qy,qz,qw=qv
        tx=2*(qy*z-qz*y); ty=2*(qz*x-qx*z); tz=2*(qx*y-qy*x)
        return (x+qw*tx+qy*tz-qz*ty+t[0],y+qw*ty+qz*tx-qx*tz+t[1],z+qw*tz+qx*ty-qy*tx+t[2])
    def world_point(node_index,point):
        current=node_index; value=point
        while current is not None:
            value=local_transform(doc["nodes"][current],value); current=parents.get(current)
        return value
    names={"surface-grass":"grass","surface-sand":"sand","rail-left":"wood","rail-right":"wood"}
    # Crystal banks use the blue stone palette and matching stone friction.
    # Material metadata from GLB is not needed to infer this authored surface.
    if layout=="crystals": names["rail-left"]=names["rail-right"]="stone"
    colliders=[]
    for node_index,node in enumerate(doc.get("nodes",[])):
        surface=names.get(node.get("name"))
        if surface is None or "mesh" not in node: continue
        for primitive in doc["meshes"][node["mesh"]].get("primitives",[]):
            if primitive.get("mode",4)!=4: continue
            positions=[world_point(node_index,p) for p in accessor(primitive["attributes"]["POSITION"])]
            indices=[p[0] for p in accessor(primitive["indices"])] if "indices" in primitive else list(range(len(positions)))
            if len(indices)%3: raise RuntimeError("Exported collider primitive is not triangulated: "+node["name"])
            # JSON's shortest-roundtrip representation preserves the unpacked
            # float32 value exactly when the runtime parses the manifest.
            colliders.append({"type":"mesh","vertices":[float(v) for p in positions for v in p],"indices":indices,"surface":surface})
    if not colliders: raise RuntimeError("No fixed collision meshes found in GLB: "+path)
    return colliders


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
    return (obj.type != "MESH" or base in FUNCTIONAL_NAMES or base.startswith(FUNCTIONAL_PREFIXES)
            or bool(obj.get("aurago_obstacle")) or bool(obj.get("aurago_obstacle_component")))


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

    Functional nodes (lane, rails, trims, obstacles, templates, flag, shore, sun
    wheel blades) stay as they are. Builder bookkeeping properties are dropped.
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
