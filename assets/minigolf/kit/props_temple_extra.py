"""Jungle temple vine curtain and pebbles (MIT License).

``vines``: a bark bough on two slanted poles with a curtain of hanging liana
strands, leaves and a few blossoms (strands and leaves are ``foliage``).
``pebble``: a small group of mossy pebbles for scatter decoration.
"""

import math

from . import shapes
from .prop import Prop
from .props_temple_util import stone_fn

WORLD = "temple"
BOUGH_Z = 2.3


def build_vines(seed=1, width=2.9):
    """Liana curtain hanging from a bark bough between two slanted poles (the curtain faces -Y)."""
    p = Prop(WORLD, "vines", seed)
    rng = p.rng
    half = width / 2
    for s in (-1, 1):
        pole = shapes.tube("pole", [(s * (half + 0.06), 0.0, 0.0), (s * (half + 0.02), 0.03, 1.2), (s * (half - 0.04), 0.0, BOUGH_Z)],
                           radius=[0.13, 0.1, 0.085], seg=8)
        p.add(pole, "matte", ("bark", "bark_light"), bevel=0, dens=0, ao=0.6, smooth_angle=80.0, noise_amp=0.06)
        for k in range(3):  # moss tuft at the foot
            tuft = shapes.blob("tuft", rng.uniform(0.14, 0.22), rng, subdiv=2, amp=0.3, squash=(1.0, 1.0, 0.55), relax=1)
            p.add(tuft, "matte", "moss", loc=(s * (half + 0.06) + rng.uniform(-0.2, 0.2), rng.uniform(-0.15, 0.15), 0.04), bevel=0, dens=0,
                  smooth_angle=80.0, ao=0.6, noise_amp=0.1)
    pts = [(-half + width * k / 14, 0.04 * math.sin(k * 0.9), BOUGH_Z + 0.04 * math.sin(k * 1.3 + 1.0)) for k in range(15)]
    bough = shapes.tube("bough", pts, radius=[0.1 + 0.02 * math.sin(k * 1.1) for k in range(15)], seg=8)
    p.add(bough, "matte", ("bark", "bark_light"), bevel=0, dens=0, ao=0.6, smooth_angle=80.0, noise_amp=0.06)
    n = 11
    for i in range(n):
        x = -half + 0.2 + (width - 0.4) * (i + rng.uniform(-0.25, 0.25)) / (n - 1)
        length = rng.uniform(1.0, 2.15) * (0.75 + 0.25 * math.cos(math.pi * (x / half) * 0.5))
        ph = rng.uniform(0, math.tau)
        sway = rng.uniform(0.04, 0.12)
        z_top = BOUGH_Z - 0.02
        spine, radii = [], []
        steps = 9
        for j in range(steps + 1):
            t = j / steps
            spine.append((x + sway * math.sin(ph + t * 4.0) * t, rng.uniform(-0.01, 0.01) + 0.05 * math.sin(ph + t * 3.0) * t,
                          z_top - length * t))
            radii.append(0.028 * (1.0 - 0.6 * t))
        strand = shapes.tube("strand", spine, radius=radii, seg=5)
        p.add(strand, "foliage", ("jungle_dark", "moss"), sway=0.06, bevel=0, dens=0, ao=0.5, smooth_angle=80.0, mode="z")
        for j in range(2, steps + 1, 3):
            base = spine[j]
            for side in (-1, 1):
                if rng.random() < 0.2:
                    continue
                a = math.pi / 2 + side * rng.uniform(0.5, 1.3)
                ln = rng.uniform(0.16, 0.26)
                tip = (base[0] + math.cos(a) * ln, base[1] + rng.uniform(-0.08, 0.04), base[2] - ln * rng.uniform(0.3, 0.7))
                leaf_spine = [(base[0] + (tip[0] - base[0]) * k / 4, base[1] + (tip[1] - base[1]) * k / 4,
                               base[2] + (tip[2] - base[2]) * k / 4 + 0.025 * math.sin(math.pi * k / 4)) for k in range(5)]
                widths = [0.015 + 0.06 * math.sin(math.pi * (k / 4) ** 0.8) for k in range(5)]
                leaf = shapes.ribbon("leaf", leaf_spine, widths, normal_hint=(0, -1, 0.2), fold=0.3, across=2)
                p.add(leaf, "foliage", "leaf", sway=0.06, solidify=0.006, bevel=0, dens=0, ao=0.5, jitter=0.05, mode="z")
        if rng.random() < 0.45:  # a blossom near the tip
            tip = spine[-2]
            bl = shapes.ico_sphere("bloom", 0.05, 1)
            p.add(bl, "satin", ("pink", "bloom"), loc=(tip[0], tip[1] - 0.03, tip[2]), bevel=0, dens=0, ao=0.3)
    return p.build()


def build_pebble(seed=1):
    """A little group of five pebbles: two bigger, three small, some with a moss cap."""
    p = Prop(WORLD, "pebble", seed)
    rng = p.rng
    pal = p.pal
    off = (rng.uniform(-40, 40), rng.uniform(-40, 40), rng.uniform(-40, 40))
    sites = ((0.0, 0.0, 0.19, 3), (0.36, 0.14, 0.14, 3), (-0.3, 0.22, 0.11, 2), (0.1, -0.34, 0.09, 2), (0.42, -0.22, 0.07, 2))
    for k, (x, y, r, sub) in enumerate(sites):
        rock = shapes.blob("pebble", r, rng, subdiv=sub, amp=0.2, squash=(1.0, 0.9, 0.7), cuts=2, flat_bottom=0.4, relax=2)
        fn = stone_fn(pal, -r * 0.4, r * 0.8, off, moss=0.7 if k == 0 else 0.0, moss_z=r * 0.8, moss_depth=r * 0.9, noise_amp=0.1)
        p.add(rock, "matte", fn=fn, loc=(x, y, r * 0.35), rot=(0, 0, rng.uniform(0, 360)), bevel=0, dens=0, ao=0.7, smooth_angle=80.0)
    return p.build()
