"""Tundra: the rally hatch. Short, tall-stanced, black flares, roof scoop and rack, spot lights, mud flaps (MIT, AuraGo contributors).

A square hull with a big upright greenhouse (vertical tailgate with spare tyre), a snorkel up the
right pillar, a roof light bar with four spots, a luggage rack and mud flaps behind every wheel.
About 3.9 x 2.0 x 1.62 m, wheel radius 0.42.
"""

from .cars_common import CarBuilder
from .core import hex_linear
from .shapes import Hull, arc_band, box, cylinder_x, spline, strut, taper_box, torus, xf

CAR_ID = "tundra"
ACCENT = "#2a2a2a"

HULL_X = [-1.85, -1.70, -1.20, -0.50, 0.30, 0.95, 1.40, 1.70, 1.90]
HALF_WIDTH = spline([(-1.85, 0.80), (-1.5, 0.88), (-0.5, 0.90), (0.8, 0.90), (1.5, 0.87), (1.9, 0.80)])
TOP = spline([(-1.85, 0.98), (-1.0, 0.98), (0.4, 1.00), (1.2, 1.02), (1.7, 0.96), (1.9, 0.84)])
BOTTOM = spline([(-1.85, 0.36), (-1.5, 0.32), (1.6, 0.32), (1.9, 0.36)])

CANOPY_X = [-1.80, -1.72, -1.2, -0.5, 0.15, 0.50, 0.78]
CANOPY_HW = spline([(-1.8, 0.70), (-1.7, 0.76), (-1.0, 0.80), (0.1, 0.80), (0.5, 0.74), (0.78, 0.66)])
CANOPY_TOP = spline([(-1.8, 1.34), (-1.72, 1.55), (-1.2, 1.60), (-0.3, 1.60), (0.15, 1.52), (0.5, 1.28), (0.78, 1.02)])

R = 0.42
TRACK = 0.86
AXLE = 1.25


def build_hull(car):
    stations = [
        {"x": x, "hw": HALF_WIDTH(x), "yb": BOTTOM(x), "yt": TOP(x), "power": 4.0, "top_flat": HALF_WIDTH(x) * 0.55, "bot_flat": HALF_WIDTH(x) * 0.5}
        for x in HULL_X
    ]
    hull = Hull(stations, m=6)
    hood = hull.band([hull.TOP], *hull.span(0.95, 1.9))
    under = hull.band([3, 4, 5, 6, 7])
    car.add("paint_accent", hull.shape, faces=hull.pick(hood))
    car.add("trim", hull.shape, faces=hull.pick(under))
    car.add("paint", hull.shape, faces=hull.rest(hood, under))


def build_canopy(car):
    stations = [{"x": x, "hw": CANOPY_HW(x), "yb": 0.84, "yt": CANOPY_TOP(x), "power": 3.2, "top_flat": 0.32} for x in CANOPY_X]
    canopy = Hull(stations, m=6)
    roof = canopy.band([canopy.TOP], *canopy.span(-1.72, 0.15))
    car.add("paint_accent", canopy.shape, faces=canopy.pick(roof))
    car.add("glass", canopy.shape, faces=canopy.rest(roof))
    for k in (1, canopy.n - 2):
        for i in range(len(CANOPY_X) - 1):
            if CANOPY_X[i] >= 0.1:
                car.add("trim", strut(canopy.point(i, k), canopy.point(i + 1, k), 0.04, 4))
    car.driver((-0.15, 0.62, 0.0))
    # snorkel up the right pillar
    car.add("trim", strut((0.55, 0.9, 0.78), (0.55, 1.55, 0.8), 0.045, 6))
    car.add("trim", strut((0.55, 1.55, 0.8), (0.33, 1.66, 0.8), 0.045, 6))
    car.add("trim", cylinder_x(0.065, 0.065, 0.1, 8), xf((0.28, 1.66, 0.8)))


def build_roof(car):
    top = 1.60
    # roof scoop at the front of the roof (dark opening)
    car.add("paint_accent", taper_box(0.56, 0.5, 0.05, 0.5, 0.2, 0.075, 0.025, 1), xf((-0.35, top + 0.06, 0.0)))
    car.add("trim", box(0.04, 0.13, 0.4, 0.008, 1), xf((-0.06, top + 0.17, 0.0)))
    # rack: four feet, two rails, three cross bars and a duffel
    for x in (-1.45, -0.75):
        for z in (0.6, -0.6):
            car.add("trim", box(0.06, 0.1, 0.06), xf((x, top + 0.05, z)))
    for z in (0.6, -0.6):
        car.add("chrome", strut((-1.55, top + 0.12, z), (-0.65, top + 0.12, z), 0.03, 6))
    for x in (-1.5, -1.1, -0.7):
        car.add("chrome", strut((x, top + 0.12, -0.6), (x, top + 0.12, 0.6), 0.025, 6))
    car.add("paint_accent", box(0.55, 0.2, 0.55, 0.05, 1), xf((-1.15, top + 0.26, -0.1)))
    car.add("trim", box(0.58, 0.04, 0.12, 0.01, 1), xf((-1.15, top + 0.31, -0.1)))
    car.add("brake_light", box(0.34, 0.24, 0.18, 0.03, 1), xf((-1.12, top + 0.25, 0.36)))
    car.add("chrome", box(0.06, 0.05, 0.06), xf((-0.98, top + 0.39, 0.36)))
    car.add("trim", strut((-1.48, top + 0.12, -0.52), (-1.68, top + 0.58, -0.52), 0.014, 4))
    # light bar over the windscreen with four spots
    car.add("trim", box(0.12, 0.08, 1.42, 0.02, 1), xf((0.2, top - 0.02, 0.0)))
    for z in (-0.54, -0.18, 0.18, 0.54):
        car.add("chrome", cylinder_x(0.1, 0.1, 0.12, 10, 0.01), xf((0.3, top + 0.1, z)))
        car.add("head_light", cylinder_x(0.075, 0.075, 0.03, 10), xf((0.37, top + 0.1, z)))
    car.anchor("anchor_weapon", (-0.9, top + 0.42, 0.55))


def build_fenders(car):
    for x in (AXLE, -AXLE):
        flare = arc_band(R + 0.04, R + 0.13, 3, 177, 0.45, TRACK + 0.2, 7, 0.03)
        car.pair("paint_accent", flare, (x, R, 0.0))
        # mud flap hanging behind the wheel
        car.pair("trim", box(0.035, 0.3, 0.36, 0.01, 1), (x - R - 0.06, 0.22, TRACK))
        car.pair("chrome", box(0.04, 0.1, 0.2, 0.01, 1), (x - R - 0.06, 0.22, TRACK))


def build_nose(car):
    car.add("trim", box(0.08, 0.3, 1.2, 0.03, 1), xf((1.89, 0.62, 0.0)))
    for z in (-0.35, -0.12, 0.12, 0.35):
        car.add("chrome", box(0.04, 0.2, 0.03), xf((1.935, 0.62, z)))
    car.lamp("head", (1.9, 0.74, 0.7), (0.07, 0.18, 0.26), (0, 0, 0), 0.02, 0.025)
    car.add("trim", box(0.2, 0.12, 1.72, 0.03, 1), xf((1.88, 0.38, 0.0)))
    car.add("chrome", box(0.04, 0.05, 1.5, 0.01, 1), xf((1.99, 0.38, 0.0)))
    for z in (0.6, -0.6):
        car.add("chrome", cylinder_x(0.07, 0.07, 0.1, 8), xf((1.97, 0.5, z)))  # fog lights
    car.add("paint_accent", box(0.34, 0.03, 0.5, 0.01, 1), xf((1.6, TOP(1.6) + 0.015, 0.0)))
    car.anchor_pair("anchor_head", (1.96, 0.74, 0.7))
    car.mirror_housing((0.52, 1.0, 0.82))


def build_tail(car):
    car.lamp("brake", (-1.85, 1.0, 0.64), (0.07, 0.34, 0.16), (0, 0, 0), 0.02, 0.025)
    car.add("trim", box(0.18, 0.12, 1.74, 0.03, 1), xf((-1.84, 0.4, 0.0)))
    car.exhaust(-1.9, 0.36, 0.55, 0.07, 0.22)
    # spare tyre on the tailgate
    car.add("trim", torus(0.25, 0.115, 14, 6), xf((-1.99, 1.08, 0.0), rot=(0, 0, 90)))
    car.add("paint_accent", cylinder_x(0.13, 0.13, 0.05, 10, 0.01), xf((-2.03, 1.08, 0.0)))
    car.anchor_pair("anchor_tail", (-1.91, 1.0, 0.64))


def build():
    car = CarBuilder(CAR_ID, wheel_radius=R, wheel_width=0.38, wheelbase=2 * AXLE, track=TRACK, hub=hex_linear("#ffd23f"))
    car.wheel_kwargs = {"spokes": 6, "tread_blocks": 12, "tread_h": 0.06}
    build_hull(car)
    build_canopy(car)
    build_roof(car)
    build_fenders(car)
    build_nose(car)
    build_tail(car)
    return car.finish()
