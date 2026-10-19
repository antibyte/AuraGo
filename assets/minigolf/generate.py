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

from kit import courses_v1  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
RUNTIME = os.path.join(ROOT, "ui", "3d", "minigolf")
BUILD = os.path.join(HERE, "build")


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
    ap = argparse.ArgumentParser()
    ap.add_argument("--only", default="kits,courses")
    ap.add_argument("--course", type=int, default=0, help="rebuild one course (1-18) and patch its manifest entry")
    ap.add_argument("--save-blend", action="store_true")
    return ap.parse_args(argv)


def write_manifest(manifest):
    with open(os.path.join(RUNTIME, "manifest.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(manifest, f, separators=(",", ":"))
        f.write("\n")


def main():
    args = parse_args()
    bpy.ops.object.select_all(action="SELECT")
    bpy.ops.object.delete(use_global=False)
    os.makedirs(RUNTIME, exist_ok=True)
    courses = [courses_v1.build_course(i, c) for i, c in enumerate(courses_v1.COURSES, 1)]
    write_manifest({"version": 1, "ballRadius": courses_v1.BALL_RADIUS, "courses": courses})
    if args.save_blend:
        os.makedirs(BUILD, exist_ok=True)
        bpy.context.preferences.filepaths.save_version = 0
        bpy.ops.wm.save_as_mainfile(filepath=os.path.join(BUILD, "aura-golf.blend"))
    print(f"Wrote {len(courses)} courses to {RUNTIME}")


if __name__ == "__main__":
    main()
