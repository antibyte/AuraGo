"""Pack reviewed Imagegen artwork into deterministic 10x10 RGBA game sheets.

Requires Pillow. Sources and explicit frame selections live in the production
manifest; no background is guessed from RGB colors and no artwork is synthesized.
"""
import argparse
import hashlib
import json
import io
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
PACKS = ROOT / "internal/gamemaker/asset_packs"


def write_output(path, data, check):
    if check:
        if not path.exists() or path.read_bytes() != data:
            raise ValueError(f"Generated sprite artifact differs: {path}")
    else:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)


def assembly_cell(crop, layout):
    """Slice a shared canvas; never resize or re-anchor individual building parts."""
    width, height = layout["columns"] * 64, layout["rows"] * 64
    canvas = Image.new("RGBA", (width, height))
    sprite = crop if layout.get("preserve_cell") else crop.crop(crop.getbbox())
    scale = min((width - 8) / sprite.width, (height - 8) / sprite.height)
    sprite = sprite.resize((max(1, round(sprite.width * scale)), max(1, round(sprite.height * scale))), Image.Resampling.NEAREST)
    canvas.alpha_composite(sprite, ((width - sprite.width) // 2, (height - sprite.height) // 2))
    x, y = layout["column"] * 64, layout["row"] * 64
    return canvas.crop((x, y, x + 64, y + 64))


def pack(definition, source_root, check=False):
    sheet = Image.new("RGBA", (640, 640))
    frames, assets, animations = [], [], []
    sources = {}
    for key, spec in definition["sources"].items():
        data = (source_root / spec["file"]).read_bytes()
        image = Image.open(source_root / spec["file"]).convert("RGBA")
        if image.getchannel("A").getextrema()[0] >= 128:
            raise ValueError(f"{key}: source has no real transparency")
        # Pixel-art output uses binary alpha. Remove near-transparent codec halos,
        # preserving actual alpha rather than deleting a guessed background color.
        image.putalpha(image.getchannel("A").point(lambda a: 255 if a >= 128 else 0))
        if "crop" in spec:
            image = image.crop(spec["crop"])
        sources[key] = (image, spec)
        spec["sha256"] = hashlib.sha256(data).hexdigest()
    for asset in definition["assets"]:
        image, spec = sources[asset["source"]]
        cols, rows = spec["columns"], spec["rows"]
        crops = []
        for index in asset["source_frames"]:
            x, y = index % cols, index // cols
            rect = spec.get("rects", {}).get(str(index),
                (round(x * image.width / cols), round(y * image.height / rows),
                 round((x + 1) * image.width / cols), round((y + 1) * image.height / rows)))
            crop = image.crop(rect)
            bbox = crop.getbbox()
            if not bbox:
                raise ValueError(f"Empty source cell: {asset['id']} {index}")
            crops.append((crop, bbox))
        scale = spec.get("scale", min(56 / (image.width / cols), 56 / (image.height / rows)))
        ids = []
        for source_index, (crop, bbox) in zip(asset["source_frames"], crops):
            index = len(frames)
            if index >= 100:
                raise ValueError("More than 100 frames")
            if "slice" in asset:
                sprite = assembly_cell(crop, asset["slice"])
                if not sprite.getbbox():
                    raise ValueError(f"Empty assembly part: {asset['id']} {source_index}")
                x, y = index % 10 * 64, index // 10 * 64
                sheet.alpha_composite(sprite, (x, y))
                ids.append(index)
                frames.append({"index": index, "asset_id": asset["id"], "x": x, "y": y, "w": 64, "h": 64})
                continue
            sprite = crop.crop(bbox)
            sprite = sprite.resize((max(1, round(sprite.width * scale)), max(1, round(sprite.height * scale))), Image.Resampling.NEAREST)
            if sprite.width > 60 or sprite.height > 60:
                sprite.thumbnail((60, 60), Image.Resampling.NEAREST)
            if asset.get("tile"):
                sprite = crop.crop(bbox).resize((64, 64), Image.Resampling.NEAREST)
            x, y = index % 10 * 64, index // 10 * 64
            top = 60 - sprite.height if asset.get("view") == "side" else (64 - sprite.height) // 2
            if asset.get("tile"):
                top = 0
            if asset.get("action") == "jump":
                top -= min(top - 2, round(max(0, crop.height * .9 - bbox[3]) * scale))
            left = (64 - sprite.width) // 2
            if "baseline" in asset:
                top = asset["baseline"] - sprite.height
            anchor = spec.get("anchors", {}).get(str(source_index))
            if anchor:
                rect = spec["rects"][str(source_index)]
                left = 32 - round((anchor[0] - rect[0] - bbox[0]) * scale)
                top = asset.get("baseline", 46) - round((anchor[1] - rect[1] - bbox[1]) * scale)
            if left < 0 or top < 0 or left + sprite.width > 64 or top + sprite.height > 64:
                raise ValueError(f"Sprite exceeds cell: {asset['id']} {source_index}")
            sheet.alpha_composite(sprite, (x + left, y + top))
            ids.append(index)
            frames.append({"index": index, "asset_id": asset["id"], "x": x, "y": y, "w": 64, "h": 64})
        item = {key: asset[key] for key in ("id", "name", "description", "tags", "view")}
        item.update(frames=ids, direction=asset.get("direction", "none"),
                    origin={"x": .5, "y": .9375 if asset.get("view") == "side" else .5},
                    flip_x=asset.get("view") == "side")
        if asset.get("tile"):
            item["tile"] = True
        if "slice" in asset:
            item.update(assembly_part=True, origin={"x": 0, "y": 0}, flip_x=False)
        assets.append(item)
        if "action" in asset:
            action = asset["action"]
            animations.append({"id": asset["id"], "asset_id": asset["id"], "frames": ids,
                               "frame_rate": 6 if action == "idle" else 10 if action in ("walk", "run", "move") else 12,
                               "repeat": -1 if asset.get("loop", action in ("idle", "walk", "run", "move")) else 0,
                               "yoyo": False})
    if len(frames) != 100:
        raise ValueError(f"{definition['id']}: expected 100 frames, got {len(frames)}")
    for animation in animations:
        poses = {sheet.crop((i % 10 * 64, i // 10 * 64, i % 10 * 64 + 64, i // 10 * 64 + 64)).tobytes() for i in animation["frames"]}
        asset = next(a for a in assets if a["id"] == animation["asset_id"])
        if len(animation["frames"]) > 1 and len(poses) < 2 and not asset.get("assembly_part"):
            raise ValueError(f"Animation has no movement: {animation['id']}")
    for alias in definition.get("animation_aliases", []):
        asset = next(a for a in assets if a["id"] == alias["asset_id"])
        animations.append({"id": alias["id"], "asset_id": asset["id"], "frames": [asset["frames"][0]],
                           "frame_rate": 6, "repeat": -1, "yoyo": False})
    metadata = {key: definition[key] for key in ("id", "name", "description", "tags")}
    metadata.update(schema_version=1, version=definition["version"], image="sheet.png", columns=10, rows=10,
                    frame_width=64, frame_height=64, frames=frames, assets=assets, animations=animations,
                    provenance={"generator": "OpenAI Imagegen", "license": "MIT", "license_text": (ROOT / "LICENSE").read_text(encoding="utf8"), "source_manifest": "production/manifest.json",
                                "sources": [{"file": s["file"], "sha256": s["sha256"]} for s in definition["sources"].values()],
                                "note": "Reviewed source poses; repeated source frames are intentional animation holds."})
    if definition.get("assemblies"):
        metadata["assemblies"] = []
        asset_map = {a["id"]: a for a in assets}
        for assembly in definition["assemblies"]:
            item = {k: v for k, v in assembly.items() if k != "parts"}
            item["parts"] = []
            for part in assembly["parts"]:
                asset = asset_map[part["asset_id"]]
                item["parts"].append({**part, "frame": asset["frames"][0]})
            metadata["assemblies"].append(item)
    target = PACKS / definition["id"]
    output = io.BytesIO()
    sheet.save(output, format="PNG", optimize=True)
    write_output(target / "sheet.png", output.getvalue(), check)
    write_output(target / "sheet.json", (json.dumps(metadata, ensure_ascii=False, indent=2) + "\n").encode("utf8"), check)
    return metadata


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, default=PACKS / "production/manifest.json")
    parser.add_argument("--source-root", type=Path, default=PACKS / "production")
    parser.add_argument("--pack")
    parser.add_argument("--check", action="store_true", help="Verify committed output without changing files")
    args = parser.parse_args()
    definitions = json.loads(args.manifest.read_text(encoding="utf8"))
    if args.pack and not any(d["id"] == args.pack for d in definitions):
        parser.error("unknown pack ID")
    for definition in definitions:
        if args.pack and args.pack != definition["id"]:
            continue
        result = pack(definition, args.source_root, args.check)
        print(f"Packed {result['id']}: 100 RGBA frames, {len(result['animations'])} animations")
    write_output(args.manifest, (json.dumps(definitions, ensure_ascii=False, indent=2) + "\n").encode("utf8"), args.check)
    catalog = [{k: d[k] for k in ("id", "name", "description", "tags", "version")} for d in sorted(definitions, key=lambda d: d["id"])]
    write_output(PACKS / "catalog.json", (json.dumps(catalog, ensure_ascii=False, indent=2) + "\n").encode("utf8"), args.check)


if __name__ == "__main__":
    main()
