"""Salt flat scenery: crystal cluster, landed flying saucer, six-wheel rover and a claim flag (MIT, AuraGo contributors).

Pastel crystals, a silver saucer with magenta rim lights and a sunbathing alien, a white-and-gold rover.
The saucer rests on three legs with 3 m of clearance under the belly (a cow fits) and carries a
``beam_emitter`` empty under its hull. Props stand at y=0, front toward +X.
"""

import math

from . import scenery_a_common as c
from .core import Model
from .shapes import box, cylinder, cylinder_x, cylinder_z, ellipsoid, loft, revolve, sphere, strut, xf

SALT_WHITE = "#f4f2ff"


def crystal(r, h, seg=6):
    """Hex prism with a pointed cap, slightly tapered."""
    return revolve([(0.0, 0.0), (r, 0.0), (r * 0.86, h), (0.0, h + r * 1.3)], seg)


def salt_crystal():
    """Cluster of seven leaning hexagonal crystals in lavender, rose and ice blue."""
    p = c.Piece("salt_crystal")
    specs = [
        ((0.0, 0.0, 0.0), 0.95, 3.6, (4, 0, -5), "#d9c8ff"),
        ((1.5, 0.0, 0.7), 0.7, 2.3, (-10, 20, -18), "#ffc6e4"),
        ((-1.3, 0.0, 0.9), 0.65, 2.7, (12, 0, 14), "#b8ecff"),
        ((0.3, 0.0, -1.6), 0.7, 2.0, (-15, 30, 12), "#e4d6ff"),
        ((-1.5, 0.0, -0.8), 0.45, 1.4, (8, 40, 28), "#ffc6e4"),
        ((2.2, 0.0, -0.9), 0.45, 1.2, (-8, 10, -30), "#b8ecff"),
        ((0.6, 0.0, 1.9), 0.4, 1.0, (22, 0, -10), "#e4d6ff"),
    ]
    tip = lambda base: lambda x, y, z: tuple(a * 0.6 + 0.4 for a in c.colour(base))
    for i, (pos, r, h, rot, col) in enumerate(specs):
        base = c.colour(col)

        def fn(x, y, z, base=base, top=h * 0.8):
            t = min(1.0, max(0.0, y / (top + 0.01)))
            return tuple(b * (0.78 + 0.22 * t) + 0.22 * t * (1.0 - b) for b in base)

        p.add(crystal(r, h), xf(pos, rot), fn, grad=0.2, jitter=0.02)
    p.add(c.lump(0.9, 0.35, 0.9, "salt-foot", 7, 3, 0.2, flat_bottom=True), xf((0.4, 0.18, 0.2)), SALT_WHITE, grad=0.3)
    return p


def ufo():
    """Landed saucer: silver hull with a magenta band, rim lights, dome, tripod legs, emitter ring, sunbathing alien."""
    p = c.Piece("ufo")
    band = lambda x, y, z: "#ff4fa0" if 4.3 < y < 4.7 else ("#aab4cc" if y < 4.3 else "#d8def0")
    p.add(revolve([(0.0, 3.2), (1.5, 3.2), (3.0, 3.55), (4.4, 4.1), (5.0, 4.5), (4.4, 4.95), (3.0, 5.25), (0.0, 5.35)], 16), None, band, grad=0.25)
    p.add(ellipsoid(2.2, 1.7, 2.2, 14, 5), xf((0, 5.2, 0)), "#9fe6ff", grad=0.3, jitter=0.0)
    p.add_flat(cylinder(1.35, 1.35, 0.14, 12), xf((0, 3.15, 0)), "#9dff8a", mat="glow_green")  # beam emitter ring
    for k in range(12):
        a = math.tau * k / 12
        mat, col = ("glow_pink", "#ff9ccf") if k % 2 else ("glow_warm", "#ffe2a0")
        p.add_flat(sphere(0.27, 5, 3), xf((math.cos(a) * 4.9, 4.5, math.sin(a) * 4.9)), col, mat=mat)
    for k in range(3):
        a = math.radians(90 + 120 * k)
        top = (math.cos(a) * 2.9, 3.7, math.sin(a) * 2.9)
        foot = (math.cos(a) * 4.0, 0.3, math.sin(a) * 4.0)
        p.add(strut(top, foot, 0.2, 6), None, "#7d87a2", grad=0.4)
        p.add(cylinder(0.6, 0.7, 0.3, 8), xf((foot[0], 0.15, foot[2])), "#5a6480", grad=0.2)
    # alien lounging on the dome: head, big eyes, antenna with a glow bulb, waving arm
    green = "#6ee05a"
    p.add(ellipsoid(0.75, 0.85, 0.7, 10, 5), xf((0.2, 7.1, 0)), green, grad=0.2, jitter=0.0)
    for z in (0.34, -0.34):
        p.add(ellipsoid(0.12, 0.28, 0.17, 6, 3), xf((0.85, 7.2, z), (0, 0, 0)), (0.02, 0.02, 0.05), grad=0.0, jitter=0.0)
    p.add(strut((0.2, 7.85, 0), (0.0, 8.6, 0.2), 0.05, 4), None, green, grad=0.0)
    p.add_flat(sphere(0.18, 5, 3), xf((0.0, 8.7, 0.2)), "#9dff8a", mat="glow_green")
    p.add(strut((0.1, 6.6, 0.7), (-0.2, 7.6, 1.5), 0.1, 5), None, green, grad=0.0)
    p.add(box(0.5, 0.08, 0.35), xf((0.4, 6.15, 0.0)), "#ff9a3c", grad=0.0, jitter=0.0)  # sun lounger mat
    return p


def rover():
    """Six-wheel rover: gold chassis, white body, blue solar deck, mast camera, dish and a drill arm."""
    p = c.Piece("rover")
    white, gold, dark = "#f2f2f8", "#e8b84a", "#3a4156"
    p.add(box(3.4, 0.6, 1.9, 0.06, 1), xf((0.0, 1.35, 0)), gold, grad=0.4)
    p.add(box(2.4, 0.9, 1.6, 0.1, 1), xf((-0.2, 2.1, 0)), white, grad=0.3)
    p.add(box(2.3, 0.1, 1.5), xf((-0.3, 2.62, 0), (0, 0, 0)), "#2a58b8", grad=0.0)
    for k in range(5):
        p.add(box(0.05, 0.12, 1.5), xf((-1.25 + k * 0.5, 2.66, 0)), "#9cb8ff", grad=0.0, jitter=0.0)
    for x in (-1.4, 0.0, 1.5):
        for z in (1.45, -1.45):
            p.add(cylinder_z(0.58, 0.58, 0.55, 10), xf((x, 0.58, z)), dark, grad=0.2, jitter=0.0)
            p.add(cylinder_z(0.3, 0.3, 0.58, 8), xf((x, 0.58, z)), "#ff7a2e", grad=0.1, jitter=0.0)
            p.add(strut((x * 0.7, 1.3, z * 0.55), (x, 0.58, z * 0.98), 0.09, 4), None, "#b8bfce", grad=0.2)
    p.add(strut((1.2, 2.5, 0.0), (1.4, 3.9, 0.0), 0.08, 5), None, "#b8bfce", grad=0.2)  # mast
    p.add(box(0.5, 0.4, 0.9, 0.05, 1), xf((1.5, 4.0, 0)), white, grad=0.2)
    for z in (0.28, -0.28):
        p.add(sphere(0.14, 5, 3), xf((1.78, 4.0, z)), (0.02, 0.02, 0.06), grad=0.0, jitter=0.0)
    p.add(revolve([(0.0, 0.0), (0.7, 0.0), (0.0, 0.22)], 10), xf((-1.3, 2.9, 0.5), (0, 0, -35)), white, grad=0.2)  # dish
    p.add(strut((-1.3, 2.7, 0.5), (-1.3, 2.9, 0.5), 0.05, 4), None, "#b8bfce", grad=0.0)
    p.add(strut((1.6, 1.4, -0.6), (2.5, 0.9, -0.6), 0.1, 5), None, "#b8bfce", grad=0.2)  # arm
    p.add(strut((2.5, 0.9, -0.6), (2.7, 0.35, -0.6), 0.14, 5, 0.04), None, "#ff7a2e", grad=0.0)
    p.add(box(0.1, 0.5, 0.8), xf((1.7, 2.1, 0.8)), "#ff7a2e", grad=0.0, jitter=0.0)  # stripe
    return p


def flag():
    """Claim flag: pole on a salt cairn, wavy two-tone cloth with a white planet disc."""
    p = c.Piece("flag")
    for i, (x, z, r) in enumerate(((0.0, 0.0, 0.7), (0.7, 0.4, 0.45), (-0.5, -0.5, 0.45), (0.2, -0.7, 0.35))):
        p.add(c.lump(r, r * 0.7, r, ("flag", i), 6, 3, 0.2, flat_bottom=True), xf((x, r * 0.38, z)), SALT_WHITE, grad=0.35)
    p.add(strut((0.0, 0.2, 0.0), (0.0, 7.0, 0.0), 0.1, 6, 0.07), None, "#d8dce8", grad=0.4)
    p.add(sphere(0.2, 6, 3), xf((0.0, 7.1, 0.0)), "#ffd23c", grad=0.0, jitter=0.0)
    stations = 8
    rings = []
    for i in range(stations):
        z = 0.1 + 2.9 * i / (stations - 1)
        x = 0.05 + 0.28 * math.sin(i * 0.95) * (i / (stations - 1))
        y0, y1 = 4.9 - 0.1 * (i / stations), 6.7 - 0.1 * (i / stations)
        rings.append([(x - 0.04, y0, z), (x + 0.04, y0, z), (x + 0.04, y1, z), (x - 0.04, y1, z)])
    cloth = lambda x, y, z: "#ff4fa0" if y > 5.8 else "#2fd0dc"
    p.add(loft(rings), None, cloth, grad=0.1, jitter=0.0)
    p.add(cylinder_x(0.55, 0.55, 0.08, 10), xf((0.35, 5.8, 1.6)), "#ffffff", grad=0.0, jitter=0.0)
    p.add(cylinder_x(0.24, 0.24, 0.1, 8), xf((0.37, 5.8, 1.6)), "#ff4fa0", grad=0.0, jitter=0.0)
    return p


def build():
    c.ensure_materials()
    model = Model("scenery-salt")
    for name, piece in (("salt_crystal", salt_crystal()), ("ufo", ufo()), ("rover", rover()), ("flag", flag())):
        piece.bake(ao_height=2.4)
        root = model.mesh("prop-" + name, piece.items())
        if name == "ufo":
            model.empty("beam_emitter", (0.0, 3.1, 0.0), parent=root)
    return model
