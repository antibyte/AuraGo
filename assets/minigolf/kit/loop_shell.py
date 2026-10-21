"""Outer shell of a loop: the visible underside of the floor (MIT License, see ../LICENSE).

Kit materials cull back faces and a track floor is a single surface, so the loop floor has no outer side: from the
game camera, which looks down onto the loop from outside, the top and the sides of the ring are culled and only the
rail trims remain. ``build`` gives every ``loop`` strip a thin metal shell (scenery, no collision) that closes the
ring:

* the floor surface offset along its own normal by ``THICKNESS`` (the depth of the rail trims, 7 cm, so the underside
  runs flush into the trim), as a bottom strip of two quads across the lane with outward (``-normal``) faces;
* two side walls from the bottom edge up to the floor plane, ``REACH`` beyond the lane edge, that is inside the volume
  of the rail trim (the trim reaches 0.17 m outside the edge), so they close the shell without z-fighting the trim.

The shell lies on the far side of the floor, away from the ball (clearance), and never touches the collision nodes. It
is joined with the other structure parts (``structures.build``), baked and merged into ``scenery-<material>``.
``shell_geometry(strip)`` is plain Python (no ``bpy``), so ``structure_selftest`` can check the winding and extents.
"""

import math

from . import part_utils as pu
from .track import ROWS

THICKNESS = 0.07            # underside depth along the floor normal (the rail trims reach 7 cm below the lane edge)
REACH = 0.16                # side walls stand this far outside the lane edge, 1 cm inside the outer face of the trim
RING = 5                    # per section: top left, bottom left, bottom middle, bottom right, top right
BAND = 0.45                 # route metres per alternating panel tone (the metal floor uses the same)
BAND_AMPLITUDE = 0.04
AO_WEIGHT = 0.7
MATERIALS = {"pirate": "matte", "temple": "matte", "moon": "satin"}      # the structure material of each world


def _ring(section):
    """The five cross-section points of the shell of ``section`` (see ``RING``)."""
    left, right = section.points[0], section.points[ROWS]
    across, up = pu.unit(pu.sub(left, right)), pu.unit(section.normal)
    top_left, top_right = pu.at(left, (across, REACH)), pu.at(right, (across, -REACH))
    down = pu.mul(up, -THICKNESS)
    bottom_left, bottom_right = pu.add(top_left, down), pu.add(top_right, down)
    return [top_left, bottom_left, pu.lerp(bottom_left, bottom_right, 0.5), bottom_right, top_right]


def shell_geometry(strip):
    """``(vertices, faces, arcs)`` of the shell of one strip: ``RING`` vertices per section, outward-facing triangles.

    ``arcs`` is the route distance of the section of each vertex (for the panel tones).
    """
    rings, arcs, travelled = [], [], 0.0
    for i, section in enumerate(strip.sections):
        if i:
            travelled += pu.length(pu.sub(section.center, strip.sections[i - 1].center))
        rings.append(_ring(section))
        arcs.extend([travelled] * RING)
    verts = [point for ring in rings for point in ring]
    faces = []
    for i in range(len(rings) - 1):
        a, b = i * RING, (i + 1) * RING
        for j in range(RING - 1):
            # with forward T and the ring edge d the normal is T x d: down for the bottom, outwards for both walls
            faces.extend([(a + j, b + j, b + j + 1), (a + j, b + j + 1, a + j + 1)])
    return verts, faces, arcs


def _colours(look, arcs):
    """Vertex colours: the world's metal floor gradient, lighter in the middle, with alternating panel tones."""
    from . import floor_paint, palette
    from .palette import mix
    lo, hi = palette.get(look).grad(floor_paint.GRADIENTS[look]["metal"])
    tones = (0.30, 0.42, 0.76, 0.42, 0.30)         # position in the gradient of the ring points (walls dark, middle light)
    out = []
    for k, arc in enumerate(arcs):
        c = mix(lo, hi, tones[k % RING])
        band = 1.0 + (BAND_AMPLITUDE if int(arc / BAND) % 2 == 0 else -BAND_AMPLITUDE)
        out.append((c[0] * band, c[1] * band, c[2] * band, 1.0))
    return out


def build(look, track, number):
    """One joined mesh ``loop-shell`` of every loop strip (None without loops); call with the structures' collection active."""
    strips = [strip for strip in track.strips if strip.kind == "loop"]
    if not strips:
        return None
    from . import colors, materials, shapes
    from .prop import Prop
    p = Prop(look, "loop-shell", seed=number, full_name="loop-shell")
    for strip in strips:
        verts, faces, arcs = shell_geometry(strip)
        obj = shapes.make_object("loop-shell-body", verts, faces, smooth=False)
        shapes.finish(obj, bevel=0, dens=0, weighted=False, smooth_angle=50.0)
        colors.paint_values(obj, _colours(look, arcs), ao=AO_WEIGHT)
        obj.data.materials.append(materials.get(MATERIALS[look]))
        p.parts.append(obj)
    return p.build()
