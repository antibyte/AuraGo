"""Vertex-colour helpers (MIT License).

Every kit part carries two POINT-domain attributes before baking:

* ``palette`` - the gradient colour (linear RGBA, float),
* ``ao_w``   - how strongly baked occlusion darkens this vertex (0..1).

``bake.finalize`` later multiplies them into the exported ``Col`` attribute.
"""

import colorsys
import random

from mathutils import Vector, noise

PALETTE = "palette"
AO_WEIGHT = "ao_w"


def jitter_color(c, rng, amount):
    """Tiny seeded hue/value shift so repeated parts are not identical."""
    if not amount:
        return c
    h, s, v = colorsys.rgb_to_hsv(max(c[0], 0.0), max(c[1], 0.0), max(c[2], 0.0))
    h = (h + rng.uniform(-1, 1) * amount * 0.25) % 1.0
    v = v * (1.0 + rng.uniform(-1, 1) * amount * 2.0)
    r, g, b = colorsys.hsv_to_rgb(h, s, v)
    return (r, g, b, c[3])


def _lerp(a, b, t):
    return (a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t,
            a[2] + (b[2] - a[2]) * t, a[3] + (b[3] - a[3]) * t)


def _stop_color(stops, t):
    if t <= stops[0][0]:
        return stops[0][1]
    for k in range(1, len(stops)):
        if t <= stops[k][0]:
            t0, c0 = stops[k - 1]
            t1, c1 = stops[k]
            return _lerp(c0, c1, (t - t0) / max(t1 - t0, 1e-9))
    return stops[-1][1]


def coords(mesh):
    flat = [0.0] * (len(mesh.vertices) * 3)
    mesh.vertices.foreach_get("co", flat)
    return [(flat[i], flat[i + 1], flat[i + 2]) for i in range(0, len(flat), 3)]


def write_attribute(mesh, name, values, kind="color"):
    """(Re)create a POINT attribute from a list of RGBA tuples or floats."""
    old = mesh.attributes.get(name)
    if old is not None:
        mesh.attributes.remove(old)
    if kind == "color":
        attr = mesh.color_attributes.new(name, "FLOAT_COLOR", "POINT")
        flat = [c for rgba in values for c in rgba]
        attr.data.foreach_set("color", flat)
    else:
        attr = mesh.attributes.new(name, "FLOAT", "POINT")
        attr.data.foreach_set("value", list(values))
    return attr


def set_ao_weight(obj, weight):
    write_attribute(obj.data, AO_WEIGHT, [weight] * len(obj.data.vertices), kind="float")


def paint(obj, bottom, top=None, mode="z", ease=1.0, span=None, rng=None, jitter=0.02,
          noise_amp=0.0, noise_scale=1.6, ao=0.65, center=None, stops=None, pattern=None):
    """Write the ``palette`` gradient of a part.

    ``mode`` is the gradient axis: ``"z"``, ``"x"``, ``"y"`` or ``"radial"``
    (distance from ``center`` in XY). ``span=(lo, hi)`` overrides the part's own
    bounding range; ``ease`` bends the ramp (>1 keeps the bottom colour longer);
    ``noise_amp`` adds low-frequency value variation; ``stops`` is an optional
    multi-stop ramp ``[(t, colour), ...]`` replacing bottom/top; ``pattern`` is an
    optional ``fn(x, y, z) -> multiplier`` (float or RGB triple) for grooves,
    stripes and other painted detail.
    """
    rng = rng or random.Random(0)
    pts = coords(obj.data)
    if top is None:
        top = bottom
    if stops is None:
        stops = [(0.0, jitter_color(bottom, rng, jitter)), (1.0, jitter_color(top, rng, jitter))]
    else:
        stops = [(t, jitter_color(c, rng, jitter)) for t, c in stops]
    if mode == "radial":
        cx, cy = center if center is not None else (
            sum(p[0] for p in pts) / len(pts), sum(p[1] for p in pts) / len(pts))
        vals = [((p[0] - cx) ** 2 + (p[1] - cy) ** 2) ** 0.5 for p in pts]
    else:
        axis = {"x": 0, "y": 1, "z": 2}[mode]
        vals = [p[axis] for p in pts]
    lo, hi = span if span is not None else (min(vals), max(vals))
    inv = 1.0 / (hi - lo) if hi - lo > 1e-9 else 0.0
    offset = Vector((rng.uniform(-40, 40), rng.uniform(-40, 40), rng.uniform(-40, 40)))
    out = []
    for p, v in zip(pts, vals):
        t = min(1.0, max(0.0, (v - lo) * inv)) ** ease
        c = _stop_color(stops, t)
        if noise_amp:
            k = 1.0 + noise_amp * noise.noise(Vector(p) * noise_scale + offset)
            c = (c[0] * k, c[1] * k, c[2] * k, c[3])
        if pattern is not None:
            m = pattern(*p)
            if not isinstance(m, (tuple, list)):
                m = (m, m, m)
            c = (c[0] * m[0], c[1] * m[1], c[2] * m[2], c[3])
        out.append(c)
    write_attribute(obj.data, PALETTE, out)
    write_attribute(obj.data, AO_WEIGHT, [ao] * len(pts), kind="float")
    return obj


def paint_fn(obj, fn, ao=0.65):
    """Write ``palette`` from ``fn(x, y, z) -> RGBA`` (and a uniform AO weight)."""
    pts = coords(obj.data)
    write_attribute(obj.data, PALETTE, [fn(*p) for p in pts])
    write_attribute(obj.data, AO_WEIGHT, [ao] * len(pts), kind="float")
    return obj


def paint_values(obj, colors, ao_weights=None, ao=0.65):
    """Write precomputed per-vertex colours (and optionally AO weights)."""
    write_attribute(obj.data, PALETTE, colors)
    weights = ao_weights if ao_weights is not None else [ao] * len(colors)
    write_attribute(obj.data, AO_WEIGHT, weights, kind="float")
    return obj
