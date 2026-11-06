"""Aura Drift weather props: six extra toy-diorama models for a living sky (MIT, AuraGo contributors).

Two flavours, because the two kit halves use different collectors:

* ``windsock`` and ``rain_shelter`` return ``toy.Parts`` and are attached to ``shared.glb`` by
  ``shared_props.build()``. They use the toy materials (``toy``, ``matte``, ``chrome``, ``glow_*``).
* ``snow_gauge``, ``anemometer``, ``storm_lantern`` and ``snow_fence`` fill a scenery ``Piece``
  passed in by the caller and use only materials common to scenery A and B (``prop`` plus
  ``glow_warm`` / ``glow_cyan``). Call ``piece.bake()`` before exporting.

Everything stands on y=0, is centred on its footprint and faces +X. Shapes stay coarse (6 to 10
segments) so one prop costs roughly 250 to 700 triangles and the pack keeps its byte budget.
"""

import math

from .shapes import box, cylinder, cylinder_x, ellipsoid, extrude_z, sphere, strut, torus, xf
from .toy import Parts

ORANGE, CREAM, NAVY, STEEL = "#ff8a1f", "#fff7ec", "#252f5c", "#c9d3e3"
RED, SNOW, ICE, WOOD = "#e8262d", "#f2f7ff", "#8cc8f0", "#8a5a34"


# --------------------------------------------------------------------------
# shared.glb: toy materials, attached through shared_props.attach_prop()
# --------------------------------------------------------------------------


def windsock():
    """A striped orange-and-cream wind sock on a ring mast: the wind is visible from the stands.

    Drawn at racing distance, so the mast rings and the sock bands share one part each: every part
    of a furniture prop is its own instanced draw call, and the ultra tier is at its call budget.
    """
    p = Parts()
    p.add("matte", box(1.1, 0.18, 1.1, 0.04, 1), xf((0, 0.09, 0)), tint=NAVY)
    p.add("toy", cylinder(0.09, 0.15, 6.1, 8), xf((0, 3.05, 0)), tint=STEEL)
    p.add("chrome", torus(0.16, 0.045, 8, 3, y=3.4), xf((0, 0, 0)), tint=CREAM)
    p.add("chrome", torus(0.52, 0.05, 10, 4, y=6.12), tint=STEEL)
    p.add("glow_cyan", sphere(0.16, 6, 3), xf((0, 6.42, 0)), tint="#dffaff")
    # Five tapered bands read as a sock stretched downwind along +X; alternating tints in one part.
    for k in range(5):
        r0, r1 = 0.5 - k * 0.075, 0.5 - (k + 1) * 0.075
        p.add("toy", cylinder_x(r0, r1, 0.56, 8), xf((0.55 + k * 0.56, 6.12, 0)),
              tint=ORANGE if k % 2 == 0 else CREAM)
    p.add("toy", torus(0.13, 0.035, 8, 3, y=6.12), xf((3.35, 0, 0)), tint=ORANGE)
    p.add("matte", extrude_z([(0.0, 0.0), (0.0, 0.5), (0.85, 0.25)], -0.02, 0.02),
          xf((-0.12, 5.55, 0), rot=(0, 90, 0)), tint=RED)
    return p


def rain_shelter():
    """A small covered grandstand box: striped valance, bench, lamps and a rain barrel.

    One part per material rather than per element: the valance stripes, the posts and the bench legs
    would otherwise cost three draw calls each at every tier.
    """
    p = Parts()
    p.add("matte", box(4.2, 0.16, 2.8, 0.05, 1), xf((0, 0.08, 0)), tint="#3a4763")
    for sx in (-1.9, 1.9):
        for sz in (-1.2, 1.2):
            p.add("toy", cylinder(0.09, 0.1, 2.9, 6), xf((sx, 1.45, sz)), tint=STEEL)
    p.add("toy", box(4.7, 0.18, 3.1, 0.05, 1), xf((0, 3.0, 0)), tint=NAVY)
    p.add("toy", box(4.9, 0.1, 3.3, 0.03, 1), xf((0, 3.14, 0)), tint=CREAM)
    for k in range(9):
        p.add("toy", box(0.44, 0.44, 0.07), xf((-2.05 + k * 0.51, 2.72, 1.6)),
              tint=RED if k % 2 == 0 else CREAM)
    p.add("toy", box(3.6, 0.14, 0.7), xf((0, 0.78, -0.65)), tint=WOOD)
    for sx in (-1.5, 1.5):
        p.add("toy", box(0.14, 0.6, 0.6), xf((sx, 0.38, -0.65)), tint=NAVY)
    for sx in (-1.45, 1.45):
        p.add("glow_white", sphere(0.18, 6, 3), xf((sx, 2.65, -1.1)), tint="#fff1d0")
        p.add("matte", cylinder(0.06, 0.06, 0.34, 5), xf((sx, 2.9, -1.1)), tint=STEEL)
    p.add("toy", cylinder(0.42, 0.46, 1.05, 8), xf((2.35, 0.52, 1.05)), tint="#2f78f0")
    p.add("chrome", cylinder(0.44, 0.44, 0.08, 8), xf((2.35, 1.06, 1.05)), tint=STEEL)
    return p


# --------------------------------------------------------------------------
# scenery kits: fill the caller's Piece, using only the shared scenery materials
# --------------------------------------------------------------------------


def snow_gauge(p):
    """Alpine: a red-and-white graduated snow pole with a collection funnel and a snow cap."""
    p.add(cylinder(0.36, 0.44, 0.18, 8), xf((0, 0.09, 0)), "#8a97a8", grad=0.25)
    p.add(cylinder(0.075, 0.085, 3.1, 6), xf((0, 1.6, 0)), RED, grad=0.2)
    for k in range(6):
        p.add(box(0.22, 0.07, 0.22), xf((0.02, 0.62 + k * 0.42, 0)), CREAM, grad=0.0, jitter=0.0)
    p.add(cylinder(0.48, 0.17, 0.42, 8), xf((0, 3.3, 0)), ICE, grad=0.2)
    p.add(sphere(0.15, 6, 3), xf((0, 3.58, 0)), SNOW, grad=0.0, jitter=0.0)
    p.add(strut((-0.42, 2.52, 0), (0.42, 2.52, 0), 0.035, 4), None, STEEL, grad=0.0, jitter=0.0)
    return p


def anemometer(p):
    """Desert: three bright cups on a slim mast, an unmistakable little wind measurer."""
    p.add(cylinder(0.42, 0.5, 0.22, 8), xf((0, 0.11, 0)), "#b58a4e", grad=0.25)
    p.add(cylinder(0.08, 0.12, 4.2, 6), xf((0, 2.1, 0)), "#8a97a8", grad=0.25)
    for k in range(3):
        a = k * 2 * math.pi / 3
        x, z = 1.05 * math.cos(a), 1.05 * math.sin(a)
        p.add(strut((0, 4.25, 0), (x, 4.25, z), 0.045, 5), None, STEEL, grad=0.0, jitter=0.0)
        p.add(sphere(0.27, 7, 4), xf((x, 4.25, z)), "#e8563f", grad=0.2)
        p.add_flat(sphere(0.1, 5, 3), xf((x, 4.42, z)), "#ffffff", "glow_warm")
    p.add(sphere(0.19, 6, 3), xf((0, 4.25, 0)), "#ffd21f", grad=0.0, jitter=0.0)
    return p


def storm_lantern(p):
    """Harbour: a caged storm beacon with a rain hood, for wet night reflections."""
    p.add(cylinder(0.5, 0.6, 0.5, 8), xf((0, 0.25, 0)), "#25324a", grad=0.25)
    p.add(cylinder(0.12, 0.15, 3.0, 7), xf((0, 1.75, 0)), "#8fa0b8", grad=0.25)
    p.add(cylinder(0.58, 0.58, 0.1, 8), xf((0, 3.08, 0)), STEEL, grad=0.0, jitter=0.0)
    p.add_flat(sphere(0.4, 8, 5), xf((0, 3.48, 0)), "#ffffff", "glow_warm")
    for k in range(4):
        a = k * math.pi / 2
        x, z = 0.42 * math.cos(a), 0.42 * math.sin(a)
        p.add(strut((x, 3.08, z), (x, 3.92, z), 0.035, 4), None, STEEL, grad=0.0, jitter=0.0)
    p.add(cylinder(0.56, 0.42, 0.22, 8), xf((0, 4.02, 0)), "#25324a", grad=0.2)
    p.add(cylinder(0.16, 0.0, 0.36, 6), xf((0, 4.28, 0)), RED, grad=0.0, jitter=0.0)
    p.add_flat(box(0.7, 0.12, 0.12), xf((0.55, 2.5, 0)), "#ffffff", "glow_cyan")
    return p


def snow_fence(p):
    """Glacier: a red-and-white slatted drift fence with snow piled against its foot."""
    for sx in (-2.4, 0.0, 2.4):
        p.add(cylinder(0.09, 0.11, 2.2, 6), xf((sx, 1.1, 0)), WOOD, grad=0.25)
    for k in range(5):
        p.add(box(5.2, 0.16, 0.1), xf((0, 0.45 + k * 0.36, 0)),
              SNOW if k % 2 else RED, grad=0.15, jitter=0.0)
    for sx in (-1.6, 0.3, 1.8):
        p.add(ellipsoid(0.9, 0.28, 0.55, 8, 4), xf((sx, 0.16, 0.25)), SNOW, grad=0.15)
    return p
