"""Jungle temple statues: the golden idol and the stone sun wheel (MIT License).

``idol``: a squat golden figure with a feather crown on a two-step plinth (the
figure is ``glow``). ``sunwheel``: a standing stone disc with a carved sun face
and a ring of rays; static decoration (it does not turn). Both face -Y; base at z=0,
footprint centred.
"""

import math

from . import shapes
from .palette import mix
from .prop import Prop
from .props_temple_util import shade, smoothstep, stone_fn

WORLD = "temple"


# --------------------------------------------------------------------------
# Idol
# --------------------------------------------------------------------------

def build_idol(seed=1):
    """Golden squat idol with a feather crown, glowing eyes and a belly gem on a stepped plinth."""
    p = Prop(WORLD, "idol", seed)
    rng = p.rng
    off = (rng.uniform(-40, 40), rng.uniform(-40, 40), rng.uniform(-40, 40))
    pal = p.pal
    fn = stone_fn(pal, 0.0, 0.5, off, moss=0.7, moss_z=0.38, moss_depth=0.12, noise_amp=0.08)
    for w, h, z in ((1.0, 0.2, 0.0), (0.78, 0.2, 0.2)):
        step = shapes.box("plinth", (w, w, h), base=True)
        p.add(step, "matte", fn=fn, loc=(0, 0, z), rot=(0, 0, rng.uniform(-1.5, 1.5)), bevel=0.05, segments=2, dens=0.4, ao=0.7,
              smooth_angle=50.0)
    gold = ("gold_dark", "gold_light")
    z0 = 0.4
    seat = shapes.sphere("seat", 0.4, seg=20, rings=12)
    p.add(seat, "glow", gold, loc=(0, 0, z0 + 0.13), scale=(1.15, 0.95, 0.5), bevel=0, dens=0, ao=0.5, strength=0.3, smooth_angle=80.0)
    body = shapes.lathe("body", [(0.0, 0.0), (0.26, 0.0), (0.33, 0.14), (0.34, 0.3), (0.3, 0.46), (0.22, 0.6), (0.17, 0.7), (0.0, 0.72)],
                        seg=20)
    p.add(body, "glow", gold, loc=(0, 0, z0 + 0.16), bevel=0, dens=0, ao=0.5, strength=0.3, smooth_angle=80.0, noise_amp=0.04)
    belt = shapes.torus("belt", 0.3, 0.05, seg=22, rseg=7)
    p.add(belt, "glow", ("gold_dark", "gold"), loc=(0, 0, z0 + 0.34), bevel=0, dens=0, ao=0.4, strength=0.3, smooth_angle=80.0)
    gem = shapes.sphere("belly-gem", 0.075, seg=10, rings=7)
    p.add(gem, "glow", ("totem_red", "fire_orange"), loc=(0, -0.3, z0 + 0.34), scale=(1.0, 0.6, 1.0), bevel=0, dens=0, ao=0.2,
          strength=0.7, smooth_angle=80.0)
    for s in (-1, 1):  # arms resting on the knees
        arm = shapes.tube("arm", [(s * 0.27, 0.0, z0 + 0.74), (s * 0.4, -0.14, z0 + 0.6), (s * 0.43, -0.3, z0 + 0.4),
                                  (s * 0.34, -0.4, z0 + 0.26)], radius=[0.075, 0.07, 0.065, 0.06], seg=8)
        p.add(arm, "glow", gold, bevel=0, dens=0, ao=0.5, strength=0.3, smooth_angle=80.0)
        hand = shapes.sphere("hand", 0.09, seg=10, rings=7)
        p.add(hand, "glow", gold, loc=(s * 0.34, -0.42, z0 + 0.26), scale=(1.0, 1.1, 0.8), bevel=0, dens=0, ao=0.5, strength=0.3,
              smooth_angle=80.0)
    zh = z0 + 1.02
    head = shapes.sphere("head", 0.27, seg=20, rings=12)
    p.add(head, "glow", gold, loc=(0, 0, zh), scale=(1.0, 0.95, 1.05), bevel=0, dens=0, ao=0.45, strength=0.3, smooth_angle=80.0)
    for s in (-1, 1):
        eye = shapes.sphere("eye", 0.055, seg=10, rings=7)
        p.add(eye, "glow", ("gold", "eye_glow"), loc=(s * 0.105, -0.245, zh + 0.04), scale=(1.0, 0.6, 1.1), bevel=0, dens=0, ao=0.2,
              strength=0.7, smooth_angle=80.0)
        brow = shapes.tube("brow", [(s * 0.04, -0.25, zh + 0.11), (s * 0.12, -0.255, zh + 0.115), (s * 0.19, -0.22, zh + 0.09)],
                           radius=0.025, seg=6)
        p.add(brow, "glow", ("gold_dark", "gold_dark"), bevel=0, dens=0, ao=0.4, strength=0.3, smooth_angle=80.0)
        spool = shapes.cylinder("spool", 0.075, 0.075, 0.07, seg=12, base=False)
        p.add(spool, "glow", ("gold", "gold_light"), loc=(s * 0.28, 0.0, zh - 0.02), rot=(0, 90, 0), bevel=0.01, segments=1, dens=0,
              ao=0.4, strength=0.3)
    nose = shapes.sphere("nose", 0.06, seg=10, rings=7)
    p.add(nose, "glow", gold, loc=(0, -0.265, zh - 0.04), scale=(1.0, 0.8, 1.0), bevel=0, dens=0, ao=0.4, strength=0.3, smooth_angle=80.0)
    mouth = shapes.tube("mouth", [(-0.09 + 0.18 * k / 6, -0.245 + 0.025 * math.sin(math.pi * k / 6) * -1.0 + 0.0,
                                   zh - 0.12 - 0.035 * math.sin(math.pi * k / 6)) for k in range(7)], radius=0.02, seg=6)
    p.add(mouth, "matte", ("black", "stone_deep"), bevel=0, dens=0, ao=0.4, smooth_angle=80.0)
    band = shapes.torus("headband", 0.265, 0.032, seg=22, rseg=6)
    p.add(band, "matte", ("totem_red", "totem_red"), loc=(0, 0, zh + 0.12), scale=(1.0, 0.95, 1.0), bevel=0, dens=0, ao=0.4,
          smooth_angle=80.0)
    neck = shapes.torus("necklace", 0.2, 0.04, seg=20, rseg=6)
    p.add(neck, "matte", ("totem_teal", "totem_teal"), loc=(0, -0.03, z0 + 0.8), bevel=0, dens=0, ao=0.4, smooth_angle=80.0)
    for k in range(9):  # feather crown fanning out in the XZ plane
        ang = -64 + 16 * k
        long = 0.48 - 0.12 * abs(k - 4) / 4.0
        feather = shapes.cone("feather", 0.075, long, seg=8)
        a = math.radians(ang)
        p.add(feather, "glow", ("gold_dark", "gold_light") if k % 2 else ("totem_teal", "gold"),
              loc=(math.sin(a) * 0.13, 0.0, zh + 0.2 + 0.02 * abs(k - 4)), rot=(0, ang, 0), scale=(1.0, 0.45, 1.0), bevel=0, dens=0,
              ao=0.4, strength=0.3, smooth_angle=80.0)
    return p.build()


# --------------------------------------------------------------------------
# Sun wheel
# --------------------------------------------------------------------------

WHEEL_Z = 1.3  # wheel centre height
WHEEL_R = 0.88


def _wheel_colour(p, off):
    pal = p.pal
    base = stone_fn(pal, 0.4, 2.2, off, noise_amp=0.07, strata=0.0)
    teal, red, gold = pal.c("totem_teal"), pal.c("totem_red"), pal.c("gold_dark")

    def fn(x, y, z):
        c = base(x, y, z)
        r = math.hypot(x, z - WHEEL_Z)
        if r < WHEEL_R + 0.02:
            ring = smoothstep(abs(r - 0.7), 0.06, 0.03)
            c = mix(c, shade(teal, 0.85), 0.9 * ring)
            ring2 = smoothstep(abs(r - 0.52), 0.045, 0.02)
            c = mix(c, shade(red, 0.9), 0.85 * ring2)
            c = mix(c, shade(c, 0.78), 0.55 * smoothstep(abs(r - 0.6), 0.02, 0.0))  # groove between the bands
        return c

    return fn


def build_sunwheel(seed=1):
    """Standing stone wheel with a carved sun face and a ring of rays on a mossy base block."""
    p = Prop(WORLD, "sunwheel", seed)
    rng = p.rng
    off = (rng.uniform(-40, 40), rng.uniform(-40, 40), rng.uniform(-40, 40))
    pal = p.pal
    base_fn = stone_fn(pal, 0.0, 2.3, off, moss=0.8, moss_z=0.5, moss_depth=0.14, noise_amp=0.08)
    plinth = shapes.box("plinth", (1.9, 1.0, 0.28), base=True)
    p.add(plinth, "matte", fn=base_fn, bevel=0.06, segments=2, dens=0.5, ao=0.7, smooth_angle=50.0)
    block = shapes.box("block", (1.5, 0.7, 0.26), base=True)
    p.add(block, "matte", fn=base_fn, loc=(0, 0, 0.28), rot=(0, 0, rng.uniform(-1, 1)), bevel=0.06, segments=2, dens=0.5, ao=0.7,
          smooth_angle=50.0)
    wedge_fn = stone_fn(pal, 0.0, 2.3, off, moss=0.7, moss_z=1.22, moss_depth=0.1, noise_amp=0.08)
    for s in (-1, 1):  # wedge props holding the wheel
        wedge = shapes.box("wedge", (0.3, 0.55, 0.7), base=True)
        p.add(wedge, "matte", fn=wedge_fn, loc=(s * 0.78, 0.0, 0.52), rot=(0, s * -14, 0), bevel=0.05, segments=2, dens=0.5, ao=0.7,
              smooth_angle=50.0)
    # the wheel: lathe axis Z -> rotated so the front faces -Y
    disc = shapes.lathe("disc", [(0.0, 0.0), (0.8, 0.0), (0.88, 0.06), (0.88, 0.2), (0.8, 0.26), (0.0, 0.26)], seg=44)
    p.add(disc, "matte", fn=_wheel_colour(p, off), loc=(0, 0.13, WHEEL_Z), rot=(90, 0, 0), bevel=0, dens=0.3, ao=0.65, smooth_angle=60.0)
    for r_ring, minor in ((0.7, 0.028), (0.52, 0.024)):
        ring = shapes.torus("relief", r_ring, minor, seg=40, rseg=6)
        p.add(ring, "matte", ("stone", "stone_light"), loc=(0, -0.14, WHEEL_Z), rot=(90, 0, 0), bevel=0, dens=0, ao=0.5, smooth_angle=80.0)
    dome = shapes.lathe("sun-disc", [(0.0, 0.0), (0.4, 0.0), (0.39, 0.04), (0.33, 0.1), (0.0, 0.14)], seg=26)
    p.add(dome, "satin", ("gold_dark", "gold_light"), loc=(0, -0.13, WHEEL_Z), rot=(90, 0, 0), bevel=0, dens=0, ao=0.35, smooth_angle=70.0)
    for s in (-1, 1):  # sun face
        eye = shapes.sphere("sun-eye", 0.055, seg=10, rings=7)
        p.add(eye, "glow", ("gold", "eye_glow"), loc=(s * 0.14, -0.285, WHEEL_Z + 0.08), scale=(1.0, 0.55, 1.1), bevel=0, dens=0,
              ao=0.2, strength=0.7, smooth_angle=80.0)
        brow = shapes.tube("sun-brow", [(s * 0.06, -0.275, WHEEL_Z + 0.17), (s * 0.15, -0.285, WHEEL_Z + 0.185), (s * 0.24, -0.265, WHEEL_Z + 0.15)],
                           radius=0.022, seg=6)
        p.add(brow, "satin", ("gold_dark", "gold_dark"), bevel=0, dens=0, ao=0.4, smooth_angle=80.0)
    nose = shapes.cone("sun-nose", 0.055, 0.09, seg=8)
    p.add(nose, "satin", ("gold", "gold_light"), loc=(0, -0.27, WHEEL_Z + 0.0), rot=(90, 0, 0), bevel=0, dens=0, ao=0.4)
    mouth = shapes.tube("sun-mouth", [(-0.14 + 0.28 * k / 8, -0.27, WHEEL_Z - 0.16 - 0.05 * math.sin(math.pi * k / 8)) for k in range(9)],
                        radius=0.024, seg=6)
    p.add(mouth, "matte", ("black", "stone_deep"), bevel=0, dens=0, ao=0.4, smooth_angle=80.0)
    n = 16
    for k in range(n):  # rays: long gold ones alternate with short stone ones
        a = k * math.tau / n
        long = k % 2 == 0
        ray = shapes.cone("ray", 0.1 if long else 0.075, 0.4 if long else 0.24, seg=8)
        p.add(ray, "satin" if long else "matte", ("gold_dark", "gold_light") if long else ("stone", "stone_light"),
              loc=(math.cos(a) * (WHEEL_R - 0.04), 0.13, WHEEL_Z + math.sin(a) * (WHEEL_R - 0.04)), rot=(0, 90.0 - math.degrees(a), 0),
              scale=(1.0, 0.55, 1.0), bevel=0, dens=0, ao=0.4, smooth_angle=60.0)
    return p.build()
