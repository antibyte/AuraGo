"""Generate AuraGo's original MIT-licensed mini golf assets with Blender 5.2.

Run from the repository root:
  blender --background --factory-startup --python assets/minigolf/generate.py -- [--out DIR] [--only kits,courses,previews|all] [--course N] [--worlds W] [--save-blend]

The courses are those of the ``kit/course_data_<world>.py`` modules, built with the track kit (``kit/courses.py``);
the manifest is version 2. ``--out`` defaults to the runtime folder ``ui/3d/minigolf``; kits and previews are
written there too. ``--set p3`` is accepted for old command lines (it is the only course set).
"""

import argparse
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import bpy  # noqa: E402

from kit import kits  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
RUNTIME = os.path.join(ROOT, "ui", "3d", "minigolf")
BUILD = os.path.join(HERE, "build")
PARTS = ("kits", "courses", "previews")
MANIFEST_ORDER = ("version", "courses", "kits", "worlds")
WORLD_ORDER = ("pirate", "temple", "moon")
COURSE_COUNT = 18


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
    ap = argparse.ArgumentParser()
    ap.add_argument("--set", dest="course_set", choices=("p3",), default="p3", help="course set (p3 is the only one)")
    ap.add_argument("--out", default=None, help="output folder for GLBs, manifest and previews (default ui/3d/minigolf)")
    ap.add_argument("--only", default=None, help="kits, courses, previews or all (default kits,courses; with --course: courses)")
    ap.add_argument("--course", type=int, default=0, help="rebuild one course (1-18) and patch its manifest entry")
    ap.add_argument("--worlds", default="all", help="worlds whose menu preview to render: pirate, temple, moon (comma separated) or all")
    ap.add_argument("--save-blend", action="store_true")
    args = ap.parse_args(argv)
    args.out = os.path.abspath(args.out) if args.out else RUNTIME
    if args.only is None:
        args.only = "courses" if args.course else "kits,courses"
    parts = [part.strip() for part in args.only.split(",") if part.strip()]
    if "all" in parts:
        parts = list(PARTS)
    unknown = [part for part in parts if part not in PARTS]
    if unknown or not parts:
        ap.error("--only takes kits, courses, previews or all (got %s)" % args.only)
    if args.course and not 1 <= args.course <= COURSE_COUNT:
        ap.error("--course takes a number from 1 to %d" % COURSE_COUNT)
    if args.course and "courses" not in parts:
        ap.error("--course needs the courses part")
    if args.save_blend and "courses" in parts and not args.course:
        ap.error("--save-blend keeps the course scenes in memory: use it with --course N (a full run deletes every course "
                 "after its export, otherwise later courses would get numbered node names)")
    worlds = [w.strip() for w in args.worlds.split(",") if w.strip()]
    args.worlds = list(WORLD_ORDER) if "all" in worlds else worlds
    if not args.worlds or any(world not in WORLD_ORDER for world in args.worlds):
        ap.error("--worlds takes pirate, temple, moon or all (got %s)" % args.worlds)
    args.parts = parts
    return args


def read_manifest(out):
    path = os.path.join(out, "manifest.json")
    if not os.path.exists(path):
        return {}
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def write_manifest(out, manifest):
    ordered = {key: manifest[key] for key in MANIFEST_ORDER if key in manifest}
    ordered.update({key: value for key, value in manifest.items() if key not in ordered})
    with open(os.path.join(out, "manifest.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(ordered, f, separators=(",", ":"))
        f.write("\n")


def fresh_scene():
    """Start from an empty file so every kit and the courses build in a clean datablock space."""
    bpy.ops.wm.read_factory_settings(use_empty=True)


def clear_scene():
    """Delete the objects of the active scene and every datablock nothing uses (the factory cube, kit leftovers).

    A full run and a single ``--course N`` start from the same state, so both write the same GLBs.
    """
    bpy.ops.object.select_all(action="SELECT")
    bpy.ops.object.delete(use_global=False)
    bpy.data.orphans_purge(do_local_ids=True, do_linked_ids=True, do_recursive=True)


def build_kits(out):
    entries = {}
    for look in kits.MODULES:
        fresh_scene()
        entries[look], _ = kits.build_kit(look, out)
    return entries


# -- courses ------------------------------------------------------------------------------------

def world_entries(courses):
    """Menu worlds (``{id, preview}``) in the order pirate, temple, moon, for the worlds that have a course."""
    present = {course["world"] for course in courses}
    return [{"id": world, "preview": "previews/%s.webp" % world} for world in WORLD_ORDER if world in present]


def build_courses(out, only=0, keep_scene=False, existing=()):
    """Build every course of the data modules (or only course ``only``, patched into ``existing``); returns the entries."""
    from kit import courses
    todo = courses.all_courses()
    if only:
        todo = [item for item in todo if item[0] == only]
        if not todo:
            raise SystemExit("--course %d has no data: the data modules hold courses %s" % (only, [n for n, _, _ in courses.all_courses()]))
    clear_scene()
    built = {n: courses.build_course(n, data, world, out, keep_scene=keep_scene) for n, data, world in todo}
    merged = {course["number"]: course for course in existing} if only else {}
    merged.update(built)
    return [merged[n] for n in sorted(merged)]


def main():
    args = parse_args()
    out = args.out
    os.makedirs(out, exist_ok=True)
    manifest = read_manifest(out)
    if manifest.get("version") != 2:
        manifest = {}               # an old manifest of another version is not patched
    manifest["version"] = 2
    if "kits" in args.parts:
        manifest["kits"] = build_kits(out)
        write_manifest(out, manifest)
    if "courses" in args.parts:
        manifest["courses"] = build_courses(out, args.course, args.save_blend, manifest.get("courses", ()))
        manifest["worlds"] = world_entries(manifest["courses"])
        write_manifest(out, manifest)
        print("Patched course %d in %s" % (args.course, out) if args.course else "Wrote %d courses to %s" % (len(manifest["courses"]), out))
    if "previews" in args.parts:
        from kit import previews
        for world in args.worlds:
            number = previews.COURSES[world]
            size, quality = previews.render(world, number, os.path.join(out, "previews", world + ".webp"))
            print(f"preview {world}: course {number}, {size} bytes, quality {quality}")
    if args.save_blend:
        os.makedirs(BUILD, exist_ok=True)
        bpy.context.preferences.filepaths.save_version = 0
        bpy.ops.wm.save_as_mainfile(filepath=os.path.join(BUILD, "aura-golf.blend"))


if __name__ == "__main__":
    main()
