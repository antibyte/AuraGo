"""Aura Drift shared props, the big crowd & hero pieces: grandstand, fan camp, gnome statue, inflatable arch (MIT, AuraGo contributors).

Props stand on y = 0, centred, front toward +X. Spectators populate ``p.crowd`` and ``p.crowd_arms`` so the
3D runtime animates jumping, La-Ola stadium waves, and waving arms/flags/foam fingers on the GPU.
"""

import math

from .characters import gnome
from .core import rng_for
from .shapes import box, cylinder, cylinder_x, ellipsoid, extrude_z, sphere, torus, xf
from .toy import Parts, tube

SHIRTS = ("#e8262d", "#2f78f0", "#ffd21f", "#2fb86a", "#ff8a1f", "#a45cf0", "#ff5fa8", "#ffffff", "#2fc9e8")
PANTS = ("#1f3558", "#2b3147", "#5c4033", "#3b5249", "#4a3b5c", "#6e5239")
SHOES = ("#fff7ec", "#e8262d", "#2f78f0", "#ffd21f", "#222630")
HAIRS = ("#2a1b12", "#5a3822", "#d89b3c", "#8c3b2b", "#1c1e26", "#ff4b3a", "#2fc9e8")
SKINS = ("#ffd2b0", "#f0b48a", "#c98a5e", "#8a5a3a", "#ffe0c8")


def _crowd_bins(p):
    if not hasattr(p, "crowd"):
        p.crowd = Parts()
        p.crowd_arms = Parts()
    return p.crowd, p.crowd_arms


def person(p, rng, x, y, z, arms=True, s=1.5, standing=False):
    """Articulated low-poly spectator facing +X with legs/shoes, layered outfit, face/glasses, headgear and animated arms/props."""
    body, waving = _crowd_bins(p)
    shirt = SHIRTS[rng.randrange(len(SHIRTS))]
    accent = SHIRTS[(rng.randrange(len(SHIRTS) - 1) + 1) % len(SHIRTS)]
    pants = PANTS[rng.randrange(len(PANTS))]
    shoe = SHOES[rng.randrange(len(SHOES))]
    skin = SKINS[rng.randrange(len(SKINS))]
    hair = HAIRS[rng.randrange(len(HAIRS))]

    # Legs & sneakers (standing vs seated on bench)
    for side in (1, -1):
        dz = z + side * 0.085 * s
        if standing:
            body.add("toy", box(0.15 * s, 0.07 * s, 0.085 * s), xf((x + 0.03 * s, y + 0.035 * s, dz)), tint=shoe)
            body.add("toy", box(0.095 * s, 0.34 * s, 0.095 * s), xf((x, y + 0.22 * s, dz)), tint=pants)
        else:
            body.add("toy", box(0.22 * s, 0.09 * s, 0.09 * s), xf((x + 0.09 * s, y + 0.045 * s, dz)), tint=pants)
            body.add("toy", box(0.085 * s, 0.22 * s, 0.085 * s), xf((x + 0.19 * s, y - 0.08 * s, dz), rot=(0, 0, -10)), tint=pants)
            body.add("toy", box(0.14 * s, 0.07 * s, 0.09 * s), xf((x + 0.23 * s, y - 0.19 * s, dz)), tint=shoe)

    hy = y + (0.35 * s if standing else 0.04 * s)
    # Torso, team chest stripe & collar
    body.add("toy", ellipsoid(0.17 * s, 0.26 * s, 0.20 * s, 5, 3), xf((x, hy + 0.26 * s, z)), tint=shirt)
    body.add("toy", box(0.24 * s, 0.075 * s, 0.32 * s), xf((x + 0.04 * s, hy + 0.28 * s, z)), tint=accent)
    body.add("toy", box(0.17 * s, 0.055 * s, 0.20 * s), xf((x + 0.01 * s, hy + 0.49 * s, z)), tint=accent)

    # Head, nose & eyes / sunglasses
    hc_y = hy + 0.64 * s
    body.add("toy", sphere(0.145 * s, 5, 3), xf((x + 0.01 * s, hc_y, z)), tint=skin)
    body.add("toy", box(0.045 * s, 0.045 * s, 0.045 * s), xf((x + 0.145 * s, hc_y - 0.01 * s, z)), tint=skin)
    if rng.random() < 0.45:
        visor = "#181c28" if rng.random() < 0.65 else accent
        body.add("toy", box(0.055 * s, 0.05 * s, 0.21 * s), xf((x + 0.13 * s, hc_y + 0.02 * s, z)), tint=visor)
    else:
        for side in (1, -1):
            body.add("toy", box(0.03 * s, 0.038 * s, 0.038 * s), xf((x + 0.14 * s, hc_y + 0.02 * s, z + side * 0.052 * s)), tint="#181c28")

    # Headgear / Hairstyles (5 distinct styles)
    style = rng.randrange(5)
    if style == 0:  # Baseball / racing cap with forward visor
        body.add("toy", ellipsoid(0.155 * s, 0.085 * s, 0.155 * s, 5, 3), xf((x, hc_y + 0.08 * s, z)), tint=accent)
        body.add("toy", box(0.15 * s, 0.024 * s, 0.19 * s), xf((x + 0.12 * s, hc_y + 0.065 * s, z), rot=(0, 0, -6)), tint=shirt)
    elif style == 1:  # Beanie with pom-pom
        body.add("toy", ellipsoid(0.155 * s, 0.105 * s, 0.155 * s, 5, 3), xf((x, hc_y + 0.09 * s, z)), tint=shirt)
        body.add("toy", sphere(0.052 * s, 4, 2), xf((x - 0.01 * s, hc_y + 0.20 * s, z)), tint="#fff7ec")
    elif style == 2:  # Sunhat / bucket hat
        body.add("toy", cylinder(0.22 * s, 0.24 * s, 0.028 * s, 6), xf((x, hc_y + 0.075 * s, z)), tint="#f0d290")
        body.add("toy", cylinder(0.135 * s, 0.145 * s, 0.095 * s, 6), xf((x, hc_y + 0.125 * s, z)), tint="#f0d290")
    elif style == 3:  # Sculpted hair + headband + ponytail
        body.add("toy", ellipsoid(0.155 * s, 0.095 * s, 0.16 * s, 5, 3), xf((x - 0.01 * s, hc_y + 0.075 * s, z)), tint=hair)
        body.add("toy", box(0.24 * s, 0.032 * s, 0.26 * s), xf((x + 0.01 * s, hc_y + 0.065 * s, z)), tint=accent)
        body.add("toy", sphere(0.065 * s, 4, 2), xf((x - 0.14 * s, hc_y + 0.04 * s, z)), tint=hair)
    else:  # Curly fan wig / afro
        body.add("toy", ellipsoid(0.175 * s, 0.125 * s, 0.185 * s, 5, 3), xf((x - 0.01 * s, hc_y + 0.085 * s, z)), tint=hair)

    # Articulated arms & cheering fan accessories (placed in p.crowd_arms for energetic waving)
    for side in (1, -1):
        az = z + side * 0.23 * s
        if arms:
            waving.add("toy", box(0.075 * s, 0.30 * s, 0.075 * s), xf((x + 0.05 * s, hy + 0.52 * s, az), rot=(side * -22, 0, -16)), tint=shirt)
            hx, hy_h, hz = x + 0.10 * s, hy + 0.69 * s, az + side * 0.06 * s
            waving.add("toy", sphere(0.05 * s, 4, 2), xf((hx, hy_h, hz)), tint=skin)
        else:
            waving.add("toy", box(0.22 * s, 0.075 * s, 0.075 * s), xf((x + 0.12 * s, hy + 0.34 * s, az), rot=(0, side * -18, 12)), tint=shirt)
            hx, hy_h, hz = x + 0.22 * s, hy + 0.38 * s, az - side * 0.03 * s
            waving.add("toy", sphere(0.048 * s, 4, 2), xf((hx, hy_h, hz)), tint=skin)

    # Accessories on the right hand (foam finger, flag, megaphone, pom-poms, or stadium drink cup)
    prop_kind = rng.randrange(5)
    rx, ry, rz = (x + 0.11 * s, hy + 0.72 * s, z + 0.29 * s) if arms else (x + 0.24 * s, hy + 0.42 * s, z + 0.18 * s)
    if arms and prop_kind == 0:  # Giant #1 foam finger
        waving.add("toy", box(0.045 * s, 0.22 * s, 0.13 * s), xf((rx, ry + 0.10 * s, rz)), tint=accent)
        waving.add("toy", box(0.04 * s, 0.15 * s, 0.045 * s), xf((rx, ry + 0.26 * s, rz - 0.03 * s)), tint=accent)
    elif arms and prop_kind == 1:  # Waving cheering flag on stick
        waving.add("toy", box(0.025 * s, 0.48 * s, 0.025 * s), xf((rx, ry + 0.20 * s, rz)), tint="#fff7ec")
        waving.add("toy", box(0.03 * s, 0.18 * s, 0.28 * s), xf((rx, ry + 0.35 * s, rz + 0.15 * s)), tint=accent)
        waving.add("toy", box(0.034 * s, 0.06 * s, 0.28 * s), xf((rx, ry + 0.35 * s, rz + 0.15 * s)), tint="#fff7ec")
    elif arms and prop_kind == 2:  # Fan megaphone
        waving.add("toy", cylinder(0.035 * s, 0.105 * s, 0.22 * s, 5), xf((rx + 0.06 * s, ry + 0.04 * s, rz), rot=(0, 0, -55)), tint=accent)
    elif arms and prop_kind == 3:  # Twin cheering pom-poms
        for side in (1, -1):
            waving.add("toy", sphere(0.085 * s, 4, 2), xf((x + 0.11 * s, hy + 0.73 * s, z + side * 0.29 * s)), tint=accent)
    elif not arms and prop_kind < 3:  # Stadium drink cup with straw
        waving.add("toy", cylinder(0.04 * s, 0.05 * s, 0.13 * s, 5), xf((rx, ry + 0.04 * s, rz)), tint="#e8262d")
        waving.add("toy", box(0.018 * s, 0.08 * s, 0.018 * s), xf((rx, ry + 0.12 * s, rz)), tint="#fff7ec")


def grandstand():
    """Four tiers of animated cheering spectators with bucket seats, front rail fans, PA speakers and bunting."""
    p = Parts()
    rng = rng_for("grandstand")
    for k, (x, top) in enumerate(((3.3, 1.0), (1.1, 2.0), (-1.1, 3.0), (-3.3, 4.0))):
        p.add("toy", box(2.2, top, 25.0), xf((x, top / 2, 0)), tint="#2f78f0" if k % 2 == 0 else "#4a90ff")
        p.add("toy", box(2.26, 0.12, 25.2), xf((x, top + 0.06, 0)), tint="#fff7ec")
        # Contoured stadium seat backrest strip along the row
        p.add("toy", box(0.14, 0.38, 24.4), xf((x - 0.38, top + 0.30, 0)), tint="#1d3566" if k % 2 == 0 else "#e8262d")
        for i in range(11):
            if rng.random() < 0.06:
                continue
            pz = -11.0 + 2.2 * i + 0.28 * (rng.random() - 0.5)
            person(p, rng, x + 0.12 * (rng.random() - 0.5), top + 0.12, pz, rng.random() < 0.74, s=1.48)
    # Standing ultra-fans cheering right along the front apron railing
    for i in range(6):
        pz = -9.5 + 3.8 * i + 0.25 * (rng.random() - 0.5)
        person(p, rng, 4.05, 0.0, pz, True, s=1.45, standing=True)
    for i in range(8):
        p.add("toy", box(0.05, 0.55, 2.4), xf((4.43, 0.5, -10.5 + 3.0 * i)), tint=SHIRTS[(i * 2) % len(SHIRTS)])
    p.add("toy", box(0.12, 0.12, 25.4), xf((4.55, 1.5, 0)), tint="#fff7ec")
    for i in range(9):
        p.add("toy", box(0.1, 0.6, 0.12), xf((4.55, 1.2, -12.0 + 3.0 * i)), tint="#fff7ec")
    p.add("toy", box(0.3, 8.4, 26.0), xf((-4.75, 4.2, 0)), tint="#252f5c")
    for z in (-12.9, 12.9):
        p.add("toy", box(9.6, 8.0, 0.3), xf((0, 4.0, z)), tint="#252f5c")
    p.add("toy", box(6.4, 0.36, 26.8, 0.04, 1), xf((-1.7, 8.4, 0), rot=(0, 0, 5)), tint="#e8262d")
    p.add("toy", box(0.1, 0.4, 26.8), xf((1.43, 8.1, 0), rot=(0, 0, 5)), tint="#fff7ec")
    for z in (-12.9, -6.5, 0.0, 6.5, 12.9):
        p.add("toy", cylinder(0.2, 0.2, 7.6, 8), xf((0.9, 3.8, z)), tint="#fff7ec")
    flag = extrude_z([(0.0, 0.0), (0.0, 0.7), (-0.9, 0.35)], -0.01, 0.01)
    for i in range(16):
        p.add("toy", flag, xf((1.95, 7.35, -12.0 + 1.6 * i)), tint=SHIRTS[(i * 3) % len(SHIRTS)])
    for z in (-11.8, 11.8):
        p.add("matte", cylinder(0.16, 0.14, 2.4, 8), xf((-3.6, 9.6, z)), tint="#39415f")
        p.add("matte", box(0.35, 0.55, 1.9, 0.04, 1), xf((-3.4, 10.85, z), rot=(0, 0, -18)), tint="#222630")
        for dz in (-0.6, 0.0, 0.6):
            p.add("glow_white", box(0.12, 0.38, 0.42), xf((-3.22, 10.82, z + dz), rot=(0, 0, -18)), tint="#fff9e6")
    for z in (-6.5, 6.5):
        p.add("matte", cylinder_x(0.12, 0.32, 0.48, 8), xf((1.35, 7.75, z), rot=(0, 0, -14)), tint="#2b2f38")
    return p


def fan_camp():
    """Trackside fan-club canopy pavilion with wooden viewing deck, feather flags, cooler, boombox and 6 animated cheering fans."""
    p = Parts()
    rng = rng_for("fan_camp")
    # Wooden viewing deck + front guardrail
    p.add("matte", box(3.2, 0.22, 5.4, 0.04, 1), xf((0.0, 0.11, 0.0)), tint="#8a5a36")
    p.add("toy", box(0.10, 0.85, 5.2), xf((1.48, 0.64, 0.0)), tint="#fff7ec")
    p.add("toy", box(0.06, 0.48, 4.6), xf((1.52, 0.58, 0.0)), tint="#e8262d")
    p.add("toy", box(0.08, 0.16, 3.8), xf((1.54, 0.58, 0.0)), tint="#ffd21f")
    # Canopy poles & pitched two-tone racing tent roof
    for sx in (-1.35, 1.35):
        for sz in (-2.35, 2.35):
            p.add("chrome", cylinder(0.06, 0.06, 2.9, 6), xf((sx, 1.55, sz)), tint="#c9d3e3")
    p.add("toy", box(3.1, 0.24, 5.1, 0.04, 1), xf((0.0, 3.05, 0.0)), tint="#e8262d")
    p.add("toy", box(2.3, 0.38, 4.2, 0.06, 1), xf((0.0, 3.34, 0.0)), tint="#fff7ec")
    p.add("toy", box(1.2, 0.32, 2.8, 0.05, 1), xf((0.0, 3.64, 0.0)), tint="#2f78f0")
    # Warm festoon string bulbs under the front canopy edge
    for z in (-1.8, -0.9, 0.0, 0.9, 1.8):
        p.add("glow_white", sphere(0.10, 6, 3), xf((1.48, 2.88, z)), tint="#ffe8a6")
    # Flanking feather flags
    for side in (-1, 1):
        fz = side * 3.15
        p.add("chrome", cylinder(0.045, 0.03, 3.8, 6), xf((0.4, 1.9, fz)), tint="#c9d3e3")
        p.add("toy", box(0.04, 2.2, 0.65, 0.02, 1), xf((0.4, 2.45, fz - side * 0.34)), tint="#ffd21f" if side > 0 else "#2fc9e8")
    # Beverage cooler & boombox on the deck
    p.add("toy", box(0.55, 0.46, 0.78, 0.04, 1), xf((-0.85, 0.45, -1.75)), tint="#2f78f0")
    p.add("toy", box(0.58, 0.08, 0.82), xf((-0.85, 0.70, -1.75)), tint="#fff7ec")
    p.add("matte", box(0.36, 0.34, 0.68, 0.03, 1), xf((-0.85, 0.39, 1.75)), tint="#222630")
    # Rear bench + 6 animated spectators (4 standing at the rail, 2 seated on the rear riser)
    p.add("toy", box(0.65, 0.45, 3.6, 0.03, 1), xf((-0.85, 0.44, 0.0)), tint="#252f5c")
    for z in (-1.8, -0.6, 0.6, 1.8):
        person(p, rng, 0.72, 0.22, z, True, s=1.42, standing=True)
    for z in (-0.85, 0.85):
        person(p, rng, -0.78, 0.68, z, True, s=1.42, standing=False)
    return p


def gnome_statue():
    """A four metre garden gnome statue grinning on a stone plinth."""
    p = Parts()
    p.add("matte", cylinder(1.5, 1.4, 0.2, 12, 0.04), xf((0, 0.1, 0)), tint="#8d8678")
    p.add("matte", cylinder(1.25, 1.15, 0.7, 12, 0.05), xf((0, 0.55, 0)), tint="#b9b2a6")
    p.add("matte", cylinder(1.4, 1.4, 0.14, 12), xf((0, 0.95, 0)), tint="#8d8678")
    p.add("chrome", box(0.08, 0.36, 0.72, 0.02, 1), xf((1.22, 0.55, 0)), tint="#ffd24a")
    for k in range(6):
        a = math.tau * k / 6.0
        bx, bz = 1.32 * math.cos(a), 1.32 * math.sin(a)
        p.add("fuzz", sphere(0.18, 6, 3), xf((bx, 0.22, bz)), tint="#2fb86a")
        p.add("toy", sphere(0.11, 6, 3), xf((bx, 0.34, bz)), tint=SHIRTS[k % len(SHIRTS)])
    p.include(gnome(), xf((0, 1.0, 0), scale=2.15))
    return p


def inflatable_arch():
    """Striped inflatable tube arch, 14 m wide and 8.6 m high, with pennants and sandbag feet."""
    p = Parts()
    half, spring, rise = 6.4, 5.2, 3.0
    path = [(0.0, 0.7 + k * (spring - 0.7) / 4.0, -half) for k in range(5)]
    for k in range(1, 15):
        a = math.pi * k / 15.0
        path.append((0.0, spring + rise * math.sin(a), -half * math.cos(a)))
    path += [(0.0, spring - k * (spring - 0.7) / 4.0, half) for k in range(5)]
    step = 3
    tones = ("#e8262d", "#fff7ec", "#2f78f0", "#fff7ec")
    for i, start in enumerate(range(0, len(path) - 1, step)):
        seg = path[start : min(start + step, len(path) - 1) + 1]
        if len(seg) > 1:
            p.add("toy", tube(seg, 0.95, 8), tint=tones[i % len(tones)])
    for z in (-half, half):
        p.add("matte", cylinder(1.25, 1.35, 0.5, 10, 0.08), xf((0, 0.25, z)), tint="#3a4666")
        p.add("matte", box(0.75, 0.65, 0.65, 0.05, 1), xf((-1.35, 0.34, z)), tint="#252f5c")
        p.add("chrome", cylinder_x(0.22, 0.22, 0.12, 8), xf((-1.72, 0.36, z)), tint="#c9d3e3")
        p.add("glow", torus(0.96, 0.07, 10, 4, 2.4), xf((0, 0, z)), tint="#ffd21f")
        p.add("toy", cylinder(0.05, 0.05, 1.8, 6), xf((0, spring + rise + 1.4, z * 0.4)), tint="#c9d3e3")
        p.add("toy", extrude_z([(0.0, 0.0), (0.0, 0.8), (-1.1, 0.4)], -0.01, 0.01), xf((0, spring + rise + 1.9, z * 0.4)), tint="#ffd21f")
    return p
