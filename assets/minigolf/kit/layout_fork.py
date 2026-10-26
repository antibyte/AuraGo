"""Forks of the layout (MIT License, see ../LICENSE).

``("fork", {...})`` in a piece list splits the lane into two branches that merge again::

    ("fork", {"length": 2.0, "spread": 2.2933, "left": [...], "right": [...], "main": "left",
              "join_length": 2.0, "exit_offset": 0, "width": None})

``pads.fork`` builds the splitting area (two exits ``spread`` m apart), each branch is a piece list walked
with the normal layout functions (so a branch may contain any lane piece, a gap or another fork, but not the
green), and ``pads.join`` closes the fork from the two branch end sections (direction 0.01 degrees, height
and forward coordinate 1 mm, lateral distance an even multiple of the row spacing). The ``main`` branch is the
route in ``Track.path`` and ``Track.length``; the other branch becomes an extra polyline in ``Track.corridors``
from the section before the fork to the exit of the join. A fork resets the straight run-up of a loop.
"""

from . import pads
from .track import STYLE_KEYS, Cursor

BRANCHES = ("left", "right")


def build_fork(track, state, cursor, params):
    """Build a fork on ``state``; returns the cursor after the join pad."""
    from . import layout

    prev, shared = state["prev"], state["shared"]
    if prev is None:
        raise ValueError("the first piece must be the tee")
    options = dict(params)
    items = {side: options.pop(side, []) for side in BRANCHES}
    main = options.pop("main", "left")
    join_length = options.pop("join_length", 2.0)
    exit_offset = options.pop("exit_offset", 0)
    if main not in BRANCHES:
        raise ValueError("main must be 'left' or 'right' (got %r)" % (main,))
    if "length" not in options or "spread" not in options:
        raise ValueError("a fork needs length and spread")
    style = {key: options[key] for key in STYLE_KEYS if key in options}
    fork_id = shared["forks"]
    shared["forks"] += 1
    pad_group = state["group"] + (("fork_pad", fork_id),)

    split, exits, _ = pads.fork(prev, cursor, **options)
    split.arc0 = prev.arc
    track.pads.append(split)
    layout.add_pad_samples(state, split, pad_group)
    for exit_section in exits:
        exit_section.arc = split.arc0 + split.size[0]
    routes = {}
    for side, exit_section in zip(BRANCHES, exits):
        sub = layout.new_state(shared, exit_section, state["group"] + (("branch", fork_id, side),), branch=True)
        sub["path"].append(exit_section.center)
        try:
            layout.walk_items(track, sub, Cursor.at(exit_section), items[side], None)
        except ValueError as exc:
            raise ValueError("%s branch, %s" % (side, exc)) from None
        routes[side] = sub
    join, exit_section, new_cursor = pads.join(routes["left"]["prev"], routes["right"]["prev"], join_length, exit_offset, **style)
    other = BRANCHES[1 - BRANCHES.index(main)]
    join.arc0 = routes[main]["last"].arc
    track.pads.append(join)
    layout.add_pad_samples(state, join, pad_group)
    exit_section.arc = join.arc0 + join.size[0]
    state["path"].extend(routes[main]["path"])
    state["path"].append(exit_section.center)
    shared["alts"].append([prev.center] + routes[other]["path"] + [exit_section.center])
    state["prev"] = state["last"] = exit_section
    state["run"] = 0.0
    return new_cursor
