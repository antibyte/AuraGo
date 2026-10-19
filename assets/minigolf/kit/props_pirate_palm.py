"""Palm trees of the pirate bay (MIT License).

Two registrations share one builder: ``palm`` (leaning, classic coconut palm)
and ``palm2`` (taller, S-curved, bushier crown, different ring rhythm). The
trunk is a stack of rounded rings along a curved path, the crown carries dead
frond stubs and a coconut cluster, and every frond is a rachis with pinnate
leaflets that sweep towards the tip, droop and end in points. The fronds rise,
kink and hang, so no two read alike.
"""

import math

from mathutils import Vector

from . import shapes
from .palette import mix
from .prop import Prop
from .props_pirate_util import add_painted, orient_face, scaled, smoothstep, tone

WORLD = "pirate"
SWAY = 0.06  # shares the kit's foliage material (tuft, seaweed)

# Ring profile per trunk segment: (position along the segment, radius factor).
_RING = ((0.0, 0.90), (0.13, 1.03), (0.5, 1.07), (0.87, 1.03))

STYLES = {
    "palm": dict(rings=17, base_r=0.195, top_r=0.112, flare=0.075, side=0.09, wave=0.0, ring_depth=1.0,
                 pitch=(30, 48), inner_pitch=(62, 74), length=(1.75, 2.25), nuts=4, nut_r=0.085, spears=2, leaf=0.5),
    "palm2": dict(rings=14, base_r=0.17, top_r=0.1, flare=0.10, side=0.16, wave=0.14, ring_depth=1.35,
                  pitch=(34, 54), inner_pitch=(64, 78), length=(1.6, 2.1), nuts=3, nut_r=0.1, spears=3, leaf=0.46),
}


def _trunk_path(height, lean, side, wave, heading, rings):
    hx, hy = math.cos(heading), math.sin(heading)
    px, py = -hy, hx

    def at(t):
        bend = lean * t ** 1.7 + wave * math.sin(t * math.tau)
        sd = side * math.sin(t * math.pi) + 0.35 * wave * math.sin(t * math.tau * 1.5)
        return Vector((hx * bend + px * sd, hy * bend + py * sd, height * t))

    pts, mult = [], []
    for j in range(rings):
        for u, m in _RING:
            pts.append((j + u) / rings)
            mult.append(m)
    pts.append(1.0)
    mult.append(0.88)
    return at, pts, mult


def _trunk(p, style, height, lean, heading):
    rng = p.rng
    rings = style["rings"]
    at, ts, mults = _trunk_path(height, lean, style["side"], style["wave"], heading, rings)
    path, radii = [], []
    for t, m in zip(ts, mults):
        base = style["base_r"] - (style["base_r"] - style["top_r"]) * t + style["flare"] * (1.0 - t) ** 7
        depth = 1.0 + (m - 1.0) * style["ring_depth"]
        radii.append(base * depth)
        path.append(at(t))
    seg = 10
    trunk = shapes.tube("trunk", path, radius=radii, seg=seg)
    lo, hi = tone("#835733"), tone("#dba368")
    cols = []
    for t, m in zip(ts, mults):
        groove = 0.78 + 0.22 * smoothstep(0.90, 1.05, m)
        wob = 1.0 + 0.04 * (rng.random() - 0.5)
        c = mix(lo, hi, min(1.0, t ** 0.9 * 0.85 + 0.1))
        c = scaled(c, groove * wob)
        for k in range(seg):
            # darker back side gives the rings a soft turned-wood look
            side = 0.93 + 0.07 * math.cos(math.tau * k / seg - heading)
            cols.append(scaled(c, side))
    add_painted(p, trunk, "matte", cols, ao=0.6, smooth_angle=75)
    return at(1.0), at


def _crown(p, style, top, at):
    rng = p.rng
    dark, mid = tone("#6b4528"), tone("#835733")
    # frond-base knot
    knot = shapes.sphere("crown", 0.17, seg=12, rings=7)
    p.add(knot, "matte", (dark, mid), loc=(top.x, top.y, top.z + 0.01), scale=(1, 1, 0.8), bevel=0, dens=0, ao=0.5)
    # dead frond stubs hanging like a skirt
    n = 9
    for k in range(n):
        a = k * math.tau / n + rng.uniform(-0.2, 0.2)
        stub = shapes.lathe("stub", [(0.045, 0.0), (0.035, -0.12), (0.0, -0.17)], seg=7)
        p.add(stub, "matte", (dark, mid), loc=(top.x + math.cos(a) * 0.16, top.y + math.sin(a) * 0.16, top.z - 0.02),
              rot=(math.sin(a) * 38, -math.cos(a) * 38, 0), bevel=0, dens=0, ao=0.45)
    # coconut cluster
    nut_lo, nut_hi = tone("#4c2f18"), tone("#9a6a3a")
    for k in range(style["nuts"]):
        a = k * math.tau / style["nuts"] + rng.uniform(0.2, 0.9)
        rad = 0.13 + rng.uniform(0.0, 0.04)
        r = style["nut_r"] * rng.uniform(0.9, 1.1)
        nut = shapes.sphere("coconut", r, seg=10, rings=7)
        p.add(nut, "matte", (nut_lo, nut_hi), loc=(top.x + math.cos(a) * rad, top.y + math.sin(a) * rad, top.z - 0.14 - rng.uniform(0, 0.07)),
              scale=(1.0, 1.0, 1.12), bevel=0, dens=0, ao=0.5)


def _frond(verts, faces, cols, rng, top, a, length, pitch0, bend, kink, kink_at, curl, leaf, steps, tint):
    """One pinnate frond: rachis plus swept, drooping, pointed leaflets on both sides."""
    out = Vector((math.cos(a), math.sin(a), 0.0))
    side = Vector((-math.sin(a), math.cos(a), 0.0))
    up = Vector((0.0, 0.0, 1.0))
    ds = length / steps
    spine, tang = [], []
    pos = Vector(top) + out * 0.06 + up * 0.08
    for i in range(steps + 1):
        s = i / steps
        th = math.radians(pitch0 - bend * s - kink * smoothstep(kink_at - 0.09, kink_at + 0.09, s))
        t = (out * math.cos(th) + up * math.sin(th) + side * (curl * s)).normalized()
        spine.append(pos.copy())
        tang.append(t)
        pos = pos + t * ds
    rach_lo, rach_hi = tone("#9fcb55"), tone("#d9ec86")
    leaf_lo, leaf_hi, leaf_tip = tone("#1f7a3c", tint), tone("#4fb54a", tint), tone("#95e05c", tint)

    def add(point, colour):
        verts.append(tuple(point))
        cols.append(colour)
        return len(verts) - 1

    # rachis strip
    prev = None
    for i in range(steps + 1):
        s = i / steps
        b = tang[i].cross(up)
        if b.length < 1e-4:
            b = side
        b.normalize()
        w = 0.034 * (1.0 - 0.7 * s)
        c = mix(rach_lo, rach_hi, s)
        lift = up * 0.012
        row = (add(spine[i] - b * w + lift, c), add(spine[i] + b * w + lift, c))
        if prev is not None:
            faces.append(orient_face(verts, (prev[0], prev[1], row[1], row[0]), up))
        prev = row
    # leaflets
    first = 2
    for i in range(first, steps + 1):
        s = i / steps
        if s >= 0.995:
            continue
        T = tang[i]
        B = T.cross(up)
        if B.length < 1e-4:
            B = side
        B.normalize()
        hw = ds * 0.56
        ln = leaf * math.sin(math.pi * (0.10 + 0.88 * s)) ** 0.7 * (1.05 - 0.25 * s)
        ln *= 1.0 + 0.08 * math.sin(i * 2.3 + a)
        if ln < 0.04:
            continue
        for sd in (-1, 1):
            d = (B * sd * 0.82 + T * 0.38).normalized()
            droop = 0.20 + 0.30 * s
            root = spine[i]
            rows = []
            spec = ((0.0, 0.0, 1.0, 0.0), (0.36, 0.015, 1.0, 0.10), (0.7, 0.075, 0.78, 0.2), (1.0, 0.24, 0.0, 0.34))
            for f, dz, w, sweep in spec:
                centre = root + d * (ln * f) - up * (ln * dz * (1.0 + droop)) + T * (ln * sweep)
                rows.append((centre, hw * w))
            shade = 0.88 + 0.1 * (0.5 + 0.5 * math.sin(i * 1.7 + sd))
            ids = []
            for k, (centre, w) in enumerate(rows):
                f = spec[k][0]
                c = mix(leaf_lo, leaf_hi, min(1.0, 0.25 + 0.55 * f + 0.35 * s))
                if k == 3:
                    c = mix(c, leaf_tip, 0.8)
                c = scaled(c, shade)
                if k < 3:
                    ids.append((add(centre - T * w, c), add(centre + T * w, c)))
                else:
                    ids.append((add(centre, c),))
            hint = up + d * 0.25
            for k in range(2):
                faces.append(orient_face(verts, (ids[k][0], ids[k][1], ids[k + 1][1], ids[k + 1][0]), hint))
            faces.append(orient_face(verts, (ids[2][0], ids[2][1], ids[3][0]), hint))


def _fronds(p, style, top, count):
    rng = p.rng
    verts, faces, cols = [], [], []
    spears = style["spears"]
    for k in range(count):
        a = k * math.tau / count + rng.uniform(-0.22, 0.22)
        inner = k % 3 == 1
        if inner:
            pitch0 = rng.uniform(*style["inner_pitch"])
            length = rng.uniform(*style["length"]) * 0.8
            bend, kink, kink_at = rng.uniform(55, 80), rng.uniform(15, 30), rng.uniform(0.55, 0.7)
        else:
            pitch0 = rng.uniform(*style["pitch"])
            length = rng.uniform(*style["length"])
            bend, kink, kink_at = rng.uniform(70, 100), rng.uniform(25, 52), rng.uniform(0.42, 0.62)
        _frond(verts, faces, cols, rng, top, a, length, pitch0, bend, kink, kink_at, rng.uniform(-0.55, 0.55),
               style["leaf"] * rng.uniform(0.9, 1.1), 15, rng.uniform(0.92, 1.08))
    for k in range(spears):
        a = rng.uniform(0, math.tau)
        _frond(verts, faces, cols, rng, top, a, rng.uniform(0.75, 1.0), rng.uniform(78, 86), rng.uniform(12, 24), rng.uniform(6, 14),
               0.6, rng.uniform(-0.2, 0.2), style["leaf"] * 0.6, 9, 1.08)
    obj = shapes.make_object("fronds", verts, faces)
    add_painted(p, obj, "foliage", cols, ao=0.55, smooth_angle=80, sway=SWAY)


def _build(name, seed, height, lean, fronds, heading, style_key):
    style = STYLES[style_key]
    p = Prop(WORLD, name, seed)
    rng = p.rng
    heading = rng.uniform(0, math.tau) if heading is None else math.radians(heading)
    top, at = _trunk(p, style, height, lean, heading)
    _crown(p, style, top, at)
    _fronds(p, style, top, fronds)
    return p.build()


def build_palm(seed=1, height=2.65, lean=0.55, fronds=10, heading=None):
    """Classic leaning palm: ring-segmented curved trunk, coconuts, kinked pinnate fronds."""
    return _build("palm", seed, height, lean, fronds, heading, "palm")


def build_palm2(seed=2, height=2.95, lean=0.9, fronds=9, heading=None):
    """Taller S-curved palm with a bushier crown and heavier coconuts."""
    return _build("palm2", seed, height, lean, fronds, heading, "palm2")
