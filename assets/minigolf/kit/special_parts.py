"""Hand-built layout parts of the dressed courses (MIT License, see ../LICENSE).

Pier beams and piles, jump ramps, bridge parapets, the sun wheel, the floating
moon pads and the gate hub follow the lane (``frame``) and keep the place and
size of the legacy parts they replace; style and material come from the world.
Every function adds parts to a ``Prop`` (one joined ``scenery`` object that is
baked) or returns standalone objects (the turning sun wheel blades).
"""

import math

from . import shapes
from .courses_v1 import lane_center_height
from .prop import Prop


def strip(obj):
    """Drop the builder's bookkeeping properties so only ``aurago_*`` extras reach the GLB."""
    for key in [k for k in obj.keys() if not k.startswith("aurago_")]:
        del obj[key]
    return obj


def frame(course, path, t):
    """Lane point at path fraction ``t``: x, y, lane height and the unit tangent."""
    n = len(path)
    i = max(0, min(n - 1, round(t * (n - 1))))
    prev, nxt = path[max(0, i - 1)], path[min(n - 1, i + 1)]
    dx, dy = nxt[0] - prev[0], nxt[1] - prev[1]
    ln = max(1e-4, math.hypot(dx, dy))
    return {"x": path[i][0], "y": path[i][1], "h": lane_center_height(course, path, i), "tx": dx / ln, "ty": dy / ln}


def yaw_deg(f):
    """Degrees about Z that turn the local Y axis of a part along the lane."""
    return math.degrees(math.atan2(-f["tx"], f["ty"]))


def side(f, offset):
    """Point ``offset`` metres to the left (positive) of the lane at frame ``f``."""
    return f["x"] - f["ty"] * offset, f["y"] + f["tx"] * offset


def pier(p, look, course, path):
    """Cross beams every 0.7 m with a post at each beam end (pirate pier)."""
    for i in range(10):
        f = frame(course, path, 0.16 + i * 0.071)
        beam = shapes.box("pier-beam", (2.7, 0.16, 0.09), base=True)
        p.add(beam, "matte", "wood", loc=(f["x"], f["y"], f["h"] - 0.075), rot=(0, 0, yaw_deg(f)), bevel=0.016, segments=1, dens=0.3,
              ao=0.7)
    for k in range(5):
        f = frame(course, path, 0.12 + k * 0.17)
        for sign in (-1, 1):
            x, y = side(f, sign * 1.38)
            post = shapes.cylinder("pier-post", 0.085, 0.07, 0.78, seg=8)
            p.add(post, "matte", "wood_dark", loc=(x, y, f["h"] - 0.62), bevel=0.012, segments=1, dens=0, ao=0.7)


def ramps(p, look, course, path):
    """Two plank lips at the take-off and landing side of the jump."""
    for t in (0.43, 0.60):
        f = frame(course, path, t)
        slab = shapes.box("leap-ramp", (1.7, 0.42, 0.09), base=True)
        p.add(slab, "matte", "wood", loc=(f["x"], f["y"], f["h"] - 0.07), rot=(0, 0, yaw_deg(f) + 90.0), bevel=0.018, segments=1,
              dens=0.2, ao=0.7)


def parapets(p, look, course, path, t=0.5):
    """Low parapet walls with end posts along both sides of a bridge (stone in the temple, metal on the moon)."""
    f = frame(course, path, t)
    moon = look == "moon"
    for sign in (-1, 1):
        x, y = side(f, sign * 1.16)
        wall = shapes.box("bridge-parapet", (0.16, 2.55, 0.34 if moon else 0.46), base=True)
        p.add(wall, "satin" if moon else "matte", ("panel_grey", "panel_white") if moon else "stone",
              loc=(x, y, f["h"] - 0.06), rot=(0, 0, yaw_deg(f)), bevel=0.03, segments=2, dens=0.35, ao=0.7)
        for k in range(4):
            along = -1.1 + k * 0.733
            px, py = x + f["tx"] * along, y + f["ty"] * along
            post = shapes.cylinder("bridge-post", 0.07, 0.07, 0.62 if not moon else 0.5, seg=8)
            if moon:
                p.add(post, "satin", "orange", loc=(px, py, f["h"] - 0.06), bevel=0.012, segments=1, dens=0, ao=0.6)
            else:
                p.add(post, "matte", ("stone", "stone_light"), loc=(px, py, f["h"] - 0.06), bevel=0.014, segments=1, dens=0, ao=0.6)
        if moon:
            strip = shapes.box("bridge-neon", (0.04, 2.5, 0.02), base=True)
            tone = "cyan" if sign < 0 else "magenta"
            p.add(strip, "emissive", (tone, tone), loc=(x, y, f["h"] + 0.28 + 0.004), rot=(0, 0, yaw_deg(f)), finish=False, ao=0.0,
                  strength=1.6, jitter=0.0)
        else:
            moss = shapes.box("bridge-moss", (0.2, 1.9, 0.05), base=True)
            p.add(moss, "matte", "moss", loc=(x, y, f["h"] + 0.38), rot=(0, 0, yaw_deg(f)), bevel=0.02, segments=1, dens=0.3,
                  ao=0.6, noise_amp=0.12)


def pads(p, look, course, path):
    """Three floating landing pads inside the gap of the lily jump, ringed with neon."""
    for j, t in enumerate((0.485, 0.51, 0.535)):
        f = frame(course, path, t)
        x, y = side(f, (-0.24 if j == 0 else (0.23 if j == 2 else 0.0)))
        disc = shapes.cylinder("pad", 0.38, 0.36, 0.1, seg=16)
        p.add(disc, "satin", ("panel_grey", "panel_white"), loc=(x, y, f["h"] - 0.2), bevel=0.016, segments=1, dens=0.2, ao=0.6)
        rim = shapes.torus("pad-ring", 0.31, 0.02, seg=24, rseg=6)
        p.add(rim, "emissive", ("cyan", "cyan"), loc=(x, y, f["h"] - 0.098), finish=False, ao=0.0, strength=1.6, jitter=0.0)
        under = shapes.cone("pad-under", 0.34, 0.16, seg=12)
        p.add(under, "metal", "metal", loc=(x, y, f["h"] - 0.2), rot=(180, 0, 0), bevel=0, dens=0, ao=0.6)


def gate_hub(p, look, course, path, t=0.53):
    """Pivot post under the revolving gate (the gate itself is the dynamic obstacle)."""
    f = frame(course, path, t)
    base = shapes.cylinder("gate-hub-base", 0.26, 0.2, 0.07, seg=14)
    p.add(base, "metal", "metal", loc=(f["x"], f["y"], f["h"]), bevel=0.012, segments=1, dens=0, ao=0.6)
    post = shapes.cylinder("gate-hub", 0.11, 0.11, 0.3, seg=12)
    p.add(post, "satin", "orange", loc=(f["x"], f["y"], f["h"] + 0.06), bevel=0.01, segments=1, dens=0, ao=0.5)


def sun_wheel(p, look, course, path, t=0.55):
    """Stone mast, gold hub and sun face of the sun wheel; returns the four turning blades as standalone objects."""
    f = frame(course, path, t)
    mx, my, mz = f["x"] + 1.48, f["y"], f["h"]
    mast = shapes.cylinder("wheel-mast", 0.13, 0.09, 2.1, seg=10)
    p.add(mast, "matte", "stone", loc=(mx, my, mz), bevel=0.02, segments=1, dens=0.4, ao=0.7)
    foot = shapes.box("wheel-foot", (0.5, 0.5, 0.16), base=True)
    p.add(foot, "matte", ("stone", "stone_light"), loc=(mx, my, mz), bevel=0.03, segments=2, dens=0.3, ao=0.7)
    hub = shapes.cylinder("wheel-hub", 0.2, 0.2, 0.2, seg=14, base=False)
    p.add(hub, "satin", "gold", loc=(mx, my, mz + 2.13), rot=(90, 0, 0), bevel=0.02, segments=1, dens=0, ao=0.4)
    face = shapes.cylinder("wheel-face", 0.32, 0.3, 0.05, seg=16, base=False)
    p.add(face, "satin", ("gold", "gold_light"), loc=(mx, my - 0.11, mz + 2.13), rot=(90, 0, 0), bevel=0.01, segments=1, dens=0, ao=0.3)
    blades = []
    for i, angle in enumerate((0.0, math.pi / 2, math.pi, math.pi * 1.5)):
        q = Prop(look, "blade", seed=i + 1, full_name="windmill-sail")
        blade = shapes.box("sun-blade", (0.88, 0.12, 0.26), base=False)
        q.add(blade, "satin", ("fire_orange", "fire_yellow") if i % 2 else ("gold_dark", "gold_light"), mode="x", bevel=0.03,
              segments=2, dens=0.25, ao=0.3)
        obj = strip(q.build())
        obj.location = (mx + 0.48 * math.cos(angle), my, mz + 2.13 + 0.48 * math.sin(angle))
        obj.rotation_euler[1] = angle
        blades.append(obj)
    return blades, (mx, my, 0.95)
