"""Deterministic prop placement of a dressed course (MIT License, see ../LICENSE).

``place`` returns ``[(name, x, y, z, yaw, scale)]`` in Blender coordinates: the fixed parts first (``fixed``,
kept where they are unless they reach over the rim), then the hero at its data position, the fixed props of the
course data, the medium props and the scatter. Everything comes from ``random.Random(1000 + number)`` and a
fixed candidate grid, so a run is byte-stable.

Rules (asset test): no footprint circle (``radius`` x scale, the kit catalog) enters the corridor around any
route centre line (the main route, fork branches and shortcuts: ``Track.corridors``) except ``span`` props,
everything lies inside the island rim with a margin (``rim_share``: how much of its radius a prop needs inside the
rim, the rest may overhang), and the instance count stays below ``LIMIT``. The corridor here is a little wider
than the test's 1.0 m so the rail trims and their posts stay clear. Wide pads (plaza, green, funnel) are covered
too: free placements also keep ``FLOOR_GAP`` from every floor vertex. The hero stands where the course data puts it
at scale 1.0; it shrinks in 0.05 steps down to 0.9 when the rim, a corridor or the floor demand it and the build
fails otherwise. ``check`` applies every rule to every placement, fixed ones too.
"""

import math
import random

from . import placement_plan

CORRIDOR = 1.2
HERO_CORRIDOR = 1.05      # the hero circle is mostly thin parts: it may come a little closer than free props
EDGE = 0.3
GRID = 0.25
LIMIT = 220
HERO_SCALES = (1.0, 0.95, 0.9)
FLOOR_GAP = 0.45          # free props keep this far from every floor vertex (the lane edge is 0.86 m from the centre line)
HERO_FLOOR_GAP = 0.25
FOOT_GAP = 0.04
CANOPY = 0.5  # share of a tall prop's radius that is solid for other props (the rest is overhang)
HERO_SOLID = 0.6  # the same for a hero (a long thin hull or a head fills little of its enclosing circle)
VIEW = (-0.58, 0.81)  # horizontal viewing direction of the game camera in Blender XY
SHADOW = 1.35  # 1 / tan(37 degrees): ground covered behind a prop per metre of its height
DEFAULT_SINK = 0.03
TOLERANCE = 0.005  # metres: slack of the rim check for scales rounded to millimetres
SETTLE_STEP, SETTLE_STEPS = 0.05, 40  # a fixed special that reaches over the rim moves inwards in these steps (metres)

# front direction of the kit props per look (+1: front towards +Y, -1: towards -Y)
FRONT = {"pirate": 1.0, "temple": -1.0, "moon": -1.0}
# how a prop is turned: "lane" = front towards the nearest lane point, "camera" = front towards the viewer,
# "along" = front along the lane direction (to the cup); everything else gets a random yaw
FACING = {"chest": "lane", "cannon": "lane", "totem": "lane", "idol": "lane", "waterfall": "camera", "solar": "camera",
          "dishbase": "camera", "dishhead": "camera", "habitat": "lane", "ship": "along", "gate": "along", "head": "mix",
          "rover": "lane", "skullrock": "camera"}
YAW_NOISE = {"chest": 0.2, "cannon": 0.2, "totem": 0.25, "idol": 0.25, "waterfall": 0.15, "solar": 0.2, "dishbase": 0.3,
             "dishhead": 0.3, "habitat": 0.05, "ship": 0.1, "rover": 0.4, "skullrock": 0.3}
# scatter that lives on the beach: distance to the island rim in metres
SHORE = {"seaweed": (0.3, 1.0), "shell": (0.3, 1.6), "starfish": (0.3, 1.3), "crab": (0.3, 1.8)}
# share of the radius that is solid for other props / needs room at the island rim (leaves overhang, thin props do not)
SOLID = {"fern": 0.38, "bush": 0.4, "tuft": 0.5, "seaweed": 0.5, "pebble": 0.6, "crate": 0.7, "crater": 0.5, "liana": 0.6,
         "waterfall": 0.8, "chest": 0.7, "cannon": 0.7}
SPREAD = 2.2  # centre distance between two medium props of the same kind
HERO_RIM = 0.85  # a hero's rigging or brow may overhang the rim a little
TALL_RIM = 0.6  # fronds and tops overhang, the foot stays on the island
RIM = {"fern": 0.5, "bush": 0.5, "tuft": 0.5, "seaweed": 0.4, "shell": 0.6, "crab": 0.6, "starfish": 0.6}
EXTRA_SINK = {"boulder": 0.1, "rock": 0.05, "crystal": 0.03, "islet": 0.1}

FOOTPRINT = {"gate": ((-1.9, 0.0, 1.1), (1.9, 0.0, 1.1))}  # (x, y, radius) circles in prop space

PLANS = {
    "pirate": {"hero": "ship",
               "medium": [("palm", 3), ("palm2", 1), ("barrel", 3), ("chest", 1), ("cannon", 1), ("rock", 3), ("post", 2),
                          ("skullrock", 1), ("crab", 2)],
               "scatter": [("tuft", 24), ("shell", 14), ("starfish", 6), ("seaweed", 8), ("footprint", 6), ("xmark", 1)]},
    "temple": {"hero": "head",
               "medium": [("totem", 2), ("pillar", 2), ("torch", 4), ("liana", 3), ("rock", 3), ("idol", 1), ("waterfall", 1)],
               "scatter": [("fern", 18), ("bush", 6), ("tile", 12), ("pebble", 14)]},
    "moon": {"hero": "habitat",
             "medium": [("dish", 1), ("rover", 1), ("solar", 2), ("crate", 3), ("boulder", 4), ("neonpost", 4),
                        ("crystal", 3), ("antenna", 2)],
             "scatter": [("crater", 6), ("pebble", 18)]},
}


class Field:
    """Distances to the route centre lines, the floor and the island rim, cached per query point.

    ``corridors`` are the centre polylines of every route (``Track.corridors``, points ``(x, y, ...)``);
    ``floor`` the plan points of all floor vertices; ``trench(x, y, margin)`` is True where a gap or a pond cuts
    the island. Segments and floor points are hashed in cells of ``HASH`` m.
    """

    HASH = 2.0

    def __init__(self, corridors, outline, trench=None, floor=()):
        self.corridors = corridors
        self.outline = outline
        self.trench = trench
        self.segments = []
        self.cells = {}
        for line in corridors:
            for a, b in zip(line, line[1:]):
                if math.hypot(b[0] - a[0], b[1] - a[1]) < 1e-9:
                    continue
                k = len(self.segments)
                self.segments.append((a[0], a[1], b[0], b[1]))
                for i in range(math.floor(min(a[0], b[0]) / self.HASH), math.floor(max(a[0], b[0]) / self.HASH) + 1):
                    for j in range(math.floor(min(a[1], b[1]) / self.HASH), math.floor(max(a[1], b[1]) / self.HASH) + 1):
                        self.cells.setdefault((i, j), []).append(k)
        self.floor_cells = {}
        for x, y in floor:
            self.floor_cells.setdefault((math.floor(x / self.HASH), math.floor(y / self.HASH)), []).append((x, y))
        self._lane = {}
        self._floor = {}

    @staticmethod
    def _measure(x, y, segment):
        ax, ay, bx, by = segment
        vx, vy = bx - ax, by - ay
        l2 = vx * vx + vy * vy
        t = max(0.0, min(1.0, ((x - ax) * vx + (y - ay) * vy) / l2))
        px, py = ax + t * vx, ay + t * vy
        ln = math.sqrt(l2)
        return (math.hypot(x - px, y - py), px, py, vx / ln, vy / ln)

    def lane(self, x, y):
        """``(distance, nearest x, nearest y, unit tangent x, unit tangent y)`` of the nearest route centre line."""
        key = (round(x, 4), round(y, 4))
        hit = self._lane.get(key)
        if hit is not None:
            return hit
        ci, cj = math.floor(x / self.HASH), math.floor(y / self.HASH)
        best = None
        for reach in (1, 3):                       # a window of cells; its guaranteed exact radius is reach * HASH
            seen = set()
            for i in range(ci - reach, ci + reach + 1):
                for j in range(cj - reach, cj + reach + 1):
                    for k in self.cells.get((i, j), ()):
                        if k not in seen:
                            seen.add(k)
                            m = self._measure(x, y, self.segments[k])
                            if best is None or m[0] < best[0]:
                                best = m
            if best is not None and best[0] <= reach * self.HASH:
                break
        else:
            for segment in self.segments:
                m = self._measure(x, y, segment)
                if best is None or m[0] < best[0]:
                    best = m
        self._lane[key] = best
        return best

    def floor_distance(self, x, y):
        """Plan distance to the nearest floor vertex (large when none is within 6 m)."""
        key = (round(x, 4), round(y, 4))
        hit = self._floor.get(key)
        if hit is not None:
            return hit
        ci, cj = math.floor(x / self.HASH), math.floor(y / self.HASH)
        best = 99.0
        for i in range(ci - 3, ci + 4):
            for j in range(cj - 3, cj + 4):
                for px, py in self.floor_cells.get((i, j), ()):
                    best = min(best, math.hypot(x - px, y - py))
        self._floor[key] = best
        return best

    def edge(self, x, y):
        """Distance to the island rim (positive inside)."""
        return -self.outline.contains_signed(x, y)

    def dry(self, x, y, radius):
        """False where a footprint of ``radius`` would reach into a trench or a pond."""
        return not (self.trench and self.trench(x, y, radius))

    def clearance(self, x, y, rim=1.0):
        """Largest footprint radius that keeps the corridors and the floor free and ``rim`` times the radius inside the rim."""
        return min(self.lane(x, y)[0] - CORRIDOR, self.floor_distance(x, y) - FLOOR_GAP, (self.edge(x, y) - EDGE) / rim)


def rim_share(name, catalog):
    """Share of a prop's footprint radius that must lie inside the island rim (the rest may overhang)."""
    tags = catalog[name]["tags"]
    return RIM.get(name, TALL_RIM if "tall" in tags else HERO_RIM if "hero" in tags else 1.0)


def yaw_for(front, direction):
    """Yaw (radians, about +Z) that turns a prop whose front is ``front`` (+1/-1 along Y) towards ``direction``."""
    dx, dy = direction
    return math.atan2(-dx, dy) if front > 0 else math.atan2(dx, -dy)


def _facing(name, mode, x, y, front, field, rng, cup):
    near = field.lane(x, y)
    if mode == "lane":
        yaw = yaw_for(front, (near[1] - x, near[2] - y))
    elif mode == "along":
        yaw = yaw_for(front, (near[3], near[4]))
    elif mode == "camera":
        yaw = yaw_for(front, (-VIEW[0], -VIEW[1]))
    elif mode == "mix":
        to_cup = (cup[0] - x, cup[1] - y)
        ln = max(1e-6, math.hypot(*to_cup))
        yaw = yaw_for(front, (0.4 * to_cup[0] / ln - 0.6 * VIEW[0], 0.4 * to_cup[1] / ln - 0.6 * VIEW[1]))
    else:
        return rng.uniform(0.0, math.tau)
    return yaw + rng.uniform(-1.0, 1.0) * YAW_NOISE.get(name, 0.2)


def _ground(name, info, x, y, scale, top_height):
    """Base height: flat props ride the highest ground under them, upright props sink a little into the lowest."""
    flat = "flat" in info["tags"]
    reach = min(0.3, 0.25 * info["radius"]) * scale
    samples = [top_height(x, y)] + [top_height(x + math.cos(a) * reach, y + math.sin(a) * reach) for a in (0.0, 1.57, 3.14, 4.71)]
    if flat:
        return max(samples) - 0.004
    return min(samples) - DEFAULT_SINK - EXTRA_SINK.get(name, 0.0) * scale


def _blocks_view(field, x, y, height, radius):
    """True when the prop would hide the lane from the game camera.

    The camera looks down at 37 degrees, so a prop of ``height`` covers the ground behind it (along ``VIEW``) for
    ``SHADOW`` x height, and its crown, ``radius`` wide, reaches half a radius further and as far to each side.
    """
    reach = SHADOW * height + 0.5 * radius
    for k in (0.2, 0.4, 0.6, 0.8, 1.0):
        if field.lane(x + VIEW[0] * reach * k, y + VIEW[1] * reach * k)[0] < 0.4 + 0.6 * radius:
            return True
    return False


def hero_conflict(field, catalog, name, x, y, scale):
    """Why a hero at ``(x, y)`` and ``scale`` does not fit (rim, corridor, floor, water), or None."""
    info = catalog[name]
    r = info["radius"] * scale
    need = EDGE + rim_share(name, catalog) * r
    if field.edge(x, y) < need - TOLERANCE:
        return "it reaches %.2f m over the island rim" % (need - field.edge(x, y))
    if field.lane(x, y)[0] - r < HERO_CORRIDOR:
        return "it comes within %.2f m of a route centre line (needs %.2f m)" % (field.lane(x, y)[0] - r, HERO_CORRIDOR)
    if field.floor_distance(x, y) - r < HERO_FLOOR_GAP:
        return "it comes within %.2f m of the floor (needs %.2f m)" % (field.floor_distance(x, y) - r, HERO_FLOOR_GAP)
    if not field.dry(x, y, 0.7 * r):
        return "it stands in a pond or a trench"
    return None


def place(look, number, field, top_height, catalog, plan, fixed=(), reserved=(), cup=(0.0, 0.0), hero=None, props=()):
    """Placements ``[(name, x, y, z, yaw, scale)]``; see the module docstring.

    ``fixed`` are ``(name, x, y, yaw, scale)`` parts that are kept first (moved inwards only as far as the rim
    rule needs); ``reserved`` are ``(x, y, radius)`` circles taken by other geometry; ``hero`` is
    ``{"name", "at": (x, y), "yaw": radians}`` or None (no hero); ``props`` are the course data's fixed props
    ``(name, x, y, yaw, scale)``; ``plan`` holds the ``medium`` and ``scatter`` lists of ``(name, count)`` (a
    medium prop that is a fixed part or a fixed prop is not placed again).
    """
    rng = random.Random(1000 + number)
    plan = placement_plan.scaled(plan, field.outline.half)
    front = FRONT[look]
    cx, cy = field.outline.center.x, field.outline.center.y
    hx, hy = field.outline.half
    x0, x1 = math.floor((cx - 1.1 * hx) / GRID), math.ceil((cx + 1.1 * hx) / GRID)
    y0, y1 = math.floor((cy - 1.1 * hy) / GRID), math.ceil((cy + 1.1 * hy) / GRID)
    cells = [(x * GRID, y * GRID) for y in range(y0, y1 + 1) for x in range(x0, x1 + 1)]
    cells = [c for c in cells if field.edge(*c) > EDGE]
    out, occupied, bubbles = [], [(x, y, r) for x, y, r in reserved], []

    def rim(name):
        return rim_share(name, catalog)

    def solid(name, scale):
        tags = catalog[name]["tags"]
        share = SOLID.get(name, CANOPY if "tall" in tags else HERO_SOLID if "hero" in tags else 1.0)
        return catalog[name]["radius"] * scale * share

    def reach(name):
        """Footprint radius that must clear the corridor and the rim: the dish head is wider than its base."""
        r = catalog[name]["radius"]
        return max(r, catalog["dishhead"]["radius"]) if name == "dishbase" and "dishhead" in catalog else r

    def overlaps(x, y, r, gap, soft=False):
        return any(math.hypot(x - ox, y - oy) < r + orad + gap for ox, oy, orad in (occupied + bubbles if soft else occupied))

    def add(name, x, y, yaw, scale):
        info = catalog[name]
        out.append((name, x, y, _ground(name, info, x, y, scale, top_height), yaw, scale))
        occupied.append((x, y, solid(name, scale)))
        if name == "dishbase":  # the head turns on its own: same transform, no second footprint
            out.append(("dishhead", x, y, out[-1][3], yaw, scale))

    def settle(name, x, y, scale):
        """A fixed part keeps its spot unless that reaches over the rim: pull it towards the island centre."""
        need = EDGE + rim(name) * catalog[name]["radius"] * scale
        for _ in range(SETTLE_STEPS):
            d = math.hypot(cx - x, cy - y)
            if field.edge(x, y) >= need or d < SETTLE_STEP:
                break
            x, y = x + (cx - x) / d * SETTLE_STEP, y + (cy - y) / d * SETTLE_STEP
        return x, y

    def add_fixed(name, x, y, yaw, scale):
        if name not in catalog:
            raise ValueError("unknown prop %r in the %s kit" % (name, look))
        x, y = settle(name, x, y, scale)
        add(name, x, y, yaw, scale)
        if name in FOOTPRINT:  # a wide span prop blocks its two legs, not a circle around the lane
            occupied.pop()
            for dx, dy, r in FOOTPRINT[name]:
                occupied.append((x + (dx * math.cos(yaw) - dy * math.sin(yaw)) * scale, y + (dx * math.sin(yaw) + dy * math.cos(yaw)) * scale, r * scale))

    for name, x, y, yaw, scale in fixed:
        add_fixed(name, x, y, yaw, scale)

    hero_front = None
    if hero:
        name, (x, y), yaw = hero["name"], hero["at"], hero["yaw"]
        if name not in catalog:
            raise ValueError("unknown hero prop %r in the %s kit" % (name, look))
        why = None
        for scale in HERO_SCALES:
            why = hero_conflict(field, catalog, name, x, y, scale)
            if why is None:
                break
        else:
            raise ValueError("the hero %s at (%.2f, %.2f) does not fit even at scale %.2f: %s" % (name, x, y, HERO_SCALES[-1], why))
        add(name, x, y, yaw, scale)
        r = catalog[name]["radius"]
        bubbles.append((x, y, r * scale * 0.95 - occupied[-1][2]))  # medium props keep clear of the whole hero circle
        hero_front = lambda px, py, hx=x, hy=y, hr=r * scale: abs(px - hx) < 0.55 * hr + 0.4 and hy - 0.9 * hr - 0.8 < py < hy
    for name, x, y, yaw, scale in props:
        add_fixed(name, x, y, yaw, scale)

    skip = {name for name, *_ in fixed} | {name for name, *_ in props}
    entries = [("dishbase" if name == "dish" else name, count) for name, count in plan["medium"]]  # the dish is two props, one transform
    for key, count in sorted(entries, key=lambda e: -catalog[e[0]]["radius"]):
        if key in skip:
            continue
        info = catalog[key]
        r = info["radius"]
        tall = "tall" in info["tags"]
        placed = 0
        order = cells[:]
        rng.shuffle(order)
        for x, y in order:
            if placed >= count:
                break
            base = round(rng.uniform(0.85, 1.15), 3)
            room = field.clearance(x, y, rim(key))
            scale = round(min(base, room / reach(key)), 3) if r else base
            if scale < 0.6 * base or overlaps(x, y, solid(key, scale), FOOT_GAP, soft=True) or not field.dry(x, y, 0.7 * r * scale):
                continue
            if hero_front and info["height"] * scale > 0.9 and hero_front(x, y):
                continue
            if any(o[0] == key and math.hypot(o[1] - x, o[2] - y) < SPREAD for o in out):
                continue
            if tall and info["height"] * scale > 0.9 and _blocks_view(field, x, y, info["height"] * scale, r * scale):
                continue
            add(key, x, y, _facing(key, FACING.get(key, "none"), x, y, front, field, rng, cup), scale)
            placed += 1

    for name, count in plan["scatter"]:
        info = catalog[name]
        r = info["radius"]
        band = SHORE.get(name)
        order = cells[:]
        rng.shuffle(order)
        placed = 0
        for x, y in order:
            if placed >= count or len(out) >= LIMIT:
                break
            scale = round(rng.uniform(0.85, 1.15), 3)
            if band and not band[0] <= field.edge(x, y) <= band[1]:
                continue
            lane_room = field.lane(x, y)[0] - 0.97 - r * scale  # the asset test: 1.0 + radius * scale - 0.05 (margin of 0.02)
            floor_room = field.floor_distance(x, y) - FLOOR_GAP - r * scale
            rim_room = field.edge(x, y) - EDGE - rim(name) * r * scale
            if lane_room < 0.0 or floor_room < 0.0 or rim_room < 0.0 or field.lane(x, y)[0] < CORRIDOR or overlaps(x, y, solid(name, scale), 0.02):
                continue
            if not field.dry(x, y, 0.5 * r * scale):
                continue
            add(name, x, y, rng.uniform(0.0, math.tau), scale)
            placed += 1
    return out[:LIMIT]


def check(placements, field, catalog):
    """Raise when a placement breaks a rule of the asset test (instance limit, corridors, island rim, hero size).

    Every placement is checked, the fixed ones too: the footprint keeps clear of every corridor and the centre
    keeps ``EDGE`` plus its ``rim_share`` of the footprint radius inside the island rim (``TOLERANCE`` covers the
    rounding of the scale to millimetres); a hero is at least 0.9 times its modelled size.
    """
    if not placements or len(placements) > LIMIT:
        raise RuntimeError("%d placements (limit %d)" % (len(placements), LIMIT))
    for name, x, y, z, yaw, scale in placements:
        info = catalog[name]
        if "hero" in info["tags"] and scale < HERO_SCALES[-1] - 1e-9:
            raise RuntimeError("the hero %s is scaled to %.2f (minimum %.2f)" % (name, scale, HERO_SCALES[-1]))
        if "span" not in info["tags"] and field.lane(x, y)[0] < 1.0 + info["radius"] * scale - 0.05:
            raise RuntimeError("%s at (%.2f, %.2f) scale %.2f blocks the lane corridor" % (name, x, y, scale))
        need = EDGE + rim_share(name, catalog) * info["radius"] * scale
        if field.edge(x, y) < need - TOLERANCE:
            raise RuntimeError("%s at (%.2f, %.2f) scale %.2f is %.2f m inside the island rim (needs %.2f)"
                               % (name, x, y, scale, field.edge(x, y), need))
