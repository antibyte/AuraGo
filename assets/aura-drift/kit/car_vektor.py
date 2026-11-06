"""Vektor: the wedge. Low chisel nose, cab-forward glass canopy, big rear wing (MIT, AuraGo contributors).

Reference build for the kit: a lofted hull with coloured face bands, a glass canopy with a
paint_accent roof, flared wheel arches, splitter, diffuser, side scoops, lamps, tailpipes and a
driver. Size about 4.4 x 1.9 x 1.15 m, wheelbase 2.7, wheel radius 0.38.
"""

from .cars_common import CarBuilder
from .core import hex_linear
from .shapes import Hull, arc_band, box, spline, strut, taper_box, xf

CAR_ID = "vektor"
ACCENT = "#fff7e6"

HULL_X = [-2.20, -2.05, -1.75, -1.35, -0.95, -0.45, 0.05, 0.55, 0.95, 1.35, 1.70, 1.95, 2.20]
HALF_WIDTH = spline([(-2.2, 0.58), (-2.05, 0.70), (-1.7, 0.78), (-1.35, 0.80), (-0.5, 0.76), (0.5, 0.74), (1.35, 0.80), (1.8, 0.76), (2.05, 0.70), (2.2, 0.60)])
TOP = spline([(-2.2, 0.74), (-2.05, 0.86), (-1.7, 0.92), (-1.0, 0.84), (-0.2, 0.78), (0.7, 0.75), (1.35, 0.74), (1.95, 0.60), (2.2, 0.50)])
BOTTOM = spline([(-2.2, 0.34), (-1.9, 0.24), (-1.0, 0.20), (1.9, 0.19), (2.2, 0.20)])

CANOPY_X = [-1.65, -1.30, -0.90, -0.40, 0.10, 0.55, 1.00]
CANOPY_HW = spline([(-1.65, 0.34), (-1.3, 0.50), (-0.9, 0.60), (-0.4, 0.64), (0.1, 0.64), (0.55, 0.58), (1.0, 0.44)])
CANOPY_TOP = spline([(-1.65, 0.84), (-1.3, 0.96), (-0.9, 1.06), (-0.4, 1.10), (0.1, 1.08), (0.55, 0.97), (1.0, 0.79)])

R = 0.38
TRACK = 0.78
AXLE = 1.35


def build_hull(car):
    stations = [
        {"x": x, "hw": HALF_WIDTH(x), "yb": BOTTOM(x), "yt": TOP(x), "power": 3.4, "top_flat": min(0.20, HALF_WIDTH(x) * 0.4), "bot_flat": HALF_WIDTH(x) * 0.5}
        for x in HULL_X
    ]
    hull = Hull(stations, m=6)
    hood = hull.band([hull.TOP], *hull.span(0.55, 2.2))
    deck = hull.band([hull.TOP], *hull.span(-2.2, -1.35))
    shoulder = hull.band([1], *hull.span(-1.35, 1.35))
    under = hull.band([3, 4, hull.BOTTOM])
    car.add("paint_accent", hull.shape, faces=hull.pick(hood + deck + shoulder))
    car.add("trim", hull.shape, faces=hull.pick(under))
    car.add("paint", hull.shape, faces=hull.rest(hood, deck, shoulder, under))


def build_canopy(car):
    stations = [{"x": x, "hw": CANOPY_HW(x), "yb": 0.66, "yt": CANOPY_TOP(x), "power": 2.6, "top_flat": 0.16} for x in CANOPY_X]
    canopy = Hull(stations, m=6)
    roof = canopy.band([canopy.TOP, 0], *canopy.span(-0.9, 0.1))
    car.add("paint_accent", canopy.shape, faces=canopy.pick(roof))
    car.add("glass", canopy.shape, faces=canopy.rest(roof))
    # window frame: A pillars follow the glass shoulder, thin rails run back along the roof edge
    for k in (1, canopy.n - 2):
        for i in range(len(CANOPY_X) - 1):
            if CANOPY_X[i] >= -0.95:
                car.add("trim", strut(canopy.point(i, k), canopy.point(i + 1, k), 0.028, 4))
    car.driver((-0.10, 0.40, 0.0))


def build_fenders(car):
    for x, flare in ((AXLE, 0.12), (-AXLE, 0.17)):
        arch = arc_band(R + 0.04, R + flare, 4, 176, TRACK - 0.19, TRACK + 0.08, 8, 0.025)
        car.pair("paint", arch, (x, R, 0.0))


def build_nose(car):
    car.add("trim", box(0.07, 0.11, 0.62, 0.02, 1), xf((2.18, 0.31, 0.0)))
    car.add("chrome", box(0.04, 0.02, 0.62, 0.006, 1), xf((2.215, 0.385, 0.0)))
    car.lamp("head", (2.19, 0.37, 0.44), (0.07, 0.09, 0.26), (0, 0, 0), 0.015, 0.02)
    car.add("trim", taper_box(0.44, 1.52, 0.04, 1.64, 0.045, 0.0, 0.012, 1), xf((1.99, 0.125, 0.0)))
    car.pair("trim", box(0.3, 0.09, 0.03), (1.99, 0.16, 0.82))
    for z in (0.35, -0.35):
        car.add("trim", box(0.08, 0.08, 0.04, 0.01, 1), xf((1.82, 0.16, z)))
    for z in (0.1, -0.1):
        car.add("trim", box(0.26, 0.014, 0.05), xf((1.62, TOP(1.62) + 0.004, z), rot=(0, 0, -13)))
    car.anchor_pair("anchor_head", (2.24, 0.37, 0.44))


def build_flanks(car):
    car.pair("trim", box(0.5, 0.2, 0.06, 0.03, 1), (-0.72, 0.5, 0.78))
    for dx in (-0.17, 0.0, 0.17):
        car.pair("paint_accent", box(0.03, 0.16, 0.075, 0.006, 1), (-0.72 + dx, 0.5, 0.78))
    car.pair("trim", box(1.9, 0.08, 0.06, 0.015, 1), (0.0, 0.2, 0.78))
    car.mirror_housing((0.52, 0.86, 0.72))


def build_tail(car):
    car.lamp("brake", (-2.2, 0.58, 0.36), (0.07, 0.10, 0.42), (0, 0, 0), 0.015, 0.02)
    car.add("brake_light", box(0.05, 0.03, 0.3, 0.008, 1), xf((-2.19, 0.58, 0.0)))
    car.add("trim", box(0.4, 0.05, 1.14, 0.015, 1), xf((-2.06, 0.30, 0.0), rot=(0, 0, 10)))
    for z in (-0.5, -0.2, 0.2, 0.5):
        car.add("trim", box(0.5, 0.12, 0.03, 0.006, 1), xf((-2.05, 0.30, z)))
    car.exhaust(-2.08, 0.46, 0.30)
    car.anchor_pair("anchor_tail", (-2.24, 0.58, 0.36))
    car.wing(-2.0, 1.02, 1.74, chord=0.62, thick=0.07, aoa=7.0, plate_h=0.20, plate_len=0.74, pylons=0.36, deck_y=0.84)


def build():
    car = CarBuilder(CAR_ID, wheel_radius=R, wheel_width=0.34, wheelbase=2 * AXLE, track=TRACK, hub=hex_linear(ACCENT))
    build_hull(car)
    build_canopy(car)
    build_fenders(car)
    build_nose(car)
    build_flanks(car)
    build_tail(car)
    car.anchor("anchor_weapon", (-0.35, 1.13, 0.0))
    return car.finish()
