"""Frozen multilingual E5 plus a tiny linear head, calibrated on validation only."""
from __future__ import annotations

import argparse
import os
import time

from .assets import MODEL_DIR, PINS
from .category_data import load
from .category_eval import report, quality_key
from .category_router import CATEGORIES
from .category_train import sampling_weights
from .common import canonical, file_digest, inference_query, read_json, write_json
from .local_pilot import output_path


def encoder():
    import torch
    from sentence_transformers import SentenceTransformer
    torch.set_num_threads(2)
    return SentenceTransformer(str(MODEL_DIR / "retriever"), device="cpu", local_files_only=True)


def vectors(model, rows):
    return model.encode(["query: " + inference_query(row) for row in rows], batch_size=16,
                        normalize_embeddings=True, show_progress_bar=False)


def predictions(rows, probabilities, threshold, mapping, gate=None, gate_threshold=0.):
    result = {}
    for index, (row, values) in enumerate(zip(rows, probabilities)):
        categories = [c for c, p in zip(CATEGORIES, values) if p >= threshold]
        if gate is not None and gate[index] < gate_threshold:
            categories = []
        allowed = row.get("available_manuals", list(mapping))
        categories = [c for c in categories if any(mapping[mid] == c for mid in allowed)]
        result[row["id"]] = {"category_ids": categories, "formal_valid": True, "latency_ms": 0.,
                             "manuals_by_category": {c: [m for m in allowed if mapping[m] == c] for c in categories}}
    return result


def fit(directory):
    import numpy as np
    from sklearn.linear_model import LogisticRegression
    directory = output_path(directory)
    if (directory / "reference-selection.json").exists():
        raise ValueError("reference is already frozen")
    manifest, rows = load(directory)
    training = [r for r in rows if r["split"] == "train"]
    validation = [r for r in rows if r["split"] == "validation"]
    model = encoder()
    x_train, x_validation = vectors(model, training), vectors(model, validation)
    weights = np.array(sampling_weights(training)) * len(training)
    labels = np.array([[c in r["categories"] for c in CATEGORIES] for r in training])
    choices = []
    for strength in (1., 4., 16., 64.):
        heads = [LogisticRegression(C=strength, max_iter=500).fit(x_train, labels[:, i], sample_weight=weights) for i in range(len(CATEGORIES))]
        gate = LogisticRegression(C=strength, max_iter=500).fit(x_train, labels.any(axis=1), sample_weight=weights)
        gate_probabilities = gate.predict_proba(x_validation)[:, 1]
        probabilities = np.stack([head.predict_proba(x_validation)[:, 1] for head in heads], axis=1)
        for threshold in np.arange(.05, .76, .025):
            for gate_threshold in (0., .25, .5, .75, .9):
                pred = predictions(validation, probabilities, float(threshold), manifest["mapping"], gate_probabilities, gate_threshold)
                result = {"report": report(validation, pred)}
                choices.append((quality_key(result), strength, float(threshold), gate_threshold, heads, gate, result))
    _, strength, threshold, gate_threshold, heads, gate, result = max(choices, key=lambda x: x[0])
    head_path = directory / "reference-head.npz"
    np.savez(head_path, coef=np.concatenate([head.coef_ for head in heads]), intercept=np.concatenate([head.intercept_ for head in heads]),
             gate_coef=gate.coef_, gate_intercept=gate.intercept_)
    write_json(directory / "reference-selection.json", {"C": strength, "threshold": threshold, "gate_threshold": gate_threshold,
               "head_sha256": file_digest(head_path), "categories": CATEGORIES, "encoder_pin": PINS["retriever"],
               "manifest_sha256": file_digest(directory / "manifest.json"), "source_sha256": file_digest(__file__),
               "selection": "validation F2, all-required, empty accuracy, exact set", **result})
    print(canonical({"reference": "frozen_e5_linear", "C": strength, "threshold": threshold, "gate_threshold": gate_threshold,
                     **result["report"]["overall"]}), flush=True)


def measure(directory):
    import numpy as np
    directory = output_path(directory)
    output = directory / "holdout-reference.json"
    if output.exists():
        raise ValueError("reference holdout cannot be overwritten")
    manifest, rows = load(directory)
    selected = read_json(directory / "reference-selection.json")
    claim = read_json(directory / "holdout.lock.json")
    if (claim["reference_selection_sha256"] != file_digest(directory / "reference-selection.json")
            or claim["cases_sha256"] != manifest["files"]["cases.jsonl"]
            or claim["selection_sha256"] != file_digest(directory / "selection.json")
            or selected["manifest_sha256"] != file_digest(directory / "manifest.json")
            or selected["categories"] != list(CATEGORIES)
            or selected["head_sha256"] != file_digest(directory / "reference-head.npz")
            or selected["source_sha256"] != file_digest(__file__)
            or selected["encoder_pin"] != PINS["retriever"]):
        raise ValueError("reference differs from the frozen holdout claim")
    with np.load(directory / "reference-head.npz", allow_pickle=False) as saved:
        coef, intercept = saved["coef"], saved["intercept"]
        gate_coef, gate_intercept = saved["gate_coef"], saved["gate_intercept"]
    begun = time.perf_counter()
    model = encoder()
    vectors(model, [{"query": "Hello"}])
    startup_ms = (time.perf_counter() - begun) * 1000
    subset = [r for r in rows if r["split"] == "holdout"]
    results = {}
    for row in subset:
        begun = time.perf_counter()
        x = vectors(model, [row])
        p = 1 / (1 + np.exp(-(x @ coef.T + intercept)))
        gate = (1 / (1 + np.exp(-(x @ gate_coef.T + gate_intercept))))[:, 0]
        pred = predictions([row], p, selected["threshold"], manifest["mapping"], gate, selected["gate_threshold"])[row["id"]]
        pred["latency_ms"] = (time.perf_counter() - begun) * 1000
        results[row["id"]] = pred
    result = {"report": report(subset, results), "predictions": results, "startup_ms": startup_ms,
              "reference": "frozen_e5_linear", "selection_sha256": file_digest(directory / "reference-selection.json")}
    write_json(output, result)
    print(canonical({**result["report"]["overall"], "latency": result["report"]["latency"]}), flush=True)


if __name__ == "__main__":
    os.environ.update({"DO_NOT_TRACK": "1", "HF_HUB_OFFLINE": "1", "TRANSFORMERS_OFFLINE": "1"})
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=["fit", "measure"])
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    fit(args.out) if args.phase == "fit" else measure(args.out)
