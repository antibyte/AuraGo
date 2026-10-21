"""P3 course pipeline: layout, scene, dressing, export, manifest entry (MIT License, see ../LICENSE).

``build_course(number, data, world, out, keep_scene=False)`` turns the data of one course (a dict of
``course_data_<world>``) into ``out/course-NN.glb`` and returns its manifest v2 entry:

1. ``layout.build`` makes the ``Track`` (pieces, route, hazards); data errors name the course and the piece;
2. a new scene gets the floors, collision rails and the collision cup (``build_track``) and the hole marks
   (``cup_parts``);
3. the ``Dresser`` fits the island to the layout, builds terrain, trims and shore, places the hero, the fixed
   props, the medium props and the scatter, restyles the functional parts and bakes the vertex AO;
4. ``export.join_scenery`` merges the scenery per material, the scene is exported, the collision nodes are read
   back (``export.glb_collision``) and the entry is assembled (``course_entry``).

The scene is deleted after the export unless ``keep_scene`` (``--save-blend``); Blender numbers a name that is
still taken, so a course that stays in memory would leak numbered names into the GLBs of later courses.
"""

import hashlib
import os

import bpy

from . import basic_parts, build_track, course_data_moon, course_data_pirate, course_data_temple, cup_parts, export, layout
from .course_entry import entry as manifest_entry
from .dress import Dresser

WORLD_DATA = {"pirate": course_data_pirate, "temple": course_data_temple, "moon": course_data_moon}
MAX_BYTES, MAX_TRIANGLES, MAX_MESH_NODES = 1024 * 1024, 32000, 32


def all_courses():
    """``[(number, data, world)]`` of every course of the data modules, ordered by number."""
    out = [(data["number"], data, world) for world, module in WORLD_DATA.items() for data in module.COURSES]
    out.sort(key=lambda item: item[0])
    numbers = [item[0] for item in out]
    if len(set(numbers)) != len(numbers):
        raise ValueError("course numbers are not unique: %s" % numbers)
    return out


def find_course(number):
    """``(data, world)`` of course ``number``; ``KeyError`` when no data module has it."""
    for n, data, world in all_courses():
        if n == number:
            return data, world
    raise KeyError(number)


def discard_scene(scene, model, home):
    """Delete ``scene`` with its objects, data, collection and every orphan; ``home`` becomes the active scene again."""
    bpy.context.window.scene = home
    for obj in list(model.objects):
        data = obj.data
        bpy.data.objects.remove(obj, do_unlink=True)
        if data is not None and data.users == 0:
            (bpy.data.meshes if isinstance(data, bpy.types.Mesh) else bpy.data.cameras if isinstance(data, bpy.types.Camera) else bpy.data.lights).remove(data)
    bpy.data.collections.remove(model)
    bpy.data.scenes.remove(scene)
    bpy.data.orphans_purge(do_local_ids=True, do_linked_ids=True, do_recursive=True)


def check_budget(course_id, size, triangles, mesh_nodes):
    """Raise when the GLB exceeds a runtime budget (1 MiB, 32,000 triangles, 32 mesh nodes)."""
    problems = []
    if size > MAX_BYTES:
        problems.append("%d bytes (limit %d)" % (size, MAX_BYTES))
    if triangles > MAX_TRIANGLES:
        problems.append("%d triangles (limit %d)" % (triangles, MAX_TRIANGLES))
    if mesh_nodes > MAX_MESH_NODES:
        problems.append("%d mesh nodes (limit %d)" % (mesh_nodes, MAX_MESH_NODES))
    if problems:
        raise RuntimeError("%s is over budget: %s" % (course_id, ", ".join(problems)))


def build_course(number, data, world, out, keep_scene=False):
    """Build, export and describe one course; see the module docstring."""
    data = dict(data, number=number)
    track = layout.build(data, world)
    course_id = "%s-%02d" % (world, number)
    home = bpy.context.window.scene
    scene = bpy.data.scenes.new("%02d - %s" % (number, data.get("name", course_id)))
    scene["license"] = "MIT"
    scene["aurago_course_id"] = course_id
    scene["authoring"] = "AuraGo original Blender asset"
    model = basic_parts.link_collection(scene, "Course Model")
    # The dressing builds modifier-finished meshes and bakes, which both need this scene to be the active one.
    bpy.context.window.scene = scene
    dresser = Dresser(number, data, track)
    built = build_track.build(track, scene, model)
    cup_parts.hole_marks(track, scene, model, number)
    dresser.dress(model, built)
    dresser.place()
    dresser.restyle(model)
    dresser.bake(model)
    export.join_scenery(model)
    bpy.context.window.scene = scene
    bpy.ops.object.select_all(action="DESELECT")
    for obj in model.objects:
        obj.select_set(True)
    bpy.context.view_layer.objects.active = next((obj for obj in model.objects if obj.type == "MESH"), None)
    os.makedirs(out, exist_ok=True)
    glb = os.path.join(out, "course-%02d.glb" % number)
    export.export_glb(glb, model.objects)
    collision = export.glb_collision(glb)
    with open(glb, "rb") as f:
        blob = f.read()
    triangles = export.glb_triangles(glb)
    doc = export.glb_json(glb)
    mesh_nodes = sum(1 for node in doc["nodes"] if "mesh" in node)
    check_budget(course_id, len(blob), triangles, mesh_nodes)
    result = manifest_entry(number, data, track, dresser, os.path.basename(glb), len(blob), hashlib.sha256(blob).hexdigest(), triangles,
                            collision, dresser.outline_box())
    print("%s: %d bytes, %d triangles, %d mesh nodes, collision %d nodes %d triangles digest %s, %d props" %
          (course_id, len(blob), triangles, mesh_nodes, collision["nodes"], collision["triangles"], collision["digest"][:12], len(result["props"])))
    if not keep_scene:
        discard_scene(scene, model, home)
    return result
