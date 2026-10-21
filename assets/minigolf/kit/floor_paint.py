"""Per-vertex colours of the floor looks (MIT License, see ../LICENSE).

The P2 felt colouring, generalised to every floor look: a world gradient with a lighter middle and faint
bands across the lane. ``metas`` holds one ``(edge, arc)`` per vertex: ``edge`` is 0 in the middle of the
lane and 1 at its side, ``arc`` the distance along the route. The colours go into the ``palette`` and
``ao_w`` attributes (``colors.paint_values``); the dresser's bake turns them into the exported ``Col``.
"""

from . import colors, palette
from .island import smoothstep
from .palette import mix

# world -> look -> gradient preset of the palette
GRADIENTS = {
    "pirate": {"felt": "felt", "sand": "sand", "deck": "wood", "slab": "rock", "metal": "iron"},
    "temple": {"felt": "felt", "sand": "tile", "deck": "bark", "slab": "stone", "metal": "stone_deep"},
    "moon": {"felt": "felt", "sand": "regolith", "deck": "metal", "slab": "panel", "metal": "metal"},
}
BANDS = {"felt": (0.62, 0.035), "sand": (0.62, 0.035), "deck": (0.31, 0.07), "slab": (0.9, 0.05), "metal": (0.45, 0.04)}
LANE_AO = 0.75


def colours(world, look, metas):
    """RGBA per vertex for ``look`` in ``world``."""
    lo, hi = palette.get(world).grad(GRADIENTS[world][look])
    period, amplitude = BANDS[look]
    out = []
    for edge, arc in metas:
        c = mix(lo, hi, 0.30 + 0.38 * smoothstep(1.0 - edge, 0.0, 0.75))
        band = 1.0 + (amplitude if int(arc / period) % 2 == 0 else -amplitude)
        out.append((c[0] * band, c[1] * band, c[2] * band, 1.0))
    return out


def paint(obj, world, look, metas):
    colors.paint_values(obj, colours(world, look, metas), ao=LANE_AO)
