"""Sprite atlas of the residents and the furniture seen behind the interior-mapped windows
(sysworld-windows.js).

The window shader draws rooms without geometry, so everything in them is a flat card. This script
renders the real resident robots (robot_assets.py, LOD 0, their own rig and clips) and furniture
groups made of the lobby kit's pieces (interior_assets3.py) with an orthographic camera into one
atlas, as light-independent data the shader relights per room:

    R = sqrt(luminance under the ceiling lamp)      (lamp from above, weak room bounce)
    G = sqrt(luminance under the window daylight)   (broad light from the glass side)
    B = accent mask: 0 body, .5 accent-coloured part, above .5 a part that glows (eyes, strips, screens)
    A = coverage (straight alpha; the colour channels are dilated into the empty area, so
        bilinear filtering and mipmaps do not pull a dark fringe into the edges)

Accent and glowing parts are rendered neutral and tinted by the shader, so one render serves any
colour. The atlas has two blocks:

    residents  COLS x ROWS cells of CELL_W x CELL_H metres, seen level; the feet stand FEET metres
               above the lower cell edge
    furniture  below them, F_COLS x F_ROWS cells of F_SIZE x F_SIZE metres, seen from F_TILT degrees
               above so table tops show; the floor line under the group's middle lies F_FLOOR
               metres above the lower cell edge

    blender -b --factory-startup --python assets/system-world/build_window_life.py -- [--out DIR]
        [--preview FILE] [--samples N] [--keep]
    python -I assets/system-world/build_window_life.py --check     # committed bytes match the manifest

Cycles output is not byte-stable across machines and drivers; `--check` therefore verifies the
committed atlas against the hash in its manifest, it does not render again.
"""
import hashlib
import json
import struct
import sys
import zlib
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'ui' / '3d' / 'system-world' / 'textures' / 'v1'
NAME = 'window-life'
COLS, ROWS = 6, 4
CELL_PX = (288, 432)           # 180 pixels per metre
CELL_W, CELL_H, FEET = 1.6, 2.4, .06
WALK = tuple(range(0, 24, 3))        # one gait cycle is 24 frames
# (kind, clip, frame, yaw in degrees: 0 faces the glass, +90 walks to the right, 180 faces the room)
CELLS = (
    [('technician', 'walk', f, 78) for f in WALK]
    + [('courier', 'carry', f, 78) for f in WALK]
    + [('technician', 'idle', 0, 18), ('courier', 'idle', 0, -18), ('archivist', 'idle', 0, -12),
       ('archivist', 'greet', 12, -12), ('archivist', 'greet', 16, -12), ('archivist', 'greet', 20, -12),
       ('technician', 'work', 0, 118), ('technician', 'work', 12, 118)]
)
assert len(CELLS) == COLS*ROWS
# Furniture block: it stands deeper in the room than the residents, so 128 pixels per metre are enough.
F_COLS, F_ROWS = 6, 2
F_PX = 288
F_SIZE, F_FLOOR, F_TILT = 2.25, .22, 12
# name, layer (mid: in the room, back: against the back wall), lobby only, body tinted as foliage
FURNITURE = (
    ('desk-screen', 'mid', False, False), ('desk-lamp', 'mid', False, False), ('console', 'mid', False, False),
    ('meeting', 'mid', False, False), ('rack', 'back', False, False), ('archive', 'back', False, False),
    ('plant', 'back', False, True), ('plant-lamp', 'back', False, True), ('sofa', 'back', False, False),
    ('lounge', 'mid', True, False), ('totem', 'back', True, False), ('reception', 'mid', True, False),
)
assert len(FURNITURE) == F_COLS*F_ROWS


def sha256(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def check(out):
    manifest = json.loads((out/f'{NAME}.json').read_text(encoding='utf-8'))
    path = out/manifest['file']
    actual = (path.stat().st_size, sha256(path))
    expected = (manifest['bytes'], manifest['sha256'])
    if actual != expected:
        raise SystemExit(f'{path.name}: {actual} does not match the manifest {expected}')
    print(f'System World window life verified: {manifest["cells"]["columns"]}x{manifest["cells"]["rows"]} resident cells, '
          f'{len(manifest["furniture"]["items"])} furniture cells, {actual[0]} bytes')


def write_png(path, rgba):
    """8-bit RGBA PNG, rows top to bottom; written by hand so empty pixels keep their colour."""
    import numpy as np
    height, width, _ = rgba.shape
    rows = rgba.reshape(height, width*4).astype(np.int16)
    left = np.concatenate([np.zeros((height, 4), np.int16), rows[:, :-4]], 1)
    above = np.concatenate([np.zeros((1, width*4), np.int16), rows[:-1]], 0)
    # Row filters 0 (none), 1 (sub), 2 (up) and 3 (average); the smallest residual per row wins.
    filtered = np.stack([rows, rows-left, rows-above, rows-(left+above)//2]).astype(np.uint8)
    cost = np.abs(filtered.astype(np.int8).astype(np.int16)).sum(2)
    pick = cost.argmin(0)
    raw = b''.join(bytes([int(pick[y])]) + filtered[pick[y], y].tobytes() for y in range(height))

    def chunk(tag, data):
        body = tag + data
        return struct.pack('>I', len(data)) + body + struct.pack('>I', zlib.crc32(body) & 0xffffffff)
    Path(path).write_bytes(b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', width, height, 8, 6, 0, 0, 0))
                           + chunk(b'IDAT', zlib.compress(raw, 9)) + chunk(b'IEND', b''))


def dilate(values, alpha, rounds=8):
    """Spread colour into the empty area, one pixel per round, from the covered neighbours; what is
    still empty then takes the figure's mean colour (flat, so it costs no bytes and small mipmaps
    do not darken)."""
    import numpy as np
    filled = alpha > .02
    out = np.where(filled[..., None], values, 0).astype(np.float32)
    for _ in range(rounds):
        if filled.all(): break
        total = np.zeros_like(out)
        count = np.zeros(filled.shape, np.float32)
        for dy, dx in ((0, 1), (0, -1), (1, 0), (-1, 0), (1, 1), (1, -1), (-1, 1), (-1, -1)):
            src_f = np.roll(filled, (dy, dx), (0, 1))
            if dy == 1: src_f[0] = False
            if dy == -1: src_f[-1] = False
            if dx == 1: src_f[:, 0] = False
            if dx == -1: src_f[:, -1] = False
            total += np.roll(out, (dy, dx), (0, 1))*src_f[..., None]
            count += src_f
        grow = (~filled) & (count > 0)
        out[grow] = total[grow]/count[grow][..., None]
        filled = filled | grow
    if not filled.all() and (alpha > .5).any():
        out[~filled] = np.round(values[alpha > .5].mean(0)*255)/255
    return out


def render(out, preview, samples, keep):
    import bmesh
    import bpy
    import numpy as np
    from math import radians, sin, cos
    from mathutils import Matrix, Vector
    sys.path.insert(0, str(Path(__file__).parent))
    from build_city import PALETTE
    from robot_assets import (ROBOT_PALETTE, JOINTS, V, T_CYCLE, create_robot_asset, clip_poses)
    import kit3
    from interior_assets3 import FLOOR, sofa, low_table, pot_plant, floor_lamp, totem, cabinet, counter, table
    from furniture_assets3 import blob

    work = ROOT/'reports'/'system-world-assets'/'window-life'
    work.mkdir(parents=True, exist_ok=True)
    scene = bpy.data.scenes.new('Window life')
    bpy.context.window.scene = scene
    scene.render.engine = 'CYCLES'
    scene.cycles.samples = samples
    scene.cycles.use_denoising = False
    scene.cycles.max_bounces = 4
    scene.cycles.filter_width = 1.5
    preferences = bpy.context.preferences.addons['cycles'].preferences
    try:
        preferences.compute_device_type = 'ONEAPI'
        preferences.get_devices()
        for device in preferences.devices: device.use = device.type == 'ONEAPI'
        scene.cycles.device = 'GPU' if any(d.use for d in preferences.devices) else 'CPU'
    except Exception:
        scene.cycles.device = 'CPU'
    scene.render.use_persistent_data = True
    scene.render.film_transparent = True
    scene.render.resolution_percentage = 100
    scene.render.image_settings.file_format = 'OPEN_EXR'
    scene.render.image_settings.color_mode = 'RGBA'
    scene.render.image_settings.color_depth = '32'
    scene.world = bpy.data.worlds.new('Window life world')
    scene.world.use_nodes = True
    background = scene.world.node_tree.nodes.get('Background')
    background.inputs['Color'].default_value = (.5, .5, .5, 1)

    camera_data = bpy.data.cameras.new('Window life camera')
    camera_data.type = 'ORTHO'
    camera_data.clip_start, camera_data.clip_end = .1, 30
    camera = bpy.data.objects.new('Window life camera', camera_data)
    scene.collection.objects.link(camera)
    scene.camera = camera

    def frame_residents():
        scene.render.resolution_x, scene.render.resolution_y = CELL_PX
        camera_data.ortho_scale = CELL_H
        camera.location, camera.rotation_euler = (0, -8, CELL_H/2-FEET), (radians(90), 0, 0)

    def frame_furniture():
        # Seen from F_TILT degrees above; the floor point under the group's middle lands F_FLOOR above the lower edge.
        scene.render.resolution_x = scene.render.resolution_y = F_PX
        camera_data.ortho_scale = F_SIZE
        tilt = radians(F_TILT)
        target = Vector((0, 0, (F_SIZE/2-F_FLOOR)/cos(tilt)))
        camera.location = target-8*Vector((0, cos(tilt), -sin(tilt)))
        camera.rotation_euler = (radians(90)-tilt, 0, 0)

    def area(name, location, target, power, size):
        data = bpy.data.lights.new(name, 'AREA')
        data.energy, data.shape, data.size, data.size_y = power, 'RECTANGLE', size[0], size[1]
        obj = bpy.data.objects.new(name, data)
        scene.collection.objects.link(obj)
        obj.location = location
        obj.rotation_euler = (Vector(target)-obj.location).to_track_quat('-Z', 'Y').to_euler()
        return obj
    lamp = area('Ceiling lamp', (.1, -.55, 2.95), (0, 0, 1.2), 90, (1.1, .8))
    window = area('Window light', (1.5, -3.4, 2.0), (0, 0, 1.15), 420, (2.6, 2.6))

    def material(name, color, metal, rough, tinted=False):
        """Principled material; `tinted` multiplies the colour by the kit's per-face `tint` attribute."""
        mat = bpy.data.materials.new(name)
        mat.use_nodes = True
        tree = mat.node_tree
        bsdf = tree.nodes.get('Principled BSDF')
        bsdf.inputs['Base Color'].default_value = color
        bsdf.inputs['Metallic'].default_value = metal
        bsdf.inputs['Roughness'].default_value = rough
        if tinted:
            attribute, scale = tree.nodes.new('ShaderNodeAttribute'), tree.nodes.new('ShaderNodeVectorMath')
            attribute.attribute_type, attribute.attribute_name = 'GEOMETRY', 'tint'
            scale.operation = 'SCALE'
            scale.inputs[0].default_value = color[:3]
            tree.links.new(attribute.outputs['Fac'], scale.inputs['Scale'])
            tree.links.new(scale.outputs['Vector'], bsdf.inputs['Base Color'])
        return mat

    def emitter(name, value):
        mat = bpy.data.materials.new(name)
        mat.use_nodes = True
        tree = mat.node_tree
        tree.nodes.clear()
        node, output = tree.nodes.new('ShaderNodeEmission'), tree.nodes.new('ShaderNodeOutputMaterial')
        node.inputs['Color'].default_value = (value, value, value, 1)
        node.inputs['Strength'].default_value = 1
        tree.links.new(node.outputs['Emission'], output.inputs['Surface'])
        return mat
    accent = ('bronze', 'warm', 'ivory', 'cyan', 'jade')
    grey = (.78, .78, .78, 1)

    # -- residents ----------------------------------------------------------------------------
    neutral, mask = {}, {}
    for key, (color, metal, rough, emission) in {**PALETTE, **ROBOT_PALETTE}.items():
        if key in accent:
            neutral[key] = material('wl.n.'+key, grey, .15, max(rough, .3))
            mask[key] = emitter('wl.m.'+key, .5+.5*min(1, emission/2.1))
        else:
            # Behind glass the pale shell has to carry the figure: a lighter, less metallic ceramic.
            neutral[key] = (material('wl.n.'+key, (.8, .82, .8, 1), .08, .42) if key == 'ceramic'
                            else material('wl.n.'+key, color, metal, rough))
            mask[key] = emitter('wl.m.'+key, 0)
    robots = {}
    for kind in ('technician', 'courier', 'archivist'):
        for variant, mats in (('lit', neutral), ('mask', mask)):
            collection = bpy.data.collections.new(f'wl.{kind}.{variant}')
            scene.collection.children.link(collection)
            root = create_robot_asset(f'wl-{kind}-{variant}', kind, 0, collection, mats)
            objects = {o['component']: o for o in root.children_recursive}
            robots[kind, variant] = (root, objects, {n: o.location.copy() for n, o in objects.items()})

    def pose(kind, variant, clip, frame, yaw):
        root, objects, rest = robots[kind, variant]
        p = clip_poses(kind, clip)[frame]
        for name in JOINTS+(['parcel'] if kind == 'courier' else []):
            obj = objects[name]
            obj.rotation_euler = p.rot.get(name, Matrix.Identity(3)).to_euler('XYZ')
            obj.location = rest[name]+p.loc.get(name, Vector())
        root.rotation_euler = (0, 0, radians(yaw))

    # -- furniture ----------------------------------------------------------------------------
    # The kit's dark graphite would only be a silhouette behind glass: a lighter, duller one here.
    lighter = {'graphite': ((.11, .13, .15, 1), .35, .42), 'leaf': ((.34, .36, .34, 1), .0, .6),
               'road': ((.1, .09, .08, 1), .0, .8)}
    f_neutral, f_mask = {}, {}
    for key, (color, metal, rough, emission, _alpha) in kit3.PALETTE.items():
        if key in accent:
            f_neutral[key] = material('wf.n.'+key, grey, .15, max(rough, .3), True)
            f_mask[key] = emitter('wf.m.'+key, .5+.5*min(1, emission/1.3))
        else:
            color, metal, rough = lighter.get(key, (color, metal, rough))
            f_neutral[key] = material('wf.n.'+key, color, metal, rough, True)
            f_mask[key] = emitter('wf.m.'+key, 0)

    def desk_screen(g):
        z = FLOOR
        table(g, 0, 0, 1.5, .7, 0, lamp=False)
        g.box((0, .14, z + .775), (.24, .16, .02), 'graphite', .005, 1)
        g.box((0, .16, z + .88), (.04, .03, .2), 'graphite', 0)
        g.box((0, .16, z + 1.13), (.66, .03, .4), 'graphite', .01, 1)
        g.box((0, .142, z + 1.13), (.6, .012, .34), 'cyan', 0, tint=.6)
        g.box((-.04, -.16, z + .775), (.44, .14, .02), 'titanium', .005, 1)
        g.box((.52, .06, z + .83), (.18, .24, .12), 'ceramic', .01, 1)
        g.box((.52, .06, z + .9), (.14, .2, .02), 'bronze', 0)

    def desk_lamp(g):
        z = FLOOR
        table(g, 0, 0, 1.35, .7, 0, lamp=False)
        g.cyl((-.48, .1, z + .775), .09, .02, 'graphite', n=14)
        g.cyl((-.48, .1, z + .98), .012, .4, 'bronze', n=6, caps=(False, False))
        blob(g, (-.48, .1, z + 1.22), .11, 'ivory', squash=.8, n=12, steps=6)
        g.box((.12, -.02, z + .775), (.4, .28, .016), 'graphite', .004, 1)
        g.box((.12, .13, z + .9), (.4, .014, .25), 'graphite', .004, 1)
        g.box((.12, .121, z + .9), (.36, .008, .21), 'ivory', 0, tint=.7)
        g.cyl((0, .7, z + .25), .03, .5, 'graphite', n=6, caps=(False, False))
        g.cyl((0, .7, z + .5), .18, .05, 'titanium', n=12)

    def reception(g):
        counter(g, -.28, 0, 1.5, 3.14159265, screens=1, glow='jade')
        floor_lamp(g, .86, .1)

    def plant_lamp(g):
        floor_lamp(g, -.5, .1)
        pot_plant(g, .32, 0, .85)

    def lounge(g):
        sofa(g, 0, .3, 1.9, 0)
        low_table(g, 0, -.5, .4)

    factories = {
        'desk-screen': desk_screen, 'desk-lamp': desk_lamp,
        'console': lambda g: counter(g, 0, 0, 1.7, 3.14159265, screens=2),
        'meeting': lambda g: table(g, 0, 0, 1.8, .8, 0, lamp=True, stools=2),
        'rack': lambda g: cabinet(g, 0, 0, 0, (1.5, .45, 1.8)),
        'archive': lambda g: cabinet(g, 0, 0, 0, (1.85, .4, 1.45), lights=('ivory', 'cyan'), rows=3),
        'plant': lambda g: pot_plant(g, 0, 0, 1.1), 'plant-lamp': plant_lamp,
        'sofa': lambda g: sofa(g, 0, 0, 1.95, 0), 'lounge': lounge,
        'totem': lambda g: totem(g, 0, 0, 0, height=1.72), 'reception': reception,
    }

    def kit_objects(name, factory, mats, plant):
        """One furniture group as Blender meshes (kit3.create_asset without the occlusion bake); the floor is z = 0."""
        kit = kit3.Kit(0)
        factory(kit)
        collection = bpy.data.collections.new(name)
        scene.collection.children.link(collection)
        objects = []
        for part, data in kit.parts.items():
            # A plant's pot takes the accent colour, so the body tint can be the foliage green.
            keys = ['bronze' if plant and key == 'ceramic' else key for key in data['mats']]
            used = list(dict.fromkeys(keys))
            mesh = bpy.data.meshes.new(f'{name}.{part}')
            for key in used: mesh.materials.append(mats[key])
            bm = bmesh.new()
            tint_layer = bm.faces.layers.float.new('tint')
            verts = [bm.verts.new(v) for v in data['verts']]
            for face, key, tint in zip(data['faces'], keys, data['tints']):
                corners = [verts[i] for i in face]
                if len(set(corners)) < 3: continue
                try:
                    f = bm.faces.new(corners)
                except ValueError:
                    continue
                f.material_index, f[tint_layer], f.smooth = used.index(key), tint, True
            bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)
            bm.normal_update()
            bm.to_mesh(mesh)
            bm.free()
            if part != 'foliage': mesh.set_sharp_from_angle(angle=radians(kit3.SHARP_ANGLE))
            mesh.update()
            obj = bpy.data.objects.new(f'{name}.{part}', mesh)
            obj.location = (0, 0, -FLOOR)
            collection.objects.link(obj)
            objects.append(obj)
        return objects
    furniture = {}
    for name, _layer, _lobby, plant in FURNITURE:
        for variant, mats in (('lit', f_neutral), ('mask', f_mask)):
            furniture[name, variant] = kit_objects(f'wf.{name}.{variant}', factories[name], mats, plant)

    def show(key):
        for other, (_root, objects, _rest) in robots.items():
            for obj in objects.values(): obj.hide_render = other != key
        for other, objects in furniture.items():
            for obj in objects: obj.hide_render = other != key

    def shot(tag, size):
        path = work/f'{tag}.exr'
        scene.render.filepath = str(path)
        bpy.ops.render.render(write_still=True)
        image = bpy.data.images.load(str(path))
        data = np.empty(size[0]*size[1]*4, np.float32)
        image.pixels.foreach_get(data)
        bpy.data.images.remove(image)
        if not keep: path.unlink()
        return data.reshape(size[1], size[0], 4)[::-1]       # top row first

    passes = (('lamp', 'lit', (False, True), .2), ('window', 'lit', (True, False), .14), ('mask', 'mask', (True, True), 0))
    layers = {}
    for tag, variant, (hide_lamp, hide_window), ambient in passes:
        lamp.hide_render, window.hide_render = hide_lamp, hide_window
        background.inputs['Strength'].default_value = ambient
        scene.cycles.use_denoising = variant == 'lit'      # the mask is exact data, never denoised
        cells = []
        frame_residents()
        for index, (kind, clip, frame, yaw) in enumerate(CELLS):
            show((kind, variant))
            pose(kind, variant, clip, frame, yaw)
            cells.append(shot(f'{tag}-{index:02d}', CELL_PX))
        frame_furniture()
        for name, _layer, _lobby, _plant in FURNITURE:
            show((name, variant))
            cells.append(shot(f'{tag}-{name}', (F_PX, F_PX)))
        layers[tag] = cells
        print(f'window life: pass {tag} rendered', flush=True)

    weights = np.array([.2126, .7152, .0722], np.float32)
    count = len(CELLS)

    def straight(cell):
        alpha = np.clip(cell[..., 3], 0, 1)
        return np.where(alpha[..., None] > 1e-4, cell[..., :3]/np.maximum(alpha[..., None], 1e-4), 0), alpha
    luma, alphas = {}, []
    for tag in ('lamp', 'window'):
        values = []
        for cell in layers[tag]:
            rgb, alpha = straight(cell)
            values.append(rgb @ weights)
            if tag == 'lamp': alphas.append(alpha)
        # One scale per light from the residents (their brightest 3 % may clip, the lit shell lands near its
        # own albedo); the furniture takes the same scale, so both answer the room light alike.
        solid = np.concatenate([v[a > .99] for v, a in zip(values[:count], alphas[:count])])
        scale = .9/max(float(np.percentile(solid, 97)), 1e-6)
        luma[tag] = [np.sqrt(np.clip(v*scale, 0, 1)) for v in values]
    masks = [np.clip(straight(cell)[0][..., 0], 0, 1) for cell in layers['mask']]

    top = ROWS*CELL_PX[1]
    atlas = np.zeros((top+F_ROWS*F_PX, COLS*CELL_PX[0], 4), np.uint8)
    assert F_COLS*F_PX == atlas.shape[1]
    for index in range(count+len(FURNITURE)):
        alpha = alphas[index]
        data = dilate(np.stack([luma['lamp'][index], luma['window'][index], masks[index]], -1), alpha)
        cell = np.round(np.concatenate([data, alpha[..., None]], -1)*255).astype(np.uint8)
        edge = np.concatenate([alpha[0], alpha[-1], alpha[:, 0], alpha[:, -1]])
        label = CELLS[index] if index < count else FURNITURE[index-count][0]
        assert edge.max() < .02, f'cell {index} {label} touches its border'
        if index < count:
            y, x = divmod(index, COLS)
            atlas[y*CELL_PX[1]:(y+1)*CELL_PX[1], x*CELL_PX[0]:(x+1)*CELL_PX[0]] = cell
        else:
            y, x = divmod(index-count, F_COLS)
            atlas[top+y*F_PX:top+(y+1)*F_PX, x*F_PX:(x+1)*F_PX] = cell
    out.mkdir(parents=True, exist_ok=True)
    path = out/f'{NAME}.png'
    write_png(path, atlas)
    manifest = {
        'file': path.name, 'bytes': path.stat().st_size, 'sha256': sha256(path), 'license': 'MIT',
        'generator': f'Blender {bpy.app.version_string} / assets/system-world/build_window_life.py',
        'size': [atlas.shape[1], atlas.shape[0]],
        'channels': {'r': 'sqrt luminance, ceiling lamp', 'g': 'sqrt luminance, window daylight',
                     'b': 'accent mask (0 body, .5 accent, 1 glowing)', 'a': 'coverage, straight'},
        'cells': {'columns': COLS, 'rows': ROWS, 'pixels': list(CELL_PX), 'metres': [CELL_W, CELL_H], 'feet': FEET,
                  'order': 'cell i sits in column i % columns of row floor(i / columns); row 0 is the top row of the image'},
        'stride': round(V*T_CYCLE, 4),
        'sets': {'walk': {'first': 0, 'count': 8, 'kind': 'technician'},
                 'carry': {'first': 8, 'count': 8, 'kind': 'courier'},
                 'idleWalk': {'first': 16, 'count': 1, 'kind': 'technician'},
                 'idleCarry': {'first': 17, 'count': 1, 'kind': 'courier'},
                 'idle': {'first': 18, 'count': 1, 'kind': 'archivist'},
                 'greet': {'first': 19, 'count': 3, 'kind': 'archivist'},
                 'work': {'first': 22, 'count': 2, 'kind': 'technician'}},
        'furniture': {'origin': [0, top], 'columns': F_COLS, 'rows': F_ROWS, 'pixels': [F_PX, F_PX],
                      'metres': [F_SIZE, F_SIZE], 'floor': F_FLOOR, 'tilt': F_TILT,
                      'order': 'item i sits in column i % columns of row floor(i / columns) below the resident cells',
                      'items': [{'name': name, 'layer': layer, 'lobby': lobby, 'foliage': plant}
                                for name, layer, lobby, plant in FURNITURE]},
    }
    (out/f'{NAME}.json').write_text(json.dumps(manifest, indent=2)+'\n', encoding='utf-8')
    print(f'window life: {path} {manifest["bytes"]} bytes', flush=True)

    if preview:
        # The shader's composition on wall tones: night (lamp), day (window light only) and the mask.
        data = atlas.astype(np.float32)/255
        lit_lamp, lit_window, m, a = data[..., 0]**2, data[..., 1]**2, data[..., 2], data[..., 3:4]
        tint = np.zeros(atlas.shape[:2]+(3,), np.float32)
        tint[:top] = (.1, .75, .9)
        tint[top:] = (.62, .36, .2)                                  # furniture accent: warm wood
        glow_colour = np.zeros_like(tint)
        glow_colour[:top] = (.1, .75, .9)
        glow_colour[top:] = (.55, .85, 1.0)
        body = np.ones_like(tint)
        for i, (_name, _layer, _lobby, plant) in enumerate(FURNITURE):
            if plant:
                y, x = divmod(i, F_COLS)
                body[top+y*F_PX:top+(y+1)*F_PX, x*F_PX:(x+1)*F_PX] = (.36, .8, .42)
        accent_mix, glow = np.minimum(2*m, 1)[..., None], np.maximum(2*m-1, 0)[..., None]
        base = body+(tint-body)*accent_mix
        base = base+(glow_colour-base)*glow
        sheets = []
        for wall, lamp_l, fill in (((.62, .56, .48), np.array([.64, .51, .36]), .02), ((.4, .48, .55), np.zeros(3), .2)):
            light = lit_lamp[..., None]*lamp_l+lit_window[..., None]*fill*.55*1.8
            figure = base*light+glow*glow_colour*1.0
            back = np.array(wall, np.float32)*(lamp_l*.75+fill*.3)
            sheets.append(figure*a+back*(1-a))
        sheet = np.concatenate(sheets+[np.repeat(m[..., None], 3, -1)*a], 1)
        sheet = np.clip(sheet, 0, 1)**(1/2.2)
        rgba = np.concatenate([sheet, np.ones_like(sheet[..., :1])], -1)
        Path(preview).parent.mkdir(parents=True, exist_ok=True)
        write_png(preview, np.round(rgba*255).astype(np.uint8))
        print(f'window life: preview {preview}', flush=True)


def main():
    argv = sys.argv[sys.argv.index('--')+1:] if '--' in sys.argv else sys.argv[1:]
    out, preview, samples, keep, verify = OUT, None, 160, False, False
    i = 0
    while i < len(argv):
        arg = argv[i]
        if arg == '--check': verify = True
        elif arg == '--keep': keep = True
        elif arg == '--out': i += 1; out = Path(argv[i])
        elif arg == '--preview': i += 1; preview = Path(argv[i])
        elif arg == '--samples': i += 1; samples = int(argv[i])
        else: raise SystemExit(f'unknown argument {arg}')
        i += 1
    if verify: return check(out)
    render(out, preview, samples, keep)


if __name__ == '__main__':
    main()
