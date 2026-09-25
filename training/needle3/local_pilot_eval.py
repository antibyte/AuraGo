"""Native validation selection and a once-claimed local development holdout."""
from __future__ import annotations

import argparse
from contextlib import nullcontext
import os
from pathlib import Path
import subprocess
import sys
import time
from unittest.mock import patch
import warnings

from .assets import PINS
from .common import HOME, canonical, catalog, file_digest, read_json, write_json
from .evaluate import paired_f2_advantage, report
from .latency import measure, timing
from .local_pilot import load_pack, output_path


def search_predictions(rows, count, threshold):
    return {row["id"]: {"manual_ids": [mid for mid in row["candidates"][:count]
                                       if row["embedding_scores"][mid] >= threshold],
                         "formal_valid": True} for row in rows}


def quality_key(result):
    overall = result["overall"]
    return overall["f2"], overall["recall_at_3"] or 0, overall["empty_accuracy"] or 0


def best_trained(models):
    # The experimental arm must actually contain training, including when every
    # checkpoint loses to baseline. Otherwise the comparison hides regressions.
    trained = [model for model in models if model["step"] > 0]
    if not trained:
        raise ValueError("no completed training checkpoint to compare")
    return max(trained, key=lambda model: (*quality_key(model["validation"]), -model["step"]))


def choose_search(validation):
    # All reference hyperparameters are selected without looking at the holdout.
    if not validation or any(row["split"] != "validation" for row in validation):
        raise ValueError("reference selection requires validation only")
    options = []
    for count in (1, 2, 3):
        for threshold in [-1.] + [value / 100 for value in range(60, 96)]:
            scores = report(validation, search_predictions(validation, count, threshold))
            options.append((quality_key(scores), -count, threshold, scores))
    _, negative_count, threshold, scores = max(options, key=lambda x: x[:3])
    return {"count": -negative_count, "threshold": threshold, "validation": scores}


def measure_model(directory, weights, split, result_path, resident=False):
    """A fresh process owns exactly one fixed model, including private-hook runs."""
    from needle import Needle
    from needle.model.export import read_layers
    from .retrieval import Retriever
    from .runtime import Selector
    directory = output_path(directory)
    result_path = output_path(result_path)
    if result_path.exists():
        raise ValueError("refusing to overwrite native evaluation evidence")
    manifest, all_rows = load_pack(directory)
    rows = [r for r in all_rows if r["split"] == split]
    weights = Path(weights).resolve()
    sidecar = read_json(str(weights) + ".json")
    if (sidecar["pins"] != PINS or sidecar["layers"] != 4 or read_layers(str(weights)) != 4
            or sidecar["weight_bits"] != 4 or sidecar["output_sha256"] != file_digest(weights)):
        raise ValueError("expected a verified pinned four-layer W4 export")
    if split == "holdout":
        claim = read_json(directory / "holdout.lock.json")
        if (claim["cases_sha256"] != manifest["files"]["cases.jsonl"]
                or claim["selection_sha256"] != file_digest(directory / "selection.json")
                or file_digest(weights) not in claim["model_hashes"]):
            raise ValueError("holdout model was not frozen before evaluation")
    warnings.filterwarnings("ignore", message="these weights carry no confidence head")
    cat = catalog()
    start = time.perf_counter()
    retriever = Retriever(cat, manifest["search_binary"])

    def factory(**kwargs):
        if Path(kwargs.pop("weights")).resolve() != weights:
            raise ValueError("resident experiment cannot change models in one process")
        return Needle(generation=3, **kwargs)

    hook = patch("needle._base_weights_path", return_value=str(weights)) if resident else nullcontext()
    predictions = {}
    with hook:
        selector = Selector(weights, cat, factory=factory if resident else None)
        try:
            warmup = {"query": "Show the current Fritz!Box connection status.", "context": []}
            selector.select(warmup, retriever.retrieve(warmup, k=12))
            startup_ms = (time.perf_counter() - start) * 1000
            for row in rows:
                prediction = measure(row, retriever, selector)
                prediction["native_path_ms"] = prediction["latency_ms"]
                prediction["latency_ms"] = prediction["end_to_end_ms"]
                predictions[row["id"]] = prediction
        finally:
            selector.close()
            retriever.close()
    scores = report(rows, predictions)
    scores["overall"]["exact_set_accuracy"] = sum(set(r["gold"]) == set(predictions[r["id"]]["manual_ids"])
                                                   and predictions[r["id"]]["formal_valid"] for r in rows) / len(rows)
    result = {"split": split, "model_sha256": file_digest(weights), "predictions": predictions,
              "metrics": scores, "timing": timing([r["end_to_end_ms"] for r in predictions.values()]),
              "startup_ms": startup_ms, "resident_prototype": resident,
              "private_hook": "needle._base_weights_path" if resident else None,
              "fresh_single_model_process": True, "live_retrieval": True,
              "runtime_source_sha256": file_digest(HOME / "runtime.py"),
              "evaluation_source_sha256": file_digest(__file__)}
    write_json(result_path, result)
    print(canonical({"result": str(result_path), "metrics": scores["overall"], "timing": result["timing"]}), flush=True)


def child_measure(directory, weights, split, name, resident=False):
    path = directory / (name + ".json")
    command = [sys.executable, "-m", "training.needle3.local_pilot_eval", "measure", "--out", str(directory),
               "--weights", str(weights), "--split", split, "--result", str(path)]
    if resident:
        command.append("--resident")
    env = {**os.environ, "OMP_NUM_THREADS": "2", "MKL_NUM_THREADS": "2", "OPENBLAS_NUM_THREADS": "2",
           "DO_NOT_TRACK": "1", "HF_HUB_OFFLINE": "1", "TRANSFORMERS_OFFLINE": "1"}
    subprocess.run(command, check=True, env=env)
    return read_json(path)


def select(directory):
    from .export import export
    directory = output_path(directory)
    _, rows = load_pack(directory)
    if (directory / "selection.json").exists() or (directory / "holdout.lock.json").exists():
        raise ValueError("pilot selection is already frozen")
    validation = [r for r in rows if r["split"] == "validation"]
    adapters = read_json(directory / "adapters.json")
    models = []
    for step, saved in sorted(adapters.items(), key=lambda x: int(x[0])):
        if file_digest(saved["path"]) != saved["sha256"]:
            raise ValueError("adapter checkpoint changed")
        weights = directory / "models" / f"step-{int(step):04d}.cact"
        export(weights, adapter=saved["path"] if int(step) else None, layers=4)
        result = child_measure(directory, weights, "validation", f"validation-{int(step):04d}")
        models.append({"step": int(step), "path": str(weights), "sha256": file_digest(weights), "validation": result["metrics"]})
    if not models or models[0]["step"] != 0:
        raise ValueError("matched untrained baseline is missing")
    selected = best_trained(models)
    selection = {"models": models, "selected": selected, "baseline": models[0],
                 "simple_reference": choose_search(validation), "criterion": "validation F2, recall, empty accuracy, earliest step",
                 "trained_beats_baseline_on_validation": quality_key(selected["validation"]) > quality_key(models[0]["validation"]),
                 "selection_source_sha256": file_digest(__file__), "holdout_used": False}
    write_json(directory / "selection.json", selection)
    print(canonical({"selected_step": selected["step"], "validation": selected["validation"]["overall"]}), flush=True)


def finalize(directory):
    directory = output_path(directory)
    manifest, all_rows = load_pack(directory)
    rows = [r for r in all_rows if r["split"] == "holdout"]
    selection = read_json(directory / "selection.json")
    if selection["selection_source_sha256"] != file_digest(__file__):
        raise ValueError("evaluation source changed after validation selection")
    claim = {"cases_sha256": manifest["files"]["cases.jsonl"],
             "selection_sha256": file_digest(directory / "selection.json"),
             "model_hashes": [selection[key]["sha256"] for key in ("baseline", "selected")],
             "purpose": "local_development_holdout_not_the_sealed_final_test"}
    with (directory / "holdout.lock.json").open("x", encoding="utf-8") as f:
        f.write(canonical(claim) + "\n")
    arms = {}
    for name, key in (("baseline", "baseline"), ("trained", "selected")):
        arms[name] = child_measure(directory, selection[key]["path"], "holdout", "holdout-" + name)
    reference = selection["simple_reference"]
    search = search_predictions(rows, reference["count"], reference["threshold"])
    arms["search"] = {"predictions": search, "metrics": report(rows, search), "configuration": reference}
    resident = child_measure(directory, selection["selected"]["path"], "holdout", "holdout-resident", resident=True)
    differences = [r["id"] for r in rows if any(resident["predictions"][r["id"]][key] != arms["trained"]["predictions"][r["id"]][key]
                                              for key in ("manual_ids", "formal_valid"))]
    comparison = {"scope": manifest["purpose"], "selected_step": selection["selected"]["step"],
                  "arms": {key: value["metrics"] for key, value in arms.items()},
                  "trained_vs_baseline": paired_f2_advantage(rows, arms["trained"]["predictions"], arms["baseline"]["predictions"]),
                  "trained_vs_search": paired_f2_advantage(rows, arms["trained"]["predictions"], search),
                  "timing": {key: arms[key]["timing"] for key in ("baseline", "trained")},
                  "resident_timing": resident["timing"], "resident_output_differences": differences,
                  "resident_timing_comparable": not differences, "api_spend_usd": 0,
                  "limitations": manifest["limitations"] + [
                      "Only 24 held-out scenario groups; this cannot determine the necessary full training-data volume.",
                      "Resident timing uses a private binding hook; it is an isolated prototype, not production integration.",
                      "Windows CPU evaluation only; Linux export validation remains separate.",
                      "Observed latency is not a real-time guarantee; startup is reported separately."]}
    write_json(directory / "comparison.json", comparison)
    print(canonical(comparison), flush=True)


def main():
    os.environ.update({"DO_NOT_TRACK": "1", "HF_HUB_OFFLINE": "1", "TRANSFORMERS_OFFLINE": "1", "JAX_PLATFORMS": "cpu"})
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=["select", "finalize", "measure"])
    parser.add_argument("--out", required=True)
    parser.add_argument("--weights")
    parser.add_argument("--split", choices=["validation", "holdout"], default="validation")
    parser.add_argument("--result")
    parser.add_argument("--resident", action="store_true")
    args = parser.parse_args()
    if args.phase == "measure":
        if not args.weights or not args.result:
            parser.error("measure requires --weights and --result")
        measure_model(args.out, args.weights, args.split, args.result, args.resident)
    elif args.phase == "select":
        select(args.out)
    else:
        finalize(args.out)


if __name__ == "__main__":
    main()
