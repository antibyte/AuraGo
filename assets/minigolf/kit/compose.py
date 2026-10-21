"""Shared scene-composition helpers for the probes (MIT License)."""

import math
import random

from mathutils import Vector, noise

from . import shapes
from .island import smoothstep


class Composer:
    """Collects scene objects by role and places props on the island."""

    def __init__(self, world, seed=1):
        self.world = world
        self.rng = random.Random(seed)
        self.bake_objs = []
        self.hidden = []
        self.passthrough = []
        self.backdrop = []  # sky bodies (stars, Earth): counted separately from game geometry
        self.ground = None  # callable (x, y) -> z
        self.prop_tris = {}

    def add(self, obj, kind="bake", backdrop=False):
        {"bake": self.bake_objs, "hidden": self.hidden, "pass": self.passthrough}[kind].append(obj)
        if backdrop:
            self.backdrop.append(obj)
        return obj

    def height(self, x, y):
        return self.ground(x, y) if self.ground else 0.0

    def place(self, obj, x, y, yaw=0.0, tilt=(0.0, 0.0), sink=0.02, z=None, kind="bake", scale=None):
        """Put ``obj`` on the ground at (x, y); ``yaw``/``tilt`` in degrees."""
        base = self.height(x, y) - sink if z is None else z
        obj.location = (x, y, base)
        obj.rotation_euler = (math.radians(tilt[0]), math.radians(tilt[1]), math.radians(yaw))
        if scale is not None:
            obj.scale = (scale, scale, scale) if not isinstance(scale, (tuple, list)) else scale
        self.add(obj, kind)
        return obj

    def record(self, key, obj):
        self.prop_tris.setdefault(key, shapes.tri_count(obj))

    def total_tris(self):
        """Triangles of everything except the sky backdrop."""
        everything = self.bake_objs + self.hidden + self.passthrough
        return sum(shapes.tri_count(o) for o in everything if o not in self.backdrop)

    def backdrop_tris(self):
        return sum(shapes.tri_count(o) for o in self.backdrop)


def fbm(x, y, freq, off, octaves=2):
    v, amp, tot = 0.0, 1.0, 0.0
    for k in range(octaves):
        v += amp * noise.noise(Vector((x * freq * 2 ** k, y * freq * 2 ** k, 0.0)) + off)
        tot += amp
        amp *= 0.5
    return v / tot


def lane_bed(lane, x, y, base_z, bed_z=-0.06, inner=0.2, outer=1.1):
    """Flatten the terrain under and next to the lane (felt is at z=0)."""
    d, _ = lane.nearest(x, y)
    reach = 0.86 + 0.14
    w = smoothstep(d, reach + inner, reach + outer)
    return bed_z + (base_z - bed_z) * w


def cup_dip(lane, x, y, z, depth=0.34):
    """Local hollow under the cup so the open cup shows only its dark inside."""
    cup = lane.cup
    r = math.hypot(x - cup.x, y - cup.y)
    return z - depth * (1.0 - smoothstep(r, 0.34, 0.62))
