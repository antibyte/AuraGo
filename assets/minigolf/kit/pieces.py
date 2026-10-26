"""Lane pieces of the track kit (MIT License, see ../LICENSE).

Every piece has the signature ``piece(prev, cursor, **params) -> (sections, cursor, extras)``.
``prev`` is the last section of the previous piece; the piece returns only its *new* sections
(the layout puts ``prev`` in front of them, so the seam points are the same objects). The
optional fields ``surface``, ``look``, ``walls``, ``grounded`` go into every new section.
``extras`` may carry ``hazard`` (a ``gap``) and ``break`` (the next strip starts with a fresh
section). Pieces check their own parameter ranges and raise ``ValueError``.

``PIECES`` is the registry the layout dispatches on; more modules (moon pieces, forks)
register themselves by updating it.
"""

import math

from .track import MAX_STEP, LANE_WIDTH, Cursor, lane_style, make_section

STRAIGHT_STEP = 0.62      # a straight needs a cross-section only about every stripe length
MIN_RADIUS = 2.0
MAX_BANK = 30.0
MAX_RAMP = 22.0           # degrees, plain ramps, hills and the body of a ramp with lip
MAX_LIP = 30.0
MAX_STEP_DROP = 0.35
MIN_TREAD = 0.5
MAX_GAP = 8.0
END_SLOPE_STEP, END_SLOPE_FLAT = 1e-4, 1e-3     # a profile ends level when its slope over the last 0.1 mm is below 0.1 %


def _positive(value, name):
    if not isinstance(value, (int, float)) or value <= 0:
        raise ValueError("%s must be a positive number (got %r)" % (name, value))


def _finish(sections, cursor):
    last = sections[-1]
    return sections, Cursor(last.center[0], last.center[1], last.center[2], last.heading)


def straight(prev, cursor, length, **style):
    """Straight lane of ``length`` metres."""
    _positive(length, "length")
    st = lane_style(style)
    n = max(1, math.ceil(length / STRAIGHT_STEP - 1e-9))
    fx, fy = cursor.forward()
    sections = []
    for k in range(1, n + 1):
        d = length * k / n
        sections.append(make_section((cursor.x + fx * d, cursor.y + fy * d, cursor.z), cursor.heading, style=st))
    return _finish(sections, cursor) + ({},)


def _arc(cursor, radius, angle_deg, bank_deg, st):
    """Circular bend; ``bank_deg`` > 0 raises the outer edge, < 0 the inner one (tilt follows sin^2)."""
    _positive(radius, "radius")
    if radius < MIN_RADIUS:
        raise ValueError("radius must be at least %.1f m (got %g)" % (MIN_RADIUS, radius))
    if not isinstance(angle_deg, (int, float)) or angle_deg == 0 or abs(angle_deg) > 360:
        raise ValueError("angle must be a non-zero number of degrees up to 360 (got %r)" % (angle_deg,))
    if abs(bank_deg) > MAX_BANK:
        raise ValueError("bank must not exceed %g degrees (got %g)" % (MAX_BANK, bank_deg))
    angle = math.radians(angle_deg)
    sgn = 1.0 if angle > 0 else -1.0
    n = max(2, math.ceil(abs(angle) * radius / MAX_STEP - 1e-9))
    fx, fy = cursor.forward()
    lx, ly = cursor.left()
    cx, cy = cursor.x + sgn * radius * lx, cursor.y + sgn * radius * ly   # circle centre
    rx, ry = -sgn * radius * lx, -sgn * radius * ly                       # centre -> start
    tx, ty = sgn * radius * fx, sgn * radius * fy                         # up x radial
    sections = []
    for k in range(1, n + 1):
        theta = angle * k / n
        tilt = 0.0 if (k == n or not bank_deg) else sgn * math.radians(bank_deg) * math.sin(math.pi * k / n) ** 2
        heading = cursor.heading + (angle if k == n else theta)
        pos = (cx + rx * math.cos(theta) + tx * math.sin(theta), cy + ry * math.cos(theta) + ty * math.sin(theta), cursor.z)
        sections.append(make_section(pos, heading, bank=tilt, style=st))
    return sections


def curve(prev, cursor, radius, angle, **style):
    """Flat circular bend; ``angle`` in degrees, positive turns left."""
    sections = _arc(cursor, radius, angle, 0.0, lane_style(style))
    return _finish(sections, cursor) + ({},)


def bank(prev, cursor, radius, angle, bank, **style):
    """Banked bend (steep curve): tilt ``bank`` degrees in the middle, 0 at both ends; positive raises the outer edge."""
    sections = _arc(cursor, radius, angle, bank, lane_style(style))
    return _finish(sections, cursor) + ({},)


def _profile_sections(cursor, length, z_of, rise, st):
    """Straight with a height profile ``z_of(x)``; the last section gets exactly ``cursor.z + rise``.

    The last section is ``flat`` (a loop, half-pipe or pad may start on it) only when the profile ends level: the tail
    of a lip ramp ends at the lip angle.
    """
    n = max(2, math.ceil(length / MAX_STEP - 1e-9))
    fx, fy = cursor.forward()
    sections = []
    for k in range(1, n + 1):
        x = length * k / n
        z = cursor.z + (rise if k == n else z_of(x))
        sections.append(make_section((cursor.x + fx * x, cursor.y + fy * x, z), cursor.heading, style=st))
    if abs(z_of(length) - z_of(length - END_SLOPE_STEP)) / END_SLOPE_STEP > END_SLOPE_FLAT:
        sections[-1].flat = False
    return sections


def _max_slope(z_of, length):
    steps = 200
    return max(abs(z_of(length * (k + 1) / steps) - z_of(length * k / steps)) / (length / steps) for k in range(steps))


def ramp(prev, cursor, length, rise, lip=None, **style):
    """Smooth S-shaped ramp of ``length`` m climbing (or dropping) ``rise`` m.

    With ``lip`` (degrees, > 0, needs ``rise`` > 0) the last fifth is a straight line at that angle that joins
    the S-curve with a continuous slope, ready for a jump.
    """
    _positive(length, "length")
    if not isinstance(rise, (int, float)) or rise == 0:
        raise ValueError("rise must be a non-zero number (got %r)" % (rise,))
    st = lane_style(style)
    if lip is None:
        def z_of(x):
            u = x / length
            return rise * (3 * u * u - 2 * u ** 3)
        limit = MAX_RAMP
    else:
        if not 0 < lip <= MAX_LIP or rise < 0:
            raise ValueError("lip must be 0-%g degrees and needs a positive rise" % MAX_LIP)
        slope, tail = math.tan(math.radians(lip)), 0.2 * length
        body, z1 = length - tail, rise - math.tan(math.radians(lip)) * tail

        def z_of(x):
            if x >= body:
                return z1 + slope * (x - body)
            u = x / body
            return z1 * (3 * u * u - 2 * u ** 3) + body * slope * (u ** 3 - u * u)
        if z1 <= 0 or any(z_of(length * (k + 1) / 100) < z_of(length * k / 100) - 1e-9 for k in range(100)):
            raise ValueError("lip is too steep for this rise and length")
        limit = MAX_LIP
    if math.degrees(math.atan(_max_slope(z_of, length))) > limit + 1e-6:
        raise ValueError("ramp is steeper than %g degrees" % limit)
    return _finish(_profile_sections(cursor, length, z_of, rise, st), cursor) + ({},)


def hill(prev, cursor, length, height, **style):
    """Cosine bump (``height`` > 0) or dip (``height`` < 0) over ``length`` m, level at both ends."""
    _positive(length, "length")
    if not isinstance(height, (int, float)) or height == 0:
        raise ValueError("height must be a non-zero number (got %r)" % (height,))
    st = lane_style(style)

    def z_of(x):
        return height * (1 - math.cos(2 * math.pi * x / length)) / 2
    if math.degrees(math.atan(abs(height) * math.pi / length)) > MAX_RAMP + 1e-6:
        raise ValueError("hill is steeper than %g degrees" % MAX_RAMP)
    return _finish(_profile_sections(cursor, length, z_of, 0.0, st), cursor) + ({},)


def steps(prev, cursor, count, drop, tread, **style):
    """``count`` flat treads of ``tread`` m, each followed by a vertical riser of ``drop`` m (negative climbs).

    Every step gives two sections at the same XY place (top and bottom of the riser); the riser is the
    vertical quad between them and belongs to the floor mesh. Rails skip the zero-length riser edges.
    """
    if not isinstance(count, int) or count < 1:
        raise ValueError("count must be a positive integer (got %r)" % (count,))
    if not isinstance(drop, (int, float)) or drop == 0 or abs(drop) > MAX_STEP_DROP:
        raise ValueError("drop must be a non-zero height of at most %.2f m (got %r)" % (MAX_STEP_DROP, drop))
    if not isinstance(tread, (int, float)) or tread < MIN_TREAD:
        raise ValueError("tread must be at least %.1f m (got %r)" % (MIN_TREAD, tread))
    st = lane_style(style)
    fx, fy = cursor.forward()
    sections = []
    for k in range(1, count + 1):
        x, y = cursor.x + fx * tread * k, cursor.y + fy * tread * k
        sections.append(make_section((x, y, cursor.z - (k - 1) * drop), cursor.heading, style=st))
        sections.append(make_section((x, y, cursor.z - k * drop), cursor.heading, style=st))
    return _finish(sections, cursor) + ({},)


def gap(prev, cursor, length, hazard="water", dz=0.0, **style):
    """No floor for ``length`` m; the next strip starts with a fresh flat section ``dz`` m higher.

    ``extras["hazard"]`` describes the volume under the gap: ``kind`` (``water`` or ``void``), the
    plan rectangle ``rect`` (``center``, ``heading``, ``half`` along/across, lane half width + 0.3 m)
    its axis-aligned ``min``/``max`` in X/Y, the height range ``z`` and the ``landing`` section (the first
    section of the next strip).
    """
    _positive(length, "length")
    if length > MAX_GAP:
        raise ValueError("gap length must not exceed %g m" % MAX_GAP)
    if hazard not in ("water", "void"):
        raise ValueError("hazard must be 'water' or 'void' (got %r)" % (hazard,))
    st = lane_style(style)
    fx, fy = cursor.forward()
    landing = make_section((cursor.x + fx * length, cursor.y + fy * length, cursor.z + dz), cursor.heading, style=st)
    half = (length / 2, LANE_WIDTH / 2 + 0.3)
    mid = (cursor.x + fx * length / 2, cursor.y + fy * length / 2)
    ex, ey = abs(fx) * half[0] + abs(fy) * half[1], abs(fy) * half[0] + abs(fx) * half[1]
    rect = {"center": mid, "heading": cursor.heading, "half": half}
    info = {"kind": hazard, "rect": rect, "min": (mid[0] - ex, mid[1] - ey), "max": (mid[0] + ex, mid[1] + ey),
            "z": (min(cursor.z, cursor.z + dz), max(cursor.z, cursor.z + dz)), "landing": landing}
    return [landing], Cursor(landing.center[0], landing.center[1], landing.center[2], cursor.heading), {"break": True, "hazard": info}


PIECES = {"straight": straight, "curve": curve, "bank": bank, "ramp": ramp, "hill": hill, "steps": steps, "gap": gap}
