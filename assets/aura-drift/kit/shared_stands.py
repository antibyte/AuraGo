"""Aura Drift shared props, the big three: grandstand with crowd, gnome statue, inflatable arch (MIT, AuraGo contributors).

Props stand on y = 0, centred, front toward +X. The crowd colours are baked into the vertex colours.
"""

import math

from .characters import gnome
from .core import rng_for
from .shapes import box, cylinder, extrude_z, sphere, ellipsoid, xf
from .toy import Parts, tube

SHIRTS = ("#e8262d", "#2f78f0", "#ffd21f", "#2fb86a", "#ff8a1f", "#a45cf0", "#ff5fa8", "#ffffff", "#2fc9e8")
SKINS = ("#ffd2b0", "#f0b48a", "#c98a5e", "#8a5a3a", "#ffe0c8")


def person(p, rng, x, y, z, arms, s=1.5):
    """One spectator facing +X: egg body, round head, optional cheering arms (toy proportions, scale ``s``)."""
    shirt = SHIRTS[rng.randrange(len(SHIRTS))]
    skin = SKINS[rng.randrange(len(SKINS))]
    p.add("toy", ellipsoid(0.25 * s, 0.34 * s, 0.21 * s, 5, 3), xf((x, y + 0.34 * s, z)), tint=shirt)
    p.add("toy", sphere(0.16 * s, 5, 3), xf((x, y + 0.8 * s, z)), tint=skin)
    if arms:
        for side in (1, -1):
            p.add("toy", box(0.08 * s, 0.42 * s, 0.08 * s), xf((x, y + 0.82 * s, z + side * 0.3 * s), rot=(side * -18, 0, 0)), tint=skin)


def grandstand():
    """Four tiers of cheering toy people under a red roof with bunting; about 26 m wide, 8 m high."""
    p = Parts()
    rng = rng_for("grandstand")
    for k, (x, top) in enumerate(((3.3, 1.0), (1.1, 2.0), (-1.1, 3.0), (-3.3, 4.0))):
        p.add("toy", box(2.2, top, 25.0), xf((x, top / 2, 0)), tint="#2f78f0" if k % 2 == 0 else "#4a90ff")
        p.add("toy", box(2.26, 0.12, 25.2), xf((x, top + 0.06, 0)), tint="#fff7ec")
        for i in range(15):
            if rng.random() < 0.12:
                continue
            person(p, rng, x + 0.2 * (rng.random() - 0.5), top + 0.12, -11.2 + 1.6 * i + 0.4 * (rng.random() - 0.5), rng.random() < 0.38)
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
    return p


def gnome_statue():
    """A four metre garden gnome statue grinning on a stone plinth."""
    p = Parts()
    p.add("matte", cylinder(1.5, 1.4, 0.2, 12, 0.04), xf((0, 0.1, 0)), tint="#8d8678")
    p.add("matte", cylinder(1.25, 1.15, 0.7, 12, 0.05), xf((0, 0.55, 0)), tint="#b9b2a6")
    p.add("matte", cylinder(1.4, 1.4, 0.14, 12), xf((0, 0.95, 0)), tint="#8d8678")
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
        p.add("toy", cylinder(0.05, 0.05, 1.8, 6), xf((0, spring + rise + 1.4, z * 0.4)), tint="#c9d3e3")
        p.add("toy", extrude_z([(0.0, 0.0), (0.0, 0.8), (-1.1, 0.4)], -0.01, 0.01), xf((0, spring + rise + 1.9, z * 0.4)), tint="#ffd21f")
    return p
