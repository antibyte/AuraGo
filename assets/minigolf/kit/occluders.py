"""Kit props as ambient-occlusion occluders of a course (MIT License, see ../LICENSE).

The runtime draws the placed props as instances of the world kit; the course GLB
does not contain them. To still get contact shading on the island, the lane and
the trims, every placement is put into the course scene as a linked copy of the
kit prop while the AO is baked, and removed again before the export.

``Templates`` builds the (unbaked) kit objects of one look once per process in a
scene of its own and answers the placement catalog (radius, height, tags) with
the same measurement the kit export writes into the manifest.
"""

import importlib

import bpy

from . import kits, shapes

_CACHE = {}


class Templates:
    """The props of one look as unbaked template objects."""

    def __init__(self, look):
        self.look = look
        module = importlib.import_module(kits.MODULES[look])
        keep = bpy.context.window.scene
        self.scene = bpy.data.scenes.new("occluders-" + look)
        coll = bpy.data.collections.new("occluders-" + look)
        self.scene.collection.children.link(coll)
        bpy.context.window.scene = self.scene
        shapes.set_collection(coll)
        try:
            self.objects = {name: module.PROPS[name](seed=1) for name in sorted(module.PROPS)}
            for name, obj in self.objects.items():
                if "hero" in module.PROP_META.get(name, {}).get("tags", []):
                    kits.centre_footprint(obj)
        finally:
            shapes.set_collection(None)
            bpy.context.window.scene = keep
        self.catalog = {}
        for name, obj in self.objects.items():
            radius, height = kits._measure(obj)
            info = module.PROP_META.get(name, {})
            self.catalog[name] = {"radius": radius, "height": height, "tags": sorted(info.get("tags", [])),
                                  "spin": "spin" in info}

    def instantiate(self, collection, placements):
        """Link a copy of the prop for every ``(name, x, y, z, yaw, scale)`` into ``collection``; returns the copies."""
        copies = []
        for name, x, y, z, yaw, scale in placements:
            copy = self.objects[name].copy()
            copy.location = (x, y, z)
            copy.rotation_euler = (0.0, 0.0, yaw)
            copy.scale = (scale, scale, scale)
            copy.hide_render = False
            collection.objects.link(copy)
            copies.append(copy)
        return copies


def templates(look):
    """The cached templates of ``look`` (rebuilt when a factory reset removed them)."""
    cached = _CACHE.get(look)
    try:
        alive = cached is not None and cached.scene.name in bpy.data.scenes
    except ReferenceError:
        alive = False
    if not alive:
        _CACHE[look] = Templates(look)
    return _CACHE[look]


def remove(copies):
    """Delete the linked copies (the shared meshes stay with the templates)."""
    for copy in copies:
        bpy.data.objects.remove(copy, do_unlink=True)
