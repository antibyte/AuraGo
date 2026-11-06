"""Aura Drift characters: the grinning garden gnome and the rubber duck (MIT, AuraGo contributors).

Both are authored in the character frame (x right, y up, z forward) at 1:1 and returned as a ``Parts``
that callers place with ``Parts.include(part, matrix)`` (mine, statue, duck item).
"""

import math

from .shapes import arc_band, box, cylinder, ellipsoid, sphere, torus
from .toy import Parts, arc_points, ribbon, tube

HAT = "#ff3b30"
JACKET = "#2f78f0"
BEARD = "#fff7ec"
SKIN = "#ffb59a"
NOSE = "#ff7f8a"
BOOT = "#6a3f23"
BELT = "#3a2418"
GOLD = "#ffcc33"
EYE = "#ffffff"
INK = "#1b1b24"


def eye_pair(p, x, y, z, r, look=(0.0, 0.0), lid=None, pupil=0.55, tone=EYE, shine=True, detail=1):
    """Two googly eyes on the surface point (x, y, z); ``look`` shifts the pupils, ``lid`` is a lid colour."""
    for mirror in (False, True):
        p.place("toy", sphere(r, int(6 + 4 * detail), int(3 + 2 * detail)), (x, y, z), tint=tone, mirror=mirror)
        px = look[0] * (-1.0 if mirror else 1.0)
        p.place("toy", sphere(r * pupil, int(5 + 3 * detail), int(3 + detail)), (x + px * r, y + look[1] * r, z + r * 0.62), tint=INK, mirror=mirror)
        if shine:
            p.place("toy", sphere(r * pupil * 0.34, 6, 3), (x + px * r + r * 0.12, y + look[1] * r + r * 0.18, z + r * 1.12), tint="#ffffff", mirror=mirror)
        if lid is not None:
            p.place("toy", sphere(r * 1.07, 10, 4), (x, y + r * 0.34, z - r * 0.02), scale=(1.0, 0.62, 1.0), tint=lid, mirror=mirror)


def gnome():
    """The grinning garden gnome: pointy red hat with a blinking bulb, huge beard, big nose, big grin. ~1.9 m."""
    g = Parts()
    # boots, jacket, belt
    g.pair("toy", ellipsoid(0.15, 0.1, 0.24, 7, 3), (0.15, 0.1, 0.07), tint=BOOT)
    g.place("toy", ellipsoid(0.37, 0.42, 0.33, 10, 5), (0, 0.5, 0), tint=JACKET)
    g.place("toy", cylinder(0.335, 0.335, 0.09, 10), (0, 0.4, 0), tint=BELT)
    g.place("toy", box(0.15, 0.12, 0.05), (0, 0.4, 0.335), tint=GOLD)
    # arms with mittens
    g.pair("toy", ellipsoid(0.1, 0.24, 0.1, 6, 3), (0.4, 0.52, 0.08), rot=(-25, 0, -22), tint=JACKET)
    g.pair("toy", sphere(0.1, 6, 3), (0.47, 0.34, 0.2), tint=BEARD)
    # head
    g.place("toy", sphere(0.29, 10, 5), (0, 1.0, 0), tint=SKIN)
    g.pair("matte", sphere(0.075, 6, 3), (0.17, 0.93, 0.2), scale=(1.0, 0.8, 0.5), tint="#ff8f93")  # rosy cheeks
    eye_pair(g, 0.105, 1.07, 0.23, 0.075, look=(-0.1, -0.05), shine=False, detail=0)
    g.pair("fuzz", box(0.15, 0.04, 0.06), (0.105, 1.165, 0.255), rot=(0, 0, 14), tint=BEARD)  # bushy brows
    g.place("toy", sphere(0.088, 8, 4), (0, 0.985, 0.29), tint=NOSE)
    # moustache curling up into the cheeks, beard, grin
    for mirror in (False, True):
        g.place("fuzz", ellipsoid(0.15, 0.055, 0.075, 6, 3), (0.115, 0.93, 0.255), rot=(0, 0, 18), tint=BEARD, mirror=mirror)
    g.place("fuzz", ellipsoid(0.27, 0.36, 0.2, 8, 4), (0, 0.58, 0.14), tint=BEARD)
    g.place("matte", arc_band(0.085, 0.125, 205, 335, 0.0, 0.05, 6), (0, 0.9, 0.275), tint="#4a0f1b")
    g.pair("toy", box(0.045, 0.05, 0.03), (0.055, 0.82, 0.3), tint="#ffffff")
    # hat: cone, brim, pom with the blinking bulb
    g.place("toy", cylinder(0.33, 0.33, 0.07, 10), (0, 1.16, 0), tint="#d92a22")
    g.place("toy", cylinder(0.31, 0.015, 0.78, 10), (0, 1.52, 0), rot=(-8, 0, 0), tint=HAT)
    g.place("glow_red", sphere(0.1, 8, 4), (0, 1.93, -0.054), tint="#ffd0c0")
    g.place("toy", torus(0.3, 0.03, 10, 4, 1.2), tint=GOLD)
    return g


def duck():
    """Irresistible rubber duck, ~1.3 m long, beak toward +X: round head, big wet eyes, pink cheeks."""
    d = Parts()
    Y, O, P = "#ffd21f", "#ff8a1f", "#ff8fa0"
    d.place("toy", ellipsoid(0.52, 0.4, 0.46, 14, 7), (0, 0.42, -0.04), tint=Y)  # body
    d.place("toy", ellipsoid(0.26, 0.22, 0.3, 8, 4), (0, 0.48, -0.44), rot=(-28, 0, 0), tint=Y)  # tail flick
    d.place("toy", sphere(0.36, 14, 7), (0, 0.98, 0.2), tint=Y)  # head
    d.pair("toy", ellipsoid(0.1, 0.22, 0.34, 8, 4), (0.5, 0.5, -0.06), rot=(-8, 0, -12), tint="#ffbf14")  # wings
    d.place("toy", ellipsoid(0.17, 0.065, 0.2, 10, 4), (0, 0.9, 0.55), tint=O)  # beak, upper
    d.place("toy", ellipsoid(0.15, 0.04, 0.16, 8, 4), (0, 0.83, 0.54), tint="#ee7414")  # beak, lower
    d.pair("matte", ellipsoid(0.07, 0.05, 0.03, 8, 3), (0.2, 0.86, 0.46), tint=P)  # cheeks
    for mirror in (False, True):
        d.place("toy", sphere(0.105, 10, 5), (0.15, 1.08, 0.46), tint="#ffffff", mirror=mirror)
        d.place("toy", sphere(0.07, 8, 4), (0.14, 1.07, 0.55), tint="#1a1a24", mirror=mirror)
        d.place("toy", sphere(0.025, 6, 3), (0.12, 1.11, 0.6), tint="#ffffff", mirror=mirror)
    # a small curl of head feathers
    d.place("toy", tube([(0, 1.3, 0.22), (0, 1.42, 0.28), (0, 1.5, 0.2)], [0.045, 0.035, 0.02], 5), tint=Y)
    return d
