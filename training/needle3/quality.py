"""Release gates distinguish drafts, blind agreement, and fully accepted data."""
from __future__ import annotations

import argparse
from collections import Counter, defaultdict
import gzip
import os
from pathlib import Path

from .common import HOME, canonical, catalog, config, digest, file_digest, read_json, read_jsonl, write_json, write_jsonl
from .data import corpus_report, validate_row


def language_detector():
    from lingua import Language, LanguageDetectorBuilder
    names = ["GERMAN", "ENGLISH", "FRENCH", "SPANISH", "CHINESE", "JAPANESE", "DUTCH", "PORTUGUESE", "POLISH", "CZECH", "ITALIAN", "SWEDISH", "BOKMAL", "DANISH", "GREEK", "HINDI"]
    languages = [getattr(Language, n) for n in names]
    detector = LanguageDetectorBuilder.from_languages(*languages).with_minimum_relative_distance(0.12).build()
    mapping = dict(zip(languages, config()["languages"]))
    return lambda text: mapping.get(detector.detect_language_of(text))


def requires_review(row):
    return (row["split"] != "train" or row["kind"] in {"multi", "confusable", "context"} or
            row["variation"] != "natural" or len(row["query"]) <= 35 or int(digest(row["id"])[:8], 16) % 10 == 0)


def audit_queue(rows, count=640):
    # Cover every manual and language first; hash order alone can miss families.
    cells, families, languages = defaultdict(list), defaultdict(list), defaultdict(list)
    for row in rows:
        languages[row["language"]].append(row)
        for mid in row["all_manual_ids"] or ["none"]:
            cells[mid, row["language"]].append(row)
            families[mid].append(row)
    selected, seen = [], set()
    pools = [families[k] for k in sorted(families)] + [languages[k] for k in sorted(languages)]
    pools += [cells[k] for k in sorted(cells, key=lambda k: digest(canonical(k)))] + [rows]
    for pool in pools:
        for row in sorted(pool, key=lambda r: digest(r["id"])):
            if row["id"] not in seen:
                seen.add(row["id"])
                selected.append({"row_id": row["id"], "row_sha256": digest(canonical(row)), "sources": row["sources"],
                                 "query": row["query"], "context": row["context"], "answers": row["answers"],
                                 "reviewer": "", "source_evidence": {}, "approved": False})
                break
        if len(selected) == count:
            break
    if len(selected) < count:
        for row in sorted(rows, key=lambda r: digest(r["id"])):
            if row["id"] not in seen:
                seen.add(row["id"])
                selected.append({"row_id": row["id"], "row_sha256": digest(canonical(row)), "sources": row["sources"], "query": row["query"], "context": row["context"], "answers": row["answers"], "reviewer": "", "source_evidence": {}, "approved": False})
            if len(selected) == count:
                break
    return selected


def release_report(rows, cat, cfg, audit_receipts, semantic_receipt, check_language=True):
    report = corpus_report(rows, cat, cfg)
    issues = report["issues"]
    detector = language_detector() if check_language else None
    languages_checked = 0
    operations = set()
    for row in rows:
        if detector:
            try:
                validate_row(row, cat, detector)
                if detector(row["query"]) is None and row.get("blind_review", {}).get("language_ok") is not True:
                    issues.append("uncertain language requires blind review: " + row["id"])
                languages_checked += 1
            except ValueError as exc:
                issues.append(row["id"] + ": " + str(exc))
        review = row.get("blind_review", {})
        if requires_review(row):
            original = {k: v for k, v in row.items() if k != "blind_review"}
            if (review.get("model") != cfg["reviewer_model"] or not review.get("request_id") or
                review.get("row_sha256") != digest(canonical(original)) or
                review.get("manual_ids") != row["all_manual_ids"] or
                not all(review.get(k) is True for k in ("language_ok", "natural", "supported", "distinct_scenario"))):
                issues.append("missing/invalid independent review: " + row["id"])
        if row["split"] == "train":
            operations.update((r["tool"], r.get("selector", ""), str(r.get("value", ""))) for r in row["operation_refs"])
    expected = {(t["name"], o.get("selector", ""), str(o.get("value", ""))) for t in cat["tools"] if t.get("manual_id") for o in t.get("operations") or [{}]}
    for missing in sorted(expected - operations):
        issues.append("operation coverage: " + canonical(missing))
    rows_by_id = {r["id"]: r for r in rows}
    audited, audit_languages, audit_manuals = set(), set(), set()
    for receipt in audit_receipts:
        row = rows_by_id.get(receipt.get("row_id"))
        if not row or not receipt.get("approved") or not receipt.get("reviewer"):
            continue
        if receipt.get("row_sha256") != digest(canonical(row)) or receipt.get("sources") != row["sources"]:
            issues.append("stale source audit: " + receipt["row_id"])
            continue
        evidence = receipt.get("source_evidence", {})
        manuals = {m["id"]: m["body"] for m in cat["manuals"]}
        if any(not isinstance(evidence.get(mid), dict) or
               len(evidence[mid].get("quote", "")) < 20 or evidence[mid]["quote"] not in manuals[mid] or
               len(evidence[mid].get("explanation", "")) < 20 for mid in row["all_manual_ids"]):
            issues.append("missing exact source quote or explanation: " + receipt["row_id"])
            continue
        if not row["all_manual_ids"] and len(receipt.get("no_manual_explanation", "")) < 20:
            issues.append("missing no-manual explanation: " + receipt["row_id"])
            continue
        audited.add(row["id"])
        audit_languages.add(row["language"])
        audit_manuals.update(row["all_manual_ids"])
    if len(audited) < cfg["manual_audit_rows"] or audit_languages != set(cfg["languages"]) or audit_manuals != {m["id"] for m in cat["manuals"]}:
        issues.append("source audit incomplete across languages/manuals")
    if (semantic_receipt.get("approved") is not True or semantic_receipt.get("corpus_sha256") != report["corpus_sha256"] or
        semantic_receipt.get("distinct_groups", 0) < cfg["minimum_semantic_groups"] or
        semantic_receipt.get("distinct_groups", 0) > report["groups"] or
        semantic_receipt.get("unresolved_pairs", 1) != 0 or not semantic_receipt.get("reviewer")):
        issues.append("semantic distinctness / cross-split leakage audit incomplete")
    if not check_language:
        issues.append("language detector gate not executed")
    report.update({"language_checked_rows": languages_checked, "audited_rows": len(audited),
                   "ready_for_compilation": not issues, "ready_for_gpu": False,
                   "catalog_sha256": cat["catalog_sha256"]})
    return report


def publish_shards(rows, report, directory):
    if not report.get("ready_for_compilation") or report.get("corpus_sha256") != digest(canonical(rows)):
        raise ValueError("only the exact fully accepted corpus may be published")
    directory = Path(directory) / report["corpus_sha256"][:16]
    directory.mkdir(parents=True, exist_ok=True)
    languages, files = defaultdict(list), {}
    for row in rows:
        languages[row["language"]].append(row)
    for language, items in sorted(languages.items()):
        name = language + ".jsonl.gz"
        temporary = directory / (name + ".tmp")
        data = "".join(canonical(r) + "\n" for r in sorted(items, key=lambda r: r["id"])).encode("utf-8")
        temporary.write_bytes(gzip.compress(data, mtime=0))
        os.replace(temporary, directory / name)
        files[name] = file_digest(directory / name)
    write_json(directory / "manifest.json", {"corpus_sha256": report["corpus_sha256"], "catalog_sha256": report["catalog_sha256"],
                                             "accepted_rows": len(rows), "files": files, "ready_for_gpu": False})
    return directory


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input")
    parser.add_argument("--audit")
    parser.add_argument("--semantic")
    parser.add_argument("--output", "--out", required=True)
    parser.add_argument("--audit-queue")
    parser.add_argument("--accept-out", help="write deterministic compressed language shards only after all quality gates pass")
    args = parser.parse_args()
    rows = list(read_jsonl(args.input))
    if args.audit_queue:
        write_jsonl(args.audit_queue, audit_queue(rows))
    report = release_report(rows, catalog(), config(), list(read_jsonl(args.audit)) if args.audit else [], read_json(args.semantic) if args.semantic else {})
    write_json(args.output, report)
    if args.accept_out:
        publish_shards(rows, report, args.accept_out)
    print(canonical({"rows": report["rows"], "issues": len(report["issues"]), "ready_for_compilation": report["ready_for_compilation"]}))
    raise SystemExit(0 if report["ready_for_compilation"] else 2)


if __name__ == "__main__":
    main()
