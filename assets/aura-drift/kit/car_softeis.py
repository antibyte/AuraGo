"""Softeis: reviewed NC AI concept geometry in the native Aura Drift rig (MIT).

The retained vertex-colour source has no textures or online build dependency.
Wheel pivots and the roof-joint cone use the shared runtime animation contract.
"""

import gzip
import json
from pathlib import Path

from .core import Accum, Model, hex_linear
from .cars_common import wheel_accum


def build():
    source = Path(__file__).with_name("softeis_mesh.json.gz")
    with gzip.open(source, "rt", encoding="utf-8") as handle:
        data = json.load(handle)
    model = Model("car-softeis")
    body = None
    for node in data["nodes"]:
        parts = []
        for material, geometry in sorted(node["parts"].items()):
            acc = Accum()
            acc.verts = geometry["verts"]
            acc.faces = geometry["faces"]
            acc.cols = geometry["cols"]
            parts.append((material, acc))
        obj = model.mesh(node["name"], parts, shade="auto", parent=body,
                         location=node["location"])
        if node["name"] == "body":
            body = obj
    right = wheel_accum(data["wheels"]["wheel_fr"][1], 0.56, hub=hex_linear("#ffd1e3"), spokes=6, tread_blocks=8)
    for side in ("l", "r"):
        parts = [("tyre", right)]
        if side == "l":
            mirrored = Accum()
            mirrored.verts = [(x, y, -z) for x, y, z in right.verts]
            mirrored.faces = [list(reversed(face)) for face in right.faces]
            mirrored.cols = right.cols
            parts = [("tyre", mirrored)]
        front = "wheel_f" + side
        wheel = model.mesh(front, parts, shade="auto", location=data["wheels"][front])
        rear = "wheel_r" + side
        model.instance(rear, wheel, data["wheels"][rear])
    for name, location in sorted(data["anchors"].items()):
        model.empty(name, location)
    return model
