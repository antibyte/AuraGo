"""Dressing of a P3 course as a pirate, temple or moon diorama (MIT License, see ../LICENSE).

``courses.build_course`` builds the floors, rails and cup of the track (``build_track``) and the hole marks
(``cup_parts``); the ``Dresser`` replaces everything around them with the diorama:

* ``dress``       island outline from the layout (``footprint``), terrain and island mesh (``terrain``,
                  ``dress_island``), the ``water-shore`` strip and the rail trims (``trim``, from the rail runs)
* ``build_structures``  supports, bridge parts, tunnel roofs and portals, pond water, trench walls (``structures``)
* ``place``       the hero and the fixed props of the course data, then medium props and scatter
                  (``placement``); ``runtime_props`` converts them for the manifest
* ``restyle``     world materials and colours for the functional parts (``dress_parts``)
* ``bake``        vertex AO for island, trims, floors, cup parts and extras with the placed kit props as occluders
                  (``occluders``, removed again before the export); templates, obstacles, rails and the shore are
                  hidden from the occluders, the shore and the templates only get ``Col``

``Dresser.extra`` and ``Dresser.pass_parts`` take further objects (supports, portals, mechanic and obstacle
parts) that are baked with the island or only coloured.

Floors (``floor-*``) and collision rails (``rail-*``) are built by ``build_track``; the rails carry no colour
(they are hidden at runtime and ``COLOR_0`` would cost about 30 KB per course).
"""

import copy
import math

from . import bake, dress_island, dress_parts, footprint, occluders, palette, placement, shapes, structures, trim
from . import detail as detail_mod
from .terrain import Terrain

BAKE_TINT = {"pirate": (0.10, 0.10, 0.22), "temple": (0.16, 0.08, 0.10), "moon": (0.14, 0.12, 0.30)}
BAKE_SAMPLES, BAKE_DISTANCE = 160, 1.5
RAIL_NODES = ("rail-wood", "rail-stone", "rail-metal")


def _base(obj):
    return obj.name.split(".")[0]


class Dresser:
    """Per-course dressing state; build with ``Dresser(number, data, track)``."""

    def __init__(self, number, data, track):
        self.number, self.data, self.track = number, data, track
        self.look = track.world
        self.pal = palette.get(self.look)
        self.templates = occluders.templates(self.look)
        self.catalog = self.templates.catalog
        self.where = "course %d: " % number
        self.detail = detail_mod.resolve(data)
        self.hero, self.props = self._data_placements()
        self.terrain = None
        self.island = None
        self.waterline = []
        self.shore = None
        self.trims = []
        self.extra = []
        self.ponds = []
        self.reserved = []
        self.bake_parts = []
        self.pass_parts = []
        self.movers = []            # moving nodes (obstacles, spinning rings): hidden from the ambient occlusion bake
        self.placements = []

    # -- course data ------------------------------------------------------------

    def _data_placements(self):
        """``(hero, props)`` from the course data: yaws in radians, names checked against the kit."""
        hero = self.data.get("hero")
        if hero:
            hero = {"name": hero.get("name") or placement.PLANS[self.look]["hero"], "at": tuple(hero["at"]),
                    "yaw": math.radians(hero.get("yaw", 0.0))}
            self._known(hero["name"], "hero")
        props = []
        for entry in self.data.get("props", []):
            name, (x, y), yaw, scale = entry
            self._known(name, "prop")
            props.append((name, float(x), float(y), math.radians(yaw), float(scale)))
        return hero, props

    def _known(self, name, what):
        if name not in self.catalog:
            raise ValueError("%sthe %s %r is not a prop of the %s kit (known: %s)" %
                             (self.where, what, name, self.look, ", ".join(sorted(self.catalog))))

    def footprint(self):
        """Plan points that the island must contain: all floors, the hero circle and the fixed props."""
        points = footprint.floor_points(self.track)
        circles = ([(self.hero["at"], self.catalog[self.hero["name"]]["radius"])] if self.hero else [])
        circles += [((x, y), self.catalog[name]["radius"] * scale) for name, x, y, _, scale in self.props]
        for (x, y), radius in circles:
            points.extend(footprint.circle_points(x, y, radius))
        points.extend(structures.footprint_points(self.track))
        return points

    # -- island, shore and trims ---------------------------------------------------

    def dress(self, coll, built):
        """Build island, shore and trims into ``coll`` (``built`` is the result of ``build_track.build``)."""
        outline = footprint.fit_outline(self.footprint(), self.number, label=self.where)
        shapes.set_collection(coll)
        try:
            self.terrain = Terrain(self.look, self.number, self.track, outline)
            self.island, self.waterline = dress_island.build(self.terrain, self.number, self.detail["island_quads"])
            self.shore = dress_island.build_shore(self.terrain, self.waterline) if self.waterline else None
            self.trims = trim.build_trims(self.look, built["runs"], self.number, self.track.corridors, self.terrain.top_height,
                                         self.detail["trim_facets"])
        finally:
            shapes.set_collection(None)

    def build_structures(self, coll, built):
        """Supports, bridge parts, tunnels and pond water (``structures``); call after ``dress`` and before ``place``.

        ``built`` is the result of ``build_track.build`` (its rail runs keep the supports clear of other lanes' rails)."""
        shapes.set_collection(coll)
        try:
            built = structures.build(self.track, self.terrain, self.look, self.number, built["runs"])
        finally:
            shapes.set_collection(None)
        self.extra.extend(built.baked)
        self.ponds = built.water
        self.reserved = built.reserved

    def top_height(self, x, y):
        return self.terrain.top_height(x, y)

    # -- placement ---------------------------------------------------------------------

    def place(self, fixed=(), reserved=()):
        """Hero and fixed props first, then the medium props and the scatter around them."""
        field = placement.Field(self.track.corridors, self.terrain.outline, self.terrain.wet, footprint.floor_points(self.track))
        plan = copy.deepcopy(placement.PLANS[self.look])
        cup = self.track.cup
        try:
            self.placements = placement.place(self.look, self.number, field, self.terrain.top_height, self.catalog, plan, fixed=fixed,
                                              reserved=tuple(reserved) + tuple(self.reserved), cup=(cup[0], cup[1]), hero=self.hero, props=self.props)
            placement.check(self.placements, field, self.catalog)
        except (ValueError, RuntimeError) as exc:
            raise type(exc)(self.where + str(exc)) from None

    def restyle(self, coll):
        """World material and colours for obstacles, templates, flag, hole and tee parts."""
        self.bake_parts, self.pass_parts = dress_parts.restyle(self.look, coll, cup_patch=False)

    def runtime_props(self):
        """Placements as ``[name, x, y, z, yaw, scale]`` in runtime coordinates, rounded to millimetres."""
        out = []
        for name, x, y, z, yaw, scale in self.placements:
            yaw = math.atan2(math.sin(yaw), math.cos(yaw))
            out.append([name] + [round(v, 3) for v in (x, z, -y, yaw, scale)])
        return out

    def runtime_island(self):
        """Island outline in runtime coordinates: ``{center: [x, z], half: [hx, hz], power, wobble}``.

        The rim is a superellipse around ``center`` (``|dx/hx|^power + |dz/hz|^power = 1``) whose
        radius is scaled by at most ``1 +/- wobble`` along the angle; ``wobble`` is the largest
        deviation of the true outline (measured, rounded up to 1/1000).
        """
        outline = self.terrain.outline
        return {"center": [round(outline.center.x, 3), round(-outline.center.y, 3)],
                "half": [round(outline.half[0], 3), round(outline.half[1], 3)],
                "power": round(outline.power, 3),
                "wobble": math.ceil(outline.deviation() * 1000.0) / 1000.0}

    def outline_box(self, samples=720):
        """Bounding box ``(min_x, min_y, max_x, max_y)`` (Blender coordinates) of the true wobbled outline."""
        outline = self.terrain.outline
        points = [(outline.center.x + outline.radius(math.tau * k / samples) * math.cos(math.tau * k / samples),
                   outline.center.y + outline.radius(math.tau * k / samples) * math.sin(math.tau * k / samples)) for k in range(samples)]
        return (min(p[0] for p in points), min(p[1] for p in points), max(p[0] for p in points), max(p[1] for p in points))

    # -- bake ----------------------------------------------------------------------------

    def bake(self, coll):
        """Bake AO into the diorama parts with the placed props as occluders; templates, obstacles and shore only get ``Col``."""
        bakeable = [self.island] + list(self.trims) + list(self.extra) + list(self.bake_parts)
        passthrough = ([self.shore] if self.shore else []) + list(self.ponds) + list(self.pass_parts)
        hidden = list(self.movers)
        for obj in coll.objects:
            base = _base(obj)
            if base.startswith("floor-"):
                bakeable.append(obj)
            elif base in RAIL_NODES or base in ("ball-template", "putter-template") or obj.get("aurago_obstacle") or obj.get("aurago_obstacle_component"):
                hidden.append(obj)
        copies = self.templates.instantiate(coll, self.placements)
        try:
            return bake.process(bakeable, passthrough=passthrough, hidden=hidden + ([self.shore] if self.shore else []) + list(self.ponds),
                                samples=BAKE_SAMPLES, distance=BAKE_DISTANCE, tint=BAKE_TINT[self.look])
        finally:
            occluders.remove(copies)
