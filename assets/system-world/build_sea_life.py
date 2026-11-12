"""Light copies of the Ocean chat theme's schooling fish and moon jelly for the System World sea.

The System World lets schools of many fish swim in its sea (sysworld-sealife.js). The chat kit's
fish (ui/3d/chat-ocean-life/v1, 1158-1414 triangles each) are built for a close camera; a school
of sixty in the harbour needs far fewer. This script imports those GLBs, decimates them and
exports them again with everything the swim shader reads kept intact: the frame (glTF +Y up,
+Z nose), TEXCOORD_0 = (along, part), TEXCOORD_1 = (height, side) and the material names.
The moon jelly of the abyss kit (ui/3d/screensaver/abyss/v1, 6896 triangles, most of them in its
tentacles) gets the same treatment, because dozens drift at the quay. The big animals (sharks,
turtle, whale, manta) are few and are loaded from their own kits unchanged.

    blender -b --factory-startup --python assets/system-world/build_sea_life.py -- [--preview DIR]
    python -I assets/system-world/build_sea_life.py --check     # committed bytes match the manifest

The decimation is deterministic for one Blender version; `--check` verifies the committed files
against the hashes in the manifest (and that the chat sources are still the ones they were made
from), it does not run Blender.
"""
import hashlib
import json
import struct
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
KITS = {'chat-ocean-life': ROOT / 'ui' / '3d' / 'chat-ocean-life' / 'v1',
        'abyss': ROOT / 'ui' / '3d' / 'screensaver' / 'abyss' / 'v1'}
OUT = ROOT / 'ui' / '3d' / 'system-world' / 'sea-life' / 'v1'
# id in its kit's manifest -> (kit, share of the triangles to keep)
LIGHT = {'fish_sardine': ('chat-ocean-life', .2), 'fish_tang': ('chat-ocean-life', .22),
         'fish_clown': ('chat-ocean-life', .22), 'fish_butterfly': ('chat-ocean-life', .22),
         'jelly_moon': ('abyss', .2)}


def sha256(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def glb_json(path):
    data = Path(path).read_bytes()
    length = struct.unpack_from('<I', data, 12)[0]
    return json.loads(data[20:20 + length])


def describe(path):
    """Triangles, material names and the attributes of every primitive, read from the GLB itself."""
    document = glb_json(path)
    triangles, materials, attributes = 0, [], set()
    for mesh in document['meshes']:
        for primitive in mesh['primitives']:
            triangles += document['accessors'][primitive['indices']]['count'] // 3
            materials.append(document['materials'][primitive['material']]['name'])
            attributes |= set(primitive['attributes'])
    return triangles, materials, sorted(attributes)


def check():
    manifest = json.loads((OUT / 'manifest.json').read_text(encoding='utf-8'))
    for entry in manifest['assets']:
        path, source = OUT / entry['file'], KITS[entry['kit']] / entry['source']
        if (path.stat().st_size, sha256(path)) != (entry['bytes'], entry['sha256']):
            raise SystemExit(f'{path.name} does not match the manifest')
        if sha256(source) != entry['source_sha256']:
            raise SystemExit(f'{source.name} changed since {path.name} was made from it: rebuild')
        triangles, materials, attributes = describe(path)
        if triangles != entry['triangles'] or materials != entry['materials']:
            raise SystemExit(f'{path.name}: triangles or materials differ from the manifest')
        if not {'POSITION', 'NORMAL', 'TEXCOORD_0', 'TEXCOORD_1'} <= set(attributes):
            raise SystemExit(f'{path.name}: the swim shader needs TEXCOORD_0 and TEXCOORD_1')
    total = sum(entry['bytes'] for entry in manifest['assets'])
    print(f'System World sea life verified: {len(manifest["assets"])} light models, {total} bytes')


def build(preview):
    import bpy
    kits = {kit: {entry['id']: entry for entry in json.loads((folder / 'manifest.json').read_text(encoding='utf-8'))['assets']}
            for kit, folder in KITS.items()}
    OUT.mkdir(parents=True, exist_ok=True)
    assets = []
    for asset_id, (kit, ratio) in LIGHT.items():
        entry = kits[kit][asset_id]
        source = KITS[kit] / entry['file']
        # Chat animals record their length; the abyss kit records bounds only (glTF axes, nose along Z).
        length = entry.get('length') or round(entry['bounds']['max'][2] - entry['bounds']['min'][2], 4)
        # A clean file for every fish: left-over materials would force suffixes onto the next import's names.
        for obj in list(bpy.data.objects): bpy.data.objects.remove(obj, do_unlink=True)
        for mesh in list(bpy.data.meshes): bpy.data.meshes.remove(mesh)
        for material in list(bpy.data.materials): bpy.data.materials.remove(material)
        bpy.ops.import_scene.gltf(filepath=str(source))
        meshes = [obj for obj in bpy.context.scene.objects if obj.type == 'MESH']
        assert len(meshes) == 1, (asset_id, [obj.name for obj in meshes])
        obj = meshes[0]
        assert len(obj.data.uv_layers) == 2, (asset_id, [layer.name for layer in obj.data.uv_layers])
        bpy.ops.object.select_all(action='DESELECT')
        obj.select_set(True)
        bpy.context.view_layer.objects.active = obj
        # Material names come back with the importer's suffixes when a name is taken; the shader
        # picks parts by name, so they return to the kit's names.
        for material in obj.data.materials: material.name = material.name.split('.')[0]
        modifier = obj.modifiers.new('light', 'DECIMATE')
        modifier.decimate_type, modifier.ratio, modifier.use_collapse_triangulate = 'COLLAPSE', ratio, True
        bpy.ops.object.modifier_apply(modifier='light')
        output = OUT / entry['file'].replace('.glb', '.lod1.glb')
        options = {
            'filepath': str(output), 'export_format': 'GLB', 'use_selection': True, 'use_active_scene': True,
            'export_animations': False, 'export_cameras': False, 'export_lights': False, 'export_extras': False,
            'export_texcoords': True, 'export_normals': True, 'export_tangents': False, 'export_yup': True,
            'export_apply': True, 'export_materials': 'EXPORT', 'export_image_format': 'NONE',
            'export_skins': False, 'export_morph': False,
        }
        supported = set(bpy.ops.export_scene.gltf.get_rna_type().properties.keys())
        bpy.ops.export_scene.gltf(**{key: value for key, value in options.items() if key in supported})
        # Same tidy-up as the chat kit: stable names, no generator stamp.
        data = output.read_bytes()
        json_length = struct.unpack_from('<I', data, 12)[0]
        document = json.loads(data[20:20 + json_length])
        for index, node in enumerate(document.get('nodes', [])): node['name'] = asset_id if index == 0 else f'{asset_id}{index}'
        for index, mesh in enumerate(document.get('meshes', [])): mesh['name'] = f'{asset_id}_mesh{index}'
        for scene in document.get('scenes', []): scene['name'] = 'SeaLifeAsset'
        document.get('asset', {}).pop('generator', None)
        payload = json.dumps(document, separators=(',', ':'), sort_keys=True).encode()
        payload += b' ' * (-len(payload) % 4)
        tail = data[20 + json_length:]
        # GLB: 12-byte header, the JSON chunk (length, type, payload), then the binary chunk as it was.
        output.write_bytes(struct.pack('<III', 0x46546C67, 2, 20 + len(payload) + len(tail))
                           + struct.pack('<II', len(payload), 0x4E4F534A) + payload + tail)
        triangles, materials, attributes = describe(output)
        assert {'POSITION', 'NORMAL', 'TEXCOORD_0', 'TEXCOORD_1'} <= set(attributes), (asset_id, attributes)
        assets.append({'id': asset_id, 'file': output.name, 'bytes': output.stat().st_size, 'sha256': sha256(output),
                       'triangles': triangles, 'materials': materials, 'length': length, 'kit': kit,
                       'source': entry['file'], 'source_sha256': sha256(source), 'source_triangles': entry['triangles']})
        print(f'sea life: {output.name} {triangles} of {entry["triangles"]} triangles, {materials}', flush=True)
        if preview:
            Path(preview).mkdir(parents=True, exist_ok=True)
            scene = bpy.context.scene
            scene.render.engine = 'BLENDER_WORKBENCH'
            scene.display.shading.light, scene.display.shading.color_type = 'STUDIO', 'MATERIAL'
            scene.render.resolution_x, scene.render.resolution_y = 900, 500
            camera_data = bpy.data.cameras.new('preview')
            camera_data.type, camera_data.ortho_scale = 'ORTHO', length * 1.25
            camera = bpy.data.objects.new('preview', camera_data)
            scene.collection.objects.link(camera)
            scene.camera = camera
            # The imported fish lies along Blender -Y (glTF +Z nose): look at its flank, slightly from above.
            from math import radians
            camera.location, camera.rotation_euler = (-3, -length * .05, .6 - (length * .35 if kit == 'abyss' else 0)), (radians(79), 0, radians(-90))
            scene.render.filepath = str(Path(preview) / f'{asset_id}.png')
            bpy.ops.render.render(write_still=True)
            bpy.data.objects.remove(camera, do_unlink=True)
    (OUT / 'LICENSE.txt').write_bytes((KITS['chat-ocean-life'] / 'LICENSE.txt').read_bytes())
    manifest = {'kit': 'aurago-system-world-sea-life', 'version': 1, 'license': 'MIT',
                'generator': 'assets/system-world/build_sea_life.py',
                'note': 'decimated copies of the chat-ocean-life fish and the abyss moon jelly; both UV sets and the material names are kept',
                'assets': assets}
    (OUT / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')


def main():
    argv = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else sys.argv[1:]
    if '--check' in argv: return check()
    build(argv[argv.index('--preview') + 1] if '--preview' in argv else None)


if __name__ == '__main__':
    main()
