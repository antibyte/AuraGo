"""World preview images for the menu cards (MIT License, see ../LICENSE).

``render(look, number, path)`` builds the dressed course ``number`` of the world
(the real generator path: island, trims, specials, baked contact shading), links
real copies of the baked kit props in place of the runtime instances, adds the
water or the space backdrop and the world light of the style probes, and renders
a 640x360 Cycles image (96 samples, OIDN) saved as WebP. The images are the card
art of ``manifest.worlds[].preview``; Cycles output is not bit-reproducible, so
they are only written by ``generate.py --only previews``.
"""

import os
import tempfile

import bpy
from mathutils import Vector, noise

from . import bake, compose, courses_v1, dress, dress_island, island, kits, materials, palette, scene_moon, scene_pirate, scene_temple, shapes, stage
from .island import smoothstep
from .palette import mix

# Cove Curve (the ship is the bay's hero; Lighthouse Way shows a lighthouse instead), Flower Arch, Grand Finale
COURSES = {"pirate": 7, "temple": 2, "moon": 18}
SIZE = (640, 360)
SAMPLES = 96
MAX_BYTES = 80 * 1024
CAMERA = dict(target=(0.0, 0.2, 0.2), distance=15.4, azimuth=-35.0, elevation=38.0)
OFFSETS = [0.0, 0.07, 0.16, 0.3, 0.5, 0.78, 1.15, 1.6, 2.2, 3.0, 4.0, 5.2, 6.8, 8.8, 11.5, 15, 20, 28, 40, 60]
LIGHTING = {"pirate": scene_pirate.lighting, "temple": scene_temple.lighting, "moon": scene_moon.lighting}


def _water_colour(look, pal, off):
    """Colour of the water at ``offset`` metres from the shore: foam, shallows, deep (the probe ramps)."""
    foam = pal.c("foam") if pal.has("foam") else (1.0, 1.0, 1.0, 1.0)
    if look == "pirate":
        shore, light, mid, deep = pal.c("lagoon_shore"), pal.c("lagoon_light"), pal.c("lagoon_mid"), pal.c("lagoon_deep")
    else:
        shore, light, mid, deep = pal.c("water_light"), pal.c("water_light"), pal.c("water"), pal.c("water_deep")

    def colour(x, y, offset):
        if offset < 0.9:
            c = mix(shore, light, smoothstep(offset, 0.1, 0.9))
        elif offset < 4.0:
            c = mix(light, mid, smoothstep(offset, 0.9, 4.0))
        else:
            c = mix(mid, deep, smoothstep(offset, 4.0, 14.0))
        n = noise.noise(Vector((x * 0.7, y * 0.7, 3.0)) + off)
        c = (c[0] * (1 + 0.08 * n), c[1] * (1 + 0.08 * n), c[2] * (1 + 0.06 * n), 1.0)
        f = (1.0 - smoothstep(offset, 0.03, 0.24)) * (0.65 + 0.35 * noise.noise(Vector((x * 3.0, y * 3.0, 1.0)) + off))
        return mix(c, foam, min(1.0, f))

    return colour


def _save_webp(scene, path):
    """Save the render result as WebP, lowering the quality until it fits the card budget."""
    image = bpy.data.images["Render Result"]
    scene.render.image_settings.file_format = "WEBP"
    scene.render.image_settings.color_mode = "RGB"
    quality = 82
    while True:
        scene.render.image_settings.quality = quality
        image.save_render(path, scene=scene)
        size = os.path.getsize(path)
        if size <= MAX_BYTES or quality <= 40:
            return size, quality
        quality -= 4


def render(look, number, path):
    """Render the preview of ``look`` from course ``number`` to the WebP ``path``; returns ``(bytes, quality)``."""
    stage.reset()
    pal = palette.get(look)
    course = courses_v1.COURSES[number - 1]
    # The baked kit of the world (real objects with COLOR_0 sources) and the dressed course.
    kit_scene = bpy.data.scenes.new("preview-kit")
    bpy.context.window.scene = kit_scene
    _, kit_objs = kits.build_kit(look, tempfile.mkdtemp(prefix="auragolf-kit"))
    by_name = {obj.name[len("prop-%s-" % look):]: obj for obj in kit_objs}
    dresser = dress.Dresser(number, course)
    built = courses_v1.build_scene(number, course, dresser)
    dresser.bake(built["model"])
    scene, model = built["scene"], built["model"]
    bpy.context.window.scene = scene
    for name, x, y, z, yaw, scale in dresser.placements:
        copy = by_name[name].copy()
        copy.location, copy.rotation_euler, copy.scale = (x, y, z), (0.0, 0.0, yaw), (scale, scale, scale)
        model.objects.link(copy)
    for obj in list(built["studio"].objects):
        obj.hide_render = True
    for obj in model.objects:
        base = obj.name.split(".")[0]
        if base in ("rail-left", "rail-right", "ball-template", "putter-template"):
            obj.hide_render = True
    stage.configure_render(scene, samples=SAMPLES, res=SIZE)
    shapes.set_collection(model)
    try:
        if look != "moon":
            outline = dresser.terrain.outline
            water = island.build_water(dresser.waterline, outline.center, dress_island.WATER_Z, _water_colour(look, pal, Vector((7.0, -11.0, 0.0))), OFFSETS)
            water.data.materials.append(materials.get("water", bump=0.05))
            bake.commit_colours(water)
        else:
            comp = compose.Composer("moon")
            scene_moon.space(scene, comp, pal)
            for obj in comp.passthrough:
                bake.commit_colours(obj)
    finally:
        shapes.set_collection(None)
    LIGHTING[look](scene, pal)
    camera = stage.add_camera(scene, CAMERA["target"], CAMERA["distance"], CAMERA["azimuth"], CAMERA["elevation"])
    if look == "moon":
        scene_moon.place_sky(comp, camera)
    bpy.ops.render.render(write_still=False)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    return _save_webp(scene, path)
