"""Recall-first metrics, explicit retrieval failures, and one frozen final test."""
from __future__ import annotations

import argparse
from collections import defaultdict
import json
import math
import os
from pathlib import Path

from .common import canonical, file_digest, read_jsonl, write_json


def metrics(rows, predictions):
    tp = fp = fn = candidate_hit = gold_count = empty_correct = empty_count = valid = 0
    latency, tokens, rss = [], [], []
    all_candidate_hit = all_gold_count = 0
    if set(predictions) != {r["id"] for r in rows}:
        raise ValueError("predictions must cover exactly the evaluated rows")
    for row in rows:
        prediction = predictions[row["id"]]
        result = prediction["manual_ids"]
        if len(result) > 3 or len(result) != len(set(result)) or set(result) - set(row["candidates"]):
            raise ValueError("unguarded/duplicate predictions cannot enter evaluation")
        gold, chosen = set(row["gold"]), set(result)
        tp += len(gold & chosen)
        fp += len(chosen - gold)
        fn += len(gold - chosen)
        candidate_hit += len(gold & set(row["candidates"]))
        gold_count += len(gold)
        all_gold = set(row.get("all_manual_ids", row["gold"]))
        all_candidate_hit += len(all_gold & set(row["candidates"]))
        all_gold_count += len(all_gold)
        if not gold:
            empty_count += 1
            empty_correct += not chosen and bool(prediction.get("formal_valid"))
        valid += bool(prediction.get("formal_valid"))
        if "latency_ms" in prediction:
            latency.append(prediction["latency_ms"])
        if "process_rss_mb" in prediction:
            rss.append(prediction["process_rss_mb"])
        tokens.append(prediction.get("prompt_tokens", 0))
    precision = tp / (tp + fp) if tp + fp else 0
    recall = tp / (tp + fn) if tp + fn else None
    f2 = 5 * tp / (5 * tp + 4 * fn + fp) if 5 * tp + 4 * fn + fp else 0
    return {"rows": len(rows), "precision": precision, "recall_at_3": recall, "f2": f2,
            "candidate_recall": candidate_hit / gold_count if gold_count else None,
            "all_required_candidate_recall": all_candidate_hit / all_gold_count if all_gold_count else None,
            "conditional_selection_recall": tp / candidate_hit if candidate_hit else None,
            "retrieval_missed_manuals": gold_count - candidate_hit, "unnecessary_manuals": fp,
            "empty_accuracy": empty_correct / empty_count if empty_count else None,
            "formal_valid_fraction": valid / len(rows) if rows else None,
            "mean_prompt_tokens": sum(tokens) / len(tokens) if tokens else None,
            "maximum_observed_worker_rss_mb": max(rss) if rss else None,
            "latency_p50_ms": sorted(latency)[len(latency) // 2] if latency else None,
            "latency_p95_ms": sorted(latency)[min(len(latency) - 1, math.ceil(.95 * len(latency)) - 1)] if latency else None}


def report(rows, predictions):
    result = {"overall": metrics(rows, predictions)}
    for dimension in ("language", "kind"):
        groups = defaultdict(list)
        for row in rows:
            groups[row[dimension]].append(row)
        result[dimension] = {key: metrics(group, {r["id"]: predictions[r["id"]] for r in group}) for key, group in groups.items()}
    manuals = defaultdict(list)
    for row in rows:
        for mid in row["gold"]:
            manuals[mid].append(row)
    result["manual"] = {mid: {"examples": len(group), "selected_fraction": sum(mid in predictions[r["id"]]["manual_ids"] for r in group) / len(group)} for mid, group in manuals.items()}
    challenge = [r for r in rows if r.get("challenge")]
    if challenge:
        result["challenge"] = metrics(challenge, {r["id"]: predictions[r["id"]] for r in challenge})
    return result


def claim_final_test(directory, dataset_sha, selection_sha):
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / "final-test.lock.json"
    # O_EXCL prevents another selection from reusing the held-out test. A crashed
    # run must be explicitly reconciled using its immutable saved predictions.
    with path.open("x", encoding="utf-8", newline="\n") as f:
        json.dump({"dataset_sha256": dataset_sha, "selection_sha256": selection_sha}, f)
    return path


_selector = None


def _worker_init(weights):
    import atexit
    global _selector
    from .runtime import Selector
    _selector = Selector(weights)
    atexit.register(_selector.close)


def _predict(row):
    try:
        result = _selector.select(row, row["candidates"])
        # Linux evaluation workers include native allocation in their peak RSS.
        if os.name == "posix":
            import resource
            result["process_rss_mb"] = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024
        return row["id"], result
    except (RuntimeError, ValueError, OSError) as exc:
        return row["id"], {"manual_ids": [], "formal_valid": False, "fallback_required": True, "error_type": type(exc).__name__}


def run_model(rows, weights, workers=6):
    from concurrent.futures import ProcessPoolExecutor
    import multiprocessing
    # Native sessions are isolated by process; never share mutable schemas or
    # model state across concurrent requests. All model arms use the same pool.
    with ProcessPoolExecutor(max_workers=min(workers, os.cpu_count() or 1), mp_context=multiprocessing.get_context("spawn"), initializer=_worker_init, initargs=(str(weights),)) as pool:
        return dict(pool.map(_predict, rows, chunksize=16))


def paired_f2_advantage(rows, left, right, repetitions=1000):
    """Resample whole scenario groups so translated variants stay dependent."""
    import random
    groups = defaultdict(lambda: [0] * 6)
    for row in rows:
        gold = set(row["gold"])
        for offset, predictions in ((0, left), (3, right)):
            chosen = set(predictions[row["id"]]["manual_ids"])
            counts = (len(gold & chosen), len(chosen - gold), len(gold - chosen))
            for i, value in enumerate(counts):
                groups[row["group_id"]][offset + i] += value
    values, deltas = list(groups.values()), []
    rng = random.Random(20260925)
    def f2(counts):
        tp, fp, fn = counts
        return 5 * tp / max(1, 5 * tp + fp + 4 * fn)
    for _ in range(repetitions):
        sums = [sum(c[i] for c in sampled) for i in range(6)] if (sampled := rng.choices(values, k=len(values))) else [0] * 6
        deltas.append(f2(sums[:3]) - f2(sums[3:]))
    deltas.sort()
    return {"unit": "scenario_group", "repetitions": repetitions, "groups": len(values),
            "f2_delta_ci95": [deltas[int(.025 * repetitions)], deltas[min(repetitions - 1, int(.975 * repetitions))]],
            "positive_advantage": bool(values) and deltas[int(.025 * repetitions)] > 0}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data", required=True)
    parser.add_argument("--weights", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    if Path(args.data).name != "validation.jsonl":
        raise SystemExit("individual checkpoint evaluation is restricted to validation.jsonl; use the final-test controller for test")
    rows = list(read_jsonl(args.data))
    predictions = run_model(rows, args.weights)
    write_json(args.out, {"model_sha256": file_digest(args.weights), "data_sha256": file_digest(args.data), "metrics": report(rows, predictions), "predictions": predictions})


if __name__ == "__main__":
    main()
