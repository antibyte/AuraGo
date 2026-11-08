"""Aura Drift kit materials (MIT, AuraGo contributors).

Every material is a Principled BSDF whose Base Color is the material colour multiplied by the
``Col`` colour attribute through a Color Attribute node (Blender renders it that way). The glTF
exporter writes the socket's default value as ``baseColorFactor`` and the active attribute as ``COLOR_0``. Surfaces cull back faces.
"""

from . import core
from .core import COLOR_LAYER, MATERIALS, hex_linear

# name -> (base, metallic, roughness, emission, strength, alpha)
CAR_MATERIALS = {
    "paint": ("#ffffff", 0.05, 0.22, None, 0.0, 1.0),
    "paint_accent": ("#ffffff", 0.05, 0.28, None, 0.0, 1.0),
    "trim": ("#2b2f38", 0.15, 0.55, None, 0.0, 1.0),
    "glass": ("#1a2c46", 0.0, 0.04, None, 0.0, 0.62),
    "chrome": ("#dfe4ec", 1.0, 0.16, None, 0.0, 1.0),
    "head_light": ("#fff2cc", 0.0, 0.3, "#fff2cc", 3.0, 1.0),
    "brake_light": ("#ff2a1a", 0.0, 0.3, "#ff2a1a", 2.0, 1.0),
    "tyre": ("#ffffff", 0.0, 0.62, None, 0.0, 1.0),
}


def make_material(name, base, metallic, roughness, emission=None, strength=0.0, alpha=1.0, vertex_color=True):
    """Create (or return the already created) material ``name`` and register it."""
    if name in MATERIALS:
        return MATERIALS[name]
    import bpy

    material = bpy.data.materials.new(name)
    material.use_nodes = True
    tree = material.node_tree
    bsdf = tree.nodes["Principled BSDF"]
    rgb = hex_linear(base)
    base_rgba = (rgb[0], rgb[1], rgb[2], 1.0)
    # Keep the material factor even when the vertex-colour Mix drives the preview.
    bsdf.inputs["Base Color"].default_value = base_rgba
    bsdf.inputs["Metallic"].default_value = metallic
    bsdf.inputs["Roughness"].default_value = roughness
    if emission is not None:
        e = hex_linear(emission)
        bsdf.inputs["Emission Color"].default_value = (e[0], e[1], e[2], 1.0)
        bsdf.inputs["Emission Strength"].default_value = strength
    else:
        bsdf.inputs["Emission Strength"].default_value = 0.0
    if alpha < 1.0:
        bsdf.inputs["Alpha"].default_value = alpha
        try:
            material.surface_render_method = "BLENDED"
        except (AttributeError, TypeError):
            pass
    if vertex_color:
        attribute = tree.nodes.new("ShaderNodeVertexColor")
        attribute.layer_name = COLOR_LAYER
        mix = tree.nodes.new("ShaderNodeMix")
        mix.data_type = "RGBA"
        mix.blend_type = "MULTIPLY"
        mix.inputs[0].default_value = 1.0
        mix.inputs[6].default_value = base_rgba
        tree.links.new(attribute.outputs["Color"], mix.inputs[7])
        tree.links.new(mix.outputs[2], bsdf.inputs["Base Color"])
    else:
        bsdf.inputs["Base Color"].default_value = base_rgba
    material.use_backface_culling = True
    material.diffuse_color = base_rgba
    MATERIALS[name] = material
    return material


def build_car_materials():
    for name, (base, metallic, roughness, emission, strength, alpha) in CAR_MATERIALS.items():
        make_material(name, base, metallic, roughness, emission, strength, alpha)


def build_all():
    """Materials shared by every group; groups add their own with make_material()."""
    core.MATERIALS.clear()
    build_car_materials()
