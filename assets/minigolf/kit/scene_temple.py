"""Jungle temple probe scene (MIT License): golden evening, stone head gate."""

import random

from mathutils import Vector, noise

from . import lane as lane_mod, materials, palette, props_temple, stage
from .compose import Composer, cup_dip, fbm, lane_bed
from .island import Layer, Outline, build_island, build_water, smoothstep
from .palette import mix

WORLD = "temple"
WATER_Z = -0.9
CAMERA = dict(target=(0.2, 1.0, 1.3), distance=16.0, azimuth=-22.0, elevation=36.0, shift=(0.0, 0.0))
BAKE = dict(samples_bake=160, distance=1.5, tint=(0.16, 0.08, 0.10), gamma=1.0)
HEAD_Y = 3.55
HEAD_SCALE = 1.1


def build(sc, quick=False):
    comp = Composer(WORLD, seed=17)
    pal = palette.get(WORLD)
    rng = random.Random(9)
    off = Vector((rng.uniform(-40, 40), rng.uniform(-40, 40), 0.0))
    lane = lane_mod.Lane(length=8.0, amp=1.1, origin=(0.0, 0.7), shape="jog", tail=0.5, wave=0.25, cup_t=0.55,
                         ball_t=0.09, sign=1.0)
    comp.lane = lane
    outline = Outline(center=(0.2, 1.1), half=(6.3, 5.5), power=3.0, wobble=0.04, seed=5)
    comp.outline = outline

    def surface(x, y, edge):
        z = 0.05 * fbm(x, y, 0.4, off) + 0.03 * fbm(x, y, 1.2, off * 1.3)
        z *= smoothstep(edge, 0.1, 1.1)
        z -= 0.06 * (1.0 - smoothstep(edge, 0.0, 0.32))
        z = lane_bed(lane, x, y, z)
        # keep the ground under the head and tunnel low and flat
        if abs(x) < 2.9 and y > HEAD_Y - 2.1:
            z = min(z, -0.06)
        return cup_dip(lane, x, y, z)

    comp.ground = lambda x, y: surface(x, y, -outline.contains_signed(x, y))
    moss_d, moss, moss_l = pal.c("moss_dark"), pal.c("moss"), pal.c("moss_light")
    dirt_d, dirt, dirt_l = pal.c("dirt_dark"), pal.c("dirt"), pal.c("dirt_light")
    jungle, yg = pal.c("jungle"), pal.c("yellow_green")

    def ground_color(x, y, z, edge):
        n = fbm(x, y, 0.8, off * 0.7)
        c = mix(moss_d, moss, min(1.0, max(0.0, 0.5 + 0.8 * n)))
        c = mix(c, jungle, 0.45 * smoothstep(fbm(x, y, 0.35, off * 2.0), 0.0, 0.5))
        c = mix(c, yg, 0.35 * smoothstep(fbm(x, y, 0.6, off * 3.1), 0.25, 0.6))
        d, _ = lane.nearest(x, y)
        path = 1.0 - smoothstep(d, 1.0, 2.1)
        c = mix(c, mix(dirt, dirt_l, 0.5 + 0.5 * n), path * 0.85)
        wet = 1.0 - smoothstep(edge, 0.05, 0.8)
        c = mix(c, dirt_d, wet * 0.5)
        return c

    seam = pal.c("stone_deep")
    layers = [
        Layer(-0.18, mix(moss, moss_l, 0.4), moss_d, off=0.07, bulge=0.05, noise_amp=0.02, steps=3),
        Layer(-0.44, dirt_l, dirt, off=0.03, bulge=0.025, noise_amp=0.02, steps=3, ledge=0.02, speck=(pal.c("stone"), 0.4)),
        Layer(-0.70, pal.c("stone_light"), pal.c("stone"), off=0.1, bulge=0.02, noise_amp=0.01, steps=3, ledge=0.07,
              blocks=(0.95, seam, 0.0)),
        Layer(WATER_Z, pal.c("stone"), pal.c("stone_dark"), off=0.12, bulge=0.02, noise_amp=0.01, steps=3, ledge=0.0,
              blocks=(0.8, seam, 0.5)),
        Layer(-1.6, pal.c("stone_deep"), pal.c("stone_deep"), off=0.12, bulge=0.0, noise_amp=0.02, steps=2),
    ]
    nx, ny = (54, 52) if quick else (84, 82)
    island, waterline = build_island(outline, nx, ny, surface, ground_color, layers, WATER_Z, seed=4)
    island.data.materials.append(materials.get("matte"))
    comp.add(island)

    def water_color(x, y, offset):
        foam = pal.c("foam")
        light, mid, deep = pal.c("water_light"), pal.c("water"), pal.c("water_deep")
        if offset < 1.0:
            c = mix(light, mid, smoothstep(offset, 0.1, 1.0))
        else:
            c = mix(mid, deep, smoothstep(offset, 1.0, 12.0))
        n = noise.noise(Vector((x * 0.7, y * 0.7, 3.0)) + off)
        c = (c[0] * (1 + 0.08 * n), c[1] * (1 + 0.08 * n), c[2] * (1 + 0.06 * n), 1.0)
        f = (1.0 - smoothstep(offset, 0.03, 0.22)) * (0.65 + 0.35 * noise.noise(Vector((x * 3.0, y * 3.0, 1.0)) + off))
        return mix(c, foam, min(1.0, f))

    offsets = [0.0, 0.07, 0.16, 0.3, 0.5, 0.78, 1.15, 1.6, 2.2, 3.0, 4.0, 5.2, 6.8, 8.8, 11.5, 15, 20, 28, 40, 60]
    water = build_water(waterline, outline.center, WATER_Z, water_color, offsets)
    water.data.materials.append(materials.get("water", bump=0.05))
    comp.add(water, "pass")

    lane_obj = lane_mod.build_lane(WORLD, "stone", lane, seed=7, flag=(("totem_red", "fire_orange")),
                                   collar=("gold_dark", "gold_light"), tee=("felt", "felt_light"))
    comp.add(lane_obj)
    comp.add(lane_mod.build_ball(WORLD, lane, stripe="totem_red"))

    place_props(comp, pal, lane)
    lighting(sc, pal)
    c = CAMERA
    stage.add_camera(sc, c["target"], c["distance"], c["azimuth"], c["elevation"], shift=c["shift"])
    return comp


def place_props(comp, pal, lane):
    rng = random.Random(33)
    P = props_temple
    taken = [(lane.pos(t).x, lane.pos(t).y, 1.3) for t in [i / 16 for i in range(17)]]

    def free(x, y, r):
        return all((x - tx) ** 2 + (y - ty) ** 2 >= (r + tr) ** 2 for tx, ty, tr in taken)

    def put(key, obj, x, y, r=0.5, **kw):
        comp.place(obj, x, y, **kw)
        comp.record(key, obj)
        taken.append((x, y, r))
        return obj

    put("head", P.build_head(), 0.0, HEAD_Y, 0.0, sink=0.1, scale=HEAD_SCALE)
    put("waterfall", P.build_waterfall(), -4.7, 3.7, 2.0, yaw=14.0, sink=0.1, scale=0.72)
    for sx, seed in ((-1, 2), (1, 3)):
        put("totem", P.build_totem(seed=seed), sx * 3.7, 0.3, 0.9, yaw=-sx * 16.0, sink=0.1)
    # torches along the first third of the lane
    for t, side in ((0.2, 1), (0.2, -1), (0.4, -1)):
        pos = lane.pos(t) + lane.left(t) * (side * 1.55)
        put("torch", P.build_torch(seed=int(t * 10) + side), pos.x, pos.y, 0.5, yaw=rng.uniform(0, 90), sink=0.03)
    # ruins
    put("pillar", P.build_pillar(seed=2), -4.0, -0.6, 0.9, yaw=20.0, sink=0.05)
    put("pillar", P.build_pillar(seed=3, height=1.1), 3.9, -1.9, 0.9, yaw=-30.0, sink=0.05)
    # stone plaza tiles near the tee and in front of the mouth
    for i in range(30):
        for _ in range(40):
            t = rng.uniform(0.02, 0.58)
            side = rng.choice((-1, 1))
            lat = rng.uniform(1.25, 2.9)
            q = lane.pos(t) + lane.left(t) * (side * lat)
            if free(q.x, q.y, 0.45) and comp.outline.contains_signed(q.x, q.y) < -0.8:
                put("tile", P.build_tile(seed=100 + i), q.x, q.y, 0.45, yaw=lane.yaw(t) + rng.uniform(-6, 6), sink=0.015)
                break
    # mossy rock islets and lily pads in the lagoon
    for sd, (x, y, size) in enumerate(((-8.3, 5.5, 1.5), (9.0, 0.6, 1.3), (-6.8, -5.4, 1.0), (8.4, -5.0, 1.4), (-2.5, 10.5, 1.3))):
        put("islet", P.build_islet(seed=sd + 1, size=size), x, y, 0.0, z=WATER_Z - 0.35, sink=0)
    for i, (cx, cy) in enumerate(((-6.8, -1.0), (-3.5, -6.0), (4.5, -6.6), (7.6, 0.2), (-7.3, 2.4), (6.0, 7.0))):
        for k in range(3):
            q = Vector((cx, cy)) + Vector((rng.uniform(-0.9, 0.9), rng.uniform(-0.7, 0.7)))
            put("lilypad", P.build_lilypad(seed=200 + i * 3 + k, flower=(k == 0)), q.x, q.y, 0.0, z=WATER_Z + 0.02, sink=0,
                yaw=rng.uniform(0, 360))
    # lianas hanging from the head (attach points are given in head space)
    for k, (hx, hy, top, ln) in enumerate(((-2.2, -1.5, 4.2, 2.4), (2.25, -1.6, 4.2, 2.2), (-1.3, -1.95, 4.1, 1.4),
                                           (1.45, -2.0, 4.1, 1.6), (-2.5, 0.2, 3.9, 1.8))):
        x, y, zt, length = hx * HEAD_SCALE, HEAD_Y + hy * HEAD_SCALE, top * HEAD_SCALE, ln * 1.0
        comp.place(P.build_liana(seed=20 + k, length=length), x, y, z=zt - length - 0.1, sink=0)
        comp.record("liana", comp.bake_objs[-1])
    # greenery
    spots = []
    for i in range(18):
        for _ in range(40):
            x, y = rng.uniform(-5.2, 6.0), rng.uniform(-4.4, 6.0)
            if comp.outline.contains_signed(x, y) < -0.7 and free(x, y, 0.55):
                spots.append((x, y))
                put("fern", P.build_fern(seed=40 + i, size=rng.uniform(0.8, 1.15)), x, y, 0.55, yaw=rng.uniform(0, 360), sink=0.02)
                break
    for i in range(9):
        for _ in range(40):
            x, y = rng.uniform(-5.0, 5.8), rng.uniform(-4.4, 5.8)
            if comp.outline.contains_signed(x, y) < -0.9 and free(x, y, 0.7):
                put("bush", P.build_bush(seed=60 + i, size=rng.uniform(0.9, 1.3)), x, y, 0.7, yaw=rng.uniform(0, 360), sink=0.04)
                break
    for i, (x, y, s) in enumerate(((-4.2, 2.4, 0.8), (-3.2, 4.4, 0.6), (5.0, 1.2, 0.5), (-0.6, -4.3, 0.45))):
        put("rock", P.build_rock(seed=70 + i, size=s), x, y, s, yaw=rng.uniform(0, 360), sink=0.1)
    for i in range(10):
        for _ in range(30):
            x, y = rng.uniform(-5.0, 6.0), rng.uniform(-4.4, 5.8)
            if comp.outline.contains_signed(x, y) < -0.6 and free(x, y, 0.3):
                put("pebble", P.build_rock(seed=90 + i, size=rng.uniform(0.08, 0.18), cuts=2, subdiv=2), x, y, 0.2,
                    yaw=rng.uniform(0, 360), sink=0.02)
                break


def lighting(sc, pal):
    stage.add_sun(sc, "kit-sun", -38.0, 18.0, 6.5, (1.0, 0.66, 0.34), angle_deg=3.0)
    stage.set_sky(sc, [(-0.25, (1.0, 0.62, 0.42, 1)), (0.1, (1.0, 0.74, 0.55, 1)), (0.55, (0.45, 0.62, 0.62, 1)),
                       (1.0, (0.10, 0.42, 0.52, 1))], 0.8)
