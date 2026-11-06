"""Kobalt: the elegant GT. Very long bonnet, cabin set back, one sweeping fastback (MIT, AuraGo contributors).

Lofted hull with a thin accent stripe over bonnet, roof and tail, round chrome headlamp pods, an oval
grille, fender gills, chrome belt line and twin round tail lamps. About 4.7 x 1.9 x 1.2 m.
"""

from .cars_common import CarBuilder
from .core import hex_linear
from .shapes import Hull, arc_band, box, cylinder_x, spline, strut, taper_box, xf

CAR_ID = "kobalt"
ACCENT = "#ffe066"

HULL_X = [-2.30, -2.12, -1.75, -1.25, -0.65, 0.0, 0.65, 1.15, 1.65, 2.00, 2.25, 2.40]
HALF_WIDTH = spline([(-2.3, 0.56), (-2.12, 0.70), (-1.7, 0.78), (-1.2, 0.80), (-0.4, 0.77), (0.6, 0.78), (1.4, 0.80), (1.95, 0.74), (2.2, 0.66), (2.4, 0.54)])
TOP = spline([(-2.3, 0.78), (-2.1, 0.86), (-1.6, 0.88), (-0.9, 0.84), (0.0, 0.82), (1.0, 0.84), (1.6, 0.78), (2.05, 0.66), (2.4, 0.46)])
BOTTOM = spline([(-2.3, 0.34), (-2.0, 0.24), (-1.0, 0.20), (2.0, 0.20), (2.4, 0.24)])

CANOPY_X = [-2.05, -1.75, -1.35, -0.95, -0.5, -0.05, 0.3, 0.62]
CANOPY_HW = spline([(-2.05, 0.34), (-1.75, 0.50), (-1.35, 0.60), (-0.8, 0.65), (-0.2, 0.65), (0.3, 0.58), (0.62, 0.50)])
CANOPY_TOP = spline([(-2.05, 0.88), (-1.75, 1.04), (-1.35, 1.19), (-0.9, 1.29), (-0.5, 1.30), (-0.05, 1.22), (0.3, 1.05), (0.62, 0.86)])

R = 0.38
TRACK = 0.76
AXLE = 1.40


def build_hull(car):
    stations = [
        {"x": x, "hw": HALF_WIDTH(x), "yb": BOTTOM(x), "yt": TOP(x), "power": 3.2, "top_flat": min(0.11, HALF_WIDTH(x) * 0.3), "bot_flat": HALF_WIDTH(x) * 0.5}
        for x in HULL_X
    ]
    hull = Hull(stations, m=6)
    stripe = hull.band([hull.TOP], *hull.span(-2.3, 2.4))
    under = hull.band([3, 4, 5, 6, 7])
    car.add("paint_accent", hull.shape, faces=hull.pick(stripe))
    car.add("trim", hull.shape, faces=hull.pick(under))
    car.add("paint", hull.shape, faces=hull.rest(stripe, under))


def build_canopy(car):
    stations = [{"x": x, "hw": CANOPY_HW(x), "yb": 0.68, "yt": CANOPY_TOP(x), "power": 2.5, "top_flat": 0.11} for x in CANOPY_X]
    canopy = Hull(stations, m=6)
    roof = canopy.band([canopy.TOP], *canopy.span(-2.05, 0.3))
    quarter = canopy.band([canopy.TOP, 0, 1], *canopy.span(-2.05, -1.35))  # solid body-coloured C pillars of the fastback
    car.add("paint_accent", canopy.shape, faces=canopy.pick(roof))
    car.add("paint", canopy.shape, faces=canopy.pick([i for i in quarter if i not in roof]))
    car.add("glass", canopy.shape, faces=canopy.rest(roof, quarter))
    for k in (1, canopy.n - 2):
        for i in range(len(CANOPY_X) - 1):
            if CANOPY_X[i] >= -1.35:
                car.add("chrome", strut(canopy.point(i, k), canopy.point(i + 1, k), 0.022, 4))
    car.driver((-0.55, 0.44, 0.0))


def build_fenders(car):
    for x, flare in ((AXLE, 0.07), (-AXLE, 0.11)):
        arch = arc_band(R + 0.04, R + flare, 4, 176, 0.34, TRACK + 0.07, 8, 0.025)
        car.pair("paint", arch, (x, R, 0.0))
    # fender gills behind the front wheel and a chrome waist line
    for dx in (-0.12, 0.0, 0.12):
        car.pair("chrome", box(0.04, 0.2, 0.05, 0.008, 1), (0.82 + dx, 0.47, 0.775))
    car.pair("trim", box(0.5, 0.2, 0.04), (0.82, 0.47, 0.77))
    car.pair("chrome", box(2.9, 0.026, 0.03, 0.006, 1), (-0.35, 0.50, 0.805))


def build_nose(car):
    car.add("chrome", box(0.07, 0.24, 0.66, 0.05, 2), xf((2.38, 0.34, 0.0)))
    car.add("trim", box(0.08, 0.17, 0.56, 0.04, 1), xf((2.395, 0.34, 0.0)))
    for z in (-0.18, 0.0, 0.18):
        car.add("chrome", box(0.04, 0.14, 0.02), xf((2.43, 0.34, z)))
    for z in (0.52, -0.52):  # round headlamp pods
        car.add("chrome", cylinder_x(0.145, 0.15, 0.17, 12, 0.012), xf((2.18, 0.55, z)))
        car.add("head_light", cylinder_x(0.11, 0.11, 0.05, 12), xf((2.27, 0.55, z)))
    car.add("chrome", box(0.05, 0.035, 1.5, 0.008, 1), xf((2.40, 0.20, 0.0)))
    car.add("trim", taper_box(0.3, 1.46, 0.035, 1.54, 0.04, 0.0, 0.01, 1), xf((2.2, 0.13, 0.0)))
    car.anchor_pair("anchor_head", (2.34, 0.55, 0.52))
    car.mirror_housing((0.72, 0.84, 0.72))


def build_tail(car):
    for z in (0.5, -0.5):
        car.add("chrome", cylinder_x(0.13, 0.13, 0.08, 12, 0.01), xf((-2.3, 0.68, z)))
        car.add("brake_light", cylinder_x(0.10, 0.10, 0.04, 12), xf((-2.35, 0.68, z)))
    car.add("brake_light", box(0.03, 0.03, 0.5, 0.006, 1), xf((-2.31, 0.58, 0.0)))
    car.add("chrome", box(0.05, 0.035, 1.2, 0.008, 1), xf((-2.32, 0.40, 0.0)))
    car.add("trim", box(0.4, 0.05, 1.1, 0.012, 1), xf((-2.14, 0.30, 0.0)))
    car.exhaust(-2.15, 0.36, 0.34, 0.07, 0.26)
    # tiny ducktail spoiler
    car.add("paint", box(0.22, 0.05, 1.0, 0.02, 1), xf((-2.20, 0.88, 0.0), rot=(0, 0, 8)))
    car.anchor_pair("anchor_tail", (-2.36, 0.68, 0.5))


def build():
    car = CarBuilder(CAR_ID, wheel_radius=R, wheel_width=0.34, wheelbase=2 * AXLE, track=TRACK, hub=hex_linear(ACCENT))
    car.wheel_kwargs = {"spokes": 7, "tread_blocks": 13}
    build_hull(car)
    build_canopy(car)
    build_fenders(car)
    build_nose(car)
    build_tail(car)
    car.anchor("anchor_weapon", (-0.5, 1.32, 0.0))
    return car.finish()
