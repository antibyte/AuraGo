"""Offline development diagnostics for untrained Needle; never claims the final test."""
from __future__ import annotations

import argparse
from collections import Counter
from concurrent.futures import ProcessPoolExecutor, as_completed
import multiprocessing
import os
from pathlib import Path
import platform
import random
import time
import warnings

from .assets import MODEL_DIR, verify
from .common import HOME, ROOT, canonical, catalog, digest, file_digest, read_json, read_jsonl, repository_revision, write_json, write_jsonl
from .diagnostic_fixtures import catalog_probes, fixture_rows
from .evaluate import paired_f2_advantage, report

MODELS = {"published": MODEL_DIR / "needle" / "needle3.cact", "untrained_w4": MODEL_DIR / "baseline-w4.cact"}
_selector = None


def validate_cases(rows, cat):
    known = {m["id"] for m in cat["manuals"]}
    if len({r["id"] for r in rows}) != len(rows):
        raise ValueError("duplicate diagnostic ID")
    for row in rows:
        gold = row["gold"]
        allowed = row.get("available_manuals")
        if len(gold) > 3 or len(gold) != len(set(gold)) or set(gold) - known:
            raise ValueError("invalid diagnostic labels")
        if allowed is not None and (set(allowed) - known or set(gold) - set(allowed)):
            raise ValueError("gold is not available")
        if row["split"] != "development_diagnostic" or not row["query"].strip():
            raise ValueError("diagnostics cannot consume a sealed split")


def oracle_candidates(row, k=12):
    """Counterfactual selector test only; keep it outside end-to-end metrics."""
    gold = row["gold"]
    result = (gold + [mid for mid in row["candidates"] if mid not in gold])[:k]
    random.Random(digest(row["id"])).shuffle(result)
    return result


def read_cached_cases(output, identity):
    manifest = read_json(output / "manifest.json")
    if manifest["identity"] != identity:
        raise ValueError("diagnostic inputs changed; use a new output directory")
    path = output / "cases.jsonl"
    if file_digest(path) != manifest["cases_sha256"]:
        raise ValueError("cached diagnostic cases changed")
    return list(read_jsonl(path))


def prepare(output, search_binary):
    verify()
    cat = catalog()
    rows = catalog_probes(cat) + fixture_rows(cat)
    validate_cases(rows, cat)
    identity = {"revision": repository_revision(), "catalog_sha256": cat["catalog_sha256"],
                "fixture_sha256": digest(canonical(rows)), "search_binary_sha256": file_digest(search_binary),
                "source_sha256": file_digest(ROOT / "training" / "dataset_native_fc.jsonl"),
                "models": {name: file_digest(path) for name, path in MODELS.items()},
                "code": {name: file_digest(HOME / name) for name in (
                    "diagnostic.py", "diagnostic_fixtures.py", "retrieval.py", "serialization.py", "runtime.py", "evaluate.py")}}
    path = output / "manifest.json"
    if path.exists():
        return read_cached_cases(output, identity)
    from .retrieval import Retriever
    retriever = Retriever(cat, search_binary)
    try:
        for index, row in enumerate(rows):
            started = time.perf_counter()
            details = retriever.retrieve_details(row, row.get("available_manuals"), len(cat["manuals"]))
            row["retrieval_ms"] = (time.perf_counter() - started) * 1000
            row["ranking"] = details["manual_ids"]
            row["embedding_scores"] = details["embedding_scores"]
            row["candidates"] = row["ranking"][:12]
            if index % 25 == 0:
                print(f"Retrieved {index + 1}/{len(rows)}", flush=True)
    finally:
        retriever.close()
    write_jsonl(output / "cases.jsonl", rows)
    manifest = {"identity": identity, "rows": len(rows), "groups": len({r["group_id"] for r in rows}),
                "language_counts": dict(Counter(r["language"] for r in rows)),
                "cohort_counts": dict(Counter(r["cohort"] for r in rows)),
                "manual_families": len({mid for r in rows for mid in r["gold"]}),
                "platform": platform.platform(), "python": platform.python_version(),
                "cpu_count": os.cpu_count(), "api_spend_usd": 0, "trained": False,
                "sealed_test": False, "cases_sha256": file_digest(output / "cases.jsonl")}
    write_json(path, manifest)
    print(canonical({k: v for k, v in manifest.items() if k != "identity"}), flush=True)
    return rows


def make_tasks(rows):
    tasks = []
    # Preselect probes before seeing model outputs. Every minor language appears.
    natural = [r for r in rows if r["cohort"] == "natural_challenge"]
    order_ids = {r["id"] for r in natural if r["language"] not in {"de", "en"}}
    order_ids.update(r["id"] for r in sorted(natural, key=lambda r: digest(r["id"]))[:48])
    for row in rows:
        tasks.append({**row, "variant": "retrieved12", "base_id": row["id"]})
        if set(row["gold"]) - set(row["candidates"]):
            tasks.append({**row, "id": row["id"] + "@oracle12", "base_id": row["id"],
                          "variant": "oracle12", "candidates": oracle_candidates(row)})
        if row["id"] in order_ids:
            tasks.append({**row, "id": row["id"] + "@reverse12", "base_id": row["id"],
                          "variant": "reverse12", "candidates": list(reversed(row["candidates"]))})
        if row["cohort"] == "natural_challenge":
            tasks.append({**row, "id": row["id"] + "@retrieved20", "base_id": row["id"],
                          "variant": "retrieved20", "candidates": row["ranking"][:20]})
    return tasks


def initialize_worker(weights):
    import atexit
    from needle import Needle
    from .runtime import Selector
    global _selector
    warnings.filterwarnings("ignore", message="these weights carry no confidence head")

    class RecordingNeedle(Needle):
        def complete(self, text="", max_new_tokens=256):
            self.diagnostic_response = super().complete(text, max_new_tokens=max_new_tokens)
            return self.diagnostic_response

    _selector = Selector(weights, factory=RecordingNeedle)
    atexit.register(_selector.close)


def predict_one(row):
    started = time.perf_counter()
    try:
        prediction = _selector.select(row, row["candidates"], row.get("available_manuals"))
        raw = _selector.agent.diagnostic_response
        prediction["reported_confidence"] = raw.get("confidence")
        prediction["raw_function_calls"] = raw.get("function_calls")
    except (ValueError, RuntimeError, OSError) as exc:
        prediction = {"manual_ids": [], "formal_valid": False, "fallback_required": True,
                      "error_type": type(exc).__name__, "error": str(exc),
                      "latency_ms": (time.perf_counter() - started) * 1000}
    prediction["pipeline_ms"] = (time.perf_counter() - started) * 1000 + row["retrieval_ms"]
    return {"id": row["id"], "base_id": row["base_id"], "variant": row["variant"], "prediction": prediction}


def predict(output, rows, workers):
    tasks = make_tasks(rows)
    write_jsonl(output / "tasks.jsonl", tasks)
    for name, weights in MODELS.items():
        path = output / (name + ".jsonl")
        completed = {r["id"] for r in read_jsonl(path)} if path.exists() else set()
        remaining = [row for row in tasks if row["id"] not in completed]
        if not remaining:
            continue
        print(f"{name}: {len(remaining)} native complete() calls, {workers} workers", flush=True)
        with path.open("a", encoding="utf-8", newline="\n") as f, ProcessPoolExecutor(
                max_workers=workers, mp_context=multiprocessing.get_context("spawn"),
                initializer=initialize_worker, initargs=(str(weights),)) as pool:
            futures = {pool.submit(predict_one, row): row["id"] for row in remaining}
            for number, future in enumerate(as_completed(futures), 1):
                result = future.result()
                result["prediction"]["parallel_workers"] = workers
                f.write(canonical(result) + "\n")
                f.flush()
                if number % 25 == 0 or number == len(remaining):
                    print(f"{name}: {number}/{len(remaining)} complete", flush=True)


def summarize(rows, predictions):
    value = report(rows, predictions)
    value["overall"]["exact_set_fraction"] = sum(set(predictions[r["id"]]["manual_ids"]) == set(r["gold"]) and predictions[r["id"]]["formal_valid"] for r in rows) / len(rows)
    value["overall"]["scenario_groups"] = len({r["group_id"] for r in rows})
    value["errors"] = dict(Counter(p["error"] for p in predictions.values() if p.get("error")))
    return value


def make_report(output, rows):
    tasks = make_tasks(rows)
    result = {"manifest": read_json(output / "manifest.json"), "models": {}, "retrieval": {},
              "limitations": ["Development diagnostics, never the sealed final test.",
                              "Catalog probes use mechanical repository templates and explicit integration names.",
                              "Minor-language samples are only four translated scenarios per language.",
                              "Oracle results are counterfactual and excluded from end-to-end metrics.",
                              "A baseline measures errors, not the sample count needed to learn them."]}
    for cohort in ("all", "catalog_probe", "natural_challenge"):
        subset = rows if cohort == "all" else [r for r in rows if r["cohort"] == cohort]
        result["retrieval"][cohort] = {}
        for k in (12, 16, 20):
            selected = [{**r, "candidates": r["ranking"][:k]} for r in subset]
            predicted = {r["id"]: {"manual_ids": r["candidates"][:3], "formal_valid": True} for r in selected}
            result["retrieval"][cohort][str(k)] = report(selected, predicted)
    baselines = {}
    for name, mode in (("search_top1", "one"), ("search_top3", "three"), ("embedding_top3", "embedding")):
        baselines[name] = {}
        for row in rows:
            mids = row["candidates"][:1 if mode == "one" else 3]
            if mode == "embedding":
                # Independent reference uses its own real candidate list.
                mids = sorted(row["embedding_scores"], key=lambda mid: (-row["embedding_scores"][mid], mid))[:3]
            baselines[name][row["id"]] = {"manual_ids": mids, "formal_valid": True}
    result["baselines"] = {}
    for name, predictions in baselines.items():
        wide = [{**r, "candidates": r["ranking"]} for r in rows]
        result["baselines"][name] = summarize(wide, predictions)
    for name in MODELS:
        records = list(read_jsonl(output / (name + ".jsonl")))
        predictions = {r["id"]: r["prediction"] for r in records}
        if len(predictions) != len(records) or set(predictions) != {r["id"] for r in tasks}:
            raise ValueError("incomplete or duplicate native results")
        arms = {}
        for variant in ("retrieved12", "oracle12", "reverse12", "retrieved20"):
            subset = [r for r in tasks if r["variant"] == variant]
            if not subset:
                continue
            arms[variant] = summarize(subset, {r["id"]: predictions[r["id"]] for r in subset})
        arms["cohorts"] = {}
        for cohort in ("catalog_probe", "natural_challenge"):
            subset = [r for r in rows if r["cohort"] == cohort]
            arms["cohorts"][cohort] = summarize(subset, {r["id"]: predictions[r["id"]] for r in subset})
        reversed_rows = [r for r in tasks if r["variant"] == "reverse12"]
        arms["order_sensitivity"] = {"rows": len(reversed_rows), "set_changes": sum(
            set(predictions[r["id"]]["manual_ids"]) != set(predictions[r["base_id"]]["manual_ids"]) for r in reversed_rows)}
        selected = {r["id"]: predictions[r["id"]] for r in rows}
        arms["paired_vs_search_top3"] = paired_f2_advantage(rows, selected, baselines["search_top3"])
        result["models"][name] = arms
        errors = [{"row": r, "prediction": selected[r["id"]],
                   "retrieval_miss": sorted(set(r["gold"]) - set(r["candidates"]))}
                  for r in rows if set(selected[r["id"]]["manual_ids"]) != set(r["gold"]) or not selected[r["id"]]["formal_valid"]]
        write_jsonl(output / (name + "-errors.jsonl"), errors)
    write_json(output / "summary.json", result)
    print(canonical({name: arms["retrieved12"]["overall"] for name, arms in result["models"].items()}), flush=True)
    return result


def main():
    global MODELS
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument("--search-binary", default=str(HOME / ".cache" / "catalog-search.exe"))
    parser.add_argument("--phase", choices=("prepare", "predict", "report", "all"), default="all")
    parser.add_argument("--workers", type=int, choices=range(1, 5), default=4)
    parser.add_argument("--models", nargs="+", choices=tuple(MODELS), default=list(MODELS))
    args = parser.parse_args()
    MODELS = {name: MODELS[name] for name in args.models}
    output = Path(args.out).resolve()
    if not output.is_relative_to((ROOT / "reports" / "needle3").resolve()):
        raise ValueError("diagnostic artifacts belong under reports/needle3")
    output.mkdir(parents=True, exist_ok=True)
    # Even resume/report checks input identities before using old predictions.
    rows = prepare(output, args.search_binary)
    if args.phase in {"predict", "all"}:
        predict(output, rows, args.workers)
    if args.phase in {"report", "all"}:
        make_report(output, rows)


if __name__ == "__main__":
    main()
