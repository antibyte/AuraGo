"""Aura Drift shared props: hoarding, marshal post, road sign, hotdog stand, camper van (MIT, AuraGo contributors).

Entry point of ``shared.glb``; ``shared_stands.py`` holds the grandstand, gnome statue and inflatable arch.
Props stand on y = 0, centred, front toward +X. ``prop-hoarding`` and ``prop-road_sign`` are empties with a
``face`` child: a flat card with UV 0 to 1 that the runtime textures (poster, warning icon).
"""

import math

from .core import Model
from .shapes import box, cylinder, cylinder_x, cylinder_z, ellipsoid, extrude_z, sphere, torus, xf
from .shared_stands import gnome_statue, grandstand, inflatable_arch
from .toy import Parts, add_face_uv, ensure_materials, ground_ao, install_name_hook, quad_facing_x, tube

RED, WHITE, NAVY, STEEL = "#e8262d", "#fff7ec", "#252f5c", "#c9d3e3"


def card(model, name, frame, shape):
    """Empty ``prop-<name>`` with the frame mesh and a UV'd ``face`` card."""
    root = model.empty("prop-" + name)
    frame.bake(ground_ao(1.5, 0.78))
    model.mesh("%s|frame" % name, frame.items(), shade="auto", parent=root)
    face = Parts()
    face.add("face", shape)
    add_face_uv(model.mesh("%s|face" % name, face.items(), shade="flat", parent=root))


def hoarding(model):
    """Roadside poster board 9 x 3 m on two posts with a lamp bar; ``face`` is the poster."""
    p = Parts()
    for z in (-3.6, 3.6):
        p.add("toy", box(0.34, 2.6, 0.34), xf((-0.1, 1.3, z)), tint="#6b7488")
        p.add("matte", box(0.9, 0.2, 0.9), xf((-0.1, 0.1, z)), tint="#39415f")
    p.add("toy", box(0.22, 3.2, 9.4, 0.04, 1), xf((0, 3.7, 0)), tint=NAVY)
    p.add("toy", box(0.3, 0.16, 9.7), xf((0.03, 5.35, 0)), tint="#ffd21f")
    for z in (-3.0, 0.0, 3.0):
        p.add("toy", box(0.4, 0.08, 0.1), xf((0.25, 5.46, z)), tint=STEEL)
        p.add("glow_white", sphere(0.12, 6, 3), xf((0.45, 5.4, z)), tint="#fff5d0")
    card(model, "hoarding", p, quad_facing_x(0.12, 2.3, 5.1, 4.3))


def road_sign(model):
    """Warning sign on a post; ``face`` is the 1.5 m card for the runtime icon."""
    p = Parts()
    p.add("toy", cylinder(0.07, 0.07, 2.6, 8), xf((-0.12, 1.3, 0)), tint=STEEL)
    p.add("matte", cylinder(0.3, 0.34, 0.12, 8), xf((-0.12, 0.06, 0)), tint="#39415f")
    p.add("toy", box(0.12, 1.9, 1.9, 0.04, 1), xf((0, 2.9, 0)), tint="#ffd21f")
    card(model, "road_sign", p, quad_facing_x(0.075, 2.15, 3.65, 0.75))


def marshal_post(model):
    """Orange track marshal tower with a ladder, a flashing beacon and a waving marshal."""
    p = Parts()
    for sx in (1, -1):
        for sz in (1, -1):
            p.add("toy", box(0.14, 2.3, 0.14), xf((sx * 0.85, 1.15, sz * 0.85)), tint=WHITE)
    p.add("toy", box(2.2, 0.16, 2.2), xf((0, 2.3, 0)), tint="#ff8a1f")
    p.add("toy", box(0.1, 0.45, 1.9), xf((-0.95, 2.62, 0)), tint="#ff8a1f")
    for z in (0.95, -0.95):
        p.add("toy", box(1.9, 0.45, 0.1), xf((0, 2.62, z)), tint="#ff8a1f")
    p.add("toy", cylinder(0.07, 0.07, 2.2, 6), xf((-0.8, 3.4, -0.8)), tint=STEEL)
    for z in (0.3, -0.3):
        p.add("toy", box(0.1, 2.3, 0.07), xf((1.15, 1.15, z)), tint=STEEL)
    for k in range(6):
        p.add("toy", box(0.09, 0.07, 0.66), xf((1.15, 0.3 + 0.38 * k, 0.0)), tint=STEEL)
    p.add("toy", cylinder(0.14, 0.14, 0.2, 8), xf((-0.8, 4.6, -0.8)), tint=RED)
    p.add("glow_red", sphere(0.25, 8, 4), xf((-0.8, 4.85, -0.8)), tint="#ffd0c0")
    # the marshal: vest, head, helmet, waving arm with a yellow flag (a toy-sized hero)
    p.add("toy", ellipsoid(0.34, 0.52, 0.3, 8, 4), xf((0.25, 2.98, 0.0)), tint="#ffe14d")
    p.add("toy", sphere(0.26, 8, 4), xf((0.25, 3.62, 0.0)), tint="#f0b48a")
    p.add("toy", sphere(0.3, 8, 3), xf((0.25, 3.74, 0.0), scale=(1.0, 0.7, 1.0)), tint=WHITE)
    p.add("toy", box(0.12, 0.7, 0.12), xf((0.25, 3.3, 0.5), rot=(-35, 0, 0)), tint="#ffe14d")
    p.add("toy", cylinder(0.03, 0.03, 1.5, 5), xf((0.25, 3.9, 0.85)), tint=STEEL)
    p.add("toy", extrude_z([(0.0, 0.0), (0.0, 0.7), (0.9, 0.35)], -0.015, 0.015), xf((0.25, 4.35, 0.85), rot=(0, -90, 0)), tint="#ffe14d")
    p.bake(ground_ao(2.0, 0.8))
    model.mesh("prop-marshal_post", p.items(), shade="auto")


def hotdog_stand(model):
    """Striped food cart with a scalloped awning and a giant mustard hotdog on top."""
    p = Parts()
    p.add("toy", box(3.0, 1.2, 1.9, 0.1, 1), xf((0, 1.2, 0)), tint="#2f78f0")
    p.add("toy", box(3.04, 0.2, 1.94), xf((0, 1.05, 0)), tint=WHITE)
    p.add("toy", box(3.3, 0.14, 2.2, 0.04, 1), xf((0.1, 1.86, 0)), tint="#fff1d6")
    p.add("toy", box(0.06, 0.7, 1.4), xf((1.52, 1.3, 0)), tint="#1b2236")
    for z in (0.98, -0.98):
        p.add("toy", cylinder_z(0.55, 0.55, 0.16, 12, 0.04), xf((-0.5, 0.55, z)), tint="#252f5c")
        p.add("toy", cylinder_z(0.3, 0.3, 0.2, 8), xf((-0.5, 0.55, z)), tint=STEEL)
    p.add("toy", cylinder(0.08, 0.08, 0.55, 6), xf((1.25, 0.28, 0)), tint=STEEL)
    for sx in (1.5, -1.5):
        for sz in (1.0, -1.0):
            p.add("toy", cylinder(0.05, 0.05, 1.5, 6), xf((sx, 2.65, sz)), tint=STEEL)
    for k in range(6):
        p.add("toy", box(3.6, 0.1, 0.42), xf((0, 3.46 - 0.0, -1.05 + 0.42 * k), rot=(0, 0, 0)), tint=RED if k % 2 == 0 else WHITE)
        p.add("toy", sphere(0.23, 6, 3), xf((1.78, 3.4, -1.05 + 0.42 * k), scale=(0.7, 1.0, 1.0)), tint=RED if k % 2 == 0 else WHITE)
    p.add("toy", cylinder(0.1, 0.1, 0.6, 6), xf((0, 3.8, 0)), tint=STEEL)
    p.add("toy", ellipsoid(1.7, 0.4, 0.62, 10, 4), xf((0, 4.4, 0)), tint="#f0b66a")
    p.add("toy", tube([(-1.75, 4.62, 0.0), (0.0, 4.64, 0.0), (1.75, 4.62, 0.0)], 0.3, 8), tint="#c4622b")
    zig = [(-1.4 + 0.28 * k, 4.95, 0.12 * (1 if k % 2 else -1)) for k in range(11)]
    p.add("toy", tube(zig, 0.06, 5), tint="#ffd21f")
    p.add("glow", sphere(0.14, 6, 3), xf((1.62, 1.95, 0.7)), tint="#fff1b0")
    p.bake(ground_ao(1.5, 0.8))
    model.mesh("prop-hotdog_stand", p.items(), shade="auto")


def camper_van(model):
    """Retro two-tone camper van with a pop-up roof, surfboard, awning and a spare wheel (nose toward +X)."""
    p = Parts()
    low = [(-2.6, 0.38), (-2.6, 1.5), (1.5, 1.5), (2.45, 1.25), (2.6, 0.9), (2.6, 0.38)]
    p.add("toy", extrude_z(low, -1.05, 1.05, 0.12, 2), tint="#35c6bd")
    cab = [(-2.5, 1.5), (-2.5, 2.75), (0.9, 2.75), (1.9, 1.95), (2.0, 1.5)]
    p.add("toy", extrude_z(cab, -1.0, 1.0, 0.12, 2), tint="#fff1d6")
    p.add("toy", box(3.0, 0.6, 1.7, 0.14, 2), xf((-0.6, 3.05, 0)), tint="#fff1d6")
    for sz in (1.015, -1.015):
        for x in (-1.7, -0.5, 0.55):
            p.add("toy", box(0.9, 0.62, 0.04), xf((x, 2.2, sz)), tint="#2b4a6e")
    p.add("toy", box(0.07, 1.0, 1.7), xf((1.62, 2.18, 0), rot=(0, 0, 48)), tint="#2b4a6e")
    p.add("toy", box(0.07, 0.07, 1.9), xf((1.18, 2.62, 0), rot=(0, 0, 48)), tint="#fff1d6")
    for k, z in enumerate((-0.75, 0.75)):
        p.add("glow_white", sphere(0.22, 7, 4), xf((2.5, 0.95, z), scale=(0.7, 1.0, 1.0)), tint="#fff1b0")
    p.add("toy", box(0.07, 0.07, 1.6), xf((2.58, 0.55, 0)), tint=STEEL)
    for side in (1, -1):
        p.add("toy", box(0.06, 0.07, 1.0), xf((2.56, 1.0, side * 0.45), rot=(0, side * 40, 0)), tint="#1b8a84")
    p.add("toy", box(0.22, 0.2, 2.2, 0.04, 1), xf((2.62, 0.46, 0)), tint=STEEL)
    p.add("toy", box(0.22, 0.2, 2.2, 0.04, 1), xf((-2.66, 0.46, 0)), tint=STEEL)
    p.add("toy", box(5.1, 0.12, 2.12), xf((0, 1.52, 0)), tint="#fff1d6")
    p.add("toy", ellipsoid(1.4, 0.08, 0.3, 8, 3), xf((-0.6, 3.5, 0.3), rot=(0, 8, 0)), tint="#ff7a59")
    p.add("toy", ellipsoid(1.3, 0.07, 0.27, 8, 3), xf((-0.4, 3.52, -0.35), rot=(0, -6, 0)), tint="#ffd21f")
    for x in (-1.5, 0.0):
        p.add("toy", box(0.06, 0.1, 1.9), xf((x, 3.4, 0)), tint=STEEL)
    for x in (1.65, -1.65):
        for z in (1.0, -1.0):
            p.add("toy", cylinder_z(0.5, 0.5, 0.34, 12, 0.08), xf((x, 0.5, z)), tint="#23262c")
            p.add("toy", cylinder_z(0.28, 0.28, 0.38, 8), xf((x, 0.5, z)), tint=STEEL)
    p.add("toy", cylinder_x(0.45, 0.45, 0.2, 10, 0.04), xf((-2.78, 1.1, 0)), tint="#23262c")
    p.add("toy", cylinder_x(0.24, 0.24, 0.24, 8), xf((-2.78, 1.1, 0)), tint="#35c6bd")
    # side awning on the right: striped canvas on two poles
    for k in range(4):
        p.add("toy", box(2.6, 0.05, 0.4), xf((-0.4, 2.62 - 0.14 * k, 1.25 + 0.4 * k), rot=(20, 0, 0)), tint="#ff7a59" if k % 2 == 0 else WHITE)
    for x in (-1.6, 0.8):
        p.add("toy", cylinder(0.04, 0.04, 2.0, 5), xf((x, 1.0, 2.7)), tint=STEEL)
    p.bake(ground_ao(1.4, 0.78))
    model.mesh("prop-camper_van", p.items(), shade="auto")


ORDER = (
    ("grandstand", grandstand, "auto"),
    ("gnome_statue", gnome_statue, "auto"),
    ("inflatable_arch", inflatable_arch, "auto"),
)


def build():
    ensure_materials()
    install_name_hook()
    model = Model("shared")
    for name, make, shade in ORDER:
        parts = make()
        parts.bake(ground_ao(1.5, 0.78))
        model.mesh("prop-" + name, parts.items(), shade=shade)
    for make in (hoarding, marshal_post, road_sign, hotdog_stand, camper_van):
        make(model)
    return model
