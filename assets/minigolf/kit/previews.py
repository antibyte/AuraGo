"""World preview images for the menu cards (MIT License, see ../LICENSE).

``render(world, number, path)`` builds the dressed P3 course ``number`` of the world (the real generator path,
``courses.build_scene``: track, island, trims, structures, props, mechanics and baked contact shading), links real
copies of the baked kit props in place of the runtime instances, adds the water or the space backdrop and the
world light of the style probes, and renders a 640x360 Cycles image (96 samples, OIDN) saved as WebP. The images
are the card art of ``manifest.worlds[].preview``; Cycles output is not bit-reproducible, so they are only written by
``generate.py --only previews``.

``COURSES`` picks the showcase course of each world; the camera frames its playable area and the hero prop.
"""

import math
import os
import tempfile

import bpy
from mathutils import Vector, noise

from . import bake, compose, courses, dress_island, island, kits, materials, palette, scene_moon, scene_pirate, scene_temple, shapes, stage
from .island import smoothstep
from .palette import mix

# Pirate: Kraken's Whirlpool (ship, fork over the strait); Temple: Stone Head (the head over its tunnel);
# Moon: Orbit Loop (habitat, rover and the loop)
COURSES = {"pirate": 6, "temple": 9, "moon": 18}
SIZE = (640, 360)
SAMPLES = 96
MAX_BYTES = 80 * 1024
ASPECT = SIZE[0] / SIZE[1]
AZIMUTH, ELEVATION = -35.0, 38.0
MARGIN, FILL = 2.5, 0.92       # metres around the playable area, share of the image half-size the content may fill
OFFSETS = [0.0, 0.07, 0.16, 0.3, 0.5, 0.78, 1.15, 1.6, 2.2, 3.0, 4.0, 5.2, 6.8, 8.8, 11.5, 15, 20, 28, 40, 60]
LIGHTING = {"pirate": scene_pirate.lighting, "temple": scene_temple.lighting, "moon": scene_moon.lighting}
HIDDEN = ("rail-wood", "rail-stone", "rail-metal", "ball-template", "putter-template")


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


def frame(built, catalog):
    """Camera ``(target, distance)`` that fits the playable area, the hero and a margin into the 16:9 image.

    The content points are the plan points of every route (``Track.path`` and ``corridors``) pushed out by ``MARGIN``
    and the circle of the hero prop; the camera looks from the fixed azimuth and elevation. Target and distance come
    from a few rounds of the perspective projection: centre the extent of the projected points, then scale the
    distance until they fill ``FILL`` of the image.
    """
    track, hero = built["track"], built["dresser"].hero
    forward = -stage.direction(AZIMUTH, ELEVATION)
    right = forward.cross(Vector((0.0, 0.0, 1.0))).normalized()
    up = right.cross(forward)
    around = [(math.cos(a), math.sin(a)) for a in (0, math.pi / 2, math.pi, 3 * math.pi / 2)]
    points = [Vector((x + MARGIN * dx, y + MARGIN * dy, z)) for line in [track.path] + list(track.corridors) for x, y, z in line for dx, dy in around]
    if hero:
        (x, y), prop = hero["at"], catalog[hero["name"]]
        points.extend(Vector((x + prop["radius"] * dx, y + prop["radius"] * dy, 0.0)) for dx, dy in around)
        points.append(Vector((x, y, prop["height"])))
    half_height = math.tan(math.radians(stage.FOV_DEG) / 2)
    target = sum(points, Vector()) / len(points)
    distance = 20.0
    for _ in range(8):
        camera = target - forward * distance
        shot = []
        for point in points:
            rel = point - camera
            depth = rel.dot(forward)
            shot.append((rel.dot(right) / (depth * half_height * ASPECT), rel.dot(up) / (depth * half_height)))
        lo_x, hi_x = min(v[0] for v in shot), max(v[0] for v in shot)
        lo_y, hi_y = min(v[1] for v in shot), max(v[1] for v in shot)
        target = target + right * ((hi_x + lo_x) / 2 * distance * half_height * ASPECT) + up * ((hi_y + lo_y) / 2 * distance * half_height)
        distance *= max(hi_x - lo_x, hi_y - lo_y) / 2 / FILL
    return target, distance


def render(look, number, path):
    """Render the preview of ``look`` from course ``number`` to the WebP ``path``; returns ``(bytes, quality)``."""
    stage.reset()
    pal = palette.get(look)
    data, world = courses.find_course(number)
    if world != look:
        raise ValueError("preview of the %s world: course %d belongs to the %s world" % (look, number, world))
    # The baked kit of the world (real objects with COLOR_0 sources) and the dressed course.
    kit_scene = bpy.data.scenes.new("preview-kit")
    bpy.context.window.scene = kit_scene
    _, kit_objs = kits.build_kit(look, tempfile.mkdtemp(prefix="auragolf-kit"))
    by_name = {obj.name[len("prop-%s-" % look):]: obj for obj in kit_objs}
    built = courses.build_scene(number, data, world)
    dresser, scene, model = built["dresser"], built["scene"], built["model"]
    bpy.context.window.scene = scene
    for name, x, y, z, yaw, scale in dresser.placements:
        copy = by_name[name].copy()
        copy.location, copy.rotation_euler, copy.scale = (x, y, z), (0.0, 0.0, yaw), (scale, scale, scale)
        model.objects.link(copy)
    for obj in model.objects:
        if obj.name.split(".")[0] in HIDDEN:
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
    target, distance = frame(built, dresser.catalog)
    camera = stage.add_camera(scene, target, distance, AZIMUTH, ELEVATION)
    if look == "moon":
        scene_moon.place_sky(comp, camera)
    bpy.ops.render.render(write_still=False)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    return _save_webp(scene, path)
