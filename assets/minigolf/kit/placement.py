"""Deterministic prop placement of a dressed course (MIT License, see ../LICENSE).

``place`` returns ``[(name, x, y, z, yaw, scale)]`` in Blender coordinates: the
fixed special placements first, then the hero, the medium props and the scatter.
Everything comes from ``random.Random(1000 + number)`` and a fixed candidate
grid, so a run is byte-stable.

Rules (spec 3.4, asset test): no footprint circle (``radius`` x scale, the kit
catalog) enters the corridor around the lane centre line except ``span`` props,
everything lies inside the island rim with a margin (``rim_share``: how much of
its radius a prop needs inside the rim, the rest may overhang), and the instance
count stays below ``LIMIT``. The corridor here is a little wider than the test's
1.0 m so the rail trims and their posts stay clear. The lane polyline is exactly
the manifest ``path`` (every third sample plus the last), the one the test
measures against. ``check`` applies every rule to every placement, fixed ones too.
"""

import math
import random

CORRIDOR = 1.2
EDGE = 0.3
GRID = 0.25
LIMIT = 160
HERO_MIN_SCALE = 0.42
FOOT_GAP = 0.04
CANOPY = 0.5  # share of a tall prop's radius that is solid for other props (the rest is overhang)
HERO_SOLID = 0.6  # the same for a hero (a long thin hull or a head fills little of its enclosing circle)
HERO_MAX = {"lighthouse": 1.3}
VIEW = (-0.58, 0.81)  # horizontal viewing direction of the game camera in Blender XY
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
    """Distances to the lane polyline and the island rim, cached per query point."""

    def __init__(self, path, outline, trench=None):
        self.path = path
        self.outline = outline
        self.trench = trench  # optional ``trench(x, y, margin)``: True where a jump gap cuts the island
        self._lane = {}

    def lane(self, x, y):
        """``(distance, nearest x, nearest y, unit tangent x, unit tangent y)`` of the lane centre line."""
        key = (round(x, 4), round(y, 4))
        hit = self._lane.get(key)
        if hit is not None:
            return hit
        best = None
        for a, b in zip(self.path, self.path[1:]):
            vx, vy = b[0] - a[0], b[1] - a[1]
            l2 = max(1e-9, vx * vx + vy * vy)
            t = max(0.0, min(1.0, ((x - a[0]) * vx + (y - a[1]) * vy) / l2))
            px, py = a[0] + t * vx, a[1] + t * vy
            d = math.hypot(x - px, y - py)
            if best is None or d < best[0]:
                ln = math.sqrt(l2)
                best = (d, px, py, vx / ln, vy / ln)
        self._lane[key] = best
        return best

    def edge(self, x, y):
        """Distance to the island rim (positive inside)."""
        return -self.outline.contains_signed(x, y)

    def dry(self, x, y, radius):
        """False where a footprint of ``radius`` would reach into the trench of a jump gap."""
        return not (self.trench and self.trench(x, y, radius))

    def clearance(self, x, y, rim=1.0):
        """Largest footprint radius that keeps the lane corridor free and ``rim`` times the radius inside the island rim."""
        return min(self.lane(x, y)[0] - CORRIDOR, (self.edge(x, y) - EDGE) / rim)


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


def _blocks_view(field, x, y, height):
    """True when the prop would hide the lane from the game camera (its shadow along the view direction reaches the lane)."""
    reach = 1.2 * height
    for k in (0.35, 0.7, 1.0):
        if field.lane(x + VIEW[0] * reach * k, y + VIEW[1] * reach * k)[0] < 0.75:
            return True
    return False


def place(look, number, field, top_height, catalog, plan, fixed=(), reserved=(), cup=(0.0, 0.0), hero=None):
    """Placements ``[(name, x, y, z, yaw, scale)]``; see the module docstring.

    ``fixed`` are ``(name, x, y, yaw, scale)`` special placements that are kept
    first (moved inwards only as far as the rim rule needs); ``reserved`` are ``(x, y, radius)`` circles taken by special geometry;
    ``plan`` holds ``medium`` and ``scatter`` lists of ``(name, count)``; ``hero``
    overrides the plan's hero prop (None keeps it, False drops it).
    """
    rng = random.Random(1000 + number)
    front = FRONT[look]
    cells = [(x * GRID, y * GRID) for y in range(-26, 27) for x in range(-26, 27)]
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
        """A fixed special keeps its legacy spot unless that reaches over the rim: pull it towards the island centre."""
        need = EDGE + rim(name) * catalog[name]["radius"] * scale
        cx, cy = field.outline.center.x, field.outline.center.y
        for _ in range(SETTLE_STEPS):
            d = math.hypot(cx - x, cy - y)
            if field.edge(x, y) >= need or d < SETTLE_STEP:
                break
            x, y = x + (cx - x) / d * SETTLE_STEP, y + (cy - y) / d * SETTLE_STEP
        return x, y

    for name, x, y, yaw, scale in fixed:
        x, y = settle(name, x, y, scale)
        add(name, x, y, yaw, scale)
        if name in FOOTPRINT:  # a wide span prop blocks its two legs, not a circle around the lane
            occupied.pop()
            for dx, dy, r in FOOTPRINT[name]:
                occupied.append((x + (dx * math.cos(yaw) - dy * math.sin(yaw)) * scale, y + (dx * math.sin(yaw) + dy * math.cos(yaw)) * scale, r * scale))

    hero_name = plan.get("hero") if hero is None else (hero or None)
    if hero_name:
        info = catalog[hero_name]
        r = info["radius"]
        best = None
        for x, y in cells:
            scale = min(HERO_MAX.get(hero_name, 1.0), field.clearance(x, y, rim(hero_name)) / r)
            if scale < HERO_MIN_SCALE or overlaps(x, y, solid(hero_name, scale), FOOT_GAP) or not field.dry(x, y, 0.7 * r * scale):
                continue
            score = 2.0 * scale + 0.05 * y / 5.0 - 0.004 * math.hypot(x, y)
            if best is None or score > best[0] + 1e-9:
                best = (score, x, y, scale)
        if best:
            _, x, y, scale = best
            scale = round(scale - 0.01, 3)
            add(hero_name, x, y, _facing(hero_name, FACING.get(hero_name, "none"), x, y, front, field, rng, cup), scale)
            bubbles.append((x, y, r * scale * 0.95 - occupied[-1][2]))  # medium props keep clear of the whole hero circle

    hero_front = None
    if out and hero_name and out[-1][0] == hero_name:
        hx, hy, hr = out[-1][1], out[-1][2], catalog[hero_name]["radius"] * out[-1][5]
        hero_front = lambda x, y: abs(x - hx) < 0.55 * hr + 0.4 and hy - 0.9 * hr - 0.8 < y < hy
    skip = {name for name, *_ in fixed}
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
            if tall and info["height"] * scale > 0.9 and _blocks_view(field, x, y, info["height"] * scale):
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
            rim_room = field.edge(x, y) - EDGE - rim(name) * r * scale
            if lane_room < 0.0 or rim_room < 0.0 or field.lane(x, y)[0] < CORRIDOR or overlaps(x, y, solid(name, scale), 0.02):
                continue
            if not field.dry(x, y, 0.5 * r * scale):
                continue
            add(name, x, y, rng.uniform(0.0, math.tau), scale)
            placed += 1
    return out[:LIMIT]


def check(placements, field, catalog):
    """Raise when a placement breaks a rule of the asset test (instance limit, lane corridor, island rim).

    Every placement is checked, the fixed special placements too: the centre keeps
    ``EDGE`` plus its ``rim_share`` of the footprint radius inside the island rim
    (``TOLERANCE`` covers the rounding of the scale to millimetres).
    """
    if not placements or len(placements) > LIMIT:
        raise RuntimeError("%d placements (limit %d)" % (len(placements), LIMIT))
    for name, x, y, z, yaw, scale in placements:
        info = catalog[name]
        if "span" not in info["tags"] and field.lane(x, y)[0] < 1.0 + info["radius"] * scale - 0.05:
            raise RuntimeError("%s at (%.2f, %.2f) scale %.2f blocks the lane corridor" % (name, x, y, scale))
        need = EDGE + rim_share(name, catalog) * info["radius"] * scale
        if field.edge(x, y) < need - TOLERANCE:
            raise RuntimeError("%s at (%.2f, %.2f) scale %.2f is %.2f m inside the island rim (needs %.2f)"
                               % (name, x, y, scale, field.edge(x, y), need))
