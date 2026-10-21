"""Headless selftest of the track kit (MIT License, see ../LICENSE).

Two probe courses (``track_checks``) are built, exported and read back:

* ``selftest`` (pirate): tee, straight, curve, bank, ramp, hill, steps, straight, a jump (ramp with a lip,
  a water gap, a flat landing) and a green;
* ``selftest-moon`` (moon): half-pipe, loop (R 1.3, offset 2.2) behind a boost pad, a fork with a straight
  and a hill branch, and a funnel with the hole in its centre.

For each course the pieces must share their seam objects, the Blender floor and rail nodes are exported as one
GLB, ``export.glb_collision`` reads the collision back (groups, identity transforms, digest) and no floor
triangle may reach the cup radius. Building a course twice must give the same geometry. ``<name>.json`` next
to each GLB holds digest, counts, GLB hash and the runtime-coordinate data of the physics probe. Prints
``selftest ok``.

Run from the repository root:
  blender --background --factory-startup --python assets/minigolf/kit/track_selftest.py -- --out disposable/_p3/selftest
The pure checks in ``track_checks`` also run without Blender.
"""

import argparse
import hashlib
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))

from kit import layout  # noqa: E402
from kit.track import CUP_RADIUS  # noqa: E402
from kit.track_checks import check_geometry, check_rules, check_seams, probe_course, probe_course_moon, signature, _distance_to_triangle  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", "..", ".."))

SPECS = (
    {"name": "selftest", "course": probe_course, "world": "pirate", "floors": (("floor-felt", "grass"),), "rail": ("rail-wood", "wood")},
    {"name": "selftest-moon", "course": probe_course_moon, "world": "moon",
     "floors": (("floor-felt", "grass"), ("floor-metal", "grass")), "rail": ("rail-metal", "metal")},
)




def discard_scene(scene, collection):
    """Delete a probe scene with its objects and meshes so the next course gets unnumbered node names."""
    import bpy

    for obj in list(collection.objects):
        data = obj.data
        bpy.data.objects.remove(obj, do_unlink=True)
        if data is not None and data.users == 0:
            bpy.data.meshes.remove(data)
    bpy.data.collections.remove(collection)
    bpy.data.scenes.remove(scene)
    bpy.data.orphans_purge(do_local_ids=True, do_linked_ids=True, do_recursive=True)


def run_course(spec, out_dir):
    import bpy

    from kit import basic_parts, bake, build_track, export

    name, world = spec["name"], spec["world"]
    track = layout.build(spec["course"](), world)
    if signature(track) != signature(layout.build(spec["course"](), world)):
        raise AssertionError("%s: building the track twice gives different geometry" % name)
    distinct = check_seams(track)
    check_geometry(track, name)
    scene = bpy.data.scenes.new("track-" + name)
    bpy.context.window.scene = scene
    model = basic_parts.link_collection(scene, "Course Model")
    built = build_track.build(track, scene, model)
    for obj in built["floors"]:
        bake.commit_colours(obj)
    meshes = {obj.name: obj for obj in built["floors"] + built["rails"]}
    welded = sum(len(obj.data.vertices) for obj in built["floors"])
    if welded != distinct:
        raise AssertionError("%s: floor meshes have %d vertices, expected the %d distinct points" % (name, welded, distinct))
    bpy.ops.object.select_all(action="DESELECT")
    for obj in model.objects:
        obj.select_set(True)
    bpy.context.view_layer.objects.active = built["floors"][0]
    os.makedirs(out_dir, exist_ok=True)
    glb = os.path.join(out_dir, name + ".glb")
    export.export_glb(glb, model.objects)
    nodes = {node["name"]: node for node in export.glb_json(glb)["nodes"]}
    expected = [(floor, surface, False) for floor, surface in spec["floors"]] + [spec["rail"] + (True,)]
    if sorted(nodes) != sorted(n for n, _, _ in expected):
        raise AssertionError("%s: unexpected nodes %s" % (name, sorted(nodes)))
    for node_name, surface, hidden in expected:
        node = nodes[node_name]
        extras = node.get("extras", {})
        if extras.get("aurago_collider") != surface or bool(extras.get("aurago_hidden")) != hidden:
            raise AssertionError("node %s: extras are wrong: %s" % (node_name, extras))
        if any(key in node for key in ("matrix", "translation", "rotation", "scale", "children")):
            raise AssertionError("collision node %s must have the identity transform" % node_name)
    result = export.glb_collision(glb)
    surfaces = [group["surface"] for group in result["groups"]]
    if surfaces != ["grass", spec["rail"][1]] or result["nodes"] != len(meshes):
        raise AssertionError("%s: unexpected collision groups %s with %d nodes" % (name, surfaces, result["nodes"]))
    cup = basic_parts.runtime(track.cup)
    grass = result["groups"][0]
    vertices, indices = grass["vertices"], grass["indices"]
    nearest = min(_distance_to_triangle((cup[0], cup[2]), *[(vertices[3 * i], vertices[3 * i + 2]) for i in indices[k:k + 3]])
                  for k in range(0, len(indices), 3))
    if nearest < CUP_RADIUS - 1e-6:
        raise AssertionError("%s: a floor triangle reaches %.3f m from the cup centre (radius %.2f)" % (name, nearest, CUP_RADIUS))
    data = open(glb, "rb").read()
    to_runtime = lambda p: [round(v, 6) for v in basic_parts.runtime(p)]
    summary = {"digest": result["digest"], "nodes": result["nodes"], "triangles": result["triangles"],
               "groups": {g["surface"]: {"vertices": len(g["vertices"]) // 3, "triangles": len(g["indices"]) // 3} for g in result["groups"]},
               "floor_triangles": sum(len(obj.data.polygons) for obj in built["floors"]),
               "rail_triangles": sum(len(obj.data.polygons) for obj in built["rails"]),
               "cup_clearance": round(nearest, 4), "length": round(track.length, 3), "glb_sha256": hashlib.sha256(data).hexdigest(),
               "glb_bytes": len(data), "signature": signature(track),
               "probe": {"tee": to_runtime(track.tee), "cup": to_runtime(track.cup), "gravity": track.gravity,
                         "mechanics": track.mechanics, "anchors": {k: to_runtime(v[:3]) for k, v in sorted(track.anchors.items())}}}
    with open(os.path.join(out_dir, name + ".json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(summary, f, indent=2, sort_keys=True)
        f.write("\n")
    print("%s: %d strips, %d pads, %d corridors, %.2f m, %d distinct points" %
          (name, len(track.strips), len(track.pads), len(track.corridors), track.length, distinct))
    print("%s collision: %d nodes, %d triangles (floor %d + rails %d), cup clearance %.3f m" %
          (name, result["nodes"], result["triangles"], summary["floor_triangles"], summary["rail_triangles"], nearest))
    print("%s digest %s" % (name, result["digest"]))
    print("%s glb %d bytes sha256 %s" % (name, len(data), summary["glb_sha256"]))
    discard_scene(scene, model)


def run(out_dir):
    check_rules()
    for spec in SPECS:
        run_course(spec, out_dir)
    print("selftest ok")


def main():
    argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else sys.argv[1:]
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=os.path.join(ROOT, "disposable", "_p3", "selftest"))
    args = ap.parse_args(argv)
    run(os.path.abspath(args.out))


if __name__ == "__main__":
    main()
