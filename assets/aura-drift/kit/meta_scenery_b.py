"""Manifest metadata for the scenery B kits (forest, industry, glacier, metro); MIT, AuraGo contributors.

Tags drive the triangle budget in the validator (scatter 450, default 1200, hero 4500) and tell the
runtime which props are instanced in bulk (``scatter``) or read from far away (``tall``).
Radius and height are measured from the GLBs by ``kit/manifest.py``.
"""

PROPS = {
    "forest": {
        "oak": {"tags": ["scatter"]},
        "birch": {"tags": ["scatter"]},
        "mushroom_cluster": {"tags": ["hero"]},
        "log_pile": {"tags": []},
        "hunting_hide": {"tags": []},
        "fern_glade": {"tags": ["scatter"]},
        "watermill": {"tags": ["hero", "tall"]},
    },
    "industry": {
        "silo": {"tags": ["hero", "tall"]},
        "smokestack": {"tags": ["tall"]},
        "pipes": {"tags": ["scatter"]},
        "forklift": {"tags": []},
        "pylon": {"tags": ["tall"]},
        "cooling_tower": {"tags": ["hero", "tall", "far"]},
        "chem_tank": {"tags": ["hero"]},
    },
    "glacier": {
        "ice_block": {"tags": ["scatter"]},
        "ice_arch": {"tags": []},
        "snow_pine": {"tags": ["scatter"]},
        "igloo": {"tags": []},
        "research_station": {"tags": ["hero"]},
        "ice_spire": {"tags": ["scatter", "tall"]},
        "snow_cat": {"tags": ["hero"]},
    },
    "metro": {
        "skyscraper_a": {"tags": ["tall"]},
        "skyscraper_b": {"tags": ["tall"]},
        "skyscraper_c": {"tags": ["tall"]},
        "billboard": {"tags": ["tall"]},
        "street_lamp": {"tags": ["scatter"]},
        "food_truck": {"tags": []},
        "taxi": {"tags": []},
        "metro_entrance": {"tags": ["hero"]},
        "arcade_kiosk": {"tags": ["hero", "tall"]},
    },
}
