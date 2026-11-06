"""Forest scenery: oak, birch, a gnome-door mushroom cluster, a log pile and a hunting hide (MIT, AuraGo contributors).

Deep emerald and moss greens with a golden-birch accent, a huge fly-agaric with a gnome door (the
comic touch) and a stilted hunting hide with a lit slit window. Props stand at y=0, front toward +X.
"""

import math

from . import scenery_b_common as c
from .core import Model
from .shapes import box, cylinder, cylinder_x, cylinder_z, ellipsoid, extrude_z, revolve, sphere, spline, strut, xf

BARK = "#7a4a30"
BARK_DARK = "#5a3622"


def oak():
    """Scatter oak: flared trunk, two stub branches, five faceted leaf clumps, red apples and an owl hollow."""
    p = c.Piece("oak")
    p.add(revolve([(0.0, 0.0), (0.95, 0.0), (0.55, 1.1), (0.42, 3.5), (0.0, 3.5)], 6), None, BARK, grad=0.5)
    p.add(strut((0.0, 2.6, 0.0), (1.6, 4.3, 0.5), 0.22, 5), None, BARK, grad=0.3)
    p.add(strut((0.0, 2.9, 0.0), (-1.5, 4.6, -0.6), 0.2, 5), None, BARK, grad=0.3)
    clumps = [
        ((0.0, 5.9, 0.0), (3.4, 2.7, 3.4), "#2c853c"),
        ((2.0, 5.1, 1.3), (2.3, 1.9, 2.3), "#34963f"),
        ((-1.9, 5.5, -1.1), (2.4, 2.0, 2.4), "#2a7a38"),
        ((-0.6, 4.5, 2.0), (2.0, 1.6, 2.0), "#3a9f45"),
        ((0.4, 7.9, -0.5), (2.3, 1.8, 2.3), "#6cc64a"),
    ]
    for i, (pos, rad, col) in enumerate(clumps):
        p.add(c.lump(rad[0], rad[1], rad[2], ("oak", i), 7, 4, 0.2), xf(pos, (0, 37 * i, 0)), col, grad=0.55)
    for pos in ((2.6, 4.4, 2.2), (-1.5, 3.8, 2.6), (1.0, 4.0, -2.4), (3.0, 5.1, -0.4)):
        p.add(sphere(0.2, 4, 2), xf(pos), "#e8352e", grad=0.0, jitter=0.0)
    # owl hollow facing +X
    p.add(ellipsoid(0.16, 0.38, 0.3, 6, 3), xf((0.44, 2.0, 0.0)), "#2a1a12", grad=0.0, jitter=0.0)
    for z in (-0.12, 0.12):
        p.add(sphere(0.085, 4, 2), xf((0.58, 2.1, z)), "#ffd23c", grad=0.0, jitter=0.0)
    return p


def birch():
    """Scatter birch: slim white trunk with dark bands, lime and golden clumps."""
    p = c.Piece("birch")
    radius = spline([(0.0, 0.5), (2.0, 0.32), (6.8, 0.25)])
    p.add(revolve([(0.0, 0.0), (0.5, 0.0), (0.32, 2.0), (0.25, 6.8), (0.0, 6.8)], 6), None, "#f1eee2", grad=0.4)
    for y in (0.9, 2.0, 3.1, 4.3, 5.5):
        p.add(cylinder(radius(y) + 0.025, radius(y) + 0.025, 0.2, 6), xf((0, y, 0), (0, 13 * y, 0)), "#2b2b30", grad=0.0, jitter=0.0)
    p.add(strut((0.0, 4.2, 0.0), (1.1, 5.7, 0.3), 0.14, 5), None, "#d9d4c4", grad=0.2)
    p.add(strut((0.0, 4.9, 0.0), (-1.0, 6.3, -0.3), 0.13, 5), None, "#d9d4c4", grad=0.2)
    for i, (pos, rad, col) in enumerate(
        [
            ((0.0, 7.0, 0.0), (1.7, 2.6, 1.7), "#8fd04b"),
            ((1.2, 6.0, 0.5), (1.3, 1.9, 1.3), "#a9de58"),
            ((-1.1, 6.6, -0.5), (1.3, 2.0, 1.3), "#e0c43c"),
            ((0.1, 8.7, 0.2), (1.1, 1.7, 1.1), "#c3e868"),
        ]
    ):
        p.add(c.lump(rad[0], rad[1], rad[2], ("birch", i), 7, 4, 0.18), xf(pos, (0, 53 * i, 0)), col, grad=0.5)
    return p


def mushroom(p, x, z, s, cap, stem="#f4e6c8", spots=None, seg=7, tilt=0.0):
    """Toy toadstool of scale ``s`` (height about 3.2 s): fat stem, domed cap, cream gills."""
    place = xf((x, 0, z), (tilt, 0, tilt * 0.6))
    p.add(revolve([(0.0, 0.0), (0.62 * s, 0.0), (0.46 * s, 0.4 * s), (0.4 * s, 1.5 * s), (0.0, 1.5 * s)], seg), place, stem, grad=0.45)
    R = 1.5 * s
    base = 1.35 * s
    gills = c.mix("#e8d2a0", cap, 0.0)

    def fn(px, py, pz):
        return gills if py < base + 0.08 * s else cap

    prof = [(0.0, base), (R, base + 0.04 * s), (R * 1.03, base + 0.5 * s), (R * 0.72, base + 1.2 * s), (R * 0.36, base + 1.7 * s), (0.0, base + 1.85 * s)]
    p.add(revolve(prof, seg), place, fn, grad=0.4)
    if spots:
        top = spline([(0.0, base + 1.85 * s), (R * 0.36, base + 1.7 * s), (R * 0.72, base + 1.2 * s), (R * 1.03, base + 0.5 * s)])
        for i, (rf, ang, size) in enumerate(spots):
            r = R * rf
            ax = math.radians(ang)
            pos = (r * math.cos(ax), top(r) - 0.03 * s, r * math.sin(ax))
            p.add(ellipsoid(size * s, size * 0.45 * s, size * s, 6, 3), place @ xf(pos), "#fff6e0", grad=0.0, jitter=0.0)


def mushroom_cluster():
    """Huge fake fly-agaric with a teal gnome door and a lit round window, flanked by small toadstools."""
    p = c.Piece("mushroom_cluster")
    big = 2.1
    mushroom(p, 0, 0, big, "#e0322e", spots=[(0.2, 20, 0.34), (0.55, 70, 0.3), (0.62, 150, 0.32), (0.6, 230, 0.3), (0.5, 300, 0.33), (0.8, 345, 0.22)], seg=10)
    # gnome door on the +X side of the stem (the stem is 1.3 m wide at the foot)
    door_x = 0.4 * big + 0.5
    p.add(box(0.28, 1.65, 1.15), xf((door_x - 0.02, 0.82, 0.0)), "#fff0d0", grad=0.1, jitter=0.0)
    p.add(cylinder_x(0.575, 0.575, 0.28, 8), xf((door_x - 0.02, 1.64, 0.0)), "#fff0d0", grad=0.1, jitter=0.0)
    p.add(box(0.3, 1.5, 0.9), xf((door_x + 0.0, 0.76, 0.0)), "#1f9d94", grad=0.35, jitter=0.0)
    p.add(cylinder_x(0.45, 0.45, 0.3, 8), xf((door_x + 0.0, 1.5, 0.0)), "#1f9d94", grad=0.2, jitter=0.0)
    p.add(sphere(0.07, 5, 3), xf((door_x + 0.2, 0.75, 0.28)), "#ffd36a", grad=0.0, jitter=0.0)
    p.add(box(0.8, 0.07, 1.3, 0.0), xf((door_x + 0.55, 0.04, 0.0)), "#d8402e", grad=0.1, jitter=0.0)
    p.add(cylinder(0.2, 0.26, 0.1, 7), xf((door_x + 0.35, 0.05, 0.95)), "#b9b3a4", grad=0.2)
    p.add(cylinder(0.16, 0.21, 0.1, 7), xf((door_x + 0.75, 0.05, -0.9)), "#b9b3a4", grad=0.2)
    p.add_flat(cylinder_x(0.3, 0.3, 0.1, 8), xf((0.4 * big + 0.12, 2.35, 0.0)), "#ffe2a0", mat="glow_warm")
    p.add(cylinder_x(0.38, 0.38, 0.08, 8), xf((0.4 * big + 0.08, 2.35, 0.0)), "#7a4a30", grad=0.0, jitter=0.0)
    p.add(strut((door_x + 0.9, 0.0, 0.65), (door_x + 0.9, 1.25, 0.65), 0.05, 4), None, BARK, grad=0.2)
    p.add_flat(sphere(0.14, 5, 3), xf((door_x + 0.9, 1.35, 0.65)), "#ffe2a0", mat="glow_warm")
    mushroom(p, 3.6, 2.8, 1.0, "#ff8a3c", spots=[(0.4, 30, 0.3), (0.65, 160, 0.26), (0.5, 270, 0.28)], tilt=6.0)
    mushroom(p, -2.6, 3.4, 0.8, "#9a5fd0", spots=[(0.45, 80, 0.3), (0.7, 220, 0.25)], tilt=-5.0)
    mushroom(p, 2.4, -3.3, 0.9, "#e0322e", spots=[(0.4, 10, 0.3), (0.7, 200, 0.26)], tilt=4.0)
    for x, z, s, col in ((-3.2, -2.0, 0.45, "#4aa8ff"), (-2.5, -2.7, 0.35, "#ff6fb0"), (4.5, -0.6, 0.4, "#ffd23c"), (-1.4, 3.9, 0.3, "#4aa8ff"), (1.6, 4.4, 0.35, "#ff6fb0")):
        mushroom(p, x, z, s, col, seg=6)
    return p


def log(p, cx, cy, cz, r, half, col, bark_hex):
    """A log along Z with pale end grain (a ring of bark colour in the chamfer)."""
    grain = c.mix("#e9c98f", "#c99a5e", 0.35)
    bark = c.colour(bark_hex)

    def fn(x, y, z):
        end = abs(z - cz) > half - 0.01
        inner = math.hypot(x - cx, y - cy) < r * 0.93
        return grain if (end and inner) else bark

    p.add(cylinder_z(r, r, half * 2.0, 6, chamfer=0.05), xf((cx, cy, cz)), fn, grad=0.2)


def log_pile():
    """Stacked pyramid of nine logs between stakes, a chopping stump with an axe and wood chips."""
    p = c.Piece("log_pile")
    rows = [4, 3, 2]
    cols = ["#9a6334", "#8a552b", "#a8703c", "#7e4c28"]
    i = 0
    for row, count in enumerate(rows):
        for k in range(count):
            half = 1.6 + 0.12 * ((i * 7) % 3)
            log(p, (k - (count - 1) / 2.0) * 1.0, 0.5 + row * 0.87, 0.1 * ((i * 5) % 3 - 1), 0.5, half, cols[i % 4], cols[i % 4])
            i += 1
    for z in (-2.1, 2.1):
        for x in (-2.3, 2.3):
            p.add(strut((x, 0.0, z), (x * 1.05, 2.9, z), 0.1, 4), None, "#5a3622", grad=0.2)
    # chopping stump with an axe stuck in it, in front (+X)
    p.add(cylinder(0.9, 1.0, 1.1, 8, chamfer=0.06), xf((3.9, 0.55, 0.8)), "#8a552b", grad=0.4)
    p.add(cylinder(0.76, 0.76, 0.05, 8), xf((3.9, 1.1, 0.8)), "#e9c98f", grad=0.0, jitter=0.0)
    p.add(strut((3.8, 1.1, 0.8), (3.5, 2.6, 0.9), 0.07, 4), None, "#a8703c", grad=0.0)
    p.add(box(0.5, 0.42, 0.1), xf((3.92, 1.4, 0.8)), "#9aa3b4", grad=0.0, jitter=0.0)
    for pos, ang in (((2.9, 0.06, -0.9), 20), ((4.4, 0.06, 1.9), 70), ((3.1, 0.06, 2.0), 140), ((4.5, 0.06, -0.5), 100)):
        p.add(box(0.5, 0.1, 0.2), xf(pos, (0, ang, 0)), "#d9b074", grad=0.0)
    return p


def hunting_hide():
    """A cabin on stilts with a ladder, lantern-lit shooting slit, moss roof and a cardboard stag decoy."""
    p = c.Piece("hunting_hide")
    H = 4.2
    for sx in (1, -1):
        for sz in (1, -1):
            p.add(strut((sx * 2.0, 0.0, sz * 1.8), (sx * 1.7, H, sz * 1.55), 0.17, 5), None, BARK, grad=0.5)
            p.add(box(0.7, 0.3, 0.7), xf((sx * 2.0, 0.15, sz * 1.8)), "#8d90a6", grad=0.3)
    for sx in (1, -1):
        p.add(strut((sx * 1.95, 0.7, 1.75), (sx * 1.75, 3.5, -1.6), 0.07, 4), None, "#6a4328", grad=0.2)
        p.add(strut((sx * 1.95, 0.7, -1.75), (sx * 1.75, 3.5, 1.6), 0.07, 4), None, "#6a4328", grad=0.2)
    for sz in (1, -1):
        p.add(strut((1.9, 1.3, sz * 1.7), (-1.9, 1.3, sz * 1.7), 0.07, 4), None, "#6a4328", grad=0.0)
    p.add(box(4.8, 0.3, 4.3, 0.04, 1), xf((0, H + 0.15, 0)), "#a5683c", grad=0.3)
    shades = ["#c27a46", "#b36a3c", "#cf864d", "#a85f35", "#bd7442", "#b36a3c", "#c98049"]
    for i in range(7):
        p.add(box(3.7, 0.33, 3.5), xf((-0.3, H + 0.45 + i * 0.33 + 0.1, 0)), shades[i], grad=0.1, jitter=0.0)
    top = H + 0.45 + 7 * 0.33 + 0.1
    p.add(extrude_z([(-2.1, top - 0.1), (1.5, top - 0.1), (-0.3, top + 1.05)], -1.95, 1.95), None, "#c27a46", grad=0.15)
    for sgn in (1, -1):
        p.add(box(2.35, 0.2, 4.6, 0.03, 1), xf((-0.3 + sgn * 0.92, top + 0.62, 0), (0, 0, -sgn * 27)), "#3f6e45", grad=0.4)
    p.add(c.lump(0.9, 0.3, 1.2, "moss", 6, 3, 0.2), xf((-0.9, top + 1.02, 0.5)), "#6cc64a", grad=0.3)
    # shooting slit and lantern on the +X face (front)
    fx = 1.55 + 0.05
    p.add(box(0.1, 0.62, 1.9), xf((fx, H + 1.55, 0)), "#2a1a12", grad=0.0, jitter=0.0)
    p.add_flat(c.wall_quad((fx + 0.06, H + 1.55, 0), (1, 0, 0), 1.6, 0.34), None, "#ffe2a0", mat="glow_warm")
    p.add(box(0.3, 0.14, 1.5), xf((fx + 0.1, H + 1.18, 0)), "#7a4a30", grad=0.2)
    # ladder at the front, leaning on the platform
    for z in (-0.55, 0.55):
        p.add(strut((3.2, 0.0, z), (2.45, H + 0.2, z), 0.07, 4), None, "#a8703c", grad=0.2)
    for k in range(10):
        t = (k + 0.7) / 10.8
        p.add(strut((3.2 - 0.75 * t, t * (H + 0.2), -0.55), (3.2 - 0.75 * t, t * (H + 0.2), 0.55), 0.045, 4), None, "#8a552b", grad=0.0, jitter=0.0)
    for z in (-2.0, 2.0):
        p.add(strut((2.4, H + 0.3, z), (2.4, H + 1.3, z), 0.07, 4), None, BARK, grad=0.2)
    for z in (-2.0, 2.0):
        p.add(strut((2.4, H + 1.25, z), (2.4, H + 1.25, z * 0.3), 0.05, 4), None, BARK, grad=0.0)
    # cardboard stag decoy
    sx0, sz0 = 5.2, -1.8
    p.add(ellipsoid(0.7, 0.42, 0.34, 7, 3), xf((sx0, 1.25, sz0)), "#b8763c", grad=0.3)
    for dx, dz in ((0.45, 0.14), (0.45, -0.14), (-0.45, 0.14), (-0.45, -0.14)):
        p.add(strut((sx0 + dx, 1.05, sz0 + dz), (sx0 + dx, 0.0, sz0 + dz), 0.06, 4), None, "#8a552b", grad=0.2)
    p.add(strut((sx0 + 0.55, 1.4, sz0), (sx0 + 0.85, 2.0, sz0), 0.12, 5, 0.09), None, "#b8763c", grad=0.2)
    p.add(ellipsoid(0.26, 0.2, 0.18, 6, 3), xf((sx0 + 1.0, 2.1, sz0)), "#b8763c", grad=0.2)
    p.add(sphere(0.07, 4, 2), xf((sx0 + 1.24, 2.1, sz0)), "#2a1a12", grad=0.0, jitter=0.0)
    for z in (-0.1, 0.1):
        p.add(strut((sx0 + 0.9, 2.28, sz0 + z), (sx0 + 0.78, 2.95, sz0 + z * 4.0), 0.04, 4), None, "#e9c98f", grad=0.0, jitter=0.0)
        p.add(strut((sx0 + 0.84, 2.62, sz0 + z * 3.0), (sx0 + 1.2, 2.9, sz0 + z * 3.5), 0.03, 4), None, "#e9c98f", grad=0.0, jitter=0.0)
    return p


def build():
    c.ensure_materials()
    model = Model("scenery-forest")
    for name, piece in (("oak", oak()), ("birch", birch()), ("mushroom_cluster", mushroom_cluster()), ("log_pile", log_pile()), ("hunting_hide", hunting_hide())):
        piece.bake()
        model.mesh("prop-" + name, piece.items())
    return model
