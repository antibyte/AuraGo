"""Pirate bay probe scene (MIT License): sunny midday lagoon island."""

import math
import random

from mathutils import Vector, noise

from . import lane as lane_mod, materials, palette, props_pirate, stage
from .compose import Composer, cup_dip, fbm, lane_bed
from .island import Layer, Outline, build_island, build_water, smoothstep
from .palette import mix

WORLD = "pirate"
WATER_Z = -1.3
GRASS = [(-3.0, 1.9, 1.5), (2.2, 4.0, 1.3), (0.6, -3.8, 1.1), (-3.5, -2.9, 1.0)]
CAMERA = dict(target=(0.2, 0.7, 0.5), distance=14.4, azimuth=-24.0, elevation=37.0, shift=(0.0, 0.0))
BAKE = dict(samples_bake=160, distance=1.5, tint=(0.10, 0.10, 0.22), gamma=1.0)


def build(sc, quick=False):
    comp = Composer(WORLD, seed=11)
    pal = palette.get(WORLD)
    rng = random.Random(5)
    off = Vector((rng.uniform(-40, 40), rng.uniform(-40, 40), 0.0))
    lane = lane_mod.Lane(length=7.0, amp=0.62, origin=(0.0, 0.0))
    comp.lane = lane
    outline = Outline(center=(0.8, 0.4), half=(5.8, 4.5), power=3.0, wobble=0.045, seed=3)
    comp.outline = outline

    def surface(x, y, edge):
        z = 0.06 * fbm(x, y, 0.33, off) + 0.03 * fbm(x, y, 1.0, off * 1.3)
        z *= smoothstep(edge, 0.1, 1.1)
        z -= 0.06 * (1.0 - smoothstep(edge, 0.0, 0.32))
        z = lane_bed(lane, x, y, z)
        return cup_dip(lane, x, y, z)

    comp.ground = lambda x, y: surface(x, y, -outline.contains_signed(x, y))

    sand_wet, sand, sand_light = pal.c("sand_wet"), pal.c("sand"), pal.c("sand_light")
    grass_lo, grass_hi = pal.grad("grass")

    def ground_color(x, y, z, edge):
        n = fbm(x, y, 0.9, off * 0.7)
        c = mix(sand, sand_light, min(1.0, max(0.0, 0.5 + 1.6 * z + 0.25 * n)))
        wet = 1.0 - smoothstep(edge, 0.05, 0.9)
        c = mix(c, sand_wet, wet * 0.8)
        d, _ = lane.nearest(x, y)
        c = mix(c, sand_wet, 0.30 * (1.0 - smoothstep(d, 1.0, 2.2)))
        gm = 0.0
        for gx, gy, gr in GRASS:
            d2 = math.hypot(x - gx, y - gy)
            ragged = gr * (0.8 + 0.35 * fbm(x, y, 1.1, off * 2.7))
            gm = max(gm, smoothstep(ragged - d2, 0.0, 0.3 * gr))
        gm *= smoothstep(edge, 0.5, 1.2) * smoothstep(d, 1.1, 1.5)
        if gm > 0:
            gc = mix(grass_lo, grass_hi, min(1.0, max(0.0, 0.5 + 0.55 * n)))
            c = mix(c, gc, gm)
        return c

    layers = [
        Layer(-0.22, mix(sand, sand_wet, 0.2), sand_wet, off=0.07, bulge=0.04, noise_amp=0.012, steps=3),
        Layer(-0.64, pal.c("soil_light"), pal.c("soil"), off=0.035, bulge=0.03, noise_amp=0.02, steps=4,
              ledge=0.035, speck=(pal.c("rock_light"), 0.35)),
        Layer(-1.0, pal.c("soil"), pal.c("soil_dark"), off=0.08, bulge=0.035, noise_amp=0.03, steps=4,
              ledge=0.04, speck=(pal.c("rock"), 0.3)),
        Layer(WATER_Z, pal.c("rock_light"), pal.c("rock"), off=0.24, bulge=0.05, noise_amp=0.05, steps=3,
              ledge=0.07, speck=(pal.c("rock_dark"), 0.3)),
        Layer(-2.0, pal.c("rock_dark"), pal.c("rock_dark"), off=0.15, bulge=0.0, noise_amp=0.03, steps=2),
    ]
    nx, ny = (56, 48) if quick else (86, 74)
    island, waterline = build_island(outline, nx, ny, surface, ground_color, layers, WATER_Z, seed=2)
    island.data.materials.append(materials.get("matte"))
    comp.add(island)
    comp.island_obj = island

    def water_color(x, y, offset):
        foam = pal.c("foam")
        shore, light, mid, deep = pal.c("lagoon_shore"), pal.c("lagoon_light"), pal.c("lagoon_mid"), pal.c("lagoon_deep")
        if offset < 0.9:
            c = mix(shore, light, smoothstep(offset, 0.1, 0.9))
        elif offset < 4.0:
            c = mix(light, mid, smoothstep(offset, 0.9, 4.0))
        else:
            c = mix(mid, deep, smoothstep(offset, 4.0, 14.0))
        n = noise.noise(Vector((x * 0.7, y * 0.7, 3.0)) + off)
        c = (c[0] * (1 + 0.08 * n), c[1] * (1 + 0.08 * n), c[2] * (1 + 0.06 * n), 1.0)
        f = (1.0 - smoothstep(offset, 0.03, 0.24)) * (0.65 + 0.35 * noise.noise(Vector((x * 3.0, y * 3.0, 1.0)) + off))
        return mix(c, foam, min(1.0, f))

    offsets = [0.0, 0.07, 0.16, 0.3, 0.5, 0.78, 1.15, 1.6, 2.2, 3.0, 4.0, 5.2, 6.8, 8.8, 11.5, 15, 20, 28, 40, 60]
    water = build_water(waterline, outline.center, WATER_Z, water_color, offsets)
    water.data.materials.append(materials.get("water", bump=0.05))
    comp.add(water, "pass")

    # lane ------------------------------------------------------------------
    lane_obj = lane_mod.build_lane(WORLD, "wood", lane, seed=7, flag=("red_dark", "red"),
                                   collar=("gold_dark", "gold_light"), tee=("felt", "felt_light"))
    comp.add(lane_obj)
    ball = lane_mod.build_ball(WORLD, lane, stripe="red")
    comp.add(ball)

    place_props(comp, pal, lane)
    lighting(sc, pal)
    c = CAMERA
    stage.add_camera(sc, c["target"], c["distance"], c["azimuth"], c["elevation"], shift=c["shift"])
    return comp


def place_props(comp, pal, lane):
    rng = random.Random(21)
    taken = [(lane.pos(t).x, lane.pos(t).y, 1.25) for t in [i / 14 for i in range(15)]]

    def free(x, y, r):
        return all((x - tx) ** 2 + (y - ty) ** 2 >= (r + tr) ** 2 for tx, ty, tr in taken)

    def put(key, obj, x, y, r=0.5, **kw):
        comp.place(obj, x, y, **kw)
        comp.record(key, obj)
        taken.append((x, y, r))
        return obj

    P = props_pirate
    # hero ----------------------------------------------------------------
    put("ship", P.build_ship(), 3.7, 1.4, 3.0, yaw=-65.0, tilt=(3.0, 8.0), sink=0.36, scale=0.95)
    # palms
    put("palm", P.build_palm(seed=4, heading=200.0), 2.0, 4.0, 0.8, sink=0.05)
    put("palm", P.build_palm(seed=9, heading=250.0, height=2.6, lean=0.7), -3.0, 1.9, 0.8, sink=0.05)
    # barrels by the tee
    for i, (x, y) in enumerate(((-1.95, -2.75), (-2.55, -2.25), (-1.9, -2.05))):
        put("barrel", P.build_barrel(seed=30 + i), x, y, 0.4, yaw=rng.uniform(0, 360), sink=0.03)
    # chest near the cup, opening towards the lane
    put("chest", P.build_chest(), -2.2, 1.1, 0.9, yaw=-90.0, sink=0.01)
    put("xmark", P.build_xmark(), -3.3, -0.4, 0.4, yaw=15.0, sink=0.0)
    put("cannon", P.build_cannon(), 1.95, -2.6, 0.9, yaw=-100.0, sink=0.02)
    put("dock", P.build_dock(length=2.9), -4.4, -0.9, 1.5, yaw=90.0, z=comp.height(-4.4, -0.9) - 1.6 + 0.06, sink=0)
    # rocks
    put("rock", P.build_rock(seed=2, size=0.85), -3.4, 3.6, 0.9, yaw=20.0, sink=0.1)
    put("rock", P.build_rock(seed=5, size=0.55, squash=(1.0, 0.8, 1.0)), -2.5, 3.9, 0.55, yaw=70.0, sink=0.1)
    put("rock", P.build_rock(seed=7, size=0.4), -3.8, 2.8, 0.4, yaw=140.0, sink=0.06)
    put("rock", P.build_rock(seed=8, size=0.7), 0.0, 4.2, 0.7, yaw=10.0, sink=0.1)
    for sd, size, x, y in ((11, 1.1, -6.6, 2.3), (13, 0.75, 7.1, -2.8), (14, 0.5, -6.9, -3.6)):
        put("rock", P.build_rock(seed=sd, size=size), x, y, 0.0, z=WATER_Z - 0.3, sink=0)
        put("foam", P.build_foam_ring(radius=size * 0.95), x, y, 0.0, z=WATER_Z + 0.015, sink=0)
    # footprints from the dock to the chest
    a, b = Vector((-4.0, -0.8)), Vector((-2.9, 0.95))
    dirv = (b - a).normalized()
    side = Vector((-dirv.y, dirv.x))
    steps = int((b - a).length / 0.42)
    for i in range(steps):
        q = a + dirv * (i * 0.42) + side * (0.11 if i % 2 else -0.11)
        put("footprint", P.build_footprint(seed=i, left=bool(i % 2)), q.x, q.y, 0.0,
            yaw=math.degrees(math.atan2(dirv.y, dirv.x)) - 90.0 + (6 if i % 2 else -6), sink=0.012)
    # crabs and shells
    put("crab", P.build_crab(), -2.9, 0.5, 0.3, yaw=130.0, sink=0.0)
    put("crab", P.build_crab(seed=2), -0.6, -4.2, 0.3, yaw=-30.0, sink=0.0)
    put("starfish", P.build_starfish(), 4.9, -3.5, 0.3, yaw=20.0, sink=0.005)
    put("starfish", P.build_starfish(seed=3), -4.0, 3.0, 0.3, yaw=70.0, sink=0.005)
    for i in range(9):
        for _ in range(30):
            x, y = rng.uniform(-4.5, 6.2), rng.uniform(-4.2, 4.6)
            if comp.outline.contains_signed(x, y) < -0.9 and free(x, y, 0.25):
                put("shell", P.build_shell(seed=40 + i), x, y, 0.2, yaw=rng.uniform(0, 360), tilt=(rng.uniform(-12, 12), rng.uniform(-12, 12)),
                    sink=0.012)
                break
    for i in range(14):
        for _ in range(30):
            x, y = rng.uniform(-4.5, 6.2), rng.uniform(-4.2, 4.6)
            if comp.outline.contains_signed(x, y) < -0.6 and free(x, y, 0.3):
                put("pebble", P.build_rock(seed=60 + i, size=rng.uniform(0.07, 0.16), cuts=2, subdiv=2), x, y, 0.15,
                    yaw=rng.uniform(0, 360), sink=0.02)
                break
    for i in range(46):
        for _ in range(40):
            x, y = rng.uniform(-4.6, 6.3), rng.uniform(-4.4, 4.8)
            if comp.outline.contains_signed(x, y) < -0.5 and free(x, y, 0.3):
                put("tuft", P.build_tuft(seed=80 + i, height=rng.uniform(0.34, 0.58)), x, y, 0.25,
                    yaw=rng.uniform(0, 360), sink=0.01)
                break


def lighting(sc, pal):
    stage.add_sun(sc, "kit-sun", -42.0, 54.0, 2.7, (1.0, 0.95, 0.84), angle_deg=3.5)
    stage.set_sky(sc, [(-0.3, (0.55, 0.78, 0.86, 1)), (0.1, (0.42, 0.72, 0.92, 1)), (1.0, (0.22, 0.52, 0.88, 1))], 0.9)
