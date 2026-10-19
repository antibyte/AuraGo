"""Jungle temple props (MIT License).

Each builder returns one joined mesh object named ``prop-temple-<name>`` with
its base at z=0 and its footprint centred on the origin. ``PROPS`` maps the
short name to its builder; the hero head lives in ``head_temple``.
"""

import math

from . import shapes
from .head_temple import build_head
from .palette import mix, scale
from .prop import Prop

WORLD = "temple"


def _dark(p, tone, k=0.55):
    c = p.pal.c(tone)
    return scale(c, k), c


# --------------------------------------------------------------------------
# Totem
# --------------------------------------------------------------------------

def build_totem(seed=1):
    """Carved three-tier totem pole: face, eagle, jaguar and a winged crown."""
    p = Prop(WORLD, "totem", seed)
    rng = p.rng
    base = shapes.box("totem-base", (1.3, 1.3, 0.24), base=True)
    p.add(base, "matte", "stone_deep", bevel=0.06, segments=2, dens=0.3, noise_amp=0.05)
    segs = [(0.24, 0.9, 0.98, ("stone_dark", "stone_light")),
            (1.12, 0.82, 0.88, _dark(p, "totem_teal", 0.5)),
            (2.0, 0.74, 0.86, _dark(p, "totem_red", 0.55))]
    for z, w, h, grad in segs:
        seg = shapes.box("totem-seg", (w, w, h), base=True)
        p.add(seg, "matte", grad, loc=(0, 0, z), rot=(0, 0, rng.uniform(-2.5, 2.5)), bevel=0.09, segments=3, dens=0.3,
              noise_amp=0.05, jitter=0.02)
    # face (tier 1)
    for s in (-1, 1):
        eye = shapes.cylinder("t-eye", 0.085, 0.085, 0.05, seg=12, base=False)
        p.add(eye, "satin", ("gold_dark", "gold_light"), loc=(s * 0.2, -0.46, 0.84), rot=(90, 0, 0), bevel=0, dens=0, ao=0.4)
        brow = shapes.box("t-brow", (0.34, 0.1, 0.1), base=False)
        p.add(brow, "matte", ("stone_deep", "stone"), loc=(s * 0.2, -0.46, 0.98), rot=(0, -s * 14, 0), bevel=0.025, segments=2, dens=0)
    nose = shapes.box("t-nose", (0.16, 0.2, 0.26), base=False)
    p.add(nose, "matte", ("stone", "stone_light"), loc=(0, -0.52, 0.66), bevel=0.04, segments=2, dens=0)
    mouth = shapes.box("t-mouth", (0.52, 0.08, 0.14), base=False)
    p.add(mouth, "matte", ("black", "stone_deep"), loc=(0, -0.45, 0.43), bevel=0.02, segments=1, dens=0, ao=0.9)
    for s in (-1, 1):
        tooth = shapes.box("t-tooth", (0.1, 0.06, 0.12), base=True)
        p.add(tooth, "satin", ("stone_light", "gold_light"), loc=(s * 0.14, -0.48, 0.37), bevel=0.015, segments=1, dens=0)
    # eagle (tier 2)
    beak = shapes.box("t-beak", (0.2, 0.46, 0.22), base=False)
    p.add(beak, "satin", ("gold_dark", "gold_light"), loc=(0, -0.62, 1.55), rot=(24, 0, 0), bevel=0.05, segments=2, dens=0)
    for s in (-1, 1):
        eye = shapes.sphere("t-eagle-eye", 0.065, seg=10, rings=6)
        p.add(eye, "glow", ("gold_dark", "gold_light"), loc=(s * 0.22, -0.42, 1.68), bevel=0, dens=0, ao=0.3, strength=0.7)
        wing = shapes.box("t-wing", (0.12, 0.5, 0.6), base=False)
        p.add(wing, "matte", _dark(p, "totem_teal", 0.7), loc=(s * 0.46, 0.0, 1.55), rot=(0, 0, s * 10), bevel=0.04, segments=2, dens=0.3)
    # jaguar (tier 3)
    for s in (-1, 1):
        ear = shapes.cone("t-ear", 0.11, 0.2, seg=10)
        p.add(ear, "matte", _dark(p, "totem_red", 0.6), loc=(s * 0.26, 0.0, 2.84), bevel=0, dens=0)
        eye = shapes.sphere("t-cat-eye", 0.07, seg=10, rings=6)
        p.add(eye, "glow", ("gold_dark", "gold_light"), loc=(s * 0.2, -0.4, 2.52), scale=(1.2, 0.6, 0.9), bevel=0, dens=0, ao=0.3,
              strength=0.7)
        fang = shapes.cone("t-fang", 0.045, 0.16, seg=8)
        p.add(fang, "satin", ("stone_light", "gold_light"), loc=(s * 0.14, -0.43, 2.22), rot=(180, 0, 0), bevel=0, dens=0)
    snout = shapes.box("t-snout", (0.3, 0.22, 0.2), base=False)
    p.add(snout, "matte", ("black", "stone_deep"), loc=(0, -0.42, 2.34), bevel=0.04, segments=2, dens=0, ao=0.7)
    # crown with spread wings
    crown = shapes.box("t-crown", (1.7, 0.34, 0.2), base=True)
    p.add(crown, "matte", _dark(p, "totem_teal", 0.6), loc=(0, 0, 2.86), bevel=0.07, segments=3, dens=0.3)
    for s in (-1, 1):
        tip = shapes.box("t-tip", (0.34, 0.3, 0.3), base=True)
        p.add(tip, "matte", _dark(p, "totem_red", 0.6), loc=(s * 0.74, 0, 2.84), rot=(0, 0, s * 18), bevel=0.06, segments=2, dens=0)
    ball = shapes.sphere("t-finial", 0.13, seg=12, rings=8)
    p.add(ball, "satin", ("gold_dark", "gold_light"), loc=(0, 0, 3.17), bevel=0, dens=0, ao=0.35)
    return p.build()


# --------------------------------------------------------------------------
# Torch
# --------------------------------------------------------------------------

def build_torch(seed=1):
    """Stone torch pedestal with a bowl and an emissive flame."""
    p = Prop(WORLD, "torch", seed)
    low = shapes.box("torch-base", (0.66, 0.66, 0.2), base=True)
    p.add(low, "matte", ("stone_dark", "stone"), bevel=0.05, segments=2, dens=0.3, noise_amp=0.05)
    col = shapes.box("torch-column", (0.44, 0.44, 0.62), base=True)
    p.add(col, "matte", ("stone_dark", "stone_light"), loc=(0, 0, 0.18), rot=(0, 0, 8), bevel=0.05, segments=2, dens=0.3, noise_amp=0.05)
    ring = shapes.box("torch-collar", (0.56, 0.56, 0.1), base=True)
    p.add(ring, "matte", ("stone", "stone_light"), loc=(0, 0, 0.76), rot=(0, 0, 8), bevel=0.03, segments=2, dens=0.3)
    bowl = shapes.lathe("torch-bowl", [(0.0, 0.0), (0.16, 0.0), (0.27, 0.08), (0.33, 0.22), (0.29, 0.22), (0.24, 0.11), (0.0, 0.07)],
                        seg=18)
    p.add(bowl, "matte", ("stone_deep", "stone_light"), loc=(0, 0, 0.84), bevel=0.012, segments=2, dens=0.3)
    outer = shapes.flame("flame", 0.2, 0.7)
    p.add(outer, "emissive", ("fire_red", "fire_orange"), loc=(0, 0, 1.0), scale=(1, 1, 1), bevel=0, dens=0, ao=0.0, strength=1.0,
          smooth_angle=80)
    inner = shapes.flame("flame", 0.11, 0.5)
    p.add(inner, "emissive", ("fire_orange", "fire_yellow"), loc=(0.0, 0.0, 1.0), bevel=0, dens=0, ao=0.0, strength=1.3)
    for k, (dx, dy, h) in enumerate(((0.16, 0.04, 0.42), (-0.12, -0.1, 0.34))):
        fl = shapes.flame("flame", 0.09, h)
        p.add(fl, "emissive", ("fire_red", "fire_yellow"), loc=(dx, dy, 1.0), rot=(dy * 40, -dx * 40, 0), bevel=0, dens=0, ao=0.0,
              strength=1.1)
    for dx, dy, dz in ((0.05, 0.1, 1.78), (-0.1, -0.02, 1.95), (0.12, -0.08, 2.1)):
        spark = shapes.ico_sphere("spark", 0.018, 1)
        p.add(spark, "emissive", ("fire_yellow", "fire_yellow"), loc=(dx, dy, dz), bevel=0, dens=0, ao=0.0, strength=1.6)
    return p.build()


# --------------------------------------------------------------------------
# Plants: liana, fern, bush
# --------------------------------------------------------------------------

def build_liana(seed=1, length=2.4):
    """Hanging vine with leaves; the top is at ``z=length`` and the tip near z=0."""
    p = Prop(WORLD, "liana", seed)
    rng = p.rng
    n = 16
    sway_x, sway_y = rng.uniform(-0.28, 0.28), rng.uniform(-0.2, 0.2)
    pts, radii = [], []
    for i in range(n + 1):
        t = i / n
        pts.append((sway_x * math.sin(t * math.pi * 1.2) * t, sway_y * math.sin(t * math.pi * 1.7) * t, length * (1 - t * 0.96)))
        radii.append(0.035 * (1 - 0.55 * t))
    stem = shapes.tube("liana-stem", pts, radius=radii, seg=6)
    p.add(stem, "matte", ("jungle_dark", "moss"), bevel=0, dens=0, ao=0.5, mode="z")
    for i in range(2, n + 1, 1):
        t = i / n
        base = pts[i]
        for side in (-1, 1):
            if rng.random() < 0.25:
                continue
            a = rng.uniform(0, math.tau)
            ln = rng.uniform(0.2, 0.34) * (1 - 0.3 * t)
            tip = (base[0] + math.cos(a) * ln * 0.8, base[1] + math.sin(a) * ln * 0.8, base[2] - ln * 0.55)
            spine = [(base[0] + (tip[0] - base[0]) * k / 4, base[1] + (tip[1] - base[1]) * k / 4,
                      base[2] + (tip[2] - base[2]) * k / 4 + 0.03 * math.sin(math.pi * k / 4)) for k in range(5)]
            widths = [0.02 + 0.075 * math.sin(math.pi * (k / 4) ** 0.8) for k in range(5)]
            leaf = shapes.ribbon("leaf", spine, widths, normal_hint=(math.cos(a + 1.6), math.sin(a + 1.6), 0.3), fold=0.3, across=2)
            p.add(leaf, "matte", "leaf", solidify=0.006, bevel=0, dens=0, ao=0.5, jitter=0.05, mode="z")
    return p.build()


def build_fern(seed=1, fronds=11, size=1.0):
    p = Prop(WORLD, "fern", seed)
    rng = p.rng
    for k in range(fronds):
        a = k * math.tau / fronds + rng.uniform(-0.25, 0.25)
        length = size * rng.uniform(0.8, 1.25)
        spine, widths = [], []
        steps = 14
        for i in range(steps + 1):
            s = i / steps
            reach = length * s
            z = size * 0.55 * math.sin(math.pi * min(1.0, s * 0.75)) * (1 - 0.35 * s) - 0.22 * length * s * s + 0.04
            spine.append((math.cos(a) * reach, math.sin(a) * reach, max(0.02, z)))
            w = 0.14 * math.sin(math.pi * (0.06 + 0.94 * s)) ** 0.5
            widths.append(w * (1.0 if i % 2 == 0 else 0.7))
        frond = shapes.ribbon("fern-frond", spine, widths, normal_hint=(0, 0, 1), fold=0.4, across=2)
        p.add(frond, "matte", "leaf", solidify=0.008, bevel=0, dens=0, ao=0.55, mode="radial", center=(0, 0), jitter=0.05)
    return p.build()


def build_bush(seed=1, size=1.0):
    """Leafy bush of squashed blobs with a few blossoms."""
    p = Prop(WORLD, "bush", seed)
    rng = p.rng
    for k in range(6):
        a = rng.uniform(0, math.tau)
        d = rng.uniform(0.0, 0.42) * size
        r = rng.uniform(0.3, 0.52) * size
        blob = shapes.blob("bush-blob", r, rng, subdiv=2, amp=0.3, freq=1.8, squash=(1.0, 1.0, 0.85), relax=1)
        p.add(blob, "matte", "jungle", loc=(math.cos(a) * d, math.sin(a) * d, r * 0.78), bevel=0, dens=0, ao=0.75, smooth_angle=80,
              noise_amp=0.12, jitter=0.04)
    for k in range(5):
        a = rng.uniform(0, math.tau)
        d = rng.uniform(0.2, 0.62) * size
        bl = shapes.ico_sphere("bloom", 0.055 * size, 1)
        p.add(bl, "satin", ("pink", "bloom") if k % 2 else ("bloom", "bloom"),
              loc=(math.cos(a) * d, math.sin(a) * d, rng.uniform(0.5, 0.78) * size), bevel=0, dens=0, ao=0.3)
    return p.build()


# --------------------------------------------------------------------------
# Waterfall
# --------------------------------------------------------------------------

def _curtain(p, rng, x, y, z_top, drop, width, bulge, steps=7, across=10):
    """Continuous water curtain over a lip: streaked teal, foam at the top and the bottom."""
    spine, widths = [], []
    for i in range(steps + 1):
        s = i / steps
        spine.append((x, y - bulge * (1 - (1 - s) ** 2.2), z_top - drop * s ** 1.15))
        widths.append(width / 2)
    sheet = shapes.ribbon("curtain", spine, widths, normal_hint=(0, 1, 0), fold=-0.05, across=across)
    stops = [(0.0, p.pal.c("foam")), (0.18, p.pal.c("water_white")), (0.55, p.pal.c("water_light")), (1.0, p.pal.c("water"))]
    seed = rng.uniform(0, 6)

    def streaks(px, py, pz):
        return 0.86 + 0.14 * math.sin(px * 31.0 + seed + pz * 1.3)

    p.add(sheet, "glow", "water", stops=stops, solidify=0.05, bevel=0, dens=0, ao=0.25, mode="z", strength=0.3, pattern=streaks,
          jitter=0.0)


def build_waterfall(seed=2):
    """Stepped sandstone cascade: three terraces with water curtains, pool and foam (front faces -Y)."""
    p = Prop(WORLD, "waterfall", seed)
    rng = p.rng
    stops = [(0.0, p.pal.c("stone_deep")), (0.55, p.pal.c("stone_dark")), (0.86, p.pal.c("stone")),
             (0.93, p.pal.c("moss")), (1.0, p.pal.c("moss_light"))]
    # terraces: (width, depth, height, y offset)
    tiers = [(3.6, 1.9, 0.9, 0.0), (3.0, 1.7, 0.9, 0.75), (2.4, 1.5, 0.9, 1.45)]
    z = 0.0
    tops = []
    for w, d, h, yo in tiers:
        block = shapes.rounded_box("terrace", (w, d, h), bevel=0.1, segments=2, base=True, dens=0.9)
        p.add(block, "matte", "stone", stops=stops, span=(0.0, 2.8), loc=(0, yo, z), rot=(0, 0, rng.uniform(-1.5, 1.5)), finish=False,
              noise_amp=0.06, ao=0.7)
        tops.append((z + h, yo - d / 2, w, d, yo))
        z += h
    cliff = shapes.rounded_box("cliff", (3.0, 1.0, 3.9), bevel=0.16, segments=2, base=True, dens=0.9)
    p.add(cliff, "matte", "stone", stops=stops, span=(0.0, 3.9), loc=(0, 2.35, 0.0), finish=False, noise_amp=0.07, ao=0.7)
    # water on each terrace top and curtains over the lips
    for k, (zt, yf, w, d, yo) in enumerate(tops):
        film = shapes.box("film", (w - 0.5, d - 0.35, 0.04), base=True)
        p.add(film, "glow", ("water", "water_light"), loc=(0, yo + 0.05, zt - 0.005), bevel=0.01, segments=1, dens=0, ao=0.2,
              strength=0.25, mode="y")
        below = tops[k - 1][0] if k else 0.0
        _curtain(p, rng, 0.0, yf + 0.05, zt - 0.01, zt - below - 0.04, w - 0.5, 0.2)
    pool = shapes.lathe("pool", [(0.0, 0.03), (1.7, 0.03), (1.85, 0.0)], seg=28, caps=False)
    p.add(pool, "glow", ("water_deep", "water"), loc=(0, -1.45, 0.0), scale=(1.15, 0.7, 1), bevel=0, dens=0, ao=0.2, strength=0.25,
          mode="radial", center=(0, -1.45))
    for k in range(7):
        a = rng.uniform(math.pi * 0.85, math.pi * 2.15)
        fo = shapes.ico_sphere("foam", rng.uniform(0.1, 0.22), 1)
        p.add(fo, "satin", ("water_white", "foam"), loc=(math.cos(a) * 0.9, -1.0 + math.sin(a) * 0.4, 0.1), scale=(1, 1, 0.6), bevel=0,
              dens=0, ao=0.2)
    for k, (x, y, r) in enumerate(((-2.2, 0.2, 0.7), (2.15, 0.1, 0.62), (-1.9, 1.5, 0.8), (1.8, 1.7, 0.7))):
        rock = shapes.blob("rock-side", r, rng, subdiv=2, amp=0.2, squash=(1.0, 0.9, 1.1), cuts=3, flat_bottom=0.45, relax=2)
        p.add(rock, "matte", "stone", stops=stops, span=(0.0, 2.8), loc=(x, y, r * 0.5), bevel=0, dens=0, ao=0.7, noise_amp=0.08,
              smooth_angle=80)
    return p.build()


# --------------------------------------------------------------------------
# Ruins
# --------------------------------------------------------------------------

def build_tile(seed=1, size=0.8):
    p = Prop(WORLD, "tile", seed)
    rng = p.rng
    slab = shapes.box("tile", (size * rng.uniform(0.92, 1.05), size * rng.uniform(0.92, 1.05), 0.08), base=True)
    p.add(slab, "matte", "tile", rot=(0, 0, rng.uniform(-4, 4)), bevel=0.02, segments=2, dens=0.3, noise_amp=0.07, jitter=0.04, ao=0.6)
    return p.build()


def build_pillar(seed=1, height=1.5):
    """Broken, mossy column stump on a square plinth."""
    p = Prop(WORLD, "pillar", seed)
    plinth = shapes.box("plinth", (1.0, 1.0, 0.22), base=True)
    p.add(plinth, "matte", ("stone_deep", "stone"), bevel=0.05, segments=2, dens=0.3, noise_amp=0.05)
    prof = [(0.0, 0.0), (0.42, 0.0)]
    n = 8
    for k in range(1, n + 1):
        t = k / n
        z = height * t
        prof.append((0.36 + 0.025 * (0.5 + 0.5 * math.cos(t * math.pi * 6)) - 0.03 * t, z))
    prof.append((0.0, height))
    col = shapes.lathe("column", prof, seg=14)
    p.add(col, "matte", "stone", stops=[(0.0, p.pal.c("stone_deep")), (0.7, p.pal.c("stone")), (0.9, p.pal.c("moss")),
                                         (1.0, p.pal.c("moss_light"))], loc=(0, 0, 0.2), bevel=0, dens=0.3, noise_amp=0.07, smooth_angle=60)
    return p.build()


def build_rock(seed=1, size=0.5, cuts=3, subdiv=3, squash=(1.0, 0.85, 0.75)):
    p = Prop(WORLD, "rock", seed)
    rock = shapes.blob("rock", size, p.rng, subdiv=subdiv, amp=0.2, freq=1.2, squash=squash, cuts=cuts, flat_bottom=0.55, relax=2)
    p.add(rock, "matte", "stone", stops=[(0.0, p.pal.c("stone_deep")), (0.7, p.pal.c("stone_dark")), (0.9, p.pal.c("moss")),
                                          (1.0, p.pal.c("moss_light"))], bevel=0, dens=0, ao=0.7, noise_amp=0.08, smooth_angle=80)
    return p.build()


def build_islet(seed=1, size=1.4, haze=0.0):
    """Small mossy rock islet with ferns, standing in the water (base at the waterline)."""
    p = Prop(WORLD, "islet", seed)
    rng = p.rng
    stops = [(0.0, p.pal.c("stone_deep")), (0.5, p.pal.c("stone_cold")), (0.8, mix(p.pal.c("stone_cold"), p.pal.c("stone"), 0.4)),
             (0.9, p.pal.c("moss_dark")), (1.0, p.pal.c("moss"))]
    top = 0.0
    for i in range(3):
        r = size * (1.0 - 0.28 * i) * rng.uniform(0.9, 1.1)
        rock = shapes.blob("islet-rock", r, rng, subdiv=3, amp=0.18, squash=(1.0, 1.0, 0.95), cuts=4, cut_depth=0.82, relax=2,
                           flat_bottom=0.35)
        z = top + r * 0.55
        p.add(rock, "matte", "stone", stops=stops, span=(-0.3, size * 1.9), loc=(rng.uniform(-0.2, 0.2) * i, rng.uniform(-0.2, 0.2) * i, z),
              bevel=0, dens=0, ao=0.7, smooth_angle=80, noise_amp=0.07)
        top = z + r * 0.55
    for k in range(4):
        a = rng.uniform(0, math.tau)
        d = rng.uniform(0.0, size * 0.35)
        bsh = shapes.blob("islet-bush", rng.uniform(0.25, 0.45), rng, subdiv=2, amp=0.3, squash=(1.0, 1.0, 0.8), relax=1)
        p.add(bsh, "matte", ("jungle_dark", "jungle_light"), loc=(math.cos(a) * d, math.sin(a) * d, top - 0.05),
              bevel=0, dens=0, ao=0.7, smooth_angle=80, noise_amp=0.1)
    return p.build()


def build_lilypad(seed=1, flower=False):
    """Floating lily pad (optionally with a pink blossom); flat on the water at z=0."""
    p = Prop(WORLD, "lilypad", seed)
    rng = p.rng
    r = rng.uniform(0.3, 0.45)
    pad = shapes.lathe("pad", [(0.0, 0.012), (r * 0.8, 0.008), (r, 0.03), (r, 0.012), (r * 0.8, 0.0), (0.0, 0.0)], seg=18,
                       caps=False, start_angle=rng.uniform(0, math.tau), sweep=math.tau * 0.92)
    p.add(pad, "matte", ("jungle_dark", "jungle_light"), bevel=0, dens=0, ao=0.3, mode="radial", center=(0, 0), smooth_angle=80, jitter=0.05)
    if flower:
        for k in range(6):
            a = k * math.tau / 6
            petal = shapes.sphere("petal", 0.06, seg=8, rings=5)
            p.add(petal, "satin", ("pink", "foam"), loc=(math.cos(a) * 0.07, math.sin(a) * 0.07, 0.07), scale=(1.0, 0.6, 1.6),
                  rot=(0, 0, math.degrees(a)), bevel=0, dens=0, ao=0.2)
        core = shapes.sphere("core", 0.035, seg=8, rings=5)
        p.add(core, "satin", ("bloom", "bloom"), loc=(0, 0, 0.06), bevel=0, dens=0, ao=0.2)
    return p.build()


PROPS = {"head": build_head, "totem": build_totem, "torch": build_torch, "liana": build_liana, "fern": build_fern,
         "bush": build_bush, "waterfall": build_waterfall, "tile": build_tile, "pillar": build_pillar, "rock": build_rock,
         "islet": build_islet, "lilypad": build_lilypad}
