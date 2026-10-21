"""Jungle Temple courses as data (MIT License, see ../LICENSE).

Same shape as ``course_data_pirate``: plain literals, positions in Blender metres (x, y), yaws and headings in degrees,
a positive curve angle turns left, ``hero`` is ``{"at": (x, y), "yaw": degrees}`` (the stone head), ``props`` are
``(name, (x, y), yaw, scale)``. Temple props face -Y at yaw 0; the front points along ``(sin yaw, -cos yaw)``.

The game camera looks along (-0.58, 0.81) in plan, so the far side of a lane is its upper left (+y) and tall props
stand there (or well to the side), never between the camera and the lane.
"""

COURSES = [
    {"number": 7, "par": 3, "name": "Temple Steps",
     "start": {"at": (0.0, 0.0, 0.0), "heading": 0},
     "pieces": [("tee", {"length": 1.0}),
                ("straight", {"length": 2.4}),
                ("steps", {"count": 3, "drop": 0.28, "tread": 1.3}),
                ("straight", {"length": 1.2}),
                ("curve", {"radius": 3.0, "angle": -85}),
                ("straight", {"length": 0.8}),
                ("green", {"length": 3.2, "width": 3.44, "cup": (1.9, 0.0)})],
     "hero": {"at": (5.2, 4.6), "yaw": 30},
     "props": [("stairs", (10.3, 4.7), -20, 1.0), ("vines", (0.3, 4.5), 0, 1.0),
               ("torch", (0.6, 1.9), 0, 1.0), ("torch", (9.4, 1.9), 0, 1.0), ("torch", (14.4, -3.6), 0, 1.0), ("torch", (7.0, -1.9), 0, 0.8)],
     "mechanics": [], "obstacles": [], "cycle": None},
]
