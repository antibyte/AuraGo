"""Manifest metadata for the scenery A kits: alpine, desert, harbour, salt (MIT, AuraGo contributors).

Tags drive the validator's triangle budget (scatter 450, default 1200, hero 4500) and the runtime's
instancing: ``scatter`` props are placed hundreds of times, ``tall`` ones keep a taller clearance and
``far`` ones are kept for distance dressing. Radius and height are measured from the GLB.
"""

PROPS = {
    "alpine": {
        "pine_a": {"tags": ["scatter", "tall"]},
        "pine_b": {"tags": ["scatter", "tall"]},
        "rock_a": {"tags": ["scatter"]},
        "rock_b": {"tags": ["scatter"]},
        "chalet": {"tags": ["hero"]},
        "cablecar_pylon": {"tags": ["tall"]},
    },
    "desert": {
        "cactus_a": {"tags": ["scatter"]},
        "cactus_b": {"tags": ["scatter"]},
        "mesa_rock": {"tags": ["hero", "tall", "far"]},
        "water_tower": {"tags": ["hero", "tall"]},
        "windmill_pump": {"tags": ["hero", "tall"]},
    },
    "harbour": {
        "container_stack": {"tags": ["hero"]},
        "crane": {"tags": ["hero", "tall", "far"]},
        "lighthouse": {"tags": ["hero", "tall", "far"]},
        "fishing_boat": {"tags": ["hero"]},
        "neon_sign": {"tags": ["tall"]},
    },
    "salt": {
        "salt_crystal": {"tags": ["scatter"]},
        "ufo": {"tags": ["hero"]},
        "rover": {"tags": []},
        "flag": {"tags": ["scatter", "tall"]},
    },
}
