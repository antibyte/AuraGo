"""Moon station masts: comm tower, neon post and small antenna (MIT License).

All three are tall, slim props with soft chunky joints, white and orange
aviation striping and neon accents. The tower's blinking tip uses the flicker
variant of the 1.6 emissive material; the other lights stay steady.
"""

import math

from mathutils import Vector

from . import shapes
from .prop import Prop

WORLD = "moon"
BLINK = dict(strength=1.6, flicker=0.6)


def _capsule(name, r, length, seg=8):
    """Rounded neon tube along Z (base at z = 0)."""
    prof = [(0.0, 0.0)]
    for k in range(1, 4):
        a = math.pi / 2 * k / 3
        prof.append((r * math.sin(a), r - r * math.cos(a)))
    for k in range(1, 3):
        a = math.pi / 2 * k / 3
        prof.append((r * math.cos(a), length - r + r * math.sin(a)))
    prof.append((0.0, length))
    return shapes.lathe(name, prof, seg=seg)


def _small_dish(name, r, depth, seg=16):
    """Shallow parabolic shell (body of revolution about Z, opening towards +Z)."""
    prof = [(0.0, -0.03)] + [(r * f, depth * f * f - 0.03) for f in (0.3, 0.6, 0.85, 1.0)] + [(r, depth)]         + [(r * f, depth * f * f) for f in (0.85, 0.6, 0.3, 0.0)]
    return shapes.lathe(name, prof, seg=seg, caps=False)


def _ring_band(r, z, half, lift=0.03, seg=24):
    return shapes.lathe("band", [(r, z - half), (r + lift, z - half * 0.7), (r + lift, z + half * 0.7), (r, z + half)], seg=seg,
                        caps=False)


# --------------------------------------------------------------------------
# Comm tower
# --------------------------------------------------------------------------

def build_tower(seed=6):
    """Striped lattice tower on a concrete pad with a platform, sector panels, a small dish and a blinking tip."""
    p = Prop(WORLD, "tower", seed)
    levels = [0.14, 0.95, 1.65, 2.3, 2.95]
    r_base, r_top = 0.66, 0.22

    def r_of(z):
        return r_base - (r_base - r_top) * (z - levels[0]) / (levels[-1] - levels[0])

    def node(i, k):
        a = math.radians(90 + 120 * k)
        r = r_of(levels[i])
        return Vector((math.cos(a) * r, math.sin(a) * r, levels[i]))

    pad = shapes.cylinder("pad", 0.9, 0.84, 0.14, seg=20)
    p.add(pad, "satin", ("panel_dark", "panel_grey"), bevel=0.03, segments=2, dens=0.3, noise_amp=0.04)
    p.add(_ring_band(0.86, 0.1, 0.03, seg=20), "satin", "orange", bevel=0, dens=0, ao=0.4)
    for k in range(3):
        a = math.radians(90 + 120 * k)
        plinth = shapes.cylinder("plinth", 0.14, 0.12, 0.12, seg=10)
        p.add(plinth, "satin", ("panel_dark", "panel_grey"), loc=(math.cos(a) * r_of(levels[0]), math.sin(a) * r_of(levels[0]), 0.14),
              bevel=0.02, segments=2, dens=0)
    # legs: one tube per band so the aviation stripes stay crisp
    for i in range(len(levels) - 1):
        for k in range(3):
            a, b = node(i, k), node(i + 1, k)
            a.z += 0.1 if i == 0 else 0.0
            leg = shapes.tube("leg", [a, b], radius=[0.065 - 0.012 * i, 0.065 - 0.012 * (i + 1)], seg=6)
            p.add(leg, "satin", "orange" if i % 2 == 0 else ("panel_grey", "panel_white"), bevel=0, dens=0, ao=0.45)
    # triangle rings and zig-zag diagonals
    for i in range(1, len(levels)):
        for k in range(3):
            ring = shapes.tube("ring", [node(i, k), node(i, (k + 1) % 3)], radius=0.026, seg=5)
            p.add(ring, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.4)
    for i in range(len(levels) - 1):
        for k in range(3):
            a, b = (node(i, k), node(i + 1, (k + 1) % 3)) if i % 2 == 0 else (node(i, (k + 1) % 3), node(i + 1, k))
            diag = shapes.tube("diag", [a, b], radius=0.02, seg=5)
            p.add(diag, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.4)
    for i in range(1, len(levels)):
        for k in range(3):
            joint = shapes.sphere("joint", 0.062, seg=8, rings=5)
            p.add(joint, "satin", ("panel_grey", "panel_white"), loc=node(i, k), bevel=0, dens=0, ao=0.4)
    # equipment cabinet with status light
    cab = shapes.box("cabinet", (0.5, 0.34, 0.5), base=True)
    p.add(cab, "satin", ("panel_grey", "panel_white"), loc=(0.42, -0.5, 0.14), rot=(0, 0, -30), bevel=0.04, segments=2, dens=0.3)
    cab_stripe = shapes.box("cabinet-stripe", (0.52, 0.36, 0.07), base=True)
    p.add(cab_stripe, "satin", "orange", loc=(0.42, -0.5, 0.45), rot=(0, 0, -30), bevel=0.012, segments=1, dens=0.4)
    to_world = shapes.matrix(rot=(0, 0, -30))
    q = to_world @ Vector((0.12, -0.18, 0.0))
    led = shapes.sphere("cabinet-led", 0.033, seg=8, rings=5)
    p.add(led, "emissive", ("cyan", "cyan"), loc=(0.42 + q.x, -0.5 + q.y, 0.42), bevel=0, dens=0, ao=0.0, **BLINK)
    # work platform with rail
    plat = shapes.cylinder("platform", 0.5, 0.5, 0.06, seg=18)
    p.add(plat, "satin", ("panel_grey", "panel_white"), loc=(0, 0, levels[3]), bevel=0.012, segments=2, dens=0.3)
    p.add(_ring_band(0.5, levels[3] + 0.03, 0.03, seg=18), "satin", "orange", bevel=0, dens=0, ao=0.4)
    rail = shapes.torus("rail", 0.47, 0.014, seg=24, rseg=5)
    p.add(rail, "metal", ("metal_dark", "metal"), loc=(0, 0, levels[3] + 0.34), bevel=0, dens=0, ao=0.4)
    for k in range(6):
        a = math.tau * k / 6 + math.radians(30)
        post = shapes.tube("rail-post", [(math.cos(a) * 0.47, math.sin(a) * 0.47, levels[3] + 0.04),
                                         (math.cos(a) * 0.47, math.sin(a) * 0.47, levels[3] + 0.34)], radius=0.014, seg=5)
        p.add(post, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.4)
    for k in range(3):
        a = math.radians(90 + 120 * k + 60)
        lamp = shapes.sphere("rail-lamp", 0.04, seg=8, rings=5)
        p.add(lamp, "emissive", ("cyan", "window"), loc=(math.cos(a) * 0.47, math.sin(a) * 0.47, levels[3] + 0.38), bevel=0, dens=0,
              ao=0.0, **BLINK)
    # small dish on a bracket below the platform
    phi = 90 + 60
    out = Vector((math.cos(math.radians(phi)), math.sin(math.radians(phi)), 0.0))
    dish = _small_dish("tower-dish", 0.26, 0.1)
    dpos = out * (r_of(1.65) + 0.34) + Vector((0, 0, 1.65))
    p.add(dish, "satin", ("panel_grey", "panel_white"), loc=dpos, rot=(90, 0, phi + 90), bevel=0, dens=0, ao=0.5, smooth_angle=70)
    drim = shapes.torus("dish-rim", 0.26, 0.022, seg=16, rseg=6)
    shapes.transform(drim, loc=(0, 0, 0.1))
    p.add(drim, "satin", "orange", loc=dpos, rot=(90, 0, phi + 90), bevel=0, dens=0, ao=0.4)
    bracket = shapes.tube("dish-bracket", [node(2, 0).lerp(node(2, 1), 0.5), dpos - out * 0.05], radius=0.03, seg=6)
    p.add(bracket, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.4)
    # head frame, sector panels, mast, cross arms and the blinking tip
    head = shapes.cylinder("head", 0.28, 0.24, 0.14, seg=14)
    p.add(head, "satin", ("panel_grey", "panel_white"), loc=(0, 0, levels[4]), bevel=0.02, segments=2, dens=0.3)
    for k in range(3):
        a = math.radians(90 + 120 * k + 60)
        panel = shapes.box("sector", (0.18, 0.05, 0.56), base=False)
        p.add(panel, "satin", ("panel_white", "panel_white"), loc=(math.cos(a) * 0.3, math.sin(a) * 0.3, levels[4] + 0.38),
              rot=(0, 0, math.degrees(a) - 90), bevel=0.014, segments=2, dens=0.4, ao=0.5)
        cap = shapes.box("sector-cap", (0.19, 0.055, 0.07), base=False)
        p.add(cap, "satin", "orange", loc=(math.cos(a) * 0.3, math.sin(a) * 0.3, levels[4] + 0.68), rot=(0, 0, math.degrees(a) - 90),
              bevel=0.01, segments=1, dens=0.4)
    mast = shapes.tube("mast", [(0, 0, levels[4] + 0.1), (0, 0, 3.72)], radius=[0.04, 0.022], seg=8)
    p.add(mast, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.4)
    for z, ln in ((3.35, 0.5), (3.55, 0.34)):
        arm = shapes.tube("arm", [(-ln / 2, 0, z), (ln / 2, 0, z)], radius=0.015, seg=5)
        p.add(arm, "metal", ("metal_dark", "metal"), loc=(0, 0, 0), rot=(0, 0, 25 if z < 3.5 else 25 + 90), bevel=0, dens=0, ao=0.4)
    tip = shapes.sphere("tip", 0.095, seg=12, rings=8)
    p.add(tip, "emissive", ("magenta", "magenta"), loc=(0, 0, 3.8), bevel=0, dens=0, ao=0.0, **BLINK)
    return p.build()


# --------------------------------------------------------------------------
# Neon post
# --------------------------------------------------------------------------

def build_neonpost(seed=1):
    """Slim dark post with three cyan neon tubes and a floating magenta halo ring."""
    p = Prop(WORLD, "neonpost", seed)
    foot = shapes.lathe("foot", [(0.0, 0.0), (0.2, 0.0), (0.2, 0.05), (0.16, 0.11), (0.1, 0.14), (0.0, 0.14)], seg=14)
    p.add(foot, "satin", ("panel_dark", "panel_blue"), bevel=0, dens=0.3, smooth_angle=60, ao=0.6)
    glow = shapes.torus("foot-glow", 0.145, 0.016, seg=24, rseg=6)
    p.add(glow, "emissive", ("cyan_dark", "cyan"), loc=(0, 0, 0.105), bevel=0, dens=0, ao=0.0, strength=2.5)
    post = shapes.cylinder("post", 0.068, 0.05, 1.12, seg=10)
    p.add(post, "satin", ("panel_dark", "panel_blue"), loc=(0, 0, 0.1), bevel=0, dens=0.5, ao=0.5)
    for z in (0.2, 0.86):
        collar = shapes.cylinder("collar", 0.105, 0.105, 0.06, seg=10)
        p.add(collar, "satin", ("panel_blue", "panel_grey"), loc=(0, 0, z), bevel=0.012, segments=2, dens=0)
    for k in range(3):
        a = math.radians(90 + k * 120)
        tube = _capsule("neon-tube", 0.03, 0.64)
        p.add(tube, "emissive", ("cyan_dark", "cyan"), loc=(math.cos(a) * 0.105, math.sin(a) * 0.105, 0.24), bevel=0, dens=0, ao=0.0,
              strength=2.5)
    top = shapes.cylinder("top", 0.06, 0.08, 0.07, seg=10)
    p.add(top, "satin", ("panel_blue", "panel_grey"), loc=(0, 0, 1.18), bevel=0.012, segments=2, dens=0)
    for k in range(3):
        a = math.radians(30 + k * 120)
        strut = shapes.tube("halo-strut", [(math.cos(a) * 0.05, math.sin(a) * 0.05, 1.22), (math.cos(a) * 0.16, math.sin(a) * 0.16, 1.3)],
                            radius=0.012, seg=5)
        p.add(strut, "satin", ("panel_blue", "panel_grey"), bevel=0, dens=0, ao=0.4)
    halo = shapes.torus("halo", 0.17, 0.026, seg=28, rseg=8)
    p.add(halo, "emissive", ("magenta_dark", "magenta"), loc=(0, 0, 1.3), bevel=0, dens=0, ao=0.0, strength=2.5)
    return p.build()


# --------------------------------------------------------------------------
# Small antenna
# --------------------------------------------------------------------------

def build_antenna(seed=7):
    """Guyed mast with a Yagi boom, a cross dipole, a small dish and a magenta tip."""
    p = Prop(WORLD, "antenna", seed)
    plate = shapes.cylinder("plate", 0.34, 0.3, 0.07, seg=14)
    p.add(plate, "satin", ("panel_dark", "panel_grey"), bevel=0.02, segments=2, dens=0.3)
    housing = shapes.cylinder("housing", 0.15, 0.12, 0.32, seg=12)
    p.add(housing, "satin", ("panel_grey", "panel_white"), loc=(0, 0, 0.07), bevel=0.02, segments=2, dens=0.3)
    p.add(_ring_band(0.14, 0.2, 0.035, seg=12), "satin", "orange", bevel=0, dens=0, ao=0.4)
    mast = shapes.tube("mast", [(0, 0, 0.35), (0, 0, 1.2), (0, 0, 2.2)], radius=[0.06, 0.048, 0.034], seg=8)
    p.add(mast, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.45)
    for z in (0.85, 1.65):
        coll = shapes.cylinder("mast-collar", 0.06, 0.06, 0.07, seg=8)
        p.add(coll, "satin", "orange", loc=(0, 0, z), bevel=0.01, segments=1, dens=0)
    for k in range(3):
        a = math.radians(30 + k * 120)
        c, s = math.cos(a), math.sin(a)
        wire = shapes.tube("guy", [(0, 0, 1.7), (c * 0.5, s * 0.5, 0.88), (c * 1.0, s * 1.0, 0.07)], radius=0.016, seg=5)
        p.add(wire, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.4)
        anchor = shapes.cylinder("anchor", 0.07, 0.055, 0.06, seg=8)
        p.add(anchor, "satin", ("panel_dark", "panel_grey"), loc=(c * 1.0, s * 1.0, 0.0), bevel=0.01, segments=1, dens=0)
    # Yagi: boom plus shrinking elements
    boom = shapes.tube("boom", [(-0.5, 0, 1.95), (0.5, 0, 1.95)], radius=0.026, seg=6)
    p.add(boom, "metal", ("metal_dark", "metal"), rot=(0, 0, 20), bevel=0, dens=0, ao=0.4)
    for k, (x, ln) in enumerate(((-0.4, 0.52), (-0.2, 0.46), (0.0, 0.4), (0.22, 0.34), (0.44, 0.28))):
        el = shapes.tube("element", [(x, -ln / 2, 1.95), (x, ln / 2, 1.95)], radius=0.017, seg=5)
        p.add(el, "metal", ("metal_dark", "metal"), rot=(0, 0, 20), bevel=0, dens=0, ao=0.4)
    # cross dipole
    for yaw in (0, 90):
        dip = shapes.tube("dipole", [(-0.3, 0, 1.45), (0.3, 0, 1.45)], radius=0.02, seg=5)
        p.add(dip, "metal", ("metal_dark", "metal"), rot=(0, 0, yaw + 55), bevel=0, dens=0, ao=0.4)
    cup = _small_dish("cup", 0.23, 0.09)
    p.add(cup, "satin", ("panel_grey", "panel_white"), loc=(-0.1, -0.12, 1.2), rot=(65, 0, 0), bevel=0, dens=0, ao=0.5, smooth_angle=70)
    cup_rim = shapes.torus("cup-rim", 0.23, 0.014, seg=16, rseg=5)
    shapes.transform(cup_rim, loc=(0, 0, 0.09))
    p.add(cup_rim, "satin", "orange", loc=(-0.1, -0.12, 1.2), rot=(65, 0, 0), bevel=0, dens=0, ao=0.4)
    stub = shapes.tube("cup-stub", [(0, 0, 1.2), (-0.1, -0.12, 1.2)], radius=0.018, seg=5)
    p.add(stub, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.4)
    tip = shapes.sphere("tip", 0.07, seg=10, rings=6)
    p.add(tip, "emissive", ("magenta", "magenta"), loc=(0, 0, 2.27), bevel=0, dens=0, ao=0.0, strength=2.5)
    return p.build()
