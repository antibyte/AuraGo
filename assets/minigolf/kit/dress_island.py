"""Diorama island of a dressed course (MIT License, see ../LICENSE).

``terrain.Terrain`` answers ``top_height(x, y)`` for the track (bed under the grounded floor, cup hollow, ponds,
trenches). ``build`` turns it into the island mesh (strata walls, closed underside) with a top grid of about
0.35 m cells and the optional ``water-shore`` strip; colours are written to the POINT ``palette`` / ``ao_w``
attributes so ``bake.process`` can finish them.
"""

import math

from . import colors, materials, shapes
from .compose import fbm
from .island import Layer, build_island, smoothstep
from .palette import mix
from .terrain import HOLLOW_OUTER, WATER_Z, Terrain, clamp01  # noqa: F401  (Terrain: re-exported)

TIP = {"pirate": -2.4, "temple": -2.2, "moon": -3.2}
CELL = 0.35               # top grid cell in metres
MAX_TOP_QUADS = 4400      # larger islands get coarser cells to stay inside the triangle budget
SHORE_WIDTH = 0.9
HOLE_DARK = (0.012, 0.016, 0.02, 1.0)


def strata_color(t, z):
    top_z = -0.1
    for layer in t.layers:
        if z >= layer.z:
            return mix(layer.top, layer.bottom, clamp01((top_z - z) / max(1e-6, top_z - layer.z)))
        top_z = layer.z
    return t.layers[-1].bottom


def ground_color(t, x, y, z, edge):
    c = _GROUND[t.look](t, x, y, z, edge)
    wall = smoothstep(-z, 0.22, 0.4)
    if wall > 0.0:
        c = mix(c, strata_color(t, z), wall)
    # The cup hollow shows through the cut-out around the cup: keep it dark like the inside of a hole.
    hole = 1.0 - smoothstep(math.hypot(x - t.cup[0], y - t.cup[1]), HOLLOW_OUTER + 0.15, HOLLOW_OUTER + 0.35)
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


def grid_for(outline, max_quads=MAX_TOP_QUADS):
    """Top grid ``(nx, ny)`` of the island: cells of ``CELL`` m, coarser when the island is so large that the top
    would exceed ``max_quads`` quads (``detail.island_quads``, default ``MAX_TOP_QUADS``)."""
    hx, hy = outline.half
    cell = max(CELL, math.sqrt(4.0 * hx * hy / max_quads))
    return math.ceil(2.0 * hx / cell), math.ceil(2.0 * hy / cell)


def build(terrain, number, max_quads=MAX_TOP_QUADS):
    """Build the island and return ``(island_obj, waterline)`` (waterline empty on the moon)."""
    look = terrain.look
    terrain.layers = layers_for(look, terrain.pal)
    nx, ny = grid_for(terrain.outline, max_quads)

    def height(x, y, edge):
        return terrain.top_height(x, y)

    obj, waterline = build_island(terrain.outline, nx, ny, height, lambda x, y, z, edge: ground_color(terrain, x, y, z, edge),
                                  terrain.layers, WATER_Z if look != "moon" else -100.0, seed=number, tip=TIP[look],
                                  clamp_down=True, warp=terrain.warp)
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
