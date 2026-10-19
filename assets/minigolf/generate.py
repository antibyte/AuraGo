"""Generate AuraGo's original MIT-licensed mini golf assets with Blender 5.2.

Run from the repository root:
  blender --background --factory-startup --python assets/minigolf/generate.py -- [--only kits,courses,previews|all] [--course N] [--save-blend]
"""

import argparse
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import bpy  # noqa: E402

from kit import courses_v1, kits  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
RUNTIME = os.path.join(ROOT, "ui", "3d", "minigolf")
BUILD = os.path.join(HERE, "build")
PARTS = ("kits", "courses", "previews")
MANIFEST_ORDER = ("version", "ballRadius", "courses", "kits", "worlds")


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
    ap = argparse.ArgumentParser()
    ap.add_argument("--only", default="kits,courses")
    ap.add_argument("--course", type=int, default=0, help="rebuild one course (1-18) and patch its manifest entry")
    ap.add_argument("--save-blend", action="store_true")
    args = ap.parse_args(argv)
    parts = [part.strip() for part in args.only.split(",") if part.strip()]
    if "all" in parts:
        parts = list(PARTS)
    unknown = [part for part in parts if part not in PARTS]
    if unknown or not parts:
        ap.error("--only takes kits, courses, previews or all (got %s)" % args.only)
    if args.course:
        ap.error("--course is not implemented yet")
    args.parts = parts
    return args


def read_manifest():
    path = os.path.join(RUNTIME, "manifest.json")
    if not os.path.exists(path):
        return {}
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def write_manifest(manifest):
    ordered = {key: manifest[key] for key in MANIFEST_ORDER if key in manifest}
    ordered.update({key: value for key, value in manifest.items() if key not in ordered})
    with open(os.path.join(RUNTIME, "manifest.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(ordered, f, separators=(",", ":"))
        f.write("\n")


def fresh_scene():
    """Start from an empty file so every kit and the courses build in a clean datablock space."""
    bpy.ops.wm.read_factory_settings(use_empty=True)


def build_kits():
    entries = {}
    for look in kits.MODULES:
        fresh_scene()
        entries[look], _ = kits.build_kit(look, RUNTIME)
    return entries


def build_courses():
    bpy.ops.object.select_all(action="SELECT")
    bpy.ops.object.delete(use_global=False)
    return [courses_v1.build_course(i, c) for i, c in enumerate(courses_v1.COURSES, 1)]


def main():
    args = parse_args()
    os.makedirs(RUNTIME, exist_ok=True)
    manifest = read_manifest()
    manifest["version"] = 1
    manifest["ballRadius"] = courses_v1.BALL_RADIUS
    if "kits" in args.parts:
        manifest["kits"] = build_kits()
        write_manifest(manifest)
    if "courses" in args.parts:
        courses = build_courses()
        manifest["courses"] = courses
        write_manifest(manifest)
        print(f"Wrote {len(courses)} courses to {RUNTIME}")
    if "previews" in args.parts:
        print("previews: not implemented yet")
    if args.save_blend:
        os.makedirs(BUILD, exist_ok=True)
        bpy.context.preferences.filepaths.save_version = 0
        bpy.ops.wm.save_as_mainfile(filepath=os.path.join(BUILD, "aura-golf.blend"))


if __name__ == "__main__":
    main()
