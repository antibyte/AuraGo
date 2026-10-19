"""Shared material set (MIT License).

All colour comes from the ``Col`` vertex-colour attribute, so a world needs
only a handful of materials: matte, satin, metal, glass, water and emissive.
The Color Attribute node feeding Principled Base Color is what the glTF
exporter turns into ``COLOR_0``. Emissive materials also take their emission
colour from ``Col`` so neon, flames and windows keep their gradients.
"""

import bpy

from .palette import hex_to_linear

COLOR_ATTRIBUTE = "Col"

# kind -> Principled settings (colour always comes from the attribute).
_KINDS = {
    "matte": {"Roughness": 0.88, "Metallic": 0.0},
    "satin": {"Roughness": 0.46, "Metallic": 0.0},
    "metal": {"Roughness": 0.42, "Metallic": 0.5},
    "glass": {"Roughness": 0.06, "Metallic": 0.0, "Transmission Weight": 0.9},
    "water": {"Roughness": 0.10, "Metallic": 0.0},
    "emissive": {"Roughness": 0.5, "Metallic": 0.0},
    "glow": {"Roughness": 0.35, "Metallic": 0.0},
}


def _set(bsdf, name, value):
    sock = bsdf.inputs.get(name)
    if sock is not None:
        sock.default_value = value


def get(kind, strength=3.0, bump=0.0):
    """Return the shared material ``kind`` (created on first use).

    ``strength`` applies to ``emissive`` (self-lit, base nearly black) and ``glow``
    (lit surface plus a soft emission, for gems and eyes); ``bump`` adds a render-only
    ripple to ``water``. Variants get their own datablock name so repeated
    calls with the same arguments return the same material.
    """
    if kind not in _KINDS:
        raise KeyError(kind)
    name = "kit-" + kind
    if kind in ("emissive", "glow"):
        name += "-%g" % strength
    if kind == "water" and bump:
        name += "-b%g" % bump
    mat = bpy.data.materials.get(name)
    if mat is not None:
        return mat
    mat = bpy.data.materials.new(name)
    mat.use_nodes = True
    tree = mat.node_tree
    for node in list(tree.nodes):
        tree.nodes.remove(node)
    out = tree.nodes.new("ShaderNodeOutputMaterial")
    out.location = (500, 0)
    bsdf = tree.nodes.new("ShaderNodeBsdfPrincipled")
    bsdf.location = (200, 0)
    attr = tree.nodes.new("ShaderNodeVertexColor")
    attr.layer_name = COLOR_ATTRIBUTE
    attr.location = (-200, 0)
    if kind == "emissive":
        # Emission dominates: the diffuse base is only a faint copy of the colour, so strong
        # light does not wash the glow out. (Runtime maps this kind to emissive + COLOR_0.)
        dim = tree.nodes.new("ShaderNodeMix")
        dim.data_type = "RGBA"
        dim.blend_type = "MULTIPLY"
        dim.inputs["Factor"].default_value = 1.0
        dim.inputs["B"].default_value = (0.12, 0.12, 0.12, 1.0)
        dim.location = (0, 0)
        tree.links.new(attr.outputs["Color"], dim.inputs["A"])
        tree.links.new(dim.outputs["Result"], bsdf.inputs["Base Color"])
    else:
        tree.links.new(attr.outputs["Color"], bsdf.inputs["Base Color"])
    tree.links.new(bsdf.outputs["BSDF"], out.inputs["Surface"])
    for key, value in _KINDS[kind].items():
        _set(bsdf, key, value)
    if kind in ("emissive", "glow"):
        tree.links.new(attr.outputs["Color"], bsdf.inputs["Emission Color"])
        _set(bsdf, "Emission Strength", strength)
    if kind == "glass":
        _set(bsdf, "IOR", 1.45)
    if kind == "water":
        _set(bsdf, "Specular IOR Level", 0.3)
    if kind == "water" and bump:
        noise = tree.nodes.new("ShaderNodeTexNoise")
        noise.inputs["Scale"].default_value = 2.2
        noise.inputs["Detail"].default_value = 4.0
        noise.location = (-200, -300)
        node_bump = tree.nodes.new("ShaderNodeBump")
        node_bump.inputs["Strength"].default_value = bump
        node_bump.inputs["Distance"].default_value = 0.06
        node_bump.location = (0, -300)
        tree.links.new(noise.outputs["Fac"], node_bump.inputs["Height"])
        tree.links.new(node_bump.outputs["Normal"], bsdf.inputs["Normal"])
    mat.diffuse_color = (0.8, 0.8, 0.8, 1.0)
    return mat


def earth(sun_dir, name="kit-earth"):
    """Procedural emissive Earth for the sky backdrop (Cycles-only; the runtime bakes a texture).

    Continents, swirled clouds and polar ice come from object-space noise; the
    terminator follows ``sun_dir`` (world space) and a blue atmosphere rim is
    added on the lit limb. The mesh must be a unit sphere with the real size
    applied through the object scale.
    """
    mat = bpy.data.materials.get(name)
    if mat is not None:
        return mat
    mat = bpy.data.materials.new(name)
    mat.use_nodes = True
    tree = mat.node_tree
    for node in list(tree.nodes):
        tree.nodes.remove(node)
    nodes, links = tree.nodes, tree.links

    def node(kind, **kw):
        n = nodes.new(kind)
        for k, v in kw.items():
            setattr(n, k, v)
        return n

    def ramp(stops):
        r = node("ShaderNodeValToRGB")
        r.color_ramp.interpolation = "LINEAR"
        elems = r.color_ramp.elements
        while len(elems) < len(stops):
            elems.new(0.5)
        for el, (pos, col) in zip(elems, stops):
            el.position, el.color = pos, col
        return r

    def mix(fac, a, b, blend="MIX"):
        m = node("ShaderNodeMix", data_type="RGBA", blend_type=blend)
        if isinstance(fac, (int, float)):
            m.inputs["Factor"].default_value = fac
        else:
            links.new(fac, m.inputs["Factor"])
        for sock, val in (("A", a), ("B", b)):
            if isinstance(val, tuple):
                m.inputs[sock].default_value = val
            else:
                links.new(val, m.inputs[sock])
        return m.outputs["Result"]

    def lin(h):
        r, g, b, _ = hex_to_linear(h)
        return (r, g, b, 1.0)

    coord = node("ShaderNodeTexCoord").outputs["Object"]
    cont = node("ShaderNodeTexNoise", noise_dimensions="3D")
    cont.inputs["Scale"].default_value = 1.9
    cont.inputs["Detail"].default_value = 7.0
    cont.inputs["Roughness"].default_value = 0.56
    cont.inputs["Distortion"].default_value = 0.25
    links.new(coord, cont.inputs["Vector"])
    land = ramp([(0.0, lin("#0a3694")), (0.40, lin("#1d74dd")), (0.515, lin("#43b4d8")), (0.545, lin("#3f9d4a")),
                 (0.64, lin("#5aa84c")), (0.76, lin("#b3a35a")), (0.92, lin("#8a7a5c"))])
    links.new(cont.outputs["Fac"], land.inputs["Fac"])
    cloud_n = node("ShaderNodeTexNoise", noise_dimensions="3D")
    cloud_n.inputs["Scale"].default_value = 3.4
    cloud_n.inputs["Detail"].default_value = 8.0
    cloud_n.inputs["Roughness"].default_value = 0.62
    cloud_n.inputs["Distortion"].default_value = 0.9
    links.new(coord, cloud_n.inputs["Vector"])
    cloud_r = ramp([(0.5, (0.0, 0.0, 0.0, 1.0)), (0.76, (1.0, 1.0, 1.0, 1.0))])
    links.new(cloud_n.outputs["Fac"], cloud_r.inputs["Fac"])
    surf = mix(cloud_r.outputs["Color"], land.outputs["Color"], lin("#f4f8ff"))
    z = node("ShaderNodeSeparateXYZ")
    links.new(coord, z.inputs["Vector"])
    ab = node("ShaderNodeMath", operation="ABSOLUTE")
    links.new(z.outputs["Z"], ab.inputs[0])
    ice = node("ShaderNodeMapRange")
    ice.inputs["From Min"].default_value = 0.84
    ice.inputs["From Max"].default_value = 0.93
    links.new(ab.outputs["Value"], ice.inputs["Value"])
    surf = mix(ice.outputs["Result"], surf, lin("#f4f8ff"))
    geo = node("ShaderNodeNewGeometry")
    dot = node("ShaderNodeVectorMath", operation="DOT_PRODUCT")
    links.new(geo.outputs["Normal"], dot.inputs[0])
    dot.inputs[1].default_value = tuple(sun_dir)
    lit = node("ShaderNodeMapRange")
    lit.inputs["From Min"].default_value = -0.3
    lit.inputs["From Max"].default_value = 0.5
    links.new(dot.outputs["Value"], lit.inputs["Value"])
    lit_col = mix(lit.outputs["Result"], lin("#050c24"), surf)
    fres = node("ShaderNodeLayerWeight")
    fres.inputs["Blend"].default_value = 0.2
    glow_w = node("ShaderNodeMath", operation="MULTIPLY")
    links.new(fres.outputs["Fresnel"], glow_w.inputs[0])
    links.new(lit.outputs["Result"], glow_w.inputs[1])
    final = mix(glow_w.outputs["Value"], lit_col, lin("#4aa6ff"), blend="ADD")
    emit = node("ShaderNodeEmission")
    emit.inputs["Strength"].default_value = 1.0
    links.new(final, emit.inputs["Color"])
    out = node("ShaderNodeOutputMaterial")
    links.new(emit.outputs["Emission"], out.inputs["Surface"])
    return mat
