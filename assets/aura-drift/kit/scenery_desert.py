"""Desert scenery: two cacti, a banded mesa, a water tower with a cowboy hat and a windmill pump (MIT, AuraGo contributors).

Terracotta, ochre and sun-baked pastels with teal cacti. The windmill's ``rotor`` is a child node
with its pivot on the hub, axle along X (spin it around X). Props stand at y=0, front toward +X.
"""

import math

from . import scenery_a_common as c
from .core import Model
from .polish_props import sunshade
from .weather_props import anemometer
from .shapes import box, cylinder, cylinder_x, ellipsoid, revolve, sphere, strut, xf


def capsule(r, h, seg=8):
    """Closed rounded post from y=0 to y=h (cactus arms and trunks)."""
    return revolve([(0.0, 0.0), (r * 0.8, 0.0), (r, r * 0.45), (r, h - r * 0.9), (r * 0.72, h - r * 0.25), (0.0, h)], seg)


def cactus_a():
    """Saguaro with two raised arms, pink bloom on top."""
    p = c.Piece("cactus_a")
    p.add(capsule(0.7, 6.4, 8), None, "#2fa36a", grad=0.55)
    for sign, y, reach, rise in ((1, 2.6, 1.9, 2.4), (-1, 3.6, 1.7, 1.9)):
        p.add(strut((0.0, y, sign * 0.45), (0.0, y, sign * (0.45 + reach)), 0.38, 8), None, "#2a9460", grad=0.4)
        p.add(capsule(0.38, rise + 0.4, 8), xf((0, y - 0.2, sign * (0.45 + reach))), "#33ad70", grad=0.55)
    p.add(sphere(0.3, 6, 3), xf((0.0, 6.5, 0.0)), "#ff5fa2", grad=0.0, jitter=0.0)
    p.add(sphere(0.2, 5, 3), xf((0.65, 5.0, 0.0)), "#ffd23c", grad=0.0, jitter=0.0)
    return p


def cactus_b():
    """Prickly pear: stacked flat pads with red fruit and yellow blooms."""
    p = c.Piece("cactus_b")
    pads = [((0.0, 0.9, 0.0), (1.5, 1.05, 0.35), (0, 0, 8)), ((0.9, 2.3, 0.5), (1.2, 0.95, 0.32), (20, 25, -20)), ((-0.9, 2.2, -0.4), (1.1, 0.9, 0.3), (-25, -30, 22)),
            ((1.5, 3.5, 1.1), (0.95, 0.8, 0.28), (30, 40, -40)), ((-1.3, 3.5, -0.9), (0.9, 0.75, 0.27), (-30, -40, 40)), ((0.2, 3.6, 0.1), (0.85, 0.75, 0.26), (10, 80, 5))]
    for i, (pos, rad, rot) in enumerate(pads):
        p.add(ellipsoid(rad[0], rad[1], rad[2] * 1.5, 8, 4), xf(pos, rot), "#5fbf6f" if i % 2 else "#4aa860", grad=0.5)
    for pos in ((1.5, 4.4, 1.1), (-1.3, 4.3, -0.9), (0.2, 4.4, 0.1), (0.1, 2.0, 0.5)):
        p.add(sphere(0.22, 5, 3), xf(pos), "#ff4f6e", grad=0.0, jitter=0.0)
    p.add(sphere(0.26, 5, 3), xf((-0.4, 4.4, -0.1)), "#ffd23c", grad=0.0, jitter=0.0)
    return p


STRATA = ["#e0603f", "#f2993f", "#f7c46e", "#d4553a", "#ee7a40"]


def strata(y_offset=0.0):
    def fn(x, y, z):
        return STRATA[int((y + y_offset) / 1.4) % len(STRATA)]

    return fn


def mesa_rock():
    """Flat-topped butte with painted strata, a talus skirt and a sibling butte."""
    p = c.Piece("mesa_rock")
    seg = 12
    table = c.wobble_table("mesa", seg)
    p.add(c.rough_frustum(9.0, 6.4, 3.0, seg, table, 0.1), None, strata(), grad=0.3)
    for y0, r0, r1, h in ((3.0, 6.4, 5.7, 4.2), (7.2, 5.9, 5.5, 3.6), (10.8, 5.5, 6.2, 1.7)):
        p.add(c.rough_frustum(r0, r1, h, seg, table, 0.13, y0), None, strata(), grad=0.3)
    p.add(c.rough_frustum(6.2, 4.6, 0.45, seg, table, 0.1, 12.5), None, "#f6c98a", grad=0.1)
    t2 = c.wobble_table("mesa2", 9)
    p.add(c.rough_frustum(4.2, 3.0, 6.0, 9, t2, 0.13), xf((10.0, 0, -6.0)), strata(1.0), grad=0.3)
    p.add(c.rough_frustum(3.0, 2.2, 0.9, 9, t2, 0.1, 6.0), xf((10.0, 0, -6.0)), "#f1b45e", grad=0.2)
    for i, (x, z, r) in enumerate(((12.0, 3.0, 1.5), (8.5, 7.0, 1.1), (-9.0, 8.0, 1.4))):
        p.add(c.lump(r, r * 0.8, r, ("mesa", i), 7, 3, 0.22, flat_bottom=True), xf((x, r * 0.4, z)), "#c9503a", grad=0.5)
    return p


def square(p, y, h, radius, colour, y_limit=None):
    for a, b in (((h, y, h), (h, y, -h)), ((h, y, -h), (-h, y, -h)), ((-h, y, -h), (-h, y, h)), ((-h, y, h), (h, y, h))):
        p.add(strut(a, b, radius, 4), None, colour, grad=0.25)


def water_tower():
    """Blue tank on braced legs, red cap, hoops, a ladder and a cowboy hat."""
    p = c.Piece("water_tower")
    for sx in (1, -1):
        for sz in (1, -1):
            p.add(strut((sx * 3.0, 0.0, sz * 3.0), (sx * 2.0, 8.5, sz * 2.0), 0.22, 6), None, "#8a5a3a", grad=0.5)
            p.add(box(0.9, 0.4, 0.9, 0.04, 1), xf((sx * 3.0, 0.2, sz * 3.0)), "#8d90a6", grad=0.3)
    square(p, 2.8, 2.7, 0.12, "#6a4328")
    square(p, 5.6, 2.3, 0.12, "#6a4328")
    for s in (1, -1):
        p.add(strut((s * 2.9, 0.4, 2.9), (s * 2.1, 5.6, -2.1), 0.07, 4), None, "#6a4328", grad=0.2)
        p.add(strut((s * 2.9, 0.4, -2.9), (s * 2.1, 5.6, 2.1), 0.07, 4), None, "#6a4328", grad=0.2)
    p.add(cylinder(2.4, 2.4, 0.4, 12), xf((0, 8.7, 0)), "#5c3b24", grad=0.2)
    sky = lambda x, y, z: "#d8402e" if 11.0 < y < 11.7 else ("#4aa4e8" if y > 9.0 else "#2f6fb8")
    p.add(cylinder(3.0, 3.0, 4.2, 12), xf((0, 10.9, 0)), sky, grad=0.3)
    for y in (9.6, 12.3):
        p.add(cylinder(3.07, 3.07, 0.14, 12), xf((0, y, 0)), "#3a3f56", grad=0.1)
    p.add(revolve([(0.0, 13.0), (3.1, 13.0), (0.0, 14.7)], 12), None, "#d8402e", grad=0.4)
    p.add(cylinder(1.35, 1.35, 0.14, 12), xf((0, 14.75, 0)), "#a86a30", grad=0.2)  # cowboy hat
    p.add(cylinder(0.62, 0.52, 0.52, 10), xf((0, 15.05, 0)), "#b87a38", grad=0.3)
    p.add(cylinder(0.64, 0.64, 0.12, 10), xf((0, 14.92, 0)), "#d8402e", grad=0.0)
    p.add(strut((3.15, 0.3, 0.5), (2.4, 8.7, 0.5), 0.07, 4), None, "#6a4328", grad=0.2)  # ladder
    p.add(strut((3.15, 0.3, -0.5), (2.4, 8.7, -0.5), 0.07, 4), None, "#6a4328", grad=0.2)
    for k in range(9):
        y = 0.8 + k * 0.95
        p.add(strut((3.1 - 0.09 * k, y, 0.5), (3.1 - 0.09 * k, y, -0.5), 0.05, 4), None, "#6a4328", grad=0.0)
    p.add(cylinder_x(0.18, 0.18, 1.6, 6), xf((3.6, 9.5, 0)), "#3a3f56", grad=0.2)  # outlet spout
    return p


def windmill_pump():
    """Western windmill: lattice tower, gearbox head, tail vane, trough."""
    p = c.Piece("windmill_pump")
    H = 9.0
    for sx in (1, -1):
        for sz in (1, -1):
            p.add(strut((sx * 1.8, 0.0, sz * 1.8), (sx * 0.5, H, sz * 0.5), 0.14, 6), None, "#c9ced8", grad=0.4)
    half = lambda y: 1.8 - 1.3 * y / H
    levels = [0.4, 3.0, 5.6, 8.0]
    for y0, y1 in zip(levels, levels[1:]):
        h0, h1 = half(y0), half(y1)
        for s in (1, -1):
            p.add(strut((s * h0, y0, h0), (s * h1, y1, -h1), 0.06, 4), None, "#e4ecf4", grad=0.2)
            p.add(strut((s * h0, y0, -h0), (s * h1, y1, h1), 0.06, 4), None, "#e4ecf4", grad=0.2)
            p.add(strut((h0, y0, s * h0), (-h1, y1, s * h1), 0.06, 4), None, "#e4ecf4", grad=0.2)
            p.add(strut((-h0, y0, s * h0), (h1, y1, s * h1), 0.06, 4), None, "#e4ecf4", grad=0.2)
    for y in levels:
        square(p, y, half(y), 0.07, "#8f98aa")
    p.add(box(1.8, 0.8, 1.0, 0.08, 1), xf((0.1, H + 0.4, 0)), "#3d7fd0", grad=0.3)  # gearbox
    p.add(cylinder_x(0.28, 0.28, 0.8, 8), xf((0.9, H + 0.5, 0)), "#e4ecf4", grad=0.2)
    p.add(strut((-0.8, H + 0.55, 0), (-4.2, H + 0.55, 0), 0.07, 4), None, "#8f98aa", grad=0.2)
    p.add(box(2.6, 1.7, 0.08, 0.02, 1), xf((-3.6, H + 0.75, 0)), "#e0402e", grad=0.3)  # tail vane
    p.add(box(2.6, 0.8, 1.3, 0.05, 1), xf((2.9, 0.4, 0.0)), "#8a5a3a", grad=0.5)  # trough
    p.add(box(2.4, 0.06, 1.1), xf((2.9, 0.78, 0.0)), "#5fc7ff", grad=0.0, jitter=0.0)
    p.add(strut((0.0, 2.0, 0.0), (1.6, 1.6, 0.0), 0.1, 5), None, "#8f98aa", grad=0.2)
    p.add(strut((1.6, 1.6, 0.0), (1.9, 1.0, 0.0), 0.1, 5), None, "#8f98aa", grad=0.2)
    return p


def windmill_rotor():
    """Sixteen paddle blades with spokes around the origin, in the YZ plane (axle along X)."""
    r = c.Piece("windmill_rotor")
    for k in range(16):
        a = 360.0 * k / 16
        r.add(box(0.1, 1.5, 0.65), xf((0, 0, 0), (a, 0, 0)) @ xf((0.0, 1.9, 0.0)), "#e4ecf4" if k % 2 else "#e0402e", grad=0.0, jitter=0.0)
        r.add(strut((0, 0, 0), (0, 2.65 * math.cos(math.radians(a)), 2.65 * math.sin(math.radians(a))), 0.05, 4), None, "#8f98aa", grad=0.0)
    r.add(cylinder_x(0.45, 0.45, 0.6, 8), None, "#3d7fd0", grad=0.2)
    r.add(cylinder_x(0.7, 0.0, 0.5, 8), xf((0.5, 0, 0)), "#e0402e", grad=0.2)
    return r


def rock_arch():
    """Sculpted terracotta sandstone natural arch spanning two desert buttresses with scrub bushes."""
    p = c.Piece("rock_arch")
    tones = ("#b84b2c", "#d66838", "#e88f4c", "#c95832")
    for side in (-1, 1):
        bz = side * 5.2
        p.add(c.lump(2.6, 3.2, 2.8, ("arch-base", side), 7, 4, 0.18, flat_bottom=True), xf((0.0, 1.8, bz)), tones[0], grad=0.45)
        p.add(c.lump(2.0, 3.0, 2.1, ("arch-mid", side), 7, 4, 0.18), xf((0.0, 4.8, bz * 0.82)), tones[1], grad=0.4)
        p.add(c.lump(1.6, 2.2, 1.8, ("arch-top", side), 7, 3, 0.16), xf((0.0, 7.4, bz * 0.56)), tones[2], grad=0.35)
    for k in range(7):
        a = math.pi * (0.16 + 0.68 * k / 6.0)
        az = 5.0 * math.cos(a)
        ay = 4.6 + 5.2 * math.sin(a)
        p.add(c.lump(1.45, 1.15, 1.65, ("arch-span", k), 7, 3, 0.16), xf((0.0, ay, az)), tones[k % len(tones)], grad=0.35)
    for idx, (x, z, r) in enumerate(((1.8, -6.4, 0.9), (-1.5, 6.2, 1.1), (1.4, 4.2, 0.75))):
        p.add(c.lump(r, r * 0.7, r, ("arch-talus", idx), 6, 3, 0.2, flat_bottom=True), xf((x, r * 0.4, z)), "#a44226", grad=0.35)
        p.add(c.lump(0.55, 0.45, 0.55, ("arch-bush", idx), 6, 3, 0.2, flat_bottom=True), xf((x * 0.8, 0.28, z * 0.9)), "#5f8c44", grad=0.3)
    return p


def palm_oasis():
    """Desert oasis spring: shimmering turquoise pool rimmed with sandstone boulders and three curved date palms."""
    p = c.Piece("palm_oasis")
    p.add(cylinder(3.4, 3.8, 0.28, 12), xf((0.0, 0.14, 0.0)), "#d89a5b", grad=0.3)
    p.add(cylinder(2.7, 2.8, 0.10, 12), xf((0.0, 0.26, 0.0)), "#2fd4c8", grad=0.1, jitter=0.0)
    for k in range(7):
        a = math.tau * k / 7.0
        rx, rz = 3.1 * math.cos(a), 3.1 * math.sin(a)
        p.add(c.lump(0.75, 0.48, 0.68, ("oasis-rk", k), 6, 3, 0.2, flat_bottom=True), xf((rx, 0.26, rz)), "#c46238", grad=0.35)
    # Three curved desert date palms with drooping frond crowns and coconuts
    for idx, (px, pz, lean_x, lean_z, h) in enumerate(((-1.4, -1.8, -0.8, -0.6, 7.2), (1.6, -1.5, 0.9, -0.5, 6.4), (-0.4, 2.2, -0.3, 0.9, 5.6))):
        pts = [(px + lean_x * (t ** 1.5), h * t, pz + lean_z * (t ** 1.5)) for t in (0.0, 0.25, 0.5, 0.75, 1.0)]
        for s0, s1 in zip(pts, pts[1:]):
            p.add(strut(s0, s1, 0.24, 6, 0.18), None, "#7a4e2d", grad=0.35)
        tx, ty, tz = pts[-1]
        for c_i in range(3):
            ca = math.tau * c_i / 3.0
            p.add(sphere(0.18, 5, 3), xf((tx + 0.22 * math.cos(ca), ty - 0.18, tz + 0.22 * math.sin(ca))), "#4d2e18", grad=0.2)
        for f_i in range(6):
            fa = 60.0 * f_i + idx * 20.0
            p.add(ellipsoid(1.35, 0.14, 0.36, 6, 3), xf((tx, ty + 0.1, tz), (0, fa, -24)) @ xf((0.95, 0, 0)), "#2f9e54", grad=0.25)
    return p


def build():
    c.ensure_materials()
    model = Model("scenery-desert")
    for name, piece in (("cactus_a", cactus_a()), ("cactus_b", cactus_b()), ("mesa_rock", mesa_rock()), ("water_tower", water_tower()), ("rock_arch", rock_arch()), ("palm_oasis", palm_oasis()), ("sunshade", sunshade(c.Piece("sunshade"))),
         ("anemometer", anemometer(c.Piece("anemometer")))):
        piece.bake()
        model.mesh("prop-" + name, piece.items())
    root = model.mesh("prop-windmill_pump", windmill_pump().bake().items())
    model.mesh("rotor", windmill_rotor().items(), parent=root, location=(1.55, 9.5, 0.0))
    return model
