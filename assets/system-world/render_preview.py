"""Render exported GLBs for visual review; never ship the preview stage."""
from pathlib import Path
from math import pi
import sys
import bpy
from mathutils import Vector

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'ui/3d/system-world/v1'
REPORT = ROOT / 'reports/system-world-assets'
sys.path.insert(0, str(Path(__file__).parent))
from build_city import materials, create_asset


def look_at(obj, target):
    obj.rotation_euler = (Vector(target)-obj.location).to_track_quat('-Z', 'Y').to_euler()


def area(name, position, target, power, color, size):
    data = bpy.data.lights.new(name, 'AREA')
    data.energy, data.color, data.shape, data.size = power, color, 'DISK', size
    obj = bpy.data.objects.new(name, data)
    bpy.context.scene.collection.objects.link(obj)
    obj.location = position
    look_at(obj, target)


def start_scene(name='City Asset Review'):
    old = bpy.data.scenes.get(name)
    if old:
        for obj in list(old.objects):
            bpy.data.objects.remove(obj, do_unlink=True)
        bpy.data.scenes.remove(old)
    scene = bpy.data.scenes.new(name)
    bpy.context.window.scene = scene
    scene.render.engine = 'CYCLES'
    scene.cycles.samples = 40
    scene.cycles.use_denoising = True
    preferences = bpy.context.preferences.addons['cycles'].preferences
    preferences.compute_device_type = 'ONEAPI'
    preferences.get_devices()
    for device in preferences.devices:
        device.use = device.type == 'ONEAPI'
    scene.cycles.device = 'GPU' if any(d.use for d in preferences.devices) else 'CPU'
    scene.cycles.max_bounces = 5
    scene.render.resolution_x, scene.render.resolution_y = 1920, 1080
    scene.render.resolution_percentage = 100
    scene.render.image_settings.file_format = 'PNG'
    scene.view_settings.view_transform = 'AgX'
    scene.world = bpy.data.worlds.new(name + ' world')
    scene.world.use_nodes = True
    background = scene.world.node_tree.nodes.get('Background')
    background.inputs['Color'].default_value = (.018, .033, .06, 1)
    background.inputs['Strength'].default_value = .35
    camera_data = bpy.data.cameras.new(name+' camera')
    camera = bpy.data.objects.new(name+' camera', camera_data)
    scene.collection.objects.link(camera)
    scene.camera = camera
    camera_data.lens, camera_data.clip_end = 44, 1500
    return scene


def load(asset_id, location=(0, 0, 0), angle=0, scale=1, lod=0):
    before = set(bpy.context.scene.objects)
    bpy.ops.import_scene.gltf(filepath=str(OUT/f'{asset_id}.lod{lod}.glb'))
    loaded = [obj for obj in bpy.context.scene.objects if obj not in before]
    roots = [obj for obj in loaded if obj.parent not in loaded]
    for root in roots:
        root.location = location
        root.rotation_euler.z += angle
        root.scale *= scale
    return roots


def city():
    scene = start_scene()
    collection = bpy.data.collections.new('Review stage')
    scene.collection.children.link(collection)
    mats = materials()
    def stage(g):
        g.box((0, 8, -1.25), (152, 148, 2.5), 'graphite', .6)
        g.box((0, 8, -.06), (149, 145, .12), 'stone', 0)
        for x in (-76, 76):
            g.box((x, 8, -.8), (.07, 138, .08), 'cyan', 0)
        for y in (-66, 82):
            g.box((0, y, -.8), (143, .07, .08), 'ivory', 0)
    create_asset('review-ground', stage, 0, collection, mats)
    road_x, road_y = (-62, -16, 17, 62), (-50, -12, 30, 69)
    for y in road_y:
        for x in road_x:
            load('street-crossing', (x, y, 0))
        edges = (-76, *road_x, 76)
        for i, (a, b) in enumerate(zip(edges, edges[1:])):
            lo = a if i == 0 else a+6
            hi = b if i == len(edges)-2 else b-6
            for root in load('street-tile', ((lo+hi)/2, y, 0)):
                root.scale.x = (hi-lo)/16
    for x in road_x:
        edges = (-66, *road_y, 82)
        for i, (a, b) in enumerate(zip(edges, edges[1:])):
            lo = a if i == 0 else a+6
            hi = b if i == len(edges)-2 else b-6
            for root in load('street-tile', (x, (lo+hi)/2, 0), pi/2):
                root.scale.x = (hi-lo)/16
    placements = [
        ('agent-spire', (0, 12, 0)),
        ('memory-archive', (-39, 10, 0)),
        ('integration-gate', (39, 10, 0)),
        ('mission-terminal', (40, -32, 0)),
        ('compute-foundry', (-39, 50, 0)),
        ('knowledge-atrium', (-39, -32, 0)),
        ('data-tower-a', (29, 51, 0)),
        ('data-tower-b', (48, 51, 0)),
        ('data-tower-b', (-5, 51, 0)),
        ('operations-beacon', (0, -32, 0)),
        ('skybridge', (13, 51, 16)),
    ]
    for asset_id, location in placements:
        load(asset_id, location)
    for y in (-50, -12, 30, 69):
        for x in (-52, -28, 28, 52):
            load('street-lamp', (x, y+4.5, .5), -pi/2)
            load('planter', (x+5, y+4.6, .5))
    for loc in ((-18, -50, 0), (33, -12, 0), (-42, 30, 0)):
        load('data-tram', loc)
    for loc in ((-8, -16, 16), (30, 12, 30), (-30, 38, 23)):
        load('service-drone', loc, .6, 1.7)
    area('Moon softbox', (-30, -20, 125), (0, 5, 0), 100000, (.68, .82, 1), 85)
    area('Warm horizon', (70, 50, 55), (0, 10, 20), 85000, (1, .63, .31), 70)
    area('Facade fill', (10, -85, 45), (0, 10, 30), 55000, (.47, .70, 1), 70)
    scene.camera.location = (190, -244, 156)
    look_at(scene.camera, (0, 12, 25))
    scene.render.filepath = str(REPORT/'city-overview.png')
    bpy.ops.wm.save_as_mainfile(filepath=str(REPORT/'city-preview.blend'), compress=True)
    return scene


def render():
    scene = city()
    bpy.ops.render.render(write_still=True)
    scene.camera.location = (36, -61, 36)
    look_at(scene.camera, (0, 12, 39))
    scene.render.filepath = str(REPORT/'agent-spire-close.png')
    bpy.ops.render.render(write_still=True)
    print('RENDERED', scene.render.filepath)


if __name__ == '__main__':
    render()
