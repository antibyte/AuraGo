"""Small geometry helpers of the course structures (MIT License, see ../LICENSE).

Plain vector maths on tuples (no ``bpy``, so the planners can run outside Blender) plus two Blender builders:
``oriented_box`` (a box along three arbitrary orthogonal axes, used for beams, braces and arms) and
``section_frame`` (the frame of a track section: origin, forward, across to the left, up).
"""

import math

from . import shapes
from .track import ROWS

_BOX_FACES = [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)]


def sub(a, b):
    return (a[0] - b[0], a[1] - b[1], a[2] - b[2])


def add(a, b):
    return (a[0] + b[0], a[1] + b[1], a[2] + b[2])


def mul(a, k):
    return (a[0] * k, a[1] * k, a[2] * k)


def dot(a, b):
    return a[0] * b[0] + a[1] * b[1] + a[2] * b[2]


def cross(a, b):
    return (a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0])


def length(a):
    return math.sqrt(dot(a, a))


def unit(a):
    n = length(a)
    return (a[0] / n, a[1] / n, a[2] / n) if n > 1e-12 else (0.0, 0.0, 1.0)


def lerp(a, b, t):
    return (a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t)


def at(origin, *terms):
    """``origin + sum(vector * factor)`` for ``(vector, factor)`` pairs."""
    x, y, z = origin
    for vec, k in terms:
        x, y, z = x + vec[0] * k, y + vec[1] * k, z + vec[2] * k
    return (x, y, z)


def section_frame(section):
    """``(origin, forward, across, up)`` of a track section; ``across`` points to the left edge of the lane."""
    across = unit(sub(section.points[0], section.points[ROWS]))
    up = unit(section.normal)
    forward = unit(cross(across, up))
    return section.center, forward, across, up


def frame_between(s0, s1, u):
    """Frame ``(origin, forward, across, up)`` of the cross-section a fraction ``u`` of the way from section ``s0`` to ``s1``."""
    f0, f1 = section_frame(s0), section_frame(s1)
    across = unit(lerp(f0[2], f1[2], u))
    up = unit(lerp(f0[3], f1[3], u))
    return lerp(s0.center, s1.center, u), unit(cross(across, up)), across, up


def oriented_box(name, center, u, v, w, size):
    """Box of full edge lengths ``size = (su, sv, sw)`` along the orthonormal axes ``u``, ``v``, ``w`` around ``center``.

    A left-handed axis triple is handled (the faces are flipped), so callers need not care about the order.
    """
    hu, hv, hw = size[0] / 2.0, size[1] / 2.0, size[2] / 2.0
    corners = [(-1, -1, -1), (1, -1, -1), (1, 1, -1), (-1, 1, -1), (-1, -1, 1), (1, -1, 1), (1, 1, 1), (-1, 1, 1)]
    verts = [at(center, (u, a * hu), (v, b * hv), (w, c * hw)) for a, b, c in corners]
    faces = _BOX_FACES if dot(cross(u, v), w) > 0 else [f[::-1] for f in _BOX_FACES]
    return shapes.make_object(name, verts, faces, smooth=False)


def beam_between(name, a, b, thickness, depth, up_hint=(0.0, 0.0, 1.0)):
    """Rectangular beam from ``a`` to ``b``: ``thickness`` across, ``depth`` along ``up_hint`` (made orthogonal)."""
    axis = unit(sub(b, a))
    side = cross(up_hint, axis)
    side = unit(side) if length(side) > 1e-6 else (1.0, 0.0, 0.0)
    up = unit(cross(axis, side))
    centre = lerp(a, b, 0.5)
    return oriented_box(name, centre, axis, side, up, (length(sub(b, a)), thickness, depth))
