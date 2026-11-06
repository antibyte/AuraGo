"""Helpers shared by the scenery B kits: forest, industry, glacier, metro (MIT, AuraGo contributors).

Same recipe as scenery A (kept separate so the two authors never depend on each other): a ``Piece``
collects shapes per material with a per-vertex colour gradient (dark at the base, lit tops) and a baked
ground occlusion. Materials are ``prop`` (vertex colour only), six emissive ``glow_*`` colours for
windows, neon strips and beacons, and ``poster`` for the roadside poster face (white, UVs 0 to 1).
"""

import math

from mathutils import Vector

from . import materials
from .core import Accum, hex_linear, rng_for
from .shapes import cylinder_z, orient_outward, revolve, xf

# name -> (base, metallic, roughness, emission, strength, alpha)
SCENERY_MATERIALS = {
    "prop": ("#ffffff", 0.0, 0.78, None, 0.0, 1.0),
    "poster": ("#ffffff", 0.0, 0.55, None, 0.0, 1.0),
    "glow_warm": ("#ffffff", 0.0, 0.4, "#ffc15e", 2.6, 1.0),
    "glow_pink": ("#ffffff", 0.0, 0.4, "#ff3fa8", 3.0, 1.0),
    "glow_cyan": ("#ffffff", 0.0, 0.4, "#41f0ff", 3.0, 1.0),
    "glow_green": ("#ffffff", 0.0, 0.4, "#7dff6a", 3.0, 1.0),
    "glow_red": ("#ffffff", 0.0, 0.4, "#ff2a1a", 3.0, 1.0),
    "glow_amber": ("#ffffff", 0.0, 0.4, "#ffa21f", 3.0, 1.0),
}


def ensure_materials():
    for name in sorted(SCENERY_MATERIALS):
        base, metallic, rough, emission, strength, alpha = SCENERY_MATERIALS[name]
        materials.make_material(name, base, metallic, rough, emission, strength, alpha)


def colour(value):
    return hex_linear(value) if isinstance(value, str) else tuple(value)


def mix(a, b, t):
    a, b = colour(a), colour(b)
    return tuple(x * (1.0 - t) + y * t for x, y in zip(a, b))


class Piece:
    """One prop: shapes grouped by material, finished with a baked ground-occlusion."""

    def __init__(self, name):
        self.name = name
        self.parts = {}
        self.rng = rng_for("scenery-b", name)

    def acc(self, material):
        return self.parts.setdefault(material, Accum())

    def add(self, shape, matrix=None, color="#ffffff", mat="prop", grad=0.3, jitter=0.05):
        """``color``: hex, linear tuple or fn(x, y, z). ``grad`` darkens the shape's low end, lifts its top."""
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
            col = colour(color(*p)) if base is None else base
            tint.append(tuple(min(1.0, ch * factor) for ch in col))
        self.acc(mat).add(shape, matrix, tint)
        return self

    def add_flat(self, shape, matrix=None, color="#ffffff", mat="prop"):
        return self.add(shape, matrix, color, mat, grad=0.0, jitter=0.0)

    def bake(self, ao_height=2.2, strength=0.34):
        for name, acc in self.parts.items():
            if name == "prop":
                acc.tint_by(lambda x, y, z: 1.0 - strength * max(0.0, 1.0 - y / ao_height) ** 1.6)
        return self

    def triangles(self):
        return sum(acc.triangles() for acc in self.parts.values())

    def items(self):
        return sorted(self.parts.items())


def lump(rx, ry, rz, seed, seg=7, rings=4, amp=0.22, flat_bottom=False):
    """Faceted boulder or leaf clump: a coarse sphere with jittered vertex radii (deterministic)."""
    rng = rng_for("lump-b", seed)
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


def wall_quad(centre, normal, width, height, up=(0.0, 1.0, 0.0)):
    """Single-sided quad facing ``normal`` (windows, neon panels). Winding is fixed so it never culls itself."""
    n = Vector(normal).normalized()
    u = Vector(up).cross(n)
    if u.length < 1e-6:
        u = Vector((1.0, 0.0, 0.0))
    u.normalize()
    v = n.cross(u)
    c = Vector(centre)
    hw, hh = width / 2.0, height / 2.0
    # u = up x n, v = n x u: (u, v, n) is right handed, so u then v counter-clockwise looks down +n
    pts = [c - u * hw - v * hh, c + u * hw - v * hh, c + u * hw + v * hh, c - u * hw + v * hh]
    return [tuple(p) for p in pts], [(0, 1, 2, 3)]


def disc(centre, normal, radius, seg=8, up=(0.0, 1.0, 0.0)):
    """Single-sided n-gon facing ``normal`` (portholes, round lamps): ``seg - 2`` triangles."""
    n = Vector(normal).normalized()
    u = Vector(up).cross(n)
    if u.length < 1e-6:
        u = Vector((1.0, 0.0, 0.0))
    u.normalize()
    v = n.cross(u)
    pts = [Vector(centre) + (u * math.cos(math.tau * k / seg) + v * math.sin(math.tau * k / seg)) * radius for k in range(seg)]
    return [tuple(pt) for pt in pts], [tuple(range(seg))]


def wheel(p, x, y, z, r, w, detail=1, rim="#c9ced8", tyre="#262a33"):
    """Chunky toy wheel with an axle along Z. ``detail`` 0: 8-sided tyre and a rim disc; 1: chamfered tyre, rim and hub."""
    side = 1.0 if z >= 0 else -1.0
    if detail == 0:
        p.add(cylinder_z(r, r, w, 8), xf((x, y, z)), tyre, grad=0.1, jitter=0.0)
        p.add(cylinder_z(r * 0.58, r * 0.58, 0.06, 6), xf((x, y, z + side * (w / 2.0))), rim, grad=0.0, jitter=0.0)
        return
    p.add(cylinder_z(r, r, w, 10, chamfer=min(0.06, w * 0.2)), xf((x, y, z)), tyre, grad=0.1, jitter=0.0)
    p.add(cylinder_z(r * 0.62, r * 0.62, 0.05, 10), xf((x, y, z + side * (w / 2.0))), rim, grad=0.0, jitter=0.0)
    p.add(cylinder_z(r * 0.22, r * 0.22, 0.08, 6), xf((x, y, z + side * (w / 2.0 + 0.03))), "#f0f2f6", grad=0.0, jitter=0.0)


def place(pos=(0, 0, 0), rot=None, scale=None):
    return xf(pos, rot, scale)
