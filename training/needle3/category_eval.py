"""Native category metrics, validation selection, and a once-claimed holdout."""
from __future__ import annotations

import argparse
from collections import Counter
import os
from pathlib import Path
import statistics
import subprocess
import sys
import time
import warnings

from .category_data import load
from .category_router import CATEGORIES, CategoryRouter
from .common import HOME, canonical, file_digest, read_json, write_json
from .local_pilot import output_path


def metrics(rows, predictions):
    tp = fp = fn = exact = complete = empty_ok = negatives = valid = positives = 0
    counts, sizes = [], []
    for row in rows:
        pred = predictions[row["id"]]
        gold, actual = set(row["categories"]), set(pred["category_ids"])
        tp += len(gold & actual)
        fp += len(actual - gold)
        fn += len(gold - actual)
        exact += actual == gold and pred["formal_valid"]
        valid += pred["formal_valid"]
        if gold:
            positives += 1
            complete += gold <= actual
        else:
            negatives += 1
            empty_ok += not actual and pred["formal_valid"]
        counts.append(len(actual))
        sizes.append(sum(len(x) for x in pred.get("manuals_by_category", {}).values()))
    return {"rows": len(rows), "required_categories": tp + fn, "hits": tp, "extras": fp,
            "recall": tp / (tp + fn) if tp + fn else None,
            "precision": tp / (tp + fp) if tp + fp else 0.,
            "f2": 5 * tp / (5 * tp + 4 * fn + fp) if tp + fn else 0.,
            "all_required": complete / positives if positives else None,
            "exact": exact / len(rows) if rows else 0.,
            "empty_accuracy": empty_ok / negatives if negatives else None,
            "negative_rows": negatives, "formal_valid": valid / len(rows) if rows else 0.,
            "mean_categories": statistics.mean(counts) if counts else 0.,
            "mean_manual_fanout": statistics.mean(sizes) if sizes else 0.}


def report(rows, predictions):
    values = sorted(p["latency_ms"] for p in predictions.values())
    by_category = {}
    for category in CATEGORIES:
        needed = sum(category in r["categories"] for r in rows)
        returned = sum(category in predictions[r["id"]]["category_ids"] for r in rows)
        correct = sum(category in r["categories"] and category in predictions[r["id"]]["category_ids"] for r in rows)
        by_category[category] = {"required": needed, "returned": returned, "hits": correct,
                                 "recall": correct / needed if needed else None,
                                 "precision": correct / returned if returned else 0.}
    return {"overall": metrics(rows, predictions),
            "languages": {l: metrics([r for r in rows if r["language"] == l], predictions) for l in {r["language"] for r in rows}},
            "kinds": {k: metrics([r for r in rows if r["kind"] == k], predictions) for k in {r["kind"] for r in rows}},
            "categories": by_category,
            "latency": {"p50_ms": statistics.median(values), "p95_ms": values[max(0, int(len(values) * .95 + .999) - 1)],
                        "max_ms": max(values), "violations_600ms": sum(x >= 600 for x in values)},
            "prompt_tokens": {"max": max(p.get("prompt_tokens", 0) for p in predictions.values())}}


def measure(directory, weights, split, output):
    from .assets import PINS
    directory, output = output_path(directory), output_path(output)
    if output.exists():
        raise ValueError("evaluation evidence cannot be overwritten")
    manifest, all_rows = load(directory)
    rows = [r for r in all_rows if r["split"] == split]
    weights = Path(weights).resolve()
    sidecar = read_json(str(weights) + ".json")
    if sidecar["pins"] != PINS or sidecar["weight_bits"] != 4 or sidecar["output_sha256"] != file_digest(weights):
        raise ValueError("model export does not match its identity")
    if split == "holdout":
        claim = read_json(directory / "holdout.lock.json")
        if (claim["cases_sha256"] != manifest["files"]["cases.jsonl"]
                or claim["selection_sha256"] != file_digest(directory / "selection.json")
                or file_digest(weights) not in claim["model_hashes"]):
            raise ValueError("model was not frozen before holdout")
    warnings.filterwarnings("ignore", message="these weights carry no confidence head")
    started = time.perf_counter()
    router = CategoryRouter(weights)
    predictions = {}
    try:
        router.select({"query": "Hello", "context": []})
        startup = (time.perf_counter() - started) * 1000
        # Deterministic mixed order avoids timing by language/category blocks.
        import random
        random.Random(manifest["seed"]).shuffle(rows)
        for row in rows:
            begun = time.perf_counter()
            value = router.select(row)
            value["latency_ms"] = (time.perf_counter() - begun) * 1000
            predictions[row["id"]] = value
    finally:
        router.close()
    result = {"weights": str(weights), "weights_sha256": file_digest(weights), "split": split,
              "contract_sha256": manifest["contract_sha256"], "startup_ms": startup,
              "report": report(rows, predictions), "predictions": predictions}
    write_json(output, result)
    print(canonical({"output": str(output), **result["report"]["overall"], "latency": result["report"]["latency"]}), flush=True)
    return result


def child_measure(directory, weights, split, name):
    path = Path(directory) / name
    env = {**os.environ, "OMP_NUM_THREADS": "2", "MKL_NUM_THREADS": "2", "OPENBLAS_NUM_THREADS": "2",
           "DO_NOT_TRACK": "1", "HF_HUB_OFFLINE": "1", "TRANSFORMERS_OFFLINE": "1"}
    subprocess.run([sys.executable, "-m", "training.needle3.category_eval", "measure", "--out", str(directory),
                    "--weights", str(weights), "--split", split, "--result", str(path)], env=env, check=True)
    return read_json(path)


def quality_key(result):
    m = result["report"]["overall"]
    return m["f2"], m["all_required"], m["empty_accuracy"], m["exact"]


def select(directory):
    directory = output_path(directory)
    load(directory)
    if (directory / "selection.json").exists():
        raise ValueError("selection is already frozen")
    baseline = read_json(directory / "validation-baseline.json")
    reference = read_json(directory / "reference-selection.json")
    trained = [read_json(p) for p in sorted(directory.glob("validation-step-*.json"))]
    if not trained:
        raise ValueError("need a genuinely trained comparison arm")
    for result in trained:
        metadata = read_json(result["weights"] + ".json")
        if (not metadata.get("adapter_sha256") or result["weights_sha256"] == baseline["weights_sha256"]
                or result["weights_sha256"] != file_digest(result["weights"])):
            raise ValueError("trained arm must contain a verified non-baseline adapter export")
    best = max(trained, key=quality_key)
    write_json(directory / "selection.json", {"baseline": baseline["weights"], "trained": best["weights"],
               "baseline_report": baseline["report"], "trained_report": best["report"],
               "rule": "validation F2, then all-required, empty accuracy and exact set; always retain trained arm",
               "reference_selection_sha256": file_digest(directory / "reference-selection.json"),
               "reference_report": reference["report"],
               "evaluation_source_sha256": file_digest(__file__),
               "trained_beats_baseline": quality_key(best) > quality_key(baseline)})


def finalize(directory):
    directory = output_path(directory)
    manifest, rows = load(directory)
    selection = read_json(directory / "selection.json")
    if selection["evaluation_source_sha256"] != file_digest(__file__):
        raise ValueError("evaluation code changed after selection")
    claim = {"cases_sha256": manifest["files"]["cases.jsonl"], "selection_sha256": file_digest(directory / "selection.json"),
             "reference_selection_sha256": selection["reference_selection_sha256"],
             "model_hashes": [file_digest(selection[k]) for k in ("baseline", "trained")]}
    with (directory / "holdout.lock.json").open("x", encoding="utf-8") as f:
        f.write(canonical(claim) + "\n")
    results = {k: child_measure(directory, selection[k], "holdout", "holdout-" + k + ".json") for k in ("baseline", "trained")}
    subprocess.run([sys.executable, "-m", "training.needle3.category_reference", "measure", "--out", str(directory)], check=True)
    results["reference"] = read_json(directory / "holdout-reference.json")
    # Group bootstrap: keep DE/EN siblings together in every resample.
    import random
    rng = random.Random(manifest["seed"])
    holdout = [r for r in rows if r["split"] == "holdout"]
    groups = sorted({r["group_id"] for r in holdout})
    deltas = []
    for _ in range(2000):
        sample = [r for g in rng.choices(groups, k=len(groups)) for r in holdout if r["group_id"] == g]
        deltas.append(metrics(sample, results["trained"]["predictions"])["f2"] - metrics(sample, results["baseline"]["predictions"])["f2"])
    deltas.sort()
    write_json(directory / "comparison.json", {"arms": {k: v["report"] for k, v in results.items()},
               "group_count": len(groups), "paired_f2_delta_95_percent": [deltas[50], deltas[1949]],
               "claims": {"local_cpu": True, "production_ready": False, "other_languages_verified": False}})


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=["measure", "select", "finalize"])
    parser.add_argument("--out", required=True)
    parser.add_argument("--weights")
    parser.add_argument("--split", choices=["validation", "holdout"], default="validation")
    parser.add_argument("--result")
    args = parser.parse_args()
    if args.phase == "measure":
        measure(args.out, args.weights, args.split, args.result)
    elif args.phase == "select":
        select(args.out)
    else:
        finalize(args.out)
