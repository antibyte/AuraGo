"""Manifest v2 entry of a course: bounds, water, routes (MIT License, see ../LICENSE).

Plain Python, no ``bpy``. All outputs are in runtime coordinates ``(x, z, -y)`` of Blender, rounded to
millimetres (``q`` rounds to 1/100 mm like the legacy entries). ``bounds`` is the island outline plus 1.5 m in x/z;
in y it runs from ``min(water level - 0.4, lowest floor - 4.0)`` (on the moon only the second) to the highest
floor or cannon arc plus 6 m. ``water`` is the sea slab (pirate, temple), the volume of every pond and, outside the
moon, of every gap; on the moon only ponds (void pools and gaps are holes through the island, falling out of
``bounds`` ends the shot).
"""

import copy

from .terrain import POND_DEPTH, POND_DROP, TRENCH_Z, WATER_Z
from .track import BALL_RADIUS, CUP_RADIUS

BOUNDS_MARGIN = 1.5
BOUNDS_BELOW, BOUNDS_ABOVE = 4.0, 6.0
SEA_BELOW = 0.4
GAP_ABOVE_BOTTOM = 0.25
POND_SURFACE_GAP = 0.02


def runtime(p):
    return [float(p[0]), float(p[2]), float(-p[1])]


def q(v, places=5):
    return [round(float(x), places) for x in v]


def millimetres(points):
    return [[round(v, 3) for v in runtime(p)] for p in points]


def floor_extent(track):
    """``(lowest, highest)`` floor height of the track."""
    zs = []
    for strip in track.strips:
        for section in strip.sections:
            zs.extend(p[2] for p in section.points)
    for pad in track.pads:
        grid = pad.grid
        rows = grid["rings"] if grid.get("polar") else grid["points"]
        zs.extend(p[2] for row in rows for p in row)
    return min(zs), max(zs)


def cannon_apex(track):
    """Highest point of any cannon flight (runtime y), or None."""
    apex = None
    for m in track.mechanics:
        if m.get("kind") == "cannon":
            vy, y = m["velocity"][1], m["muzzle"][1]
            top = y + (vy * vy / (2.0 * track.gravity) if vy > 0 else 0.0)
            apex = top if apex is None else max(apex, top)
    return apex


def bounds(track, box):
    """``{"min", "max"}`` for the island outline box ``(min_x, min_y, max_x, max_y)`` (Blender coordinates)."""
    low, high = floor_extent(track)
    bottom = low - BOUNDS_BELOW
    if track.world != "moon":
        bottom = min(WATER_Z - SEA_BELOW, bottom)
    top = high
    apex = cannon_apex(track)
    if apex is not None:
        top = max(top, apex)
    return {"min": q([box[0] - BOUNDS_MARGIN, bottom, -box[3] - BOUNDS_MARGIN]), "max": q([box[2] + BOUNDS_MARGIN, top + BOUNDS_ABOVE, -box[1] + BOUNDS_MARGIN])}


def volume(x0, y0, x1, y1, z_low, z_high):
    """A water volume from plan corners (Blender x/y) and heights; runtime ``min``/``max``."""
    return {"min": q([min(x0, x1), z_low, -max(y0, y1)]), "max": q([max(x0, x1), z_high, -min(y0, y1)])}


def water(track, limits):
    """Water volumes: sea slab, ponds and (outside the moon) gaps."""
    out = []
    if track.world != "moon":
        out.append({"min": q([limits["min"][0], WATER_Z - SEA_BELOW, limits["min"][2]]), "max": q([limits["max"][0], WATER_Z, limits["max"][2]])})
    for hazard in track.hazards:
        if hazard.get("kind") == "water" and "polygon" in hazard:
            surface = hazard["z"] - POND_DROP
            out.append(volume(*hazard["min"], *hazard["max"], surface - POND_DEPTH, surface - POND_SURFACE_GAP))
        elif "rect" in hazard and track.world != "moon":
            out.append(volume(*hazard["min"], *hazard["max"], TRENCH_Z[track.world], hazard["z"][0] - GAP_ABOVE_BOTTOM))
    return out


def entry(number, data, track, dresser, glb_name, glb_bytes, sha256, triangles, collision, box):
    """The manifest entry (contract V2) of a built course."""
    limits = bounds(track, box)
    tee = runtime(track.tee)
    return {
        "id": "%s-%02d" % (track.world, number), "number": number, "world": track.world, "par": data["par"],
        "model": glb_name, "sha256": sha256, "bytes": glb_bytes, "triangles": triangles,
        "tee": q([tee[0], tee[1] + BALL_RADIUS, tee[2]]), "cup": q(runtime(track.cup)), "cupRadius": CUP_RADIUS,
        "gravity": track.gravity, "cycle": track.cycle, "length": round(track.length, 3),
        "bounds": limits, "water": water(track, limits),
        "collision": {"nodes": collision["nodes"], "triangles": collision["triangles"], "digest": collision["digest"]},
        "obstacles": copy.deepcopy(track.obstacles), "mechanics": copy.deepcopy(track.mechanics),
        "path": millimetres(track.path), "corridors": [millimetres(line) for line in track.corridors],
        "props": dresser.runtime_props(), "island": dresser.runtime_island(),
    }
