"""Glacier scenery: ice block, ice arch, frosted snow pine, igloo and a research station (MIT, AuraGo contributors).

Cold cyan and teal ice with white snow, warm orange paint and lit windows for contrast. Comic touch:
a penguin-shaped weather vane (with a red scarf) on the research station and a rubber-duck-free
snowmobile parked at the door. Props stand at y=0, front toward +X.
"""

import math

from . import scenery_b_common as c
from .core import Model
from .shapes import arc_band, box, cylinder, cylinder_x, ellipsoid, extrude_z, revolve, revolve_x, sphere, strut, torus, xf

SNOW = "#f2f7ff"
SNOW_SHADE = "#c9d8f0"
ICE_DEEP = "#2f7fc0"
ICE_LIGHT = "#bff0ff"


def ice(y0, y1, deep=ICE_DEEP, light=ICE_LIGHT):
    """Vertex colour: deep blue at ``y0`` rising to pale cyan at ``y1``."""

    def fn(x, y, z):
        t = min(1.0, max(0.0, (y - y0) / max(y1 - y0, 1e-6)))
        return c.mix(deep, light, t * t)

    return fn


def crystal(p, pos, r, h, rot, seed=0):
    p.add(revolve([(0.0, 0.0), (r, 0.0), (r, h), (0.0, h + r * 1.1)], 6), xf(pos, rot), ice(pos[1], pos[1] + h + r), grad=0.15)


def ice_block():
    """Scatter: a chunky cube of ice with leaning hex crystals and a snow cap."""
    p = c.Piece("ice_block")
    p.add(box(3.4, 2.6, 3.0, 0.2, 1), xf((0, 1.3, 0), (0, 22, 0)), ice(0.0, 2.6, "#3f8fd0", "#a8e4ff"), grad=0.2)
    p.add(c.lump(1.9, 0.5, 1.6, "ib-cap", 6, 3, 0.2), xf((-0.1, 2.55, 0.1), (0, 10, 0)), SNOW, grad=0.1, jitter=0.0)
    for pos, r, h, rot in (
        ((1.9, 0.0, 1.5), 0.7, 3.4, (0, 0, -14)),
        ((-1.8, 0.0, 1.3), 0.6, 2.6, (10, 20, 12)),
        ((0.4, 0.0, -2.0), 0.75, 3.9, (-12, 40, 4)),
        ((2.4, 0.0, -1.4), 0.5, 2.0, (6, 0, -18)),
        ((-2.3, 0.0, -1.1), 0.55, 2.3, (-8, 70, 14)),
    ):
        crystal(p, pos, r, h, rot)
    return p


def ice_arch():
    """A drive-under glacier arch (spans Z, depth along X), icicles, a snow crown and glowing crystals at the feet."""
    p = c.Piece("ice_arch")
    rot = xf((0, 0, 0), (0, 90, 0))
    p.add(arc_band(4.6, 7.8, 0.0, 180.0, -1.9, 1.9, 9), rot, ice(0.0, 8.0, "#2a74b8", "#a8e4ff"), grad=0.1)
    for sign in (1, -1):
        p.add(c.lump(2.0, 1.6, 2.2, ("arch-foot", sign), 7, 4, 0.22, flat_bottom=True), xf((0.0, 0.8, sign * 6.2)), ice(0.0, 2.4, "#2a74b8", "#7cc8f0"), grad=0.2)
    p.add(c.lump(2.8, 0.9, 2.4, "arch-crown", 7, 3, 0.18), xf((0.1, 7.6, 0.0), (0, 20, 0)), SNOW, grad=0.1, jitter=0.0)
    p.add(c.lump(1.5, 0.6, 1.4, "arch-crown2", 6, 3, 0.18), xf((0.0, 7.5, -3.9), (0, 50, 0)), SNOW_SHADE, grad=0.1, jitter=0.0)
    for k in range(9):
        a = math.radians(18 + 18 * k)
        r = 4.55
        length = (1.0 + 0.5 * ((k * 5) % 3)) if k not in (4,) else 1.9
        p.add(revolve([(0.0, 0.0), (0.3, 0.0), (0.0, -length * 1.5)], 5), xf((0.0, r * math.sin(a), r * math.cos(a)), (0, 0, 0)), "#cfeeff", grad=0.2, jitter=0.0)
    for sign in (1, -1):
        p.add_flat(revolve([(0.0, 0.0), (0.5, 0.0), (0.5, 1.2), (0.0, 1.9)], 5), xf((1.5, 0.0, sign * 4.1), (0, 0, -8)), "#bff8ff", mat="glow_cyan")
        p.add_flat(revolve([(0.0, 0.0), (0.32, 0.0), (0.32, 0.7), (0.0, 1.1)], 5), xf((1.6, 0.0, sign * 4.9), (10, 0, 12)), "#bff8ff", mat="glow_cyan")
    return p


def snow_pine():
    """Scatter: frosted teal spruce, five skirts with snowy tips and a few fairy lights."""
    p = c.Piece("snow_pine")
    p.add(revolve([(0.0, 0.0), (0.42, 0.0), (0.3, 1.8), (0.0, 1.8)], 5), None, "#6e4429", grad=0.5)
    greens = ["#14565c", "#1b6e69", "#238575", "#2b9c7e", "#35b088"]
    y = 1.1
    for i in range(5):
        h = 2.5 - 0.15 * i
        r = 2.45 - 0.42 * i
        yaw = 17.0 * i
        p.add(revolve([(0.0, y), (r, y), (r * 0.12, y + h), (0.0, y + h)], 7), xf((0, 0, 0), (0, yaw, 0)), greens[i], grad=0.45)
        p.add(revolve([(0.0, y + h * 0.4), (r * 0.74, y + h * 0.4), (0.0, y + h + 0.1)], 7), xf((0, 0, 0), (0, yaw, 0)), SNOW, grad=0.1, jitter=0.0)
        y += 1.5 - 0.1 * i
    p.add_flat(sphere(0.26, 5, 3), xf((0, y + 0.55, 0)), "#ffe2a0", mat="glow_warm")
    for pos, mat, col in (((1.7, 2.1, 0.8), "glow_pink", "#ff9ad0"), ((-0.6, 3.3, 1.2), "glow_cyan", "#bff8ff"), ((0.2, 4.4, -0.9), "glow_pink", "#ff9ad0"), ((-1.2, 2.3, -1.3), "glow_cyan", "#bff8ff"), ((0.8, 5.4, 0.3), "glow_warm", "#ffe2a0")):
        p.add_flat(sphere(0.13, 4, 2), xf(pos), col, mat=mat)
    return p


def arch_polygon(r, wall, count=8):
    pts = [(-r, 0.0)]
    pts += [(r * math.cos(math.pi * (1 - k / count)), wall + r * math.sin(math.pi * (1 - k / count))) for k in range(count + 1)]
    pts += [(r, 0.0)]
    return pts


def igloo():
    """Mosaic ice-block dome with a tunnel entrance, a lit doorway, snow drifts, a fishing hole and a flag."""
    p = c.Piece("igloo")
    R = 3.7
    prof = [(0.0, 0.0)] + [(R * math.cos(math.radians(a)), R * math.sin(math.radians(a))) for a in (0, 18, 36, 54, 72, 90)]
    prof[-1] = (0.0, R)
    sn, sh = c.colour("#eef6ff"), c.colour("#9dbde0")

    def mosaic(x, y, z):
        row = int(y / 0.95)
        k = int(round((math.atan2(z, x) % math.tau) / math.tau * 12) + (row % 2)) % 2
        return sn if k else sh

    p.add(revolve(prof, 12), None, mosaic, grad=0.2, jitter=0.0)
    p.add(extrude_z(arch_polygon(1.45, 0.45, 6), 0.0, 2.4), xf((R - 0.5, 0, 0), (0, 90, 0)), "#bcd6f0", grad=0.2, jitter=0.0)
    front = R + 1.9 - 0.0
    p.add_flat(extrude_z(arch_polygon(0.95, 0.35, 6), 0.0, 0.03), xf((front, 0, 0), (0, 90, 0)), "#1c2a4a")
    p.add_flat(extrude_z(arch_polygon(0.68, 0.3, 6), 0.0, 0.03), xf((front + 0.04, 0, 0), (0, 90, 0)), "#ffe2a0", mat="glow_warm")
    for pos, rad in (((R - 0.3, 0.3, 2.9), (1.8, 0.6, 1.2)), ((-1.0, 0.2, -R - 0.4), (2.2, 0.55, 1.1)), ((R + 1.6, 0.25, -1.9), (1.4, 0.5, 0.9))):
        p.add(c.lump(rad[0], rad[1], rad[2], ("drift", pos), 6, 3, 0.2), xf(pos), SNOW, grad=0.15, jitter=0.0)
    p.add(ellipsoid(1.0, 0.04, 0.8, 8, 2), xf((R + 1.4, 0.06, 3.2)), "#23509a", grad=0.0, jitter=0.0)
    p.add(strut((R + 2.0, 0.0, 3.9), (R + 1.3, 1.5, 3.2), 0.04, 4), None, "#a8703c", grad=0.0)
    p.add(strut((R + 1.3, 1.5, 3.2), (R + 1.4, 0.2, 3.1), 0.012, 3), None, "#ffffff", grad=0.0, jitter=0.0)
    p.add(strut((-0.5, R + 0.1, 0.0), (-0.5, R + 1.8, 0.0), 0.05, 4), None, "#a8703c", grad=0.0)
    p.add(box(0.04, 0.5, 0.8), xf((-0.5, R + 1.5, 0.45)), "#e0322e", grad=0.0, jitter=0.0)
    return p


def penguin(p, pos, s=1.0, yaw=0.0):
    place = xf(pos, (0, yaw, 0))
    p.add(ellipsoid(0.38 * s, 0.7 * s, 0.34 * s, 8, 4), place @ xf((0, 0.7 * s, 0)), "#20242e", grad=0.2, jitter=0.0)
    p.add(ellipsoid(0.2 * s, 0.52 * s, 0.26 * s, 6, 3), place @ xf((0.22 * s, 0.66 * s, 0)), "#f6f6f6", grad=0.1, jitter=0.0)
    p.add(sphere(0.27 * s, 7, 3), place @ xf((0.0, 1.5 * s, 0)), "#20242e", grad=0.2, jitter=0.0)
    p.add(revolve([(0.0, 0.0), (0.09 * s, 0.0), (0.0, 0.32 * s)], 4), place @ xf((0.22 * s, 1.45 * s, 0), (0, 0, -90)), "#ff9a2a", grad=0.0, jitter=0.0)
    for z in (-0.1, 0.1):
        p.add(sphere(0.06 * s, 4, 2), place @ xf((0.2 * s, 1.58 * s, z * s)), "#ffffff", grad=0.0, jitter=0.0)
    for z in (-1, 1):
        p.add(box(0.1 * s, 0.6 * s, 0.14 * s), place @ xf((0.0, 0.85 * s, z * 0.42 * s), (z * 22, 0, 0)), "#20242e", grad=0.0, jitter=0.0)
        p.add(box(0.34 * s, 0.07 * s, 0.14 * s), place @ xf((0.12 * s, 0.05 * s, z * 0.16 * s)), "#ff9a2a", grad=0.0, jitter=0.0)
    p.add(cylinder(0.3 * s, 0.3 * s, 0.12 * s, 8), place @ xf((0.0, 1.27 * s, 0)), "#e0322e", grad=0.0, jitter=0.0)


def station():
    p = c.Piece("research_station")
    W, O = "#dfe9f4", "#ff7a2f"
    for x in (-3.4, 0.0, 3.4):
        for z in (-1.4, 1.4):
            p.add(strut((x, 0.0, z), (x * 0.95, 3.0, z * 0.8), 0.2, 5), None, "#8d90a6", grad=0.4)
            p.add(box(0.8, 0.25, 0.8), xf((x, 0.12, z)), "#d2e3f6", grad=0.2)
    for z in (-1.4, 1.4):
        p.add(strut((-3.4, 0.4, z), (3.4, 2.6, z), 0.05, 3), None, "#8d90a6", grad=0.0)
        p.add(strut((3.4, 0.4, z), (-3.4, 2.6, z), 0.05, 3), None, "#8d90a6", grad=0.0)
    body = [(0.0, -5.4), (1.2, -5.2), (2.0, -4.3), (2.0, 4.3), (1.2, 5.2), (0.0, 5.4)]
    belly = lambda x, y, z: O if y < 3.15 else W
    p.add(revolve_x(body, 10), xf((0, 4.0, 0)), belly, grad=0.25)
    for x in (-3.4, 3.4):
        p.add(cylinder_x(2.04, 2.04, 0.5, 10), xf((x, 4.0, 0)), O, grad=0.2, jitter=0.0)
    for x in (-2.2, -0.8, 0.6, 2.0):
        for z in (-1, 1):
            p.add(c.disc((x, 4.25, z * 1.925), (0, 0, z), 0.5, 8), None, "#3a4156", grad=0.0, jitter=0.0)
            p.add_flat(c.disc((x, 4.25, z * 1.945), (0, 0, z), 0.4, 8), None, "#ffe2a0", mat="glow_warm")
    p.add(box(0.2, 1.7, 1.1), xf((5.25, 3.3, 0)), "#3a4156", grad=0.0)
    p.add_flat(box(0.05, 0.3, 0.9), xf((5.4, 4.15, 0)), "#ffe2a0", mat="glow_warm")
    for k in range(6):
        p.add(box(0.5, 0.18, 1.5), xf((7.6 - k * 0.5, 0.25 + k * 0.5, 0)), "#8d90a6", grad=0.2, jitter=0.0)
    p.add(box(0.6, 0.15, 1.6), xf((5.5, 2.45, 0)), "#8d90a6", grad=0.0)
    for z in (-0.75, 0.75):
        p.add(strut((7.9, 0.9, z), (5.7, 3.3, z), 0.04, 3), None, O, grad=0.0, jitter=0.0)
    # side pod on a connector tube
    p.add(cylinder_x(0.55, 0.55, 3.0, 8), xf((0, 3.4, -3.3), (0, 90, 0)), "#c9ced8", grad=0.3)
    p.add(c.lump(2.0, 1.7, 2.0, "pod", 8, 4, 0.04), xf((0, 3.6, -6.0)), W, grad=0.25, jitter=0.0)
    p.add(cylinder_x(0.4, 0.4, 0.4, 8), xf((0, 3.0, -4.2)), O, grad=0.1, jitter=0.0)
    for z in (-0.6, 0.6):
        p.add_flat(c.disc((2.03, 4.0 - 0.3, -6.0 + z), (1, 0, 0), 0.34, 8), None, "#ffe2a0", mat="glow_warm")
    # radar dome on the roof
    p.add(cylinder(1.3, 1.3, 0.5, 10), xf((-2.0, 6.2, 0)), "#c9ced8", grad=0.2)
    p.add(c.lump(1.4, 1.35, 1.4, "dome", 10, 5, 0.0), xf((-2.0, 6.9, 0)), "#e8f0fa", grad=0.2, jitter=0.0)
    # dish and mast
    p.add(revolve([(0.0, 0.0), (1.2, 0.45), (1.2, 0.56), (0.0, 0.16)], 10), xf((2.6, 6.25, 1.0), (0, 0, -34)), "#e4ecf4", grad=0.2, jitter=0.0)
    p.add(strut((2.6, 6.0, 1.0), (2.9, 6.5, 1.0), 0.09, 4), None, "#8d90a6", grad=0.0)
    for i in range(4):
        p.add(cylinder(0.12, 0.12, 1.5, 6), xf((-3.8, 6.9 + i * 1.5, 0.8)), "#d8402e" if i % 2 == 0 else "#f4f4f8", grad=0.1, jitter=0.0)
    p.add_flat(sphere(0.2, 5, 3), xf((-3.8, 12.9, 0.8)), "#ff7a6a", mat="glow_red")
    # penguin weather vane at the +X end of the roof
    p.add(strut((4.0, 6.0, -0.6), (4.0, 7.6, -0.6), 0.06, 4), None, "#8d90a6", grad=0.0)
    for ang in range(4):
        a = math.radians(90 * ang)
        p.add(strut((4.0, 7.3, -0.6), (4.0 + 0.9 * math.cos(a), 7.3, -0.6 + 0.9 * math.sin(a)), 0.03, 3), None, "#8d90a6", grad=0.0)
        p.add(box(0.14, 0.14, 0.14), xf((4.0 + 1.0 * math.cos(a), 7.3, -0.6 + 1.0 * math.sin(a))), O if ang == 0 else "#8d90a6", grad=0.0, jitter=0.0)
    penguin(p, (4.0, 7.55, -0.6), 0.55)
    # solar panels, barrels, snowmobile, flag
    for z in (3.1, 4.7):
        p.add(box(0.12, 1.5, 1.4), xf((-5.6, 1.6, z), (0, 0, 25)), "#27468a", grad=0.3)
        p.add(strut((-5.6, 0.0, z), (-5.6, 1.1, z), 0.06, 4), None, "#8d90a6", grad=0.0)
    for x, z, col in ((6.4, 2.6, O), (7.2, 2.2, "#41a8d8"), (6.8, 3.4, O)):
        p.add(cylinder(0.35, 0.35, 0.9, 6), xf((x, 0.45, z)), col, grad=0.3)
    sx, sz = 8.0, -3.2
    p.add(box(1.9, 0.6, 0.9, 0.08, 1), xf((sx, 0.75, sz), (0, 25, 0)), O, grad=0.3)
    p.add(box(0.9, 0.25, 0.6), xf((sx - 0.5, 1.15, sz + 0.2), (0, 25, 0)), "#2b2f38", grad=0.0, jitter=0.0)
    for dz in (-0.5, 0.5):
        p.add(box(2.1, 0.1, 0.22), xf((sx + 0.1, 0.3, sz + dz * 1.0), (0, 25, 0)), "#3a4156", grad=0.0, jitter=0.0)
    p.add_flat(sphere(0.12, 4, 2), xf((sx + 0.95, 0.85, sz - 0.35)), "#fff2cc", mat="glow_warm")
    p.add(strut((-6.5, 0.0, -2.5), (-6.5, 4.2, -2.5), 0.05, 4), None, "#8d90a6", grad=0.0)
    p.add(box(0.04, 0.7, 1.1), xf((-6.5, 3.8, -3.1)), "#e0322e", grad=0.0, jitter=0.0)
    # snow on roof and drifts
    p.add(c.lump(3.6, 0.55, 1.5, "roof-snow", 8, 3, 0.1), xf((0.3, 5.9, 0.0)), SNOW, grad=0.1, jitter=0.0)
    for pos, rad in (((-2.0, 0.2, 2.4), (1.8, 0.6, 1.2)), ((3.0, 0.2, -2.6), (1.6, 0.5, 1.2))):
        p.add(c.lump(rad[0], rad[1], rad[2], ("st-drift", pos), 6, 3, 0.2), xf(pos), SNOW, grad=0.15, jitter=0.0)
    return p


def build():
    c.ensure_materials()
    model = Model("scenery-glacier")
    for name, piece in (("ice_block", ice_block()), ("ice_arch", ice_arch()), ("snow_pine", snow_pine()), ("igloo", igloo()), ("research_station", station())):
        piece.bake()
        model.mesh("prop-" + name, piece.items())
    return model
