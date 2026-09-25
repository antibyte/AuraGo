"""Source-bound row validation and auditable, grouped dataset release gates."""
from __future__ import annotations

from collections import Counter, defaultdict
import re
import unicodedata

from .common import canonical, digest, inference_query

KINDS = {"single", "multi", "confusable", "context", "none"}
LANGUAGES = {"de", "en", "fr", "es", "zh", "ja", "nl", "pt", "pl", "cs", "it", "sv", "no", "da", "el", "hi"}
VARIATIONS = {"natural", "typo", "colloquial", "keywords", "transcription", "code_switch"}
SENSITIVE = re.compile(r"sk-or-v1-[a-zA-Z0-9]{20,}|-----BEGIN [A-Z ]*PRIVATE KEY|[A-Za-z0-9._%+-]+@(?!example\.(?:com|org|net)\b)[A-Za-z0-9.-]+\.[a-z]{2,}")


def normalized(text):
    return " ".join(unicodedata.normalize("NFKC", text).casefold().split())


def validate_row(row, cat, language_detector=None):
    known = {m["id"]: m for m in cat["manuals"]}
    required = {"id", "group_id", "split", "language", "kind", "query", "context", "all_manual_ids", "answers", "reasoning", "scenario", "sources", "operation_refs", "variation", "challenge"}
    if not required <= row.keys():
        raise ValueError("missing row fields: " + ",".join(sorted(required - row.keys())))
    if row["split"] not in {"train", "validation", "test"} or row["kind"] not in KINDS:
        raise ValueError("invalid split or kind")
    if row["language"] not in LANGUAGES or row["variation"] not in VARIATIONS:
        raise ValueError("invalid language or variation")
    if not all(isinstance(row[k], str) and row[k].strip() for k in ("id", "group_id")) or type(row["challenge"]) is not bool:
        raise ValueError("invalid row/group identity or challenge flag")
    provenance = row.get("provenance", {})
    if provenance.get("type") != "repository_synthetic" or provenance.get("catalog_sha256") != cat["catalog_sha256"]:
        raise ValueError("missing or stale repository provenance")
    if not isinstance(row["query"], str) or not 4 <= len(row["query"].strip()) <= 1400:
        raise ValueError("query length outside bounds")
    if not isinstance(row["reasoning"], str) or not 3 <= len(row["reasoning"]) <= 500:
        raise ValueError("reasoning must be short and nonempty")
    if not isinstance(row["scenario"], str) or not 8 <= len(row["scenario"]) <= 600:
        raise ValueError("scenario must describe the distinct intent")
    inference_query(row)
    if any(len(c) > 1000 for c in row["context"]):
        raise ValueError("context too long")
    if SENSITIVE.search(canonical(row)):
        raise ValueError("possible non-synthetic identifier or secret")
    gold, answers = row["all_manual_ids"], row["answers"]
    if not isinstance(gold, list) or not isinstance(answers, list) or any(not isinstance(x, str) for x in gold + answers):
        raise ValueError("labels must be string arrays")
    if len(set(gold)) != len(gold) or len(set(answers)) != len(answers) or len(answers) > 3:
        raise ValueError("duplicate labels or more than three selections")
    if set(gold) - known.keys() or answers != gold[:3]:
        raise ValueError("unknown labels or incorrect execution priority")
    if (row["kind"] == "none") != (not gold):
        raise ValueError("none/positive label mismatch")
    if row["kind"] == "multi" and len(gold) < 2:
        raise ValueError("multi requires multiple manuals")
    if row["kind"] == "context" and not row["context"]:
        raise ValueError("context example without preceding request")
    if row["sources"] != {mid: known[mid]["sha256"] for mid in gold}:
        raise ValueError("source hash or relevant source set mismatch")
    tools = {t["name"]: t for t in cat["tools"]}
    covered = set()
    for ref in row["operation_refs"]:
        tool = tools.get(ref.get("tool"))
        if not tool or tool.get("manual_id") not in gold:
            raise ValueError("operation reference outside labels")
        if tool.get("operations") and not any(ref.get("selector", "") == op.get("selector", "") and ref.get("value", "") == op.get("value", "") for op in tool["operations"]):
            raise ValueError("operation not present in strict schema")
        covered.add(tool["manual_id"])
    if covered != set(gold):
        raise ValueError("missing operation evidence")
    if language_detector and len(row["query"]) > 35 and row["variation"] != "code_switch":
        detected = language_detector(row["query"])
        if detected and detected != row["language"]:
            raise ValueError(f"language mismatch: expected {row['language']}, detected {detected}")
    return row


def expand_group(job, generated, cat, provenance):
    if generated.get("group_id") != job["group_id"]:
        raise ValueError("generation group identity mismatch")
    variants = generated["variants"]
    if [v["language"] for v in variants] != ["de", "de", "en", job["language"]]:
        raise ValueError("incorrect four-variant language allocation")
    gold = generated["manual_ids"]
    if set(gold) != set(job["manual_ids"]):
        raise ValueError("generated gold differs from requested capabilities")
    known = {m["id"]: m for m in cat["manuals"]}
    rows = []
    for i, variant in enumerate(variants):
        row = {"id": job["group_id"] + f"-{i}", "group_id": job["group_id"], "split": job["split"],
               "kind": job["kind"], "query": variant["query"], "language": variant["language"],
               "context": variant.get("context", []), "all_manual_ids": gold, "answers": gold[:3],
               "reasoning": generated["reasoning"], "scenario": generated["scenario"],
               "sources": {mid: known[mid]["sha256"] for mid in gold}, "operation_refs": job["operation_refs"],
               "variation": variant.get("variation", "natural"), "challenge": job["challenge"],
               "provenance": provenance}
        validate_row(row, cat)
        rows.append(row)
    if normalized(rows[0]["query"]) == normalized(rows[1]["query"]):
        raise ValueError("German paraphrase identical to source")
    return rows


def corpus_report(rows, cat, cfg):
    issues, seen, groups, counts, coverage = [], {}, defaultdict(list), Counter(), Counter()
    variations, seen_ids, kinds = 0, set(), Counter()
    for row in rows:
        validate_row(row, cat)
        if row["id"] in seen_ids:
            issues.append("duplicate row ID: " + row["id"])
        seen_ids.add(row["id"])
        kinds[row["kind"]] += 1
        text = normalized(inference_query(row))
        if text in seen:
            issues.append("duplicate query: " + row["id"])
        seen[text] = row["id"]
        groups[row["group_id"]].append(row)
        counts[row["split"], row["language"]] += 1
        variations += row["variation"] != "natural"
        if row["split"] == "train":
            coverage.update((mid, row["language"]) for mid in row["all_manual_ids"])
    for group, siblings in groups.items():
        if len({r["split"] for r in siblings}) != 1 or len(siblings) != 4:
            issues.append("split leakage or missing siblings: " + group)
        allocation = Counter(r["language"] for r in siblings)
        if allocation["de"] != 2 or allocation["en"] != 1 or len(allocation) != 3:
            issues.append("group language allocation: " + group)
        for field in ("scenario", "kind", "all_manual_ids", "answers", "sources", "operation_refs", "challenge"):
            if len({canonical(r[field]) for r in siblings}) != 1:
                issues.append("inconsistent group " + field + ": " + group)
    for kind, percent in cfg["kind_weights"].items():
        if kinds[kind] * 100 != len(rows) * percent:
            issues.append("content mixture: " + kind)
    targets = {}
    for split, n in cfg["groups"].items():
        for lang in cfg["languages"]:
            target = n * 2 if lang == "de" else n if lang == "en" else n // 14
            targets[f"{split}/{lang}"] = {"actual": counts[split, lang], "target": target}
            if counts[split, lang] != target:
                issues.append(f"row quota: {split}/{lang}")
    for manual in cat["manuals"]:
        for lang in cfg["languages"]:
            minimum = cfg["minimum_manual_train"].get(lang, cfg["minimum_manual_train"]["other"])
            if coverage[manual["id"], lang] < minimum:
                issues.append(f"manual coverage: {manual['id']}/{lang}")
    if len(groups) < cfg["minimum_semantic_groups"]:
        issues.append("insufficient scenario groups (IDs alone do not prove semantic uniqueness)")
    if variations < len(rows) * cfg["minimum_language_variation_fraction"]:
        issues.append("insufficient linguistic variation")
    if sum(r["challenge"] for r in rows if r["split"] == "test") != cfg["challenge_rows"]:
        issues.append("challenge quota")
    return {"rows": len(rows), "groups": len(groups), "quotas": targets, "kinds": dict(kinds), "issues": issues,
            "structural_ready": not issues, "corpus_sha256": digest(canonical(rows))}
