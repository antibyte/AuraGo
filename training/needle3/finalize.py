"""Validation-only selection followed by one frozen, comparative test execution."""
from __future__ import annotations

import argparse
from collections import defaultdict
from pathlib import Path
import time

from .common import canonical, config, digest, file_digest, read_json, read_jsonl, write_json
from .evaluate import claim_final_test, paired_f2_advantage, report, run_model
from .export import export


def selection_score(result):
    # Recall is deliberately first; F2 and precision break ties.
    overall = result["overall"]
    return (overall["recall_at_3"] or 0, overall["f2"], overall["precision"])


def stratified_probe(rows, per_language=16):
    groups = defaultdict(list)
    for row in rows:
        groups[row["language"]].append(row)
    return [row for language in sorted(groups) for row in sorted(groups[language], key=lambda r: digest(r["id"]))[:per_language]]


def simple_predictions(rows, n, threshold=-1):
    return {row["id"]: {"manual_ids": [mid for mid in row["retrieved"] if row["retrieved_scores"].get(mid, -2) >= threshold][:n], "formal_valid": True} for row in rows}


def finalize(args):
    pack, runs = Path(args.pack), Path(args.runs)
    validation = list(read_jsonl(pack / "validation.jsonl"))
    probe = stratified_probe(validation)
    models = runs / "models"
    models.mkdir(exist_ok=True)
    baseline = models / "untrained-w4.cact"
    export(baseline)
    candidates = []
    # Score regularly saved main checkpoints on a fixed stratified validation
    # probe. Cap the number BEFORE evaluation so test data cannot influence it.
    paths = sorted((runs / "main" / "checkpoints").glob("step-*/adapter.safetensors"))
    if not paths:
        raise ValueError("no trained adapters exist")
    indices = sorted({round(i * (len(paths) - 1) / min(7, len(paths) - 1)) for i in range(min(8, len(paths)))} if len(paths) > 1 else {0})
    for i in indices:
        if time.time() + 1800 >= args.deadline:
            raise TimeoutError("insufficient reserve for validation and the frozen test")
        adapter = paths[i]
        model = models / (adapter.parent.name + ".cact")
        export(model, str(adapter))
        result = report(probe, run_model(probe, model))
        candidates.append({"model": str(model), "adapter": str(adapter), "probe_metrics": result})
    candidates.sort(key=lambda item: selection_score(item["probe_metrics"]), reverse=True)
    full = []
    for candidate in candidates[:2]:
        result = report(validation, run_model(validation, candidate["model"]))
        full.append({**candidate, "validation_metrics": result, "model_sha256": file_digest(candidate["model"]), "adapter_sha256": file_digest(candidate["adapter"])})
    selected = max(full, key=lambda c: selection_score(c["validation_metrics"]))
    baselines = [{"top_n": n, "threshold": threshold, "validation_metrics": report(validation, simple_predictions(validation, n, threshold))}
                 for n in (0, 1, 2, 3) for threshold in (-1, .7, .75, .8, .85, .9, .95)]
    simple = max(baselines, key=lambda c: (c["validation_metrics"]["overall"]["f2"], selection_score(c["validation_metrics"])))
    pilots = {}
    for arm in ("pilot_de_en", "pilot_multilingual"):
        saved = read_json(runs / arm / "latest_adapter.json")
        model = models / (arm + ".cact")
        export(model, saved["path"])
        pilots[arm] = report(validation, run_model(validation, model))
    de_en_regression = {lang: (pilots["pilot_multilingual"]["language"][lang]["recall_at_3"] or 0) - (pilots["pilot_de_en"]["language"][lang]["recall_at_3"] or 0) for lang in ("de", "en")}
    selection = {"selected": selected, "simple_reference": simple, "candidates": full, "pilot_metrics": pilots,
                 "multilingual_pilot_de_en_recall_delta": de_en_regression,
                 "validation_sha256": file_digest(pack / "validation.jsonl"), "baseline_sha256": file_digest(baseline)}
    write_json(runs / "selection.json", selection)
    if time.time() + 1800 >= args.deadline:
        raise TimeoutError("not enough final-test reserve; test remains sealed")
    claim_final_test(runs, file_digest(pack / "test.jsonl"), file_digest(runs / "selection.json"))
    test = list(read_jsonl(pack / "test.jsonl"))
    predictions = {"search": simple_predictions(test, simple["top_n"], simple["threshold"])}
    write_json(runs / "test-predictions-search.json", predictions["search"])
    for name, weights in (("untrained_w4", baseline), ("trained_w4", selected["model"])):
        predictions[name] = run_model(test, weights)
        write_json(runs / ("test-predictions-" + name + ".json"), predictions[name])
    results = {name: report(test, values) for name, values in predictions.items()}
    tuned = results["trained_w4"]
    language_targets = all((tuned["language"].get(lang, {}).get("recall_at_3") or 0) >= (0.95 if lang in {"de", "en"} else 0.9) for lang in config()["languages"])
    advantage = {name: paired_f2_advantage(test, predictions["trained_w4"], predictions[name]) for name in ("search", "untrained_w4")}
    recommendation = language_targets and (tuned["overall"]["empty_accuracy"] or 0) >= .95 and tuned["overall"]["formal_valid_fraction"] >= .995 and all(result["positive_advantage"] for result in advantage.values())
    write_json(runs / "evaluation.json", {"results": results, "synthetic_acceptance_passed": recommendation,
                                          "paired_f2_advantage": advantage, "production_validated": False, "confidence": None, "selection_sha256": file_digest(runs / "selection.json")})


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pack", required=True)
    parser.add_argument("--runs", required=True)
    parser.add_argument("--deadline", type=float, required=True)
    finalize(parser.parse_args())
