"""Support parts under elevated floors in the style of each world (MIT License, see ../LICENSE).

``build(look, stations, number)`` turns the stations of ``support_plan.plan`` into one joined mesh ``supports``
(or None): pirate wooden piles with a cap plank and cross braces, temple stone pillars with a lintel, moon metal
struts with an A-frame leg and clamp bands. The parts of a station stand below the floor edge or outside the
rails (see ``support_plan``), never inside the ball clearance. A loop frame (``kind == "frame"``) adds an arm from
each post to the outer face of the rail.
"""

import math
import random

from . import part_utils as pu
from . import shapes
from .prop import Prop

PLAIN = {"bevel": 0, "dens": 0, "weighted": False, "smooth_angle": 50.0}
TOP_GAP = 0.02              # the top of a cap beam lies this far under the floor edge (the posts end 3 cm lower, inside it)


def _lower(point, drop):
    return (point[0], point[1], point[2] - drop)


def _cap_ends(st):
    """Ends of the cap beam under the floor edge, just outside the edge (centre of the posts)."""
    flat_l = pu.unit((st["across"][0], st["across"][1], 0.0))
    left = pu.at(st["edge_left"], (flat_l, 0.07))
    right = pu.at(st["edge_right"], (flat_l, -0.07))
    return left, right


def _beam(p, st, mat, grad, thickness, depth, extend=0.03):
    """Cap beam across the lane: its top lies ``TOP_GAP`` under the floor edge, its ends inside the post footprint."""
    left, right = _cap_ends(st)
    a, b = _lower(left, TOP_GAP + depth / 2.0), _lower(right, TOP_GAP + depth / 2.0)
    axis = pu.unit(pu.sub(b, a))
    a, b = pu.at(a, (axis, -extend)), pu.at(b, (axis, extend))
    p.add(pu.beam_between("cap", a, b, thickness, depth), mat, grad, ao=0.7, **PLAIN)


# -- pirate ---------------------------------------------------------------------------------------

def _pile(p, post, radius, rng):
    height = post["top"] - post["bottom"]
    pile = shapes.cylinder("pile", radius, radius * 0.86, height, seg=6)
    p.add(pile, "matte", "wood_dark", loc=(post["x"], post["y"], post["bottom"]),
          rot=(rng.uniform(-0.8, 0.8), rng.uniform(-0.8, 0.8), 0.0), ao=0.7, noise_amp=0.1, **PLAIN)


def _pirate(p, st, rng):
    for side in ("left", "right"):
        if st[side]:
            _pile(p, st[side], 0.085 if st["kind"] == "frame" else 0.072, rng)
    if st["kind"] == "beam":
        _beam(p, st, "matte", "wood", 0.13, 0.09)
        _braces(p, st)


def _braces(p, st):
    """Two crossing planks between the posts when both stand and are tall enough."""
    l, r = st["left"], st["right"]
    if not (l and r):
        return
    height = min(l["top"] - l["bottom"], r["top"] - r["bottom"])
    if height < 0.6:
        return
    forward = st["forward"]
    for k, (z0, z1) in enumerate(((0.18, height - 0.25), (height - 0.25, 0.18))):
        off = 0.035 * (1 if k else -1)
        a = (l["x"] + forward[0] * off, l["y"] + forward[1] * off, l["bottom"] + (z0 if k == 0 else z1))
        b = (r["x"] + forward[0] * off, r["y"] + forward[1] * off, r["bottom"] + (z1 if k == 0 else z0))
        p.add(pu.beam_between("brace", a, b, 0.045, 0.1), "matte", "wood", ao=0.7, **PLAIN)
    if height > 1.4:
        a = (l["x"], l["y"], l["bottom"] + height * 0.5)
        b = (r["x"], r["y"], r["bottom"] + height * 0.5)
        p.add(pu.beam_between("tie", a, b, 0.07, 0.07), "matte", "wood", ao=0.7, **PLAIN)


# -- temple ---------------------------------------------------------------------------------------

def _pillar(p, post, half, moss):
    height = post["top"] - post["bottom"]
    x, y, z = post["x"], post["y"], post["bottom"]
    shaft = shapes.cylinder("pillar", half, half * 0.86, max(0.05, height - 0.1), seg=8)
    stops = [(0.0, moss), (0.22, p.pal.c("stone_dark")), (1.0, p.pal.c("stone"))]
    p.add(shaft, "matte", "stone", loc=(x, y, z + 0.1), ao=0.7, mode="z", stops=stops, noise_amp=0.08, **PLAIN)
    foot = shapes.box("pillar-foot", (half * 2.9, half * 2.9, 0.16), base=True)
    p.add(foot, "matte", ("stone_dark", "stone"), loc=(x, y, z), ao=0.7, bevel=0.015, segments=1, dens=0, weighted=False, smooth_angle=50.0)
    cap = shapes.box("pillar-cap", (half * 2.5, half * 2.5, 0.07), base=True)
    p.add(cap, "matte", ("stone", "stone_light"), loc=(x, y, post["top"] - 0.07), ao=0.7, bevel=0.012, segments=1, dens=0, weighted=False,
          smooth_angle=50.0)


def _temple(p, st, rng):
    moss = p.pal.c("moss_dark")
    for side in ("left", "right"):
        if st[side]:
            _pillar(p, st[side], 0.115 if st["kind"] == "frame" else 0.1, moss)
    if st["kind"] == "beam":
        _beam(p, st, "matte", ("stone_dark", "stone"), 0.22, 0.12)
        l, r = st["left"], st["right"]
        if l and r and min(l["top"] - l["bottom"], r["top"] - r["bottom"]) > 1.5:
            a = (l["x"], l["y"], l["bottom"] + 0.7)
            b = (r["x"], r["y"], r["bottom"] + 0.7)
            p.add(pu.beam_between("tie", a, b, 0.14, 0.12), "matte", ("stone_dark", "stone"), ao=0.7, **PLAIN)


# -- moon -----------------------------------------------------------------------------------------

def _strut(p, post, st, radius, side):
    height = post["top"] - post["bottom"]
    x, y, z = post["x"], post["y"], post["bottom"]
    mast = shapes.cylinder("strut", radius, radius * 0.75, height, seg=8)
    p.add(mast, "satin", "panel", loc=(x, y, z), ao=0.7, **PLAIN)
    plate = shapes.cylinder("strut-plate", 0.19, 0.18, 0.05, seg=8)
    p.add(plate, "satin", "metal", loc=(x, y, z), ao=0.7, **PLAIN)
    if height > 0.45:
        band = shapes.cylinder("strut-band", radius + 0.012, radius + 0.012, 0.07, seg=8)
        p.add(band, "satin", "orange", loc=(x, y, z + height * 0.6), ao=0.6, **PLAIN)
    if st["kind"] == "beam" and height > 0.7:
        flat = pu.unit((st["across"][0], st["across"][1], 0.0))
        reach = min(0.55, height * 0.35)
        foot = (x + flat[0] * side * reach, y + flat[1] * side * reach, z + 0.03)
        p.add(pu.beam_between("leg", (x, y, post["top"] - 0.25), foot, 0.06, 0.06), "satin", "metal", ao=0.7, **PLAIN)


def _moon(p, st, rng):
    for side, sign in (("left", 1.0), ("right", -1.0)):
        if st[side]:
            _strut(p, st[side], st, 0.085 if st["kind"] == "frame" else 0.075, sign)
    if st["kind"] == "beam":
        _beam(p, st, "satin", ("panel_dark", "metal"), 0.13, 0.1)
        _braces_moon(p, st)


def _braces_moon(p, st):
    l, r = st["left"], st["right"]
    if not (l and r) or min(l["top"] - l["bottom"], r["top"] - r["bottom"]) < 0.9:
        return
    h = min(l["top"] - l["bottom"], r["top"] - r["bottom"])
    a = (l["x"], l["y"], l["bottom"] + 0.2)
    b = (r["x"], r["y"], r["bottom"] + h - 0.2)
    p.add(pu.beam_between("brace", a, b, 0.04, 0.05), "satin", "metal", ao=0.7, **PLAIN)


STYLES = {"pirate": _pirate, "temple": _temple, "moon": _moon}


def _arms(p, st):
    """Arm from each frame post to the outer face of the rail at the loop's cross-section."""
    centre, across = st["center"], st["across"]
    for side, sign in (("left", 1.0), ("right", -1.0)):
        post = st[side]
        if not post:
            continue
        flat = pu.unit((across[0], across[1], 0.0))
        a = (post["x"], post["y"], post["top"] - 0.06)
        b = pu.at((centre[0], centre[1], post["top"] - 0.06), (flat, sign * 1.02))
        grade = {"pirate": ("matte", "wood"), "temple": ("matte", ("stone_dark", "stone")), "moon": ("satin", "metal")}[p.world]
        p.add(pu.beam_between("arm", a, b, 0.09, 0.09), grade[0], grade[1], ao=0.7, **PLAIN)


def build(look, stations, number):
    """One joined mesh ``supports`` of all stations (None without stations)."""
    if not stations:
        return None
    p = Prop(look, "supports", seed=number, full_name="supports")
    rng = random.Random(number + 8100)
    for st in stations:
        STYLES[look](p, st, rng)
        if st["kind"] == "frame":
            _arms(p, st)
    return p.build()
