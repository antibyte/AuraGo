"""Generate the Aura Drift model pack in Blender 5.2 (MIT, AuraGo contributors).

    "D:\\Blender 5.2\\blender.exe" --background --factory-startup --python <abs>/assets/aura-drift/generate.py -- \\
        --only cars|items|track|animals|shared|scenery-alpine,...|all|manifest [--out <abs dir>] [--preview <abs dir>]

``--only`` takes a comma list of groups. ``cars`` builds every ``kit/car_<id>.py`` that exists.
``manifest`` rescans the GLBs in ``--out`` and rewrites manifest.json and LICENSE.txt without building
anything. ``--preview`` renders EEVEE views of what was built into a scratch directory (never the repo).
Default ``--out`` is the absolute ``ui/3d/aura-drift/v1``. See README.md.
"""

import importlib
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)

from kit import core, materials  # noqa: E402
from kit.export import export_glb, render_previews  # noqa: E402

CAR_IDS = ["vektor", "kobalt", "lumen", "tundra", "softeis", "maehkoenig", "couch", "bertha"]
# group -> module with build() returning ONE core.Model whose name is the GLB stem
GROUPS = {
    "items": "kit.items",
    "track": "kit.track_parts",
    "animals": "kit.animals",
    "shared": "kit.shared_props",
    "scenery-alpine": "kit.scenery_alpine",
    "scenery-desert": "kit.scenery_desert",
    "scenery-harbour": "kit.scenery_harbour",
    "scenery-salt": "kit.scenery_salt",
    "scenery-forest": "kit.scenery_forest",
    "scenery-industry": "kit.scenery_industry",
    "scenery-glacier": "kit.scenery_glacier",
    "scenery-metro": "kit.scenery_metro",
}


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    options = {"only": ["all"], "out": core.DEFAULT_OUT, "preview": None}
    i = 0
    while i < len(argv):
        if argv[i] == "--only" and i + 1 < len(argv):
            options["only"] = argv[i + 1].split(",")
            i += 2
        elif argv[i] == "--out" and i + 1 < len(argv):
            options["out"] = argv[i + 1]
            i += 2
        elif argv[i] == "--preview" and i + 1 < len(argv):
            options["preview"] = argv[i + 1]
            i += 2
        else:
            i += 1
    return options


def car_modules():
    found = []
    for car_id in CAR_IDS:
        if os.path.exists(os.path.join(HERE, "kit", "car_%s.py" % car_id)):
            found.append(("kit.car_" + car_id, "car-" + car_id))
    return found


def jobs(only):
    """Ordered [(module, glb stem)] for the requested groups."""
    wanted = []
    for name in only:
        if name == "all":
            wanted += ["cars"] + list(GROUPS)
        elif name != "manifest":
            wanted.append(name)
    out = []
    for name in wanted:
        if name == "cars":
            out += car_modules()
        elif name in GROUPS:
            if os.path.exists(os.path.join(HERE, GROUPS[name].replace(".", os.sep) + ".py")):
                out.append((GROUPS[name], name))
            else:
                print("skip %s: %s.py is not written yet" % (name, GROUPS[name]))
        elif name.startswith("car-") or name.replace("car_", "") in CAR_IDS:
            car_id = name.replace("car-", "").replace("car_", "")
            out.append(("kit.car_" + car_id, "car-" + car_id))
        else:
            raise SystemExit("unknown group %r (cars, %s, manifest, all)" % (name, ", ".join(GROUPS)))
    return out


def main():
    options = parse_args()
    out_dir = os.path.abspath(options["out"])
    only = options["only"]
    built = []
    for module_name, stem in jobs(only):
        core.clear_scene()
        materials.build_all()
        module = importlib.import_module(module_name)
        model = module.build()
        path = os.path.join(out_dir, stem + ".glb")
        export_glb(model, path)
        built.append(model)
        print("  " + ", ".join("%s=%d" % item for item in sorted(model.stats.items(), key=lambda kv: -kv[1])))
        print("exported %s: %d bytes, %d triangles" % (stem, os.path.getsize(path), model.triangles()))
        if options["preview"]:
            render_previews([model], options["preview"])
    if "manifest" in only or "all" in only:
        from kit import manifest

        manifest.write(out_dir)


if __name__ == "__main__":
    main()
