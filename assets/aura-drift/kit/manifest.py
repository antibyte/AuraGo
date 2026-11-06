"""Aura Drift manifest writer: scans the GLBs in the output folder (MIT, AuraGo contributors).

``write(out_dir)`` never builds anything. It reads every ``*.glb`` it finds, measures nodes and
triangles straight from the glTF JSON and writes ``manifest.json`` and ``LICENSE.txt``. Parallel
authors therefore never edit the manifest; optional per-group metadata lives in ``kit/meta_*.py``:

    meta_cars.py       CARS    = {"kobalt": {"wheelRadius": 0.4}}            (overrides measured values)
    meta_items.py      ITEMS   = {"crate": {"radius": 1.0}}
    meta_track.py      TRACK   = {"ramp": {"radius": 6}}
    meta_animals.py    ANIMALS = {"cow": {"length": 2.2, "height": 1.5}}
    meta_shared.py     PROPS   = {"grandstand": {"tags": ["hero"], "radius": 14, "height": 12}}
    meta_scenery_a.py / meta_scenery_b.py
                       PROPS   = {"alpine": {"pine_a": {"tags": ["scatter"]}}, "desert": {...}}

Tags are ``hero``, ``tall``, ``scatter`` and ``far``; the validator derives the triangle budget from them.
"""

import hashlib
import importlib
import json
import math
import os
import struct

CAR_IDS = ["vektor", "kobalt", "lumen", "tundra", "softeis", "maehkoenig", "couch", "bertha"]
PREFIX = {"items": "item-", "track": "track-", "animals": "animal-"}
SCENERY = ["alpine", "desert", "harbour", "salt", "forest", "industry", "glacier", "metro"]

LICENSE_TEXT = """AuraGo Aura Drift model pack

Original work for AuraGo; every mesh is generated from code by assets/aura-drift/generate.py.
No third-party content (no scans, textures, meshes or fonts) was used.

Copyright (c) 2026 AuraGo contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
"""


# --------------------------------------------------------------------------
# minimal glTF reading (no dependencies)
# --------------------------------------------------------------------------


def read_glb(data):
    length = struct.unpack("<I", data[12:16])[0]
    return json.loads(data[20 : 20 + length].decode("utf8").rstrip("\x00 "))


def identity():
    return [[1.0 if i == j else 0.0 for j in range(4)] for i in range(4)]


def mat_mul(a, b):
    return [[sum(a[i][k] * b[k][j] for k in range(4)) for j in range(4)] for i in range(4)]


def node_matrix(node):
    if "matrix" in node:
        m = node["matrix"]  # column-major
        return [[m[c * 4 + r] for c in range(4)] for r in range(4)]
    tx, ty, tz = node.get("translation", (0, 0, 0))
    x, y, z, w = node.get("rotation", (0, 0, 0, 1))
    sx, sy, sz = node.get("scale", (1, 1, 1))
    r = [
        [1 - 2 * (y * y + z * z), 2 * (x * y - z * w), 2 * (x * z + y * w)],
        [2 * (x * y + z * w), 1 - 2 * (x * x + z * z), 2 * (y * z - x * w)],
        [2 * (x * z - y * w), 2 * (y * z + x * w), 1 - 2 * (x * x + y * y)],
    ]
    return [
        [r[0][0] * sx, r[0][1] * sy, r[0][2] * sz, tx],
        [r[1][0] * sx, r[1][1] * sy, r[1][2] * sz, ty],
        [r[2][0] * sx, r[2][1] * sy, r[2][2] * sz, tz],
        [0.0, 0.0, 0.0, 1.0],
    ]


def apply(m, p):
    return [sum(m[i][k] * (p[k] if k < 3 else 1.0) for k in range(4)) for i in range(3)]


def mesh_triangles(doc, mesh):
    total = 0
    for primitive in mesh["primitives"]:
        accessor = primitive.get("indices", primitive["attributes"]["POSITION"])
        total += doc["accessors"][accessor]["count"] // 3
    return total


def subtree(doc, index, own=False, parent=None):
    """(min, max, triangles) of a node and its children; ``own`` includes the node's own transform."""
    node = doc["nodes"][index]
    matrix = parent if parent is not None else identity()
    if own or parent is not None:
        matrix = mat_mul(matrix, node_matrix(node))
    low, high, tris = [1e9] * 3, [-1e9] * 3, 0
    if "mesh" in node:
        mesh = doc["meshes"][node["mesh"]]
        tris += mesh_triangles(doc, mesh)
        for primitive in mesh["primitives"]:
            acc = doc["accessors"][primitive["attributes"]["POSITION"]]
            for corner in range(8):
                point = [acc["max"][a] if corner >> a & 1 else acc["min"][a] for a in range(3)]
                world = apply(matrix, point)
                for a in range(3):
                    low[a] = min(low[a], world[a])
                    high[a] = max(high[a], world[a])
    for child in node.get("children", []):
        c_low, c_high, c_tris = subtree(doc, child, True, matrix)
        tris += c_tris
        for a in range(3):
            low[a] = min(low[a], c_low[a])
            high[a] = max(high[a], c_high[a])
    return low, high, tris


def roots(doc):
    return doc["scenes"][doc.get("scene", 0)]["nodes"]


def r2(value):
    return round(value + 0.0, 3)


def load_meta(module, attribute):
    try:
        return getattr(importlib.import_module("kit." + module), attribute)
    except (ImportError, AttributeError):
        return {}


# --------------------------------------------------------------------------
# entries
# --------------------------------------------------------------------------


def car_entry(doc, file, overrides):
    by_name = {doc["nodes"][i]["name"]: i for i in roots(doc)}
    low, high = [1e9] * 3, [-1e9] * 3
    for index in by_name.values():
        n_low, n_high, _ = subtree(doc, index, True)
        for a in range(3):
            low[a], high[a] = min(low[a], n_low[a]), max(high[a], n_high[a])
    entry = {"file": file, "length": r2(high[0] - low[0]), "width": r2(high[2] - low[2]), "height": r2(high[1])}
    if "wheel_fl" in by_name:
        w_low, w_high, _ = subtree(doc, by_name["wheel_fl"], False)
        entry["wheelRadius"] = r2((w_high[1] - w_low[1]) / 2.0)
        fl = doc["nodes"][by_name["wheel_fl"]].get("translation", (0, 0, 0))
        rl = doc["nodes"][by_name["wheel_rl"]].get("translation", (0, 0, 0))
        entry["wheelbase"] = r2(fl[0] - rl[0])
    entry.update(overrides)
    return entry


def piece_entry(doc, index, file, kind, overrides):
    node = doc["nodes"][index]
    low, high, tris = subtree(doc, index, False)
    half = max(high[0] - low[0], high[2] - low[2]) / 2.0
    entry = {"file": file, "node": node["name"]}
    if kind == "animals":
        entry["length"] = r2(high[0] - low[0])
        entry["height"] = r2(high[1] - low[1])
    elif kind in ("items", "track"):
        entry["radius"] = r2(half)
    else:
        entry.update({"radius": r2(half), "height": r2(high[1]), "tags": [], "triangles": tris})
    entry.update(overrides)
    if kind not in ("animals", "items", "track"):
        entry["triangles"] = tris
    return entry


def write(out_dir):
    names = sorted(n for n in os.listdir(out_dir) if n.endswith(".glb"))
    manifest = {
        "version": 1,
        "blender": ".".join(str(v) for v in _blender_version()),
        "generator": "assets/aura-drift/generate.py",
        "totalBytes": 0,
        "files": [],
        "cars": {},
        "items": {},
        "track": {},
        "animals": {},
        "props": {},
    }
    meta_cars = load_meta("meta_cars", "CARS")
    meta = {
        "items": load_meta("meta_items", "ITEMS"),
        "track": load_meta("meta_track", "TRACK"),
        "animals": load_meta("meta_animals", "ANIMALS"),
    }
    meta_props = {"shared": load_meta("meta_shared", "PROPS")}
    for module in ("meta_scenery_a", "meta_scenery_b"):
        meta_props.update(load_meta(module, "PROPS"))

    for name in names:
        data = open(os.path.join(out_dir, name), "rb").read()
        doc = read_glb(data)
        stem = name[:-4]
        placed = sum(mesh_triangles(doc, doc["meshes"][n["mesh"]]) for n in doc["nodes"] if "mesh" in n)
        if stem.startswith("car-"):
            kind = "car"
            car_id = stem[4:]
            manifest["cars"][car_id] = car_entry(doc, name, meta_cars.get(car_id, {}))
        elif stem in PREFIX:
            kind = stem
            for index in roots(doc):
                node = doc["nodes"][index]
                if node["name"].startswith(PREFIX[stem]):
                    key = node["name"][len(PREFIX[stem]) :]
                    manifest[stem][key] = piece_entry(doc, index, name, stem, meta[stem].get(key, {}))
        elif stem == "shared" or stem.startswith("scenery-"):
            kind = "shared" if stem == "shared" else "scenery"
            group = "shared" if stem == "shared" else stem[len("scenery-") :]
            bucket = manifest["props"].setdefault(group, {})
            for index in roots(doc):
                node = doc["nodes"][index]
                if node["name"].startswith("prop-"):
                    key = node["name"][len("prop-") :]
                    bucket[key] = piece_entry(doc, index, name, kind, meta_props.get(group, {}).get(key, {}))
        else:
            kind = "other"
        manifest["files"].append(
            {"name": name, "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest(), "triangles": placed, "kind": kind}
        )
        manifest["totalBytes"] += len(data)

    manifest["props"] = {k: manifest["props"][k] for k in sorted(manifest["props"])}
    with open(os.path.join(out_dir, "manifest.json"), "w", encoding="utf-8", newline="\n") as handle:
        handle.write(json.dumps(manifest, indent=2) + "\n")
    with open(os.path.join(out_dir, "LICENSE.txt"), "w", encoding="utf-8", newline="\n") as handle:
        handle.write(LICENSE_TEXT)
    print("manifest: %d files, %d bytes" % (len(names), manifest["totalBytes"]))


def _blender_version():
    try:
        import bpy

        return bpy.app.version
    except ImportError:
        return (0, 0, 0)
