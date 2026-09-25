"""Matched W4 exports from pinned local weights; no automatic upstream fetch."""
from __future__ import annotations

import argparse
from pathlib import Path

from .assets import MODEL_DIR, PINS
from .common import file_digest, write_json


def export(output, adapter=None, layers=20):
    import jax.numpy as jnp
    from needle.model.architecture import ConfidenceHead, effective_kv_window
    from needle.model.checkpoints import read_adapter
    from needle.model.export import read_tokenizer_blob, write_export
    from needle.model.finetune import rung, merge_lora
    from needle.model.run import load_checkpoint
    base = MODEL_DIR / "needle" / "checkpoints" / "needle3.safetensors"
    archive = MODEL_DIR / "needle" / "needle3.cact"
    if not base.is_file() or not archive.is_file():
        raise FileNotFoundError("download and verify pinned assets before exporting")
    params, cfg = load_checkpoint(str(base))
    params, cfg = rung(params, cfg, layers)
    if adapter:
        saved = read_adapter(adapter)
        lora = {tuple(k.split("/")): {k2: jnp.asarray(v2) for k2, v2 in v.items()} for k, v in saved["lora"].items()}
        params = merge_lora(params, lora, saved["scale"])
    # Drop the confidence head for BOTH the untrained and trained comparison.
    params = {k: v for k, v in params.items() if k != ConfidenceHead.key}
    Path(output).parent.mkdir(parents=True, exist_ok=True)
    info = write_export(params, cfg, str(output), bits=4, tokenizer=read_tokenizer_blob(str(archive)), kv_window=effective_kv_window(cfg))
    manifest = {"pins": PINS, "layers": layers, "weight_bits": 4, "confidence": None,
                "base_sha256": file_digest(base), "tokenizer_archive_sha256": file_digest(archive),
                "adapter_sha256": file_digest(adapter) if adapter else None,
                "output_sha256": file_digest(output), "export": info}
    write_json(str(output) + ".json", manifest)
    return manifest


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument("--adapter")
    parser.add_argument("--layers", type=int, choices=range(2, 21), default=20,
                        help="Export a trained ladder depth from the pinned base model")
    args = parser.parse_args()
    export(args.out, args.adapter, layers=args.layers)
