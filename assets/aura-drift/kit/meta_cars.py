"""Aura Drift car manifest overrides (MIT, AuraGo contributors).

The manifest measures every car straight from its GLB, which includes awnings, a side table, a hat,
a flag or a light bar. These entries give the *vehicle body* size (what the shell and the HUD scale
against); wheelRadius and wheelbase stay measured.
"""

CARS = {
    "tundra": {"length": 3.9, "width": 2.0, "height": 1.62},
    "softeis": {"length": 5.1, "width": 2.2, "height": 2.4},
    "maehkoenig": {"length": 2.7, "width": 1.3, "height": 1.5},
    "couch": {"length": 3.0, "width": 1.8, "height": 1.62},
    "bertha": {"length": 5.3, "width": 3.0, "height": 3.1},
}
