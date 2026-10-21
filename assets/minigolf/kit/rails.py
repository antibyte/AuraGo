"""Rail runs of a track and their collision mesh data (MIT License, see ../LICENSE).

Plain Python, no ``bpy``. ``rail_runs(track)`` collects the wall segments of all strips (a segment is walled
on a side when its *later* section has the flag; zero-length riser edges of steps are skipped) and of all
pad perimeters, and chains them into connected runs by their exact end points (seam points are shared
objects, so chaining needs no tolerance). A run is a list of ``(point, outward, up)``: ``point`` is the
inner floor edge, ``up`` the wall direction (the section's ``wall_up`` or its normal), ``outward`` points
away from the lane, mitred at bends so the wall keeps its thickness. ``rail_mesh(run)`` turns a run into
vertices and triangles shaped like the P1 rails: the inner face reaches ``WALL_DEPTH`` below the edge and
``WALL_HEIGHT`` above it, the outer face is ``WALL_THICKNESS`` further out, both ends are closed.
The triangles face outwards (normal of the inner face points into the lane).
"""

import math

from .track import ROWS, WALL_DEPTH, WALL_HEIGHT, WALL_THICKNESS


def _sub(a, b):
    return (a[0] - b[0], a[1] - b[1], a[2] - b[2])


def _add(a, b):
    return (a[0] + b[0], a[1] + b[1], a[2] + b[2])


def _mul(a, k):
    return (a[0] * k, a[1] * k, a[2] * k)


def _dot(a, b):
    return a[0] * b[0] + a[1] * b[1] + a[2] * b[2]


def _cross(a, b):
    return (a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0])


def _unit(a):
    length = math.sqrt(_dot(a, a))
    return (a[0] / length, a[1] / length, a[2] / length) if length > 1e-12 else (0.0, 0.0, 1.0)


def _edge(section, side):
    """Floor edge point, outward direction and wall direction of ``section`` on ``side`` (0 left, 1 right)."""
    up = _unit(section.wall_up[side] if section.wall_up else section.normal)
    lateral = _unit(_sub(section.points[0], section.points[ROWS]))
    out = lateral if side == 0 else _mul(lateral, -1.0)
    return section.points[ROWS if side else 0], _unit(_sub(out, _mul(up, _dot(out, up)))), up


def strip_segments(strip):
    """Wall segments ``(point_a, point_b, outward, up_a, up_b)`` of one strip."""
    segments = []
    for a, b in zip(strip.sections, strip.sections[1:]):
        for side in (0, 1):
            if not b.walls[side]:
                continue
            pa, oa, ua = _edge(a, side)
            pb, ob, ub = _edge(b, side)
            if math.hypot(pa[0] - pb[0], pa[1] - pb[1]) < 1e-9:
                continue
            segments.append((pa, pb, _unit(_add(oa, ob)), ua, ub))
    return segments


def pad_segments(pad):
    up = (0.0, 0.0, 1.0)
    return [(pa, pb, out, up, up) for pa, pb, out in pad.walls]


def _miter(o1, o2):
    d = 1.0 + _dot(o1, o2)
    if d < 0.3:
        return _unit(_add(o1, o2)) if d > 1e-9 else o1
    return _mul(_add(o1, o2), 1.0 / d)


def chain_runs(segments):
    """Chain segments into runs; returns a list of vertex lists ``[(point, outward, up), ...]``."""
    ends = {}
    for k, seg in enumerate(segments):
        ends.setdefault(seg[0], []).append(k)
        ends.setdefault(seg[1], []).append(k)
    used = [False] * len(segments)

    def free(point):
        return next((k for k in ends.get(point, ()) if not used[k]), None)

    def turned(seg):
        return (seg[1], seg[0], seg[2], seg[4], seg[3])

    runs = []
    for first in range(len(segments)):
        if used[first]:
            continue
        used[first] = True
        chain = [segments[first]]
        while (k := free(chain[-1][1])) is not None:
            used[k] = True
            chain.append(segments[k] if segments[k][0] == chain[-1][1] else turned(segments[k]))
        while (k := free(chain[0][0])) is not None:
            used[k] = True
            chain.insert(0, segments[k] if segments[k][1] == chain[0][0] else turned(segments[k]))
        verts = [(chain[0][0], chain[0][2], chain[0][3])]
        for before, after in zip(chain, chain[1:]):
            verts.append((after[0], _miter(before[2], after[2]), _unit(_add(before[4], after[3]))))
        verts.append((chain[-1][1], chain[-1][2], chain[-1][4]))
        runs.append(verts)
    return runs


def rail_runs(track):
    """All wall runs of ``track`` (strips and pads)."""
    segments = []
    for strip in track.strips:
        segments.extend(strip_segments(strip))
    for pad in track.pads:
        segments.extend(pad_segments(pad))
    return chain_runs(segments)


def rail_mesh(run):
    """Vertices and triangles of one run (see the module docstring); the run is turned if needed so the faces point outwards."""
    point, out, up = run[0]
    if _dot(_cross(_sub(run[1][0], point), up), out) > 0:
        run = run[::-1]
    verts = []
    for point, out, up in run:
        inner_bottom = _sub(point, _mul(up, WALL_DEPTH))
        inner_top = _add(point, _mul(up, WALL_HEIGHT))
        verts.extend([inner_bottom, _add(inner_bottom, _mul(out, WALL_THICKNESS)), inner_top, _add(inner_top, _mul(out, WALL_THICKNESS))])
    faces = []
    for i in range(len(run) - 1):
        a, b = i * 4, (i + 1) * 4
        faces.extend([(a, b, b + 2), (a, b + 2, a + 2), (a + 1, a + 3, b + 3), (a + 1, b + 3, b + 1),
                      (a + 2, b + 2, b + 3), (a + 2, b + 3, a + 3), (a, a + 1, b + 1), (a, b + 1, b)])
    end = 4 * (len(run) - 1)
    faces.extend([(0, 2, 3), (0, 3, 1), (end, end + 1, end + 3), (end, end + 3, end + 2)])
    return verts, faces
