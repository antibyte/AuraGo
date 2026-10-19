"""Ambient-occlusion bake into vertex colours (MIT License).

Pipeline per scene:

1. ``bake_ao`` runs Cycles' AO bake with target ``VERTEX_COLORS`` (CPU, fixed
   seed and sample count, no denoise) into an ``ao`` attribute of every
   object.
2. ``smooth_ao`` runs a one-ring neighbour relaxation to remove vertex noise.
3. ``finalize`` writes ``Col = palette x occlusion`` as an 8-bit colour
   attribute (the glTF exporter later emits it as ``COLOR_0``) and removes the
   scratch attributes.

``process`` does all three. Emissive and sky objects are excluded from the
occluder set while baking and keep their palette colour unchanged.
"""

import time

import bpy

from . import colors, materials

AO_ATTRIBUTE = "ao"


def _flat(attr, count):
    buf = [0.0] * (count * 4)
    attr.data.foreach_get("color", buf)
    return buf


def bake_ao(objs, samples=192, distance=1.4, seed=11, hidden=()):
    """Bake raw AO (0 = occluded, 1 = open) into each object's ``ao`` attribute.

    ``hidden`` objects (water, sky, stars) are hidden from the render while
    baking so they neither occlude nor get baked.
    """
    sc = bpy.context.scene
    sc.render.engine = "CYCLES"
    sc.cycles.device = "CPU"
    sc.cycles.samples = samples
    sc.cycles.seed = seed
    sc.cycles.use_adaptive_sampling = False
    sc.cycles.use_denoising = False
    sc.render.bake.target = "VERTEX_COLORS"
    sc.render.bake.use_clear = True
    if sc.world is None:
        sc.world = bpy.data.worlds.new("kit-world")
    sc.world.light_settings.distance = distance
    sc.world.light_settings.ao_factor = 1.0

    objs = list(objs)
    for obj in objs:
        mesh = obj.data
        old = mesh.attributes.get(AO_ATTRIBUTE)
        if old is not None:
            mesh.attributes.remove(old)
        attr = mesh.color_attributes.new(AO_ATTRIBUTE, "FLOAT_COLOR", "POINT")
        mesh.color_attributes.active_color = attr

    previous = {}
    for obj in hidden:
        if obj not in previous:  # the same object may appear in several role lists
            previous[obj] = obj.hide_render
            obj.hide_render = True
    bpy.ops.object.select_all(action="DESELECT")
    for obj in objs:
        obj.select_set(True)
    bpy.context.view_layer.objects.active = objs[0]
    t0 = time.time()
    try:
        bpy.ops.object.bake(type="AO", target="VERTEX_COLORS")
    finally:
        for obj, state in previous.items():
            obj.hide_render = state
    return time.time() - t0


def _adjacency(mesh):
    n = len(mesh.vertices)
    flat = [0] * (len(mesh.edges) * 2)
    mesh.edges.foreach_get("vertices", flat)
    nbrs = [[] for _ in range(n)]
    for i in range(0, len(flat), 2):
        a, b = flat[i], flat[i + 1]
        nbrs[a].append(b)
        nbrs[b].append(a)
    return nbrs


def smooth_ao(mesh, iterations=2, blend=0.5):
    """One-ring relaxation of the raw AO values (returns the new list)."""
    n = len(mesh.vertices)
    buf = _flat(mesh.color_attributes[AO_ATTRIBUTE], n)
    ao = buf[0::4]
    nbrs = _adjacency(mesh)
    for _ in range(iterations):
        nxt = ao[:]
        for i in range(n):
            nb = nbrs[i]
            if nb:
                nxt[i] = ao[i] * (1.0 - blend) + blend * (sum(ao[j] for j in nb) / len(nb))
        ao = nxt
    return ao


def finalize(obj, ao_values, tint=(0.0, 0.0, 0.0), gamma=1.0, keep_scratch=False):
    """Write ``Col`` = palette x lerp(1, AO, weight) as an 8-bit attribute."""
    mesh = obj.data
    n = len(mesh.vertices)
    pal = _flat(mesh.color_attributes[colors.PALETTE], n)
    weights = [0.0] * n
    mesh.attributes[colors.AO_WEIGHT].data.foreach_get("value", weights)
    out = [0.0] * (n * 4)
    for i in range(n):
        occ = (1.0 - min(1.0, max(0.0, ao_values[i])) ** gamma) * weights[i]
        j = i * 4
        out[j] = pal[j] * (1.0 - occ * (1.0 - tint[0]))
        out[j + 1] = pal[j + 1] * (1.0 - occ * (1.0 - tint[1]))
        out[j + 2] = pal[j + 2] * (1.0 - occ * (1.0 - tint[2]))
        out[j + 3] = 1.0
    old = mesh.attributes.get(materials.COLOR_ATTRIBUTE)
    if old is not None:
        mesh.attributes.remove(old)
    col = mesh.color_attributes.new(materials.COLOR_ATTRIBUTE, "BYTE_COLOR", "POINT")
    col.data.foreach_set("color", out)
    mesh.color_attributes.active_color = col
    mesh.color_attributes.default_color_name = col.name
    if not keep_scratch:
        for name in (AO_ATTRIBUTE, colors.PALETTE, colors.AO_WEIGHT):
            attr = mesh.attributes.get(name)
            if attr is not None:
                mesh.attributes.remove(attr)


def process(objs, passthrough=(), hidden=(), samples=192, distance=1.4, seed=11, iterations=2,
            tint=(0.0, 0.0, 0.0), gamma=1.0, keep_scratch=False):
    """Bake, smooth and finalize ``objs``; ``passthrough`` objects only get ``Col``.

    ``passthrough`` objects (emissive sky, stars) also stay in the render while
    baking unless listed in ``hidden``. Returns the bake time in seconds.
    """
    objs = list(objs)
    elapsed = bake_ao(objs, samples=samples, distance=distance, seed=seed, hidden=hidden)
    for obj in objs:
        finalize(obj, smooth_ao(obj.data, iterations=iterations), tint=tint, gamma=gamma,
                 keep_scratch=keep_scratch)
    for obj in passthrough:
        n = len(obj.data.vertices)
        pal = _flat(obj.data.color_attributes[colors.PALETTE], n)
        old = obj.data.attributes.get(materials.COLOR_ATTRIBUTE)
        if old is not None:
            obj.data.attributes.remove(old)
        col = obj.data.color_attributes.new(materials.COLOR_ATTRIBUTE, "BYTE_COLOR", "POINT")
        col.data.foreach_set("color", pal)
        obj.data.color_attributes.active_color = col
        for name in (colors.PALETTE, colors.AO_WEIGHT):
            attr = obj.data.attributes.get(name)
            if attr is not None:
                obj.data.attributes.remove(attr)
    return elapsed
