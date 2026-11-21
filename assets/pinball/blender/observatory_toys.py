"""Clockwork Observatory hero toys for AuraPinball (Blender 4.2+/5.x, background mode).

Run from the repo root:
  blender --background --factory-startup --python assets/pinball/blender/observatory_toys.py -- \
      --out assets/pinball/models/observatory-toys.glb [--params params.json] [--preview reports/pinball-toys]

Conventions (the game relies on them, see ui/js/desktop/apps/pinball/AGENTS.md):
  * Metres. Blender X = table x, Y = table y (towards the backbox), Z = height above the playfield.
    The glTF exporter (+Y up) turns that into the game's tableRoot space (X = x, Y = h, Z = -y).
  * Every toy is one root empty named after its kind (orrery, pendulum, telescope, gears, dome) with its origin at
    the toy's base point. The game clones the root, places it and animates the named children listed in ANIMATED.
    Nothing is animated in the file itself.
  * Materials are plain Principled BSDF values (no textures). Emissive materials are named glow_*, glass glass_*.
  * --params overrides entries of P (JSON object), so the models follow the table layout numbers.
"""
import json
import math
import os
import sys

import bmesh
import bpy
from mathutils import Matrix, Vector

# name -> what the game does with it (glTF axes). Checked at build time and by the asset test.
ANIMATED = {
    'orrery_arm_1': 'rotation.y, orbit (fastest)', 'orrery_arm_2': 'rotation.y', 'orrery_arm_3': 'rotation.y',
    'orrery_arm_4': 'rotation.y (slowest)', 'orrery_rings': 'rotation.y, slow tumble',
    'gear_1': 'rotation.y', 'gear_2': 'rotation.y', 'gear_3': 'rotation.y', 'gear_4': 'rotation.y', 'gear_5': 'rotation.y',
    'pendulum_arm': 'rotation.z = theta (origin is the pivot; positive swings the bob towards +x)',
    'clock_hand_hour': 'rotation.z', 'clock_hand_minute': 'rotation.z',
    'dome_shutter': 'rotation.y, 0 closed .. 0.5 open',
    'telescope_barrel': 'placed and aimed by the game along the telescope tube path (local -Z is the bore axis)',
}
GEAR_RATIOS = (1.0, -1.76, 1.48, -2.37, -1.28)

P = {
    # Orrery: stands on the hub post in the middle of the rotating disc; the arms clear a ball (27 mm) below them.
    'orrery_hub_r': 0.0052, 'orrery_arm_z': 0.040, 'orrery_arm_step': 0.0038,
    'orrery_radii': [0.014, 0.021, 0.028, 0.034], 'orrery_planets': [0.0036, 0.0044, 0.0038, 0.0054], 'orrery_top': 0.086,
    # Pendulum: from the layout device (pivotH, length, bobR); the posts stand at (+-post_dx, post_dy) from the pivot.
    'pend_pivot_h': 0.0885, 'pend_len': 0.075, 'pend_bob_r': 0.011, 'pend_post_dx': 0.024, 'pend_post_dy': 0.017,
    # Dome and gear train: built at design size, then scaled to fit the back corners.
    'dome_r': 0.036, 'gears_scale': 0.70,
    # Telescope: barrel length along the bore, outer radius; where the bore axis starts relative to the pier base
    # (preview placement only, the game places the barrel) and its pitch.
    'tele_len': 0.121, 'tele_r': 0.0225, 'tele_bore': 0.0203, 'tele_pitch': math.radians(26.5), 'tele_yaw': math.radians(16.3),
    'tele_start': [-0.0025, -0.052, 0.0255], 'tele_pier_h': 0.024, 'tele_pier_r': 0.022,
}

MATS = {}


def mat(name, color, metallic=0.0, rough=0.5, emission=None, strength=0.0, alpha=1.0):
    if name in MATS:
        return MATS[name]
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    b = m.node_tree.nodes.get('Principled BSDF')
    b.inputs['Base Color'].default_value = (*color, 1.0)
    b.inputs['Metallic'].default_value = metallic
    b.inputs['Roughness'].default_value = rough
    if emission is not None:
        b.inputs['Emission Color'].default_value = (*emission, 1.0)
        b.inputs['Emission Strength'].default_value = strength
    if alpha < 1.0:
        b.inputs['Alpha'].default_value = alpha
        if hasattr(m, 'surface_render_method'):
            m.surface_render_method = 'BLENDED'
        elif hasattr(m, 'blend_method'):
            m.blend_method = 'BLEND'
    MATS[name] = m
    return m


def materials():
    return {
        'brass': mat('pb_brass', (0.83, 0.60, 0.22), 1.0, 0.26),
        'brass_dark': mat('pb_brass_dark', (0.45, 0.30, 0.10), 1.0, 0.42),
        'copper': mat('pb_copper', (0.80, 0.38, 0.22), 1.0, 0.30),
        'steel': mat('pb_steel', (0.72, 0.75, 0.80), 1.0, 0.22),
        'iron': mat('pb_iron', (0.10, 0.11, 0.13), 0.9, 0.55),
        'mahogany': mat('pb_mahogany', (0.20, 0.065, 0.035), 0.0, 0.42),
        'enamel': mat('pb_enamel_night', (0.03, 0.07, 0.17), 0.1, 0.25),
        'ivory': mat('pb_ivory', (0.86, 0.80, 0.66), 0.0, 0.45),
        'sun': mat('glow_sun', (1.0, 0.72, 0.25), 0.0, 0.3, (1.0, 0.62, 0.18), 3.2),
        'star': mat('glow_star', (0.55, 0.9, 1.0), 0.0, 0.3, (0.45, 0.88, 1.0), 2.6),
        'earth': mat('pb_planet_earth', (0.10, 0.34, 0.62), 0.0, 0.4),
        'mars': mat('pb_planet_mars', (0.70, 0.24, 0.12), 0.0, 0.5),
        'venus': mat('pb_planet_venus', (0.86, 0.72, 0.46), 0.0, 0.4),
        'saturn': mat('pb_planet_saturn', (0.78, 0.63, 0.38), 0.2, 0.38),
    }


def empty(name, parent=None, loc=(0, 0, 0), rot=(0, 0, 0)):
    o = bpy.data.objects.new(name, None)
    o.empty_display_size = 0.01
    bpy.context.scene.collection.objects.link(o)
    o.parent = parent
    o.location = loc
    o.rotation_euler = rot
    return o


def finish(name, bm, material, parent=None, loc=(0, 0, 0), rot=(0, 0, 0), smooth=None):
    """bm -> object. `smooth` is a predicate on the face (default: every face smooth)."""
    me = bpy.data.meshes.new(name)
    bm.normal_update()
    for f in bm.faces:
        f.smooth = bool(smooth(f)) if smooth else True
    bm.to_mesh(me)
    bm.free()
    me.materials.append(material)
    o = bpy.data.objects.new(name, me)
    bpy.context.scene.collection.objects.link(o)
    o.parent = parent
    o.location = loc
    o.rotation_euler = rot
    return o


def lathe(name, profile, material, parent=None, loc=(0, 0, 0), rot=(0, 0, 0), segs=24, flat=False, caps=True):
    """Revolve [(r, z), ...] about +Z. Points with r == 0 close the ends."""
    bm = bmesh.new()
    rings = []
    for r, z in profile:
        if r <= 1e-7:
            rings.append([bm.verts.new((0, 0, z))])
        else:
            rings.append([bm.verts.new((r * math.cos(2 * math.pi * i / segs), r * math.sin(2 * math.pi * i / segs), z)) for i in range(segs)])
    for a, b in zip(rings, rings[1:]):
        for i in range(segs):
            j = (i + 1) % segs
            if len(a) == 1 and len(b) == 1:
                continue
            if len(a) == 1:
                bm.faces.new((a[0], b[i], b[j]))
            elif len(b) == 1:
                bm.faces.new((a[i], a[j], b[0]))
            else:
                bm.faces.new((a[i], a[j], b[j], b[i]))
    if caps:
        for ring, flip in ((rings[0], True), (rings[-1], False)):
            if len(ring) > 1:
                bm.faces.new(list(reversed(ring)) if flip else ring)
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    return finish(name, bm, material, parent, loc, rot, smooth=(lambda f: False) if flat else (lambda f: abs(f.normal.z) < 0.92 and len(f.verts) <= 4))


def cyl(name, r, h, material, parent=None, loc=(0, 0, 0), rot=(0, 0, 0), segs=20, r2=None):
    return lathe(name, [(r, 0), (r if r2 is None else r2, h)], material, parent, loc, rot, segs)


def box(name, size, material, parent=None, loc=(0, 0, 0), rot=(0, 0, 0)):
    bm = bmesh.new()
    bmesh.ops.create_cube(bm, size=1.0)
    bmesh.ops.scale(bm, vec=Vector(size), verts=bm.verts[:])
    return finish(name, bm, material, parent, loc, rot, smooth=lambda f: False)


def sphere(name, r, material, parent=None, loc=(0, 0, 0), segs=20, scale=(1, 1, 1)):
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=segs, v_segments=max(6, segs // 2), radius=r)
    bmesh.ops.scale(bm, vec=Vector(scale), verts=bm.verts[:])
    return finish(name, bm, material, parent, loc)


def torus(name, R, r, material, parent=None, loc=(0, 0, 0), rot=(0, 0, 0), segs=48, minor=8, flat_band=None):
    """Ring about +Z. flat_band=(width, thickness) builds a rectangular band instead of a round tube."""
    bm = bmesh.new()
    if flat_band:
        w, t = flat_band
        prof = [(-t / 2, -w / 2), (t / 2, -w / 2), (t / 2, w / 2), (-t / 2, w / 2)]
    else:
        prof = [(r * math.cos(2 * math.pi * k / minor), r * math.sin(2 * math.pi * k / minor)) for k in range(minor)]
    rings = []
    for i in range(segs):
        a = 2 * math.pi * i / segs
        rings.append([bm.verts.new(((R + pr) * math.cos(a), (R + pr) * math.sin(a), pz)) for pr, pz in prof])
    n = len(prof)
    for i in range(segs):
        a, b = rings[i], rings[(i + 1) % segs]
        for k in range(n):
            bm.faces.new((a[k], b[k], b[(k + 1) % n], a[(k + 1) % n]))
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    return finish(name, bm, material, parent, loc, rot, smooth=(lambda f: False) if flat_band else None)


def gear(name, teeth, r_root, tooth, thick, material, parent=None, loc=(0, 0, 0), rot=(0, 0, 0), r_hole=None, spokes=5, hub=None):
    """Spur gear about +Z: toothed rim, spokes and hub. Origin at the gear centre, mid thickness."""
    r_hole = r_hole if r_hole else r_root * 0.72
    hub = hub if hub else max(0.0022, r_root * 0.24)
    bm = bmesh.new()
    outer, inner = [], []
    for i in range(teeth):
        a0 = 2 * math.pi * i / teeth
        da = 2 * math.pi / teeth
        for frac, rad in ((0.0, r_root), (0.22, r_root + tooth), (0.50, r_root + tooth), (0.72, r_root)):
            a = a0 + da * frac
            outer.append((rad * math.cos(a), rad * math.sin(a)))
            inner.append((r_hole * math.cos(a), r_hole * math.sin(a)))
    n = len(outer)
    lo = [bm.verts.new((x, y, -thick / 2)) for x, y in outer]
    hi = [bm.verts.new((x, y, thick / 2)) for x, y in outer]
    li = [bm.verts.new((x, y, -thick / 2)) for x, y in inner]
    hi_i = [bm.verts.new((x, y, thick / 2)) for x, y in inner]
    for i in range(n):
        j = (i + 1) % n
        bm.faces.new((lo[i], lo[j], hi[j], hi[i]))          # outer wall
        bm.faces.new((li[j], li[i], hi_i[i], hi_i[j]))      # inner wall
        bm.faces.new((hi[i], hi[j], hi_i[j], hi_i[i]))      # top
        bm.faces.new((lo[j], lo[i], li[i], li[j]))          # bottom
    for s in range(spokes):
        a = 2 * math.pi * s / spokes + 0.3
        w = max(0.0012, r_root * 0.13)
        length = r_hole + 0.0006
        mtx = Matrix.Rotation(a, 4, 'Z') @ Matrix.Translation((length / 2, 0, 0))
        ret = bmesh.ops.create_cube(bm, size=1.0)
        bmesh.ops.scale(bm, vec=Vector((length, w, thick * 0.7)), verts=ret['verts'])
        bmesh.ops.transform(bm, matrix=mtx, verts=ret['verts'])
    bmesh.ops.create_cone(bm, cap_ends=True, segments=14, radius1=hub, radius2=hub, depth=thick * 1.5)
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    return finish(name, bm, material, parent, loc, rot, smooth=lambda f: False)


def bake_scale(root, s):
    """Uniformly scale a toy by rewriting mesh data and child offsets (node scales stay 1)."""
    def walk(o):
        for c in o.children:
            c.location = c.location * s
            if c.type == 'MESH':
                for v in c.data.vertices:
                    v.co = v.co * s
                for m in c.modifiers:
                    if m.type == 'SOLIDIFY':
                        m.thickness *= s
            walk(c)
    walk(root)


# ---------------------------------------------------------------------------------------------------------------------
def build_orrery(M):
    root = empty('orrery')
    az, st, top = P['orrery_arm_z'], P['orrery_arm_step'], P['orrery_top']
    hr = P['orrery_hub_r']
    lathe('orrery_spindle', [(hr, 0), (hr, 0.004), (0.0032, 0.006), (0.0032, az - 0.004), (0.0054, az - 0.0022),
                             (0.0054, az + 4 * st - 0.001), (0.0026, az + 4 * st + 0.002), (0.0026, top - 0.022)], M['brass'], root, segs=16)
    names = ['venus', 'earth', 'mars', 'saturn']
    for k, R in enumerate(P['orrery_radii']):
        z = az + k * st
        pr = P['orrery_planets'][k]
        arm = empty('orrery_arm_%d' % (k + 1), root, (0, 0, z), (0, 0, k * 1.7))
        box('orrery_arm_%d_rod' % (k + 1), (R + 0.005, 0.0014, 0.0011), M['brass'], arm, ((R - 0.005) / 2, 0, 0))
        sphere('orrery_arm_%d_weight' % (k + 1), 0.0022, M['brass_dark'], arm, (-0.0062, 0, 0), segs=10)
        cyl('orrery_arm_%d_collar' % (k + 1), 0.0048, st - 0.0007, M['brass_dark'], arm, (0, 0, -(st - 0.0007) / 2), segs=14)
        cyl('orrery_arm_%d_stem' % (k + 1), 0.0006, 0.004, M['brass'], arm, (R, 0, 0), segs=6)
        pz = 0.004 + pr * 0.6
        sphere('orrery_planet_%d' % (k + 1), pr, M[names[k]], arm, (R, 0, pz), segs=16)
        if names[k] == 'saturn':
            torus('orrery_planet_4_ring', pr * 1.4, 0, M['brass'], arm, (R, 0, pz), (0.38, 0.12, 0), segs=28, flat_band=(0.0004, pr * 0.5))
        if names[k] == 'earth':
            sphere('orrery_moon', 0.0013, M['ivory'], arm, (R, 0.0068, pz + 0.0012), segs=8)
            box('orrery_moon_rod', (0.0005, 0.0068, 0.0005), M['brass'], arm, (R, 0.0034, pz + 0.0006))
    zs = top - 0.016
    sphere('orrery_sun', 0.0088, M['sun'], root, (0, 0, zs), segs=20)
    rings = empty('orrery_rings', root, (0, 0, zs))
    torus('orrery_ring_a', 0.0140, 0, M['brass'], rings, rot=(math.radians(90), 0, 0), segs=40, flat_band=(0.0020, 0.0008))
    torus('orrery_ring_b', 0.0156, 0, M['brass'], rings, rot=(math.radians(66), 0, math.radians(40)), segs=40, flat_band=(0.0020, 0.0008))
    torus('orrery_ring_c', 0.0172, 0, M['copper'], rings, rot=(math.radians(23), 0, math.radians(100)), segs=40, flat_band=(0.0024, 0.0008))
    return root


def build_gears(M):
    """A gear train on a mahogany plinth; origin at the plinth centre, underside at z 0."""
    root = empty('gears')
    lathe('gears_plinth', [(0.0, 0), (0.05, 0), (0.05, 0.004), (0.047, 0.006), (0.0, 0.006)], M['mahogany'], root, segs=6, flat=True, rot=(0, 0, math.radians(30)))
    spec = [  # name, teeth, r_root, x, y, z, material
        ('gear_1', 20, 0.0185, -0.016, 0.004, 0.014, 'brass'),
        ('gear_2', 12, 0.0105, 0.0155, 0.0135, 0.014, 'copper'),
        ('gear_3', 14, 0.0125, 0.018, -0.0145, 0.019, 'brass_dark'),
        ('gear_4', 9, 0.0078, -0.0015, -0.024, 0.019, 'steel'),
        ('gear_5', 16, 0.0145, -0.0295, -0.0215, 0.0245, 'brass'),
    ]
    for name, teeth, rr, x, y, z, mk in spec:
        cyl(name + '_axle', 0.0016, z + 0.004, M['steel'], root, (x, y, 0.004), segs=8)
        gear(name, teeth, rr, rr * 0.2, 0.0034, M[mk], root, (x, y, z))
    box('gears_bridge', (0.062, 0.0045, 0.0022), M['brass_dark'], root, (-0.004, -0.004, 0.0305), (0, 0, math.radians(28)))
    for x, y in ((-0.031, -0.0185), (0.0235, 0.0105)):
        cyl('gears_pillar', 0.0021, 0.0255, M['brass'], root, (x, y, 0.005), segs=8)
    sphere('gears_jewel', 0.0026, M['star'], root, (-0.004, -0.004, 0.0325), segs=10)
    bake_scale(root, P['gears_scale'])
    return root


def build_pendulum(M):
    """Gantry over a lane with a hanging pendulum. Origin: on the playfield under the pivot.
    The posts stand behind the swing plane; `pendulum_arm` has its origin at the pivot."""
    root = empty('pendulum')
    pz, L, r = P['pend_pivot_h'], P['pend_len'], P['pend_bob_r']
    dx, dy = P['pend_post_dx'], P['pend_post_dy']
    for sx in (-1, 1):
        lathe('pendulum_post', [(0.0042, 0), (0.0042, 0.003), (0.0024, 0.006), (0.0024, pz - 0.004), (0.0034, pz - 0.002), (0.0034, pz + 0.004), (0.0, pz + 0.007)],
              M['brass'], root, (sx * dx, dy, 0), segs=12)
        # Outrigger from the post top forward to the swing plane.
        box('pendulum_outrigger', (0.0036, dy + 0.004, 0.0026), M['brass_dark'], root, (sx * dx, dy / 2, pz + 0.002))
    bm = bmesh.new()
    n = 14
    w, th = 0.0046, 0.0026
    prev = None
    for i in range(n + 1):
        t = i / n
        x = -dx + 2 * dx * t
        z = pz + 0.002 + 0.0042 * math.sin(math.pi * t)
        cur = [bm.verts.new((x, -w / 2, z - th / 2)), bm.verts.new((x, w / 2, z - th / 2)), bm.verts.new((x, w / 2, z + th / 2)), bm.verts.new((x, -w / 2, z + th / 2))]
        if prev:
            for k in range(4):
                bm.faces.new((prev[k], prev[(k + 1) % 4], cur[(k + 1) % 4], cur[k]))
        prev = cur
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    finish('pendulum_beam', bm, M['brass_dark'], root, smooth=lambda f: False)
    cyl('pendulum_pivot', 0.003, 0.008, M['steel'], root, (0, 0.004, pz), (math.radians(90), 0, 0), segs=12)
    arm = empty('pendulum_arm', root, (0, 0, pz))
    box('pendulum_rod', (0.0015, 0.0015, L - r * 0.6), M['steel'], arm, (0, 0, -(L - r * 0.6) / 2))
    lathe('pendulum_bob', [(0.0, -r), (r * 0.55, -r * 0.82), (r * 0.92, -r * 0.38), (r, 0), (r * 0.92, r * 0.38), (r * 0.55, r * 0.82), (0.0, r)],
          M['brass'], arm, (0, 0, -L), segs=20)
    torus('pendulum_bob_band', r * 1.005, 0.001, M['copper'], arm, (0, 0, -L), segs=24, minor=6)
    sphere('pendulum_bob_star', 0.003, M['star'], arm, (0, -r * 0.93, -L), segs=10, scale=(1, 0.45, 1))
    return root


def build_dome(M):
    """Observatory dome. Built at design radius 0.085, then scaled to P['dome_r']. Origin at the base ring centre."""
    root = empty('dome')
    R, H, Z0 = 0.085, 0.084, 0.036
    lathe('dome_drum', [(R * 1.0, 0), (R * 1.0, Z0 - 0.008), (R * 1.04, Z0 - 0.006), (R * 1.04, Z0 - 0.002), (R * 0.985, Z0)], M['mahogany'], root, segs=32)
    torus('dome_cornice', R * 1.03, 0.0021, M['brass'], root, (0, 0, Z0 - 0.002), segs=40, minor=6)
    bm = bmesh.new()
    U, V = 40, 9
    slit = math.radians(13)
    grid = []
    for j in range(V + 1):
        ph = (math.pi / 2) * j / V
        row = []
        for i in range(U + 1):
            a = -math.pi / 2 + slit + (2 * math.pi - 2 * slit) * i / U
            row.append(bm.verts.new((R * 0.98 * math.cos(ph) * math.cos(a), R * 0.98 * math.cos(ph) * math.sin(a), Z0 + (H - Z0) * math.sin(ph))))
        grid.append(row)
    for j in range(V):
        for i in range(U):
            bm.faces.new((grid[j][i], grid[j][i + 1], grid[j + 1][i + 1], grid[j + 1][i]))
    bmesh.ops.remove_doubles(bm, verts=bm.verts[:], dist=1e-6)
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    shell = finish('dome_shell', bm, M['enamel'], root)
    sol = shell.modifiers.new('solid', 'SOLIDIFY')
    sol.thickness = 0.0016
    sol.offset = -1
    for i in range(8):
        a = -math.pi / 2 + slit + (2 * math.pi - 2 * slit) * (i + 0.5) / 8
        bm = bmesh.new()
        prev = None
        for j in range(V + 1):
            ph = (math.pi / 2) * j / V
            rr = R * 0.99 * math.cos(ph)
            c = Vector((rr * math.cos(a), rr * math.sin(a), Z0 + (H - Z0) * math.sin(ph) + 0.0004))
            t = Vector((-math.sin(a), math.cos(a), 0)) * 0.0013
            up = Vector((math.cos(ph) * math.cos(a), math.cos(ph) * math.sin(a), math.sin(ph))) * 0.0014
            cur = [bm.verts.new(c - t), bm.verts.new(c + t), bm.verts.new(c + t + up), bm.verts.new(c - t + up)]
            if prev:
                for k in range(4):
                    bm.faces.new((prev[k], prev[(k + 1) % 4], cur[(k + 1) % 4], cur[k]))
            prev = cur
        bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
        finish('dome_rib_%d' % i, bm, M['brass'], root, smooth=lambda f: False)
    lathe('dome_finial', [(0.0, H + 0.0125), (0.0022, H + 0.0085), (0.0012, H + 0.0055), (0.0046, H + 0.003), (0.0046, H - 0.0005), (0.009, H - 0.003)], M['brass'], root, segs=12)
    shutter = empty('dome_shutter', root)
    bm = bmesh.new()
    grid = []
    for j in range(V + 1):
        ph = (math.pi / 2) * j / V * 0.94
        row = []
        for i in range(5):
            a = -math.pi / 2 - slit * 1.08 + 2.16 * slit * i / 4
            row.append(bm.verts.new((R * 1.0 * math.cos(ph) * math.cos(a), R * 1.0 * math.cos(ph) * math.sin(a), Z0 + 0.0005 + (H - Z0) * math.sin(ph))))
        grid.append(row)
    for j in range(V):
        for i in range(4):
            bm.faces.new((grid[j][i], grid[j][i + 1], grid[j + 1][i + 1], grid[j + 1][i]))
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    sh = finish('dome_shutter_panel', bm, M['brass_dark'], shutter)
    s2 = sh.modifiers.new('solid', 'SOLIDIFY')
    s2.thickness = 0.0012
    # Clock on the drum front (faces the player, -Y). Enlarged so it still reads after scaling.
    cr = 0.0135
    clock = empty('clock', root, (0, -R * 1.045, 0.0165))
    cyl('clock_bezel', cr * 1.15, 0.003, M['brass'], clock, (0, 0.0008, 0), (math.radians(90), 0, 0), segs=28)
    cyl('clock_face', cr, 0.0008, M['ivory'], clock, (0, -0.0024, 0), (math.radians(90), 0, 0), segs=28)
    for k in range(12):
        a = 2 * math.pi * k / 12
        box('clock_tick', (0.0013 if k % 3 else 0.0022, 0.0006, 0.004 if k % 3 else 0.0056), M['iron'], clock,
            (cr * 0.8 * math.sin(a), -0.0034, cr * 0.8 * math.cos(a)), (0, -a, 0))
    hh = empty('clock_hand_hour', clock, (0, -0.0040, 0))
    box('clock_hand_hour_blade', (0.0022, 0.0006, cr * 0.5), M['iron'], hh, (0, 0, cr * 0.21))
    mh = empty('clock_hand_minute', clock, (0, -0.0048, 0))
    box('clock_hand_minute_blade', (0.0015, 0.0006, cr * 0.78), M['iron'], mh, (0, 0, cr * 0.33))
    sphere('clock_boss', 0.002, M['brass'], clock, (0, -0.0054, 0), segs=8)
    sphere('dome_star', 0.006, M['star'], root, (0, -R * 0.35, H * 0.55), segs=12)
    bake_scale(root, P['dome_r'] / R)
    return root


def build_telescope(M):
    """Refractor on a short pier. Origin at the pier base (the scopeBase post). `telescope_barrel` has its origin at
    the lower end of the bore axis and points along local +Y; the bore is open at both ends so a ball can pass."""
    root = empty('telescope')
    ph, pr = P['tele_pier_h'], P['tele_pier_r']
    lathe('telescope_pier', [(pr, 0), (pr, 0.004), (pr * 0.86, 0.007), (pr * 0.86, ph - 0.006), (pr * 0.96, ph - 0.004), (pr * 0.96, ph), (0.0, ph)], M['mahogany'], root, segs=24)
    torus('telescope_pier_band', pr * 0.88, 0.0013, M['brass'], root, (0, 0, ph - 0.0085), segs=28, minor=6)
    R, bore, L = P['tele_r'], P['tele_bore'], P['tele_len']
    sx, sy, sz = P['tele_start']
    pitch, yaw = P['tele_pitch'], P['tele_yaw']
    # Cradle: a saddle block under the barrel where it passes the pier (it stays above ball height outside the pier).
    d = -sy / math.cos(yaw) if abs(math.cos(yaw)) > 1e-6 else L / 2      # bore distance to the pier's y
    cx = sx - math.sin(yaw) * d
    cz = sz + math.tan(pitch) * d
    box('telescope_saddle', (2 * R + 0.007, 0.016, max(0.004, cz - R * 0.55 - ph)), M['brass_dark'], root, (cx, 0, ph + max(0.004, cz - R * 0.55 - ph) / 2), (0, 0, yaw))
    for s in (-1, 1):
        box('telescope_cheek', (0.003, 0.014, R * 1.1), M['brass'], root, (cx + s * (R + 0.0035) * math.cos(yaw), s * (R + 0.0035) * math.sin(yaw), cz - R * 0.1), (0, 0, yaw))
    barrel = empty('telescope_barrel', root, (sx, sy, sz), (pitch, 0, yaw))
    # Tube shell: outer profile along the bore (z of the lathe = distance along the bore), inner bore, joined at the ends.
    outer = [(R * 0.97, 0.0), (R * 0.97, 0.004), (R, 0.006), (R, L - 0.026), (R * 1.035, L - 0.024), (R * 1.035, L - 0.003), (R * 1.0, L)]
    prof = outer + [(bore, L), (bore, 0.0)]
    bm = bmesh.new()
    segs = 28
    rings = [[bm.verts.new((r * math.cos(2 * math.pi * i / segs), r * math.sin(2 * math.pi * i / segs), z)) for i in range(segs)] for r, z in prof]
    for a, b in zip(rings, rings[1:] + rings[:1]):
        for i in range(segs):
            j = (i + 1) % segs
            bm.faces.new((a[i], a[j], b[j], b[i]))
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    finish('telescope_tube', bm, M['brass'], barrel, rot=(math.radians(-90), 0, 0), smooth=lambda f: abs(f.normal.z) < 0.5)
    for z, mk, w in ((0.012, 'copper', 0.0036), (L * 0.42, 'brass_dark', 0.006), (L - 0.036, 'copper', 0.0036)):
        torus('telescope_band', R + 0.0007, 0, M[mk], barrel, (0, z, 0), (math.radians(90), 0, 0), segs=28, flat_band=(w, 0.0014))
    # Finder scope on the lower half (most headroom under the glass there) and a focus knob.
    cyl('telescope_finder', 0.0028, L * 0.3, M['brass_dark'], barrel, (0, 0.012, R + 0.0046), (math.radians(-90), 0, 0), segs=10)
    for y in (0.02, 0.012 + L * 0.26):
        box('telescope_finder_mount', (0.0018, 0.0026, 0.0046), M['brass'], barrel, (0, y, R + 0.0018))
    cyl('telescope_knob', 0.0036, 0.005, M['steel'], barrel, (R + 0.0008, 0.016, 0), (0, math.radians(90), 0), segs=10)
    torus('telescope_lens_glow', bore + 0.0006, 0.0011, M['star'], barrel, (0, L - 0.002, 0), (math.radians(90), 0, 0), segs=28, minor=6)
    return root


BUILDERS = {'orrery': build_orrery, 'gears': build_gears, 'pendulum': build_pendulum, 'dome': build_dome, 'telescope': build_telescope}
PREVIEW_AT = {'orrery': (-0.17, 0.0), 'gears': (-0.07, 0.0), 'pendulum': (0.03, 0.0), 'dome': (0.03, 0.14), 'telescope': (0.16, 0.03)}


def clear_scene():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    MATS.clear()


def apply_modifiers():
    dg = bpy.context.evaluated_depsgraph_get()
    for o in list(bpy.context.scene.objects):
        if o.type == 'MESH' and o.modifiers:
            me = bpy.data.meshes.new_from_object(o.evaluated_get(dg))
            o.modifiers.clear()
            o.data = me


def stats():
    tris = 0
    for o in bpy.context.scene.objects:
        if o.type == 'MESH':
            o.data.calc_loop_triangles()
            tris += len(o.data.loop_triangles)
    return tris


def extents(root):
    """World-space bounding box of a toy (after the dependency graph update)."""
    lo, hi = Vector((1e9, 1e9, 1e9)), Vector((-1e9, -1e9, -1e9))
    def walk(o):
        if o.type == 'MESH':
            for v in o.data.vertices:
                w = o.matrix_world @ v.co
                for k in range(3):
                    lo[k] = min(lo[k], w[k])
                    hi[k] = max(hi[k], w[k])
        for c in o.children:
            walk(c)
    walk(root)
    return [round(v, 4) for v in lo], [round(v, 4) for v in hi]


def render_preview(folder):
    scene = bpy.context.scene
    os.makedirs(folder, exist_ok=True)
    world = bpy.data.worlds.new('w')
    world.use_nodes = True
    world.node_tree.nodes['Background'].inputs[0].default_value = (0.05, 0.06, 0.09, 1)
    world.node_tree.nodes['Background'].inputs[1].default_value = 0.35
    scene.world = world
    scene.view_settings.view_transform = 'Standard'
    bm = bmesh.new()
    bmesh.ops.create_grid(bm, x_segments=1, y_segments=1, size=0.6)
    finish('_floor', bm, mat('_floor', (0.03, 0.035, 0.05), 0.0, 0.35), smooth=lambda f: False)
    for name, loc, energy, color, size in (('key', (0.25, -0.45, 0.55), 7, (1.0, 0.93, 0.82), 0.3), ('fill', (-0.4, -0.2, 0.3), 2.5, (0.6, 0.78, 1.0), 0.4), ('rim', (0.0, 0.5, 0.35), 4, (1.0, 0.8, 0.55), 0.3)):
        light = bpy.data.lights.new(name, 'AREA')
        light.energy = energy
        light.color = color
        light.size = size
        lo = bpy.data.objects.new(name, light)
        scene.collection.objects.link(lo)
        lo.location = loc
        lo.rotation_euler = (Vector((0, 0, 0.03)) - Vector(loc)).to_track_quat('-Z', 'Y').to_euler()
    cam_data = bpy.data.cameras.new('cam')
    cam_data.lens = 58
    cam = bpy.data.objects.new('cam', cam_data)
    scene.collection.objects.link(cam)
    scene.camera = cam
    scene.render.resolution_x, scene.render.resolution_y = 1600, 900
    scene.render.image_settings.file_format = 'PNG'
    engines = [e.identifier for e in bpy.types.RenderSettings.bl_rna.properties['engine'].enum_items]
    scene.render.engine = 'CYCLES' if 'CYCLES' in engines else engines[0]
    if scene.render.engine == 'CYCLES':
        scene.cycles.samples = 48
        scene.cycles.use_denoising = True
    for label, loc, target in (('all', (0.0, -0.5, 0.3), (0.0, 0.05, 0.03)), ('left', (-0.2, -0.22, 0.15), (-0.12, 0.0, 0.035)), ('right', (0.22, -0.22, 0.16), (0.1, 0.04, 0.04))):
        cam.location = loc
        cam.rotation_euler = (Vector(target) - Vector(loc)).to_track_quat('-Z', 'Y').to_euler()
        scene.render.filepath = os.path.join(folder, 'observatory-toys-%s.png' % label)
        bpy.ops.render.render(write_still=True)


def main():
    argv = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
    out = argv[argv.index('--out') + 1] if '--out' in argv else None
    preview = argv[argv.index('--preview') + 1] if '--preview' in argv else None
    if '--params' in argv:
        with open(argv[argv.index('--params') + 1], encoding='utf-8') as fh:
            P.update(json.load(fh))
    clear_scene()
    M = materials()
    roots = {k: fn(M) for k, fn in BUILDERS.items()}
    apply_modifiers()
    names = {o.name for o in bpy.context.scene.objects}
    missing = sorted(n for n in ANIMATED if n not in names)
    if missing:
        raise SystemExit('missing animated nodes: ' + ', '.join(missing))
    bpy.context.view_layer.update()
    for k, root in roots.items():
        lo, hi = extents(root)
        print('TOYS %s box lo=%s hi=%s' % (k, lo, hi))
    print('TOYS triangles=%d objects=%d' % (stats(), len(names)))
    if out:
        os.makedirs(os.path.dirname(os.path.abspath(out)), exist_ok=True)
        bpy.ops.export_scene.gltf(filepath=os.path.abspath(out), export_format='GLB', export_yup=True, export_apply=True,
                                  export_animations=False, export_cameras=False, export_lights=False, export_extras=False)
        print('TOYS wrote %s (%d bytes)' % (out, os.path.getsize(out)))
    if preview:
        for k, (x, y) in PREVIEW_AT.items():
            roots[k].location = (x, y, 0)
        render_preview(os.path.abspath(preview))
        print('TOYS previews in %s' % preview)


main()
