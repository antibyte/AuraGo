"""Visible parts of the mechanics: bumpers and boost pads, and the build entry for all of them (MIT License, see ../LICENSE).

``build_all(res, dresser, model)`` turns the ``Resolved`` mechanics and obstacles of ``mechanic_data`` into Blender
objects after the dressing (island and placement are known). Static parts are scenery: baked into world coordinates,
added to ``dresser.extra`` (they get ambient occlusion and are joined per material at the export). Animated parts
stay single nodes: ``obstacle-<id>`` (``obstacle_parts``) and ``mechanic-<id>-ring`` (``mechanic_portals``, carries the node
extra ``aurago_spin``); they go to ``dresser.pass_parts`` (plain colours) and ``dresser.movers`` (hidden from the bake).

The visible core of a bumper is exactly its collision cylinder (radius and height of the manifest): decoration
(hoops, bands, lids) is flush with or inside it. The boost pad is a flat plate 5 mm over the floor (a draped decal,
no collision) with glowing arrows.
"""

import math

from . import mech_util, mechanic_cannon, mechanic_portals, obstacle_parts, shapes
from .mech_util import add_cylinder
from .prop import Prop

PLATE_LIFT = 0.005
BOOST_FRAME = 0.06


class Context:
    """What the part builders need: the world, the floor and ground height, and the lists that receive the objects."""

    def __init__(self, res, dresser):
        self.res, self.world, self.number = res, res.world, res.track.number
        self.floor, self.dresser = res.floor, dresser
        self.static, self.moving = [], []
        self.triangles = {}
        self.count = 0

    def seed(self):
        self.count += 1
        return 5000 + 40 * self.number + self.count

    def floor_z(self, x, y, fallback=0.0):
        z = self.floor.height(x, y)
        return fallback if z is None else z

    def ground(self, x, y):
        """Height of the lowest surface under ``(x, y)``: island ground or lane floor, whichever is lower."""
        top = self.dresser.top_height(x, y)
        z = self.floor.height(x, y)
        return top if z is None else min(top, z)

    def add_static(self, obj, location, heading=0.0):
        mech_util.to_world(obj, location, heading)
        self.static.append(obj)
        return obj


# -- bumpers ------------------------------------------------------------------------------------------------------

def _staves(x, y, z):
    return 0.8 if shapes.stave_index(x, y, 20) % 2 == 0 else 1.0


def _pirate_bumper(p, r, h):
    body = shapes.barrel("body", r - 0.035, r - 0.012, h - 0.04, seg=20, rows=8, groove=0.97)      # ends under the lid: no shared plane
    p.add(body, "matte", "wood", bevel=0, dens=0.3, pattern=_staves, noise_amp=0.05, smooth_angle=50)
    for z, half in ((0.13, 0.06), (h - 0.1, 0.05)):
        band = shapes.lathe("hoop", [(r - 0.012, z - half), (r, z - half), (r, z + half), (r - 0.012, z + half)], seg=20, caps=False)
        p.add(band, "metal", "iron", bevel=0, dens=0, ao=0.5)
    lid = shapes.cylinder("lid", r - 0.02, r - 0.03, 0.03, seg=20)
    p.add(lid, "matte", ("wood", "wood_light"), loc=(0, 0, h - 0.04), bevel=0.006, dens=0.3, ao=0.7)
    bung = shapes.cylinder("bung", 0.045, 0.04, 0.02, seg=10)
    p.add(bung, "matte", "wood_dark", loc=(0.09, 0.05, h - 0.02), bevel=0, dens=0)


def _temple_bumper(p, r, h):
    """A stone mushroom: foot, narrow waist and a domed cap, a gold band and a gem; the foot and the cap are the full radius."""
    add_cylinder(p, (0, 0, 0.11), r, 0.22, "stone_deep", bevel=0.02, noise_amp=0.06, ao=0.7)
    add_cylinder(p, (0, 0, 0.27), r * 0.68, 0.12, "stone", bevel=0.012, ao=0.8)
    cap = r - 0.012
    dome = shapes.lathe("dome", [(cap * 0.82, 0.33), (cap, 0.37), (cap, h - 0.13), (cap * 0.95, h - 0.075), (cap * 0.72, h - 0.03), (cap * 0.4, h - 0.007), (0.0, h)], seg=20)
    shapes.smooth_by_angle(dome.data, 55.0)
    p.add(dome, "matte", "stone", finish=False, noise_amp=0.05, ao=0.6)
    add_cylinder(p, (0, 0, 0.43), r, 0.07, "gold", mat="satin", bevel=0.01, ao=0.4)
    gem = shapes.ico_sphere("gem", 0.05, 1)
    p.add(gem, "glow", ("gold_light", "gold_light"), loc=(0, 0, h - 0.05), bevel=0, dens=0, ao=0.0, strength=0.7)


def _moon_bumper(p, r, h):
    add_cylinder(p, (0, 0, 0.07), r, 0.14, ("panel_dark", "metal_dark"), mat="satin", bevel=0.015, ao=0.6)
    add_cylinder(p, (0, 0, 0.14 + (h - 0.3) / 2), r - 0.014, h - 0.3, ("panel_dark", "metal_dark"), mat="satin", bevel=0.01, ao=0.5)
    for z, tone in ((0.22, "cyan"), (0.4, "magenta")):
        add_cylinder(p, (0, 0, z), r, 0.04, (tone, tone), mat="emissive", bevel=0.004, segments=1, dens=0, ao=0.0, strength=2.5)
    add_cylinder(p, (0, 0, h - 0.085), r - 0.01, 0.15, ("metal_dark", "metal"), mat="satin", bevel=0.02, ao=0.5)
    add_cylinder(p, (0, 0, h - 0.012), r * 0.55, 0.024, ("cyan", "cyan"), mat="emissive", bevel=0, dens=0, ao=0.0, strength=2.5)


def _bumper(ctx, item):
    r, h = item["radius"], item["height"]
    p = Prop(ctx.world, "bumper-" + item["id"], seed=ctx.seed())
    {"pirate": _pirate_bumper, "temple": _temple_bumper, "moon": _moon_bumper}[ctx.world](p, r, h)
    obj = p.build()
    radial = max(math.hypot(v.co.x, v.co.y) for v in obj.data.vertices)
    low, top = min(v.co.z for v in obj.data.vertices), max(v.co.z for v in obj.data.vertices)
    if radial > r + 0.002 or radial < r - 0.01 or abs(top - h) > 0.003 or low < -0.002:
        raise RuntimeError("bumper '%s': the visible core (radius %.3f, z %.3f to %.3f) differs from the collision cylinder (radius %.3f, height %.3f)"
                           % (item["id"], radial, low, top, r, h))
    ctx.add_static(obj, (item["x"], item["y"], item["z"]))


# -- boost pad ----------------------------------------------------------------------------------------------------

def _chevron(u0, depth, width, thick):
    return [(u0 + depth, 0.0), (u0 - depth, width), (u0 - depth - thick, width), (u0 + depth - thick, 0.0), (u0 - depth - thick, -width), (u0 - depth, -width)]


def _boost(ctx, item):
    x, y, h, length, width = item["x"], item["y"], item["heading"], item["length"], item["width"]
    cos, sin = math.cos(h), math.sin(h)

    def world(points):
        return [(x + u * cos - v * sin, y + u * sin + v * cos) for u, v in points]

    pal = Prop(ctx.world, "boost").pal
    dark, frame = pal.c("panel_dark"), pal.c("orange")
    cyan = pal.c("cyan")
    inner = [(-length / 2 + BOOST_FRAME, -width / 2 + BOOST_FRAME), (length / 2 - BOOST_FRAME, -width / 2 + BOOST_FRAME),
             (length / 2 - BOOST_FRAME, width / 2 - BOOST_FRAME), (-length / 2 + BOOST_FRAME, width / 2 - BOOST_FRAME)]
    outer = [(-length / 2, -width / 2), (length / 2, -width / 2), (length / 2, width / 2), (-length / 2, width / 2)]
    ground = ctx.floor_z
    frame_quads = [world([outer[k], outer[(k + 1) % 4], inner[(k + 1) % 4], inner[k]]) for k in range(4)]
    base = mech_util.draped("boost-plate", [world(inner)], ground, PLATE_LIFT, lambda px, py, pz: dark, "satin", ao=0.4)
    rim = mech_util.draped("boost-frame", frame_quads, ground, PLATE_LIFT, lambda px, py, pz: frame, "satin", ao=0.4)
    depth, arrow, thick = length * 0.075, width * 0.3, length * 0.07
    arrows = mech_util.draped("boost-arrows", [world(_chevron(u0, depth, arrow, thick)) for u0 in (-length / 4.2, 0.0, length / 4.2)],
                              ground, PLATE_LIFT + 0.003, lambda px, py, pz: cyan, "glow", ao=0.0, strength=1.0)
    ctx.static.extend([base, rim, arrows])


# -- entry ----------------------------------------------------------------------------------------------------------

def build_all(res, dresser, model):
    """Build the visible parts of every mechanic and obstacle of ``res`` and hand them to ``dresser`` (see the module docstring).

    Returns the context; ``ctx.triangles`` maps every item id to the triangles of the objects it added.
    """
    ctx = Context(res, dresser)
    shapes.set_collection(model)
    try:
        for item in res.mechanic_items + res.obstacle_items:
            before = len(ctx.static), len(ctx.moving)
            if item.get("type") == "mechanic":
                {"bumper": _bumper, "boost": _boost, "teleport": mechanic_portals.build, "cannon": mechanic_cannon.build}[item["kind"]](ctx, item)
            else:
                obstacle_parts.build(ctx, item)
            ctx.triangles[item["id"]] = sum(shapes.tri_count(obj) for obj in ctx.static[before[0]:] + ctx.moving[before[1]:])
    finally:
        shapes.set_collection(None)
    dresser.extra.extend(ctx.static)
    dresser.pass_parts.extend(ctx.moving)
    dresser.movers.extend(ctx.moving)
    return ctx
