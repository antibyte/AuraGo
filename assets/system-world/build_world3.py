"""Build AuraGo's System World kit v3 in Blender 5.2 LTS.

    & 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/system-world/build_world3.py
    ... build_world3.py -- --only street-zebra,street-crossing --preview

Writes uncompressed GLBs and manifest.raw.json to disposable/system-world/v3-raw/ and saves
production/aurago-world-3.blend. `node scripts/build-system-world-v3.mjs` then packs the raw
files with meshopt into ui/3d/system-world/v3/. --preview also renders every LOD 0 model to
reports/system-world-assets/v3/ (authoring aid, not shipped). --only limits the run to the
listed ids and leaves the manifest and the source scene untouched.
"""
from pathlib import Path
from math import radians
import json
import sys

import bpy
from mathutils import Vector

HERE = Path(__file__).resolve().parent
if str(HERE) not in sys.path:
    sys.path.insert(0, str(HERE))

import kit3  # noqa: E402
import street_assets3  # noqa: E402
import facade_assets3  # noqa: E402
import district_assets3  # noqa: E402
import furniture_assets3  # noqa: E402
import interior_assets3  # noqa: E402

VERSION = '3.2.0'
FAMILIES = [street_assets3, facade_assets3, district_assets3, furniture_assets3, interior_assets3]
SOURCES = ['kit3.py', 'build_world3.py'] + [Path(m.__file__).name for m in FAMILIES]


def arguments():
    argv = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
    only = None
    if '--only' in argv:
        only = set(argv[argv.index('--only') + 1].split(','))
    return only, '--preview' in argv


def preview_materials():
    """Show the baked occlusion in renders: base colour times the colour attribute."""
    for material in bpy.data.materials:
        if not material.name.startswith('city.') or not material.use_nodes:
            continue
        nodes, links = material.node_tree.nodes, material.node_tree.links
        bsdf = nodes.get('Principled BSDF')
        if not bsdf or nodes.get('occlusion'):
            continue
        attribute = nodes.new('ShaderNodeVertexColor')
        attribute.name = 'occlusion'
        attribute.layer_name = 'Color'
        mix = nodes.new('ShaderNodeMix')
        mix.data_type = 'RGBA'
        mix.blend_type = 'MULTIPLY'
        mix.inputs['Factor'].default_value = 1
        mix.inputs['A'].default_value = bsdf.inputs['Base Color'].default_value
        links.new(attribute.outputs['Color'], mix.inputs['B'])
        links.new(mix.outputs['Result'], bsdf.inputs['Base Color'])


def render_previews(scene, roots):
    kit3.REPORT.mkdir(parents=True, exist_ok=True)
    preview_materials()
    for engine in ('BLENDER_EEVEE', 'BLENDER_EEVEE_NEXT'):
        try:
            scene.render.engine = engine
            break
        except TypeError:
            continue
    scene.render.resolution_x, scene.render.resolution_y = 1400, 800
    scene.render.image_settings.file_format = 'PNG'
    scene.view_settings.view_transform = 'Standard'
    world = bpy.data.worlds.new('preview')
    world.use_nodes = True
    world.node_tree.nodes['Background'].inputs['Color'].default_value = (.55, .68, .82, 1)
    world.node_tree.nodes['Background'].inputs['Strength'].default_value = .9
    scene.world = world
    sun = bpy.data.objects.new('preview-sun', bpy.data.lights.new('preview-sun', 'SUN'))
    sun.data.energy = 4.2
    sun.data.angle = radians(3)
    sun.rotation_euler = (radians(52), radians(8), radians(38))
    scene.collection.objects.link(sun)
    camera = bpy.data.objects.new('preview-camera', bpy.data.cameras.new('preview-camera'))
    camera.data.lens = 38
    scene.collection.objects.link(camera)
    scene.camera = camera
    everything = [obj for root in roots for obj in [root] + list(root.children_recursive)]
    for root in roots:
        members = {root, *root.children_recursive}
        for obj in everything:
            obj.hide_render = obj not in members
        corners = [root.matrix_world @ child.matrix_local @ Vector(p)
                   for child in root.children if child.type == 'MESH' for p in child.bound_box]
        low = Vector([min(c[i] for c in corners) for i in range(3)])
        high = Vector([max(c[i] for c in corners) for i in range(3)])
        centre, size, tall = (low + high) / 2, (high - low).length, high.z - low.z
        distance = max(size * .82, tall * 2.0, 3.2)
        views = [('', centre, centre + Vector((.9, -1.2, .62)).normalized() * distance),
                 ('-low', centre, centre + Vector((.75, -1.1, .12)).normalized() * distance)]
        if tall > 8:
            # Street view: eye level in front of the entrance (-Y).
            origin = Vector((root.matrix_world.translation.x, root.matrix_world.translation.y, low.z))
            views.append(('-street', origin + Vector((0, low.y - centre.y + (centre.y - origin.y), 4.2)),
                          origin + Vector((6.5, low.y - origin.y - 13, 1.9))))
        for suffix, target, position in views:
            camera.location = position
            camera.rotation_euler = (target - position).to_track_quat('-Z', 'Y').to_euler()
            scene.render.filepath = str(kit3.REPORT / f"{root['asset_id']}{suffix}.png")
            bpy.ops.render.render(write_still=True)
    for obj in everything:
        obj.hide_render = False


def build():
    only, preview = arguments()
    for directory in (kit3.RAW, kit3.SOURCE):
        directory.mkdir(parents=True, exist_ok=True)
    scene = bpy.data.scenes.get('AuraGo World 3') or bpy.data.scenes.new('AuraGo World 3')
    if bpy.context.window:
        bpy.context.window.scene = scene
    collection = kit3.reset_collection(scene)
    mats = kit3.materials()
    assets = [entry for family in FAMILIES for entry in family.ASSETS]
    ids = [entry[0] for entry in assets]
    assert len(ids) == len(set(ids)), 'Duplicate v3 asset id'
    if only:
        missing = only - set(ids)
        assert not missing, f'Unknown asset id: {sorted(missing)}'
    manifest = {'schema_version': 3, 'version': VERSION, 'license': 'MIT',
                'generator': f'Blender {bpy.app.version_string} / build_world3.py', 'units': 'metres',
                'up_axis': 'Y', 'sources': {name: kit3.source_hash(HERE / name) for name in SOURCES},
                'assets': []}
    roots = []
    for asset_id, label, category, factory in assets:
        if only and asset_id not in only:
            continue
        entry = {'id': asset_id, 'name': label, 'category': category, 'lods': []}
        for lod in range(3):
            root, info = kit3.create_asset(asset_id, factory, lod, collection, mats)
            result = kit3.export_asset(root, kit3.RAW / f'{asset_id}.lod{lod}.glb')
            result['level'] = lod
            entry['lods'].append(result)
            if lod == 0:
                entry['components'] = [child.get('component') for child in root.children]
                entry['navigation'] = info.get('navigation', {'colliders': []})
                entry['animations'] = info.get('animations', [])
                roots.append(root)
            else:
                kit3.remove_tree(root)
        triangles = [lod['triangles'] for lod in entry['lods']]
        assert triangles[0] >= triangles[1] >= triangles[2], f'{asset_id}: LOD triangles must not increase'
        manifest['assets'].append(entry)
        print('EXPORTED', asset_id, triangles)
    for index, root in enumerate(roots):
        root.location = ((index % 5) * 20, (index // 5) * 20, 0)
    bpy.context.view_layer.update()
    for obj in list(scene.collection.objects):
        if obj.name in ('Cube', 'Camera', 'Light'):
            bpy.data.objects.remove(obj, do_unlink=True)
    if preview:
        render_previews(scene, roots)
    if only:
        print('WORLD3_PARTIAL', json.dumps(sorted(only)))
        return
    (kit3.RAW / 'manifest.raw.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8', newline='\n')
    bpy.data.libraries.write(str(kit3.SOURCE / 'aurago-world-3.blend'), {scene}, compress=True)
    print('WORLD3', json.dumps({'designs': len(manifest['assets']), 'models': len(manifest['assets']) * 3}))


if __name__ == '__main__':
    build()
