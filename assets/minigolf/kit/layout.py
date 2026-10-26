"""Course data to ``Track`` (MIT License, see ../LICENSE).

``build(course_data, world)`` walks the piece list of one course (data shape of the design, section
4.2: ``start``, ``pieces``, ``hero``, ``props``, ``mechanics``, ``obstacles``, ``cycle``), asks the
registries for each piece, chains the sections (each strip starts with the previous section object),
collects the route (``path``, ``corridors``, ``length``), hazards and anchors and checks the kit rules.
``hero`` and ``props`` belong to the placement and are not read here; ``mechanics``, ``obstacles`` and
``cycle`` are passed through; ``ponds`` (free ponds in world coordinates, see ``layout_ponds``) become hazards.
Errors are ``ValueError`` with the course number and the piece index.

``PIECES`` and ``PADS`` (the registries of ``pieces`` and ``pads``) are extended by ``pieces_moon`` (half-pipe,
loop) and ``pads_funnel`` (funnel), which this module imports; the layout dispatches on them by name. A
``fork`` is handled by ``layout_fork``: it builds both branches with the functions below.

Rules checked here besides the piece parameters: a loop needs a ``boost`` mechanic in the course or a flat straight
run-up of 8 m (``mechanic_data.resolve`` then requires the pad within 1.5 m of route arc before the entry unless the
run-up is there); the route is 10-35 m long; parts of the route further apart than
4 m along it keep their distance (clearance), except the turns of one loop; sibling branches of a fork are
always checked against each other.
"""

import copy
import math

from . import detail as detail_mod, layout_fork, layout_ponds, pads, pads_funnel, pieces, pieces_moon  # noqa: F401  (the last two register their pieces)
from .track import GRAVITY, LANE_WIDTH, WALL_THICKNESS, Cursor, Strip, Track, distance

PIECES = pieces.PIECES
PADS = pads.PADS
LENGTH_RANGE = (10.0, 35.0)
COVERS = (None, "tunnel")
DECKS = (None, "bridge")
BRIDGE_LOOK = {"pirate": "deck", "temple": "deck", "moon": "metal"}   # floor look of a bridge unless the piece sets its own
NO_COVER = ("loop", "halfpipe")                                       # a ball can reach the roof (the roof has no collision)
CLEARANCE = 0.5
SEPARATE_ARC = 4.0       # parts of the route further apart than this must keep their distance
LOOP_RUN_UP = 8.0
LOOP_BOOST_REACH = 1.5   # a boost pad this far before the loop entry (route arc) carries the ball through (measured, design 4.4)


def _point(value):
    return tuple(float(v) for v in value) + ((0.0,) if len(value) == 2 else ())


def new_state(shared, prev=None, group=(), branch=False):
    """Walking state of one route (the trunk or a fork branch); ``shared`` is common to all of them."""
    return {"prev": prev, "last": prev, "path": [], "done": False, "group": group, "run": 0.0, "branch": branch, "shared": shared}


def _chain(state, sections, group=None):
    """Set ``arc`` on new sections, remember the last one and record clearance samples."""
    samples = state["shared"]["samples"]
    for section in sections:
        section.arc = state["last"].arc + distance(state["last"].center, section.center)
        state["last"] = section
        samples.append((section.arc, section.center, group or state["group"]))


def add_pad_samples(state, pad, group=None):
    """Clearance samples of a pad: ``(arc, point, group)`` along its centre line (a funnel adds its rim)."""
    grid = pad.grid
    if grid.get("polar"):
        pairs = grid["samples"]
    else:
        pairs = [(grid["xs"][i], grid["points"][i][grid["ny"] // 2]) for i in range(grid["nx"] + 1)]
    state["shared"]["samples"].extend((pad.arc0 + offset, point, group or state["group"]) for offset, point in pairs)


def walk_items(track, state, cursor, items, number):
    """Run the pieces ``items`` on ``state``; ``number`` (the course number) prefixes error messages, ``None`` for branches."""
    for index, item in enumerate(items):
        name, params = item[0], (dict(item[1]) if len(item) > 1 else {})
        try:
            cursor = _piece(track, state, cursor, name, params)
        except ValueError as exc:
            where = "piece %d (%s): %s" % (index, name, exc)
            raise ValueError(("course %d, " % number) + where if number is not None else where) from None
    return cursor


def _pad_piece(track, state, cursor, name, params):
    prev = state["prev"]
    pad, exit_section, cursor = PADS[name](prev, cursor, **params)
    if state["branch"] and pad.cup is not None:
        raise ValueError("the green or funnel with the hole must not be inside a fork branch")
    track.pads.append(pad)
    if prev is None:
        pad.arc0 = 0.0
        if pad.mark is None:
            raise ValueError("the first piece must be the tee")
        track.tee = pad.mark
        state["path"].append(pad.mark)
    else:
        pad.arc0 = prev.arc
    add_pad_samples(state, pad)
    if exit_section is not None:
        exit_section.arc = pad.arc0 + pad.size[0]
        state["prev"] = state["last"] = exit_section
        state["path"].append(exit_section.center)
    for hole in pad.holes:
        if hole["kind"] != "cup":
            track.hazards.append(hole)
    state["run"] = pad.size[0] if prev is None else (state["run"] + pad.size[0] if name == "plaza" else 0.0)
    if pad.cup is not None:
        track.cup = pad.cup
        state["path"].append(pad.cup)
        state["done"] = True
    return cursor


def _lane_piece(track, state, cursor, name, params):
    prev, shared = state["prev"], state["shared"]
    if prev is None:
        raise ValueError("the first piece must be the tee")
    cover, deck = params.pop("cover", None), params.pop("deck", None)
    if cover not in COVERS or deck not in DECKS:
        raise ValueError("cover must be 'tunnel' and deck 'bridge' (or omitted)")
    if cover and (name in NO_COVER or params.get("lip")):
        raise ValueError("a tunnel roof has no collision: it must not cover a %s (a ball could reach it)" % ("ramp with a lip" if name == "ramp" else name))
    if deck and "look" not in params:
        params["look"] = BRIDGE_LOOK[track.world]
    if name == "loop" and shared["loop_sections"] and "sections" not in params:
        params["sections"] = shared["loop_sections"]
    if name == "loop" and state["run"] < LOOP_RUN_UP - 1e-9 and not shared["boost"]:
        raise ValueError("a loop needs a boost pad before it or a flat straight run-up of %g m (got %.1f m)" % (LOOP_RUN_UP, state["run"]))
    if name == "loop":
        track.loops.append({"arc": prev.arc, "run": state["run"]})   # mechanic_data.resolve checks where the boost pad lies
    length = params.get("length", 0.0) if name == "straight" else 0.0
    sections, cursor, extras = PIECES[name](prev, cursor, **params)
    group = None
    if name == "loop":
        group = state["group"] + (("loop", shared["loops"]),)
        shared["loops"] += 1
    state["run"] = state["run"] + length if name == "straight" else 0.0
    if extras.get("break"):
        track.hazards.append(extras["hazard"])
        _chain(state, sections, group)
        state["prev"] = sections[-1]
        state["path"].append(sections[-1].center)
        return cursor
    track.strips.append(Strip([prev] + sections, name, cover, deck))
    _chain(state, sections, group)
    state["prev"] = sections[-1]
    state["path"].extend(s.center for s in sections)
    return cursor


def _piece(track, state, cursor, name, params):
    if state["done"]:
        raise ValueError("nothing may follow the green")
    if name == "anchor":
        if params["name"] in track.anchors:
            raise ValueError("anchor %r is already defined (anchor names are unique over the course, branches included)" % params["name"])
        track.anchors[params["name"]] = (cursor.x, cursor.y, cursor.z, cursor.heading)
        return cursor
    if name == "fork":
        return layout_fork.build_fork(track, state, cursor, params)
    if name in PADS:
        return _pad_piece(track, state, cursor, name, params)
    if name not in PIECES:
        raise ValueError("unknown piece (known: %s)" % ", ".join(sorted(list(PADS) + list(PIECES) + ["anchor", "fork"])))
    return _lane_piece(track, state, cursor, name, params)


def _related(a, b):
    """True when two groups never need a clearance check against each other (same loop, a fork pad and its branches)."""
    loops = {e for e in a if e[0] == "loop"}
    if loops & {e for e in b if e[0] == "loop"}:
        return True
    pads_of = {e[1] for e in a + b if e[0] == "fork_pad"}
    return any(e[0] == "branch" and e[1] in pads_of for e in a + b) and any(e[0] == "fork_pad" for e in a + b)


def _siblings(a, b):
    return any(x[0] == "branch" and y[0] == "branch" and x[1] == y[1] and x[2] != y[2] for x in a for y in b)


def _check_clearance(samples):
    limit = LANE_WIDTH + 2 * WALL_THICKNESS
    for a in range(len(samples)):
        for b in range(a + 1, len(samples)):
            (arc_a, p, group_a), (arc_b, q, group_b) = samples[a], samples[b]
            if abs(p[2] - q[2]) >= CLEARANCE or math.hypot(p[0] - q[0], p[1] - q[1]) >= limit or _related(group_a, group_b):
                continue
            if abs(arc_a - arc_b) > SEPARATE_ARC or _siblings(group_a, group_b):
                raise ValueError("two parts of the route (%.1f m and %.1f m along it) overlap with less than %.1f m clearance"
                                 % (arc_a, arc_b, CLEARANCE))


def polyline(points):
    """``points`` without consecutive duplicates."""
    out = []
    for point in points:
        if not out or distance(out[-1], point) > 1e-9:
            out.append(point)
    return out


def build(course_data, world, check_length=True):
    """Build the ``Track`` of one course; see the module docstring."""
    number = course_data.get("number", 0)
    if world not in GRAVITY:
        raise ValueError("course %d: unknown world %r" % (number, world))
    track = Track(world, number, course_data.get("par", 0))
    mechanics = list(course_data.get("mechanics", []))
    shared = {"samples": [], "alts": [], "forks": 0, "loops": 0, "boost": any(m.get("kind") == "boost" for m in mechanics),
              "loop_sections": detail_mod.resolve(course_data)["loop_sections"]}
    state = new_state(shared)
    start = course_data["start"]
    at = _point(start["at"])
    walk_items(track, state, Cursor(at[0], at[1], at[2], math.radians(start.get("heading", 0.0))), course_data["pieces"], number)
    where = "course %d: " % number
    if track.cup is None:
        raise ValueError(where + "the course must end on a green with the hole")
    path = polyline(state["path"])
    track.path = path
    track.corridors = [list(path)] + [polyline(alt) for alt in shared["alts"]]
    track.length = sum(distance(path[i], path[i + 1]) for i in range(len(path) - 1))
    if check_length and not LENGTH_RANGE[0] <= track.length <= LENGTH_RANGE[1]:
        raise ValueError(where + "the main route is %.1f m long, allowed %g-%g m" % ((track.length,) + LENGTH_RANGE))
    try:
        _check_clearance(shared["samples"])
    except ValueError as exc:
        raise ValueError(where + str(exc)) from None
    layout_ponds.add_ponds(track, course_data.get("ponds", []), number)
    track.mechanics = copy.deepcopy(mechanics)
    track.obstacles = copy.deepcopy(list(course_data.get("obstacles", [])))
    track.cycle = course_data.get("cycle")
    track.gravity = course_data.get("gravity", track.gravity)
    return track
