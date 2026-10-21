"""Free ponds of the layout (MIT License, see ../LICENSE).

``"ponds": [{"polygon": [(x, y), ...], "kind": "water", "z": 0.0}, ...]`` in the course data puts a pond (with
``"kind": "void"`` a chasm) beside or under the track in world coordinates, instead of inside a plaza. It becomes the
same hazard dict as a plaza pool (``kind``, ``polygon``, ``z``, ``min``, ``max``), so the island hollow, the pond water,
the manifest water volume and the supports under a bridge follow without further code. ``z`` is the height of the
ground around the pond (default: the tee height); the water lies 0.25 m under it.

A pond keeps clear of the floors: where a floor lies within ``MARGIN`` of the polygon (plan) it must be at least
``CLEARANCE`` above ``z``, so a pond lies beside a lane or under an elevated bridge, never in a lane.
"""

import math

from .mechanic_floor import FloorField

KINDS = ("water", "void")
MARGIN = 0.4
CLEARANCE = 0.5
STEP = 0.25
KEYS = {"polygon", "kind", "z"}


def _distance(polygon, x, y):
    """Signed plan distance of ``(x, y)`` to ``polygon`` (negative inside)."""
    inside, best = False, math.inf
    for k in range(len(polygon)):
        (x0, y0), (x1, y1) = polygon[k], polygon[(k + 1) % len(polygon)]
        if (y0 > y) != (y1 > y) and x < x0 + (y - y0) * (x1 - x0) / (y1 - y0):
            inside = not inside
        dx, dy = x1 - x0, y1 - y0
        length2 = dx * dx + dy * dy
        t = 0.0 if length2 < 1e-18 else max(0.0, min(1.0, ((x - x0) * dx + (y - y0) * dy) / length2))
        best = min(best, math.hypot(x - x0 - t * dx, y - y0 - t * dy))
    return -best if inside else best


def add_ponds(track, items, number):
    """Append one hazard per pond of ``items`` to ``track.hazards``; ``ValueError`` names the course and the pond."""
    if not items:
        return
    floor = FloorField(track)
    default_z = track.tee[2]
    for index, item in enumerate(items):
        where = "course %d, ponds[%d]: " % (number, index)
        unknown = sorted(set(item) - KEYS)
        if unknown:
            raise ValueError(where + "unknown field(s) " + ", ".join(unknown))
        polygon = [(float(p[0]), float(p[1])) for p in item.get("polygon", ())]
        kind = item.get("kind", "water")
        if len(polygon) < 3 or kind not in KINDS:
            raise ValueError(where + "a pond needs a polygon of at least 3 points and kind 'water' or 'void'")
        level = float(item.get("z", default_z))
        xs, ys = [p[0] for p in polygon], [p[1] for p in polygon]
        x0, x1, y0, y1 = min(xs) - MARGIN, max(xs) + MARGIN, min(ys) - MARGIN, max(ys) + MARGIN
        for i in range(int((x1 - x0) / STEP) + 1):
            for j in range(int((y1 - y0) / STEP) + 1):
                x, y = x0 + i * STEP, y0 + j * STEP
                if _distance(polygon, x, y) > MARGIN:
                    continue
                height = floor.height(x, y)
                if height is not None and height < level + CLEARANCE:
                    raise ValueError(where + "the pond reaches a lane at (%.2f, %.2f); keep it %.1f m clear of the floors (only a bridge "
                                     "at least %.1f m above z may pass over it)" % (x, y, MARGIN, CLEARANCE))
        track.hazards.append({"kind": kind, "polygon": polygon, "z": level, "min": (min(xs), min(ys)), "max": (max(xs), max(ys))})
