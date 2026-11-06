"""Aura Drift items: crate, rocket, bratwurst, harpoon, air horn, frost grenade (MIT, AuraGo contributors).

Entry point of ``items.glb``. The grinning gnome mine, rubber duck, boxing glove and disco ball live
in ``items_fun.py``. Every item is one mesh root named ``item-<name>``, nose toward +X, base at y = 0,
all at the origin (the runtime clones nodes by name).
"""

import math

from .characters import eye_pair
from .core import Model
from .items_fun import boxing_glove, disco_ball, gnome_mine, rubber_duck
from .shapes import arc_band, box, cylinder, cylinder_x, cylinder_z, ellipsoid, extrude_z, merge, revolve_x, sphere, torus, xf
from .toy import Parts, arc_points, ensure_materials, ground_ao, ribbon, tube

VIOLET, PANEL, GOLD, STEEL = "#6a4cff", "#8d77ff", "#ffc933", "#d9e1ee"


def qmark(depth=0.06, width=0.17):
    """The glowing question mark, in the XY plane facing +Z, about 0.75 tall."""
    path = arc_points(0.0, 0.3, 0.3, 165.0, -90.0, 7) + [(0.0, -0.05), (0.0, -0.14)]
    return merge([ribbon(path, width, depth), ribbon([(0.0, -0.31), (0.0, -0.4)], width, depth)])


def crate():
    p = Parts()
    h = 0.95
    p.add("toy", box(1.7, 1.7, 1.7, 0.13, 2), xf((0, h, 0)), tint=VIOLET)
    glyph = qmark()
    fit = xf(scale=1.2) @ xf((0, -0.1, 0))
    for rot in ((0, 0, 0), (0, 90, 0), (0, 180, 0), (0, -90, 0)):
        face = xf((0, h, 0), rot)
        p.add("matte", box(1.28, 1.28, 0.07), face @ xf((0, 0, 0.855)), tint=PANEL)
        p.add("glow", glyph, face @ xf((0, 0, 0.885)) @ fit, tint="#ffe27a")
    p.add("matte", box(1.28, 0.07, 1.28), xf((0, h + 0.855, 0)), tint=PANEL)
    p.add("glow", glyph, xf((0, h + 0.885, 0), (-90, 0, 0)) @ xf(rot=(0, 0, -90)) @ fit, tint="#ffe27a")
    for sx in (-1, 1):
        for sy in (-1, 1):
            for sz in (-1, 1):
                p.add("toy", box(0.34, 0.34, 0.34), xf((sx * 0.8, h + sy * 0.8, sz * 0.8)), tint=GOLD)
    return p


def rocket():
    p = Parts()
    cy = 0.68
    prof = [(0.0, -1.18), (0.2, -1.14), (0.33, -0.95), (0.4, -0.55), (0.42, 0.0), (0.4, 0.45), (0.3, 0.8), (0.16, 1.1), (0.0, 1.25)]

    def paint(x, y, z):
        if x > 0.52:
            return "#ff3b30"
        if -0.78 < x < -0.62 or 0.28 < x < 0.4:
            return "#2f78f0"
        return "#fffaf0"

    p.add("toy", revolve_x(prof, 14), xf((0, cy, 0)), tint=paint)
    fin = extrude_z([(-1.15, 0.22), (-0.45, 0.36), (-0.78, 0.62), (-1.2, 0.66)], -0.035, 0.035, 0.01, 1)
    for k in range(3):
        p.add("toy", fin, xf((0, cy, 0), (k * 120.0, 0, 0)), tint="#ff3b30")
    p.add("matte", cylinder_x(0.27, 0.17, 0.32, 10), xf((-1.3, cy, 0)), tint="#2b2f38")
    p.add("glow", cylinder_x(0.2, 0.2, 0.05, 10), xf((-1.46, cy, 0)), tint="#ffe29a")
    for z in (0.405, -0.405):
        p.add("toy", cylinder_z(0.16, 0.16, 0.05, 10), xf((0.12, cy + 0.07, z)), tint=STEEL)
        p.add("toy", ellipsoid(0.125, 0.125, 0.04, 8, 4), xf((0.12, cy + 0.07, z * 1.01)), tint="#7fd0ff")
    p.add("toy", cylinder(0.16, 0.16, 0.05, 10), xf((0.12, cy + 0.405, 0)), tint=STEEL)
    p.add("toy", ellipsoid(0.125, 0.04, 0.125, 8, 4), xf((0.12, cy + 0.41, 0)), tint="#7fd0ff")
    return p


def bratwurst():
    p = Parts()
    n = 13

    def centre(t):
        return 0.3 + 0.13 * (1.0 - (2 * t - 1) ** 2)

    def radius(t):
        return max(0.07, 0.235 * (1.0 - (2 * t - 1) ** 8) ** 0.4)

    pts = [(-0.8 + 1.6 * i / (n - 1), centre(i / (n - 1)), 0.0) for i in range(n)]
    p.add("toy", tube(pts, [radius(i / (n - 1)) for i in range(n)], 9), tint=lambda x, y, z: "#c4622b" if (int((x + 1.0) * 5.0) % 2 == 0 and y < 0.4) else "#df8a3c")
    for end in (-1, 1):
        p.add("matte", sphere(0.1, 8, 4), xf((end * 0.82, 0.3, 0)), tint="#7a3b1b")
    zig = []
    for k in range(11):
        t = 0.1 + 0.08 * k
        zig.append((-0.8 + 1.6 * t, centre(t) + radius(t) * 0.96, 0.1 * (1 if k % 2 else -1)))
    p.add("toy", tube(zig, 0.036, 5), tint="#ffd21f")
    return p


def harpoon():
    p = Parts()
    cy = 0.55
    p.add("toy", cylinder_x(0.085, 0.085, 2.7, 8), xf((-0.2, cy, 0)), tint="#a8683a")
    for x in (-1.0, 0.5):
        p.add("toy", cylinder_x(0.105, 0.105, 0.14, 8), xf((x, cy, 0)), tint="#e8262d")
    p.add("toy", cylinder_x(0.27, 0.02, 0.8, 8), xf((1.5, cy, 0)), tint=STEEL)
    p.add("toy", cylinder_x(0.13, 0.13, 0.3, 8), xf((1.0, cy, 0)), tint="#8fa0b8")
    barb = extrude_z([(1.3, 0.08), (0.78, 0.46), (0.9, 0.05)], -0.045, 0.045, 0.012, 1)
    for k in range(4):
        p.add("toy", barb, xf((0, cy, 0), (k * 90.0, 0, 0)), tint=STEEL)
    fin = extrude_z([(-1.5, 0.05), (-0.9, 0.09), (-1.2, 0.55), (-1.62, 0.58)], -0.03, 0.03, 0.006, 1)
    for k in range(3):
        p.add("toy", fin, xf((0, cy, 0), (k * 120.0 + 60.0, 0, 0)), tint="#ff8a1f")
    p.add("matte", torus(0.22, 0.05, 12, 4), xf((-1.66, cy, 0), (0, 0, 90)), tint="#e8d3a2")
    p.add("matte", torus(0.22, 0.05, 12, 4), xf((-1.76, cy - 0.07, 0.05), (0, 0, 90)), tint="#cdb785")
    return p


def air_horn():
    p = Parts()
    p.add("toy", cylinder(0.31, 0.31, 1.0, 14, 0.06), xf((0, 0.56, 0)), tint="#2f7cf0")
    p.add("toy", cylinder(0.315, 0.315, 0.3, 14), xf((0, 0.5, 0)), tint="#fff7ec")
    p.add("toy", cylinder(0.32, 0.32, 0.06, 14), xf((0, 0.67, 0)), tint="#ff3b30")
    p.add("toy", cylinder(0.31, 0.15, 0.2, 14), xf((0, 1.16, 0)), tint="#2f7cf0")
    p.add("toy", cylinder(0.12, 0.1, 0.22, 10), xf((0, 1.37, 0)), tint=STEEL)
    p.add("toy", cylinder(0.1, 0.1, 0.1, 10, 0.02), xf((0, 1.53, 0)), tint="#ff3b30")
    p.add("toy", cylinder_x(0.13, 0.62, 1.4, 14), xf((0.693, 1.467, 0), (0, 0, 8)), tint="#ff4a3d")
    p.add("toy", torus(0.6, 0.06, 14, 5), xf((1.386, 1.564, 0), (0, 0, -82)), tint=STEEL)
    p.add("matte", cylinder_x(0.5, 0.5, 0.02, 14), xf((1.4, 1.566, 0), (0, 0, 8)), tint="#2b2f38")
    eyes = Parts()
    eye_pair(eyes, 0.11, 0.87, 0.29, 0.1, look=(0.1, -0.05), shine=False, detail=0)
    eyes.place("matte", arc_band(0.075, 0.11, 205, 335, 0.0, 0.03, 8), (0, 0.84, 0.3), tint="#1b1b24")
    p.include(eyes)
    return p


def frost_grenade():
    p = Parts()
    p.add("toy", sphere(0.6, 10, 6), xf((0, 0.66, 0)), tint=lambda x, y, z: "#7fe3ff" if (int(y * 9.0) % 2 == 0) else "#5ccdf5")
    p.add("fuzz", sphere(0.4, 9, 4), xf((0, 1.08, 0), scale=(1.0, 0.55, 1.0)), tint="#ffffff")
    for k in range(6):
        a = math.tau * k / 6
        p.add("fuzz", cylinder(0.1, 0.01, 0.34, 6), xf((0.5 * math.cos(a), 0.9, 0.5 * math.sin(a)), (0, 0, 180)), tint="#f2fcff")
        p.add("fuzz", sphere(0.12, 6, 3), xf((0.36 * math.cos(a + 0.5), 1.1, 0.36 * math.sin(a + 0.5))), tint="#ffffff")
    p.add("toy", cylinder(0.17, 0.14, 0.24, 10), xf((0, 1.38, 0)), tint=STEEL)
    p.add("toy", box(0.5, 0.05, 0.14), xf((0.12, 1.52, 0), (0, 0, -8)), tint=STEEL)
    p.add("toy", torus(0.13, 0.03, 10, 4), xf((-0.2, 1.44, 0), (90, 0, 0)), tint=STEEL)
    p.add("toy", box(0.06, 0.62, 0.15), xf((0.3, 1.16, 0), (0, 0, -14)), tint=STEEL)
    for k in range(3):
        p.add("glow_white", box(0.05, 0.42, 0.06), xf((0.605, 0.66, 0), (k * 60.0, 0, 0)), tint="#ffffff")
    return p


ORDER = (
    ("crate", crate, "auto"),
    ("gnome_mine", gnome_mine, "auto"),
    ("rubber_duck", rubber_duck, "auto"),
    ("rocket", rocket, "auto"),
    ("bratwurst", bratwurst, "auto"),
    ("boxing_glove", boxing_glove, "auto"),
    ("disco_ball", disco_ball, "flat"),
    ("harpoon", harpoon, "auto"),
    ("air_horn", air_horn, "auto"),
    ("frost_grenade", frost_grenade, "flat"),
)


def build():
    ensure_materials()
    model = Model("items")
    for name, make, shade in ORDER:
        parts = make()
        parts.bake(ground_ao(1.0, 0.7))
        model.mesh("item-" + name, parts.items(), shade=shade)
    return model
