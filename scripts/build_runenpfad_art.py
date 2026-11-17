#!/usr/bin/env python3
"""Runenpfad art pipeline.

--import <rawdir> <promptsdir> [ids]  clean + downscale raw Codex PNG sheets into ui/img/runenpfad/*.webp
                                and (re)write assets/runenpfad-art/catalog.json
--write (default)               measure trimmed frame rects from the committed WebPs and write
                                ui/img/runenpfad/atlas.json + provenance.json
--check                         recompute everything and fail when a committed file is stale
"""
import hashlib
import json
import statistics
import sys
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parent.parent
IMG_DIR = ROOT / "ui" / "img" / "runenpfad"
CATALOG = ROOT / "assets" / "runenpfad-art" / "catalog.json"
ATLAS = IMG_DIR / "atlas.json"
PROVENANCE = IMG_DIR / "provenance.json"

ALPHA_CUT = 24        # below this the generated "fog" is erased
ALPHA_BOX = 48        # trimmed rectangles measure only solid pixels
MAX_CELL = 256
MAX_CELL_BOSS = 340
MAX_CELL_DRAGON = 360

NOTE = "Original generated art for AuraGo Runenpfad; not copied from any third-party game or asset pack."
TOOL = "Codex CLI image generation"

# id -> (cols, rows, pivot, kind)
SHEETS = {
    "boss-boar": (4, 3, "feet", "boss"),
    "boss-goblin": (4, 3, "feet", "boss"),
    "boss-orb": (4, 3, "center", "boss"),
    "boss-quad": (4, 3, "center", "boss"),
    "boss-mech": (4, 3, "feet", "boss"),
    "enemy-sporecap": (4, 3, "feet", "enemy"),
    "enemy-crystalbat": (4, 3, "center", "enemy"),
    "enemy-scarab": (4, 3, "feet", "enemy"),
    "enemy-frostwraith": (4, 3, "center", "enemy"),
    "enemy-clockknight": (4, 3, "feet", "enemy"),
    "fx-slash": (3, 2, "center", "fx"),
    "fx-hit": (3, 2, "center", "fx"),
    "fx-dust": (3, 2, "base", "fx"),
    "fx-frost": (3, 2, "center", "fx"),
    "fx-poison": (3, 2, "center", "fx"),
    "fx-explosion": (4, 2, "center", "fx"),
    "fx-rune-burst": (4, 2, "center", "fx"),
    "fx-sparkle": (4, 2, "center", "fx"),
    "fx-fire-pillar": (4, 2, "base", "fx"),
    "fx-lightning": (6, 1, "base", "fx"),
    "fx-steam": (6, 1, "base", "fx"),
    "prop-chest": (3, 2, "feet", "prop"),
    "prop-portal": (4, 2, "feet", "prop"),
    "prop-checkpoint": (4, 2, "feet", "prop"),
    "prop-switch": (4, 1, "feet", "prop"),
    "powerups": (4, 2, "center", "prop"),
    # round 4
    "dragon-ember": (4, 3, "center", "dragon"),
    "dragon-frost": (4, 3, "center", "dragon"),
    "enemy-drakeling": (4, 3, "center", "enemy"),
    "fx-dragonfire": (2, 4, "left", "fx"),
    "fx-fireball": (4, 2, "center", "fx"),
    "prop-glyphs": (4, 2, "feet", "prop"),
    "prop-lever": (4, 1, "feet", "prop"),
    "prop-brazier": (4, 2, "feet", "prop"),
    "prop-plate": (4, 1, "feet", "prop"),
    "prop-oracle": (4, 1, "feet", "prop"),
    "prop-swing": (4, 1, "center", "prop"),
}


def clips_for(sheet_id, kind, count):
    if kind == "dragon":
        return "idle", {
            "idle": {"frames": [0, 1, 2, 3], "ms": 130, "loop": True},
            "telegraph": {"frames": [4, 5], "ms": 200, "loop": False},
            "attack": {"frames": [6, 7], "ms": 110, "loop": True},
            "roar": {"frames": [8], "ms": 200, "loop": False},
            "enraged": {"frames": [8], "ms": 200, "loop": False},
            "hurt": {"frames": [9], "ms": 120, "loop": False},
            "defeat": {"frames": [10, 11], "ms": 420, "loop": False},
        }
    if sheet_id == "fx-dragonfire":
        return "loop", {
            "start": {"frames": [0, 1], "ms": 70, "loop": False},
            "loop": {"frames": [2, 3, 4, 5], "ms": 75, "loop": True},
            "end": {"frames": [6, 7], "ms": 110, "loop": False},
        }
    if sheet_id == "fx-fireball":
        return "loop", {
            "loop": {"frames": [0, 1, 2, 3], "ms": 70, "loop": True},
            "impact": {"frames": [4, 5, 6, 7], "ms": 65, "loop": False},
        }
    if sheet_id == "prop-glyphs":
        clips = {}
        for g in range(4):
            clips["dormant_%d" % g] = {"frames": [g], "ms": 200, "loop": False}
            clips["lit_%d" % g] = {"frames": [4 + g], "ms": 200, "loop": False}
        return "dormant_0", clips
    if sheet_id == "prop-lever":
        return "off", {
            "off": {"frames": [0], "ms": 200, "loop": False},
            "mid": {"frames": [1], "ms": 200, "loop": False},
            "on": {"frames": [2], "ms": 200, "loop": False},
            "glint": {"frames": [2, 3], "ms": 240, "loop": True},
        }
    if sheet_id == "prop-brazier":
        return "unlit", {
            "unlit": {"frames": [0], "ms": 200, "loop": False},
            "ignite": {"frames": [1], "ms": 160, "loop": False},
            "burn": {"frames": [2, 3, 4, 5, 6, 7], "ms": 95, "loop": True},
        }
    if sheet_id == "prop-plate":
        return "up", {
            "up": {"frames": [0], "ms": 200, "loop": False},
            "down": {"frames": [1], "ms": 200, "loop": False},
            "lit": {"frames": [2], "ms": 200, "loop": False},
            "pulse": {"frames": [2, 3], "ms": 220, "loop": True},
        }
    if sheet_id == "prop-oracle":
        return "dormant", {
            "dormant": {"frames": [0], "ms": 200, "loop": False},
            "eyes": {"frames": [1], "ms": 200, "loop": False},
            "speaking": {"frames": [2], "ms": 200, "loop": False},
            "calm": {"frames": [3], "ms": 200, "loop": False},
        }
    if sheet_id == "prop-swing":
        return "ring", {
            "ring": {"frames": [0], "ms": 200, "loop": False},
            "branch": {"frames": [1], "ms": 200, "loop": False},
            "grip": {"frames": [2], "ms": 200, "loop": False},
            "rope": {"frames": [3], "ms": 200, "loop": False},
        }
    if kind == "boss":
        return "idle", {
            "idle": {"frames": [0, 1, 2, 3], "ms": 150, "loop": True},
            "telegraph": {"frames": [4, 5], "ms": 160, "loop": False},
            "attack": {"frames": [6, 7], "ms": 120, "loop": False},
            "hurt": {"frames": [8], "ms": 120, "loop": False},
            "enraged": {"frames": [9], "ms": 200, "loop": False},
            "defeat": {"frames": [10, 11], "ms": 420, "loop": False},
        }
    if kind == "enemy":
        return "idle", {
            "idle": {"frames": [0, 1, 2, 3], "ms": 140, "loop": True},
            "telegraph": {"frames": [4, 5], "ms": 140, "loop": False},
            "attack": {"frames": [6, 7], "ms": 110, "loop": False},
            "hurt": {"frames": [8, 9], "ms": 90, "loop": False},
            "death": {"frames": [10, 11], "ms": 220, "loop": False},
        }
    if sheet_id == "prop-portal":
        return "loop", {"loop": {"frames": list(range(8)), "ms": 100, "loop": True}}
    if sheet_id == "prop-checkpoint":
        return "active", {
            "dormant": {"frames": [0], "ms": 200, "loop": False},
            "activate": {"frames": [1, 2, 3], "ms": 110, "loop": False},
            "active": {"frames": [4, 5, 6, 7], "ms": 130, "loop": True},
        }
    if sheet_id == "prop-chest":
        return "closed", {
            "closed": {"frames": [0], "ms": 200, "loop": False},
            "open": {"frames": [1, 2, 3, 4, 5], "ms": 90, "loop": False},
        }
    if sheet_id == "prop-switch":
        return "off", {
            "off": {"frames": [0], "ms": 200, "loop": False},
            "flash": {"frames": [1], "ms": 160, "loop": False},
            "on": {"frames": [2, 3], "ms": 200, "loop": True},
        }
    if sheet_id == "powerups":
        return "fire", {
            "fire": {"frames": [0, 4], "ms": 260, "loop": True},
            "frost": {"frames": [1, 5], "ms": 260, "loop": True},
            "storm": {"frames": [2, 6], "ms": 260, "loop": True},
            "magnet": {"frames": [3, 7], "ms": 260, "loop": True},
        }
    ms = {"fx-lightning": 55, "fx-steam": 100, "fx-explosion": 70, "fx-fire-pillar": 85}.get(sheet_id, 65)
    frames = list(range(count))
    clips = {
        "play": {"frames": frames, "ms": ms, "loop": False},
        "loop": {"frames": frames, "ms": ms + 20, "loop": True},
    }
    if sheet_id in ("fx-fire-pillar", "fx-steam"):
        # sustained column (tall frames only) and the short growth used for warnings
        clips["sustain"] = {"frames": [2, 3, 4, 3], "ms": 95, "loop": True}
        clips["warn"] = {"frames": [0, 1], "ms": 140, "loop": True}
    return "play", clips


def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()


def sha256_file(path):
    return sha256_bytes(Path(path).read_bytes())


def dump_json(obj):
    return json.dumps(obj, indent=1, sort_keys=False, ensure_ascii=False) + "\n"


def clean_alpha(im):
    """Zero the near-invisible fog (garbage RGB under low alpha)."""
    r, g, b, a = im.split()
    a = a.point(lambda v: 0 if v < ALPHA_CUT else v)
    im = Image.merge("RGBA", (r, g, b, a))
    # RGB below zero alpha is meaningless; make it deterministic and cheap to encode
    black = Image.new("RGBA", im.size, (0, 0, 0, 0))
    mask = a.point(lambda v: 255 if v > 0 else 0)
    return Image.composite(im, black, mask)


def cell_size(src_w, src_h, cols, rows, limit):
    cw, ch = src_w / cols, src_h / rows
    k = min(1.0, limit / max(cw, ch))
    return max(8, int(round(cw * k))), max(8, int(round(ch * k)))


def do_import(raw_dir, prompt_dir, only=None):
    raw_dir, prompt_dir = Path(raw_dir), Path(prompt_dir)
    only = set(only or [])
    for unknown in only - set(SHEETS):
        raise SystemExit("unknown sheet id: %s" % unknown)
    old = {}
    if only and CATALOG.is_file():
        for entry in json.loads(CATALOG.read_text(encoding="utf-8"))["sheets"]:
            old[entry["id"]] = entry
    IMG_DIR.mkdir(parents=True, exist_ok=True)
    CATALOG.parent.mkdir(parents=True, exist_ok=True)
    sheets = []
    for sheet_id, (cols, rows, pivot, kind) in SHEETS.items():
        if only and sheet_id not in only:
            if sheet_id in old:
                sheets.append(old[sheet_id])
            continue
        src = raw_dir / (sheet_id + ".png")
        if not src.is_file():
            raise SystemExit("missing raw sheet: %s" % src)
        raw_bytes = src.read_bytes()
        im = Image.open(src).convert("RGBA")
        im = clean_alpha(im)
        limit = MAX_CELL_DRAGON if kind == "dragon" else (MAX_CELL_BOSS if kind == "boss" else MAX_CELL)
        cw, ch = cell_size(im.width, im.height, cols, rows, limit)
        out = im.resize((cw * cols, ch * rows), Image.LANCZOS)
        out = clean_alpha(out)
        out.save(IMG_DIR / (sheet_id + ".webp"), "WEBP", quality=90, method=6, alpha_quality=100, exact=False)
        default, clips = clips_for(sheet_id, kind, cols * rows)
        prompt_file = prompt_dir / (sheet_id + ".txt")
        prompt = prompt_file.read_text(encoding="utf-8").strip() if prompt_file.is_file() else ""
        sheets.append({
            "id": sheet_id,
            "file": sheet_id + ".webp",
            "cols": cols,
            "rows": rows,
            "pivot": pivot,
            "kind": kind,
            "default": default,
            "clips": clips,
            "prompt": prompt,
            "source": src.name,
            "sourceSha256": sha256_bytes(raw_bytes),
        })
        print("imported %-18s %dx%d cell %dx%d" % (sheet_id, cols, rows, cw, ch))
    CATALOG.write_text(dump_json({"version": 1, "sheets": sheets}), encoding="utf-8", newline="\n")


def trimmed_rects(im, cols, rows):
    """Per-cell bounding box of solid pixels (alpha >= ALPHA_BOX), in sheet pixels."""
    cw, ch = im.width // cols, im.height // rows
    alpha = im.getchannel("A").point(lambda v: 255 if v >= ALPHA_BOX else 0)
    rects = []
    for r in range(rows):
        for c in range(cols):
            box = alpha.crop((c * cw, r * ch, (c + 1) * cw, (r + 1) * ch)).getbbox()
            if box is None:
                rects.append(None)
            else:
                rects.append((c * cw + box[0], r * ch + box[1], box[2] - box[0], box[3] - box[1]))
    return cw, ch, rects


def median(values):
    return int(round(statistics.median(values))) if values else 0


def build_sheet(entry):
    path = IMG_DIR / entry["file"]
    if not path.is_file():
        raise SystemExit("missing WebP: %s (run --import first)" % path)
    im = Image.open(path).convert("RGBA")
    cols, rows = entry["cols"], entry["rows"]
    if im.width % cols or im.height % rows:
        raise SystemExit("%s: size %dx%d is not an exact %dx%d grid" % (entry["file"], im.width, im.height, cols, rows))
    cw, ch, rects = trimmed_rects(im, cols, rows)
    first = [r for r in rects[:4] if r is not None] or [r for r in rects if r is not None]
    if not first:
        raise SystemExit("%s: no visible pixels" % entry["file"])
    pivot = entry["pivot"]
    kind = entry["kind"]
    if pivot == "feet":
        pivot_dy = median([rects[i][1] + rects[i][3] - (i // cols) * ch for i in range(min(4, len(rects))) if rects[i] is not None])
        pivot_dx = cw / 2.0
    elif pivot == "left":
        # stream sheets: x = left cell edge, y = median vertical centre of the content over the loop frames
        mids = [rects[i][1] + rects[i][3] / 2.0 - (i // cols) * ch for i in range(len(rects)) if rects[i] is not None]
        pivot_dx, pivot_dy = 0.0, float(int(round(statistics.median(mids))))
    elif pivot == "base":
        # ground line = lowest solid pixel over all frames (the sheets do not sit on the cell bottom)
        ground = max(rects[i][1] + rects[i][3] - (i // cols) * ch for i in range(len(rects)) if rects[i] is not None)
        pivot_dx, pivot_dy = cw / 2.0, float(ground)
    else:
        pivot_dx, pivot_dy = cw / 2.0, ch / 2.0
    if kind in ("boss", "enemy", "dragon"):
        ref_h = median([r[3] for r in first])
    else:
        ref_h = max(r[3] for r in rects if r is not None)
    frames = []
    for idx, r in enumerate(rects):
        col, row = idx % cols, idx // cols
        if r is None:
            frames.append([col * cw, row * ch, 0, 0, 0, 0])
            continue
        # offsets are relative to the pivot inside the same cell, in sheet pixels
        ox = r[0] - (col * cw + pivot_dx)
        oy = r[1] - (row * ch + pivot_dy)
        frames.append([r[0], r[1], r[2], r[3], int(round(ox)), int(round(oy))])
    if pivot == "base":
        bottoms = [f[5] + f[3] for f in frames if f[3] > 0]
        if max(bottoms) != 0:
            raise SystemExit("%s: base sheet max content bottom offset is %d, expected 0" % (entry["file"], max(bottoms)))
    return {
        "file": entry["file"],
        "w": im.width,
        "h": im.height,
        "cols": cols,
        "rows": rows,
        "cw": cw,
        "ch": ch,
        "pivot": pivot,
        "refH": ref_h,
        "default": entry["default"],
        "clips": entry["clips"],
        "frames": frames,
    }


def load_catalog():
    if not CATALOG.is_file():
        raise SystemExit("missing %s (run --import first)" % CATALOG)
    return json.loads(CATALOG.read_text(encoding="utf-8"))


def compute():
    cat = load_catalog()
    for entry in cat["sheets"]:
        # clip tables live in code; the catalog mirrors them
        cols, rows, _pivot, kind = SHEETS[entry["id"]]
        entry["default"], entry["clips"] = clips_for(entry["id"], kind, cols * rows)
    atlas = {"version": 1, "sheets": {}}
    prov = {"version": 1, "tool": TOOL, "note": NOTE, "files": []}
    for entry in cat["sheets"]:
        atlas["sheets"][entry["id"]] = build_sheet(entry)
        data = (IMG_DIR / entry["file"]).read_bytes()
        prov["files"].append({
            "file": entry["file"],
            "sha256": sha256_bytes(data),
            "bytes": len(data),
            "prompt": entry["prompt"],
            "tool": TOOL,
            "source": entry["source"],
            "sourceSha256": entry["sourceSha256"],
            "note": NOTE,
        })
    return dump_json(atlas), dump_json(prov), dump_json(cat)


def main(argv):
    if argv[:1] == ["--import"]:
        if len(argv) < 3:
            raise SystemExit("usage: --import <rawdir> <promptsdir> [sheet-id ...]")
        do_import(argv[1], argv[2], argv[3:])
        return 0
    check = argv[:1] == ["--check"]
    atlas_text, prov_text, cat_text = compute()
    total = sum(f["bytes"] for f in json.loads(prov_text)["files"])
    if check:
        stale = []
        for path, text in ((ATLAS, atlas_text), (PROVENANCE, prov_text), (CATALOG, cat_text)):
            if not path.is_file() or path.read_text(encoding="utf-8") != text:
                stale.append(str(path.relative_to(ROOT)))
        if stale:
            print("stale: " + ", ".join(stale) + " (run: py -3.13 scripts/build_runenpfad_art.py --write)")
            return 1
        print("runenpfad art up to date: %d sheets, %d bytes" % (len(SHEETS), total))
        return 0
    ATLAS.write_text(atlas_text, encoding="utf-8", newline="\n")
    PROVENANCE.write_text(prov_text, encoding="utf-8", newline="\n")
    CATALOG.write_text(cat_text, encoding="utf-8", newline="\n")
    print("wrote atlas.json + provenance.json + catalog.json (%d WebP bytes)" % total)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
