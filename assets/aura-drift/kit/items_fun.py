"""Aura Drift items, the funny ones: gnome mine, rubber duck, boxing glove on a spring, disco ball (MIT, AuraGo contributors)."""

import math

from .characters import duck, gnome
from .core import hex_linear, rng_for
from .shapes import arc_band, box, cylinder, cylinder_x, ellipsoid, sphere, torus, xf
from .toy import Parts, helix, tube


def gnome_mine():
    """A grinning garden gnome standing on a spiky olive mine plate; the hat bulb is the blinking ``glow_red``."""
    p = Parts()
    for k in range(8):
        a0 = 45.0 * k
        stripe = "#ffcf1f" if k % 2 == 0 else "#2b2f38"
        p.add("toy", arc_band(0.66, 0.9, a0, a0 + 45.0, 0.0, 0.2, 2), xf(rot=(-90, 0, 0)), tint=stripe)
    p.add("matte", cylinder(0.68, 0.68, 0.2, 14), xf((0, 0.1, 0)), tint="#6b7a2f")
    p.add("toy", cylinder(0.38, 0.32, 0.1, 10), xf((0, 0.23, 0)), tint="#e8262d")
    for k in range(8):
        a = math.tau * (k + 0.5) / 8
        p.add("toy", cylinder(0.1, 0.012, 0.34, 5), xf((0.95 * math.cos(a), 0.34, 0.95 * math.sin(a)), (math.degrees(math.sin(a)) * 0.6, 0, -math.degrees(math.cos(a)) * 0.6)), tint="#c9d3e3")
    p.include(gnome(), xf((0, 0.2, 0), scale=0.9))
    return p


def rubber_duck():
    p = Parts()
    p.include(duck(), xf(scale=1.1))
    return p


def boxing_glove():
    """Red boxing glove on a yellow spring, punching toward +X from a little pedestal."""
    p = Parts()
    cy = 1.0
    p.add("toy", box(0.5, cy - 0.15, 0.7), xf((-1.05, (cy - 0.15) / 2, 0)), tint="#2b3a67")
    p.add("toy", cylinder_x(0.5, 0.5, 0.12, 12), xf((-0.9, cy, 0)), tint="#c9d3e3")
    p.add("toy", tube(helix(0.3, 0.95, 3.5, 30, cy), 0.07, 5), xf((-0.4, 0, 0)), tint="#ffd23a")
    p.add("toy", cylinder_x(0.34, 0.34, 0.1, 12), xf((0.05, cy, 0)), tint="#c9d3e3")
    p.add("toy", ellipsoid(0.7, 0.6, 0.52, 12, 6), xf((0.8, cy + 0.06, 0)), tint="#e8262d")
    p.add("toy", ellipsoid(0.42, 0.3, 0.46, 10, 5), xf((1.0, cy - 0.28, 0)), tint="#d11f27")  # curled fingers
    p.add("toy", ellipsoid(0.42, 0.2, 0.2, 8, 4), xf((0.72, cy + 0.0, 0.52), (0, -25, -14)), tint="#ff4a50")  # thumb
    for x in (1.06, 1.22):
        p.add("matte", torus(0.52 - (x - 1.06) * 1.3, 0.022, 14, 4), xf((x, cy + 0.04, 0), (0, 0, 90)), tint="#8f0f16")
    p.add("toy", cylinder_x(0.43, 0.4, 0.42, 12), xf((0.24, cy, 0)), tint="#fff7ec")
    p.add("toy", cylinder_x(0.44, 0.44, 0.08, 12), xf((0.34, cy, 0)), tint="#2f78f0")
    p.add("toy", torus(0.46, 0.04, 12, 4), xf((0.5, cy + 0.03, 0), (0, 0, 90)), tint="#fff7ec")
    return p


def disco_ball():
    """Mirrored ball on a short stand: each tile is its own flat quad with a pastel tint."""
    p = Parts()
    cy, r = 1.4, 0.8
    rng = rng_for("disco")
    palette = ["#ffffff", "#9fd0ff", "#ff9fe0", "#c3a6ff", "#ffe58a", "#9fffd2", "#ffffff"]
    p.add("matte", sphere(r * 0.94, 14, 7), xf((0, cy, 0)), tint="#2a2f5a")
    bands, around = 10, 18
    for i in range(bands):
        lat0 = math.radians(-82.0 + 164.0 * i / bands)
        lat1 = math.radians(-82.0 + 164.0 * (i + 1) / bands)
        for j in range(around):
            lon0 = math.tau * j / around
            lon1 = math.tau * (j + 1) / around
            corners = [(lat0, lon0), (lat0, lon1), (lat1, lon1), (lat1, lon0)]
            pts = [(r * math.cos(a) * math.cos(b), r * math.sin(a), r * math.cos(a) * math.sin(b)) for a, b in corners]
            c = tuple(sum(q[k] for q in pts) / 4.0 for k in range(3))
            pts = [tuple(c[k] + (q[k] - c[k]) * 0.88 for k in range(3)) for q in pts]
            n1 = (pts[1][0] - pts[0][0], pts[1][1] - pts[0][1], pts[1][2] - pts[0][2])
            n2 = (pts[2][0] - pts[0][0], pts[2][1] - pts[0][1], pts[2][2] - pts[0][2])
            normal = (n1[1] * n2[2] - n1[2] * n2[1], n1[2] * n2[0] - n1[0] * n2[2], n1[0] * n2[1] - n1[1] * n2[0])
            face = (0, 1, 2, 3) if sum(normal[k] * c[k] for k in range(3)) > 0 else (3, 2, 1, 0)
            p.add("mirror", (pts, [face]), xf((0, cy, 0)), tint=palette[rng.randrange(len(palette))])
    p.add("toy", torus(0.16, 0.04, 10, 4), xf((0, cy + r + 0.1, 0), (90, 0, 0)), tint="#c9d3e3")
    p.add("toy", cylinder(0.06, 0.06, 0.3, 8), xf((0, 0.34, 0)), tint="#c9d3e3")
    p.add("toy", cylinder(0.5, 0.42, 0.14, 14), xf((0, 0.07, 0)), tint="#ff4fb8")
    p.add("toy", cylinder(0.32, 0.26, 0.12, 14), xf((0, 0.18, 0)), tint="#7a4bff")
    return p
