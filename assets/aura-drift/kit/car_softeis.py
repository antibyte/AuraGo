"""Softeis: the ice-cream van. Tall rounded box, smiling grille, striped serving hatch with awning,
a huge wobbling swirl cone on the roof and two jingle speakers (MIT, AuraGo contributors).

The cone is a separate ``cone`` object parented to ``body`` with its pivot at the roof joint, so the
runtime can wobble it. About 5.1 x 2.2 x 2.4 m body, cone up to 3.85 m.
"""

import math

from .cars_common import CarBuilder
from .core import Accum, hex_linear
from .shapes import Hull, arc_band, box, cylinder_x, ellipsoid, extrude_z, revolve, spline, sphere, strut, taper_box, xf

CAR_ID = "softeis"
ACCENT = "#fff1c9"

HULL_X = [-2.50, -2.35, -1.90, -0.90, 0.40, 0.90, 1.25, 1.65, 2.10, 2.40, 2.55]
HALF_WIDTH = spline([(-2.5, 0.84), (-2.35, 0.96), (-1.9, 1.04), (0.9, 1.04), (1.25, 1.00), (1.65, 0.96), (2.1, 0.92), (2.55, 0.78)])
TOP = spline([(-2.5, 2.12), (-2.35, 2.32), (-1.9, 2.38), (0.9, 2.38), (1.25, 1.50), (1.65, 1.18), (2.1, 1.10), (2.4, 1.02), (2.55, 0.90)])
BOTTOM = spline([(-2.5, 0.46), (-2.0, 0.40), (2.0, 0.38), (2.55, 0.42)])

R = 0.46
TRACK = 0.94
AXLE = 1.55
ROOF = 2.38
CONE_AT = (-0.75, ROOF, 0.0)


def build_hull(car):
    stations = [
        {"x": x, "hw": HALF_WIDTH(x), "yb": BOTTOM(x), "yt": TOP(x), "power": 3.4, "top_flat": HALF_WIDTH(x) * 0.6, "bot_flat": HALF_WIDTH(x) * 0.5}
        for x in HULL_X
    ]
    hull = Hull(stations, m=6)
    i0, i1 = hull.span(-2.35, 0.9)
    stripe = hull.band([2, 8], i0, i1)
    skirt = hull.band([4, 5, 6])
    car.add("paint_accent", hull.shape, faces=hull.pick(stripe))
    car.add("trim", hull.shape, faces=hull.pick(skirt))
    car.add("paint", hull.shape, faces=hull.rest(stripe, skirt))


def window(car, polygon, z):
    """Dark backing plus glass slab on the flank at |z| = z, on both sides."""
    car.pair("trim", extrude_z(polygon, z - 0.01, z + 0.035, 0.0), (0, 0, 0))
    car.pair("glass", extrude_z(polygon, z + 0.03, z + 0.055, 0.0), (0, 0, 0))


def build_cab(car):
    # windscreen: a tilted slab on the steep front face
    ang = math.degrees(math.atan2(0.35, 0.93))
    mid = ((0.9 + 1.25) / 2.0 + 0.02, (ROOF + 1.5) / 2.0 + 0.02)
    length = math.hypot(0.35, 0.93)
    car.add("trim", box(0.05, length * 0.84, 1.66, 0.02, 1), xf((mid[0], mid[1], 0.0), rot=(0, 0, ang)))
    car.add("glass", box(0.05, length * 0.78, 1.56, 0.02, 1), xf((mid[0] + 0.012, mid[1] + 0.004, 0.0), rot=(0, 0, ang)))
    car.add("chrome", box(0.06, 0.05, 1.7, 0.01, 1), xf((1.22, 1.55, 0.0), rot=(0, 0, ang)))
    side = [(0.34, 1.5), (0.34, 2.15), (0.97, 2.15), (1.2, 1.56)]
    window(car, side, HALF_WIDTH(0.8))
    car.pair("chrome", box(0.12, 0.03, 0.03), (0.52, 1.74, 1.02))  # door handle
    car.mirror_housing((1.22, 1.78, 1.06), size=(0.14, 0.34, 0.12))
    car.anchor("anchor_weapon", (0.55, ROOF + 0.3, 0.0))


def build_serving_hatch(car):
    zs = HALF_WIDTH(-0.8)
    # opening: trim frame, dark interior, chrome counter, menu panel
    car.add("trim", box(1.56, 0.96, 0.06, 0.03, 1), xf((-0.8, 1.62, zs - 0.01)))
    car.add("glass", box(1.36, 0.78, 0.04, 0.02, 1), xf((-0.8, 1.66, zs + 0.025)))
    car.add("chrome", box(1.7, 0.06, 0.3, 0.015, 1), xf((-0.8, 1.14, zs + 0.13)))
    car.add("paint_accent", box(0.5, 0.34, 0.03, 0.01, 1), xf((-1.3, 1.7, zs + 0.05)))
    # striped awning
    colours = ("paint", "paint_accent")
    for i in range(7):
        x = -0.8 - 0.84 + 0.28 * i + 0.14
        car.add(colours[i % 2], box(0.28, 0.05, 0.78, 0.01, 1), xf((x, 2.17, zs + 0.34), rot=(20, 0, 0)))
        car.add(colours[i % 2], box(0.28, 0.17, 0.04, 0.01, 1), xf((x, 2.0 + 0.0, zs + 0.69), rot=(20, 0, 0)))
    for x in (-1.62, -0.0):
        car.add("chrome", strut((x, 1.2, zs + 0.52), (x, 2.0, zs + 0.69), 0.025, 6))
    # tiny sign on a post: a board with a miniature cone
    car.add("trim", strut((0.15, 1.2, zs + 0.52), (0.15, 2.1, zs + 0.52), 0.03, 6))
    car.add("paint_accent", box(0.42, 0.42, 0.05, 0.03, 1), xf((0.15, 2.3, zs + 0.52)))
    car.add("paint", sphere(0.085, 8, 4), xf((0.15, 2.37, zs + 0.56)))
    car.add("trim", revolve([(0.0, -0.13), (0.08, 0.05), (0.0, 0.05)], 6), xf((0.15, 2.28, zs + 0.56)))


def build_speakers(car):
    for z in (0.55, -0.55):
        car.add("trim", box(0.2, 0.1, 0.2, 0.02, 1), xf((0.42, ROOF + 0.05, z)))
        car.add("chrome", cylinder_x(0.05, 0.24, 0.4, 10, 0.01), xf((0.56, ROOF + 0.24, z)))
    car.add("chrome", box(0.22, 0.05, 0.22, 0.02, 1), xf((0.78, ROOF + 0.02, 0.0)))
    car.add("brake_light", sphere(0.11, 8, 4), xf((0.78, ROOF + 0.1, 0.0)))
    zs = HALF_WIDTH(-0.8)
    for dx in (-0.48, -0.34, -0.20):
        car.add("paint_accent", revolve([(0.0, -0.08), (0.045, 0.03), (0.0, 0.03)], 6), xf((dx, 1.23, zs + 0.18)))
        car.add("paint", sphere(0.05, 6, 3), xf((dx, 1.28, zs + 0.18)))


def build_nose(car):
    # the smile: a dark panel with a chrome grin of slats and two big round headlamps
    car.add("trim", box(0.07, 0.4, 1.0, 0.03, 1), xf((2.55, 0.62, 0.0)))
    for k in range(7):
        t = (k - 3) / 3.0
        car.add("chrome", box(0.04, 0.08, 0.07, 0.01, 1), xf((2.6, 0.56 + 0.14 * t * t, t * 0.38)))
    for z in (0.66, -0.66):
        car.add("chrome", cylinder_x(0.17, 0.17, 0.14, 12, 0.015), xf((2.47, 0.62, z)))
        car.add("head_light", cylinder_x(0.13, 0.13, 0.05, 12), xf((2.55, 0.62, z)))
    car.add("chrome", box(0.12, 0.08, 1.9, 0.02, 1), xf((2.52, 0.42, 0.0)))
    car.anchor_pair("anchor_head", (2.6, 0.62, 0.66))


def build_tail(car):
    car.lamp("brake", (-2.46, 0.95, 0.8), (0.07, 0.36, 0.14), (0, 0, 0), 0.02, 0.025)
    car.add("chrome", box(0.12, 0.08, 1.9, 0.02, 1), xf((-2.45, 0.5, 0.0)))
    car.add("trim", box(0.05, 1.3, 0.04), xf((-2.4, 1.3, 0.0)))  # rear door seam
    car.add("glass", box(0.05, 0.5, 0.5, 0.02, 1), xf((-2.46, 1.7, 0.42)))
    car.add("glass", box(0.05, 0.5, 0.5, 0.02, 1), xf((-2.46, 1.7, -0.42)))
    car.exhaust(-2.5, 0.46, 0.45, 0.07, 0.22)
    car.anchor_pair("anchor_tail", (-2.52, 0.95, 0.8))


def build_fenders(car):
    for x in (AXLE, -AXLE + 0.1):
        arch = arc_band(R + 0.04, R + 0.12, 4, 176, 0.5, TRACK + 0.19, 8, 0.03)
        car.pair("paint_accent", arch, (x, R, 0.0))


def cone_object(model, body):
    """Waffle cone (point down), three swirl layers, a curl and a cherry; pivot at the roof joint."""
    waffle = Accum()
    swirl = Accum()
    tan = hex_linear("#d9a05a")
    dark = hex_linear("#b9773a")
    cone = revolve([(0.0, 0.0), (0.12, 0.02), (0.34, 0.2), (0.66, 0.42), (0.68, 0.46), (0.0, 0.46)], 12)
    waffle.add(cone, xf((0, 0.1, 0)), tint=[dark if (i % 2) else tan for i in range(len(cone[0]))])
    waffle.add(revolve([(0.0, 0.0), (0.36, 0.0), (0.4, 0.1), (0.0, 0.1)], 12), None, tint=dark)
    layers = [(0.80, 0.26, 0.64), (0.68, 0.24, 0.82), (0.54, 0.22, 0.98), (0.38, 0.20, 1.11)]
    for i, (r, h, y) in enumerate(layers):
        tone = 1.0 if i % 2 == 0 else 0.88
        swirl.add(ellipsoid(r, h, r, 12, 5), xf((0.04 * i, y, 0.0), rot=(0, 28 * i, 0)), tint=(tone, tone * 0.97, tone * 0.97))
    swirl.add(revolve([(0.0, 0.0), (0.16, 0.0), (0.08, 0.2), (0.0, 0.3)], 8), xf((0.1, 1.17, 0.0), rot=(0, 0, -16)))
    swirl.add(sphere(0.12, 8, 4), xf((0.2, 1.45, 0.0)), tint=hex_linear("#ff3b4a"))
    return model.mesh("cone", [("paint_accent", waffle), ("paint", swirl)], parent=body, location=CONE_AT)


def build():
    car = CarBuilder(CAR_ID, wheel_radius=R, wheel_width=0.40, wheelbase=2 * AXLE - 0.1, track=TRACK, rear_shift=0.05, hub=hex_linear("#ffd1e3"))
    car.wheel_kwargs = {"spokes": 6, "tread_blocks": 13}
    build_hull(car)
    build_cab(car)
    build_serving_hatch(car)
    build_speakers(car)
    build_nose(car)
    build_tail(car)
    build_fenders(car)
    model = car.finish()
    cone_object(model, model.body)
    return model
