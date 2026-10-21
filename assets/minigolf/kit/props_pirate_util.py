"""Small helpers shared by the pirate bay prop modules (MIT License).

Part builders that write their own vertex colours (ring-segmented trunks,
frond meshes, striped towers) need vertex order to survive ``shapes.finish``;
``add_painted`` wraps that dance: finish with ``dens=0`` and no bevel (order
preserving), write the colours, register the part unfinished.
"""

from mathutils import Vector

from . import colors, shapes
from .palette import hex_to_linear


def smoothstep(a, b, x):
    """Hermite step from 0 (x <= a) to 1 (x >= b)."""
    if b == a:
        return 1.0 if x >= b else 0.0
    t = min(1.0, max(0.0, (x - a) / (b - a)))
    return t * t * (3.0 - 2.0 * t)


def tone(hex_value, k=1.0):
    """Extra sRGB tone as linear RGBA, optionally scaled in brightness."""
    r, g, b, a = hex_to_linear(hex_value)
    return (r * k, g * k, b * k, a)


def scaled(c, k):
    return (c[0] * k, c[1] * k, c[2] * k, c[3])


def orient_face(verts, ids, hint):
    """Face index tuple wound so that its normal points along ``hint``."""
    a, b, c = (Vector(verts[i]) for i in ids[:3])
    n = (b - a).cross(c - a)
    return tuple(ids) if n.dot(hint) >= 0.0 else tuple(reversed(ids))


def add_painted(p, obj, mat, cols, ao=0.6, smooth_angle=60.0, **mat_opts):
    """Finish ``obj`` without changing vertex order, paint ``cols`` and register it.

    ``cols`` holds one linear RGBA tuple per vertex of ``obj`` (as built, before
    any topology change; ``finish`` is called with no bevel and no densify).
    """
    shapes.finish(obj, bevel=0, dens=0, smooth_angle=smooth_angle)
    if len(cols) != len(obj.data.vertices):
        raise ValueError("%s: %d colours for %d vertices" % (obj.name, len(cols), len(obj.data.vertices)))
    colors.paint_values(obj, cols, ao=ao)
    return p.add(obj, mat, finish=False, paint=False, **mat_opts)
