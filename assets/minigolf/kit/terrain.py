"""Height of the diorama island under a track (MIT License, see ../LICENSE).

``Terrain.top_height(x, y)`` (Blender X/Y/Z, metres) is

* the base ground (a little noise, sinking to the rim),
* pulled to a bed 6 cm under the nearest grounded floor triangle of the whole track, smoothed from 1.1 m to
  2.0 m away (``bed_field``); elevated pieces do not pull the ground up, and where a second stretch of the track
  passes close by the ground never stands above its bed,
* a hollow under the cup,
* a hollow under every pond (the water surface is 0.25 m under the floor, the bottom another 0.45 m lower) and a
  chasm under every void pool,
* a trench under every ``gap`` hazard: a strip across the whole island (down below the water line, on the moon
  through the island) that stops short where another stretch of the track passes through it.

Colours and the island mesh are in ``dress_island``.
"""

import math
import random

from mathutils import Vector

from . import palette
from .bed_field import BedField, grounded_triangles
from .compose import fbm
from .island import smoothstep

WATER_Z = -1.15
BED_DROP = 0.06
BED_NEAR, BED_FAR = 1.1, 2.0
SCAN_REACH = 2.4          # farther than this a point counts as "far from the track" (colours fade out before)
SAME_STRETCH = 3.0        # route metres: floors this close along the route belong to the same stretch of the track
# The hollow under the cup is wider than the island's top grid cell (0.35 m) so the cup tube never meets the ground:
# every grid cell that touches the tube (radius 0.32) has all corners at the bottom, 4 cm below the tube bottom (CUP_DEPTH 0.30).
HOLLOW_FLAT, HOLLOW_OUTER, HOLLOW_DEPTH = 0.85, 1.2, 0.34
POND_DROP, POND_DEPTH = 0.25, 0.45
POOL_IN, POOL_OUT = 0.30, 0.15     # a pool bank slopes from this far inside its polygon to this far outside
TRENCH_Z = {"pirate": WATER_Z - 0.3, "temple": WATER_Z - 0.3, "moon": -3.0}
TRENCH_PAD, TRENCH_SOFT = 0.05, 0.12
DAM_LATERAL = (2.4, 3.2)  # beyond this lateral distance from the lane a trench gives way to other stretches of the track


def clamp01(v):
    return 0.0 if v < 0.0 else (1.0 if v > 1.0 else v)


def polygon_distance(polygon, x, y):
    """Signed distance of ``(x, y)`` to a polygon: negative inside."""
    inside, best = False, math.inf
    for k in range(len(polygon)):
        (x0, y0), (x1, y1) = polygon[k], polygon[(k + 1) % len(polygon)]
        if (y0 > y) != (y1 > y) and x < x0 + (y - y0) * (x1 - x0) / (y1 - y0):
            inside = not inside
        dx, dy = x1 - x0, y1 - y0
        length2 = dx * dx + dy * dy
        t = 0.0 if length2 < 1e-18 else max(0.0, min(1.0, ((x - x0) * dx + (y - y0) * dy) / length2))
        best = min(best, math.hypot(x - x0 - t * dx, y - y0 - t * dy))
    return -best if inside else best


class Terrain:
    """Height of the island top for one track."""

    def __init__(self, look, number, track, outline):
        self.look = look
        self.pal = palette.get(look)
        self.cup = track.cup
        self.outline = outline
        rng = random.Random(4000 + number)
        self.off = Vector((rng.uniform(-40, 40), rng.uniform(-40, 40), 0.0))
        self.field = BedField(grounded_triangles(track))
        self.gaps = [self._strip(h) for h in track.hazards if "rect" in h]
        self.pools = [self._pool(h) for h in track.hazards if "polygon" in h]
        self._memo = None
        self.patches = self._patches(rng)
        self.layers = None          # set by dress_island (strata need the palette)

    # -- set-up ---------------------------------------------------------------

    @staticmethod
    def _strip(hazard):
        rect = hazard["rect"]
        mx, my = rect["center"]
        return (mx, my, math.cos(rect["heading"]), math.sin(rect["heading"]), rect["half"][0] + TRENCH_PAD)

    def _pool(self, hazard):
        polygon = [tuple(p) for p in hazard["polygon"]]
        floor = TRENCH_Z[self.look] if hazard["kind"] == "void" else hazard["z"] - POND_DROP - POND_DEPTH
        return {"polygon": polygon, "floor": floor, "box": (hazard["min"][0] - 1.0, hazard["min"][1] - 1.0, hazard["max"][0] + 1.0,
                                                           hazard["max"][1] + 1.0)}

    def _patches(self, rng):
        """Grass patches away from the track: a few per 9 m^2 of island, seeded."""
        count = max(3, int(self.outline.half[0] * self.outline.half[1] / 9.0))
        cx, cy = self.outline.center.x, self.outline.center.y
        hx, hy = self.outline.half[0] * 0.9, self.outline.half[1] * 0.9
        out = []
        for _ in range(200):
            if len(out) >= count:
                break
            x, y = cx + rng.uniform(-hx, hx), cy + rng.uniform(-hy, hy)
            if self.outline.contains_signed(x, y) < -1.0 and self.lane_distance(x, y) > 2.2:
                out.append((x, y, rng.uniform(0.7, 1.3)))
        return out

    # -- queries --------------------------------------------------------------

    def _scan(self, x, y):
        """``(distance, bed_z, others)``: the nearest grounded floor (``bed_z`` 6 cm below it) and the
        ``(distance, bed_z)`` of other stretches of the track within ``BED_FAR``; no floor near: ``(99, None, [])``."""
        memo = self._memo
        if memo is not None and memo[0] == x and memo[1] == y:
            return memo[2]
        hits = self.field.nearest(x, y, SCAN_REACH)
        if not hits:
            result = (99.0, None, [])
        else:
            best = min(hits, key=lambda h: h[0])
            others = [(d, z - BED_DROP) for d, z, arc in hits if abs(arc - best[2]) > SAME_STRETCH and d < BED_FAR]
            result = (best[0], best[1] - BED_DROP, others)
        self._memo = (x, y, result)
        return result

    def lane_distance(self, x, y):
        """Plan distance to the nearest grounded floor (99 when none is within ``SCAN_REACH``)."""
        return self._scan(x, y)[0]

    def in_trench(self, x, y, soft=TRENCH_SOFT):
        """True inside the strip of a ``gap`` hazard (``soft`` metres of margin)."""
        return any(abs((x - mx) * fx + (y - my) * fy) <= half + soft for mx, my, fx, fy, half in self.gaps)

    def wet(self, x, y, margin=0.0):
        """True where the ground is a trench or a pool (``margin`` metres around): nothing may stand there."""
        if self.in_trench(x, y, margin + TRENCH_SOFT):
            return True
        for pool in self.pools:
            x0, y0, x1, y1 = pool["box"]
            if x0 - margin <= x <= x1 + margin and y0 - margin <= y <= y1 + margin and polygon_distance(pool["polygon"], x, y) <= margin + POOL_OUT:
                return True
        return False

    def top_height(self, x, y, edge=None):
        if edge is None:
            edge = -self.outline.contains_signed(x, y)
        z0 = -0.04 + 0.03 * fbm(x, y, 0.4, self.off) * smoothstep(edge, 0.1, 1.1)
        z0 -= 0.06 * (1.0 - smoothstep(edge, 0.0, 0.32))
        d, bed, others = self._scan(x, y)
        z = z0 if bed is None else z0 + (bed - z0) * (1.0 - smoothstep(d, BED_NEAR, BED_FAR))
        for d_o, bed_o in others:
            # A second stretch of the track close by (hairpin, fork branch): never stand above its bed.
            z = min(z, bed_o + smoothstep(d_o, BED_NEAR, BED_NEAR + 0.6))
        cx, cy, cz = self.cup
        r = math.hypot(x - cx, y - cy)
        if r < HOLLOW_OUTER:
            floor = cz - HOLLOW_DEPTH
            z = floor + (z - floor) * smoothstep(r, HOLLOW_FLAT, HOLLOW_OUTER)
        for pool in self.pools:
            x0, y0, x1, y1 = pool["box"]
            if x0 <= x <= x1 and y0 <= y <= y1:
                k = 1.0 - smoothstep(polygon_distance(pool["polygon"], x, y), -POOL_IN, POOL_OUT)
                if k > 0.0:
                    z += (pool["floor"] - z) * k
        for mx, my, fx, fy, half in self.gaps:
            along = (x - mx) * fx + (y - my) * fy
            k = 1.0 - smoothstep(abs(along), half, half + TRENCH_SOFT)
            if k <= 0.0:
                continue
            lateral = abs(-(x - mx) * fy + (y - my) * fx)
            dam = (1.0 - smoothstep(d, BED_NEAR, BED_FAR)) * smoothstep(lateral, *DAM_LATERAL)
            k *= 1.0 - dam
            z += (TRENCH_Z[self.look] - z) * k
        return z
