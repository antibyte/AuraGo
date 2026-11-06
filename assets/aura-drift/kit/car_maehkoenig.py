"""Maehkoenig: the ride-on lawnmower king (MIT, AuraGo contributors).

An out-front mower: open cutting deck with a spinning ``blade`` object, stubby bonnet with a hood
crown, a chute pipe along the right flank to a big grass-catcher bag, a roll bar with a pennant and
a grinning garden ``gnome`` (red hat, white beard, blue jacket) leaning at the wheel. The gnome uses
``paint_accent`` with vertex colours, so the livery accent should be a near-white cream.
About 2.7 x 1.3 x 1.6 m, wheel radius 0.31.
"""

import math

from .cars_common import CarBuilder
from .core import Accum, hex_linear
from .shapes import Hull, arc_band, box, cylinder, cylinder_x, ellipsoid, extrude_z, revolve, sphere, strut, torus, xf

CAR_ID = "maehkoenig"

R = 0.31
TRACK = 0.52
FRONT_X = 0.50
REAR_X = -0.70
DECK_X = 1.25
SEAT = (-0.35, 0.62, 0.0)  # gnome pivot (hip)

BLUE = hex_linear("#2f6fd0")
DENIM = hex_linear("#274fa8")
SKIN = hex_linear("#f2b38f")
NOSE = hex_linear("#ff7f6f")
BEARD = hex_linear("#fbfbf6")
HAT = hex_linear("#e32f2a")
BOOT = hex_linear("#6b4423")
GOLD = hex_linear("#ffc83a")


def build_chassis(car):
    car.add("trim", box(1.9, 0.1, 0.5, 0.02, 1), xf((-0.05, 0.27, 0.0)))
    car.add("trim", box(1.25, 0.06, 0.18), xf((0.35, 0.34, 0.0)))
    for x, z in ((FRONT_X, 0.0), (REAR_X, 0.0)):
        car.add("chrome", strut((x, R, -TRACK + 0.1), (x, R, TRACK - 0.1), 0.035, 6))
    # hood, with a stripe down the middle
    stations = [
        {"x": x, "hw": hw, "yb": 0.3, "yt": yt, "power": 3.0, "top_flat": hw * 0.45, "bot_flat": hw * 0.5}
        for x, hw, yt in ((0.28, 0.25, 0.70), (0.42, 0.28, 0.80), (0.72, 0.28, 0.76), (0.90, 0.22, 0.62))
    ]
    hood = Hull(stations, m=6)
    stripe = hood.band([hood.TOP], 0, 3)
    car.add("paint_accent", hood.shape, faces=hood.pick(stripe))
    car.add("paint", hood.shape, faces=hood.rest(stripe))
    # grille and round lamps
    car.add("trim", box(0.05, 0.26, 0.3, 0.02, 1), xf((0.91, 0.5, 0.0)))
    for z in (0.15, -0.15):
        car.add("chrome", box(0.03, 0.2, 0.025), xf((0.945, 0.5, z)))
    for z in (0.3, -0.3):
        car.add("chrome", cylinder_x(0.09, 0.09, 0.09, 10, 0.01), xf((0.86, 0.62, z)))
        car.add("head_light", cylinder_x(0.07, 0.07, 0.03, 10), xf((0.92, 0.62, z)))
    car.anchor_pair("anchor_head", (0.95, 0.62, 0.3))
    # hood crown
    crown = Accum()
    crown.add(cylinder(0.1, 0.12, 0.08, 10), xf((0, 0.04, 0)), tint=GOLD)
    for k in range(5):
        a = math.tau * k / 5.0
        crown.add(revolve([(0.0, 0.0), (0.035, 0.0), (0.0, 0.1)], 5), xf((0.1 * math.cos(a), 0.08, 0.1 * math.sin(a))), tint=GOLD)
    car.acc["chrome"].add((crown.verts, crown.faces), xf((0.58, 0.8, 0.0)), tint=GOLD)
    # dash, steering column and wheel, little wind deflector
    car.add("trim", box(0.18, 0.2, 0.46, 0.03, 1), xf((0.2, 0.62, 0.0)))
    car.add("trim", strut((0.2, 0.7, 0.0), (0.12, 0.97, 0.0), 0.035, 6))
    car.add("trim", torus(0.14, 0.022, 10, 5), xf((0.12, 0.99, 0.0), rot=(0, 0, -22)))
    car.add("glass", box(0.03, 0.26, 0.4, 0.01, 1), xf((0.27, 0.95, 0.0), rot=(0, 0, 12)))
    # seat
    car.add("trim", box(0.42, 0.1, 0.46, 0.03, 1), xf((SEAT[0], 0.5, 0.0)))
    car.add("trim", box(0.1, 0.38, 0.42, 0.03, 1), xf((SEAT[0] - 0.22, 0.72, 0.0), rot=(0, 0, 8)))
    car.add("paint_accent", box(0.34, 0.03, 0.38, 0.01, 1), xf((SEAT[0], 0.56, 0.0)))
    # footrests
    car.add("trim", box(0.3, 0.04, 0.5, 0.01, 1), xf((0.1, 0.32, 0.0)))


def build_fenders(car):
    pod = arc_band(R + 0.04, R + 0.11, 5, 175, 0.28, TRACK + 0.17, 7, 0.02)
    car.pair("paint", pod, (REAR_X, R, 0.0))


def build_deck(car):
    wall = revolve([(0.33, 0.0), (0.41, 0.0), (0.41, 0.28), (0.33, 0.28)], 14, closed=True)
    car.add("paint", wall, xf((DECK_X, 0.14, 0.0)))
    car.add("trim", cylinder(0.4, 0.4, 0.05, 14), xf((DECK_X, 0.15, 0.0)))
    car.add("trim", box(0.12, 0.14, 0.2, 0.02, 1), xf((DECK_X - 0.28, 0.3, 0.0)))
    # discharge chute and pipe to the catcher along the right flank
    car.add("paint", box(0.2, 0.16, 0.16, 0.03, 1), xf((DECK_X - 0.12, 0.34, 0.4)))
    pts = [(DECK_X - 0.2, 0.36, 0.38), (0.55, 0.5, 0.33), (-0.1, 0.58, 0.33), (-0.62, 0.72, 0.3)]
    for p, q in zip(pts, pts[1:]):
        car.add("trim", strut(p, q, 0.05, 6))
    for p in pts[1:3]:
        car.add("chrome", sphere(0.065, 7, 4), xf(p))
    # front guard bar
    for z in (0.34, -0.34):
        car.add("chrome", strut((DECK_X + 0.2, 0.3, z), (DECK_X + 0.46, 0.34, z * 0.9), 0.028, 6))
    car.add("chrome", strut((DECK_X + 0.46, 0.34, 0.31), (DECK_X + 0.46, 0.34, -0.31), 0.03, 6))


def build_catcher(car):
    x = -0.97
    car.add("trim", box(0.56, 0.46, 0.62, 0.06, 1), xf((x, 0.62, 0.0)))
    car.add("paint", box(0.6, 0.08, 0.66, 0.03, 1), xf((x, 0.88, 0.0)))
    car.add("paint_accent", box(0.1, 0.06, 0.3, 0.02, 1), xf((x, 0.95, 0.0)))  # lid handle
    for dz in (-0.2, -0.07, 0.07, 0.2):
        car.add("paint_accent", box(0.02, 0.38, 0.045), xf((x - 0.285, 0.62, dz)))
    for dx in (-0.17, 0.0, 0.17):
        car.pair("paint_accent", box(0.045, 0.38, 0.02), (x + dx, 0.62, 0.315))
    car.add("trim", box(0.2, 0.3, 0.46, 0.02, 1), xf((-0.68, 0.45, 0.0)))  # mount bracket
    car.lamp("brake", (-1.25, 0.5, 0.22), (0.05, 0.1, 0.14), (0, 0, 0), 0.01, 0.015)
    car.exhaust(-1.1, 0.27, 0.16, 0.045, 0.2)
    car.anchor_pair("anchor_tail", (-1.28, 0.5, 0.22))


def build_rollbar(car):
    x = -0.62
    for z in (0.36, -0.36):
        car.add("chrome", strut((x, 0.4, z), (x - 0.04, 1.42, z * 0.8), 0.035, 6))
    car.add("chrome", strut((x - 0.04, 1.42, 0.29), (x - 0.04, 1.42, -0.29), 0.035, 6))
    car.add("trim", strut((x - 0.04, 1.42, 0.0), (x - 0.04, 1.78, 0.0), 0.02, 4))
    car.add("paint", extrude_z([(0.0, 0.0), (0.0, 0.2), (-0.34, 0.1)], -0.015, 0.015), xf((x - 0.04, 1.58, 0.0)))
    car.anchor("anchor_weapon", (x, 1.5, 0.0))


def gnome_object(model, body):
    """Gnome pivoted at the hip, local frame = glTF axes: knees forward, hat up."""
    g = Accum()
    k = Accum()
    for s in (1, -1):
        g.add(box(0.46, 0.16, 0.14, 0.03, 1), xf((0.22, 0.02, s * 0.1)), tint=DENIM)
        g.add(box(0.15, 0.36, 0.14, 0.03, 1), xf((0.43, -0.14, s * 0.1)), tint=DENIM)
        g.add(box(0.3, 0.12, 0.17, 0.04, 1), xf((0.52, -0.34, s * 0.1)), tint=BOOT)
        a = (0.0, 0.43, s * 0.23)
        g.add(strut(a, (0.4, 0.34, s * 0.14), 0.055, 6), tint=BLUE)
        g.add(sphere(0.065, 7, 4), xf((0.43, 0.34, s * 0.13)), tint=SKIN)
        k.add(sphere(0.032, 6, 3), xf((0.205, 0.76, s * 0.085)))
        k.add(box(0.03, 0.03, 0.1), xf((0.2, 0.84, s * 0.085), rot=(s * 10, 0, 14 * s)))  # eyebrows
    g.add(ellipsoid(0.21, 0.28, 0.25, 10, 5), xf((0.0, 0.3, 0.0)), tint=BLUE)
    k.add(box(0.215, 0.05, 0.5, 0.01, 1), xf((0.0, 0.1, 0.0)))  # belt
    g.add(box(0.03, 0.07, 0.07), xf((0.115, 0.1, 0.0)), tint=GOLD)
    g.add(sphere(0.18, 10, 5), xf((0.04, 0.7, 0.0)), tint=SKIN)
    g.add(sphere(0.07, 8, 4), xf((0.22, 0.66, 0.0)), tint=NOSE)
    g.add(ellipsoid(0.16, 0.26, 0.19, 8, 4), xf((0.11, 0.5, 0.0)), tint=BEARD)
    g.add(ellipsoid(0.08, 0.05, 0.12, 6, 3), xf((0.2, 0.56, 0.0)), tint=BEARD)  # moustache
    g.add(revolve([(0.0, 0.0), (0.24, 0.0), (0.22, 0.06), (0.14, 0.1), (0.0, 0.62)], 10), xf((0.0, 0.82, 0.0), rot=(0, 0, 12)), tint=HAT)
    g.add(sphere(0.05, 6, 3), xf((-0.16, 1.4, 0.0)), tint=BEARD)
    return model.mesh("gnome", [("paint_accent", g), ("trim", k)], parent=body, location=SEAT)


def blade_object(model, body):
    b = Accum()
    steel = hex_linear("#e3e7ee")
    for rot in (0, 90):
        b.add(box(0.74, 0.022, 0.09), xf((0, 0, 0), rot=(0, rot, 0)), tint=steel)
        for s in (1, -1):
            tip = xf((0, 0, 0), rot=(0, rot, 0)) @ xf((s * 0.35, 0.035, 0.0), rot=(0, 0, s * -35))
            b.add(box(0.1, 0.05, 0.085), tip, tint=steel)
    hub = Accum()
    hub.add(cylinder(0.09, 0.09, 0.06, 8), None, tint=hex_linear("#c2c8d2"))
    return model.mesh("blade", [("chrome", b), ("trim", hub)], parent=body, location=(DECK_X, 0.3, 0.0))


def build():
    car = CarBuilder(CAR_ID, wheel_radius=R, wheel_width=0.28, wheelbase=FRONT_X - REAR_X, track=TRACK, rear_shift=(FRONT_X + REAR_X) / 2.0, hub=hex_linear("#fff7e6"))
    car.wheel_kwargs = {"spokes": 5, "tread_blocks": 12, "tread_h": 0.05}
    build_chassis(car)
    build_fenders(car)
    build_deck(car)
    build_catcher(car)
    build_rollbar(car)
    model = car.finish()
    gnome_object(model, model.body)
    blade_object(model, model.body)
    return model
