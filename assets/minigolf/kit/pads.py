"""Raster pads of the track kit: tee, green, plaza, fork and join (MIT License, see ../LICENSE).

A pad is a rectangular raster in the pad frame (origin = centre of the entry section, ``x``
forward, ``y`` left). Columns are ``SPACING`` apart, rows at most ``PAD_STEP``; the width must be a
whole number of columns that leaves an even number of columns beside the lane. The entry port is the
row of the lane width on the back edge: its raster points ARE the point tuples of the incoming
section (so the seam needs no welding). An exit port produces a new section from the pad's own raster
points. Walls run along every perimeter edge except the ports; holes (cup, pools) have no walls.

``PADS`` is the registry the layout dispatches on: ``pad(prev, cursor, **params) -> (pad, exit_section | None, cursor)``.
"""

import math

from . import cup_hole
from .track import CUP_RADIUS, LANE_WIDTH, PAD_STEP, ROWS, SPACING, Cursor, Pad, Section, lane_style

CUP_REACH = CUP_RADIUS + SPACING   # raster cells touching this circle have no floor; a zipper joins the rest to the exact hole


def _columns(width):
    count = round(width / SPACING)
    if abs(width - count * SPACING) > 1e-6:
        raise ValueError("width must be a multiple of the row spacing %.4f m (got %g)" % (SPACING, width))
    if count < ROWS or (count - ROWS) % 2:
        raise ValueError("width must be the lane width plus a whole number of column pairs (got %d columns)" % count)
    return count


def _inside(polygon, x, y):
    inside = False
    for k in range(len(polygon)):
        (x0, y0), (x1, y1) = polygon[k], polygon[(k + 1) % len(polygon)]
        if (y0 > y) != (y1 > y) and x < x0 + (y - y0) * (x1 - x0) / (y1 - y0):
            inside = not inside
    return inside


def _rect_distance(x0, x1, y0, y1, px, py):
    return math.hypot(max(x0 - px, 0.0, px - x1), max(y0 - py, 0.0, py - y1))


def _bump(bumps):
    def z_of(x, y):
        z = 0.0
        for bx, by, radius, height in bumps:
            d = math.hypot(x - bx, y - by)
            if d < radius:
                z += height * 0.5 * (1 + math.cos(math.pi * d / radius))
        return z
    return z_of


def _pad(kind, prev, cursor, length, width, st, z_of=None, cup=None, pools=(), exit_shifts=(), mark=None, entries=None, tol=1e-6):
    """Build a raster pad. ``entries`` = ``[(section, first_column)]`` replaces the single centred entry of ``prev``
    (a join has two); ``tol`` is the distance an entry's points may lie off the raster before it is rejected.

    The hole of ``cup`` is exact: cells that touch the circle of radius ``CUP_REACH`` are removed, and the gap
    to the polygon of radius ``CUP_RADIUS`` is filled with a zipper of triangles (``pad.tris``, ``pad.cup_ring``).
    """
    if not isinstance(length, (int, float)) or length < 0.5:
        raise ValueError("length must be at least 0.5 m (got %r)" % (length,))
    ncols = _columns(width)
    if entries is None and prev is not None:
        entries = [(prev, (ncols - ROWS) // 2)]
    if any(not section.flat for section, _ in entries or ()):
        raise ValueError("a pad needs a flat entry: the piece before it must end level")
    z_of = z_of or (lambda x, y: 0.0)
    ox, oy, oz, heading = cursor.x, cursor.y, cursor.z, cursor.heading
    fx, fy = math.cos(heading), math.sin(heading)
    lx, ly = -fy, fx
    nx = max(1, math.ceil(length / PAD_STEP - 1e-9))
    xs = [length * i / nx for i in range(nx)] + [length]
    ys = [(ncols / 2 - j) * SPACING for j in range(ncols + 1)]
    if any(abs(z_of(0.0, y)) > 1e-12 for y in ys):
        raise ValueError("the pad surface must be level along the entry edge")
    points = [[(ox + fx * x + lx * y, oy + fy * x + ly * y, oz + z_of(x, y)) for y in ys] for x in xs]
    entry_cols = []
    ports = []
    for section, c0 in entries or ():
        entry_cols.append((c0, c0 + ROWS))
        for k in range(ROWS + 1):
            if math.dist(points[0][c0 + k], section.points[k]) > tol:
                raise ValueError("entry section does not match the pad frame")
            points[0][c0 + k] = section.points[k]       # the very same tuple objects
        ports.append({"side": "entry", "cols": entry_cols[-1], "section": section})
    # holes: raster cells without floor
    holes, removed, cup_cells = [], set(), None
    cell_centre = lambda i, j: ((xs[i] + xs[i + 1]) / 2, (ys[j] + ys[j + 1]) / 2)
    world_xy = lambda x, y: (ox + fx * x + lx * y, oy + fy * x + ly * y)
    world_point = lambda x, y: world_xy(x, y) + (oz + z_of(x, y),)
    if cup is not None:
        cx, cy = cup
        need_x, need_y = CUP_REACH + 2.0 * (length / nx), CUP_REACH + 2.0 * SPACING
        if not (need_x <= cx <= length - need_x and abs(cy) <= width / 2 - need_y):
            raise ValueError("cup must be at least %.2f m from the front and back edge and %.2f m from the sides of the pad "
                             "(pad frame x=%g, y=%g)" % (need_x, need_y, cx, cy))
        for i in range(nx):
            for j in range(ncols):
                if _rect_distance(xs[i], xs[i + 1], ys[j + 1], ys[j], cx, cy) < CUP_REACH:
                    removed.add((i, j))
        cup_cells = set(removed)
        cup = world_point(cx, cy)
        holes.append({"kind": "cup", "center": cup, "radius": CUP_RADIUS})
    for pool in pools:
        polygon = [tuple(p) for p in pool["polygon"]]
        if len(polygon) < 3 or pool.get("kind") not in ("water", "void"):
            raise ValueError("a pool needs a polygon of at least 3 points and kind 'water' or 'void'")
        for i in range(nx):
            for j in range(ncols):
                if _inside(polygon, *cell_centre(i, j)):
                    removed.add((i, j))
        world = [(ox + fx * x + lx * y, oy + fy * x + ly * y) for x, y in polygon]
        level = oz + z_of(sum(p[0] for p in polygon) / len(polygon), sum(p[1] for p in polygon) / len(polygon))
        holes.append({"kind": pool["kind"], "polygon": world, "z": level,
                      "min": (min(p[0] for p in world), min(p[1] for p in world)), "max": (max(p[0] for p in world), max(p[1] for p in world))})
    if any(i in (0, nx - 1) or j in (0, ncols - 1) for i, j in removed):
        raise ValueError("a hole or pool reaches the pad edge: keep it at least one raster cell inside")
    tris, cup_ring = [], None
    if cup_cells:
        tris, cup_ring = cup_hole.fill(cup_cells, lambda i, j: points[i][j], lambda i, j: (xs[i], ys[j]), cx, cy, world_point)
    faces = [(i, j) for i in range(nx) for j in range(ncols) if (i, j) not in removed]
    # exit ports (a fork has two); the cursor continues from the first
    exit_cols, exit_sections, new_cursor = [], [], cursor
    for shift in exit_shifts:
        e0 = (ncols - ROWS) // 2 + shift
        if e0 < 0 or e0 + ROWS > ncols:
            raise ValueError("exit_offset puts the exit outside the pad")
        row = points[nx][e0:e0 + ROWS + 1]
        if max(p[2] for p in row) - min(p[2] for p in row) > 1e-9:
            raise ValueError("the pad surface must be level across the exit")
        exit_cols.append((e0, e0 + ROWS))
        exit_sections.append(Section(list(row), row[ROWS // 2], heading, walls=st["walls"], surface=st["surface"], look=st["look"],
                                     grounded=st["grounded"], flat=True))
        ports.append({"side": "exit", "cols": exit_cols[-1], "section": exit_sections[-1]})
        if len(exit_sections) == 1:
            new_cursor = Cursor(row[ROWS // 2][0], row[ROWS // 2][1], row[ROWS // 2][2], heading)
    # perimeter walls without the ports
    walls = []
    if st["walls"] != (False, False):
        left_wall, right_wall = st["walls"]
        out_l, out_r, out_f, out_b = (lx, ly, 0.0), (-lx, -ly, 0.0), (fx, fy, 0.0), (-fx, -fy, 0.0)
        for i in range(nx):
            if left_wall:
                walls.append((points[i][0], points[i + 1][0], out_l))
            if right_wall:
                walls.append((points[i][ncols], points[i + 1][ncols], out_r))
        for j in range(ncols):
            if not any(c0 <= j < c1 for c0, c1 in entry_cols):
                walls.append((points[0][j], points[0][j + 1], out_b))
            if not any(c0 <= j < c1 for c0, c1 in exit_cols):
                walls.append((points[nx][j], points[nx][j + 1], out_f))
    pad = Pad(kind, {"nx": nx, "ny": ncols, "points": points, "xs": xs, "ys": ys}, faces, st["surface"], st["look"], walls, ports,
              (ox, oy, oz, heading), (length, width), cup=cup, holes=holes, mark=mark, tris=tris, cup_ring=cup_ring,
              grounded=st["grounded"])
    return pad, exit_sections, new_cursor


def _single(built):
    """``(pad, exit_sections, cursor)`` of ``_pad`` as ``(pad, exit_section | None, cursor)`` for one-exit pads."""
    pad, exits, cursor = built
    return pad, (exits[0] if exits else None), cursor


def tee(prev, cursor, length=1.0, width=LANE_WIDTH, **style):
    """Start pad: the cursor is the middle of its back edge; the tee mark sits in the middle. Has an exit, no entry."""
    if prev is not None:
        raise ValueError("the tee must be the first piece")
    st = lane_style(style)
    mark = (cursor.x + math.cos(cursor.heading) * length / 2, cursor.y + math.sin(cursor.heading) * length / 2, cursor.z)
    return _single(_pad("tee", None, cursor, length, width, st, exit_shifts=(0,), mark=mark))


def green(prev, cursor, length, width, cup, slope=0.0, contour=(), **style):
    """Putting green with the hole at ``cup = (x, y)`` in the pad frame (x forward, y left).

    ``slope`` is the fall line in degrees along the lane (positive climbs), ``contour`` a list of smooth
    bumps ``(x, y, radius, height)``. Has no exit.
    """
    if prev is None:
        raise ValueError("a green needs a piece before it")
    if abs(slope) > 12:
        raise ValueError("slope must not exceed 12 degrees")
    bumps = [tuple(b) for b in contour]
    tilt, bump = math.tan(math.radians(slope)), _bump(bumps)
    st = lane_style(style)
    return _single(_pad("green", prev, cursor, length, width, st, z_of=lambda x, y: x * tilt + bump(x, y), cup=tuple(cup)))


def plaza(prev, cursor, length, width, pools=(), exit_offset=0, **style):
    """Wide flat area; ``pools`` are polygons ``{"polygon": [(x, y), ...], "kind": "water"|"void"}`` in the pad
    frame (cells with their centre inside have no floor, the rim has no wall). The exit is shifted by
    ``exit_offset`` columns (0 = centred)."""
    if prev is None:
        raise ValueError("a plaza needs a piece before it")
    if not isinstance(exit_offset, int):
        raise ValueError("exit_offset must be a whole number of columns")
    return _single(_pad("plaza", prev, cursor, length, width, lane_style(style), pools=list(pools), exit_shifts=(exit_offset,)))


MIN_SPREAD_COLUMNS = 16     # two lanes plus the rails between them need at least 2.29 m between the exit centres
MIN_FORK_LENGTH = 1.5
JOIN_TOLERANCE = 1e-3       # heights and forward coordinates of the branch ends
JOIN_ANGLE = math.radians(0.01)


def _spread_columns(spread):
    columns = round(spread / SPACING)
    if abs(spread - columns * SPACING) > 1e-6 or columns % 2:
        raise ValueError("spread must be an even multiple of the row spacing %.4f m (got %g)" % (SPACING, spread))
    if columns < MIN_SPREAD_COLUMNS:
        raise ValueError("spread must be at least %.2f m (got %g)" % (MIN_SPREAD_COLUMNS * SPACING, spread))
    return columns


def fork(prev, cursor, length, spread, width=None, **style):
    """Splitting area with two exits ``spread`` m apart (an even multiple of the row spacing).

    Returns ``(pad, [left_exit, right_exit], cursor)``; the layout builds the two branches from the exits and
    closes the fork with ``join``. The default ``width`` makes the outer rails continue the branch rails.
    """
    if prev is None:
        raise ValueError("a fork needs a piece before it")
    if not isinstance(length, (int, float)) or length < MIN_FORK_LENGTH:
        raise ValueError("a fork needs a length of at least %.1f m (got %r)" % (MIN_FORK_LENGTH, length))
    columns = _spread_columns(spread)
    width = (ROWS + columns) * SPACING if width is None else width
    if round(width / SPACING) < ROWS + columns:
        raise ValueError("width %g m is too narrow for two exits %g m apart" % (width, spread))
    return _pad("fork", prev, cursor, length, width, lane_style(style), exit_shifts=(-columns // 2, columns // 2))


def join(left, right, length, exit_offset=0, **style):
    """Merging area built from the end sections of the left and right branch of a fork.

    Both ends must be flat, point the same way (0.01 degrees), have the same height and forward coordinate
    (1 mm) and lie an even multiple of the row spacing apart. The back row is the left section, the raster
    points between the lanes and the right section; the exit is centred or shifted by ``exit_offset`` columns.
    Returns ``(pad, exit_section, cursor)``.
    """
    if not isinstance(length, (int, float)) or length < MIN_FORK_LENGTH:
        raise ValueError("the join needs a length of at least %.1f m (got %r)" % (MIN_FORK_LENGTH, length))
    if not isinstance(exit_offset, int):
        raise ValueError("exit_offset must be a whole number of columns")
    if not (left.flat and right.flat):
        raise ValueError("both branches must end level")
    if abs(left.heading - right.heading) > JOIN_ANGLE:
        raise ValueError("the branches end in different directions (%.3f and %.3f degrees)" % (math.degrees(left.heading), math.degrees(right.heading)))
    (lx0, ly0, lz0), (rx0, ry0, rz0) = left.center, right.center
    if abs(lz0 - rz0) > JOIN_TOLERANCE:
        raise ValueError("the branches end at different heights (%.1f mm apart)" % (abs(lz0 - rz0) * 1000))
    heading = left.heading
    fx, fy = math.cos(heading), math.sin(heading)
    dx, dy = lx0 - rx0, ly0 - ry0
    if abs(dx * fx + dy * fy) > JOIN_TOLERANCE:
        raise ValueError("the branches end at different distances along the route (%.1f mm apart)" % (abs(dx * fx + dy * fy) * 1000))
    lateral = -dx * fy + dy * fx
    if lateral < 0:
        raise ValueError("the left branch ends to the right of the right branch")
    columns = round(lateral / SPACING)
    if abs(lateral - columns * SPACING) > JOIN_TOLERANCE:
        raise ValueError("the branches end %.4f m apart, not a multiple of the row spacing %.4f m" % (lateral, SPACING))
    if columns % 2 or columns < MIN_SPREAD_COLUMNS:
        raise ValueError("the branches must end an even number of columns (at least %d) apart (got %d)" % (MIN_SPREAD_COLUMNS, columns))
    origin = Cursor((lx0 + rx0) / 2, (ly0 + ry0) / 2, lz0, heading)
    return _single(_pad("join", None, origin, length, (ROWS + columns) * SPACING, lane_style(style), exit_shifts=(exit_offset,),
                        entries=[(left, 0), (right, columns)], tol=4 * JOIN_TOLERANCE))


PADS = {"tee": tee, "green": green, "plaza": plaza}
