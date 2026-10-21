"""Generate AuraGo's original MIT-licensed mini golf assets with Blender 5.2.

Run from the repository root:
  blender --background --factory-startup --python assets/minigolf/generate.py -- [--set v1|p3] [--out DIR] [--only kits,courses,previews|all] [--course N] [--save-blend]

``--set p3`` builds the courses of the ``kit/course_data_<world>.py`` modules with the track kit (``kit/courses.py``)
and writes manifest version 2; it needs ``--out`` until the switch-over makes the runtime folder the default.
``--set v1`` is the legacy build of the 18 P2 courses (frozen: the dressing modules now belong to the P3 pipeline,
so only ``--only kits`` is expected to keep working there).
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
MANIFEST_ORDER = ("version", "ballRadius", "courses", "kits", "worlds")
WORLD_ORDER = ("pirate", "temple", "moon")
COURSE_COUNT = 18


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
    ap = argparse.ArgumentParser()
    ap.add_argument("--set", dest="course_set", choices=("v1", "p3"), default="v1", help="v1 = legacy P2 courses, p3 = track kit courses (default v1)")
    ap.add_argument("--out", default=None, help="output folder for GLBs and manifest (default ui/3d/minigolf; required for --set p3)")
    ap.add_argument("--only", default=None, help="kits, courses, previews or all (default kits,courses; with --course: courses)")
    ap.add_argument("--course", type=int, default=0, help="rebuild one course (1-18) and patch its manifest entry")
    ap.add_argument("--save-blend", action="store_true")
    args = ap.parse_args(argv)
    if args.course_set == "p3" and args.out is None:
        ap.error("--set p3 needs --out DIR (for example assets/minigolf/build/p3): the runtime folder ui/3d/minigolf stays on the "
                 "legacy courses until the switch-over")
    args.out = os.path.abspath(args.out) if args.out else RUNTIME
    if args.only is None:
        args.only = "courses" if args.course else "kits,courses"
    parts = [part.strip() for part in args.only.split(",") if part.strip()]
    if "all" in parts:
        parts = list(PARTS)
    unknown = [part for part in parts if part not in PARTS]
    if unknown or not parts:
        ap.error("--only takes kits, courses, previews or all (got %s)" % args.only)
    if args.course_set == "p3" and "previews" in parts:
        ap.error("--only previews is not available for --set p3 yet")
    if args.course and not 1 <= args.course <= COURSE_COUNT:
        ap.error("--course takes a number from 1 to %d" % COURSE_COUNT)
    if args.course and "courses" not in parts:
        ap.error("--course needs the courses part")
    if args.save_blend and "courses" in parts and not args.course:
        ap.error("--save-blend keeps the course scenes in memory: use it with --course N (a full run deletes every course "
                 "after its export, otherwise later courses would get numbered node names)")
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


# -- P3 courses ---------------------------------------------------------------------------------

def world_entries_p3(courses):
    """Menu worlds (``{id, preview}``) in the order pirate, temple, moon, for the worlds that have a course."""
    present = {course["world"] for course in courses}
    return [{"id": world, "preview": "previews/%s.webp" % world} for world in WORLD_ORDER if world in present]


def build_courses_p3(out, only=0, keep_scene=False, existing=()):
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


# -- legacy P2 courses (frozen) ---------------------------------------------------------------------

def world_entries_v1(courses):
    """Menu worlds in order of first appearance: ``{id, look, preview}`` (the preview images come from ``--only previews``)."""
    from kit import dress
    return [{"id": world, "look": dress.LOOK[world], "preview": "previews/%s.webp" % dress.LOOK[world]}
            for world in dict.fromkeys(c["world"] for c in courses)]


def build_courses_v1(only=0, manifest=None, keep_scene=False):
    from kit import courses_v1, dress
    clear_scene()
    if only:
        courses = (manifest or {}).get("courses")
        if not courses or len(courses) != len(courses_v1.COURSES):
            raise SystemExit("--course patches an existing manifest: run a full build first")
        course = courses_v1.COURSES[only - 1]
        courses[only - 1] = courses_v1.build_course(only, course, dress.Dresser(only, course), keep_scene=keep_scene)
        return courses
    return [courses_v1.build_course(i, c, dress.Dresser(i, c)) for i, c in enumerate(courses_v1.COURSES, 1)]


def main():
    args = parse_args()
    out = args.out
    os.makedirs(out, exist_ok=True)
    manifest = read_manifest(out)
    p3 = args.course_set == "p3"
    if p3 and manifest.get("version") != 2:
        manifest = {}               # an old manifest of another version is not patched
    manifest["version"] = 2 if p3 else 1
    if p3:
        manifest.pop("ballRadius", None)
    else:
        from kit import courses_v1
        manifest["ballRadius"] = courses_v1.BALL_RADIUS
    if "kits" in args.parts:
        manifest["kits"] = build_kits(out)
        write_manifest(out, manifest)
    if "courses" in args.parts:
        if p3:
            manifest["courses"] = build_courses_p3(out, args.course, args.save_blend, manifest.get("courses", ()))
            manifest["worlds"] = world_entries_p3(manifest["courses"])
        else:
            manifest["courses"] = build_courses_v1(args.course, manifest, args.save_blend)
            manifest["worlds"] = world_entries_v1(manifest["courses"])
        write_manifest(out, manifest)
        print("Patched course %d in %s" % (args.course, out) if args.course else "Wrote %d courses to %s" % (len(manifest["courses"]), out))
    if "previews" in args.parts:
        from kit import previews
        for look, number in previews.COURSES.items():
            size, quality = previews.render(look, number, os.path.join(out, "previews", look + ".webp"))
            print(f"preview {look}: course {number}, {size} bytes, quality {quality}")
    if args.save_blend:
        os.makedirs(BUILD, exist_ok=True)
        bpy.context.preferences.filepaths.save_version = 0
        bpy.ops.wm.save_as_mainfile(filepath=os.path.join(BUILD, "aura-golf.blend"))


if __name__ == "__main__":
    main()
