"""Blender objects around the hole of a course: collision cup, cup rim, tee mark, flag (MIT License, see ../LICENSE).

``floor_cup`` is the collision cup: a tube of ``CUP_SEGMENTS`` corners that reaches ``CUP_DEPTH`` below the rim
plus a bottom disc, one node ``floor-cup`` with ``aurago_collider`` = ``grass`` and a dark vertex colour. Its rim
corners are the very tuple objects of the hole polygon in the floor (``pad.cup_ring``), so the float bit patterns
agree and the runtime welds floor and cup. The tube faces inwards, the disc up. The cup lining and bottom of the
legacy courses are gone: this node is also the visible inside of the hole.

``cup_rim`` is the thin ring (named ``cup-rim``) that ``dress_parts.restyle`` thickens, bevels and colours; its
inner edge follows the hole polygon 4 mm above the floor. ``tee_ring`` and ``flag`` complete the marks.
"""

from . import basic_parts, colors, materials
from .track import CUP_DEPTH, CUP_RADIUS

CUP_COLOUR = (0.035, 0.045, 0.055, 1.0)
RIM_LIFT = 0.004
RIM_WIDTH = 0.075
FLAG_OFFSET = 0.43


def cup_pad(track):
    """The pad of the track that carries the hole (green or funnel)."""
    for pad in track.pads:
        if pad.cup_ring is not None:
            return pad
    raise ValueError("the track has no cup")


def floor_cup(track, scene, coll):
    """Collision cup node ``floor-cup`` (see the module docstring)."""
    pad = cup_pad(track)
    ring, centre = pad.cup_ring, pad.cup
    bottom = centre[2] - CUP_DEPTH
    n = len(ring)
    verts = list(ring) + [(p[0], p[1], bottom) for p in ring] + [(centre[0], centre[1], bottom)]
    faces = []
    for k in range(n):
        k2 = (k + 1) % n
        faces.append((k, k2, n + k2))              # inwards: top edge, then down
        faces.append((k, n + k2, n + k))
        faces.append((2 * n, n + k, n + k2))       # bottom disc, up
    obj = basic_parts.mesh_object(scene, coll, "floor-cup", verts, faces, materials.get("matte"), "play-surface")
    obj["aurago_collider"] = "grass"
    colors.paint_values(obj, [CUP_COLOUR] * len(verts), ao=0.9)
    return obj


def cup_rim(track, scene, coll):
    """Flat ring ``cup-rim`` around the hole polygon (faces wind downwards like the legacy rings)."""
    pad = cup_pad(track)
    ring, centre = pad.cup_ring, pad.cup
    scale = (CUP_RADIUS + RIM_WIDTH) / CUP_RADIUS
    n = len(ring)
    verts = [(p[0], p[1], p[2] + RIM_LIFT) for p in ring]
    verts += [(centre[0] + (p[0] - centre[0]) * scale, centre[1] + (p[1] - centre[1]) * scale,
               centre[2] + (p[2] - centre[2]) * scale + RIM_LIFT) for p in ring]
    faces = []
    for k in range(n):
        k2 = (k + 1) % n
        faces.append((k, k2, n + k2))
        faces.append((k, n + k2, n + k))
    return basic_parts.mesh_object(scene, coll, "cup-rim", verts, faces, None, "decoration")


def tee_ring(track, scene, coll):
    """Thin inlaid ring ``tee-ring`` on the tee mark."""
    tee = track.tee
    return basic_parts.ring(scene, coll, "tee-ring", tee, .10, .18, tee[2] + .012, None, 12)


def flag(track, scene, coll, number):
    """Flag pole, flag and the raised hole number beside the cup."""
    cup = track.cup
    pole_mat = basic_parts.material("flag-pole", (.83, .84, .79, 1), metallic=.45, roughness=.28)
    basic_parts.cylinder(scene, coll, "flag-pole-base", (cup[0] + FLAG_OFFSET, cup[1], cup[2] + .43), .025, .86, pole_mat, 7)
    flag_mat = basic_parts.material("flag-cloth-" + track.world, (.9, .2, .2, 1), roughness=.82)
    verts = [(cup[0] + FLAG_OFFSET, cup[1], cup[2] + .82), (cup[0] + FLAG_OFFSET, cup[1], cup[2] + .55),
             (cup[0] + FLAG_OFFSET + .39, cup[1], cup[2] + .67)]
    basic_parts.mesh_object(scene, coll, "flag", verts, [(0, 1, 2)], flag_mat, "marker")
    digit_mat = basic_parts.material("hole-number-" + track.world, (1.0, .87, .46, 1), roughness=.42, emission=1.2)
    basic_parts.hole_number(scene, coll, cup, number, digit_mat)


def hole_marks(track, scene, coll, number):
    """Tee ring, cup rim, flag and number; the templates come from ``basic_parts.add_templates``."""
    tee_ring(track, scene, coll)
    cup_rim(track, scene, coll)
    flag(track, scene, coll, number)
    basic_parts.add_templates(scene, coll, {}, track.tee)
