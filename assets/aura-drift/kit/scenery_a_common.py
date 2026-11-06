"""Helpers shared by the scenery A kits: alpine, desert, harbour, salt (MIT, AuraGo contributors).

A ``Piece`` collects shapes per material with a per-vertex colour gradient (dark at the base, lit
tops), so every prop reads like a lit toy diorama at golden hour without any texture. Materials
are ``prop`` (vertex colour only) and four emissive ``glow_*`` tubes and windows.
"""

import math

from mathutils import Vector

from . import materials
from .core import Accum, hex_linear, rng_for
from .shapes import orient_outward, revolve, strut, xf

# name -> (base, metallic, roughness, emission, strength, alpha)
SCENERY_MATERIALS = {
    "prop": ("#ffffff", 0.0, 0.78, None, 0.0, 1.0),
    "glow_warm": ("#ffffff", 0.0, 0.4, "#ffc15e", 2.6, 1.0),
    "glow_pink": ("#ffffff", 0.0, 0.4, "#ff3fa8", 3.0, 1.0),
    "glow_cyan": ("#ffffff", 0.0, 0.4, "#41f0ff", 3.0, 1.0),
    "glow_green": ("#ffffff", 0.0, 0.4, "#7dff6a", 3.0, 1.0),
}


def ensure_materials():
    for name in sorted(SCENERY_MATERIALS):
        base, metallic, rough, emission, strength, alpha = SCENERY_MATERIALS[name]
        materials.make_material(name, base, metallic, rough, emission, strength, alpha)


def colour(value):
    """'#rrggbb' or a linear tuple -> linear tuple."""
    return hex_linear(value) if isinstance(value, str) else tuple(value)


def lit(value, factor):
    c = colour(value)
    return tuple(min(1.0, channel * factor) for channel in c)


class Piece:
    """One prop: shapes grouped by material, finished with a baked ground-occlusion."""

    def __init__(self, name):
        self.name = name
        self.parts = {}
        self.rng = rng_for("scenery-a", name)

    def acc(self, material):
        return self.parts.setdefault(material, Accum())

    def add(self, shape, matrix=None, color="#ffffff", mat="prop", grad=0.3, jitter=0.05):
        """Add a shape. ``color`` is a hex string, a linear tuple or fn(x, y, z) -> colour.

        The shape's own height range drives the gradient: ``grad`` darkens the lower end and lifts the
        top by a quarter of that (``grad`` 0 = flat colour). ``jitter`` varies the colour per shape.
        """
        verts, _ = shape
        world = [tuple(matrix @ Vector(v)) for v in verts] if matrix is not None else [tuple(v) for v in verts]
        ys = [p[1] for p in world]
        low, high = min(ys), max(ys)
        span = (high - low) or 1.0
        vary = 1.0 + self.rng.uniform(-jitter, jitter) if jitter else 1.0
        base = None if callable(color) else colour(color)
        tint = []
        for p in world:
            t = (p[1] - low) / span
            factor = vary * ((1.0 - grad) + grad * t * 1.25)
            c = colour(color(*p)) if base is None else base
            tint.append(tuple(min(1.0, ch * factor) for ch in c))
        self.acc(mat).add(shape, matrix, tint)
        return self

    def add_flat(self, shape, matrix=None, color="#ffffff", mat="prop"):
        """No gradient, no jitter (glow tubes, emblems)."""
        return self.add(shape, matrix, color, mat, grad=0.0, jitter=0.0)

    def bake(self, ao_height=2.2, strength=0.34):
        """Ground contact shadow: ``strength`` darker at y=0, fading out by ``ao_height`` metres."""
        for name, acc in self.parts.items():
            if name != "prop":
                continue
            acc.tint_by(lambda x, y, z: 1.0 - strength * max(0.0, 1.0 - y / ao_height) ** 1.6)
        return self

    def triangles(self):
        return sum(acc.triangles() for acc in self.parts.values())

    def items(self):
        return sorted(self.parts.items())


def lump(rx, ry, rz, seed, seg=7, rings=4, amp=0.22, flat_bottom=False):
    """Faceted boulder: a coarse sphere with jittered vertex radii (deterministic)."""
    rng = rng_for("lump", seed)
    pts = [(0.0, -1.0)]
    for k in range(1, rings):
        a = math.pi * k / rings
        pts.append((math.sin(a), -math.cos(a)))
    pts.append((0.0, 1.0))
    verts, faces = revolve(pts, seg)
    out = []
    for x, y, z in verts:
        k = 1.0 + rng.uniform(-amp, amp) if (abs(x) + abs(z)) > 1e-6 else 1.0
        py = max(y * k, -0.55) if flat_bottom else y * k
        out.append((x * k * rx, py * ry, z * k * rz))
    return out, orient_outward(out, faces)


def rough_frustum(r0, r1, height, seg, table, amp, y0=0.0, caps=True):
    """Y frustum from y0 to y0+height whose radius wobbles with a per-segment ``table`` (so stacked tiers match)."""
    pts = [(0.0, y0), (r0, y0), (r1, y0 + height)]
    pts.append((0.0, y0 + height))
    verts, faces = revolve(pts, seg)
    out = []
    for x, y, z in verts:
        if abs(x) + abs(z) < 1e-6:
            out.append((x, y, z))
            continue
        k = 1.0 + amp * table[int(round((math.atan2(z, x) % math.tau) / math.tau * seg)) % seg]
        out.append((x * k, y, z * k))
    return out, orient_outward(out, faces)


def wobble_table(seed, seg):
    rng = rng_for("wobble", seed)
    return [rng.uniform(-1.0, 1.0) for _ in range(seg)]


def tube_path(points, radius, sides=4, x=0.0):
    """Capped prisms along a polyline given as (y, z) points in the plane x (glow tubes, rails)."""
    shapes = []
    for a, b in zip(points, points[1:]):
        shapes.append(strut((x, a[0], a[1]), (x, b[0], b[1]), radius, sides))
    return shapes


def place(pos=(0, 0, 0), rot=None, scale=None):
    return xf(pos, rot, scale)
