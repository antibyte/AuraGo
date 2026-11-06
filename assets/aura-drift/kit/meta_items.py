"""Manifest metadata for items.glb (MIT, AuraGo contributors). Values extend the measured entry.

``height`` is the top of the model above its base (y = 0) in metres, so the runtime can hover an
item at the right level; ``length`` is the nose-to-tail extent for the directional ones.
"""

ITEMS = {
    "crate": {"radius": 1.0, "height": 1.92},
    "gnome_mine": {"height": 2.03},
    "rubber_duck": {"height": 1.66},
    "rocket": {"height": 1.34, "length": 2.74},
    "bratwurst": {"height": 0.69, "length": 1.84},
    "boxing_glove": {"height": 1.66, "length": 2.8},
    "disco_ball": {"height": 2.49},
    "harpoon": {"height": 1.0, "length": 3.71},
    "air_horn": {"height": 2.22, "length": 1.85},
    "frost_grenade": {"height": 1.59},
}
