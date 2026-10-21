"""Jungle temple waterfall (MIT License).

A rounded mossy rock wall with a carved channel; the water is a band of curved,
widening strips (light and dark alternating, foam at the lip and the splash)
that follows the channel, falls into a pool ringed with foam and small rocks.
Prop space: the water falls towards -Y; footprint centred, base at z=0.
"""

import math

from . import shapes
from .prop import Prop
from .props_temple_util import clamp01, nz, ring_grid, smoothstep, stone_fn

WORLD = "temple"
HALF_W = 1.75  # half width of the rock wall
HEIGHT = 3.5
BACK_Y = 1.9
LIP_Z = HEIGHT - 0.3  # water level at the notch in the top edge
CHANNEL = 0.62  # half width of the carved channel at the top


def _wall_y(x, z, off):
    """Front surface of the rock wall: leaning back with height, noisy, with a carved channel."""
    t = clamp01(z / HEIGHT)
    y = 0.55 + 0.75 * t ** 1.4
    y += 0.24 * nz(x, z, 0.0, off, 1.2) + 0.09 * nz(x, z, 4.0, off, 3.2)
    half = CHANNEL + 0.28 * (1.0 - t)
    y += 0.24 * (1.0 - smoothstep(abs(x), 0.0, half))
    return y


def _top_z(x):
    return HEIGHT - 0.3 * math.exp(-(x / 0.75) ** 2)


def _cliff(p, off):
    rings = []
    nx, nf = 30, 17
    for k in range(nx + 1):
        x = -HALF_W + 2 * HALF_W * k / nx
        env = math.sqrt(max(0.0, 1.0 - (abs(x) / HALF_W) ** 6))
        env = max(env, 0.04)
        zt = _top_z(x) * (0.35 + 0.65 * env)
        ring = []
        for i in range(nf + 1):
            z = 0.02 + (zt - 0.02) * i / nf
            ring.append((x, _wall_y(x, z, off), z))
        yf = ring[-1][1]
        for t in (0.25, 0.5, 0.75):
            ring.append((x, yf + (BACK_Y - yf) * t, zt + 0.12 * math.sin(math.pi * t)))
        ring.append((x, BACK_Y, zt * 0.55))
        ring.append((x, BACK_Y, 0.02))
        # pinch the ends into rounded caps
        yc = 0.5 * (ring[0][1] + BACK_Y)
        ring = [(rx * (1.0 - 0.14 * clamp01(rz / HEIGHT)), yc + (ry - yc) * (0.35 + 0.65 * env), rz) for rx, ry, rz in ring]
        rings.append(ring)
    cliff = ring_grid("cliff", rings, closed=True, cap_first=True, cap_last=True, center=(0.0, 1.0, 1.5))
    p.add(cliff, "matte", fn=stone_fn(p.pal, 0.0, HEIGHT, off, moss=0.95, moss_z=HEIGHT - 0.1, noise_amp=0.1, deep=True, strata=0.08), bevel=0,
          dens=0, smooth_angle=70.0, ao=0.75)


def _strip_colour(p, k):
    pal = p.pal
    stops = [(0.0, pal.c("foam")), (0.1, pal.c("water_white")), (0.3, pal.c("water_light")), (0.68, pal.c("water")),
             (0.9, pal.c("water_light")), (1.0, pal.c("water_white"))]
    return stops, (0.78 if k % 2 else 1.04)


def _water(p, rng, off):
    n = 7
    for k in range(n):
        xi = (k - (n - 1) / 2) * 0.17 + rng.uniform(-0.03, 0.03)
        w0 = 0.115 + 0.02 * rng.random()
        ph = rng.uniform(0, math.tau)
        spine, widths = [], []
        steps = 15
        for i in range(steps + 1):
            s = i / steps
            z = LIP_Z + 0.06 - (LIP_Z - 0.1) * s ** 1.05
            x = xi * (1.0 + 0.55 * s) + 0.05 * math.sin(5.0 * s + ph) * s
            y = _wall_y(x, z, off) - 0.09 - 0.62 * s ** 3.0
            if s < 0.1:  # arc over the lip from behind the edge
                y += 0.3 * (1.0 - s / 0.1) ** 2
            spine.append((x, y, z))
            widths.append(w0 * (1.0 + 0.7 * s ** 1.2) * (0.6 + 0.4 * smoothstep(s, 0.0, 0.12)))
        strip = shapes.ribbon("flow", spine, widths, normal_hint=(0, -1, 0), fold=0.12, across=3)
        stops, mult = _strip_colour(p, k)
        p.add(strip, "glow", "water", stops=stops, solidify=0.035, bevel=0, dens=0, ao=0.25, mode="z", strength=0.3,
              pattern=lambda x, y, z, m=mult: m * (0.93 + 0.07 * math.sin(x * 23.0 + z * 1.7)), jitter=0.0, smooth_angle=80.0)


def _boulders(p, rng):
    """Mossy boulders on the wall: they break the smooth loaf silhouette."""
    stops = [(0.0, p.pal.c("stone_deep")), (0.4, p.pal.c("stone_dark")), (0.62, p.pal.c("stone")), (0.74, p.pal.c("moss_dark")),
             (0.9, p.pal.c("moss")), (1.0, p.pal.c("moss_light"))]
    sites = [(-1.15, 0.9, HEIGHT - 0.5, 0.6), (1.05, 1.0, HEIGHT - 0.55, 0.52), (1.72, 0.7, 1.7, 0.45), (-1.7, 0.7, 1.2, 0.5)]
    for x, y, z, r in sites:
        rock = shapes.blob("boulder", r, rng, subdiv=3 if r > 0.55 else 2, amp=0.24, squash=(1.0, 0.95, 0.8), cuts=3, relax=2)
        p.add(rock, "matte", "stone", stops=stops, loc=(x, y, z), rot=(0, 0, rng.uniform(0, 360)), bevel=0, dens=0, ao=0.7,
              noise_amp=0.08, smooth_angle=80.0, span=(z - r * 0.9, z + r * 0.8))


def _pool(p, rng):
    cy = -0.95
    pool = shapes.lathe("pool", [(0.0, 0.04), (1.0, 0.04), (1.08, 0.02), (1.14, 0.0)], seg=32, caps=False)
    stops = [(0.0, p.pal.c("water_deep")), (0.55, p.pal.c("water")), (0.85, p.pal.c("water_light")), (1.0, p.pal.c("water_white"))]
    p.add(pool, "glow", "water", stops=stops, loc=(0, cy, 0.0), scale=(1.85, 1.1, 1), bevel=0, dens=0, ao=0.2, strength=0.3,
          mode="radial", center=(0, cy), span=(0.0, 2.0), smooth_angle=80.0, jitter=0.0)
    for k in range(10):  # foam clumps at the foot of the fall and on the pool rim
        if k < 5:
            a = rng.uniform(math.radians(195), math.radians(345))
            x, y, r = 0.9 * math.cos(a) * 0.5 + rng.uniform(-0.25, 0.25), -0.05 + 0.3 * math.sin(a), rng.uniform(0.12, 0.26)
        else:
            a = rng.uniform(0, math.tau)
            x, y, r = 1.7 * math.cos(a), cy + 1.0 * math.sin(a), rng.uniform(0.06, 0.12)
        foam = shapes.ico_sphere("foam", r, 2)
        p.add(foam, "satin", ("water_white", "foam"), loc=(x, y, r * 0.35), scale=(1.0, 1.0, 0.65), bevel=0, dens=0, ao=0.2,
              smooth_angle=80.0)


def _rocks(p, rng):
    sites = [(-2.05, 0.2, 0.75), (2.0, 0.15, 0.7), (-1.55, -0.55, 0.42), (1.6, -0.6, 0.38), (-2.0, -1.2, 0.3), (2.05, -1.35, 0.33),
             (-1.05, -1.95, 0.24), (1.0, -2.0, 0.22), (0.0, -2.05, 0.18)]
    stops = [(0.0, p.pal.c("stone_deep")), (0.55, p.pal.c("stone_dark")), (0.85, p.pal.c("stone")), (0.93, p.pal.c("moss")),
             (1.0, p.pal.c("moss_light"))]
    for x, y, r in sites:
        rock = shapes.blob("rock-side", r, rng, subdiv=3 if r > 0.6 else 2, amp=0.22, squash=(1.0, 0.95, 0.8), cuts=2, flat_bottom=0.4, relax=2)
        p.add(rock, "matte", "stone", stops=stops, loc=(x, y, r * 0.38), rot=(0, 0, rng.uniform(0, 360)), bevel=0, dens=0, ao=0.7,
              noise_amp=0.07, smooth_angle=80.0, span=(-r * 0.5, r * 0.9))


def build_waterfall(seed=2):
    """Waterfall: rock wall with channel, flowing strip band, foamy pool, framing rocks (front faces -Y)."""
    p = Prop(WORLD, "waterfall", seed)
    rng = p.rng
    off = (rng.uniform(-40, 40), rng.uniform(-40, 40), rng.uniform(-40, 40))
    _cliff(p, off)
    _boulders(p, rng)
    _water(p, rng, off)
    _pool(p, rng)
    _rocks(p, rng)
    return p.build()
