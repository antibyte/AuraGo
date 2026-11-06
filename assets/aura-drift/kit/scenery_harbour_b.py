"""Harbour scenery, part two: the fishing boat on its cradle and the neon sign with a seagull (MIT, AuraGo contributors).

Imported by ``scenery_harbour``; bow and sign face point toward +X.
"""

import math

from . import scenery_a_common as c
from .shapes import box, ellipsoid, loft, prism, revolve, sphere, spline, strut, torus, xf


def fishing_boat():
    """Blue-red cutter on wooden blocks: loft hull, cabin, mast, nets, buoys and a life ring."""
    p = c.Piece("fishing_boat")
    xs = [-4.6, -4.2, -3.2, -1.6, 0.0, 1.6, 3.2, 4.3, 5.0]
    width = spline([(-4.6, 0.95), (-4.2, 1.5), (-3.0, 1.95), (-1.0, 2.1), (1.5, 2.0), (3.2, 1.55), (4.3, 0.85), (5.0, 0.12)])
    depth = spline([(-4.6, 0.9), (-3.0, 1.45), (-1.0, 1.65), (1.5, 1.5), (3.2, 1.3), (5.0, 0.85)])
    top = lambda x: 2.05 + 0.5 * max(0.0, x / 5.0) ** 2 + 0.12 * max(0.0, -x / 4.6) ** 2
    rings = []
    for x in xs:
        w, d, t = width(x), depth(x), top(x)
        ring = [(x, t - d * math.sin(math.pi * k / 8), w * math.cos(math.pi * k / 8)) for k in range(9)]
        rings.append(ring)
    hull = loft(rings)

    def paint(x, y, z):
        if y < 1.2:
            return "#c9362e"
        if y < 2.0:
            return "#2f72d6"
        return "#f6efdc"

    p.add(hull, None, paint, grad=0.3)
    outline = [(x, top(x) + 0.12, width(x) * 0.93) for x in xs]
    for side in (1, -1):
        for a, b in zip(outline, outline[1:]):
            p.add(strut((a[0], a[1], side * a[2]), (b[0], b[1], side * b[2]), 0.1, 4), None, "#fff6e0", grad=0.1)
    deck = [(x, width(x) * 0.9) for x in xs] + [(x, -width(x) * 0.9) for x in reversed(xs)]
    p.add(prism(deck, 2.09, 2.21), None, "#c9915a", grad=0.1)
    # wooden cradle
    for x in (-2.4, 2.6):
        p.add(box(1.0, 0.8, 3.6, 0.04, 1), xf((x, 0.4, 0)), "#8a5a3a", grad=0.4)
    p.add(box(9.0, 0.2, 0.4), xf((0.2, 0.7, 0)), "#6a4328", grad=0.2)
    # wheelhouse
    p.add(box(2.8, 2.0, 2.2, 0.08, 1), xf((-1.8, 3.2, 0)), "#f6efdc", grad=0.35)
    p.add(box(3.3, 0.22, 2.7, 0.05, 1), xf((-1.8, 4.3, 0)), "#d8402e", grad=0.3)
    for z in (1.12, -1.12):
        p.add_flat(box(1.8, 0.8, 0.05), xf((-1.8, 3.4, z)), "#ffe2a0", mat="glow_warm")
    p.add_flat(box(0.05, 0.8, 1.6), xf((-0.38, 3.4, 0)), "#ffe2a0", mat="glow_warm")
    p.add(c.lump(0.5, 0.5, 0.5, "stack", 6, 3, 0.05), xf((-2.7, 4.8, 0.5)), "#3a3f56", grad=0.2)
    # mast, boom, lantern, flag
    p.add(strut((1.8, 2.1, 0), (1.8, 8.4, 0), 0.13, 6, 0.09), None, "#6a4328", grad=0.3)
    p.add(strut((1.8, 5.2, 0), (-1.2, 6.0, 0), 0.07, 4), None, "#6a4328", grad=0.2)
    p.add(strut((1.8, 8.3, 0), (4.6, 2.2, 1.6), 0.03, 4), None, "#d8d0c0", grad=0.0)
    p.add(strut((1.8, 8.3, 0), (4.6, 2.2, -1.6), 0.03, 4), None, "#d8d0c0", grad=0.0)
    p.add_flat(sphere(0.28, 6, 3), xf((1.8, 8.6, 0)), "#ffe2a0", mat="glow_warm")
    p.add(box(0.05, 0.7, 1.1), xf((1.8, 7.7, -0.58)), "#e0402e", grad=0.0)
    # nets, buoys, life ring, crates
    for i, (x, z, r) in enumerate(((3.0, 0.7, 0.8), (3.5, -0.5, 0.6), (-4.0, -0.4, 0.5))):
        p.add(c.lump(r, r * 0.7, r, ("net", i), 7, 3, 0.2, flat_bottom=True), xf((x, 2.25 + r * 0.4, z)), "#ff8a2a", grad=0.3)
    for i, (x, z, col) in enumerate(((0.3, 1.5, "#e0402e"), (0.9, 1.6, "#f6efdc"), (-0.3, -1.4, "#e0402e"))):
        p.add(sphere(0.32, 7, 4), xf((x, 2.5, z)), col, grad=0.2)
    p.add(torus(0.42, 0.13, 10, 5), xf((-3.35, 3.5, 0.0), (0, 0, 90)), "#ff8a2a", grad=0.0, jitter=0.0)
    p.add(box(0.9, 0.6, 0.7, 0.04, 1), xf((0.2, 2.5, -1.0)), "#b97d4a", grad=0.3)
    return p


def neon_sign():
    """Pole sign with a navy board, a pink anchor, a cyan fish, a wave line and a seagull on top."""
    p = c.Piece("neon_sign")
    for z in (-2.2, 2.2):
        p.add(strut((0, 0, z), (0, 5.4, z), 0.2, 6), None, "#4a526c", grad=0.4)
        p.add(box(0.9, 0.3, 0.9, 0.03, 1), xf((0, 0.15, z)), "#8d90a6", grad=0.3)
    p.add(box(0.5, 3.8, 6.6, 0.08, 1), xf((0, 6.8, 0)), "#1b2140", grad=0.15)
    p.add(box(0.62, 0.22, 6.9, 0.04, 1), xf((0, 8.8, 0)), "#e8e6f4", grad=0.1)
    p.add(box(0.62, 0.22, 6.9, 0.04, 1), xf((0, 4.8, 0)), "#e8e6f4", grad=0.1)
    X = 0.32
    pink, cyan, warm = "#ff7ac0", "#7af4ff", "#ffe2a0"
    # anchor on the left (z < 0)
    az = -2.1
    p.add_flat(torus(0.38, 0.07, 14, 5), xf((X, 8.0, az), (0, 0, 90)), pink, mat="glow_pink")
    for shape in c.tube_path([(7.6, az), (5.7, az)], 0.07, 5, X) + c.tube_path([(7.2, az - 0.6), (7.2, az + 0.6)], 0.07, 5, X):
        p.add_flat(shape, None, pink, mat="glow_pink")
    pts = [(az + 1.1 * math.cos(math.radians(a)), 6.3 + 0.8 * math.sin(math.radians(a))) for a in range(180, 361, 30)]
    for shape in c.tube_path([(y, z) for z, y in pts], 0.07, 5, X):
        p.add_flat(shape, None, pink, mat="glow_pink")
    # fish on the right (z > 0): body outline, tail, eye
    fz = 1.5
    body = [(6.8 + 0.95 * math.sin(math.radians(a)), fz + 1.4 * math.cos(math.radians(a))) for a in range(0, 361, 36)]
    for shape in c.tube_path(body, 0.07, 5, X):
        p.add_flat(shape, None, cyan, mat="glow_cyan")
    for shape in c.tube_path([(6.8, fz - 1.4), (7.9, fz - 2.1), (5.7, fz - 2.1), (6.8, fz - 1.4)], 0.07, 5, X):
        p.add_flat(shape, None, cyan, mat="glow_cyan")
    p.add_flat(sphere(0.12, 5, 3), xf((X, 7.1, fz + 0.8)), cyan, mat="glow_cyan")
    for i in range(3):
        p.add_flat(torus(0.11, 0.035, 8, 4), xf((X, 7.7 + 0.35 * i, fz + 1.5 + 0.12 * i), (0, 0, 90)), cyan, mat="glow_cyan")
    wave = [(4.95 + 0.14 * math.sin(i * 1.4), -3.0 + 0.5 * i) for i in range(13)]
    for shape in c.tube_path(wave, 0.05, 4, X):
        p.add_flat(shape, None, warm, mat="glow_warm")
    # seagull on the board
    p.add(ellipsoid(0.55, 0.38, 0.32, 8, 4), xf((0.0, 9.15, 2.0)), "#f6f6fa", grad=0.1)
    p.add(sphere(0.2, 6, 3), xf((0.4, 9.5, 2.0)), "#f6f6fa", grad=0.0, jitter=0.0)
    p.add(box(0.3, 0.06, 0.1), xf((0.66, 9.46, 2.0)), "#ff9a2a", grad=0.0, jitter=0.0)
    p.add(box(0.6, 0.08, 0.2), xf((-0.2, 9.2, 2.3), (0, 0, 10)), "#a9b0c4", grad=0.0, jitter=0.0)
    p.add(box(0.6, 0.08, 0.2), xf((-0.2, 9.2, 1.7), (0, 0, 10)), "#a9b0c4", grad=0.0, jitter=0.0)
    return p


def tugboat():
    """Chunky red-and-black harbour tugboat with tyre fenders, two-tier wheelhouse, twin diesel stacks and radar mast."""
    p = c.Piece("tugboat")
    xs = [-4.2, -3.6, -2.0, 0.0, 2.0, 3.5, 4.4]
    width = spline([(-4.2, 1.4), (-3.6, 2.1), (-1.5, 2.4), (1.5, 2.35), (3.5, 1.8), (4.4, 0.65)])
    rings = [[(x, 2.1 - 1.55 * math.sin(math.pi * k / 8), width(x) * math.cos(math.pi * k / 8)) for k in range(9)] for x in xs]
    p.add(loft(rings), None, lambda x, y, z: "#1e222e" if y < 1.35 else "#d6382c", grad=0.35)
    deck = [(x, width(x) * 0.92) for x in xs] + [(x, -width(x) * 0.92) for x in reversed(xs)]
    p.add(prism(deck, 2.08, 2.22), None, "#7a5234", grad=0.15)
    # Rubber tyre fenders hung along both gunwales + heavy bow pushing fender
    for side in (-1, 1):
        for fx in (-2.4, -0.8, 0.8, 2.4):
            p.add(torus(0.36, 0.13, 8, 4), xf((fx, 1.75, side * (width(fx) + 0.06))), "#222630", grad=0.1)
    p.add(box(0.55, 0.75, 1.5, 0.08, 1), xf((4.35, 1.85, 0.0)), "#222630", grad=0.2)
    # Two-tier deckhouse & wheelhouse with warm bridge windows
    p.add(box(3.8, 1.35, 3.0, 0.08, 1), xf((-0.4, 2.85, 0.0)), "#f5efe0", grad=0.3)
    p.add(box(2.2, 1.35, 2.3, 0.08, 1), xf((0.2, 4.15, 0.0)), "#ff8a24", grad=0.3)
    p.add(box(2.5, 0.18, 2.6, 0.04, 1), xf((0.2, 4.90, 0.0)), "#f5efe0", grad=0.2)
    p.add_flat(box(0.08, 0.62, 1.7), xf((1.32, 4.25, 0.0)), "#ffe2a0", mat="glow_warm")
    for side in (-1, 1):
        p.add_flat(box(1.4, 0.62, 0.08), xf((0.2, 4.25, side * 1.16)), "#ffe2a0", mat="glow_warm")
    # Twin yellow diesel smokestacks & radar mast
    for side in (-0.65, 0.65):
        p.add(strut((-1.4, 3.5, side), (-1.6, 5.4, side), 0.28, 7, 0.24), None, "#ffd238", grad=0.3)
        p.add(strut((-1.58, 5.15, side), (-1.62, 5.5, side), 0.26, 7), None, "#222630", grad=0.1)
    p.add(strut((0.2, 4.95, 0.0), (0.2, 6.65, 0.0), 0.08, 5), None, "#d8dce8", grad=0.2)
    p.add(box(0.18, 0.12, 1.25), xf((0.2, 6.25, 0.0)), "#f5efe0", grad=0.1)
    p.add_flat(sphere(0.20, 6, 3), xf((0.2, 6.80, 0.0)), "#7af4ff", mat="glow_cyan")
    return p


def buoy_cluster():
    """Nautical sea-mark & dock mooring cluster: lattice bell buoy with beacon, timber pilings and lobster crates."""
    p = c.Piece("buoy_cluster")
    p.add(torus(1.05, 0.38, 8, 4), xf((0.0, 0.35, 0.0), (90, 0, 0)), "#e03c2e", grad=0.25)
    p.add(revolve([(0.0, 0.3), (0.85, 0.3), (0.68, 1.25), (0.0, 1.25)], 8), None, "#f5efe0", grad=0.3)
    for k in range(4):
        a = math.tau * k / 4.0 + 0.4
        p.add(strut((0.68 * math.cos(a), 1.2, 0.68 * math.sin(a)), (0.22 * math.cos(a), 4.1, 0.22 * math.sin(a)), 0.07, 4), None, "#e03c2e", grad=0.3)
    p.add(revolve([(0.0, 2.4), (0.36, 2.4), (0.24, 2.95), (0.0, 2.95)], 7), None, "#ffd238", grad=0.2)
    p.add(box(0.65, 0.16, 0.65, 0.03, 1), xf((0.0, 4.18, 0.0)), "#252f5c", grad=0.2)
    p.add_flat(sphere(0.26, 6, 3), xf((0.0, 4.52, 0.0)), "#ffe2a0", mat="glow_warm")
    # Timber mooring pilings & lobster crates beside the buoy
    for px, pz, h in ((-1.55, 1.15, 2.4), (-1.85, 0.55, 1.95)):
        p.add(strut((px, 0.0, pz), (px, h, pz), 0.22, 6), None, "#6a4328", grad=0.35)
        p.add(torus(0.24, 0.06, 7, 4), xf((px, h * 0.72, pz), (90, 0, 0)), "#d8c398", grad=0.1)
    p.add(box(0.85, 0.58, 0.65, 0.04, 1), xf((1.35, 0.29, -0.95)), "#b97d4a", grad=0.3)
    return p
