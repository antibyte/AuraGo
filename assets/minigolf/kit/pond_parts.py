"""Water surfaces of ponds and pools (MIT License, see ../LICENSE).

Every pool of a plaza with ``kind: "water"`` becomes an object ``pond-water`` (kit material kind ``water``, joined
into ``scenery-water`` by ``export.join_scenery``) lying ``terrain.POND_DROP`` (0.25 m) under the surrounding floor.
The surface is a grid of 0.4 m quads over the pool polygon enlarged by ``GROW``: the pool bank (``terrain``) crosses
the water level at about the polygon edge, so the enlarged border disappears under the bank. Vertex colours run from
a light shore tone to the deep tone over ``DEPTH_SPAN`` metres from the edge. The hazard volume in the manifest
(``course_entry.water``) runs from the pool bottom to ``course_entry.POND_SURFACE_GAP`` (2 cm) under this surface.

Chasms (``void`` pools and gaps on the moon) get no water: the island is cut down to ``terrain.TRENCH_Z`` and the
walls show the strata of the island.
"""

import math

from . import colors, materials, palette, shapes
from .palette import mix
from .terrain import POND_DROP, polygon_distance

CELL = 0.4
GROW = 0.45
DEPTH_SPAN = 0.9
TONES = {"pirate": ("lagoon_shore", "lagoon_mid", "lagoon_deep"), "temple": ("water_light", "water", "water_deep")}


def _inside(polygon, x, y):
    """Even-odd point in polygon."""
    inside = False
    for k in range(len(polygon)):
        (x0, y0), (x1, y1) = polygon[k], polygon[(k + 1) % len(polygon)]
        if (y0 > y) != (y1 > y) and x < x0 + (y - y0) * (x1 - x0) / (y1 - y0):
            inside = not inside
    return inside


def surface(hazard, look):
    """Vertices, faces and vertex colours of the water surface of one pool hazard (a grid of quads)."""
    polygon = [tuple(p) for p in hazard["polygon"]]
    z = hazard["z"] - POND_DROP
    pal = palette.get(look)
    shore, mid, deep = (pal.c(t) for t in TONES[look])
    x0, y0 = hazard["min"][0] - GROW, hazard["min"][1] - GROW
    nx = int(math.ceil((hazard["max"][0] + GROW - x0) / CELL))
    ny = int(math.ceil((hazard["max"][1] + GROW - y0) / CELL))
    index, verts, cols, faces = {}, [], [], []

    def vertex(i, j):
        key = (i, j)
        if key not in index:
            x, y = x0 + i * CELL, y0 + j * CELL
            depth = min(1.0, max(0.0, -polygon_distance(polygon, x, y) / DEPTH_SPAN))
            index[key] = len(verts)
            verts.append((x, y, z))
            cols.append(mix(shore, mix(mid, deep, max(0.0, depth - 0.5) * 2.0), depth ** 0.7))
        return index[key]

    for i in range(nx):
        for j in range(ny):
            cx, cy = x0 + (i + 0.5) * CELL, y0 + (j + 0.5) * CELL
            if polygon_distance(polygon, cx, cy) <= GROW:
                faces.append((vertex(i, j), vertex(i + 1, j), vertex(i + 1, j + 1), vertex(i, j + 1)))
    return verts, faces, cols


def build(track, look):
    """The ``pond-water`` objects of the water pools of ``track`` (empty on the moon or without ponds)."""
    out = []
    if look == "moon":
        return out
    for hazard in track.hazards:
        if "polygon" not in hazard or hazard["kind"] != "water":
            continue
        verts, faces, cols = surface(hazard, look)
        if not faces:
            continue
        obj = shapes.make_object("pond-water", verts, faces, smooth=False)
        colors.paint_values(obj, cols, ao=0.0)
        obj.data.materials.append(materials.get("water"))
        out.append(obj)
    return out
