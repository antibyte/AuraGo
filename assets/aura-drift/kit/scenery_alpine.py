"""Alpine scenery: two pines, two boulders, a timber chalet with a gnome and a cable-car pylon (MIT, AuraGo contributors).

Golden-hour toy palette: deep teal-green pines with snow tips, blue-grey rocks with snow caps,
a red-roofed chalet. Props stand at y=0, footprint centred, front toward +X.
"""

import math

from . import scenery_a_common as c
from .core import Model
from .polish_props import trail_marker
from .weather_props import snow_gauge
from .shapes import box, cylinder, cylinder_z, extrude_z, revolve, sphere, strut, xf

SNOW = "#f2f7ff"
SNOW_SHADE = "#c9d8f0"


def tier(piece, y0, height, radius, color, tip=None, seg=8, yaw=0.0):
    """One skirt of a conifer: a cone with a flat underside, optional snow-dusted tip cone."""
    piece.add(revolve([(0.0, y0), (radius, y0), (radius * 0.12, y0 + height), (0.0, y0 + height)], seg), xf((0, 0, 0), (0, yaw, 0)), color, grad=0.45)
    if tip:
        piece.add(revolve([(0.0, y0 + height * 0.46), (radius * 0.68, y0 + height * 0.46), (0.0, y0 + height + 0.12)], seg), xf((0, 0, 0), (0, yaw, 0)), tip, grad=0.12, jitter=0.0)


def pine_a():
    """Chunky round pine: four fat skirts, each lighter and smaller."""
    p = c.Piece("pine_a")
    p.add(cylinder(0.42, 0.3, 2.2, 6), xf((0, 1.1, 0)), "#7a4a30", grad=0.5)
    greens = ["#1b5d46", "#237a4f", "#2f9a56", "#4fba5c"]
    for i, (y, h, r) in enumerate([(1.3, 3.3, 3.0), (3.4, 3.0, 2.45), (5.3, 2.6, 1.9), (6.95, 2.4, 1.3)]):
        tier(p, y, h, r, greens[i], seg=9, yaw=14.0 * i)
    p.add(sphere(0.22, 6, 3), xf((0, 9.5, 0)), "#ffd36a", grad=0.0, jitter=0.0)
    return p


def pine_b():
    """Slim snow spruce: six narrow skirts with white tips."""
    p = c.Piece("pine_b")
    p.add(cylinder(0.34, 0.26, 2.0, 6), xf((0, 1.0, 0)), "#6e4429", grad=0.5)
    greens = ["#15473f", "#1b5b48", "#237052", "#2b8558", "#349a5e", "#44b068"]
    y = 1.2
    for i in range(6):
        h = 2.5 - 0.18 * i
        r = 2.2 - 0.3 * i
        tier(p, y, h, r, greens[i], tip=SNOW, seg=8, yaw=19.0 * i)
        y += 1.45 - 0.07 * i
    return p


def snowy(rock, line, soft):
    """Vertex colour: rock below ``line`` metres, snow above, blended over ``soft`` metres (a toy snow line)."""
    r, s, s2 = c.colour(rock), c.colour(SNOW), c.colour(SNOW_SHADE)

    def fn(x, y, z):
        t = min(1.0, max(0.0, (y - line) / soft))
        t = t * t * (3.0 - 2.0 * t)
        top = s if (x + z) > 0 else s2
        return tuple(a * (1 - t) + b * t for a, b in zip(r, top))

    return fn


def rock_a():
    """Boulder cluster: a big faceted rock, two siblings, snow settled on the tops."""
    p = c.Piece("rock_a")
    for i, (pos, rad, col, line) in enumerate([((0, -0.5, 0), (2.9, 3.0, 2.5), "#4f5a7c", 3.4), ((2.6, -0.3, 1.6), (1.7, 1.7, 1.6), "#5d6a8e", 2.2), ((-2.0, -0.3, -1.8), (1.5, 1.4, 1.5), "#47516f", 1.7)]):
        p.add(c.lump(rad[0], rad[1], rad[2], ("rock_a", i), 8, 5, 0.26, flat_bottom=True), xf((pos[0], 0.52 * rad[1], pos[2]), (0, 31.0 * i, 0)), snowy(col, line, 0.9), grad=0.5)
    return p


def rock_b():
    """Three leaning spires (tall facets read from the chase camera) with a snow line."""
    p = c.Piece("rock_b")
    for i, (pos, rad, rot, col, line) in enumerate([((0, 0.0, 0), (1.8, 5.2, 1.6), (4, 20, -6), "#46506f", 5.6), ((2.0, 0.0, 0.9), (1.3, 3.0, 1.2), (-5, 48, 9), "#56628a", 3.5), ((-1.7, 0.0, -1.0), (1.3, 2.4, 1.4), (3, 70, -9), "#626f96", 2.9)]):
        p.add(c.lump(rad[0], rad[1], rad[2], ("rock_b", i), 7, 5, 0.22, flat_bottom=True), xf((pos[0], 0.45 * rad[1], pos[2]), rot), snowy(col, line, 1.2), grad=0.5)
    p.add(c.lump(1.2, 0.8, 1.2, "rock_b-foot", 7, 3, 0.25, flat_bottom=True), xf((2.6, 0.42, -1.8)), "#8190b0", grad=0.5)
    return p


def window(p, pos, size, rot=None):
    p.add_flat(box(*size), xf(pos, rot), "#ffe2a0", mat="glow_warm")
    frame = (size[0] + 0.02, size[1] + 0.3, size[2] + 0.3) if size[0] < 0.2 else (size[0] + 0.3, size[1] + 0.3, size[2] + 0.02)
    p.add(box(*frame), xf((pos[0] - (0.03 if size[0] < 0.2 else 0), pos[1], pos[2] - (0.03 if size[2] < 0.2 else 0))), "#f0e6d0", grad=0.1)


def gnome(p, x, z):
    s = 1.45
    p.add(box(0.55, 0.3, 0.6, 0.05, 1), xf((x, 1.2 + 0.15 * s, z), None, s), "#3a3f5a", grad=0.1)  # boots
    p.add(revolve([(0.0, 0.0), (0.34, 0.0), (0.22, 0.7), (0.0, 0.8)], 7), xf((x, 1.3 + 0.15 * s, z), None, s), "#2f7fd8", grad=0.4)
    p.add(sphere(0.22, 7, 4), xf((x, 1.3 + 0.95 * s, z), None, s), "#f2b896", grad=0.1, jitter=0.0)
    p.add(revolve([(0.0, 0.0), (0.24, 0.0), (0.0, -0.6)], 7), xf((x + 0.1 * s, 1.3 + 0.88 * s, z), (0, 0, 14), s), "#f6f6f6", grad=0.1, jitter=0.0)  # beard
    p.add(revolve([(0.0, 0.0), (0.3, 0.0), (0.0, 0.95)], 7), xf((x, 1.3 + 1.08 * s, z), None, s), "#e0322e", grad=0.3, jitter=0.0)  # hat


def chalet():
    p = c.Piece("chalet")
    # stone plinth, timber walls with log courses, front toward +X
    p.add(box(6.8, 1.0, 8.2, 0.08, 1), xf((0, 0.5, 0)), "#8d90a6", grad=0.5)
    p.add(box(6.0, 3.2, 7.4, 0.08, 1), xf((0, 2.6, 0)), "#b36a3c", grad=0.25)
    for i in range(7):
        y = 1.3 + i * 0.45
        shade = "#9e5a32" if i % 2 else "#c27a46"
        p.add(box(6.12, 0.16, 7.5), xf((0, y + 0.1, 0)), shade, grad=0.1)
    # gable ends and roof
    p.add(extrude_z([(-3.0, 4.2), (3.0, 4.2), (0.0, 6.4)], -3.7, 3.7), None, "#c27a46", grad=0.15)
    tilt = 38.0
    for sign in (1, -1):
        p.add(box(4.9, 0.32, 9.0, 0.05, 1), xf((sign * 1.78, 5.4, 0), (0, 0, -sign * tilt)), "#c8352e", grad=0.4)
        p.add(box(2.6, 0.2, 8.3), xf((sign * 1.1, 5.88, 0), (0, 0, -sign * tilt)), SNOW, grad=0.2, jitter=0.0)
    p.add(box(0.45, 0.4, 9.1, 0.05, 1), xf((0, 6.78, 0)), "#7a3b2a", grad=0.2)
    # chimney with a snow cap
    p.add(box(1.0, 2.2, 1.0, 0.05, 1), xf((-1.0, 6.2, -2.4)), "#8d90a6", grad=0.5)
    p.add(box(1.2, 0.25, 1.2), xf((-1.0, 7.35, -2.4)), SNOW, grad=0.1)
    # porch on the +X side: deck, posts, shed roof, door, gnome
    p.add(box(2.2, 0.25, 5.0, 0.04, 1), xf((4.3, 0.95, 0)), "#a5683c", grad=0.3)
    for z in (-2.2, 2.2):
        p.add(box(0.28, 3.0, 0.28), xf((5.2, 2.45, z)), "#7a4a30", grad=0.4)
    p.add(box(2.7, 0.22, 5.4, 0.04, 1), xf((4.2, 4.0, 0), (0, 0, -9)), "#c8352e", grad=0.3)
    p.add(box(2.4, 0.14, 5.2), xf((4.2, 4.18, 0), (0, 0, -9)), SNOW, grad=0.1, jitter=0.0)
    p.add(box(0.2, 2.3, 1.5, 0.04, 1), xf((3.03, 2.28, 0)), "#5a3a8a", grad=0.3)
    p.add(sphere(0.1, 5, 3), xf((3.15, 2.2, 0.5)), "#ffd36a", grad=0.0, jitter=0.0)
    window(p, (3.02, 3.0, 2.3), (0.08, 0.9, 0.9))
    window(p, (3.02, 3.0, -2.3), (0.08, 0.9, 0.9))
    window(p, (0.0, 3.0, 3.72), (1.0, 0.9, 0.08))
    window(p, (0.0, 3.0, -3.72), (1.0, 0.9, 0.08))
    window(p, (2.0, 3.0, -3.72), (1.0, 0.9, 0.08))
    window(p, (3.02, 5.1, 0), (0.08, 0.7, 0.7))
    # flower boxes
    for z in (2.3, -2.3):
        p.add(box(0.35, 0.28, 1.1), xf((3.22, 2.4, z)), "#7a4a30", grad=0.2)
        for k in (-0.35, 0.0, 0.35):
            p.add(sphere(0.16, 5, 3), xf((3.25, 2.65, z + k)), ["#ff5a7a", "#ffd23c", "#ff8a3c"][int(k * 3 + 1) % 3], grad=0.0, jitter=0.0)
    # log pile along the back wall
    for row, count in enumerate((4, 3, 2)):
        for k in range(count):
            p.add(cylinder_z(0.22, 0.22, 2.0, 6), xf((-3.35, 1.3 + row * 0.38, 1.0 + k * 0.46 + row * 0.23 - 1.3), (0, 0, 0)), "#b97d4a", grad=0.2)
    p.add_flat(sphere(0.18, 6, 3), xf((4.85, 3.68, -1.2)), "#ffe2a0", mat="glow_warm")
    for dz, col in ((-1.95, "#e8382d"), (-1.75, "#2f88f0")):
        p.add(box(0.08, 1.95, 0.14, 0.02, 1), xf((5.02, 1.95, dz), (0, 0, 12)), col, grad=0.2)
    gnome(p, 4.9, 1.1)
    return p


def cablecar_pylon():
    """Red and white lattice mast: four legs, ring braces, X bracing, a cross-arm with sheave wheels."""
    p = c.Piece("cablecar_pylon")
    for sx in (1, -1):
        for sz in (1, -1):
            p.add(strut((sx * 2.4, 0.0, sz * 2.4), (sx * 0.8, 13.0, sz * 0.8), 0.2, 6), None, "#d8dce8", grad=0.35)
            p.add(box(1.0, 0.5, 1.0, 0.05, 1), xf((sx * 2.4, 0.25, sz * 2.4)), "#8d90a6", grad=0.3)
    half = lambda y: 2.4 - 1.6 * y / 13.0
    levels = [0.5, 3.8, 7.0, 10.0, 12.9]
    for i, y in enumerate(levels):
        h = half(y)
        for a, b in (((h, y, h), (h, y, -h)), ((h, y, -h), (-h, y, -h)), ((-h, y, -h), (-h, y, h)), ((-h, y, h), (h, y, h))):
            p.add(strut(a, b, 0.11, 4), None, "#d8402e" if i % 2 == 0 else "#f4f4f8", grad=0.2)
    for y0, y1 in zip(levels, levels[1:]):
        h0, h1 = half(y0), half(y1)
        for s in (1, -1):
            p.add(strut((s * h0, y0, h0), (s * h1, y1, -h1), 0.08, 4), None, "#f4f4f8", grad=0.2)
            p.add(strut((s * h0, y0, -h0), (s * h1, y1, h1), 0.08, 4), None, "#f4f4f8", grad=0.2)
            p.add(strut((h0, y0, s * h0), (-h1, y1, s * h1), 0.08, 4), None, "#f4f4f8", grad=0.2)
    p.add(box(0.9, 0.5, 7.4, 0.06, 1), xf((0, 13.5, 0)), "#d8402e", grad=0.3)
    p.add(box(1.5, 0.7, 1.5, 0.06, 1), xf((0, 13.0, 0)), "#f4f4f8", grad=0.3)
    for z in (3.2, -3.2):
        p.add(cylinder(0.55, 0.55, 0.3, 10), xf((0, 14.25, z), (90, 0, 0)), "#3a4156", grad=0.2)
        p.add(box(0.2, 1.0, 0.2), xf((0, 13.95, z * 1.05)), "#3a4156", grad=0.0)
    p.add_flat(sphere(0.26, 6, 3), xf((0, 14.2, 0)), "#ff9a8a", mat="glow_pink")
    return p


def flower_meadow():
    """Alpine pasture patch: mossy hummocks, limestone boulders, split-rail fence and mountain wildflowers."""
    p = c.Piece("flower_meadow")
    for x, z, rx, ry, rz in ((0.0, 0.0, 2.2, 0.45, 1.9), (-1.1, 0.8, 1.3, 0.38, 1.2), (1.2, -0.7, 1.4, 0.40, 1.1)):
        p.add(c.lump(rx, ry, rz, ("mead", x, z), 7, 3, 0.14, flat_bottom=True), xf((x, ry * 0.4, z)), "#3e8c45", grad=0.35)
    p.add(c.lump(0.85, 0.65, 0.75, "mead-r1", 6, 3, 0.2, flat_bottom=True), xf((-0.9, 0.35, -0.6)), "#8d96a8", grad=0.4)
    p.add(c.lump(0.55, 0.45, 0.50, "mead-r2", 6, 3, 0.2, flat_bottom=True), xf((-0.3, 0.25, -0.95)), "#a2abbd", grad=0.35)
    for z in (-1.5, 0.0, 1.5):
        p.add(strut((-1.6, 0.0, z), (-1.6, 1.15, z), 0.09, 5), None, "#6e472b", grad=0.3)
    for y in (0.48, 0.88):
        p.add(box(0.10, 0.10, 3.3), xf((-1.52, y, 0.0)), "#8a5a36", grad=0.25)
    tones = ("#ffd238", "#3b78ff", "#ff66a8", "#fff9ee")
    rng = p.rng
    for i in range(10):
        fx, fz = rng.uniform(-1.1, 1.4), rng.uniform(-1.2, 1.2)
        p.add(sphere(0.14, 5, 3), xf((fx, 0.36 + rng.uniform(0.0, 0.12), fz)), tones[i % 4], grad=0.1, jitter=0.0)
    return p


def hay_barn():
    """Traditional Alpine timber hay barn (Stadel) on stone piers with hayloft, hay bales and milk churns."""
    p = c.Piece("hay_barn")
    for sx in (-2.2, 2.2):
        for sz in (-1.8, 1.8):
            p.add(box(0.65, 0.75, 0.65, 0.04, 1), xf((sx, 0.38, sz)), "#8d929e", grad=0.35)
    p.add(box(5.4, 0.22, 4.4, 0.04, 1), xf((0, 0.82, 0)), "#7a4e2d", grad=0.25)
    p.add(box(5.0, 2.9, 4.0, 0.08, 1), xf((0, 2.35, 0)), "#5c3820", grad=0.45)
    for k in range(4):
        p.add(box(5.12, 0.12, 4.12), xf((0, 1.25 + k * 0.68, 0)), "#472a16", grad=0.2)
    gable = [(2.6, 3.75), (0.0, 5.45), (-2.6, 3.75)]
    p.add(extrude_z(gable, -2.02, 2.02), None, "#4d2e18", grad=0.35)
    roof = [(3.2, 3.55), (0.0, 5.65), (-3.2, 3.55), (-2.9, 3.32), (0.0, 5.25), (2.9, 3.32)]
    p.add(extrude_z(roof, -2.55, 2.55), None, "#4a3b32", grad=0.35)
    p.add(box(0.95, 1.05, 0.12), xf((0.0, 4.35, 2.04)), "#8c5832", grad=0.2)
    p.add(box(0.14, 0.14, 0.95), xf((0.0, 5.05, 2.35)), "#6e472b", grad=0.2)
    p.add(box(1.7, 2.0, 0.12), xf((0.0, 1.92, 2.04)), "#7a4b2a", grad=0.25)
    p.add_flat(sphere(0.16, 6, 3), xf((1.15, 2.75, 2.15)), "#ffe29a", mat="glow_warm")
    for bx, by, bz in ((1.8, 0.42, 2.45), (2.45, 0.42, 2.25), (2.1, 1.15, 2.35)):
        p.add(box(0.75, 0.72, 0.95, 0.08, 1), xf((bx, by, bz)), "#ebb844", grad=0.3)
    for mz in (-1.45, -1.95):
        p.add(strut((-1.6, 0.0, mz), (-1.6, 0.78, mz), 0.22, 7, 0.16), None, "#c9d2e2", grad=0.25)
    return p


def build():
    c.ensure_materials()
    model = Model("scenery-alpine")
    for name, piece in (("pine_a", pine_a()), ("pine_b", pine_b()), ("rock_a", rock_a()), ("rock_b", rock_b()), ("chalet", chalet()), ("cablecar_pylon", cablecar_pylon()), ("flower_meadow", flower_meadow()), ("hay_barn", hay_barn()), ("trail_marker", trail_marker(c.Piece("trail_marker"))),
         ("snow_gauge", snow_gauge(c.Piece("snow_gauge")))):
        piece.bake()
        model.mesh("prop-" + name, piece.items())
    return model
