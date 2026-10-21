"""Track kit data types and shared constants (MIT License, see ../LICENSE).

Plain Python, no ``bpy``: pieces, pads and the layout can be tested outside
Blender. Coordinates are Blender X/Y/Z (metres, Z up); ``heading`` is in radians,
0 = +X, positive turns left (towards +Y).

A lane is described by ``Section`` cross-sections of ``ROWS + 1`` points from the
left edge to the right edge. A ``Strip`` is the sequence of sections of one piece;
its first section is the very same object as the last section of the previous
piece, so seam vertices are bit-identical and need no welding. A ``Pad`` is a
raster area (tee, green, plaza) whose entry row reuses the point tuples of the
incoming section and whose exit sections are made from its own raster points.
"""

import math

ROWS = 12
LANE_WIDTH = 1.72
SPACING = LANE_WIDTH / ROWS
WALL_HEIGHT = 0.24
WALL_DEPTH = 0.045       # rail reaches this far below the floor edge
WALL_THICKNESS = 0.14
CUP_RADIUS = 0.32
BALL_RADIUS = 0.16
MAX_STEP = 0.30          # longest cross-section distance in bends and height changes
GROUNDED_MAX = 0.30
HOLE_MARGIN = 0.13       # raster cells whose centre is closer than CUP_RADIUS + this have no floor
PAD_STEP = 0.25          # longest pad raster cell along the lane

SURFACES = ("grass", "sand")
LOOKS = ("felt", "sand", "deck", "slab", "metal")
RAIL_SURFACES = {"pirate": "wood", "temple": "stone", "moon": "metal"}
GRAVITY = {"pirate": 9.81, "temple": 9.81, "moon": 4.0}
WALL_MODES = {"both": (True, True), "left": (True, False), "right": (False, True), "none": (False, False)}
STYLE_KEYS = ("surface", "look", "walls", "grounded")


class Cursor:
    """End state of a piece: position and heading (radians)."""

    __slots__ = ("x", "y", "z", "heading")

    def __init__(self, x=0.0, y=0.0, z=0.0, heading=0.0):
        self.x, self.y, self.z, self.heading = x, y, z, heading

    def forward(self):
        return (math.cos(self.heading), math.sin(self.heading))

    def left(self):
        return (-math.sin(self.heading), math.cos(self.heading))

    @classmethod
    def at(cls, section):
        """The cursor standing on the centre of ``section``."""
        return cls(section.center[0], section.center[1], section.center[2], section.heading)


class Section:
    """Cross-section: ``ROWS + 1`` points from left to right, floor normal and rail flags.

    ``walls`` is ``(left, right)``: a rail runs along a side between two sections
    when the *later* section has the flag (it belongs to the piece that follows the
    seam). ``wall_up`` is ``(left_vec, right_vec)`` or ``None`` (then ``normal``).
    ``arc`` is the distance along the main route, set by the layout.
    """

    __slots__ = ("points", "center", "heading", "normal", "walls", "wall_up", "surface", "look", "grounded", "flat", "arc")

    def __init__(self, points, center, heading, normal=(0.0, 0.0, 1.0), walls=(True, True), wall_up=None,
                 surface="grass", look="felt", grounded=True, flat=True):
        self.points, self.center, self.heading, self.normal = points, center, heading, normal
        self.walls, self.wall_up, self.surface, self.look = walls, wall_up, surface, look
        self.grounded, self.flat, self.arc = grounded, flat, None


class Strip:
    """The sections of one piece; ``sections[0]`` is the last section of the previous piece."""

    __slots__ = ("sections", "kind", "cover", "deck")

    def __init__(self, sections, kind, cover=None, deck=None):
        self.sections, self.kind, self.cover, self.deck = sections, kind, cover, deck


class Pad:
    """Raster area. ``grid`` = ``{"nx", "ny", "points"}`` with ``points[i][j]`` (row ``i`` forward,
    column ``j`` from the left); ``faces`` are the kept cells ``(i, j)``; ``walls`` are perimeter
    edges ``(point_a, point_b, outward)`` without the ports; ``ports`` are ``{"side", "cols",
    "section"}``; ``cup`` is the world position of the hole, ``holes`` the hazard pools,
    ``mark`` the tee mark, ``frame`` = ``(x, y, z, heading)`` of the pad origin."""

    __slots__ = ("kind", "grid", "faces", "surface", "look", "walls", "ports", "cup", "holes", "mark", "frame", "arc0", "size")

    def __init__(self, kind, grid, faces, surface, look, walls, ports, frame, size, cup=None, holes=(), mark=None):
        self.kind, self.grid, self.faces, self.surface, self.look = kind, grid, faces, surface, look
        self.walls, self.ports, self.cup, self.holes, self.mark = walls, ports, cup, list(holes), mark
        self.frame, self.size, self.arc0 = frame, size, 0.0


class Track:
    """Everything of one course that the Blender build and the manifest need."""

    __slots__ = ("world", "number", "par", "strips", "pads", "tee", "cup", "path", "corridors", "hazards", "anchors",
                 "length", "mechanics", "obstacles", "cycle", "gravity")

    def __init__(self, world, number=0, par=0):
        self.world, self.number, self.par = world, number, par
        self.strips, self.pads, self.hazards = [], [], []
        self.tee = self.cup = None
        self.path, self.corridors, self.anchors = [], [], {}
        self.length = 0.0
        self.mechanics, self.obstacles, self.cycle = [], [], None
        self.gravity = GRAVITY[world]


def lane_style(style):
    """Validate the optional piece fields ``surface``, ``look``, ``walls``, ``grounded``; returns the resolved dict."""
    unknown = sorted(set(style) - set(STYLE_KEYS))
    if unknown:
        raise ValueError("unknown piece field(s): " + ", ".join(unknown))
    surface = style.get("surface", "grass")
    if surface not in SURFACES:
        raise ValueError("surface must be one of %s (got %r)" % (", ".join(SURFACES), surface))
    look = style.get("look", "sand" if surface == "sand" else "felt")
    if look not in LOOKS:
        raise ValueError("look must be one of %s (got %r)" % (", ".join(LOOKS), look))
    walls = style.get("walls", "both")
    if walls not in WALL_MODES:
        raise ValueError("walls must be one of %s (got %r)" % (", ".join(WALL_MODES), walls))
    return {"surface": surface, "look": look, "walls": WALL_MODES[walls], "grounded": bool(style.get("grounded", True))}


def make_section(center, heading, bank=0.0, width=LANE_WIDTH, z_profile=None, style=None):
    """Section across the lane at ``center`` (x, y, z) looking along ``heading``.

    Point ``i`` lies at ``center + left*a*cos(b) + up*(-a*sin(b))`` with ``a = (ROWS/2 - i) * width / ROWS``
    (``a`` is exactly 0 for the middle point). ``bank`` is the tilt ``b`` in radians, positive lowers the
    left edge (raises the right one). ``z_profile(a)`` adds a height per lateral offset. ``style`` is a
    resolved ``lane_style`` dict.
    """
    style = style or lane_style({})
    cx, cy, cz = center
    lx, ly = -math.sin(heading), math.cos(heading)
    cb, sb = math.cos(bank), math.sin(bank)
    spacing = width / ROWS
    points = []
    for i in range(ROWS + 1):
        a = (ROWS / 2 - i) * spacing
        dz = -a * sb
        if z_profile is not None:
            dz += z_profile(a)
        points.append((cx + lx * a * cb, cy + ly * a * cb, cz + dz))
    return Section(points, (cx, cy, cz), heading, normal=(lx * sb, ly * sb, cb), walls=style["walls"], surface=style["surface"],
                   look=style["look"], grounded=style["grounded"], flat=bank == 0.0 and z_profile is None)


def distance(a, b):
    """3D distance of two points."""
    return math.sqrt((a[0] - b[0]) ** 2 + (a[1] - b[1]) ** 2 + (a[2] - b[2]) ** 2)
