"""Moon station probe scene (MIT License): floating regolith chunk in space."""

import math
import random

import bmesh
from mathutils import Vector, noise

from . import colors, lane as lane_mod, materials, palette, props_moon, shapes, stage
from .compose import Composer, cup_dip, fbm, lane_bed
from .island import Layer, Outline, build_island, smoothstep
from .palette import mix

WORLD = "moon"
CAMERA = dict(target=(0.9, 0.5, 0.2), distance=16.2, azimuth=-24.0, elevation=37.0, shift=(0.0, 0.0))
BAKE = dict(samples_bake=160, distance=1.5, tint=(0.14, 0.12, 0.30), gamma=1.0)
KEY_SUN = (-62.0, 24.0)
CRATERS = [(-3.6, -1.6, 1.5, 0.22, 0.10), (-1.9, 3.3, 0.95, 0.15, 0.07), (-4.7, 1.5, 0.7, 0.11, 0.05),
           (4.6, -3.2, 0.8, 0.12, 0.06), (2.1, 4.7, 0.55, 0.09, 0.05)]


def crater_shape(x, y):
    """Height offset and a (rim, floor) weight pair from the crater list."""
    dz, rim_w, floor_w = 0.0, 0.0, 0.0
    for cx, cy, r, depth, rim in CRATERS:
        d = math.hypot(x - cx, y - cy)
        if d < r * 1.5:
            bowl = (1.0 - (d / r) ** 2) if d < r else 0.0
            dz -= depth * max(0.0, bowl)
            k = math.exp(-(((d - r) / (0.2 * r + 0.08)) ** 2))
            dz += rim * k
            rim_w = max(rim_w, k)
            floor_w = max(floor_w, smoothstep(r - d, 0.0, r * 0.5) if d < r else 0.0)
    return dz, rim_w, floor_w


TRACK = [(3.1, -2.7), (2.4, -3.2), (1.6, -3.45), (0.9, -3.4)]


def track_weight(x, y):
    """Soft darkening along two rover tire tracks (distance to a polyline)."""
    best = 9.0
    for (ax, ay), (bx, by) in zip(TRACK, TRACK[1:]):
        dx, dy = bx - ax, by - ay
        ln = dx * dx + dy * dy
        t = max(0.0, min(1.0, ((x - ax) * dx + (y - ay) * dy) / ln))
        px, py = ax + dx * t, ay + dy * t
        d = math.hypot(x - px, y - py)
        best = min(best, abs(d - 0.34))
    return 1.0 - smoothstep(best, 0.04, 0.11)


def build(sc, quick=False):
    comp = Composer(WORLD, seed=23)
    pal = palette.get(WORLD)
    rng = random.Random(14)
    off = Vector((rng.uniform(-40, 40), rng.uniform(-40, 40), 0.0))
    lane = lane_mod.Lane(length=7.0, amp=0.62, origin=(0.0, 0.0), cup_t=0.86, ball_t=0.10, sign=-1.0)
    comp.lane = lane
    outline = Outline(center=(0.9, 0.9), half=(6.0, 4.9), power=2.6, wobble=0.07, seed=8)
    comp.outline = outline

    def surface(x, y, edge):
        z = 0.04 * fbm(x, y, 0.45, off) + 0.02 * fbm(x, y, 1.3, off * 1.3)
        dz, _, _ = crater_shape(x, y)
        z += dz
        z *= smoothstep(edge, 0.1, 1.1) if edge < 1.1 else 1.0
        z -= 0.06 * (1.0 - smoothstep(edge, 0.0, 0.32))
        z = lane_bed(lane, x, y, z)
        return cup_dip(lane, x, y, z)

    comp.ground = lambda x, y: surface(x, y, -outline.contains_signed(x, y))
    reg_d, reg, reg_l, dust = pal.c("regolith_dark"), pal.c("regolith"), pal.c("regolith_light"), pal.c("dust")
    deep = pal.c("regolith_deep")

    def ground_color(x, y, z, edge):
        n = fbm(x, y, 0.9, off * 0.7)
        c = mix(reg, reg_l, min(1.0, max(0.0, 0.5 + 0.9 * n)))
        patches = smoothstep(fbm(x, y, 0.4, off * 2.0), 0.1, 0.5)
        c = mix(c, reg_d, 0.45 * patches)
        _, rim_w, floor_w = crater_shape(x, y)
        c = mix(c, dust, 0.7 * rim_w)
        c = mix(c, deep, 0.6 * floor_w)
        tk = track_weight(x, y)
        c = mix(c, reg_d, 0.55 * tk)
        spk = noise.noise(Vector((x * 6.0, y * 6.0, 2.0)) + off)
        if spk > 0.55:
            c = mix(c, dust, 0.5)
        return c

    layers = [
        Layer(-0.22, mix(reg_l, dust, 0.3), reg, off=0.06, bulge=0.05, noise_amp=0.02, steps=3),
        Layer(-0.55, reg, reg_d, off=0.03, bulge=0.03, noise_amp=0.03, steps=3, ledge=0.03, speck=(dust, 0.35)),
        Layer(-0.95, pal.c("regolith_dark"), deep, off=0.07, bulge=0.03, noise_amp=0.04, steps=3, ledge=0.06,
              speck=(reg_l, 0.4)),
        Layer(-1.5, deep, mix(deep, pal.c("black"), 0.4), off=0.0, bulge=0.04, noise_amp=0.06, steps=3, ledge=0.05, scale=0.97),
        Layer(-2.3, mix(deep, pal.c("black"), 0.4), mix(deep, pal.c("black"), 0.6), off=0.0, bulge=0.05, noise_amp=0.08, steps=3,
              scale=0.8),
        Layer(-3.2, mix(deep, pal.c("black"), 0.6), mix(deep, pal.c("black"), 0.75), off=0.0, bulge=0.05, noise_amp=0.1, steps=3,
              scale=0.55),
        Layer(-3.9, mix(deep, pal.c("black"), 0.75), pal.c("black"), off=0.0, bulge=0.04, noise_amp=0.1, steps=3, scale=0.3),
    ]
    nx, ny = (52, 44) if quick else (84, 70)
    island, _ = build_island(outline, nx, ny, surface, ground_color, layers, -100.0, seed=6, tip=-4.5)
    island.data.materials.append(materials.get("matte"))
    comp.add(island)

    lane_obj = lane_mod.build_lane(WORLD, "metal", lane, seed=7, flag=("magenta", "magenta"), collar=("cyan", "cyan"),
                                   tee=("felt", "felt_light"), collar_mat="emissive", flag_mat="emissive", glow=2.0)
    comp.add(lane_obj)
    comp.add(lane_mod.build_ball(WORLD, lane, stripe="cyan"))

    place_props(comp, pal, lane)
    space(sc, comp, pal)
    lighting(sc, pal)
    c = CAMERA
    cam = stage.add_camera(sc, c["target"], c["distance"], c["azimuth"], c["elevation"], shift=c["shift"])
    place_sky(comp, cam)
    return comp


def place_props(comp, pal, lane):
    rng = random.Random(41)
    P = props_moon
    taken = [(lane.pos(t).x, lane.pos(t).y, 1.3) for t in [i / 16 for i in range(17)]]

    def free(x, y, r):
        return all((x - tx) ** 2 + (y - ty) ** 2 >= (r + tr) ** 2 for tx, ty, tr in taken)

    def put(key, obj, x, y, r=0.5, **kw):
        comp.place(obj, x, y, **kw)
        comp.record(key, obj)
        taken.append((x, y, r))
        return obj

    put("habitat", P.build_habitat(), 3.9, 2.1, 3.0, yaw=-8.0, sink=0.12)
    put("pad", P.build_pad(), 3.7, -2.2, 1.7, yaw=-8.0, sink=0.03)
    put("rover", P.build_rover(), 3.6, -2.2, 1.0, yaw=148.0, z=comp.height(3.7, -2.2) + 0.065, sink=0)
    put("dish", P.build_dish(), -3.1, 2.7, 1.6, yaw=24.0, sink=0.05)
    put("solar", P.build_solar(panels=3), -0.4, 4.3, 2.6, yaw=-4.0, sink=0.03)
    put("crate", P.build_crate(), 5.6, -0.4, 0.8, yaw=30.0, sink=0.02)
    # beacons along the tee
    for t, side in ((0.04, 1), (0.04, -1), (0.3, -1), (0.62, 1)):
        q = lane.pos(t) + lane.left(t) * (side * 1.42)
        put("beacon", P.build_beacon(tone="cyan" if side < 0 else "magenta"), q.x, q.y, 0.3, sink=0.02)
    for i, (x, y, tone) in enumerate(((-2.9, -3.4, "cyan"), (5.4, -3.6, "magenta"), (-5.0, 0.9, "magenta"), (5.9, 3.7, "cyan"))):
        put("crystal", P.build_crystal(seed=50 + i, tone=tone), x, y, 0.5, yaw=rng.uniform(0, 360), sink=0.03)
    for i, (x, y, s, sq) in enumerate(((-4.3, -0.2, 0.85, (1.1, 0.9, 0.75)), (-2.0, -3.6, 0.55, (1.0, 0.8, 0.7)),
                                       (-2.5, 1.7, 0.42, (1.0, 0.9, 0.8)), (2.2, -0.9, 0.38, (1.0, 0.8, 0.7)),
                                       (5.7, 3.1, 0.7, (1.0, 0.9, 0.75)), (-5.0, 3.2, 0.6, (1.0, 0.85, 0.8)),
                                       (1.5, 4.7, 0.5, (1.0, 0.9, 0.8)))):
        put("boulder", P.build_boulder(seed=i + 3, size=s, squash=sq), x, y, s, yaw=rng.uniform(0, 360), sink=0.1)
    for i in range(24):
        for _ in range(30):
            x, y = rng.uniform(-5.2, 6.6), rng.uniform(-4.0, 5.4)
            if comp.outline.contains_signed(x, y) < -0.6 and free(x, y, 0.3):
                put("pebble", P.build_boulder(seed=30 + i, size=rng.uniform(0.07, 0.2), cuts=2, subdiv=2), x, y, 0.2,
                    yaw=rng.uniform(0, 360), sink=0.03)
                break


def space(sc, comp, pal):
    """Stars (one emissive mesh) and the Earth (unit sphere with a procedural emissive shader)."""
    rng = random.Random(77)
    bm = bmesh.new()
    cols = []
    tints = [(1.0, 1.0, 1.0), (0.8, 0.9, 1.0), (1.0, 0.95, 0.8), (0.9, 0.8, 1.0)]
    for i in range(1300):
        d = Vector((rng.gauss(0, 1), rng.gauss(0, 1), rng.gauss(0, 1))).normalized()
        pos = d * rng.uniform(190, 230)
        size = rng.choice((0.12, 0.12, 0.16, 0.2, 0.3, 0.45))
        verts = [bm.verts.new(pos + v * size) for v in (Vector((1, 0, 0)), Vector((-1, 0, 0)), Vector((0, 1, 0)),
                                                         Vector((0, -1, 0)), Vector((0, 0, 1)), Vector((0, 0, -1)))]
        for tri in ((0, 2, 4), (2, 1, 4), (1, 3, 4), (3, 0, 4), (2, 0, 5), (1, 2, 5), (3, 1, 5), (0, 3, 5)):
            bm.faces.new([verts[t] for t in tri])
        tint = rng.choice(tints)
        br = rng.uniform(0.55, 1.0)
        cols.extend([(tint[0] * br, tint[1] * br, tint[2] * br, 1.0)] * 6)
    stars = shapes.from_bmesh("stars", bm)
    colors.paint_values(stars, cols, ao=0.0)
    stars.data.materials.append(materials.get("emissive", strength=1.0))
    comp.add(stars, "pass", backdrop=True)
    earth = shapes.ico_sphere("earth", 1.0, 5)
    comp.add(earth, "hidden", backdrop=True)
    comp.earth = earth


def place_sky(comp, cam):
    """Place the Earth in camera space; its terminator follows a sun lighting the left of the frame."""
    quat = cam.rotation_euler.to_quaternion()
    fwd, up, right = quat @ Vector((0, 0, -1)), quat @ Vector((0, 1, 0)), quat @ Vector((1, 0, 0))
    earth = comp.earth
    earth.location = cam.location + fwd * 150.0 + up * 29.0 + right * -45.0
    earth.scale = (25.0, 25.0, 25.0)
    lit_dir = (quat @ Vector((-0.78, 0.38, 0.5))).normalized()
    earth.data.materials.append(materials.earth(lit_dir))


def lighting(sc, pal):
    stage.add_sun(sc, "kit-key", KEY_SUN[0], KEY_SUN[1], 4.6, (0.88, 0.94, 1.0), angle_deg=2.0)
    stage.add_sun(sc, "kit-rim", 158.0, 24.0, 5.5, (0.5, 0.8, 1.0), angle_deg=3.0)
    stage.add_sun(sc, "kit-fill", -25.0, 52.0, 1.1, (0.55, 0.66, 1.0), angle_deg=45.0)
    stage.set_sky(sc, [(-1.0, (0.012, 0.016, 0.05, 1)), (0.0, (0.01, 0.014, 0.045, 1)), (1.0, (0.008, 0.01, 0.03, 1))], 1.6,
                  camera_strength=1.0)
