"""AuraGo mini golf procedural kit (MIT License, see ../LICENSE).

Reusable Blender 5.2 modules for the stylized diorama look: palettes, soft
shape builders, vertex-colour gradients, baked ambient occlusion, per-world
props and lane sections. Everything is deterministic: every random choice goes
through a seeded ``random.Random`` and no iteration depends on dict or set
ordering that is not insertion ordered.

Modules import ``bpy`` and are meant to run under ``blender --background
--python``; see ``probe.py`` for the entry point.
"""

WORLDS = ("pirate", "temple", "moon")
