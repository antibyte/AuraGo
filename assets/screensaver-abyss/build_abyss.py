"""Deterministic Blender generator for the Tiefsee (abyss) screensaver creatures.

Run:
    & 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/screensaver-abyss/build_abyss.py

Each creature is authored from parametric surfaces and exported as one compact GLB
into ui/3d/screensaver/abyss/v1/. Animation happens in the runtime vertex shaders,
driven by two baked UV sets:

    TEXCOORD_0 = (along, phase)   along: 0 at the attachment point -> 1 at the tip
                                  phase: stable per strand/row value in [0, 1)
    TEXCOORD_1 = (angle, side)    angle: azimuth / span coordinate in [0, 1]
                                  side : part specific (0 outer, 1 inner / belly)

Materials are named after the part so the runtime can assign its own shaders:
bell, gonad, arm, tentacle, comb_body, comb_row, manta_top, manta_belly.
"""

from pathlib import Path
from math import sin, cos, pi, atan2, sqrt
import hashlib
import json
import random
import struct

import bpy
import bmesh

ROOT = Path(__file__).resolve().parent
OUTPUT = ROOT.parents[1] / 'ui' / '3d' / 'screensaver' / 'abyss' / 'v1'
PRODUCTION = ROOT / 'production'
TAU = 2 * pi


def reset_scene():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    for block in (bpy.data.meshes, bpy.data.materials, bpy.data.objects):
        for item in list(block):
            block.remove(item)


def material(name):
    mat = bpy.data.materials.get(name) or bpy.data.materials.new(name)
    mat.use_nodes = True
    return mat


class Builder:
    """Collects vertices/faces with two UV sets and a material per face."""

    def __init__(self, name):
        self.name = name
        self.verts = []
        self.faces = []  # (indices, material, uv0 per corner, uv1 per corner)
        self.materials = []

    def mat_index(self, name):
        if name not in self.materials:
            self.materials.append(name)
        return self.materials.index(name)

    def grid(self, rows, cols, point, uv0, uv1, mat, wrap=False, flip=False):
        """Add a (rows+1) x (cols+1) surface; point(i, j) -> xyz; uv functions per vertex."""
        base = len(self.verts)
        ncols = cols if wrap else cols + 1
        for i in range(rows + 1):
            for j in range(ncols):
                self.verts.append(point(i, j))
        corner_uv0 = {}
        corner_uv1 = {}
        for i in range(rows + 1):
            for j in range(cols + 1):
                corner_uv0[(i, j)] = uv0(i, j)
                corner_uv1[(i, j)] = uv1(i, j)
        m = self.mat_index(mat)

        def vid(i, j):
            return base + i * ncols + (j % ncols if wrap else j)

        for i in range(rows):
            for j in range(cols):
                quad = [(i, j), (i, j + 1), (i + 1, j + 1), (i + 1, j)]
                if flip:
                    quad.reverse()
                self.faces.append(([vid(a, b) for a, b in quad], m,
                                   [corner_uv0[c] for c in quad], [corner_uv1[c] for c in quad]))

    def tube(self, path, radius, sides, mat, phase, extra=0.0):
        """Thin tube along a list of points; radius(t) tapers along the path."""
        segs = len(path) - 1
        frames = []
        for k, p in enumerate(path):
            a = path[max(0, k - 1)]
            b = path[min(segs, k + 1)]
            tangent = normalize(sub(b, a))
            ref = (0.0, 0.0, 1.0) if abs(tangent[2]) < 0.9 else (1.0, 0.0, 0.0)
            n1 = normalize(cross(tangent, ref))
            n2 = cross(tangent, n1)
            frames.append((n1, n2))

        def point(i, j):
            t = i / segs
            n1, n2 = frames[i]
            ang = TAU * j / sides
            r = radius(t)
            p = path[i]
            return (p[0] + (n1[0] * cos(ang) + n2[0] * sin(ang)) * r,
                    p[1] + (n1[1] * cos(ang) + n2[1] * sin(ang)) * r,
                    p[2] + (n1[2] * cos(ang) + n2[2] * sin(ang)) * r)

        self.grid(segs, sides, point,
                  lambda i, j: (i / segs, 1.0 - phase),
                  lambda i, j: (j / sides, 1.0 - extra),
                  mat, wrap=True)

    def build(self):
        mesh = bpy.data.meshes.new(self.name)
        bm = bmesh.new()
        bverts = [bm.verts.new(v) for v in self.verts]
        uv0 = bm.loops.layers.uv.new('UVMap')
        uv1 = bm.loops.layers.uv.new('Anim')
        for indices, m, c0, c1 in self.faces:
            if len(set(indices)) < 3:
                continue
            try:
                face = bm.faces.new([bverts[i] for i in indices])
            except ValueError:
                continue
            face.material_index = m
            face.smooth = True
            for loop, a, b in zip(face.loops, c0, c1):
                loop[uv0].uv = a
                loop[uv1].uv = b
        bm.to_mesh(mesh)
        bm.free()
        for name in self.materials:
            mesh.materials.append(material(name))
        obj = bpy.data.objects.new(self.name, mesh)
        bpy.context.scene.collection.objects.link(obj)
        return obj


def sub(a, b):
    return (a[0] - b[0], a[1] - b[1], a[2] - b[2])


def cross(a, b):
    return (a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0])


def normalize(v):
    length = sqrt(v[0] * v[0] + v[1] * v[1] + v[2] * v[2]) or 1.0
    return (v[0] / length, v[1] / length, v[2] / length)


# Blender is Z-up; the exporter converts to glTF Y-up. Creatures hang along -Z here.

def bell(b, rings, segments, height, lobes, lobe_depth, notch, flare, mat='bell'):
    def point(i, j):
        u = i / rings
        phi = TAU * j / segments
        r = sin(u * pi / 2) ** 0.82
        r *= 1.0 + lobe_depth * cos(lobes * phi) * u ** 3 + flare * u ** 6
        z = height * (1.0 - u ** 1.9)
        z -= notch * (1.0 - cos(2 * lobes * phi)) * u ** 7
        return (r * cos(phi), r * sin(phi), z)

    b.grid(rings, segments, point,
           lambda i, j: (i / rings, 1.0),
           lambda i, j: (j / segments, 1.0),
           mat, wrap=True)


def rim_point(rng, phi, lobes, lobe_depth, height):
    r = 1.0 + lobe_depth * cos(lobes * phi)
    return (r * cos(phi), r * sin(phi), 0.0 + height)


def hanging_path(start, length, segs, sway, rng, drift=0.25):
    ax = rng.uniform(-drift, drift)
    ay = rng.uniform(-drift, drift)
    freq = rng.uniform(1.5, 3.0)
    phase = rng.uniform(0, TAU)
    points = []
    for k in range(segs + 1):
        t = k / segs
        z = start[2] - length * t
        x = start[0] + ax * t * length * 0.4 + sway * sin(t * freq * pi + phase) * t
        y = start[1] + ay * t * length * 0.4 + sway * cos(t * freq * pi * 0.8 + phase) * t
        points.append((x, y, z))
    return points


def ribbon(b, path, width, ruffle, mat, phase, twist):
    """Frilly oral arm / curtain: a strip with ruffled edges along a path."""
    segs = len(path) - 1

    def point(i, j):
        t = i / segs
        p = path[i]
        side = j / 2.0 - 0.5
        ang = twist * t + phase * TAU
        w = width(t)
        edge = abs(side) * 2
        ruff = ruffle * sin(t * 26 + phase * 11 + side * 3) * edge
        return (p[0] + cos(ang) * side * w + sin(ang) * ruff,
                p[1] + sin(ang) * side * w - cos(ang) * ruff,
                p[2] + edge * 0.02)

    b.grid(segs, 2, point,
           lambda i, j: (i / segs, 1.0 - phase),
           lambda i, j: (j / 2.0, 0.0),
           mat)


def moon_jelly():
    rng = random.Random(1101)
    b = Builder('jelly_moon')
    bell(b, rings=14, segments=48, height=0.46, lobes=8, lobe_depth=0.035, notch=0.03, flare=0.04)
    # Four horseshoe gonads inside the bell.
    for g in range(4):
        base = g * TAU / 4 + pi / 4
        path = []
        for k in range(15):
            a = base + (k / 14 - 0.5) * pi * 1.45
            rr = 0.23 + 0.05 * sin((k / 14) * pi)
            path.append((cos(a) * rr + cos(base) * 0.16, sin(a) * rr + sin(base) * 0.16, 0.3))
        b.tube(path, lambda t: 0.034 * (0.7 + 0.3 * sin(t * pi)), 5, 'gonad', g / 4)
    # Four frilly oral arms.
    for a in range(4):
        ang = a * TAU / 4 + 0.3
        start = (cos(ang) * 0.05, sin(ang) * 0.05, 0.05)
        path = hanging_path(start, 1.15, 24, 0.12, rng, drift=0.3)
        ribbon(b, path, lambda t: 0.2 * (1 - t) + 0.04, 0.05, 'arm', a / 4, twist=2.4)
    # Fine marginal tentacles around the rim.
    count = 64
    for k in range(count):
        phi = TAU * (k + 0.5) / count
        start = rim_point(rng, phi, 8, 0.035, -0.01)
        length = rng.uniform(0.38, 0.62)
        path = hanging_path(start, length, 12, 0.05, rng, drift=0.35)
        b.tube(path, lambda t: 0.009 * (1.0 - 0.75 * t), 3, 'tentacle', (k * 0.618) % 1.0, extra=phi / TAU)
    return b.build()


def lion_jelly():
    rng = random.Random(2203)
    b = Builder('jelly_lion')
    bell(b, rings=14, segments=48, height=0.36, lobes=8, lobe_depth=0.065, notch=0.06, flare=0.02)
    # Oral curtains.
    for a in range(8):
        ang = a * TAU / 8
        start = (cos(ang) * 0.12, sin(ang) * 0.12, 0.02)
        path = hanging_path(start, 1.6, 28, 0.2, rng, drift=0.35)
        ribbon(b, path, lambda t: 0.26 * (1 - 0.7 * t) + 0.03, 0.08, 'arm', a / 8, twist=3.2)
    # Eight clusters of long tentacles.
    for cluster in range(8):
        base = cluster * TAU / 8 + TAU / 16
        for k in range(8):
            phi = base + (k / 7 - 0.5) * 0.42
            start = (cos(phi) * 0.78, sin(phi) * 0.78, -0.02)
            length = rng.uniform(2.4, 3.9)
            path = hanging_path(start, length, 20, 0.35, rng, drift=0.45)
            b.tube(path, lambda t: 0.011 * (1.0 - 0.8 * t), 3, 'tentacle', (cluster * 8 + k) * 0.1618 % 1.0, extra=phi / TAU % 1.0)
    return b.build()


def comb_jelly():
    b = Builder('jelly_comb')
    rings, segments = 22, 36

    def body(i, j):
        u = i / rings
        phi = TAU * j / segments
        lobe = 1.0 + 0.08 * cos(2 * phi) * sin(u * pi)
        r = 0.42 * sin(u * pi) ** 0.9 * lobe
        z = 0.55 - 1.2 * u
        return (r * cos(phi), r * 0.82 * sin(phi), z)

    b.grid(rings, segments, body,
           lambda i, j: (i / rings, 1.0),
           lambda i, j: (j / segments, 1.0),
           'comb_body', wrap=True)
    # Eight comb rows along meridians, slightly raised.
    for row in range(8):
        phi0 = TAU * (row + 0.5) / 8
        segs = 30

        def row_point(i, j, phi0=phi0, segs=segs):
            u = 0.12 + 0.72 * i / segs
            phi = phi0 + (j - 0.5) * 0.09
            lobe = 1.0 + 0.08 * cos(2 * phi) * sin(u * pi)
            r = (0.42 * sin(u * pi) ** 0.9 * lobe) + 0.014
            z = 0.55 - 1.2 * u
            return (r * cos(phi), r * 0.82 * sin(phi), z)

        b.grid(segs, 1, row_point,
               lambda i, j, segs=segs: (i / segs, 1.0 - row / 8),
               lambda i, j: (float(j), 0.0),
               'comb_row')
    return b.build()


def manta():
    b = Builder('manta')
    span_steps, chord_steps = 48, 16
    half_span = 1.75

    def leading(s):
        return 0.55 - 0.5 * abs(s) ** 1.25

    def chord(s):
        return 1.12 * (1.0 - abs(s) ** 1.35) + 0.04

    def thickness(s, v):
        return 0.19 * (1.0 - abs(s)) ** 1.7 * max(sin(pi * v), 0.0) ** 0.6 + 0.004

    def surface(top):
        def point(i, j):
            s = -1.0 + 2.0 * j / span_steps
            v = i / chord_steps
            x = s * half_span
            y = leading(s) - v * chord(s)
            tip_curl = 0.12 * abs(s) ** 3
            z = (thickness(s, v) if top else -0.55 * thickness(s, v)) + tip_curl
            return (x, y, z)
        return point

    for top in (True, False):
        b.grid(chord_steps, span_steps, surface(top),
               lambda i, j: (abs(-1.0 + 2.0 * j / span_steps), 1.0 - (0.0 if j < span_steps / 2 else 0.5)),
               lambda i, j: (i / chord_steps, 1.0 - (0.0 if top else 1.0)),
               'manta_top' if top else 'manta_belly', flip=not top)
    # Cephalic fins: two short curled lobes at the front.
    for side in (-1, 1):
        path = []
        for k in range(9):
            t = k / 8
            path.append((side * (0.16 + 0.05 * t), 0.56 + 0.18 * t, 0.02 - 0.12 * t * t))
        b.tube(path, lambda t: 0.045 * (1.0 - 0.6 * t), 6, 'manta_top', 0.0)
    # Whip-like tail.
    tail = [(0.0, -0.45 - 1.0 * k / 14, 0.01 - 0.03 * k / 14) for k in range(15)]
    b.tube(tail, lambda t: 0.025 * (1.0 - 0.9 * t), 5, 'manta_top', 0.0)
    return b.build()


def triangle_count(obj):
    return sum(len(p.vertices) - 2 for p in obj.data.polygons)


def bounds(obj):
    xs = [v.co.x for v in obj.data.vertices]
    ys = [v.co.y for v in obj.data.vertices]
    zs = [v.co.z for v in obj.data.vertices]
    # Report in glTF Y-up axes.
    return {'min': [round(min(xs), 4), round(min(zs), 4), round(-max(ys), 4)],
            'max': [round(max(xs), 4), round(max(zs), 4), round(-min(ys), 4)]}


def export(obj, filename, asset_id):
    bpy.ops.object.select_all(action='DESELECT')
    obj.select_set(True)
    bpy.context.view_layer.objects.active = obj
    obj['asset_id'] = asset_id
    output = OUTPUT / filename
    bpy.ops.export_scene.gltf(filepath=str(output), export_format='GLB', use_selection=True,
                              use_active_scene=True, export_animations=False, export_cameras=False,
                              export_lights=False, export_extras=True, export_texcoords=True,
                              export_normals=True, export_tangents=False, export_yup=True,
                              export_apply=True, export_materials='EXPORT')
    data = output.read_bytes()
    json_length = struct.unpack_from('<I', data, 12)[0]
    document = json.loads(data[20:20 + json_length])
    for index, node in enumerate(document.get('nodes', [])):
        node['name'] = asset_id if index == 0 else f'{asset_id}{index}'
    for index, mesh in enumerate(document.get('meshes', [])):
        mesh['name'] = f'{asset_id}_mesh{index}'
    for scene in document.get('scenes', []):
        scene['name'] = 'AbyssAsset'
    document.get('asset', {}).pop('generator', None)
    payload = json.dumps(document, separators=(',', ':'), sort_keys=True).encode()
    payload += b' ' * (-len(payload) % 4)
    tail = data[20 + json_length:]
    data = struct.pack('<III', 0x46546C67, 2, 20 + len(payload) + len(tail))
    data += struct.pack('<II', len(payload), 0x4E4F534A) + payload + tail
    output.write_bytes(data)
    return {'id': asset_id, 'file': filename, 'bytes': len(data),
            'sha256': hashlib.sha256(data).hexdigest(),
            'triangles': triangle_count(obj), 'bounds': bounds(obj)}


def main():
    reset_scene()
    OUTPUT.mkdir(parents=True, exist_ok=True)
    PRODUCTION.mkdir(parents=True, exist_ok=True)
    creatures = [
        (moon_jelly(), 'jelly-moon.glb', 'jelly_moon'),
        (lion_jelly(), 'jelly-lion.glb', 'jelly_lion'),
        (comb_jelly(), 'jelly-comb.glb', 'jelly_comb'),
        (manta(), 'manta.glb', 'manta'),
    ]
    assets = [export(obj, filename, asset_id) for obj, filename, asset_id in creatures]
    source = Path(__file__).read_bytes()
    manifest = {
        'kit': 'aurago-screensaver-abyss',
        'version': 1,
        'license': 'MIT',
        'generator': 'assets/screensaver-abyss/build_abyss.py',
        'generator_sha256': hashlib.sha256(source.replace(b'\r\n', b'\n')).hexdigest(),
        'uv0': 'along, phase',
        'uv1': 'angle, side',
        'assets': assets,
    }
    (OUTPUT / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
    bpy.ops.wm.save_as_mainfile(filepath=str(PRODUCTION / 'aurago-abyss.blend'))
    print('ABYSS_EXPORT', json.dumps([{k: a[k] for k in ('file', 'bytes', 'triangles')} for a in assets]))


main()
