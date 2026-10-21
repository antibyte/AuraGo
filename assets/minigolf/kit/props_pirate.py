"""Pirate bay props (MIT License).

Each builder returns one joined mesh object named ``prop-pirate-<name>`` with
its base at z=0 and its footprint centred on the origin. ``PROPS`` maps the
short name to its builder; the hero ship lives in ``ship_pirate``, the palms in
``props_pirate_palm``, lighthouse, post, buoy and flagpole in
``props_pirate_harbour`` and skull rock, seaweed, pebbles and the shell group
in ``props_pirate_shore`` (shared helpers: ``props_pirate_util``).
"""

import math

from . import shapes
from .prop import Prop
from .props_pirate_harbour import build_buoy, build_flagpole, build_lighthouse, build_post
from .props_pirate_palm import build_palm, build_palm2
from .props_pirate_shore import build_pebble, build_seaweed, build_shell, build_skullrock
from .ship_pirate import build_ship

WORLD = "pirate"


# --------------------------------------------------------------------------
# Barrel
# --------------------------------------------------------------------------

def build_barrel(seed=1, height=0.64):
    """Wooden barrel with iron bands, stave grooves and a recessed lid."""
    p = Prop(WORLD, "barrel", seed)
    h = height
    rb, rm = 0.245, 0.315
    body = shapes.barrel("barrel-body", rb, rm, h, seg=24)

    def radius(z):
        return rb + (rm - rb) * math.sin(math.pi * z / h) ** 0.85

    def staves(x, y, z):
        return 0.78 if shapes.stave_index(x, y) % 2 == 0 else 1.0

    p.add(body, "matte", "wood", bevel=0, dens=0.1, pattern=staves, noise_amp=0.05, smooth_angle=50)
    for z in (0.16, 0.48):
        r = radius(z) + 0.012
        band = shapes.lathe("barrel-band", [(r - 0.012, z - 0.034), (r + 0.008, z - 0.034),
                                            (r + 0.008, z + 0.034), (r - 0.012, z + 0.034)], seg=24, caps=False)
        p.add(band, "metal", "iron", bevel=0.008, segments=2, dens=0.12, ao=0.5)
    lid = shapes.cylinder("barrel-lid", rb - 0.018, rb - 0.03, 0.03, seg=24)
    p.add(lid, "matte", ("wood", "wood_light"), loc=(0, 0, h - 0.012), bevel=0.008, ao=0.7)
    bung = shapes.cylinder("barrel-bung", 0.032, 0.026, 0.02, seg=10)
    p.add(bung, "matte", "wood_dark", loc=(0.1, 0.05, h + 0.012), bevel=0, dens=0)
    return p.build()


# --------------------------------------------------------------------------
# Treasure chest
# --------------------------------------------------------------------------

def _arch(name, length, radius, seg=12, x0=0.0):
    """Half-cylinder along X (arch up, flat base at z=0), centred on y=0."""
    rings = []
    for x in (x0 - length / 2, x0 + length / 2):
        rings.append([(x, radius * math.cos(math.pi * k / seg), radius * math.sin(math.pi * k / seg)) for k in range(seg + 1)])
    return shapes.loft(name, rings, closed=True, recalc=True)


def build_chest(seed=2, open_deg=105.0):
    """Banded treasure chest with an open lid, a heap of gold and spilled coins."""
    p = Prop(WORLD, "chest", seed)
    rng = p.rng
    W, D, H = 0.8, 0.48, 0.3
    body = shapes.box("chest-body", (W, D, H), base=True)

    def planks(x, y, z):
        return 0.86 if int(math.floor(z / 0.075)) % 2 == 0 else 1.0

    p.add(body, "matte", ("wood_dark", "wood"), bevel=0.02, segments=2, dens=0.2, pattern=planks, noise_amp=0.05)
    for sx in (-0.26, 0.26):
        band = shapes.box("chest-band", (0.07, D + 0.03, H + 0.025), base=True)
        p.add(band, "metal", "iron", loc=(sx, 0, -0.005), bevel=0.008, segments=1, dens=0, ao=0.5)
    r = D / 2
    lid = _arch("chest-lid", W, r)
    shapes.finish(lid, bevel=0.016, segments=2, dens=0.2)
    shapes.transform(lid, loc=(0, r, 0))
    shapes.transform(lid, rot=(open_deg, 0, 0))
    p.add(lid, "matte", ("wood_dark", "wood_light"), loc=(0, -r, H), finish=False, ao=0.6, noise_amp=0.05)
    for sx in (-0.26, 0.26):
        strap = _arch("lid-strap", 0.07, r + 0.014, x0=sx)
        shapes.finish(strap, bevel=0.008, segments=1, dens=0)
        shapes.transform(strap, loc=(0, r, 0))
        shapes.transform(strap, rot=(open_deg, 0, 0))
        p.add(strap, "metal", "iron", loc=(0, -r, H), finish=False, ao=0.5)
    lock = shapes.box("chest-lock", (0.1, 0.03, 0.12), base=True)
    p.add(lock, "satin", "gold", loc=(0, r + 0.01, H - 0.14), bevel=0.01, segments=1, dens=0, ao=0.4)
    key = shapes.cylinder("keyhole", 0.016, 0.016, 0.02, seg=8, base=False)
    p.add(key, "matte", ("black", "black"), loc=(0, r + 0.03, H - 0.1), rot=(90, 0, 0), bevel=0, dens=0, ao=0.0)
    coins = []

    def coin(x, y, z, rx, ry, rz):
        c = shapes.cylinder("coin", 0.052, 0.052, 0.014, seg=9, base=False)
        shapes.transform(c, loc=(x, y, z), rot=(rx, ry, rz))
        coins.append(c)

    for i in range(40):
        x, y = rng.uniform(-0.31, 0.31), rng.uniform(-0.15, 0.15)
        dome = 1.0 - (x / 0.34) ** 2 - (y / 0.19) ** 2
        if dome > 0:
            coin(x, y, H - 0.06 + 0.17 * dome ** 0.8 + rng.uniform(0.0, 0.02),
                 rng.uniform(-35, 35), rng.uniform(-35, 35), rng.uniform(0, 360))
    for i in range(15):
        a, d = rng.uniform(-0.9, 0.9), rng.uniform(0.34, 0.95)
        coin(math.sin(a) * d, r + 0.08 + math.cos(a) * d * 0.75, 0.008 + rng.uniform(0, 0.004),
             rng.uniform(-6, 6), rng.uniform(-6, 6), rng.uniform(0, 360))
    for k in range(3):
        for j in range(3 + k):
            coin(-0.55 + k * 0.1, r + 0.4 + k * 0.08, 0.008 + j * 0.0152, 0, 0, rng.uniform(0, 360))
    gold = shapes.join("coins", coins)
    p.add(gold, "satin", ("gold_dark", "gold_light"), bevel=0, dens=0, ao=0.45, noise_amp=0.08, jitter=0.0, smooth_angle=30)
    for i, (gx, gy, tone) in enumerate(((0.1, 0.02, "red"), (-0.14, -0.05, "glass"), (0.2, 0.09, "glass"))):
        gem = shapes.ico_sphere("gem", 0.036, 1)
        p.add(gem, "satin", (tone, tone), loc=(gx, gy, H + 0.12 - i * 0.015), bevel=0, dens=0, ao=0.2)
    return p.build()


# --------------------------------------------------------------------------
# Cannon
# --------------------------------------------------------------------------

def build_cannon(seed=5):
    """Iron cannon on a wooden carriage, muzzle towards +Y, with a few balls."""
    p = Prop(WORLD, "cannon", seed)
    prof = [(0.0, -0.1), (0.035, -0.085), (0.05, -0.05), (0.04, -0.015), (0.1, 0.03), (0.15, 0.1), (0.155, 0.2),
            (0.14, 0.5), (0.118, 0.85), (0.105, 1.02), (0.135, 1.04), (0.14, 1.11), (0.095, 1.13), (0.075, 1.13),
            (0.075, 0.9), (0.0, 0.9)]
    barrel = shapes.lathe("cannon-barrel", prof, seg=20)
    p.add(barrel, "metal", "iron", loc=(0, -0.55, 0.44), rot=(-82, 0, 0), bevel=0, dens=0.25, smooth_angle=60, ao=0.5)
    for s in (-1, 1):
        cheek = shapes.box("cheek", (0.09, 1.1, 0.3), base=True)
        p.add(cheek, "matte", ("wood_dark", "wood"), loc=(s * 0.19, -0.05, 0.12), bevel=0.02, segments=2, dens=0.3, noise_amp=0.05)
        wheel = shapes.cylinder("wheel", 0.21, 0.21, 0.09, seg=14, base=False)
        p.add(wheel, "matte", ("wood_dark", "wood"), loc=(s * 0.3, -0.22, 0.21), rot=(0, 90, 0), bevel=0.012, segments=1, dens=0.5)
        rim = shapes.torus("wheel-rim", 0.205, 0.018, seg=16, rseg=5)
        p.add(rim, "metal", "iron", loc=(s * 0.3, -0.22, 0.21), rot=(0, 90, 0), bevel=0, dens=0, ao=0.5)
        hub = shapes.cylinder("hub", 0.05, 0.05, 0.14, seg=10, base=False)
        p.add(hub, "metal", "iron", loc=(s * 0.3, -0.22, 0.21), rot=(0, 90, 0), bevel=0.006, segments=1, dens=0, ao=0.4)
        wheel2 = shapes.cylinder("wheel", 0.15, 0.15, 0.08, seg=12, base=False)
        p.add(wheel2, "matte", ("wood_dark", "wood"), loc=(s * 0.27, 0.42, 0.15), rot=(0, 90, 0), bevel=0.01, segments=1, dens=0.5)
    axle = shapes.cylinder("axle", 0.035, 0.035, 0.66, seg=8, base=False)
    p.add(axle, "matte", "wood_dark", loc=(0, -0.22, 0.21), rot=(0, 90, 0), bevel=0, dens=0.4)
    cross = shapes.box("cross", (0.38, 0.1, 0.1), base=True)
    p.add(cross, "matte", ("wood_dark", "wood"), loc=(0, 0.4, 0.1), bevel=0.015, segments=1, dens=0.3)
    for bx, by, bz in ((0.72, -0.2, 0.09), (0.9, -0.05, 0.09), (0.74, 0.0, 0.09), (0.8, -0.1, 0.25)):
        ball = shapes.sphere("ball", 0.09, seg=10, rings=6)
        p.add(ball, "metal", ("iron_dark", "iron_light"), loc=(bx, by, bz), bevel=0, dens=0, ao=0.5)
    return p.build()


# --------------------------------------------------------------------------
# Dock
# --------------------------------------------------------------------------

def build_dock(seed=4, length=3.4, width=1.15, deck_h=1.6):
    """Short wooden dock along +Y. Posts start at z=0; the deck sits at ``deck_h``."""
    p = Prop(WORLD, "dock", seed)
    rng = p.rng
    n = int(length / 0.19)
    for i in range(n):
        ln = width + rng.uniform(-0.02, 0.06)
        pl = shapes.plank("plank", 0.172, ln, 0.07, cuts=1)
        p.add(pl, "matte", ("wood", "wood_light"), loc=(rng.uniform(-0.01, 0.01), 0.1 + i * 0.19, deck_h + rng.uniform(-0.004, 0.006)),
              rot=(0, 0, rng.uniform(-1.2, 1.2)), bevel=0.012, segments=1, dens=0, jitter=0.04, noise_amp=0.04)
    for sx in (-0.38, 0.38):
        beam = shapes.box("beam", (0.12, length, 0.11), base=True)
        p.add(beam, "matte", ("wood_dark", "wood"), loc=(sx, length / 2, deck_h - 0.1), bevel=0.015, segments=1, dens=0.6)
    ys = (0.25, 1.3, 2.35, 3.2)
    tops = [deck_h + (0.42 if k % 2 == 0 else 0.3) for k in range(4)]
    for k, y in enumerate(ys):
        for s in (-1, 1):
            x = s * (width / 2 + 0.05)
            post = shapes.cylinder("post", 0.085, 0.075, tops[k], seg=9)
            p.add(post, "matte", ("wood_dark", "wood"), loc=(x, y, 0.0), bevel=0, dens=0.7, noise_amp=0.05)
            cap = shapes.sphere("post-cap", 0.08, seg=10, rings=4)
            p.add(cap, "matte", "wood", loc=(x, y, tops[k]), scale=(1, 1, 0.7), bevel=0, dens=0)
            band = shapes.lathe("rope-band", [(0.074, -0.018), (0.098, -0.01), (0.098, 0.01), (0.074, 0.018)], seg=10, caps=False)
            p.add(band, "matte", "rope", loc=(x, y, tops[k] - 0.1), bevel=0, dens=0, ao=0.45)
    for s in (-1, 1):
        x = s * (width / 2 + 0.05)
        for a, b in ((0, 1), (2, 3)):
            pts = []
            for i in range(11):
                t = i / 10
                pts.append((x, ys[a] + (ys[b] - ys[a]) * t,
                            tops[a] - 0.1 + (tops[b] - tops[a]) * t - 0.1 * 4 * t * (1 - t) - 0.1))
            rope = shapes.tube("dock-rope", pts, radius=0.017, seg=6)
            p.add(rope, "matte", "rope", bevel=0, dens=0, ao=0.45)
    return p.build()


# --------------------------------------------------------------------------
# Small things: grass tuft, shell, starfish, rock, crab, X mark
# --------------------------------------------------------------------------

def build_tuft(seed=1, blades=16, height=0.45):
    p = Prop(WORLD, "tuft", seed)
    rng = p.rng
    for i in range(blades):
        a = rng.uniform(0, math.tau)
        rho = rng.uniform(0.0, 0.05)
        h = height * rng.uniform(0.55, 1.0)
        out = rng.uniform(0.04, 0.2)
        spine = []
        for k in range(5):
            t = k / 4
            r = rho + out * t * t * 1.4
            spine.append((math.cos(a) * r, math.sin(a) * r, h * t * (1.0 - 0.18 * t)))
        widths = [0.052 * (1 - 0.9 * (k / 4) ** 1.2) for k in range(5)]
        blade = shapes.ribbon("blade", spine, widths, normal_hint=(math.cos(a), math.sin(a), 0.0), fold=0.35, across=2)
        p.add(blade, "foliage", "grass", sway=0.06, finish=False, ao=0.6, jitter=0.05, mode="z")
    return p.build()


def build_starfish(seed=1):
    p = Prop(WORLD, "starfish", seed)
    outline = []
    for k in range(10):
        a = math.tau * k / 10 + math.pi / 2
        r = 0.15 if k % 2 == 0 else 0.062
        outline.append((r * math.cos(a), r * math.sin(a)))
    verts = [(x, y, 0.0) for x, y in outline] + [(x, y, 0.035) for x, y in outline] + [(0.0, 0.0, 0.05)]
    faces = []
    for k in range(10):
        k2 = (k + 1) % 10
        faces.append((k, k2, 10 + k2, 10 + k))
        faces.append((10 + k, 10 + k2, 20))
    faces.append(tuple(reversed(range(10))))
    star = shapes.make_object("star", verts, faces)
    p.add(star, "matte", ("red_dark", "gold"), bevel=0, subsurf=1, dens=0, ao=0.5, mode="radial")
    return p.build()


def build_rock(seed=1, size=0.5, cuts=3, subdiv=3, squash=(1.0, 0.85, 0.7)):
    p = Prop(WORLD, "rock", seed)
    rock = shapes.blob("rock", size, p.rng, subdiv=subdiv, amp=0.2, freq=1.2, squash=squash, cuts=cuts, flat_bottom=0.55, relax=2)
    p.add(rock, "matte", "rock", bevel=0, dens=0, ao=0.7, noise_amp=0.08, smooth_angle=80, jitter=0.03)
    return p.build()


def build_crab(seed=1):
    p = Prop(WORLD, "crab", seed)
    red = ("red_dark", "red")
    body = shapes.sphere("crab-body", 0.1, seg=14, rings=8)
    p.add(body, "satin", red, loc=(0, 0, 0.05), scale=(1.2, 0.9, 0.55), bevel=0, dens=0, ao=0.5)
    for s in (-1, 1):
        stalk = shapes.cylinder("stalk", 0.008, 0.008, 0.045, seg=5)
        p.add(stalk, "satin", red, loc=(s * 0.04, 0.085, 0.07), bevel=0, dens=0, ao=0.4)
        eye = shapes.sphere("eye", 0.016, seg=8, rings=5)
        p.add(eye, "satin", ("black", "black"), loc=(s * 0.04, 0.085, 0.12), bevel=0, dens=0, ao=0.2)
        claw = shapes.sphere("claw", 0.04, seg=10, rings=6)
        p.add(claw, "satin", red, loc=(s * 0.17, 0.16, 0.05), scale=(1.1, 1.3, 0.8), bevel=0, dens=0, ao=0.4)
        arm = shapes.tube("arm", [(s * 0.1, 0.06, 0.05), (s * 0.15, 0.12, 0.05)], radius=0.012, seg=6)
        p.add(arm, "satin", red, bevel=0, dens=0, ao=0.4)
        for k in range(3):
            y = -0.05 + k * 0.045
            leg = shapes.tube("leg", [(s * 0.1, y, 0.05), (s * 0.19, y - 0.02, 0.07), (s * 0.24, y - 0.04, 0.0)], radius=0.009, seg=5)
            p.add(leg, "satin", red, bevel=0, dens=0, ao=0.4)
    return p.build()


def build_xmark(seed=1):
    p = Prop(WORLD, "xmark", seed)
    for ang in (38, -38):
        bar = shapes.box("x-bar", (0.7, 0.085, 0.014), base=True)
        p.add(bar, "matte", ("red_dark", "red"), rot=(0, 0, ang), bevel=0.005, segments=1, dens=0.4, ao=0.5)
    return p.build()


def build_footprint(seed=1, left=True):
    """Pressed-in bare footprint: sole and ball ellipsoids in dark wet sand."""
    p = Prop(WORLD, "footprint", seed)
    sole = shapes.sphere("sole", 0.5, seg=10, rings=6)
    p.add(sole, "matte", ("sand_dark", "sand_wet"), loc=(0, 0, 0), scale=(0.085, 0.15, 0.02), bevel=0, dens=0, ao=0.2)
    toe = shapes.sphere("toes", 0.5, seg=8, rings=5)
    p.add(toe, "matte", ("sand_dark", "sand_wet"), loc=(0.01 if left else -0.01, 0.17, 0), scale=(0.07, 0.06, 0.02), bevel=0, dens=0, ao=0.2)
    return p.build()


def build_foam_ring(seed=1, radius=0.8):
    """Flat white foam ring for rocks standing in the water."""
    p = Prop(WORLD, "foam", seed)
    ring = shapes.lathe("foam-ring", [(radius * 0.92, 0.0), (radius * 1.13, 0.0), (radius * 1.16, 0.008), (radius * 0.92, 0.008)],
                        seg=40, caps=False)
    for v in ring.data.vertices:
        a = math.atan2(v.co.y, v.co.x)
        k = 1.0 + 0.05 * math.sin(a * 5 + 1.0) + 0.025 * math.sin(a * 11)
        v.co.x *= k
        v.co.y *= k
    p.add(ring, "satin", ("lagoon_shore", "foam"), bevel=0, dens=0, ao=0.0, smooth_angle=80)
    return p.build()


# The foam ring (``build_foam_ring``) is not a kit prop: the runtime draws the shore foam.
PROPS = {"ship": build_ship, "palm": build_palm, "palm2": build_palm2, "barrel": build_barrel, "chest": build_chest,
         "cannon": build_cannon, "dock": build_dock, "tuft": build_tuft, "shell": build_shell,
         "starfish": build_starfish, "rock": build_rock, "crab": build_crab, "xmark": build_xmark,
         "footprint": build_footprint, "lighthouse": build_lighthouse, "post": build_post, "buoy": build_buoy,
         "flagpole": build_flagpole, "skullrock": build_skullrock, "seaweed": build_seaweed, "pebble": build_pebble}

# Placement tags per prop (hero, tall, scatter, flat, span) plus optional ``clearance`` / ``spin``.
PROP_META = {
    "ship": {"tags": ["hero"]}, "palm": {"tags": ["tall"]}, "palm2": {"tags": ["tall"]}, "barrel": {"tags": []}, "chest": {"tags": []},
    "cannon": {"tags": []}, "dock": {"tags": ["flat"]}, "tuft": {"tags": ["scatter"]}, "shell": {"tags": ["scatter", "flat"]},
    "starfish": {"tags": ["scatter", "flat"]}, "rock": {"tags": []}, "crab": {"tags": ["scatter"]},
    "xmark": {"tags": ["scatter", "flat"]}, "footprint": {"tags": ["scatter", "flat"]},
    "lighthouse": {"tags": ["tall"]}, "post": {"tags": []}, "buoy": {"tags": []}, "flagpole": {"tags": ["tall"]},
    "skullrock": {"tags": []}, "seaweed": {"tags": ["scatter"]}, "pebble": {"tags": ["scatter"]},
}
