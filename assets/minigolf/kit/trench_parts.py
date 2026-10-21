"""Strata walls of trenches and chasms (MIT License, see ../LICENSE).

A ``gap`` hazard cuts a trench across the whole island (``terrain.Terrain``): down below the water line on the pirate
bay and in the temple, through the island on the moon, where the chasm shows its strata and no water. The terrain
snaps the top grid vertices next to the trench onto its edge lines (``Terrain.warp``), so the island's own wall is a
straight, almost vertical strip with only two rows of vertices. ``build(terrain)`` lays one ``trench-wall`` object per
edge in front of it (1.2 cm towards the void): rows at the stratum boundaries of the island (``terrain.layers``), each
stratum with the gradient of the island wall, so the cut reads as the same rock. The wall follows the real heights
(``terrain.top_height`` on both sides of the edge) and stops where the trench gives way to another stretch of the
track (``Terrain`` dam). The objects are baked with the island.
"""

import math

from . import colors, materials, shapes
from .dress_island import strata_color
from .terrain import TRENCH_SOFT

STEP = 0.3                  # distance between two wall columns along the edge
OFFSET = 0.012              # the wall stands this far in front of the island's own wall
MIN_DROP = 0.3              # no wall where the trench is shallower than this (a dam)


def _columns(terrain, strip, sign):
    """Runs of ``(point_in, point_out, z_bottom, z_top)`` along the edge ``sign`` of ``strip`` inside the island."""
    mx, my, fx, fy, half = strip
    lx, ly = -fy, fx
    reach = 1.4 * max(terrain.outline.half)
    runs, run = [], []
    n = int(2 * reach / STEP)
    for k in range(n + 1):
        t = -reach + k * STEP
        bx, by = mx + lx * t, my + ly * t
        p_in = (bx + fx * sign * half, by + fy * sign * half)
        p_out = (bx + fx * sign * (half + TRENCH_SOFT), by + fy * sign * (half + TRENCH_SOFT))
        if terrain.outline.contains_signed(*p_out) > -0.1 or terrain.outline.contains_signed(*p_in) > -0.1:
            ok = False
        else:
            zb, zt = terrain.top_height(*p_in), terrain.top_height(*p_out)
            ok = zt - zb > MIN_DROP
        if ok:
            run.append((p_in, p_out, zb, zt))
        elif run:
            runs.append(run)
            run = []
    if run:
        runs.append(run)
    return [r for r in runs if len(r) >= 2]


def _levels(terrain, zb, zt):
    """Heights of the wall rows from top to bottom: ``zt``, every stratum boundary clamped into ``[zb, zt]``, ``zb``."""
    return [zt] + [min(zt, max(zb, layer.z)) for layer in terrain.layers] + [zb]


def _wall(terrain, strip, sign, runs, name):
    mx, my, fx, fy, half = strip
    toward = (-fx * sign, -fy * sign)                       # from the island into the void
    verts, quads = [], []
    for run in runs:
        columns = []
        for p_in, p_out, zb, zt in run:
            rows = []
            for z in _levels(terrain, zb, zt):
                u = (z - zb) / (zt - zb)
                rows.append(len(verts))
                verts.append((p_in[0] + (p_out[0] - p_in[0]) * u + toward[0] * OFFSET,
                              p_in[1] + (p_out[1] - p_in[1]) * u + toward[1] * OFFSET, z))
            columns.append(rows)
        for a, b in zip(columns, columns[1:]):
            for j in range(len(a) - 1):
                if abs(verts[a[j]][2] - verts[a[j + 1]][2]) > 1e-6 or abs(verts[b[j]][2] - verts[b[j + 1]][2]) > 1e-6:
                    quads.append((a[j], b[j], b[j + 1], a[j + 1]))
    if not quads:
        return None
    # the faces point into the void: test the first one and flip them all if needed
    q = quads[0]
    u = tuple(verts[q[1]][k] - verts[q[0]][k] for k in range(3))
    v = tuple(verts[q[3]][k] - verts[q[0]][k] for k in range(3))
    normal = (u[1] * v[2] - u[2] * v[1], u[2] * v[0] - u[0] * v[2])
    if normal[0] * toward[0] + normal[1] * toward[1] < 0:
        quads = [f[::-1] for f in quads]
    obj = shapes.make_object(name, verts, quads, smooth=False)
    colors.paint_values(obj, [strata_color(terrain, z) for _, _, z in verts], ao=0.3)
    obj.data.materials.append(materials.get("matte"))
    return obj


def build(terrain):
    """The ``trench-wall`` objects of all trench edges (empty without ``gap`` hazards)."""
    out = []
    for strip in terrain.gaps:
        for sign in (1, -1):
            obj = _wall(terrain, strip, sign, _columns(terrain, strip, sign), "trench-wall")
            if obj is not None:
                out.append(obj)
    return out
