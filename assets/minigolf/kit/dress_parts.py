"""World style for the functional parts of a dressed course (MIT License, see ../LICENSE).

Obstacles, ball and putter templates, flag, hole digits, tee ring and cup parts
come from ``basic_parts`` / ``cup_parts`` with legacy colour materials. After
the build, ``restyle`` swaps their materials for the shared vertex-coloured kit
materials and paints the world colours. Obstacle boxes only get a bevel (the asset
test compares their exact extents), the ball stays exactly on r = 0.16 m and the
putter keeps its 0 to 0.82 m shaft. The hole digit segments get a small bevel and
the flat cup rim a 2 cm thickness (top unchanged) with the same bevel; neither is
a collider input. The flat tee ring and cup patch are turned face up (the legacy
``ring`` winds downwards and the kit materials are single sided).
"""

import bpy
from mathutils import Matrix

from . import colors, materials, palette, shapes
from .basic_parts import ring
from .palette import mix
from .prop import Prop
from .track import BALL_RADIUS, CUP_RADIUS

OBSTACLE = {"pirate": (("red_dark", "red"), ("wood_dark", "wood"), "matte"),
            "temple": (("totem_red", "fire_orange"), ("totem_teal", "totem_teal"), "matte"),
            "moon": (("orange_dark", "orange"), ("panel_grey", "panel_white"), "satin")}
FLAG = {"pirate": "red", "temple": "totem_red", "moon": "magenta"}
PUTTER_HEAD = {"pirate": ("red_dark", "red"), "temple": ("gold_dark", "gold"), "moon": ("orange_dark", "orange")}
SHAFT = {"pirate": ("iron", "iron_light"), "temple": ("bark", "bark_light"), "moon": ("metal_dark", "metal")}
POLE = {"pirate": "cream", "temple": "cream", "moon": "panel_white"}
ROUND = ("cup-bottom", "cup-lining", "flag-pole-base")
SMALL_BEVEL = 0.008  # hole digit segments and cup rim
RIM_THICKNESS = 0.02


def _base(obj):
    return obj.name.split(".")[0]


def _paint(obj, colour, kind, ao, strength=1.0):
    """One flat colour on a legacy mesh and the shared kit material ``kind``."""
    obj.data.materials.clear()
    obj.data.materials.append(materials.get(kind, strength=strength))
    colors.paint_values(obj, [colour] * len(obj.data.vertices), ao=ao)


def _gradient(obj, pal, spec, kind, ao, axis="z"):
    lo, hi = (pal.c(spec[0]), pal.c(spec[1]))
    obj.data.materials.clear()
    obj.data.materials.append(materials.get(kind))
    colors.paint(obj, lo, hi, mode=axis, ao=ao, jitter=0.0)


def _obstacles(look, pal, objs):
    main_grad, cross_grad, kind = OBSTACLE[look]
    out = []
    for obj in objs:
        base = _base(obj)
        if obj.get("aurago_obstacle"):
            shapes.finish(obj, bevel=0.022, segments=2, dens=0, smooth_angle=50)
            _gradient(obj, pal, main_grad, kind, 0.0, axis="x" if base != "obstacle-harbor-gate" else "z")
            out.append(obj)
        elif obj.get("aurago_obstacle_component"):
            shapes.finish(obj, bevel=0.02, segments=2, dens=0, smooth_angle=50)
            _gradient(obj, pal, cross_grad, kind, 0.0, axis="y")
            out.append(obj)
    return out


def _face_up(obj):
    """Turn a flat legacy ring face up: ``courses_v1.ring`` winds its faces downwards, and the kit materials
    are single sided, so the runtime would cull the ring when seen from above."""
    obj.data.flip_normals()


def _cup_patch(look, pal, model, rim):
    """Felt-coloured collar around the cup that covers the dark ground in the stepped lane cut-out."""
    pts = [v.co for v in rim.data.vertices]
    cx, cy = sum(p.x for p in pts) / len(pts), sum(p.y for p in pts) / len(pts)
    z = pts[0].z - 0.001
    patch = ring(None, model, "cup-patch", (cx, cy), CUP_RADIUS + 0.06, CUP_RADIUS + 0.4, z, None, segments=32)
    _face_up(patch)
    lo, hi = pal.grad("felt")
    _paint(patch, mix(lo, hi, 0.62), "matte", 0.3)
    return patch


def _bevel_digit(obj):
    """Soften a seven-segment bar (a plain box in the legacy code)."""
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


def restyle(look, model, cup_patch=True):
    """Restyle the functional parts of ``model``; returns ``(bake, passthrough)`` object lists.

    ``cup_patch`` adds the felt collar around the cup rim of the legacy courses; the exact cup of the track kit
    needs none.
    """
    pal = palette.get(look)
    bake, passthrough = [], []
    gold, black = pal.c("gold"), pal.c("black")
    objs = list(model.objects)
    passthrough += _obstacles(look, pal, objs)
    for obj in objs:
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
            patch = _cup_patch(look, pal, model, obj) if cup_patch else None  # centre and height come from the flat ring
            _bevel_rim(obj)
            _paint(obj, pal.c("cyan") if moon else gold, "emissive" if moon else "satin", 0.0 if moon else 0.5)
            (passthrough if moon else bake).append(obj)
            if patch is not None:
                bake.append(patch)
        elif base == "cup-bottom":
            _paint(obj, black, "matte", 0.0)
            bake.append(obj)
        elif base == "cup-lining":
            _paint(obj, pal.c("black"), "matte", 0.9)
            bake.append(obj)
    return bake, passthrough
