"""Style-probe entry point (MIT License).

Run with Blender 5.2 from the repository root::

    blender --background --factory-startup --python assets/minigolf/kit/probe.py -- \\
        --out <dir> [--world pirate|temple|moon|all] [--samples N] [--quick]

Renders one 1600x900 Cycles probe per world (diorama island, lane section, hero
prop, secondary props, water or space), bakes ambient occlusion into vertex
colours first, and saves PNG (in --out) plus WebP (documentation/auragolf/img). ``--prop world:name`` renders a single
prop close-up for quick iteration. Everything is deterministic: fixed seeds,
fixed bake/render samples, no time-based input.
"""

import argparse
import hashlib
import importlib
import json
import os
import struct
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))  # assets/minigolf, so ``import kit`` works
REPO = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
DEFAULT_WEBP_DIR = os.path.join(REPO, "documentation", "auragolf", "img")

import bpy  # noqa: E402
from mathutils import Vector  # noqa: E402

from kit import bake, palette, shapes, stage  # noqa: E402

SCENES = {"pirate": "kit.scene_pirate", "temple": "kit.scene_temple", "moon": "kit.scene_moon"}
PROPS_MODULES = {"pirate": "kit.props_pirate", "temple": "kit.props_temple", "moon": "kit.props_moon"}


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", required=True)
    ap.add_argument("--world", default="all")
    ap.add_argument("--samples", type=int, default=128)
    ap.add_argument("--quick", action="store_true", help="800x450, 40 samples, coarse grids")
    ap.add_argument("--prop", default="", help="world:name close-up preview")
    ap.add_argument("--view", default="-35,32,1.6", help="preview azimuth,elevation,distance factor")
    ap.add_argument("--webp-dir", default=None, help="default: documentation/auragolf/img (never in --quick)")
    ap.add_argument("--no-webp", action="store_true")
    ap.add_argument("--webp-quality", type=int, default=88)
    ap.add_argument("--save-blend", action="store_true")
    ap.add_argument("--no-render", action="store_true")
    return ap.parse_args(argv)


def save_outputs(sc, out_dir, world, webp_dir, quality):
    img = bpy.data.images["Render Result"]
    png = os.path.join(out_dir, "p2-stilprobe-%s.png" % world)
    sc.render.image_settings.file_format = "PNG"
    img.save_render(png, scene=sc)
    result = {"png": png}
    if webp_dir:
        os.makedirs(webp_dir, exist_ok=True)
        path = os.path.join(webp_dir, "p2-stilprobe-%s.webp" % world)
        sc.render.image_settings.file_format = "WEBP"
        sc.render.image_settings.color_mode = "RGB"
        sc.render.dither_intensity = 0.0
        q = quality
        while True:
            sc.render.image_settings.quality = q
            img.save_render(path, scene=sc)
            size = os.path.getsize(path)
            if size <= 450 * 1024 or q <= 60:
                break
            q -= 4
        result.update(webp=path, webp_quality=q, webp_bytes=size)
    return result


def scene_hash(objs):
    """MD5 over vertex positions and baked 8-bit colours: the reproducibility fingerprint."""
    h = hashlib.md5()
    for obj in sorted(objs, key=lambda o: o.name):
        mesh = obj.data
        n = len(mesh.vertices)
        co = [0.0] * (n * 3)
        mesh.vertices.foreach_get("co", co)
        h.update(obj.name.encode())
        h.update(struct.pack("%df" % len(co), *co))
        col = mesh.color_attributes.get("Col")
        if col is not None:
            buf = [0.0] * (n * 4)
            col.data.foreach_get("color", buf)
            h.update(bytes(min(255, int(v * 255.0 + 0.5)) for v in buf))
    return h.hexdigest()


def run_world(world, args):
    t_all = time.time()
    sc = stage.reset()
    quick = args.quick
    res = (800, 450) if quick else (stage.RES_X, stage.RES_Y)
    samples = 40 if quick else args.samples
    stage.configure_render(sc, samples=samples, res=res)
    mod = importlib.import_module(SCENES[world])
    t0 = time.time()
    comp = mod.build(sc, quick=quick)
    build_s = time.time() - t0
    t0 = time.time()
    bake_cfg = dict(mod.BAKE)
    if quick:
        bake_cfg["samples_bake"] = 64
    bake_s = 0.0 if args.no_render and False else bake.process(
        comp.bake_objs, passthrough=comp.passthrough, hidden=comp.hidden + comp.passthrough + comp.backdrop,
        samples=bake_cfg["samples_bake"], distance=bake_cfg["distance"], tint=bake_cfg["tint"],
        gamma=bake_cfg.get("gamma", 1.0))
    bake_total = time.time() - t0
    info = {"world": world, "build_s": round(build_s, 1), "bake_s": round(bake_s, 1), "bake_total_s": round(bake_total, 1),
            "total_tris": comp.total_tris(), "backdrop_tris": comp.backdrop_tris(),
            "prop_tris": dict(sorted(comp.prop_tris.items()))}
    info["scene_hash"] = scene_hash(comp.bake_objs + comp.hidden + comp.passthrough)
    if args.save_blend:
        bpy.ops.wm.save_as_mainfile(filepath=os.path.join(args.out, "probe-%s.blend" % world))
    if not args.no_render:
        t0 = time.time()
        path = os.path.join(args.out, "render-%s.png" % world)
        sc.render.filepath = path
        bpy.ops.render.render(write_still=True)
        info["render_s"] = round(time.time() - t0, 1)
        webp_dir = args.webp_dir if args.webp_dir is not None else ("" if quick or args.no_webp else DEFAULT_WEBP_DIR)
        info.update(save_outputs(sc, args.out, world, webp_dir, args.webp_quality))
        try:
            info["clip"] = stage.clip_report(info["png"])
        except Exception as exc:  # diagnostics only
            info["clip"] = str(exc)
    info["wall_s"] = round(time.time() - t_all, 1)
    return info


def run_prop(spec, args):
    world, name = spec.split(":")
    sc = stage.reset()
    stage.configure_render(sc, samples=40, res=(800, 450))
    mod = importlib.import_module(PROPS_MODULES[world])
    scene_mod = importlib.import_module(SCENES[world])
    pal = palette.get(world)
    obj = mod.PROPS[name]()
    lo, hi = shapes.bounds(obj)
    size = max(hi.x - lo.x, hi.y - lo.y, hi.z - lo.z)
    ground = shapes.grid("preview-ground", 40, 40, size * 4 + 2, size * 4 + 2)
    from kit import colors, materials
    base = pal.c("sand_wet") if pal.has("sand_wet") else (pal.c("stone") if pal.has("stone") else pal.c("regolith"))
    colors.paint(ground, base, base, ao=0.7)
    ground.data.materials.append(materials.get("matte"))
    scene_mod.lighting(sc, pal)
    center = Vector(((lo.x + hi.x) / 2, (lo.y + hi.y) / 2, (hi.z) * 0.45))
    az, el, k = (float(v) for v in args.view.split(","))
    stage.add_camera(sc, center, size * k + 1.0, az, el)
    bake.process([ground, obj], samples=96, distance=max(0.8, size * 0.5), tint=scene_mod.BAKE["tint"])
    path = os.path.join(args.out, "prop-%s-%s%s.png" % (world, name, "" if args.view == "-35,32,1.6" else "-v"))
    t0 = time.time()
    sc.render.filepath = path
    bpy.ops.render.render(write_still=True)
    return {"prop": spec, "top_parts": obj.get("kit_part_tris", ""), "tris": shapes.tri_count(obj), "size": [round(v, 2) for v in (hi - lo)],
            "render_s": round(time.time() - t0, 1), "png": path}


def main():
    args = parse_args()
    os.makedirs(args.out, exist_ok=True)
    results = []
    if args.prop:
        results.append(run_prop(args.prop, args))
    else:
        worlds = ["pirate", "temple", "moon"] if args.world == "all" else [args.world]
        for world in worlds:
            results.append(run_world(world, args))
    print("PROBE_RESULT " + json.dumps(results, indent=1))


main()
