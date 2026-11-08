"""Aura Drift kit export: GLB writer, file facts and EEVEE previews (MIT, AuraGo contributors)."""

import hashlib
import json
import math
import os
import struct
import tempfile

import bpy
from mathutils import Vector

from .core import to_blender


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def export_glb(model, path):
    """Export the model's objects (selection only) with the pinned option set."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
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
        "export_image_format": "AUTO",
        "export_normals": True,
        "export_texcoords": True,
        "export_tangents": False,
        "export_vertex_color": "ACTIVE",
        "export_all_vertex_colors": False,
        "export_attributes": False,
        "export_extras": False,
        "export_cameras": False,
        "export_lights": False,
        "export_animations": False,
        "export_skins": False,
        "export_morph": False,
    }
    supported = set(bpy.ops.export_scene.gltf.get_rna_type().properties.keys())
    # Readers can keep the old model open on Windows; never truncate the live file.
    with tempfile.TemporaryDirectory(prefix=".aura-drift-", dir=os.path.dirname(path)) as staging:
        options["filepath"] = os.path.join(staging, os.path.basename(path))
        bpy.ops.export_scene.gltf(**{key: value for key, value in options.items() if key in supported})
        os.replace(options["filepath"], path)


def read_glb(path):
    with open(path, "rb") as handle:
        data = handle.read()
    length = struct.unpack("<I", data[12:16])[0]
    return json.loads(data[20 : 20 + length].decode("utf8").rstrip("\x00 "))


def glb_triangles(path):
    """Triangles placed in the scene graph (a mesh referenced twice counts twice)."""
    doc = read_glb(path)
    total = 0
    for node in doc.get("nodes", []):
        if "mesh" in node:
            total += mesh_triangles(doc, doc["meshes"][node["mesh"]])
    return total


def mesh_triangles(doc, mesh):
    total = 0
    for primitive in mesh["primitives"]:
        accessor = primitive.get("indices", primitive["attributes"]["POSITION"])
        total += doc["accessors"][accessor]["count"] // 3
    return total


# --------------------------------------------------------------------------
# previews (EEVEE, always written outside the repository)
# --------------------------------------------------------------------------

CAR_VIEWS = [
    ("hero", 36.0, 17.0, 1.0),
    ("chase", 184.0, 27.0, 1.0),
    ("side", 90.0, 3.0, 1.0),
    ("front", 6.0, 9.0, 1.0),
    ("top", 30.0, 78.0, 1.05),
    ("rear", 156.0, 14.0, 1.0),
]


def world_bounds(objects):
    low = Vector((1e9, 1e9, 1e9))
    high = Vector((-1e9, -1e9, -1e9))
    for obj in objects:
        if obj.type != "MESH":
            continue
        for corner in obj.bound_box:
            point = obj.matrix_world @ Vector(corner)
            for axis in range(3):
                low[axis] = min(low[axis], point[axis])
                high[axis] = max(high[axis], point[axis])
    return low, high


def _sun(scene, name, location, energy, color, size=0.06):
    data = bpy.data.lights.new(name, "SUN")
    data.energy = energy
    data.color = color
    data.angle = math.radians(size * 60)
    obj = bpy.data.objects.new(name, data)
    obj.location = location
    scene.collection.objects.link(obj)
    return obj


LIVERY = {"paint": "#ff5a4e", "paint_accent": "#fff7e6"}


def apply_livery(livery):
    """Preview only: runtime sets material.color per livery, here we tint the Mix node instead."""
    from .core import hex_linear

    for name, code in livery.items():
        material = bpy.data.materials.get(name)
        if material is None:
            continue
        tree = material.node_tree
        texture = next((n for n in tree.nodes if n.type == "TEX_IMAGE"), None)
        if texture:
            tint = tree.nodes.new("ShaderNodeMixRGB"); tint.blend_type = "MULTIPLY"
            tint.inputs[0].default_value = 1
            tint.inputs[2].default_value = (*hex_linear(code), 1)
            tree.links.new(texture.outputs["Color"], tint.inputs[1])
            mask = tree.nodes.new("ShaderNodeMath"); mask.operation = "MULTIPLY_ADD"
            mask.inputs[1].default_value = 2; mask.inputs[2].default_value = -1
            tree.links.new(texture.outputs["Alpha"], mask.inputs[0])
            mix = tree.nodes.new("ShaderNodeMixRGB")
            tree.links.new(mask.outputs[0], mix.inputs[0])
            tree.links.new(texture.outputs["Color"], mix.inputs[1])
            tree.links.new(tint.outputs[0], mix.inputs[2])
            tree.links.new(mix.outputs[0], tree.nodes["Principled BSDF"].inputs["Base Color"])
            continue
        for node in material.node_tree.nodes:
            if node.type == "MIX":
                rgb = hex_linear(code)
                node.inputs[6].default_value = (rgb[0], rgb[1], rgb[2], 1.0)


def render_previews(models, outdir, views=None, ground=True, livery=None):
    """Golden-hour toy-diorama preview: warm low key sun, cool sky fill, cream ground disc."""
    os.makedirs(outdir, exist_ok=True)
    scene = bpy.context.scene
    apply_livery(LIVERY if livery is None else livery)
    scene.render.engine = "BLENDER_EEVEE"
    scene.render.resolution_x = 1024
    scene.render.resolution_y = 768
    scene.render.film_transparent = False
    scene.render.image_settings.file_format = "PNG"
    try:
        scene.eevee.taa_render_samples = 64
    except AttributeError:
        pass
    try:
        scene.view_settings.view_transform = "Standard"
    except TypeError:
        pass

    world = bpy.data.worlds.new("AgDriftPreview")
    world.use_nodes = True
    tree = world.node_tree
    for node in list(tree.nodes):
        tree.nodes.remove(node)
    coord = tree.nodes.new("ShaderNodeTexCoord")
    split = tree.nodes.new("ShaderNodeSeparateXYZ")
    ramp = tree.nodes.new("ShaderNodeValToRGB")
    ramp.color_ramp.interpolation = "EASE"
    stops = ramp.color_ramp.elements
    stops[0].position, stops[0].color = 0.40, (0.30, 0.22, 0.16, 1.0)
    stops[1].position, stops[1].color = 0.82, (0.40, 0.58, 1.0, 1.0)
    mid = stops.new(0.50)
    mid.color = (1.0, 0.68, 0.40, 1.0)
    background = tree.nodes.new("ShaderNodeBackground")
    background.inputs[1].default_value = 0.8
    out = tree.nodes.new("ShaderNodeOutputWorld")
    tree.links.new(coord.outputs["Generated"], split.inputs[0])
    tree.links.new(split.outputs["Z"], ramp.inputs[0])
    tree.links.new(ramp.outputs[0], background.inputs[0])
    tree.links.new(background.outputs[0], out.inputs[0])
    scene.world = world

    aim_target = bpy.data.objects.new("PreviewTarget", None)
    scene.collection.objects.link(aim_target)
    for name, loc, energy, color in (
        ("Key", (8.0, -6.0, 4.5), 3.4, (1.0, 0.74, 0.46)),
        ("Fill", (-6.0, 7.0, 7.0), 1.1, (0.62, 0.74, 1.0)),
        ("Rim", (-7.0, -5.0, 3.5), 2.6, (1.0, 0.88, 0.7)),
    ):
        light = _sun(scene, name, loc, energy, color)
        constraint = light.constraints.new("TRACK_TO")
        constraint.target = aim_target
        constraint.track_axis = "TRACK_NEGATIVE_Z"
        constraint.up_axis = "UP_Y"

    if ground:
        plane = bpy.data.meshes.new("PreviewGround")
        plane.from_pydata([(-90, -90, -0.002), (90, -90, -0.002), (90, 90, -0.002), (-90, 90, -0.002)], [], [(0, 1, 2, 3)])
        floor = bpy.data.materials.new("PreviewGround")
        floor.use_nodes = True
        bsdf = floor.node_tree.nodes["Principled BSDF"]
        bsdf.inputs["Base Color"].default_value = (0.62, 0.50, 0.36, 1.0)
        bsdf.inputs["Roughness"].default_value = 0.95
        plane.materials.append(floor)
        ground_obj = bpy.data.objects.new("PreviewGround", plane)
        scene.collection.objects.link(ground_obj)

    camera_data = bpy.data.cameras.new("PreviewCam")
    camera_data.lens = 50
    camera_data.clip_start = 0.01
    camera_data.clip_end = 200.0
    camera = bpy.data.objects.new("PreviewCam", camera_data)
    scene.collection.objects.link(camera)
    scene.camera = camera
    constraint = camera.constraints.new("TRACK_TO")
    constraint.track_axis = "TRACK_NEGATIVE_Z"
    constraint.up_axis = "UP_Y"
    constraint.target = aim_target

    all_objects = [obj for model in models for obj in model.objects]
    for model in models:
        for obj in all_objects:
            obj.hide_render = obj not in model.objects
        low, high = world_bounds(model.objects)
        center = (low + high) / 2.0
        radius = (high - low).length / 2.0
        for view in views or CAR_VIEWS:
            suffix, azimuth, elevation, fill = view[:4]
            a, e = math.radians(azimuth), math.radians(elevation)
            distance = radius * 2.95 * fill
            # azimuth 0 = in front of the nose (+X), 90 = the right side (+Z), 180 = behind
            offset = (distance * math.cos(e) * math.cos(a), distance * math.sin(e), distance * math.cos(e) * math.sin(a))
            camera.location = center + Vector(to_blender(offset))
            aim_target.location = center
            scene.render.filepath = os.path.join(outdir, "%s-%s.png" % (model.name, suffix))
            bpy.ops.render.render(write_still=True)
            print("preview %s" % scene.render.filepath)
