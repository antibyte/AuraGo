"""Deterministic coverage requests, never counted as generated or accepted data."""
from __future__ import annotations

from collections import Counter, defaultdict
import random

from .common import catalog, config, digest, canonical, write_jsonl, write_json, HOME

CONFUSIONS = [
    ["docker", "proxmox", "remote_control_shell"],
    ["email", "discord", "send_agodesk_chat"],
    ["filesystem", "workspace_search", "knowledge_graph", "core_memory"],
    ["home_assistant", "mqtt"],
]
VARIATIONS = ["typo", "colloquial", "keywords", "transcription", "code_switch"]
REPORTING = {
    "email": {"tool": "send_email", "selector": "", "value": ""},
    "discord": {"tool": "send_discord", "selector": "", "value": ""},
    "filesystem": {"tool": "filesystem", "selector": "operation", "value": "write_file"},
    "manage_notes": {"tool": "manage_notes", "selector": "operation", "value": "add"},
}


def allocation(total, weights):
    raw = {k: total * v / sum(weights.values()) for k, v in weights.items()}
    result = {k: int(v) for k, v in raw.items()}
    for k in sorted(raw, key=lambda k: (-(raw[k] - result[k]), k))[:total - sum(result.values())]:
        result[k] += 1
    return result


def build_schedule(cat=None, cfg=None):
    cat, cfg = cat or catalog(), cfg or config()
    rng = random.Random(cfg["seed"])
    manuals = [m["id"] for m in cat["manuals"]]
    operations = defaultdict(list)
    for tool in cat["tools"]:
        if tool.get("manual_id"):
            for op in tool.get("operations") or [{"selector": "", "value": ""}]:
                operations[tool["manual_id"]].append({"tool": tool["name"], **op})
    cursor = Counter()
    result = []
    other = cfg["languages"][2:]
    kinds_all = [kind for kind, count in allocation(sum(cfg["groups"].values()), cfg["kind_weights"]).items() for _ in range(count)]
    rng.shuffle(kinds_all)
    kind_position = 0
    for split, count in cfg["groups"].items():
        if count % len(other):
            raise ValueError("group count must divide evenly across secondary languages")
        for li, language in enumerate(other):
            per_language = count // len(other)
            kinds = kinds_all[kind_position:kind_position + per_language]
            kind_position += per_language
            positive = 0
            for i, kind in enumerate(kinds):
                primary = manuals[(positive + li * 37) % len(manuals)] if kind != "none" else None
                if primary and split == "train" and positive >= 4 * len(manuals):
                    debt = {mid: len(operations[mid]) - cursor[mid] for mid in manuals}
                    if max(debt.values()) > 0:
                        primary = max(manuals, key=lambda mid: (debt[mid], mid))
                selected = []
                if primary:
                    positive += 1
                    selected = [primary]
                    if kind == "multi":
                        # Connected reporting/storage workflows avoid random,
                        # unrelated integrations being forced into one request.
                        choices = [m for m in REPORTING if m != primary]
                        selected += rng.sample(choices, 1 + (i % 7 == 0) + (i % 17 == 0))
                refs = []
                for mid in selected:
                    if mid != primary and mid in REPORTING:
                        refs.append(REPORTING[mid])
                    else:
                        refs.append(operations[mid][cursor[mid] % len(operations[mid])])
                        cursor[mid] += 1
                result.append({"group_id": f"{split}-{language}-{i:04d}", "split": split,
                               "language": language, "kind": kind, "manual_ids": selected,
                               "confusable_manual_ids": sorted({mid for cluster in CONFUSIONS if primary in cluster for mid in cluster if mid in manuals and mid not in selected}) if kind == "confusable" else [],
                               "operation_refs": refs, "variation": VARIATIONS[(i // 3) % 5] if i % 3 == 0 else "natural",
                               "challenge": False, "seed": rng.randrange(2**31)})
    hard = [g for g in result if g["split"] == "test" and g["kind"] in ("multi", "confusable", "context")]
    rng.shuffle(hard)
    if len(hard) * 4 < cfg["challenge_rows"]:
        raise ValueError("insufficient challenge groups")
    for group in hard[:cfg["challenge_rows"] // 4]:
        group["challenge"] = True
    return result


def summary(groups):
    return {"groups": len(groups), "requested_rows": 4 * len(groups), "accepted_rows": 0,
            "split_groups": dict(Counter(g["split"] for g in groups)),
            "split_rows": {s: dict(Counter(lang for g in groups if g["split"] == s for lang in ["de", "de", "en", g["language"]])) for s in ("train", "validation", "test")},
            "kind_groups": dict(Counter(g["kind"] for g in groups)),
            "challenge_rows": 4 * sum(g["challenge"] for g in groups),
            "schedule_sha256": digest(canonical(groups))}


def main():
    groups = build_schedule()
    write_jsonl(HOME / "generated" / "requests.jsonl", groups)
    write_json(HOME / "schedule_manifest.json", summary(groups))
    print(canonical(summary(groups)))


if __name__ == "__main__":
    main()
