"""Diorama island of a dressed course (MIT License, see ../LICENSE).

``Terrain`` knows the legacy lane (path samples, hill, bank, gap, cup) and
answers ``top_height(x, y)``: base ground with a little noise, a bed 6 cm under
the lane that follows hills and banked bends, a hollow under the cup and a
trench across the whole island where the lane has a gap. ``build`` turns it into
the island mesh (strata walls, closed underside) and the optional ``water-shore``
strip; colours are written to the POINT ``palette`` / ``ao_w`` attributes so
``bake.process`` can finish them.
"""

import math
import random

from mathutils import Vector

from . import colors, materials, palette, shapes
from .compose import fbm
from .courses_v1 import LANE_WIDTH, lane_center_height
from .island import Layer, Outline, build_island, smoothstep
from .palette import mix

WATER_Z = -1.15
BED_DROP = 0.06
BED_NEAR, BED_FAR = 1.1, 2.0
SAME_STRETCH = 10  # path samples: segments this close in index belong to the same stretch of lane
HOLLOW_FLAT, HOLLOW_OUTER, HOLLOW_DEPTH = 0.36, 0.45, 0.34
TRENCH_Z = {"pirate": WATER_Z - 0.3, "temple": WATER_Z - 0.3, "moon": -3.0}
TRENCH_PAD, TRENCH_SOFT = 0.05, 0.12
TIP = {"pirate": -2.4, "temple": -2.2, "moon": -3.2}
GRID = (56, 60)
SHORE_WIDTH = 0.9
HALF = LANE_WIDTH / 2
HOLE_DARK = (0.012, 0.016, 0.02, 1.0)


def clamp01(v):
    return 0.0 if v < 0.0 else (1.0 if v > 1.0 else v)


class Terrain:
    """Height and colour of the island top for one course (Blender X/Y/Z, metres)."""

    def __init__(self, look, number, course, path, cup):
        self.look = look
        self.pal = palette.get(look)
        self.cup = cup
        rng = random.Random(4000 + number)
        self.off = Vector((rng.uniform(-40, 40), rng.uniform(-40, 40), 0.0))
        self.outline = Outline(center=(0, 0.1), half=(4.9, 5.35), power=2.6, wobble=0.05, seed=number)
        self.samples = self._samples(course, path)
        self.gap = self._gap(course, path)
        self._memo = None
        self.patches = self._patches(rng, 3)
        self.layers = layers_for(look, self.pal)

    # -- lane geometry ------------------------------------------------------

    @staticmethod
    def _samples(course, path):
        n = len(path)
        bank = course.get("bank", 0)
        out = []
        for i, p in enumerate(path):
            prev, nxt = path[max(0, i - 1)], path[min(n - 1, i + 1)]
            dx, dy = nxt[0] - prev[0], nxt[1] - prev[1]
            ln = max(.0001, math.hypot(dx, dy))
            t = i / (n - 1)
            slope = bank * math.sin(math.pi * max(0, min(1, (t - .18) / .66))) if bank else 0.0
            out.append((p[0], p[1], dy / ln, -dx / ln, lane_center_height(course, path, i), slope))
        return out

    @staticmethod
    def _gap(course, path):
        gap = course.get("gap")
        if not gap:
            return None
        n = len(path)
        skipped = [i for i in range(n - 1) if gap[0] <= (i + .5) / (n - 1) <= gap[1]]
        if not skipped:
            return None
        a, b = path[skipped[0]], path[skipped[-1] + 1]
        length = math.hypot(b[0] - a[0], b[1] - a[1])
        return ((a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (b[0] - a[0]) / length, (b[1] - a[1]) / length,
                length / 2 + TRENCH_PAD)

    def _patches(self, rng, count):
        out = []
        for _ in range(200):
            if len(out) >= count:
                break
            x, y = rng.uniform(-4.4, 4.4), rng.uniform(-4.8, 5.0)
            if self.outline.contains_signed(x, y) < -1.0 and self.lane_distance(x, y) > 2.2:
                out.append((x, y, rng.uniform(0.7, 1.3)))
        return out

    def _bed_of(self, hit):
        """``(distance, bed_z)`` of one scan hit: the lane surface height 6 cm lower, level outwards from the edge."""
        d2, k, u, px, py, x, y = hit
        a, b = self.samples[k], self.samples[k + 1]
        nx, ny = a[2] + (b[2] - a[2]) * u, a[3] + (b[3] - a[3]) * u
        lateral = ((x - px) * nx + (y - py) * ny) / max(1e-9, math.hypot(nx, ny))
        h = a[4] + (b[4] - a[4]) * u
        slope = a[5] + (b[5] - a[5]) * u
        return math.sqrt(d2), h + slope * max(-HALF, min(HALF, lateral)) - BED_DROP

    def _scan(self, x, y):
        """``(distance, bed_z, others)`` for the nearest lane segment; ``others`` are the (distance, bed_z)
        of segments of a different stretch of the lane (a hairpin or loop passing close by)."""
        memo = self._memo
        if memo is not None and memo[0] == x and memo[1] == y:
            return memo[2]
        s = self.samples
        far2 = BED_FAR * BED_FAR
        best = None
        found = []
        for k in range(len(s) - 1):
            ax, ay, bx, by = s[k][0], s[k][1], s[k + 1][0], s[k + 1][1]
            vx, vy = bx - ax, by - ay
            l2 = vx * vx + vy * vy
            u = 0.0 if l2 < 1e-12 else max(0.0, min(1.0, ((x - ax) * vx + (y - ay) * vy) / l2))
            px, py = ax + u * vx, ay + u * vy
            d2 = (x - px) ** 2 + (y - py) ** 2
            hit = (d2, k, u, px, py, x, y)
            if best is None or d2 < best[0]:
                best = hit
            if d2 < far2:
                found.append(hit)
        d, zb = self._bed_of(best)
        others = [self._bed_of(h) for h in found if abs(h[1] - best[1]) > SAME_STRETCH]
        result = (d, zb, others)
        self._memo = (x, y, result)
        return result

    def lane_distance(self, x, y):
        return self._scan(x, y)[0]

    def in_trench(self, x, y, soft=TRENCH_SOFT):
        if not self.gap:
            return False
        mx, my, tx, ty, half = self.gap
        return abs((x - mx) * tx + (y - my) * ty) <= half + soft

    # -- height -------------------------------------------------------------

    def top_height(self, x, y, edge=None):
        if edge is None:
            edge = -self.outline.contains_signed(x, y)
        z0 = -0.04 + 0.03 * fbm(x, y, 0.4, self.off) * smoothstep(edge, 0.1, 1.1)
        z0 -= 0.06 * (1.0 - smoothstep(edge, 0.0, 0.32))
        d, zb, others = self._scan(x, y)
        z = z0 + (zb - z0) * (1.0 - smoothstep(d, BED_NEAR, BED_FAR))
        for d_o, zb_o in others:
            # A second stretch of the lane close by (hairpin): never stand above its bed.
            z = min(z, zb_o + smoothstep(d_o, BED_NEAR, BED_NEAR + 0.6))
        cx, cy, cz = self.cup
        r = math.hypot(x - cx, y - cy)
        if r < HOLLOW_OUTER:
            floor = cz - HOLLOW_DEPTH
            z = floor + (z - floor) * smoothstep(r, HOLLOW_FLAT, HOLLOW_OUTER)
        if self.gap:
            mx, my, tx, ty, half = self.gap
            k = 1.0 - smoothstep(abs((x - mx) * tx + (y - my) * ty), half, half + TRENCH_SOFT)
            if k > 0.0:
                z += (TRENCH_Z[self.look] - z) * k
        return z

    # -- colour -------------------------------------------------------------

    def strata_color(self, z):
        top_z = -0.1
        for layer in self.layers:
            if z >= layer.z:
                return mix(layer.top, layer.bottom, clamp01((top_z - z) / max(1e-6, top_z - layer.z)))
            top_z = layer.z
        return self.layers[-1].bottom

    def ground_color(self, x, y, z, edge):
        c = _GROUND[self.look](self, x, y, z, edge)
        wall = smoothstep(-z, 0.22, 0.4)
        if wall > 0.0:
            c = mix(c, self.strata_color(z), wall)
        # The cup hollow shows through the lane cut-out around the cup: keep it dark like the inside of a hole.
        hole = 1.0 - smoothstep(math.hypot(x - self.cup[0], y - self.cup[1]), HOLLOW_OUTER + 0.15, HOLLOW_OUTER + 0.35)
        return mix(c, HOLE_DARK, 0.9 * hole) if hole > 0.0 else c


def _ground_pirate(t, x, y, z, edge):
    pal = t.pal
    n = fbm(x, y, 0.9, t.off * 0.7)
    c = mix(pal.c("sand"), pal.c("sand_light"), clamp01(0.5 + 1.6 * (z + 0.04) + 0.25 * n))
    c = mix(c, pal.c("sand_wet"), (1.0 - smoothstep(edge, 0.05, 0.9)) * 0.8)
    d = t.lane_distance(x, y)
    c = mix(c, pal.c("sand_wet"), 0.30 * (1.0 - smoothstep(d, 1.0, 2.2)))
    gm = 0.0
    for gx, gy, gr in t.patches:
        ragged = gr * (0.8 + 0.35 * fbm(x, y, 1.1, t.off * 2.7))
        gm = max(gm, smoothstep(ragged - math.hypot(x - gx, y - gy), 0.0, 0.3 * gr))
    gm *= smoothstep(edge, 0.5, 1.2) * smoothstep(d, 1.1, 1.5)
    if gm > 0.0:
        lo, hi = pal.grad("grass")
        c = mix(c, mix(lo, hi, clamp01(0.5 + 0.55 * n)), gm)
    return c


def _ground_temple(t, x, y, z, edge):
    pal = t.pal
    n = fbm(x, y, 0.8, t.off * 0.7)
    c = mix(pal.c("moss_dark"), pal.c("moss"), clamp01(0.5 + 0.8 * n))
    c = mix(c, pal.c("jungle"), 0.45 * smoothstep(fbm(x, y, 0.35, t.off * 2.0), 0.0, 0.5))
    c = mix(c, pal.c("yellow_green"), 0.35 * smoothstep(fbm(x, y, 0.6, t.off * 3.1), 0.25, 0.6))
    d = t.lane_distance(x, y)
    path = 1.0 - smoothstep(d, 1.0, 2.1)
    c = mix(c, mix(pal.c("dirt"), pal.c("dirt_light"), 0.5 + 0.5 * n), path * 0.85)
    return mix(c, pal.c("dirt_dark"), (1.0 - smoothstep(edge, 0.05, 0.8)) * 0.5)


def _ground_moon(t, x, y, z, edge):
    pal = t.pal
    n = fbm(x, y, 0.9, t.off * 0.7)
    c = mix(pal.c("regolith"), pal.c("regolith_light"), clamp01(0.5 + 0.9 * n))
    c = mix(c, pal.c("regolith_dark"), 0.45 * smoothstep(fbm(x, y, 0.4, t.off * 2.0), 0.1, 0.5))
    d = t.lane_distance(x, y)
    c = mix(c, pal.c("regolith_dark"), 0.18 * (1.0 - smoothstep(d, 1.0, 2.2)))
    if fbm(x * 6.0, y * 6.0, 1.0, t.off, octaves=1) > 0.55:
        c = mix(c, pal.c("dust"), 0.5)
    return c


_GROUND = {"pirate": _ground_pirate, "temple": _ground_temple, "moon": _ground_moon}


def layers_for(look, pal):
    """Three strata per world plus a submerged skirt (water worlds) or the floating underside (moon)."""
    if look == "pirate":
        sand, wet = pal.c("sand"), pal.c("sand_wet")
        return [
            Layer(-0.40, mix(sand, wet, 0.2), wet, off=0.07, bulge=0.04, noise_amp=0.012, steps=2),
            Layer(-0.85, pal.c("soil_light"), pal.c("soil"), off=0.035, bulge=0.03, noise_amp=0.02, steps=2,
                  ledge=0.035, speck=(pal.c("rock_light"), 0.35)),
            Layer(WATER_Z, pal.c("rock_light"), pal.c("rock"), off=0.2, bulge=0.05, noise_amp=0.05, steps=2,
                  ledge=0.07, speck=(pal.c("rock_dark"), 0.3)),
            Layer(-1.9, pal.c("rock_dark"), pal.c("rock_dark"), off=0.12, bulge=0.0, noise_amp=0.03, steps=1),
        ]
    if look == "temple":
        seam = pal.c("stone_deep")
        moss, moss_l = pal.c("moss"), pal.c("moss_light")
        return [
            Layer(-0.35, mix(moss, moss_l, 0.4), pal.c("moss_dark"), off=0.07, bulge=0.05, noise_amp=0.02, steps=2),
            Layer(-0.75, pal.c("dirt_light"), pal.c("dirt"), off=0.03, bulge=0.025, noise_amp=0.02, steps=2,
                  ledge=0.02, speck=(pal.c("stone"), 0.4)),
            Layer(WATER_Z, pal.c("stone_light"), pal.c("stone_dark"), off=0.12, bulge=0.02, noise_amp=0.01, steps=3,
                  ledge=0.07, blocks=(0.95, seam, 0.0)),
            Layer(-1.8, seam, seam, off=0.1, bulge=0.0, noise_amp=0.02, steps=1),
        ]
    reg, reg_l, reg_d = pal.c("regolith"), pal.c("regolith_light"), pal.c("regolith_dark")
    deep, black, dust = pal.c("regolith_deep"), pal.c("black"), pal.c("dust")
    return [
        Layer(-0.30, mix(reg_l, dust, 0.3), reg, off=0.06, bulge=0.05, noise_amp=0.02, steps=2),
        Layer(-0.85, reg, reg_d, off=0.03, bulge=0.03, noise_amp=0.03, steps=2, ledge=0.03, speck=(dust, 0.35)),
        Layer(-1.6, reg_d, deep, off=0.06, bulge=0.04, noise_amp=0.05, steps=3, ledge=0.05, scale=0.9,
              speck=(reg_l, 0.4)),
        Layer(-2.5, mix(deep, black, 0.4), mix(deep, black, 0.7), off=0.0, bulge=0.04, noise_amp=0.08, steps=2,
              scale=0.5),
    ]


def build(terrain, number):
    """Build the island and return ``(island_obj, waterline)`` (waterline empty on the moon)."""
    look = terrain.look
    nx, ny = GRID

    def height(x, y, edge):
        return terrain.top_height(x, y)

    obj, waterline = build_island(terrain.outline, nx, ny, height, terrain.ground_color, terrain.layers,
                                  WATER_Z if look != "moon" else -100.0, seed=number, tip=TIP[look],
                                  clamp_down=True)
    obj.data.materials.append(materials.get("matte"))
    return obj, waterline


def build_shore(terrain, waterline):
    """Flat strip 0 to 0.9 m outwards from the waterline; ``uv.x`` is 0 at the line and 1 outside."""
    pal = terrain.pal
    inner_col, outer_col = ((pal.c("lagoon_shore"), pal.c("lagoon_light")) if terrain.look == "pirate"
                            else (pal.c("water_light"), pal.c("water")))
    points = waterline[::2]
    m = len(points)
    verts, uvs, cols, faces = [], [], [], []
    arc = 0.0
    for k in range(m):
        a, b = points[(k - 1) % m], points[(k + 1) % m]
        nx, ny = b[1] - a[1], -(b[0] - a[0])
        ln = max(1e-9, math.hypot(nx, ny))
        nx, ny = nx / ln, ny / ln
        if k:
            arc += math.hypot(points[k][0] - points[k - 1][0], points[k][1] - points[k - 1][1])
        x, y = points[k]
        z = WATER_Z + 0.01
        verts += [(x, y, z), (x + nx * SHORE_WIDTH, y + ny * SHORE_WIDTH, z)]
        uvs += [(0.0, arc), (1.0, arc)]
        cols += [inner_col, outer_col]
    for k in range(m):
        k2 = (k + 1) % m
        if terrain.in_trench(*points[k], soft=0.3) or terrain.in_trench(*points[k2], soft=0.3):
            continue
        faces.append((2 * k, 2 * k + 1, 2 * k2 + 1, 2 * k2))
    obj = shapes.make_object("water-shore", verts, faces, smooth=False)
    mesh = obj.data
    layer = mesh.uv_layers.new(name="UVMap")
    for loop in mesh.loops:
        layer.data[loop.index].uv = uvs[loop.vertex_index]
    colors.paint_values(obj, cols, ao=0.0)
    obj.data.materials.append(materials.get("water"))
    return obj
