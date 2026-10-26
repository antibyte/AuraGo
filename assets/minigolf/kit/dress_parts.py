"""World style for the functional parts of a dressed course (MIT License, see ../LICENSE).

Ball and putter templates, flag, hole digits, tee ring and cup rim come from ``basic_parts`` / ``cup_parts`` with
plain colour materials. After the build, ``restyle`` swaps their materials for the shared vertex-coloured kit
materials and paints the world colours. The ball stays exactly on r = 0.16 m and the putter keeps its 0 to 0.82 m
shaft. The hole digit segments get a small bevel and the flat cup rim a 2 cm thickness (top unchanged) with the
same bevel; neither is a collider input. The flat tee ring is turned face up (``basic_parts.ring`` winds its
faces downwards and the kit materials are single sided). Obstacles and mechanic parts are painted by
``obstacle_parts`` / ``mechanic_parts``.
"""

import bpy
from mathutils import Matrix

from . import colors, materials, palette, shapes
from .prop import Prop
from .track import BALL_RADIUS

FLAG = {"pirate": "red", "temple": "totem_red", "moon": "magenta"}
PUTTER_HEAD = {"pirate": ("red_dark", "red"), "temple": ("gold_dark", "gold"), "moon": ("orange_dark", "orange")}
SHAFT = {"pirate": ("iron", "iron_light"), "temple": ("bark", "bark_light"), "moon": ("metal_dark", "metal")}
POLE = {"pirate": "cream", "temple": "cream", "moon": "panel_white"}
ROUND = ("flag-pole-base",)
SMALL_BEVEL = 0.008  # hole digit segments and cup rim
RIM_THICKNESS = 0.02


def _base(obj):
    return obj.name.split(".")[0]


def _paint(obj, colour, kind, ao, strength=1.0):
    """One flat colour on a plain mesh and the shared kit material ``kind``."""
    obj.data.materials.clear()
    obj.data.materials.append(materials.get(kind, strength=strength))
    colors.paint_values(obj, [colour] * len(obj.data.vertices), ao=ao)


def _face_up(obj):
    """Turn a flat ring face up: ``basic_parts.ring`` winds its faces downwards, and the kit materials
    are single sided, so the runtime would cull the ring when seen from above."""
    obj.data.flip_normals()


def _bevel_digit(obj):
    """Soften a seven-segment bar (a plain box)."""
    shapes.finish(obj, bevel=SMALL_BEVEL, segments=2, dens=0)


def _bevel_rim(obj):
    """Give the flat cup ring a thickness below its top (the top stays where it was) and bevel it."""
    shapes.finish(obj, bevel=SMALL_BEVEL, segments=2, dens=0, solidify=RIM_THICKNESS)
    obj.data.transform(Matrix.Translation((0.0, 0.0, -RIM_THICKNESS / 2)))


def _ball(obj):
    """Finer ball: an icosphere whose vertices all lie exactly on the ball radius, plain white (the game tints it)."""
    fresh = shapes.ico_sphere("ball-fresh", BALL_RADIUS, subdiv=2)
    obj.data = fresh.data
    bpy.data.objects.remove(fresh, do_unlink=True)
    _paint(obj, (1.0, 1.0, 1.0, 1.0), "satin", 0.0)
    return obj


def _putter(look, model, old):
    """Steel shaft (0 to 0.82 m) and a bevelled head in the world accent colour."""
    # The old object and mesh give up their names first, so the new ones are not numbered (".001").
    old.name, old.data.name = "putter-template-old", "putter-template-old-mesh"
    shapes.set_collection(model)
    try:
        p = Prop(look, "putter", full_name="putter-template")
        shaft = shapes.cylinder("putter-shaft", 0.035, 0.03, 0.82, seg=12)
        p.add(shaft, "metal", SHAFT[look], finish=False, ao=0.0)
        head = shapes.box("putter-head", (0.4, 0.13, 0.09))
        p.add(head, "satin", PUTTER_HEAD[look], loc=(0.0, 0.0, 0.055), bevel=0.014, segments=2, dens=0, ao=0.0)
        new = p.build()
    finally:
        shapes.set_collection(None)
    for key in [k for k in new.keys() if not k.startswith("aurago_")]:
        del new[key]
    new["aurago_template"] = "putter"
    new["aurago_role"] = "template"
    old_mesh = old.data
    bpy.data.objects.remove(old, do_unlink=True)
    if old_mesh.users == 0:
        bpy.data.meshes.remove(old_mesh)
    return new


def restyle(look, model):
    """Restyle the functional parts of ``model``; returns ``(bake, passthrough)`` object lists."""
    pal = palette.get(look)
    bake, passthrough = [], []
    gold = pal.c("gold")
    for obj in list(model.objects):
        base = _base(obj)
        if base in ROUND:
            shapes.smooth_by_angle(obj.data, 40.0)
        if base == "ball-template":
            passthrough.append(_ball(obj))
        elif base == "putter-template":
            passthrough.append(_putter(look, model, obj))
        elif base == "flag":
            _paint(obj, pal.c(FLAG[look]), "cloth", 0.0)
            bake.append(obj)
        elif base == "flag-pole-base":
            _paint(obj, pal.c(POLE[look]), "satin", 0.4)
            bake.append(obj)
        elif base == "hole-digit-segment":
            _bevel_digit(obj)
            _paint(obj, gold, "emissive", 0.0, strength=1.0)
            passthrough.append(obj)
        elif base == "tee-ring":
            _face_up(obj)
            _paint(obj, gold, "satin", 0.4)
            bake.append(obj)
        elif base == "cup-rim":
            moon = look == "moon"
            _bevel_rim(obj)
            _paint(obj, pal.c("cyan") if moon else gold, "emissive" if moon else "satin", 0.0 if moon else 0.5)
            (passthrough if moon else bake).append(obj)
    return bake, passthrough
