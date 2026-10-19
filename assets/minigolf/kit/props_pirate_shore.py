"""Shore props of the pirate bay: skull rock, seaweed, pebbles, shell group (MIT License).

Same prop contract as ``props_pirate``: one joined mesh ``prop-pirate-<name>``,
Z-up, base at z=0, footprint centred, front towards +Y, built from ``seed``.
"""

import math

import bmesh
from mathutils import Vector, noise

from . import shapes
from .prop import Prop
from .palette import mix
from .props_pirate_util import scaled, smoothstep, tone

WORLD = "pirate"
FOLIAGE_SWAY = 0.06  # the kit's shared foliage material (palms, tuft)


def _grass(p, x, y, z, blades, height, rng, spread=0.12):
    """A few grass blades growing from (x, y, z); the same look as the ``tuft`` prop."""
    for _ in range(blades):
        a = rng.uniform(0, math.tau)
        rho = rng.uniform(0.0, 0.03)
        h = height * rng.uniform(0.55, 1.0)
        out = rng.uniform(0.03, spread)
        spine = []
        for k in range(5):
            t = k / 4
            r = rho + out * t * t * 1.4
            spine.append((x + math.cos(a) * r, y + math.sin(a) * r, z + h * t * (1.0 - 0.18 * t)))
        widths = [0.042 * (1 - 0.9 * (k / 4) ** 1.2) for k in range(5)]
        blade = shapes.ribbon("blade", spine, widths, normal_hint=(math.cos(a), math.sin(a), 0.0), fold=0.35, across=2)
        p.add(blade, "foliage", "grass", sway=FOLIAGE_SWAY, finish=False, ao=0.6, jitter=0.05, mode="z")


# --------------------------------------------------------------------------
# Skull rock
# --------------------------------------------------------------------------

_CENTRE_Z = 0.66
_EYES = ((-0.42, 0.82, 0.2), (0.42, 0.82, 0.2))
_EYE_R, _EYE_DEPTH = 0.31, 0.2
_NOSE = (0.0, 0.97, -0.1)
_NOSE_R, _NOSE_DEPTH = 0.15, 0.14


def _dir(v):
    return Vector(v).normalized()


def _skull_radius(d, rng_off):
    """Distance from the skull centre along unit direction ``d`` (front is +Y)."""
    a, b, c = 0.68, 0.62, 0.62
    r = 1.0 / math.sqrt((d.x / a) ** 2 + (d.y / b) ** 2 + (d.z / c) ** 2)
    low = max(0.0, -d.z)
    r *= 1.0 - 0.30 * low ** 1.3 * (1.0 - 0.55 * max(0.0, d.y))  # jaw narrows towards the chin
    brow = math.exp(-((d.z - 0.38) / 0.16) ** 2) * max(0.0, d.y)
    r *= 1.0 + 0.07 * brow  # brow ridge above the sockets
    cheek = math.exp(-((d.z + 0.12) / 0.22) ** 2) * math.exp(-((abs(d.x) - 0.62) / 0.28) ** 2) * max(0.0, d.y)
    r *= 1.0 + 0.05 * cheek
    r *= 1.0 + 0.085 * noise.noise(d * 1.7 + rng_off) + 0.04 * noise.noise(d * 4.3 + rng_off * 1.7)
    return r


def _carve(d):
    """Depth of the sockets, nose and mouth along direction ``d`` (0 = untouched)."""
    depth = 0.0
    for e in _EYES:
        ang = math.acos(max(-1.0, min(1.0, d.dot(_dir(e)))))
        depth = max(depth, _EYE_DEPTH * (1.0 - smoothstep(0.55 * _EYE_R, _EYE_R, ang)))
    ang = math.acos(max(-1.0, min(1.0, d.dot(_dir(_NOSE)))))
    depth = max(depth, _NOSE_DEPTH * (1.0 - smoothstep(0.45 * _NOSE_R, _NOSE_R, ang)))
    if d.y > 0.45:
        m = ((d.x / 0.5) ** 2 + ((d.z + 0.4) / 0.14) ** 2)
        depth = max(depth, 0.085 * (1.0 - smoothstep(0.4, 1.0, m)))
    return depth


def _refine(bm, zones):
    """Split edges near the carved zones; ``zones`` is ``[(unit direction, radius, passes)]``."""
    for level in range(2):
        edges = []
        for e in bm.edges:
            mid = ((e.verts[0].co + e.verts[1].co) * 0.5).normalized()
            for direction, radius, passes in zones:
                if level < passes and math.acos(max(-1.0, min(1.0, mid.dot(direction)))) < radius:
                    edges.append(e)
                    break
        if not edges:
            break
        bmesh.ops.subdivide_edges(bm, edges=edges, cuts=1, use_grid_fill=True)
        bmesh.ops.triangulate(bm, faces=bm.faces[:])
        for v in bm.verts:
            v.co = v.co.normalized()


def build_skullrock(seed=1):
    """Skull-shaped rock with hollow cave eyes, a nose hollow, a toothy mouth and a rocky nest (about 1.4 m)."""
    p = Prop(WORLD, "skullrock", seed)
    rng = p.rng
    off = Vector((rng.uniform(-30, 30), rng.uniform(-30, 30), rng.uniform(-30, 30)))
    bm = bmesh.new()
    bmesh.ops.create_icosphere(bm, subdivisions=3, radius=1.0)
    zones = [(_dir(e), 0.62, 2) for e in _EYES] + [(_dir(_NOSE), 0.34, 1), (_dir((0.0, 0.95, -0.4)), 0.5, 1)]
    zones += [(_dir(e), 0.42, 1) for e in ((-0.42, 0.82, 0.2), (0.42, 0.82, 0.2))]
    _refine(bm, zones)
    centre = Vector((0.0, 0.0, _CENTRE_Z))
    for v in bm.verts:
        d = v.co.normalized()
        v.co = centre + d * (_skull_radius(d, off) - _carve(d))
    skull = shapes.from_bmesh("skull", bm)
    lo, hi = p.pal.grad("rock")
    dark = p.pal.c("coal")
    eye_dirs = [_dir(e) for e in _EYES]
    nose_dir = _dir(_NOSE)

    def paint(x, y, z):
        pt = Vector((x, y, z)) - centre
        d = pt.normalized()
        t = min(1.0, max(0.0, (z - 0.2) / 1.4))
        c = mix(lo, hi, 0.25 + 0.7 * t)
        n = 1.0 + 0.1 * noise.noise(pt * 2.6 + off)
        c = scaled(c, n)
        cave = 0.0
        for e in eye_dirs:
            ang = math.acos(max(-1.0, min(1.0, d.dot(e))))
            cave = max(cave, 1.0 - smoothstep(0.35 * _EYE_R, 0.95 * _EYE_R, ang))
        ang = math.acos(max(-1.0, min(1.0, d.dot(nose_dir))))
        cave = max(cave, 1.0 - smoothstep(0.3 * _NOSE_R, 0.9 * _NOSE_R, ang))
        if d.y > 0.45:
            m = ((d.x / 0.5) ** 2 + ((d.z + 0.4) / 0.14) ** 2)
            cave = max(cave, 1.0 - smoothstep(0.1, 0.85, m))
        return mix(c, dark, 0.92 * cave)

    p.add(skull, "matte", "rock", fn=paint, ao=0.75, bevel=0, dens=0, smooth_angle=80)

    # teeth: an upper row hanging from and a lower row rising out of the mouth groove
    bone_lo, bone_hi = p.pal.c("cream_dark"), p.pal.c("skull")
    for row, (count, dz, height) in enumerate(((7, 0.1, 0.07), (6, -0.1, 0.062))):
        for k in range(count):
            phi = math.radians((k - (count - 1) / 2.0) * 9.0)
            d = Vector((math.sin(phi), math.cos(phi), 0.0)) * math.sqrt(1.0 - (-0.4 + dz) ** 2)
            d.z = -0.4 + dz
            d.normalize()
            rr = _skull_radius(d, off) - 0.02
            pos = centre + d * rr
            tooth = shapes.box("tooth", (0.082, 0.085, height), base=False)
            tz = pos.z + (-0.012 if row == 0 else 0.012)
            p.add(tooth, "matte", (bone_lo, bone_hi), loc=(pos.x, pos.y, tz), rot=(0, 0, math.degrees(math.atan2(-d.x, d.y))),
                  bevel=0.016, segments=1, dens=0, ao=0.6, smooth_angle=60)

    # rocky nest under the jaw
    for a, rd, sz, sq in ((20, 0.5, 0.38, (1.0, 0.9, 0.6)), (95, 0.55, 0.3, (1.1, 0.9, 0.6)), (170, 0.52, 0.4, (1.0, 0.85, 0.6)),
                          (245, 0.5, 0.32, (1.0, 1.0, 0.6)), (315, 0.56, 0.36, (1.1, 0.9, 0.6))):
        rock = shapes.blob("nest", sz, rng, subdiv=2, amp=0.2, freq=1.3, squash=sq, cuts=2, flat_bottom=0.45, relax=2)
        ar = math.radians(a)
        p.add(rock, "matte", "rock", loc=(math.cos(ar) * rd, math.sin(ar) * rd * 0.9, 0.0), rot=(0, 0, a * 2), bevel=0, dens=0, ao=0.7,
              noise_amp=0.08, smooth_angle=80)
    base = shapes.blob("nest-core", 0.46, rng, subdiv=2, amp=0.15, freq=1.1, squash=(1.0, 0.95, 0.6), cuts=1, flat_bottom=0.4, relax=2)
    p.add(base, "matte", "rock", loc=(0, 0.02, 0.1), bevel=0, dens=0, ao=0.7, noise_amp=0.08, smooth_angle=80)
    # a little grass on the crown
    _grass(p, 0.05, -0.1, _CENTRE_Z + 0.5, 7, 0.22, rng, spread=0.1)
    _grass(p, -0.2, -0.02, _CENTRE_Z + 0.44, 5, 0.18, rng, spread=0.1)
    return p.build()


# --------------------------------------------------------------------------
# Seaweed
# --------------------------------------------------------------------------

def build_seaweed(seed=1, blades=11):
    """Wavy kelp tuft with frilled blades and a few golden air bladders (shore scatter)."""
    p = Prop(WORLD, "seaweed", seed)
    rng = p.rng
    lo, hi, tip = tone("#3b4f2a"), tone("#738a34"), tone("#c9bc4e")
    for k in range(blades):
        a = rng.uniform(0, math.tau)
        rho = rng.uniform(0.0, 0.07)
        length = rng.uniform(0.4, 0.85)
        lean = rng.uniform(0.1, 0.4)
        wave, phase = rng.uniform(0.04, 0.08), rng.uniform(0, math.tau)
        steps = 14
        spine, widths = [], []
        ca, sa = math.cos(a), math.sin(a)
        for i in range(steps + 1):
            t = i / steps
            r = rho + lean * t ** 1.4
            side = wave * math.sin(t * 5.0 + phase) * t
            spine.append((ca * r - sa * side, sa * r + ca * side, length * t * (1.0 - 0.14 * t)))
            frill = 1.0 + 0.16 * math.sin(i * 1.9 + k)
            widths.append(0.052 * frill * (1.0 - 0.62 * t ** 1.3) + 0.004)
        blade = shapes.ribbon("kelp", spine, widths, normal_hint=(ca, sa, 0.0), fold=0.25, across=2)
        stops = [(0.0, lo), (0.5, mix(lo, hi, 0.8)), (1.0, mix(hi, tip, 0.65))]
        p.add(blade, "foliage", "grass", sway=FOLIAGE_SWAY, finish=False, ao=0.6, jitter=0.03, mode="z", stops=stops)
        if k % 3 == 0:
            idx = int(steps * 0.62)
            px, py, pz = spine[idx]
            bladder = shapes.sphere("bladder", 0.026, seg=8, rings=5)
            p.add(bladder, "matte", (tone("#b79a33"), tone("#e8cf62")), loc=(px + ca * 0.03, py + sa * 0.03, pz + 0.01), bevel=0, dens=0, ao=0.3)
    return p.build()


# --------------------------------------------------------------------------
# Pebbles
# --------------------------------------------------------------------------

_PEBBLE_TONES = (("rock_dark", "rock_light"), ("sand_dark", "sand_light"), ("rock", "rock_light"), ("wood_dark", "sand"),
                 ("rock_dark", "rock"), ("sand_wet", "sand_light"))


def build_pebble(seed=1):
    """Small group of smooth beach pebbles in grey, sand and brown tones (shore scatter)."""
    p = Prop(WORLD, "pebble", seed)
    rng = p.rng
    off = Vector((rng.uniform(-30, 30), rng.uniform(-30, 30), rng.uniform(-30, 30)))
    placed = []
    spec = ((0.075, -0.07, 0.03), (0.06, 0.08, 0.06), (0.05, 0.02, -0.1), (0.04, 0.14, -0.05), (0.034, -0.15, -0.06), (0.03, -0.02, 0.12))
    for i, (r, x, y) in enumerate(spec):
        pebble = shapes.sphere("pebble", r, seg=12, rings=7)
        o = off + Vector((i * 5.3, 0.0, 0.0))
        for v in pebble.data.vertices:
            d = Vector(v.co).normalized()
            v.co = d * (r * (1.0 + 0.16 * noise.noise(d * 1.4 + o)))
        sx, sy, sz = rng.uniform(1.0, 1.35), rng.uniform(0.85, 1.1), rng.uniform(0.5, 0.7)
        lo_t, hi_t = _PEBBLE_TONES[(i + int(seed)) % len(_PEBBLE_TONES)]
        p.add(pebble, "matte", (lo_t, hi_t), loc=(x, y, r * sz * 0.9), rot=(rng.uniform(-8, 8), rng.uniform(-8, 8), rng.uniform(0, 360)),
              scale=(sx, sy, sz), bevel=0, dens=0, ao=0.7, noise_amp=0.05, smooth_angle=80)
        placed.append((x, y, r))
    return p.build()


# --------------------------------------------------------------------------
# Shell group
# --------------------------------------------------------------------------

def _scallop(p, loc, yaw, k=1.0, tilt=(0.0, 0.0)):
    prof = [(0.0, 0.052), (0.035, 0.048), (0.07, 0.034), (0.1, 0.012), (0.108, 0.0)]
    shell = shapes.lathe("scallop", prof, seg=26, caps=False)
    for v in shell.data.vertices:
        r = math.hypot(v.co.x, v.co.y)
        if r > 1e-5:
            a = math.atan2(v.co.y, v.co.x)
            f = 1.0 + 0.09 * math.cos(a * 9)
            v.co.x *= f
            v.co.y *= f
            v.co.z *= 1.0 + 0.12 * math.cos(a * 9) * (r / 0.1)

    def ridges(x, y, z):
        return 0.9 + 0.1 * math.cos(math.atan2(y, x) * 9)

    p.add(shell, "satin", ("shell_pink", "shell_white"), loc=loc, rot=(tilt[0], tilt[1], yaw), scale=(k, 0.88 * k, k), bevel=0, dens=0,
          pattern=ridges, ao=0.5, smooth_angle=70)


def _whelk(p, loc, yaw, k=1.0):
    """Spiral turret shell lying on its side: bulging whorls that shrink to a tip, banded, with a pink mouth."""
    rows, whorl_h, length = 26, 0.056, 0.22
    prof = [(0.0, 0.0)]
    for i in range(rows + 1):
        z = length * i / rows
        base = 0.07 * (1.0 - 0.9 * (z / length)) ** 0.9
        bulge = 1.0 + 0.3 * abs(math.sin(math.pi * z / whorl_h)) ** 0.5
        prof.append((max(0.004, base * bulge), z))
    prof.append((0.0, length + 0.004))
    shell = shapes.lathe("whelk", prof, seg=14)
    for v in shell.data.vertices:
        # twist the rings so the whorls spiral
        a = math.atan2(v.co.y, v.co.x) + v.co.z * 12.0
        r = math.hypot(v.co.x, v.co.y)
        v.co.x, v.co.y = r * math.cos(a), r * math.sin(a)
    sand, white, pink = p.pal.c("sand_wet"), p.pal.c("shell_white"), p.pal.c("shell_pink")
    brown = tone("#b8794a")
    # fn paints by final position: undo the placement to read the shell's own axis
    spin = math.radians(yaw)
    cy, sy = math.cos(spin), math.sin(spin)

    def paint(x, y, z):
        lx, ly = x - loc[0], y - loc[1]
        ax = lx * cy + ly * sy  # distance along the lying axis (the shell's own z)
        t = min(1.0, max(0.0, ax / (length * k)))
        c = mix(mix(sand, pink, 0.4), white, t ** 0.7)
        band = (ax / (whorl_h * k)) % 1.0
        if band < 0.2:
            c = mix(c, brown, 0.55)
        if ax < 0.012 * k:
            c = mix(c, tone("#e98a86"), 0.85)  # the mouth
        return c

    p.add(shell, "satin", "rope", fn=paint, loc=(loc[0], loc[1], loc[2] + 0.063 * k), rot=(0, 84, yaw), scale=(k, k, k), bevel=0, dens=0,
          ao=0.5, smooth_angle=70)


def build_shell(seed=1):
    """Group of sand-washed shells: a big scallop, a small one and a spiral whelk."""
    p = Prop(WORLD, "shell", seed)
    rng = p.rng
    _scallop(p, (-0.07, 0.02, 0.0), rng.uniform(0, 360), 1.0, tilt=(rng.uniform(-6, 6), rng.uniform(-6, 6)))
    _scallop(p, (0.0, -0.17, 0.0), rng.uniform(0, 360), 0.6, tilt=(rng.uniform(-10, 10), rng.uniform(-10, 10)))
    _whelk(p, (0.08, 0.11, 0.0), rng.uniform(-60, 60))
    return p.build()
