"""Moving obstacles: course data to manifest entry and part list (MIT License, see ../LICENSE).

Plain Python, no ``bpy``. ``resolve`` turns one entry of the course data ``obstacles`` list (Blender metres, degrees)
into ``{"runtime": manifest entry, "parts": collision parts, "pose": ..., "frame": ...}``. The collision parts are
the single source of the sizes: ``obstacle_parts`` draws the visible core from the very same list, so a box part is
a box of exactly those extents and nothing sticks out of it. Parts live in the body frame of the obstacle (origin =
the kinematic body origin, which is also the origin of the ``obstacle-<id>`` node): Blender axes with X along the
body ``heading``, Y to the left and Z up. ``runtime_part`` converts one part to the manifest (runtime axes
``(x, z, -y)``).

Data keys, common to all kinds: ``id``, ``kind``, ``at``/``anchor``/``offset``/``heading`` (see ``mechanic_floor``),
``speed`` (rad/s) or ``period`` (s), ``phase`` (rad), ``surface``, ``sharp`` (gate only, 1-4).
``heading`` is the direction of the lane at the spot (default: the route direction).

* ``rotate``  pendulum: ``amplitude`` (rad, default 0.6), ``length`` (pivot to the middle of the weight), ``swing``
              ``"across"`` the lane (axis along the lane, default) or ``"along"`` it, ``clearance`` (rest height of
              the weight's lower edge, 0.03), ``span`` (distance of the gallows posts from the pivot).
              Pirate: iron anchor weight on a chain; temple: log on two lianas; no moon pendulum.
* ``gate``    hoist gate across the lane that rises and stops: ``travel`` (m, default 0.75), ``width``
              (default 2.0, the whole lane with rails), ``height`` (0.70); pirate hoist gate, temple stone door,
              moon blast door.
* ``slide``   a block that slides across the lane: ``width`` (0.9), ``height`` (0.5), ``amplitude`` (m).
* ``boulder`` temple stone ball that rolls across the lane (``move: "along"`` for along it): ``radius`` (0.5),
              ``amplitude`` (m, default 0.55).
* ``spin``    moon radar arm that turns around a vertical hub: ``radius`` (arm half length, 0.75).
"""

import math

from .mechanic_floor import blender, rounded, runtime
from .track import LANE_WIDTH, WALL_THICKNESS

WALL_SURFACE = {"pirate": "wood", "temple": "stone", "moon": "metal"}
SURFACES = ("grass", "sand", "wood", "stone", "metal")
SINK = 0.02              # gates and arms reach this far into the floor
POLE = 0.04              # half width of the thin pendulum rods (chain, lianas)
GATE_THICK = {"pirate": 0.08, "temple": 0.14, "moon": 0.07}      # half thickness of the panel
GATE_WIDTH = LANE_WIDTH + 2 * WALL_THICKNESS
POST = 0.13              # half width of a gate post
ANCHOR_WEIGHT = (0.28, 0.20, 0.30)      # half extents of the pirate weight (along axis, swing direction, height)
LOG = (0.20, 0.62)                      # temple log: radius, half length
KINDS = {"pirate": ("rotate", "gate", "slide"), "temple": ("rotate", "gate", "slide", "boulder"), "moon": ("gate", "slide", "spin")}
KEYS = {"id", "kind", "at", "anchor", "offset", "heading", "z", "speed", "period", "phase", "surface", "sharp", "amplitude",
        "length", "swing", "clearance", "span", "travel", "width", "height", "radius", "move"}


# -- quaternions [x, y, z, w], the same maths as ``minigolf-mechanics.js`` ------------------------------------------

def axis_angle(axis, angle):
    sine = math.sin(angle / 2)
    return [axis[0] * sine, axis[1] * sine, axis[2] * sine, math.cos(angle / 2)]


def multiply(a, b):
    return [a[3] * b[0] + a[0] * b[3] + a[1] * b[2] - a[2] * b[1],
            a[3] * b[1] - a[0] * b[2] + a[1] * b[3] + a[2] * b[0],
            a[3] * b[2] + a[0] * b[1] - a[1] * b[0] + a[2] * b[3],
            a[3] * b[3] - a[0] * b[0] - a[1] * b[1] - a[2] * b[2]]


def unit(v):
    length = math.sqrt(sum(c * c for c in v)) or 1.0
    return [c / length for c in v]


def pose_at(spec, time=0.0):
    """``(position, rotation)`` of the manifest obstacle ``spec`` at ``time``: ``obstaclePose`` of the runtime."""
    phase = time * spec["speed"] + spec.get("phase", 0.0)
    axis = unit(spec.get("axis", [0, 1, 0]))
    base = spec.get("rotation", [0, 0, 0, 1])
    amp = spec.get("amplitude", 0.0)
    at = lambda offset: [p + a * offset for p, a in zip(spec["position"], axis)]
    kind = spec["kind"]
    if kind == "rotate":
        return list(spec["position"]), multiply(axis_angle(axis, math.sin(phase) * amp), base)
    if kind == "spin":
        return list(spec["position"]), multiply(axis_angle(axis, phase), base)
    if kind == "slide":
        return at(math.sin(phase) * amp), base
    if kind == "gate":
        return at(amp * max(-1.0, min(1.0, spec.get("sharp", 2) * math.sin(phase)))), base
    offset = amp * math.sin(phase)
    return at(offset), multiply(axis_angle(unit([axis[2], 0, -axis[0]]), offset / spec["radius"]), base)


# -- collision parts ------------------------------------------------------------------------------------------------

def box(role, center, half):
    return {"shape": "box", "role": role, "center": tuple(center), "half": tuple(half)}


def cylinder(role, center, radius, half, axis="z"):
    return {"shape": "cylinder", "role": role, "center": tuple(center), "radius": radius, "half": half, "axis": axis}


def ball(role, center, radius):
    return {"shape": "ball", "role": role, "center": tuple(center), "radius": radius}


def runtime_part(part):
    """The manifest collider of one body-frame part."""
    position = rounded(runtime(part["center"]))
    if part["shape"] == "box":
        return {"shape": "box", "position": position, "halfExtents": rounded((part["half"][0], part["half"][2], part["half"][1]))}
    if part["shape"] == "ball":
        return {"shape": "ball", "position": position, "radius": round(part["radius"], 5)}
    out = {"shape": "cylinder", "position": position, "radius": round(part["radius"], 5), "halfHeight": round(part["half"], 5)}
    if part["axis"] == "x":                 # the runtime cylinder stands on Y: lay it down along the body X axis
        out["rotation"] = [0.0, 0.0, round(-math.sqrt(0.5), 6), round(math.sqrt(0.5), 6)]
    elif part["axis"] != "z":
        raise ValueError("cylinder parts stand (z) or lie along the body X axis (x)")
    return out


def part_extent(part):
    """Half extents ``(x, y, z)`` of the axis-aligned box around a body-frame part."""
    if part["shape"] == "box":
        return part["half"]
    if part["shape"] == "ball":
        return (part["radius"],) * 3
    r, h = part["radius"], part["half"]
    return (h, r, r) if part["axis"] == "x" else (r, r, h)


# -- kinds ----------------------------------------------------------------------------------------------------------

def _num(item, key, default, low, high, where):
    value = item.get(key, default)
    if not isinstance(value, (int, float)) or isinstance(value, bool) or not low <= value <= high:
        raise ValueError("%s: %s must be a number from %g to %g (got %r)" % (where, key, low, high, value))
    return float(value)


def _posts(x, y, side, offset):
    """Plan positions of the two posts of a frame, ``offset`` either side of ``(x, y)`` along the direction ``side``."""
    return [(x + sign * offset * math.cos(side), y + sign * offset * math.sin(side)) for sign in (-1, 1)]


def _pendulum(world, item, x, y, z, heading, where):
    if world == "moon":
        raise ValueError("%s: the moon has no pendulum (use gate, slide or spin)" % where)
    across = item.get("swing", "across")
    if across not in ("across", "along"):
        raise ValueError("%s: swing is 'across' or 'along' the lane" % where)
    axis_heading = heading if across == "across" else heading + math.pi / 2
    length = _num(item, "length", 1.3, 0.8, 2.6, where)
    clearance = _num(item, "clearance", 0.03, 0.0, 0.12, where)
    if world == "pirate":
        hx, hy, hz = ANCHOR_WEIGHT
        parts = [box("core", (0, 0, -length), (hx, hy, hz)),
                 box("chain", (0, 0, -(length - hz) / 2), (POLE, POLE, (length - hz) / 2))]
        low = hz
    else:
        radius, half = LOG
        parts = [cylinder("log", (0, 0, -length), radius, half, "x")]
        for sign in (-1, 1):
            parts.append(box("vine", (sign * (half - 0.14), 0, -(length - radius) / 2), (POLE, POLE, (length - radius) / 2)))
        low = radius
    pivot_z = z + clearance + low + length
    amplitude = _num(item, "amplitude", 0.6, 0.1, 1.45, where)
    span = _num(item, "span", LANE_WIDTH / 2 + 0.34, 0.9, 3.0, where)
    return {"origin": (x, y, pivot_z), "body_heading": axis_heading, "axis": [math.cos(axis_heading), 0.0, -math.sin(axis_heading)],
            "amplitude": amplitude, "parts": parts,
            "frame": {"kind": "gallows", "pivot": (x, y, pivot_z), "lateral": heading + math.pi / 2, "span": span, "floor": z,
                      "posts": _posts(x, y, heading + math.pi / 2, span)}}


def _gate(world, item, x, y, z, heading, where):
    width = _num(item, "width", GATE_WIDTH, 0.4, 6.0, where)
    height = _num(item, "height", 0.70, 0.45, 1.6, where)
    travel = _num(item, "travel", 0.75, 0.3, 2.0, where)
    sharp = _num(item, "sharp", 2.0, 1.0, 4.0, where)
    closed = z - SINK + height / 2
    return {"origin": (x, y, closed + travel / 2), "body_heading": heading, "axis": [0.0, 1.0, 0.0], "amplitude": travel / 2, "sharp": sharp,
            "parts": [box("panel", (0, 0, 0), (GATE_THICK[world], width / 2, height / 2))],
            "frame": {"kind": "door", "centre": (x, y, z), "heading": heading, "width": width, "height": height, "travel": travel,
                      "posts": _posts(x, y, heading + math.pi / 2, width / 2 + POST)}}


def _slide(world, item, x, y, z, heading, where):
    width = _num(item, "width", 0.9, 0.3, 3.0, where)
    height = _num(item, "height", 0.5, 0.4, 1.2, where)
    amplitude = _num(item, "amplitude", 0.7, 0.1, 3.0, where)
    lateral = heading + math.pi / 2
    return {"origin": (x, y, z - SINK + height / 2), "body_heading": heading, "axis": [math.cos(lateral), 0.0, -math.sin(lateral)],
            "amplitude": amplitude, "parts": [box("panel", (0, 0, 0), (GATE_THICK[world] * 1.4, width / 2, height / 2))], "frame": None}


def _boulder(world, item, x, y, z, heading, where):
    if world != "temple":
        raise ValueError("%s: only the temple has a boulder" % where)
    radius = _num(item, "radius", 0.5, 0.25, 0.9, where)
    direction = heading + math.pi / 2 if item.get("move", "across") == "across" else heading
    return {"origin": (x, y, z + radius), "body_heading": None, "axis": [math.cos(direction), 0.0, -math.sin(direction)],
            "amplitude": _num(item, "amplitude", 0.55, 0.1, 3.0, where), "radius": radius,
            "parts": [ball("stone", (0, 0, 0), radius)], "frame": None}


def _spin(world, item, x, y, z, heading, where):
    if world != "moon":
        raise ValueError("%s: only the moon has a radar arm" % where)
    arm = _num(item, "radius", 0.75, 0.4, 1.2, where)
    parts = [cylinder("hub", (0, 0, 0), 0.22, 0.30), box("arm", (0, 0, 0.08), (arm, 0.08, 0.11))]
    parts += [box("blade", (sign * (arm - 0.045), 0, 0), (0.045, 0.18, 0.28)) for sign in (-1, 1)]
    return {"origin": (x, y, z - SINK + 0.30), "body_heading": heading, "axis": [0.0, 1.0, 0.0], "amplitude": None, "parts": parts, "frame": None}


BUILDERS = {"rotate": _pendulum, "gate": _gate, "slide": _slide, "boulder": _boulder, "spin": _spin}


def resolve(world, item, located, where):
    """Resolve one obstacle of the course data; ``located`` is ``(x, y, z, heading)`` of its spot (``mechanic_floor.locate``)."""
    unknown = sorted(set(item) - KEYS)
    if unknown:
        raise ValueError("%s: unknown field(s) %s" % (where, ", ".join(unknown)))
    kind = item.get("kind")
    if kind not in KINDS[world]:
        raise ValueError("%s: kind %r does not exist in the %s world (known: %s)" % (where, kind, world, ", ".join(KINDS[world])))
    if "speed" in item and "period" in item:
        raise ValueError("%s: give 'speed' or 'period', not both" % where)
    speed = float(item["speed"]) if "speed" in item else (2 * math.pi / float(item["period"]) if item.get("period") else None)
    if speed is None or not math.isfinite(speed) or speed == 0:
        raise ValueError("%s: give a non-zero 'speed' (rad/s) or a 'period' (s)" % where)
    surface = item.get("surface", WALL_SURFACE[world])
    if surface not in SURFACES:
        raise ValueError("%s: surface must be one of %s" % (where, ", ".join(SURFACES)))
    x, y, z, heading = located
    built = BUILDERS[kind](world, item, x, y, z, heading, where)
    phase = float(item.get("phase", 0.0))
    node = "obstacle-" + item["id"]
    spec = {"id": item["id"], "kind": kind, "node": node, "position": rounded(runtime(built["origin"])), "axis": rounded(built["axis"], 6)}
    if built["amplitude"] is not None:
        spec["amplitude"] = round(built["amplitude"], 5)
    spec.update({"speed": round(speed, 6), "phase": round(phase, 6)})
    if kind == "gate":
        spec["sharp"] = round(built["sharp"], 3)
    if kind == "boulder":
        spec["radius"] = round(built["radius"], 5)
    if built["body_heading"] is not None:
        spec["rotation"] = rounded(axis_angle([0.0, 1.0, 0.0], built["body_heading"]), 6)
    spec["surface"] = surface
    spec["colliders"] = [runtime_part(part) for part in built["parts"]]
    position, rotation = pose_at(spec, 0.0)
    return {"id": item["id"], "kind": kind, "world": world, "node": node, "runtime": spec, "parts": built["parts"], "frame": built["frame"],
            "pose": {"location": blender(position), "rotation": rotation}, "heading": built["body_heading"]}
