"""Tunnel stretches of a track and their portals (MIT License, see ../LICENSE).

Plain Python, no ``bpy``. A tunnel is a run of connected strips with ``cover: "tunnel"``. ``plan(track)`` returns
one record per run:

* ``chain``    a ``Chain`` over the sections of the run (``frame(s)`` is the frame at route length ``s``);
* ``portal``   the entrance portal: ``skullrock`` (pirate), ``head`` (temple) or ``collar`` (moon) with its depth;
* ``length``   route length of the run.

The inner opening is ``HALF`` (1.25 m) to each side of the centre line and ``CROWN`` (1.45 m) above the floor, wider
and higher than the required 1.9 x 1.0 m and clear of the rail trims (1.17 m). The roof and the portals have no
collision: no ball can reach them because ``layout`` rejects a tunnel over a loop, a half-pipe or a ramp with a lip,
and the lane has its rails. The first ``portal depth`` metres of the run must be straight and level (the portal is a
solid body around the lane) and the run must be at least that long; both are checked with a message that names
the piece. Another stretch of the track must not enter the volume of the roof either (checked against all floors).
"""

import math

from . import part_utils as pu
from .bed_field import BedField, floor_triangles

HALF, SPRING, CROWN = 1.25, 0.55, 1.45
ROOF = 0.4                   # roof thickness above the inner crown
FOOT = 0.62                  # the outer mound is this much wider than the opening at its foot
OPEN_MIN = (1.9, 1.0)        # required opening (width, height) over the floor
PORTAL = {"pirate": ("skullrock", 2.7), "temple": ("head", 2.9), "moon": ("collar", 0.34)}
PORTAL_RADIUS = {"pirate": 2.5, "temple": 3.5, "moon": 1.9}
STRAIGHT_TOL = (0.04, 0.04)  # lateral and vertical deviation allowed inside the portal depth
RESERVE_STEP, RESERVE_RADIUS = 1.2, 1.95
ROOF_TOP = CROWN + ROOF + 0.1   # height of the roof above the floor, for the crossing check
PORTAL_HEIGHT = {"pirate": 2.8, "temple": 3.6, "moon": 1.9}   # top of the portal body over the floor (the skull rock is 2.75 m high)
PORTAL_HALF = {"pirate": 2.1, "temple": 3.0, "moon": HALF + FOOT}     # half width of the portal body (the skull rock is 2.05 m)
COVER_STEP = 0.8             # length of one roof box of ``cover_boxes``


class Chain:
    """A run of sections with cumulative route length; ``frame(s)`` interpolates the frame at length ``s``."""

    def __init__(self, sections):
        self.sections = sections
        self.cum = [0.0]
        for a, b in zip(sections, sections[1:]):
            self.cum.append(self.cum[-1] + pu.length(pu.sub(b.center, a.center)))
        self.length = self.cum[-1]

    def frame(self, s):
        s = min(max(s, 0.0), self.length)
        for i in range(len(self.cum) - 1):
            if self.cum[i + 1] >= s and self.cum[i + 1] - self.cum[i] > 1e-9:
                return pu.frame_between(self.sections[i], self.sections[i + 1], (s - self.cum[i]) / (self.cum[i + 1] - self.cum[i]))
        return pu.section_frame(self.sections[-1])


def _runs(track):
    """Connected strips with ``cover == "tunnel"`` as lists of strips."""
    runs = []
    for strip in track.strips:
        if strip.cover != "tunnel":
            continue
        if runs and runs[-1][-1].sections[-1] is strip.sections[0]:
            runs[-1].append(strip)
        else:
            runs.append([strip])
    return runs


def _chain_of(strips):
    sections = list(strips[0].sections)
    for strip in strips[1:]:
        sections.extend(strip.sections[1:])
    keep = [sections[0]]
    for section in sections[1:]:                       # drop zero-length repeats (the two sections of a riser)
        if pu.length(pu.sub(section.center, keep[-1].center)) > 1e-6:
            keep.append(section)
    return Chain(keep)


def _check_straight(chain, depth, where):
    origin, forward, across, up = chain.frame(0.0)
    s = 0.0
    while s <= depth + 1e-9:
        point = chain.frame(s)[0]
        rel = pu.sub(point, origin)
        lateral = abs(pu.dot(rel, across))
        vertical = abs(rel[2])
        if lateral > STRAIGHT_TOL[0] or vertical > STRAIGHT_TOL[1]:
            raise ValueError("%sthe first %.1f m of the tunnel must be straight and level (the portal is a solid body around the lane); "
                             "it leaves the line by %.2f m sideways and %.2f m in height at %.1f m" % (where, depth, lateral, vertical, s))
        s += 0.25


def _roof_conflict(chain, index, strips_arc, where):
    """Raise when another stretch of the track enters the volume of the roof."""
    s = 0.0
    while s <= chain.length + 1e-9:
        origin, forward, across, up = chain.frame(s)
        for lateral in (-1.6, 0.0, 1.6):
            x, y = origin[0] + across[0] * lateral, origin[1] + across[1] * lateral
            for distance, z, arc in index.nearest(x, y, 0.2):
                if abs(arc - (strips_arc + s)) > 3.0 and origin[2] + 0.1 < z < origin[2] + ROOF_TOP:
                    raise ValueError("%sanother stretch of the track passes through the tunnel roof at %.1f m of the tunnel "
                                     "(%.2f m above its floor): keep it %.1f m higher or move it" % (where, s, z - origin[2], ROOF_TOP))
        s += 0.5


def plan(track):
    """The tunnel records of ``track`` (see the module docstring)."""
    runs = _runs(track)
    if not runs:
        return []
    index = BedField(floor_triangles(track, grounded_only=False))
    out = []
    for number, strips in enumerate(runs):
        chain = _chain_of(strips)
        kind, depth = PORTAL[track.world]
        where = "course %d: tunnel %d: " % (track.number, number + 1)
        if chain.length < depth + 0.3:
            raise ValueError("%sthe tunnel is %.1f m long, its %s portal needs at least %.1f m" % (where, chain.length, kind, depth + 0.3))
        _check_straight(chain, depth, where)
        _roof_conflict(chain, index, strips[0].sections[0].arc or 0.0, where)
        out.append({"chain": chain, "portal": kind, "depth": depth, "length": chain.length, "world": track.world})
    return out


def footprint_points(track):
    """Plan points that the island must contain: the entrance portal circle and the exit end of every tunnel."""
    points = []
    for tunnel in plan(track):
        chain, depth, world = tunnel["chain"], tunnel["depth"], tunnel["world"]
        origin, forward, _, _ = chain.frame(0.0)
        cx, cy = origin[0] + forward[0] * depth / 2.0, origin[1] + forward[1] * depth / 2.0
        r = PORTAL_RADIUS[world]
        points.extend((cx + r * math.cos(math.tau * k / 16), cy + r * math.sin(math.tau * k / 16)) for k in range(16))
    return points


def cover_boxes(tunnels):
    """Boxes (Blender metres) around the portal and the roof of every tunnel, for the cannon flight check.

    Each box is ``{name, origin, forward, across, length, half, foot, height}``: ``origin`` (x, y) and ``foot`` (z) at its
    start, the unit vectors ``forward`` and ``across`` in plan, ``length`` along the route, ``half`` width to each side and
    ``height`` over ``foot``. The roof and the portals have no collision for the ball, but a shot must not fly through the
    rock: the first box is the entrance body (``PORTAL_HALF`` wide, ``depth`` long, level and straight), the roof follows
    in ``COVER_STEP`` pieces (the opening plus the outer mound, ``ROOF_TOP`` high, taller where the lane climbs).
    """
    out = []
    for number, tunnel in enumerate(tunnels, 1):
        chain, depth, world = tunnel["chain"], tunnel["depth"], tunnel["world"]
        origin, forward, across, _ = chain.frame(0.0)
        out.append({"name": "the portal of tunnel %d" % number, "origin": origin[:2], "forward": forward[:2], "across": across[:2],
                    "length": depth, "half": PORTAL_HALF[world], "foot": origin[2], "height": PORTAL_HEIGHT[world]})
        s = depth
        while s < chain.length - 1e-9:
            end = min(s + COVER_STEP, chain.length)
            (o0, f0, a0, _), (o1, _, _, _) = chain.frame(s), chain.frame(end)
            out.append({"name": "the roof of tunnel %d" % number, "origin": o0[:2], "forward": f0[:2], "across": a0[:2], "length": end - s,
                        "half": HALF + FOOT, "foot": min(o0[2], o1[2]), "height": ROOF_TOP + abs(o1[2] - o0[2])})
            s = end
    return out


def reserved_circles(tunnels):
    """``(x, y, radius)`` circles along every tunnel that keep props off the roof and the portal."""
    out = []
    for tunnel in tunnels:
        chain = tunnel["chain"]
        s = 0.0
        while s <= chain.length + 1e-9:
            origin = chain.frame(s)[0]
            out.append((origin[0], origin[1], PORTAL_RADIUS[tunnel["world"]] if s < tunnel["depth"] else RESERVE_RADIUS))
            s += RESERVE_STEP
    return out
