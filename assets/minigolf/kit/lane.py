"""Probe lane section: felt, rails, cup with flag, golf ball (MIT License).

A lane is a gentle S-curve along +Y, 1.72 m wide, felt at z=0, side rails
0.14 m wide rising 0.24 m. ``style`` picks the rail dressing: ``wood`` (rounded
planks with joints), ``stone`` (carved bevelled blocks) or ``metal`` (panel
rail with a neon strip). Everything is joined into one ``lane-<world>`` mesh;
the ball is its own object.
"""

import math

from mathutils import Vector

from . import colors, materials, shapes
from .palette import mix
from .prop import Prop

LANE_WIDTH = 1.72
BALL_RADIUS = 0.16
CUP_RADIUS = 0.32
RAIL_WIDTH = 0.14
RAIL_HEIGHT = 0.24
RAIL_SINK = 0.10
TEE_MAT = 0.022


class Lane:
    """Centreline of the S-curve with sampling and distance queries.

    ``shape="wave"`` is a sine S-curve; ``shape="jog"`` is a smooth lateral jog
    plus a small wave that ends straight (``tail`` is the straight fraction at
    the far end), used where the lane has to enter a gate head-on.
    """

    def __init__(self, length=7.0, amp=0.62, origin=(0.0, 0.0), n=70, cup_t=0.86, ball_t=0.10, sign=1.0,
                 shape="wave", tail=0.0, wave=0.35):
        self.length, self.amp, self.origin, self.n = length, amp, origin, n
        self.cup_t, self.ball_t, self.sign = cup_t, ball_t, sign
        self.shape, self.tail, self.wave = shape, tail, wave
        self.pts = [self.pos(i / n) for i in range(n + 1)]

    def _x(self, t):
        if self.shape == "wave":
            return self.sign * self.amp * math.sin(2 * math.pi * t)
        u = min(1.0, t / (1.0 - self.tail))
        sm = u * u * (3 - 2 * u)
        return self.sign * (self.amp * (1 - sm) + self.wave * math.sin(2 * math.pi * u) * (1 - u) * 2)

    def pos(self, t):
        return Vector((self.origin[0] + self._x(t), self.origin[1] - self.length / 2 + self.length * t, 0.0))

    def tangent(self, t):
        e = 1e-3
        a, b = self.pos(max(0.0, t - e)), self.pos(min(1.0, t + e))
        return (b - a).normalized()

    def left(self, t):
        tg = self.tangent(t)
        return Vector((-tg.y, tg.x, 0.0))

    def yaw(self, t):
        """Rotation (degrees about Z) that turns local +Y onto the lane tangent."""
        tg = self.tangent(t)
        return math.degrees(math.atan2(tg.y, tg.x)) - 90.0

    @property
    def cup(self):
        return self.pos(self.cup_t)

    @property
    def ball(self):
        return self.pos(self.ball_t)

    def nearest(self, x, y):
        """Return ``(distance, t)`` to the centreline."""
        best, bt = 1e9, 0.0
        for i, p in enumerate(self.pts):
            d = (p.x - x) ** 2 + (p.y - y) ** 2
            if d < best:
                best, bt = d, i / self.n
        return math.sqrt(best), bt


def rail_profile(width=RAIL_WIDTH, top=RAIL_HEIGHT, sink=RAIL_SINK, arc=8):
    """Closed rounded-top section in (lateral-out, z), counter-clockwise."""
    r = width / 2.0
    pts = [(0.0, -sink), (width, -sink), (width, top - r)]
    for k in range(1, arc):
        phi = math.pi * k / arc
        pts.append((r + r * math.cos(phi), top - r + r * math.sin(phi)))
    pts.append((0.0, top - r))
    return pts


def _sweep(poly, outward, profile, ring_scale=None):
    rings = []
    for i, p in enumerate(poly):
        k = ring_scale[i] if ring_scale else (1.0, 0.0)
        ring = []
        for u, z in profile:
            uu = RAIL_WIDTH / 2 + (u - RAIL_WIDTH / 2) * k[0]
            zz = z - (k[1] if z > RAIL_HEIGHT - 0.12 else 0.0)
            q = p + outward[i] * uu
            ring.append((q.x, q.y, zz))
        rings.append(ring)
    return rings


def _felt(lane, pal, cup_cut=0.42, lateral=16):
    n = lane.n
    verts, faces, cols = [], [], []
    felt_lo, felt_hi = pal.grad("felt")
    cup = lane.cup
    bands = max(2, int(lane.length / 0.62))
    for i in range(n + 1):
        t = i / n
        c, left = lane.pos(t), lane.left(t)
        stripe = 1.0 + (0.085 if int(t * bands) % 2 == 0 else -0.07)
        for j in range(lateral + 1):
            s = -1 + 2 * j / lateral
            q = c + left * (s * (LANE_WIDTH / 2 + 0.03))
            verts.append((q.x, q.y, 0.0))
            base = mix(felt_lo, felt_hi, 0.4 + 0.3 * (1 - abs(s)) + 0.1 * math.sin(t * 40 + s * 3))
            cols.append((base[0] * stripe, base[1] * stripe, base[2] * stripe, 1.0))
    for i in range(n):
        for j in range(lateral):
            a = i * (lateral + 1) + j
            quad = (a, a + lateral + 1, a + lateral + 2, a + 1)
            cx = sum(verts[v][0] for v in quad) / 4
            cy = sum(verts[v][1] for v in quad) / 4
            if math.hypot(cx - cup.x, cy - cup.y) < cup_cut:
                continue
            faces.append(quad)
    return verts, faces, cols


def _rail_colour(style, pal, z, joint, jitter):
    if style == "wood":
        lo, hi, dark = pal.c("wood_dark"), pal.c("wood_light"), pal.c("wood_deep")
    else:
        lo, hi, dark = pal.c("panel_grey"), pal.c("panel_white"), pal.c("metal_dark")
    tz = min(1.0, max(0.0, (z + 0.02) / (RAIL_HEIGHT + 0.02)))
    c = mix(lo, hi, tz ** 0.9)
    if style == "metal" and 0.025 < z < 0.075:
        c = pal.c("orange")
    if joint and style == "wood":
        c = mix(c, dark, 0.55)
    return (c[0] * jitter, c[1] * jitter, c[2] * jitter, 1.0)


def _add_loft_rail(p, name, rings, flip, style, joints, rng):
    obj = shapes.loft(name, rings, closed=True, cap_start=True, cap_end=True, flip=flip)
    shapes.finish(obj, bevel=0, dens=0, smooth_angle=55)
    cols = []
    for i, ring in enumerate(rings):
        jitter = 1.0 + 0.05 * math.sin(i * 1.7) + rng.uniform(-0.02, 0.02)
        for (_, _, z) in ring:
            cols.append(_rail_colour(style, p.pal, z, i in joints, jitter))
    colors.paint_values(obj, cols, ao=0.6)
    obj.data.materials.append(materials.get("matte" if style == "wood" else "satin"))
    p.parts.append(obj)


def _loft_rails(p, lane, style, rng):
    n = lane.n
    half = LANE_WIDTH / 2
    centers = [lane.pos(i / n) for i in range(n + 1)]
    lefts = [lane.left(i / n) for i in range(n + 1)]
    prof = rail_profile()
    joints = set(range(0, n + 1, 9))
    k_ring = [(0.94, 0.012) if i in joints else (1.0, 0.0) for i in range(n + 1)]
    for side in (1, -1):
        poly = [centers[i] + lefts[i] * (side * half) for i in range(n + 1)]
        outward = [lefts[i] * side for i in range(n + 1)]
        _add_loft_rail(p, "rail-%d" % side, _sweep(poly, outward, prof, k_ring), side < 0, style, joints, rng)
    for t_end, sgn in ((0.0, -1.0), (1.0, 1.0)):
        c, tg, lf = lane.pos(t_end), lane.tangent(t_end), lane.left(t_end)
        m = 12
        poly = [c + lf * (-(half + RAIL_WIDTH) + 2 * (half + RAIL_WIDTH) * j / m) for j in range(m + 1)]
        outward = [tg * sgn] * (m + 1)
        _add_loft_rail(p, "rail-end%d" % int(t_end), _sweep(poly, outward, prof), sgn < 0, style, set(), rng)
    if style == "metal":
        for side, tone in ((1, "cyan"), (-1, "magenta")):
            pts = [centers[i] + lefts[i] * (side * (half + RAIL_WIDTH / 2)) for i in range(n + 1)]
            pts = [(q.x, q.y, RAIL_HEIGHT + 0.004) for q in pts]
            tube = shapes.tube("neon-" + tone, pts, radius=0.02, seg=8, caps=True)
            p.add(tube, "emissive", (tone, tone), finish=False, ao=0.0, strength=2.6)
        for i in range(6, n, 14):
            for side in (1, -1):
                q = centers[i] + lefts[i] * (side * (half + RAIL_WIDTH / 2))
                post = shapes.box("rail-post", (0.2, 0.22, 0.34), base=True)
                p.add(post, "satin", ("orange_dark", "orange"), loc=(q.x, q.y, -0.05),
                      rot=(0, 0, lane.yaw(i / n)), bevel=0.03, segments=3, dens=0.09)


def _stone_rails(p, lane, rng):
    pal = p.pal
    half = LANE_WIDTH / 2
    stops = [(0.0, pal.c("stone_deep")), (0.45, pal.c("stone_dark")), (0.78, pal.c("stone_light")),
             (0.9, pal.c("moss")), (1.0, pal.c("moss_light"))]
    for side in (1, -1):
        t = 0.0
        index = 0
        while t < 1.0 - 1e-4:
            length = rng.uniform(0.5, 0.78)
            t1 = min(1.0, t + length / (lane.length * 1.08))
            a, b = lane.pos(t), lane.pos(t1)
            tm = (t + t1) / 2
            chord = (b - a).length + 0.03
            mid = lane.pos(tm) + lane.left(tm) * (side * (half + RAIL_WIDTH / 2))
            height = RAIL_HEIGHT + RAIL_SINK + rng.uniform(-0.012, 0.012)
            blk = shapes.box("rail-block", (RAIL_WIDTH + rng.uniform(0.0, 0.03), chord, height), base=True)
            p.add(blk, "matte", "stone", stops=stops, loc=(mid.x, mid.y, -RAIL_SINK),
                  rot=(0, 0, lane.yaw(tm) + rng.uniform(-2.0, 2.0)), bevel=0.032, segments=3, dens=0.1,
                  noise_amp=0.06, jitter=0.03, ao=0.6)
            if index % 2 == 0:
                glyph = shapes.box("rail-glyph", (0.07, 0.07, 0.03), base=True)
                p.add(glyph, "matte", ("stone_deep", "stone_dark"),
                      loc=(mid.x, mid.y, RAIL_HEIGHT - 0.012), rot=(0, 0, lane.yaw(tm) + 45.0),
                      bevel=0.008, dens=0, ao=0.4)
            t = t1
            index += 1
    for t_end, sgn in ((0.0, -1.0), (1.0, 1.0)):
        c, lf = lane.pos(t_end), lane.left(t_end)
        tg = lane.tangent(t_end)
        count = 4
        span = LANE_WIDTH + 2 * RAIL_WIDTH
        for j in range(count):
            a = -span / 2 + span * (j + 0.5) / count
            mid = c + lf * a + tg * (sgn * RAIL_WIDTH / 2)
            blk = shapes.box("rail-end-block", (RAIL_WIDTH, span / count + 0.03, RAIL_HEIGHT + RAIL_SINK), base=True)
            p.add(blk, "matte", "stone", stops=stops, loc=(mid.x, mid.y, -RAIL_SINK),
                  rot=(0, 0, lane.yaw(t_end) + 90.0), bevel=0.032, segments=3, dens=0.1,
                  noise_amp=0.06, jitter=0.03, ao=0.6)


def build_lane(world, style, lane, seed=7, flag=("red", "red"), collar=("gold", "gold"),
               tee=("felt_light", "felt_light"), collar_mat="satin", flag_mat="satin", glow=2.4):
    """Build the whole lane as one mesh named ``lane-<world>``."""
    p = Prop(world, "lane", seed, full_name="lane-" + world)
    pal = p.pal
    rng = p.rng

    verts, faces, cols = _felt(lane, pal)
    felt = shapes.make_object("lane-felt", verts, faces)
    shapes.finish(felt, bevel=0, dens=0, smooth_angle=89)
    colors.paint_values(felt, cols, ao=0.75)
    felt.data.materials.append(materials.get("matte"))
    p.parts.append(felt)

    bt = lane.ball_t
    b = lane.ball
    mat = shapes.box("tee-mat", (1.05, 0.78, TEE_MAT), base=True)
    p.add(mat, "matte", tee, loc=(b.x, b.y - 0.06, 0.0), rot=(0, 0, lane.yaw(bt)), bevel=0.008, dens=0.08, ao=0.5)

    if style == "stone":
        _stone_rails(p, lane, rng)
    else:
        _loft_rails(p, lane, style, rng)

    cup = lane.cup
    r = CUP_RADIUS
    inner = shapes.lathe("cup-inside", [(r, 0.0), (r, -0.22), (r * 0.9, -0.29), (0.0, -0.31)], seg=32, caps=False)
    p.add(inner, "matte", ("black", "black"), loc=(cup.x, cup.y, 0.0), finish=False, ao=1.0)
    ring = shapes.lathe("cup-collar", [(0.50, -0.01), (0.50, 0.012), (0.468, 0.031), (0.372, 0.034),
                                       (0.334, 0.02), (0.32, -0.01), (0.50, -0.01)], seg=40, caps=False)
    p.add(ring, collar_mat, collar, loc=(cup.x, cup.y, 0.0), bevel=0.006, dens=0.1, ao=0.0 if collar_mat == "emissive" else 0.6,
          strength=glow)

    pole = shapes.cylinder("flag-pole", 0.016, 0.013, 1.62, seg=10)
    p.add(pole, "satin", ("panel_grey", "panel_white") if pal.has("panel_white") else ("cream_dark", "cream"),
          loc=(cup.x, cup.y, -0.22), bevel=0.004, dens=0.1, ao=0.5)
    fin = shapes.sphere("flag-finial", 0.03, seg=12, rings=8)
    p.add(fin, "satin", ("gold", "gold") if pal.has("gold") else ("orange", "orange"),
          loc=(cup.x, cup.y, 1.41), bevel=0, dens=0, ao=0.3)
    spine = [(0.016 + 0.46 * k / 10, 0.045 * math.sin(k * 0.85) * (k / 10) ** 0.7, 1.31) for k in range(11)]
    widths = [0.15 * (1 - 0.75 * (k / 10)) for k in range(11)]
    pennant = shapes.ribbon("flag", spine, widths, normal_hint=(0, 1, 0), across=2)
    p.add(pennant, flag_mat, flag, loc=(cup.x, cup.y, 0.0), solidify=0.012, bevel=0, dens=0.1,
          ao=0.0 if flag_mat == "emissive" else 0.3, mode="x", strength=glow)
    return p.build()


def build_ball(world, lane, seed=3, stripe="red"):
    """Off-white golf ball (r = 0.16 m) resting on the tee mat."""
    p = Prop(world, "ball", seed, full_name="ball-" + world)
    pal = p.pal
    ball = shapes.sphere("ball", BALL_RADIUS, seg=32, rings=16)
    band = pal.c(stripe)

    def pattern(x, y, z):
        if abs(z) < 0.018:
            return (band[0] * 1.4, band[1] * 1.4, band[2] * 1.4)
        return 1.0

    low, high = (0.80, 0.78, 0.70, 1.0), (1.0, 0.97, 0.88, 1.0)
    p.add(ball, "satin", (low, high), loc=(0, 0, BALL_RADIUS), rot=(18, 0, 24), bevel=0, dens=0, subsurf=0,
          pattern=pattern, ao=0.6)
    obj = p.build()
    b = lane.ball
    obj.location = (b.x + 0.03, b.y, TEE_MAT)
    return obj
