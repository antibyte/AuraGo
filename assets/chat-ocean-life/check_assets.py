"""Verify the Ocean chat sea life runtime GLBs against their manifest, the contract and the budgets."""

from pathlib import Path
import hashlib
import json
import struct
import sys

ROOT = Path(__file__).resolve().parent
RUNTIME = ROOT.parents[1] / 'ui' / '3d' / 'chat-ocean-life' / 'v1'
GENERATOR = 'build_ocean_life.py'
TOTAL_BUDGET = int(1.5 * 1024 * 1024)
ASSET_IDS = ['fish_tang', 'fish_clown', 'fish_butterfly', 'fish_sardine',
             'shark_reef', 'shark_hammer', 'turtle', 'whale']
TRIANGLE_BUDGET = {'fish_tang': 3000, 'fish_clown': 3000, 'fish_butterfly': 3000, 'fish_sardine': 3000,
                   'shark_reef': 8000, 'shark_hammer': 8000, 'turtle': 8000, 'whale': 8000}
REQUIRED_ATTRIBUTES = {'POSITION', 'NORMAL', 'TEXCOORD_0', 'TEXCOORD_1'}
ALLOWED_MATERIALS = {'body', 'fin', 'eye', 'shell', 'flipper', 'fluke'}
# TEXCOORD_0.y ("part") values each material may carry.
PART_VALUES = {'body': {0.0}, 'shell': {0.0}, 'fin': {0.25, 1.0}, 'eye': {0.5}, 'flipper': {0.75}, 'fluke': {1.0}}
CLOSED_MATERIALS = {'body', 'shell', 'eye'}  # closed surfaces: signed volume must be positive (outward normals)
LENGTH_TOLERANCE = 0.03  # |z extent - length| / length, for every animal (the turtle is scaled to its whole length)
NOSE_RATIO = 0.6  # bounds max z must exceed |bounds min z| * NOSE_RATIO (nose is +Z)
COMPONENT_SIZES = {5120: 1, 5121: 1, 5122: 2, 5123: 2, 5125: 4, 5126: 4}
COMPONENT_FORMATS = {5120: 'b', 5121: 'B', 5122: 'h', 5123: 'H', 5125: 'I', 5126: 'f'}
TYPE_COUNTS = {'SCALAR': 1, 'VEC2': 2, 'VEC3': 3, 'VEC4': 4}
EPS = 1e-4


def glb_parts(data):
    magic, version, _ = struct.unpack_from('<III', data, 0)
    if magic != 0x46546C67 or version != 2:
        raise ValueError('not a glTF 2.0 binary')
    json_length = struct.unpack_from('<I', data, 12)[0]
    document = json.loads(data[20:20 + json_length])
    bin_start = 20 + json_length
    blob = b''
    if bin_start + 8 <= len(data):
        bin_length, bin_type = struct.unpack_from('<II', data, bin_start)
        if bin_type == 0x004E4942:
            blob = data[bin_start + 8:bin_start + 8 + bin_length]
    return document, blob


def glb_document(data):
    return glb_parts(data)[0]


def triangles(document):
    total = 0
    for mesh in document.get('meshes', []):
        for prim in mesh.get('primitives', []):
            accessor = document['accessors'][prim['indices']]
            total += accessor['count'] // 3
    return total


def read_accessor(document, blob, index):
    """Decode an accessor into a list of tuples (or ints for SCALAR)."""
    accessor = document['accessors'][index]
    view = document['bufferViews'][accessor['bufferView']]
    count = TYPE_COUNTS[accessor['type']]
    ctype = accessor['componentType']
    size = COMPONENT_SIZES[ctype]
    stride = view.get('byteStride') or size * count
    offset = view.get('byteOffset', 0) + accessor.get('byteOffset', 0)
    fmt = '<' + COMPONENT_FORMATS[ctype] * count
    rows = []
    for i in range(accessor['count']):
        values = struct.unpack_from(fmt, blob, offset + i * stride)
        rows.append(values[0] if count == 1 else values)
    return rows


def primitive_material(document, prim):
    index = prim.get('material')
    if index is None:
        return None
    return document['materials'][index].get('name')


def cross(a, b):
    return (a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0])


def dot(a, b):
    return a[0] * b[0] + a[1] * b[1] + a[2] * b[2]


def check_geometry(name, document, blob, errors):
    """Per-primitive content checks; returns the POSITION bounds over all primitives."""
    low = [1e9, 1e9, 1e9]
    high = [-1e9, -1e9, -1e9]
    for mesh in document.get('meshes', []):
        for prim in mesh.get('primitives', []):
            material = primitive_material(document, prim)
            attrs = prim['attributes']
            positions = read_accessor(document, blob, attrs['POSITION'])
            normals = read_accessor(document, blob, attrs['NORMAL'])
            uv0 = read_accessor(document, blob, attrs['TEXCOORD_0'])
            uv1 = read_accessor(document, blob, attrs['TEXCOORD_1'])
            indices = read_accessor(document, blob, prim['indices'])
            for p in positions:
                for axis in range(3):
                    low[axis] = min(low[axis], p[axis])
                    high[axis] = max(high[axis], p[axis])
            for n in normals:
                length = (n[0] * n[0] + n[1] * n[1] + n[2] * n[2]) ** 0.5
                if abs(length - 1.0) > 0.02:
                    errors.append(f'{name}/{material}: non-unit normal (length {length:.3f})')
                    break
            allowed = PART_VALUES.get(material, set())
            for u, v in uv0:
                if not (-EPS <= u <= 1 + EPS):
                    errors.append(f'{name}/{material}: TEXCOORD_0.x (along) {u:.4f} outside 0..1')
                    break
            for u, v in uv0:
                if not any(abs(v - allowed_value) < EPS for allowed_value in allowed):
                    errors.append(f'{name}/{material}: TEXCOORD_0.y (part) {v:.4f} not in {sorted(allowed)}')
                    break
            for u, v in uv1:
                if not (-EPS <= u <= 1 + EPS and -EPS <= v <= 1 + EPS):
                    errors.append(f'{name}/{material}: TEXCOORD_1 ({u:.4f}, {v:.4f}) outside 0..1')
                    break
            if material in CLOSED_MATERIALS:
                volume = 0.0
                for t in range(0, len(indices), 3):
                    a, b, c = positions[indices[t]], positions[indices[t + 1]], positions[indices[t + 2]]
                    volume += dot(a, cross(b, c)) / 6.0
                if volume <= 0:
                    errors.append(f'{name}/{material}: signed volume {volume:.6f} <= 0 (inside-out faces)')
    return low, high


def main():
    manifest = json.loads((RUNTIME / 'manifest.json').read_text(encoding='utf-8'))
    errors = []
    source = (ROOT / GENERATOR).read_bytes().replace(b'\r\n', b'\n')
    if hashlib.sha256(source).hexdigest() != manifest.get('generator_sha256'):
        errors.append(f'{GENERATOR} changed since the last export; rerun the Blender generator')
    if manifest.get('kit') != 'aurago-chat-ocean-life':
        errors.append(f"manifest kit is {manifest.get('kit')!r}")
    if manifest.get('uv0') != 'along, part' or manifest.get('uv1') != 'height, side':
        errors.append('manifest uv0/uv1 descriptions do not match the contract')
    ids = [asset.get('id') for asset in manifest['assets']]
    if ids != ASSET_IDS:
        errors.append(f'manifest assets {ids} != expected order {ASSET_IDS}')
    listed = set()
    total = 0
    for asset in manifest['assets']:
        path = RUNTIME / asset['file']
        listed.add(asset['file'])
        first_error = len(errors)
        data = path.read_bytes()
        total += len(data)
        if len(data) != asset['bytes']:
            errors.append(f"{asset['file']}: size {len(data)} != manifest {asset['bytes']}")
        if hashlib.sha256(data).hexdigest() != asset['sha256']:
            errors.append(f"{asset['file']}: sha256 mismatch")
        length = asset.get('length')
        if not isinstance(length, (int, float)) or length <= 0:
            errors.append(f"{asset['file']}: manifest length must be positive, got {length!r}")
            length = None
        document, blob = glb_parts(data)
        tri = triangles(document)
        if tri != asset['triangles']:
            errors.append(f"{asset['file']}: {tri} triangles != manifest {asset['triangles']}")
        budget = TRIANGLE_BUDGET.get(asset['id'])
        if budget is None:
            errors.append(f"{asset['file']}: no triangle budget for id {asset['id']}")
        elif tri > budget:
            errors.append(f"{asset['file']}: {tri} triangles exceed budget {budget}")
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
        for material in document.get('materials', []):
            if material.get('name') not in ALLOWED_MATERIALS:
                errors.append(f"{asset['file']}: material {material.get('name')!r} not in {sorted(ALLOWED_MATERIALS)}")
        if len(errors) > first_error:
            continue  # structural problems: the geometry checks would only add noise
        low, high = check_geometry(asset['file'], document, blob, errors)
        bounds = asset.get('bounds', {})
        for axis in range(3):
            tolerance = 2e-3 * max(1.0, abs(low[axis]), abs(high[axis]))
            if abs(bounds.get('min', [0, 0, 0])[axis] - low[axis]) > tolerance or abs(bounds.get('max', [0, 0, 0])[axis] - high[axis]) > tolerance:
                errors.append(f"{asset['file']}: manifest bounds differ from the GLB positions on axis {axis}")
                break
        if high[2] <= abs(low[2]) * NOSE_RATIO:
            errors.append(f"{asset['file']}: nose must point along +Z (max z {high[2]:.3f}, min z {low[2]:.3f})")
        if length is not None:
            extent = high[2] - low[2]
            if abs(extent - length) > length * LENGTH_TOLERANCE:
                errors.append(f"{asset['file']}: z extent {extent:.3f} differs from manifest length {length} by more than {LENGTH_TOLERANCE:.0%}")
    extra = {p.name for p in RUNTIME.iterdir() if p.suffix == '.glb'} - listed
    if extra:
        errors.append(f'unlisted runtime GLBs: {sorted(extra)}')
    if not (RUNTIME / 'LICENSE.txt').is_file():
        errors.append('LICENSE.txt is missing from the runtime kit')
    if total > TOTAL_BUDGET:
        errors.append(f'runtime GLBs total {total} bytes exceed {TOTAL_BUDGET}')
    if errors:
        print('\n'.join(errors))
        return 1
    print(f"{len(manifest['assets'])} ocean-life GLBs verified: {total} bytes")
    return 0


if __name__ == '__main__':
    sys.exit(main())
