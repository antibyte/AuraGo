"""Dressing of the legacy courses as pirate, temple and moon dioramas (MIT License).

``courses_v1.build_course(number, course, dresser)`` keeps building the lane,
collision rails, obstacles, tee, cup and flag exactly as before; the ``Dresser``
replaces the old decor around them:

* ``lane_mats``     material set for ``make_surface`` (one shared ``matte``; the
                    colour comes from POINT vertex colours, no shading change)
* ``dress``         island (``dress_island``), ``water-shore`` strip, rail trims
                    (``trim``) and the lane colours
* ``top_height``    ground height under any point (cup floor, placements)
* ``specials``      layout specific parts (``dress_specials``) and the prop
                    placements in Blender coordinates (``placement``);
                    ``runtime_props`` converts them for the manifest
* ``restyle``       world materials and colours for the functional parts
                    (``dress_parts``)
* ``bake``          vertex AO for island, trims, specials, cup parts and lane with
                    the placed kit props as occluders (``occluders``, removed
                    again before the export); templates, obstacles, rails and the
                    shore are hidden from the occluders, the shore and the
                    templates only get ``Col``

Lane meshes (``surface-*``) only receive a POINT colour attribute and the
collision rails (``rail-*``) nothing but ``aurago_hidden``; neither gets a bevel,
smoothing or shading change, so the exporter splits their vertices exactly as
before (guarded by the P1 collider fingerprint). The rails carry no colour: they
are hidden at runtime and COLOR_0 would cost about 30 KB per course.
"""

import copy
import math

from . import bake, colors, dress_island, dress_parts, dress_specials, materials, occluders, palette, placement, shapes, trim
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
        self.waterline = []
        self.shore = None
        self.trims = []
        self.specials_out = None
        self.bake_parts = []
        self.pass_parts = []
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
            self.island, self.waterline = dress_island.build(self.terrain, self.number)
            self.shore = dress_island.build_shore(self.terrain, self.waterline) if self.waterline else None
            self.trims = trim.build_trims(self.look, self.course, path, self.number, self.terrain.top_height)
        finally:
            shapes.set_collection(None)
        self._paint_lane(coll)

    def specials(self, scene, coll, course, path, mats):
        """Layout parts (pier, gate, wheel, ...), then the placement of every prop around them."""
        templates = occluders.templates(self.look)
        shapes.set_collection(coll)
        try:
            self.specials_out = dress_specials.build(self.look, course, path, templates.catalog)
        finally:
            shapes.set_collection(None)
        out = self.specials_out
        lane = [(p[0], p[1]) for i, p in enumerate(path) if i % 3 == 0 or i == len(path) - 1]
        plan = copy.deepcopy(placement.PLANS[self.look])
        plan["scatter"] = plan["scatter"] + out.scatter
        plan["medium"] = [(name, out.medium.get(name, count)) for name, count in plan["medium"]]
        cup = self.terrain.cup
        self.placements = placement.place(self.look, self.number, placement.Field(lane, self.terrain.outline, self.terrain.in_trench), self.terrain.top_height,
                                          templates.catalog, plan, fixed=out.fixed, reserved=out.reserved, cup=(cup[0], cup[1]),
                                          hero=out.hero)
        placement.check(self.placements, lane, templates.catalog)

    def restyle(self, coll):
        """World material and colours for obstacles, templates, flag, hole and tee parts."""
        self.bake_parts, self.pass_parts = dress_parts.restyle(self.look, coll)

    def runtime_props(self):
        """Placements as ``[name, x, y, z, yaw, scale]`` in runtime coordinates, rounded to millimetres."""
        out = []
        for name, x, y, z, yaw, scale in self.placements:
            yaw = math.atan2(math.sin(yaw), math.cos(yaw))
            out.append([name] + [round(v, 3) for v in (x, z, -y, yaw, scale)])
        return out

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
        """Bake AO into the diorama parts with the placed props as occluders; templates, obstacles and shore only get ``Col``."""
        bakeable = [self.island] + list(self.trims) + list(self.specials_out.objects) + list(self.bake_parts)
        passthrough = ([self.shore] if self.shore else []) + list(self.pass_parts)
        hidden = []
        for obj in coll.objects:
            base = _base(obj)
            if base.startswith("surface-"):
                bakeable.append(obj)
            elif (base in ("rail-left", "rail-right", "ball-template", "putter-template")
                  or obj.get("aurago_obstacle") or obj.get("aurago_obstacle_component")):
                hidden.append(obj)
        copies = occluders.templates(self.look).instantiate(coll, self.placements)
        try:
            return bake.process(bakeable, passthrough=passthrough, hidden=hidden + ([self.shore] if self.shore else []),
                                samples=BAKE_SAMPLES, distance=BAKE_DISTANCE, tint=BAKE_TINT[self.look])
        finally:
            occluders.remove(copies)
