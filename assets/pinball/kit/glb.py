"""Keep Blender's parallel triangle ordering out of reproducible asset hashes."""
import json
import math
import struct


def canonicalize(path):
    data = bytearray(path.read_bytes())
    size = struct.unpack_from('<I', data, 12)[0]
    document = json.loads(data[20:20 + size])
    binary = 28 + size
    uvs = {value for mesh in document['meshes'] for primitive in mesh['primitives'] for key, value in primitive['attributes'].items() if key.startswith('TEXCOORD_')}
    for index in uvs:
        accessor = document['accessors'][index]
        view = document['bufferViews'][accessor['bufferView']]
        assert accessor['componentType'] == 5126 and accessor['type'] == 'VEC2' and not view.get('byteStride')
        offset = binary + view.get('byteOffset', 0) + accessor.get('byteOffset', 0)
        count = accessor['count'] * 2
        values = struct.unpack_from('<' + str(count) + 'f', data, offset)
        struct.pack_into('<' + str(count) + 'f', data, offset, *(round(value, 5) + 0.0 for value in values))
    indices = {primitive['indices'] for mesh in document['meshes'] for primitive in mesh['primitives']}
    for index in indices:
        accessor = document['accessors'][index]
        view = document['bufferViews'][accessor['bufferView']]
        assert accessor['type'] == 'SCALAR' and accessor['count'] % 3 == 0
        code = {5121: 'B', 5123: 'H', 5125: 'I'}[accessor['componentType']]
        offset = binary + view.get('byteOffset', 0) + accessor.get('byteOffset', 0)
        values = struct.unpack_from('<' + str(accessor['count']) + code, data, offset)
        triangles = []
        for start in range(0, len(values), 3):
            triangle = values[start:start + 3]
            triangles.append(min(triangle, triangle[1:] + triangle[:1], triangle[2:] + triangle[:2]))
        values = [value for triangle in sorted(triangles) for value in triangle]
        struct.pack_into('<' + str(len(values)) + code, data, offset, *values)
    # Match computeVertexNormals: sorted triangles make smooth normals deterministic.
    for mesh in document['meshes']:
        for primitive in mesh['primitives']:
            attributes = primitive['attributes']
            if 'NORMAL' not in attributes:
                continue
            position = document['accessors'][attributes['POSITION']]
            normal = document['accessors'][attributes['NORMAL']]
            index = document['accessors'][primitive['indices']]
            def offset_of(accessor):
                return binary + document['bufferViews'][accessor['bufferView']].get('byteOffset', 0) + accessor.get('byteOffset', 0)
            count = position['count']
            points = list(struct.iter_unpack('<fff', data[offset_of(position):offset_of(position) + count * 12]))
            code = {5121: 'B', 5123: 'H', 5125: 'I'}[index['componentType']]
            faces = struct.unpack_from('<' + str(index['count']) + code, data, offset_of(index))
            normals = [[0., 0., 0.] for _ in points]
            for start in range(0, len(faces), 3):
                ids = faces[start:start + 3]
                a, b, c = (points[i] for i in ids)
                u, v = [b[i] - a[i] for i in range(3)], [c[i] - a[i] for i in range(3)]
                cross = [u[1]*v[2]-u[2]*v[1], u[2]*v[0]-u[0]*v[2], u[0]*v[1]-u[1]*v[0]]
                for vertex in ids:
                    for axis in range(3): normals[vertex][axis] += cross[axis]
            values = []
            for vector in normals:
                length = math.sqrt(sum(value * value for value in vector))
                values.extend(value / length for value in vector) if length else values.extend([0., 1., 0.])
            struct.pack_into('<' + str(len(values)) + 'f', data, offset_of(normal), *values)
    # Blender may share equal accessors before its parallel triangle ordering settles.
    old = document['accessors']; views = document['bufferViews']
    accessors, packed_views, payload, lookup, remap = [], [], bytearray(), {}, {}
    def pack(index):
        if index in remap: return remap[index]
        source = old[index]; view = views[source['bufferView']]
        assert not view.get('byteStride') and 'sparse' not in source
        meta = {key: value for key, value in source.items() if key not in ['bufferView', 'byteOffset']}
        width = {'SCALAR':1,'VEC2':2,'VEC3':3,'VEC4':4,'MAT4':16}[source['type']]
        size = {5120:1,5121:1,5122:2,5123:2,5125:4,5126:4}[source['componentType']]
        start = binary + view.get('byteOffset', 0) + source.get('byteOffset', 0)
        content = bytes(data[start:start + source['count'] * width * size])
        target = view.get('target')
        key = (json.dumps(meta, sort_keys=True), target, content)
        if key not in lookup:
            payload.extend(b'\0' * (-len(payload) % 4))
            packed = {'buffer':0,'byteOffset':len(payload),'byteLength':len(content)}
            if target: packed['target'] = target
            lookup[key] = len(accessors)
            accessors.append({**meta,'bufferView':len(packed_views)})
            packed_views.append(packed); payload.extend(content)
        remap[index] = lookup[key]
        return remap[index]
    for mesh in document['meshes']:
        for primitive in mesh['primitives']:
            primitive['attributes'] = {key: pack(value) for key, value in sorted(primitive['attributes'].items())}
            primitive['indices'] = pack(primitive['indices'])
    for animation in document.get('animations', []):
        for sampler in animation['samplers']:
            for key in ['input', 'output']: sampler[key] = pack(sampler[key])
    assert not document.get('images') and not document.get('skins')
    document['accessors'] = accessors; document['bufferViews'] = packed_views
    document['buffers'] = [{'byteLength':len(payload)}]
    encoded = json.dumps(document, sort_keys=True, separators=(',',':')).encode()
    encoded += b' ' * (-len(encoded) % 4); payload.extend(b'\0' * (-len(payload) % 4))
    total = 28 + len(encoded) + len(payload)
    path.write_bytes(struct.pack('<III',0x46546c67,2,total) + struct.pack('<II',len(encoded),0x4e4f534a) + encoded + struct.pack('<II',len(payload),0x004e4942) + payload)
