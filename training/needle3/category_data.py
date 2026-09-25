"""Freeze a small source-grounded category pack, separate from the 70k corpus."""
from __future__ import annotations

import argparse
from collections import Counter
import math

from .assets import PINS, tokenizer, verify
from .category_fixtures import TRAIN, VALIDATION, HOLDOUT, CONTEXT
from .category_router import CATEGORIES, contract_hash, encode, taxonomy
from .common import HOME, canonical, catalog, file_digest, read_json, read_jsonl, repository_revision, verify_files, write_json, write_jsonl
from .local_pilot import output_path


def cases(cat):
    mapping = taxonomy(cat)
    hashes = {m["id"]: m["sha256"] for m in cat["manuals"]}
    result = []

    def add(split, group, mids, de, en, contexts=None, kind=None):
        manuals = mids.split(",") if mids else []
        labels = sorted({mapping[m] for m in manuals}, key=CATEGORIES.index)
        kind = kind or ("none" if not labels else "multi" if len(labels) > 1 else "single")
        for lang, text in (("de", de), ("en", en)):
            result.append({"id": group + "-" + lang, "group_id": group, "split": split,
                           "language": lang, "query": text, "context": (contexts or {}).get(lang, []),
                           "categories": labels, "manual_ids": manuals, "kind": kind,
                           "sources": {m: hashes[m] for m in manuals}})

    for split, block in (("train", TRAIN), ("validation", VALIDATION), ("holdout", HOLDOUT)):
        for i, line in enumerate(block.splitlines()):
            mids, de, en = line.split("|")
            add(split, f"category-{split}-{i:03d}", mids, de, en)
    for i, (split, mids, de, en, prior_de, prior_en) in enumerate(CONTEXT):
        add(split, f"category-context-{i:03d}", mids, de, en, {"de": [prior_de], "en": [prior_en]}, "context")
    # Lookup probes teach exhaustive catalog membership. They are deliberately
    # marked mechanical and never count as natural holdout success.
    for mid in sorted(mapping):
        add("train", "category-catalog-" + mid, mid,
            f"In welchem Bereich finde ich die Bedienungsanleitung für {mid}?",
            f"Which area contains the usage manual for {mid}?", kind="catalog")
    validate(result, cat)
    return result


def validate(rows, cat):
    mapping, ids, groups, queries = taxonomy(cat), set(), {}, {}
    hashes = {m["id"]: m["sha256"] for m in cat["manuals"]}
    for row in rows:
        if row["id"] in ids or row["split"] not in {"train", "validation", "holdout"}:
            raise ValueError("duplicate category ID or invalid split")
        ids.add(row["id"])
        for index, key in ((groups, row["group_id"]), (queries, canonical([row["context"], row["query"].casefold()]))):
            if index.setdefault(key, row["split"]) != row["split"]:
                raise ValueError("category scenario leaked across splits")
        if (set(row["categories"]) != {mapping[m] for m in row["manual_ids"]}
                or row["sources"] != {m: hashes[m] for m in row["manual_ids"]}):
            raise ValueError("category label differs from source manuals")
    for split in ("train", "validation", "holdout"):
        subset = [r for r in rows if r["split"] == split]
        if ({c for r in subset for c in r["categories"]} != set(CATEGORIES)
                or not {"single", "multi", "none", "context"} <= {r["kind"] for r in subset}):
            raise ValueError("every partition must exercise all categories and challenge kinds")


def prepare(directory):
    directory = output_path(directory)
    if directory.exists():
        raise ValueError("category preparation requires a new directory")
    verify()
    cat, tok = catalog(), tokenizer()
    rows, compiled = cases(cat), []
    for row in rows:
        ids, mask = encode(row, tok)
        if row["split"] == "train":
            compiled.append({"id": row["id"], "group_id": row["group_id"], "kind": row["kind"],
                             "categories": row["categories"], "ids": ids, "mask": mask})
    directory.mkdir(parents=True)
    write_jsonl(directory / "cases.jsonl", rows)
    write_jsonl(directory / "train-tokens.jsonl", compiled)
    manifest = {"purpose": "offline_category_experiment_not_production_acceptance", "seed": 20260926,
                "pins": PINS, "contract_sha256": contract_hash(cat), "catalog_sha256": cat["catalog_sha256"],
                "repository_revision": repository_revision(), "mapping": taxonomy(cat),
                "files": {p: file_digest(directory / p) for p in ("cases.jsonl", "train-tokens.jsonl")},
                "source": {p: file_digest(HOME / p) for p in ("category_router.py", "category_data.py", "category_fixtures.py")},
                "rows": dict(Counter(r["split"] for r in rows)),
                "groups": {s: len({r["group_id"] for r in rows if r["split"] == s}) for s in ("train", "validation", "holdout")},
                "kinds": {s: dict(Counter(r["kind"] for r in rows if r["split"] == s)) for s in ("train", "validation", "holdout")},
                "bucket": math.ceil(max(len(r["ids"]) for r in compiled) / 64) * 64,
                "api_spend_usd": 0, "limitations": ["Hand-authored synthetic DE/EN goals; translations are dependent.",
                    "Catalog lookup probes provide mapping coverage, not natural-language coverage of every manual.",
                    "Category membership inherits the existing heterogeneous discovery catalog.",
                    "No production, multilingual, or Linux acceptance is implied."]}
    write_json(directory / "manifest.json", manifest)
    print(canonical({k: manifest[k] for k in ("rows", "groups", "kinds", "bucket")}), flush=True)


def load(directory):
    directory = output_path(directory)
    manifest = read_json(directory / "manifest.json")
    verify_files(directory, manifest["files"])
    verify_files(HOME, manifest["source"])
    cat = catalog()
    if manifest["pins"] != PINS or manifest["contract_sha256"] != contract_hash(cat):
        raise ValueError("category model/catalog contract changed")
    rows = list(read_jsonl(directory / "cases.jsonl"))
    validate(rows, cat)
    return manifest, rows


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    prepare(parser.parse_args().out)
