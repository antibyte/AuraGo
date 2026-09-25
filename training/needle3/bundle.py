"""Allowlisted transfer archives and checksum verification; never copy secrets."""
from __future__ import annotations

import argparse
from pathlib import Path
import tarfile

from .assets import MODEL_DIR, verify as verify_assets
from .common import ROOT, HOME, file_digest, read_json, verify_files, write_json


def bundle(output, pack):
    pack = Path(pack).resolve()
    manifest = read_json(pack / "manifest.json")
    if not manifest.get("ready_for_gpu"):
        raise ValueError("incomplete data may not be bundled as a GPU training run")
    verify_files(pack, manifest["files"])
    assets = verify_assets()
    files = {p for pattern in ("*.py", "*.json", "*.toml", "*.lock", "*.sh", "*.md") for p in HOME.glob(pattern)}
    files.update(pack / p for p in manifest["files"])
    files.add(pack / "manifest.json")
    for name in assets["files"]:
        files.add(MODEL_DIR / name)
    files.add(MODEL_DIR / "manifest.json")
    for path in files:
        if not path.resolve().is_relative_to(HOME.resolve()) or path.suffix in {".sqlite", ".log"}:
            raise ValueError("file outside the transfer allowlist")
    checksums = {p.relative_to(ROOT).as_posix(): file_digest(p) for p in files}
    # An uncompressed tar avoids spending setup time recompressing model weights.
    with tarfile.open(output, "w") as archive:
        for path in sorted(files):
            archive.add(path, arcname=path.relative_to(ROOT).as_posix(), recursive=False)
    write_json(str(output) + ".manifest.json", {"archive_sha256": file_digest(output), "files": checksums,
                                               "network_volume_deletion_allowed": False})


def verify_download(directory, manifest_path):
    manifest = read_json(manifest_path)
    verify_files(directory, manifest["files"])
    receipt = {"verified_manifest_sha256": file_digest(manifest_path), "files": len(manifest["files"]),
               "local_download_verified": True, "volume_deletion_performed": False}
    write_json(Path(directory) / "download-verified.json", receipt)
    return receipt


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out")
    parser.add_argument("--pack", default=str(HOME / "generated" / "compiled"))
    parser.add_argument("--verify-download")
    parser.add_argument("--manifest")
    args = parser.parse_args()
    if args.verify_download:
        verify_download(args.verify_download, args.manifest)
    elif args.out:
        bundle(args.out, args.pack)
    else:
        parser.error("provide --out or --verify-download with --manifest")
