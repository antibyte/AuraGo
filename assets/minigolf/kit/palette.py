"""Per-world palettes of named tones and gradient presets (MIT License).

Tones are written as sRGB hex strings, the way an artist picks them, and are
converted to Blender's linear RGBA on access. A gradient preset is a pair of
tone names: ``(bottom, top)``. Bottoms are darker and cooler, tops lighter and
warmer, which reads as soft sky light plus ground bounce once ambient
occlusion is multiplied in.
"""


def srgb_to_linear(c):
    c = float(c)
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def linear_to_srgb(c):
    c = max(0.0, float(c))
    return c * 12.92 if c <= 0.0031308 else 1.055 * (c ** (1 / 2.4)) - 0.055


def hex_to_linear(value, alpha=1.0):
    """Convert ``"#rrggbb"`` (sRGB) to a linear ``(r, g, b, a)`` tuple."""
    value = value.lstrip("#")
    r, g, b = (int(value[i:i + 2], 16) / 255.0 for i in (0, 2, 4))
    return (srgb_to_linear(r), srgb_to_linear(g), srgb_to_linear(b), alpha)


def mix(a, b, t):
    """Linear interpolation between two RGBA tuples."""
    return tuple(a[i] + (b[i] - a[i]) * t for i in range(4))


def scale(c, k):
    """Scale the RGB part of a colour, keeping alpha."""
    return (c[0] * k, c[1] * k, c[2] * k, c[3])


class Palette:
    """Named tones and gradient presets for one world."""

    def __init__(self, name, tones, gradients):
        self.name = name
        self._tones = {k: hex_to_linear(v) for k, v in tones.items()}
        self._grads = dict(gradients)

    def c(self, tone):
        return self._tones[tone]

    def grad(self, preset):
        """Return ``(bottom, top)`` linear colours of a gradient preset."""
        lo, hi = self._grads[preset]
        return self._tones[lo], self._tones[hi]

    def pair(self, lo, hi):
        return self._tones[lo], self._tones[hi]

    def has(self, tone):
        return tone in self._tones


_PIRATE_TONES = {
    # water
    "foam": "#f6fffb", "lagoon_shore": "#a3eadb", "lagoon_light": "#52d0c4",
    "lagoon_mid": "#27a5b1", "lagoon_deep": "#14708d",
    # sand and earth
    "sand_light": "#fbe3a6", "sand": "#efc880", "sand_wet": "#cfa066",
    "sand_dark": "#a97b47", "soil": "#8a5b34", "soil_dark": "#5d3b22",
    "soil_light": "#b07e4d",
    # felt and grass
    "felt_light": "#52c762", "felt": "#35a652", "felt_dark": "#237c43",
    "grass_light": "#a6dc4e", "grass": "#5fb745", "grass_dark": "#2f8438",
    # wood
    "wood_light": "#d79a5c", "wood": "#b06c36", "wood_dark": "#7a4623",
    "wood_deep": "#4c2a15", "plank_gap": "#3d2112",
    # cloth, rope
    "cream": "#f7ecc8", "cream_dark": "#dcc592", "rope": "#d6bb86",
    "rope_dark": "#9f8458",
    # accents
    "red": "#dc3b2e", "red_dark": "#a02620", "gold": "#ffc83d",
    "gold_dark": "#cf8f1c", "gold_light": "#ffe48a",
    # rock
    "rock_light": "#c8bcad", "rock": "#9a8e82", "rock_dark": "#675d55",
    # metal
    "iron": "#3a3d46", "iron_light": "#757a85", "iron_dark": "#23252c",
    "black": "#22242b", "skull": "#f7f3e8",
    # palms
    "trunk_light": "#d0955a", "trunk_dark": "#835733",
    "frond_light": "#8fe05a", "frond": "#47b04a", "frond_dark": "#237f3f",
    "shell_pink": "#f6b7b0", "shell_white": "#fff1dc",
    "glass": "#7fd6e8", "coal": "#14161b",
}
_PIRATE_GRADS = {
    "wood": ("wood_dark", "wood_light"), "wood_dark": ("wood_deep", "wood"),
    "cream": ("cream_dark", "cream"), "rope": ("rope_dark", "rope"),
    "red": ("red_dark", "red"), "gold": ("gold_dark", "gold_light"),
    "rock": ("rock_dark", "rock_light"), "iron": ("iron_dark", "iron_light"),
    "trunk": ("trunk_dark", "trunk_light"), "frond": ("frond_dark", "frond_light"),
    "grass": ("grass_dark", "grass_light"), "felt": ("felt_dark", "felt_light"),
    "sand": ("sand_wet", "sand_light"), "soil": ("soil_dark", "soil_light"),
}

_TEMPLE_TONES = {
    "jungle_deep": "#0f3a1d", "jungle_dark": "#1b5a26", "jungle": "#2f8a2f",
    "jungle_light": "#62b033", "yellow_green": "#9ab530", "moss": "#4a6420",
    "moss_light": "#788f2a", "moss_dark": "#2f4418",
    "stone_dark": "#7f6040", "stone": "#b99867", "stone_light": "#dbc08f",
    "stone_cold": "#938063", "stone_deep": "#54402d",
    "gold": "#ffcb3e", "gold_dark": "#c8861a", "gold_light": "#ffe992",
    "fire_orange": "#ff7b19", "fire_yellow": "#ffd24e", "fire_red": "#e8400f",
    "water": "#35bdb4", "water_deep": "#0f7c84", "water_light": "#8ae9d6",
    "water_white": "#e1fff6", "foam": "#f1fffa",
    "dirt": "#8b6841", "dirt_dark": "#59412a", "dirt_light": "#b38e5c",
    "felt_light": "#3fb67a", "felt": "#25966b", "felt_dark": "#17704f",
    "tile": "#cbae7c", "tile_dark": "#957650", "eye_glow": "#ffe27a",
    "totem_red": "#c4452a", "totem_teal": "#2aa59a", "black": "#1d1a18",
    "bark": "#6b4a2c", "bark_light": "#9a6e3e", "lianas": "#3f8f2e",
    "cream": "#f3e6c0", "cream_dark": "#cdb98a",
    "pink": "#ff7aa8", "bloom": "#ffc24b",
}
_TEMPLE_GRADS = {
    "stone": ("stone_dark", "stone_light"), "stone_deep": ("stone_deep", "stone"),
    "moss": ("moss_dark", "moss_light"), "jungle": ("jungle_dark", "jungle_light"),
    "leaf": ("jungle_dark", "yellow_green"), "gold": ("gold_dark", "gold_light"),
    "dirt": ("dirt_dark", "dirt_light"), "felt": ("felt_dark", "felt_light"),
    "bark": ("bark", "bark_light"), "tile": ("tile_dark", "tile"),
    "fire": ("fire_red", "fire_yellow"), "water": ("water_deep", "water_light"),
}

_MOON_TONES = {
    "regolith_light": "#b4b0c6", "regolith": "#908da6", "regolith_dark": "#625f7c",
    "regolith_deep": "#3b3a58", "dust": "#cdc9dc",
    "panel_white": "#f1f4fa", "panel_grey": "#c5cbda", "panel_dark": "#3b4358",
    "panel_blue": "#6f7fa6", "metal": "#a4adc0", "metal_dark": "#4b536a",
    "orange": "#ff8c2b", "orange_dark": "#c85d12",
    "cyan": "#2ff4ff", "magenta": "#ff3ff0", "cyan_dark": "#0a8fb0", "magenta_dark": "#a8148f", "window": "#a8f7ff",
    "felt_light": "#33349a", "felt": "#25257a", "felt_dark": "#17175a",
    "earth_ocean": "#1f78e0", "earth_deep": "#0c3e9c", "earth_land": "#58b04f",
    "earth_cloud": "#f4f8ff", "black": "#14161f", "rubber": "#262a36",
    "gold": "#ffd24a", "solar": "#1c3b8f", "solar_light": "#4a76d6",
    "star": "#ffffff", "void": "#07091a",
}
_MOON_GRADS = {
    "regolith": ("regolith_dark", "regolith_light"), "rock": ("regolith_deep", "regolith"),
    "panel": ("panel_grey", "panel_white"), "metal": ("metal_dark", "metal"),
    "orange": ("orange_dark", "orange"), "felt": ("felt_dark", "felt_light"),
    "dark": ("black", "panel_dark"), "solar": ("solar", "solar_light"),
}

_BUILDERS = {
    "pirate": ("pirate", _PIRATE_TONES, _PIRATE_GRADS),
    "temple": ("temple", _TEMPLE_TONES, _TEMPLE_GRADS),
    "moon": ("moon", _MOON_TONES, _MOON_GRADS),
}
_CACHE = {}


def get(world):
    """Return the (cached) :class:`Palette` of ``world``."""
    if world not in _CACHE:
        name, tones, grads = _BUILDERS[world]
        _CACHE[world] = Palette(name, tones, grads)
    return _CACHE[world]
