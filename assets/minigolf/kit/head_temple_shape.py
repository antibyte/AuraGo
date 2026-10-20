"""Skull form of the jungle temple head: surface maths, skull mesh, moss cap (MIT License).

Prop space: the face looks towards -Y, the base is z=0, the footprint is centred.
The skull is a rounded super-ellipse drum with an elliptic dome (widest at the
cheeks, narrower towards the jaw), shaped by a few Gaussian bumps on the front.
``front_y(x, z)`` gives the face surface so the face parts can sit on it.
"""

import math

from mathutils import Vector

from . import shapes
from .props_temple_util import arch_outline, bevel_edges, boolean_cut, nz, ring_grid, smoothstep

ZTOP = 4.7  # top of the dome
HALF_W = 2.3  # half width at the widest ring
HALF_D = 1.85  # half depth at the widest ring
EXP = 2.2  # cross-section exponent (2 = ellipse, large = box)
DOME_Z = 3.1  # the dome starts here
CENTER = (0.0, 0.0, 2.3)
MOUTH_HALF = 1.1
SPRING = 0.55  # where the mouth arch starts to curve (arch top = SPRING + MOUTH_HALF)
MOUTH_BACK = 0.6  # back wall of the mouth recess

# (x, z, amplitude, sigma_x, sigma_z): outward bumps on the front of the skull.
_BUMPS = ((0.0, 3.85, 0.2, 1.5, 0.55), (-1.45, 2.0, 0.38, 0.8, 0.62), (1.45, 2.0, 0.38, 0.8, 0.62),
          (0.0, 0.9, 0.16, 1.6, 0.8), (-1.65, 3.3, 0.1, 0.5, 0.5), (1.65, 3.3, 0.1, 0.5, 0.5))
BROW_Z = 3.3  # brow line height at the nose bridge; it rises towards the temples


def width_factor(z):
    """Ring scale 0..1 at height ``z`` (jaw narrower, cheeks widest, elliptic dome)."""
    body = (0.82 + 0.18 * smoothstep(min(z, DOME_Z), 0.0, 1.8)) * (1.0 - 0.03 * smoothstep(min(z, DOME_Z), 2.4, DOME_Z))
    if z >= DOME_Z:
        u = (z - DOME_Z) / (ZTOP - DOME_Z)
        return body * math.sqrt(max(0.0, 1.0 - u * u))
    return body


def brow_line(x):
    return BROW_Z + 0.36 * (abs(x) / 2.0) ** 1.15


def bump(x, z):
    """Outward face relief: soft bumps, the brow ridge with its eye hollow and the nose bridge."""
    total = sum(a * math.exp(-((x - fx) / sx) ** 2 - ((z - fz) / sz) ** 2) for fx, fz, a, sx, sz in _BUMPS)
    ax = abs(x)
    dz = z - brow_line(x)
    ridge = 0.58 * (1.0 - 0.5 * smoothstep(ax, 0.5, 2.1)) * math.exp(-(dz / (0.3 if dz > 0 else 0.17)) ** 2)
    hollow = -0.36 * math.exp(-((ax - 0.98) / 0.52) ** 2 - ((z - (brow_line(x) - 0.52)) / 0.34) ** 2)
    nose = 0.58 * math.exp(-(x / 0.4) ** 2) * smoothstep(z, 3.55, 2.6) * smoothstep(z, 1.7, 2.2)
    return total + ridge + hollow + nose


def arch_sd(x, z):
    """Approximate signed distance to the mouth arch outline (negative inside)."""
    if z <= SPRING:
        return abs(x) - MOUTH_HALF
    return math.hypot(x, z - SPRING) - MOUTH_HALF


def skull_xy(theta, z):
    """Surface point of the skull at angle ``theta`` (0 = +X, -pi/2 = the face) and height ``z``."""
    f = width_factor(z)
    c, s = math.cos(theta), math.sin(theta)
    e = 2.0 / EXP
    x = HALF_W * f * math.copysign(abs(c) ** e, c)
    y = HALF_D * f * math.copysign(abs(s) ** e, s)
    if y < 0.0:
        y -= bump(x, z) * smoothstep(-y / (HALF_D * f + 1e-9), 0.1, 0.7)
    return x, y


def front_y(x, z):
    """Y of the front surface at (x, z) (negative = towards the viewer)."""
    f = width_factor(z)
    if abs(x) >= HALF_W * f:
        return 0.0
    y0 = -HALF_D * f * (1.0 - (abs(x) / (HALF_W * f)) ** EXP) ** (1.0 / EXP)
    return y0 - bump(x, z) * smoothstep(-y0 / (HALF_D * f + 1e-9), 0.1, 0.7)


def _levels():
    zs = [0.0, 0.2, 0.4] + [0.6 + 0.12 * k for k in range(21)]
    zs.extend(DOME_Z + (ZTOP - DOME_Z) * math.sin(0.5 * math.pi * k / 9) for k in range(0, 10))
    return sorted(set(round(z, 4) for z in zs))


def _theta(j, cols):
    """Column angle: denser at the face (u=0 is the face direction, -Y), sparser at the back."""
    u = j / cols
    return -math.pi / 2 + math.tau * u - 0.45 * math.sin(math.tau * u)


def build_skull(cols=88):
    """Closed skull with the mouth recess cut into the front (Boolean), before bevel and colour."""
    rings = []
    for z in _levels():
        ring = []
        for j in range(cols):
            x, y = skull_xy(_theta(j, cols), z)
            ring.append((x, y, z))
        rings.append(ring)
    skull = ring_grid("skull", rings, cap_first=True, center=CENTER)
    pts = arch_outline(MOUTH_HALF, SPRING, steps=18, z0=-0.4)
    cut_rings = [[(x, y, z) for x, z in pts] for y in (-4.0, MOUTH_BACK)]
    cutter = shapes.loft("mouth-cut", cut_rings, closed=True, recalc=True)
    boolean_cut(skull, cutter)
    # soften the mouth rim (and the floor edge) only; the relief of the face is smooth already
    return bevel_edges(skull, lambda m, e: m.y > -3.9 and (abs(arch_sd(m.x, m.z)) < 0.03 or m.z < 0.02), 0.11, 3, 55.0)


def _drip(theta, centre, width, depth):
    d = (theta - centre + math.pi) % math.tau - math.pi
    return depth * math.exp(-(d / width) ** 2)


def build_cap(rng, cols=64):
    """Irregular moss cap over the dome: lumpy, with a thick rounded lip and a few drips."""
    off = (rng.uniform(-40, 40), rng.uniform(-40, 40), rng.uniform(-40, 40))
    centre = Vector(CENTER)
    drips = [(-1.95, 0.26, 0.55), (-1.0, 0.2, 0.42), (0.35, 0.28, 0.5), (2.55, 0.24, 0.55), (-2.7, 0.3, 0.4)]
    v_levels = [0.0, 0.015, 0.04, 0.08, 0.14, 0.22, 0.32, 0.44, 0.57, 0.7, 0.81, 0.9, 0.96, 1.0]
    thick = [0.0, 0.12, 0.2, 0.25, 0.28, 0.3, 0.31, 0.32, 0.32, 0.31, 0.28, 0.22, 0.14, 0.06]
    rings = []
    for i, (v, t0) in enumerate(zip(v_levels, thick)):
        ring = []
        for j in range(cols):
            theta = -math.pi + math.tau * j / cols
            front = math.cos(theta + math.pi / 2)
            z_edge = (3.98 + 0.3 * front + 0.2 * nz(theta, 0.0, 0.0, off, 1.7) + 0.08 * nz(theta, 1.0, 0.0, off, 4.2)
                      - sum(_drip(theta, c, w, d) for c, w, d in drips))
            z = z_edge + (ZTOP - z_edge) * v
            x, y = skull_xy(theta, z)
            pt = Vector((x, y, z))
            n = (pt - centre).normalized()
            lump = 1.0 + 0.65 * nz(x, y, z, off, 1.8) + (0.0 if i < 3 else 0.25 * nz(x, y, z, off, 4.4))
            pt += n * (t0 * max(0.25, lump))
            ring.append(tuple(pt))
        rings.append(ring)
    return ring_grid("moss-cap", rings, center=CENTER)
