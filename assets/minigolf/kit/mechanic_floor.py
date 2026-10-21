"""Floor height and placement of mechanics and obstacles (MIT License, see ../LICENSE).

Plain Python, no ``bpy``. The course data places a mechanic or obstacle in Blender metres, either at ``at``
(``(x, y)`` or ``(x, y, z)``) or at a named anchor of the piece list (``anchor`` plus ``offset``
``(forward, left[, up])`` in the anchor frame). ``heading`` is in degrees; with an anchor it is added to the anchor
heading, without it the default is the direction of the nearest route centre line. ``z`` is the floor height: when
the data does not give it, ``FloorField`` reads it from the floor triangles of the track (the lowest one where two
stretches lie above each other; give ``z`` there). ``runtime`` and ``blender`` swap the axes like
``basic_parts`` (runtime ``(x, z, -y)``), kept here so the data layer needs no ``bpy``.
"""

import math

from .bed_field import BedField
from .track import ROWS

EDGE_REACH = 1e-6


def runtime(p):
    return [float(p[0]), float(p[2]), float(-p[1])]


def blender(p):
    return (float(p[0]), float(-p[2]), float(p[1]))


def rounded(values, places=5):
    return [round(float(v), places) for v in values]


def _quad(a, b, c, d, out):
    out.append((a, b, c, 0.0))
    out.append((a, c, d, 0.0))


def all_triangles(track):
    """``[(a, b, c, arc)]`` plan-view triangles of every floor of the track (3D points, risers are skipped later)."""
    out = []
    for strip in track.strips:
        for s0, s1 in zip(strip.sections, strip.sections[1:]):
            for j in range(ROWS):
                _quad(s0.points[j], s0.points[j + 1], s1.points[j + 1], s1.points[j], out)
    for pad in track.pads:
        grid = pad.grid
        if grid.get("polar"):
            out.extend((a, b, c, 0.0) for a, b, c in pad.faces)
        else:
            points = grid["points"]
            for i, j in pad.faces:
                _quad(points[i][j], points[i][j + 1], points[i + 1][j + 1], points[i + 1][j], out)
            out.extend((a, b, c, 0.0) for a, b, c in pad.tris)
    return out


class FloorField:
    """Floor height under a plan point (Blender coordinates)."""

    def __init__(self, track):
        self.triangles = all_triangles(track)
        self.field = BedField(self.triangles)

    def height(self, x, y):
        """Lowest floor height at ``(x, y)``, or None when no floor lies there."""
        hits = [z for distance, z, _ in self.field.nearest(x, y, EDGE_REACH) if distance <= EDGE_REACH]
        return min(hits) if hits else None

    def spread(self, x, y, radius):
        """``(lowest, highest)`` floor height over the centre and eight points of a circle; None when part of it is off the floor."""
        zs = []
        for k in range(9):
            px, py = (x, y) if k == 8 else (x + radius * math.cos(k * math.pi / 4), y + radius * math.sin(k * math.pi / 4))
            z = self.height(px, py)
            if z is None:
                return None
            zs.append(z)
        return min(zs), max(zs)


def route_heading(track, x, y):
    """Direction (radians) of the route centre line segment nearest to ``(x, y)``."""
    best = None
    for line in track.corridors or [track.path]:
        for a, b in zip(line, line[1:]):
            vx, vy = b[0] - a[0], b[1] - a[1]
            length2 = vx * vx + vy * vy
            if length2 < 1e-12:
                continue
            t = max(0.0, min(1.0, ((x - a[0]) * vx + (y - a[1]) * vy) / length2))
            distance = math.hypot(x - a[0] - t * vx, y - a[1] - t * vy)
            if best is None or distance < best[0]:
                best = (distance, math.atan2(vy, vx))
    if best is None:
        raise ValueError("the course has no route to take a heading from")
    return best[1]


def locate(track, floor, item, where):
    """Resolve the placement keys of ``item`` to ``(x, y, z, heading)``: Blender metres and radians.

    ``where`` names the entry in error messages.
    """
    if "anchor" in item:
        anchor = track.anchors.get(item["anchor"])
        if anchor is None:
            raise ValueError("%s: unknown anchor %r (known: %s)" % (where, item["anchor"], ", ".join(sorted(track.anchors)) or "none"))
        ax, ay, az, ah = anchor
        forward, left, up = (tuple(item.get("offset", (0.0, 0.0))) + (0.0,))[:3]
        x = ax + forward * math.cos(ah) - left * math.sin(ah)
        y = ay + forward * math.sin(ah) + left * math.cos(ah)
        z = item["z"] if "z" in item else floor.height(x, y)
        z = (az if z is None else z) + up
        heading = ah + math.radians(item.get("heading", 0.0))
    elif "at" in item:
        at = item["at"]
        if len(at) not in (2, 3):
            raise ValueError("%s: 'at' takes (x, y) or (x, y, z)" % where)
        x, y = float(at[0]), float(at[1])
        z = float(at[2]) if len(at) == 3 else (float(item["z"]) if "z" in item else floor.height(x, y))
        if z is None:
            raise ValueError("%s: (%.2f, %.2f) is not on the floor; give 'at': (x, y, z) or move it" % (where, x, y))
        heading = math.radians(item["heading"]) if "heading" in item else route_heading(track, x, y)
    else:
        raise ValueError("%s: give 'at': (x, y) or 'anchor': name" % where)
    return x, y, float(z), heading
