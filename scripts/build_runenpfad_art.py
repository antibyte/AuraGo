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
import re
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
ALPHA_CUT_NPC = 40    # NPC encounter sheets come with a faint haze up to alpha ~40
ALPHA_CUT_PORTRAIT = 110  # busts carry a pale semi-transparent halo around the silhouette
ALPHA_BOX = 48        # trimmed rectangles measure only solid pixels
MAX_CELL = 256
MAX_CELL_BOSS = 340
MAX_CELL_DRAGON = 360
MAX_CELL_BG_PROPS = 384
MAX_CELL_DUEL = 384   # close-up duel sheets and dialogue portraits
WEBP_QUALITY_NPC = 82
LAZY_KINDS = ("portrait", "duel")
# new sheets (places / village / story round); the pre-existing bg-* sheets stay eager in the atlas
LAZY_ID_RE = re.compile(r"^(tdp|td|tex|fac|cutfg|cut|vil)-")
TD_CHAR_KINDS = ("td-hero", "td-mob", "td-folk")   # top-down characters, treated like npc sheets
MAX_CELL_TD_PROPS = 256
MAX_CELL_FACADE = 512
TEX_CELL = 320        # tex sheets: opaque 2x2, every cell exactly 320x320
WEBP_QUALITY_TEX = 86
CUT_SIZE = (1280, 720)
WEBP_QUALITY_TD = 70   # new top-down characters, props and facades (alpha_quality stays 100)
WEBP_QUALITY_CUT = 80  # story panels (opaque) and cutfg overlays
MAX_WIDTH_BG_FAR = 2048
WEBP_QUALITY_BG = 80
BIOMES = ("woodland", "cavern", "desert", "glacier", "citadel")

NOTE = "Original generated art for AuraGo Runenpfad; not copied from any third-party game or asset pack."
TOOL = "Codex image generation"

# id -> (cols, rows, pivot, kind)
SHEETS = {
    "boss-boar": (4, 3, "feet", "boss"),
    "boss-goblin": (4, 3, "feet", "boss"),
    "boss-orb": (4, 3, "center", "boss"),
    "boss-quad": (4, 3, "center", "boss"),
    "boss-mech": (4, 3, "feet", "boss"),
    "enemy-sporecap": (4, 3, "feet", "enemy"),
    "enemy-bell-toad": (4, 3, "self", "enemy"),
    "enemy-lantern-moth": (4, 3, "center", "enemy"),
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
    "prop-waystones": (5, 1, "feet", "prop"),
    "prop-moon-thorns": (2, 1, "center", "prop"),
    "creature-sky-ray": (2, 2, "center", "creature"),
    "prop-lever": (4, 1, "feet", "prop"),
    "prop-brazier": (4, 2, "feet", "prop"),
    "prop-plate": (4, 1, "feet", "prop"),
    "prop-oracle": (4, 1, "feet", "prop"),
    "prop-swing": (4, 1, "center", "prop"),
    # round 5: painted parallax backgrounds + grounded fire
    "fx-groundfire": (4, 3, "base", "fx"),
    "fx-torchflame": (4, 2, "base", "fx"),
    "bg-cavern-ceiling": (1, 1, "top", "bg-far"),
    # NPC encounters phase 1: level sprites, dialogue portraits, duel sheets (lazy), duel FX, emote icons
    "fx-duel-spells": (4, 4, "center", "fx"),
    "ui-npc-icons": (4, 2, "center", "ui"),
    "duel-knight": (4, 4, "feet", "duel"),
    "duel-ranger": (4, 4, "feet", "duel"),
}
NPC_IDS = ("witch", "wizard", "merchant", "chief", "seer", "alchemist")
for _n in NPC_IDS:
    SHEETS["npc-%s" % _n] = (4, 2, "feet", "npc")
    SHEETS["portrait-%s" % _n] = (3, 1, "bottom", "portrait")
    SHEETS["duel-%s" % _n] = (4, 4, "feet", "duel")
for _b in BIOMES:
    SHEETS["bg-%s-far" % _b] = (1, 1, "bottom", "bg-far")
    SHEETS["bg-%s-mid" % _b] = (4, 2, "feet", "bg-props")
    SHEETS["bg-%s-near" % _b] = (4, 2, "feet", "bg-props")

# places / village / story round: top-down characters, props, textures, facades, villagers, cutscene art
for _h in ("knight", "ranger"):
    SHEETS["td-%s" % _h] = (4, 4, "feet", "td-hero")
for _m in ("rat", "slime", "bat", "spider", "mummy", "icewisp", "clockbug"):
    SHEETS["td-%s" % _m] = (4, 2, "feet", "td-mob")
for _f in ("village", "guests", "world", "story-a", "story-b"):
    SHEETS["td-folk-%s" % _f] = (4, 4, "feet", "td-folk")
for _p in ("tavern", "trade", "cave", "ruin"):
    SHEETS["tdp-%s" % _p] = (4, 4, "self", "td-props")
for _t in ("wood", "cave", "tomb", "ice", "works"):
    SHEETS["tex-%s" % _t] = (2, 2, "corner", "tex")
for _fa in ("village-a", "village-b", "wild-a", "wild-b"):
    SHEETS["fac-%s" % _fa] = (2, 2, "self", "facade")
SHEETS["vil-props"] = (4, 2, "feet", "bg-props")
for _n in ("villager", "guard"):
    SHEETS["npc-%s" % _n] = (4, 2, "feet", "npc")
for _n in ("elder", "innkeeper", "smith", "shopkeeper", "bard", "hermit", "barkeep", "scribe", "climber", "automaton"):
    SHEETS["portrait-%s" % _n] = (3, 1, "bottom", "portrait")
CUT_IDS = ("prologue-1", "prologue-2", "prologue-3", "prologue-4", "prologue-5",
           "intro2-1", "intro2-2", "intro3-1", "intro3-2", "intro4-1", "intro4-2", "intro5-1", "intro5-2",
           "after1-1", "after1-2", "after2-1", "after2-2", "after3-1", "after3-2", "after4-1", "after4-2",
           "finale-1", "finale-2", "finale-3", "end-friend", "end-neutral", "end-feared")
for _c in CUT_IDS:
    SHEETS["cut-%s" % _c] = (1, 1, "corner", "cut")
for _b in BIOMES:
    SHEETS["cutfg-%s" % _b] = (1, 1, "corner", "cutfg")
# round 9: painted props that replace primitive drawing (chains, conveyors, projectiles, warning rings, pickup pops)
SHEETS["prop-chain"] = (4, 1, "center", "prop")
SHEETS["prop-conveyor"] = (3, 1, "center", "prop")
SHEETS["fx-projectiles"] = (4, 2, "center", "fx")
SHEETS["fx-telegraph"] = (2, 2, "center", "fx")
SHEETS["fx-pickup"] = (4, 2, "center", "fx")
MAX_CELL_BY_ID = {"fx-telegraph": 384}   # thin rune lines need the larger cell to survive the downscale
# chronicle round: interior mechanics props, room guardians, the observatory, Edda and the true ending
for _p in ("mech", "lore", "stars"):
    SHEETS["tdp-%s" % _p] = (4, 4, "self", "td-props")
for _g in ("brood", "weaver", "tombward", "frostheart", "boilerward", "counter"):
    SHEETS["td-boss-%s" % _g] = (4, 2, "feet", "td-mob")
    MAX_CELL_BY_ID["td-boss-%s" % _g] = 384   # guardians are drawn about twice as large as normal room monsters
SHEETS["td-folk-epilogue"] = (4, 4, "feet", "td-folk")
for _n in ("edda", "core"):
    SHEETS["portrait-%s" % _n] = (3, 1, "bottom", "portrait")
SHEETS["fac-observatory"] = (2, 2, "self", "facade")
for _c in ("obs-1", "trueend-1", "trueend-2", "trueend-3"):
    SHEETS["cut-%s" % _c] = (1, 1, "corner", "cut")


def is_npc_sheet(sheet_id, kind):
    return kind in ("npc", "portrait", "duel", "ui") + TD_CHAR_KINDS or sheet_id == "fx-duel-spells"


def is_lazy(sheet_id, kind):
    """Sheets the loader keeps out of the boot load (decoded on demand)."""
    return kind in LAZY_KINDS or sheet_id == "fx-duel-spells" or bool(LAZY_ID_RE.match(sheet_id))


def clips_for(sheet_id, kind, count):
    if kind == "td-hero":
        clips = {
            "down": {"frames": [0, 1, 2, 3], "ms": 110, "loop": True},
            "up": {"frames": [4, 5, 6, 7], "ms": 110, "loop": True},
            "right": {"frames": [8, 9, 10, 11], "ms": 110, "loop": True},
        }
        for name, idx in (("atk_down", 12), ("atk_up", 13), ("atk_right", 14), ("hurt", 15)):
            clips[name] = {"frames": [idx], "ms": 200, "loop": False}
        return "down", clips
    if kind == "td-mob":
        return "move", {
            "move": {"frames": [0, 1, 2, 3], "ms": 120, "loop": True},
            "tell": {"frames": [4], "ms": 200, "loop": False},
            "attack": {"frames": [5], "ms": 200, "loop": False},
            "hurt": {"frames": [6], "ms": 200, "loop": False},
            "death": {"frames": [7], "ms": 200, "loop": False},
        }
    if kind == "td-folk":
        clips = {}
        for r in range(4):
            clips["r%d_idle" % r] = {"frames": [4 * r, 4 * r + 1], "ms": 420, "loop": True}
            clips["r%d_talk" % r] = {"frames": [4 * r + 2, 4 * r + 3], "ms": 220, "loop": True}
        return "r0_idle", clips
    if kind in ("td-props", "facade", "tex"):
        letter = {"td-props": "p", "facade": "f", "tex": "t"}[kind]
        return letter + "0", {"%s%d" % (letter, i): {"frames": [i], "ms": 200, "loop": False} for i in range(count)}
    if kind in ("cut", "cutfg"):
        return "full", {"full": {"frames": [0], "ms": 200, "loop": False}}
    if kind == "npc":
        return "idle", {
            "idle": {"frames": [0, 1], "ms": 320, "loop": True},
            "talk": {"frames": [2, 3], "ms": 220, "loop": True},
            "alert": {"frames": [4], "ms": 200, "loop": False},
            "hurt": {"frames": [5], "ms": 200, "loop": False},
            "cast": {"frames": [6, 7], "ms": 160, "loop": True},
        }
    if kind == "portrait":
        return "neutral", {
            "neutral": {"frames": [0], "ms": 200, "loop": False},
            "happy": {"frames": [1], "ms": 200, "loop": False},
            "angry": {"frames": [2], "ms": 200, "loop": False},
        }
    if kind == "duel" and sheet_id in ("duel-knight", "duel-ranger"):
        return "idle", {
            "idle": {"frames": [0, 1], "ms": 260, "loop": True},
            "run": {"frames": [2, 3], "ms": 120, "loop": True},
            "jump": {"frames": [4], "ms": 200, "loop": False},
            "combo": {"frames": [5, 6, 7], "ms": 95, "loop": False},
            "block": {"frames": [8], "ms": 200, "loop": False},
            "roll": {"frames": [9, 10], "ms": 80, "loop": False},
            "hurt": {"frames": [11], "ms": 200, "loop": False},
            "special": {"frames": [12, 13], "ms": 140, "loop": False},
            "defeat": {"frames": [14], "ms": 420, "loop": False},
            "victory": {"frames": [15], "ms": 300, "loop": False},
        }
    if kind == "duel":
        return "idle", {
            "idle": {"frames": [0, 1], "ms": 280, "loop": True},
            "walk": {"frames": [2, 3], "ms": 180, "loop": True},
            "a_tell": {"frames": [4], "ms": 200, "loop": False},
            "a": {"frames": [5], "ms": 200, "loop": False},
            "b_tell": {"frames": [6], "ms": 200, "loop": False},
            "b": {"frames": [7], "ms": 200, "loop": False},
            "c_tell": {"frames": [8], "ms": 200, "loop": False},
            "c": {"frames": [9], "ms": 200, "loop": False},
            "block": {"frames": [10], "ms": 200, "loop": False},
            "hurt": {"frames": [11], "ms": 200, "loop": False},
            "enraged": {"frames": [12], "ms": 200, "loop": False},
            "defeat": {"frames": [13, 14], "ms": 420, "loop": False},
            "taunt": {"frames": [15], "ms": 300, "loop": False},
        }
    if sheet_id == "fx-duel-spells":
        return "hex", {
            "hex": {"frames": [0, 1, 2, 3], "ms": 80, "loop": True},
            "arcane": {"frames": [4, 5, 6, 7], "ms": 70, "loop": True},
            "acid": {"frames": [8, 9, 10, 11], "ms": 110, "loop": False},
            "vortex": {"frames": [12, 13, 14, 15], "ms": 90, "loop": True},
        }
    if sheet_id == "ui-npc-icons":
        return "talk", {name: {"frames": [i], "ms": 200, "loop": False}
                        for i, name in enumerate(("talk", "alert", "ask", "coins", "heart", "duel", "quest", "trade"))}
    if kind == "bg-far":
        return "band", {"band": {"frames": [0], "ms": 200, "loop": False}}
    if kind == "bg-props":
        return "p0", {"p%d" % i: {"frames": [i], "ms": 200, "loop": False} for i in range(8)}
    if sheet_id == "fx-groundfire":
        return "loop", {
            "ignite": {"frames": [0, 1], "ms": 90, "loop": False},
            "loop": {"frames": [2, 3, 4, 5, 6, 7, 8, 9], "ms": 80, "loop": True},
            "fade": {"frames": [10, 11], "ms": 120, "loop": False},
        }
    if sheet_id == "fx-torchflame":
        return "loop", {"loop": {"frames": list(range(8)), "ms": 90, "loop": True}}
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
    if sheet_id == "prop-waystones":
        return BIOMES[0], {name: {"frames": [i], "ms": 200, "loop": False} for i, name in enumerate(BIOMES)}
    if sheet_id == "prop-moon-thorns":
        return "roller", {name: {"frames": [i], "ms": 200, "loop": False} for i, name in enumerate(("roller", "moon"))}
    if sheet_id == "creature-sky-ray":
        return "glide", {"glide": {"frames": [0, 1, 0, 2, 3, 2], "ms": 175, "loop": True}}
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
    if sheet_id in ("prop-chain", "prop-conveyor", "fx-projectiles"):
        names = {
            "prop-chain": ("link", "mount", "shackle", "rail"),
            "prop-conveyor": ("left", "mid", "right"),
            "fx-projectiles": ("arrow", "spear", "missile", "orb", "cannonball", "spore", "crest", "shard"),
        }[sheet_id]
        return names[0], {name: {"frames": [i], "ms": 200, "loop": False} for i, name in enumerate(names)}
    if sheet_id == "fx-telegraph":
        return "ring", {"ring": {"frames": [0, 1, 2, 3], "ms": 110, "loop": True}}
    if sheet_id == "fx-pickup":
        return "coin", {
            "coin": {"frames": [0, 1, 2, 3], "ms": 55, "loop": False},
            "gem": {"frames": [4, 5, 6, 7], "ms": 60, "loop": False},
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


def clean_alpha(im, cut=ALPHA_CUT):
    """Zero the near-invisible fog (garbage RGB under low alpha)."""
    r, g, b, a = im.split()
    a = a.point(lambda v: 0 if v < cut else v)
    im = Image.merge("RGBA", (r, g, b, a))
    # RGB below zero alpha is meaningless; make it deterministic and cheap to encode
    black = Image.new("RGBA", im.size, (0, 0, 0, 0))
    mask = a.point(lambda v: 255 if v > 0 else 0)
    return Image.composite(im, black, mask)


def cell_size(src_w, src_h, cols, rows, limit):
    cw, ch = src_w / cols, src_h / rows
    k = min(1.0, limit / max(cw, ch))
    return max(8, int(round(cw * k))), max(8, int(round(ch * k)))


def crop_16_9(im):
    """Centre-crop to exactly 16:9."""
    w, h = im.size
    if w * 9 > h * 16:
        nw = int(round(h * 16 / 9.0))
        x0 = (w - nw) // 2
        return im.crop((x0, 0, x0 + nw, h))
    nh = int(round(w * 9 / 16.0))
    y0 = (h - nh) // 2
    return im.crop((0, y0, w, y0 + nh))


def import_opaque(src, cols, rows, kind):
    """Opaque kinds: no alpha channel at all. tex = exact 320 cells, cut = cropped 16:9 panel."""
    im = Image.open(src).convert("RGB")
    if kind == "tex":
        cw = ch = TEX_CELL
        return im.resize((cw * cols, ch * rows), Image.LANCZOS), cw, ch
    cw, ch = CUT_SIZE
    return crop_16_9(im).resize(CUT_SIZE, Image.LANCZOS), cw, ch


def catalog_entry(sheet_id, src, raw_bytes, prompt_dir, cols, rows, pivot, kind):
    default, clips = clips_for(sheet_id, kind, cols * rows)
    prompt_file = prompt_dir / (sheet_id + ".txt")
    prompt = prompt_file.read_text(encoding="utf-8").strip() if prompt_file.is_file() else ""
    return {
        "id": sheet_id,
        "file": sheet_id + ".webp",
        "cols": cols,
        "rows": rows,
        "pivot": pivot,
        "kind": kind,
        "default": default,
        "clips": clips,
        "prompt": prompt,
        "tool": TOOL,
        "source": src.name,
        "sourceSha256": sha256_bytes(raw_bytes),
    }


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
        if kind in ("tex", "cut"):
            out, cw, ch = import_opaque(src, cols, rows, kind)
            out.save(IMG_DIR / (sheet_id + ".webp"), "WEBP",
                     quality=WEBP_QUALITY_TEX if kind == "tex" else WEBP_QUALITY_CUT, method=6)
            sheets.append(catalog_entry(sheet_id, src, raw_bytes, prompt_dir, cols, rows, pivot, kind))
            print("imported %-18s %dx%d cell %dx%d" % (sheet_id, cols, rows, cw, ch))
            continue
        im = Image.open(src).convert("RGBA")
        if kind == "cutfg":
            im = crop_16_9(im)
        cut = ALPHA_CUT_NPC if (is_npc_sheet(sheet_id, kind) or kind in ("td-props", "facade")) else ALPHA_CUT
        if kind == "portrait":
            cut = ALPHA_CUT_PORTRAIT
        im = clean_alpha(im, cut)
        limit = MAX_CELL_DRAGON if kind == "dragon" else (MAX_CELL_BOSS if kind == "boss" else MAX_CELL)
        if kind in ("duel", "portrait"):
            limit = MAX_CELL_DUEL
        if kind == "bg-props":
            limit = MAX_CELL_BG_PROPS
        if kind == "td-props":
            limit = MAX_CELL_TD_PROPS
        if kind == "facade":
            limit = MAX_CELL_FACADE
        limit = MAX_CELL_BY_ID.get(sheet_id, limit)
        if kind == "cutfg":
            cw, ch = CUT_SIZE
        elif kind == "bg-far":
            k = min(1.0, MAX_WIDTH_BG_FAR / float(im.width))
            cw, ch = max(8, int(round(im.width * k))), max(8, int(round(im.height * k)))
        else:
            cw, ch = cell_size(im.width, im.height, cols, rows, limit)
        out = im.resize((cw * cols, ch * rows), Image.LANCZOS)
        out = clean_alpha(out, cut)
        quality = WEBP_QUALITY_BG if kind in ("bg-far", "bg-props") else 90
        if kind == "cutfg":
            quality = WEBP_QUALITY_CUT
        if is_npc_sheet(sheet_id, kind) or kind in ("td-props", "facade"):
            quality = WEBP_QUALITY_NPC
        if kind in TD_CHAR_KINDS or kind in ("td-props", "facade"):
            quality = WEBP_QUALITY_TD
        out.save(IMG_DIR / (sheet_id + ".webp"), "WEBP", quality=quality, method=6, alpha_quality=100, exact=False)
        sheets.append(catalog_entry(sheet_id, src, raw_bytes, prompt_dir, cols, rows, pivot, kind))
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
    kind = entry["kind"]
    if kind in ("tex", "cut", "cutfg"):
        # full cells, no trimming, pivot = top-left corner of each cell
        if kind == "tex" and (cw, ch) != (TEX_CELL, TEX_CELL):
            raise SystemExit("%s: tex cell is %dx%d, expected %d" % (entry["file"], cw, ch, TEX_CELL))
        if kind != "tex" and (cw, ch) != CUT_SIZE:
            raise SystemExit("%s: size %dx%d, expected %dx%d" % (entry["file"], cw, ch, CUT_SIZE[0], CUT_SIZE[1]))
        frames = [[(i % cols) * cw, (i // cols) * ch, cw, ch, 0, 0] for i in range(cols * rows)]
        out = {"file": entry["file"], "w": im.width, "h": im.height, "cols": cols, "rows": rows, "cw": cw, "ch": ch,
               "pivot": "corner", "refH": ch, "default": entry["default"], "clips": entry["clips"], "frames": frames}
        if is_lazy(entry["id"], kind):
            out["lazy"] = True
        return out
    first = [r for r in rects[:4] if r is not None] or [r for r in rects if r is not None]
    if not first:
        raise SystemExit("%s: no visible pixels" % entry["file"])
    pivot = entry["pivot"]
    kind = entry["kind"]
    if pivot == "bottom":
        # far band: baseline = lowest solid pixel (the flat bottom edge of the band)
        pivot_dx = cw / 2.0
        pivot_dy = float(max(rects[i][1] + rects[i][3] for i in range(len(rects)) if rects[i] is not None))
    elif pivot == "top":
        pivot_dx = cw / 2.0
        pivot_dy = float(min(rects[i][1] for i in range(len(rects)) if rects[i] is not None))
    elif pivot == "feet" and kind == "bg-props":
        # props share one baseline: the lowest content bottom over all cells, relative to the cell
        pivot_dy = max(rects[i][1] + rects[i][3] - (i // cols) * ch for i in range(len(rects)) if rects[i] is not None)
        pivot_dx = cw / 2.0
    elif pivot == "feet" and kind in TD_CHAR_KINDS:
        # one shared baseline over all frames (every row/character of the sheet)
        pivot_dy = median([rects[i][1] + rects[i][3] - (i // cols) * ch for i in range(len(rects)) if rects[i] is not None])
        pivot_dx = cw / 2.0
    elif pivot == "self":
        pivot_dx = pivot_dy = 0.0  # unused: every frame is anchored at its own bottom centre
    elif pivot == "feet":
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
    if kind in ("boss", "enemy", "dragon", "npc", "duel") + TD_CHAR_KINDS:
        ref_h = median([r[3] for r in first])
    else:
        ref_h = max(r[3] for r in rects if r is not None)
    frames = []
    for idx, r in enumerate(rects):
        col, row = idx % cols, idx // cols
        if r is None:
            frames.append([col * cw, row * ch, 0, 0, 0, 0])
            continue
        if pivot == "self":
            # x = centre of the content box, y = its bottom edge
            frames.append([r[0], r[1], r[2], r[3], -int(round(r[2] / 2.0)), -r[3]])
            continue
        # offsets are relative to the pivot inside the same cell, in sheet pixels
        ox = r[0] - (col * cw + pivot_dx)
        oy = r[1] - (row * ch + pivot_dy)
        if entry["id"] == "creature-sky-ray" and row == 1:
            # The downstroke poses sit higher in their source cells. Anchor the rider to the dorsal markings.
            oy += round(ch * 0.06)
        frames.append([r[0], r[1], r[2], r[3], int(round(ox)), int(round(oy))])
    if pivot == "base":
        bottoms = [f[5] + f[3] for f in frames if f[3] > 0]
        if max(bottoms) != 0:
            raise SystemExit("%s: base sheet max content bottom offset is %d, expected 0" % (entry["file"], max(bottoms)))
    out = {
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
    if is_lazy(entry["id"], kind):
        out["lazy"] = True
    return out


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
            "tool": entry.get("tool", "Codex CLI image generation"),
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
