"""Verify the shipped city kit without Blender or third-party dependencies."""
from pathlib import Path
import hashlib
import json
import math
import struct

ROOT = Path(__file__).resolve().parents[2]
DIRECTORY = ROOT / 'ui/3d/system-world/v1'


def check():
    manifest = json.loads((DIRECTORY / 'manifest.json').read_text(encoding='utf-8'))
    total = triangles = buffers = 0
    files, ids = set(), set()
    assert manifest['up_axis'] == 'Y' and manifest['units'] == 'metres'
    assert manifest['license'] == 'MIT' and manifest['textures'] == 0
    assert len(manifest['assets']) == 17, 'Incomplete asset catalog'
    for asset in manifest['assets']:
        assert asset['id'] not in ids
        ids.add(asset['id'])
        assert [item['level'] for item in asset['lods']] == [0, 1, 2]
        counts = []
        for lod in asset['lods']:
            name = lod['file']
            assert Path(name).name == name and name.endswith('.glb')
            assert name not in files
            files.add(name)
            data = (DIRECTORY / name).read_bytes()
            assert len(data) == lod['bytes'], name
            assert hashlib.sha256(data).hexdigest() == lod['sha256'], name
            magic, version, size = struct.unpack_from('<III', data)
            assert magic == 0x46546C67 and version == 2 and size == len(data), name
            length, kind = struct.unpack_from('<II', data, 12)
            assert kind == 0x4E4F534A
            doc = json.loads(data[20:20+length])
            offset = 20+length
            binary_size, kind = struct.unpack_from('<II', data, offset)
            assert kind == 0x004E4942 and offset+8+binary_size == len(data)
            assert len(doc.get('buffers', [])) == 1
            assert not doc['buffers'][0].get('uri')
            assert 0 <= binary_size-doc['buffers'][0]['byteLength'] <= 3
            assert not doc.get('images') and not doc.get('textures')
            assert not doc.get('animations') and not doc.get('cameras') and not doc.get('skins')
            assert set(doc.get('extensionsUsed', [])) <= {'KHR_materials_emissive_strength'}
            assert len(doc['scenes']) == 1 and len(doc['scenes'][0]['nodes']) == 1
            root = doc['nodes'][doc['scenes'][0]['nodes'][0]]
            assert root['extras']['asset_id'] == asset['id'], name
            assert root['extras']['lod'] == lod['level']
            assert len(root['children'])+1 == len(doc['nodes']), name
            assert sorted(doc['nodes'][i]['extras']['component'] for i in root['children']) == sorted(asset['components'])
            count = 0
            for mesh in doc['meshes']:
                assert len(mesh['primitives']) <= 12, name
                for primitive in mesh['primitives']:
                    assert primitive.get('mode', 4) == 4
                    accessor = doc['accessors'][primitive['indices']]
                    assert accessor['count'] % 3 == 0
                    count += accessor['count']//3
                    position = doc['accessors'][primitive['attributes']['POSITION']]
                    assert all(math.isfinite(v) for v in position['min']+position['max'])
            assert count == lod['triangles'], (name, count, lod['triangles'])
            bounds = lod['bounds']
            assert all(math.isfinite(v) for v in bounds['min']+bounds['max'])
            assert all(a < b for a, b in zip(bounds['min'], bounds['max'])), name
            assert abs(bounds['min'][1]) < .5, (name, 'Pivot must be near ground level')
            counts.append(count)
            total += len(data)
            buffers += doc['buffers'][0]['byteLength']
            triangles += count
        assert counts[0] >= counts[1] >= counts[2] > 0, (asset['id'], counts)
    assert {p.name for p in DIRECTORY.glob('*.glb')} == files, 'Uncatalogued models'
    assert total == manifest['total_model_bytes']
    assert sum(p.stat().st_size for p in DIRECTORY.iterdir() if p.is_file()) <= 8*1024*1024
    assert total <= manifest['budget_bytes'] == 8*1024*1024
    print(json.dumps({'assets': len(ids), 'models': len(files), 'model_bytes': total,
                      'mesh_buffer_bytes_all_lods': buffers, 'triangles_all_lods': triangles,
                      'textures': 0, 'checks': 'passed'}, indent=2))


def check_robot():
    directory = DIRECTORY.parent
    manifest = json.loads((directory / 'white-robot.json').read_text(encoding='utf-8'))
    data = (directory / manifest['file']).read_bytes()
    assert manifest['source'] == 'ui/3d/robot.glb'
    assert hashlib.sha256((ROOT / manifest['source']).read_bytes()).hexdigest() == manifest['source_sha256']
    assert hashlib.sha256(data).hexdigest() == manifest['sha256']
    assert len(data) == manifest['bytes'] < 2 * 1024 * 1024
    magic, version, size, length, kind = struct.unpack_from('<IIIII', data)
    assert magic == 0x46546C67 and version == 2 and size == len(data) and kind == 0x4E4F534A
    doc = json.loads(data[20:20+length])
    assert not doc.get('extensionsRequired') and not doc.get('skins') and not doc.get('animations')
    assert len(doc['buffers']) == 1 and not doc['buffers'][0].get('uri')
    count = 0
    for mesh in doc['meshes']:
        for primitive in mesh['primitives']:
            assert primitive.get('mode', 4) == 4
            assert 'TANGENT' in primitive['attributes']
            count += doc['accessors'][primitive['indices']]['count'] // 3
    assert count == manifest['triangles'] <= 25000
    assert len(doc['images']) == 3
    for image in doc['images']:
        assert image['mimeType'] == 'image/png' and not image.get('uri')
        view = doc['bufferViews'][image['bufferView']]
        image_start = 28 + length + view.get('byteOffset', 0)
        width, height = struct.unpack_from('>II', data, image_start + 16)
        assert 0 < width <= 512 and 0 < height <= 512
    total = sum(p.stat().st_size for p in directory.rglob('*') if p.is_file() and p.suffix in {'.glb', '.json', '.txt'})
    assert total < 8 * 1024 * 1024
    print(json.dumps({'robot_triangles': count, 'robot_bytes': len(data),
                      'combined_runtime_bytes': total, 'robot_checks': 'passed'}))


if __name__ == '__main__':
    check()
    check_robot()
