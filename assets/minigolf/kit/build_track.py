"""Blender objects of a ``Track``: floor and collision rails (MIT License, see ../LICENSE).

``build(track, scene, coll)`` creates

* one ``floor-<look>`` node per (look, surface) pair (``floor-<look>-<surface>`` when a look is used with both
  surfaces) with the custom property ``aurago_collider`` = ``grass`` or ``sand``. Vertices are merged through
  a dict keyed by the exact coordinate tuple, so the shared seam objects of strips and pads become one vertex.
  The vertex colours (``palette`` / ``ao_w``, see ``floor_paint``) are written for the dresser's bake;
* one ``rail-<surface>`` node (``wood`` pirate, ``stone`` temple, ``metal`` moon) with ``aurago_collider`` and
  ``aurago_hidden``, made from the runs of ``rails.rail_runs``.

All collision objects keep the identity transform: their vertices are world coordinates, which is what the
runtime reader expects. The wall runs are returned as well (the rail trims are built from them).
"""

import math

from . import basic_parts, floor_paint, materials, rails
from .track import RAIL_SURFACES, ROWS


class _Mesh:
    """Vertices merged by exact tuple, triangles, and one ``(edge, arc)`` colour sample per vertex."""

    def __init__(self):
        self.verts, self.faces, self.metas, self.index = [], [], [], {}

    def vertex(self, point, meta):
        i = self.index.get(point)
        if i is None:
            i = self.index[point] = len(self.verts)
            self.verts.append(point)
            self.metas.append(meta)
        return i

    def quad(self, a, d, c, b):
        """Two triangles ``(a, d, c)`` and ``(a, c, b)``; the points are ``((x, y, z), meta)``."""
        ia, id_, ic, ib = (self.vertex(*p) for p in (a, d, c, b))
        self.faces.extend([(ia, id_, ic), (ia, ic, ib)])


def _strip_quads(strip, mesh_of):
    for s0, s1 in zip(strip.sections, strip.sections[1:]):
        mesh = mesh_of(s1.look, s1.surface)           # the later section belongs to this piece
        for j in range(ROWS):
            edge_a, edge_d = abs(ROWS / 2 - j) / (ROWS / 2), abs(ROWS / 2 - j - 1) / (ROWS / 2)
            mesh.quad((s0.points[j], (edge_a, s0.arc)), (s0.points[j + 1], (edge_d, s0.arc)),
                      (s1.points[j + 1], (edge_d, s1.arc)), (s1.points[j], (edge_a, s1.arc)))


def _pad_triangles(pad, mesh_of):
    """Floor of a polar pad (funnel): ``pad.faces`` are point triples; ``edge`` grows from the centre to the rim."""
    mesh = mesh_of(pad.look, pad.surface)
    centre, radius = pad.grid["centre"], pad.grid["radius"]

    def meta(point):
        r = math.hypot(point[0] - centre[0], point[1] - centre[1])
        return (min(1.0, r / radius), pad.arc0 + max(0.0, radius - r))
    for triangle in pad.faces:
        mesh.faces.append(tuple(mesh.vertex(point, meta(point)) for point in triangle))


def _pad_quads(pad, mesh_of):
    if pad.grid.get("polar"):
        return _pad_triangles(pad, mesh_of)
    mesh = mesh_of(pad.look, pad.surface)
    grid = pad.grid
    points, xs, ys = grid["points"], grid["xs"], grid["ys"]
    half = pad.size[1] / 2
    meta = lambda i, j: (abs(ys[j]) / half, pad.arc0 + xs[i])
    for i, j in pad.faces:
        mesh.quad((points[i][j], meta(i, j)), (points[i][j + 1], meta(i, j + 1)),
                  (points[i + 1][j + 1], meta(i + 1, j + 1)), (points[i + 1][j], meta(i + 1, j)))


def floor_objects(track, scene, coll):
    """The floor nodes of ``track`` (see the module docstring)."""
    meshes = {}

    def mesh_of(look, surface):
        return meshes.setdefault((look, surface), _Mesh())

    for strip in track.strips:
        _strip_quads(strip, mesh_of)
    for pad in track.pads:
        _pad_quads(pad, mesh_of)
    surfaces_of = {}
    for look, surface in meshes:
        surfaces_of.setdefault(look, []).append(surface)
    matte = materials.get("matte")
    objects = []
    for (look, surface), mesh in meshes.items():
        name = "floor-" + look if len(surfaces_of[look]) == 1 else "floor-%s-%s" % (look, surface)
        obj = basic_parts.mesh_object(scene, coll, name, mesh.verts, mesh.faces, matte, "play-surface")
        obj["aurago_collider"] = surface
        floor_paint.paint(obj, track.world, look, mesh.metas)
        objects.append(obj)
    return objects


def rail_objects(track, runs, scene, coll):
    """The collision rail node of ``track`` (one per rail surface, only ``RAIL_SURFACES[world]`` for now)."""
    verts, faces = [], []
    for run in runs:
        v, f = rails.rail_mesh(run)
        faces.extend(tuple(k + len(verts) for k in face) for face in f)
        verts.extend(v)
    if not faces:
        return []
    surface = RAIL_SURFACES[track.world]
    obj = basic_parts.mesh_object(scene, coll, "rail-" + surface, verts, faces, materials.get("matte"), "boundary")
    obj["aurago_collider"] = surface
    obj["aurago_hidden"] = True
    return [obj]


def build(track, scene, coll):
    """Floor and rail objects of ``track`` in ``coll``: ``{"floors", "rails", "runs"}``."""
    runs = rails.rail_runs(track)
    return {"floors": floor_objects(track, scene, coll), "rails": rail_objects(track, runs, scene, coll), "runs": runs}
