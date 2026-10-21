"""Stone-head tunnel portal of the jungle temple (MIT License, see ../LICENSE).

The kit's giant head (``head_temple``) has only a decorative mouth recess, so the portal is a course-side copy in the
same style: the skull form, colours, moss cap, brow, glowing eyes and glyph row of the kit head (``head_temple_shape``
and ``head_temple`` are used as they are), squashed to ``DEPTH_SCALE`` in depth, with the mouth cut through the whole
head. The cut is the tunnel arch (``tunnel_plan.HALF`` wide, ``CROWN`` high, the same ellipse as the roof behind it),
framed by a stone lip and a row of upper teeth that stay above 1.3 m, outside the 1.9 x 1.0 m opening. The cut is dark
inside. Face features use fewer segments than the hero prop to keep the course inside its triangle budget.

``build(seed, index)`` returns one joined mesh in the local portal frame (x right, y forward, z up, the middle of the
mouth on the floor at the origin, the head behind it); ``tunnel_parts.place`` moves it onto the lane.
"""

import math

from . import shapes
from .head_temple import _skull_colour
from .head_temple_shape import HALF_D, ZTOP, brow_line, build_cap, front_y, skull_xy, width_factor
from .palette import mix
from .prop import Prop
from .props_temple_util import arch_outline, boolean_cut, clamp01, nz, ring_grid, shade, smoothstep
from .tunnel_plan import CROWN, HALF, PORTAL, SPRING

WORLD = "temple"
SX = SZ = HALF / 1.1                 # the head's mouth is 1.1 wide; the tunnel arch fixes the scale
DEPTH = PORTAL["temple"][1]
SY = 0.8                             # the head is squashed in depth so the straight run stays short
COLS = 44
FRONT_Z = 0.6                        # height at which the face plane meets the mouth plane


def _levels():
    zs = [0.0, 0.3, 0.6] + [0.9 + 0.3 * k for k in range(8)] + [3.1 + (ZTOP - 3.1) * math.sin(0.5 * math.pi * k / 6) for k in range(0, 7)]
    return sorted(set(round(z, 4) for z in zs))


def _theta(j):
    u = j / COLS
    return -math.pi / 2 + math.tau * u - 0.45 * math.sin(math.tau * u)


def _skull():
    rings = []
    for z in _levels():
        rings.append([(*skull_xy(_theta(j), z), z) for j in range(COLS)])
    skull = ring_grid("skull", rings, cap_first=True, center=(0.0, 0.0, 2.3))
    pts = arch_outline(HALF / SX, SPRING / SZ, steps=14, z0=-0.5, rise=(CROWN - SPRING) / SZ)
    cutter = shapes.loft("mouth-cut", [[(x, y, z) for x, z in pts] for y in (-4.5, 4.5)], closed=True, recalc=True)
    return boolean_cut(skull, cutter)


def _cap(p):
    pal = p.pal
    moss_d, moss, moss_l, yg = pal.c("moss_dark"), pal.c("moss"), pal.c("moss_light"), pal.c("yellow_green")
    off = (11.0, -7.0, 3.0)

    def fn(x, y, z):
        t = clamp01((z - 3.5) / 1.2)
        c = mix(moss_d, moss, smoothstep(t, 0.0, 0.55))
        c = mix(c, moss_l, 0.85 * smoothstep(t, 0.35, 1.0))
        c = mix(c, yg, 0.18 * smoothstep(nz(x, y, z, off, 2.2), 0.1, 0.7))
        return shade(c, 1.0 + 0.1 * nz(x, y, z, off, 3.5))

    p.add(build_cap(p.rng, cols=36), "matte", fn=fn, bevel=0, dens=0, smooth_angle=85.0, ao=0.6)


def _face(p):
    """Eyes, brow, nose, forehead medallion and glyphs: the kit head's features with fewer segments."""
    for s in (-1, 1):
        ex = s * 0.98
        ez = brow_line(ex) - 0.52
        ey = front_y(ex, ez)
        socket = shapes.sphere("socket", 0.52, seg=10, rings=6)
        p.add(socket, "matte", ("black", "stone_deep"), loc=(ex, ey + 0.12, ez), scale=(1.0, 0.45, 0.8), bevel=0, dens=0, ao=0.9, smooth_angle=80)
        eye = shapes.sphere("eye", 0.39, seg=10, rings=6)
        p.add(eye, "glow", ("gold", "eye_glow"), loc=(ex, ey - 0.06, ez), scale=(1.0, 0.62, 0.9), bevel=0, dens=0, ao=0.25, strength=0.7, smooth_angle=80)
        upper = [(ex + math.cos(a) * 0.45, ey - 0.16, ez + math.sin(a) * 0.4) for a in (math.pi * (0.03 + 0.94 * k / 8) for k in range(9))]
        lid = shapes.tube("lid", upper, radius=[0.07 + 0.075 * math.sin(math.pi * k / 8) for k in range(9)], seg=6)
        p.add(lid, "matte", ("stone", "stone_light"), bevel=0, dens=0, smooth_angle=80, ao=0.5)
        ear = shapes.cylinder("ear", 0.7, 0.66, 0.3, seg=14, base=False)
        p.add(ear, "matte", ("stone_dark", "stone"), loc=(s * (2.3 * width_factor(2.5) + 0.04), 0.05, 2.5), rot=(0, 90, 0), bevel=0, dens=0, smooth_angle=60)
        spool = shapes.cylinder("ear-spool", 0.27, 0.27, 0.42, seg=10, base=False)
        p.add(spool, "satin", ("gold_dark", "gold_light"), loc=(s * (2.3 * width_factor(2.5) + 0.32), 0.05, 2.5), rot=(0, 90, 0), bevel=0, dens=0, ao=0.45)
    tip_y = front_y(0.0, 2.15) - 0.12
    tip = shapes.sphere("nose-tip", 0.5, seg=12, rings=8)
    p.add(tip, "matte", ("stone", "stone_light"), loc=(0.0, tip_y, 2.1), scale=(1.1, 0.85, 0.72), bevel=0, dens=0, smooth_angle=80, ao=0.55)
    ey = front_y(0.0, 3.85)
    medal = shapes.cylinder("medallion", 0.3, 0.3, 0.12, seg=14, base=False)
    p.add(medal, "satin", ("gold_dark", "gold_light"), loc=(0.0, ey - 0.02, 3.88), rot=(90, 0, 0), bevel=0, dens=0, ao=0.4)
    for k in range(8):
        x = -1.8 + 3.6 * k / 7
        if abs(x) < 0.45:
            continue
        slope = (front_y(x + 0.05, 3.8) - front_y(x - 0.05, 3.8)) / 0.1
        glyph = shapes.box("glyph", (0.27, 0.14, 0.25), base=False)
        p.add(glyph, "matte", ("totem_red", "fire_orange") if k % 2 else ("totem_teal", "totem_teal"), loc=(x, front_y(x, 3.8) - 0.03, 3.8),
              rot=(0, 0, math.degrees(math.atan(slope))), bevel=0, dens=0, ao=0.5)


def _mouth(p):
    """Stone lip around the arch and upper teeth (head space; the arch is the tunnel arch in head units)."""
    half, spring, rise = HALF / SX, SPRING / SZ, (CROWN - SPRING) / SZ
    pts = []
    for k in range(0, 15):
        a = math.pi * k / 14
        pts.append((-(half + 0.1) * math.cos(a), spring + (rise + 0.1) * math.sin(a)))
    path = [(x, front_y(x, z) - 0.02, z) for x, z in pts]
    lip = shapes.tube("lip", path, radius=[0.15 + 0.07 * math.sin(math.pi * k / 14) for k in range(15)], seg=7)
    p.add(lip, "matte", ("stone", "stone_light"), bevel=0, dens=0, smooth_angle=80, ao=0.55)
    for deg in range(56, 125, 9):
        a = math.radians(deg)
        x, z = (half - 0.02) * math.cos(a), spring + (rise - 0.02) * math.sin(a)
        nx, nz_ = -math.cos(a) / half, -math.sin(a) / rise
        n = math.hypot(nx, nz_)
        phi = math.degrees(math.atan2(nx / n, nz_ / n))
        tooth = shapes.box("tooth", (0.24, 0.28, 0.4), base=False)
        p.add(tooth, "satin", ("stone_light", "gold_light"), loc=(x + nx / n * 0.15, front_y(x, z) + 0.15, z + nz_ / n * 0.15), rot=(0, phi, 0), bevel=0.05,
              segments=1, dens=0, ao=0.5)


def build(seed, index):
    """The portal mesh (see the module docstring)."""
    p = Prop(WORLD, "tunnel-portal-%d" % index, seed=seed, full_name="tunnel-portal-%d" % index)
    off = (p.rng.uniform(-30, 30), p.rng.uniform(-30, 30), p.rng.uniform(-30, 30))
    p.add(_skull(), "matte", fn=_skull_colour(p, off), bevel=0, dens=0, smooth_angle=50.0, weighted=False, ao=0.75)
    _cap(p)
    _face(p)
    _mouth(p)
    obj = p.build()
    shift = -front_y(0.0, FRONT_Z)                      # the face plane at the mouth meets y = 0 after the squash
    obj.data.transform(shapes.matrix((0.0, SY * shift, 0.0), (0, 0, 0), (SX, SY, SZ)))
    return obj
