"""Mechanics and obstacles of a course: data to manifest entries and build specs (MIT License, see ../LICENSE).

Plain Python, no ``bpy``. ``resolve(track)`` reads the ``mechanics`` and ``obstacles`` of the course data (authored
in Blender metres and degrees, see below and ``obstacle_data``), replaces ``track.mechanics`` and ``track.obstacles``
by the runtime entries of the manifest (axes ``(x, z, -y)``, exactly the fields ``validateMechanic`` and
``validateObstacle`` of ``minigolf-mechanics.js`` read) and returns the ``Resolved`` object that the part builders,
the placement (``reserved`` circles) and the checks use.

Placement keys of every entry: ``id`` (``^[a-z][a-z0-9-]{0,31}$``, unique over mechanics and obstacles), ``kind``,
``at`` ``(x, y[, z])`` or ``anchor`` + ``offset`` ``(forward, left[, up])``, ``heading`` (degrees; default: the route
direction). Kinds and their keys (defaults in brackets, they depend on the world):

* ``bumper``   ``radius`` (0.30), ``height`` (0.50), ``kick`` (5.0 m/s), ``surface`` (the world's wall surface)
* ``boost``    moon only. ``length`` (1.4) along the heading, ``width`` (1.0), ``speed`` (9.0 m/s)
* ``teleport`` ``radius`` (0.45), ``exit`` ``(x, y[, z])`` or a placement dict, ``exit_heading`` (degrees),
               ``min_speed`` (2.0 m/s)
* ``cannon``   pirate only. ``target`` ``(x, y[, z])`` or a placement dict where the ball lands, ``radius`` (0.45), ``flight`` (1.4 s),
               ``delay`` (0.6 s), ``barrel`` (1.5 m), ``breech`` (0.62 m)

Checks at build time: unknown keys, worlds that lack a kind, positions off the floor, cycle times that are not a
whole number of turns (``cycle * speed / (2 pi)`` must be an integer for every obstacle), and for a cannon the 20
sample flight (``cannon_flight``): inside ``bounds`` and 0.5 m from floor, rails, obstacles and props.
"""

import math
import re

from . import cannon_flight, obstacle_data
from .mechanic_floor import FloorField, blender, locate, rounded, runtime
from .rails import rail_runs
from .track import BALL_RADIUS

ID = re.compile(r"^[a-z][a-z0-9-]{0,31}$")
WALL_SURFACE = obstacle_data.WALL_SURFACE
BUMPER = {"pirate": (0.30, 0.52), "temple": (0.30, 0.50), "moon": (0.26, 0.62)}
KICK, BOOST, TELEPORT_RADIUS, MIN_SPEED, CANNON_RADIUS = 5.0, (1.4, 1.0, 9.0), 0.45, 2.0, 0.45
WORLD_KINDS = {"pirate": ("bumper", "teleport", "cannon"), "temple": ("bumper", "teleport"), "moon": ("bumper", "boost", "teleport")}
PLACE = {"id", "kind", "at", "anchor", "offset", "heading", "z"}
KEYS = {"bumper": {"radius", "height", "kick", "surface"}, "boost": {"length", "width", "speed"},
        "teleport": {"radius", "exit", "exit_heading", "min_speed"},
        "cannon": {"target", "radius", "flight", "delay", "barrel", "breech"}}
SLOPE = 0.05


def _number(item, key, default, low, high, where):
    value = item.get(key, default)
    if not isinstance(value, (int, float)) or isinstance(value, bool) or not low <= value <= high:
        raise ValueError("%s: %s must be a number from %g to %g (got %r)" % (where, key, low, high, value))
    return float(value)


def _direction(heading):
    return [round(math.cos(heading), 6), round(-math.sin(heading), 6)]


class Resolved:
    """The mechanics and obstacles of one course in runtime form plus what the Blender builders need."""

    def __init__(self, track, floor):
        self.track, self.floor, self.world = track, floor, track.world
        self.mechanics, self.obstacles, self.mechanic_items, self.obstacle_items = [], [], [], []
        self.reserved = []
        self.flights = []

    # -- mechanics ------------------------------------------------------------------------------------------------

    def add_mechanic(self, item, where):
        kind = item.get("kind")
        if kind not in WORLD_KINDS[self.world]:
            raise ValueError("%s: kind %r does not exist in the %s world (known: %s)" % (where, kind, self.world, ", ".join(WORLD_KINDS[self.world])))
        unknown = sorted(set(item) - PLACE - KEYS[kind])
        if unknown:
            raise ValueError("%s: unknown field(s) %s" % (where, ", ".join(unknown)))
        x, y, z, heading = locate(self.track, self.floor, item, where)
        spec = {"id": item["id"], "kind": kind}
        build = {"type": "mechanic", "kind": kind, "id": item["id"], "world": self.world, "x": x, "y": y, "z": z, "heading": heading}
        getattr(self, "_" + kind)(item, spec, build, where)
        self.mechanics.append(spec)
        self.mechanic_items.append(build)

    def _bumper(self, item, spec, build, where):
        radius_default, height_default = BUMPER[self.world]
        radius = _number(item, "radius", radius_default, 0.15, 0.6, where)
        height = _number(item, "height", height_default, 0.3, 1.2, where)
        z = build["z"]
        if "z" not in item and len(item.get("at", ())) != 3:
            spread = self.floor.spread(build["x"], build["y"], radius)
            if spread is None:
                raise ValueError("%s: the bumper stands partly off the floor" % where)
            if spread[1] - spread[0] > SLOPE:
                raise ValueError("%s: the floor under the bumper is %.0f cm out of level (limit %.0f cm)" % (where, 100 * (spread[1] - spread[0]), 100 * SLOPE))
            z = spread[0]
        surface = item.get("surface", WALL_SURFACE[self.world])
        build.update(z=z, radius=radius, height=height)
        spec.update({"position": rounded(runtime((build["x"], build["y"], z))), "radius": round(radius, 5), "height": round(height, 5),
                     "surface": surface, "kick": round(_number(item, "kick", KICK, 1.0, 12.0, where), 4)})
        self.reserved.append((build["x"], build["y"], radius + 0.35))

    def _boost(self, item, spec, build, where):
        length = _number(item, "length", BOOST[0], 0.4, 4.0, where)
        width = _number(item, "width", BOOST[1], 0.4, 4.0, where)
        speed = _number(item, "speed", BOOST[2], 2.0, 14.0, where)
        x, y, h = build["x"], build["y"], build["heading"]
        heights = []
        for u in (-1, 0, 1):
            for v in (-1, 0, 1):
                px = x + u * length / 2 * math.cos(h) - v * width / 2 * math.sin(h)
                py = y + u * length / 2 * math.sin(h) + v * width / 2 * math.cos(h)
                z = self.floor.height(px, py)
                if z is None:
                    raise ValueError("%s: the boost pad reaches off the floor at (%.2f, %.2f)" % (where, px, py))
                heights.append(z)
        low, high = min(heights), max(heights)
        centre = (low + high) / 2 + BALL_RADIUS
        build.update(length=length, width=width, low=low, high=high)
        spec.update({"position": rounded(runtime((x, y, centre))), "half": rounded((length / 2, max(0.3, (high - low) / 2 + 0.25), width / 2)),
                     "yaw": round(h, 6), "direction": _direction(h), "speed": round(speed, 4)})
        self.reserved.append((x, y, max(length, width) / 2 + 0.1))

    def _teleport(self, item, spec, build, where):
        radius = _number(item, "radius", TELEPORT_RADIUS, 0.25, 0.8, where)
        exit_item = item.get("exit")
        if exit_item is None:
            raise ValueError("%s: a teleporter needs 'exit': (x, y) or a placement dict" % where)
        exit_item = {"at": tuple(exit_item)} if isinstance(exit_item, (tuple, list)) else dict(exit_item)
        if "exit_heading" in item:
            exit_item["heading"] = item["exit_heading"]
        ex, ey, ez, eh = locate(self.track, self.floor, exit_item, where + " exit")
        build.update(radius=radius, exit=(ex, ey, ez), exit_heading=eh)
        spec.update({"position": rounded(runtime((build["x"], build["y"], build["z"]))), "radius": round(radius, 5),
                     "exit": rounded(runtime((ex, ey, ez + BALL_RADIUS + 0.01))), "direction": _direction(eh),
                     "minSpeed": round(_number(item, "min_speed", MIN_SPEED, 0.0, 10.0, where), 4)})
        self.reserved.extend([(build["x"], build["y"], radius + 0.5), (ex, ey, 0.9)])

    def _cannon(self, item, spec, build, where):
        radius = _number(item, "radius", CANNON_RADIUS, 0.25, 0.8, where)
        flight = _number(item, "flight", cannon_flight.FLIGHT, 0.8, 2.5, where)
        delay = _number(item, "delay", cannon_flight.DELAY, 0.0, 3.0, where)
        barrel = _number(item, "barrel", cannon_flight.BARREL, 1.0, 3.0, where)
        breech = _number(item, "breech", cannon_flight.BREECH_HEIGHT, 0.5, 1.2, where)
        target = item.get("target")
        if isinstance(target, dict):
            target = locate(self.track, self.floor, target, where + " target")[:3]
        if not isinstance(target, (tuple, list)) or len(target) not in (2, 3):
            raise ValueError("%s: a cannon needs 'target': (x, y), (x, y, z) or a placement dict, the spot where the ball lands" % where)
        tz = float(target[2]) if len(target) == 3 else self.floor.height(target[0], target[1])
        if tz is None:
            raise ValueError("%s: the cannon target (%.2f, %.2f) is not on the floor; give (x, y, z) to aim at the water" % (where, target[0], target[1]))
        goal = (float(target[0]), float(target[1]), tz + BALL_RADIUS)
        sol = cannon_flight.solve((build["x"], build["y"], build["z"]), goal, self.track.gravity, flight, barrel, breech, where)
        build.update(radius=radius, solution=sol, target=goal, flight=flight, barrel=barrel)
        spec.update({"position": rounded(runtime((build["x"], build["y"], build["z"]))), "radius": round(radius, 5), "muzzle": sol["muzzle_rt"],
                     "velocity": sol["velocity"], "delay": round(delay, 4), "target": rounded(runtime(goal)), "flight": round(flight, 4)})
        self.flights.append((build, spec))
        self.reserved.append((build["x"], build["y"], 1.6))
        steps = max(2, int(math.hypot(goal[0] - build["x"], goal[1] - build["y"])))
        self.reserved.extend((build["x"] + (goal[0] - build["x"]) * k / steps, build["y"] + (goal[1] - build["y"]) * k / steps, 1.0) for k in range(1, steps + 1))

    # -- obstacles ------------------------------------------------------------------------------------------------

    def add_obstacle(self, item, where):
        built = obstacle_data.resolve(self.world, item, locate(self.track, self.floor, item, where), where)
        self.obstacles.append(built["runtime"])
        self.obstacle_items.append(built)
        frame = built["frame"]
        x, y, _ = built["pose"]["location"]
        self.reserved.append((x, y, 0.9))
        if frame:
            self.reserved.extend((px, py, 0.6) for px, py in frame["posts"])

    # -- checks after the dressing ----------------------------------------------------------------------------------

    def obstructions(self):
        """``(spheres, cylinders)`` of everything a flight must keep clear of (Blender metres)."""
        spheres, cylinders = [], []
        for built in self.obstacle_items:
            spec = built["runtime"]
            reach = max(math.hypot(*[abs(c) for c in part["center"]]) + max(obstacle_data.part_extent(part)) for part in built["parts"])
            reach += abs(spec.get("amplitude") or 0.0) if spec["kind"] in ("slide", "boulder", "gate") else 0.0
            if built["frame"] and built["frame"]["kind"] == "door":
                reach = max(reach, built["frame"]["width"] / 2 + 0.3)
            x, y, z = built["pose"]["location"]
            spheres.append(("obstacle '%s'" % built["id"], x, y, z, reach))
        for build in self.mechanic_items:
            if build["kind"] == "bumper":
                cylinders.append(("bumper '%s'" % build["id"], build["x"], build["y"], build["z"], build["radius"], build["height"]))
        return spheres, cylinders

    def check(self, limits):
        """Positions inside ``limits`` (runtime ``min``/``max``) and every cannon flight clear of the course."""
        def inside(point):
            return all(limits["min"][i] <= point[i] <= limits["max"][i] for i in range(3))
        for spec in self.mechanics + self.obstacles:
            points = [spec["position"]] + [spec[key] for key in ("exit", "muzzle", "target") if key in spec]
            for point in points:
                if not inside(point):
                    raise ValueError("%s '%s' lies outside the course bounds (%s)" % (spec["kind"], spec["id"], point))
        if not self.flights:
            return
        tops = []
        for run in rail_runs(self.track):
            tops.append([(p[0] + u[0] * h, p[1] + u[1] * h, p[2] + u[2] * h) for p, _, u, h in run])
        spheres, cylinders = self.obstructions()
        for build, spec in self.flights:
            found = cannon_flight.problems(build["solution"], self.track.gravity, build["flight"], build["target"], self.floor, tops, spheres,
                                           cylinders, limits)
            if found:
                raise ValueError("cannon '%s': the flight is not clear: %s" % (spec["id"], "; ".join(found[:4])))

    def check_props(self, placements, catalog):
        """Every cannon flight keeps 0.5 m from the placed props (a circle of the prop's solid share of its radius)."""
        for build, spec in self.flights:
            for name, x, y, z, _, scale in placements:
                info = catalog[name]
                solid = info["radius"] * scale * (0.6 if "tall" in info["tags"] else 1.0)
                for px, py, pz, t in cannon_flight.samples(build["solution"]["muzzle_rt"], build["solution"]["velocity"], self.track.gravity, build["flight"]):
                    b = blender((px, py, pz))
                    if math.hypot(b[0] - x, b[1] - y) < solid + cannon_flight.CLEARANCE and b[2] < z + info["height"] * scale + cannon_flight.CLEARANCE:
                        raise ValueError("cannon '%s': the flight at %.2f s passes within 0.5 m of the %s at (%.1f, %.1f)" % (spec["id"], t, name, x, y))


def resolve(track):
    """Convert ``track.mechanics`` / ``track.obstacles`` (course data) to manifest entries; returns the ``Resolved`` object."""
    res = Resolved(track, FloorField(track))
    number = track.number
    seen = set()
    for label, items, add in (("mechanics", list(track.mechanics), res.add_mechanic), ("obstacles", list(track.obstacles), res.add_obstacle)):
        for index, item in enumerate(items):
            ident = item.get("id")
            where = "course %d, %s[%d] %r" % (number, label, index, ident)
            if not isinstance(ident, str) or not ID.match(ident):
                raise ValueError("%s: id must match ^[a-z][a-z0-9-]{0,31}$" % where)
            if ident in seen:
                raise ValueError("%s: the id is used twice" % where)
            seen.add(ident)
            add(item, where)
    track.mechanics, track.obstacles = res.mechanics, res.obstacles
    validate_runtime(res.mechanics, res.obstacles, track.cycle, track.gravity)
    return res


# -- the checks of ``validateMechanic`` / ``validateObstacle`` and the cycle rule, for the finished entries ----------------------

def _vec(value, size=3):
    return isinstance(value, list) and len(value) == size and all(isinstance(v, (int, float)) and math.isfinite(v) for v in value)


def _unit(value):
    return _vec(value, 2) and abs(math.hypot(*value) - 1) <= 0.01


def _positive(value):
    return isinstance(value, (int, float)) and math.isfinite(value) and value > 0


def _part_ok(part):
    shape = part.get("shape", "box")
    if "position" in part and not _vec(part["position"]) or "rotation" in part and not _vec(part["rotation"], 4):
        return False
    if shape == "box":
        return _vec(part.get("halfExtents")) and all(v > 0 for v in part["halfExtents"])
    return _positive(part.get("radius")) and (shape == "ball" or (shape == "cylinder" and _positive(part.get("halfHeight"))))


def validate_runtime(mechanics, obstacles, cycle, gravity):
    """Raise ``ValueError`` for an entry that ``validateMechanic`` / ``validateObstacle`` would reject, a repeated id or a bad cycle."""
    if not (isinstance(gravity, (int, float)) and 0 < gravity <= 20):
        raise ValueError("gravity must be above 0 and at most 20")
    ids = set()
    for spec in mechanics + obstacles:
        if not isinstance(spec.get("id"), str) or not ID.match(spec["id"]) or spec["id"] in ids:
            raise ValueError("mechanic/obstacle id %r is invalid or repeated" % spec.get("id"))
        ids.add(spec["id"])
        if not _vec(spec.get("position")):
            raise ValueError("'%s' has no valid position" % spec["id"])
    for spec in mechanics:
        kind = spec["kind"]
        ok = {
            "bumper": lambda: _positive(spec.get("radius")) and _positive(spec.get("height")) and _positive(spec.get("kick")) and spec.get("surface", "stone") in ("wood", "stone", "metal"),
            "boost": lambda: _vec(spec.get("half")) and all(v > 0 for v in spec["half"]) and _unit(spec.get("direction")) and _positive(spec.get("speed")),
            "teleport": lambda: _positive(spec.get("radius")) and _vec(spec.get("exit")) and _unit(spec.get("direction")) and spec.get("minSpeed", 0) >= 0,
            "cannon": lambda: _positive(spec.get("radius")) and _vec(spec.get("muzzle")) and _vec(spec.get("velocity")) and spec.get("delay", 0) >= 0,
        }.get(kind)
        if ok is None or not ok():
            raise ValueError("mechanic '%s' (%s) is not valid for the runtime" % (spec["id"], kind))
    for spec in obstacles:
        axis = spec.get("axis", [0, 1, 0])
        kind = spec["kind"]
        ok = (kind in ("rotate", "slide", "spin", "gate", "boulder") and _vec(axis) and math.hypot(*axis) > 1e-6
              and isinstance(spec.get("speed"), (int, float)) and math.isfinite(spec["speed"])
              and (kind == "spin" or isinstance(spec.get("amplitude"), (int, float)))
              and ("rotation" not in spec or _vec(spec["rotation"], 4)) and 1 <= spec.get("sharp", 1) <= 4
              and (kind != "boulder" or (_positive(spec.get("radius")) and abs(axis[1]) < 1e-6 * math.hypot(*axis)))
              and spec.get("colliders") and all(_part_ok(part) for part in spec["colliders"]))
        if not ok:
            raise ValueError("obstacle '%s' (%s) is not valid for the runtime" % (spec["id"], kind))
        if not _positive(cycle):
            raise ValueError("a course with obstacles needs a 'cycle' time (seconds)")
        turns = cycle * spec["speed"] / (2 * math.pi)
        if abs(turns - round(turns)) > 1e-4 or round(turns) == 0:
            raise ValueError("obstacle '%s': cycle %g s x speed %g rad/s is %.4f turns, it must be a whole number" % (spec["id"], cycle, spec["speed"], turns))
