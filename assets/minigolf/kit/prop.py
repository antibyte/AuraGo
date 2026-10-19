"""Prop composition helper (MIT License).

A prop is built from parts. ``Prop.add`` finishes each raw shape (bevel,
subsurf, shading normals), places it in prop space, paints its gradient and
assigns one of the shared materials; ``Prop.build`` joins everything into one
mesh named ``prop-<world>-<name>`` with its base at z=0 and footprint centred
on the origin.
"""

import random

from . import colors, materials, palette, shapes

_FINISH_KEYS = ("bevel", "segments", "subsurf", "dens", "angle", "smooth_angle", "solidify", "weighted")
_PAINT_KEYS = ("mode", "ease", "span", "jitter", "noise_amp", "noise_scale", "center", "stops", "pattern")


class Prop:
    def __init__(self, world, name, seed=1, full_name=None):
        self.world = world
        self.short = name
        self.full_name = full_name
        self.pal = palette.get(world)
        self.rng = random.Random(seed)
        self.parts = []

    @property
    def name(self):
        return self.full_name or "prop-%s-%s" % (self.world, self.short)

    def colour(self, spec):
        if isinstance(spec, str):
            return self.pal.c(spec)
        return spec

    def gradient(self, grad):
        """Resolve a preset name, ``(lo, hi)`` names/colours or a single tone."""
        if isinstance(grad, str):
            if grad in self.pal._grads:
                return self.pal.grad(grad)
            c = self.pal.c(grad)
            return c, c
        lo, hi = grad
        return self.colour(lo), self.colour(hi)

    def add(self, obj, mat="matte", grad="wood", loc=(0, 0, 0), rot=(0, 0, 0), scale=(1, 1, 1),
            finish=True, paint=True, ao=None, fn=None, **opts):
        """Finish, place, paint and register a raw shape. Returns the object.

        ``paint=False`` keeps colour attributes the caller wrote itself (their
        vertex order must survive ``finish``, i.e. ``dens=0``). ``fn(x, y, z)``
        paints by final position (after finish and placement) instead of a
        gradient, which is vertex-order independent.
        """
        fin = {k: opts.pop(k) for k in list(opts) if k in _FINISH_KEYS}
        pnt = {k: opts.pop(k) for k in list(opts) if k in _PAINT_KEYS}
        strength = opts.pop("strength", 3.0)
        if opts:
            raise TypeError("unknown options: %s" % sorted(opts))
        if finish:
            shapes.finish(obj, **fin)
        shapes.transform(obj, loc, rot, scale)
        if fn is not None:
            colors.paint_fn(obj, fn, ao=0.65 if ao is None else ao)
        elif paint:
            lo, hi = self.gradient(grad)
            if ao is None:
                ao = 0.0 if mat == "emissive" else 0.65
            colors.paint(obj, lo, hi, rng=self.rng, ao=ao, **pnt)
        obj.data.materials.append(materials.get(mat, strength=strength))
        self.parts.append(obj)
        return obj

    def build(self, loc=(0, 0, 0), rot=(0, 0, 0)):
        self.part_tris = {}
        for part in self.parts:
            key = part.name.split(".")[0]
            self.part_tris[key] = self.part_tris.get(key, 0) + shapes.tri_count(part)
        obj = shapes.join(self.name, self.parts)
        obj["kit_part_tris"] = str(sorted(self.part_tris.items(), key=lambda kv: -kv[1]))
        obj.location = loc
        obj.rotation_euler = [r * 3.14159265358979 / 180.0 for r in rot]
        return obj
