"""Metro scenery: three night skyscrapers, a roadside poster stand, a street lamp, a food truck and a taxi (MIT, AuraGo contributors).

Deep indigo towers with grids of emissive windows (warm, cyan, pink) and neon strips, so bloom makes the
city sing. The poster stand is node ``prop-billboard`` with a UV-mapped child ``face`` (the runtime
paints a poster on it). Comic touches: a hot-dog sign on the food truck and a pigeon on the lamp.
Props stand at y=0, front toward +X.
"""

import math

from . import scenery_b_common as c
from .core import Model
from .scenery_metro_b import arcade_kiosk, metro_entrance
from .shapes import box, cylinder, cylinder_x, cylinder_z, ellipsoid, revolve, revolve_x, sphere, strut, xf

NAVY = "#252b55"
NAVY_LIGHT = "#3a4380"
DARK_WIN = "#141830"
WARM, CYAN, PINK = "glow_warm", "glow_cyan", "glow_pink"
WIN_COLOURS = {WARM: "#ffe2a0", CYAN: "#bff8ff", PINK: "#ffb0dc"}


def pick_window(p):
    r = p.rng.random()
    if r < 0.58:
        return None
    if r < 0.86:
        return WARM
    return CYAN if r < 0.94 else PINK


def facade(p, centre, normal, width, height, cols, rows, margin=0.12, lit_bias=1.0):
    """Grid of single-sided window quads on one wall: lit ones emissive, unlit ones dark glass."""
    n = c.Vector(normal).normalized()
    u = c.Vector((0.0, 1.0, 0.0)).cross(n).normalized()
    cw, ch = width / cols, height / rows
    for i in range(cols):
        for j in range(rows):
            pos = c.Vector(centre) + u * ((i + 0.5) * cw - width / 2.0) + c.Vector((0, (j + 0.5) * ch - height / 2.0, 0)) + n * 0.05
            kind = pick_window(p)
            size = (cw * (1.0 - margin * 2.4), ch * (1.0 - margin * 3.0))
            if kind is None:
                p.add_flat(c.wall_quad(tuple(pos), tuple(n), size[0], size[1]), None, DARK_WIN)
            else:
                p.add_flat(c.wall_quad(tuple(pos), tuple(n), size[0], size[1]), None, WIN_COLOURS[kind], mat=kind)


def facade_box(p, w, d, y0, y1, cols, rows, cols_d=None):
    """Windows on all four sides of a block of x-size ``w`` and z-size ``d``."""
    cols_d = cols_d or cols
    mid = (y0 + y1) / 2.0
    for sign in (1, -1):
        facade(p, (sign * w / 2.0, mid, 0.0), (sign, 0, 0), d * 0.88, (y1 - y0) * 0.94, cols_d, rows)
        facade(p, (0.0, mid, sign * d / 2.0), (0, 0, sign), w * 0.88, (y1 - y0) * 0.94, cols, rows)


def neon_ring(p, w, d, y, mat, col, thick=0.22):
    for sign in (1, -1):
        p.add_flat(box(w + 0.3, thick, thick), xf((0, y, sign * (d / 2.0 + 0.1))), col, mat=mat)
        p.add_flat(box(thick, thick, d + 0.3), xf((sign * (w / 2.0 + 0.1), y, 0)), col, mat=mat)


def skyscraper_a():
    """Art-deco stepped tower: podium, shaft, two setbacks, a pink neon crown and a spire."""
    p = c.Piece("skyscraper_a")
    p.add(box(15.0, 6.0, 15.0, 0.2, 1), xf((0, 3.0, 0)), "#323a6e", grad=0.45)
    p.add(box(10.4, 28.0, 10.4), xf((0, 20.0, 0)), NAVY, grad=0.5)
    p.add(box(7.6, 12.0, 7.6), xf((0, 40.0, 0)), "#2d3466", grad=0.4)
    p.add(box(4.6, 6.0, 4.6, 0.12, 1), xf((0, 49.0, 0)), NAVY_LIGHT, grad=0.3)
    p.add(revolve([(0.0, 52.0), (1.0, 52.0), (0.12, 60.0), (0.0, 61.0)], 6), None, "#c9ced8", grad=0.3)
    for y, w in ((6.0, 15.0), (34.0, 10.4), (46.0, 7.6)):
        p.add(box(w + 0.5, 0.5, w + 0.5), xf((0, y, 0)), "#4a54a0", grad=0.1, jitter=0.0)
    facade_box(p, 10.4, 10.4, 7.5, 33.0, 5, 11)
    facade_box(p, 7.6, 7.6, 34.5, 45.5, 4, 5)
    facade_box(p, 15.0, 15.0, 0.8, 5.2, 6, 2)
    neon_ring(p, 10.4, 10.4, 33.6, "glow_pink", "#ff9ad0")
    neon_ring(p, 4.6, 4.6, 51.9, "glow_cyan", "#bff8ff", 0.2)
    for sx in (1, -1):
        for sz in (1, -1):
            p.add_flat(box(0.22, 27.0, 0.22), xf((sx * 5.3, 20.0, sz * 5.3)), "#bff8ff", mat="glow_cyan")
    p.add_flat(sphere(0.35, 5, 3), xf((0, 60.9, 0)), "#ff7a6a", mat="glow_red")
    return p


def skyscraper_b():
    """Glassy slab tower with a lit neon spine on the front, a water tank and a helipad ring on the roof."""
    p = c.Piece("skyscraper_b")
    p.add(box(9.0, 4.0, 20.0, 0.15, 1), xf((0, 2.0, 0)), "#3a4380", grad=0.4)
    p.add(box(7.6, 46.0, 17.0), xf((0, 27.0, 0)), "#1e3a63", grad=0.5)
    p.add(box(8.0, 1.0, 17.4), xf((0, 50.5, 0)), "#4a6aa0", grad=0.2)
    facade(p, (3.85, 27.0, 0.0), (1, 0, 0), 15.5, 44.0, 9, 17)
    facade(p, (-3.85, 27.0, 0.0), (-1, 0, 0), 15.5, 44.0, 9, 17)
    for sign in (1, -1):
        facade(p, (0.0, 27.0, sign * 8.55), (0, 0, sign), 6.6, 44.0, 3, 17)
    p.add_flat(box(0.25, 44.0, 0.4), xf((3.95, 27.0, 0.0)), "#ff9ad0", mat="glow_pink")
    for z in (-6.0, 6.0):
        p.add_flat(box(0.2, 44.0, 0.2), xf((3.95, 27.0, z)), "#bff8ff", mat="glow_cyan")
    for y in (14.0, 30.0):
        p.add_flat(box(0.3, 0.3, 17.2), xf((3.95, y, 0.0)), "#bff8ff", mat="glow_cyan")
    # roof: water tank on legs, helipad ring
    for sx in (1, -1):
        for sz in (1, -1):
            p.add(strut((sx * 1.2 - 1.8, 51.0, sz * 1.2), (sx * 1.0 - 1.8, 53.2, sz * 1.0), 0.1, 4), None, "#5a3622", grad=0.2)
    p.add(cylinder(1.8, 1.8, 2.4, 8), xf((-1.8, 54.4, 0)), "#a8703c", grad=0.3)
    p.add(revolve([(0.0, 55.6), (1.95, 55.6), (0.0, 57.2)], 8), None, "#5a3622", grad=0.3)
    p.add(cylinder(2.4, 2.4, 0.12, 12), xf((2.0, 51.1, 3.8)), "#3a4156", grad=0.0)
    p.add_flat(c.disc((2.0, 51.2, 3.8), (0, 1, 0), 2.0, 12, up=(1, 0, 0)), None, "#7dff6a", mat="glow_green")
    p.add(c.disc((2.0, 51.25, 3.8), (0, 1, 0), 1.6, 12, up=(1, 0, 0)), None, "#3a4156", grad=0.0, jitter=0.0)
    p.add_flat(sphere(0.3, 5, 3), xf((3.6, 51.5, -6.5)), "#ff7a6a", mat="glow_red")
    return p


def skyscraper_c():
    """Octagonal tower with a glowing revolving restaurant disc, a crown ring and a lightning-rod spire."""
    p = c.Piece("skyscraper_c")
    R = 5.6
    p.add(cylinder(7.2, 7.2, 5.0, 8), xf((0, 2.5, 0), (0, 22.5, 0)), "#323a6e", grad=0.45)
    p.add(cylinder(R, R, 31.0, 8), xf((0, 20.5, 0), (0, 22.5, 0)), "#2a3566", grad=0.5)
    p.add(cylinder(8.0, 8.0, 1.7, 8), xf((0, 38.0, 0), (0, 22.5, 0)), "#4a54a0", grad=0.15)
    p.add_flat(cylinder(8.1, 8.1, 0.7, 8), xf((0, 38.0, 0), (0, 22.5, 0)), "#ffe2a0", mat="glow_warm")
    p.add(revolve([(0.0, 39.0), (5.6, 39.0), (4.0, 44.0), (1.2, 48.0), (0.0, 48.5)], 8), xf((0, 0, 0), (0, 22.5, 0)), "#3a4380", grad=0.3)
    p.add(strut((0.0, 48.0, 0.0), (0.0, 58.0, 0.0), 0.35, 5, 0.1), None, "#c9ced8", grad=0.3)
    p.add_flat(sphere(0.34, 5, 3), xf((0, 58.2, 0)), "#ff7a6a", mat="glow_red")
    for k in range(8):
        a = math.radians(45 * k)
        n = (math.cos(a), 0.0, math.sin(a))
        radius = R * math.cos(math.radians(22.5)) + 0.02
        facade(p, (n[0] * radius, 20.5, n[2] * radius), n, 3.7, 28.0, 3, 12, margin=0.14)
    for k in range(8):
        a = math.radians(45 * k + 22.5)
        p.add_flat(strut((math.cos(a) * 5.75, 5.2, math.sin(a) * 5.75), (math.cos(a) * 5.75, 35.8, math.sin(a) * 5.75), 0.1, 3), None, "#41f0ff" if k % 2 else "#ff9ad0", mat="glow_cyan" if k % 2 else "glow_pink")
    for y, r in ((41.0, 5.2), (44.5, 3.4)):
        p.add_flat(cylinder(r, r, 0.3, 8), xf((0, y, 0), (0, 22.5, 0)), "#ff9ad0", mat="glow_pink")
    return p


def add_face_uv(obj):
    """UV 0..1 on a card facing +X: u grows to the viewer's right (-Z), v upward."""
    mesh = obj.data
    ys = [v.co.y for v in mesh.vertices]
    zs = [v.co.z for v in mesh.vertices]
    y_lo, y_hi, z_lo, z_hi = min(ys), max(ys), min(zs), max(zs)
    layer = mesh.uv_layers.new(name="UVMap")
    for loop in mesh.loops:
        co = mesh.vertices[loop.vertex_index].co
        layer.data[loop.index].uv = ((co.y - y_lo) / max(y_hi - y_lo, 1e-9), (co.z - z_lo) / max(z_hi - z_lo, 1e-9))


def poster_frame():
    """Roadside poster stand: steel posts, frame, catwalk with a rail, floodlights and back bracing."""
    p = c.Piece("billboard")
    W, H, Y = 16.0, 6.4, 12.0
    for z in (-5.0, 5.0):
        p.add(box(0.9, 9.0, 0.9, 0.06, 1), xf((-0.2, 4.5, z)), "#5b6590", grad=0.45)
        p.add(box(1.8, 0.5, 1.8, 0.05, 1), xf((-0.2, 0.25, z)), "#3a4156", grad=0.3)
    p.add(box(0.8, 0.8, 11.5, 0.05, 1), xf((-0.2, 9.4, 0)), "#5b6590", grad=0.3)
    p.add(box(0.8, H + 0.8, W + 0.8, 0.06, 1), xf((-0.15, Y, 0)), "#3a4156", grad=0.2)
    p.add(box(0.4, 0.3, W + 0.4), xf((0.2, Y + H / 2.0 + 0.45, 0)), "#4a54a0", grad=0.1, jitter=0.0)
    p.add(box(0.4, 0.3, W + 0.4), xf((0.2, Y - H / 2.0 - 0.45, 0)), "#4a54a0", grad=0.1, jitter=0.0)
    for z in (-W / 2.0 - 0.3, W / 2.0 + 0.3):
        p.add(box(0.4, H + 0.8, 0.3), xf((0.2, Y, z)), "#4a54a0", grad=0.1, jitter=0.0)
    p.add(box(1.0, 0.16, W + 0.6), xf((0.9, Y - H / 2.0 - 0.9, 0)), "#8d90a6", grad=0.2)
    for z in (-W / 2.0 - 0.2, 0.0, W / 2.0 + 0.2):
        p.add(strut((1.35, Y - H / 2.0 - 0.9, z), (1.35, Y - H / 2.0 + 0.1, z), 0.05, 3), None, "#8d90a6", grad=0.0)
    p.add(strut((1.35, Y - H / 2.0 + 0.1, -W / 2.0 - 0.2), (1.35, Y - H / 2.0 + 0.1, W / 2.0 + 0.2), 0.05, 3), None, "#8d90a6", grad=0.0)
    for z in (-5.5, -1.8, 1.8, 5.5):
        p.add(strut((0.2, Y + H / 2.0 + 0.5, z), (1.8, Y + H / 2.0 + 1.6, z), 0.07, 4), None, "#8d90a6", grad=0.0)
        p.add(box(0.7, 0.3, 0.7), xf((1.9, Y + H / 2.0 + 1.65, z), (0, 0, 25)), "#2b2f38", grad=0.0)
        p.add_flat(c.wall_quad((2.04, Y + H / 2.0 + 1.5, z), (0.6, -0.8, 0), 0.5, 0.5), None, "#fff2cc", mat="glow_warm")
    for z in (-W / 2.0, W / 2.0):
        p.add_flat(box(0.12, H + 1.2, 0.12), xf((0.5, Y, z * 1.06)), "#ff9ad0", mat="glow_pink")
    for z in (-5.0, 5.0):
        p.add(strut((-0.6, 0.2, z), (-2.6, 0.0, z * 1.4), 0.2, 4), None, "#3a4156", grad=0.2)
        p.add(strut((-0.6, 8.0, z), (-3.4, 0.0, z * 1.6), 0.2, 4), None, "#3a4156", grad=0.2)
    p.add_flat(sphere(0.25, 5, 3), xf((-0.2, Y + H / 2.0 + 1.0, W / 2.0 + 0.7)), "#ff7a6a", mat="glow_red")
    return p, (W, H, Y)


def poster_face(w, h):
    """The card the runtime paints a poster on: faces +X, pastel gradient until it is textured."""
    acc = c.Accum()
    shape = ([(0.0, -h / 2.0, w / 2.0), (0.0, -h / 2.0, -w / 2.0), (0.0, h / 2.0, -w / 2.0), (0.0, h / 2.0, w / 2.0)], [(0, 1, 2, 3)])
    colours = [c.colour("#ff9ad0"), c.colour("#9ae8ff"), c.colour("#fff2b0"), c.colour("#ffc0e8")]
    acc.add(shape, None, colours)
    return [("poster", acc)]


def street_lamp():
    """Scatter: tapered pole, curved double arm, big warm lamp, a small cyan pedestrian lamp and a pigeon."""
    p = c.Piece("street_lamp")
    p.add(box(0.9, 0.35, 0.9), xf((0, 0.17, 0)), "#3a4156", grad=0.3)
    p.add(cylinder(0.34, 0.3, 0.9, 6), xf((0, 0.8, 0)), "#5b6590", grad=0.3)
    p.add(strut((0.0, 1.2, 0.0), (0.0, 7.4, 0.0), 0.2, 6, 0.13), None, "#7280b8", grad=0.4)
    p.add(strut((0.0, 7.2, 0.0), (0.7, 8.3, 0.0), 0.12, 4), None, "#7280b8", grad=0.2)
    p.add(strut((0.7, 8.3, 0.0), (2.3, 8.45, 0.0), 0.11, 4), None, "#7280b8", grad=0.2)
    p.add(box(1.5, 0.3, 0.8, 0.05, 1), xf((2.4, 8.5, 0)), "#2b2f38", grad=0.0)
    p.add_flat(c.wall_quad((2.4, 8.34, 0.0), (0, -1, 0), 1.3, 0.62, up=(1, 0, 0)), None, "#ffe2a0", mat="glow_warm")
    p.add(box(1.2, 0.08, 0.7), xf((2.4, 8.7, 0)), "#4a54a0", grad=0.0)
    p.add(strut((0.0, 4.0, 0.0), (-0.7, 4.8, 0.0), 0.08, 4), None, "#7280b8", grad=0.2)
    p.add(box(0.5, 0.35, 0.5, 0.03, 1), xf((-0.9, 4.85, 0)), "#2b2f38", grad=0.0)
    p.add_flat(c.wall_quad((-0.9, 4.66, 0.0), (0, -1, 0), 0.4, 0.4), None, "#bff8ff", mat="glow_cyan")
    # pigeon on the lamp head
    p.add(ellipsoid(0.26, 0.18, 0.16, 6, 3), xf((2.05, 8.98, 0.0)), "#9aa3b4", grad=0.3, jitter=0.0)
    p.add(sphere(0.1, 5, 3), xf((2.3, 9.1, 0.0)), "#8d90a6", grad=0.2, jitter=0.0)
    p.add(box(0.1, 0.04, 0.05), xf((2.42, 9.08, 0.0)), "#ff9a2a", grad=0.0, jitter=0.0)
    p.add(box(0.28, 0.05, 0.18), xf((1.8, 9.05, 0.0)), "#7a8396", grad=0.0, jitter=0.0)
    return p


def food_truck():
    """Teal truck with a lit serving hatch, striped awning, string lights and a giant hot-dog sign on the roof."""
    p = c.Piece("food_truck")
    TEAL, PINK_BODY = "#2fc4b2", "#e8458f"
    p.add(box(5.9, 0.5, 2.2, 0.05, 1), xf((0.0, 0.85, 0)), "#2b2f38", grad=0.3)
    p.add(box(3.9, 2.2, 2.3, 0.1, 1), xf((-0.9, 2.1, 0)), TEAL, grad=0.35)
    p.add(box(1.7, 1.2, 2.2, 0.1, 1), xf((2.35, 1.6, 0)), PINK_BODY, grad=0.3)
    p.add(box(0.9, 0.9, 2.1, 0.06, 1), xf((2.95, 1.35, 0)), PINK_BODY, grad=0.3)
    p.add_flat(c.wall_quad((2.78, 2.0, 0.0), (1, 0.28, 0), 1.7, 0.8), None, "#1a2c46")
    for z in (-1, 1):
        p.add_flat(c.wall_quad((2.35, 2.0, z * 1.11), (0, 0, z), 1.0, 0.65), None, "#1a2c46")
        p.add(box(1.0, 0.06, 0.06), xf((2.35, 2.38, z * 1.12)), "#e8f0fa", grad=0.0)
    p.add(box(4.0, 0.1, 2.4), xf((-0.9, 3.25, 0)), "#e8f0fa", grad=0.2)
    for z in (-1, 1):
        p.add(box(2.3, 1.05, 0.12), xf((-1.0, 2.15, z * 1.17)), "#10162e", grad=0.0)
        p.add_flat(c.wall_quad((-1.0, 2.15, z * 1.23), (0, 0, z), 2.1, 0.88), None, "#ffe2a0", mat="glow_warm")
        p.add_flat(c.wall_quad((-1.0, 1.5, z * 1.17), (0, 0, z), 3.4, 0.08), None, "#ff9ad0", mat="glow_pink")
        p.add(box(1.0, 0.45, 0.06), xf((-2.55, 2.5, z * 1.17)), "#10162e", grad=0.0)
        p.add_flat(c.wall_quad((-2.55, 2.5, z * 1.2), (0, 0, z), 0.8, 0.3), None, "#ffe2a0", mat="glow_warm")
    # awning on +Z
    for k in range(7):
        x = -2.15 + k * 0.5
        p.add(box(0.5, 0.1, 1.15), xf((x, 3.0, 1.75), (-18, 0, 0)), "#ff5a7a" if k % 2 else "#fff2e0", grad=0.1, jitter=0.0)
    for k in range(8):
        p.add_flat(sphere(0.07, 4, 2), xf((-2.4 + k * 0.5, 2.55, 2.28)), "#ffe2a0", mat="glow_warm")
    for sx in (-1.55, -0.45):
        p.add(strut((sx, 0.0, 1.95), (sx, 0.68, 1.95), 0.05, 5), None, "#c9ced8", grad=0.2)
        p.add(cylinder(0.24, 0.24, 0.09, 8), xf((sx, 0.72, 1.95)), "#ff5a7a", grad=0.1, jitter=0.0)
    # roof: hot-dog sign
    for x in (-2.1, 0.3):
        p.add(strut((x, 3.3, 0.0), (x, 3.9, 0.0), 0.07, 4), None, "#8d90a6", grad=0.0)
    p.add(ellipsoid(1.9, 0.38, 0.68, 8, 3), xf((-0.9, 4.25, 0.0)), "#e8b46a", grad=0.3, jitter=0.0)
    p.add(revolve_x([(0.0, -1.95), (0.26, -1.75), (0.3, 1.6), (0.0, 1.95)], 6), xf((-0.9, 4.58, 0.0)), "#c2432e", grad=0.3, jitter=0.0)
    for k in range(5):
        x = -2.0 + k * 0.5
        p.add_flat(strut((x, 4.9, -0.2 if k % 2 else 0.2), (x + 0.5, 4.9, 0.2 if k % 2 else -0.2), 0.05, 3), None, "#ffd23c", mat="glow_amber")
    p.add(ellipsoid(1.9, 0.3, 0.68, 8, 3), xf((-0.9, 4.0, 0.0)), "#d99b52", grad=0.3, jitter=0.0)
    # chimney vent and steam
    p.add(cylinder(0.2, 0.2, 0.7, 6), xf((-2.7, 3.7, -0.6)), "#8d90a6", grad=0.3)
    p.add(c.lump(0.35, 0.3, 0.35, "vent", 6, 3, 0.2), xf((-2.7, 4.3, -0.6)), "#eef2fa", grad=0.2, jitter=0.0)
    for x, z in ((-1.7, 0.95), (1.9, 0.95), (-1.7, -0.95), (1.9, -0.95)):
        c.wheel(p, x, 0.5, z, 0.5, 0.3, detail=0, tyre="#2b2f38")
    for z in (-0.7, 0.7):
        p.add_flat(box(0.06, 0.24, 0.4), xf((3.43, 1.2, z)), "#fff2cc", mat="glow_warm")
        p.add_flat(box(0.06, 0.24, 0.4), xf((-2.87, 1.45, z)), "#ff7a6a", mat="glow_red")
    p.add(box(0.2, 0.25, 2.0), xf((3.5, 0.82, 0.0)), "#c9ced8", grad=0.1)
    return p


def taxi():
    """Yellow toy sedan with a chequered stripe, a lit roof sign and lamps."""
    p = c.Piece("taxi")
    Y = "#ffc928"
    p.add(box(4.5, 0.75, 1.95, 0.14, 1), xf((0.0, 0.85, 0)), Y, grad=0.35)
    p.add(box(2.3, 0.85, 1.65, 0.14, 1), xf((-0.35, 1.65, 0)), Y, grad=0.3)
    p.add(box(1.2, 0.3, 1.7, 0.06, 1), xf((1.5, 1.28, 0)), Y, grad=0.3)
    for z in (-1, 1):
        p.add_flat(c.wall_quad((-0.35, 1.68, z * 0.84), (0, 0, z), 2.0, 0.62), None, "#1a2c46")
        p.add(box(0.1, 0.7, 0.06), xf((-0.35, 1.68, z * 0.84)), Y, grad=0.0, jitter=0.0)
    p.add_flat(c.wall_quad((0.8, 1.7, 0.0), (1, 0.35, 0), 1.45, 0.6), None, "#1a2c46")
    p.add_flat(c.wall_quad((-1.52, 1.68, 0.0), (-1, 0, 0), 1.35, 0.55), None, "#1a2c46")
    for k in range(11):
        for z in (-1, 1):
            p.add_flat(c.wall_quad((-2.0 + k * 0.4, 0.95, z * 0.995), (0, 0, z), 0.4, 0.22), None, "#f6f6f6" if k % 2 else "#20242e")
    p.add(box(0.95, 0.34, 0.4, 0.05, 1), xf((-0.35, 2.28, 0)), "#f4f4f8", grad=0.2)
    for z in (-1, 1):
        p.add_flat(c.wall_quad((-0.35, 2.28, z * 0.21), (0, 0, z), 0.8, 0.24), None, "#ffe2a0", mat="glow_warm")
    p.add_flat(c.wall_quad((0.13, 2.28, 0.0), (1, 0, 0), 0.34, 0.24), None, "#ffe2a0", mat="glow_warm")
    p.add(box(0.3, 0.2, 2.1), xf((2.3, 0.62, 0)), "#2b2f38", grad=0.0)
    p.add(box(0.3, 0.2, 2.1), xf((-2.3, 0.62, 0)), "#2b2f38", grad=0.0)
    for z in (-0.65, 0.65):
        p.add_flat(box(0.06, 0.24, 0.42), xf((2.27, 1.0, z)), "#fff2cc", mat="glow_warm")
        p.add_flat(box(0.06, 0.24, 0.42), xf((-2.27, 1.05, z)), "#ff7a6a", mat="glow_red")
    p.add(box(0.06, 0.24, 0.72), xf((2.27, 0.96, 0.0)), "#20242e", grad=0.0)
    for z in (-0.98, 0.98):
        p.add(box(0.16, 0.12, 0.22, 0.02, 1), xf((0.65, 1.36, z)), Y, grad=0.1)
    for x in (-1.4, 1.45):
        for z in (-0.95, 0.95):
            c.wheel(p, x, 0.4, z, 0.4, 0.3, detail=0, tyre="#20242e")
    return p


def build():
    c.ensure_materials()
    model = Model("scenery-metro")
    for name, piece in (
        ("skyscraper_a", skyscraper_a()),
        ("skyscraper_b", skyscraper_b()),
        ("skyscraper_c", skyscraper_c()),
        ("street_lamp", street_lamp()),
        ("food_truck", food_truck()),
        ("taxi", taxi()),
        ("metro_entrance", metro_entrance()),
        ("arcade_kiosk", arcade_kiosk()),
    ):
        piece.bake(ao_height=3.0 if name.startswith("skyscraper") else 2.2, strength=0.3)
        model.mesh("prop-" + name, piece.items())
    frame, (w, h, y) = poster_frame()
    frame.bake()
    root = model.mesh("prop-billboard", frame.items())
    face = model.mesh("face", poster_face(w - 0.4, h - 0.4), parent=root, location=(0.38, y, 0.0))
    add_face_uv(face)
    return model
