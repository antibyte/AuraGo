"""Generate distinct 20x20 enemy frames and 24x24 boss art; patch galaxa-sprites.js.

Note: The JS runtime expands these to 24x24 (enemies) and 32x32 (boss/player)
via expandP() + withPx() post-processing. This generator produces the base art
at the original sizes; coordinate indices in transform functions are relative
to those base sizes.
"""
from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SPRITES = ROOT / "ui/js/desktop/apps/galaxa-sprites.js"

CURRENT_VERSION = 5  # bump when art changes


def pad(rows: list[str], w: int, h: int) -> list[str]:
    out = []
    for r in rows:
        r = (r + "0" * w)[:w]
        out.append(r)
    while len(out) < h:
        out.append("0" * w)
    return out[:h]


def p_block(rows: list[str], indent: str = "                    ") -> str:
    lines = [indent + "p(["]
    for i in range(0, len(rows), 4):
        chunk = ", ".join("'" + rows[j] + "'" for j in range(i, min(i + 4, len(rows))))
        lines.append(indent + "    " + chunk + ",")
    lines[-1] = lines[-1].rstrip(",")
    lines.append(indent + "].join('\\n')),")
    return "\n".join(lines)


def frames_block(name: str, frames: list[list[str]]) -> str:
    parts = [f"                {name}: ["]
    for fr in frames:
        parts.append(p_block(fr))
    parts[-1] = parts[-1].rstrip(",")
    parts.append("                ],")
    return "\n".join(parts)


# --- 20x20 art: digit maps to each type's palette keys ---

BEE0 = pad(
    [
        "00000004444000000000",
        "00000045554000000000",
        "00000455554400000000",
        "00004565565400000000",
        "000456f66f65400000000",
        "00456666666540000000",
        "04566666666654000000",
        "45666666666665400000",
        "04566666666654000000",
        "00456666666540000000",
        "00045666665400000000",
        "00004565565400000000",
        "00000455554400000000",
        "00000045554000000000",
        "00000004444000000000",
    ],
    20,
    20,
)


def wing_bee(base: list[str], spread: int) -> list[str]:
    r = [row[:] for row in base]
    for y in (7, 8, 9):
        if spread:
            if y < len(r) and len(r[y]) >= 20:
                row = list(r[y])
                if row[0] == "0":
                    row[0] = "4"
                if row[19] == "0":
                    row[19] = "4"
                r[y] = "".join(row)
        else:
            if y < len(r):
                row = list(r[y])
                if row[0] == "4":
                    row[0] = "0"
                if row[19] == "4":
                    row[19] = "0"
                r[y] = "".join(row)
    return r


BF0 = pad(
    [
        "00000000660000000000",
        "00000067766000000000",
        "00000677776600000000",
        "00006787787600000000",
        "00067877787600000000",
        "00678777778760000000",
        "06787766667876000000",
        "06787666667876000000",
        "00678777778760000000",
        "00067877787600000000",
        "00006787787600000000",
        "00000677776600000000",
        "00000067766000000000",
        "00000000660000000000",
    ],
    20,
    20,
)


def bf_wings(base: list[str], high: bool) -> list[str]:
    r = [list(row) for row in base]
    for y in (2, 3, 4, 10, 11, 12):
        if y >= len(r):
            continue
        for x in (2, 3, 16, 17):
            if x < len(r[y]) and r[y][x] in "067":
                r[y][x] = "7" if high else "6"
    return ["".join(row) for row in r]


STALKER0 = pad(
    [
        "00000000880000000000",
        "00000089998000000000",
        "00000899999800000000",
        "000089aaaaa980000000",
        "00089aaaaaaa98000000",
        "0089aaaaaaaa98000000",
        "0089aaaaaaaa98000000",
        "00089aaaaaaa98000000",
        "000089aaaaa980000000",
        "00000899999800000000",
        "00000089998000000000",
        "00000000880000000000",
    ],
    20,
    20,
)


def stalker_vis(base: list[str], bright: bool) -> list[str]:
    r = [list(row) for row in base]
    y = 6
    if y < len(r):
        for x in range(4, 16):
            if x < len(r[y]):
                r[y][x] = "a" if bright else "9"
    return ["".join(row) for row in r]


SNIPER0 = pad(
    [
        "00000000044000000000",
        "00000000445400000000",
        "00000004455544000000",
        "00000044555554400000",
        "00000445555554400000",
        "00004455555554400000",
        "00044555555554400000",
        "00445555555554400000",
        "00044555555554400000",
        "00004455555554400000",
        "00000445555554400000",
        "00000044555554400000",
        "00000004455544000000",
        "00000000445400000000",
        "00000000044000000000",
    ],
    20,
    20,
)


def sniper_glow(base: list[str], n: int) -> list[str]:
    r = [list(row) for row in base]
    for y in range(0, min(4 + n, len(r))):
        if 9 < len(r[y]):
            r[y][9] = "6"
    return ["".join(row) for row in r]


HUNTER0 = pad(
    [
        "00000000000000000000",
        "00000008800000000000",
        "00000089980000000000",
        "00000899998000000000",
        "000089aa9a9800000000",
        "00089aaaaa9800000000",
        "0089aaaaaa9800000000",
        "089aaaaaaa9800000000",
        "0089aaaaaa9800000000",
        "00089aaaaa9800000000",
        "000089aa9a9800000000",
        "00000899998000000000",
        "00000089980000000000",
        "00000008800000000000",
    ],
    20,
    20,
)


def hunter_claws(base: list[str], out: bool) -> list[str]:
    r = [list(row) for row in base]
    pairs = [(10, 2), (10, 17), (11, 1), (11, 18)] if out else []
    for y, x in pairs:
        if y < len(r) and x < len(r[y]):
            r[y][x] = "b"
    return ["".join(row) for row in r]


SPINNER0 = pad(
    [
        "00000000000000000000",
        "00000000660000000000",
        "00000067766000000000",
        "00000677777600000000",
        "00006777777660000000",
        "00067777777766000000",
        "00677777777776600000",
        "00067777777766000000",
        "00006777777660000000",
        "00000677777600000000",
        "00000067766000000000",
        "00000000660000000000",
    ],
    20,
    20,
)


def spinner_spoke(base: list[str], mode: int) -> list[str]:
    r = [pad([row], 20, 1)[0] for row in base]
    r = [list(row) for row in pad(r, 20, 20)]
    cx, cy = 9, 6
    spokes = [
        [(cx, cy - 2), (cx, cy + 2)],
        [(cx - 2, cy), (cx + 2, cy)],
        [(cx - 2, cy - 2), (cx + 2, cy + 2)],
        [(cx - 2, cy + 2), (cx + 2, cy - 2)],
    ]
    for x, y in spokes[mode % 4]:
        if 0 <= y < len(r) and 0 <= x < len(r[y]):
            r[y][x] = "7"
    return ["".join(row) for row in r]


BOMBER0 = pad(
    [
        "00000000888000000000",
        "00000089998000000000",
        "00000899999800000000",
        "00008999999980000000",
        "00089999999998000000",
        "00899999999999800000",
        "00899999999999800000",
        "00089999999998000000",
        "00008999999980000000",
        "00000899999800000000",
        "00000089998000000000",
        "00000008880000000000",
        "00000000999000000000",
        "00000000088000000000",
    ],
    20,
    20,
)


def bomber_core(base: list[str], open_bay: bool) -> list[str]:
    r = [list(row) for row in base]
    for y in (5, 6):
        for x in (8, 9, 10, 11):
            if y < len(r) and x < len(r[y]):
                r[y][x] = "a" if open_bay else "9"
    if open_bay and 12 < len(r):
        row = list(r[12])
        for x in (8, 9, 10, 11):
            if x < len(row):
                row[x] = "a"
        r[12] = "".join(row)
    return ["".join(row) for row in r]


LASHER0 = pad(
    [
        "00000000444000000000",
        "00000045554000000000",
        "00000455555400000000",
        "00004566665400000000",
        "00045666665400000000",
        "00456666665400000000",
        "00456666665400000000",
        "00045666665400000000",
        "00004566665400000000",
        "00000455555400000000",
        "00000045554000000000",
        "00000004444000000000",
    ],
    20,
    20,
)


def lasher_tendrils(base: list[str], len_: int) -> list[str]:
    r = [list(row) for row in base]
    for i, x in enumerate((6, 9, 12, 14)):
        for dy in range(1, 1 + len_):
            y = 11 + dy
            if y < len(r) and x < len(r[y]):
                r[y][x] = "5" if dy < len_ else "6"
    return ["".join(row) for row in r]


WEAVER0 = pad(
    [
        "00000000888000000000",
        "00000089998000000000",
        "00000899999800000000",
        "00008999999980000000",
        "00089999999998000000",
        "00899999999999800000",
        "00899999999999800000",
        "00089999999998000000",
        "00008999999980000000",
        "00000899999800000000",
        "00000089998000000000",
        "00000000888000000000",
    ],
    20,
    20,
)


def weaver_nodes(base: list[str], side: int) -> list[str]:
    r = [list(row) for row in base]
    y = 6
    if side < 0 and y < len(r):
        for x in (2, 3):
            if x < len(r[y]):
                r[y][x] = "a"
    elif side > 0 and y < len(r):
        for x in (16, 17):
            if x < len(r[y]):
                r[y][x] = "a"
    else:
        for x in (9, 10):
            if y < len(r) and x < len(r[y]):
                r[y][x] = "9"
    return ["".join(row) for row in r]


SPLITTER0 = pad(
    [
        "00000000660000000000",
        "00000067766000000000",
        "00000677777600000000",
        "00006777777660000000",
        "00067777777766000000",
        "00677777777776600000",
        "00677777777776600000",
        "00067777777766000000",
        "00006777777660000000",
        "00000677777600000000",
        "00000067766000000000",
        "00000000660000000000",
    ],
    20,
    20,
)


def splitter_seam(base: list[str], split: bool) -> list[str]:
    r = [list(row) for row in base]
    for y in range(4, 9):
        if y < len(r) and 9 < len(r[y]):
            r[y][9] = "7" if split else "6"
            if split and 8 < len(r[y]):
                r[y][8] = "7"
            if split and 10 < len(r[y]):
                r[y][10] = "7"
    return ["".join(row) for row in r]


SHIELD_BEE0 = pad(
    [
        "00000000004400000000",
        "000000000444400000000",
        "00000000444440000000",
        "00000004444444000000",
        "00000044444444400000",
        "00000444444444400000",
        "00004444554444000000",
        "00044445664444400000",
        "00004444554444000000",
        "00000444444444400000",
        "00000044444444400000",
        "00000004444444000000",
        "00000000444440000000",
        "000000000444400000000",
        "00000000004400000000",
    ],
    20,
    20,
)


def shield_ring(base: list[str], thick: bool) -> list[str]:
    r = [list(row) for row in base]
    ring_y = [1, 2, 12, 13]
    for y in ring_y:
        if y >= len(r):
            continue
        row = list(r[y])
        for x in range(20):
            if row[x] == "0" and (
                (y in (1, 13) and 4 <= x <= 15)
                or (y in (2, 12) and 3 <= x <= 16)
            ):
                row[x] = "3" if thick else "2"
        r[y] = "".join(row)
    return ["".join(row) for row in r]


KAMIKAZE0 = pad(
    [
        "00000000066000000000",
        "00000000666000000000",
        "00000006666000000000",
        "00000066666000000000",
        "00000666666000000000",
        "00006666666000000000",
        "00066666666000000000",
        "00666666666000000000",
        "00066666666000000000",
        "00006666666000000000",
        "00000666666000000000",
        "00000066666000000000",
        "00000006666000000000",
        "00000000666000000000",
        "00000000066000000000",
    ],
    20,
    20,
)


def kamikaze_flame(base: list[str], n: int) -> list[str]:
    r = [list(row) for row in base]
    for y in range(13, min(13 + n, len(r))):
        if 9 < len(r[y]):
            r[y][9] = "5"
        if 8 < len(r[y]):
            r[y][8] = "4"
        if 10 < len(r[y]):
            r[y][10] = "4"
    return ["".join(row) for row in r]


CARRIER0 = pad(
    [
        "00000008888000000000",
        "00000089998000000000",
        "00000899999800000000",
        "00008999999980000000",
        "00089999999998000000",
        "00899999999999800000",
        "08999999999999880000",
        "08999999999999880000",
        "00899999999999800000",
        "00089999999998000000",
        "00008999999980000000",
        "00000899999800000000",
        "00000089998000000000",
        "00000008888000000000",
        "00000000333000000000",
        "00000000333000000000",
    ],
    20,
    20,
)


def carrier_bay(base: list[str], open_: bool) -> list[str]:
    r = [list(row) for row in base]
    for y in (14, 15):
        if y < len(r):
            row = list(r[y])
            for x in (8, 9, 10, 11):
                if x < len(row):
                    row[x] = "b" if open_ else "3"
            r[y] = "".join(row)
    return ["".join(row) for row in r]


TELEPORT0 = pad(
    [
        "00000000660000000000",
        "00000067766000000000",
        "00000677777600000000",
        "00006787787600000000",
        "00067877787600000000",
        "00678777778760000000",
        "06787666667876000000",
        "00678777778760000000",
        "00067877787600000000",
        "00006787787600000000",
        "00000677777600000000",
        "00000067766000000000",
        "00000000660000000000",
    ],
    20,
    20,
)


def tele_phase(base: list[str], corners: bool) -> list[str]:
    r = [list(row) for row in base]
    pts = [(2, 2), (17, 2), (2, 11), (17, 11)] if corners else [(9, 5), (10, 5)]
    for x, y in pts:
        if y < len(r) and x < len(r[y]):
            r[y][x] = "7"
    return ["".join(row) for row in r]


BOSS_BASE = pad(
    [
        "000000008888880000000000",
        "000000089999980000000000",
        "000000899999998000000000",
        "000008999999999800000000",
        "000089999999999980000000",
        "000899999999999998000000",
        "008999999999999999800000",
        "00899bbbbbbbbbb998000000",
        "0899bbbbcccccbbbb99800000",
        "0899bbbcccccccbbb99800000",
        "08999bbccccccccbb999800000",
        "008999bbbbbbbbbb999800000",
        "000899999999999998000000",
        "000089aa999999aa980000000",
        "0000089a999999a9800000000",
        "000000899999998000000000",
        "000000089999980000000000",
        "000000008888880000000000",
        "000000000bb00bb0000000000",
        "000000000bb00bb0000000000",
    ],
    24,
    24,
)


def boss_hit(base: list[str]) -> list[str]:
    r = [list(row) for row in base]
    for y in range(6, 12):
        if y < len(r):
            row = list(r[y])
            for x in range(6, 18):
                if x < len(row) and row[x] in "89bc":
                    row[x] = "a" if row[x] in "bc" else "9"
            r[y] = "".join(row)
    return ["".join(row) for row in r]


def boss_crit(base: list[str]) -> list[str]:
    r = [list(row) for row in boss_hit(base)]
    for y in range(7, 11):
        if y < len(r):
            row = list(r[y])
            for x in (10, 11, 12, 13):
                if x < len(row):
                    row[x] = "c"
            r[y] = "".join(row)
    return ["".join(row) for row in r]


def single_p(name: str, rows: list[str], indent: str = "                ") -> str:
    bl = p_block(rows, indent + "    ")
    return f"{indent}{name}: " + bl[len(indent) + 4 : -1]  # p([...].join)


def main() -> None:
    text = SPRITES.read_text(encoding="utf-8")

    enemy_sections = {
        "bee": [wing_bee(BEE0, s) for s in (0, 1, 1, 0)],
        "bf": [bf_wings(BF0, h) for h in (True, False, True, False)],
        "stalker": [stalker_vis(STALKER0, b) for b in (False, True, True, False)],
        "sniper": [sniper_glow(SNIPER0, n) for n in (0, 1, 2, 3)],
        "hunter": [hunter_claws(HUNTER0, o) for o in (False, True, True, False)],
        "spinner": [spinner_spoke(SPINNER0, m) for m in range(4)],
        "bomber": [bomber_core(BOMBER0, o) for o in (False, True, True, False)],
        "lasher": [lasher_tendrils(LASHER0, L) for L in (1, 2, 3, 2)],
        "weaver": [weaver_nodes(WEAVER0, s) for s in (-1, 0, 1, 0)],
        "splitter": [splitter_seam(SPLITTER0, s) for s in (False, True, True, True)],
        "shield_bee": [shield_ring(SHIELD_BEE0, t) for t in (False, True, True, False)],
        "kamikaze": [kamikaze_flame(KAMIKAZE0, n) for n in (0, 1, 2, 3)],
        "carrier": [carrier_bay(CARRIER0, o) for o in (False, True, True, False)],
        "teleporter": [tele_phase(TELEPORT0, c) for c in (True, False, True, False)],
    }

    def replace_enemy_block(text: str, key: str, repl: str) -> str:
        pat = rf"                {key}: \[.*?\n                \],"
        m = re.search(pat, text, re.S)
        if not m:
            raise SystemExit(f"Failed to find {key}")
        return text[: m.start()] + repl + text[m.end() :]

    def replace_boss_p(text: str, name: str, rows: list[str]) -> str:
        pat = rf"                {name}: p\(\[.*?\]\.join\('\\n'\)\),"
        m = re.search(pat, text, re.S)
        if not m:
            raise SystemExit(f"Failed to find {name}")
        repl = (
            f"                {name}: p([\n"
            + "\n".join("                    '" + r + "'," for r in rows)
            + "\n                ].join('\\n')),\n"
        )
        return text[: m.start()] + repl.rstrip("\n") + text[m.end() :]

    for key, frames in enemy_sections.items():
        text = replace_enemy_block(text, key, frames_block(key, frames))

    text = replace_boss_p(text, "boss", BOSS_BASE)
    text = replace_boss_p(text, "bossHit", boss_hit(BOSS_BASE))
    text = replace_boss_p(text, "bossCrit", boss_crit(BOSS_BASE))

    text = re.sub(
        r"const PREMIUM_PIXEL_ART_VERSION = 'galaxa-premium-v\d+'",
        f"const PREMIUM_PIXEL_ART_VERSION = 'galaxa-premium-v{CURRENT_VERSION}'",
        text,
    )

    # Expand palettes for new digits used
    pal_updates = [
        ("bfC: { 6: '#ff3366', 7: '#44bbff' }", "bfC: { 6: '#ff3366', 7: '#44bbff', 8: '#ff5588' }"),
        ("spinnerC: { 6: '#00cccc', 7: '#44ffff', 8: '#0088aa' }", "spinnerC: { 6: '#00cccc', 7: '#44ffff', 8: '#0088aa' }"),
        ("teleporterC: { 6: '#44ffff', 7: '#66ffff' }", "teleporterC: { 6: '#44ffff', 7: '#66ffff', 8: '#aaffff' }"),
        ("kamikazeC: { 4: '#ff2222', 5: '#ff4444', 6: '#ff6666' }", "kamikazeC: { 4: '#ff2222', 5: '#ff4444', 6: '#ff6666' }"),
        ("carrierC: { 8: '#cc88ff', 9: '#ddaaff', a: '#eeccff', b: '#bb99dd' }", "carrierC: { 8: '#cc88ff', 9: '#ddaaff', a: '#eeccff', b: '#bb99dd', 3: '#8866cc' }"),
        ("shield_beeC: { 2: '#4488ff', 3: '#66aaff', 4: '#ffcc00', 5: '#ffaa00', 6: '#ff4444' }", "shield_beeC: { 2: '#4488ff', 3: '#66aaff', 4: '#ffcc00', 5: '#ffaa00', 6: '#ff4444' }"),
    ]
    for old, new in pal_updates:
        if old != new and old in text:
            text = text.replace(old, new, 1)

    SPRITES.write_text(text, encoding="utf-8")
    lines = text.count("\n") + (0 if text.endswith("\n") else 1)
    print(f"wrote {SPRITES.name} lines={lines}")


if __name__ == "__main__":
    main()