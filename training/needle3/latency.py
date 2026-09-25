"""Paired CPU depth/latency diagnostics with live retrieval and no response cache."""
from __future__ import annotations

import argparse
from collections import defaultdict
import math
import os
from pathlib import Path
import platform
import statistics
import time
import warnings

from .assets import MODEL_DIR, PINS, verify
from .common import HOME, ROOT, canonical, catalog, file_digest, read_json, read_jsonl, write_json, write_jsonl
from .diagnostic import validate_cases
from .evaluate import metrics

TARGET_MS = 600


def paired_cases(rows):
    """One DE/EN member per natural scenario, chosen without model predictions."""
    groups = defaultdict(dict)
    for row in rows:
        if row["cohort"] == "natural_challenge" and row["language"] in {"de", "en"} and row["kind"] != "unavailable":
            groups[row["group_id"]][row["language"]] = row
    result = []
    for index, group in enumerate(sorted(groups)):
        language = "de" if index % 2 == 0 else "en"
        if set(groups[group]) != {"de", "en"}:
            raise ValueError("paired latency cases require both main languages")
        result.append(groups[group][language])
    if not result:
        raise ValueError("no paired development cases")
    return result


def timing(values):
    ordered = sorted(values)
    if not ordered or any(not math.isfinite(x) or x < 0 for x in ordered):
        raise ValueError("finite, nonnegative timings required")
    return {"samples": len(ordered), "p50_ms": statistics.median(ordered),
            "p95_ms": ordered[math.ceil(.95 * len(ordered)) - 1], "maximum_ms": ordered[-1],
            "under_600_ms": sum(x < TARGET_MS for x in ordered),
            "all_observed_under_600_ms": ordered[-1] < TARGET_MS}


def measure(row, retriever, selector):
    started = time.perf_counter()
    candidates = retriever.retrieve(row, row.get("available_manuals"), 12)
    retrieved = time.perf_counter()
    # Refuse search drift instead of silently comparing different shortlists.
    if candidates != row["candidates"]:
        raise ValueError("live candidate search differs from the frozen diagnostic")
    result = selector.select(row, candidates, row.get("available_manuals"))
    result["end_to_end_ms"] = (time.perf_counter() - started) * 1000
    result["retrieval_ms"] = (retrieved - started) * 1000
    result["completion_ms"] = result["latency_ms"] - result["schema_setup_ms"]
    return result


def benchmark(diagnostic, output, weights, search_binary):
    from needle.model.export import read_layers
    from .retrieval import Retriever
    from .runtime import Selector

    verify()
    output = Path(output).resolve()
    if not output.is_relative_to((ROOT / "reports" / "needle3").resolve()):
        raise ValueError("latency evidence belongs under reports/needle3")
    if output.exists():
        raise ValueError("use a new output directory for a new timing run")
    diagnostic = Path(diagnostic)
    source_manifest = read_json(diagnostic / "manifest.json")
    if file_digest(diagnostic / "cases.jsonl") != source_manifest["cases_sha256"]:
        raise ValueError("diagnostic cases changed")
    cat = catalog()
    if cat["catalog_sha256"] != source_manifest["identity"]["catalog_sha256"]:
        raise ValueError("diagnostic catalog changed")
    if file_digest(search_binary) != source_manifest["identity"]["search_binary_sha256"]:
        raise ValueError("diagnostic search binary changed")
    rows = paired_cases(list(read_jsonl(diagnostic / "cases.jsonl")))
    validate_cases(rows, cat)
    models = {}
    for path in map(Path, weights):
        manifest = read_json(str(path) + ".json")
        layers = read_layers(str(path))
        if (manifest["pins"] != PINS or manifest["layers"] != layers or manifest["weight_bits"] != 4
                or manifest["adapter_sha256"] is not None or manifest["output_sha256"] != file_digest(path)):
            raise ValueError("expected an unchanged, untrained, pinned W4 depth export")
        if path.stem in models:
            raise ValueError("duplicate model name")
        models[path.stem] = {"path": str(path.resolve()), "layers": layers, "manifest": manifest}
    warnings.filterwarnings("ignore", message="these weights carry no confidence head")
    output.mkdir(parents=True)
    write_jsonl(output / "cases.jsonl", rows)
    identity = {"source_cases_sha256": source_manifest["cases_sha256"], "models": models,
                "catalog_sha256": cat["catalog_sha256"], "pins": PINS,
                "runtime_sha256": file_digest(MODEL_DIR / ("windows/libneedle3.dll" if os.name == "nt" else "linux/libneedle3.so")),
                "code": {name: file_digest(HOME / name) for name in ("latency.py", "runtime.py", "retrieval.py", "serialization.py")},
                "platform": platform.platform(), "python": platform.python_version(), "cpu_count": os.cpu_count(),
                "thread_environment": {key: os.environ.get(key) for key in ("OMP_NUM_THREADS", "MKL_NUM_THREADS", "OPENBLAS_NUM_THREADS")},
                "target_ms": TARGET_MS, "trained": False, "api_spend_usd": 0,
                "limitations": ["Development sample, not final-test accuracy or a real-time guarantee.",
                                "Repeated identical requests measure prefix reuse, not general changing-candidate latency.",
                                "No response cache; both modes include live E5 and catalog retrieval.",
                                "One CPU worker; system background load is not controlled."]}
    write_json(output / "manifest.json", identity)
    started = time.perf_counter()
    retriever = Retriever(cat, search_binary)
    startup_ms = (time.perf_counter() - started) * 1000
    retriever.retrieve({"query": "List Docker containers.", "context": []}, k=12)
    results = {}
    try:
        for name, model in models.items():
            selector = Selector(model["path"], cat)
            records = []
            try:
                # Separate first native initialization from the measured service loop.
                started = time.perf_counter()
                selector.select(rows[-1], rows[-1]["candidates"])
                cold_ms = (time.perf_counter() - started) * 1000
                with (output / (name + ".jsonl")).open("w", encoding="utf-8", newline="\n") as stream:
                    for index, row in enumerate(rows):
                        for mode in ("live_request", "repeat_same_request"):
                            result = {"id": row["id"], "mode": mode, "prediction": measure(row, retriever, selector)}
                            records.append(result)
                            stream.write(canonical(result) + "\n")
                            stream.flush()
                        if (index + 1) % 10 == 0 or index + 1 == len(rows):
                            print(f"{name}: {index + 1}/{len(rows)} paired requests", flush=True)
            finally:
                selector.close()
            summary = {"layers": model["layers"], "cold_native_initialization_and_query_ms": cold_ms}
            for mode in ("live_request", "repeat_same_request"):
                predictions = {r["id"]: r["prediction"] for r in records if r["mode"] == mode}
                summary[mode] = {"quality": metrics(rows, predictions),
                                 "timing": {key: timing([p[key] for p in predictions.values()]) for key in
                                            ("end_to_end_ms", "retrieval_ms", "schema_setup_ms", "completion_ms")}}
            summary["repeat_output_changes"] = sum(records[i]["prediction"]["manual_ids"] != records[i + 1]["prediction"]["manual_ids"]
                                                   for i in range(0, len(records), 2))
            results[name] = summary
            write_json(output / "summary.json", {"identity": identity, "retriever_startup_ms": startup_ms,
                                                  "complete": len(results) == len(models), "rows": len(rows), "models": results})
            print(canonical({name: summary["live_request"]}), flush=True)
    finally:
        retriever.close()
    return results


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--diagnostic", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--weights", nargs="+", required=True)
    parser.add_argument("--search-binary", default=str(HOME / ".cache" / "catalog-search.exe"))
    args = parser.parse_args()
    benchmark(args.diagnostic, args.out, args.weights, args.search_binary)


if __name__ == "__main__":
    main()
