"""Hero prop of the jungle temple: the giant carved stone head (MIT License).

Tiki / Olmec style head whose open mouth is the tunnel the lane runs into.
Prop space: the face looks towards -Y, the tunnel floor is z=0 (lane level)
and runs along +Y; footprint centred on the origin.
"""

import math

from . import shapes
from .palette import mix
from .prop import Prop

WORLD = "temple"

HALF_W = 2.3  # half width of the head
FRONT = -1.7  # face plane
BACK = 1.9
MOUTH_HALF = 1.2  # half width of the mouth opening
SPRING = 0.9  # height where the mouth arch starts
ARCH_TOP = SPRING + MOUTH_HALF


def _smooth(x, a, b):
    t = min(1.0, max(0.0, (x - a) / (b - a)))
    return t * t * (3 - 2 * t)


def _mouth_block(p):
    """Lower head: cheeks and jaw with the arched mouth tunnel (extruded along Y)."""
    h = 2.9
    section = [(-HALF_W + 0.3, 0.0), (-HALF_W, 1.3), (-HALF_W, h), (HALF_W, h), (HALF_W, 1.3), (HALF_W - 0.3, 0.0), (MOUTH_HALF, 0.0)]
    steps = 14
    for k in range(steps + 1):
        a = math.pi * k / steps
        section.append((MOUTH_HALF * math.cos(a), SPRING + MOUTH_HALF * math.sin(a)))
    section.append((-MOUTH_HALF, 0.0))
    rings = [[(x, y, z) for x, z in section] for y in (FRONT, BACK)]
    block = shapes.loft("mouth-block", rings, closed=True, recalc=True)
    pal = p.pal
    deep, stone_d, stone, stone_l = pal.c("stone_deep"), pal.c("stone_dark"), pal.c("stone"), pal.c("stone_light")
    moss = pal.c("moss")

    def colour(x, y, z):
        c = mix(stone_d, stone_l, min(1.0, max(0.0, z / 2.9)) ** 0.8)
        n = math.sin(x * 2.1 + z * 1.7) * math.sin(y * 1.3 + x) * 0.5 + 0.5
        c = (c[0] * (0.92 + 0.14 * n), c[1] * (0.92 + 0.14 * n), c[2] * (0.92 + 0.14 * n), 1.0)
        inside = abs(x) < MOUTH_HALF + 0.05 and z < ARCH_TOP + 0.05 and y > FRONT + 0.12
        if inside:
            c = mix(c, (0.05 * deep[0], 0.05 * deep[1], 0.06 * deep[2], 1.0), _smooth(y, FRONT + 0.1, FRONT + 1.3) * 0.97)
        elif z > 2.5:
            c = mix(c, moss, 0.35 * _smooth(z, 2.5, 2.9))
        return c

    p.add(block, "matte", fn=colour, bevel=0.42, segments=4, dens=0.32, ao=0.7)


def _upper_head(p):
    pal = p.pal
    stops = [(0.0, pal.c("stone_dark")), (0.5, pal.c("stone")), (0.82, pal.c("stone_light")),
             (0.92, pal.c("moss")), (1.0, pal.c("moss_light"))]
    upper = shapes.box("forehead", (2 * HALF_W - 0.1, BACK - FRONT - 0.2, 1.65), base=True)
    p.add(upper, "matte", "stone", stops=stops, loc=(0, (BACK + FRONT) / 2, 2.62), bevel=0.62, segments=4, dens=0.32,
          noise_amp=0.05)
    cap = shapes.sphere("helmet", 1.0, seg=20, rings=10)
    p.add(cap, "matte", "stone", stops=[(0.0, pal.c("stone_light")), (0.35, pal.c("moss")), (1.0, pal.c("moss_light"))],
          loc=(0, 0.1, 4.12), scale=(2.05, 1.65, 0.55), bevel=0, dens=0.35, noise_amp=0.08, smooth_angle=70)
    band = shapes.box("headband", (2 * HALF_W + 0.1, 0.34, 0.5), base=False)
    p.add(band, "matte", ("stone_dark", "stone"), loc=(0, FRONT - 0.04, 3.88), bevel=0.07, segments=2, dens=0.3, noise_amp=0.05)
    for k in range(7):
        x = -1.8 + k * 0.6
        glyph = shapes.box("glyph", (0.28, 0.12, 0.3), base=False)
        tone = ("totem_red", "fire_orange") if k % 2 else ("stone_deep", "stone_dark")
        p.add(glyph, "matte", tone, loc=(x, FRONT - 0.24, 3.88), bevel=0.04, segments=1, dens=0)
    medal = shapes.cylinder("medallion", 0.3, 0.3, 0.1, seg=18, base=False)
    p.add(medal, "satin", ("gold_dark", "gold_light"), loc=(0, FRONT - 0.3, 3.88), rot=(90, 0, 0), bevel=0.02, segments=2, dens=0)


def _face(p):
    for side in (-1, 1):
        x = side * 1.08
        socket = shapes.cylinder("socket", 0.56, 0.5, 0.2, seg=20, base=False)
        p.add(socket, "matte", ("stone_deep", "black"), loc=(x, FRONT - 0.02, 3.12), rot=(90, 0, 0), bevel=0.04, segments=2, dens=0)
        rim = shapes.torus("eye-rim", 0.56, 0.08, seg=16, rseg=6)
        p.add(rim, "matte", ("stone", "stone_light"), loc=(x, FRONT - 0.09, 3.12), rot=(90, 0, 0), bevel=0, dens=0, ao=0.5)
        eye = shapes.sphere("eye", 0.3, seg=12, rings=8)
        p.add(eye, "glow", ("gold_dark", "gold_light"), loc=(x, FRONT - 0.08, 3.1), scale=(1.0, 0.55, 1.0), bevel=0,
              dens=0, ao=0.3, strength=0.8)
        brow = shapes.box("brow", (1.95, 0.5, 0.44), base=False)
        p.add(brow, "matte", ("stone", "stone_light"), loc=(x * 1.02 + side * 0.04, FRONT - 0.12, 3.64), rot=(0, -side * 11, 0),
              bevel=0.12, segments=2, dens=0.6, noise_amp=0.05)
        cheek = shapes.sphere("cheek", 0.62, seg=12, rings=8)
        p.add(cheek, "matte", ("stone_dark", "stone_light"), loc=(side * 1.75, FRONT - 0.1, 2.35), scale=(1.0, 0.7, 0.85),
              bevel=0, dens=0.3, smooth_angle=70, noise_amp=0.05)
        ear = shapes.cylinder("ear", 0.68, 0.62, 0.34, seg=22, base=False)
        p.add(ear, "matte", ("stone_dark", "stone"), loc=(side * (HALF_W + 0.1), 0.05, 2.75), rot=(0, 90, 0), bevel=0.09, segments=3, dens=0.3,
              noise_amp=0.05)
        spool = shapes.cylinder("ear-spool", 0.26, 0.26, 0.5, seg=14, base=False)
        p.add(spool, "satin", ("gold_dark", "gold_light"), loc=(side * (HALF_W + 0.28), 0.05, 2.75), rot=(0, 90, 0), bevel=0.03,
              segments=2, dens=0, ao=0.45)
        nostril = shapes.sphere("nostril", 0.17, seg=10, rings=6)
        p.add(nostril, "matte", ("black", "stone_deep"), loc=(side * 0.3, FRONT - 0.62, 2.78), scale=(0.9, 0.8, 1.1), bevel=0, dens=0, ao=0.9)
    nose = shapes.box("nose", (0.92, 1.0, 1.65), base=False)
    p.add(nose, "matte", ("stone", "stone_light"), loc=(0, FRONT - 0.3, 3.0), rot=(-17, 0, 0), bevel=0.22, segments=3,
          dens=0.5, noise_amp=0.05)
    bridge = shapes.box("nose-bridge", (0.5, 0.7, 1.0), base=False)
    p.add(bridge, "matte", ("stone", "stone_light"), loc=(0, FRONT - 0.1, 3.35), rot=(-8, 0, 0), bevel=0.15, segments=2, dens=0.3)


def _mouth_details(p):
    teeth_angles = [28, 44, 60, 76, 90, 104, 120, 136, 152]
    for a_deg in teeth_angles:
        a = math.radians(a_deg)
        x, z = MOUTH_HALF * math.cos(a), SPRING + MOUTH_HALF * math.sin(a)
        phi = math.degrees(math.atan2(-math.cos(a), -math.sin(a)))
        size = 0.55 - 0.1 * abs(90 - a_deg) / 62.0
        tooth = shapes.box("tooth", (0.27, 0.3, size), base=True)
        p.add(tooth, "satin", ("stone_light", "gold_light"), loc=(x, FRONT + 0.26, z + 0.02), rot=(0, phi, 0), bevel=0.06, segments=1, dens=0)
    for side in (-1, 1):
        fang = shapes.cone("fang", 0.14, 0.62, seg=10)
        p.add(fang, "satin", ("stone_light", "stone_light"), loc=(side * (MOUTH_HALF + 0.14), FRONT + 0.2, 0.0), bevel=0, dens=0, ao=0.5)
    lip = shapes.tube("lip", [(MOUTH_HALF * math.cos(math.pi * k / 16), FRONT - 0.02, SPRING + MOUTH_HALF * math.sin(math.pi * k / 16))
                              for k in range(17)], radius=0.13, seg=8)
    p.add(lip, "matte", ("stone", "stone_light"), bevel=0, dens=0, ao=0.55)


def build_head(seed=3):
    """Giant carved stone head with a tunnel mouth, glowing eyes and a mossy crown."""
    p = Prop(WORLD, "head", seed)
    _mouth_block(p)
    _upper_head(p)
    _face(p)
    _mouth_details(p)
    rng = p.rng
    orb = shapes.sphere("tunnel-orb", 0.3, seg=14, rings=8)
    p.add(orb, "emissive", ("fire_orange", "gold"), loc=(0.0, 0.4, 0.55), bevel=0, dens=0, ao=0.0, strength=1.2)
    for k in range(8):
        a = rng.uniform(0, math.tau)
        r = rng.uniform(0.5, 1.9)
        clump = shapes.blob("moss", rng.uniform(0.28, 0.5), rng, subdiv=2, amp=0.25, squash=(1.0, 1.0, 0.6))
        p.add(clump, "matte", "moss", loc=(r * math.cos(a) * 0.9, 0.1 + r * math.sin(a) * 0.7, 4.3), bevel=0, dens=0, smooth_angle=80,
              ao=0.6, noise_amp=0.1)
    return p.build()
