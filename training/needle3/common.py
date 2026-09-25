"""Deterministic files, provenance, and explicit preparation failures."""
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
HOME = Path(__file__).resolve().parent


def read_json(path):
    return json.loads(Path(path).read_text(encoding="utf-8"))


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def digest(value):
    if isinstance(value, str):
        value = value.encode("utf-8")
    return hashlib.sha256(value).hexdigest()


def file_digest(path):
    h = hashlib.sha256()
    with Path(path).open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def atomic_text(path, text):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=path.name + ".", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as f:
            f.write(text)
            f.flush()
            os.fsync(f.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def write_json(path, value):
    atomic_text(path, json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2) + "\n")


def read_jsonl(path):
    with Path(path).open(encoding="utf-8") as f:
        for number, line in enumerate(f, 1):
            if line.strip():
                try:
                    yield json.loads(line)
                except json.JSONDecodeError as exc:
                    raise ValueError(f"invalid JSON at {path}:{number}") from exc


def write_jsonl(path, rows):
    atomic_text(path, "".join(canonical(row) + "\n" for row in rows))


def config():
    return read_json(HOME / "config.json")


def repository_revision():
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()


def catalog(path=None):
    value = read_json(path or HOME / "catalog.json")
    ids = [m["id"] for m in value["manuals"]]
    if len(set(ids)) != len(ids) or value["version"] != 1:
        raise ValueError("invalid manual catalog version or duplicate family")
    for manual in value["manuals"]:
        if digest(manual["body"]) != manual["sha256"]:
            raise ValueError(f"manual digest mismatch: {manual['id']}")
    for tool in value["tools"]:
        if tool.get("manual_id") not in ids and not tool.get("absence_reason"):
            raise ValueError(f"unexplained missing manual: {tool['name']}")
    return value


def verify_files(root, hashes):
    root = Path(root).resolve()
    for relative, expected in hashes.items():
        path = (root / relative).resolve()
        if not path.is_relative_to(root) or not path.is_file() or file_digest(path) != expected:
            raise ValueError(f"artifact missing, changed or outside package: {relative}")


def inference_query(row):
    previous = row.get("context", [])
    if not isinstance(previous, list) or len(previous) > 2 or not all(isinstance(x, str) for x in previous):
        raise ValueError("context must contain at most two preceding human messages")
    if not previous:
        return row["query"]
    return "Previous user requests:\n" + "\n".join(previous) + "\nCurrent user request:\n" + row["query"]
