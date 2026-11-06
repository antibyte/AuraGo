"""Aura Drift polish props: six extra toy-diorama models for more trackside variety (MIT, AuraGo contributors).

Two flavours live here because the two kit halves use different collectors:

* ``balloon_bunch`` and ``photo_tower`` return ``toy.Parts`` and are attached to ``shared.glb``
  by ``shared_props.build()``. They use the toy materials (``toy``, ``matte``, ``chrome``, ``glow_*``).
* ``trail_marker``, ``sunshade``, ``signal_mast`` and ``holo_arrow`` fill a scenery ``Piece`` passed
  in by the caller and use only the materials common to scenery A and B (``prop`` and
  ``glow_warm`` / ``glow_pink`` / ``glow_cyan``). Call ``piece.bake()`` before exporting.

Everything stands on y=0, is centred on its footprint and faces +X. Shapes stay coarse (6 to 10
segments) so a prop costs roughly 250 to 400 triangles and the shared pack keeps its byte budget.
"""

import math

from .shapes import box, cylinder, cylinder_x, ellipsoid, extrude_z, sphere, strut, torus, xf
from .toy import Parts

PARTY = ["#ff4d6d", "#ffd166", "#06d6a0", "#4cc9f0", "#b388ff", "#ff9f1c"]
CREAM, NAVY, STEEL, WOOD = "#fff7ec", "#252f5c", "#c9d3e3", "#a8743a"


# --------------------------------------------------------------------------
# shared.glb: toy materials, attached through shared_props.attach_prop()
# --------------------------------------------------------------------------


def balloon_bunch():
    """A tethered bunch of seven glossy balloons on a striped post: colour near the grandstands."""
    p = Parts()
    p.add("matte", box(1.0, 0.2, 1.0, 0.04, 1), xf((0, 0.1, 0)), tint=NAVY)
    p.add("toy", cylinder(0.07, 0.07, 4.0, 6), xf((0, 2.1, 0)), tint=STEEL)
    for k in range(8):
        y = 0.35 + k * 0.42
        p.add("toy", cylinder(0.085, 0.085, 0.2, 6), xf((0, y, 0)), tint=PARTY[k % 2] if k % 2 == 0 else CREAM)
    p.add("chrome", torus(0.46, 0.05, 8, 4, y=4.15), tint=STEEL)
    for k, (dx, dz, dy) in enumerate([(0.0, 0.0, 0.85), (0.48, 0.28, 0.25), (0.1, 0.52, 0.3), (-0.44, 0.3, 0.2),
                                     (-0.46, -0.28, 0.35), (0.05, -0.52, 0.22), (0.46, -0.3, 0.42)]):
        tint = PARTY[k % len(PARTY)]
        p.add("toy", cylinder(0.012, 0.012, 0.9 + dy, 4), xf((dx * 0.7, 4.15 + (dy + 0.5) * 0.5, dz * 0.7)), tint=CREAM)
        p.add("toy", ellipsoid(0.44, 0.52, 0.44, 10, 6), xf((dx, 4.75 + dy, dz)), tint=tint)
        p.add("glow_white", sphere(0.09, 5, 3), xf((dx - 0.16, 4.98 + dy, dz + 0.28)), tint="#ffffff")
        p.add("toy", cylinder(0.06, 0.0, 0.16, 5), xf((dx, 4.18 + dy, dz)), tint=tint)
    return p


def photo_tower():
    """A slender camera scaffold with a glowing tally lamp and a little pennant."""
    p = Parts()
    for sx in (-0.72, 0.72):
        for sz in (-0.72, 0.72):
            p.add("toy", box(0.14, 5.2, 0.14), xf((sx, 2.6, sz)), tint=STEEL)
            p.add("matte", box(0.34, 0.16, 0.34), xf((sx, 0.08, sz)), tint=NAVY)
    for y in (1.3, 2.7, 4.1):
        for sz in (-0.72, 0.72):
            p.add("toy", box(1.6, 0.09, 0.09), xf((0, y, sz)), tint=NAVY)
        for sx in (-0.72, 0.72):
            p.add("toy", box(0.09, 0.09, 1.6), xf((sx, y, 0)), tint=NAVY)
        p.add("chrome", strut((-0.72, y, 0.72), (0.72, y + 1.4, 0.72), 0.045, 4), tint=STEEL)
    p.add("toy", box(2.0, 0.16, 2.0, 0.04, 1), xf((0, 5.28, 0)), tint=NAVY)
    for sz in (-0.9, 0.9):
        p.add("toy", box(1.9, 0.08, 0.08), xf((0, 6.0, sz)), tint=CREAM)
    p.add("toy", box(0.95, 0.62, 0.78, 0.06, 1), xf((0.1, 5.72, 0)), tint="#2f78f0")
    p.add("chrome", cylinder_x(0.26, 0.32, 0.55, 8), xf((0.82, 5.74, 0)), tint=STEEL)
    p.add("glow_cyan", sphere(0.17, 6, 3), xf((1.12, 5.74, 0)), tint="#dffaff")
    p.add("glow_red", sphere(0.1, 5, 3), xf((0.1, 6.14, 0)), tint="#ffd0c0")
    p.add("chrome", cylinder(0.025, 0.025, 0.9, 4), xf((-0.75, 5.8, -0.75)), tint=STEEL)
    p.add("toy", extrude_z([(0.0, 0.0), (0.0, 0.42), (0.62, 0.21)], -0.02, 0.02),
          xf((-0.75, 6.05, -0.75), rot=(0, 90, 0)), tint="#ffd21f")
    return p


# --------------------------------------------------------------------------
# scenery kits: fill the caller's Piece, using only the shared scenery materials
# --------------------------------------------------------------------------


def trail_marker(p):
    """Alpine: a red-and-white waymark post with a little cowbell under a snow-capped roof."""
    for k, tint in enumerate(["#e8262d", CREAM, "#e8262d", CREAM]):
        p.add(box(0.18, 0.62, 0.18), xf((0, 0.31 + k * 0.62, 0)), tint)
    p.add(box(0.9, 0.12, 0.7), xf((0, 2.54, 0)), "#6b442b")
    p.add(box(0.94, 0.1, 0.74), xf((0, 2.64, 0)), "#f2f7ff")
    p.add(box(0.62, 0.06, 0.5), xf((0, 2.71, 0)), "#dbe8fb")
    p.add(cylinder(0.22, 0.15, 0.34, 7), xf((0, 2.16, 0)), "#ffd21f", grad=0.15)
    p.add(sphere(0.09, 6, 3), xf((0, 1.97, 0)), "#6b442b")
    p.add(strut((0.1, 2.5, 0), (0.1, 2.24, 0), 0.02, 4), None, "#c9d3e3")
    p.add(extrude_z([(0.0, 0.0), (0.0, 0.42), (0.66, 0.21)], -0.03, 0.03),
          xf((0.12, 1.62, 0), rot=(0, 90, 0)), "#e8262d", grad=0.0)
    return p


def sunshade(p):
    """Desert: a striped parasol over two crates and a water jug, a tiny roadside rest stop."""
    p.add(cylinder(0.08, 0.07, 3.2, 6), xf((0, 1.6, 0)), "#8a5a34")
    p.add(cylinder(0.34, 0.42, 0.18, 8), xf((0, 0.09, 0)), "#b58a4e")
    stripe = lambda x, y, z: "#ffd166" if math.floor(math.atan2(z, x) * 4 / math.pi) % 2 else "#e8563f"
    p.add(cylinder(1.85, 0.0, 0.62, 10), xf((0, 3.24, 0)), stripe, grad=0.0, jitter=0.0)
    p.add(sphere(0.14, 6, 3), xf((0, 3.6, 0)), "#ffd21f", grad=0.0)
    for dx, dz in [(-1.15, 0.55), (1.05, -0.5)]:
        p.add(box(0.86, 0.72, 0.86, 0.05, 1), xf((dx, 0.36, dz)), WOOD)
        p.add(box(0.9, 0.08, 0.9), xf((dx, 0.74, dz)), "#d8b060")
    p.add(cylinder(0.24, 0.28, 0.6, 7), xf((0.35, 0.3, 1.05)), "#3f8fa8")
    p.add(cylinder(0.12, 0.12, 0.18, 6), xf((0.35, 0.69, 1.05)), "#e8d6b0")
    return p


def signal_mast(p):
    """Harbour: a signal mast with two neon rings and a warm beacon, for wet night reflections."""
    p.add(cylinder(0.52, 0.62, 0.3, 8), xf((0, 0.15, 0)), "#25324a")
    p.add(cylinder(0.13, 0.1, 6.0, 8), xf((0, 3.1, 0)), "#8fa0b8")
    for y, mat in [(3.55, "glow_cyan"), (5.1, "glow_pink")]:
        p.add_flat(torus(0.72, 0.1, 10, 5, y=y), None, "#ffffff", mat)
        for k in range(4):
            a = k * math.pi / 2
            p.add(strut((0.0, y, 0.0), (0.7 * math.cos(a), y, 0.7 * math.sin(a)), 0.05, 4), None, "#8fa0b8")
    p.add(box(1.5, 0.12, 0.16), xf((0, 4.35, 0)), "#8fa0b8")
    for dx in (-0.62, 0.62):
        p.add_flat(sphere(0.16, 6, 3), xf((dx, 4.35, 0)), "#ffffff", "glow_warm")
    p.add_flat(sphere(0.3, 7, 4), xf((0, 6.3, 0)), "#ffffff", "glow_warm")
    p.add(cylinder(0.24, 0.0, 0.4, 6), xf((0, 6.72, 0)), "#e8262d", grad=0.0)
    return p


def holo_arrow(p):
    """Metro: a floating neon chevron over a slim post, adding colour at street level."""
    p.add(box(0.7, 0.18, 0.7), xf((0, 0.09, 0)), "#2a2d3a")
    p.add(box(0.14, 3.1, 0.14), xf((0, 1.6, 0)), "#5a6072")
    p.add(box(0.5, 0.1, 0.5), xf((0, 3.16, 0)), "#8a93a8")
    arrow = extrude_z([(-0.9, -0.34), (0.0, -0.34), (0.9, 0.0), (0.0, 0.34), (-0.9, 0.34), (-0.42, 0.0)], -0.06, 0.06)
    p.add_flat(arrow, xf((0, 3.95, 0), rot=(0, 90, 0)), "#ffffff", "glow_cyan")
    for dy in (-0.46, 0.46):
        p.add_flat(box(0.06, 0.06, 2.1), xf((0, 3.95 + dy, 0)), "#ffffff", "glow_pink")
    for k, dy in enumerate([-0.28, 0.0, 0.28]):
        p.add_flat(sphere(0.1, 5, 3), xf((0.18, 3.95 + dy, 1.0 + 0.32 * (1 - abs(k - 1)))), "#ffffff", "glow_warm")
    return p
