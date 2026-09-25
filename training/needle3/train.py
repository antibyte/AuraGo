"""Fixed-split Float32 QAT LoRA with accumulation, checkpoints and a hard lease."""
from __future__ import annotations

import argparse
import math
import os
from pathlib import Path
import signal
import time

from .assets import MODEL_DIR
from .common import HOME, canonical, config, digest, file_digest, read_json, read_jsonl, verify_files, write_json
from . import checkpoint


def epoch_batches(rows, batch, rng):
    buckets = {}
    for i, row in enumerate(rows):
        buckets.setdefault(row["bucket"], []).append(i)
    batches = []
    for indices in buckets.values():
        order = rng.permutation(indices).tolist()
        batches.extend(order[i:i + batch] for i in range(0, len(order), batch))
    rng.shuffle(batches)
    return batches


def padded(rows, indices, microbatch, bucket):
    import numpy as np
    ids = np.zeros((microbatch, bucket), dtype=np.int32)
    masks = np.zeros((microbatch, bucket), dtype=np.float32)
    for i, index in enumerate(indices):
        row = rows[index]
        if len(row["tokens"]) > bucket or len(row["tokens"]) != len(row["mask"]):
            raise ValueError("invalid compiled sequence; refusing truncation")
        ids[i, :len(row["tokens"])] = row["tokens"]
        masks[i, :len(row["mask"])] = row["mask"]
    return ids, masks


def train(args):
    os.environ.setdefault("XLA_PYTHON_CLIENT_PREALLOCATE", "false")
    os.environ.setdefault("DO_NOT_TRACK", "1")
    import jax
    import jax.numpy as jnp
    import numpy as np
    import optax
    from needle.model.architecture import SimpleAttentionNetwork
    from needle.model.checkpoints import write_adapter
    from needle.model.finetune import rung, init_lora, lora_target_paths, merge_lora
    from needle.model.quantize import configure_deploy, cq_ste_params
    from needle.model.run import load_checkpoint
    cfg = config()
    settings = cfg["train"]
    pack = Path(args.pack)
    manifest = read_json(pack / "manifest.json")
    if not manifest.get("ready_for_gpu") or manifest["config_sha256"] != file_digest(HOME / "config.json") or manifest["dependency_lock_sha256"] != file_digest(HOME / "uv.lock"):
        raise ValueError("training pack is unready or configuration drifted")
    verify_files(pack, manifest["files"])
    lease = read_json(args.lease)
    started_at = float(lease["created_at_unix"])
    deadline = min(started_at + cfg["runpod"]["training_deadline_seconds"], time.time() + args.max_seconds)
    if not lease.get("watchdog_armed") or time.time() - lease.get("watchdog_heartbeat_unix", 0) > 60 or time.time() >= deadline:
        raise ValueError("valid independent shutdown watchdog and unexpired lease are required")
    if jax.default_backend() != "gpu":
        raise ValueError("the real training controller requires a verified CUDA GPU")
    rows = list(read_jsonl(pack / (args.arm + ".jsonl")))
    validation = list(read_jsonl(pack / "validation.jsonl"))
    if not rows or not validation or set(r["group_id"] for r in rows) & set(r["group_id"] for r in validation):
        raise ValueError("missing data or train/validation group leakage")
    base = MODEL_DIR / "needle" / "checkpoints" / "needle3.safetensors"
    params, model_config = load_checkpoint(str(base))
    params, model_config = rung(params, model_config, settings["layers"])
    model_config.dtype = "float32"
    params = jax.device_put(jax.tree.map(lambda a: np.asarray(a, dtype=np.float32), params))
    model = SimpleAttentionNetwork(model_config)
    configure_deploy(act_bits=8, kv_bits=8)
    rng = np.random.default_rng(cfg["seed"])
    lora = init_lora(params, lora_target_paths(params), settings["rank"], jax.random.PRNGKey(cfg["seed"]))
    scale = settings["alpha"] / settings["rank"]
    steps_per_epoch = len(epoch_batches(rows, settings["effective_batch"], np.random.default_rng(0)))
    maximum = min(args.max_steps, steps_per_epoch * settings["epochs"])
    if maximum < 2:
        raise ValueError("at least two training steps required")
    schedule = optax.warmup_cosine_decay_schedule(0.0, settings["lr"], min(maximum - 1, max(1, int(maximum * settings["warmup_fraction"]))), maximum)
    optimizer = optax.chain(optax.clip_by_global_norm(settings["clip_norm"]), optax.adamw(schedule))
    opt_state = optimizer.init(lora)
    out = Path(args.out)
    identity = {"pack_sha256": file_digest(pack / "manifest.json"), "base_sha256": file_digest(base),
                "arm": args.arm, "maximum_steps": maximum, "settings": settings}
    state = {"step": 0, "epoch": 0, "position": 0, "batches": [], "rng": rng.bit_generator.state, "seconds_per_step": []}
    if (out / "checkpoints" / "latest.json").exists():
        (lora, opt_state), state = checkpoint.restore(out / "checkpoints", (lora, opt_state), identity)
        rng.bit_generator.state = state["rng"]
    stop = []
    signal.signal(signal.SIGTERM, lambda *_: stop.append(True))
    signal.signal(signal.SIGINT, lambda *_: stop.append(True))

    def loss_sum(adapter, ids, mask):
        merged = cq_ste_params(merge_lora(params, adapter, scale), 4)
        logits = model.apply({"params": merged}, ids, quant=True)
        loss = optax.softmax_cross_entropy_with_integer_labels(logits[:, :-1], ids[:, 1:])
        return (loss * mask[:, 1:]).sum(), mask[:, 1:].sum()

    gradient = jax.jit(jax.value_and_grad(loss_sum, has_aux=True))
    validate = jax.jit(loss_sum)

    @jax.jit
    def update(adapter, opt, grads):
        updates, opt = optimizer.update(grads, opt, adapter)
        return optax.apply_updates(adapter, updates), opt

    def save_state():
        state["rng"] = rng.bit_generator.state
        location = checkpoint.save(out / "checkpoints", (lora, opt_state), state, identity)
        adapter_path = location / "adapter.safetensors"
        write_adapter(str(adapter_path), {"lora": {"/".join(p): {k: np.asarray(v) for k, v in a.items()} for p, a in lora.items()}, "scale": scale, "base": str(base), "rank": settings["rank"], "seed": cfg["seed"]})
        write_json(out / "latest_adapter.json", {"path": str(adapter_path.resolve()), "sha256": file_digest(adapter_path), "step": state["step"]})
        return location

    save_state()
    last_save = time.monotonic()
    try:
        while state["step"] < maximum and state["epoch"] < settings["epochs"] and not stop:
            observed = state["seconds_per_step"][-10:]
            reserve = max([120.0] + observed) * 2
            if time.time() + reserve >= deadline:
                break
            if not state["batches"]:
                state["batches"] = epoch_batches(rows, settings["effective_batch"], rng)
                state["position"] = 0
            batch = state["batches"][state["position"]]
            bucket = rows[batch[0]]["bucket"]
            begun = time.monotonic()
            grads = None
            total_loss = total_tokens = 0.0
            for offset in range(0, len(batch), args.microbatch):
                ids, mask = padded(rows, batch[offset:offset + args.microbatch], args.microbatch, bucket)
                (loss, count), partial = gradient(lora, jnp.asarray(ids), jnp.asarray(mask))
                total_loss += float(loss)
                total_tokens += float(count)
                grads = partial if grads is None else jax.tree.map(lambda a, b: a + b, grads, partial)
            if total_tokens <= 0 or not math.isfinite(total_loss):
                raise ValueError("empty or non-finite training loss")
            grads = jax.tree.map(lambda g: g / total_tokens, grads)
            if not bool(jnp.all(jnp.stack([jnp.all(jnp.isfinite(g)) for g in jax.tree.leaves(grads)]))):
                raise ValueError("non-finite gradients; preserving last valid optimizer state")
            lora, opt_state = update(lora, opt_state, grads)
            jax.block_until_ready(lora)
            elapsed = time.monotonic() - begun
            state["step"] += 1
            state["position"] += 1
            state["seconds_per_step"] = (state["seconds_per_step"] + [elapsed])[-20:]
            if state["position"] == len(state["batches"]):
                state["epoch"] += 1
                state["batches"] = []
            print(canonical({"step": state["step"], "loss": total_loss / total_tokens, "seconds": elapsed}), flush=True)
            # Checkpoint well before ten minutes; a slow step triggers immediate save.
            if time.monotonic() - last_save >= min(300, max(1, 600 - elapsed * 2)):
                save_state()
                last_save = time.monotonic()
            if state["step"] % settings["validation_steps"] == 0 and time.time() + 900 < deadline:
                loss = count = 0.0
                for i, row in enumerate(validation):
                    a, b = padded(validation, [i], 1, row["bucket"])
                    x, n = validate(lora, jnp.asarray(a), jnp.asarray(b))
                    loss, count = loss + float(x), count + float(n)
                    if time.monotonic() - last_save > 300:
                        save_state()
                        last_save = time.monotonic()
                    if time.time() + 120 >= deadline:
                        break
                write_json(out / f"validation-loss-{state['step']:08d}.json", {"loss": loss / max(count, 1), "rows_scored": i + 1, "selection_eligible": False})
                save_state()
                last_save = time.monotonic()
    finally:
        save_state()
    write_json(out / "training-summary.json", {"steps": state["step"], "maximum_steps": maximum, "deadline_unix": deadline, "identity": identity, "seconds_per_step": state["seconds_per_step"], "requires_validation_selection": True})


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pack", required=True)
    parser.add_argument("--lease", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--arm", choices=["train", "pilot_de_en", "pilot_multilingual"], default="train")
    parser.add_argument("--microbatch", type=int, choices=[1, 2, 4, 8, 16], default=1)
    parser.add_argument("--max-steps", type=int, default=100000)
    parser.add_argument("--max-seconds", type=int, default=25200)
    train(parser.parse_args())
