"""Plan of the supports under elevated floors (MIT License, see ../LICENSE).

Plain Python, no ``bpy``. ``plan(track, ground)`` lists the support *stations* of a track; the parts of a station
are built by ``support_parts``. A station is a cross-section of a strip about every ``SPACING`` (2 m) of route where
the floor edge stands more than ``MIN_DROP`` above the ground (``ground(x, y)`` is the island top height): two posts
under the lane edges (just inside the rail, so a post is hidden below it) and a cross beam between them. Bridges,
ramp ends and the lanes over ponds and trenches all get their stations this way; a lane that rests on the ground
gets none.

A loop is carried by a frame instead: the ring is the lane and the ball runs on its inside, so no post may stand
under it. At about every 2 m of its arc, wherever the ring is higher than ``FRAME_MIN``, a post stands beside each
rail (``FRAME_SIDES`` metres from the centre line, the first free candidate) with an arm to the rail's outer face.

A post is dropped where it would pierce another floor (any floor triangle within ``CLEAR`` metres in plan whose height
lies between the post's bottom and ``CROSS_GAP`` under its top), so a support never blocks a lane that passes
underneath. Nothing here touches the ball clearance: posts stand below the floor or outside the rails.
"""

import math

from . import part_utils as pu
from .bed_field import BedField, floor_triangles
from .track import ROWS

SPACING = 2.0
INSET = 0.3                 # first and last station of a strip, from its ends
MIN_DROP = 0.14             # no post where the floor is less than this above the ground
SINK = 0.12                 # posts end this far below the ground top
EDGE_IN = 0.07              # post centre this far outside the floor edge (under the rail)
BEAM_BELOW = 0.05           # top of the cross beam under the floor edge
MERGE = 1.0                 # a station closer than this to an earlier one is dropped
CLEAR = 0.25                # plan distance in which another floor blocks a post
CROSS_GAP = 0.12
RAIL_CELL, RAIL_REACH, RAIL_GAP = 0.5, 0.2, 0.15   # a rail sample (centre of rail and trim) blocks a post within RAIL_REACH
OWN_ARC = 1.2               # floors this close along the route belong to the station's own stretch and never block it
FRAME_MIN = 0.7             # loop sections higher than this above the start get frame posts
FRAME_SIDES = (1.30, 1.55)  # post centre from the loop's centre line, first free candidate wins
FRAME_GAP = 2.0


def _rail_samples(runs):
    """Hash of ``(x, y, z_bottom, z_top)`` samples of the rail runs (rail and trim centre, every 0.12 m)."""
    cells = {}
    for run in runs:
        for (pa, oa, ua, ha), (pb, ob, ub, hb) in zip(run, run[1:]):
            n = max(1, int(math.ceil(pu.length(pu.sub(pb, pa)) / 0.12)))
            for k in range(n + 1):
                t = k / n
                point, out, up = pu.lerp(pa, pb, t), pu.unit(pu.lerp(oa, ob, t)), pu.unit(pu.lerp(ua, ub, t))
                x, y = point[0] + out[0] * 0.2, point[1] + out[1] * 0.2
                height = ha + (hb - ha) * t
                cells.setdefault((math.floor(x / RAIL_CELL), math.floor(y / RAIL_CELL)), []).append((x, y, point[2] - 0.045, point[2] + height))
    return cells


def _interp_section(s0, s1, u):
    """Left edge, right edge and centre of the cross-section between two sections (``u`` 0..1)."""
    return (pu.lerp(s0.points[0], s1.points[0], u), pu.lerp(s0.points[ROWS], s1.points[ROWS], u), pu.lerp(s0.center, s1.center, u))


def _arc_positions(strip):
    """Cumulative 3D length of the sections of ``strip``."""
    out = [0.0]
    for a, b in zip(strip.sections, strip.sections[1:]):
        out.append(out[-1] + pu.length(pu.sub(b.center, a.center)))
    return out


def _station_arcs(total):
    if total < 0.8:
        return [total / 2.0]
    n = max(2, math.ceil((total - 2.0 * INSET) / SPACING) + 1)
    return [INSET + (total - 2.0 * INSET) * k / (n - 1) for k in range(n)]


def _section_at(strip, cum, arc):
    """``(s0, s1, u)``: the two sections around the route length ``arc`` of the strip and the blend between them."""
    for i in range(len(cum) - 1):
        if cum[i + 1] >= arc and cum[i + 1] - cum[i] > 1e-9:
            return strip.sections[i], strip.sections[i + 1], (arc - cum[i]) / (cum[i + 1] - cum[i])
    return strip.sections[-2], strip.sections[-1], 1.0


class Planner:
    """Collects stations of one track; ``blocked`` asks the all-floor index."""

    def __init__(self, track, ground, runs=()):
        self.track, self.ground = track, ground
        self.index = BedField(floor_triangles(track, grounded_only=False))
        self.rails = _rail_samples(runs)
        self.base = track.tee[2]
        self.stations = []

    def blocked(self, x, y, bottom, top, arc):
        """True when another stretch or a rail passes through the column ``(x, y)`` between ``bottom`` and ``top`` (see the module docstring)."""
        for distance, z, other in self.index.nearest(x, y, CLEAR):
            if abs(other - arc) > OWN_ARC and bottom + 0.05 < z < top - CROSS_GAP:
                return True
        for rx, ry, z0, z1 in self.rails.get((math.floor(x / RAIL_CELL), math.floor(y / RAIL_CELL)), []) + self._rail_neighbours(x, y):
            if math.hypot(rx - x, ry - y) < RAIL_REACH and z1 > bottom + 0.05 and z0 < top - RAIL_GAP:
                return True
        return False

    def _rail_neighbours(self, x, y):
        cx, cy = math.floor(x / RAIL_CELL), math.floor(y / RAIL_CELL)
        out = []
        for i in (-1, 0, 1):
            for j in (-1, 0, 1):
                if i or j:
                    out.extend(self.rails.get((cx + i, cy + j), ()))
        return out

    def post(self, x, y, top, arc):
        """A post record ``{x, y, bottom, top}`` or None (too short or blocked); ``arc`` is the route length of its station."""
        ground = self.ground(x, y)
        bottom = ground - SINK
        if top - ground < MIN_DROP or self.blocked(x, y, bottom, top, arc):
            return None
        return {"x": x, "y": y, "bottom": bottom, "top": top}

    def near_existing(self, centre):
        return any(pu.length(pu.sub(centre, s["center"])) < MERGE for s in self.stations)

    def beam_station(self, strip, cum, arc):
        s0, s1, u = _section_at(strip, cum, arc)
        left, right, centre = _interp_section(s0, s1, u)
        if self.near_existing(centre):
            return
        across = pu.unit(pu.sub(left, right))
        flat = pu.unit((across[0], across[1], 0.0))
        route = s0.arc + (s1.arc - s0.arc) * u
        posts = []
        for point, sign in ((left, 1.0), (right, -1.0)):
            x, y = point[0] + flat[0] * sign * EDGE_IN, point[1] + flat[1] * sign * EDGE_IN
            posts.append(self.post(x, y, point[2] - BEAM_BELOW, route))
        if not any(posts):
            return
        origin, forward, _, up = pu.frame_between(s0, s1, u)
        self.stations.append({"kind": "beam", "center": centre, "forward": forward, "across": across, "up": up, "left": posts[0],
                              "right": posts[1], "edge_left": left, "edge_right": right, "strip": strip.kind})

    def frame_station(self, strip, cum, arc):
        s0, s1, u = _section_at(strip, cum, arc)
        left, right, centre = _interp_section(s0, s1, u)
        if centre[2] - self.base < FRAME_MIN or self.near_existing(centre):
            return
        origin, forward, across, up = pu.frame_between(s0, s1, u)
        flat = pu.unit((across[0], across[1], 0.0))
        route = s0.arc + (s1.arc - s0.arc) * u
        posts = []
        for sign in (1.0, -1.0):
            chosen = None
            for lateral in FRAME_SIDES:
                x, y = centre[0] + flat[0] * sign * lateral, centre[1] + flat[1] * sign * lateral
                chosen = self.post(x, y, centre[2], route)
                if chosen:
                    chosen["lateral"] = lateral
                    break
            posts.append(chosen)
        if not any(posts):
            return
        self.stations.append({"kind": "frame", "center": centre, "forward": forward, "across": across, "up": up, "left": posts[0],
                              "right": posts[1], "edge_left": left, "edge_right": right, "strip": strip.kind})


def plan(track, ground, runs=()):
    """The support stations of ``track`` (a list of dicts, see ``Planner``); ``ground(x, y)`` is the island top height,
    ``runs`` the rail runs of ``rails.rail_runs``."""
    planner = Planner(track, ground, runs)
    for strip in track.strips:
        cum = _arc_positions(strip)
        total = cum[-1]
        if total < 1e-6:
            continue
        if strip.kind == "loop":
            arcs = [total * k / max(1, round(total / FRAME_GAP)) for k in range(1, max(1, round(total / FRAME_GAP)))]
            for arc in arcs:
                planner.frame_station(strip, cum, arc)
        elif strip.kind != "halfpipe":
            for arc in _station_arcs(total):
                planner.beam_station(strip, cum, arc)
    return planner.stations
