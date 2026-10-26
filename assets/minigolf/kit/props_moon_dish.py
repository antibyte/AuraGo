"""Moon dish: a fixed tripod base and a separate turning head (MIT License).

Both props are built in the same frame with their origin on the vertical axis
x = y = 0, so the course places them with identical transforms and the runtime
turns ``dishhead`` about that axis (``spin`` in ``PROP_META``). The head is
radially safe to spin: everything of the base that reaches the head's sweep is a
body of revolution about the axis.
"""

import math

from mathutils import Vector

from . import shapes
from .palette import mix
from .prop import Prop

WORLD = "moon"
R, DEPTH, THICK = 1.15, 0.62, 0.05  # bowl radius, depth and shell thickness
HUB = Vector((0.0, 0.0, 1.98))  # bowl centre on the vertical axis
TILT = (52.0, 0.0, 0.0)  # degrees about X: the bowl looks up and towards -Y
TURNTABLE = 1.4  # top of the fixed base ring


def _band(r, z, half, lift=0.04, seg=36):
    """Thin raised ring (body of revolution) used for accent bands."""
    return shapes.lathe("band", [(r, z - half), (r + lift, z - half * 0.7), (r + lift, z + half * 0.7), (r, z + half)], seg=seg,
                        caps=False)


def build_dishbase(seed=3):
    """Tripod, pedestal, fixed turntable ring, control box with status light."""
    p = Prop(WORLD, "dishbase", seed)
    boss = shapes.cylinder("boss", 0.36, 0.28, 0.14, seg=16)
    p.add(boss, "satin", ("panel_dark", "panel_grey"), bevel=0.03, segments=2, dens=0.3)
    ped = shapes.lathe("pedestal", [(0.0, 0.0), (0.17, 0.0), (0.17, 0.16), (0.12, 0.34), (0.12, TURNTABLE - 0.1), (0.0, TURNTABLE - 0.1)],
                       seg=14)
    p.add(ped, "metal", ("metal_dark", "metal"), loc=(0, 0, 0.1), bevel=0, dens=0.5, ao=0.5, smooth_angle=60)
    collar = shapes.cylinder("leg-collar", 0.2, 0.2, 0.2, seg=14)
    p.add(collar, "satin", "orange", loc=(0, 0, 0.98), bevel=0.025, segments=2, dens=0.3)
    ring = shapes.cylinder("turntable", 0.38, 0.34, 0.12, seg=24)
    p.add(ring, "satin", ("panel_dark", "panel_grey"), loc=(0, 0, TURNTABLE - 0.12), bevel=0.025, segments=2, dens=0.3)
    p.add(_band(0.355, TURNTABLE - 0.075, 0.03), "satin", "orange", bevel=0, dens=0, ao=0.4)
    for k in range(3):
        a = math.radians(90 + k * 120)
        c, s = math.cos(a), math.sin(a)
        leg = shapes.tube("tripod-leg", [(c * 0.12, s * 0.12, 1.1), (c * 0.5, s * 0.5, 0.62), (c * 0.92, s * 0.92, 0.1)],
                          radius=[0.065, 0.055, 0.045], seg=8)
        p.add(leg, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.5)
        foot = shapes.cylinder("pad", 0.2, 0.16, 0.09, seg=12)
        p.add(foot, "satin", ("panel_dark", "panel_grey"), loc=(c * 0.95, s * 0.95, 0.0), bevel=0.02, segments=2, dens=0)
    bx, by, yaw = 0.62, -0.62, -22.0
    to_world = shapes.matrix(rot=(0, 0, yaw))

    def on_box(lx, ly, z):
        v = to_world @ Vector((lx, ly, 0.0))
        return (bx + v.x, by + v.y, z)

    box = shapes.box("control-box", (0.5, 0.36, 0.36), base=True)
    p.add(box, "satin", ("panel_grey", "panel_white"), loc=(bx, by, 0.0), rot=(0, 0, yaw), bevel=0.04, segments=2, dens=0.3)
    stripe = shapes.box("box-stripe", (0.52, 0.38, 0.07), base=True)
    p.add(stripe, "satin", "orange", loc=(bx, by, 0.17), rot=(0, 0, yaw), bevel=0.012, segments=1, dens=0.4)
    led = shapes.sphere("led", 0.035, seg=8, rings=5)
    p.add(led, "emissive", ("cyan", "cyan"), loc=on_box(0.1, -0.19, 0.3), bevel=0, dens=0, ao=0.0, strength=2.5)
    strip = shapes.box("status", (0.16, 0.02, 0.05), base=False)
    p.add(strip, "emissive", ("window", "cyan"), loc=on_box(-0.07, -0.18, 0.12), rot=(0, 0, yaw), bevel=0, dens=0, ao=0.0, strength=2.5)
    cable = shapes.tube("cable", [(0.42, -0.5, 0.04), (0.3, -0.35, 0.02), (0.2, -0.2, 0.03), (0.17, -0.06, 0.06)], radius=0.022, seg=6)
    p.add(cable, "satin", ("black", "panel_dark"), bevel=0, dens=0, ao=0.5)
    return p.build()


def _bowl_profile(n=10):
    prof = [(0.0, -THICK)]
    for k in range(1, n + 1):
        r = R * k / n
        prof.append((r, DEPTH * (r / R) ** 2 - THICK))
    prof.append((R, DEPTH))
    for k in range(n - 1, -1, -1):
        r = R * k / n
        prof.append((r, DEPTH * (r / R) ** 2))
    return prof


def build_dishhead(seed=3):
    """Parabolic dish with ribs, feed horn on struts, tilt yoke and turning collar (spins about the base axis)."""
    p = Prop(WORLD, "dishhead", seed)
    pal = p.pal
    inv = shapes.matrix(rot=TILT).inverted()
    white, grey = pal.c("panel_white"), pal.c("panel_grey")
    back = mix(pal.c("metal"), pal.c("metal_dark"), 0.35)

    def bowl_colour(x, y, z):
        l = inv @ (Vector((x, y, z)) - HUB)
        r = math.hypot(l.x, l.y)
        if l.z - DEPTH * (r / R) ** 2 < -THICK * 0.5:
            return mix(back, grey, 0.4 * (r / R))
        t = min(1.0, r / R)
        return mix(white, grey, t ** 1.6)

    bowl = shapes.lathe("dish-shell", _bowl_profile(), seg=36, caps=False)
    p.add(bowl, "satin", None, loc=HUB, rot=TILT, fn=bowl_colour, bevel=0, dens=0.3, smooth_angle=70, ao=0.55)
    rot = shapes.matrix(rot=TILT)
    rim = shapes.torus("dish-rim", R, 0.045, seg=36, rseg=8)
    shapes.transform(rim, loc=(0, 0, DEPTH))
    p.add(rim, "satin", "orange", loc=HUB, rot=TILT, bevel=0, dens=0, ao=0.4)
    # radial ribs and one inner ring on the face
    for k in range(6):
        a = math.tau * k / 6 + math.radians(15)
        pts = [(R * f * math.cos(a), R * f * math.sin(a), DEPTH * f * f + 0.012) for f in (0.1, 0.25, 0.4, 0.55, 0.7, 0.85, 0.97)]
        rib = shapes.tube("dish-rib", pts, radius=0.017, seg=5)
        p.add(rib, "satin", ("panel_blue", "panel_grey"), loc=HUB, rot=TILT, bevel=0, dens=0, ao=0.4)
    inner = shapes.torus("dish-ring", R * 0.58, 0.016, seg=36, rseg=5)
    shapes.transform(inner, loc=(0, 0, DEPTH * 0.58 ** 2 + 0.012))
    p.add(inner, "satin", ("panel_blue", "panel_grey"), loc=HUB, rot=TILT, bevel=0, dens=0, ao=0.4)
    # feed struts and horn
    focus = HUB + rot @ Vector((0, 0, R * R / (4 * DEPTH)))
    for k in range(3):
        a = math.radians(30 + k * 120)
        rimpt = HUB + rot @ Vector((R * math.cos(a), R * math.sin(a), DEPTH))
        strut = shapes.tube("feed-strut", [rimpt, focus], radius=0.022, seg=6)
        p.add(strut, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.4)
    horn = shapes.lathe("feed-horn", [(0.0, 0.0), (0.05, 0.0), (0.1, 0.18), (0.1, 0.2), (0.0, 0.2)], seg=12)
    p.add(horn, "satin", "orange", loc=focus, rot=(TILT[0] + 180, 0, 0), bevel=0, dens=0, ao=0.4)
    lamp = shapes.sphere("horn-lamp", 0.04, seg=8, rings=5)
    p.add(lamp, "emissive", ("magenta", "magenta"), loc=focus + rot @ Vector((0, 0, 0.04)), bevel=0, dens=0, ao=0.0, strength=2.5)
    # backing hub, yoke and turning collar (all bodies of revolution about the axis except the hub)
    hub = shapes.cylinder("back-hub", 0.15, 0.26, 0.3, seg=14, base=False)
    p.add(hub, "satin", ("panel_dark", "panel_grey"), loc=HUB + rot @ Vector((0, 0, -0.17)), rot=TILT, bevel=0.02, segments=2, dens=0.3)
    yoke = shapes.sphere("yoke", 0.2, seg=14, rings=8)
    p.add(yoke, "satin", ("panel_dark", "panel_grey"), loc=(0, 0.08, TURNTABLE + 0.36), bevel=0, dens=0, ao=0.5)
    neck = shapes.cylinder("neck", 0.15, 0.17, 0.36, seg=14)
    p.add(neck, "metal", ("metal_dark", "metal"), loc=(0, 0, TURNTABLE + 0.03), bevel=0, dens=0, ao=0.5)
    collar = shapes.cylinder("collar", 0.31, 0.29, 0.08, seg=24)
    p.add(collar, "satin", ("panel_grey", "panel_white"), loc=(0, 0, TURNTABLE), bevel=0.02, segments=2, dens=0.3)
    p.add(_band(0.29, TURNTABLE + 0.045, 0.025), "satin", "orange", bevel=0, dens=0, ao=0.4)
    return p.build()
