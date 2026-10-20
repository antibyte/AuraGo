"""Dressing of the legacy courses as pirate, temple and moon dioramas (MIT License).

``courses_v1.build_course(number, course, dresser)`` keeps building the lane,
collision rails, obstacles, tee, cup and flag exactly as before; the ``Dresser``
replaces the old decor around them:

* ``lane_mats``     material set for ``make_surface`` (one shared ``matte``; the
                    colour comes from POINT vertex colours, no shading change)
* ``dress``         island (``dress_island``), ``water-shore`` strip, rail trims
                    (``trim``) and the lane colours
* ``top_height``    ground height under any point (cup floor, placements)
* ``specials``      layout specific parts (Task 7 hook, builds nothing yet)
* ``placements``    prop placements in Blender coordinates (Task 7 fills the list,
                    ``runtime_props`` converts it for the manifest)
* ``bake``          vertex AO for island, trims and lane; templates, obstacles,
                    rails and the shore are hidden from the occluders, the shore
                    only gets ``Col``

Lane meshes (``surface-*``) only receive a POINT colour attribute and the
collision rails (``rail-*``) nothing but ``aurago_hidden``; neither gets a bevel,
smoothing or shading change, so the exporter splits their vertices exactly as
before (guarded by the P1 collider fingerprint). The rails carry no colour: they
are hidden at runtime and COLOR_0 would cost about 30 KB per course.
"""

import math

from . import bake, colors, dress_island, materials, palette, shapes, trim
from .courses_v1 import LANE_ROWS
from .island import smoothstep
from .palette import mix

LOOK = {"coast": "pirate", "garden": "temple", "night": "moon"}
BAKE_TINT = {"pirate": (0.10, 0.10, 0.22), "temple": (0.16, 0.08, 0.10), "moon": (0.14, 0.12, 0.30)}
BAKE_SAMPLES, BAKE_DISTANCE = 160, 1.5
LANE_AO = 0.75
STRIPE_LENGTH = 0.62


def _base(obj):
    return obj.name.split(".")[0]


class Dresser:
    """Per-course dressing state; build with ``Dresser(number, course)``."""

    def __init__(self, number, course):
        self.number, self.course = number, course
        self.look = LOOK[course["world"]]
        self.pal = palette.get(self.look)
        self.terrain = None
        self.island = None
        self.shore = None
        self.trims = []
        self.placements = []

    # -- hooks used by courses_v1.build_course ------------------------------

    def lane_mats(self, mats):
        """Copy of ``mats`` whose lane and rail slots are the shared vertex-coloured matte material."""
        lane = materials.get("matte")
        return dict(mats, grass=lane, sand=lane, wood=lane, stone=lane)

    def top_height(self, x, y):
        return self.terrain.top_height(x, y)

    def dress(self, coll, path, cup):
        """Build island, shore and trims into ``coll`` and colour the lane."""
        shapes.set_collection(coll)
        try:
            self.terrain = dress_island.Terrain(self.look, self.number, self.course, path, cup)
            self.island, waterline = dress_island.build(self.terrain, self.number)
            self.shore = dress_island.build_shore(self.terrain, waterline) if waterline else None
            self.trims = trim.build_trims(self.look, self.course, path, self.number, self.terrain.top_height)
        finally:
            shapes.set_collection(None)
        self._paint_lane(coll)

    def specials(self, scene, coll, course, path, mats):
        """Layout specific parts (pier, gate, wheel, ...): Task 7."""

    def runtime_props(self):
        return [[name, x, z, -y, yaw, scale] for name, x, y, z, yaw, scale in self.placements]

    # -- colours --------------------------------------------------------------

    def _lane_colours(self, kind):
        pal = self.pal
        if kind == "sand":
            lo, hi = ((pal.c("sand_wet"), pal.c("sand_light")) if pal.has("sand_wet")
                      else pal.pair("tile_dark", "tile") if pal.has("tile") else pal.grad("regolith"))
        else:
            lo, hi = pal.grad("felt")
        return lo, hi

    def _paint_lane(self, coll):
        for obj in coll.objects:
            base = _base(obj)
            if base in ("surface-grass", "surface-sand"):
                colors.paint_values(obj, self._surface_colours(len(obj.data.vertices), base.split("-")[1]), ao=LANE_AO)
            elif base in ("rail-left", "rail-right"):
                obj["aurago_hidden"] = True

    def _surface_colours(self, count, kind):
        """Felt: world colour with a lighter middle and faint mowing bands; sand: sand colour."""
        lo, hi = self._lane_colours(kind)
        rows = LANE_ROWS + 1
        samples = count // rows
        centre = self.terrain.samples
        arc = [0.0]
        for i in range(1, samples):
            arc.append(arc[-1] + math.hypot(centre[i][0] - centre[i - 1][0], centre[i][1] - centre[i - 1][1]))
        out = []
        for v in range(count):
            i, j = divmod(v, rows)
            s = -1.0 + 2.0 * j / LANE_ROWS
            mid = 1.0 - abs(s)
            c = mix(lo, hi, 0.30 + 0.38 * smoothstep(mid, 0.0, 0.75))
            band = 1.0 + (0.035 if int(arc[min(i, samples - 1)] / STRIPE_LENGTH) % 2 == 0 else -0.035)
            out.append((c[0] * band, c[1] * band, c[2] * band, 1.0))
        return out

    # -- bake -----------------------------------------------------------------

    def bake(self, coll):
        """Bake AO into island, trims and lane; everything else is hidden from or skips the bake."""
        bakeable = [self.island] + list(self.trims)
        passthrough = [self.shore] if self.shore else []
        hidden = []
        for obj in coll.objects:
            base = _base(obj)
            if base.startswith("surface-"):
                bakeable.append(obj)
            elif (base in ("rail-left", "rail-right", "ball-template", "putter-template")
                  or obj.get("aurago_obstacle") or obj.get("aurago_obstacle_component")):
                hidden.append(obj)
        return bake.process(bakeable, passthrough=passthrough, hidden=hidden + passthrough,
                            samples=BAKE_SAMPLES, distance=BAKE_DISTANCE, tint=BAKE_TINT[self.look])
