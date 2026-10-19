"""Scene reset, camera, lights, sky and render settings (MIT License)."""

import math
import time

import bpy
from mathutils import Vector

from . import shapes

RES_X, RES_Y = 1600, 900
FOV_DEG = 43.0  # vertical, matches the game camera


def reset():
    """Start from an empty factory scene (safe between worlds in one process)."""
    bpy.ops.wm.read_factory_settings(use_empty=True)
    sc = bpy.context.scene
    shapes.set_collection(None)
    return sc


def configure_render(sc, samples=128, res=(RES_X, RES_Y), bounces=4):
    sc.render.engine = "CYCLES"
    sc.cycles.device = "CPU"
    sc.cycles.samples = samples
    sc.cycles.seed = 5
    sc.cycles.use_adaptive_sampling = True
    sc.cycles.adaptive_threshold = 0.01
    sc.cycles.adaptive_min_samples = 16
    sc.cycles.use_denoising = True
    sc.cycles.denoiser = "OPENIMAGEDENOISE"
    sc.cycles.denoising_prefilter = "ACCURATE"
    sc.cycles.max_bounces = bounces
    sc.cycles.diffuse_bounces = 2
    sc.cycles.glossy_bounces = 3
    sc.cycles.transmission_bounces = 4
    sc.cycles.transparent_max_bounces = 4
    sc.cycles.sample_clamp_indirect = 8.0
    sc.render.resolution_x, sc.render.resolution_y = res
    sc.render.resolution_percentage = 100
    sc.render.film_transparent = False
    sc.view_settings.view_transform = "Khronos PBR Neutral"
    sc.view_settings.look = "None"
    sc.view_settings.exposure = 0.0
    sc.view_settings.gamma = 1.0
    sc.display_settings.display_device = "sRGB"
    sc.render.image_settings.file_format = "PNG"
    sc.render.image_settings.color_mode = "RGB"
    sc.render.image_settings.color_depth = "8"
    sc.render.image_settings.compression = 15
    sc.render.dither_intensity = 0.0  # reproducible output files


def direction(azimuth_deg, elevation_deg):
    """Unit vector from the scene towards a source (az 0 = from -Y, +az to +X)."""
    az, el = math.radians(azimuth_deg), math.radians(elevation_deg)
    return Vector((math.sin(az) * math.cos(el), -math.cos(az) * math.cos(el), math.sin(el)))


def add_camera(sc, target, distance, azimuth_deg, elevation_deg, fov_deg=FOV_DEG, shift=(0.0, 0.0)):
    cam = bpy.data.cameras.new("kit-camera")
    cam.sensor_fit = "VERTICAL"
    cam.angle_y = math.radians(fov_deg)
    cam.shift_x, cam.shift_y = shift
    cam.clip_start = 0.1
    cam.clip_end = 600.0
    obj = bpy.data.objects.new("kit-camera", cam)
    sc.collection.objects.link(obj)
    target = Vector(target)
    pos = target + direction(azimuth_deg, elevation_deg) * distance
    obj.location = pos
    obj.rotation_euler = (target - pos).to_track_quat("-Z", "Y").to_euler()
    sc.camera = obj
    return obj


def add_sun(sc, name, azimuth_deg, elevation_deg, energy, color=(1, 1, 1), angle_deg=3.0):
    light = bpy.data.lights.new(name, "SUN")
    light.energy = energy
    light.color = color[:3]
    light.angle = math.radians(angle_deg)
    obj = bpy.data.objects.new(name, light)
    sc.collection.objects.link(obj)
    src = direction(azimuth_deg, elevation_deg)
    obj.rotation_euler = (-src).to_track_quat("-Z", "Y").to_euler()
    return obj


def add_area(sc, name, location, target, size, energy, color=(1, 1, 1), spread_deg=120.0):
    light = bpy.data.lights.new(name, "AREA")
    light.energy = energy
    light.color = color[:3]
    light.shape = "DISK"
    light.size = size
    light.spread = math.radians(spread_deg)
    obj = bpy.data.objects.new(name, light)
    sc.collection.objects.link(obj)
    obj.location = location
    obj.rotation_euler = (Vector(target) - Vector(location)).to_track_quat("-Z", "Y").to_euler()
    return obj


def set_sky(sc, stops, strength=1.0, camera_strength=None):
    """Vertical gradient world: ``stops`` = [(z, rgba), ...] with z in -1..1.

    ``strength`` lights the scene; ``camera_strength`` (optional) is what the camera sees, so a
    dark space backdrop can still give soft ambient fill.
    """
    world = bpy.data.worlds.new("kit-world")
    sc.world = world
    world.use_nodes = True
    tree = world.node_tree
    for node in list(tree.nodes):
        tree.nodes.remove(node)
    out = tree.nodes.new("ShaderNodeOutputWorld")
    bg = tree.nodes.new("ShaderNodeBackground")
    bg.inputs["Strength"].default_value = strength
    coord = tree.nodes.new("ShaderNodeTexCoord")
    sep = tree.nodes.new("ShaderNodeSeparateXYZ")
    ramp = tree.nodes.new("ShaderNodeValToRGB")
    maprange = tree.nodes.new("ShaderNodeMapRange")
    maprange.inputs["From Min"].default_value = stops[0][0]
    maprange.inputs["From Max"].default_value = stops[-1][0]
    maprange.clamp = True
    tree.links.new(coord.outputs["Generated"], sep.inputs["Vector"])
    tree.links.new(sep.outputs["Z"], maprange.inputs["Value"])
    tree.links.new(maprange.outputs["Result"], ramp.inputs["Fac"])
    tree.links.new(ramp.outputs["Color"], bg.inputs["Color"])
    tree.links.new(bg.outputs["Background"], out.inputs["Surface"])
    if camera_strength is not None:
        path = tree.nodes.new("ShaderNodeLightPath")
        blend = tree.nodes.new("ShaderNodeMath")
        blend.operation = "MULTIPLY_ADD"
        blend.inputs[1].default_value = camera_strength - strength
        blend.inputs[2].default_value = strength
        tree.links.new(path.outputs["Is Camera Ray"], blend.inputs[0])
        tree.links.new(blend.outputs["Value"], bg.inputs["Strength"])
    lo, hi = stops[0][0], stops[-1][0]
    elems = ramp.color_ramp.elements
    while len(elems) < len(stops):
        elems.new(0.5)
    for el, (z, rgba) in zip(elems, stops):
        el.position = (z - lo) / (hi - lo)
        el.color = rgba
    return world


def stats(objs):
    return sum(shapes.tri_count(o) for o in objs)


def render(sc, path):
    sc.render.filepath = path
    t0 = time.time()
    bpy.ops.render.render(write_still=True)
    return time.time() - t0


def clip_report(path):
    """Fractions of clipped (all channels >= 0.985) and near-black pixels."""
    img = bpy.data.images.load(path)
    w, h = img.size
    try:
        import numpy as np  # bundled with Blender
        px = np.empty(w * h * 4, dtype=np.float32)
        img.pixels.foreach_get(px)
        px = px.reshape(-1, 4)[:, :3]
        res = {"white": float((px.min(axis=1) >= 0.985).mean()),
               "any_channel_max": float((px.max(axis=1) >= 0.995).mean()),
               "black": float((px.max(axis=1) <= 0.03).mean())}
    finally:
        bpy.data.images.remove(img)
    return res
