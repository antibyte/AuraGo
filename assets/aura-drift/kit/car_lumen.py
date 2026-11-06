"""Lumen: the muscle car. Broad rear hips, blunt nose, bonnet scoop, twin stripes, side pipes (MIT, AuraGo contributors).

Boxy lofted hull (superellipse power 4.2) with a short greenhouse set well back, a wide black grille
between four round lamps, hip-flared fenders and chrome side-exit pipes. About 4.7 x 2.05 x 1.3 m.
"""

import math

from .cars_common import CarBuilder
from .core import hex_linear
from .shapes import Hull, arc_band, box, cylinder_x, spline, strut, taper_box, xf

CAR_ID = "lumen"
ACCENT = "#f3e3c3"

HULL_X = [-2.30, -2.15, -1.80, -1.25, -0.55, 0.20, 0.90, 1.50, 1.95, 2.22, 2.40]
HALF_WIDTH = spline([(-2.3, 0.74), (-2.1, 0.92), (-1.6, 0.99), (-1.2, 0.96), (-0.4, 0.91), (0.4, 0.88), (1.4, 0.90), (1.95, 0.88), (2.22, 0.84), (2.4, 0.76)])
TOP = spline([(-2.3, 0.88), (-2.0, 0.97), (-1.3, 0.95), (-0.4, 0.93), (0.5, 0.95), (1.2, 0.98), (1.8, 0.95), (2.2, 0.85), (2.4, 0.70)])
BOTTOM = spline([(-2.3, 0.34), (-2.0, 0.26), (-1.0, 0.22), (2.0, 0.22), (2.4, 0.28)])

CANOPY_X = [-1.65, -1.35, -0.95, -0.45, 0.05, 0.35, 0.65]
CANOPY_HW = spline([(-1.65, 0.52), (-1.35, 0.66), (-0.95, 0.72), (-0.3, 0.74), (0.2, 0.70), (0.65, 0.56)])
CANOPY_TOP = spline([(-1.65, 0.97), (-1.35, 1.17), (-0.95, 1.30), (-0.3, 1.34), (0.2, 1.28), (0.45, 1.12), (0.65, 0.96)])

R = 0.40
TRACK = 0.84
AXLE = 1.40


def stripe(car, fn, x0, x1, z, width, step=0.35, lift=0.006):
    """Thin accent ribbon lying on a top surface y = fn(x), cut into short slanted pieces."""
    n = max(1, int(round((x1 - x0) / step)))
    for i in range(n):
        a = x0 + (x1 - x0) * i / n
        b = x0 + (x1 - x0) * (i + 1) / n
        ya, yb = fn(a), fn(b)
        angle = math.degrees(math.atan2(yb - ya, b - a))
        length = math.hypot(b - a, yb - ya) + 0.02
        car.add("paint_accent", box(length, 0.014, width), xf(((a + b) / 2.0, (ya + yb) / 2.0 + lift, z), rot=(0, 0, angle)))


def build_hull(car):
    stations = [
        {"x": x, "hw": HALF_WIDTH(x), "yb": BOTTOM(x), "yt": TOP(x), "power": 4.2, "top_flat": HALF_WIDTH(x) * 0.5, "bot_flat": HALF_WIDTH(x) * 0.5}
        for x in HULL_X
    ]
    hull = Hull(stations, m=6)
    under = hull.band([3, 4, 5, 6, 7])
    car.add("trim", hull.shape, faces=hull.pick(under))
    car.add("paint", hull.shape, faces=hull.rest(under))
    for z in (0.2, -0.2):
        stripe(car, TOP, -2.2, 2.05, z, 0.15)


def build_canopy(car):
    stations = [{"x": x, "hw": CANOPY_HW(x), "yb": 0.70, "yt": CANOPY_TOP(x), "power": 3.0, "top_flat": 0.5 * CANOPY_HW(x)} for x in CANOPY_X]
    canopy = Hull(stations, m=6)
    roof = canopy.band([canopy.TOP, 0], *canopy.span(-1.35, 0.05))
    car.add("paint", canopy.shape, faces=canopy.pick(roof))
    car.add("glass", canopy.shape, faces=canopy.rest(roof))
    for z in (0.2, -0.2):
        stripe(car, lambda x: CANOPY_TOP(x), -1.3, 0.05, z, 0.15, 0.3, 0.004)
    for k in (1, canopy.n - 2):
        for i in range(len(CANOPY_X) - 1):
            if CANOPY_X[i] >= -1.0:
                car.add("trim", strut(canopy.point(i, k), canopy.point(i + 1, k), 0.034, 4))
    car.driver((-0.55, 0.52, 0.0))


def build_fenders(car):
    for x, flare in ((AXLE, 0.09), (-AXLE, 0.15)):
        arch = arc_band(R + 0.04, R + flare, 4, 176, 0.45, TRACK + 0.24, 8, 0.03)
        car.pair("paint", arch, (x, R, 0.0))


def build_nose(car):
    car.add("chrome", box(0.08, 0.30, 1.0, 0.04, 1), xf((2.39, 0.46, 0.0)))
    car.add("trim", box(0.09, 0.22, 0.86, 0.03, 1), xf((2.395, 0.46, 0.0)))
    for z in (-0.28, -0.14, 0.0, 0.14, 0.28):
        car.add("chrome", box(0.05, 0.2, 0.018), xf((2.44, 0.46, z)))
    for z in (0.74, 0.5, -0.5, -0.74):
        pod = 0.16 if abs(z) > 0.6 else 0.13
        car.add("chrome", cylinder_x(pod, pod, 0.14, 12, 0.012), xf((2.33, 0.62, z)))
        car.add("head_light", cylinder_x(pod * 0.78, pod * 0.78, 0.04, 12), xf((2.41, 0.62, z)))
    car.add("chrome", box(0.12, 0.06, 1.74, 0.02, 1), xf((2.40, 0.24, 0.0)))
    car.add("trim", taper_box(0.3, 1.6, 0.04, 1.7, 0.05, 0.0, 0.01, 1), xf((2.22, 0.14, 0.0)))
    car.anchor_pair("anchor_head", (2.46, 0.62, 0.74))
    # bonnet scoop: tall forward opening, tapering into the bonnet
    top = TOP(1.2)
    car.add("paint", taper_box(0.9, 0.62, 0.05, 0.62, 0.24, 0.095, 0.03, 1), xf((1.2, top + 0.03, 0.0), rot=(0, 0, -2.5)))
    car.add("trim", box(0.04, 0.17, 0.52, 0.008, 1), xf((1.66, top + 0.14, 0.0)))
    for z in (0.48, -0.48):
        car.add("chrome", box(0.07, 0.016, 0.07, 0.01, 1), xf((2.04, TOP(2.04) + 0.008, z)))
    for lx in (-1.56, -1.40, -1.24):
        car.add("trim", box(0.09, 0.022, 0.92, 0.005, 1), xf((lx, CANOPY_TOP(lx) + 0.018, 0.0), rot=(0, 0, 22)))
    car.mirror_housing((0.58, 0.95, 0.82))


def build_flanks(car):
    # side-exit pipes under the doors, brackets and a tip behind the front wheel
    for sign in (1, -1):
        z = sign * 0.97
        car.add("chrome", cylinder_x(0.07, 0.07, 1.9, 10, 0.015), xf((0.0, 0.25, z)))
        car.add("trim", cylinder_x(0.045, 0.045, 0.03, 10), xf((-0.96, 0.25, z)))
        for x in (-0.6, 0.0, 0.6):
            car.add("trim", box(0.1, 0.07, 0.09, 0.015, 1), xf((x, 0.27, z - sign * 0.045)))
    car.anchor_pair("anchor_exhaust", (-0.98, 0.25, 0.97))
    car.pair("trim", box(1.2, 0.05, 0.03), (-0.1, 0.55, 0.915))
    car.pair("chrome", box(0.2, 0.05, 0.03), (0.3, 0.7, 0.92))


def build_tail(car):
    for z in (0.62, 0.34, -0.34, -0.62):
        car.add("chrome", cylinder_x(0.13, 0.13, 0.08, 12, 0.01), xf((-2.3, 0.68, z)))
        car.add("brake_light", cylinder_x(0.10, 0.10, 0.04, 12), xf((-2.35, 0.68, z)))
    car.add("chrome", box(0.08, 0.06, 1.8, 0.02, 1), xf((-2.33, 0.36, 0.0)))
    car.add("trim", box(0.34, 0.05, 1.5, 0.01, 1), xf((-2.14, 0.30, 0.0)))
    car.add("paint_accent", box(0.34, 0.07, 1.46, 0.025, 1), xf((-2.2, 0.99, 0.0), rot=(0, 0, 9)))  # ducktail lip
    car.anchor_pair("anchor_tail", (-2.38, 0.68, 0.62))


def build():
    car = CarBuilder(CAR_ID, wheel_radius=R, wheel_width=0.40, wheelbase=2 * AXLE, track=TRACK, hub=hex_linear(ACCENT))
    car.wheel_kwargs = {"spokes": 5, "tread_blocks": 14}
    build_hull(car)
    build_canopy(car)
    build_fenders(car)
    build_nose(car)
    build_flanks(car)
    build_tail(car)
    car.anchor("anchor_weapon", (-0.5, 1.38, 0.0))
    return car.finish()
