"""Layout specials of the dressed courses (MIT License, see ../LICENSE).

The legacy courses carry a special part per layout (arch, bridge, pier, windmill,
lighthouse, lanterns, ...). In the dioramas they become either fixed placements
of kit props (``Specials.fixed``, kept first by ``placement.place``) or hand-built
geometry (``special_parts``) in the place and size of the old part. Function and
obstacles stay untouched; ``Specials.objects`` are baked with the island.
"""

import math

from . import placement, special_parts as sp
from .prop import Prop

LANTERNS = ((-3.75, -2.7), (3.75, -0.8), (-3.65, 2.0), (3.65, 3.3))
CRYSTALS = ((-3.5, -2.6), (3.45, -2.1), (-3.55, 0.5), (3.55, 2.55))
GEOMETRY = {"pier": sp.pier, "shortjump": sp.ramps, "lilyjump": sp.pads, "revolving_gate": sp.gate_hub}


class Specials:
    """Result of ``build``: geometry to bake, fixed placements, reserved circles and plan changes."""

    def __init__(self):
        self.objects = []
        self.fixed = []
        self.reserved = []
        self.hero = None  # None keeps the plan's hero, a name replaces it
        self.scatter = []  # extra scatter ``(name, count)``
        self.medium = {}  # counts that replace the plan's medium counts


def _beside(f, sign, offset):
    return sp.side(f, sign * offset)


def _along_yaw(f):
    return placement.yaw_for(1.0, (f["tx"], f["ty"]))


def build(look, course, path, catalog):
    """Special geometry and placements of ``course`` (layout) in the ``look`` world."""
    layout = course["layout"]
    out = Specials()
    p = Prop(look, "special", full_name="scenery-special")
    at = lambda t: sp.frame(course, path, t)
    if layout in GEOMETRY:
        GEOMETRY[layout](p, look, course, path)
    elif layout in ("bridge", "moonbridge"):
        sp.parapets(p, look, course, path)
    elif layout == "windmill":
        blades, circle = sp.sun_wheel(p, look, course, path)
        out.objects.extend(blades)
        out.reserved.append(circle)
    elif layout == "flower_arch":
        f = at(0.52)
        out.fixed.append(("gate", f["x"], f["y"], _along_yaw(f), 1.0))
    elif layout == "bank":
        f = at(0.44)
        for sign in (-1, 1):
            x, y = _beside(f, sign, placement.CORRIDOR + catalog["rock"]["radius"] * 1.5 + 0.15)
            out.fixed.append(("rock", x, y, 1.9 * sign, 1.5))
    elif layout == "beach_curve":
        out.medium.update({"palm": 2, "palm2": 2})
    elif layout == "sand":
        out.scatter.append(("tuft", 14))
    elif layout == "harbor_gate":
        f = at(0.55)
        for sign in (-1, 1):
            x, y = _beside(f, sign, placement.CORRIDOR + catalog["post"]["radius"] * 1.3 + 0.15)
            out.fixed.append(("post", x, y, 0.7 * sign, 1.3))
    elif layout == "lighthouse":
        out.hero = "lighthouse"
    elif layout == "crystals":
        out.fixed.extend(("crystal", x, y, 1.1 * k, 1.4) for k, (x, y) in enumerate(CRYSTALS))
    if look == "moon" and layout in ("lantern_curve", "revolving_gate", "finale"):
        out.fixed.extend(("neonpost", x, y, 0.8 * k, 1.0) for k, (x, y) in enumerate(LANTERNS))
    if layout == "finale":
        f = at(0.76)
        out.fixed.append(("tower", -3.25, f["y"] + 0.5, math.radians(35), 0.8))
    if p.parts:
        out.objects.append(sp.strip(p.build()))
    return out
