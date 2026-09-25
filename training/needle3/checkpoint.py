"""Atomic, non-pickle checkpoints bound to model, data, optimizer and RNG state."""
from __future__ import annotations

import os
from pathlib import Path
import uuid

from .common import canonical, digest, file_digest, read_json, verify_files, write_json


def save(directory, tree, state, identity):
    import jax
    import numpy as np
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    name = f"step-{state['step']:08d}-{uuid.uuid4().hex[:8]}"
    temporary = directory / ("." + name)
    temporary.mkdir()
    leaves, definition = jax.tree.flatten(tree)
    arrays = {f"a{i}": np.asarray(value) for i, value in enumerate(leaves)}
    with (temporary / "state.npz").open("wb") as f:
        np.savez(f, **arrays)
        f.flush()
        os.fsync(f.fileno())
    manifest = {"identity": identity, "state": state, "tree_sha256": digest(str(definition)),
                "arrays": [{"shape": list(v.shape), "dtype": str(v.dtype)} for v in arrays.values()],
                "files": {"state.npz": file_digest(temporary / "state.npz")}}
    write_json(temporary / "manifest.json", manifest)
    final = directory / name
    os.replace(temporary, final)
    write_json(directory / "latest.json", {"directory": name, "manifest_sha256": file_digest(final / "manifest.json")})
    return final


def restore(directory, template, identity):
    import jax
    import numpy as np
    directory = Path(directory)
    pointer = read_json(directory / "latest.json")
    source = (directory / pointer["directory"]).resolve()
    if source.parent != directory.resolve() or file_digest(source / "manifest.json") != pointer["manifest_sha256"]:
        raise ValueError("checkpoint pointer is invalid")
    manifest = read_json(source / "manifest.json")
    leaves, definition = jax.tree.flatten(template)
    if manifest["identity"] != identity or manifest["tree_sha256"] != digest(str(definition)):
        raise ValueError("checkpoint belongs to a different model, data or optimizer")
    verify_files(source, manifest["files"])
    with np.load(source / "state.npz", allow_pickle=False) as archive:
        arrays = [archive[f"a{i}"] for i in range(len(leaves))]
    if len(arrays) != len(manifest["arrays"]):
        raise ValueError("checkpoint leaf count differs")
    for expected, value in zip(leaves, arrays):
        if expected.shape != value.shape or expected.dtype != value.dtype:
            raise ValueError("checkpoint leaf type/shape differs")
    return jax.tree.unflatten(definition, arrays), manifest["state"]
