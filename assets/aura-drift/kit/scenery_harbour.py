"""Harbour scenery: container stack, portal crane, lighthouse with a rubber duck, fishing boat, neon sign (MIT, AuraGo contributors).

Saturated dock palette (container primaries, hazard-yellow crane, red-white lighthouse) with warm
glow windows. The boat and the sign live in ``scenery_harbour_b``. Props stand at y=0, front toward +X.
"""

import math

from . import scenery_a_common as c
from .core import Model
from .polish_props import signal_mast
from .scenery_harbour_b import buoy_cluster, fishing_boat, neon_sign, tugboat
from .shapes import box, cylinder, ellipsoid, revolve, sphere, strut, xf

BOXES = [("#d8402e", -1.35, 0.0, 0.0, 0.0), ("#2f6fd6", 1.35, 0.0, 0.0, 1.0), ("#f2b72d", -1.35, 2.6, 0.0, -0.5), ("#1fa890", 1.35, 2.6, 0.0, 2.0), ("#ff7a2e", 0.1, 5.2, 0.6, 3.0)]


def container(p, color, x, y, z, yaw, length=6.0, width=2.5, height=2.6):
    base = c.colour(color)
    rib = tuple(ch * 0.84 for ch in base)
    placement = xf((x, y + height / 2.0, z), (0, yaw, 0))
    p.add(box(length, height, width, 0.04, 1), placement, color, grad=0.35)
    for side in (1, -1):
        for k in range(7):
            p.add(box(0.2, height - 0.5, 0.1), placement @ xf((-2.4 + k * 0.8, 0.0, side * (width / 2 + 0.03))), rib, grad=0.2, jitter=0.0)
    # doors on the +X end: centre seam, two locking rods, corner posts
    p.add(box(0.08, height - 0.3, 0.07), placement @ xf((length / 2 + 0.02, 0.0, 0.0)), (0.03, 0.03, 0.04), grad=0.0, jitter=0.0)
    for zz in (0.55, -0.55):
        p.add(cylinder(0.05, 0.05, height - 0.4, 5), placement @ xf((length / 2 + 0.06, 0.0, zz)), "#b8bfce", grad=0.0, jitter=0.0)
    for sy in (1, -1):
        for sz in (1, -1):
            p.add(box(0.3, 0.3, 0.3), placement @ xf((length / 2 - 0.12, sy * (height / 2 - 0.1), sz * (width / 2 - 0.1))), "#2a2f3f", grad=0.0, jitter=0.0)


def container_stack():
    p = c.Piece("container_stack")
    for color, z, y, x, yaw in BOXES:
        container(p, color, x, y, z, yaw)
    return p


def lattice(p, a, b, drop, bays, radius, color):
    """Truss between two points: two chords ``drop`` apart plus zigzag diagonals, in the XY plane at z of a and b."""
    for z in (a[2], -a[2]):
        top0, top1 = (a[0], a[1], z), (b[0], b[1], z)
        bot0, bot1 = (a[0], a[1] - drop, z), (b[0], b[1] - drop, z)
        p.add(strut(top0, top1, radius, 4), None, color, grad=0.15)
        p.add(strut(bot0, bot1, radius, 4), None, color, grad=0.15)
        for i in range(bays):
            t0, t1 = i / bays, (i + 1) / bays
            lerp = lambda u, v, t: tuple(u[k] + (v[k] - u[k]) * t for k in range(3))
            up, down = (lerp(bot0, bot1, t0), lerp(top0, top1, t1)) if i % 2 == 0 else (lerp(top0, top1, t0), lerp(bot0, bot1, t1))
            p.add(strut(up, down, radius * 0.7, 4), None, color, grad=0.15)


def crane():
    """Portal crane: four-leg gantry, slewing deck with cabin, braced boom, counterweight, hanging box."""
    p = c.Piece("crane")
    yellow, dark = "#ffb81f", "#3a4156"
    for sx in (1, -1):
        for sz in (1, -1):
            p.add(strut((sx * 3.4, 0.0, sz * 2.6), (sx * 2.4, 14.0, sz * 1.9), 0.3, 4), None, yellow, grad=0.45)
            p.add(box(1.0, 0.5, 1.0, 0.04, 1), xf((sx * 3.4, 0.25, sz * 2.6)), dark, grad=0.2)
    for y, sx, sz in ((4.0, 3.1, 2.4), (8.5, 2.75, 2.15), (13.0, 2.45, 1.95)):
        for a, b in (((sx, y, sz), (sx, y, -sz)), ((sx, y, -sz), (-sx, y, -sz)), ((-sx, y, -sz), (-sx, y, sz)), ((-sx, y, sz), (sx, y, sz))):
            p.add(strut(a, b, 0.17, 4), None, yellow, grad=0.3)
    for y0, y1, s0, s1, z0, z1 in ((0.2, 4.0, 3.4, 3.1, 2.6, 2.4), (4.0, 8.5, 3.1, 2.75, 2.4, 2.15), (8.5, 13.0, 2.75, 2.45, 2.15, 1.95)):
        for s in (1, -1):
            p.add(strut((s * s0, y0, z0), (s * s1, y1, -z1), 0.1, 4), None, "#f6f6fa", grad=0.2)
            p.add(strut((s * s0, y0, -z0), (s * s1, y1, z1), 0.1, 4), None, "#f6f6fa", grad=0.2)
    p.add(box(7.0, 0.8, 6.0, 0.06, 1), xf((0, 14.5, 0)), dark, grad=0.3)  # slewing deck
    p.add(box(2.2, 2.0, 2.0, 0.08, 1), xf((2.0, 16.0, 2.2)), yellow, grad=0.3)  # cab
    p.add_flat(box(0.06, 1.0, 1.5), xf((3.12, 16.3, 2.2)), "#ffe2a0", mat="glow_warm")
    p.add(box(0.5, 0.6, 0.5), xf((-2.0, 15.2, -2.0)), dark, grad=0.2)
    lattice(p, (-5.0, 16.0, 1.0), (14.0, 15.4, 1.0), 1.5, 12, 0.14, yellow)
    p.add(strut((-0.5, 15.0, 0.0), (-0.5, 21.0, 0.0), 0.2, 4), None, "#f6f6fa", grad=0.2)
    p.add(strut((-0.5, 21.0, 0.0), (13.5, 16.0, 0.0), 0.06, 4), None, dark, grad=0.0)
    p.add(strut((-0.5, 21.0, 0.0), (-5.0, 16.5, 0.0), 0.06, 4), None, dark, grad=0.0)
    p.add(box(2.8, 1.8, 2.8, 0.06, 1), xf((-5.6, 15.4, 0)), "#e0402e", grad=0.3)  # counterweight
    for i in range(4):
        p.add(box(0.1, 1.9, 2.9), xf((-6.7 + i * 0.6, 15.4, 0)), "#f6f6fa", grad=0.0, jitter=0.0)
    p.add_flat(sphere(0.3, 6, 3), xf((-0.5, 21.3, 0)), "#ff9a8a", mat="glow_pink")
    # trolley, cables and a hanging container
    p.add(box(1.2, 0.5, 2.2, 0.04, 1), xf((11.0, 14.2, 0)), dark, grad=0.2)
    for sx in (1, -1):
        for sz in (1, -1):
            p.add(strut((11.0 + sx * 0.4, 14.0, sz * 0.9), (11.0 + sx * 2.1, 8.5, sz * 0.95), 0.04, 4), None, dark, grad=0.0)
    p.add(box(5.0, 0.3, 2.2, 0.03, 1), xf((11.0, 8.4, 0)), dark, grad=0.2)
    p.add(box(4.8, 2.1, 2.0, 0.04, 1), xf((11.0, 7.2, 0)), "#e0402e", grad=0.3)
    for k in range(6):
        p.add(box(0.14, 1.7, 0.07), xf((9.2 + k * 0.7, 7.2, 1.03)), (0.55, 0.08, 0.05), grad=0.0, jitter=0.0)
        p.add(box(0.14, 1.7, 0.07), xf((9.2 + k * 0.7, 7.2, -1.03)), (0.55, 0.08, 0.05), grad=0.0, jitter=0.0)
    return p


def lighthouse():
    """Red and white tower on a rock, glowing lamp room, gallery rail and a rubber duck."""
    p = c.Piece("lighthouse")
    for i, (x, z, rx) in enumerate(((0, 0, 4.6), (3.6, 2.4, 2.2), (-3.3, -2.8, 2.5), (2.2, -3.6, 1.9))):
        p.add(c.lump(rx, rx * 0.55, rx, ("lh-rock", i), 8, 4, 0.2, flat_bottom=True), xf((x, rx * 0.28, z)), "#8a91a8", grad=0.5)
    r0, y = 3.3, 1.0
    for i in range(5):
        r1 = r0 - 0.28
        p.add(revolve([(0.0, y), (r0, y), (r1, y + 3.0), (0.0, y + 3.0)], 12), None, "#e0402e" if i % 2 == 0 else "#f6f1e6", grad=0.3)
        y += 3.0
        r0 = r1
    p.add(cylinder(r0 + 0.8, r0 + 0.8, 0.35, 12), xf((0, y + 0.1, 0)), "#3a4156", grad=0.2)  # gallery floor
    for k in range(12):
        a = math.tau * k / 12
        p.add(strut((math.cos(a) * (r0 + 0.65), y + 0.25, math.sin(a) * (r0 + 0.65)), (math.cos(a) * (r0 + 0.65), y + 1.2, math.sin(a) * (r0 + 0.65)), 0.05, 4), None, "#3a4156", grad=0.0)
    for k in range(12):
        a, b = math.tau * k / 12, math.tau * (k + 1) / 12
        rr = r0 + 0.65
        p.add(strut((math.cos(a) * rr, y + 1.2, math.sin(a) * rr), (math.cos(b) * rr, y + 1.2, math.sin(b) * rr), 0.06, 4), None, "#3a4156", grad=0.0)
    p.add_flat(cylinder(1.25, 1.25, 2.1, 8), xf((0, y + 1.4, 0)), "#fff0b8", mat="glow_warm")
    for k in range(8):
        a = math.tau * (k + 0.5) / 8
        p.add(strut((math.cos(a) * 1.3, y + 0.35, math.sin(a) * 1.3), (math.cos(a) * 1.3, y + 2.45, math.sin(a) * 1.3), 0.07, 4), None, "#3a4156", grad=0.0)
    p.add(revolve([(0.0, y + 2.4), (1.75, y + 2.4), (0.0, y + 3.7)], 8), None, "#e0402e", grad=0.4)
    p.add(sphere(0.25, 6, 3), xf((0, y + 3.85, 0)), "#3a4156", grad=0.1)
    # door and windows
    for yy in (5.6, 8.8, 12.0):
        p.add_flat(box(0.18, 0.9, 0.7), xf((3.3 - 0.28 * (yy - 1.0) / 3.0 + 0.0, yy, 0)), "#ffe2a0", mat="glow_warm")
    # rubber duck on the gallery floor (+X side), scaled up so it reads from the chase camera
    s = 1.4
    dx, dy = r0 + 0.4, y + 0.3
    duck = lambda *pos: xf((dx + pos[0] * s, dy + pos[1] * s, pos[2] * s), None, s)
    yellow = "#ffe033"
    p.add(ellipsoid(0.75, 0.55, 0.55, 8, 4), duck(0, 0.45, 0), yellow, grad=0.08)
    p.add(sphere(0.4, 7, 4), duck(0.45, 1.1, 0), yellow, grad=0.05)
    p.add(box(0.38, 0.1, 0.34, 0.03, 1), duck(0.9, 1.05, 0), "#ff7a1f", grad=0.0, jitter=0.0)
    p.add(sphere(0.08, 4, 3), duck(0.7, 1.3, 0.28), (0.02, 0.02, 0.03), grad=0.0, jitter=0.0)
    p.add(sphere(0.08, 4, 3), duck(0.7, 1.3, -0.28), (0.02, 0.02, 0.03), grad=0.0, jitter=0.0)
    p.add(cylinder(0.26, 0.32, 0.16, 8), duck(0.42, 1.52, 0.0), "#ffffff", grad=0.1, jitter=0.0)
    p.add(cylinder(0.27, 0.27, 0.05, 8), duck(0.42, 1.44, 0.0), "#252f5c", grad=0.0, jitter=0.0)
    p.add(box(0.5, 0.12, 0.8, 0.02, 1), xf((dx - 0.7 * s, dy + 0.6 * s, 0.0), (0, 0, 12), s), "#ffc800", grad=0.0, jitter=0.0)
    return p


def build():
    c.ensure_materials()
    model = Model("scenery-harbour")
    for name, piece in (("container_stack", container_stack()), ("crane", crane()), ("lighthouse", lighthouse()), ("fishing_boat", fishing_boat()), ("neon_sign", neon_sign()), ("tugboat", tugboat()), ("buoy_cluster", buoy_cluster()), ("signal_mast", signal_mast(c.Piece("signal_mast")))):
        piece.bake(ao_height=2.6 if name != "crane" else 3.5)
        model.mesh("prop-" + name, piece.items())
    return model
