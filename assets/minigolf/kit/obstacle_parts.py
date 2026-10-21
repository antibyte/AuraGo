"""Visible parts of the moving obstacles (MIT License, see ../LICENSE).

Every obstacle is one node ``obstacle-<id>``. Its mesh is drawn in the body frame (Blender axes, X along the body heading,
Z up) from the very same part list that becomes the manifest ``colliders`` (``obstacle_data``): the visible core is a box,
cylinder or ball of exactly the collision extents, and everything else (relief, hoops, chain links, lianas) lies inside
those extents, bevels included. The object gets the pose the physics has at time 0 (``obstaclePose(spec, 0)``), so the
node origin is the body origin and the scene can move it with the physics pose.

* pendulum, pirate: an iron anchor weight (plate with a raised gold anchor) on a chain; temple: a log on two lianas
* boulder (temple): a carved stone ball; radar arm (moon): hub, striped arm and two glowing panels
* ``obstacle_gates``: the gate panels (pirate hoist gate, temple stone door, moon blast door) and the static frames

Moving nodes get plain vertex colours (no ambient occlusion bake, they move), so their meshes are not densified. The static
frames (gallows, door posts) are scenery in world coordinates; the nodes go to ``ctx.moving``.
"""

import math

from mathutils import Vector, noise

from . import mech_util, obstacle_gates, shapes
from .mech_util import add_box, add_cylinder, finish_object
from .obstacle_data import POLE
from .palette import mix
from .prop import Prop


def _box(*args, **kw):
    kw.setdefault("dens", 0)
    return add_box(*args, **kw)


def _cyl(*args, **kw):
    kw.setdefault("dens", 0)
    return add_cylinder(*args, **kw)


def _part(built, role):
    return [part for part in built["parts"] if part["role"] == role]


def _chain(p, bottom, top):
    """Alternating torus links from ``bottom`` to ``top``: the first link touches ``bottom``, the last one ``top`` (inside a rod of half width ``POLE``)."""
    major, minor, stretch = POLE - 0.016, 0.014, 1.55
    half = (major + minor) * stretch
    count = max(2, round((top - bottom - 2 * half) / 0.1) + 1)
    pitch = (top - bottom - 2 * half) / (count - 1)
    for k in range(count):
        link = shapes.torus("link", major, minor, seg=6, rseg=4)
        centre = (0.0, 0.0, bottom + half + pitch * k)
        if k % 2 == 0:
            p.add(link, "metal", "iron", loc=centre, rot=(90, 0, 0), scale=(1.0, stretch, 1.0), bevel=0, dens=0, ao=0.4, smooth_angle=60)
        else:
            p.add(link, "metal", "iron", loc=centre, rot=(0, 90, 0), scale=(stretch, 1.0, 1.0), bevel=0, dens=0, ao=0.4, smooth_angle=60)


def _anchor_weight(p, built):
    core, chain = _part(built, "core")[0], _part(built, "chain")[0]
    (cx, cy, cz), (hx, hy, hz) = core["center"], core["half"]
    plate = 0.03
    _box(p, core["center"], (hx, hy - plate, hz), ("iron_dark", "iron"), mat="metal", bevel=0.03, ao=0.5)
    half = plate / 2
    gold = dict(mat="satin", bevel=0, ao=0.4)
    for side in (-1, 1):
        y = cy + side * (hy - half)
        for dx, dz, sx, sz in ((0, hz - 0.035, hx, 0.035), (0, -hz + 0.035, hx, 0.035), (hx - 0.035, 0, 0.035, hz - 0.07), (-hx + 0.035, 0, 0.035, hz - 0.07)):
            _box(p, (cx + dx, y, cz + dz), (sx, half, sz), "iron", mat="metal", bevel=0, ao=0.5)
        _box(p, (cx, y, cz + 0.02), (0.035, half, 0.2), "gold", **gold)
        _box(p, (cx, y, cz + 0.11), (0.15, half, 0.03), "gold", **gold)
        for sx in (-1, 1):
            _box(p, (cx + sx * 0.085, y, cz - 0.115), (0.1, half, 0.017), "gold", rot=(0, -sx * 37, 0), **gold)
            _box(p, (cx + sx * 0.165, y, cz - 0.065), (0.04, half, 0.028), "gold", rot=(0, -sx * 37, 0), **gold)
        ring = shapes.torus("ring", 0.045, 0.012, seg=10, rseg=4)
        p.add(ring, "satin", "gold", loc=(cx, y, cz + 0.225), rot=(90, 0, 0), bevel=0, dens=0, ao=0.4)
    _chain(p, chain["center"][2] - chain["half"][2], chain["center"][2] + chain["half"][2])


def _log(p, built):
    log = _part(built, "log")[0]
    (cx, cy, cz), radius, half = log["center"], log["radius"], log["half"]
    _cyl(p, log["center"], radius - 0.012, 2 * half - 0.012, "bark", axis="x", seg=20, bevel=0.012, noise_amp=0.09, noise_scale=3.0, ao=0.6)
    for sign in (-1, 1):
        _cyl(p, (cx + sign * (half - 0.16), cy, cz), radius, 0.075, ("lianas", "jungle_dark"), axis="x", seg=20, bevel=0, ao=0.6)
        disc = shapes.cylinder("end", radius - 0.035, radius - 0.04, 0.02, seg=20, base=False)
        p.add(disc, "matte", ("bark_light", "bark_light"), loc=(cx + sign * (half - 0.01), cy, cz), rot=(0, 90, 0), bevel=0, dens=0, ao=0.6)
    for vine in _part(built, "vine"):
        vx = vine["center"][0]
        pts = [(vx + 0.008 * math.sin(k * 1.7), 0.008 * math.cos(k * 1.3), vine["center"][2] + vine["half"][2] * (2 * k / 9 - 1)) for k in range(10)]
        tube = shapes.tube("vine", pts, radius=POLE - 0.008, seg=8, caps=True)
        p.add(tube, "matte", ("lianas", "jungle"), bevel=0, dens=0, ao=0.5, smooth_angle=60)


def _boulder(p, built):
    stone = built["parts"][0]
    radius = stone["radius"]
    sphere = shapes.ico_sphere("boulder", radius, 4)
    offset = Vector((3.1, -1.7, 5.3))
    for v in sphere.data.vertices:
        d = max(0.0, noise.noise(v.co * (2.4 / radius) + offset)) * 0.075 * radius
        v.co *= 1.0 - d / radius                       # only inwards: the sphere of the collider stays the envelope
    pal = p.pal
    dark, light, moss, deep, gold = pal.c("stone_cold"), pal.c("stone_light"), pal.c("moss"), pal.c("stone_deep"), pal.c("gold")
    glyphs = [Vector(d).normalized() for d in ((1, 0.2, 0.1), (-1, 0.1, -0.2), (0.1, 1, 0.3), (0.2, -1, -0.1), (0.1, 0.3, 1), (-0.2, -0.1, -1))]
    bands = [Vector(d).normalized() for d in ((0.2, 0.3, 1.0), (1.0, -0.4, 0.1))]

    def paint(x, y, z):
        v = Vector((x, y, z))
        n = v.normalized()
        base = mix(dark, light, 0.5 + 0.45 * noise.noise(v * (3.0 / radius)))
        base = mix(base, moss, min(0.85, max(0.0, noise.noise(v * (1.8 / radius) + Vector((9.0, 1.0, 4.0))) + 0.05) * 1.8))
        if any(abs(n.dot(b)) < 0.05 for b in bands):
            base = mix(base, deep, 0.75)
        if any(n.dot(g) > 0.955 for g in glyphs):
            base = mix(base, gold, 0.9)
        return base

    shapes.smooth_by_angle(sphere.data, 60.0)
    p.add(sphere, "matte", "stone", finish=False, fn=paint, ao=0.6)


def _radar(p, built):
    hub, arm = _part(built, "hub")[0], _part(built, "arm")[0]
    _cyl(p, hub["center"], hub["radius"] - 0.012, 2 * hub["half"], ("panel_dark", "metal"), mat="satin", bevel=0.02, ao=0.5)
    _cyl(p, (0, 0, hub["half"] - 0.12), hub["radius"], 0.07, "orange", mat="satin", bevel=0.006, segments=1, ao=0.4)
    _cyl(p, (0, 0, hub["half"] - 0.01), hub["radius"] * 0.6, 0.02, ("cyan", "cyan"), mat="glow", bevel=0, ao=0.0, strength=1.0)
    hx, hy, hz = arm["half"]
    count = max(2, round(2 * hx / 0.1))
    step = 2 * hx / count
    for k in range(count):                                  # alternating orange and white blocks: sharp stripes without dense meshes
        tone = ("orange", "orange") if k % 2 == 0 else ("panel_white", "panel_white")
        _box(p, (arm["center"][0] - hx + step * (k + 0.5), arm["center"][1], arm["center"][2]), (step / 2, hy, hz), tone, mat="satin", bevel=0, ao=0.5)
    for blade in _part(built, "blade"):
        bx, by, bz = blade["center"]
        bhx, bhy, bhz = blade["half"]
        _box(p, blade["center"], (bhx - 0.008, bhy, bhz), ("panel_dark", "metal_dark"), mat="satin", bevel=0.015, ao=0.5)
        for side in (-1, 1):
            _box(p, (bx + side * (bhx - 0.004), by, bz + 0.1), (0.004, bhy - 0.05, 0.035), ("magenta", "magenta"), mat="emissive", bevel=0, segments=1, ao=0.0, strength=1.6)


BUILDERS = {("pirate", "rotate"): _anchor_weight, ("temple", "rotate"): _log, ("temple", "boulder"): _boulder, ("moon", "spin"): _radar}


def build(ctx, built):
    """Build the ``obstacle-<id>`` node and its static frame."""
    node, world, kind = built["node"], built["world"], built["kind"]
    p = Prop(world, node, seed=ctx.seed(), full_name=node)
    if kind in ("gate", "slide"):
        obstacle_gates.panel(p, built)
    else:
        BUILDERS[(world, kind)](p, built)
    obj = finish_object(p.build(), node)
    mech_util.pose_object(obj, built["pose"]["location"], built["pose"]["rotation"])
    ctx.moving.append(obj)
    if built["frame"]:
        obstacle_gates.frame(ctx, built)
