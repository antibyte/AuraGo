"""Apply reviewed reconstruction geometry and retained UV artwork to native rigs (MIT)."""
import gzip
import json
from pathlib import Path

import bpy

from .core import Accum, MATERIALS, hex_linear, to_blender
from .cars_common import wheel_accum
from .materials import make_material

SOURCE = Path(__file__).resolve().parents[1] / "reviewed"


def apply(model):
    source = SOURCE / (model.name + ".json.gz")
    if not source.exists():
        return
    with gzip.open(source, "rt", encoding="utf-8") as handle:
        data = json.load(handle)
    make_material("reviewed_toy", "#ffffff", 0.05, 0.35)
    if model.name.startswith("car-"):
        # Reconstructed window shells have no separate interior behind their glazing.
        MATERIALS["glass"].node_tree.nodes["Principled BSDF"].inputs["Alpha"].default_value = 1.0
        trim_tree = MATERIALS["trim"].node_tree
        trim = trim_tree.nodes["Principled BSDF"]
        for link in list(trim.inputs["Base Color"].links):
            trim_tree.links.remove(link)
        trim.inputs["Base Color"].default_value = (*hex_linear("#10131a"), 1)
        trim.inputs["Metallic"].default_value = 0
        trim.inputs["Roughness"].default_value = .95
    if "texture" in data:
        image = bpy.data.images.load(str(SOURCE / data["texture"]), check_existing=False)
        image.alpha_mode = "CHANNEL_PACKED"
        for role in ("paint", "paint_accent"):
            mat = MATERIALS[role]
            tree = mat.node_tree
            texture = tree.nodes.new("ShaderNodeTexImage")
            texture.image = image
            tree.links.new(texture.outputs["Color"], tree.nodes["Principled BSDF"].inputs["Base Color"])
    objects = {obj.name: obj for obj in model.objects}
    for name, location in data.get("locations", {}).items():
        objects[name].location = to_blender(location)
    for node in data["nodes"]:
        target = objects[node["name"]]
        parts = []
        for material, geometry in sorted(node["parts"].items()):
            acc = Accum()
            acc.verts, acc.faces, acc.cols = geometry["verts"], geometry["faces"], geometry["cols"]
            assert len(acc.verts) == len(acc.cols) and acc.faces, node["name"]
            assert all(0 <= i < len(acc.verts) for face in acc.faces for i in face), node["name"]
            parts.append((material, acc))
        temporary = model.mesh("reviewed_replacement", parts, shade="auto")
        if any("uvs" in geometry for geometry in node["parts"].values()):
            uv = temporary.data.uv_layers.new(name="UVMap")
            coordinates = [uv for _, geometry in sorted(node["parts"].items())
                for uv in geometry.get("uvs", [(0, 0)] * sum(map(len, geometry["faces"])))]
            assert len(coordinates) == len(uv.data), node["name"]
            uv.data.foreach_set("uv", [v for pair in coordinates for v in pair])
            for attribute in list(temporary.data.color_attributes):
                temporary.data.color_attributes.remove(attribute)
        target.data = temporary.data
        model.stats[target.name] = model.stats.pop(temporary.name)
        model.objects.remove(temporary)
        model.roots.remove(temporary)
        bpy.data.objects.remove(temporary, do_unlink=True)
        if "location" in node:
            target.location = to_blender(node["location"])
    if "wheels" in data:
        wheel = data["wheels"]
        right = wheel_accum(wheel["radius"], wheel["width"], hub=hex_linear(wheel["hub"]), spokes=wheel.get("spokes", 5), tread_blocks=8)
        for side in ("l", "r"):
            geometry = right
            if side == "l":
                geometry = Accum()
                geometry.verts = [(x, y, -z) for x, y, z in right.verts]
                geometry.faces = [tuple(reversed(face)) for face in right.faces]
                geometry.cols = right.cols
            temporary = model.mesh("reviewed_wheel", [("tyre", geometry)], shade="auto")
            for axle in ("f", "r"):
                name = "wheel_" + axle + side
                objects[name].data = temporary.data
                objects[name].location = to_blender(wheel["positions"][name])
                dimensions = wheel.get("axles", {}).get(axle)
                if dimensions:
                    radial = dimensions["radius"] / wheel["radius"]
                    objects[name].scale = (radial, dimensions["width"] / wheel["width"], radial)
                model.stats[name] = geometry.triangles()
            model.stats.pop(temporary.name)
            model.objects.remove(temporary)
            model.roots.remove(temporary)
            bpy.data.objects.remove(temporary, do_unlink=True)
        for name, location in data["anchors"].items():
            objects[name].location = to_blender(location)
