"""Island outline from the layout of a track (MIT License, see ../LICENSE).

``floor_points(track)`` collects the plan points of all floors (section edges and middles, pad perimeters and
rings), ``fit_outline`` fits a ``island.Outline`` (superellipse ``power`` 3.0, ``wobble`` 0.035) around them, the
circles of the hero and the fixed props and a margin of 2.5 m. The centre is the middle of the bounding box, the
half axes keep its aspect ratio and grow until every margin sample lies inside the true wobbled outline. A half
axis above 20 m is an error: shorten or fold the course.
"""

import math

from .island import Outline
from .track import ROWS

POWER, WOBBLE = 3.0, 0.035
MARGIN = 2.5
MAX_HALF = 20.0
MIN_HALF = 4.0
GROW = 1.015
SAMPLES = 8                # margin samples around every footprint point
CIRCLE_SAMPLES = 24


def floor_points(track):
    """Plan points ``[(x, y)]`` of every floor of ``track`` (edges and middle of sections, pad outlines)."""
    points = []
    for strip in track.strips:
        for section in strip.sections:
            points.extend((section.points[k][0], section.points[k][1]) for k in (0, ROWS // 2, ROWS))
    for pad in track.pads:
        grid = pad.grid
        if grid.get("polar"):
            ring = grid["rings"][0]
            points.extend((p[0], p[1]) for p in ring)
            continue
        rows = grid["points"]
        for row in (rows[0], rows[-1]):
            points.extend((p[0], p[1]) for p in row)
        for row in rows:
            points.append((row[0][0], row[0][1]))
            points.append((row[-1][0], row[-1][1]))
    return points


def circle_points(x, y, radius):
    """Points on a circle (``CIRCLE_SAMPLES`` of them) for the footprint of a hero or a fixed prop."""
    return [(x + radius * math.cos(math.tau * k / CIRCLE_SAMPLES), y + radius * math.sin(math.tau * k / CIRCLE_SAMPLES))
            for k in range(CIRCLE_SAMPLES)]


def convex_hull(points):
    """Convex hull of 2D points (monotone chain, counter-clockwise, no duplicates)."""
    pts = sorted(set((round(x, 6), round(y, 6)) for x, y in points))
    if len(pts) < 3:
        return pts

    def half(seq):
        out = []
        for p in seq:
            while len(out) >= 2 and (out[-1][0] - out[-2][0]) * (p[1] - out[-2][1]) - (out[-1][1] - out[-2][1]) * (p[0] - out[-2][0]) <= 0:
                out.pop()
            out.append(p)
        return out
    lower, upper = half(pts), half(reversed(pts))
    return lower[:-1] + upper[:-1]


def _required(points, centre, half):
    """Smallest factor that scales the superellipse ``half`` so all ``points`` lie inside it."""
    n = POWER
    worst = 0.0
    for x, y in points:
        worst = max(worst, (abs((x - centre[0]) / half[0]) ** n + abs((y - centre[1]) / half[1]) ** n) ** (1.0 / n))
    return worst


def fit_outline(points, number, label=""):
    """``island.Outline`` around ``points`` plus the margin; raises ``ValueError`` when it would exceed ``MAX_HALF``."""
    points = convex_hull(points)          # the superellipse is convex: the hull corners decide
    xs, ys = [p[0] for p in points], [p[1] for p in points]
    centre = ((min(xs) + max(xs)) / 2.0, (min(ys) + max(ys)) / 2.0)
    base = ((max(xs) - min(xs)) / 2.0 + MARGIN, (max(ys) - min(ys)) / 2.0 + MARGIN)
    base = (max(base[0], MIN_HALF), max(base[1], MIN_HALF))
    margin_points = []
    step = math.tau / SAMPLES
    for x, y in points:
        margin_points.extend((x + MARGIN * math.cos(step * k), y + MARGIN * math.sin(step * k)) for k in range(SAMPLES))
    scale = max(1.0, _required(margin_points, centre, base))
    while True:
        half = (base[0] * scale, base[1] * scale)
        outline = Outline(center=centre, half=half, power=POWER, wobble=WOBBLE, seed=number)
        if all(outline.contains_signed(x, y) < 0.0 for x, y in margin_points):
            break
        scale *= GROW
    if max(half) > MAX_HALF:
        raise ValueError("%sthe island needs half axes of %.1f x %.1f m for this layout, the limit is %g m: fold or shorten the "
                         "course, or move the hero and the props closer" % (label, half[0], half[1], MAX_HALF))
    return outline
