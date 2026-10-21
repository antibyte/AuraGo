"""Pirate Bay courses as data (MIT License, see ../LICENSE).

Plain literals, no logic: ``COURSES`` is a list of dicts in the shape documented in ``layout`` (``start``,
``pieces``, ``hero``, ``props``, ``mechanics``, ``obstacles``, ``cycle``). Positions are Blender metres (x, y),
yaws and headings are degrees, a positive curve angle turns left. ``hero`` is ``{"at": (x, y), "yaw": degrees}``
(the kit's hero prop of the world, optionally ``"name"``), ``props`` are ``(name, (x, y), yaw, scale)``.

Course 1 is a placeholder with probe content (tee, straight, curve, bank, hill, green and the ship) until the
real Pirate Bay courses replace it.
"""

COURSES = [
    {"number": 1, "par": 3, "name": "Anchor Cove",
     "start": {"at": (0.0, 0.0, 0.0), "heading": 90},
     "pieces": [("tee", {"length": 1.0}),
                ("straight", {"length": 3.0}),
                ("curve", {"radius": 3.0, "angle": 90}),
                ("straight", {"length": 1.0}),
                ("bank", {"radius": 4.0, "angle": -60, "bank": 20}),
                ("hill", {"length": 3.0, "height": 0.2}),
                ("green", {"length": 3.4, "width": 3.44, "cup": (1.9, 0.3)})],
     "hero": {"at": (4.6, 9.0), "yaw": 40},
     "props": [("palm", (-6.2, 3.2), 0, 1.0), ("barrel", (2.6, 2.4), 0, 1.0), ("chest", (-3.0, 11.6), 120, 1.0)],
     "mechanics": [], "obstacles": [], "cycle": None},
]
