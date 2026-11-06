"""Couch-Potato: an overstuffed sofa on small wheels (MIT, AuraGo contributors).

Roll arms, three back and three seat cushions with cream buttons, a fringe along the front, a floor
``lamp`` (swinging child, pivot at its foot), a loose ``cushion`` child, a trailing side table with a
remote and a mug, and a sleepy driver in a burgundy dressing gown and pink slippers steering a tiny
wheel with one hand and holding the remote with the other. The driver is ``paint_accent`` with vertex
colours, so the livery accent should be a near-white cream. About 3.0 x 2.2 x 2.1 m, lamp to 2.55 m.
"""

import math

from .cars_common import CarBuilder
from .core import Accum, hex_linear
from .shapes import box, cylinder, cylinder_x, ellipsoid, revolve, sphere, strut, torus, xf

CAR_ID = "couch"

R = 0.34
TRACK = 0.76
AXLE = 1.0

ROBE = hex_linear("#9b2f52")
CREAM = hex_linear("#fbf3df")
PYJAMA = hex_linear("#79a8de")
SKIN = hex_linear("#f2b38f")
HAIR = hex_linear("#4a2f1d")
SLIPPER = hex_linear("#ff8fb7")
CUSHION_PIVOT = (0.2, 0.74, -0.47)
LAMP_PIVOT = (-1.28, 1.18, -0.74)


def build_sofa(car):
    car.add("paint", box(2.8, 0.34, 1.5, 0.1, 2), xf((-0.02, 0.56, 0.0)))
    car.add("trim", box(2.9, 0.1, 1.42, 0.03, 1), xf((-0.02, 0.36, 0.0)))
    # roll arms and the back
    for s in (1, -1):
        car.add("paint", box(2.7, 0.6, 0.26, 0.11, 2), xf((-0.02, 0.98, s * 0.73)))
        car.add("paint_accent", box(2.7, 0.04, 0.05, 0.01, 1), xf((-0.02, 1.27, s * 0.73)))
    car.add("paint", box(0.5, 0.92, 1.72, 0.12, 2), xf((-1.15, 1.17, 0.0)))
    for z in (-0.5, 0.0, 0.5):
        car.add("paint", box(0.3, 0.66, 0.5, 0.1, 2), xf((-0.84, 1.18, z)))
        for dy in (0.12, -0.12):
            car.add("paint_accent", sphere(0.04, 6, 3), xf((-0.67, 1.18 + dy, z)))
    # seat cushions: the left one is the loose ``cushion`` object
    for z in (0.0, 0.47):
        car.add("paint", box(1.0, 0.26, 0.46, 0.1, 2), xf((0.2, 0.86, z)))
        car.add("paint_accent", sphere(0.04, 6, 3), xf((0.2, 1.0, z)))
    # fringe along the front edge and a plinth with the lamps
    for k in range(10):
        car.add("paint_accent", box(0.05, 0.16, 0.1, 0.01, 1), xf((1.4, 0.44, -0.67 + k * 0.149)))
    car.add("trim", box(0.12, 0.2, 1.3, 0.03, 1), xf((1.43, 0.6, 0.0)))
    for z in (0.5, -0.5):
        car.add("chrome", cylinder_x(0.13, 0.13, 0.08, 10, 0.01), xf((1.47, 0.6, z)))
        car.add("head_light", cylinder_x(0.1, 0.1, 0.04, 10), xf((1.52, 0.6, z)))
    car.anchor_pair("anchor_head", (1.54, 0.6, 0.5))
    car.lamp("brake", (-1.43, 0.5, 0.52), (0.05, 0.12, 0.2), (0, 0, 0), 0.01, 0.02)
    car.add("trim", box(0.1, 0.12, 1.5, 0.02, 1), xf((-1.42, 0.38, 0.0)))
    car.exhaust(-1.46, 0.36, 0.28, 0.055, 0.22)
    car.anchor_pair("anchor_tail", (-1.48, 0.5, 0.52))
    car.anchor("anchor_weapon", (-1.15, 1.72, 0.0))


def build_steering(car):
    car.add("trim", strut((0.95, 0.72, 0.0), (0.62, 1.22, 0.0), 0.035, 6))
    car.add("trim", torus(0.15, 0.025, 10, 5), xf((0.6, 1.27, 0.0), rot=(0, 0, 40)))
    car.add("chrome", sphere(0.04, 6, 3), xf((0.6, 1.27, 0.0)))


def build_table(car):
    """Side table riding along on the right: bracket, pole, top, remote and a mug."""
    x, z = 0.15, 1.12
    car.add("trim", box(0.14, 0.1, 0.3, 0.02, 1), xf((x, 0.8, 0.9)))
    car.add("chrome", strut((x, 0.04, z), (x, 1.04, z), 0.04, 6))
    car.add("trim", cylinder(0.14, 0.14, 0.04, 8), xf((x, 0.02, z)))
    car.add("paint_accent", cylinder(0.3, 0.3, 0.05, 10, 0.01), xf((x, 1.06, z)))
    car.add("trim", box(0.28, 0.035, 0.1, 0.012, 1), xf((x + 0.02, 1.1, z + 0.06), rot=(0, 24, 0)))
    car.add("brake_light", box(0.03, 0.02, 0.03), xf((x + 0.11, 1.13, z + 0.095), rot=(0, 24, 0)))
    car.add("paint_accent", box(0.03, 0.02, 0.03), xf((x - 0.04, 1.13, z + 0.04), rot=(0, 24, 0)))
    car.add("paint", cylinder(0.065, 0.06, 0.12, 8), xf((x - 0.1, 1.15, z - 0.12)))
    car.add("trim", cylinder(0.04, 0.04, 0.02, 8), xf((x - 0.1, 1.215, z - 0.12)))
    car.add("glass", cylinder(0.07, 0.055, 0.17, 8), xf((x + 0.1, 1.18, z - 0.1)))  # a tall glass of lemonade


def build_driver(car):
    """The couch potato: slumped, sleepy, one hand on the wheel, one on the remote."""
    robe = ROBE
    car.add("paint_accent", ellipsoid(0.27, 0.34, 0.31, 10, 5), xf((-0.53, 1.4, 0.04), rot=(0, 0, 20)), tint=robe)
    car.add("paint_accent", ellipsoid(0.23, 0.12, 0.34, 8, 4), xf((-0.4, 1.12, 0.0)), tint=robe)
    car.add("paint_accent", box(0.3, 0.07, 0.54, 0.02, 1), xf((-0.42, 1.27, 0.0), rot=(0, 0, 12)), tint=CREAM)  # belt
    for s in (1, -1):
        car.add("paint_accent", strut((-0.3, 1.1, s * 0.14), (0.35, 1.17, s * 0.2), 0.115, 7), tint=PYJAMA)
        car.add("paint_accent", strut((0.35, 1.17, s * 0.2), (0.95, 0.84, s * 0.21), 0.1, 7), tint=PYJAMA)
        car.add("paint_accent", ellipsoid(0.22, 0.1, 0.12, 8, 4), xf((1.06, 0.8, s * 0.21), rot=(0, 0, 12)), tint=SLIPPER)
        car.add("paint_accent", ellipsoid(0.07, 0.07, 0.08, 6, 3), xf((1.2, 0.82, s * 0.21)), tint=CREAM)  # pompom
    # arms: right to the wheel, left holding the remote in the air
    car.add("paint_accent", strut((-0.5, 1.62, 0.3), (0.0, 1.3, 0.42), 0.075, 6), tint=robe)
    car.add("paint_accent", strut((0.0, 1.3, 0.42), (0.55, 1.28, 0.14), 0.065, 6), tint=robe)
    car.add("paint_accent", sphere(0.07, 6, 3), xf((0.58, 1.28, 0.12)), tint=SKIN)
    car.add("paint_accent", strut((-0.5, 1.62, -0.3), (-0.1, 1.45, -0.46), 0.075, 6), tint=robe)
    car.add("paint_accent", strut((-0.1, 1.45, -0.46), (0.22, 1.55, -0.3), 0.065, 6), tint=robe)
    car.add("paint_accent", sphere(0.07, 6, 3), xf((0.25, 1.56, -0.28)), tint=SKIN)
    car.add("trim", box(0.24, 0.035, 0.09, 0.012, 1), xf((0.34, 1.6, -0.24), rot=(0, 15, 12)))
    # head lolling to one side: skin, hair, sleepy eyes, open snoring mouth, big nose
    head = (-0.68, 1.84, 0.1)
    car.add("paint_accent", sphere(0.22, 10, 5), xf(head, rot=(14, 0, 14)), tint=SKIN)
    car.add("paint_accent", sphere(0.07, 6, 3), xf((head[0] + 0.21, head[1] - 0.03, head[2])), tint=hex_linear("#ee9a85"))
    car.add("paint_accent", ellipsoid(0.24, 0.13, 0.25, 8, 4), xf((head[0] - 0.04, head[1] + 0.16, head[2])), tint=HAIR)
    car.add("paint_accent", revolve([(0.0, 0.0), (0.06, 0.0), (0.0, 0.16)], 5), xf((head[0] - 0.02, head[1] + 0.27, head[2]), rot=(0, 0, 22)), tint=HAIR)
    for s in (1, -1):
        car.add("trim", box(0.03, 0.025, 0.09), xf((head[0] + 0.19, head[1] + 0.06, head[2] + s * 0.09), rot=(0, 0, 0)))
    car.add("trim", ellipsoid(0.03, 0.045, 0.06, 6, 3), xf((head[0] + 0.2, head[1] - 0.11, head[2])))


def lamp_object(model, body):
    chrome, shade, glow = Accum(), Accum(), Accum()
    chrome.add(cylinder(0.2, 0.2, 0.05, 10, 0.01), xf((0, 0.03, 0)))
    chrome.add(strut((0, 0.05, 0), (0, 1.0, 0), 0.03, 6))
    shade.add(cylinder(0.34, 0.2, 0.4, 10, 0.01), xf((0, 1.2, 0.0)), tint=(1.0, 0.95, 0.8))
    glow.add(sphere(0.09, 7, 4), xf((0, 1.03, 0)))
    return model.mesh("lamp", [("chrome", chrome), ("paint_accent", shade), ("head_light", glow)], parent=body, location=LAMP_PIVOT)


def cushion_object(model, body):
    c, b = Accum(), Accum()
    c.add(box(1.0, 0.26, 0.46, 0.1, 2), xf((0.0, 0.12, 0.0)))
    b.add(sphere(0.04, 6, 3), xf((0.0, 0.26, 0.0)), tint=CREAM)
    # a throw pillow resting on top
    b.add(box(0.5, 0.34, 0.1, 0.05, 1), xf((-0.2, 0.42, 0.0), rot=(0, 0, -62)), tint=hex_linear("#f2a65a"))
    return model.mesh("cushion", [("paint", c), ("paint_accent", b)], parent=body, location=CUSHION_PIVOT)


def build():
    car = CarBuilder(CAR_ID, wheel_radius=R, wheel_width=0.28, wheelbase=2 * AXLE, track=TRACK, hub=hex_linear("#f3e3c3"))
    car.wheel_kwargs = {"spokes": 5, "tread_blocks": 12, "tread_h": 0.05}
    build_sofa(car)
    build_steering(car)
    build_table(car)
    build_driver(car)
    model = car.finish()
    lamp_object(model, model.body)
    cushion_object(model, model.body)
    return model
