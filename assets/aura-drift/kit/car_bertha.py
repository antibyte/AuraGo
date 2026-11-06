"""Big Bertha: the monster truck (MIT, AuraGo contributors).

Enormous tyres (radius 0.95) outside a narrow pickup body, a tall ladder frame on chrome axles with
shock absorbers and link bars, fender flares, a rear roll cage over the bed, side-exit exhaust stacks,
a roof light bar, a bull bar and flame panels (accent colour with a livery-coloured core).
About 5.3 x 3.0 x 3.1 m.
"""

from .cars_common import CarBuilder
from .core import hex_linear
from .shapes import Hull, arc_band, box, cylinder, cylinder_x, extrude_z, prism, spline, sphere, strut, taper_box, xf

CAR_ID = "bertha"
ACCENT = "#2a2a2a"

R = 0.95
TRACK = 1.14
AXLE = 1.5
AX_Y = R

HULL_X = [-2.55, -2.40, -1.80, -0.90, -0.10, 0.50, 1.10, 1.80, 2.35, 2.62]
HALF_WIDTH = spline([(-2.55, 0.52), (-2.35, 0.70), (-1.6, 0.72), (0.0, 0.72), (1.6, 0.72), (2.3, 0.68), (2.62, 0.58)])
TOP = spline([(-2.55, 2.22), (-2.2, 2.28), (-1.0, 2.26), (0.0, 2.28), (0.8, 2.40), (1.7, 2.34), (2.3, 2.12), (2.62, 1.98)])
BOTTOM = spline([(-2.55, 1.62), (-2.0, 1.56), (2.2, 1.56), (2.62, 1.66)])

CAB_X = [-1.05, -0.85, -0.30, 0.15, 0.50, 0.80]
CAB_HW = spline([(-1.05, 0.74), (-0.85, 0.88), (-0.3, 0.90), (0.2, 0.88), (0.5, 0.80), (0.8, 0.70)])
CAB_TOP = spline([(-1.05, 2.70), (-0.85, 2.88), (-0.3, 2.92), (0.1, 2.86), (0.5, 2.62), (0.8, 2.36)])


def flame(car, x0, y0, tongues, scale=1.0):
    """Row of flame tongues on both flanks, front point at x0, sweeping toward -x."""
    for k, (dy, length, height) in enumerate(tongues):
        pts = [
            (x0, y0 + dy),
            (x0 - 0.25 * length * scale, y0 + dy + 0.55 * height * scale),
            (x0 - 0.55 * length * scale, y0 + dy + 0.35 * height * scale),
            (x0 - length * scale, y0 + dy + 0.95 * height * scale),
            (x0 - 0.8 * length * scale, y0 + dy + 0.2 * height * scale),
            (x0 - 0.5 * length * scale, y0 + dy - 0.05 * height * scale),
        ]
        z = HALF_WIDTH(x0 - length * 0.4)
        for material, inset, lift in (("paint_accent", 0.0, 0.0), ("paint", 0.25, 0.012)):
            if inset:
                cx = sum(p[0] for p in pts) / len(pts)
                cy = sum(p[1] for p in pts) / len(pts)
                use = [(cx + (px - cx) * 0.55, cy + (py - cy) * 0.5) for px, py in pts]
            else:
                use = pts
            shape = extrude_z(use, z - 0.02, z + 0.025 + lift)
            car.pair(material, shape, (0, 0, 0))


def build_body(car):
    stations = [
        {"x": x, "hw": HALF_WIDTH(x), "yb": BOTTOM(x), "yt": TOP(x), "power": 3.6, "top_flat": HALF_WIDTH(x) * 0.45, "bot_flat": HALF_WIDTH(x) * 0.5}
        for x in HULL_X
    ]
    hull = Hull(stations, m=6)
    hood = hull.band([hull.TOP], *hull.span(0.5, 2.62))
    under = hull.band([4, 5, 6])
    car.add("paint_accent", hull.shape, faces=hull.pick(hood))
    car.add("trim", hull.shape, faces=hull.pick(under))
    car.add("paint", hull.shape, faces=hull.rest(hood, under))

    stations = [{"x": x, "hw": CAB_HW(x), "yb": 2.1, "yt": CAB_TOP(x), "power": 3.0, "top_flat": 0.35} for x in CAB_X]
    cab = Hull(stations, m=6)
    roof = cab.band([cab.TOP, 0, cab.n - 2], *cab.span(-1.05, 0.15))
    doors = cab.band([3, 4, 5, 6, 7])
    car.add("paint", cab.shape, faces=cab.pick(roof + doors))
    car.add("glass", cab.shape, faces=cab.rest(roof, doors))
    car.driver((-0.38, 2.12, 0.0))
    # bonnet scoop
    car.add("paint_accent", taper_box(0.8, 0.7, 0.05, 0.7, 0.26, 0.105, 0.03, 1), xf((1.35, TOP(1.35) + 0.04, 0.0)))
    car.add("trim", box(0.04, 0.18, 0.5, 0.01, 1), xf((1.75, TOP(1.35) + 0.16, 0.0)))
    flame(car, 0.62, 1.62, ((0.0, 1.45, 0.5), (0.22, 1.1, 0.4)))
    flame(car, 2.45, 1.66, ((0.0, 0.5, 0.3),), 0.9)
    # fender flares over the giant tyres, mounted on the body
    for x in (AXLE, -AXLE):
        car.pair("paint", arc_band(R + 0.08, R + 0.2, 2, 178, 0.42, TRACK - 0.12, 12, 0.03), (x, AX_Y, 0.0))


def build_chassis(car):
    for z in (0.5, -0.5):
        car.add("trim", box(4.9, 0.18, 0.2, 0.02, 1), xf((0.0, 1.3, z)))
    for x in (-2.1, -0.6, 0.6, 2.1):
        car.add("trim", box(0.16, 0.14, 1.1), xf((x, 1.33, 0.0)))
    for x in (AXLE, -AXLE):
        car.add("chrome", strut((x, AX_Y, -TRACK + 0.2), (x, AX_Y, TRACK - 0.2), 0.12, 8))
        car.add("trim", sphere(0.28, 10, 5), xf((x + (0.12 if x > 0 else -0.12), AX_Y, 0.0)))
        for z in (0.55, -0.55):
            car.add("trim", strut((x, AX_Y + 0.05, z), (x + (-1.0 if x > 0 else 1.0) * 0.8, 1.3, z * 0.9), 0.06, 6))  # link bars
        for z in (0.8, -0.8):
            car.add("paint_accent", cylinder(0.085, 0.085, 0.7, 8), xf((x + 0.15, 1.32, z), rot=(0, 0, 0)))
            car.add("chrome", cylinder(0.04, 0.04, 0.75, 6), xf((x + 0.15, 1.1, z)))
            car.add("trim", cylinder(0.12, 0.12, 0.06, 8), xf((x + 0.15, 1.02, z)))
            car.add("trim", cylinder(0.12, 0.12, 0.06, 8), xf((x + 0.15, 1.67, z)))
    # skid plate
    car.add("trim", taper_box(0.7, 1.4, 0.06, 1.5, 0.08, 0.0, 0.015, 1), xf((2.3, 1.3, 0.0)))


def build_nose(car):
    car.add("trim", box(0.1, 0.42, 1.15, 0.04, 1), xf((2.6, 1.84, 0.0)))
    for z in (-0.36, -0.12, 0.12, 0.36):
        car.add("chrome", box(0.05, 0.3, 0.05, 0.01, 1), xf((2.67, 1.84, z)))
    for z in (0.52, -0.52):
        car.add("chrome", cylinder_x(0.2, 0.2, 0.14, 12, 0.015), xf((2.55, 2.18, z)))
        car.add("head_light", cylinder_x(0.155, 0.155, 0.05, 12), xf((2.63, 2.18, z)))
    # bull bar
    car.add("chrome", strut((2.75, 1.5, -0.7), (2.75, 1.5, 0.7), 0.06, 8))
    car.add("chrome", strut((2.75, 2.05, -0.5), (2.75, 2.05, 0.5), 0.05, 8))
    for z in (0.7, -0.7, 0.0):
        car.add("chrome", strut((2.75, 1.5, z), (2.75, 2.05, z * 0.7), 0.05, 6))
    car.add("trim", box(0.2, 0.2, 0.4, 0.03, 1), xf((2.72, 1.72, 0.0)))  # winch
    car.anchor_pair("anchor_head", (2.68, 2.18, 0.52))
    car.mirror_housing((0.35, 2.55, 0.9), size=(0.22, 0.16, 0.14))


def build_roof_and_cage(car):
    top = 2.92
    # light bar: four big spots
    car.add("trim", box(0.14, 0.1, 1.5, 0.02, 1), xf((0.0, top + 0.05, 0.0)))
    for z in (-0.54, -0.18, 0.18, 0.54):
        car.add("chrome", cylinder_x(0.11, 0.11, 0.14, 10, 0.01), xf((0.1, top + 0.14, z)))
        car.add("head_light", cylinder_x(0.085, 0.085, 0.04, 10), xf((0.19, top + 0.14, z)))
    # roll cage over the bed
    for x in (-1.2, -2.3):
        for z in (0.64, -0.64):
            car.add("chrome", strut((x, 2.2, z), (x, 3.05, z * 0.92), 0.05, 6))
        car.add("chrome", strut((x, 3.05, 0.59), (x, 3.05, -0.59), 0.05, 6))
    for z in (0.59, -0.59):
        car.add("chrome", strut((-1.2, 3.05, z), (-2.3, 3.05, z), 0.05, 6))
    car.add("chrome", strut((-1.2, 2.3, 0.64), (-2.3, 3.0, -0.6), 0.04, 6))
    car.add("chrome", strut((-1.2, 2.3, -0.64), (-2.3, 3.0, 0.6), 0.04, 6))
    car.anchor("anchor_weapon", (-1.75, 3.1, 0.0))


def build_tail(car):
    # exhaust stacks on both sides of the cage
    for z in (0.86, -0.86):
        car.add("chrome", cylinder(0.11, 0.1, 1.0, 10, 0.015), xf((-1.0, 2.6, z)))
        car.add("trim", cylinder(0.075, 0.075, 0.04, 10), xf((-1.0, 3.12, z)))
        car.add("trim", box(0.22, 0.2, 0.2, 0.02, 1), xf((-0.98, 2.15, z * 0.95)))
    car.anchor_pair("anchor_exhaust", (-1.0, 3.18, 0.86))
    car.lamp("brake", (-2.55, 1.98, 0.5), (0.07, 0.2, 0.3), (0, 0, 0), 0.02, 0.025)
    car.add("trim", box(0.14, 0.18, 1.5, 0.03, 1), xf((-2.55, 1.76, 0.0)))
    car.add("chrome", box(0.08, 0.06, 1.3, 0.015, 1), xf((-2.6, 1.55, 0.0)))
    car.anchor_pair("anchor_tail", (-2.6, 1.98, 0.5))
    # tow hitch
    car.add("trim", box(0.3, 0.1, 0.12, 0.02, 1), xf((-2.7, 1.45, 0.0)))


def build():
    car = CarBuilder(CAR_ID, wheel_radius=R, wheel_width=0.72, wheelbase=2 * AXLE, track=TRACK, hub=hex_linear("#ffd23a"))
    car.wheel_kwargs = {"spokes": 8, "tread_blocks": 15, "tread_h": 0.13, "rim_ratio": 0.55}
    build_chassis(car)
    build_body(car)
    build_nose(car)
    build_roof_and_cage(car)
    build_tail(car)
    return car.finish()
