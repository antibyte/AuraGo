"""Funnel pad of the track kit (MIT License, see ../LICENSE).

A funnel is a polar mesh around the hole: ``z(r) = z_cup + depth * (r / R) ** 1.5`` with the rim ``z(R)`` at the
height of the entry section. The outer ring is the 13 point objects of the entry section followed by evenly
spaced rim points over the rest of the circle (at about the angular step of the entry), so the lane seam needs
no welding. The entry section is a straight chord, so its middle lies up to 0.25 m inside the rim circle; the
first inner ring therefore sits ``FIRST_BAND`` (0.35 m) inside the deepest entry point. Further rings follow at equal
radial steps (about 0.22 m) down to the innermost ring, which lies exactly on ``CUP_RADIUS`` with ``CUP_SEGMENTS`` (32)
points: that polygon is the hole (``pad.cup_ring``) and the rim of the collision cup. Rings with different point
counts are joined by a zipper over their angles.
The rim carries a wall everywhere except over the entry arc.

The pad is registered in ``pads.PADS`` as ``funnel``. Its ``grid`` is ``{"polar": True, "rings", "centre",
"radius", "samples"}`` and ``faces`` holds the floor triangles as point-tuple triples (counter-clockwise seen
from above, so they face up). ``samples`` are ``(route_distance, point)`` pairs for the layout's clearance check.
"""

import math

from . import pads
from .cup_hole import zipper as _zipper
from .track import CUP_RADIUS, CUP_SEGMENTS, ROWS, Pad, lane_style

MIN_RADIUS = 2.0
MAX_RADIUS = 4.0
BAND = 0.25               # the entry section lies within this distance inside the rim circle
RING_STEP = 0.22          # radial distance between rings
FIRST_BAND = 0.35         # first inner ring: this far inside the deepest entry point
INNER_RADIUS = CUP_RADIUS
MIN_RING_POINTS = 16
MAX_RIM_SLOPE = math.tan(math.radians(22.0))
EXPONENT = 1.5


def _wrap(angle):
    return (angle + math.pi) % (2.0 * math.pi) - math.pi


def funnel(prev, cursor, radius, depth, cup=None, **style):
    """Round bowl with the hole in the middle; the lane enters over a stretch of the rim.

    ``radius`` is the rim radius (2-4 m), ``depth`` the drop from the rim to the hole (the rim slope stays
    within 22 degrees). ``cup = (x, y)`` is the centre in the pad frame (x forward, y left of the entry
    section's centre); the entry section must lie completely within 0.25 m inside the rim circle. The default
    puts the entry corners 5 cm inside the rim straight ahead of the entry. Has no exit.
    """
    if prev is None:
        raise ValueError("a funnel needs a piece before it")
    if not prev.flat:
        raise ValueError("a pad needs a flat entry: the piece before it must end level")
    for name, value in (("radius", radius), ("depth", depth)):
        if not isinstance(value, (int, float)) or value <= 0:
            raise ValueError("%s must be a positive number (got %r)" % (name, value))
    if not MIN_RADIUS <= radius <= MAX_RADIUS:
        raise ValueError("radius must be between %g and %g m (got %g)" % (MIN_RADIUS, MAX_RADIUS, radius))
    if EXPONENT * depth / radius > MAX_RIM_SLOPE:
        raise ValueError("the rim would be steeper than 22 degrees: depth must not exceed %.2f m for radius %g" % (MAX_RIM_SLOPE * radius / EXPONENT, radius))
    st = lane_style(style)
    ox, oy, oz, heading = cursor.x, cursor.y, cursor.z, cursor.heading
    fx, fy = math.cos(heading), math.sin(heading)
    lx, ly = -fy, fx
    entry = [((p[0] - ox) * fx + (p[1] - oy) * fy, (p[0] - ox) * lx + (p[1] - oy) * ly) for p in prev.points]
    if max(p[2] for p in prev.points) - min(p[2] for p in prev.points) > 1e-9:
        raise ValueError("the funnel entry must be level")
    rim_z = prev.points[0][2]
    half = math.dist(entry[0], entry[ROWS]) / 2.0
    if cup is None:
        cup = (math.sqrt((radius - 0.05) ** 2 - half ** 2), 0.0)
    cx, cy = float(cup[0]), float(cup[1])
    if cx < radius / 2.0:
        raise ValueError("the funnel centre must lie ahead of the entry (x >= radius / 2)")
    for k, (x, y) in enumerate(entry):
        d = math.hypot(x - cx, y - cy)
        if not radius - BAND - 1e-9 <= d <= radius + 1e-9:
            raise ValueError("entry point %d is %.3f m from the funnel centre, must be %.2f-%g m (change radius or cup)" % (k, d, radius - BAND, radius))
    world = lambda x, y, z: (ox + fx * x + lx * y, oy + fy * x + ly * y, z)
    # outer ring: the entry points (counter-clockwise from the left) and evenly spaced rim points over the rest
    base = math.atan2(entry[ROWS // 2][1] - cy, entry[ROWS // 2][0] - cx)
    angles = [base + _wrap(math.atan2(y - cy, x - cx) - base) for x, y in entry]
    if any(b <= a for a, b in zip(angles, angles[1:])) or angles[ROWS] - angles[0] >= math.pi:
        raise ValueError("the entry points do not follow each other around the funnel centre")
    span = angles[ROWS] - angles[0]
    step = span / ROWS
    rest = 2.0 * math.pi - span
    count = max(3, round(rest / step))
    ring0 = [(angles[k], prev.points[k]) for k in range(ROWS + 1)]
    ring0 += [(angles[ROWS] + rest * k / count, world(cx + radius * math.cos(angles[ROWS] + rest * k / count),
                                                     cy + radius * math.sin(angles[ROWS] + rest * k / count), rim_z)) for k in range(1, count)]
    z_cup = rim_z - depth
    first = min(radius - RING_STEP, min(math.hypot(x - cx, y - cy) for x, y in entry) - FIRST_BAND)
    steps = max(1, math.ceil((first - INNER_RADIUS) / RING_STEP - 1e-9))
    radii = [first - (first - INNER_RADIUS) * k / steps for k in range(steps)] + [INNER_RADIUS]
    rings, triangles = [ring0], []
    for k, r in enumerate(radii):
        z = z_cup + depth * (r / radius) ** EXPONENT
        n = CUP_SEGMENTS if k == steps else max(MIN_RING_POINTS, round(2.0 * math.pi * r / (step * radius)))
        ring = [(a, world(cx + r * math.cos(a), cy + r * math.sin(a), z)) for a in (angles[0] + 2.0 * math.pi * j / n for j in range(n))]
        _zipper(rings[-1], ring, triangles)
        rings.append(ring)
    rim_z = z_cup + depth * (CUP_RADIUS / radius) ** EXPONENT
    centre = world(cx, cy, rim_z)           # the hole: its rim height is the cup height
    walls = []
    if st["walls"] != (False, False):
        rim = [prev.points[ROWS]] + [p for _, p in ring0[ROWS + 1:]] + [prev.points[0]]
        for a, b in zip(rim, rim[1:]):
            nx, ny = (a[0] + b[0]) / 2 - centre[0], (a[1] + b[1]) / 2 - centre[1]
            length = math.hypot(nx, ny)
            walls.append((a, b, (nx / length, ny / length, 0.0)))
    samples = [(math.hypot(p[0] - prev.center[0], p[1] - prev.center[1]), p) for _, p in ring0]
    samples += [(max(0.0, cx - d), world(cx - d, cy, z_cup + depth * (d / radius) ** EXPONENT)) for d in (radius * k / 8 for k in range(1, 8))]
    grid = {"polar": True, "rings": [[p for _, p in ring] for ring in rings], "centre": centre, "radius": radius, "samples": samples}
    ports = [{"side": "entry", "cols": (0, ROWS), "section": prev}]
    pad = Pad("funnel", grid, triangles, st["surface"], st["look"], walls, ports, (ox, oy, oz, heading), (2.0 * radius, 2.0 * radius),
              cup=centre, holes=[{"kind": "cup", "center": centre, "radius": CUP_RADIUS}], cup_ring=[p for _, p in rings[-1]],
              grounded=st["grounded"])
    return pad, None, cursor


PADS = {"funnel": funnel}
pads.PADS.update(PADS)
