"""Visible rail trims of a dressed course (MIT License, see ../LICENSE).

The collision rails ``rail-left`` / ``rail-right`` stay in the GLB (and are
hidden at runtime). ``rail-trim-left`` / ``rail-trim-right`` are decoration with
a world look: the inner face lies exactly in the plane of the collision rail, the
top is rounded and stands at most 2 cm above it, the outside at most 3 cm out.
Every cross-section comes from ``courses_v1.rail_edge``, the same sampling that
builds the collision rails.
"""

import math
import random

from mathutils import Vector, noise

from . import colors, materials, palette, shapes
from .courses_v1 import rail_edge
from .island import smoothstep
from .palette import mix
from .prop import Prop

SIDES = (("rail-trim-left", -1), ("rail-trim-right", 1))
INNER_BOTTOM, OUTER_BOTTOM = -0.02, -0.07
SKIRT_DEPTH = -0.5  # deepest reach of the outer face below the lane edge where the ground falls away
TOP_FLAT = 0.215
ARC_RISE = 0.045
WIDTH = 0.17
POST_SIZE, POST_SPACING = (0.07, 0.07, 0.32), 1.2
CLAMP_SIZE, CLAMP_SPACING = (0.21, 0.06, 0.30), 1.5
NEON_HALF = 0.01
JOINT = {"pirate": 0.9, "temple": 0.6, "moon": 0.9}


def trim_profile(steps=6):
    """(across, up) offsets from the inner rail edge at lane height: flush inner face, rounded top <= +2 cm.

    The plan profile with fewer arc facets; the bottom reaches under the island
    bed so no slit shows, the inner face starts just under the lane edge.
    """
    pts = [(0.0, INNER_BOTTOM), (0.0, TOP_FLAT)]
    for k in range(1, steps):
        a = math.pi - k * math.pi / steps
        pts.append((WIDTH / 2 + WIDTH / 2 * math.cos(a), TOP_FLAT + ARC_RISE * math.sin(a)))
    pts += [(WIDTH, TOP_FLAT), (WIDTH, OUTER_BOTTOM)]
    return pts


def _arclength(edges):
    arc = [0.0]
    for i in range(1, len(edges)):
        a, b = edges[i - 1][0], edges[i][0]
        arc.append(arc[-1] + math.hypot(b[0] - a[0], b[1] - a[1]))
    return arc


def _at(edges, arc, s):
    """Interpolated (x, y, lane height, outward x, outward y) of the inner edge at arc length ``s``."""
    i = max(0, min(len(arc) - 2, next((k for k in range(len(arc) - 1) if arc[k + 1] >= s), len(arc) - 2)))
    span = max(1e-9, arc[i + 1] - arc[i])
    u = max(0.0, min(1.0, (s - arc[i]) / span))
    (ax, ay), _, ah, (aux, auy) = edges[i]
    (bx, by), _, bh, (bux, buy) = edges[i + 1]
    ux, uy = aux + (bux - aux) * u, auy + (buy - auy) * u
    ul = max(1e-9, math.hypot(ux, uy))
    return ax + (bx - ax) * u, ay + (by - ay) * u, ah + (bh - ah) * u, ux / ul, uy / ul


def _colour_fn(look, pal, arc, rng):
    """Vertex colour of the trim body at ring ``i``, profile point (across, up)."""
    off = Vector((rng.uniform(-30, 30), rng.uniform(-30, 30), rng.uniform(-30, 30)))
    block = JOINT[look]

    def joint_weight(i, width):
        fr = (arc[i] / block) % 1.0
        return 1.0 - smoothstep(min(fr, 1.0 - fr) * block, 0.0, width)

    if look == "pirate":
        lo, hi, dark = pal.c("wood_dark"), pal.c("wood_light"), pal.c("wood_deep")

        def colour(i, across, up):
            tz = max(0.0, min(1.0, (up - OUTER_BOTTOM) / (TOP_FLAT + ARC_RISE - OUTER_BOTTOM))) ** 0.9
            c = mix(lo, hi, tz)
            plank = int(arc[i] / block)
            k = 1.0 + 0.07 * (((plank * 37) % 7) / 3.5 - 1.0)
            k *= 1.0 + 0.08 * noise.noise(Vector((arc[i] * 2.2, up * 14.0, across * 9.0)) + off)
            c = mix(c, dark, 0.55 * joint_weight(i, 0.08))
            return (c[0] * k, c[1] * k, c[2] * k, 1.0)
        return colour, "matte"
    if look == "temple":
        lo, mid, hi = pal.c("stone_dark"), pal.c("stone"), pal.c("stone_light")
        seam, moss, moss_l = pal.c("stone_deep"), pal.c("moss"), pal.c("moss_light")

        def colour(i, across, up):
            tz = max(0.0, min(1.0, (up - OUTER_BOTTOM) / (TOP_FLAT + ARC_RISE - OUTER_BOTTOM)))
            c = mix(lo, mid, tz / 0.6) if tz < 0.6 else mix(mid, hi, (tz - 0.6) / 0.4)
            k = 1.0 + 0.10 * noise.noise(Vector((arc[i] * 1.7, up * 10.0, across * 8.0)) + off)
            c = (c[0] * k, c[1] * k, c[2] * k, 1.0)
            c = mix(c, seam, 0.8 * joint_weight(i, 0.07))
            if up > TOP_FLAT - 0.01:
                m = 0.75 * smoothstep(up, TOP_FLAT - 0.01, TOP_FLAT + 0.02)
                c = mix(c, mix(moss, moss_l, 0.5 + 0.5 * noise.noise(Vector((arc[i] * 3.0, across * 20.0, 4.0)) + off)), m)
            return c
        return colour, "matte"
    lo, hi, dark = pal.c("panel_grey"), pal.c("panel_white"), pal.c("metal_dark")

    def colour(i, across, up):
        tz = max(0.0, min(1.0, (up - OUTER_BOTTOM) / (TOP_FLAT + ARC_RISE - OUTER_BOTTOM)))
        c = mix(lo, hi, tz ** 0.8)
        c = mix(c, dark, 0.7 * joint_weight(i, 0.05))
        return c
    return colour, "satin"


def _neon_strip(name, edges, sign):
    verts, faces = [], []
    for inner, _, h, (ux, uy) in edges:
        cx, cy = inner[0] + ux * WIDTH / 2, inner[1] + uy * WIDTH / 2
        z = h + TOP_FLAT + ARC_RISE + 0.004
        verts += [(cx - ux * NEON_HALF, cy - uy * NEON_HALF, z), (cx + ux * NEON_HALF, cy + uy * NEON_HALF, z)]
    for i in range(len(edges) - 1):
        a = 2 * i
        faces.append((a, a + 1, a + 3, a + 2) if sign > 0 else (a, a + 2, a + 3, a + 1))
    return shapes.make_object(name, verts, faces, smooth=False)


def _ring_profile(inner, h, ux, uy, ground):
    profile = trim_profile()
    if ground is not None:
        low = ground(inner[0] + ux * WIDTH, inner[1] + uy * WIDTH) - h - 0.02
        profile[-1] = (WIDTH, max(SKIRT_DEPTH, min(OUTER_BOTTOM, low)))
    return profile


def _strip_custom_props(obj):
    for key in [k for k in obj.keys() if not k.startswith("aurago_")]:
        del obj[key]


def build_trims(look, course, path, number, ground=None):
    """Build ``rail-trim-left`` / ``rail-trim-right`` in the active collection; returns the two objects.

    ``ground(x, y)`` (the island height) lets the outer face reach down to the
    ground where it falls away from the lane (hills, hairpins), up to 0.5 m.
    """
    pal = palette.get(look)
    out = []
    for name, sign in SIDES:
        rng = random.Random(7000 + number * 2 + (sign > 0))
        edges = [rail_edge(course, path, i, sign) for i in range(len(path))]
        arc = _arclength(edges)
        profiles = [_ring_profile(inner, h, ux, uy, ground) for inner, _, h, (ux, uy) in edges]
        rings = [[(inner[0] + ux * a, inner[1] + uy * a, h + up) for a, up in profile]
                 for profile, (inner, _, h, (ux, uy)) in zip(profiles, edges)]
        colour, kind = _colour_fn(look, pal, arc, rng)
        cols = [colour(i, a, up) for i, profile in enumerate(profiles) for a, up in profile]
        p = Prop(look, "trim", seed=number + (sign > 0), full_name=name)
        body = shapes.loft(name + "-body", rings, closed=True, cap_start=True, cap_end=True, recalc=True)
        shapes.finish(body, bevel=0, dens=0, smooth_angle=55)
        colors.paint_values(body, cols, ao=0.6)
        body.data.materials.append(materials.get(kind))
        p.parts.append(body)
        _variants(p, look, edges, arc, sign)
        obj = p.build()
        _strip_custom_props(obj)
        out.append(obj)
    return out


def _variants(p, look, edges, arc, sign):
    total = arc[-1]
    if look == "pirate":
        s = POST_SPACING / 2
        while s < total - 0.3:
            x, y, h, ux, uy = _at(edges, arc, s)
            post = shapes.box("trim-post", POST_SIZE, base=True)
            p.add(post, "matte", "wood", loc=(x + ux * (WIDTH + POST_SIZE[0] / 2 - 0.005), y + uy * (WIDTH + POST_SIZE[0] / 2 - 0.005), h - 0.04),
                  rot=(0, 0, math.degrees(math.atan2(uy, ux))), bevel=0.012, segments=1, dens=0, ao=0.6)
            s += POST_SPACING
    elif look == "moon":
        tone = "cyan" if sign < 0 else "magenta"
        strip = _neon_strip("trim-neon", edges, sign)
        p.add(strip, "emissive", (tone, tone), finish=False, ao=0.0, strength=1.6, jitter=0.0)
        s = CLAMP_SPACING / 2
        while s < total - 0.3:
            x, y, h, ux, uy = _at(edges, arc, s)
            clamp = shapes.box("trim-clamp", CLAMP_SIZE, base=True)
            p.add(clamp, "satin", "orange", loc=(x + ux * WIDTH / 2, y + uy * WIDTH / 2, h - 0.03),
                  rot=(0, 0, math.degrees(math.atan2(uy, ux))), bevel=0.01, segments=1, dens=0, ao=0.6)
            s += CLAMP_SPACING
