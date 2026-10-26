"""Visible parts of the teleporters (MIT License, see ../LICENSE).

None of these parts collide. The ball enters the zone disc of radius ``radius`` at the entry spot and shows up at the
``exit`` spot rolling along the exit heading, so the exit decoration stands behind the exit spot:

* pirate: a round hatch in the floor (planks around a dark pit, iron hoop; flat, a few millimetres high) as the entry and a
  barrel lying on its side with its open end at the exit
* temple: a stone arch with an idol head over the lane (teal glow trim at the entry, gold at the exit)
* moon: a transporter pad (flat dark disc, glowing ring) with a floating ring above it that turns
  (node ``mechanic-<id>-ring`` / ``mechanic-<id>-exit-ring`` with the node extra ``aurago_spin``)
"""

import math

from mathutils import Matrix

from . import mech_util, shapes
from .mech_util import add_box, draped, finish_object, ring_polygons
from .palette import mix
from .prop import Prop

RING_HEIGHT = 0.55
RING_SPEED = 1.3


def _spot_pair(item):
    return [(item["x"], item["y"], item["z"], item["heading"], "in"), (item["exit"][0], item["exit"][1], item["exit"][2], item["exit_heading"], "out")]


# -- pirate ---------------------------------------------------------------------------------------------------------

def _hatch(ctx, x, y, z, radius):
    pal = Prop(ctx.world, "hatch").pal
    pit, outer = radius * 0.95, radius + 0.14
    wood_dark, wood_light, iron, coal = pal.c("wood_dark"), pal.c("wood_light"), pal.c("iron"), pal.c("coal")
    sectors = 16

    def planks(px, py, pz):
        k = int(((math.atan2(py - y, px - x) % math.tau) / math.tau) * sectors) % 2
        return mix(wood_dark, wood_light, 0.35 + 0.35 * k)

    def depth(px, py, pz):
        return mix(coal, pal.c("wood_deep"), min(1.0, math.hypot(px - x, py - y) / pit) ** 2 * 0.8)

    ground = ctx.floor_z
    ctx.static.append(draped("hatch-pit", [mech_util.circle(x, y, pit, 32)], ground, 0.004, depth, "matte", ao=0.2))
    ctx.static.append(draped("hatch-planks", ring_polygons(x, y, pit, outer - 0.05, 32), ground, 0.008, planks, "matte", ao=0.5))
    ctx.static.append(draped("hatch-hoop", ring_polygons(x, y, outer - 0.05, outer, 32), ground, 0.011, lambda px, py, pz: iron, "metal", ao=0.4))


def _barrel_exit(ctx, x, y, z, heading):
    """A barrel lying on its side, its open end 5 cm behind the exit spot, facing along ``heading``."""
    length, r_end, r_belly, rows = 0.95, 0.31, 0.37, 8
    p = Prop(ctx.world, "exit-barrel", seed=ctx.seed())
    pal = p.pal
    outer = [(r_end + (r_belly - r_end) * math.sin(math.pi * k / rows) ** 0.85, length * k / rows) for k in range(rows + 1)]
    bore = r_end - 0.05
    profile = [(0.0, 0.0)] + outer + [(bore, length), (bore, length - 0.5), (0.0, length - 0.5)]
    body = shapes.lathe("exit-barrel-body", profile, seg=20)
    coal, deep = pal.c("coal"), pal.c("wood_deep")

    def paint(px, py, pz):
        radius = math.hypot(px, py)
        if pz > length - 0.55 and radius < bore + 0.004:
            return mix(coal, deep, 0.15)
        stave = 0.8 if shapes.stave_index(px, py) % 2 == 0 else 1.0
        base = mix(pal.c("wood_dark"), pal.c("wood_light"), min(1.0, 0.2 + 0.7 * pz / length))
        return tuple(c * stave for c in base[:3]) + (1.0,)

    shapes.smooth_by_angle(body.data, 50.0)
    p.add(body, "matte", "wood", finish=False, fn=paint, ao=0.6)
    for z0, half in ((0.15, 0.05), (length - 0.15, 0.05)):
        wall = r_end + (r_belly - r_end) * math.sin(math.pi * z0 / length) ** 0.85
        band = shapes.lathe("hoop", [(wall - 0.012, z0 - half), (wall + 0.01, z0 - half), (wall + 0.01, z0 + half), (wall - 0.012, z0 + half)], seg=20, caps=False)
        p.add(band, "metal", "iron", bevel=0, dens=0, ao=0.5)
    obj = p.build()
    obj.data.transform(Matrix.Translation((-length - 0.05, 0.0, r_belly)) @ Matrix.Rotation(math.radians(90), 4, "Y"))
    # The lathe stands on Z with its open end up; turned 90 degrees about Y the open end points along +X.
    ctx.add_static(obj, (x, y, z), heading)


# -- temple ---------------------------------------------------------------------------------------------------------

def _idol_arch(ctx, x, y, z, heading, radius, tone):
    p = Prop(ctx.world, "idol-arch", seed=ctx.seed())
    inner = radius + 0.1
    post, depth, rise = 0.12, 0.17, 0.95
    for sign in (-1, 1):
        add_box(p, (0, sign * (inner + post), rise / 2), (depth, post, rise / 2), "stone_deep", bevel=0.03, noise_amp=0.07, ao=0.7)
        add_box(p, (0, sign * (inner + post), 0.07), (depth + 0.03, post + 0.03, 0.07), "stone_deep", bevel=0.02, ao=0.8)
        add_box(p, (-depth + 0.01, sign * (inner + 0.012), rise / 2), (0.012, 0.012, rise / 2 - 0.05), (tone, tone), mat="glow", bevel=0, segments=1, dens=0, ao=0.0, strength=0.7)
    wide = inner + 2 * post + 0.03
    add_box(p, (0, 0, rise + 0.11), (depth + 0.03, wide, 0.11), "stone", bevel=0.03, noise_amp=0.06, ao=0.7)
    add_box(p, (-depth + 0.01, 0, rise - 0.012), (0.012, inner, 0.012), (tone, tone), mat="glow", bevel=0, segments=1, dens=0, ao=0.0, strength=0.7)
    # The idol head on the lintel: a block with brow, eyes, nose, mouth and a gold crown (front towards the incoming ball).
    top = rise + 0.22
    add_box(p, (0, 0, top + 0.17), (depth + 0.02, 0.3, 0.17), "stone", bevel=0.035, noise_amp=0.06, ao=0.6)
    add_box(p, (-depth - 0.02, 0, top + 0.27), (0.04, 0.3, 0.05), "stone_deep", bevel=0.015, ao=0.5)
    for sign in (-1, 1):
        eye = shapes.ico_sphere("eye", 0.05, 1)
        p.add(eye, "glow", ("gold_light", "gold_light"), loc=(-depth - 0.03, sign * 0.15, top + 0.2), bevel=0, dens=0, ao=0.0, strength=1.0)
    add_box(p, (-depth - 0.045, 0, top + 0.14), (0.04, 0.04, 0.07), "stone", bevel=0.012, segments=1, dens=0, ao=0.5)
    add_box(p, (-depth - 0.02, 0, top + 0.05), (0.025, 0.16, 0.025), ("black", "black"), bevel=0.006, segments=1, dens=0, ao=0.2)
    add_box(p, (0, 0, top + 0.37), (depth, 0.26, 0.04), "gold", mat="satin", bevel=0.012, segments=1, dens=0.2, ao=0.4)
    # Lined up with the lane: the incoming ball sees the front (-X side), so the arch is turned to face against the heading.
    ctx.add_static(p.build(), (x, y, z), heading)


# -- moon -----------------------------------------------------------------------------------------------------------

def _pad(ctx, x, y, z, radius, tone, spin, node):
    pal = Prop(ctx.world, "pad").pal
    dark, metal, glow = pal.c("panel_dark"), pal.c("metal"), pal.c(tone)
    outer = radius + 0.1
    ground = ctx.floor_z
    ctx.static.append(draped("pad-disc", [mech_util.circle(x, y, outer - 0.06, 32)], ground, 0.004, lambda px, py, pz: dark, "satin", ao=0.4))
    ctx.static.append(draped("pad-rim", ring_polygons(x, y, outer - 0.06, outer, 32), ground, 0.007, lambda px, py, pz: metal, "metal", ao=0.4))
    inner_ring = ring_polygons(x, y, radius * 0.72, radius * 0.8, 32)
    ticks = [[(x + a * math.cos(t) - b * math.sin(t), y + a * math.sin(t) + b * math.cos(t)) for a, b in ((radius * 0.25, -0.025), (radius * 0.6, -0.025), (radius * 0.6, 0.025), (radius * 0.25, 0.025))]
             for t in (k * math.pi / 2 + math.pi / 4 for k in range(4))]
    ctx.static.append(draped("pad-glow", inner_ring + ticks, ground, 0.009, lambda px, py, pz: glow, "glow", ao=0.0, strength=1.0))
    p = Prop(ctx.world, node, seed=ctx.seed(), full_name=node)
    major = radius * 0.9
    ring = shapes.torus("ring", major, 0.03, seg=28, rseg=5)
    p.add(ring, "satin", "metal", bevel=0, dens=0, ao=0.0, smooth_angle=60)
    for k in range(6):
        a = k * math.tau / 6
        tone_k = "cyan" if k % 2 == 0 else "magenta"
        block = shapes.box("tick", (0.08, 0.12, 0.07))
        p.add(block, "emissive", (tone_k, tone_k), loc=(major * math.cos(a), major * math.sin(a), 0.0), rot=(0, 0, math.degrees(a)), bevel=0, dens=0, ao=0.0, strength=2.5)
    obj = p.build()
    finish_object(obj, node, {"aurago_spin": {"axis": [0.0, 1.0, 0.0], "speed": spin}})
    obj.location = (x, y, z + RING_HEIGHT)
    ctx.moving.append(obj)


def build(ctx, item):
    """Entry and exit decoration of one teleporter."""
    radius = item["radius"]
    (x, y, z, h, _), (ex, ey, ez, eh, _) = _spot_pair(item)
    if ctx.world == "pirate":
        _hatch(ctx, x, y, z, radius)
        _barrel_exit(ctx, ex, ey, ez, eh)
    elif ctx.world == "temple":
        _idol_arch(ctx, x, y, z, h, radius, "totem_teal")
        _idol_arch(ctx, ex, ey, ez, eh, radius, "gold")
    else:
        _pad(ctx, x, y, z, radius, "cyan", RING_SPEED, "mechanic-%s-ring" % item["id"])
        _pad(ctx, ex, ey, ez, radius, "magenta", -RING_SPEED, "mechanic-%s-exit-ring" % item["id"])
