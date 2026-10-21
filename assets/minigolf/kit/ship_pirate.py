"""Hero prop of the pirate bay: the beached pirate ship (MIT License).

Plank hull lofted from stations (seam grooves, red and gold wales, rim rail),
stern cabin with portholes and lantern, helm wheel, mast with crow's nest,
patched billowing sail, skull-and-crossbones flag and rigging.
Prop space: Y is the keel direction (bow +Y), keel bottom at z=0.
"""

import math

from mathutils import Vector

from . import colors, materials, shapes
from .palette import mix
from .prop import Prop

WORLD = "pirate"
CLOTH_SWAY = 0.05  # sail, patches and flag share one cloth material


def _rope(p, pts, radius=0.014, sag=0.0, grad="rope", seg=6):
    """Straight or sagging rope between two points."""
    out = []
    n = 10 if sag else 1
    for i in range(n + 1):
        t = i / n
        q = Vector(pts[0]).lerp(Vector(pts[-1]), t)
        q.z -= sag * 4 * t * (1 - t)
        out.append(q)
    obj = shapes.tube("rope", out, radius=radius, seg=seg)
    p.add(obj, "matte", grad, bevel=0, dens=0, ao=0.45)
    return obj


SHIP_LEN = 5.6
SHIP_BEAM = 1.12  # half beam
DECK_Z = 0.86
WALL = 0.085


def _beam(t):
    tw = 0.38
    if t <= tw:
        return SHIP_BEAM * (0.80 + 0.20 * math.sin(math.pi / 2 * t / tw))
    s = (t - tw) / (1 - tw)
    return SHIP_BEAM * max(0.0, 1 - s ** 2.3) ** 0.8


def _keel(t):
    if t < 0.35:
        return 0.16 * ((0.35 - t) / 0.35) ** 2
    return 0.78 * ((t - 0.35) / 0.65) ** 2.6


def _top(t):
    if t < 0.38:
        return 1.30 + 0.34 * ((0.38 - t) / 0.38) ** 2
    return 1.30 + 0.58 * ((t - 0.38) / 0.62) ** 2.2


def _hull_y(t):
    return -SHIP_LEN / 2 + SHIP_LEN * t


def _outer_curve(t, n):
    """Outer section as (x, z) from left gunwale, round the keel, to right gunwale."""
    b, z0, z1 = _beam(t), _keel(t), _top(t)
    pts = []
    for k in range(2 * n + 1):
        psi = -1 + k / n
        q = abs(psi)
        phi = q * math.pi / 2
        flare = 1.0 + 0.05 * q ** 2 - 0.09 * q ** 8
        x = math.copysign(b * (math.sin(phi) ** 0.78) * flare, psi) if q > 1e-9 else 0.0
        z = z0 + (z1 - z0) * (1 - math.cos(phi) ** 0.78)
        pts.append((x, z))
    return pts


def _hull_x_at(t, z, n=24):
    """Right-side outer x at height z (clamped to the section)."""
    pts = [p for p in _outer_curve(t, n) if p[0] >= 0]
    pts.sort(key=lambda p: p[1])
    if z <= pts[0][1]:
        return pts[0][0]
    for a, b in zip(pts, pts[1:]):
        if a[1] <= z <= b[1] and b[1] > a[1]:
            return a[0] + (b[0] - a[0]) * (z - a[1]) / (b[1] - a[1])
    return pts[-1][0]


def _build_hull(p, rng):
    n, m, deck_pts = 14, 4, 7
    stations = [i / 26.0 for i in range(26)] + [0.985]
    rings, cols, kinds = [], [], []
    red, gold = p.pal.c("red"), p.pal.c("gold")
    wlo, whi, wdeep = p.pal.c("wood_dark"), p.pal.c("wood_light"), p.pal.c("wood_deep")
    wmid, gap = p.pal.c("wood"), p.pal.c("plank_gap")
    for t in stations:
        y = _hull_y(t)
        z1, z0 = _top(t), _keel(t)
        zdeck = min(max(DECK_Z, z0 + 0.15), z1 - 0.12)
        ring, rcols = [], []
        outer = _outer_curve(t, n)
        for k, (x, z) in enumerate(outer):
            row = min(k, 2 * n - k)  # 0 at gunwale .. n at the keel
            seam = row % 2 == 0 and 0 < row < n
            sgn = math.copysign(1.0, x) if abs(x) > 1e-6 else 0.0
            rel = z1 - z
            c = mix(wlo, whi, min(1.0, (z - z0) / max(z1 - z0, 0.1)) ** 0.8)
            c = mix(wdeep, c, min(1.0, 0.45 + (z - z0) * 1.1))
            if rel <= 0.12:
                c = mix(whi, wmid, 0.35)
            plank = 1.0 + 0.05 * (1 if (row // 2) % 2 else -1) * math.sin(t * 9 + k)
            c = (c[0] * plank, c[1] * plank, c[2] * plank, 1.0)
            if seam:
                # two close rings per seam: a crisp dark groove line
                gap_c = mix(c, gap, 0.75)
                for dz in ((0.011, -0.011) if k < n else (-0.011, 0.011)):
                    ring.append((x - sgn * 0.022, y, z + dz))
                    rcols.append(gap_c)
                continue
            ring.append((x, y, z))
            rcols.append(c)
        # inner right wall, top to deck
        for i in range(m + 1):
            zz = z1 - (z1 - zdeck) * i / m
            x = max(_hull_x_at(t, zz) - WALL, 0.025)
            ring.append((x, y, zz))
            c = mix(whi, wmid, 0.35) if i == 0 else mix(p.pal.c("wood"), wlo, min(1.0, i / m))
            rcols.append(c)
        # deck, right to left (end points belong to the walls)
        xd = max(_hull_x_at(t, zdeck) - WALL, 0.025)
        for i in range(1, deck_pts + 1):
            f = i / (deck_pts + 1)
            x = xd * (1 - 2 * f)
            ring.append((x, y, zdeck))
            plank = 1.0 + 0.07 * (1 if i % 2 else -1)
            c = mix(p.pal.c("wood"), whi, 0.45)
            rcols.append((c[0] * plank, c[1] * plank, c[2] * plank, 1.0))
        for i in range(m + 1):
            zz = zdeck + (z1 - zdeck) * i / m
            x = -max(_hull_x_at(t, zz) - WALL, 0.025)
            ring.append((x, y, zz))
            c = mix(whi, wmid, 0.35) if i == m else mix(p.pal.c("wood"), wlo, 1 - i / m)
            rcols.append(c)
        rings.append(ring)
        cols.extend(rcols)
    obj = shapes.loft("ship-hull", rings, closed=True, cap_start=True, cap_end=True, recalc=True)
    shapes.finish(obj, bevel=0, dens=0, smooth_angle=60)
    colors.paint_values(obj, cols, ao=0.65)
    obj.data.materials.append(materials.get("matte"))
    p.parts.append(obj)
    _hull_bands(p, stations)
    _rim_rail(p, stations)


def _hull_bands(p, stations):
    """Crisp red and gold wale strips hugging the planking."""
    for (rlo, rhi), grad in (((0.30, 0.43), ("gold_dark", "gold_light")), ((0.43, 0.62), ("red_dark", "red"))):
        for side in (-1, 1):
            rings = []
            for t in stations[:-1]:
                z1 = _top(t)
                zlo, zhi = z1 - rhi, z1 - rlo
                zm = (zlo + zhi) / 2
                ring = []
                for u, z in ((0.0, zlo), (0.02, zlo + 0.014), (0.03, zm), (0.02, zhi - 0.014), (0.0, zhi)):
                    zz = zlo if z == zlo else (zhi if z == zhi else z)
                    x = _hull_x_at(t, zz) + u - 0.012
                    ring.append((side * x, _hull_y(t), zz))
                rings.append(ring)
            obj = shapes.loft("wale", rings, closed=True, recalc=True)
            p.add(obj, "metal" if grad[0] == "gold_dark" else "matte", grad, bevel=0, dens=0, ao=0.55, mode="z")


def _rim_rail(p, stations):
    """Rounded cap rail along the gunwale (the deck rail)."""
    from .lane import rail_profile
    w = WALL + 0.07
    prof = rail_profile(width=w, top=0.085, sink=0.04, arc=6)
    for side in (-1, 1):
        rings = []
        for t in stations[:-1]:
            z1 = _top(t)
            xin = max(_hull_x_at(t, z1) - WALL - 0.02, 0.01)
            ring = [(side * (xin + u), _hull_y(t), z1 + z) for u, z in prof]
            rings.append(ring)
        obj = shapes.loft("rim-rail", rings, closed=True, recalc=True)
        p.add(obj, "matte", ("wood", "wood_light"), bevel=0, dens=0, ao=0.6)


def _sail_point(u, v, width=2.9, top=4.25, bottom=2.15, billow=0.36, y0=0.62):
    """Billowed square sail; u across (-1..1), v down (0 top .. 1 bottom)."""
    z_bot = bottom + 0.22 * (1 - u * u) * 1.0
    z = top + (z_bot - top) * v
    y = y0 + billow * (1 - u * u) ** 0.8 * math.sin(math.pi * min(1.0, v * 0.92 + 0.04)) ** 0.9
    return Vector((u * width / 2, y, z))


def _sail(p):
    nu, nv = 18, 12
    verts, faces = [], []
    for j in range(nv + 1):
        for i in range(nu + 1):
            verts.append(_sail_point(-1 + 2 * i / nu, j / nv))
    for j in range(nv):
        for i in range(nu):
            a = j * (nu + 1) + i
            faces.append((a, a + 1, a + nu + 2, a + nu + 1))
    sail = shapes.make_object("sail", verts, faces)
    cream_lo, cream_hi = p.pal.grad("cream")

    def seams(x, y, z):
        s = abs(((z - 2.15) / 0.52) % 1.0 - 0.5)
        stitch = 0.93 if s > 0.46 else 1.0
        return stitch

    p.add(sail, "cloth", "cream", sway=CLOTH_SWAY, mode="z", solidify=0.02, bevel=0, dens=0, pattern=seams, noise_amp=0.04, ao=0.5)
    patches = [((-0.5, 0.0), (0.2, 0.52), "red_dark"), ((0.2, 0.62), (0.5, 0.84), "cream_dark"),
               ((-0.85, -0.38), (0.6, 0.9), "cream_dark"), ((0.3, 0.7), (0.08, 0.3), "red_dark")]
    for (u0, u1), (v0, v1), tone in patches:
        for face in (1, -1):
            pn, qn = 3, 3
            pv, pf = [], []
            for j in range(qn + 1):
                for i in range(pn + 1):
                    u = u0 + (u1 - u0) * i / pn
                    v = v0 + (v1 - v0) * j / qn
                    q = _sail_point(u, v)
                    q.y += 0.03 * face
                    pv.append(q)
            for j in range(qn):
                for i in range(pn):
                    a = j * (pn + 1) + i
                    pf.append((a, a + 1, a + pn + 2, a + pn + 1) if face > 0 else (a + pn + 1, a + pn + 2, a + 1, a))
            patch = shapes.make_object("sail-patch", pv, pf)
            lo, hi = ((p.pal.c("red_dark"), p.pal.c("red")) if tone == "red_dark" else (p.pal.c("cream_dark"), p.pal.c("rope")))
            p.add(patch, "cloth", (lo, hi), sway=CLOTH_SWAY, solidify=0.014, bevel=0, dens=0, ao=0.5, jitter=0.02)


def _skull_flag(p, x, y, z):
    """Black flag with a white skull and crossbones, blowing towards -Y."""
    w, h = 0.95, 0.62
    nx, ny = 12, 6
    verts, faces = [], []
    for j in range(ny + 1):
        for i in range(nx + 1):
            u, v = i / nx, j / ny
            verts.append((x, y - u * w, z + h * (0.5 - v) - 0.04 * math.sin(u * 4.2) * u))
            verts[-1] = (x + 0.05 * math.sin(u * 4.4) * u, verts[-1][1], verts[-1][2])
    for j in range(ny):
        for i in range(nx):
            a = j * (nx + 1) + i
            faces.append((a + nx + 1, a + nx + 2, a + 1, a))
    flag = shapes.make_object("flag-cloth", verts, faces)
    p.add(flag, "cloth", ("black", "iron_dark"), sway=CLOTH_SWAY, solidify=0.014, bevel=0, dens=0, ao=0.3, mode="y")
    for fs in (1, -1):
        cx, cy, cz = x + 0.07 + fs * 0.0, y - 0.45, z + 0.01
        cx = x + fs * 0.075

        def at(dx, dy, dz, fs=fs, cx=cx, cy=cy, cz=cz):
            return (cx + fs * dx, cy + dy, cz + dz)

        skull = shapes.sphere("skull", 0.115, seg=10, rings=6)
        p.add(skull, "cloth", ("cream_dark", "skull"), sway=CLOTH_SWAY, loc=at(0, 0, 0.07), scale=(0.28, 1.0, 1.0), bevel=0, dens=0, ao=0.2)
        jaw = shapes.box("jaw", (0.05, 0.1, 0.07), base=True)
        p.add(jaw, "cloth", ("cream_dark", "skull"), sway=CLOTH_SWAY, loc=at(0, 0, -0.06), bevel=0.012, segments=1, dens=0, ao=0.2)
        for e in (-1, 1):
            eye = shapes.cylinder("eye", 0.03, 0.03, 0.03, seg=8)
            p.add(eye, "cloth", ("black", "black"), sway=CLOTH_SWAY, loc=at(0.02, e * 0.045, 0.075), rot=(0, 90, 0), bevel=0, dens=0,
                  ao=0.0)
        for ang in (38, -38):
            bone = shapes.cylinder("bone", 0.016, 0.016, 0.36, seg=6, base=False)
            p.add(bone, "cloth", ("cream_dark", "skull"), sway=CLOTH_SWAY, loc=at(0, 0, -0.01), rot=(ang, 0, 0), bevel=0, dens=0, ao=0.2)
            for e in (-1, 1):
                knob = shapes.sphere("knob", 0.03, seg=6, rings=4)
                a = math.radians(ang)
                p.add(knob, "cloth", ("cream_dark", "skull"), sway=CLOTH_SWAY,
                      loc=at(0, -e * 0.18 * math.sin(a), -0.01 + e * 0.18 * math.cos(a)), bevel=0, dens=0, ao=0.2)


def build_ship(seed=3):
    """Beached pirate ship: plank hull, poop cabin, mast, patched sail, skull flag."""
    p = Prop(WORLD, "ship", seed)
    rng = p.rng
    _build_hull(p, rng)

    def width_at(y):
        t = (y + SHIP_LEN / 2) / SHIP_LEN
        return max(_hull_x_at(t, DECK_Z + 0.2) - WALL, 0.05)

    # stern cabin (poop) --------------------------------------------------
    cy0, cy1 = -2.48, -1.22
    cw = width_at((cy0 + cy1) / 2) * 2 - 0.12
    cabin = shapes.box("cabin", (cw, cy1 - cy0, 1.0), base=True)
    p.add(cabin, "matte", ("wood", "wood_light"), loc=(0, (cy0 + cy1) / 2, DECK_Z - 0.02), bevel=0.035, segments=2, dens=0.3,
          noise_amp=0.04)
    roof = shapes.box("cabin-roof", (cw + 0.14, cy1 - cy0 + 0.14, 0.09), base=True)
    p.add(roof, "matte", ("wood_dark", "wood"), loc=(0, (cy0 + cy1) / 2, DECK_Z + 0.97), bevel=0.025, segments=2, dens=0.3)
    for s in (-1, 1):
        for k in range(2):
            ring = shapes.torus("porthole-ring", 0.085, 0.022, seg=12, rseg=5)
            ypos = cy0 + 0.42 + k * 0.5
            p.add(ring, "metal", ("gold_dark", "gold_light"), loc=(s * (cw / 2 + 0.005), ypos, DECK_Z + 0.58), rot=(0, 90, 0),
                  bevel=0, dens=0, ao=0.4)
            glass = shapes.cylinder("porthole-glass", 0.075, 0.075, 0.02, seg=14, base=False)
            p.add(glass, "emissive", ("glass", "glass"), loc=(s * (cw / 2 - 0.005), ypos, DECK_Z + 0.58), rot=(0, 90, 0), bevel=0,
                  dens=0, ao=0.0, strength=1.6, flicker=0.35)
    door = shapes.box("cabin-door", (0.42, 0.04, 0.72), base=True)
    p.add(door, "matte", ("wood_dark", "wood"), loc=(0, cy1 + 0.005, DECK_Z), bevel=0.012, segments=1, dens=0)
    # railing on the roof
    rz = DECK_Z + 1.06
    for s in (-1, 1):
        for k in range(5):
            post = shapes.cylinder("baluster", 0.022, 0.022, 0.24, seg=8)
            p.add(post, "matte", "wood_dark", loc=(s * (cw / 2 + 0.03), cy0 + 0.12 + k * (cy1 - cy0 - 0.24) / 4, rz), bevel=0,
                  dens=0, ao=0.5)
        rail = shapes.box("roof-rail", (0.05, cy1 - cy0 + 0.05, 0.045), base=True)
        p.add(rail, "matte", "wood", loc=(s * (cw / 2 + 0.03), (cy0 + cy1) / 2, rz + 0.24), bevel=0.01, segments=1, dens=0.5)
    back = shapes.box("roof-rail-back", (cw + 0.1, 0.05, 0.045), base=True)
    p.add(back, "matte", "wood", loc=(0, cy0 + 0.02, rz + 0.24), bevel=0.01, segments=1, dens=0.5)
    # stern lantern
    lpost = shapes.cylinder("lantern-post", 0.025, 0.02, 0.5, seg=8)
    p.add(lpost, "matte", "wood_dark", loc=(0, cy0 - 0.02, rz), bevel=0.004, dens=0)
    glow = shapes.sphere("lantern", 0.085, seg=12, rings=8)
    p.add(glow, "emissive", ("gold", "gold_light"), loc=(0, cy0 - 0.02, rz + 0.58), scale=(1, 1, 1.2), bevel=0, dens=0,
          ao=0.0, strength=1.6, flicker=0.35)
    cap = shapes.cone("lantern-cap", 0.1, 0.09, seg=12)
    p.add(cap, "metal", "iron", loc=(0, cy0 - 0.02, rz + 0.7), bevel=0, dens=0, ao=0.4)

    # cannon ports -----------------------------------------------------------
    for ty in (0.27, 0.42, 0.57):
        t = ty
        z1 = _top(t)
        pz = z1 - 0.72
        px = _hull_x_at(t, pz) - 0.01
        for s in (-1, 1):
            port = shapes.box("port", (0.05, 0.3, 0.26), base=False)
            p.add(port, "matte", ("coal", "black"), loc=(s * px, _hull_y(t), pz), rot=(0, 0, 0), bevel=0.018, segments=1, dens=0,
                  ao=0.7)
            lid = shapes.box("port-lid", (0.03, 0.34, 0.06), base=False)
            p.add(lid, "matte", ("wood_dark", "wood"), loc=(s * (px + 0.015), _hull_y(t), pz + 0.16), bevel=0.01, segments=1, dens=0)

    # mast -----------------------------------------------------------------
    my = 0.45
    mast = shapes.cylinder("mast", 0.12, 0.065, 5.2, seg=14)
    p.add(mast, "matte", ("wood_dark", "wood_light"), loc=(0, my, DECK_Z - 0.2), bevel=0, dens=0.9, noise_amp=0.04)
    collar = shapes.cylinder("mast-collar", 0.17, 0.17, 0.16, seg=14)
    p.add(collar, "matte", "wood_dark", loc=(0, my, DECK_Z - 0.05), bevel=0.02, dens=0.3)
    yard = shapes.cylinder("yard", 0.055, 0.04, 3.4, seg=10, base=False)
    p.add(yard, "matte", ("wood_dark", "wood"), loc=(0, my + 0.17, 4.28), rot=(0, 90, 0), bevel=0, dens=0.9)
    for s in (-1, 1):
        cap_end = shapes.sphere("yard-cap", 0.05, seg=8, rings=5)
        p.add(cap_end, "matte", "wood_dark", loc=(s * 1.7, my + 0.17, 4.28), bevel=0, dens=0)
    nest_prof = [(0.0, 0.0), (0.2, 0.0), (0.27, 0.12), (0.3, 0.26), (0.27, 0.26), (0.24, 0.13), (0.0, 0.1)]
    nest = shapes.lathe("nest", nest_prof, seg=14)
    p.add(nest, "matte", ("wood_dark", "wood"), loc=(0, my, 4.55), bevel=0, dens=0.3)
    topcap = shapes.cone("mast-top", 0.07, 0.2, seg=10)
    p.add(topcap, "metal", ("gold_dark", "gold_light"), loc=(0, my, 5.0), bevel=0, dens=0, ao=0.3)
    # sail, flag, rigging
    _sail(p)
    _skull_flag(p, 0.0, my - 0.02, 5.05)
    # forestay and backstay
    top = (0.0, my, 4.95)
    _rope(p, [top, (0.0, 3.1, 1.75)], radius=0.016, sag=0.08)
    _rope(p, [top, (0.0, -2.35, 1.9)], radius=0.016, sag=0.08)
    tm = (my + SHIP_LEN / 2) / SHIP_LEN
    gz = _top(tm) - 0.12
    for s in (-1, 1):
        gx = _hull_x_at(tm, gz) - 0.05
        a0, a1 = Vector((0.0, my - 0.06, 4.0)), Vector((s * gx, my - 0.3, gz))
        b0, b1 = Vector((0.0, my + 0.04, 4.0)), Vector((s * gx, my + 0.3, gz))
        _rope(p, [a0, a1], radius=0.014)
        _rope(p, [b0, b1], radius=0.014)
        for k in range(1, 6):
            f = k / 6.0
            _rope(p, [a0.lerp(a1, f), b0.lerp(b1, f)], radius=0.009, seg=4)
    # bowsprit and bow lantern
    bs = shapes.tube("bowsprit", [(0, 2.35, 1.55), (0, 3.15, 1.85), (0, 3.95, 2.25)], radius=[0.085, 0.062, 0.04], seg=10)
    p.add(bs, "matte", ("wood_dark", "wood"), bevel=0, dens=0, ao=0.5)
    # helm wheel
    wheel = shapes.torus("wheel", 0.2, 0.028, seg=22, rseg=8)
    wz = DECK_Z + 0.62
    wy = cy1 + 0.38
    p.add(wheel, "matte", ("wood_dark", "wood_light"), loc=(0, wy, wz), rot=(90, 0, 0), bevel=0, dens=0, ao=0.5)
    for k in range(8):
        a = k * 45
        spoke = shapes.cylinder("spoke", 0.014, 0.014, 0.5, seg=6, base=False)
        p.add(spoke, "matte", "wood_dark", loc=(0, wy, wz), rot=(90, 0, a), bevel=0, dens=0, ao=0.4)
    hub = shapes.cylinder("hub", 0.045, 0.045, 0.08, seg=10, base=False)
    p.add(hub, "metal", ("gold_dark", "gold_light"), loc=(0, wy, wz), rot=(90, 0, 0), bevel=0, dens=0, ao=0.3)
    post = shapes.cylinder("wheel-post", 0.05, 0.05, 0.62, seg=8)
    p.add(post, "matte", "wood_dark", loc=(0, wy + 0.04, DECK_Z), bevel=0.006, dens=0)
    obj = p.build()
    return obj
