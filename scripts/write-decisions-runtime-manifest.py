#!/usr/bin/env python3
"""Write the strict local bundle manifest after a native runtime build."""
import argparse
import hashlib
import json
from pathlib import Path

REVISION = "263ac27c59688a360d2db255c4f7e57f665bcca4"
REPOSITORY = "jersobh/jeff-2b-gguf"
LLAMA_COMMIT = "d81235049384534c167caea52b85a694f6103d14"
LLAMA_PATCH_SHA256 = "a3e895bce9fafa23873a481ae29f31fdb98287e0f02939bdd1213c103271a790"
MODELS = [
    ("model", "jeff-2B.Q4_K_M.gguf", "d4d9fbcf40345a1677dc92596c7c1e157fac5cba98e2a1b14ec92db94c67d46b", 1274396448),
    ("projector", "jeff-2B.mmproj.gguf", "1601a51a07942318aacb7a7298847fe505f8beb592be37f538ab85577772bb68", 671372608),
]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--target", required=True)
    parser.add_argument("--backend", choices=("cpu", "cuda", "vulkan", "metal"), required=True)
    parser.add_argument("--release", action="store_true", help="Inventory a complete flat redistributable package")
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    digest = hashlib.sha256()
    with binary.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    manifest = {
        "schema": "aurago.decisions.runtime-manifest.v1",
        "protocol": "aurago.decisions.v1",
        "build": {
            "target": args.target,
            "backend": args.backend,
            "llama_cpp_commit": LLAMA_COMMIT,
            "llama_cpp_patch_sha256": LLAMA_PATCH_SHA256,
            "linkage": "static-msvc-crt" if args.target.startswith("windows-") else "system-c-runtime",
            "required_system_components": {
                "cpu": [] if args.target.startswith("windows-") else ["OS C/C++ runtime"],
                "cuda": (["NVIDIA display/compute driver", "CUDA cuBLAS DLLs (dynamic on Windows)"]
                         if args.target.startswith("windows-") else ["NVIDIA display/compute driver"]),
                "vulkan": ["system Vulkan loader and compatible device driver"],
                "metal": ["Apple Metal framework and compatible GPU"],
            }[args.backend],
        },
        "artifact": {"file": binary.name, "sha256": digest.hexdigest(), "size_bytes": binary.stat().st_size},
        "models": [
            {"role": role, "repo": REPOSITORY, "revision": REVISION, "file": filename, "sha256": sha, "size_bytes": size}
            for role, filename, sha, size in MODELS
        ],
    }
    output = binary.parent / "release-manifest.json"
    if args.release:
        manifest["schema"] = "aurago.decisions.runtime-manifest.v2"
        manifest["files"] = []
        for member in sorted(binary.parent.iterdir()):
            if member == output:
                continue
            if member.is_symlink() or not member.is_file():
                raise ValueError(f"Release members must be regular files: {member.name}")
            with member.open("rb") as stream:
                sha256 = hashlib.file_digest(stream, "sha256").hexdigest()
            manifest["files"].append({"file": member.name, "size_bytes": member.stat().st_size, "sha256": sha256})
        manifest["build"]["required_system_components"] = {
            "cpu": ["compatible operating system"], "cuda": ["compatible NVIDIA driver"],
            "vulkan": ["compatible Vulkan device driver"], "metal": ["compatible macOS Metal device"]
        }[args.backend]
        manifest["build"]["required_system_components"].insert(0, {
            "windows": "Windows 10/11 or Server 2022+", "linux": "glibc 2.39+ (Ubuntu 24.04 or compatible)",
            "darwin": "macOS 14+"
        }[args.target.split('-')[0]])
        if args.target == "windows-arm64":
            manifest["build"]["required_system_components"][0] = "Windows 11 arm64"
    temporary = output.with_suffix(".json.tmp")
    temporary.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8", newline="\n")
    temporary.replace(output)
    print(output)


if __name__ == "__main__":
    main()
