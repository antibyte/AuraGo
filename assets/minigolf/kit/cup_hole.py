"""Exact cup hole in a raster floor (MIT License, see ../LICENSE).

Plain Python, no ``bpy``. The hole of a green is a polygon of ``CUP_SEGMENTS`` corners on the circle of radius
``CUP_RADIUS``. The raster cells that touch the circle of radius ``CUP_RADIUS`` + one cell are removed; the jagged
boundary of that region (raster nodes, counter-clockwise around the hole) is joined to the polygon by a zipper of
triangles over the angles of both rings. The funnel reuses ``zipper``. The collision cup (``cup_parts``) uses the
very same polygon point objects as its rim, so the runtime welds floor and cup.
"""

import math

from .track import CUP_RADIUS, CUP_SEGMENTS


def _area2(a, b, c):
    """Twice the plan-view area of the triangle ``abc`` (positive when counter-clockwise seen from above)."""
    return (b[0] - a[0]) * (c[1] - a[1]) - (c[0] - a[0]) * (b[1] - a[1])


def zipper(outer, inner, triangles):
    """Triangles between two closed star-shaped rings, both counter-clockwise ``[(angle, point)]`` starting at the same angle.

    Every step takes the next outer edge (triangle ``outer[i], outer[i+1], inner[j]``) or the next inner edge
    (``inner[j], outer[i], inner[j+1]``) and only counter-clockwise triangles (facing up) are allowed. Among all such
    sequences a dynamic program picks the one with the smallest sum of squared new diagonals, which avoids slivers
    and cannot run into a dead end (a plain angular merge sometimes can).
    """
    na, nb = len(outer), len(inner)
    pts_o = [p for _, p in outer]
    pts_i = [p for _, p in inner]
    inf = float("inf")
    cost = [[inf] * (nb + 1) for _ in range(na + 1)]
    came = [[None] * (nb + 1) for _ in range(na + 1)]
    cost[0][0] = 0.0
    for i in range(na + 1):
        for j in range(nb + 1):
            here = cost[i][j]
            if here == inf:
                continue
            o0, i0 = pts_o[i % na], pts_i[j % nb]
            if i < na:
                o1 = pts_o[(i + 1) % na]
                if _area2(o0, o1, i0) > -1e-12:
                    step = here + sum((a - b) ** 2 for a, b in zip(o1, i0))
                    if step < cost[i + 1][j]:
                        cost[i + 1][j], came[i + 1][j] = step, 0
            if j < nb:
                i1 = pts_i[(j + 1) % nb]
                if _area2(i0, o0, i1) > -1e-12:
                    step = here + sum((a - b) ** 2 for a, b in zip(o0, i1))
                    if step < cost[i][j + 1]:
                        cost[i][j + 1], came[i][j + 1] = step, 1
    if cost[na][nb] == inf:
        raise ValueError("the ring around the cup cannot be triangulated: move the cup by a few centimetres")
    steps, i, j = [], na, nb
    while i or j:
        kind = came[i][j]
        if kind == 0:
            i -= 1
            steps.append((pts_o[i % na], pts_o[(i + 1) % na], pts_i[j % nb]))
        else:
            j -= 1
            steps.append((pts_i[j % nb], pts_o[i % na], pts_i[(j + 1) % nb]))
    triangles.extend(reversed(steps))


def boundary_loop(removed):
    """Raster nodes ``(i, j)`` around the removed cells, counter-clockwise (the removed cells on the left).

    ``removed`` is a set of cells ``(i, j)`` (``i`` forward, ``j`` from the left). The region must be one
    simply connected blob; a pinch point or a second blob raises ``ValueError``.
    """
    nxt = {}

    def edge(a, b):
        if a in nxt:
            raise ValueError("the hole boundary touches itself")
        nxt[a] = b

    for i, j in sorted(removed):
        if (i + 1, j) not in removed:
            edge((i + 1, j + 1), (i + 1, j))
        if (i - 1, j) not in removed:
            edge((i, j), (i, j + 1))
        if (i, j - 1) not in removed:
            edge((i + 1, j), (i, j))
        if (i, j + 1) not in removed:
            edge((i, j + 1), (i + 1, j + 1))
    start = min(nxt)
    loop, node = [], start
    while True:
        loop.append(node)
        node = nxt[node]
        if node == start:
            break
        if len(loop) > len(nxt):
            raise ValueError("the hole boundary is not a single loop")
    if len(loop) != len(nxt):
        raise ValueError("the hole boundary is not a single loop")
    return loop


def ordered_by_angle(angles):
    """Rotate ``angles`` (one per boundary node, counter-clockwise) to start at the smallest and unwrap them.

    Returns ``(shift, unwrapped)``; raises ``ValueError`` unless the angles increase strictly over exactly one
    turn, which is what the zipper needs (the boundary must be star-shaped around the hole centre).
    """
    n = len(angles)
    shift = min(range(n), key=lambda k: angles[k])
    seq = [angles[(shift + k) % n] for k in range(n)]
    if any(b <= a + 1e-12 for a, b in zip(seq, seq[1:])) or seq[-1] - seq[0] >= 2.0 * math.pi:
        raise ValueError("the hole boundary is not star-shaped around the cup: move the cup by a few centimetres")
    return shift, seq


def polygon(cx, cy, start, point_at):
    """``CUP_SEGMENTS`` corners on the circle of radius ``CUP_RADIUS`` around ``(cx, cy)`` as ``[(angle, point)]``.

    ``point_at(x, y)`` makes the (world) point of a pad-frame position; the first corner is at angle ``start``.
    """
    out = []
    for k in range(CUP_SEGMENTS):
        angle = start + 2.0 * math.pi * k / CUP_SEGMENTS
        out.append((angle, point_at(cx + CUP_RADIUS * math.cos(angle), cy + CUP_RADIUS * math.sin(angle))))
    return out


def fill(removed, node_point, node_xy, cx, cy, point_at):
    """Zipper floor between the boundary of ``removed`` and the hole polygon.

    ``node_point(i, j)`` is the raster point object, ``node_xy(i, j)`` its pad-frame ``(x, y)``. Returns
    ``(triangles, ring)`` where ``ring`` is the polygon as a list of point objects (counter-clockwise).
    """
    loop = boundary_loop(removed)
    angles = [math.atan2(node_xy(*n)[1] - cy, node_xy(*n)[0] - cx) for n in loop]
    shift, seq = ordered_by_angle(angles)
    outer = [(seq[k], node_point(*loop[(shift + k) % len(loop)])) for k in range(len(loop))]
    inner = polygon(cx, cy, seq[0], point_at)
    triangles = []
    zipper(outer, inner, triangles)
    return triangles, [p for _, p in inner]
