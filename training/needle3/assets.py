"""Public, revision-pinned downloads; no credentials or GPU resources required."""
from __future__ import annotations

import argparse
import os
from pathlib import Path
import zipfile

from .common import HOME, canonical, file_digest, read_json, write_json, verify_files

PINS = {
    "needle": {"repo": "Cactus-Compute/needle3", "revision": "b274efcb211a9eef48c9a88da4b43bd569696a39"},
    "retriever": {"repo": "intfloat/multilingual-e5-small", "revision": "614241f622f53c4eeff9890bdc4f31cfecc418b3"},
    "runtime_version": "3.0.1",
}
MODEL_DIR = HOME / "models"


def fetch(include_retriever=True):
    os.environ["HF_HUB_DISABLE_TELEMETRY"] = "1"
    from huggingface_hub import hf_hub_download, snapshot_download
    MODEL_DIR.mkdir(exist_ok=True)
    needle = PINS["needle"]
    files = ["needle3.cact", "checkpoints/needle3.safetensors", "tokenizer/tokenizer.model", "tokenizer/tokenizer.vocab"]
    for tag in ("win_amd64", "manylinux2014_x86_64"):
        files.append(f"python/cactus_needle-{PINS['runtime_version']}-py3-none-{tag}.whl")
    for name in files:
        path = hf_hub_download(repo_id=needle["repo"], revision=needle["revision"], filename=name, local_dir=MODEL_DIR / "needle", token=False)
        if name.endswith(".whl"):
            platform = "windows" if "win_amd64" in name else "linux"
            suffix = "dll" if platform == "windows" else "so"
            out = MODEL_DIR / platform / f"libneedle3.{suffix}"
            out.parent.mkdir(exist_ok=True)
            with zipfile.ZipFile(path) as archive:
                out.write_bytes(archive.read(f"needle/libneedle3.{suffix}"))
    if include_retriever:
        pin = PINS["retriever"]
        snapshot_download(repo_id=pin["repo"], revision=pin["revision"], local_dir=MODEL_DIR / "retriever", token=False,
                          allow_patterns=["*.json", "*.model", "model.safetensors", "1_Pooling/config.json"],
                          ignore_patterns=["onnx/*", "openvino/*", ".eval_results/*"])
    hashes = {p.relative_to(MODEL_DIR).as_posix(): file_digest(p) for p in MODEL_DIR.rglob("*") if p.is_file() and ".cache" not in p.parts and p.name != "manifest.json"}
    manifest = {"pins": PINS, "files": hashes}
    write_json(MODEL_DIR / "manifest.json", manifest)
    print(canonical({"downloaded_files": len(hashes), "pins": PINS}))
    return manifest


def verify():
    manifest = read_json(MODEL_DIR / "manifest.json")
    if manifest["pins"] != PINS:
        raise ValueError("model revisions differ from the preparation lock")
    verify_files(MODEL_DIR, manifest["files"])
    return manifest


def tokenizer():
    from needle.model.tokenizer import SANTokenizer
    return SANTokenizer(str(MODEL_DIR / "needle" / "tokenizer" / "tokenizer.model"))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--verify", action="store_true")
    parser.add_argument("--needle-only", action="store_true")
    args = parser.parse_args()
    verify() if args.verify else fetch(not args.needle_only)
