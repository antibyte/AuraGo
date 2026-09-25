"""Freeze candidates and full token sequences after every data release gate."""
from __future__ import annotations

import argparse
from collections import defaultdict
from pathlib import Path
import random

from .assets import tokenizer, verify as verify_assets
from .common import HOME, canonical, catalog, config, digest, file_digest, read_json, read_jsonl, repository_revision, write_json, write_jsonl
from .retrieval import Retriever, choose_k
from .serialization import example, encode_exact
from .schedule import CONFUSIONS


def compile_rows(rows, cat, rankings, k, tok, maximum=2048, scores=None):
    compiled = defaultdict(list)
    for row in rows:
        candidates = list(rankings[row["id"]][:k])
        rng = random.Random(digest(row["id"]))
        if row["split"] == "train" and rng.random() < 0.5:
            # Oracle injection is confined to the declared training half.
            gold = row["answers"]
            known = {m["id"] for m in cat["manuals"]}
            confusers = sorted({mid for cluster in CONFUSIONS if set(gold) & set(cluster) for mid in cluster if mid in known and mid not in gold})
            preferred = gold + confusers
            candidates = (preferred + [mid for mid in candidates if mid not in preferred])[:k]
        rng.shuffle(candidates)
        ex = example(row, cat, candidates, row["split"] == "train")
        ids, mask = encode_exact(ex, tok, maximum)
        compiled[row["split"]].append({"id": row["id"], "group_id": row["group_id"], "language": row["language"],
                                       "kind": row["kind"], "challenge": row["challenge"], "candidates": candidates, "retrieved": list(rankings[row["id"]][:k]),
                                       "retrieved_scores": (scores or {}).get(row["id"], {}),
                                       "all_manual_ids": row["all_manual_ids"], "gold": row["answers"],
                                       "query": row["query"], "context": row["context"], "example": ex,
                                       "tokens": ids, "mask": mask, "bucket": min(maximum, ((len(ids) + 255) // 256) * 256)})
    return compiled


def pilot_pairs(train_rows):
    groups = defaultdict(list)
    for row in train_rows:
        groups[row["group_id"]].append(row)
    de_en, multilingual = [], []
    for group_id in sorted(groups, key=digest):
        group = sorted(groups[group_id], key=lambda r: r["id"])
        de = [r for r in group if r["language"] == "de"]
        en = [r for r in group if r["language"] == "en"]
        other = [r for r in group if r["language"] not in {"de", "en"}]
        if len(de) != 2 or len(en) != 1 or len(other) != 1:
            raise ValueError("pilot needs the same complete scenario groups in both language arms")
        # Equal buckets keep the trainer's shuffled batch indices identical too.
        # Merely equal input order would diverge after length-based batching.
        bucket = max(r["bucket"] for r in group)
        de_en.extend({**r, "bucket": bucket} for r in [*de, en[0], en[0]])
        multilingual.extend({**r, "bucket": bucket} for r in [*de, en[0], other[0]])
    return de_en, multilingual


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True)
    parser.add_argument("--quality", required=True)
    parser.add_argument("--search-binary", required=True)
    parser.add_argument("--out", default=str(HOME / "generated" / "compiled"))
    args = parser.parse_args()
    cfg, cat, quality = config(), catalog(), read_json(args.quality)
    rows = list(read_jsonl(args.input))
    if not quality.get("ready_for_compilation") or quality.get("corpus_sha256") != digest(canonical(rows)) or quality.get("catalog_sha256") != cat["catalog_sha256"]:
        raise SystemExit("data quality gate missing, failed or stale; no GPU-ready pack produced")
    assets = verify_assets()
    retriever = Retriever(cat, args.search_binary)
    try:
        retrieval = {r["id"]: retriever.retrieve_details(r, k=20) for r in rows}
    finally:
        retriever.close()
    rankings = {rid: result["manual_ids"] for rid, result in retrieval.items()}
    scores = {rid: result["embedding_scores"] for rid, result in retrieval.items()}
    k, report = choose_k([r for r in rows if r["split"] == "validation"], rankings, cfg)
    compiled = compile_rows(rows, cat, rankings, k, tokenizer(), cfg["max_length"], scores)
    de_en, multi = pilot_pairs(compiled["train"])
    compiled.update({"pilot_de_en": de_en, "pilot_multilingual": multi})
    out = Path(args.out)
    for split, items in compiled.items():
        write_jsonl(out / (split + ".jsonl"), items)
    manifest = {"version": 1, "ready_for_gpu": True, "catalog_sha256": cat["catalog_sha256"],
                "repository_revision": repository_revision(), "input_sha256": file_digest(args.input),
                "quality_sha256": file_digest(args.quality), "assets": assets, "candidate_count": k,
                "candidate_recall": report, "config_sha256": file_digest(HOME / "config.json"),
                "dependency_lock_sha256": file_digest(HOME / "uv.lock"),
                "files": {split + ".jsonl": file_digest(out / (split + ".jsonl")) for split in compiled}}
    write_json(out / "manifest.json", manifest)
    print(canonical({"compiled_rows": len(rows), "candidate_count": k, "manifest": str(out / "manifest.json")}))


if __name__ == "__main__":
    main()
