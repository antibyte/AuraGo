"""Moon Station courses as data (MIT License, see ../LICENSE).

Same shape as ``course_data_pirate``: ``COURSES`` is a list of dicts (``start``, ``pieces``, ``hero``, ``props``,
``mechanics``, ``obstacles``, ``cycle``). Positions are Blender metres (x, y), yaws and headings are degrees, a
positive curve angle turns left. ``hero`` is ``{"at": (x, y), "yaw": degrees}`` (default the habitat; its hatch
faces -Y at yaw 0), ``props`` are ``(name, (x, y), yaw, scale)``.

The game camera looks along (-0.58, 0.81) in plan, so the far side of a lane is its upper left; tall props (antenna,
tower, habitat) stand there or well to the side, low ones (crater, crystal, beacon) near the lanes. Gravity is 4.0 m/s^2
on every moon course; a shot rolls as far as on Earth but a jump flies 2.5 times as long.
"""

COURSES = [
    {"number": 13, "par": 2, "name": "Touchdown",
     "start": {"at": (0.0, 0.0, 0.0), "heading": 0},
     "pieces": [("tee", {"length": 1.0, "look": "metal"}),
                ("straight", {"length": 4.6}),
                ("hill", {"length": 4.6, "height": 0.42}),
                ("straight", {"length": 1.8}),
                ("green", {"length": 3.8, "width": 3.44, "cup": (2.4, 0.0)})],
     "hero": {"at": (8.4, 6.2), "yaw": 0},
     "props": [("pad", (2.0, 5.2), 0, 1.0), ("beacon", (-0.4, 1.7), 0, 1.0), ("beacon", (3.0, -1.7), 0, 1.0),
               ("dishbase", (15.2, 7.6), -25, 1.0)],
     "mechanics": [], "obstacles": [], "cycle": None},
]
