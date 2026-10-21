"""Pirate Bay courses as data (MIT License, see ../LICENSE).

Plain literals, no logic: ``COURSES`` is a list of dicts in the shape documented in ``layout`` (``start``,
``pieces``, ``hero``, ``props``, ``mechanics``, ``obstacles``, ``cycle``). Positions are Blender metres (x, y),
yaws and headings are degrees, a positive curve angle turns left. ``hero`` is ``{"at": (x, y), "yaw": degrees}``
(the kit's hero prop of the world, optionally ``"name"``), ``props`` are ``(name, (x, y), yaw, scale)``.

The game camera looks along (-0.58, 0.81) in plan, so the far side of a lane is its upper left and tall props
stand there (or well to the side), never between the camera and the lane.
"""

COURSES = [
    {"number": 1, "par": 2, "name": "Anchor Cove",
     "start": {"at": (0.0, 0.0, 0.0), "heading": 0},
     "pieces": [("tee", {"length": 1.0}),
                ("straight", {"length": 3.6}),
                ("curve", {"radius": 3.0, "angle": 90}),
                ("straight", {"length": 1.0}),
                ("green", {"length": 3.6, "width": 3.44, "cup": (2.2, 0.0)})],
     "hero": {"at": (1.4, 6.2), "yaw": -90},
     "props": [("barrel", (5.6, 9.0), 0, 1.0), ("barrel", (6.3, 8.7), 0, 0.9), ("chest", (8.0, 8.9), 180, 1.0),
               ("cannon", (-1.7, 1.4), 40, 1.0), ("barrel", (-2.5, 0.5), 0, 0.9),
               ("pebble", (3.4, 2.4), 0, 1.0), ("pebble", (3.9, 2.1), 0, 0.8), ("pebble", (3.0, 2.0), 0, 0.9),
               ("xmark", (6.2, -1.6), 20, 1.0), ("rock", (4.4, -1.8), 0, 1.0), ("rock", (5.0, -2.0), 80, 0.7), ("crab", (3.0, -1.7), 30, 1.0),
               ("crab", (9.6, 1.0), 200, 0.9), ("buoy", (9.4, 2.0), 0, 1.0), ("starfish", (7.4, -1.7), 0, 1.0), ("shell", (2.0, -1.8), 0, 1.0),
               ("shell", (7.0, -1.9), 70, 0.9), ("shell", (0.6, -1.7), 140, 1.0)],
     "mechanics": [], "obstacles": [], "cycle": None},
]
