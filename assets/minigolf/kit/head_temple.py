"""Hero prop of the jungle temple: the giant carved stone head (MIT License).

Rounded Olmec-style skull (see ``head_temple_shape``) with heavy brows, glowing
eyes, a broad nose, thick lips around an arched, dark mouth, gold ear plugs and
an irregular moss cap. The mouth recess keeps the tunnel look but is only
decoration: at runtime the head stands beside the lane.
Prop space: the face looks towards -Y, the base is z=0, footprint centred.
"""

import math

from . import shapes
from .head_temple_shape import HALF_W, MOUTH_HALF, SPRING, arch_sd, brow_line, build_cap, build_skull, front_y, width_factor
from .palette import mix
from .prop import Prop
from .props_temple_util import clamp01, nz, shade, smoothstep

WORLD = "temple"


def _skull_colour(p, off):
    pal = p.pal
    lo, mid, hi = pal.c("stone_dark"), pal.c("stone"), pal.c("stone_light")
    moss_d, moss = pal.c("moss_dark"), pal.c("moss")
    black, teal = pal.c("black"), pal.c("totem_teal")

    def fn(x, y, z):
        t = clamp01(z / 4.3)
        c = mix(lo, mid, smoothstep(t, 0.0, 0.6))
        c = mix(c, hi, 0.25 * smoothstep(t, 0.5, 1.0))
        k = 1.0 + 0.07 * nz(x, y, z, off, 1.5)
        c = shade(c, k)
        c = mix(c, mix(moss_d, moss, 0.5), 0.5 * smoothstep(z, 3.4 + 0.2 * nz(x, y, z, off, 2.0), 3.9))
        # cheek paint: three horizontal teal bars on both sides of the mouth
        ax = abs(x)
        if y < -0.5 and 1.05 < ax < 2.15:
            mask = smoothstep(ax, 1.05, 1.25) * smoothstep(ax, 2.15, 1.95)
            bar = smoothstep(min(abs(z - 1.5), abs(z - 1.8), abs(z - 2.1)), 0.11, 0.05)
            c = mix(c, shade(teal, 0.8), 0.92 * mask * bar)
        # inside the mouth recess: darker with depth, black at the back wall
        arch = arch_sd(x, z)
        if arch < 0.04 and y > front_y(x, z) + 0.05:
            depth = y - front_y(x, z)
            c = mix(c, shade(black, 1.0), 0.97 * smoothstep(depth, 0.05, 1.4))
        return c

    return fn


def _skull(p, off):
    skull = build_skull()
    p.add(skull, "matte", fn=_skull_colour(p, off), bevel=0, dens=0.32, smooth_angle=50.0, weighted=False, ao=0.75)


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

    p.add(build_cap(p.rng), "matte", fn=fn, bevel=0, dens=0, smooth_angle=85.0, ao=0.6)


def _eyes(p):
    for s in (-1, 1):
        ex = s * 0.98
        ez = brow_line(ex) - 0.52
        ey = front_y(ex, ez)
        socket = shapes.sphere("socket", 0.52, seg=14, rings=8)
        p.add(socket, "matte", ("black", "stone_deep"), loc=(ex, ey + 0.12, ez), scale=(1.0, 0.45, 0.8), bevel=0, dens=0, ao=0.9,
              smooth_angle=80)
        eye = shapes.sphere("eye", 0.39, seg=16, rings=10)
        p.add(eye, "glow", ("gold", "eye_glow"), loc=(ex, ey - 0.06, ez), scale=(1.0, 0.62, 0.9), bevel=0, dens=0, ao=0.25,
              strength=0.7, smooth_angle=80)
        pupil = shapes.sphere("pupil", 0.12, seg=8, rings=5)
        p.add(pupil, "matte", ("black", "stone_deep"), loc=(ex, ey - 0.3, ez), scale=(0.55, 0.3, 1.5), bevel=0, dens=0, ao=0.2, smooth_angle=80)
        # heavy upper lid and a thin lower lid
        upper = [(ex + math.cos(a) * 0.45, ey - 0.16, ez + math.sin(a) * 0.4) for a in
                 (math.pi * (0.03 + 0.94 * k / 12) for k in range(13))]
        lower = [(ex + math.cos(a) * 0.42, ey - 0.1, ez + math.sin(a) * 0.37) for a in
                 (math.pi * (1.1 + 0.8 * k / 8) for k in range(9))]
        lid = shapes.tube("lid", upper, radius=[0.07 + 0.075 * math.sin(math.pi * k / 12) for k in range(13)], seg=8)
        p.add(lid, "matte", ("stone", "stone_light"), bevel=0, dens=0, smooth_angle=80, ao=0.5)
        low = shapes.tube("lid-low", lower, radius=[0.04 + 0.04 * math.sin(math.pi * k / 8) for k in range(9)], seg=6)
        p.add(low, "matte", ("stone_dark", "stone"), bevel=0, dens=0, smooth_angle=80, ao=0.5)


def _nose(p):
    tip_y = front_y(0.0, 2.15) - 0.12
    tip = shapes.sphere("nose-tip", 0.5, seg=18, rings=12)
    p.add(tip, "matte", ("stone", "stone_light"), loc=(0.0, tip_y, 2.1), scale=(1.1, 0.85, 0.72), bevel=0, dens=0, smooth_angle=80,
          ao=0.55)
    for s in (-1, 1):
        wing = shapes.sphere("nose-wing", 0.33, seg=14, rings=8)
        p.add(wing, "matte", ("stone_dark", "stone_light"), loc=(s * 0.46, tip_y + 0.25, 2.02), scale=(1.0, 0.9, 0.85), bevel=0,
              dens=0, smooth_angle=80, ao=0.6)
        hole = shapes.sphere("nostril", 0.14, seg=8, rings=5)
        p.add(hole, "matte", ("black", "stone_deep"), loc=(s * 0.21, tip_y - 0.3, 1.86), scale=(0.9, 0.7, 1.15), bevel=0, dens=0, ao=0.95)


def _mouth(p):
    w = MOUTH_HALF + 0.12
    pts = [(-w, 0.05 + (SPRING - 0.05) * k / 4) for k in range(5)]
    pts += [(-w * math.cos(math.pi * k / 16), SPRING + w * math.sin(math.pi * k / 16)) for k in range(1, 17)]
    pts += [(w, SPRING - (SPRING - 0.05) * k / 4) for k in range(1, 5)]
    path3 = [(x, front_y(x, z) - 0.02, z) for x, z in pts]
    n = len(path3)
    radii = [0.17 + 0.08 * math.sin(math.pi * k / (n - 1)) for k in range(n)]
    lip = shapes.tube("lip", path3, radius=radii, seg=10)
    p.add(lip, "matte", ("stone", "stone_light"), bevel=0, dens=0, smooth_angle=80, ao=0.55)
    # swollen upper lip roll above the arch
    upper = []
    for k in range(15):
        a = math.radians(28 + 124 * k / 14)
        x, z = (w + 0.12) * math.cos(a), SPRING + (w + 0.15) * math.sin(a)
        upper.append((x, front_y(x, z) - 0.1, z))
    roll = shapes.tube("upper-lip", upper, radius=[0.1 + 0.1 * math.sin(math.pi * k / 14) for k in range(15)], seg=10)
    p.add(roll, "matte", ("stone", "stone_light"), bevel=0, dens=0, smooth_angle=80, ao=0.55)
    # teeth: rounded blocks pointing into the mouth
    for a_deg in (18, 34, 50, 66, 82, 98, 114, 130, 146, 162):
        a = math.radians(a_deg)
        x, z = (MOUTH_HALF - 0.04) * math.cos(a), SPRING + (MOUTH_HALF - 0.04) * math.sin(a)
        phi = math.degrees(math.atan2(-math.cos(a), -math.sin(a)))
        size = 0.5 - 0.1 * abs(90 - a_deg) / 72.0
        tooth = shapes.box("tooth", (0.27, 0.32, size), base=True)
        p.add(tooth, "satin", ("stone_light", "gold_light"), loc=(x, front_y(x, z) + 0.2, z), rot=(0, phi, 0), bevel=0.07, segments=1,
              dens=0, ao=0.5)
    for s in (-1, 1):
        fang = shapes.cone("fang", 0.13, 0.55, seg=10)
        p.add(fang, "satin", ("stone_light", "gold_light"), loc=(s * (MOUTH_HALF - 0.14), front_y(MOUTH_HALF, 0.2) + 0.25, 0.0), bevel=0,
              dens=0, ao=0.5)


def _ears(p):
    for s in (-1, 1):
        ex = s * (HALF_W * width_factor(2.5) + 0.04)
        ear = shapes.cylinder("ear", 0.7, 0.66, 0.3, seg=22, base=False)
        p.add(ear, "matte", ("stone_dark", "stone"), loc=(ex, 0.05, 2.5), rot=(0, 90, 0), bevel=0.08, segments=2, dens=0, smooth_angle=60)
        ring = shapes.torus("ear-ring", 0.5, 0.1, seg=18, rseg=6)
        p.add(ring, "matte", ("stone", "stone_light"), loc=(ex + s * 0.14, 0.05, 2.5), rot=(0, 90, 0), bevel=0, dens=0, ao=0.5, smooth_angle=80)
        spool = shapes.cylinder("ear-spool", 0.27, 0.27, 0.42, seg=16, base=False)
        p.add(spool, "satin", ("gold_dark", "gold_light"), loc=(ex + s * 0.28, 0.05, 2.5), rot=(0, 90, 0), bevel=0.04, segments=2, dens=0, ao=0.45)
        gem = shapes.sphere("ear-gem", 0.13, seg=8, rings=6)
        p.add(gem, "satin", ("gold", "gold_light"), loc=(ex + s * 0.5, 0.05, 2.5), scale=(0.6, 1.0, 1.0), bevel=0, dens=0, ao=0.3)


def _forehead(p):
    ey = front_y(0.0, 3.85)
    medal = shapes.cylinder("medallion", 0.3, 0.3, 0.12, seg=22, base=False)
    p.add(medal, "satin", ("gold_dark", "gold_light"), loc=(0.0, ey - 0.02, 3.88), rot=(90, 0, 0), bevel=0.03, segments=2, dens=0, ao=0.4)
    gem = shapes.sphere("medal-gem", 0.12, seg=8, rings=6)
    p.add(gem, "glow", ("totem_red", "fire_orange"), loc=(0.0, ey - 0.1, 3.88), scale=(1.0, 0.6, 1.0), bevel=0, dens=0, ao=0.2, strength=0.7)


def _glyphs(p):
    """Row of red and teal glyph blocks across the forehead, below the moss cap."""
    red, teal = ("totem_red", "fire_orange"), ("totem_teal", "totem_teal")
    z = 3.8
    for k in range(10):
        x = -1.8 + 3.6 * k / 9
        if abs(x) < 0.45:
            continue
        slope = (front_y(x + 0.05, z) - front_y(x - 0.05, z)) / 0.1
        glyph = shapes.box("glyph", (0.27, 0.14, 0.25), base=False)
        p.add(glyph, "matte", red if k % 2 else teal, loc=(x, front_y(x, z) - 0.03, z), rot=(0, 0, math.degrees(math.atan(slope))),
              bevel=0.045, segments=2, dens=0, ao=0.5)


def build_head(seed=3):
    """Giant carved stone head: rounded skull, heavy brows, glowing eyes, arched mouth, moss cap."""
    p = Prop(WORLD, "head", seed)
    off = (p.rng.uniform(-30, 30), p.rng.uniform(-30, 30), p.rng.uniform(-30, 30))
    _skull(p, off)
    _cap(p)
    _eyes(p)
    _nose(p)
    _mouth(p)
    _ears(p)
    _forehead(p)
    _glyphs(p)
    return p.build()
