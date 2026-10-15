#!/usr/bin/env python3
"""Deterministic display exports of the checked-in ImageGen atlases (Pillow)."""
import argparse
import hashlib
import io
import json
from pathlib import Path

from PIL import Image, ImageChops

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "assets/cyd-icons/sources"
OUTPUT = ROOT / "ui/img/cyd-icons"
STYLES = ("playful", "modern", "comic", "sober")
TONES = ("color", "mono")
SIZES = (24, 32, 48, 64)
ROWS = (
    "home menu back forward up down search settings add close",
    "folder file save trash check error warning info lock unlock",
    "wifi ethernet bluetooth server cloud upload download sync power cpu",
    "bulb outlet switch thermometer humidity fan heater door window motion",
    "battery battery_charging energy solar sun moon cloudy rain snow wind",
    "play pause stop volume mute microphone camera bell message clock",
)
MOTIFS = " ".join(ROWS).split()
MOTION = ("wifi upload download bulb fan battery_charging".split(),
          "rain snow wind volume microphone bell".split())


def cells(path, cols, rows):
    image = Image.open(path).convert("RGBA")
    assert image.getextrema()[3][0] == 0, f"Missing transparency: {path}"
    alpha = image.getchannel("A")
    # Generated sheets have slightly unequal margins. Split only in transparent
    # gutters, never at a nominal grid line through the neighbouring icon.
    def cuts(length, count, horizontal):
        blank = []
        for i in range(length):
            strip = alpha.crop((i, 0, i + 1, image.height) if horizontal else (0, i, image.width, i + 1))
            blank.append(strip.getextrema()[1] < 8)
        gaps, start = [], None
        for i, empty in enumerate(blank + [False]):
            if empty and start is None:
                start = i
            if not empty and start is not None:
                if start > 0 and i < length and i - start >= 2:
                    gaps.append((start + i) // 2)
                start = None
        boundaries = [0]
        for n in range(1, count):
            target = length * n / count
            # These reviewed atlases put rain drops below the nominal first row.
            if not horizontal and n == 1 and path.name in (
                "modern-mono-motion-1.png", "comic-mono-motion-1.png", "sober-mono-motion-1.png"
            ):
                target = length * 0.21
            candidates = [g for g in gaps if abs(g - target) < length / count * 0.45 and g > boundaries[-1]]
            assert candidates, f"Missing gutter {n}: {path}"
            boundaries.append(min(candidates, key=lambda g: abs(g - target)))
        return boundaries + [length]
    xs, ys = cuts(image.width, cols, True), cuts(image.height, rows, False)
    width, height = max(b - a for a, b in zip(xs, xs[1:])), max(b - a for a, b in zip(ys, ys[1:]))
    result = []
    for y in range(rows):
        for x in range(cols):
            cell = image.crop((xs[x], ys[y], xs[x + 1], ys[y + 1]))
            padded = Image.new("RGBA", (width, height))
            padded.paste(cell, ((width - cell.width) // 2, (height - cell.height) // 2))
            result.append(padded)
    return result


def export(frames, size, tone):
    masks = [f.getchannel("A").point(lambda v: v if v >= 8 else 0) for f in frames]
    union = masks[0]
    for mask in masks[1:]:
        union = ImageChops.lighter(union, mask)
    box = union.getbbox()
    assert box, "Empty icon"
    # Rotating the whole square must also stay inside the requested icon area.
    extent = max(box[2] - box[0], box[3] - box[1])
    scale = (size * 0.70) / extent
    width, height = max(1, round((box[2] - box[0]) * scale)), max(1, round((box[3] - box[1]) * scale))
    result = []
    for frame, mask in zip(frames, masks):
        frame.putalpha(mask)
        small = frame.crop(box).resize((width, height), Image.Resampling.LANCZOS)
        if tone == "mono":
            alpha = small.getchannel("A")
            small = Image.new("RGBA", small.size, (255, 255, 255, 0))
            small.putalpha(alpha)
        else:
            r, g, b, a = small.split()
            # Match the generator's high-bit RGB565 conversion, expanded for browser PNGs.
            r = r.point(lambda v: ((v >> 3) << 3) | (v >> 5))
            g = g.point(lambda v: ((v >> 2) << 2) | (v >> 6))
            b = b.point(lambda v: ((v >> 3) << 3) | (v >> 5))
            small = Image.merge("RGBA", (r, g, b, a))
        out = Image.new("RGBA", (size, size))
        out.paste(small, ((size - width) // 2, (size - height) // 2))
        result.append(out)
    return result


def write(path, raw, check):
    if check:
        assert path.read_bytes() == raw, f"Stale export: {path}"
    else:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(raw)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--contact-sheets", type=Path)
    args = parser.parse_args()
    manifest = {"version": 1, "files": {}}
    assert len(MOTIFS) == len(set(MOTIFS)) == 60
    for style in STYLES:
        for tone in TONES:
            masters = {name: [cell] for name, cell in zip(MOTIFS, cells(SOURCE / f"{style}-{tone}.png", 10, 6))}
            for group, names in enumerate(MOTION):
                atlas = cells(SOURCE / f"{style}-{tone}-motion-{group}.png", 6, 6)
                for row, name in enumerate(names):
                    masters[name] = atlas[row * 6:(row + 1) * 6]
                    if style == "playful" and tone == "mono" and name == "download":
                        masters[name] = cells(SOURCE / "playful-mono-download-repair.png", 6, 6)[12:18]
                    # The still image must show the complete symbol, e.g. all Wi-Fi arcs.
                    start = 3 if name in ("wifi", "upload", "bulb", "battery_charging", "volume", "microphone", "rain") else 0
                    masters[name] = masters[name][start:] + masters[name][:start]
            for size in SIZES:
                thumbnails = []
                for name in MOTIFS:
                    frames = export(masters[name], size, tone)
                    thumbnails.append(frames[0])
                    for number, frame in enumerate(frames):
                        buffer = io.BytesIO()
                        frame.save(buffer, format="PNG", optimize=False, compress_level=9)
                        raw = buffer.getvalue()
                        key = f"{style}/{tone}/{name}/{size}/{number}.png"
                        write(OUTPUT / key, raw, args.check)
                        manifest["files"][key] = {"sha256": hashlib.sha256(raw).hexdigest(), "bytes": len(raw)}
                if args.contact_sheets:
                    for bg, color in (("light", "#f4f4f4"), ("dark", "#202833")):
                        pitch = size + 12
                        sheet = Image.new("RGB", (pitch * 10, pitch * 6), color)
                        for index, frame in enumerate(thumbnails):
                            if tone == "mono" and bg == "light":
                                frame = frame.copy()
                                alpha = frame.getchannel("A")
                                frame.paste((28, 38, 48), (0, 0, size, size))
                                frame.putalpha(alpha)
                            sheet.paste(frame, ((index % 10) * pitch + 6, (index // 10) * pitch + 6), frame)
                        path = args.contact_sheets / f"{style}-{tone}-{size}-{bg}.png"
                        path.parent.mkdir(parents=True, exist_ok=True)
                        sheet.save(path)
    assert len(manifest["files"]) == 3840
    write(OUTPUT / "manifest.json", (json.dumps(manifest, indent=2, sort_keys=True) + "\n").encode(), args.check)
    write(ROOT / "assets/cyd-icons/export-manifest.json", (json.dumps({
        "version": 1, "sets": 16, "motifs": MOTIFS, "styles": STYLES, "tones": TONES,
        "sizes": SIZES, "variants": 960, "png_files": 3840, "fps": 6,
        "source_sha256": {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(SOURCE.glob("*.png"))},
        "runtime_manifest_sha256": hashlib.sha256((OUTPUT / "manifest.json").read_bytes()).hexdigest(),
    }, indent=2) + "\n").encode(), args.check)
    print(f"{'Verified' if args.check else 'Exported'} 16 sets, 960 variants, {len(manifest['files'])} PNGs")


if __name__ == "__main__":
    main()
