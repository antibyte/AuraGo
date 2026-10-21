"""Gate panels and the static frames of obstacles (MIT License, see ../LICENSE).

``panel`` draws the moving panel of a ``gate`` or ``slide`` obstacle into the prop of its ``obstacle-<id>`` node: a box
of exactly the collision extents (pirate: planks with iron straps, temple: carved stone with a gold medallion, moon: a
blast door with a hazard band and a light bar); every relief piece ends flush with or inside the box. ``frame`` builds the
static scenery around it in world coordinates: the posts and lintel the gate runs in, or the gallows a pendulum hangs
from. Frames stand outside the rails; the posts reach down to the ground (``ctx.ground``).
"""

import math

from . import shapes
from .mech_util import add_box, add_cylinder
from .obstacle_data import POST, SINK
from .prop import Prop

LINTEL = 0.1
INSET = 0.004


def _stripes(period, first, second, axis=1, skew=0.0):
    def paint(x, y, z):
        return first if int(math.floor(((x, y, z)[axis] + skew * z + 50.0) / period)) % 2 == 0 else second
    return paint


def _flat(*args, **kw):
    kw.setdefault("dens", 0)
    return add_box(*args, **kw)


def _quad_object(name, verts, faces):
    return shapes.make_object(name, verts, faces, smooth=False)


def _hazard(p, side, x, half_width, z0, z1):
    """A dark band with orange diagonal stripes on the face at ``x`` (outward ``side``): flat quads, so no vertex density is needed."""
    def face(points):
        ring = [(side * x, y, z) for y, z in points]
        return ring if side > 0 else ring[::-1]

    band = _quad_object("hazard-band", face([(-half_width, z0), (half_width, z0), (half_width, z1), (-half_width, z1)]), [(0, 1, 2, 3)])
    p.add(band, "satin", ("panel_dark", "panel_dark"), finish=False, ao=0.4)
    stripes, faces = [], []
    lean, thick = 0.09, 0.1
    y = -half_width
    while y + thick + lean <= half_width:
        base = len(stripes)
        stripes.extend(face([(y, z0), (y + thick, z0), (y + thick + lean, z1), (y + lean, z1)]))
        faces.append((base, base + 1, base + 2, base + 3))
        y += 2 * thick
    p.add(_quad_object("hazard-stripes", stripes, faces), "satin", ("orange", "orange"), loc=(side * 0.0005, 0, 0), finish=False, ao=0.4)


def panel(p, built):
    """The moving panel: a slab 12 mm thinner than the collision box with relief that ends exactly on the box faces."""
    part = built["parts"][0]
    t, w, h = part["half"]
    world = built["world"]
    pal = p.pal
    if world == "pirate":
        planks = max(2, round(2 * w / 0.14))
        width = 2 * w / planks
        for k in range(planks):
            tone = ("wood_dark", "wood") if k % 2 == 0 else ("wood", "wood_light")
            _flat(p, (0, -w + width * (k + 0.5), 0), (t - 0.012, width / 2, h), tone, bevel=0, ao=0.6)
        for z in (-h + 0.1, 0.0, h - 0.1):
            _flat(p, (0, 0, z), (t, w, 0.045), "iron", mat="metal", bevel=0.008, segments=1, ao=0.5)
        for y in (-w + 0.08, w - 0.08):
            _flat(p, (0, y, 0), (t, 0.035, h), "iron", mat="metal", bevel=0.008, segments=1, ao=0.5)
    elif world == "temple":
        _flat(p, (0, 0, 0), (t - 0.012, w, h), "stone", bevel=0.03, noise_amp=0.07, ao=0.6, dens=0.4)
        for side in (-1, 1):
            add_cylinder(p, (side * (t - 0.014), 0, 0.0), min(0.2, h * 0.45), 0.024, "gold", mat="satin", axis="x", seg=20, bevel=0.003, segments=1, dens=0, ao=0.4)
            add_cylinder(p, (side * (t - 0.007), 0, 0.0), min(0.12, h * 0.28), 0.014, ("gold_light", "gold_light"), mat="glow", axis="x", seg=20, bevel=0, dens=0, ao=0.0, strength=0.7)
            for z, tone in ((h - 0.09, "totem_teal"), (-h + 0.09, "totem_red")):
                _flat(p, (side * (t - 0.006), 0, z), (0.006, w - 0.1, 0.035), (tone, tone), bevel=0.002, segments=1, ao=0.5)
    else:
        _flat(p, (0, 0, 0), (t - 0.012, w, h), ("panel_dark", "metal"), mat="metal", bevel=0.02, ao=0.5, dens=0.4)
        for side in (-1, 1):
            _hazard(p, side, t - 0.004, w - 0.06, -h + 0.02, -h + 0.18)
            _flat(p, (side * (t - 0.004), 0, h - 0.12), (0.004, w - 0.2, 0.025), ("cyan", "cyan"), mat="emissive", bevel=0, segments=1, ao=0.0, strength=1.6)
            _flat(p, (side * (t - 0.004), 0, 0.0), (0.004, 0.012, h - 0.2), ("panel_dark", "panel_dark"), bevel=0, segments=1, ao=0.3)


def _base(ctx, x, y, z, side, heading, offset):
    """Height of the ground below a post at ``offset`` along ``side`` from ``(x, y)`` relative to the floor height ``z``."""
    px, py = x + offset * math.cos(side), y + offset * math.sin(side)
    return ctx.ground(px, py) - z - 0.12


def _pirate_post(p, centre_y, base, top, depth):
    add_box(p, (0, centre_y, (base + top) / 2), (depth, POST, (top - base) / 2), ("wood_dark", "wood"), bevel=0.025, noise_amp=0.06, ao=0.6)
    for z in (base + 0.35, top - 0.3):
        add_box(p, (0, centre_y, z), (depth + 0.012, POST + 0.012, 0.04), "iron", mat="metal", bevel=0.006, segments=1, dens=0.25, ao=0.5)
    add_box(p, (0, centre_y, top + 0.025), (depth + 0.025, POST + 0.025, 0.025), "iron", mat="metal", bevel=0.008, segments=1, dens=0.25, ao=0.5)


def _temple_post(p, centre_y, base, top, depth):
    add_box(p, (0, centre_y, (base + top) / 2), (depth, POST, (top - base) / 2), ("stone_deep", "stone"), bevel=0.03, noise_amp=0.08, ao=0.6)
    add_box(p, (0, centre_y, base + 0.12), (depth + 0.03, POST + 0.03, 0.12), "stone_deep", bevel=0.02, ao=0.7)
    for z in (top - 0.18, top - 0.3):
        add_box(p, (0, centre_y, z), (depth + 0.012, POST + 0.012, 0.03), "gold", mat="satin", bevel=0.006, segments=1, dens=0.25, ao=0.4)
    add_box(p, (0, centre_y, top + 0.03), (depth + 0.03, POST + 0.03, 0.03), "stone", bevel=0.012, ao=0.6)


def _moon_post(p, centre_y, base, top, depth):
    add_box(p, (0, centre_y, (base + top) / 2), (depth, POST, (top - base) / 2), ("panel_dark", "metal"), mat="metal", bevel=0.02, ao=0.5)
    for side in (-1, 1):
        add_box(p, (-depth + 0.004, centre_y + side * 0.06, (base + top) / 2 + 0.1), (0.004, 0.012, (top - base) / 2 - 0.25), ("cyan", "cyan"), mat="emissive",
                bevel=0, segments=1, dens=0, ao=0.0, strength=1.6)
    add_box(p, (0, centre_y, top + 0.03), (depth + 0.02, POST + 0.02, 0.03), "orange", mat="satin", bevel=0.01, ao=0.4)


POSTS = {"pirate": _pirate_post, "temple": _temple_post, "moon": _moon_post}


def frame(ctx, built):
    f = built["frame"]
    if f["kind"] == "door":
        _door(ctx, built, f)
    else:
        _gallows(ctx, built, f)


def _door(ctx, built, f):
    world = built["world"]
    x, y, z = f["centre"]
    heading, width, height, travel = f["heading"], f["width"], f["height"], f["travel"]
    p = Prop(world, "frame-" + built["id"], seed=ctx.seed())
    side = heading + math.pi / 2
    depth = 0.17
    top_rel = height - SINK + travel + 0.2 + LINTEL
    for sign in (-1, 1):
        offset = sign * (width / 2 + POST)
        POSTS[world](p, offset, _base(ctx, x, y, z, side, heading, offset), top_rel, depth)
    add_box(p, (0, 0, top_rel - LINTEL), (depth + 0.02, width / 2 + 2 * POST - 0.01, LINTEL), ("wood_dark", "wood") if world == "pirate" else ("stone", "stone_light") if world == "temple" else ("metal_dark", "metal"),
            mat="metal" if world == "moon" else "matte", bevel=0.025, noise_amp=0.05, ao=0.6)
    if world == "pirate":
        for sign in (-1, 1):
            add_cylinder(p, (0, sign * 0.3, top_rel - LINTEL + 0.02), 0.12, 0.05, "iron", mat="metal", axis="x", seg=14, bevel=0.008, ao=0.5)
            wheel = shapes.cylinder("pulley", 0.05, 0.05, 0.07, seg=10, base=False)
            p.add(wheel, "metal", "iron", loc=(depth + 0.04, sign * 0.3, top_rel - LINTEL + 0.02), rot=(0, 90, 0), bevel=0, dens=0, ao=0.4)
    elif world == "temple":
        add_box(p, (-depth - 0.004, 0, top_rel - LINTEL), (0.01, width / 2 - 0.1, 0.03), ("totem_teal", "totem_teal"), mat="glow", bevel=0, segments=1, dens=0, ao=0.0, strength=0.7)
    else:
        add_box(p, (-depth - 0.004, 0, top_rel - LINTEL), (0.01, width / 2, 0.025), ("cyan", "cyan"), mat="emissive", bevel=0, segments=1, dens=0, ao=0.0, strength=1.6)
    ctx.add_static(p.build(), (x, y, z), heading)


def _gallows(ctx, built, f):
    world = built["world"]
    px, py, pz = f["pivot"]
    floor = f["floor"]
    side = f["lateral"]
    heading = side - math.pi / 2
    span = f["span"]
    p = Prop(world, "gallows-" + built["id"], seed=ctx.seed())
    top_rel = pz - floor + 0.3
    for sign in (-1, 1):
        offset = sign * span
        base = _base(ctx, px, py, floor, side, heading, offset)
        POSTS[world if world != "moon" else "pirate"](p, offset, base, top_rel, 0.13)
        add_box(p, (0, sign * (span - 0.3), top_rel - 0.42), (0.06, 0.04, 0.3), ("wood_dark", "wood") if world == "pirate" else ("bark", "bark_light"),
                rot=(sign * 45, 0, 0), bevel=0.012, segments=1, dens=0.3, ao=0.6)
    beam = ("wood_dark", "wood") if world == "pirate" else ("bark", "bark_light")
    add_box(p, (0, 0, pz - floor + 0.16), (0.1, span + 0.12, 0.1), beam, bevel=0.025, noise_amp=0.06, ao=0.6)
    add_box(p, (0, 0, pz - floor + 0.05), (0.11, 0.09, 0.05), "iron" if world == "pirate" else ("lianas", "jungle_dark"), mat="metal" if world == "pirate" else "matte",
            bevel=0.01, segments=1, dens=0.25, ao=0.5)
    ctx.add_static(p.build(), (px, py, floor), heading)
