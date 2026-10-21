"""Structures of a P3 course: supports, bridges, tunnels, ponds (MIT License, see ../LICENSE).

``build(track, terrain, look, number)`` makes the decoration that the track layout asks for and returns a
``Structures`` record for the ``Dresser``:

* ``baked``    objects that are baked with the island (supports, bridge parts, tunnel roofs and portals);
* ``water``    the ``pond-water`` surfaces (colours only, hidden from the AO bake);
* ``reserved`` ``(x, y, radius)`` circles that keep props away (tunnel stretches and portals).

``footprint_points(track)`` gives the plan points the island must contain for the portals (known before the
terrain exists, so the island outline can include them). The parts live in ``support_plan`` / ``support_parts``,
``bridge_parts``, ``tunnel_plan`` / ``tunnel_parts`` and ``pond_parts``; the contracts are in ``kit/AGENTS.md``.
"""

from . import bridge_parts, pond_parts, support_parts, support_plan, trench_parts, tunnel_parts, tunnel_plan


class Structures:
    """Result of ``build``."""

    def __init__(self):
        self.baked, self.water, self.reserved = [], [], []


def footprint_points(track):
    """Plan points of the tunnel portals (the island must contain them)."""
    return tunnel_plan.footprint_points(track)


def build(track, terrain, look, number, runs=()):
    """Build the structures of one course into the active collection (``shapes.set_collection``)."""
    out = Structures()
    stations = support_plan.plan(track, terrain.top_height, runs)
    for part in (support_parts.build(look, stations, number), bridge_parts.build(look, track, number)):
        if part is not None:
            out.baked.append(part)
    out.water = pond_parts.build(track, look)
    out.baked.extend(trench_parts.build(terrain))
    tunnels = tunnel_plan.plan(track)
    out.baked.extend(tunnel_parts.build(look, tunnels, number))
    out.reserved.extend(tunnel_plan.reserved_circles(tunnels))
    return out
