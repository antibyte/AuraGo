"""Metro scenery, part two: illuminated subway station entrance and neon arcade street kiosk (MIT, AuraGo contributors).

Imported by ``scenery_metro``; fronts point toward +X.
"""

import math

from . import scenery_b_common as c
from .shapes import box, cylinder, ellipsoid, sphere, strut, torus, xf


def metro_entrance():
    """Neon-lit subway station entrance canopy with descending stairs, handrails, glowing M pylon and bollards."""
    p = c.Piece("metro_entrance")
    SLATE, STEEL = "#1e2438", "#c9ced8"
    # Side parapet walls around the stairwell
    for side in (-1, 1):
        p.add(box(4.8, 1.15, 0.26, 0.04, 1), xf((0.0, 0.58, side * 1.65)), "#2d3550", grad=0.35)
        p.add(box(4.9, 0.12, 0.34), xf((0.0, 1.20, side * 1.65)), STEEL, grad=0.15)
        p.add_flat(c.wall_quad((0.0, 0.85, side * 1.80), (0, 0, side), 4.2, 0.10), None, "#5ff4ff", mat="glow_cyan")
    # Rear wall & curved glass-and-steel canopy hood
    p.add(box(0.28, 2.75, 3.56, 0.04, 1), xf((-2.26, 1.38, 0.0)), SLATE, grad=0.35)
    for k in range(4):
        p.add(box(1.15, 0.16, 3.65, 0.03, 1), xf((-1.7 + k * 1.1, 2.85 + 0.14 * (3 - k), 0.0), (0, 0, -6)), "#283354", grad=0.25)
    for sx in (-2.1, -0.4, 1.3):
        for side in (-1, 1):
            p.add(strut((sx, 1.15, side * 1.58), (sx, 2.95, side * 1.58), 0.06, 5), None, STEEL, grad=0.2)
    # Descending steps & warm interior stairwell glow
    for k in range(5):
        p.add(box(0.75, 0.22, 2.95), xf((1.85 - k * 0.72, 0.11 + (4 - k) * 0.16, 0.0)), "#4a5268", grad=0.2)
    p.add_flat(c.wall_quad((-2.08, 1.55, 0.0), (1, 0, 0), 2.7, 1.65), None, "#ffe2a0", mat="glow_warm")
    # Tall illuminated METRO portal sign pylon at the front corner
    p.add(strut((2.15, 0.0, 1.95), (2.15, 4.4, 1.95), 0.11, 6), None, SLATE, grad=0.3)
    p.add(box(0.28, 1.15, 1.15, 0.05, 1), xf((2.15, 4.65, 1.95)), "#141a2e", grad=0.2)
    p.add_flat(c.wall_quad((2.31, 4.65, 1.95), (1, 0, 0), 0.88, 0.88), None, "#ff7ac0", mat="glow_pink")
    p.add_flat(c.wall_quad((-2.01, 4.65, 1.95), (-1, 0, 0), 0.88, 0.88), None, "#5ff4ff", mat="glow_cyan")
    # Illuminated street bollards flanking the entrance
    for side in (-2.25, 2.25):
        p.add(cylinder(0.14, 0.16, 0.88, 7), xf((2.55, 0.44, side)), SLATE, grad=0.3)
        p.add_flat(cylinder(0.145, 0.145, 0.16, 7), xf((2.55, 0.76, side)), "#5ff4ff", mat="glow_cyan")
    return p


def arcade_kiosk():
    """Cyberpunk hexagonal street kiosk with glowing drink/arcade screens, counter stools and a neon roof mascot."""
    p = c.Piece("arcade_kiosk")
    p.add(cylinder(2.45, 2.6, 0.24, 6), xf((0.0, 0.12, 0.0)), "#282e42", grad=0.3)
    p.add(cylinder(1.85, 1.95, 2.85, 6), xf((0.0, 1.65, 0.0)), "#181e32", grad=0.35)
    p.add(cylinder(2.55, 2.25, 0.38, 6), xf((0.0, 3.25, 0.0)), "#252f54", grad=0.3)
    p.add_flat(torus(2.42, 0.075, 12, 4), xf((0.0, 3.25, 0.0), (90, 0, 0)), "#ff7ac0", mat="glow_pink")
    p.add_flat(torus(2.15, 0.065, 12, 4), xf((0.0, 3.52, 0.0), (90, 0, 0)), "#5ff4ff", mat="glow_cyan")
    # Glowing arcade & vending screens around the hexagon faces + front counter & stools
    mats = ("glow_warm", "glow_cyan", "glow_pink", "glow_cyan", "glow_pink", "glow_warm")
    cols = ("#ffe2a0", "#7af4ff", "#ff8ad4", "#7af4ff", "#ff8ad4", "#ffe2a0")
    for k in range(6):
        a = math.tau * k / 6.0
        nx, nz = math.cos(a), math.sin(a)
        p.add_flat(c.wall_quad((nx * 1.66, 1.85, nz * 1.66), (nx, 0, nz), 1.15, 1.25), None, cols[k], mat=mats[k])
    p.add(box(0.55, 0.12, 2.2, 0.03, 1), xf((1.88, 1.12, 0.0)), "#ff5a7a", grad=0.2)
    for sz in (-0.75, 0.0, 0.75):
        p.add(strut((2.28, 0.24, sz), (2.28, 0.78, sz), 0.05, 5), None, "#c9ced8", grad=0.2)
        p.add(cylinder(0.24, 0.24, 0.09, 7), xf((2.28, 0.82, sz)), "#35d4ff", grad=0.1, jitter=0.0)
    # Giant neon crown / retro star ornament on the roof
    p.add(cylinder(1.1, 1.35, 0.45, 6), xf((0.0, 3.65, 0.0)), "#181e32", grad=0.25)
    p.add_flat(torus(1.08, 0.11, 12, 5), xf((0.0, 5.05, 0.0), (0, 0, 90)), "#ff7ac0", mat="glow_pink")
    p.add_flat(ellipsoid(0.32, 0.72, 0.72, 8, 4), xf((0.0, 5.05, 0.0)), "#7af4ff", mat="glow_cyan")
    p.add_flat(sphere(0.24, 6, 3), xf((0.0, 6.32, 0.0)), "#ffe2a0", mat="glow_warm")
    return p
