"""Jungle temple waterfall (MIT License).

A rounded mossy rock wall with a carved channel; the water is a translucent
sheet with many thin, overlapping strands of different width and brightness,
white water rolling over the lip and bursting at the foot, and a foamy pool
with ripples around the impact point. Prop space: Blender Z is up, the water
falls towards -Z and the sheet faces -Y; footprint centred, base at z = 0.
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
FALL_TOP = LIP_Z + 0.06  # where the water leaves the lip
FALL_BOTTOM = 0.12  # where it meets the pool
FALL_SPEED = 2.2  # metres per second, read by the runtime as aurago_flow


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


def _fall_point(xi, s, off, drift=0.0, out=0.0, wobble=0.0, ph=0.0):
    """A point of the falling water.

    ``xi`` is the lateral position of the strand, ``s`` runs from the lip (0)
    to the foot (1), ``drift`` slides it sideways, ``out`` pushes it towards the
    viewer (negative y) and ``wobble`` makes it meander.
    """
    z = FALL_TOP - (FALL_TOP - FALL_BOTTOM) * s ** 1.05
    x = xi * (1.0 + 0.55 * s) + drift * s + wobble * math.sin(5.0 * s + ph) * s
    y = _wall_y(x, z, off) - 0.09 - 0.62 * s ** 3.0 + out
    if s < 0.1:  # arc over the lip from behind the edge
        y += 0.3 * (1.0 - s / 0.1) ** 2
    return (x, y, z)


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


def _fall_stops(p):
    """The body of the fall: white water at the foot, teal water in between and a
    bright band where the water leaves the lip."""
    pal = p.pal
    return [(0.0, pal.c("water_white")), (0.16, pal.c("water_light")), (0.4, pal.c("water")),
            (0.72, pal.c("water_deep")), (0.9, pal.c("water_light")), (1.0, pal.c("water_white"))]


def _strand_stops(p):
    """The strands are the aerated streaks on the body: brighter and whiter."""
    pal = p.pal
    return [(0.0, pal.c("water_white")), (0.08, pal.c("foam")), (0.24, pal.c("water_light")),
            (0.52, pal.c("water")), (0.8, pal.c("water_deep")), (0.93, pal.c("water_light")),
            (1.0, pal.c("water_white"))]


def _foam_stops(p):
    pal = p.pal
    return [(0.0, pal.c("water_light")), (0.45, pal.c("foam")), (1.0, pal.c("water_white"))]


def _sheet(p, rng, off):
    """The body of the fall: one wide sheet behind the strands, so the water has
    mass and the rock does not shine through it like a grille."""
    spine, widths = [], []
    steps = 16
    for i in range(steps + 1):
        s = i / steps
        x = 0.05 * math.sin(3.1 * s + 0.6) * s
        z = FALL_TOP - (FALL_TOP - FALL_BOTTOM) * s ** 1.05
        y = _wall_y(x, z, off) - 0.05 - 0.62 * s ** 3.0
        if s < 0.1:
            y += 0.3 * (1.0 - s / 0.1) ** 2
        spine.append((x, y, z))
        widths.append(CHANNEL * (1.0 + 0.55 * s) * (0.88 + 0.12 * math.sin(5.3 * s)))
    sheet = shapes.ribbon("sheet", spine, widths, normal_hint=(0, -1, 0), fold=-0.12, across=6)
    p.add(sheet, "glow", "water", stops=_fall_stops(p), strength=0.3, flow=FALL_SPEED,
          solidify=0.0, bevel=0, dens=0, ao=0.18, mode="z", span=(FALL_BOTTOM, FALL_TOP),
          pattern=lambda x, y, z: 0.84 + 0.16 * math.sin(x * 19.0 + math.sin(z * 1.7) * 1.2),
          smooth_angle=80.0, jitter=0.0)


def _strands(p, rng, off):
    """Wide, overlapping streaks in front of the sheet: the texture of moving
    water. They overlap each other, so the fall reads as one curtain with
    vertical striations instead of a row of separate ribbons."""
    n = 24
    for k in range(n):
        u = (k + 0.5) / n * 2.0 - 1.0
        xi = u * 0.56 + rng.uniform(-0.03, 0.03)
        w0 = rng.uniform(0.06, 0.13)
        ph = rng.uniform(0.0, math.tau)
        drift = rng.uniform(-0.1, 0.1)
        out = rng.uniform(-0.012, 0.055)
        mult = 0.72 + 0.23 * rng.random()
        stop = rng.uniform(0.86, 1.0)  # some streaks break off above the pool
        spine, widths = [], []
        steps = 13
        for i in range(steps + 1):
            s = i / steps * stop
            x, y, z = _fall_point(xi, s, off, drift=drift, out=out, wobble=0.035, ph=ph)
            spine.append((x, y, z))
            widths.append(w0 * (0.72 + 0.75 * s) * (0.5 + 0.5 * smoothstep(s, 0.0, 0.12)))
        strand = shapes.ribbon("strand", spine, widths, normal_hint=(0, -1, 0), fold=-0.25, across=2)
        p.add(strand, "glow", "water", stops=_strand_stops(p), strength=0.3, flow=FALL_SPEED,
              solidify=0.0, bevel=0, dens=0, ao=0.22, mode="z", span=(FALL_BOTTOM, FALL_TOP),
              pattern=lambda x, y, z, m=mult: m * (0.9 + 0.1 * math.sin(x * 31.0 + z * 2.3)),
              smooth_angle=80.0, jitter=0.0)


def _crest(p, rng, off):
    """The rolled lip: white water turning over the notch, so the top edge of the
    fall is a rounded crest instead of a row of cut-off ribbons."""
    spine, widths = [], []
    for i in range(11):
        u = i / 10.0 * 2.0 - 1.0
        x = u * 0.54
        z = _top_z(x) - 0.05
        y = _wall_y(x, z, off) - 0.08
        spine.append((x, y, z))
        widths.append(0.1 + 0.05 * math.sin(u * 4.6 + 0.8) ** 2)
    crest = shapes.ribbon("crest", spine, widths, normal_hint=(0, 0, 1), fold=-0.9, across=3)
    p.add(crest, "glow", "foam", stops=_foam_stops(p), strength=0.3, flow=FALL_SPEED,
          solidify=0.0, bevel=0, dens=0, ao=0.2, mode="z", span=(FALL_TOP - 0.25, FALL_TOP + 0.15),
          pattern=lambda x, y, z: 0.92 + 0.08 * math.sin(x * 26.0),
          smooth_angle=80.0, jitter=0.0)


def _splash(p, rng):
    """The impact: broken white water, thrown droplets and foam on the pool."""
    stops = _foam_stops(p)
    for _ in range(12):
        a = rng.uniform(0.0, math.tau)
        r = rng.uniform(0.04, 0.46)
        x = math.cos(a) * r
        y = -0.16 + math.sin(a) * r * 0.55 + rng.uniform(-0.05, 0.05)
        z = rng.uniform(0.05, 0.3)
        foam = shapes.ico_sphere("splash", rng.uniform(0.08, 0.22), 1)
        p.add(foam, "glow", "foam", stops=stops, strength=0.3, flow=FALL_SPEED,
              loc=(x, y, z), scale=(1.0, 1.0, 0.7), solidify=0.0, bevel=0, dens=0, ao=0.2,
              smooth_angle=80.0, jitter=0.0)
    for _ in range(6):  # droplets thrown out of the impact
        a = rng.uniform(math.radians(200), math.radians(340))
        d = rng.uniform(0.5, 1.1)
        x = math.cos(a) * d * 0.8
        y = -0.16 + math.sin(a) * d * 0.5
        z = rng.uniform(0.3, 0.85)
        drop = shapes.ico_sphere("drop", rng.uniform(0.03, 0.055), 1)
        p.add(drop, "glow", "foam", stops=stops, strength=0.3, flow=FALL_SPEED, loc=(x, y, z),
              scale=(1.0, 1.0, 1.3), solidify=0.0, bevel=0, dens=0, ao=0.2, smooth_angle=80.0, jitter=0.0)
    for _ in range(4):  # foam left on the rim of the pool
        a = rng.uniform(0.0, math.tau)
        x = 1.55 * math.cos(a)
        y = -0.95 + 1.0 * math.sin(a)
        foam = shapes.ico_sphere("foam", rng.uniform(0.07, 0.12), 1)
        p.add(foam, "glow", "foam", stops=stops, strength=0.3, flow=FALL_SPEED,
              loc=(x, y, rng.uniform(0.02, 0.08)), scale=(1.0, 1.0, 0.55), solidify=0.0, bevel=0,
              dens=0, ao=0.2, smooth_angle=80.0, jitter=0.0)


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
    # the ripples spread from the point where the fall hits the pool
    p.add(pool, "glow", "water", stops=stops, loc=(0, cy, 0.0), scale=(1.85, 1.1, 1), bevel=0, dens=0, ao=0.2, strength=0.3,
          mode="radial", center=(0, cy), span=(0.0, 2.0), smooth_angle=80.0, jitter=0.0,
          pattern=lambda x, y, z: 1.0 + 0.16 * math.sin(math.hypot(x, y + 0.16) * 21.0))


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
    """Waterfall: rock wall with channel, falling sheet and strands, foamy lip and
    splash, a rippling pool and framing rocks (front faces -Y)."""
    p = Prop(WORLD, "waterfall", seed)
    rng = p.rng
    off = (rng.uniform(-40, 40), rng.uniform(-40, 40), rng.uniform(-40, 40))
    _cliff(p, off)
    _boulders(p, rng)
    _sheet(p, rng, off)
    _strands(p, rng, off)
    _crest(p, rng, off)
    _splash(p, rng)
    _pool(p, rng)
    _rocks(p, rng)
    return p.build()
