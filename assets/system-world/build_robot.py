"""Optimize the existing ThreeDee robot for five city residents; retain its artwork."""
import bpy
import hashlib
import json
import math
import struct
from pathlib import Path

def canonical_tangents(data):
    # Blender emits zero degenerate-UV tangents and small float variations.
    # Use an orthogonal basis there and sub-visible precision for repeatable exports.
    data = bytearray(data)
    length = struct.unpack_from('<I', data, 12)[0]
    doc = json.loads(data[20:20+length])
    def accessor(index, width):
        item = doc['accessors'][index]
        assert item['componentType'] == 5126
        view = doc['bufferViews'][item['bufferView']]
        return 28 + length + view.get('byteOffset', 0) + item.get('byteOffset', 0), view.get('byteStride', width), item['count']
    for mesh in doc['meshes']:
        for primitive in mesh['primitives']:
            start, stride, count = accessor(primitive['attributes']['TANGENT'], 16)
            normals, normal_stride, _ = accessor(primitive['attributes']['NORMAL'], 12)
            for i in range(count):
                x, y, z, w = struct.unpack_from('<4f', data, start + i * stride)
                tangent = [round(v, 3) if math.isfinite(v) else 0.0 for v in (x, y, z)]
                norm = math.sqrt(sum(v * v for v in tangent))
                if not math.isfinite(norm) or norm < 1e-6:
                    nx, ny, nz = struct.unpack_from('<3f', data, normals + i * normal_stride)
                    tangent = [0, nz, -ny] if abs(nx) < .9 else [-nz, 0, nx]
                    norm = math.sqrt(sum(v * v for v in tangent))
                    if norm < 1e-6:
                        tangent, norm = [1, 0, 0], 1
                struct.pack_into('<4f', data, start + i * stride,
                    *(v / norm if v else 0.0 for v in tangent), -1 if w == -1 else 1)
    return bytes(data)


ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / 'ui/3d/robot.glb'
OUTPUT = ROOT / 'ui/3d/system-world/white-robot.glb'
bpy.ops.wm.read_factory_settings(use_empty=True)
bpy.ops.import_scene.gltf(filepath=str(SOURCE))
meshes = [o for o in bpy.context.scene.objects if o.type == 'MESH']
source_triangles = sum(len(o.data.polygons) for o in meshes)
for obj in meshes:
    bpy.context.view_layer.objects.active = obj
    modifier = obj.modifiers.new('City silhouette budget', 'DECIMATE')
    modifier.ratio = min(1, 24000 / source_triangles)
    modifier.use_collapse_triangulate = True
    bpy.ops.object.modifier_apply(modifier=modifier.name)
    for polygon in obj.data.polygons:
        polygon.use_smooth = True
# Original UVs and PBR maps are retained at the resolution needed in street view.
for image in bpy.data.images:
    if image.size[0] and max(image.size) > 512:
        factor = 512 / max(image.size)
        image.scale(max(1, round(image.size[0] * factor)), max(1, round(image.size[1] * factor)))
        image.pack()
OUTPUT.parent.mkdir(parents=True, exist_ok=True)
bpy.ops.export_scene.gltf(filepath=str(OUTPUT), export_format='GLB',
    export_draco_mesh_compression_enable=False, export_image_format='AUTO',
    export_animations=False, export_cameras=False, export_lights=False,
    export_extras=False, export_yup=True, export_tangents=True)
triangles = sum(len(o.data.polygons) for o in meshes)
data = canonical_tangents(OUTPUT.read_bytes())
OUTPUT.write_bytes(data)
assert triangles <= 25000, triangles
assert len(data) < 2 * 1024 * 1024, len(data)
manifest = dict(source='ui/3d/robot.glb',
    source_sha256=hashlib.sha256(SOURCE.read_bytes()).hexdigest(),
    provenance='Optimized derivative of the existing AuraGo ThreeDee robot; original artwork retained.',
    generator='Blender 5.2.1 / assets/system-world/build_robot.py',
    file=OUTPUT.name, sha256=hashlib.sha256(data).hexdigest(),
    bytes=len(data), triangles=triangles, source_triangles=source_triangles,
    texture_max_size=512)
OUTPUT.with_name('white-robot.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
print(json.dumps(manifest))
