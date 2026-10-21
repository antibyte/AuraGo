"""Headless selftest of the track kit core (MIT License, see ../LICENSE).

Builds a probe course (tee, straight, curve, bank, ramp, hill, steps, straight, green), checks that
the pieces share their seam objects, builds the Blender floor and rail nodes, exports a GLB, reads the
collision back with ``export.glb_collision`` and checks that no floor triangle reaches the cup radius.
Writes ``selftest.json`` (digest, counts, GLB hash) next to the GLB and prints ``selftest ok``.

Run from the repository root:
  blender --background --factory-startup --python assets/minigolf/kit/track_selftest.py -- --out disposable/_p3/selftest
The pure checks (``probe_course``, ``check_seams``) also run without Blender.
"""

import argparse
import hashlib
import json
import math
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))

from kit import layout  # noqa: E402
from kit.track import CUP_RADIUS, ROWS  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", "..", ".."))


def probe_course():
    """Probe course data (the design's data shape) in the pirate world."""
    return {"number": 1, "par": 4, "name": "Selftest", "start": {"at": (0.0, 0.0, 0.0), "heading": 0},
            "pieces": [("tee", {"length": 1.0}),
                       ("straight", {"length": 3.0}),
                       ("curve", {"radius": 3.0, "angle": 90}),
                       ("bank", {"radius": 4.0, "angle": -60, "bank": 20}),
                       ("ramp", {"length": 3.0, "rise": 0.4}),
                       ("hill", {"length": 3.0, "height": -0.25}),
                       ("steps", {"count": 3, "drop": 0.2, "tread": 0.8}),
                       ("straight", {"length": 2.0}),
                       ("green", {"length": 3.0, "width": 3.44, "cup": (1.8, 0.4)})],
            "hero": None, "props": [], "mechanics": [], "obstacles": [], "cycle": None}


def check_seams(track):
    """Every strip starts with the object that ended its predecessor; every distinct point value is ONE tuple object.

    Returns the number of distinct points the floor uses (raster points inside a hole have no face).
    """
    ends = [strip.sections[-1] for strip in track.strips]
    exits = [port["section"] for pad in track.pads for port in pad.ports if port["side"] == "exit"]
    exits += [hazard["landing"] for hazard in track.hazards if "landing" in hazard]    # a gap starts a fresh strip
    for k, strip in enumerate(track.strips):
        if not any(strip.sections[0] is section for section in ends[:k] + exits):
            raise AssertionError("strip %d (%s) does not start with the previous section object" % (k, strip.kind))
    for pad in track.pads:
        for port in pad.ports:
            c0 = port["cols"][0]
            row = pad.grid["points"][0 if port["side"] == "entry" else pad.grid["nx"]]
            for k in range(ROWS + 1):
                if row[c0 + k] is not port["section"].points[k]:
                    raise AssertionError("pad %s %s port point %d is not the section's tuple" % (pad.kind, port["side"], k))
    by_value = {}
    for strip in track.strips:
        for section in strip.sections:
            for point in section.points:
                by_value.setdefault(point, set()).add(id(point))
    used = set(by_value)
    for pad in track.pads:
        points = pad.grid["points"]
        for row in points:
            for point in row:
                by_value.setdefault(point, set()).add(id(point))
        for i, j in pad.faces:
            used.update((points[i][j], points[i][j + 1], points[i + 1][j], points[i + 1][j + 1]))
    objects = sum(len(ids) for ids in by_value.values())
    if objects != len(by_value):
        raise AssertionError("%d point tuples carry only %d distinct values: a seam was recomputed" % (objects, len(by_value)))
    return len(used)


def _distance_to_triangle(p, a, b, c):
    """Planar distance of point ``p`` to the triangle ``abc`` (0 inside)."""
    def side(u, v, w):
        return (u[0] - w[0]) * (v[1] - w[1]) - (v[0] - w[0]) * (u[1] - w[1])
    d1, d2, d3 = side(p, a, b), side(p, b, c), side(p, c, a)
    if not ((d1 < 0 or d2 < 0 or d3 < 0) and (d1 > 0 or d2 > 0 or d3 > 0)):
        return 0.0

    def segment(u, v):
        ex, ey = v[0] - u[0], v[1] - u[1]
        t = max(0.0, min(1.0, ((p[0] - u[0]) * ex + (p[1] - u[1]) * ey) / (ex * ex + ey * ey))) if ex or ey else 0.0
        return math.hypot(p[0] - u[0] - t * ex, p[1] - u[1] - t * ey)
    return min(segment(a, b), segment(b, c), segment(c, a))


def run(out_dir):
    import bpy

    from kit import basic_parts, bake, build_track, export

    track = layout.build(probe_course(), "pirate")
    distinct = check_seams(track)
    scene = bpy.data.scenes.new("track-selftest")
    bpy.context.window.scene = scene
    model = basic_parts.link_collection(scene, "Course Model")
    built = build_track.build(track, scene, model)
    for obj in built["floors"]:
        bake.commit_colours(obj)
    meshes = {obj.name: obj for obj in built["floors"] + built["rails"]}
    welded = sum(len(obj.data.vertices) for obj in built["floors"])
    if welded != distinct:
        raise AssertionError("floor meshes have %d vertices, expected the %d distinct points" % (welded, distinct))
    bpy.ops.object.select_all(action="DESELECT")
    for obj in model.objects:
        obj.select_set(True)
    bpy.context.view_layer.objects.active = built["floors"][0]
    os.makedirs(out_dir, exist_ok=True)
    glb = os.path.join(out_dir, "selftest.glb")
    export.export_glb(glb, model.objects)
    nodes = {node["name"]: node for node in export.glb_json(glb)["nodes"]}
    for name, surface, hidden in (("floor-felt", "grass", False), ("rail-wood", "wood", True)):
        node = nodes.get(name)
        extras = (node or {}).get("extras", {})
        if node is None or extras.get("aurago_collider") != surface or bool(extras.get("aurago_hidden")) != hidden:
            raise AssertionError("node %s is missing or its extras are wrong: %s" % (name, extras))
        if any(key in node for key in ("matrix", "translation", "rotation", "scale", "children")):
            raise AssertionError("collision node %s must have the identity transform" % name)
    result = export.glb_collision(glb)
    surfaces = [group["surface"] for group in result["groups"]]
    if surfaces != ["grass", "wood"] or result["nodes"] != len(meshes):
        raise AssertionError("unexpected collision groups %s with %d nodes" % (surfaces, result["nodes"]))
    cup = basic_parts.runtime(track.cup)
    grass = result["groups"][0]
    vertices, indices = grass["vertices"], grass["indices"]
    nearest = min(_distance_to_triangle((cup[0], cup[2]), *[(vertices[3 * i], vertices[3 * i + 2]) for i in indices[k:k + 3]])
                  for k in range(0, len(indices), 3))
    if nearest < CUP_RADIUS - 1e-6:
        raise AssertionError("a floor triangle reaches %.3f m from the cup centre (radius %.2f)" % (nearest, CUP_RADIUS))
    data = open(glb, "rb").read()
    floor_triangles = sum(len(obj.data.polygons) for obj in built["floors"])
    summary = {"digest": result["digest"], "nodes": result["nodes"], "triangles": result["triangles"],
               "groups": {g["surface"]: {"vertices": len(g["vertices"]) // 3, "triangles": len(g["indices"]) // 3} for g in result["groups"]},
               "floor_triangles": floor_triangles, "rail_triangles": sum(len(obj.data.polygons) for obj in built["rails"]),
               "cup_clearance": round(nearest, 4), "length": round(track.length, 3), "glb_sha256": hashlib.sha256(data).hexdigest(),
               "glb_bytes": len(data)}
    with open(os.path.join(out_dir, "selftest.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(summary, f, indent=2, sort_keys=True)
        f.write("\n")
    print("track: %d strips, %d pads, %.2f m, %d distinct points" % (len(track.strips), len(track.pads), track.length, distinct))
    print("collision: %d nodes, %d triangles (floor %d + rails %d), cup clearance %.3f m" %
          (result["nodes"], result["triangles"], summary["floor_triangles"], summary["rail_triangles"], nearest))
    print("digest %s" % result["digest"])
    print("glb %d bytes sha256 %s" % (len(data), summary["glb_sha256"]))
    print("selftest ok")


def main():
    argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else sys.argv[1:]
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=os.path.join(ROOT, "disposable", "_p3", "selftest"))
    args = ap.parse_args(argv)
    run(os.path.abspath(args.out))


if __name__ == "__main__":
    main()
