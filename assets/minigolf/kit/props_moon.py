"""Moon station props (MIT License).

Each builder returns one joined mesh object named ``prop-moon-<name>`` with its
base at z=0 and its footprint centred on the origin. ``PROPS`` maps the short
name to its builder.
"""

import math

from mathutils import Vector

from . import shapes
from .prop import Prop

WORLD = "moon"


def _dome_profile(radius, half_len, dome, steps=8):
    """Revolve profile (axis = Z) of a capsule-like body with elliptical domes."""
    prof = [(0.0, -half_len - dome)]
    for k in range(1, steps + 1):
        a = math.pi / 2 * k / steps
        prof.append((radius * math.sin(a), -half_len - dome * math.cos(a)))
    for k in range(1, steps):
        a = math.pi / 2 * k / steps
        prof.append((radius * math.cos(a), half_len + dome * math.sin(a)))
    prof.append((0.0, half_len + dome))
    return prof


# --------------------------------------------------------------------------
# Habitat module (hero)
# --------------------------------------------------------------------------

def build_habitat(seed=2):
    """Pressurised habitat: domed cylinder on legs with window row, hatch, mast and beacon.

    The long axis is X; the windows and hatch face -Y.
    """
    p = Prop(WORLD, "habitat", seed)
    R, half, dome = 1.2, 1.7, 0.78
    zc = 1.62
    body = shapes.lathe("hab-body", _dome_profile(R, half, dome), seg=40, caps=False)
    p.add(body, "satin", "panel", loc=(0, 0, zc), rot=(0, 90, 0), bevel=0, dens=0.28, smooth_angle=70, mode="z", ao=0.6, noise_amp=0.02)
    # raised panel seams and orange accent bands
    for k, x in enumerate([-1.7 + i * 0.85 for i in range(5)]):
        band = shapes.lathe("hab-seam", [(R + 0.005, -0.04), (R + 0.045, -0.025), (R + 0.045, 0.025), (R + 0.005, 0.04)], seg=40,
                            caps=False)
        p.add(band, "satin", ("panel_grey", "panel_white"), loc=(x, 0, zc), rot=(0, 90, 0), bevel=0, dens=0, ao=0.5)
    for x in (-2.35, 2.35):
        r = R * math.sqrt(max(0.0, 1 - ((abs(x) - half) / dome) ** 2)) + 0.03
        band = shapes.lathe("hab-accent", [(r, -0.14), (r + 0.045, -0.11), (r + 0.045, 0.11), (r, 0.14)], seg=40, caps=False)
        p.add(band, "satin", "orange", loc=(x, 0, zc), rot=(0, 90, 0), bevel=0, dens=0, ao=0.5)
    # window row on both long sides
    for side in (-1, 1):
        for k in range(5):
            x = -1.45 + k * 0.72
            ang = math.radians(22)
            y = side * R * math.cos(ang)
            z = zc + R * math.sin(ang)
            ring = shapes.torus("win-ring", 0.25, 0.055, seg=18, rseg=8)
            tilt = -side * 22
            p.add(ring, "satin", ("metal_dark", "metal"), loc=(x, y, z), rot=(90 + tilt, 0, 0), bevel=0, dens=0, ao=0.5)
            glass = shapes.cylinder("win-glass", 0.23, 0.23, 0.05, seg=18, base=False)
            p.add(glass, "emissive", ("window", "cyan"), loc=(x, y - side * 0.0, z), rot=(tilt, 0, 0), bevel=0, dens=0, ao=0.0, strength=1.0,
                  mode="x")
    # hatch (front, -Y)
    ang = math.radians(-6)
    hy = -R * math.cos(ang) + 0.02
    door = shapes.box("hatch", (0.95, 0.16, 1.5), base=False)
    p.add(door, "satin", ("orange_dark", "orange"), loc=(0.0, hy - 0.03, zc - 0.28 + 0.0), bevel=0.04, segments=3, dens=0.3, ao=0.6)
    dwin = shapes.cylinder("hatch-win", 0.2, 0.2, 0.06, seg=16, base=False)
    p.add(dwin, "emissive", ("window", "cyan"), loc=(0.0, hy - 0.12, zc + 0.12), rot=(90, 0, 0), bevel=0, dens=0, ao=0.0, strength=1.0)
    wheel = shapes.torus("hatch-wheel", 0.12, 0.022, seg=14, rseg=6)
    p.add(wheel, "metal", ("metal_dark", "metal"), loc=(0.0, hy - 0.14, zc - 0.5), rot=(90, 0, 0), bevel=0, dens=0, ao=0.4)
    step = shapes.box("hatch-step", (1.3, 0.8, 0.14), base=True)
    p.add(step, "satin", ("panel_grey", "panel_white"), loc=(0.0, hy - 0.55, 0.0), bevel=0.03, segments=2, dens=0.3)
    ramp = shapes.box("hatch-ramp", (1.0, 0.7, 0.07), base=True)
    p.add(ramp, "satin", ("orange_dark", "orange"), loc=(0.0, hy - 1.15, 0.0), rot=(5, 0, 0), bevel=0.02, segments=2, dens=0.3)
    # legs
    for sx in (-1.25, 1.25):
        for sy in (-0.95, 0.95):
            leg = shapes.cylinder("leg", 0.12, 0.17, zc - 0.55, seg=10)
            p.add(leg, "metal", ("metal_dark", "metal"), loc=(sx, sy, 0.15), bevel=0, dens=0.5, ao=0.5)
            foot = shapes.cylinder("foot", 0.42, 0.34, 0.16, seg=14)
            p.add(foot, "satin", ("panel_dark", "metal"), loc=(sx, sy * 1.12, 0.0), bevel=0.03, segments=2, dens=0.3)
    # roof: vents, mast, beacon
    for k in range(3):
        vent = shapes.box("vent", (0.5, 0.34, 0.18), base=True)
        p.add(vent, "satin", ("panel_grey", "panel_white"), loc=(-1.0 + k * 0.62, 0.3 * (k - 1), zc + R - 0.06), bevel=0.03, segments=2,
              dens=0.3)
    mast = shapes.cylinder("mast", 0.045, 0.03, 1.7, seg=8)
    p.add(mast, "metal", ("metal_dark", "metal"), loc=(1.2, 0.0, zc + R - 0.05), bevel=0, dens=0.6, ao=0.4)
    for k, zz in enumerate((1.15, 1.5)):
        arm = shapes.cylinder("arm", 0.018, 0.018, 0.7 - 0.2 * k, seg=6, base=False)
        p.add(arm, "metal", ("metal_dark", "metal"), loc=(1.2, 0.0, zc + R + zz - 0.4), rot=(0, 90, 0), bevel=0, dens=0.5, ao=0.4)
    bulb = shapes.sphere("beacon", 0.075, seg=12, rings=8)
    p.add(bulb, "emissive", ("magenta", "magenta"), loc=(1.2, 0.0, zc + R + 1.7), bevel=0, dens=0, ao=0.0, strength=2.5)
    cup = shapes.sphere("mini-dish", 0.24, seg=14, rings=6)
    p.add(cup, "satin", ("panel_grey", "panel_white"), loc=(1.2, 0.2, zc + R + 1.25), scale=(1.0, 0.4, 1.0), rot=(35, 0, 0), bevel=0, dens=0,
          ao=0.4)
    # cable between the legs
    cable = shapes.tube("cable", [(1.25, 0.95, 0.3), (1.9, 1.2, 0.05), (2.8, 1.45, 0.05), (3.4, 1.7, 0.1)], radius=0.035, seg=6)
    p.add(cable, "matte", ("black", "panel_dark"), bevel=0, dens=0, ao=0.5)
    return p.build()


# --------------------------------------------------------------------------
# Dish on a tripod
# --------------------------------------------------------------------------

def build_dish(seed=3):
    """Parabolic dish with feed horn on a tripod, tilted up and towards -Y."""
    p = Prop(WORLD, "dish", seed)
    R, depth, t = 1.15, 0.62, 0.05
    prof = [(0.0, -t)]
    n = 10
    for k in range(1, n + 1):
        r = R * k / n
        prof.append((r, depth * (r / R) ** 2 - t))
    prof.append((R, depth))
    for k in range(n - 1, -1, -1):
        r = R * k / n
        prof.append((r, depth * (r / R) ** 2))
    dish = shapes.lathe("dish-shell", prof, seg=36, caps=False)
    hub = Vector((0.0, 0.0, 1.55))
    tilt = (52, 0, 0)
    p.add(dish, "matte", ("panel_grey", "panel_white"), loc=hub, rot=tilt, bevel=0, dens=0.3, smooth_angle=70, mode="radial",
          center=(0, 0), ao=0.55)
    rot = shapes.matrix(rot=tilt)
    rim = shapes.torus("dish-rim", R, 0.04, seg=36, rseg=6)
    shapes.transform(rim, loc=(0, 0, depth))
    p.add(rim, "satin", "orange", loc=hub, rot=tilt, bevel=0, dens=0, ao=0.4)
    focus = hub + rot @ Vector((0, 0, R * R / (4 * depth)))
    for k in range(3):
        a = math.radians(30 + k * 120)
        rimpt = hub + rot @ Vector((R * math.cos(a), R * math.sin(a), depth))
        strut = shapes.tube("feed-strut", [rimpt, focus], radius=0.022, seg=6)
        p.add(strut, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.4)
    horn = shapes.cone("feed-horn", 0.1, 0.2, seg=12)
    p.add(horn, "satin", "orange", loc=focus, rot=(52 + 180, 0, 0), bevel=0, dens=0, ao=0.4)
    # tripod
    post = shapes.cylinder("post", 0.1, 0.08, 1.2, seg=10)
    p.add(post, "metal", ("metal_dark", "metal"), loc=(0, 0.0, 0.3), bevel=0, dens=0.5, ao=0.5)
    mount = shapes.box("mount", (0.42, 0.34, 0.3), base=False)
    p.add(mount, "satin", ("panel_dark", "metal"), loc=(0, 0.0, 1.5), bevel=0.05, segments=2, dens=0.3)
    for k in range(3):
        a = math.radians(90 + k * 120)
        leg = shapes.tube("tripod-leg", [(0.0, 0.0, 1.35), (math.cos(a) * 0.85, math.sin(a) * 0.85, 0.06)], radius=[0.055, 0.04], seg=8)
        p.add(leg, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.5)
        foot = shapes.cylinder("pad", 0.17, 0.13, 0.08, seg=10)
        p.add(foot, "satin", ("panel_dark", "panel_grey"), loc=(math.cos(a) * 0.9, math.sin(a) * 0.9, 0.0), bevel=0.015, segments=1, dens=0)
    box = shapes.box("control-box", (0.5, 0.36, 0.34), base=True)
    p.add(box, "satin", ("panel_grey", "panel_white"), loc=(0.55, -0.55, 0.0), rot=(0, 0, -20), bevel=0.04, segments=2, dens=0.3)
    led = shapes.sphere("led", 0.035, seg=8, rings=5)
    p.add(led, "emissive", ("cyan", "cyan"), loc=(0.52, -0.78, 0.26), bevel=0, dens=0, ao=0.0, strength=2.6)
    return p.build()


# --------------------------------------------------------------------------
# Six-wheeled rover
# --------------------------------------------------------------------------

def build_rover(seed=4):
    p = Prop(WORLD, "rover", seed)
    body = shapes.box("rover-body", (1.35, 0.85, 0.36), base=True)
    p.add(body, "satin", ("panel_grey", "panel_white"), loc=(0, 0, 0.5), bevel=0.07, segments=3, dens=0.3, ao=0.6)
    stripe = shapes.box("rover-stripe", (1.38, 0.2, 0.12), base=True)
    p.add(stripe, "satin", "orange", loc=(0, 0, 0.6), bevel=0.02, segments=2, dens=0.3)
    deck = shapes.box("rover-panel", (1.1, 0.7, 0.045), base=True)
    p.add(deck, "satin", "solar", loc=(-0.1, 0, 0.88), rot=(0, -5, 0), bevel=0.012, segments=1, dens=0.25, ao=0.5)
    for sy in (-1, 1):
        wy = sy * 0.62
        for k, wx in enumerate((-0.62, 0.0, 0.62)):
            wh = shapes.cylinder("wheel", 0.27, 0.27, 0.2, seg=16, base=False)
            p.add(wh, "matte", ("black", "rubber") if p.pal.has("rubber") else ("black", "panel_dark"), loc=(wx, wy, 0.27),
                  rot=(90, 0, 0), bevel=0.04, segments=2, dens=0.3)
            hub = shapes.cylinder("hub", 0.12, 0.12, 0.23, seg=10, base=False)
            p.add(hub, "satin", ("orange_dark", "orange"), loc=(wx, wy + sy * 0.0, 0.27), rot=(90, 0, 0), bevel=0.01, segments=1, dens=0)
            arm = shapes.tube("bogie", [(wx * 0.6, sy * 0.4, 0.62), (wx, wy - sy * 0.08, 0.27)], radius=0.035, seg=6)
            p.add(arm, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.4)
    mast = shapes.cylinder("rover-mast", 0.03, 0.025, 0.7, seg=8)
    p.add(mast, "metal", ("metal_dark", "metal"), loc=(0.5, 0.0, 0.84), bevel=0, dens=0, ao=0.4)
    head = shapes.box("cam-head", (0.2, 0.3, 0.14), base=False)
    p.add(head, "satin", ("panel_dark", "panel_white"), loc=(0.52, 0.0, 1.58), bevel=0.03, segments=2, dens=0)
    for sy in (-1, 1):
        lens = shapes.cylinder("lens", 0.045, 0.045, 0.05, seg=10, base=False)
        p.add(lens, "emissive", ("cyan", "window"), loc=(0.64, sy * 0.08, 1.58), rot=(0, 90, 0), bevel=0, dens=0, ao=0.0, strength=2.0)
        lamp = shapes.cylinder("lamp", 0.05, 0.05, 0.04, seg=10, base=False)
        p.add(lamp, "emissive", ("window", "window"), loc=(0.69, sy * 0.28, 0.66), rot=(0, 90, 0), bevel=0, dens=0, ao=0.0, strength=1.5)
    whip = shapes.tube("whip", [(-0.5, 0.3, 0.86), (-0.55, 0.32, 1.4), (-0.5, 0.3, 1.8)], radius=0.012, seg=5)
    p.add(whip, "metal", ("metal_dark", "metal"), bevel=0, dens=0, ao=0.4)
    tip = shapes.sphere("whip-tip", 0.03, seg=8, rings=5)
    p.add(tip, "emissive", ("magenta", "magenta"), loc=(-0.5, 0.3, 1.8), bevel=0, dens=0, ao=0.0, strength=2.5)
    return p.build()


# --------------------------------------------------------------------------
# Solar array
# --------------------------------------------------------------------------

def _cell_layer(w, h, nx, ny, gap=0.018, raise_=0.014):
    verts, faces = [], []
    cw, ch = (w - gap * (nx + 1)) / nx, (h - gap * (ny + 1)) / ny
    for j in range(ny):
        for i in range(nx):
            x0 = -w / 2 + gap + i * (cw + gap)
            y0 = -h / 2 + gap + j * (ch + gap)
            b = len(verts)
            for (x, y, z) in ((x0, y0, 0.0), (x0 + cw, y0, 0.0), (x0 + cw, y0 + ch, 0.0), (x0, y0 + ch, 0.0)):
                verts.append((x, y, z))
            for (x, y, z) in ((x0 + 0.012, y0 + 0.012, raise_), (x0 + cw - 0.012, y0 + 0.012, raise_),
                              (x0 + cw - 0.012, y0 + ch - 0.012, raise_), (x0 + 0.012, y0 + ch - 0.012, raise_)):
                verts.append((x, y, z))
            faces.extend([(b + 4, b + 5, b + 6, b + 7), (b, b + 1, b + 5, b + 4), (b + 1, b + 2, b + 6, b + 5),
                          (b + 2, b + 3, b + 7, b + 6), (b + 3, b, b + 4, b + 7)])
    return shapes.make_object("cells", verts, faces)


def build_solar(seed=5, panels=3):
    """Row of tilted solar panels with gridded cells on a shared support rail."""
    p = Prop(WORLD, "solar", seed)
    pw, ph = 1.45, 0.95
    gap = 0.12
    total = panels * pw + (panels - 1) * gap
    tilt = 28
    zc = 0.82
    for k in range(panels):
        x = -total / 2 + pw / 2 + k * (pw + gap)
        frame = shapes.box("panel-frame", (pw, ph, 0.05), base=False)
        p.add(frame, "satin", ("panel_dark", "panel_grey"), loc=(x, 0, zc), rot=(tilt, 0, 0), bevel=0.015, segments=2, dens=0.3)
        cells = _cell_layer(pw - 0.1, ph - 0.1, 6, 4)
        p.add(cells, "satin", "solar", loc=(x, 0, zc + 0.026), rot=(tilt, 0, 0), bevel=0, dens=0, ao=0.45, mode="x", noise_amp=0.05,
              smooth_angle=30)
        for sx in (-0.5, 0.5):
            for sy, top in ((-0.38, zc - 0.2), (0.4, zc + 0.2)):
                leg = shapes.cylinder("leg", 0.035, 0.03, top, seg=8)
                p.add(leg, "metal", ("metal_dark", "metal"), loc=(x + sx * 0.8, sy, 0.0), bevel=0, dens=0.5, ao=0.4)
    box = shapes.box("battery", (0.42, 0.36, 0.34), base=True)
    p.add(box, "satin", ("panel_grey", "orange"), loc=(total / 2 + 0.45, -0.1, 0.0), bevel=0.04, segments=2, dens=0.3)
    led = shapes.sphere("led", 0.03, seg=8, rings=5)
    p.add(led, "emissive", ("cyan", "cyan"), loc=(total / 2 + 0.45, -0.3, 0.3), bevel=0, dens=0, ao=0.0, strength=2.6)
    return p.build()


# --------------------------------------------------------------------------
# Rocks, beacon, crates, pad
# --------------------------------------------------------------------------

def build_boulder(seed=1, size=0.6, cuts=4, subdiv=3, squash=(1.0, 0.85, 0.72)):
    p = Prop(WORLD, "boulder", seed)
    rock = shapes.blob("boulder", size, p.rng, subdiv=subdiv, amp=0.2, freq=1.1, squash=squash, cuts=cuts, cut_depth=0.8, flat_bottom=0.5,
                       relax=2)
    p.add(rock, "matte", "rock", bevel=0, dens=0, ao=0.7, noise_amp=0.08, smooth_angle=80, jitter=0.03)
    return p.build()


def build_beacon(seed=1, tone="cyan"):
    """Short pylon with a glowing neon cap."""
    p = Prop(WORLD, "beacon", seed)
    base = shapes.cylinder("beacon-base", 0.2, 0.16, 0.14, seg=10)
    p.add(base, "satin", ("panel_dark", "panel_grey"), bevel=0.02, segments=2, dens=0.3)
    stem = shapes.box("beacon-stem", (0.12, 0.12, 0.62), base=True)
    p.add(stem, "satin", ("panel_grey", "panel_white"), loc=(0, 0, 0.12), bevel=0.02, segments=2, dens=0.3)
    cap = shapes.sphere("beacon-cap", 0.1, seg=12, rings=8)
    p.add(cap, "emissive", (tone, tone), loc=(0, 0, 0.82), bevel=0, dens=0, ao=0.0, strength=2.6)
    return p.build()


def build_crate(seed=1):
    p = Prop(WORLD, "crate", seed)
    for k, (x, y, s, rot) in enumerate(((0.0, 0.0, 0.62, 8.0), (0.7, 0.18, 0.46, -12.0), (0.1, 0.1, 0.42, 30.0))):
        box = shapes.box("crate", (s, s, s * 0.8), base=True)
        z = 0.0 if k < 2 else 0.5
        p.add(box, "satin", ("panel_grey", "panel_white"), loc=(x, y, z), rot=(0, 0, rot), bevel=0.035, segments=2, dens=0.3, jitter=0.03)
        band = shapes.box("crate-band", (s + 0.02, s + 0.02, 0.1), base=True)
        p.add(band, "satin", "orange", loc=(x, y, z + s * 0.28), rot=(0, 0, rot), bevel=0.012, segments=1, dens=0.4)
    return p.build()


def build_pad(seed=1, size=2.4):
    """Square landing pad with orange corner marks."""
    p = Prop(WORLD, "pad", seed)
    plate = shapes.box("pad-plate", (size, size, 0.09), base=True)
    p.add(plate, "satin", ("panel_dark", "panel_grey"), bevel=0.04, segments=2, dens=0.3, noise_amp=0.04)
    for sx in (-1, 1):
        for sy in (-1, 1):
            m = shapes.box("pad-mark", (0.5, 0.12, 0.012), base=True)
            p.add(m, "satin", "orange", loc=(sx * (size / 2 - 0.35), sy * (size / 2 - 0.15), 0.09), bevel=0.004, segments=1, dens=0.4)
            m2 = shapes.box("pad-mark", (0.12, 0.5, 0.012), base=True)
            p.add(m2, "satin", "orange", loc=(sx * (size / 2 - 0.15), sy * (size / 2 - 0.35), 0.09), bevel=0.004, segments=1, dens=0.4)
    return p.build()


def build_crystal(seed=1, tone="cyan", count=5):
    """Cluster of glowing neon crystals growing out of the regolith."""
    p = Prop(WORLD, "crystal", seed)
    rng = p.rng
    for k in range(count):
        a = rng.uniform(0, math.tau)
        d = rng.uniform(0.0, 0.3) if k else 0.0
        h = rng.uniform(0.45, 0.95) * (1.25 if k == 0 else 1.0)
        w = rng.uniform(0.1, 0.17)
        prof = [(0.0, 0.0), (w, 0.0), (w * 1.05, h * 0.62), (w * 0.4, h * 0.92), (0.0, h)]
        c = shapes.lathe("crystal", prof, seg=6)
        t = tone if (k % 3) != 2 else ("magenta" if tone == "cyan" else "cyan")
        lo = (t + "_dark") if p.pal.has(t + "_dark") else t
        p.add(c, "glow", (lo, t), loc=(math.cos(a) * d, math.sin(a) * d, 0.0),
              rot=(rng.uniform(-14, 14), rng.uniform(-14, 14), rng.uniform(0, 60)), bevel=0.012, segments=1, dens=0, ao=0.25, strength=0.9,
              smooth_angle=20)
    return p.build()


PROPS = {"habitat": build_habitat, "dish": build_dish, "rover": build_rover, "solar": build_solar,
         "boulder": build_boulder, "beacon": build_beacon, "crate": build_crate, "pad": build_pad, "crystal": build_crystal}
