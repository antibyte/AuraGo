"""Bounded CPU category LoRA with balanced sampling and gradient accumulation."""
from __future__ import annotations

import argparse
from collections import Counter
import math
import os
import platform
import time

from .assets import HOME, MODEL_DIR, verify
from .category_data import load
from .common import canonical, file_digest, read_json, read_jsonl, write_json
from .local_pilot import output_path


def sampling_weights(rows):
    """Limit mechanical probes and balance categories within each natural kind."""
    masses = {"catalog": .15, "single": .30, "multi": .20, "context": .15, "none": .20}
    weights = [0.] * len(rows)
    for kind, mass in masses.items():
        indices = [i for i, row in enumerate(rows) if row["kind"] == kind]
        if not indices:
            raise ValueError("balanced sampler requires every training kind")
        counts = Counter(c for i in indices for c in rows[i]["categories"])
        raw = [sum(1 / counts[c] for c in rows[i]["categories"]) / max(1, len(rows[i]["categories"]))
               if rows[i]["categories"] else 1. for i in indices]
        for i, weight in zip(indices, raw):
            weights[i] = mass * weight / sum(raw)
    return weights


def train(directory, steps=300, max_seconds=1800, lr=1e-4, batch=8):
    if not 1 <= steps <= 1000 or not 60 <= max_seconds <= 1800 or not 0 < lr <= 1e-4 or batch not in (4, 8, 16):
        raise ValueError("CPU category experiment bounds: <=1000 updates, <=30 min, lr<=1e-4, batch 4/8/16")
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
    from .export import export
    from .category_eval import child_measure

    if jax.default_backend() != "cpu":
        raise ValueError("CPU backend required")
    directory = output_path(directory)
    manifest, _ = load(directory)
    verify()
    rows = list(read_jsonl(directory / "train-tokens.jsonl"))
    probabilities = np.asarray(sampling_weights(rows))
    jax.config.update("jax_compilation_cache_dir", str(HOME / ".cache" / "jax-category"))
    base = MODEL_DIR / "needle" / "checkpoints" / "needle3.safetensors"
    params, cfg = load_checkpoint(str(base))
    params, cfg = rung(params, cfg, 4)
    cfg.dtype = "float32"
    params = jax.tree.map(lambda x: jnp.asarray(x, jnp.float32), params)
    model = SimpleAttentionNetwork(cfg)
    configure_deploy(act_bits=8, kv_bits=8)
    seed = manifest["seed"]
    adapter = init_lora(params, lora_target_paths(params), 16, jax.random.PRNGKey(seed))
    schedule = optax.warmup_cosine_decay_schedule(0, lr, max(1, int(steps * .05)), max(2, steps))
    optimizer = optax.chain(optax.clip_by_global_norm(1.), optax.adamw(schedule))
    opt_state = optimizer.init(adapter)
    identity = {"manifest_sha256": file_digest(directory / "manifest.json"), "base_sha256": file_digest(base),
                "trainer_sha256": file_digest(__file__), "layers": 4, "rank": 16, "alpha": 32,
                "steps": steps, "lr": lr, "batch": batch, "seed": seed, "max_seconds": max_seconds,
                "uv_lock_sha256": file_digest(HOME / "uv.lock"), "python": platform.python_version()}
    state = {"step": 0, "examples": 0, "spent_seconds": 0., "losses": []}
    rng = np.random.default_rng(seed)
    if (directory / "checkpoints" / "latest.json").exists():
        (adapter, opt_state), state = checkpoint.restore(directory / "checkpoints", (adapter, opt_state), identity)
        adapter, opt_state = jax.tree.map(jnp.asarray, (adapter, opt_state))
        rng.bit_generator.state = state["rng"]

    @jax.jit
    def gradient(current, ids, mask):
        def objective(lora):
            merged = cq_ste_params(merge_lora(params, lora, 2.), 4)
            logits = model.apply({"params": merged}, ids, quant=True)
            losses = optax.softmax_cross_entropy_with_integer_labels(logits[:, :-1], ids[:, 1:])
            return (losses * mask[:, 1:]).sum() / mask[:, 1:].sum()
        return jax.value_and_grad(objective)(current)

    @jax.jit
    def accumulate(total, delta):
        return jax.tree.map(lambda a, b: a + b / batch, total, delta)

    @jax.jit
    def update(lora, opt, grads):
        changes, opt = optimizer.update(grads, opt, lora)
        return optax.apply_updates(lora, changes), opt, optax.global_norm(grads)

    started, prior = time.monotonic(), state["spent_seconds"]
    adapters = read_json(directory / "adapters.json") if (directory / "adapters.json").exists() else {}

    def save():
        state["rng"] = rng.bit_generator.state
        state["spent_seconds"] = prior + time.monotonic() - started
        location = checkpoint.save(directory / "checkpoints", (adapter, opt_state), state, identity)
        path = location / "adapter.safetensors"
        write_adapter(str(path), {"lora": {"/".join(p): {k: np.asarray(v) for k, v in a.items()} for p, a in adapter.items()},
                                 "scale": 2., "base": str(base), "rank": 16, "seed": seed})
        adapters[str(state["step"])] = {"path": str(path), "sha256": file_digest(path)}
        write_json(directory / "adapters.json", adapters)
        write_json(directory / "training-status.json", {"identity": identity, **state,
                   "trainable_parameters": sum(x.size for x in jax.tree.leaves(adapter)), "devices": [str(d) for d in jax.devices()]})
        return path

    last_save, observed = time.monotonic(), 45.
    save()
    try:
        while state["step"] < steps:
            if max_seconds - prior - (time.monotonic() - started) < max(60., observed * 2):
                break
            begun = time.monotonic()
            total = jax.tree.map(jnp.zeros_like, adapter)
            losses = []
            for index in rng.choice(len(rows), size=batch, p=probabilities):
                row = rows[index]
                padding = manifest["bucket"] - len(row["ids"])
                if padding < 0 or len(row["ids"]) != len(row["mask"]) or not any(row["mask"]):
                    raise ValueError("invalid compiled row; refusing truncation")
                ids = jnp.asarray(np.pad(np.asarray(row["ids"], np.int32), (0, padding)))[None]
                mask = jnp.asarray(np.pad(np.asarray(row["mask"], np.float32), (0, padding)))[None]
                loss, grads = gradient(adapter, ids, mask)
                losses.append(float(loss))
                total = accumulate(total, grads)
            next_adapter, next_opt, norm = update(adapter, opt_state, total)
            loss, norm = float(np.mean(losses)), float(norm)
            if not math.isfinite(loss) or not math.isfinite(norm):
                raise ValueError("nonfinite update; retaining the last valid checkpoint")
            adapter, opt_state = next_adapter, next_opt
            state["step"] += 1
            state["examples"] += batch
            observed = time.monotonic() - begun
            state["losses"].append({"step": state["step"], "loss": loss, "gradient_norm": norm, "seconds": observed})
            if state["step"] == 1 or state["step"] % 5 == 0:
                print(canonical(state["losses"][-1]), flush=True)
            if time.monotonic() - last_save >= 240 or state["step"] % 60 == 0:
                save()
                last_save = time.monotonic()
            if state["step"] % 60 == 0 and max_seconds - prior - (time.monotonic() - started) > 90:
                path = directory / "models" / f"step-{state['step']:04d}.cact"
                export(path, adapters[str(state["step"])]["path"], layers=4)
                child_measure(directory, path, "validation", f"validation-step-{state['step']:04d}.json")
    finally:
        saved = save()
    path = directory / "models" / f"step-{state['step']:04d}.cact"
    if not path.exists():
        export(path, str(saved), layers=4)
    result = directory / f"validation-step-{state['step']:04d}.json"
    if not result.exists():
        child_measure(directory, path, "validation", result.name)
    print(canonical({"finished_step": state["step"], "examples": state["examples"], "training_seconds": state["spent_seconds"]}), flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument("--steps", type=int, default=300)
    parser.add_argument("--max-seconds", type=int, default=1800)
    parser.add_argument("--lr", type=float, default=1e-4)
    parser.add_argument("--batch", type=int, choices=[4, 8, 16], default=8)
    args = parser.parse_args()
    train(args.out, args.steps, args.max_seconds, args.lr, args.batch)
