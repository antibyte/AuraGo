"""Aura Drift shared car parts: builder, wheels, lights, wings (MIT, AuraGo contributors).

Use ``CarBuilder`` for every car. It owns one Accum per body material, the wheels, and the
required anchor empties, and ``finish()`` returns the ``core.Model`` (``body``, four wheels, anchors).
See README.md for a walk-through.

Frame: nose +X, right side +Z (left is -Z), origin at ground centre, wheels rest on y=0.
"""

import math

from .core import Accum, Model, hex_linear
from .shapes import (
    annulus_z,
    box,
    cylinder_x,
    ellipsoid,
    extrude_z,
    mirror_z,
    revolve_z,
    torus,
    transform,
    xf,
)

BODY_ORDER = ("paint", "paint_accent", "trim", "glass", "chrome", "head_light", "brake_light")
REQUIRED_ANCHORS = (
    "anchor_exhaust_l",
    "anchor_exhaust_r",
    "anchor_head_l",
    "anchor_head_r",
    "anchor_tail_l",
    "anchor_tail_r",
    "anchor_weapon",
)

# Vertex-colour tones (linear). Wheels get their tones from these, the body from ``ao``.
RUBBER = hex_linear("#23262c")
TREAD = hex_linear("#30343b")
STRIPE = hex_linear("#f3ead8")
BARREL = hex_linear("#d3d8e0")
DISH = hex_linear("#3a404b")
SPOKE = hex_linear("#e9ecf2")
NUT = hex_linear("#b6bcc7")


def smoothstep(a, b, x):
    t = min(max((x - a) / (b - a), 0.0), 1.0)
    return t * t * (3.0 - 2.0 * t)


def default_ao(x, y, z):
    """Baked ambient occlusion: darker near the ground, full brightness from 0.9 m up."""
    return 0.62 + 0.38 * smoothstep(0.12, 0.9, y)


# --------------------------------------------------------------------------
# wheels
# --------------------------------------------------------------------------


def wheel_accum(radius, width, hub=None, tread_blocks=13, spokes=5, rim_ratio=0.62, tread_h=0.045):
    """Right-hand wheel (outer face toward +Z), axle along Z, centred on the origin.

    radius is the outer radius including tread lugs; ``hub`` is the linear hub-cap tone (the car's
    accent). Detail is only on the +Z face: a rounded tyre with chevron lugs, sidewall stripe, a
    dished rim with bright spokes, a hub cap and lug nuts. About 1000 triangles.
    """
    hub = hub or hex_linear("#ff5a4e")
    R, h = radius, width / 2.0
    rt = R - tread_h
    rim_r = R * rim_ratio
    hub_r = rim_r * 0.30
    acc = Accum()

    tyre = revolve_z(
        [
            (rim_r * 0.95, -h * 0.90),
            (R * 0.80, -h),
            (rt * 0.95, -h * 0.90),
            (rt * 0.995, -h * 0.58),
            (rt, -h * 0.28),
            (rt, h * 0.28),
            (rt * 0.995, h * 0.58),
            (rt * 0.95, h * 0.90),
            (R * 0.80, h),
            (rim_r * 0.95, h * 0.90),
        ],
        seg=20,
        closed=True,
    )
    acc.add(tyre, tint=RUBBER)

    acc.add(annulus_z(R * 0.70, R * 0.75, h + 0.004, 20), tint=STRIPE)

    lug_radial = tread_h + 0.03
    pitch = math.tau * rt / tread_blocks
    for side in (1, -1):
        for k in range(tread_blocks):
            a = math.degrees(math.tau * (k + (0.5 if side > 0 else 0.0)) / tread_blocks)
            place = xf((0, 0, 0), rot=(0, 0, a)) @ xf((R - lug_radial / 2.0, 0, side * h * 0.36), rot=(side * 24.0, 0, 0))
            acc.add(box(lug_radial, pitch * 0.5, h * 0.72), place, tint=TREAD)

    def rim_tone(x, y, z):
        r = math.hypot(x, y)
        if r < hub_r * 1.05 and z > h * 0.6:
            return hub
        if r > rim_r * 0.9:
            return BARREL
        return DISH

    rim = revolve_z(
        [
            (0.0, h * 0.86),
            (hub_r * 0.85, h * 0.86),
            (hub_r, h * 0.74),
            (rim_r * 0.80, h * 0.50),
            (rim_r * 0.93, h * 0.78),
            (rim_r * 1.02, h * 0.92),
            (rim_r * 1.04, h * 0.84),
            (rim_r * 1.00, h * 0.30),
            (rim_r * 0.96, 0.0),
            (0.0, 0.0),
        ],
        seg=max(spokes * 3, 15),
    )
    acc.add(rim, tint=[rim_tone(*v) for v in rim[0]])

    span = rim_r * 0.93 - hub_r * 0.8
    for k in range(spokes):
        a = 360.0 * k / spokes + 90.0
        place = xf((0, 0, 0), rot=(0, 0, a)) @ xf(((hub_r * 0.8 + rim_r * 0.93) / 2.0, 0, h * 0.70))
        acc.add(box(span, rim_r * 0.20, h * 0.30), place, tint=SPOKE)

    for k in range(5):
        a = math.tau * k / 5.0
        acc.add(box(R * 0.07, R * 0.07, 0.04), xf((hub_r * 0.55 * math.cos(a), hub_r * 0.55 * math.sin(a), h * 0.88), rot=(0, 0, math.degrees(a))), tint=NUT)
    return acc


# --------------------------------------------------------------------------
# builder
# --------------------------------------------------------------------------


class CarBuilder:
    """Accumulates body shapes per material, then writes body + wheels + anchors.

    car = CarBuilder("vektor", wheel_radius=0.38, wheel_width=0.34, wheelbase=2.7, track=0.78)
    car.add("paint", box(...), xf((x, y, z)))
    car.pair("chrome", cylinder_x(...), (x, y, z))       # right shape at +z, mirrored at -z
    car.anchor("anchor_weapon", (x, y, z))
    model = car.finish()
    """

    def __init__(self, car_id, wheel_radius=0.38, wheel_width=0.34, wheelbase=2.7, track=0.78, rear_shift=0.0, hub=None, ao=default_ao):
        self.car_id = car_id
        self.acc = {name: Accum() for name in BODY_ORDER}
        self.anchors = {}
        self.wheel_radius = wheel_radius
        self.wheel_width = wheel_width
        self.wheelbase = wheelbase
        self.track = track
        self.rear_shift = rear_shift
        self.hub = hub
        self.ao = ao
        self.wheel_kwargs = {}

    def add(self, material, shape, matrix=None, tint=None, faces=None):
        self.acc[material].add(shape, matrix, tint, faces)

    def pair(self, material, shape, pos, rot=None, tint=None, faces=None):
        """Place ``shape`` on the right (+z) side and its mirror image on the left."""
        placed = transform(shape, xf(pos, rot))
        self.acc[material].add(placed, None, tint, faces)
        self.acc[material].add(mirror_z(placed), None, tint, faces)

    def anchor(self, name, pos):
        self.anchors[name] = pos

    def anchor_pair(self, base, pos):
        """anchor_<base>_r at pos (z >= 0), anchor_<base>_l mirrored."""
        self.anchors[base + "_r"] = pos
        self.anchors[base + "_l"] = (pos[0], pos[1], -pos[2])

    def lamp(self, kind, pos, size, rot=None, bevel=0.02, frame=0.025):
        """Light unit at ``pos`` on the right plus its mirror: a trim bezel and an emissive lens.

        kind 'head' or 'brake'. size is (depth along the lamp axis, height, width).
        """
        sx, sy, sz = size
        bezel = box(sx, sy + frame * 2, sz + frame * 2, bevel, 1)
        lens = box(sx * 1.05, sy, sz, bevel * 0.8, 1)
        out = 1.0 if kind == "head" else -1.0  # the lens sits proud of the bezel, toward the outside
        self.pair("trim", bezel, pos, rot)
        self.pair("head_light" if kind == "head" else "brake_light", lens, (pos[0] + out * sx * 0.08, pos[1], pos[2]), rot)

    def exhaust(self, x, y, z, radius=0.075, length=0.26):
        """Chrome tailpipe pair ending at x - length/2, with a dark bore."""
        pipe = cylinder_x(radius, radius * 1.05, length, 10, chamfer=0.012)
        self.pair("chrome", pipe, (x, y, z))
        self.pair("trim", cylinder_x(radius * 0.7, radius * 0.7, 0.02, 10), (x - length / 2.0 - 0.002, y, z))
        self.anchor_pair("anchor_exhaust", (x - length / 2.0 - 0.02, y, z))

    def wing(self, x, y, span, chord=0.6, thick=0.09, aoa=7.0, plate_h=0.34, plate_len=0.72, pylons=0.36, deck_y=0.0):
        """Rear wing: airfoil plane (paint_accent), paint end plates, trim pylons down to ``deck_y``."""
        from .shapes import airfoil, strut

        self.add("paint_accent", airfoil(chord, thick, span, 0.012), xf((x, y, 0.0), rot=(0, 0, -aoa)))
        plate = extrude_z([(plate_len * 0.5, -plate_h * 0.45), (plate_len * 0.5, plate_h * 0.30), (-plate_len * 0.5, plate_h * 0.55), (-plate_len * 0.5, -plate_h * 0.45)], 0.0, 0.05, 0.008)
        self.pair("paint", plate, (x, y + 0.02, span / 2.0 - 0.025))
        if deck_y:
            for z in (pylons, -pylons):
                self.add("trim", box(0.16, y - deck_y, 0.05, 0.01, 1), xf((x - 0.02, (y + deck_y) / 2.0, z)))

    def mirror_housing(self, pos, size=(0.2, 0.1, 0.14)):
        """Side mirror: a stalk and a paint housing (right side plus mirror)."""
        x, y, z = pos
        self.pair("trim", box(0.05, 0.05, 0.12, 0.01, 1), (x, y - 0.02, z - 0.06))
        self.pair("paint", box(size[0], size[1], size[2], 0.03, 1), (x - 0.02, y, z + 0.02))

    def driver(self, pos, helmet_scale=1.0):
        """Seat, driver torso, helmet (paint_accent) with a dark visor, and a steering wheel."""
        x, y, z = pos
        self.add("trim", box(0.62, 0.32, 0.62), xf((x - 0.12, y - 0.1, z)))
        self.add("trim", box(0.14, 0.62, 0.62), xf((x - 0.42, y + 0.14, z), rot=(0, 0, -12)))
        self.add("trim", ellipsoid(0.17, 0.2, 0.24, 8, 4), xf((x - 0.18, y + 0.2, z)))
        helmet = 0.17 * helmet_scale
        self.add("paint_accent", ellipsoid(helmet, helmet, helmet, 8, 4), xf((x - 0.18, y + 0.52, z)))
        self.add("trim", box(0.1, 0.07, 0.2), xf((x - 0.04, y + 0.53, z)))
        self.add("trim", torus(0.1, 0.018, 8, 4), xf((x + 0.28, y + 0.22, z), rot=(0, 0, 66)))

    # ------------------------------------------------------------------

    def finish(self):
        """Create body, wheels and anchors; returns the ``core.Model`` named ``car-<id>``."""
        missing = [a for a in REQUIRED_ANCHORS if a not in self.anchors]
        if missing:
            raise ValueError("car %s lacks anchors %s" % (self.car_id, missing))
        model = Model("car-" + self.car_id)
        for name in ("paint", "paint_accent", "trim"):
            self.acc[name].tint_by(self.ao)
        model.body = model.mesh("body", [(name, self.acc[name]) for name in BODY_ORDER], shade="flat")
        print("  body materials: " + ", ".join("%s=%d" % (n, self.acc[n].triangles()) for n in BODY_ORDER))

        R = self.wheel_radius
        right = wheel_accum(R, self.wheel_width, self.hub, **self.wheel_kwargs)
        left = Accum()
        left.verts = [(x, y, -z) for x, y, z in right.verts]
        left.faces = [tuple(reversed(f)) for f in right.faces]
        left.cols = list(right.cols)
        front_x = self.wheelbase / 2.0 + self.rear_shift
        rear_x = -self.wheelbase / 2.0 + self.rear_shift
        wr = model.mesh("wheel_fr", [("tyre", right)], shade="auto", angle=40.0, location=(front_x, R, self.track))
        wl = model.mesh("wheel_fl", [("tyre", left)], shade="auto", angle=40.0, location=(front_x, R, -self.track))
        model.instance("wheel_rr", wr, (rear_x, R, self.track))
        model.instance("wheel_rl", wl, (rear_x, R, -self.track))
        for name in REQUIRED_ANCHORS:
            model.empty(name, self.anchors[name])
        return model


def finish_hull_parts(hull, bands):
    """Helper: ``bands`` maps material -> face index lists; returns [(material, faces)] with the rest as 'paint'."""
    taken = [f for faces in bands.values() for f in faces]
    out = [("paint", hull.rest(taken))]
    out += [(material, hull.pick(faces)) for material, faces in bands.items()]
    return out
