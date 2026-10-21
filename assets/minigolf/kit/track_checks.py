"""Probe courses and pure checks of the track kit (MIT License, see ../LICENSE).

No ``bpy``: everything here runs in plain Python. ``probe_course`` (pirate: tee, straight, curve, bank, ramp,
hill, steps, straight, a jump = ramp with a lip + water gap + flat landing, green) and ``probe_course_moon``
(half-pipe, loop behind a boost pad, a fork with a straight and a hill branch, funnel with the hole in its centre)
are the data of ``track_selftest``. The checks:

* ``check_seams``: strips start with the object that ended their predecessor and every distinct point value is
  ONE tuple object, so the Blender floor needs no vertex welding;
* ``check_geometry``: floor orientation, half-pipe, loop, funnel, fork, jump;
* ``check_rules``: parameter errors of the new pieces, a curved half-pipe, a loop to the right, the fork
  options ``main`` and ``exit_offset`` and the error text of a bad branch;
* ``signature``: a hash of all points and route data, equal for equal builds.
"""

import hashlib
import math
import struct

from . import layout
from .track import CUP_RADIUS, HOLE_MARGIN, ROWS, SPACING


def probe_course():
    """Pirate probe course data (the design's data shape)."""
    return {"number": 1, "par": 4, "name": "Selftest", "start": {"at": (0.0, 0.0, 0.0), "heading": 0},
            "pieces": [("tee", {"length": 1.0}),
                       ("straight", {"length": 3.0}),
                       ("curve", {"radius": 3.0, "angle": 90}),
                       ("bank", {"radius": 4.0, "angle": -60, "bank": 20}),
                       ("ramp", {"length": 3.0, "rise": 0.4}),
                       ("hill", {"length": 3.0, "height": -0.25}),
                       ("steps", {"count": 3, "drop": 0.2, "tread": 0.8}),
                       ("straight", {"length": 2.0}),
                       ("ramp", {"length": 2.0, "rise": 0.35, "lip": 20}),
                       ("gap", {"length": 1.6, "hazard": "water"}),
                       ("straight", {"length": 1.0}),
                       ("green", {"length": 3.0, "width": 3.44, "cup": (1.8, 0.4)})],
            "hero": None, "props": [], "mechanics": [], "obstacles": [], "cycle": None}


PIPE_RADIUS = 0.9          # also the lip height: the quarter circles end vertically
FUNNEL_RADIUS, FUNNEL_DEPTH = 2.4, 0.64   # on the moon (g = 4) a ball rests on slopes below 11 degrees, so the bowl is steep: rim slope 21.8 degrees
BOOST = {"id": "boost-1", "kind": "boost", "position": [9.0, 0.2, 0.0], "half": [0.4, 0.25, 0.9], "yaw": 0.0,
         "direction": [1.0, 0.0], "speed": 9.0}


def probe_course_moon():
    """Moon probe course data: runs along +X, the loop moves the lane 2.2 m to the left."""
    return {"number": 2, "par": 5, "name": "Selftest Moon", "start": {"at": (0.0, 0.0, 0.0), "heading": 0},
            "pieces": [("tee", {"length": 1.0, "look": "metal"}),
                       ("straight", {"length": 2.0}),
                       ("halfpipe", {"length": 5.0, "width": 1.2, "radius": PIPE_RADIUS, "look": "metal"}),
                       ("straight", {"length": 1.0}),
                       ("anchor", {"name": "boost"}),
                       ("straight", {"length": 1.0}),
                       ("anchor", {"name": "loop_in"}),
                       ("loop", {"radius": 1.3, "offset": 2.2, "look": "metal"}),
                       ("straight", {"length": 2.0}),
                       ("fork", {"length": 2.0, "spread": 16 * SPACING, "main": "left", "join_length": 2.0,
                                 "left": [("straight", {"length": 3.0})],
                                 "right": [("hill", {"length": 3.0, "height": 0.25})]}),
                       ("funnel", {"radius": FUNNEL_RADIUS, "depth": FUNNEL_DEPTH})],
            "hero": None, "props": [], "mechanics": [dict(BOOST)], "obstacles": [], "cycle": None}


def _all_points(track):
    for strip in track.strips:
        for section in strip.sections:
            yield from section.points
    for pad in track.pads:
        if pad.grid.get("polar"):
            for ring in pad.grid["rings"]:
                yield from ring
        else:
            for row in pad.grid["points"]:
                yield from row


def signature(track):
    """SHA-256 of every point of the track and of its route data: equal for equal builds."""
    digest = hashlib.sha256()
    for point in _all_points(track):
        digest.update(struct.pack("<3d", *point))
    for polyline in [track.path] + track.corridors:
        for point in polyline:
            digest.update(struct.pack("<3d", *point))
    digest.update(struct.pack("<d", track.length))
    return digest.hexdigest()


def check_seams(track):
    """Every strip starts with the object that ended its predecessor; every distinct point value is ONE tuple object.

    Returns the number of vertices the floor nodes need: the distinct points each (look, surface) group uses
    (raster points inside a hole have no face; a seam between two looks appears in both nodes).
    """
    ends = [strip.sections[-1] for strip in track.strips]
    exits = [port["section"] for pad in track.pads for port in pad.ports if port["side"] == "exit"]
    exits += [hazard["landing"] for hazard in track.hazards if "landing" in hazard]    # a gap starts a fresh strip
    for k, strip in enumerate(track.strips):
        if not any(strip.sections[0] is section for section in ends[:k] + exits):
            raise AssertionError("strip %d (%s) does not start with the previous section object" % (k, strip.kind))
    for pad in track.pads:
        for port in pad.ports:
            if pad.grid.get("polar"):
                row, c0 = pad.grid["rings"][0], 0
            else:
                row, c0 = pad.grid["points"][0 if port["side"] == "entry" else pad.grid["nx"]], port["cols"][0]
            for k in range(ROWS + 1):
                if row[c0 + k] is not port["section"].points[k]:
                    raise AssertionError("pad %s %s port point %d is not the section's tuple" % (pad.kind, port["side"], k))
    by_value, used = {}, {}
    for strip in track.strips:
        for section in strip.sections:
            for point in section.points:
                by_value.setdefault(point, set()).add(id(point))
        for s0, s1 in zip(strip.sections, strip.sections[1:]):
            used.setdefault((s1.look, s1.surface), set()).update(s0.points + s1.points)
    for pad in track.pads:
        for point in _pad_points(pad):
            by_value.setdefault(point, set()).add(id(point))
        group = used.setdefault((pad.look, pad.surface), set())
        if pad.grid.get("polar"):
            for triangle in pad.faces:
                group.update(triangle)
        else:
            points = pad.grid["points"]
            for i, j in pad.faces:
                group.update((points[i][j], points[i][j + 1], points[i + 1][j], points[i + 1][j + 1]))
    objects = sum(len(ids) for ids in by_value.values())
    if objects != len(by_value):
        raise AssertionError("%d point tuples carry only %d distinct values: a seam was recomputed" % (objects, len(by_value)))
    return sum(len(points) for points in used.values())


def _pad_points(pad):
    if pad.grid.get("polar"):
        return [p for ring in pad.grid["rings"] for p in ring]
    return [p for row in pad.grid["points"] for p in row]


def _normal(a, b, c):
    u, v = (b[0] - a[0], b[1] - a[1], b[2] - a[2]), (c[0] - a[0], c[1] - a[1], c[2] - a[2])
    return (u[1] * v[2] - u[2] * v[1], u[2] * v[0] - u[0] * v[2], u[0] * v[1] - u[1] * v[0])


def check_geometry(track, name):
    """Piece-specific checks: floor orientation, half-pipe, loop, funnel and fork data (moon probe only)."""
    for strip in track.strips:
        if strip.kind == "loop":
            continue
        for s0, s1 in zip(strip.sections, strip.sections[1:]):
            for j in range(ROWS):
                if _normal(s0.points[j], s0.points[j + 1], s1.points[j + 1])[2] < -1e-12 or _normal(s0.points[j], s1.points[j + 1], s1.points[j])[2] < -1e-12:
                    raise AssertionError("a %s floor quad faces down" % strip.kind)
    for pad in track.pads:
        if pad.grid.get("polar"):
            if any(_normal(*t)[2] <= 0 for t in pad.faces):
                raise AssertionError("a funnel floor triangle does not face up")
    if name == "selftest":
        check_jump(track)
    if name != "selftest-moon":
        return
    pipe = next(s for s in track.strips if s.kind == "halfpipe")
    loop = next(s for s in track.strips if s.kind == "loop")
    if not (pipe.sections[0].flat and pipe.sections[-1].flat) or any(s.flat for s in pipe.sections[1:-1]):
        raise AssertionError("the half-pipe must start and end flat and be a U in between")
    lips = max(s.points[0][2] for s in pipe.sections)
    if abs(lips - PIPE_RADIUS) > 1e-9 or any(s.wall_up != ((0.0, 0.0, 1.0), (0.0, 0.0, 1.0)) for s in pipe.sections[1:-1]):
        raise AssertionError("half-pipe lip height %.3f or wall direction wrong" % lips)
    if len(loop.sections) < 49 or not (loop.sections[0].flat and loop.sections[-1].flat):
        raise AssertionError("the loop needs at least 48 sections between two flat ends")
    first, last = loop.sections[0], loop.sections[-1]
    if abs(last.center[1] - first.center[1] - 2.2) > 1e-9 or abs(last.center[0] - first.center[0]) > 1e-9 or abs(last.center[2] - first.center[2]) > 1e-9:
        raise AssertionError("the loop must end 2.2 m to the left of where it started")
    for a, b in zip(loop.sections, loop.sections[1:]):
        if sum(x * y for x, y in zip(a.normal, b.normal)) < math.cos(math.radians(10.0)):
            raise AssertionError("loop floor normal turns more than 10 degrees between two sections")
    top = max(s.center[2] for s in loop.sections)
    if abs(top - 2.6) > 0.02:
        raise AssertionError("the loop top is %.3f m high, expected about 2.6" % top)
    funnel = next(p for p in track.pads if p.kind == "funnel")
    rings = funnel.grid["rings"]
    if any(a is not b for a, b in zip(rings[0], funnel.ports[0]["section"].points)):
        raise AssertionError("the funnel rim does not start with the entry section's point objects")
    inner = min(math.dist(p[:2], funnel.cup[:2]) for p in rings[-1])
    if abs(inner - (CUP_RADIUS + HOLE_MARGIN)) > 1e-6:
        raise AssertionError("innermost funnel ring at %.4f m" % inner)
    if abs(funnel.cup[2] - (rings[0][0][2] - FUNNEL_DEPTH)) > 1e-9:
        raise AssertionError("the funnel hole is not at the bottom of the bowl")
    if len(track.corridors) != 2 or [p.kind for p in track.pads] != ["tee", "fork", "join", "funnel"]:
        raise AssertionError("expected one fork (two corridors) and a funnel, got %d corridors and pads %s"
                             % (len(track.corridors), [p.kind for p in track.pads]))
    if track.corridors[1][-1] != track.corridors[0][-2] or track.corridors[0][-1] != track.cup:
        raise AssertionError("the branch corridor must end on the main route (the join exit)")
    anchors = track.anchors
    if not 0.0 < anchors["loop_in"][0] - BOOST["position"][0] <= 1.5 or abs(anchors["boost"][0] - BOOST["position"][0]) > 1e-9:
        raise AssertionError("the boost pad must lie at most 1.5 m before the loop")


def check_jump(track):
    """The ramp with a lip ends 20 degrees steep, the gap stores a water hazard and a flat landing at the lip height."""
    hazard = next(h for h in track.hazards if "landing" in h)
    landing = hazard["landing"]
    index = next(k for k, strip in enumerate(track.strips[:-1]) if track.strips[k + 1].sections[0] is landing)
    lip = track.strips[index].sections
    if track.strips[index].kind != "ramp":
        raise AssertionError("jump: the gap must follow a ramp")
    run = math.hypot(lip[-1].center[0] - lip[-2].center[0], lip[-1].center[1] - lip[-2].center[1])
    slope = math.degrees(math.atan2(lip[-1].center[2] - lip[-2].center[2], run))
    if abs(slope - 20.0) > 0.5 or hazard["kind"] != "water" or not landing.flat or abs(landing.center[2] - lip[-1].center[2]) > 1e-9:
        raise AssertionError("jump: lip %.2f degrees, hazard %s, landing height %.3f" % (slope, hazard["kind"], landing.center[2]))
    gap = math.dist(landing.center[:2], lip[-1].center[:2])
    if abs(gap - 1.6) > 1e-9 or track.strips[index + 1].sections[0] is not landing:
        raise AssertionError("jump: the gap is %.3f m wide or the landing does not start the next strip" % gap)


def _fails(match, build):
    """True when ``build()`` raises a ``ValueError`` whose message contains ``match``."""
    try:
        build()
    except ValueError as exc:
        if match in str(exc):
            return True
        raise AssertionError("expected %r in the error, got: %s" % (match, exc))
    return False


def _course(pieces, mechanics=()):
    return {"number": 9, "par": 3, "start": {"at": (0.0, 0.0, 0.0), "heading": 0}, "pieces": list(pieces),
            "mechanics": list(mechanics), "obstacles": [], "cycle": None}


def check_rules():
    """Parameter errors of the new pieces and the options of the half-pipe, loop and fork (no Blender needed)."""
    base = [("tee", {"length": 1.0}), ("straight", {"length": 8.0})]
    green = ("green", {"length": 3.0, "width": 3.44, "cup": (1.8, 0.4)})
    fork = {"length": 2.0, "spread": 16 * SPACING, "left": [("straight", {"length": 2.0})], "right": [("straight", {"length": 2.0})]}
    rejected = (
        ("radius must be between", base + [("loop", {"radius": 1.6, "offset": 2.2})]),
        ("offset must be at least", base + [("loop", {"radius": 1.3, "offset": 2.1})]),
        ("sections must be", base + [("loop", {"radius": 1.3, "offset": 2.2, "sections": 40})]),
        ("run-up", [("tee", {"length": 1.0}), ("straight", {"length": 4.0}), ("loop", {"radius": 1.3, "offset": 2.2})]),
        ("blend must be at least", base + [("halfpipe", {"length": 5.0, "blend": 1.0})]),
        ("too short", base + [("halfpipe", {"length": 3.0})]),
        ("either length or radius_path", base + [("halfpipe", {"length": 5.0, "radius_path": 4.0, "angle": 90})]),
        ("steeper than 22 degrees", base + [("funnel", {"radius": 2.4, "depth": 0.8})]),
        ("between 2 and 4", base + [("funnel", {"radius": 1.5, "depth": 0.3})]),
        ("must be an even multiple", base + [("fork", dict(fork, spread=17 * SPACING)), green]),
        ("left branch, piece 0 (curve)", base + [("fork", dict(fork, left=[("curve", {"radius": 1.0, "angle": 30})])), green]),
        ("different heights", base + [("fork", dict(fork, right=[("ramp", {"length": 2.0, "rise": 0.005})])), green]),
        ("hole must not be inside a fork branch", base + [("fork", dict(fork, left=[green])), green]),
    )
    for match, pieces in rejected:
        if not _fails(match, lambda pieces=pieces: layout.build(_course(pieces), "moon", check_length=False)):
            raise AssertionError("expected an error containing %r" % match)
    # a half-pipe on a left bend, then a loop to the right behind a boost pad
    bend = layout.build(_course(base + [("halfpipe", {"radius_path": 4.0, "angle": 90, "width": 1.2, "radius": 0.9}),
                                       ("loop", {"radius": 1.3, "offset": -2.4}), ("straight", {"length": 2.0}), green],
                                [{"kind": "boost", "id": "b"}]), "moon", check_length=False)
    check_seams(bend)
    pipe, loop = bend.strips[1], bend.strips[2]
    if abs(pipe.sections[-1].heading - math.pi / 2) > 1e-9 or not pipe.sections[-1].flat or max(
            math.dist(a.center, b.center) for a, b in zip(pipe.sections, pipe.sections[1:])) > 0.3 + 1e-9:
        raise AssertionError("the curved half-pipe must turn 90 degrees, end flat and keep sections within 0.30 m")
    side = math.dist(loop.sections[-1].center[:2], loop.sections[0].center[:2])
    if abs(side - 2.4) > 1e-9 or loop.sections[-1].center[0] - loop.sections[0].center[0] < 2.3:
        raise AssertionError("a negative loop offset must move the exit lane to the right (here +X after the left bend)")
    # fork: main branch, exit offset
    def routed(main, **extra):
        data = dict(fork, left=[("straight", {"length": 3.0})], right=[("hill", {"length": 3.0, "height": 0.25})], main=main, **extra)
        return layout.build(_course(base + [("fork", data), ("straight", {"length": 2.0}), green]), "moon", check_length=False)
    left, right = routed("left"), routed("right", exit_offset=2)
    if max(p[2] for p in left.path) > 1e-9 or max(p[2] for p in right.path) < 0.2 or max(p[2] for p in right.corridors[1]) > 1e-9:
        raise AssertionError("main selects the route in track.path; the other branch becomes the extra corridor")
    join = right.pads[2]
    exit_point = next(port["section"].center for port in join.ports if port["side"] == "exit")
    back = join.grid["points"][0][join.grid["ny"] // 2]
    if abs(exit_point[1] - back[1] + 2 * SPACING) > 1e-6:
        raise AssertionError("exit_offset 2 must move the join exit two columns to the right")
    check_seams(left)
    check_seams(right)


def _distance_to_triangle(p, a, b, c):
    """Planar distance of point ``p`` to the triangle ``abc`` (0 inside)."""
    def side(u, v, w):
        return (u[0] - w[0]) * (v[1] - w[1]) - (v[0] - w[0]) * (u[1] - w[1])
    d1, d2, d3 = side(p, a, b), side(p, b, c), side(p, c, a)
    if not ((d1 < 0 or d2 < 0 or d3 < 0) and (d1 > 0 or d2 > 0 or d3 > 0)):
        return 0.0

    def segment(u, v):
        ex, ey = v[0] - u[0], v[1] - u[1]
        t = max(0.0, min(1.0, ((p[0] - u[0]) * ex + (p[1] - u[1]) * ey) / (ex * ex + ey * ey))) if ex or ey else 0.0
        return math.hypot(p[0] - u[0] - t * ex, p[1] - u[1] - t * ey)
    return min(segment(a, b), segment(b, c), segment(c, a))
