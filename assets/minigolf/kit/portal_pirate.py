"""Skull-rock tunnel portal of the pirate bay (MIT License, see ../LICENSE).

A rock skull in the style of the kit's ``skullrock`` prop, about 4 m wide, 2.75 m high and ``DEPTH`` deep, whose open
mouth is the tunnel entrance: the mouth arch is cut through the whole body (inner opening ``tunnel_plan.HALF`` wide
and ``CROWN`` high, the same arch as the roof behind it) and framed by a row of bone teeth along the top of the arch
(the teeth stay above 1.3 m, outside the 1.9 x 1.0 m opening). Eye sockets, a nose hollow and a brow ridge sit on the
face; a few rocks around the foot let it grow out of the sand.

``build(seed, index)`` returns one joined mesh in the local portal frame (x right, y forward, z up, the middle of
the mouth on the floor at the origin, the body behind it); ``tunnel_parts.place`` moves it onto the lane.
"""

import math
import random

from mathutils import Vector

from . import shapes
from .palette import mix
from .prop import Prop
from .props_temple_util import arch_outline, boolean_cut, nz, ring_grid, smoothstep
from .tunnel_plan import CROWN, HALF, PORTAL, SPRING

WORLD = "pirate"
DEPTH = PORTAL["pirate"][1]
WIDTH, HEIGHT, DOME = 2.05, 2.75, 1.25     # half width, top, start of the dome
EXPONENT = 2.6
FOOT_Z = -0.2
EYES = ((-0.82, 2.02), (0.82, 2.02))
LEVELS = (FOOT_Z, 0.3, 0.8, 1.2, 1.55, 1.75, 1.92, 2.1, 2.28, 2.44, 2.56, 2.66, 2.72, HEIGHT)
COLS = 48


def _shape(z):
    """Scale 0..1 of the cross-section at height ``z`` (nearly straight sides, then an elliptic dome)."""
    if z <= DOME:
        return 0.96 + 0.04 * smoothstep(z, FOOT_Z, DOME)
    t = (z - DOME) / (HEIGHT - DOME)
    return math.sqrt(max(0.0, 1.0 - t * t))


def _theta(j):
    """Column angle, denser at the face (angle -90 degrees looks back along the lane)."""
    u = j / COLS
    return -math.pi / 2 + math.tau * u - 0.35 * math.sin(math.tau * u)


def _face_offset(x, z):
    """How far the face is pushed inwards (positive) or outwards (negative) at ``(x, z)``: sockets, nose, brow."""
    inward = 0.0
    for ex, ez in EYES:
        inward += 0.24 * math.exp(-(((x - ex) / 0.36) ** 2 + ((z - ez) / 0.31) ** 2))
    inward += 0.1 * math.exp(-((x / 0.14) ** 2 + ((z - 1.72) / 0.16) ** 2))
    brow = 0.14 * math.exp(-(((z - 2.5) / 0.14) ** 2)) * (1.0 - smoothstep(abs(x), 0.9, 1.55))
    return inward - brow


def _body(off):
    rings = []
    yc = DEPTH / 2.0
    for z in LEVELS:
        k = _shape(z)
        ring = []
        for j in range(COLS):
            a = _theta(j)
            c, s = math.cos(a), math.sin(a)
            x = WIDTH * k * math.copysign(abs(c) ** (2.0 / EXPONENT), c)
            y = yc + (yc + 0.05) * k * math.copysign(abs(s) ** (2.0 / EXPONENT), s)
            if y < yc:
                y += _face_offset(x, z) * smoothstep(yc - y, 0.0, 0.5)
            lump = 0.07 * nz(x, y, z, off, 1.3) + 0.03 * nz(x, y, z, off, 3.7)
            r = math.hypot(x, y - yc) + 1e-9
            ring.append((x + lump * x / r, y + lump * (y - yc) / r, z))
        rings.append(ring)
    body = ring_grid("skull-rock", rings, cap_first=True, center=(0.0, yc, 1.0))
    pts = arch_outline(HALF, SPRING, steps=12, z0=FOOT_Z - 0.3, rise=CROWN - SPRING)
    cutter = shapes.loft("mouth-cut", [[(x, y, z) for x, z in pts] for y in (-1.5, DEPTH + 1.5)], closed=True, recalc=True)
    return boolean_cut(body, cutter)


def _colour(p, off):
    pal = p.pal
    lo, hi, dark = pal.c("rock_dark"), pal.c("rock_light"), pal.c("coal")

    def fn(x, y, z):
        c = mix(lo, hi, 0.2 + 0.7 * min(1.0, max(0.0, (z + 0.2) / 2.6)))
        k = 1.0 + 0.1 * nz(x, y, z, off, 2.2)
        c = (c[0] * k, c[1] * k, c[2] * k, 1.0)
        cave = 0.0
        for ex, ez in EYES:
            cave = max(cave, 1.0 - smoothstep(math.hypot((x - ex) / 0.34, (z - ez) / 0.3), 0.6, 1.0))
        cave = max(cave, 1.0 - smoothstep(math.hypot(x / 0.17, (z - 1.7) / 0.19), 0.5, 1.0))
        inside = (abs(x) < HALF + 0.06 and z < SPRING + (CROWN - SPRING) * math.sqrt(max(0.0, 1.0 - (x / HALF) ** 2)) + 0.06 and y > 0.04)
        if inside:
            cave = max(cave, smoothstep(y, 0.04, 1.4) * 0.97)
        return mix(c, dark, 0.93 * cave)

    return fn


def _front(x, z):
    """Plan ``y`` of the face at ``(x, z)`` without carving (the superellipse of ``_body``)."""
    k = _shape(z)
    rel = min(1.0, abs(x) / (WIDTH * k + 1e-9))
    return DEPTH / 2.0 - (DEPTH / 2.0 + 0.05) * k * (1.0 - rel ** EXPONENT) ** (1.0 / EXPONENT)


def _sockets(p):
    """Dark, flattened ellipsoids in the eye sockets and the nose hollow: round caves whatever the mesh resolution is."""
    coal = p.pal.c("coal")
    for ex, ez in EYES:
        cave = shapes.sphere("socket", 0.3, seg=10, rings=6)
        p.add(cave, "matte", (coal, coal), loc=(ex, _front(ex, ez) + 0.2, ez), scale=(1.0, 0.4, 0.88), bevel=0, dens=0, ao=0.2, smooth_angle=80)
    nose = shapes.sphere("nose-hole", 0.14, seg=8, rings=5)
    p.add(nose, "matte", (coal, coal), loc=(0.0, _front(0.0, 1.7) + 0.1, 1.7), scale=(0.9, 0.45, 1.3), bevel=0, dens=0, ao=0.2, smooth_angle=80)


def _teeth(p):
    bone_lo, bone_hi = p.pal.c("cream_dark"), p.pal.c("skull")
    rise = CROWN - SPRING
    for deg in range(56, 125, 8):
        a = math.radians(deg)
        x, z = (HALF - 0.01) * math.cos(a), SPRING + (rise - 0.01) * math.sin(a)
        nx, nz_ = -math.cos(a) / HALF, -math.sin(a) / rise            # inward normal of the ellipse
        n = math.hypot(nx, nz_)
        nx, nz_ = nx / n, nz_ / n
        phi = math.degrees(math.atan2(nx, nz_))
        tooth = shapes.box("tooth", (0.16, 0.18, 0.26), base=False)
        p.add(tooth, "matte", (bone_lo, bone_hi), loc=(x + nx * 0.1, 0.1, z + nz_ * 0.1), rot=(0, phi, 0), bevel=0.03, segments=1, dens=0,
              ao=0.6, smooth_angle=60)


def build(seed, index):
    """The portal mesh (see the module docstring)."""
    p = Prop(WORLD, "tunnel-portal-%d" % index, seed=seed, full_name="tunnel-portal-%d" % index)
    off = (p.rng.uniform(-30, 30), p.rng.uniform(-30, 30), p.rng.uniform(-30, 30))
    p.add(_body(off), "matte", fn=_colour(p, off), bevel=0, dens=0, smooth_angle=70.0, weighted=False, ao=0.75)
    _teeth(p)
    _sockets(p)
    rng = random.Random(seed + 41 * index)
    for sign, (dx, dy, size) in zip((-1, 1, -1, 1), ((2.0, 0.5, 0.5), (2.05, 1.5, 0.42), (1.7, 2.3, 0.4), (1.9, -0.05, 0.36))):
        rock = shapes.blob("nest", size, rng, subdiv=2, amp=0.2, freq=1.3, squash=(1.0, 0.95, 0.65), cuts=2, flat_bottom=0.45, relax=2)
        p.add(rock, "matte", "rock", loc=(sign * dx, dy, FOOT_Z + 0.02), rot=(0, 0, rng.uniform(0, 360)), bevel=0, dens=0, ao=0.7, noise_amp=0.08,
              smooth_angle=80)
    return p.build()
