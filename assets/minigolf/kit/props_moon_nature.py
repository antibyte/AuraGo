"""Moon regolith props: boulders, pebbles, craters and glowing crystals (MIT License).

Rocks get their variety from the paint: soft dark spots, dust speckle and shallow
crater dents (real geometry plus matching shading) driven by ``mathutils.noise``
and ``random.Random`` seeds only. Every builder returns ONE joined mesh named
``prop-moon-<name>`` with its lowest point at z = 0.
"""

import math

from mathutils import Vector, noise

from . import shapes
from .palette import mix
from .prop import Prop

WORLD = "moon"


def _smooth(x, lo, hi):
    t = min(1.0, max(0.0, (x - lo) / (hi - lo)))
    return t * t * (3.0 - 2.0 * t)


def _offset(rng):
    return Vector((rng.uniform(-60, 60), rng.uniform(-60, 60), rng.uniform(-60, 60)))


# --------------------------------------------------------------------------
# Regolith surface variety
# --------------------------------------------------------------------------

def _dents(rng, count):
    """Random crater dents ``(unit direction, angular radius, depth)`` biased to the upper hemisphere."""
    out = []
    for _ in range(count):
        d = Vector((rng.gauss(0, 1), rng.gauss(0, 1), rng.uniform(0.0, 1.2))).normalized()
        out.append((d, math.radians(rng.uniform(19, 34)), rng.uniform(0.09, 0.15)))
    return out


def _apply_dents(obj, dents):
    """Push vertices of a rock inwards along their radial direction (dent) with a faint raised rim."""
    for v in obj.data.vertices:
        n = v.co.normalized()
        k = 0.0
        for d, theta, depth in dents:
            t = math.acos(max(-1.0, min(1.0, n.dot(d)))) / theta
            if t < 1.0:
                k -= depth * (1.0 - t * t) ** 2
            elif t < 1.6:
                k += depth * 0.35 * math.exp(-(((t - 1.12) / 0.2) ** 2))
        v.co = v.co * (1.0 + k)


def regolith_pattern(rng, centre=(0.0, 0.0, 0.0), dents=(), scale=6.5, amp=1.0):
    """Paint ``pattern(x, y, z) -> (r, g, b)`` multiplier: dark spots, dust speckle, dent shading."""
    o1, o2 = _offset(rng), _offset(rng)
    cx, cy, cz = centre

    def pattern(x, y, z):
        p = Vector((x - cx, y - cy, z - cz))
        spot = _smooth(noise.noise(p * scale + o1), 0.12, 0.40)
        speck = _smooth(noise.noise(p * scale * 2.6 + o2), 0.28, 0.55)
        spot, speck = spot * amp, speck * amp
        r, g, b = 1.0 - 0.30 * spot + 0.15 * speck, 1.0 - 0.29 * spot + 0.14 * speck, 1.0 - 0.20 * spot + 0.10 * speck
        if dents and p.length > 1e-6:
            n = p.normalized()
            for d, theta, _depth in dents:
                t = math.acos(max(-1.0, min(1.0, n.dot(d)))) / theta
                if t < 1.0:
                    w = 0.42 * (1.0 - t * t)
                    r, g, b = r * (1 - w), g * (1 - w), b * (1 - w * 0.8)
                elif t < 1.5:
                    w = 0.2 * math.exp(-(((t - 1.1) / 0.16) ** 2))
                    r, g, b = r + w, g + w, b + w * 0.9
        return (r, g, b)

    return pattern


def _rock_stops(pal, variant):
    deep, dark, reg, light = pal.c("regolith_deep"), pal.c("regolith_dark"), pal.c("regolith"), pal.c("regolith_light")
    if variant == 0:
        return [(0.0, deep), (0.35, mix(deep, dark, 0.6)), (0.8, dark), (1.0, mix(dark, reg, 0.6))]
    if variant == 1:
        return [(0.0, mix(deep, dark, 0.4)), (0.5, mix(dark, reg, 0.3)), (1.0, mix(reg, light, 0.2))]
    return [(0.0, mix(dark, reg, 0.3)), (0.5, reg), (1.0, mix(reg, light, 0.6))]


def _lift(obj):
    """Height that puts the lowest vertex of ``obj`` at z = 0."""
    return -min(v.co.z for v in obj.data.vertices)


# --------------------------------------------------------------------------
# Boulder and pebble group
# --------------------------------------------------------------------------

def build_boulder(seed=1, size=0.6, cuts=5, subdiv=4, squash=(1.0, 0.85, 0.72), dents=None):
    """Faceted regolith boulder with crater dents, dark spots and dusty tops."""
    p = Prop(WORLD, "boulder", seed)
    rock = shapes.blob("boulder", size, p.rng, subdiv=subdiv, amp=0.2, freq=1.1, squash=squash, cuts=cuts, cut_depth=0.78,
                       flat_bottom=0.5, relax=1)
    dent_list = _dents(p.rng, (3 if subdiv >= 4 else 0) if dents is None else dents)
    _apply_dents(rock, dent_list)
    lift = _lift(rock)
    pat = regolith_pattern(p.rng, centre=(0.0, 0.0, lift), dents=dent_list)
    p.add(rock, "matte", "rock", loc=(0, 0, lift), bevel=0, dens=0, ao=0.7, smooth_angle=80, jitter=0.03,
          stops=_rock_stops(p.pal, 0), pattern=pat)
    return p.build()


def build_pebble(seed=1, count=7):
    """Small group of regolith stones (3-8 cm tall) sharing one footprint."""
    p = Prop(WORLD, "pebble", seed)
    rng = p.rng
    spots = []
    for k in range(count):
        size = 0.17 if k == 0 else rng.uniform(0.06, 0.13)
        for _ in range(40):
            a, d = rng.uniform(0, math.tau), (0.0 if k == 0 else rng.uniform(0.15, 0.4))
            x, y = math.cos(a) * d, math.sin(a) * d
            if all(math.hypot(x - ox, y - oy) >= (size + osz) * 0.85 for ox, oy, osz in spots):
                spots.append((x, y, size))
                break
    cx = sum(s[0] for s in spots) / len(spots)
    cy = sum(s[1] for s in spots) / len(spots)
    for k, (x, y, size) in enumerate(spots):
        stone = shapes.blob("pebble", size, rng, subdiv=3 if size > 0.08 else 2, amp=0.26, freq=1.5,
                            squash=(1.0, rng.uniform(0.7, 1.0), rng.uniform(0.55, 0.8)), cuts=rng.choice((3, 4)), cut_depth=0.78,
                            flat_bottom=0.45, relax=0)
        lift = _lift(stone)
        dent_list = _dents(rng, 1) if size > 0.14 else []
        _apply_dents(stone, dent_list)
        pat = regolith_pattern(rng, centre=(x - cx, y - cy, lift), dents=dent_list, scale=9.0, amp=0.6)
        p.add(stone, "matte", "rock", loc=(x - cx, y - cy, lift), rot=(0, 0, rng.uniform(0, 360)), bevel=0, dens=0, ao=0.7,
              smooth_angle=80, jitter=0.03, stops=_rock_stops(p.pal, k % 3), pattern=pat)
    return p.build()


# --------------------------------------------------------------------------
# Crater ring (flat)
# --------------------------------------------------------------------------

def build_crater(seed=1, radius=0.85, height=0.16):
    """Flat crater: noisy raised rim, sloping wall, dark floor and a few rim boulders.

    The mesh is an open surface (no underside) whose outer edge lies at z = 0, so
    it blends into the ground when placed with a small sink.
    """
    p = Prop(WORLD, "crater", seed)
    rng = p.rng
    pal = p.pal
    o1, o2, o3 = _offset(rng), _offset(rng), _offset(rng)
    seg = 56
    us = [0.0, 0.2, 0.4, 0.55, 0.65, 0.74, 0.83, 0.9, 0.97, 1.04, 1.12, 1.22, 1.34, 1.46]
    floor_z = 0.035

    def rim_radius(a):
        c, s = math.cos(a), math.sin(a)
        return radius * (1.0 + 0.09 * noise.noise(Vector((c * 1.3, s * 1.3, 0.0)) + o1)
                         + 0.04 * noise.noise(Vector((c * 3.1, s * 3.1, 0.0)) + o2))

    def rim_height(a):
        return height * (1.0 + 0.32 * noise.noise(Vector((math.cos(a) * 1.6, math.sin(a) * 1.6, 0.0)) + o3))

    def profile(u, a):
        h = rim_height(a)
        if u <= 0.62:
            return floor_z + 0.012 * noise.noise(Vector((u * math.cos(a) * 4, u * math.sin(a) * 4, 3.0)) + o2)
        if u <= 0.97:
            return floor_z + (h - floor_z) * _smooth(u, 0.62, 0.97)
        return h * (1.0 - _smooth(u, 0.97, us[-1])) ** 1.15

    verts = [(0.0, 0.0, floor_z)]
    faces = []
    ring_ids = []
    for k, u in enumerate(us[1:], start=1):
        ids = []
        for i in range(seg):
            a = math.tau * i / seg
            r = u * rim_radius(a)
            ids.append(len(verts))
            verts.append((r * math.cos(a), r * math.sin(a), profile(u, a)))
        ring_ids.append(ids)
    for i in range(seg):
        faces.append((0, ring_ids[0][i], ring_ids[0][(i + 1) % seg]))
    for k in range(len(ring_ids) - 1):
        for i in range(seg):
            j = (i + 1) % seg
            faces.append((ring_ids[k][i], ring_ids[k + 1][i], ring_ids[k + 1][j], ring_ids[k][j]))
    surf = shapes.make_object("crater-ground", verts, faces)
    dust, reg, reg_d, reg_l, deep = pal.c("dust"), pal.c("regolith"), pal.c("regolith_dark"), pal.c("regolith_light"), pal.c("regolith_deep")

    def paint(x, y, z):
        a = math.atan2(y, x)
        u = math.hypot(x, y) / rim_radius(a)
        n = noise.noise(Vector((x * 5.0, y * 5.0, 1.0)) + o2)
        spot = _smooth(noise.noise(Vector((x * 7.0, y * 7.0, 4.0)) + o1), 0.15, 0.42)
        if u < 0.62:
            c = mix(deep, reg_d, 0.15 + 0.4 * _smooth(u, 0.0, 0.62))
            c = mix(c, reg_d, 0.4 * _smooth(n, 0.1, 0.5))
        elif u < 1.0:
            t = _smooth(u, 0.62, 1.0)
            c = mix(mix(reg_d, deep, 0.3), mix(reg_l, dust, 0.3), t)
        else:
            t = _smooth(u, 0.97, us[-1])
            c = mix(mix(reg_l, dust, 0.25), mix(reg, reg_l, 0.1), t)
        k = 1.0 - 0.22 * spot
        return (c[0] * k, c[1] * k, c[2] * k, 1.0)

    p.add(surf, "matte", "rock", fn=paint, finish=True, bevel=0, dens=0.16, smooth_angle=70, ao=0.55)
    # rim boulders
    for _ in range(8):
        a = rng.uniform(0, math.tau)
        u = rng.uniform(0.92, 1.12)
        size = rng.uniform(0.06, 0.14)
        r = u * rim_radius(a)
        stone = shapes.blob("rim-rock", size, rng, subdiv=2, amp=0.28, freq=1.6, squash=(1.0, rng.uniform(0.75, 1.0), 0.58), cuts=3,
                            cut_depth=0.78, flat_bottom=0.4, relax=0)
        lift = _lift(stone) + profile(u, a) - 0.03
        pat = regolith_pattern(rng, centre=(r * math.cos(a), r * math.sin(a), lift), scale=12.0)
        p.add(stone, "matte", "rock", loc=(r * math.cos(a), r * math.sin(a), lift), rot=(0, 0, rng.uniform(0, 360)), bevel=0, dens=0,
              ao=0.7, smooth_angle=80, jitter=0.03, stops=_rock_stops(pal, 1), pattern=pat)
    return p.build()


# --------------------------------------------------------------------------
# Crystal cluster
# --------------------------------------------------------------------------

def _column(name, w, body, tip, skew):
    """Hexagonal prism with a flared root, straight shaft and a two-stage bevelled, slanted tip."""

    def ring(r, z, dx=0.0):
        return [(dx + r * math.cos(math.tau * i / 6), r * math.sin(math.tau * i / 6), z) for i in range(6)]

    rings = [ring(w * 0.8, 0.0), ring(w, body * 0.12), ring(w, body), ring(w * 0.56, body + tip * 0.5, skew * 0.45),
             ring(w * 0.2, body + tip, skew)]
    return shapes.loft(name, rings, closed=True, recalc=True)


def build_crystal(seed=1, tone="cyan", count=7):
    """Cluster of 3D hexagonal glow columns with slanted tips, rooted in a small regolith outcrop."""
    p = Prop(WORLD, "crystal", seed)
    rng = p.rng
    pal = p.pal
    other = "magenta" if tone == "cyan" else "cyan"
    white = (1.0, 1.0, 1.0, 1.0)
    mound = shapes.blob("crystal-mound", 0.42, rng, subdiv=3, amp=0.22, freq=1.2, squash=(1.0, 0.9, 0.34), cuts=0, flat_bottom=0.5,
                        relax=2)
    lift = _lift(mound)
    pat = regolith_pattern(rng, centre=(0, 0, lift), scale=8.0)
    p.add(mound, "matte", "rock", loc=(0, 0, lift), bevel=0, dens=0, ao=0.75, smooth_angle=80, stops=_rock_stops(pal, 0), pattern=pat)
    for k in range(count):
        a = rng.uniform(0, math.tau) if k else 0.0
        d = (0.0 if k == 0 else rng.uniform(0.12, 0.3))
        body = rng.uniform(0.22, 0.7) * (1.4 if k == 0 else 1.0)
        w = rng.uniform(0.085, 0.135) * (1.15 if k == 0 else 1.0)
        tip = w * rng.uniform(1.5, 2.1)
        t = other if k % 3 == 2 else tone
        col = _column("crystal", w, body, tip, w * rng.uniform(0.25, 0.7) * rng.choice((-1, 1)))
        dark, mid, glow = pal.c(t + "_dark"), pal.c(t), mix(pal.c(t), white, 0.55)
        lean = 4.0 if k == 0 else rng.uniform(10, 24)
        out = math.degrees(a)
        shapes.transform(col, rot=(0, 0, rng.uniform(0, 360)))
        p.add(col, "glow", (t + "_dark", t), loc=(math.cos(a) * d, math.sin(a) * d, lift * 0.6),
              rot=(-lean * math.sin(math.radians(out)), lean * math.cos(math.radians(out)), 0.0), bevel=0.014, segments=1, dens=0,
              ao=0.3, strength=1.0, smooth_angle=20, stops=[(0.0, mix(dark, pal.c("black"), 0.3)), (0.45, mix(dark, mid, 0.55)),
                                                          (0.85, mid), (1.0, glow)])
    return p.build()
