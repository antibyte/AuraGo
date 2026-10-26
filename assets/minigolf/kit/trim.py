"""Visible rail trims of a dressed course (MIT License, see ../LICENSE).

The collision rails (``rail-<surface>``) stay in the GLB (and are hidden at runtime). ``rail-trim`` is decoration
with a world look, built from the same rail runs ``rails.rail_runs`` (``(inner point, outward, up, height)``)
that build the collision rails: the inner face lies exactly in the plane of the collision rail, the top is
rounded and stands at most 2 cm above it, the outside at most 3 cm out. The trim follows ``up`` (loop, half-pipe
lips) and the wall height of each run vertex. One node holds the trims of all runs; the seeds are
``7000 + 2 * number + side`` (side 0 = left of the route, 1 = right).
"""

import math
import random

from mathutils import Vector, noise

from . import colors, materials, palette, shapes
from .island import smoothstep
from .palette import mix
from .prop import Prop
from .track import WALL_HEIGHT

INNER_BOTTOM, OUTER_BOTTOM = -0.02, -0.07
SKIRT_DEPTH = -0.5  # deepest reach of the outer face below the lane edge where the ground falls away
SKIRT_FADE = 0.4    # where the ground is further down than that the skirt fades back to the plain trim over this much
ARC_RISE = 0.045
WIDTH = 0.17
TOP_BELOW = 0.025   # the flat top of the profile lies this far under the rail top
POST_SIZE, POST_SPACING = (0.07, 0.07, 0.32), 1.2
CLAMP_SIZE, CLAMP_SPACING = (0.21, 0.06, 0.30), 1.5
NEON_HALF = 0.01
JOINT = {"pirate": 0.9, "temple": 0.6, "moon": 0.9}
UPRIGHT = 0.95      # posts, clamps and the skirt only where the wall stands upright (z of ``up``)
SIMPLIFY_POSITION, SIMPLIFY_HEIGHT, SIMPLIFY_ANGLE = 0.004, 0.01, math.radians(4.0)   # what thinning a run may change


def trim_profile(height=WALL_HEIGHT, steps=4):
    """(across, up) offsets from the inner rail edge: flush inner face, rounded top <= +2 cm over a rail of ``height``.

    The bottom reaches under the island bed so no slit shows, the inner face starts just under the lane edge.
    """
    top = height - TOP_BELOW
    pts = [(0.0, INNER_BOTTOM), (0.0, top)]
    for k in range(1, steps):
        a = math.pi - k * math.pi / steps
        pts.append((WIDTH / 2 + WIDTH / 2 * math.cos(a), top + ARC_RISE * math.sin(a)))
    pts += [(WIDTH, top), (WIDTH, OUTER_BOTTOM)]
    return pts


def _arclength(run):
    arc = [0.0]
    for i in range(1, len(run)):
        arc.append(arc[-1] + math.dist(run[i - 1][0], run[i][0]))
    return arc


def _lerp3(a, b, u):
    return tuple(a[k] + (b[k] - a[k]) * u for k in range(3))


def _unit3(v):
    n = math.sqrt(v[0] * v[0] + v[1] * v[1] + v[2] * v[2])
    return (v[0] / n, v[1] / n, v[2] / n) if n > 1e-12 else (0.0, 0.0, 1.0)


def _at(run, arc, s):
    """Interpolated ``(point, outward, up, height)`` of the run at arc length ``s``."""
    i = max(0, min(len(arc) - 2, next((k for k in range(len(arc) - 1) if arc[k + 1] >= s), len(arc) - 2)))
    u = max(0.0, min(1.0, (s - arc[i]) / max(1e-9, arc[i + 1] - arc[i])))
    (pa, oa, ua, ha), (pb, ob, ub, hb) = run[i], run[i + 1]
    return _lerp3(pa, pb, u), _unit3(_lerp3(oa, ob, u)), _unit3(_lerp3(ua, ub, u)), ha + (hb - ha) * u


def simplify_run(run):
    """The run without vertices the trim does not need (Douglas-Peucker on position, wall height, outward and up).

    A straight pad edge with a ring every 0.14 m or a smooth bend keeps few vertices; the trim stays within 4 mm of
    the rail (the collision rails keep every vertex).
    """
    n = len(run)
    if n <= 2:
        return run
    arc = _arclength(run)
    keep = {0, n - 1}
    stack = [(0, n - 1)]
    while stack:
        i, j = stack.pop()
        span = arc[j] - arc[i]
        worst, at = 1.0, None
        for k in range(i + 1, j):
            u = (arc[k] - arc[i]) / span if span > 1e-9 else 0.5
            point, out, up, height = _lerp3(run[i][0], run[j][0], u), _unit3(_lerp3(run[i][1], run[j][1], u)), _unit3(_lerp3(run[i][2], run[j][2], u)),                 run[i][3] + (run[j][3] - run[i][3]) * u
            error = max(math.dist(point, run[k][0]) / SIMPLIFY_POSITION, abs(height - run[k][3]) / SIMPLIFY_HEIGHT,
                        _angle(out, run[k][1]) / SIMPLIFY_ANGLE, _angle(up, run[k][2]) / SIMPLIFY_ANGLE)
            if error > worst:
                worst, at = error, k
        if at is not None:
            keep.add(at)
            stack.extend(((i, at), (at, j)))
    return [run[k] for k in sorted(keep)]


def _angle(a, b):
    return math.acos(max(-1.0, min(1.0, a[0] * b[0] + a[1] * b[1] + a[2] * b[2])))


def run_side(run, corridors):
    """0 when the middle of the run lies left of the nearest route centre line, else 1."""
    p = run[len(run) // 2][0]
    best, side = math.inf, 0
    for line in corridors:
        for a, b in zip(line, line[1:]):
            vx, vy = b[0] - a[0], b[1] - a[1]
            l2 = vx * vx + vy * vy
            if l2 < 1e-12:
                continue
            t = max(0.0, min(1.0, ((p[0] - a[0]) * vx + (p[1] - a[1]) * vy) / l2))
            d = math.hypot(p[0] - a[0] - t * vx, p[1] - a[1] - t * vy)
            if d < best:
                best, side = d, 0 if vx * (p[1] - a[1]) - vy * (p[0] - a[0]) >= 0 else 1
    return side


def _colour_fn(look, pal, rng):
    """Vertex colour of the trim body: ``colour(arc, across, up, top)`` for a ring at route distance ``arc``."""
    off = Vector((rng.uniform(-30, 30), rng.uniform(-30, 30), rng.uniform(-30, 30)))
    block = JOINT[look]

    def joint_weight(arc, width):
        fr = (arc / block) % 1.0
        return 1.0 - smoothstep(min(fr, 1.0 - fr) * block, 0.0, width)

    def level(up, top):
        return max(0.0, min(1.0, (up - OUTER_BOTTOM) / (top + ARC_RISE - OUTER_BOTTOM)))

    if look == "pirate":
        lo, hi, dark = pal.c("wood_dark"), pal.c("wood_light"), pal.c("wood_deep")

        def colour(arc, across, up, top):
            c = mix(lo, hi, level(up, top) ** 0.9)
            plank = int(arc / block)
            k = 1.0 + 0.07 * (((plank * 37) % 7) / 3.5 - 1.0)
            k *= 1.0 + 0.08 * noise.noise(Vector((arc * 2.2, up * 14.0, across * 9.0)) + off)
            c = mix(c, dark, 0.55 * joint_weight(arc, 0.08))
            return (c[0] * k, c[1] * k, c[2] * k, 1.0)
        return colour, "matte"
    if look == "temple":
        lo, mid, hi = pal.c("stone_dark"), pal.c("stone"), pal.c("stone_light")
        seam, moss, moss_l = pal.c("stone_deep"), pal.c("moss"), pal.c("moss_light")

        def colour(arc, across, up, top):
            tz = level(up, top)
            c = mix(lo, mid, tz / 0.6) if tz < 0.6 else mix(mid, hi, (tz - 0.6) / 0.4)
            k = 1.0 + 0.10 * noise.noise(Vector((arc * 1.7, up * 10.0, across * 8.0)) + off)
            c = (c[0] * k, c[1] * k, c[2] * k, 1.0)
            c = mix(c, seam, 0.8 * joint_weight(arc, 0.07))
            if up > top - 0.01:
                m = 0.75 * smoothstep(up, top - 0.01, top + 0.02)
                c = mix(c, mix(moss, moss_l, 0.5 + 0.5 * noise.noise(Vector((arc * 3.0, across * 20.0, 4.0)) + off)), m)
            return c
        return colour, "matte"
    lo, hi, dark = pal.c("panel_grey"), pal.c("panel_white"), pal.c("metal_dark")

    def colour(arc, across, up, top):
        c = mix(lo, hi, level(up, top) ** 0.8)
        return mix(c, dark, 0.7 * joint_weight(arc, 0.05))
    return colour, "satin"


def _ring_profile(entry, ground, facets=4):
    point, out, up, height = entry
    profile = trim_profile(height, facets)
    if ground is not None and up[2] > UPRIGHT:
        low = ground(point[0] + out[0] * WIDTH, point[1] + out[1] * WIDTH) - point[2] - 0.02
        depth = max(SKIRT_DEPTH, min(OUTER_BOTTOM, low))
        if low < SKIRT_DEPTH:          # the ground is out of reach (bridge, ramp end): the skirt shrinks so supports and beams stay visible
            depth = SKIRT_DEPTH + (OUTER_BOTTOM - SKIRT_DEPTH) * smoothstep(SKIRT_DEPTH - low, 0.0, SKIRT_FADE)
        profile[-1] = (WIDTH, depth)
    return profile


def _neon_strip(name, run):
    verts, faces = [], []
    for point, out, up, height in run:
        lift = height - TOP_BELOW + ARC_RISE + 0.004
        for across in (WIDTH / 2 - NEON_HALF, WIDTH / 2 + NEON_HALF):
            verts.append(tuple(point[k] + out[k] * across + up[k] * lift for k in range(3)))
    for i in range(len(run) - 1):
        a = 2 * i
        faces.append((a, a + 1, a + 3, a + 2))
    a, b, c = (verts[k] for k in faces[0][:3])
    u, v = tuple(b[k] - a[k] for k in range(3)), tuple(c[k] - a[k] for k in range(3))
    n = (u[1] * v[2] - u[2] * v[1], u[2] * v[0] - u[0] * v[2], u[0] * v[1] - u[1] * v[0])
    if sum(n[k] * run[0][2][k] for k in range(3)) < 0:
        faces = [f[::-1] for f in faces]
    return shapes.make_object(name, verts, faces, smooth=False)


def _strip_custom_props(obj):
    for key in [k for k in obj.keys() if not k.startswith("aurago_")]:
        del obj[key]


def build_trims(look, runs, number, corridors, ground=None, facets=4):
    """Build the node ``rail-trim`` of all ``runs`` in the active collection; returns ``[object]`` (empty without runs).

    ``corridors`` decide the side of each run (left: cyan on the moon, seed ``7000 + 2 * number``); ``ground(x, y)``
    (the island height) lets the outer face reach down to the ground where it falls away, up to 0.5 m. ``facets`` is
    the number of facets of the rounded top (``detail.trim_facets``, 1-6).
    """
    if not runs:
        return []
    pal = palette.get(look)
    rngs = [random.Random(7000 + number * 2 + side) for side in (0, 1)]
    colours = [_colour_fn(look, pal, rng) for rng in rngs]
    p = Prop(look, "trim", seed=number, full_name="rail-trim")
    for index, run in enumerate(runs):
        run = simplify_run(run)
        side = run_side(run, corridors)
        colour, kind = colours[side]
        arc = _arclength(run)
        profiles = [_ring_profile(entry, ground, facets) for entry in run]
        rings = [[tuple(point[k] + out[k] * a + up[k] * z for k in range(3)) for a, z in profile]
                 for profile, (point, out, up, height) in zip(profiles, run)]
        cols = [colour(arc[i], a, z, run[i][3] - TOP_BELOW) for i, profile in enumerate(profiles) for a, z in profile]
        body = shapes.loft("rail-trim-body", rings, closed=True, cap_start=True, cap_end=True, recalc=True)
        shapes.finish(body, bevel=0, dens=0, smooth_angle=55)
        colors.paint_values(body, cols, ao=0.6)
        body.data.materials.append(materials.get(kind))
        p.parts.append(body)
        _variants(p, look, run, arc, side)
    obj = p.build()
    _strip_custom_props(obj)
    return [obj]


def _variants(p, look, run, arc, side):
    total = arc[-1]
    if look == "pirate":
        s = POST_SPACING / 2
        while s < total - 0.3:
            point, out, up, height = _at(run, arc, s)
            if up[2] > UPRIGHT:
                post = shapes.box("trim-post", POST_SIZE, base=True)
                reach = WIDTH + POST_SIZE[0] / 2 - 0.005
                p.add(post, "matte", "wood", loc=(point[0] + out[0] * reach, point[1] + out[1] * reach, point[2] - 0.04),
                      rot=(0, 0, math.degrees(math.atan2(out[1], out[0]))), bevel=0.012, segments=1, dens=0, ao=0.6)
            s += POST_SPACING
    elif look == "moon":
        tone = "cyan" if side == 0 else "magenta"
        p.add(_neon_strip("trim-neon", run), "emissive", (tone, tone), finish=False, ao=0.0, strength=1.6, jitter=0.0)
        s = CLAMP_SPACING / 2
        while s < total - 0.3:
            point, out, up, height = _at(run, arc, s)
            if up[2] > UPRIGHT:
                clamp = shapes.box("trim-clamp", CLAMP_SIZE, base=True)
                p.add(clamp, "satin", "orange", loc=(point[0] + out[0] * WIDTH / 2, point[1] + out[1] * WIDTH / 2, point[2] - 0.03),
                      rot=(0, 0, math.degrees(math.atan2(out[1], out[0]))), bevel=0.01, segments=1, dens=0, ao=0.6)
            s += CLAMP_SPACING
