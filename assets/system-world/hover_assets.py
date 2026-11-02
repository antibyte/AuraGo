"""Hover-pack dock of System World 2: a charging dock holding a removable personal hover pack.

Hierarchy (glTF exports it nested; the runtime hides the whole `pack` node while it flies):

    structure   the dock: plinth, charging pillar, cradle arms, conduit, standing-spot ring
    kiosk       slim status screen on the plinth
    trim        ceramic caps, bronze trim and warm warning chevrons of the dock
    pack        removable pack: ceramic shell, graphite frame, ducted shrouds with glow rings
      straps    padded shoulder straps, chest buckle, grip handles
      charge    status light bars of the power cell (breathes while charging)
      fan_l/fan_r  rotor blade sets, pivot at their own centre, spin about the duct axis

Coordinates are Blender Z-up, metres, front = -Y (exports to glTF +Z): the standing spot, the
straps and the chest buckle face the visitor. Nothing of the pack lives in `structure`.
"""
from math import sin, cos, pi
import bpy
from mathutils import Vector
from robot_assets import RobotGeometry, Z_TO_X, RX, RY, RZ

ASSETS = ['hover-pack']
PACK_C = (0, .06, 1.42)
DUCT_X, DUCT_Y, DUCT_Z = .345, .05, 1.42
RIG = {'structure': (None, (0, 0, 0)), 'kiosk': (None, (0, 0, 0)), 'trim': (None, (0, 0, 0)),
       'pack': (None, PACK_C), 'straps': ('pack', PACK_C), 'charge': ('pack', (0, .03, 1.70)),
       'fan_l': ('pack', (-DUCT_X, DUCT_Y, DUCT_Z)), 'fan_r': ('pack', (DUCT_X, DUCT_Y, DUCT_Z))}
ORDER = ['structure', 'kiosk', 'trim', 'pack', 'straps', 'charge', 'fan_l', 'fan_r']
MATERIALS = {'structure': ('graphite', 'titanium', 'cyan'), 'kiosk': ('graphite', 'glass', 'cyan'),
             'trim': ('ceramic', 'bronze', 'warm'), 'pack': ('ceramic', 'graphite', 'cyan'),
             'straps': ('robot-rubber', 'graphite', 'bronze'), 'charge': ('cyan',),
             'fan_l': ('titanium', 'graphite'), 'fan_r': ('titanium', 'graphite')}
# Whole turns per 2 s clip, opposite senses so the pack does not twist.
FAN_TURNS = {'fan_l': 4, 'fan_r': -4}


# --- dock --------------------------------------------------------------------------------
def build_structure(g):
    lod = g.lod
    g.tbox((0, 0, .05), (1.30, .90, .10), 'graphite', bevel=.035)
    g.tbox((0, .30, 1.06), (.30, .24, 1.92), 'graphite', top=(.26, .20), bevel=.03)
    g.tbox((0, .30, .17), (.42, .32, .14), 'graphite', top=(.32, .26), bevel=.02)
    # standing-spot ring on the pavement, 1.2 m in front of the dock (flat, never a collider)
    ring = [(.40, .004), (.40, .016), (.33, .016), (.33, .004)] if lod < 2 else [(.40, .014), (.33, .014)]
    g.lathe((0, -1.2, 0), ring, 'cyan', n=g.counts(24, 16, 12), caps=(False, False))
    for sd in (-1, 1):
        # cradle arm: rises from the pillar and carries the pack from below
        pts = [(sd*.14, .30, .95), (sd*.22, .24, 1.0), (sd*.22, .11, 1.07), (sd*.16, .05, 1.105)]
        if lod == 2: g.tube(pts[0], pts[-1], .028, .028, 'titanium', n=5)
        else: g.sweep(pts, .028, 'titanium', steps=2, sides=g.counts(6, 4, 4))
        g.tbox((sd*.16, .06, 1.108), (.11, .10, .035), 'graphite', bevel=.008)
        g.tbox((sd*.14, .13, 1.76), (.05, .13, .03), 'titanium', bevel=.006)
    g.tbox((0, .175, 1.42), (.20, .02, .28), 'titanium', bevel=.008)
    if lod < 2:
        for sd in (-1, 1):
            g.tbox((sd*.1, .1775, 1.42), (.012, .012, .26), 'cyan')
            g.tbox((sd*.141, .192, 1.05), (.012, .012, 1.05), 'cyan')
        g.tbox((0, .1775, 1.553), (.2, .012, .012), 'cyan')
        g.tbox((0, .1775, 1.287), (.2, .012, .012), 'cyan')
        # cable conduit from the plinth up to the charging socket
        g.sweep([(.30, .38, .10), (.23, .36, .42), (.17, .33, .86), (.08, .23, 1.30)], .028, 'graphite', steps=2)
        for z in (.35, .7, 1.05):
            g.tbox((.20-(z-.35)*.1, .345, z), (.075, .075, .03), 'titanium', bevel=.006)
    else:
        g.tbox((.141, .192, 1.05), (.012, .012, 1.05), 'cyan')


def build_kiosk(g):
    lod = g.lod
    kx, ky = .50, -.27
    g.tbox((kx, ky, .50), (.10, .10, .76), 'graphite', top=(.085, .085), bevel=.015)
    r = RX(-.38)
    head = Vector((kx, ky-.02, 1.04))

    def at(lx, ly, lz): return head+r @ Vector((lx, ly, lz))
    g.tbox(at(0, 0, 0), (.36, .05, .28), 'graphite', bevel=.014, rotation=r)
    g.tbox(at(0, -.0265, 0), (.32, .006, .24), 'glass', rotation=r)
    lines = [(0, .085, .26, .016), (-.05, .035, .18, .012), (-.05, 0, .14, .012), (-.05, -.035, .2, .012), (.1, -.035, .06, .05)]
    for lx, lz, w, h in lines[:5 if lod == 0 else 2 if lod == 1 else 1]:
        g.tbox(at(lx, -.0315, lz), (w, .003, h), 'cyan', rotation=r)


def build_trim(g):
    lod = g.lod
    g.tbox((0, 0, .118), (1.16, .76, .036), 'ceramic', bevel=.015)
    g.tbox((0, .30, 2.047), (.36, .28, .06), 'ceramic', bevel=.02)
    g.tbox((0, -.452, .06), (1.24, .014, .025), 'bronze')
    if lod < 2:
        g.tbox((0, .1795, .55), (.2, .012, .5), 'ceramic', bevel=.008)
        g.tbox((0, .30, .95), (.33, .27, .025), 'bronze', bevel=.005)
        g.tbox((0, .30, 1.90), (.31, .25, .025), 'bronze', bevel=.005)
    if lod == 0:
        for sx in (-1, 1):
            for sy in (-1, 1):
                g.tbox((sx*.62, sy*.42, .06), (.07, .07, .09), 'bronze')
    count = g.counts(4, 3, 1)
    for i in range(count):
        x = (-.36+i*.24) if lod == 0 else (-.3+i*.3) if lod == 1 else 0
        if lod == 2:
            g.tbox((x, -.345, .139), (.5, .04, .006), 'warm')
        else:
            g.tbox((x-.035, -.345, .139), (.09, .022, .006), 'warm', rotation=RZ(.55))
            g.tbox((x+.035, -.345, .139), (.09, .022, .006), 'warm', rotation=RZ(-.55))


# --- pack --------------------------------------------------------------------------------
def duct(g, sd):
    lod = g.lod
    cx, cy = sd*DUCT_X, DUCT_Y
    n = g.counts(12, 10, 6)
    # tapered nacelle: wide bell-mouth intake, narrower exhaust nozzle
    outer = [(.115, 1.25), (.135, 1.275), (.152, 1.36), (.167, 1.55), (.18, 1.59), (.18, 1.60), (.14, 1.60)]
    inner = [(.14, 1.60), (.125, 1.575), (.125, 1.34), (.1, 1.25)]
    if lod == 1:
        outer, inner = [outer[0], outer[2], outer[3], outer[5], outer[6]], [inner[0], inner[2], inner[3]]
    if lod == 2:
        outer, inner = [(.13, 1.25), (.167, 1.55), (.18, 1.6), (.14, 1.60)], [(.14, 1.60), (.1, 1.25)]
    g.lathe((cx, cy, 0), outer, 'ceramic', n=n, caps=(False, False))
    g.lathe((cx, cy, 0), inner, 'graphite', n=n, caps=(False, False))
    # glow: the whole top rim, a band inside the shroud and a ring on the nacelle body
    g.lathe((cx, cy, 0), [(.179, 1.6012), (.141, 1.6012)], 'cyan', n=n, caps=(False, False))
    if lod < 2:
        g.lathe((cx, cy, 0), [(.1235, 1.55), (.1235, 1.50)], 'cyan', n=n, caps=(False, False))
        g.lathe((cx, cy, 0), [(.155, 1.335), (.155, 1.36)], 'cyan', n=n, caps=(False, False))
    if lod == 0:
        g.lathe((cx, cy, 0), [(.173, 1.43), (.173, 1.48)], 'graphite', n=n, caps=(False, False))
        # guard grilles over the intake and the exhaust
        g.lathe((cx, cy, 0), [(.075, 1.592), (.063, 1.592)], 'graphite', n=16, caps=(False, False))
        g.tbox((cx, cy, 1.592), (.24, .012, .008), 'graphite')
        g.tbox((cx, cy, 1.592), (.012, .24, .008), 'graphite')
    if lod < 2:
        g.tbox((cx+sd*.16, cy, 1.30), (.09, .012, .12), 'ceramic', rotation=RY(sd*.35))


def build_pack(g):
    lod = g.lod
    n = g.counts(12, 8, 6)
    g.limb(0, [(1.12, .34, .21, .06), (1.18, .42, .24, .06), (1.62, .42, .24, .06), (1.72, .30, .18, .06)], 'ceramic', n=n, p=3)
    g.tbox((0, -.075, 1.42), (.34, .03, .50), 'ceramic', bevel=.012)
    if lod < 2:
        g.tbox((0, -.0915, 1.34), (.30, .004, .006), 'graphite')
        g.tbox((0, -.0915, 1.50), (.30, .004, .006), 'graphite')
        g.tbox((0, -.0915, 1.63), (.12, .004, .012), 'cyan')
    for sd in (-1, 1): duct(g, sd)
    # dorsal power cell
    g.tbox((0, .03, 1.82), (.19, .16, .30), 'graphite', bevel=.02)
    g.tbox((0, .03, 1.985), (.21, .18, .03), 'ceramic', bevel=.01)
    if lod < 2:
        for sd in (-1, 1):
            g.tbox((sd*.205, -.04, 1.42), (.035, .07, .62), 'graphite', bevel=.01)
            g.tbox((sd*.2, .03, 1.30), (.12, .08, .03), 'graphite')
            g.tbox((sd*.2, .03, 1.54), (.12, .08, .03), 'graphite')
        g.tbox((0, -.045, 1.745), (.46, .04, .035), 'graphite')
        g.tbox((0, -.045, 1.095), (.46, .04, .035), 'graphite')
    if lod == 0:
        for sd in (-1, 1):
            g.tbox((sd*.11, .2, 1.10), (.012, .12, .10), 'ceramic', rotation=RY(-sd*.3))


def build_straps(g):
    lod = g.lod
    for sd in (-1, 1):
        sections = [(1.12, .075, .04, -.105), (1.25, .08, .05, -.14), (1.45, .085, .05, -.155), (1.62, .08, .05, -.14), (1.74, .07, .04, -.105)]
        g.limb(sd*.115, sections, 'robot-rubber', n=g.counts(6, 6, 4), p=3)
        if lod == 2: continue
        g.tbox((sd*.115, -.165, 1.30), (.09, .012, .022), 'bronze')
    g.tbox((0, -.158, 1.50), (.23, .012, .03), 'graphite')
    g.tbox((0, -.17, 1.50), (.07, .018, .05), 'bronze', bevel=.006)
    for sd in (-1, 1):
        if lod == 0:
            pts = [(sd*.215, -.08, 1.28), (sd*.235, -.15, 1.28), (sd*.235, -.15, 1.40), (sd*.215, -.08, 1.40)]
            g.sweep(pts, .012, 'graphite', steps=2)
        else:
            g.tbox((sd*.235, -.12, 1.34), (.02, .07, .14), 'graphite')
        if lod < 2:
            g.tbox((sd*.215, -.08, 1.28), (.03, .03, .03), 'bronze')
            g.tbox((sd*.215, -.08, 1.40), (.03, .03, .03), 'bronze')


def build_charge(g):
    lod = g.lod
    for i in range(g.counts(5, 3, 1)):
        if lod == 2:
            g.tbox((0, -.054, 1.80), (.12, .008, .2), 'cyan')
        else:
            step = .047 if lod == 0 else .078
            g.tbox((0, -.054, 1.716+i*step), (.12, .008, .032), 'cyan')
    if lod < 2:
        for sd in (-1, 1): g.tbox((sd*.0975, .03, 1.80), (.008, .10, .22), 'cyan')


def build_fan(g, sd):
    cx, cy, cz = sd*DUCT_X, DUCT_Y, DUCT_Z
    g.cyl((cx, cy, cz), .035, .05, 'graphite', n=g.counts(10, 8, 6))
    if g.lod < 2: g.cyl((cx, cy, cz+.035), .028, .02, 'graphite', top=.01, n=g.counts(10, 8, 6))
    blades = g.counts(5, 4, 2)
    for k in range(blades):
        a = 2*pi*k/blades
        g.tbox((cx+.077*cos(a), cy+.077*sin(a), cz), (.085, .034, .007), 'titanium', rotation=RZ(a) @ RX(sd*.45))


BUILDERS = {'structure': build_structure, 'kiosk': build_kiosk, 'trim': build_trim, 'pack': build_pack,
            'straps': build_straps, 'charge': build_charge}


def create_hover_asset(asset_id, lod, collection, mats):
    """Build one LOD as a hierarchy with fixed per-part material order (identical at every LOD)."""
    g = RobotGeometry(lod)
    for part in ORDER:
        g.part = part
        if part.startswith('fan_'): build_fan(g, -1 if part.endswith('_l') else 1)
        else: BUILDERS[part](g)
    root = bpy.data.objects.new(f'{asset_id}.lod{lod}', None)
    collection.objects.link(root)
    root['asset_id'], root['lod'], root['license'] = asset_id, lod, 'MIT'
    objects = {}
    for part in ORDER:
        verts, faces, keys, smooth = g.parts[part]
        order = MATERIALS[part]
        assert set(keys) == set(order), (asset_id, lod, part, sorted(set(keys)), order)
        pivot = Vector(RIG[part][1])
        mesh = bpy.data.meshes.new(f'{asset_id}.{part}')
        mesh.from_pydata([tuple(Vector(v)-pivot) for v in verts], [], faces)
        mesh.update()
        for key in order: mesh.materials.append(mats[key])
        for polygon, key, smoothed in zip(mesh.polygons, keys, smooth):
            polygon.material_index = order.index(key)
            polygon.use_smooth = smoothed
        obj = bpy.data.objects.new(part, mesh)
        parent = RIG[part][0]
        obj.parent = objects[parent] if parent else root
        obj.location = pivot-(Vector(RIG[parent][1]) if parent else Vector())
        collection.objects.link(obj)
        obj['component'] = part
        objects[part] = obj
    return root


def animate_hover(root):
    """`idle`: fans turn whole revolutions (keyed on every frame 0..48 so the 2 s loop is exact); the
    status bars breathe. Transforms only."""
    objects = {o['component']: o for o in root.children_recursive}
    asset = root['asset_id']
    for name in ('fan_l', 'fan_r', 'charge'): objects[name].animation_data_clear()
    for name in ('fan_l', 'fan_r', 'charge'):
        obj = objects[name]
        for frame in range(0, 49):
            t = (frame % 48)/48
            if name == 'charge':
                obj.scale = (1, 1, .8+.2*sin(2*pi*t))
                obj.keyframe_insert(data_path='scale', frame=frame)
            else:
                obj.rotation_euler = (0, 0, 2*pi*FAN_TURNS[name]*frame/48)
                obj.keyframe_insert(data_path='rotation_euler', frame=frame)
        action = obj.animation_data.action
        action.name = f'{asset}.{name}.idle'
        track = obj.animation_data.nla_tracks.new()
        track.name = 'idle'
        track.strips.new('idle', 0, action)
        track.mute = True
        obj.animation_data.action = None
        obj.rotation_euler = (0, 0, 0)
        obj.scale = (1, 1, 1)
    return ['idle']


def hover_navigation(kind):
    """glTF coordinates: one box for the dock with its pack (1.4 x 2.1 x 1.0 m) and the standing spot."""
    return {'colliders': [[-.7, 0, -.5, .7, 2.1, .5]], 'surfaces': [], 'portals': [],
            'interaction': [{'name': 'take', 'position': [0, 0, 1.2]}]}
