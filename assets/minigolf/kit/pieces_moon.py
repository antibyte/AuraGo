"""Moon lane pieces of the track kit: half-pipe and loop (MIT License, see ../LICENSE).

Same signature as the pieces in ``pieces``: ``piece(prev, cursor, **params) -> (sections, cursor, extras)``.
Both pieces start from a flat, aligned ``prev`` section. Importing this module registers them in
``pieces.PIECES``.

``halfpipe``: a U-shaped cross-section (quarter circle, flat floor, quarter circle) whose ``ROWS + 1`` points
lie at equal arc-length steps on the profile. The lane blends from the flat section into the U and back
over ``blend`` (at least 1.5 m) with ``point = lerp(flat, U, smoothstep(s / blend))``. The centreline is a
straight line or an arc; the rim walls stand vertically on the lips (``wall_up`` = world up) and lean
horizontally outwards.

``loop``: a vertical circle with a lateral offset (a helix-like sweep), so the exit lane lies beside the
entry lane. With ``f`` the heading, ``up`` world up and ``side`` the left vector the centreline is
``p(t) = start + f*R*sin(t) + up*R*(1 - cos(t)) + side*offset*(3s^2 - 2s^3)``, ``s = t / 2pi``. A section sits
across the tangent ``T``: ``across = side - (side.T) T`` (normalised), floor normal ``n = T x across`` (up at
the start, pointing into the circle at the top). The walls stand along ``n``.
"""

import math

from . import pieces
from .track import GROUNDED_MAX, LANE_WIDTH, MAX_STEP, ROWS, SPACING, WALL_THICKNESS, Cursor, Section, lane_style, make_section

MIN_BLEND = 1.5
MIN_PIPE_RADIUS = 0.5
MIN_PIPE_DEPTH = 0.3
PIPE_MARGIN = 0.5          # a curved pipe's path radius exceeds the lip half span by this
LOOP_RADII = (1.2, 1.5)
LOOP_MIN_OFFSET = LANE_WIDTH + 2 * WALL_THICKNESS + 0.2
LOOP_MIN_SECTIONS = 48
LOOP_SECTIONS = 72         # default: one section per 0.12 m of centreline at R = 1.3; 40 collision triangles per section
UP = (0.0, 0.0, 1.0)


def _smooth(t):
    t = max(0.0, min(1.0, t))
    return t * t * (3.0 - 2.0 * t)


def _require_flat_start(prev, cursor, name):
    if prev is None or not prev.flat:
        raise ValueError("a %s needs a flat section before it" % name)
    if abs(prev.heading - cursor.heading) > 1e-9 or math.dist(prev.center, (cursor.x, cursor.y, cursor.z)) > 1e-6:
        raise ValueError("a %s must continue straight from the section before it" % name)


def _flat_rows():
    return [((ROWS / 2 - i) * SPACING, 0.0) for i in range(ROWS + 1)]


def _u_rows(width, radius, depth):
    """``ROWS + 1`` ``(lateral, height)`` points at equal arc-length steps on the U profile, from the left lip.

    The mirrored half is built by symmetry, so the middle point is exactly 0 and both lips match to the bit.
    """
    phi = math.acos(1.0 - depth / radius)          # angle of the lip on the quarter circle (90 degrees when depth == radius)
    arc = radius * phi
    total = width + 2 * arc
    rows = [None] * (ROWS + 1)
    for i in range(ROWS // 2 + 1):
        s = total * i / ROWS
        if s <= arc:
            a = phi - s / radius
            rows[i] = (width / 2 + radius * math.sin(a), radius * (1.0 - math.cos(a)))
        else:
            rows[i] = (width / 2 - (s - arc), 0.0)
    rows[ROWS // 2] = (0.0, 0.0)
    for i in range(ROWS // 2 + 1, ROWS + 1):
        lateral, height = rows[ROWS - i]
        rows[i] = (-lateral, height)
    return rows


def _centreline(cursor, length, radius_path, angle_deg):
    """``(total length, at(s) -> (x, y, heading))`` of a straight or circular centreline."""
    if length is not None:
        fx, fy = cursor.forward()
        return length, lambda s: (cursor.x + fx * s, cursor.y + fy * s, cursor.heading)
    angle = math.radians(angle_deg)
    sgn = 1.0 if angle > 0 else -1.0
    fx, fy = cursor.forward()
    lx, ly = cursor.left()
    cx, cy = cursor.x + sgn * radius_path * lx, cursor.y + sgn * radius_path * ly
    rx, ry = -sgn * radius_path * lx, -sgn * radius_path * ly
    tx, ty = sgn * radius_path * fx, sgn * radius_path * fy

    def at(s):
        theta = angle * s / (abs(angle) * radius_path)
        return (cx + rx * math.cos(theta) + tx * math.sin(theta), cy + ry * math.cos(theta) + ty * math.sin(theta), cursor.heading + theta)
    return abs(angle) * radius_path, at


def halfpipe(prev, cursor, width=1.2, radius=0.9, depth=None, length=None, radius_path=None, angle=None, blend=MIN_BLEND, **style):
    """U-shaped half-pipe.

    ``width`` is the flat floor, ``radius`` the quarter-circle radius, ``depth`` the lip height (default
    ``radius`` = vertical lips; a smaller value stops the arc earlier). The path is straight (``length`` m) or
    an arc (``radius_path`` m, ``angle`` degrees); ``blend`` (>= 1.5 m) is the flat-to-U transition at each end.
    """
    _require_flat_start(prev, cursor, "half-pipe")
    st = lane_style(style)
    depth = radius if depth is None else depth
    for name, value in (("width", width), ("radius", radius), ("depth", depth), ("blend", blend)):
        if not isinstance(value, (int, float)) or value <= 0:
            raise ValueError("%s must be a positive number (got %r)" % (name, value))
    if radius < MIN_PIPE_RADIUS or depth < MIN_PIPE_DEPTH or depth > radius:
        raise ValueError("radius must be at least %.1f m and depth between %.1f m and the radius" % (MIN_PIPE_RADIUS, MIN_PIPE_DEPTH))
    if blend < MIN_BLEND:
        raise ValueError("blend must be at least %.1f m" % MIN_BLEND)
    if (length is None) == (radius_path is None or angle is None):
        raise ValueError("give either length or radius_path with angle")
    rows = _u_rows(width, radius, depth)
    half_span = rows[0][0]
    if length is None:
        if not isinstance(radius_path, (int, float)) or radius_path < pieces.MIN_RADIUS or radius_path < half_span + PIPE_MARGIN:
            raise ValueError("radius_path must be at least %.1f m and %.1f m wider than the half-pipe" % (pieces.MIN_RADIUS, PIPE_MARGIN))
        if not isinstance(angle, (int, float)) or angle == 0 or abs(angle) > 180:
            raise ValueError("angle must be a non-zero number of degrees up to 180 (got %r)" % (angle,))
    elif not isinstance(length, (int, float)) or length <= 0:
        raise ValueError("length must be a positive number (got %r)" % (length,))
    total, at = _centreline(cursor, length, radius_path, angle)
    if total < 2 * blend + 0.6:
        raise ValueError("the half-pipe is too short: %.1f m, needs at least %.1f m for a blend of %.1f m at each end" % (total, 2 * blend + 0.6, blend))
    flat = _flat_rows()
    n = max(2, math.ceil(total / MAX_STEP - 1e-9))
    wall_up = (UP, UP)
    sections = []
    for k in range(1, n + 1):
        x, y, heading = at(total * k / n)
        if k == n:
            sections.append(make_section((x, y, cursor.z), heading, style=st))
            continue
        s = total * k / n
        t = _smooth(min(s, total - s) / blend)
        lx, ly = -math.sin(heading), math.cos(heading)
        points = []
        for (fa, fh), (ua, uh) in zip(flat, rows):
            a, h = fa + (ua - fa) * t, fh + (uh - fh) * t
            points.append((x + lx * a, y + ly * a, cursor.z + h))
        sections.append(Section(points, (x, y, cursor.z), heading, normal=UP, walls=st["walls"], wall_up=wall_up, surface=st["surface"],
                                look=st["look"], grounded=st["grounded"], flat=False))
    last = sections[-1]
    return sections, Cursor(last.center[0], last.center[1], last.center[2], last.heading), {}


def _unit(v):
    length = math.sqrt(v[0] * v[0] + v[1] * v[1] + v[2] * v[2])
    return (v[0] / length, v[1] / length, v[2] / length)


def _cross(a, b):
    return (a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0])


def loop(prev, cursor, radius=1.3, offset=2.2, sections=None, **style):
    """Vertical loop of ``radius`` m (1.2-1.5) whose exit lane lies ``offset`` m to the left (negative: right).

    ``offset`` must be at least a lane width plus two rail thicknesses plus 0.2 m, so the exit lane clears the
    entry lane. ``sections`` (default 72, at least 48) is the number of cross-sections of the whole circle.
    """
    _require_flat_start(prev, cursor, "loop")
    st = lane_style(style)
    if not isinstance(radius, (int, float)) or not LOOP_RADII[0] <= radius <= LOOP_RADII[1]:
        raise ValueError("radius must be between %g and %g m (got %r)" % (LOOP_RADII + (radius,)))
    if not isinstance(offset, (int, float)) or abs(offset) < LOOP_MIN_OFFSET - 1e-9:
        raise ValueError("offset must be at least %.2f m to either side (got %r)" % (LOOP_MIN_OFFSET, offset))
    n = LOOP_SECTIONS if sections is None else sections
    if not isinstance(n, int) or n < LOOP_MIN_SECTIONS:
        raise ValueError("sections must be a whole number of at least %d (got %r)" % (LOOP_MIN_SECTIONS, sections))
    fx, fy = cursor.forward()
    lx, ly = cursor.left()
    f, side, up = (fx, fy, 0.0), (lx, ly, 0.0), UP
    out = []
    for k in range(1, n + 1):
        if k == n:
            out.append(make_section((cursor.x + side[0] * offset, cursor.y + side[1] * offset, cursor.z), cursor.heading, style=st))
            continue
        theta = 2.0 * math.pi * k / n
        s = k / n
        blend, dblend = 3 * s * s - 2 * s ** 3, 6 * s * (1 - s) / (2.0 * math.pi)
        sin_t, cos_t = math.sin(theta), math.cos(theta)
        p = tuple((cursor.x, cursor.y, cursor.z)[i] + f[i] * radius * sin_t + up[i] * radius * (1.0 - cos_t) + side[i] * offset * blend
                  for i in range(3))
        tangent = _unit(tuple(f[i] * radius * cos_t + up[i] * radius * sin_t + side[i] * offset * dblend for i in range(3)))
        along = sum(side[i] * tangent[i] for i in range(3))
        across = _unit(tuple(side[i] - along * tangent[i] for i in range(3)))
        normal = _unit(_cross(tangent, across))
        points = [tuple(p[j] + across[j] * (ROWS / 2 - i) * SPACING for j in range(3)) for i in range(ROWS + 1)]
        grounded = st["grounded"] and p[2] - cursor.z <= GROUNDED_MAX
        out.append(Section(points, p, cursor.heading, normal=normal, walls=st["walls"], wall_up=(normal, normal), surface=st["surface"],
                           look=st["look"], grounded=grounded, flat=False))
    last = out[-1]
    return out, Cursor(last.center[0], last.center[1], last.center[2], last.heading), {}


PIECES = {"halfpipe": halfpipe, "loop": loop}
pieces.PIECES.update(PIECES)
