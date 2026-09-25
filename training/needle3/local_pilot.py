"""Bounded, offline CPU LoRA pilot; never bypasses the separate GPU gates."""
from __future__ import annotations

import argparse
from collections import Counter
import math
import os
from pathlib import Path
import platform
import time

from .assets import MODEL_DIR, PINS, tokenizer, verify
from .common import (HOME, ROOT, canonical, catalog, file_digest, read_json,
                     read_jsonl, repository_revision, verify_files, write_json, write_jsonl)
from .local_pilot_data import pilot_rows
from .serialization import encode_exact, example

SEED = 20260925


def output_path(path):
    path = Path(path).resolve()
    if not path.is_relative_to((ROOT / "reports" / "needle3").resolve()):
        raise ValueError("local pilot artifacts belong under ignored reports/needle3")
    return path


def validate_splits(rows):
    ids, groups, queries = set(), {}, {}
    for row in rows:
        if row["id"] in ids or row["split"] not in {"train", "validation", "holdout"}:
            raise ValueError("duplicate ID or invalid pilot split")
        ids.add(row["id"])
        for index, key in ((groups, row["group_id"]), (queries, canonical([row["context"], row["query"].casefold()]))):
            if index.setdefault(key, row["split"]) != row["split"]:
                raise ValueError("scenario or query leaked across pilot splits")
        if len(row["gold"]) > 3 or set(row["gold"]) != set(row["sources"]):
            raise ValueError("pilot labels lack source evidence")


def prepare(directory, search_binary):
    import random
    from .retrieval import Retriever
    directory = output_path(directory)
    if directory.exists():
        raise ValueError("prepare requires a new experiment directory")
    verify()
    cat, tok = catalog(), tokenizer()
    rows = pilot_rows(cat)
    validate_splits(rows)
    retriever = Retriever(cat, search_binary)
    rng = random.Random(SEED)
    compiled, injections = [], []
    try:
        for row in rows:
            result = retriever.retrieve_details(row, k=12)
            row["candidates"] = result["manual_ids"]
            row["embedding_scores"] = result["embedding_scores"]
            if row["split"] == "train":
                candidates = list(row["candidates"])
                missing = [mid for mid in row["gold"] if mid not in candidates]
                if missing:
                    candidates = (row["gold"] + [mid for mid in candidates if mid not in row["gold"]])[:12]
                    injections.append({"id": row["id"], "added": missing})
                rng.shuffle(candidates)
                target = {**row, "answers": row["gold"], "reasoning": (
                    "The request needs " + ", ".join(row["gold"]) + "." if row["gold"] else "Answer directly without a manual.")}
                ids, mask = encode_exact(example(target, cat, candidates, training=True), tok)
                compiled.append({"id": row["id"], "ids": ids, "mask": mask, "candidates": candidates})
    finally:
        retriever.close()
    bucket = math.ceil(max(len(r["ids"]) for r in compiled) / 128) * 128
    if bucket > 1536:
        raise ValueError("CPU pilot exceeded its padding bound; shorten candidates explicitly, never truncate")
    directory.mkdir(parents=True)
    write_jsonl(directory / "cases.jsonl", rows)
    write_jsonl(directory / "train-tokens.jsonl", compiled)
    manifest = {"purpose": "exploratory_local_cpu_pilot_not_accepted_corpus", "seed": SEED,
                "pins": PINS, "catalog_sha256": cat["catalog_sha256"], "repository_revision": repository_revision(),
                "search_binary": str(Path(search_binary).resolve()), "search_binary_sha256": file_digest(search_binary),
                "files": {name: file_digest(directory / name) for name in ("cases.jsonl", "train-tokens.jsonl")},
                "source": {name: file_digest(HOME / name) for name in (
                    "local_pilot.py", "local_pilot_data.py", "diagnostic_fixtures.py", "serialization.py", "retrieval.py")},
                "rows": dict(Counter(r["split"] for r in rows)),
                "groups": {split: len({r["group_id"] for r in rows if r["split"] == split})
                           for split in ("train", "validation", "holdout")},
                "training_manual_families": len({mid for r in rows if r["split"] == "train" for mid in r["gold"]}),
                "bucket": bucket, "training_gold_injections": injections,
                "heldout_candidates_repaired": False, "api_spend_usd": 0,
                "limitations": ["Hand-authored narrow DE/EN development sample, not full-catalog acceptance.",
                                "Previously inspected natural diagnostics now belong only to pilot training.",
                                "Translations remain in the same split and are dependent observations."]}
    write_json(directory / "manifest.json", manifest)
    print(canonical({key: manifest[key] for key in ("rows", "groups", "bucket", "training_manual_families")}), flush=True)


def load_pack(directory):
    directory = output_path(directory)
    manifest = read_json(directory / "manifest.json")
    verify_files(directory, manifest["files"])
    verify_files(HOME, manifest["source"])
    if manifest["pins"] != PINS or manifest["catalog_sha256"] != catalog()["catalog_sha256"]:
        raise ValueError("pilot model or catalog identity changed")
    if file_digest(manifest["search_binary"]) != manifest["search_binary_sha256"]:
        raise ValueError("pilot search binary changed")
    rows = list(read_jsonl(directory / "cases.jsonl"))
    validate_splits(rows)
    return manifest, rows


def train(directory, steps=240, max_seconds=900, lr=3e-4):
    """One CPU microbatch per update; exact resume binds optimizer and schedule."""
    if not 1 <= steps <= 1000 or not 1 <= max_seconds <= 1800 or not 0 < lr <= .001:
        raise ValueError("local pilot is limited to 1000 steps and 30 minutes; invalid learning rate")
    # Fail closed if JAX was already initialized on another backend.
    os.environ["JAX_PLATFORMS"] = "cpu"
    import jax
    import jax.numpy as jnp
    import numpy as np
    import optax
    from needle.model.architecture import SimpleAttentionNetwork
    from needle.model.checkpoints import write_adapter
    from needle.model.finetune import init_lora, lora_target_paths, merge_lora, rung
    from needle.model.quantize import configure_deploy, cq_ste_params
    from needle.model.run import load_checkpoint
    from . import checkpoint

    if jax.default_backend() != "cpu":
        raise ValueError("this experimental entry point permits CPU only")
    directory = output_path(directory)
    manifest, _ = load_pack(directory)
    verify()
    rows = list(read_jsonl(directory / "train-tokens.jsonl"))
    jax.config.update("jax_compilation_cache_dir", str(HOME / ".cache" / "jax-pilot"))
    base = MODEL_DIR / "needle" / "checkpoints" / "needle3.safetensors"
    params, cfg = load_checkpoint(str(base))
    params, cfg = rung(params, cfg, 4)
    cfg.dtype = "float32"
    params = jax.tree.map(lambda value: jnp.asarray(value, jnp.float32), params)
    model = SimpleAttentionNetwork(cfg)
    configure_deploy(act_bits=8, kv_bits=8)
    adapter = init_lora(params, lora_target_paths(params), 8, jax.random.PRNGKey(SEED))
    schedule = optax.warmup_cosine_decay_schedule(0, lr, max(1, int(steps * .05)), max(2, steps))
    optimizer = optax.chain(optax.clip_by_global_norm(1), optax.adamw(schedule))
    opt_state = optimizer.init(adapter)
    identity = {"pack_sha256": file_digest(directory / "manifest.json"), "base_sha256": file_digest(base),
                "layers": 4, "rank": 8, "alpha": 16, "steps": steps, "lr": lr,
                "seed": SEED, "batch": 1, "maximum_seconds": max_seconds,
                "uv_lock_sha256": file_digest(HOME / "uv.lock"), "python": platform.python_version()}
    state = {"step": 0, "position": 0, "order": [], "spent_seconds": 0., "losses": []}
    rng = np.random.default_rng(SEED)
    if (directory / "checkpoints" / "latest.json").exists():
        (adapter, opt_state), state = checkpoint.restore(directory / "checkpoints", (adapter, opt_state), identity)
        adapter, opt_state = jax.tree.map(jnp.asarray, (adapter, opt_state))
        rng.bit_generator.state = state["rng"]

    @jax.jit
    def update(lora, opt, ids, mask):
        def objective(current):
            merged = cq_ste_params(merge_lora(params, current, 2.), 4)
            logits = model.apply({"params": merged}, ids, quant=True)
            losses = optax.softmax_cross_entropy_with_integer_labels(logits[:, :-1], ids[:, 1:])
            return (losses * mask[:, 1:]).sum() / mask[:, 1:].sum()
        loss, grads = jax.value_and_grad(objective)(lora)
        changes, opt = optimizer.update(grads, opt, lora)
        return optax.apply_updates(lora, changes), opt, loss, optax.global_norm(grads)

    started = time.monotonic()
    prior_seconds = state["spent_seconds"]
    checkpoints = read_json(directory / "adapters.json") if (directory / "adapters.json").exists() else {}

    def save():
        state["rng"] = rng.bit_generator.state
        state["spent_seconds"] = prior_seconds + time.monotonic() - started
        location = checkpoint.save(directory / "checkpoints", (adapter, opt_state), state, identity)
        path = location / "adapter.safetensors"
        write_adapter(str(path), {"lora": {"/".join(p): {k: np.asarray(v) for k, v in a.items()} for p, a in adapter.items()},
                                 "scale": 2., "base": str(base), "rank": 8, "seed": SEED})
        checkpoints[str(state["step"])] = {"path": str(path), "sha256": file_digest(path)}
        write_json(directory / "adapters.json", checkpoints)
        write_json(directory / "training-status.json", {"identity": identity, "step": state["step"],
                   "spent_seconds": state["spent_seconds"], "trainable_parameters": sum(x.size for x in jax.tree.leaves(adapter)),
                   "losses": state["losses"], "devices": [str(d) for d in jax.devices()]})

    save()
    last_save, observed_seconds = time.monotonic(), 30.
    try:
        while state["step"] < steps:
            remaining = max_seconds - prior_seconds - (time.monotonic() - started)
            if remaining < max(30., observed_seconds * 2):
                break
            if state["position"] == len(state["order"]):
                state["order"], state["position"] = rng.permutation(len(rows)).tolist(), 0
            row = rows[state["order"][state["position"]]]
            size = manifest["bucket"]
            if len(row["ids"]) > size or len(row["ids"]) != len(row["mask"]) or not any(row["mask"][1:]):
                raise ValueError("invalid compiled sequence; refusing truncation or empty loss")
            ids = jnp.asarray(np.pad(np.asarray(row["ids"], np.int32), (0, size - len(row["ids"])))[None])
            mask = jnp.asarray(np.pad(np.asarray(row["mask"], np.float32), (0, size - len(row["mask"])))[None])
            begun = time.monotonic()
            next_adapter, next_opt, loss, norm = update(adapter, opt_state, ids, mask)
            loss, norm = float(loss), float(norm)
            if not math.isfinite(loss) or not math.isfinite(norm):
                raise ValueError("nonfinite update; retaining last valid optimizer state")
            adapter, opt_state = next_adapter, next_opt
            observed_seconds = time.monotonic() - begun
            state["step"] += 1
            state["position"] += 1
            state["losses"].append({"step": state["step"], "loss": loss, "seconds": observed_seconds})
            if state["step"] % 10 == 0 or state["step"] == 1:
                print(canonical(state["losses"][-1]), flush=True)
            if state["step"] % 80 == 0 or time.monotonic() - last_save > 240:
                save()
                last_save = time.monotonic()
    finally:
        save()
    print(canonical({"finished_step": state["step"], "seconds": state["spent_seconds"]}), flush=True)


def main():
    os.environ.update({"DO_NOT_TRACK": "1", "HF_HUB_OFFLINE": "1", "TRANSFORMERS_OFFLINE": "1"})
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=["prepare", "train"])
    parser.add_argument("--out", required=True)
    parser.add_argument("--search-binary", default=str(HOME / ".cache" / ("catalog-search.exe" if os.name == "nt" else "catalog-search")))
    parser.add_argument("--steps", type=int, default=240)
    parser.add_argument("--max-seconds", type=int, default=900)
    parser.add_argument("--lr", type=float, default=3e-4)
    args = parser.parse_args()
    prepare(args.out, args.search_binary) if args.phase == "prepare" else train(args.out, args.steps, args.max_seconds, args.lr)


if __name__ == "__main__":
    main()
