"""Grounded floor triangles of a track and the nearest-bed query (MIT License, see ../LICENSE).

Plain Python, no ``bpy``. ``grounded_triangles(track)`` lists the floor triangles that rest on the ground:
those of sections (and pads) with ``grounded`` true whose centre is at most ``GROUNDED_MAX`` above the start
height. Elevated pieces (bridges, ramp ends, loops) are left out, so they do not pull the ground up.
``BedField`` hashes them by their centroid in 1 m cells and answers ``nearest(x, y, reach)``: every triangle
within ``reach`` metres of the point with its plan distance, the triangle height at the nearest point and its
route arc (the stretch of the course it belongs to). ``floor_triangles(track, grounded_only=False)`` lists every floor
triangle, which the structures use to keep supports clear of other stretches.
"""

import math

from .track import GROUNDED_MAX, ROWS

CELL = 1.0
TRIANGLE_SPAN = 1.0       # no floor triangle is longer than this (a section is at most 0.62 m long)


def _quad(a, b, c, d, arc, out):
    out.append((a, b, c, arc))
    out.append((a, c, d, arc))


def floor_triangles(track, grounded_only=True):
    """``[(a, b, c, arc)]`` plan-view floor triangles (points are 3D): those that rest on the ground, or all of them."""
    base = track.tee[2]
    out = []
    for strip in track.strips:
        for s0, s1 in zip(strip.sections, strip.sections[1:]):
            if grounded_only and not (s1.grounded and s0.grounded and s1.center[2] - base <= GROUNDED_MAX and s0.center[2] - base <= GROUNDED_MAX):
                continue
            arc = 0.5 * (s0.arc + s1.arc)
            for j in range(ROWS):
                _quad(s0.points[j], s0.points[j + 1], s1.points[j + 1], s1.points[j], arc, out)
    for pad in track.pads:
        if grounded_only and not pad.grounded:
            continue
        grid = pad.grid
        if grid.get("polar"):
            arc = pad.arc0 + 0.5 * pad.size[0]
            out.extend((a, b, c, arc) for a, b, c in pad.faces)
        else:
            points, xs = grid["points"], grid["xs"]
            for i, j in pad.faces:
                _quad(points[i][j], points[i][j + 1], points[i + 1][j + 1], points[i + 1][j], pad.arc0 + xs[i], out)
            out.extend((a, b, c, pad.arc0 + 0.5 * pad.size[0]) for a, b, c in pad.tris)
    return out


def grounded_triangles(track):
    """``[(a, b, c, arc)]`` plan-view floor triangles of everything that rests on the ground (points are 3D)."""
    return floor_triangles(track, True)


def _closest(px, py, a, b, c):
    """``(squared plan distance, height)`` of the point ``(px, py)`` to the triangle ``abc`` (heights interpolated)."""
    (ax, ay, az), (bx, by, bz), (cx, cy, cz) = a, b, c
    v0x, v0y, v1x, v1y = bx - ax, by - ay, cx - ax, cy - ay
    det = v0x * v1y - v1x * v0y
    if abs(det) > 1e-12:
        v2x, v2y = px - ax, py - ay
        u = (v2x * v1y - v1x * v2y) / det
        v = (v0x * v2y - v2x * v0y) / det
        if u >= 0.0 and v >= 0.0 and u + v <= 1.0:
            return 0.0, az + u * (bz - az) + v * (cz - az)
    best = (math.inf, 0.0)
    for (sx, sy, sz), (ex, ey, ez) in (((ax, ay, az), (bx, by, bz)), ((bx, by, bz), (cx, cy, cz)), ((cx, cy, cz), (ax, ay, az))):
        dx, dy = ex - sx, ey - sy
        length2 = dx * dx + dy * dy
        t = 0.0 if length2 < 1e-18 else max(0.0, min(1.0, ((px - sx) * dx + (py - sy) * dy) / length2))
        d2 = (px - sx - t * dx) ** 2 + (py - sy - t * dy) ** 2
        if d2 < best[0]:
            best = (d2, sz + t * (ez - sz))
    return best


class BedField:
    """Spatial hash of floor triangles; see the module docstring."""

    def __init__(self, triangles):
        self.cells = {}
        self.count = 0
        for a, b, c, arc in triangles:
            if abs((b[0] - a[0]) * (c[1] - a[1]) - (c[0] - a[0]) * (b[1] - a[1])) < 1e-9:
                continue                                    # a vertical riser has no plan area
            cx, cy = (a[0] + b[0] + c[0]) / 3.0, (a[1] + b[1] + c[1]) / 3.0
            bbox = (min(a[0], b[0], c[0]), max(a[0], b[0], c[0]), min(a[1], b[1], c[1]), max(a[1], b[1], c[1]))
            self.cells.setdefault((math.floor(cx / CELL), math.floor(cy / CELL)), []).append((bbox, a, b, c, arc))
            self.count += 1

    def nearest(self, x, y, reach):
        """``[(distance, height, arc)]`` of the triangles within ``reach`` metres of ``(x, y)`` (unordered, may be empty)."""
        span = int(math.ceil((reach + TRIANGLE_SPAN) / CELL))
        cx, cy = math.floor(x / CELL), math.floor(y / CELL)
        limit = reach * reach
        hits = []
        for i in range(cx - span, cx + span + 1):
            for j in range(cy - span, cy + span + 1):
                for (x0, x1, y0, y1), a, b, c, arc in self.cells.get((i, j), ()):
                    dx = max(x0 - x, 0.0, x - x1)
                    dy = max(y0 - y, 0.0, y - y1)
                    if dx * dx + dy * dy > limit:
                        continue
                    d2, z = _closest(x, y, a, b, c)
                    if d2 <= limit:
                        hits.append((math.sqrt(d2), z, arc))
        return hits
