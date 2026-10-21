"""Course data to ``Track`` (MIT License, see ../LICENSE).

``build(course_data, world)`` walks the piece list of one course (data shape of the design, section
4.2: ``start``, ``pieces``, ``hero``, ``props``, ``mechanics``, ``obstacles``, ``cycle``), asks the
registries for each piece, chains the sections (each strip starts with the previous section object),
collects the route (``path``, ``corridors``, ``length``), hazards and anchors and checks the kit rules.
``hero`` and ``props`` belong to the placement and are not read here; ``mechanics``, ``obstacles`` and
``cycle`` are passed through. Errors are ``ValueError`` with the course number and the piece index.

``PIECES`` and ``PADS`` (the registries of ``pieces`` and ``pads``) are extended by later modules, for example
moon pieces and forks; the layout dispatches on them by name.
"""

import copy
import math

from . import pads, pieces
from .track import GRAVITY, LANE_WIDTH, WALL_THICKNESS, Cursor, Strip, Track, distance

PIECES = pieces.PIECES
PADS = pads.PADS
LENGTH_RANGE = (10.0, 35.0)
COVERS = (None, "tunnel")
DECKS = (None, "bridge")
CLEARANCE = 0.5
SEPARATE_ARC = 4.0       # parts of the route further apart than this must keep their distance


def _point(value):
    return tuple(float(v) for v in value) + ((0.0,) if len(value) == 2 else ())


def _chain(state, sections):
    """Set ``arc`` on new sections and remember the last one; returns the arc of the last."""
    for section in sections:
        section.arc = state["last"].arc + distance(state["last"].center, section.center)
        state["last"] = section


def _walk(track, data, number):
    start = data["start"]
    at = _point(start["at"])
    cursor = Cursor(at[0], at[1], at[2], math.radians(start.get("heading", 0.0)))
    state = {"prev": None, "last": None, "path": [], "done": False}
    for index, item in enumerate(data["pieces"]):
        name, params = item[0], (dict(item[1]) if len(item) > 1 else {})
        try:
            cursor = _piece(track, state, cursor, name, params)
        except ValueError as exc:
            raise ValueError("course %d, piece %d (%s): %s" % (number, index, name, exc)) from None
    return state


def _piece(track, state, cursor, name, params):
    prev = state["prev"]
    if state["done"]:
        raise ValueError("nothing may follow the green")
    if name == "anchor":
        track.anchors[params["name"]] = (cursor.x, cursor.y, cursor.z, cursor.heading)
        return cursor
    if name in PADS:
        pad, exit_section, cursor = PADS[name](prev, cursor, **params)
        track.pads.append(pad)
        if prev is None:
            pad.arc0 = 0.0
            if pad.mark is None:
                raise ValueError("the first piece must be the tee")
            track.tee = pad.mark
            state["path"].append(pad.mark)
        else:
            pad.arc0 = prev.arc
        if exit_section is not None:
            exit_section.arc = pad.arc0 + pad.size[0]
            state["prev"] = state["last"] = exit_section
            state["path"].append(exit_section.center)
        for hole in pad.holes:
            if hole["kind"] != "cup":
                track.hazards.append(hole)
        if pad.cup is not None:
            track.cup = pad.cup
            state["path"].append(pad.cup)
            state["done"] = True
        return cursor
    if name not in PIECES:
        raise ValueError("unknown piece (known: %s)" % ", ".join(sorted(list(PADS) + list(PIECES) + ["anchor"])))
    if prev is None:
        raise ValueError("the first piece must be the tee")
    cover, deck = params.pop("cover", None), params.pop("deck", None)
    if cover not in COVERS or deck not in DECKS:
        raise ValueError("cover must be 'tunnel' and deck 'bridge' (or omitted)")
    sections, cursor, extras = PIECES[name](prev, cursor, **params)
    if extras.get("break"):
        track.hazards.append(extras["hazard"])
        _chain(state, sections)
        state["prev"] = sections[-1]
        state["path"].append(sections[-1].center)
        return cursor
    track.strips.append(Strip([prev] + sections, name, cover, deck))
    _chain(state, sections)
    state["prev"] = sections[-1]
    state["path"].extend(s.center for s in sections)
    return cursor


def _samples(track):
    out = []
    for strip in track.strips:
        out.extend((s.arc, s.center) for s in strip.sections)
    for pad in track.pads:
        grid = pad.grid
        out.extend((pad.arc0 + grid["xs"][i], grid["points"][i][grid["ny"] // 2]) for i in range(grid["nx"] + 1))
    return out


def _check_clearance(samples):
    limit = LANE_WIDTH + 2 * WALL_THICKNESS
    for a in range(len(samples)):
        for b in range(a + 1, len(samples)):
            (arc_a, p), (arc_b, q) = samples[a], samples[b]
            if (abs(arc_a - arc_b) > SEPARATE_ARC and abs(p[2] - q[2]) < CLEARANCE
                    and math.hypot(p[0] - q[0], p[1] - q[1]) < limit):
                raise ValueError("two parts of the route (%.1f m and %.1f m along it) overlap with less than %.1f m clearance"
                                 % (arc_a, arc_b, CLEARANCE))


def build(course_data, world, check_length=True):
    """Build the ``Track`` of one course; see the module docstring."""
    number = course_data.get("number", 0)
    if world not in GRAVITY:
        raise ValueError("course %d: unknown world %r" % (number, world))
    track = Track(world, number, course_data.get("par", 0))
    state = _walk(track, course_data, number)
    where = "course %d: " % number
    if track.cup is None:
        raise ValueError(where + "the course must end on a green with the hole")
    path = []
    for point in state["path"]:
        if not path or distance(path[-1], point) > 1e-9:
            path.append(point)
    track.path = path
    track.corridors = [list(path)]
    track.length = sum(distance(path[i], path[i + 1]) for i in range(len(path) - 1))
    if check_length and not LENGTH_RANGE[0] <= track.length <= LENGTH_RANGE[1]:
        raise ValueError(where + "the main route is %.1f m long, allowed %g-%g m" % ((track.length,) + LENGTH_RANGE))
    try:
        _check_clearance(_samples(track))
    except ValueError as exc:
        raise ValueError(where + str(exc)) from None
    track.mechanics = copy.deepcopy(list(course_data.get("mechanics", [])))
    track.obstacles = copy.deepcopy(list(course_data.get("obstacles", [])))
    track.cycle = course_data.get("cycle")
    track.gravity = course_data.get("gravity", track.gravity)
    return track
