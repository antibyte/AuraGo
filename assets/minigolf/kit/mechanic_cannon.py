"""Visible parts of the pirate cannon (MIT License, see ../LICENSE).

Local frame: origin on the floor in the middle of the catch zone, +X along the flight direction, Z up. The ball rolls
into an iron horn whose mouth stands at the near edge of the zone (the zone catches the ball there); the horn bends up into
the breech of a big iron barrel on a wooden carriage; the barrel axis is the launch direction and its mouth holds the
muzzle point, where the ball shows up before it is fired (``cannon_flight.solve``). A red-and-gold ring on the floor marks
the zone. Nothing here collides.
"""

import math

from . import cannon_flight, mech_util, shapes
from .mech_util import add_box, add_cylinder, ring_polygons
from .prop import Prop

HORN_MOUTH = (-0.42, 0.30)       # mouth centre (x, z) of the horn
HORN_RADII = (0.36, 0.3, 0.24, 0.2)
CHEEK_Y = 0.36
WHEEL = 0.3


def _bezier(a, b, c, d, count):
    return [tuple((1 - t) ** 3 * a[i] + 3 * (1 - t) ** 2 * t * b[i] + 3 * (1 - t) * t * t * c[i] + t ** 3 * d[i] for i in range(3))
            for t in (k / (count - 1) for k in range(count))]


def _barrel(p, length, breech_point, alpha):
    profile = [(0.0, -0.42), (0.06, -0.41), (0.09, -0.36), (0.07, -0.3), (0.1, -0.27), (0.22, -0.22), (0.255, -0.1), (0.26, 0.1), (0.25, 0.4),
               (0.235, length * 0.6), (0.225, length - 0.2), (0.215, length - 0.12), (0.275, length - 0.08), (0.28, length + 0.02),
               (0.27, length + 0.1), (0.19, length + 0.1), (0.19, length - 0.5), (0.0, length - 0.5)]
    barrel = shapes.lathe("barrel", profile, seg=20)
    shapes.smooth_by_angle(barrel.data, 55.0)
    p.add(barrel, "metal", "iron", loc=breech_point, rot=(0, 90 - math.degrees(alpha), 0), finish=False, ao=0.5)
    for t, r in ((0.5, 0.245), (length * 0.35, 0.241), (length - 0.16, 0.222)):       # raised bands
        band = shapes.lathe("band", [(r - 0.01, t - 0.04), (r + 0.012, t - 0.04), (r + 0.012, t + 0.04), (r - 0.01, t + 0.04)], seg=20, caps=False)
        p.add(band, "metal", ("iron_light", "iron_light"), loc=breech_point, rot=(0, 90 - math.degrees(alpha), 0), bevel=0, dens=0, ao=0.5)


def _carriage(p, trunnion):
    tx, tz = trunnion
    height = tz + 0.12
    for sign in (-1, 1):
        add_box(p, (tx, sign * CHEEK_Y, height / 2), (0.4, 0.06, height / 2), ("wood_dark", "wood"), bevel=0.02, noise_amp=0.05, ao=0.6)
        add_cylinder(p, (tx, sign * (CHEEK_Y + 0.1), tz), 0.09, 0.14, "iron", mat="metal", axis="y", seg=14, bevel=0.01, ao=0.5)
        add_box(p, (tx, sign * CHEEK_Y, height + 0.01), (0.42, 0.07, 0.025), "iron", mat="metal", bevel=0.008, segments=1, dens=0.2, ao=0.5)
        wheel = shapes.cylinder("wheel", WHEEL, WHEEL, 0.09, seg=14, base=False)
        p.add(wheel, "matte", ("wood_dark", "wood"), loc=(tx, sign * 0.52, WHEEL), rot=(90, 0, 0), bevel=0.012, segments=1, dens=0.5)
        rim = shapes.torus("rim", WHEEL - 0.005, 0.02, seg=16, rseg=4)
        p.add(rim, "metal", "iron", loc=(tx, sign * 0.52, WHEEL), rot=(90, 0, 0), bevel=0, dens=0, ao=0.5)
        hub = shapes.cylinder("hub", 0.06, 0.06, 0.16, seg=10, base=False)
        p.add(hub, "metal", "iron", loc=(tx, sign * 0.52, WHEEL), rot=(90, 0, 0), bevel=0.006, segments=1, dens=0, ao=0.4)
    add_cylinder(p, (tx, 0, WHEEL), 0.04, 1.0, "wood_dark", axis="y", seg=8, bevel=0, dens=0.4)


def _horn(p, breech_point, alpha):
    u = (math.cos(alpha), 0.0, math.sin(alpha))
    end = tuple(breech_point[i] - 0.15 * u[i] for i in range(3))
    start = (HORN_MOUTH[0], 0.0, HORN_MOUTH[1])
    path = _bezier(start, (start[0] + 0.45, 0.0, start[2]), tuple(end[i] - 0.4 * u[i] for i in range(3)), end, 10)
    radii = [HORN_RADII[0] + (HORN_RADII[-1] - HORN_RADII[0]) * (k / 9) ** 0.7 for k in range(10)]
    horn = shapes.tube("horn", path, radius=radii, seg=16, caps=True)
    p.add(horn, "metal", ("iron_dark", "iron_light"), mode="x", bevel=0, dens=0, smooth_angle=60, ao=0.5)
    rim = shapes.torus("horn-rim", radii[0] - 0.02, 0.035, seg=16, rseg=5)
    p.add(rim, "satin", "gold", loc=start, rot=(0, 90, 0), bevel=0, dens=0, ao=0.4)
    pit = shapes.cylinder("horn-pit", radii[0] - 0.05, radii[0] - 0.05, 0.02, seg=16, base=False)
    p.add(pit, "matte", ("coal", "coal"), loc=(start[0] - 0.018, 0.0, start[2]), rot=(0, 90, 0), bevel=0, dens=0, ao=0.0)


def build(ctx, item):
    sol = item["solution"]
    x, y, z = item["x"], item["y"], item["z"]
    p = Prop(ctx.world, "cannon-" + item["id"], seed=ctx.seed())
    alpha = sol["elevation"]
    breech_point = (cannon_flight.BREECH_FORWARD, 0.0, sol["breech"][2] - z)
    _barrel(p, item["barrel"], breech_point, alpha)
    trunnion = (breech_point[0] + 0.55 * math.cos(alpha), breech_point[2] + 0.55 * math.sin(alpha))
    _carriage(p, trunnion)
    _horn(p, breech_point, alpha)
    ctx.add_static(p.build(), (x, y, z), sol["heading"])
    pal = p.pal
    red, gold = pal.c("red"), pal.c("gold")
    outer, inner = item["radius"], item["radius"] - 0.07
    ctx.static.append(mech_util.draped("cannon-ring", ring_polygons(x, y, inner, outer, 32), ctx.floor_z, 0.006,
                                       lambda px, py, pz: red if int((math.atan2(py - y, px - x) % math.tau) / math.tau * 16) % 2 else gold,
                                       "satin", ao=0.4))
