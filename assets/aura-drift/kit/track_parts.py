"""Aura Drift track parts: ramp, boost pad, cone, hay bale, barrel, tyre stack, barrier, gantry, rumble bars (MIT, AuraGo contributors).

Entry point of ``track.glb``. Every piece sits at the origin, base on y = 0, front toward +X. The
runtime scales ramps, pads and rumble bars to the track width, so the authored footprint is
recorded in ``meta_track.py`` (ramp 9 x 8 x 1.6 m, pad 10 x 6 m, rumble bars 9 x 12 m).
"""

import math

from .core import Model
from .shapes import annulus_z, box, cylinder, cylinder_x, cylinder_z, extrude_z, revolve, sphere, strut, torus, xf
from .toy import Parts, ensure_materials, ground_ao, install_name_hook

RED, WHITE, ORANGE, NAVY = "#e8262d", "#fff7ec", "#ff8a1f", "#252f5c"


def chevron(p, mat, tint, tip, tilt=0.0, arm=1.4, thick=0.07, wide=0.42, angle=42.0):
    """A ">" lying flat (tip toward +X) at ``tip`` = (x, y, z), pitched by ``tilt`` degrees about Z."""
    a = math.radians(angle)
    base = xf(tip, rot=(0, 0, tilt))
    for side in (1, -1):
        centre = (-math.cos(a) * arm / 2.0, 0.0, side * math.sin(a) * arm / 2.0)
        p.add(mat, box(arm, thick, wide), base @ xf(centre, rot=(0, side * angle, 0)), tint=tint)


def ramp():
    p = Parts()
    length, width, height = 9.0, 8.0, 1.6
    slope = math.degrees(math.atan2(height, length))

    def deck_y(x):
        return 0.14 + (x + length / 2.0) * height / length

    p.add("matte", extrude_z([(-4.5, 0.0), (4.5, 0.0), (4.5, height)], -3.7, 3.7), tint="#39415f")
    p.add("toy", extrude_z([(-4.5, 0.0), (-4.5, 0.14), (4.5, height + 0.14), (4.5, height)], -4.0, 4.0, 0.02, 1), tint=ORANGE)
    segs = 6
    for k in range(segs):
        x0, x1 = -4.5 + 9.0 * k / segs, -4.5 + 9.0 * (k + 1) / segs
        poly = [(x0, 0.0), (x1, 0.0), (x1, deck_y(x1) + 0.34), (x0, deck_y(x0) + 0.34)]
        for side in (1, -1):
            z0, z1 = (4.0, 4.5) if side > 0 else (-4.5, -4.0)
            p.add("toy", extrude_z(poly, z0, z1), tint=RED if k % 2 == 0 else WHITE)
    for cx in (-2.9, -1.2, 0.5, 2.2):
        chevron(p, "toy", "#fff3b0", (cx + 0.9, deck_y(cx + 0.9) + 0.045, 0.0), slope, arm=2.9, thick=0.05, wide=0.66, angle=48.0)
    p.add("toy", box(0.5, 0.2, 8.0), xf((4.3, height + 0.1, 0.0), rot=(0, 0, slope)), tint="#ffe14d")
    p.add("glow_cyan", box(0.16, 0.06, 7.8), xf((4.42, height + 0.19, 0.0), rot=(0, 0, slope)), tint="#6ff6ff")
    for z in (-3.2, -1.6, 0.0, 1.6, 3.2):
        p.add("matte", strut((4.42, 0.08, z), (4.42, height - 0.08, z), 0.08, 6), tint="#222630")
    for z0, z1 in ((-3.2, -1.6), (-1.6, 0.0), (0.0, 1.6), (1.6, 3.2)):
        p.add("matte", strut((4.42, 0.12, z0), (4.42, height - 0.12, z1), 0.055, 6), tint="#2b3140")
    for rx in (-3.2, -1.0, 1.2, 3.4):
        for side in (1, -1):
            p.add("glow", box(0.32, 0.07, 0.2), xf((rx, deck_y(rx) + 0.37, side * 4.25), rot=(0, 0, slope)), tint="#ffaa22")
    return p


def boost_pad():
    p = Parts()
    p.add("matte", box(10.0, 0.14, 6.0, 0.05, 1), xf((0, 0.07, 0)), tint=NAVY)
    for z in (2.76, -2.76):
        p.add("glow_cyan", box(9.4, 0.04, 0.16), xf((0, 0.155, z)), tint="#8ff6ff")
        for cx in (-3.6, 0.0, 3.6):
            p.add("matte", cylinder(0.22, 0.18, 0.24, 10), xf((cx, 0.24, z)), tint="#1b2238")
            p.add("glow_cyan", torus(0.2, 0.035, 10, 4, 0.24), xf((cx, 0.0, z)), tint="#5ff0ff")
            p.add("chrome", cylinder(0.14, 0.14, 0.05, 8), xf((cx, 0.37, z)), tint="#dfe6f2")
    for x in (-4.88, 4.88):
        p.add("chrome", box(0.18, 0.04, 5.4), xf((x, 0.15, 0)), tint="#cfd8e8")
    for k, cx in enumerate((-3.4, -1.15, 1.1, 3.35)):
        chevron(p, "glow_lime", "#c5ff6a" if k % 2 else "#a6ff3a", (cx + 1.0, 0.16, 0.0), 0.0, arm=3.2, thick=0.05, wide=0.7, angle=46.0)
    return p


def cone():
    p = Parts()

    def radius(y):
        return 0.25 - 0.2 * (y - 0.05) / 0.7

    p.add("matte", box(0.62, 0.05, 0.62), xf((0, 0.025, 0)), tint="#2b2f38")
    p.add("toy", cylinder(0.25, 0.05, 0.7, 10), xf((0, 0.4, 0)), tint="#ff6a1a")
    for y0, y1 in ((0.17, 0.3), (0.38, 0.48)):
        p.add("toy", cylinder(radius(y0) + 0.014, radius(y1) + 0.014, y1 - y0, 10), xf((0, (y0 + y1) / 2, 0)), tint=WHITE)
    p.add("toy", sphere(0.06, 6, 3), xf((0, 0.76, 0)), tint="#ff6a1a")
    return p


def hay_bale():
    p = Parts()
    r, w = 0.95, 1.5
    p.add("matte", cylinder_z(r, r, w, 14, 0.12), xf((0, r, 0)), tint="#e3b64a")
    for z, flip in ((w / 2 + 0.003, 0), (-w / 2 - 0.003, 180)):
        for r0, r1 in ((0.2, 0.28), (0.45, 0.53), (0.7, 0.78)):
            p.add("matte", annulus_z(r0, r1, 0.0, 16), xf((0, r, z), rot=(0, flip, 0)), tint="#b3822b")
    for z in (0.38, -0.38):
        p.add("toy", cylinder_z(r + 0.02, r + 0.02, 0.12, 14), xf((0, r, z)), tint="#d9433a")
    return p


def barrel():
    p = Parts()
    p.add("toy", cylinder(0.55, 0.55, 1.3, 12, 0.06), xf((0, 0.65, 0)), tint=RED)
    p.add("toy", cylinder(0.556, 0.556, 0.34, 12), xf((0, 0.65, 0)), tint="#ffd21f")
    for y in (0.22, 1.08):
        p.add("toy", torus(0.555, 0.035, 12, 4, y), tint="#b3151d")
    p.add("matte", cylinder(0.43, 0.43, 0.04, 12), xf((0, 1.31, 0)), tint="#2b2f38")
    p.add("toy", cylinder(0.09, 0.09, 0.06, 8), xf((0.2, 1.34, 0.1)), tint="#c9d3e3")
    return p


TYRE = [(0.42, -0.25), (0.74, -0.25), (0.88, -0.13), (0.9, 0.0), (0.88, 0.13), (0.74, 0.25), (0.42, 0.25), (0.34, 0.0)]


def tyre_stack():
    p = Parts()
    for k, tone in enumerate(("#2b2f38", WHITE, RED)):
        p.add("toy", revolve(TYRE, 14, closed=True), xf((0, 0.25 + 0.5 * k, 0)), tint=tone)
    p.add("matte", cylinder(0.42, 0.42, 0.06, 12), xf((0, 0.05, 0)), tint="#1b1d22")
    return p


def barrier_segment():
    p = Parts()
    profile = [(-0.46, 0.0), (0.46, 0.0), (0.42, 0.34), (0.22, 0.92), (-0.22, 0.92), (-0.42, 0.34)]
    for k in range(4):
        x = -1.5 + k
        shape = extrude_z(profile, -0.485, 0.485, 0.025, 1)
        p.add("toy", shape, xf((x, 0, 0), rot=(0, 90, 0)), tint=RED if k % 2 == 0 else WHITE)
        p.add("matte", box(0.18, 0.08, 0.14), xf((x, 1.01, 0)), tint="#22262e")
        p.add("glow_red" if k % 2 == 0 else "glow_white", box(0.14, 0.05, 0.16), xf((x, 1.01, 0)), tint="#ff4030" if k % 2 == 0 else WHITE)
    p.add("toy", box(4.0, 0.07, 0.34), xf((0, 0.94, 0)), tint="#ffd21f")
    return p


def rumble_bars():
    p = Parts()
    for k in range(8):
        x = -3.85 + 1.1 * k
        p.add("toy", box(0.62, 0.1, 12.0, 0.03, 1), xf((x, 0.05, 0)), tint=RED if k % 2 == 0 else WHITE)
    return p


def gantry(model):
    """Start gantry spanning the track: two striped pylons, a beam, a checker band, five lamps (children light_1 to light_5)."""
    root = model.empty("track-gantry")
    p = Parts()
    half = 6.9
    for z in (half, -half):
        p.add("toy", box(1.2, 6.2, 1.2, 0.08, 1), xf((0, 3.1, z)), tint=WHITE)
        for k in range(6):
            p.add("toy", box(1.24, 0.5, 1.24), xf((0, 0.45 + 1.0 * k, z)), tint=RED)
        p.add("matte", cylinder(1.1, 1.1, 0.3, 12, 0.05), xf((0, 0.15, z)), tint="#39415f")
        p.add("toy", sphere(0.75, 10, 5), xf((0, 6.3, z)), tint=RED)
    p.add("toy", box(1.0, 1.3, 14.6, 0.08, 1), xf((0, 6.6, 0)), tint="#252f5c")
    p.add("matte", box(1.04, 0.34, 14.6), xf((0, 6.6, 0)), tint=WHITE)
    for side in (1, -1):
        for k in range(28):
            if k % 2 == 0:
                p.add("matte", box(0.02, 0.17, 0.5), xf((side * 0.522, 6.6 + (0.085 if k % 4 == 0 else -0.085), -6.75 + 0.5 * k)), tint="#15171c")
    p.add("toy", box(1.0, 1.1, 5.9, 0.06, 1), xf((0, 5.5, 0)), tint="#15171c")
    for k in range(5):
        p.add("matte", cylinder_x(0.42, 0.42, 0.1, 10), xf((0.52, 5.5, -2.2 + 1.1 * k)), tint="#2b2f38")
    for side in (1, -1):
        p.add("matte", cylinder_x(0.12, 0.34, 0.46, 10), xf((0.42, 4.7, side * 5.95), rot=(0, -side * 18, -12)), tint="#2b2f38")
        p.add("chrome", sphere(0.11, 8, 4), xf((0.58, 4.66, side * 5.95)), tint="#dfe6f2")
        p.add("matte", box(0.42, 0.24, 0.36, 0.03, 1), xf((0.35, 5.82, side * 4.4)), tint="#1e222a")
        p.add("glow_cyan", cylinder_x(0.08, 0.08, 0.08, 8), xf((0.56, 5.8, side * 4.4)), tint="#5ff0ff")
    p.bake(ground_ao(1.2, 0.72))
    model.mesh("track-gantry|body", p.items(), shade="auto", parent=root)
    for k in range(5):
        lamp = Parts()
        lamp.add("light", sphere(0.34, 8, 4), tint="#ffffff")
        model.mesh("light_%d" % (k + 1), lamp.items(), shade="smooth", parent=root, location=(0.55, 5.5, -2.2 + 1.1 * k))


ORDER = (
    ("ramp", ramp),
    ("boost_pad", boost_pad),
    ("cone", cone),
    ("hay_bale", hay_bale),
    ("barrel", barrel),
    ("tyre_stack", tyre_stack),
    ("barrier_segment", barrier_segment),
)


def build():
    ensure_materials()
    install_name_hook()
    model = Model("track")
    for name, make in ORDER:
        parts = make()
        parts.bake(ground_ao(0.8, 0.78))
        model.mesh("track-" + name, parts.items(), shade="auto")
    gantry(model)
    parts = rumble_bars()
    parts.bake(ground_ao(0.4, 0.9))
    model.mesh("track-rumble_bars", parts.items(), shade="auto")
    return model
