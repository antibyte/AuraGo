"""Tunnel roofs and portals (MIT License, see ../LICENSE).

``build(look, tunnels, number)`` makes, for every record of ``tunnel_plan.plan``:

* a roof shell over the stretch behind the entrance portal (the whole run on the moon): an arch band swept along the
  lane, the inner opening ``tunnel_plan.HALF`` wide to each side and ``CROWN`` high over the floor, dark inside, rock
  (pirate), mossy stone (temple) or paneled metal with orange bands (moon) outside, with a thick collar at the exit;
* the entrance portal: ``portal_pirate`` (skull rock), ``portal_temple`` (stone head with the open mouth) or, on the
  moon, a collar ring at the entrance of the shell.

The portals are built in a local frame (x right, y forward, z up, origin at the middle of the mouth on the floor)
and moved onto the lane by ``place``. Nothing here has collision, see ``tunnel_plan``.
"""

import math

from mathutils import Matrix, Vector, noise

from . import colors, materials, palette, portal_pirate, portal_temple, shapes
from . import part_utils as pu
from .palette import mix
from .tunnel_plan import CROWN, FOOT, HALF, ROOF, SPRING

ARCH_STEPS = 8
RING_STEP = 0.45
COLLAR = 0.34               # length of the thick collar at the exit (and the entrance on the moon)
COLLAR_EXTRA = 0.16
ROCK = {"pirate": 0.11, "temple": 0.05, "moon": 0.0}      # how far the outside of the roof is pushed by noise (metres)


def profile(extra=0.0, steps=ARCH_STEPS):
    """Closed ``(x right, z up)`` outline of the roof band: the inner arch from left to right, then the outer one back."""
    rise = CROWN - SPRING
    inner = [(-HALF, -0.05), (-HALF, SPRING)]
    inner += [(-HALF * math.cos(math.pi * k / steps), SPRING + rise * math.sin(math.pi * k / steps)) for k in range(1, steps)]
    inner += [(HALF, SPRING), (HALF, -0.05)]
    wide, high = HALF + 0.45 + extra, rise + ROOF + extra
    outer = [(HALF + FOOT + extra, -0.25), (HALF + 0.52 + extra, 0.25), (wide, SPRING)]
    outer += [(wide * math.cos(math.pi * k / steps), SPRING + high * math.sin(math.pi * k / steps)) for k in range(1, steps)]
    outer += [(-wide, SPRING), (-(HALF + 0.52 + extra), 0.25), (-(HALF + FOOT + extra), -0.25)]
    return inner, outer


def _rough(point, origin, right, up, x, z, off, amount):
    """Push an outer roof point along its outward direction by a little noise (rock, mossy stone)."""
    out = pu.unit(pu.at((0.0, 0.0, 0.0), (right, x), (up, z - SPRING)))
    k = amount * noise.noise(Vector((point[0] * 1.9, point[1] * 1.9, point[2] * 1.9)) + off) * min(1.0, (z - 0.3) / 0.5)
    return pu.at(point, (out, k))


def _ring_arcs(first, length):
    """Route lengths of the shell rings from ``first`` to ``length``, at most ``RING_STEP`` apart; the collar rings are added by the caller."""
    n = max(1, math.ceil((length - first) / RING_STEP))
    return [first + (length - first) * k / n for k in range(n + 1)]


def _outer_colour(look, pal, x, y, z, rel_z, s, off):
    n = noise.noise(Vector((x * 1.6, y * 1.6, z * 1.6)) + off)
    if look == "pirate":
        c = mix(pal.c("rock_dark"), pal.c("rock_light"), min(1.0, max(0.0, (rel_z + 0.25) / 2.1)) ** 0.9)
        c = mix(c, pal.c("sand_wet"), 0.55 * max(0.0, 1.0 - (rel_z + 0.2) / 0.5))      # damp sand at the foot
        k = 1.0 + 0.2 * n + 0.1 * noise.noise(Vector((x * 5.0, y * 5.0, z * 5.0)) + off)
        return (c[0] * k, c[1] * k, c[2] * k, 1.0)
    if look == "temple":
        c = mix(pal.c("stone_dark"), pal.c("stone"), min(1.0, max(0.0, (rel_z + 0.25) / 1.2)))
        k = 1.0 + 0.1 * n
        c = (c[0] * k, c[1] * k, c[2] * k, 1.0)
        moss = min(1.0, max(0.0, (rel_z - 1.0 + 0.35 * n) / 0.7)) * 0.8
        return mix(c, mix(pal.c("moss_dark"), pal.c("moss_light"), 0.5 + 0.5 * n), moss)
    c = mix(pal.c("panel_grey"), pal.c("panel_white"), min(1.0, max(0.0, (rel_z + 0.25) / 2.0)))
    return mix(c, pal.c("orange"), 0.85) if (s % 1.0) < 0.1 else c


def _inner_colour(look, pal, s, depth):
    """Dark inside: lighter close to the entrance, black deeper in (``depth`` is the route length from the first ring)."""
    base = pal.c({"pirate": "rock_dark", "temple": "stone_deep", "moon": "metal_dark"}[look])
    black = pal.c("coal" if look == "pirate" else "black")
    return mix(mix(base, black, 0.45), black, min(1.0, depth / 2.2) * 0.8)


def _shell(look, tunnel, number, index):
    """The roof shell object of one tunnel (None when the portal covers the whole run)."""
    chain, pal = tunnel["chain"], palette.get(look)
    first = 0.0 if look == "moon" else tunnel["depth"] - 0.12
    if chain.length - first < 0.2:
        return None
    arcs = _ring_arcs(first, chain.length)
    stations = [(s, 0.0) for s in arcs]
    # thick collar at the exit (and entrance on the moon): two rings share the step so the change is a short chamfer
    end = chain.length
    stations = [(s, e) for s, e in stations if s < end - COLLAR - 0.05] + [(end - COLLAR - 0.04, 0.0), (end - COLLAR, COLLAR_EXTRA), (end, COLLAR_EXTRA)]
    if look == "moon":
        stations = [(0.0, COLLAR_EXTRA), (COLLAR, COLLAR_EXTRA), (COLLAR + 0.04, 0.0)] + [(s, e) for s, e in stations if s > COLLAR + 0.1]
    stations.sort(key=lambda se: (se[0], se[1]))
    off = Vector((number * 3.1 + index * 7.7, 11.0, -5.0))
    rings, cols = [], []
    for s, extra in stations:
        origin, forward, across, up = chain.frame(s)
        right = (-across[0], -across[1], -across[2])
        inner, outer = profile(extra)
        ring = []
        for kind, pts in (("in", inner), ("out", outer)):
            for x, z in pts:
                point = pu.at(origin, (right, x), (up, z))
                if kind == "out" and z > 0.3 and ROCK[look]:
                    point = _rough(point, origin, right, up, x, z, off, ROCK[look])
                ring.append(point)
                if kind == "in":
                    cols.append(_inner_colour(look, pal, s, s - first))
                else:
                    cols.append(_outer_colour(look, pal, point[0], point[1], point[2], z, s, off))
        rings.append(ring)
    obj = shapes.loft("tunnel-roof", rings, closed=True, cap_start=True, cap_end=True, recalc=True)
    shapes.smooth_by_angle(obj.data, 50.0)
    colors.paint_values(obj, cols, ao=0.7)
    obj.data.materials.append(materials.get("satin" if look == "moon" else "matte"))
    return obj


def place(obj, chain):
    """Move a portal built in the local frame onto the entrance of ``chain`` (horizontal, as the lane is straight and level there)."""
    origin, forward, across, up = chain.frame(0.0)
    f = Vector((forward[0], forward[1], 0.0)).normalized()
    r = f.cross(Vector((0.0, 0.0, 1.0)))
    matrix = Matrix(((r.x, f.x, 0.0, origin[0]), (r.y, f.y, 0.0, origin[1]), (r.z, f.z, 1.0, origin[2]), (0.0, 0.0, 0.0, 1.0)))
    obj.data.transform(matrix)
    return obj


def build(look, tunnels, number):
    """All tunnel objects (roof shells and portals) of a course; baked with the island."""
    out = []
    for index, tunnel in enumerate(tunnels):
        shell = _shell(look, tunnel, number, index)
        if shell is not None:
            out.append(shell)
        if look == "pirate":
            out.append(place(portal_pirate.build(number + index, index), tunnel["chain"]))
        elif look == "temple":
            out.append(place(portal_temple.build(number + index, index), tunnel["chain"]))
    return out
