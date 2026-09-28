"""Verify the Tiefsee screensaver runtime GLBs against their manifest and budgets."""

from pathlib import Path
import hashlib
import json
import struct
import sys

ROOT = Path(__file__).resolve().parent
RUNTIME = ROOT.parents[1] / 'ui' / '3d' / 'screensaver' / 'abyss' / 'v1'
TOTAL_BUDGET = int(1.5 * 1024 * 1024)
TRIANGLE_BUDGET = {'jelly_moon': 12000, 'jelly_lion': 12000, 'jelly_comb': 12000, 'manta': 8000}
REQUIRED_ATTRIBUTES = {'POSITION', 'NORMAL', 'TEXCOORD_0', 'TEXCOORD_1'}


def glb_document(data):
    magic, version, _ = struct.unpack_from('<III', data, 0)
    if magic != 0x46546C67 or version != 2:
        raise ValueError('not a glTF 2.0 binary')
    json_length = struct.unpack_from('<I', data, 12)[0]
    return json.loads(data[20:20 + json_length])


def triangles(document):
    total = 0
    for mesh in document.get('meshes', []):
        for prim in mesh.get('primitives', []):
            accessor = document['accessors'][prim['indices']]
            total += accessor['count'] // 3
    return total


def main():
    manifest = json.loads((RUNTIME / 'manifest.json').read_text(encoding='utf-8'))
    errors = []
    source = (ROOT / 'build_abyss.py').read_bytes().replace(b'\r\n', b'\n')
    if hashlib.sha256(source).hexdigest() != manifest.get('generator_sha256'):
        errors.append('build_abyss.py changed since the last export; rerun the Blender generator')
    listed = set()
    total = 0
    for asset in manifest['assets']:
        path = RUNTIME / asset['file']
        listed.add(asset['file'])
        data = path.read_bytes()
        total += len(data)
        if len(data) != asset['bytes']:
            errors.append(f"{asset['file']}: size {len(data)} != manifest {asset['bytes']}")
        if hashlib.sha256(data).hexdigest() != asset['sha256']:
            errors.append(f"{asset['file']}: sha256 mismatch")
        document = glb_document(data)
        tri = triangles(document)
        if tri != asset['triangles']:
            errors.append(f"{asset['file']}: {tri} triangles != manifest {asset['triangles']}")
        if tri > TRIANGLE_BUDGET[asset['id']]:
            errors.append(f"{asset['file']}: {tri} triangles exceed budget {TRIANGLE_BUDGET[asset['id']]}")
        if document.get('images') or document.get('textures'):
            errors.append(f"{asset['file']}: runtime creatures must not embed textures")
        for buffer in document.get('buffers', []):
            if 'uri' in buffer:
                errors.append(f"{asset['file']}: external buffer URIs are forbidden")
        for mesh in document.get('meshes', []):
            for prim in mesh.get('primitives', []):
                missing = REQUIRED_ATTRIBUTES - set(prim.get('attributes', {}))
                if missing:
                    errors.append(f"{asset['file']}: primitive lacks {sorted(missing)}")
    extra = {p.name for p in RUNTIME.iterdir() if p.suffix == '.glb'} - listed
    if extra:
        errors.append(f'unlisted runtime GLBs: {sorted(extra)}')
    if total > TOTAL_BUDGET:
        errors.append(f'runtime GLBs total {total} bytes exceed {TOTAL_BUDGET}')
    if errors:
        print('\n'.join(errors))
        return 1
    print(f"{len(manifest['assets'])} Tiefsee GLBs verified: {total} bytes")
    return 0


if __name__ == '__main__':
    sys.exit(main())
