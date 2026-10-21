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
    {"number": 14, "par": 3, "name": "Boost Lane",
     "start": {"at": (0.0, 0.0, 0.0), "heading": 0},
     "pieces": [("tee", {"length": 1.0, "look": "metal"}),
                ("anchor", {"name": "run"}),
                ("straight", {"length": 2.0}),
                ("bank", {"radius": 2.8, "angle": 90, "bank": 20}),
                ("straight", {"length": 3.6, "surface": "sand"}),
                ("anchor", {"name": "plaza"}),
                ("plaza", {"length": 5.4, "width": 36 * 0.14333333333333333, "look": "metal", "exit_offset": 10,
                           "pools": [{"kind": "void", "polygon": [(1.2, -1.6), (3.2, -1.6), (3.2, 0.2), (1.2, 0.2)]}]}),
                ("curve", {"radius": 2.2, "angle": -90}),
                ("straight", {"length": 0.6}),
                ("green", {"length": 3.4, "width": 3.44, "cup": (2.0, 0.0)})],
     "hero": None,
     "props": [("dishbase", (0.6, 10.4), 20, 1.0), ("rover", (0.8, 5.4), 150, 1.0)],
     "mechanics": [{"id": "boost-a", "kind": "boost", "anchor": "run", "offset": (1.0, 0.0)},
                   {"id": "boost-b", "kind": "boost", "anchor": "plaza", "offset": (4.3, -0.4), "heading": -15, "length": 1.2},
                   {"id": "pylon-a", "kind": "bumper", "anchor": "plaza", "offset": (2.3, 1.5)},
                   {"id": "pylon-b", "kind": "bumper", "anchor": "plaza", "offset": (4.5, 1.4)}],
     "obstacles": [], "cycle": None},
]
