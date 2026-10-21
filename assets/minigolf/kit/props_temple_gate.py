"""Jungle temple gate and stairs (MIT License).

``gate``: a stepped (corbelled) temple gate that spans the lane. The passage
runs along Y and is at least 2.8 m wide (posts start at x = +-1.35) with the
lowest overhead stone at 1.45 m. ``stairs``: a broken, mossy stair fragment with
cheek walls for edge decoration (rises towards +Y).
"""

import math

from . import shapes
from .prop import Prop
from .props_temple_util import stone_fn

WORLD = "temple"
IW = 1.4  # half of the clear width between the shafts
CLEARANCE = 1.4  # metadata: lowest overhead point above the lane (the build is 1.45)


def _block(p, size, x, y, z0, fn, yaw=0.0, bevel=0.07, segments=2, dens=0.8, ao=0.72):
    block = shapes.box("block", size, base=True)
    p.add(block, "matte", fn=fn, loc=(x, y, z0), rot=(0, 0, yaw), bevel=bevel, segments=segments, dens=dens, ao=ao, smooth_angle=50.0)


def _moss(p, rng, x, y, z, r):
    blob = shapes.blob("moss", r, rng, subdiv=2, amp=0.28, squash=(1.0, 1.0, 0.55), relax=1)
    p.add(blob, "matte", "moss", loc=(x, y, z), bevel=0, dens=0, smooth_angle=80.0, ao=0.6, noise_amp=0.1)


def build_gate(seed=1):
    """Stepped temple gate: plinths, carved shafts, two corbel courses, a capstone and a stepped crown."""
    p = Prop(WORLD, "gate", seed)
    rng = p.rng
    off = (rng.uniform(-40, 40), rng.uniform(-40, 40), rng.uniform(-40, 40))
    pal = p.pal

    def fn(top, moss=0.85, depth=0.2):
        return stone_fn(pal, 0.0, 3.1, off, moss=moss, moss_z=top, moss_depth=depth, noise_amp=0.08, strata=0.04)

    for s in (-1, 1):
        j = rng.uniform(-1.1, 1.1)
        _block(p, (1.3, 1.6, 0.26), s * 2.0, 0.0, 0.0, fn(0.26, 0.5, 0.1), yaw=j)
        _block(p, (1.1, 1.35, 0.26), s * 1.95, 0.0, 0.26, fn(0.52, 0.5, 0.1), yaw=-j)
        _block(p, (0.95, 1.1, 0.93), s * 1.875, 0.0, 0.52, fn(1.45, 0.3, 0.15), yaw=j * 0.5, bevel=0.09)
        _block(p, (1.5, 1.25, 0.34), s * 1.75, 0.0, 1.45, fn(1.79, 0.55, 0.12), yaw=-j * 0.6)
        _block(p, (1.7, 1.05, 0.34), s * 1.4, 0.0, 1.79, fn(2.13, 0.6, 0.12), yaw=j * 0.4)
    _block(p, (2.8, 1.05, 0.4), 0.0, 0.0, 2.13, fn(2.53, 0.7, 0.14), yaw=0.0, bevel=0.09)
    _block(p, (2.0, 0.85, 0.28), 0.0, 0.0, 2.53, fn(2.81, 0.8, 0.14), yaw=rng.uniform(-1, 1))
    _block(p, (1.2, 0.65, 0.26), 0.0, 0.0, 2.81, fn(3.07, 0.9, 0.15), yaw=rng.uniform(-1, 1))
    red, teal = ("totem_red", "fire_orange"), ("totem_teal", "totem_teal")
    for f in (-1, 1):  # decorate the front and the back
        for s in (-1, 1):
            panel = shapes.box("panel", (0.5, 0.1, 0.62), base=True)
            p.add(panel, "matte", teal, loc=(s * 1.875, f * 0.57, 0.68), bevel=0.04, segments=2, dens=0, ao=0.5)
            stud = shapes.cylinder("stud", 0.1, 0.1, 0.08, seg=14, base=False)
            p.add(stud, "satin", ("gold_dark", "gold_light"), loc=(s * 1.875, f * 0.65, 1.2), rot=(90, 0, 0), bevel=0.02, segments=1,
                  dens=0, ao=0.4)
            for k in range(4):
                glyph = shapes.box("glyph", (0.2, 0.1, 0.2), base=True)
                p.add(glyph, "matte", red if k % 2 else teal, loc=(s * (1.2 + 0.3 * k), f * 0.65, 1.55), bevel=0.035, segments=2,
                      dens=0, ao=0.5)
            tab = shapes.box("tab", (0.34, 0.1, 0.22), base=True)
            p.add(tab, "matte", teal if s > 0 else red, loc=(s * 0.9, f * 0.55, 2.25), bevel=0.04, segments=2, dens=0, ao=0.5)
        sun = shapes.cylinder("sun", 0.32, 0.32, 0.1, seg=22, base=False)
        p.add(sun, "satin", ("gold_dark", "gold_light"), loc=(0.0, f * 0.56, 2.33), rot=(90, 0, 0), bevel=0.03, segments=2, dens=0,
              ao=0.4)
        for k in range(8):  # sun rays
            a = k * math.tau / 8
            ray = shapes.cone("ray", 0.055, 0.17, seg=6)
            p.add(ray, "satin", ("gold", "gold_light"), loc=(math.cos(a) * 0.3, f * 0.56, 2.33 + math.sin(a) * 0.3),
                  rot=(0, 90.0 - math.degrees(a), 0), bevel=0, dens=0, ao=0.4)
    for sx in (-1, 1):  # gold finials on the crown corners
        fin = shapes.cone("finial", 0.12, 0.34, seg=10)
        p.add(fin, "satin", ("gold_dark", "gold_light"), loc=(sx * 0.9, 0.0, 2.81), bevel=0, dens=0, ao=0.4)
    top = shapes.sphere("top-gem", 0.17, seg=14, rings=10)
    p.add(top, "glow", ("gold", "gold_light"), loc=(0.0, 0.0, 3.2), bevel=0, dens=0, ao=0.25, strength=0.7, smooth_angle=80.0)
    for x, y, z, r in ((-1.0, 0.2, 2.53, 0.34), (0.7, -0.15, 2.53, 0.3), (-2.0, 0.3, 1.79, 0.36), (1.9, -0.2, 1.79, 0.38),
                       (-2.25, 0.0, 0.52, 0.3), (2.05, 0.25, 0.26, 0.32), (0.2, 0.1, 3.07, 0.22), (-1.5, -0.3, 2.13, 0.3),
                       (1.4, 0.3, 2.13, 0.3), (2.3, -0.2, 1.45, 0.25)):
        _moss(p, rng, x, y, z + 0.02, r)
    return p.build()


def _cheek(p, fn, x0, width, y_front, y_back, z_front, z_back, name="cheek"):
    """Sloped cheek wall of a stair flight: a polygon in (y, z) extruded along X."""
    poly = [(y_front, 0.0), (y_back, 0.0), (y_back, z_back), (y_front, z_front)]
    rings = [[(x, y, z) for y, z in poly] for x in (x0, x0 + width)]
    wall = shapes.loft(name, rings, closed=True, recalc=True)
    p.add(wall, "matte", fn=fn, bevel=0.06, segments=2, dens=0.5, ao=0.72, smooth_angle=50.0)


def build_stairs(seed=1):
    """Broken, mossy stair fragment: six steps, one tall and one broken cheek wall, a landing and a column stub."""
    p = Prop(WORLD, "stairs", seed)
    rng = p.rng
    off = (rng.uniform(-40, 40), rng.uniform(-40, 40), rng.uniform(-40, 40))
    pal = p.pal
    n, rise, tread, width = 6, 0.19, 0.42, 1.7
    y_back = 1.2

    def fn(top, moss=0.85, depth=0.16):
        return stone_fn(pal, 0.0, 1.6, off, moss=moss, moss_z=top, moss_depth=depth, noise_amp=0.09, strata=0.03)

    for i in range(n):
        y0 = -1.25 + i * tread
        h = rise * (i + 1)
        _block(p, (width + rng.uniform(-0.06, 0.06), y_back - y0, h), rng.uniform(-0.03, 0.03), (y0 + y_back) / 2, 0.0,
               fn(h, 0.9 if i % 2 else 0.5), yaw=rng.uniform(-2.0, 2.0), bevel=0.05, segments=2)
    top = rise * n
    _block(p, (width + 0.7, 0.8, 0.22), 0.0, y_back + 0.25, top - 0.06, fn(top + 0.16, 0.9), yaw=rng.uniform(-1.5, 1.5), bevel=0.06)
    _cheek(p, fn(1.5, 0.85, 0.2), width / 2 + 0.02, 0.32, -1.25, y_back + 0.1, 0.3, 1.5)
    _cheek(p, fn(0.9, 0.9, 0.25), -width / 2 - 0.34, 0.32, -1.25, 0.2, 0.3, 0.95, name="cheek-broken")
    for k in range(2):  # fallen chunks at the foot of the broken wall
        chunk = shapes.box("chunk", (rng.uniform(0.3, 0.45), rng.uniform(0.3, 0.4), rng.uniform(0.2, 0.28)), base=True)
        p.add(chunk, "matte", fn=fn(0.3, 0.9, 0.12), loc=(-width / 2 - 0.55 - k * 0.3, -0.7 + k * 0.55, 0.0),
              rot=(rng.uniform(-8, 8), rng.uniform(-8, 8), rng.uniform(0, 360)), bevel=0.04, segments=2, dens=0, ao=0.7, smooth_angle=50.0)
    stump = shapes.lathe("stump", [(0.0, 0.0), (0.3, 0.0), (0.28, 0.4), (0.27, 0.75), (0.3, 0.86), (0.0, 0.86)], seg=14)
    p.add(stump, "matte", fn=fn(top + 0.86, 0.9, 0.3), loc=(-0.5, y_back + 0.3, top + 0.14), bevel=0, dens=0, ao=0.7, smooth_angle=60.0)
    for x, y, z, r in ((0.45, 0.0, rise * 4, 0.2), (-0.4, -0.6, rise * 3, 0.17), (0.0, 0.9, top + 0.1, 0.22), (0.9, -1.2, 0.0, 0.2)):
        _moss(p, rng, x, y, z + 0.03, r)
    return p.build()
