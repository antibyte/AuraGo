"""Industry scenery: twin silos, a banded smokestack, a pipe run, a forklift and a transmission pylon (MIT, AuraGo contributors).

Steel blue, concrete grey, hazard yellow and rust orange. Red beacons and amber lamps are emissive
so bloom lights the plant at dusk. Comic touches: rubber ducks on the forklift pallet and a duck
weather vane on the silo, a smoke ring above the stack. Props stand at y=0, front toward +X.
"""

import math

from . import scenery_b_common as c
from .core import Model
from .shapes import box, cylinder, cylinder_x, ellipsoid, revolve, sphere, strut, torus, xf

STEEL = "#5b7fa6"
CONCRETE = "#b8bcc8"
DARK = "#3a4156"
HAZARD = "#ffc928"


def duck(p, x, y, z, s, yaw=0.0):
    place = xf((x, y, z), (0, yaw, 0))
    p.add(ellipsoid(0.34 * s, 0.26 * s, 0.26 * s, 6, 3), place, "#ffd23c", grad=0.3, jitter=0.0)
    p.add(sphere(0.19 * s, 5, 3), place @ xf((0.26 * s, 0.26 * s, 0)), "#ffd23c", grad=0.2, jitter=0.0)
    p.add(box(0.14 * s, 0.06 * s, 0.14 * s), place @ xf((0.44 * s, 0.24 * s, 0)), "#ff8a2a", grad=0.0, jitter=0.0)


def silo():
    """Two tall grain silos with a catwalk, a caged ladder, a slanted conveyor and a duck weather vane."""
    p = c.Piece("silo")
    for name, x, z, r, h, col in (("a", 0.0, -3.4, 3.2, 13.0, STEEL), ("b", 0.0, 3.4, 2.9, 10.0, "#7a9cc4")):
        p.add(cylinder(r, r, h, 12), xf((x, h / 2.0, z)), col, grad=0.35)
        p.add(revolve([(0.0, h), (r, h), (r * 0.82, h + 0.9), (r * 0.42, h + 1.5), (0.0, h + 1.8)], 12), xf((x, 0, z)), "#d8dce8", grad=0.3)
        for y in range(3, int(h), 3):
            p.add(cylinder(r + 0.07, r + 0.07, 0.16, 12), xf((x, y, z)), "#8fa5c0", grad=0.1, jitter=0.0)
        stripes = lambda px, py, pz, x0=x, z0=z: HAZARD if (int(round((math.atan2(pz - z0, px - x0) % math.tau) / math.tau * 12)) % 2 == 0) else "#2b2f38"
        p.add(cylinder(r + 0.1, r + 0.1, 1.0, 12), xf((x, 0.5, z)), stripes, grad=0.0, jitter=0.0)
    p.add(box(1.0, 0.1, 6.8), xf((0.0, 12.2, 0.0)), DARK, grad=0.0)  # catwalk between the roofs
    for z in (-3.6, 3.6):
        p.add(box(0.06, 0.8, 0.06), xf((0.45, 12.6, z * 0.9)), DARK, grad=0.0, jitter=0.0)
    # caged ladder up silo A, facing +X
    for z in (-0.4, 0.4):
        p.add(strut((3.3, 0.0, -3.4 + z), (3.3, 11.8, -3.4 + z), 0.06, 4), None, DARK, grad=0.0, jitter=0.0)
    for k in range(7):
        p.add(strut((3.3, 1.2 + k * 1.7, -3.9), (3.3, 1.2 + k * 1.7, -2.9), 0.05, 4), None, DARK, grad=0.0, jitter=0.0)
    for y in (3.0, 6.0, 9.0):
        p.add(torus(0.62, 0.04, 8, 4), xf((3.3, y, -3.4), (90, 0, 90)), DARK, grad=0.0, jitter=0.0)
    # slanted conveyor from a hopper to the roof of silo A
    p.add(strut((6.8, 0.8, -3.4), (3.4, 12.6, -3.4), 0.38, 4), None, "#c9ced8", grad=0.2)
    p.add(strut((6.8, 0.8, -3.4), (3.4, 12.6, -3.4), 0.5, 4, 0.5), None, HAZARD, grad=0.0, jitter=0.0)
    p.add(c.lump(1.1, 0.9, 1.1, "hopper", 6, 3, 0.1, flat_bottom=True), xf((7.2, 0.7, -3.4)), "#d9622b", grad=0.3)
    for sx, sz in ((1, 1), (-1, 1), (1, -1), (-1, -1)):
        p.add(strut((7.2 + sx * 0.8, 0.0, -3.4 + sz * 0.8), (7.2 + sx * 0.55, 0.9, -3.4 + sz * 0.55), 0.07, 4), None, DARK, grad=0.0, jitter=0.0)
    # beacons, lamp, door
    p.add_flat(sphere(0.24, 5, 3), xf((0.0, 14.95, -3.4)), "#ff7a6a", mat="glow_red")
    p.add_flat(sphere(0.2, 5, 3), xf((0.0, 11.9, 3.4)), "#ff7a6a", mat="glow_red")
    p.add(box(0.2, 2.0, 1.2), xf((3.25, 1.0, 3.4)), DARK, grad=0.0)
    p.add_flat(sphere(0.18, 5, 3), xf((3.55, 2.4, 3.4)), "#ffd36a", mat="glow_amber")
    # duck weather vane on silo B
    p.add(strut((0.0, 11.8, 3.4), (0.0, 13.0, 3.4), 0.05, 4), None, DARK, grad=0.0)
    duck(p, 0.0, 13.15, 3.4, 1.0, yaw=20.0)
    return p


def smokestack():
    """Red and white banded chimney with a plinth, a railed platform, a rung ladder, beacons and a smoke ring."""
    p = c.Piece("smokestack")
    p.add(box(6.0, 1.1, 6.0, 0.1, 1), xf((0, 0.55, 0)), CONCRETE, grad=0.4)
    y = 1.1
    for i in range(8):
        h = 3.5
        r0, r1 = 2.3 - 0.115 * i, 2.3 - 0.115 * (i + 1)
        p.add(revolve([(0.0, y), (r0, y), (r1, y + h), (0.0, y + h)], 10), None, "#d8402e" if i % 2 == 0 else "#f4f4f8", grad=0.3, jitter=0.02)
        y += h
    top = y
    p.add(cylinder(1.5, 1.5, 0.5, 10), xf((0, top + 0.25, 0)), "#2b2f38", grad=0.0)
    p.add(cylinder(1.0, 1.0, 0.04, 10), xf((0, top + 0.52, 0)), "#16181e", grad=0.0, jitter=0.0)
    plat = 18.0
    p.add(cylinder(2.1, 2.1, 0.2, 10), xf((0, plat, 0)), DARK, grad=0.1)
    p.add(torus(2.0, 0.045, 10, 4), xf((0, plat + 1.0, 0)), DARK, grad=0.0, jitter=0.0)
    for k in range(5):
        a = math.tau * k / 5
        p.add(strut((2.0 * math.cos(a), plat, 2.0 * math.sin(a)), (2.0 * math.cos(a), plat + 1.0, 2.0 * math.sin(a)), 0.05, 4), None, DARK, grad=0.0, jitter=0.0)
    # ladder on the +X flank following the taper
    def radius(yy):
        return 2.3 - 0.115 * ((yy - 1.1) / 3.5)

    for z in (-0.3, 0.3):
        p.add(strut((radius(1.5) + 0.12, 1.5, z), (radius(top - 1.0) + 0.12, top - 1.0, z), 0.04, 3), None, DARK, grad=0.0, jitter=0.0)
    yy = 2.0
    while yy < top - 1.2:
        p.add(strut((radius(yy) + 0.12, yy, -0.3), (radius(yy) + 0.12, yy, 0.3), 0.035, 3), None, DARK, grad=0.0, jitter=0.0)
        yy += 1.5
    for y_b in (top + 0.9, plat + 1.2):
        p.add_flat(sphere(0.22, 5, 3), xf((0, y_b, 0) if y_b > top else (-2.0, y_b, 0)), "#ff7a6a", mat="glow_red")
    p.add(strut((0.0, top + 0.5, 0.0), (0.0, top + 1.2, 0.0), 0.04, 3), None, DARK, grad=0.0)
    # smoke: soft puffs drifting +X and a perfect smoke ring
    for i, (pos, rad) in enumerate((((0.8, top + 1.9, 0.1), 1.0), ((2.2, top + 3.3, -0.3), 1.35), ((4.0, top + 4.7, 0.2), 1.7))):
        p.add(c.lump(rad, rad * 0.8, rad, ("smoke", i), 7, 3, 0.16), xf(pos), "#e6ebf5", grad=0.3, jitter=0.0)
    p.add(torus(1.5, 0.5, 10, 5), xf((6.4, top + 6.4, 0.0), (0, 0, 70)), "#f1f4fa", grad=0.25, jitter=0.0)
    return p


def pipes():
    """Scatter pipe run: blue main with flanges, a red valve wheel, a riser, a copper line and a leaky joint."""
    p = c.Piece("pipes")
    for x in (-3.0, 3.0):
        p.add(box(0.3, 1.7, 1.3), xf((x, 0.85, 0.0)), CONCRETE, grad=0.4)
    p.add(cylinder_x(0.62, 0.62, 9.0, 8), xf((0, 1.9, 0)), STEEL, grad=0.4)
    for x in (-4.4, 0.0, 4.4):
        p.add(cylinder_x(0.8, 0.8, 0.26, 8), xf((x, 1.9, 0)), "#c9ced8", grad=0.2, jitter=0.0)
    p.add(cylinder_x(0.34, 0.34, 9.0, 6), xf((0, 1.15, 0.95)), "#d98a3c", grad=0.4)
    for x in (-2.0, 2.0):
        p.add(cylinder_x(0.45, 0.45, 0.2, 6), xf((x, 1.15, 0.95)), "#c9ced8", grad=0.2, jitter=0.0)
    p.add(cylinder(0.42, 0.42, 3.2, 8), xf((1.9, 3.1, -0.0)), STEEL, grad=0.3)
    p.add(strut((1.9, 3.6, 0.0), (1.9, 4.2, 0.0), 0.06, 4), None, DARK, grad=0.0)
    p.add(torus(0.5, 0.07, 8, 4), xf((1.9, 4.2, 0.0)), "#e0322e", grad=0.0, jitter=0.0)
    p.add(cylinder_x(0.28, 0.28, 0.16, 8), xf((0.0, 2.9, 0.0)), "#f4f4f8", grad=0.0, jitter=0.0)
    p.add(strut((0.0, 2.45, 0.0), (0.0, 2.8, 0.0), 0.09, 4), None, DARK, grad=0.0)
    p.add(c.lump(0.75, 0.7, 0.75, "steam", 6, 3, 0.15), xf((-1.9, 3.0, 0.0)), "#eef2fa", grad=0.3, jitter=0.0)
    p.add(c.lump(0.45, 0.4, 0.45, "steam2", 6, 3, 0.15), xf((-1.4, 3.9, 0.2)), "#eef2fa", grad=0.3, jitter=0.0)
    return p


def forklift():
    """Orange toy forklift with a cage cab, mast and forks, carrying a pallet of crates and three rubber ducks."""
    p = c.Piece("forklift")
    p.add(box(2.3, 0.6, 1.5, 0.06, 1), xf((-0.15, 0.85, 0)), "#f08a24", grad=0.4)
    p.add(box(1.0, 1.0, 1.5, 0.08, 1), xf((-1.35, 1.2, 0)), "#d9622b", grad=0.4)
    p.add(box(0.9, 0.5, 1.2), xf((0.45, 1.35, 0)), "#3a4156", grad=0.2)  # engine bonnet
    p.add(box(0.7, 0.12, 0.7), xf((-0.35, 1.3, 0)), "#2b2f38", grad=0.0, jitter=0.0)  # seat
    p.add(box(0.14, 0.6, 0.7), xf((-0.72, 1.6, 0)), "#2b2f38", grad=0.0, jitter=0.0)
    for x in (-0.75, 0.3):
        for z in (-0.65, 0.65):
            p.add(strut((x, 1.15, z), (x, 2.65, z), 0.05, 4), None, "#ffc928", grad=0.0, jitter=0.0)
    p.add(box(1.3, 0.1, 1.5), xf((-0.22, 2.7, 0)), "#ffc928", grad=0.0)
    p.add_flat(sphere(0.14, 5, 3), xf((-0.22, 2.84, 0)), "#ffc15e", mat="glow_amber")
    p.add(box(0.1, 0.6, 0.6), xf((0.0, 1.7, 0.0)), "#2b2f38", grad=0.0)
    p.add(strut((0.0, 1.55, 0.0), (0.45, 1.9, 0.0), 0.04, 3), None, "#2b2f38", grad=0.0)
    # mast, carriage and forks (front +X)
    for z in (-0.5, 0.5):
        p.add(box(0.2, 3.1, 0.2), xf((1.45, 1.65, z)), "#3a4156", grad=0.1)
    p.add(box(0.22, 0.2, 1.2), xf((1.45, 3.1, 0.0)), "#3a4156", grad=0.0)
    p.add(box(0.2, 0.9, 1.3), xf((1.65, 0.9, 0.0)), "#ffc928", grad=0.2)
    for z in (-0.4, 0.4):
        p.add(box(1.6, 0.1, 0.2), xf((2.5, 0.35, z)), "#9aa3b4", grad=0.0, jitter=0.0)
    # load: pallet, crates, ducks
    p.add(box(1.5, 0.14, 1.3), xf((2.55, 0.5, 0.0)), "#c98a4a", grad=0.1)
    for z in (-0.5, 0.0, 0.5):
        p.add(box(1.5, 0.12, 0.2), xf((2.55, 0.37, z)), "#a8703c", grad=0.1, jitter=0.0)
    p.add(box(0.9, 0.8, 1.1, 0.04, 1), xf((2.35, 1.0, 0.0)), "#d8a850", grad=0.2)
    p.add(box(0.5, 0.5, 0.5), xf((2.9, 0.82, 0.3)), "#41a8d8", grad=0.2)
    duck(p, 2.35, 1.62, 0.0, 1.6, yaw=-20.0)
    duck(p, 2.95, 1.22, 0.3, 0.9, yaw=30.0)
    duck(p, 2.9, 0.84, -0.4, 0.9, yaw=0.0)
    for x in (-1.0, 0.9):
        for z in (-0.85, 0.85):
            r = 0.42 if x < 0 else 0.34
            c.wheel(p, x, r, z, r, 0.3, detail=0, tyre="#2b2f38")
    p.add_flat(box(0.06, 0.16, 0.3), xf((1.0, 1.4, 0.4)), "#fff2cc", mat="glow_warm")
    p.add_flat(box(0.06, 0.16, 0.3), xf((1.0, 1.4, -0.4)), "#fff2cc", mat="glow_warm")
    p.add_flat(box(0.06, 0.2, 0.3), xf((-1.87, 1.25, 0.45)), "#ff7a6a", mat="glow_red")
    p.add_flat(box(0.06, 0.2, 0.3), xf((-1.87, 1.25, -0.45)), "#ff7a6a", mat="glow_red")
    return p


def pylon():
    """Lattice transmission mast: tapered legs, triangular bracing, three cross-arms with glassy insulators and a red-white tip."""
    p = c.Piece("pylon")
    H = 20.0
    half = lambda y: 2.2 - 1.55 * min(y, 15.0) / 15.0 if y < 15.0 else 0.65 - 0.2 * (y - 15.0) / 5.0
    for sx in (1, -1):
        for sz in (1, -1):
            p.add(strut((sx * 2.2, 0.0, sz * 2.2), (sx * 0.65, 15.0, sz * 0.65), 0.17, 4), None, "#9aa6bc", grad=0.4)
            p.add(strut((sx * 0.65, 15.0, sz * 0.65), (sx * 0.45, H, sz * 0.45), 0.13, 4), None, "#9aa6bc", grad=0.2)
            p.add(box(0.9, 0.4, 0.9), xf((sx * 2.2, 0.2, sz * 2.2)), CONCRETE, grad=0.3)
    levels = [0.4, 4.2, 8.2, 12.0, 15.0, 17.5, H]
    for i, y in enumerate(levels):
        h = half(y)
        for a, b in (((h, y, h), (h, y, -h)), ((h, y, -h), (-h, y, -h)), ((-h, y, -h), (-h, y, h)), ((-h, y, h), (h, y, h))):
            p.add(strut(a, b, 0.07, 3), None, "#d8402e" if i % 2 == 0 else "#f4f4f8", grad=0.2)
    for y0, y1 in zip(levels[:4], levels[1:5]):
        h0, h1 = half(y0), half(y1)
        for s in (1, -1):
            p.add(strut((s * h0, y0, h0), (s * h1, y1, -h1), 0.05, 3), None, "#c9ced8", grad=0.2)
            p.add(strut((s * h0, y0, -h0), (s * h1, y1, h1), 0.05, 3), None, "#c9ced8", grad=0.2)
            p.add(strut((h0, y0, s * h0), (-h1, y1, s * h1), 0.05, 3), None, "#c9ced8", grad=0.2)
            p.add(strut((-h0, y0, s * h0), (h1, y1, s * h1), 0.05, 3), None, "#c9ced8", grad=0.2)
    for y, span in ((12.0, 5.2), (15.0, 4.3), (17.5, 3.0)):
        p.add(box(0.6, 0.35, span * 2.0, 0.03, 1), xf((0, y, 0)), "#9aa6bc", grad=0.2)
        for z in (-span, span):
            p.add(strut((0, y - 0.1, z), (0, y - 1.5, z), 0.13, 5), None, "#6fd0c8", grad=0.2, jitter=0.0)
            p.add_flat(sphere(0.16, 4, 2), xf((0, y - 1.62, z)), "#bff8ff", mat="glow_cyan")
    p.add(revolve([(0.0, H), (0.5, H), (0.0, H + 1.8)], 6), None, "#d8402e", grad=0.3)
    p.add_flat(sphere(0.22, 5, 3), xf((0, H + 2.0, 0)), "#ff7a6a", mat="glow_red")
    return p


def build():
    c.ensure_materials()
    model = Model("scenery-industry")
    for name, piece in (("silo", silo()), ("smokestack", smokestack()), ("pipes", pipes()), ("forklift", forklift()), ("pylon", pylon())):
        piece.bake()
        model.mesh("prop-" + name, piece.items())
    return model
