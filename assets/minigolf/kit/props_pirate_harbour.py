"""Harbour props of the pirate bay: lighthouse, mooring post, buoy, flagpole (MIT License).

Same prop contract as ``props_pirate``: one joined mesh ``prop-pirate-<name>``,
Z-up, base at z=0, footprint centred, front towards +Y, built from ``seed``.
Stripes are painted per vertex from the final position, so the lathe profiles
carry doubled rows at every stripe edge to keep the edges crisp.
"""

import math

from . import shapes
from .prop import Prop
from .palette import mix
from .props_pirate_util import add_painted, scaled, smoothstep

WORLD = "pirate"
CLOTH_SWAY = 0.05  # the ship's cloth material (sail, flag)


def _banded_profile(radius, z0, z1, edges, eps=0.002):
    """Lathe profile ``[(r, z), ...]`` with a doubled row at each stripe edge."""
    zs = [z0]
    for edge in edges:
        zs.extend((edge - eps, edge + eps))
    zs.append(z1)
    return [(radius(z), z) for z in zs]


def _stripes(edges, z0, z1, colours):
    """``fn(x, y, z)`` returning the stripe colour of the band containing z (ramped over the whole height)."""

    def fn(x, y, z):
        band = sum(1 for e in edges if z >= e)
        lo, hi = colours[band % len(colours)]
        return mix(lo, hi, min(1.0, max(0.0, (z - z0) / (z1 - z0))))

    return fn


def _interp(ctrl, z):
    """Piecewise-linear radius lookup over ``[(z, r), ...]``."""
    if z <= ctrl[0][0]:
        return ctrl[0][1]
    for (za, ra), (zb, rb) in zip(ctrl, ctrl[1:]):
        if z <= zb:
            return ra + (rb - ra) * (z - za) / (zb - za)
    return ctrl[-1][1]


def _arch_prism(name, width, height, depth, seg=8):
    """Door-shaped prism (rectangle with a half-round head) extruded along Y, base at z=0."""
    pts = [(-width / 2, 0.0), (width / 2, 0.0), (width / 2, height - width / 2)]
    for k in range(1, seg):
        a = math.pi * k / seg
        pts.append((width / 2 * math.cos(a), height - width / 2 + width / 2 * math.sin(a)))
    pts.extend([(-width / 2, height - width / 2)])
    rings = [[(x, y, z) for x, z in pts] for y in (-depth / 2, depth / 2)]
    return shapes.loft(name, rings, closed=True, recalc=True)


# --------------------------------------------------------------------------
# Lighthouse
# --------------------------------------------------------------------------

def build_lighthouse(seed=1):
    """Red-white striped lighthouse on a rocky foot with gallery, lamp room and roof (2.4 m)."""
    p = Prop(WORLD, "lighthouse", seed)
    rng = p.rng
    pal = p.pal
    z0, z1 = 0.2, 1.72
    r0, r1 = 0.46, 0.31

    def radius(z):
        return r0 + (r1 - r0) * (z - z0) / (z1 - z0)

    # rocky foot: one soft plinth and a ring of boulders
    foot = shapes.lathe("foot", [(0.0, 0.0), (0.72, 0.0), (0.78, 0.05), (0.72, 0.14), (0.64, 0.2), (0.58, 0.222), (0.52, 0.226),
                                 (0.0, 0.226)], seg=26)
    for v in foot.data.vertices:
        a = math.atan2(v.co.y, v.co.x)
        k = 1.0 + 0.045 * math.sin(a * 3 + 0.7) + 0.025 * math.sin(a * 7 + 2.0)
        v.co.x *= k
        v.co.y *= k
    p.add(foot, "matte", "rock", bevel=0.02, segments=2, dens=0, noise_amp=0.09, smooth_angle=60)
    for a, size, sq in ((20, 0.27, (1.0, 0.9, 0.75)), (110, 0.2, (1.1, 0.9, 0.7)), (205, 0.3, (1.0, 0.85, 0.7)),
                        (290, 0.22, (1.0, 1.0, 0.75))):
        rock = shapes.blob("foot-rock", size, rng, subdiv=2, amp=0.2, freq=1.3, squash=sq, cuts=2, flat_bottom=0.45, relax=2)
        ar = math.radians(a)
        p.add(rock, "matte", "rock", loc=(math.cos(ar) * 0.7, math.sin(ar) * 0.7, 0.0), rot=(0, 0, a * 2), bevel=0, dens=0, ao=0.65,
              noise_amp=0.08, smooth_angle=80)

    # tower: five stripes, red first
    edges = [z0 + (z1 - z0) * k / 5.0 for k in range(1, 5)]
    red_lo, red_hi = pal.c("red_dark"), pal.c("red")
    white_lo, white_hi = mix(pal.c("cream_dark"), pal.c("skull"), 0.35), pal.c("skull")
    tower = shapes.lathe("tower", _banded_profile(radius, z0, z1, edges), seg=28)
    p.add(tower, "matte", "rock", fn=_stripes(edges, z0, z1, ((red_lo, red_hi), (white_lo, white_hi))), bevel=0, dens=0, ao=0.62,
          smooth_angle=60)

    # door with arched head and a pale frame, windows
    for width, height, tint, dy in ((0.19, 0.36, ("wood", "wood"), 0.0), (0.25, 0.42, ("cream_dark", "cream_dark"), -0.014)):
        door = _arch_prism("door", width, height, 0.05)
        p.add(door, "matte", tint, loc=(0, radius(0.4) - 0.008 + dy, z0 + 0.005), bevel=0.008, segments=1, dens=0, ao=0.5, smooth_angle=50)
    for z, ang in ((0.78, 70.0), (1.16, 200.0), (1.5, 320.0)):
        ar = math.radians(ang)
        for scale, tint, inset in ((1.0, ("coal", "black"), 0.0), (1.45, ("cream_dark", "cream_dark"), 0.008)):
            rr = radius(z) - 0.004 - inset
            win = shapes.box("window", (0.085 * scale, 0.04 if scale == 1.0 else 0.03, 0.14 * scale), base=False)
            p.add(win, "matte", tint, loc=(math.cos(ar) * rr, math.sin(ar) * rr, z), rot=(0, 0, ang - 90.0), bevel=0.008, segments=1,
                  dens=0, ao=0.45)

    # gallery with balustrade (iron)
    gal = shapes.lathe("gallery", [(r1 - 0.02, 1.6), (0.47, 1.72), (0.47, 1.8), (0.0, 1.8)], seg=28)
    p.add(gal, "metal", "iron", bevel=0.01, segments=1, dens=0.15, ao=0.5, smooth_angle=60)
    posts = []
    for k in range(14):
        a = math.tau * k / 14
        post = shapes.cylinder("baluster", 0.014, 0.014, 0.17, seg=5)
        shapes.transform(post, loc=(math.cos(a) * 0.44, math.sin(a) * 0.44, 1.8))
        posts.append(post)
    p.add(shapes.join("balusters", posts), "metal", "iron", bevel=0, dens=0, ao=0.45, smooth_angle=60)
    rail = shapes.torus("rail", 0.44, 0.017, seg=28, rseg=6)
    p.add(rail, "metal", ("iron", "iron_light"), loc=(0, 0, 1.97), bevel=0, dens=0, ao=0.4)

    # lamp room: glowing core, iron frame, red roof
    lamp = shapes.cylinder("lamp", 0.215, 0.215, 0.3, seg=18)
    p.add(lamp, "emissive", ("gold", "gold_light"), loc=(0, 0, 1.83), bevel=0, dens=0, ao=0.0, strength=1.6)
    for ring_z in (1.82, 2.14):
        ring = shapes.torus("lamp-ring", 0.232, 0.02, seg=16, rseg=5)
        p.add(ring, "metal", ("iron", "iron_light"), loc=(0, 0, ring_z), bevel=0, dens=0, ao=0.4)
    bars = []
    for k in range(8):
        a = math.tau * (k + 0.5) / 8
        bar = shapes.cylinder("mullion", 0.015, 0.015, 0.32, seg=5)
        shapes.transform(bar, loc=(math.cos(a) * 0.232, math.sin(a) * 0.232, 1.82))
        bars.append(bar)
    p.add(shapes.join("mullions", bars), "metal", "iron", bevel=0, dens=0, ao=0.45, smooth_angle=60)
    roof = shapes.lathe("roof", [(0.31, 2.14), (0.31, 2.18), (0.24, 2.25), (0.12, 2.33), (0.0, 2.37)], seg=20)
    p.add(roof, "matte", "red", bevel=0.01, segments=1, dens=0.15, ao=0.55, smooth_angle=60)
    knob = shapes.sphere("finial", 0.035, seg=10, rings=6)
    p.add(knob, "metal", ("iron_light", "iron_light"), loc=(0, 0, 2.368), bevel=0, dens=0, ao=0.3)
    return p.build()


# --------------------------------------------------------------------------
# Harbour post
# --------------------------------------------------------------------------

def _rope_tube(p, pts, radius=0.022, seg=6):
    """Twisted-looking rope: alternating light and dark rings along the path."""
    tube = shapes.tube("rope", pts, radius=radius, seg=seg)
    lo, hi = p.pal.grad("rope")
    cols = []
    for i in range(len(pts)):
        k = 1.0 if i % 2 else 0.84
        t = 0.4 + 0.6 * (0.5 + 0.5 * math.sin(i * 0.35))
        cols.extend([scaled(mix(lo, hi, t), k)] * seg)
    add_painted(p, tube, "matte", cols, ao=0.5, smooth_angle=60)


def build_post(seed=1, height=1.15):
    """Weathered mooring timber with a slanted cut head, an iron band, a rope wrapped round it and a coiled tail."""
    p = Prop(WORLD, "post", seed)
    h = height
    prof = [(0.0, 0.0), (0.15, 0.0), (0.136, 0.05), (0.118, 0.2), (0.108, h - 0.16), (0.112, h - 0.08), (0.106, h - 0.035),
            (0.092, h - 0.004), (0.055, h), (0.0, h)]
    body = shapes.lathe("post-body", prof, seg=12)
    for v in body.data.vertices:
        # slanted saw cut across the head
        v.co.z += 0.07 * v.co.x / 0.11 * min(1.0, max(0.0, (v.co.z - (h - 0.08)) / 0.08))

    def grain(x, y, z):
        return 0.9 + 0.1 * math.sin(math.atan2(y, x) * 5.0 + z * 3.0)

    p.add(body, "matte", ("wood_dark", "wood"), bevel=0, dens=0.12, pattern=grain, noise_amp=0.07, smooth_angle=65, ao=0.6)
    band = shapes.lathe("band", [(0.119, 0.0), (0.134, 0.0), (0.134, 0.035), (0.119, 0.035)], seg=12, caps=False)
    p.add(band, "matte", "iron", loc=(0, 0, 0.14), bevel=0.004, segments=1, dens=0, ao=0.4)
    # iron mooring ring on the harbour side, held by an eye bolt
    ring = shapes.torus("mooring-ring", 0.05, 0.012, seg=14, rseg=5)
    p.add(ring, "matte", ("iron_dark", "iron_light"), loc=(0.0, 0.118, h - 0.3), rot=(90, 0, 0), bevel=0, dens=0, ao=0.4)
    stub = shapes.cylinder("ring-eye", 0.014, 0.014, 0.05, seg=6, base=False)
    p.add(stub, "matte", "iron", loc=(0.0, 0.106, h - 0.25), rot=(90, 0, 0), bevel=0, dens=0, ao=0.4)

    # helical wrap on the back half of the post
    z_lo, z_hi, turns = h * 0.5, h * 0.76, 3.2
    n = 44
    helix = []
    for i in range(n + 1):
        t = i / n
        a = turns * math.tau * t + math.pi / 2.0
        rr = 0.114 + 0.02
        helix.append((math.cos(a) * rr, math.sin(a) * rr, z_lo + (z_hi - z_lo) * t))
    _rope_tube(p, helix, 0.023, 6)
    # loose end hangs down and ends in a flat coil beside the post
    base_a = (turns * math.tau + math.pi / 2.0) % math.tau
    d_c, rho0 = 0.36, 0.15
    path = [helix[-1]]
    for q in (0.25, 0.5, 0.75):
        rr = 0.134 + (0.2 - 0.134) * q
        path.append((math.cos(base_a) * rr, math.sin(base_a) * rr, z_hi - (z_hi - 0.03) * q ** 0.8))
    cx, cy = math.cos(base_a) * d_c, math.sin(base_a) * d_c
    steps = 34
    for i in range(1, steps + 1):
        f = i / steps
        phi = math.pi + 1.55 * math.tau * f
        rho = rho0 * (1.0 - 0.52 * f)
        path.append((cx + math.cos(base_a + phi) * rho, cy + math.sin(base_a + phi) * rho, 0.024 + 0.004 * f))
    _rope_tube(p, path, 0.022, 6)
    return p.build()


# --------------------------------------------------------------------------
# Buoy with a little sail
# --------------------------------------------------------------------------

def _sail(p, base, height, width, billow):
    """Small triangular sail with its luff on the mast at ``base`` (x, y, z), billowing to +Y.

    Returns the unfinished object and its per-vertex colours (rows of ``nx + 1``
    vertices, then the single apex).
    """
    nx, nz = 5, 6
    verts, faces, cols = [], [], []
    cream_lo, cream_hi = p.pal.grad("cream")
    red_lo, red_hi = p.pal.grad("red")
    for j in range(nz):
        v = j / nz
        span = width * (1.0 - v) ** 0.85
        for i in range(nx + 1):
            u = i / nx
            verts.append((base[0] + span * u, base[1] + billow * math.sin(math.pi * u) * (1.0 - v) ** 0.7, base[2] + height * v))
            c = mix(cream_lo, cream_hi, 0.4 + 0.5 * (1.0 - v))
            cols.append(mix(red_lo, red_hi, 0.4 + 0.5 * v) if v > 0.6 else c)
    verts.append((base[0], base[1], base[2] + height))
    cols.append(mix(red_lo, red_hi, 0.9))
    for j in range(nz - 1):
        for i in range(nx):
            a = j * (nx + 1) + i
            faces.append((a, a + 1, a + nx + 2, a + nx + 1))
    apex = len(verts) - 1
    last = (nz - 1) * (nx + 1)
    for i in range(nx):
        faces.append((last + i, last + i + 1, apex))
    return shapes.make_object("sail", verts, faces), cols


def build_buoy(seed=1):
    """Red-white sailing buoy: banded float, iron ring, mast with a small sail and a blinking lamp."""
    p = Prop(WORLD, "buoy", seed)
    pal = p.pal
    top = 0.62
    ctrl = [(0.0, 0.16), (0.04, 0.215), (0.1, 0.255), (0.2, 0.27), (0.4, 0.268), (0.48, 0.245), (0.55, 0.18), (0.62, 0.095)]
    edges = (0.15, 0.31, 0.47)
    red_lo, red_hi = pal.c("red_dark"), pal.c("red")
    white_lo, white_hi = mix(pal.c("cream_dark"), pal.c("skull"), 0.35), pal.c("skull")
    prof = [(0.0, 0.0)] + _banded_profile(lambda z: _interp(ctrl, z), 0.0, top, edges) + [(0.06, top + 0.03), (0.0, top + 0.035)]
    float_ = shapes.lathe("float", prof, seg=22)
    p.add(float_, "matte", "red", fn=_stripes(edges, 0.0, top, ((red_lo, red_hi), (white_lo, white_hi))), bevel=0, dens=0, ao=0.6,
          smooth_angle=60)
    ring = shapes.torus("ring", 0.272, 0.019, seg=24, rseg=6)
    p.add(ring, "matte", ("iron_dark", "iron_light"), loc=(0, 0, 0.23), bevel=0, dens=0, ao=0.4)
    for a in (35.0, 215.0):
        ar = math.radians(a)
        eye = shapes.torus("eye", 0.04, 0.011, seg=10, rseg=5)
        p.add(eye, "matte", "iron", loc=(math.cos(ar) * 0.315, math.sin(ar) * 0.315, 0.23), rot=(90, 0, a), bevel=0, dens=0, ao=0.4)
    mast = shapes.cylinder("mast", 0.028, 0.02, 0.4, seg=8)
    p.add(mast, "matte", ("iron_dark", "iron_light"), loc=(0, 0, top + 0.02), bevel=0, dens=0.2, ao=0.5)
    collar = shapes.cone("collar", 0.075, 0.07, seg=10)
    p.add(collar, "matte", ("iron_dark", "iron"), loc=(0, 0, top + 0.02), bevel=0, dens=0, ao=0.4)
    cap = shapes.cone("topmark", 0.055, 0.1, seg=8)
    p.add(cap, "matte", ("red_dark", "red"), loc=(0, 0, top + 0.34), bevel=0, dens=0, ao=0.4)
    lamp = shapes.sphere("lamp", 0.05, seg=10, rings=7)
    p.add(lamp, "emissive", ("gold", "gold_light"), loc=(0, 0, top + 0.48), bevel=0, dens=0, ao=0.0, strength=1.6, flicker=0.35)
    sail, cols = _sail(p, (0.028, 0.0, top + 0.08), 0.28, 0.28, 0.06)
    add_painted(p, sail, "cloth", cols, ao=0.5, smooth_angle=80, sway=CLOTH_SWAY)
    return p.build()


# --------------------------------------------------------------------------
# Flagpole with a skull flag
# --------------------------------------------------------------------------

def _flag(p, x, z_top, width=1.0, height=0.62):
    """Black swallow-tail flag with skull and crossbones on both faces, blowing towards -Y."""
    nx, ny = 14, 7

    def wave(u, v):
        return x + 0.06 * math.sin(u * 4.6 + 0.4) * u + 0.02 * math.sin(v * 5.0) * u

    verts, faces = [], []
    for j in range(ny + 1):
        v = j / ny
        for i in range(nx + 1):
            u = i / nx
            notch = 0.26 * (1.0 - abs(2.0 * v - 1.0)) * smoothstep(0.55, 1.0, u)
            verts.append((wave(u, v), -u * (1.0 - notch) * width, z_top - height * v - 0.05 * math.sin(u * 3.0) * u))
    for j in range(ny):
        for i in range(nx):
            a = j * (nx + 1) + i
            faces.append((a + nx + 1, a + nx + 2, a + 1, a))
    flag = shapes.make_object("flag-cloth", verts, faces)
    p.add(flag, "cloth", ("black", "iron_dark"), sway=CLOTH_SWAY, solidify=0.014, bevel=0, dens=0, ao=0.3, mode="y")
    cy, cz = -0.44 * width, z_top - height * 0.5

    def surface(dy, dz):
        """Flag surface x at an offset (dy, dz) from the emblem centre."""
        return wave(min(1.0, max(0.0, -(cy + dy) / width)), min(1.0, max(0.0, (z_top - (cz + dz)) / height)))

    for fs in (1, -1):

        def at(dx, dy, dz, fs=fs):
            return (surface(dy, dz) + fs * (0.008 + dx), cy + dy, cz + dz)

        skull = shapes.sphere("skull", 0.14, seg=10, rings=6)
        p.add(skull, "cloth", ("cream_dark", "skull"), sway=CLOTH_SWAY, loc=at(0.005, 0, 0.06), scale=(0.26, 1.0, 1.0), bevel=0, dens=0,
              ao=0.2)
        jaw = shapes.box("jaw", (0.05, 0.11, 0.075), base=False)
        p.add(jaw, "cloth", ("cream_dark", "skull"), sway=CLOTH_SWAY, loc=at(0.0, 0, -0.04), bevel=0.012, segments=1, dens=0, ao=0.2)
        for e in (-1, 1):
            eye = shapes.cylinder("eye", 0.032, 0.032, 0.03, seg=8)
            p.add(eye, "cloth", ("black", "black"), sway=CLOTH_SWAY, loc=at(0.04, e * 0.052, 0.075), rot=(0, 90, 0), bevel=0, dens=0, ao=0.0)
        for ang in (38, -38):
            bone = shapes.cylinder("bone", 0.017, 0.017, 0.4, seg=6, base=False)
            p.add(bone, "cloth", ("cream_dark", "skull"), sway=CLOTH_SWAY, loc=at(-0.004, 0, -0.1), rot=(ang, 0, 0), bevel=0, dens=0, ao=0.2)
            for e in (-1, 1):
                knob = shapes.sphere("knob", 0.032, seg=6, rings=4)
                a = math.radians(ang)
                p.add(knob, "cloth", ("cream_dark", "skull"), sway=CLOTH_SWAY,
                      loc=at(-0.004, -e * 0.2 * math.sin(a), -0.1 + e * 0.2 * math.cos(a)), bevel=0, dens=0, ao=0.2)


def build_flagpole(seed=1, height=2.2):
    """Wooden flagpole on a small cairn, topped with a gold ball and a skull-and-crossbones flag."""
    p = Prop(WORLD, "flagpole", seed)
    rng = p.rng
    pole = shapes.cylinder("pole", 0.058, 0.034, height, seg=10)
    p.add(pole, "matte", ("wood_dark", "wood_light"), loc=(0, 0, 0.1), bevel=0, dens=0.5, noise_amp=0.05, smooth_angle=65)
    ball = shapes.sphere("finial", 0.062, seg=12, rings=8)
    p.add(ball, "satin", ("gold_dark", "gold_light"), loc=(0, 0, height + 0.13), bevel=0, dens=0, ao=0.3)
    ring = shapes.torus("pole-ring", 0.044, 0.012, seg=12, rseg=5)
    p.add(ring, "matte", ("iron_dark", "iron_light"), loc=(0, 0, height * 0.55), bevel=0, dens=0, ao=0.4)
    # cairn of rounded stones round the foot
    for a, rd, size, z in ((10, 0.17, 0.2, 0.0), (95, 0.19, 0.17, 0.0), (170, 0.17, 0.21, 0.0), (250, 0.2, 0.16, 0.0),
                           (320, 0.17, 0.19, 0.0), (60, 0.09, 0.14, 0.13), (200, 0.08, 0.13, 0.15)):
        ar = math.radians(a)
        rock = shapes.blob("cairn", size, rng, subdiv=2, amp=0.18, freq=1.2, squash=(1.1, 1.0, 0.75), cuts=2, flat_bottom=0.5, relax=2)
        p.add(rock, "matte", "rock", loc=(math.cos(ar) * rd, math.sin(ar) * rd, z), rot=(0, 0, a * 3), bevel=0, dens=0, ao=0.65,
              noise_amp=0.07, smooth_angle=80)
    _flag(p, 0.03, height + 0.02, width=1.15, height=0.72)
    return p.build()
