"""System World kit v3: street modules (Blender 5.2 LTS, called from build_world3.py).

Every module runs along +X and is 12 m wide (Y). Heights follow the shared street profile
in ui/js/desktop/apps/sysworld-ground.js: carriageway and gutter at ROAD, a 45 degree
splayed kerb from the kerb line to the pavement, pavement flush with the plots at 0.
Pedestrian crossings stretch the kerb into a 1.2 m ramp (dropped kerb) with flares.
Keep the numbers below equal to `surfaces` and `kerb` in the renderer; the contract test
ray-casts the exported models against that profile.
"""
from math import sin, cos, pi

ROAD, PAVE = -.14, 0.0
RISE = PAVE - ROAD
HALF = 6.0            # half street width
GUTTER = 3.00         # carriageway edge, start of the gutter setts
LINE = 3.30           # kerb line
RUN = .14             # splayed kerb run
TOP = .14             # kerb stone top width
BORDER = 3.78         # end of the pavement border course
TACTILE = (4.50, 4.90)
DROPPED_RUN = 1.2     # ramp run inside a dropped kerb
OPENING = 1.2         # half width of the dropped opening
FLARE = .8            # flare length beside the opening
CORNER = 1.2          # kerb radius at intersection corners
END_DEPTH = .5        # street end cap
JOINT = .006          # half width of a stone joint
BODY = -.62           # lower edge of the end wall, below the island body top


def lerp(a, b, t):
    return a + (b - a) * t


def kerb_run(x):
    """Kerb run and kerb-top width at distance |x| from a dropped-kerb centre."""
    s = abs(x)
    if s <= OPENING:
        return DROPPED_RUN, 0.0
    if s >= OPENING + FLARE:
        return RUN, TOP
    t = (s - OPENING) / FLARE
    return lerp(DROPPED_RUN, RUN, t), TOP * t


def section(x, lod, run=RUN, top=TOP, sign=1):
    """Cross-section points of one kerb side, from the carriageway edge to the plot."""
    points = [(GUTTER, ROAD), (LINE, ROAD)]
    if lod == 0:
        # A small chamfer rounds the upper kerb edge; it stays within 12 mm of the profile.
        points += [(LINE + run * .857, ROAD + RISE * .857), (LINE + run + min(.025, top / 2), PAVE)]
    else:
        points += [(LINE + run, PAVE)]
    end = LINE + run + top
    points += [(end, PAVE)]
    if lod < 2:
        points += [(max(BORDER, end), PAVE)]
    points += [(TACTILE[0], PAVE), (TACTILE[1], PAVE), (HALF, PAVE)]
    return [(x, sign * y, z) for y, z in points]


def bands(lod):
    """Band names between successive section points."""
    kerb = ['splay', 'chamfer', 'top'] if lod == 0 else ['splay', 'top']
    return ['setts'] + kerb + (['border'] if lod < 2 else []) + ['field', 'strip', 'outer']


BAND_MATERIAL = {'setts': 'setts', 'splay': 'kerb', 'chamfer': 'kerb', 'top': 'kerb',
                 'border': 'stone', 'field': 'stone', 'strip': 'stone', 'outer': 'stone'}


def stone_columns(x0, x1, lod):
    """Column positions with 12 mm joint strips at every whole metre (LOD 0)."""
    if lod == 2:
        return [x0, x1], [False]
    marks = [x0 + i for i in range(int(round(x1 - x0)) + 1)]
    if lod == 1:
        return marks, [False] * (len(marks) - 1)
    columns, joints = [x0], []
    for index, mark in enumerate(marks):
        if index == 0:
            columns.append(mark + JOINT)
            joints.append(True)
        elif index == len(marks) - 1:
            columns += [mark - JOINT, mark]
            joints += [False, True]
        else:
            columns += [mark - JOINT, mark + JOINT]
            joints += [False, True]
    return columns, joints


def kerb_sides(g, columns, joints, dropped=None, tactile=None, skip=None):
    """Both kerb sides for the given columns.

    `dropped` is the X of a dropped-kerb centre, `tactile` an (x0, x1) range that gets the
    tactile strip, `skip` a callable (strip_index, band, sign) that leaves a hole.
    """
    names = bands(g.lod)
    for sign in (1, -1):
        rows = []
        for x in columns:
            run, top = kerb_run(x - dropped) if dropped is not None else (RUN, TOP)
            rows.append(section(x, g.lod, run, top, sign))
        shades = [.86 + .14 * g.rng.random() for _ in columns]
        fields = [.93 + .07 * g.rng.random() for _ in columns]

        def material(r, k):
            band = names[k]
            if skip and skip(r, band, sign):
                return None
            middle = (columns[r] + columns[r + 1]) / 2
            if band == 'strip' and tactile and tactile[0] <= middle <= tactile[1]:
                return 'tactile'
            if dropped is not None and band in ('splay', 'chamfer') and abs(middle - dropped) <= OPENING:
                return 'stone'
            return BAND_MATERIAL[band]

        def tint(r, k):
            band = names[k]
            if joints[r]:
                if band in ('splay', 'chamfer', 'top'):
                    return .42
                if band == 'setts':
                    return .78
                # Pavement expansion joints only every second metre.
                return .7 if round((columns[r] + columns[r + 1]) / 2) % 2 == 0 else fields[r - 1 if r else 0]
            if band in ('splay', 'chamfer', 'top'):
                return shades[r]
            if band == 'border':
                return .88
            if band == 'setts':
                return .9 + .1 * ((r // 2) % 2)
            return fields[r]

        g.loft(rows, material, tint, up=True)


def carriageway(g, xs, ys, mark):
    """Flat road cells; `mark(xm, ym)` returns True for paint."""
    def material(i, j):
        return 'marking' if mark((xs[i] + xs[i+1]) / 2, (ys[j] + ys[j+1]) / 2) else 'road'
    g.grid(xs, ys, ROAD, material)


def merged(*lists):
    values = sorted(set(round(v, 5) for values in lists for v in values))
    return values


def manhole(g, x, y):
    """Cast iron cover lying 4 mm proud of the road."""
    if g.lod == 2:
        return
    n = (28, 14)[g.lod]
    z = ROAD + .004
    radii = [.0, .09, .11, .2, .22, .3, .32, .37] if g.lod == 0 else [.0, .3, .37]
    tints = [.8, .5, .8, .5, .8, .45, 1.0] if g.lod == 0 else [.75, 1.0]
    for inner, outer, tint in zip(radii, radii[1:], tints):
        g.lathe((x, y, 0), [(inner, z), (outer, z)], 'iron', n=n, tint=tint, up=True)
    g.lathe((x, y, 0), [(radii[-1], z), (radii[-1] + .012, ROAD)], 'iron', n=n, tint=.6, up=True)
    if g.lod == 0:
        for angle in (0, pi / 2):
            g.box((x, y, z + .003), (.5, .035, .006), 'iron', 0, angle=angle, tint=.7)


def drain(g, x0, x1, sign):
    """Gutter drain: a dark pit under cast iron bars, filling the gutter band between x0 and x1."""
    y0, y1 = GUTTER, LINE
    side = lambda y: sign * y
    if g.lod == 2:
        return False
    if g.lod == 1:
        xs = [x0 + (x1 - x0) * i / 8 for i in range(9)]
        g.grid(xs, sorted((side(y0), side(y1))), ROAD, 'iron', tint=lambda i, j: (.9, .3)[i % 2])
        return True
    pit = ROAD - .04
    frame = .022
    bars = 6
    inner0, inner1 = x0 + frame, x1 - frame
    pitch = (inner1 - inner0) / bars
    xs = [x0, inner0]
    for i in range(bars):
        xs += [inner0 + pitch * i + pitch * .42, inner0 + pitch * (i + 1)]
    xs[-1] = inner1
    xs.append(x1)
    ys = sorted((side(y0), side(y0 + frame), side(y1 - frame), side(y1)))

    def material(i, j):
        # Frame all round, then alternating slot and bar along X.
        if i == 0 or i == len(xs) - 2 or j != 1:
            return 'iron'
        return None if (i - 1) % 2 == 0 else 'iron'

    g.grid(xs, ys, ROAD, material, tint=lambda i, j: .85)
    # Pit floor and walls.
    a, b = sorted((side(y0 + frame), side(y1 - frame)))
    g.quad((inner0, a, pit), (inner1, a, pit), (inner1, b, pit), (inner0, b, pit), 'iron', .35, up=True)
    g.quad((inner0, a, pit), (inner0, a, ROAD), (inner1, a, ROAD), (inner1, a, pit), 'iron', .5)
    g.quad((inner1, b, pit), (inner1, b, ROAD), (inner0, b, ROAD), (inner0, b, pit), 'iron', .5)
    g.quad((inner0, b, pit), (inner0, b, ROAD), (inner0, a, ROAD), (inner0, a, pit), 'iron', .5)
    g.quad((inner1, a, pit), (inner1, a, ROAD), (inner1, b, ROAD), (inner1, b, pit), 'iron', .5)
    return True


def straight(g, length, dash=True, drains=False):
    x0, x1 = -length / 2, length / 2
    columns, joints = stone_columns(x0, x1, g.lod)
    grate = (-.25, .25)
    if drains and g.lod < 2:
        columns = merged(columns, grate)
        joints = [abs((b - a) - JOINT) < 1e-4 or abs((b - a) - 2 * JOINT) < 1e-4
                  for a, b in zip(columns, columns[1:])]

    def skip(r, band, sign):
        middle = (columns[r] + columns[r + 1]) / 2
        return drains and g.lod < 2 and band == 'setts' and grate[0] < middle < grate[1]

    kerb_sides(g, columns, joints, skip=skip)
    if drains:
        for sign in (1, -1):
            drain(g, grate[0], grate[1], sign)
    half = min(1.0, length / 2)
    xs = merged(columns, [-half, half]) if dash else columns
    carriageway(g, xs, [-GUTTER, -.06, .06, GUTTER],
                lambda xm, ym: dash and abs(xm) < half and abs(ym) < .06)
    if drains:
        manhole(g, .95, -1.45)


def street_plain(g):
    straight(g, 4)


def street_drain(g):
    straight(g, 4, drains=True)


def street_fill(g):
    straight(g, 1, dash=False)


def street_zebra(g):
    """Pedestrian crossing: zebra bars across the road and dropped kerbs on both sides."""
    steps = (3, 2, 1)[g.lod]
    flare = [OPENING + FLARE * i / steps for i in range(steps + 1)]
    columns = merged([-v for v in flare], flare, [0.0] if g.lod == 0 else [])
    joints = [False] * (len(columns) - 1)
    kerb_sides(g, columns, joints, dropped=0.0, tactile=(-OPENING, OPENING))
    bar = 1.5
    ys = [-GUTTER] + [-2.75 + .5 * i for i in range(12)] + [GUTTER]
    xs = merged(columns, [-bar, bar])

    def mark(xm, ym):
        return abs(xm) < bar and int((ym + 2.75) // .5) % 2 == 0 and abs(ym) < 2.75

    if g.lod == 2:
        xs, ys = [-2, -bar, bar, 2], [-GUTTER] + [-2.75 + i for i in range(6)] + [GUTTER]
        mark = lambda xm, ym: abs(xm) < bar and abs(ym) < 2.75 and int(ym + 2.75) % 2 == 0
    carriageway(g, xs, ys, mark)
    if g.lod == 0:
        # Corduroy ribs on the tactile strip catch the light and read from the street.
        for sign in (1, -1):
            for i in range(4):
                y = TACTILE[0] + .07 + i * .087
                g.box((0, sign * y, PAVE + .003), (2 * OPENING - .12, .034, .008), 'tactile', .003, 1, tint=.92)


def corner_rows(g, sx, sy):
    """Rows along the kerb of one intersection corner, each from gutter edge to kerb top."""
    arc = (10, 5, 2)[g.lod]
    offsets = [GUTTER - LINE, 0.0]
    heights = [ROAD, ROAD]
    if g.lod == 0:
        offsets += [RUN * .857, RUN + .025]
        heights += [ROAD + RISE * .857, PAVE]
    else:
        offsets += [RUN]
        heights += [PAVE]
    offsets += [RUN + TOP]
    heights += [PAVE]
    centre = LINE + CORNER
    rows = []

    def row(base, normal, radial=None):
        points = []
        for d, z in zip(offsets, heights):
            if radial is None:
                points.append((sx * (base[0] + normal[0] * d), sy * (base[1] + normal[1] * d), z))
            else:
                r = CORNER - d
                points.append((sx * (centre + r * cos(radial)), sy * (centre + r * sin(radial)), z))
        rows.append(points)

    straight_marks = [HALF, 5.25, centre] if g.lod < 2 else [HALF, centre]
    for x in straight_marks[:-1]:
        row((x, LINE), (0, 1))
    for i in range(arc + 1):
        row(None, None, -pi / 2 - (pi / 2) * i / arc)
    for y in reversed(straight_marks[:-1]):
        row((LINE, y), (1, 0))
    return rows


def street_crossing(g):
    """Four-way intersection: road plate, gutters, rounded corner kerbs and corner pavements."""
    names = ['setts', 'splay'] + (['chamfer'] if g.lod == 0 else []) + ['top']
    rim = []
    for sx, sy in ((1, 1), (-1, 1), (-1, -1), (1, -1)):
        rows = corner_rows(g, sx, sy)
        shades = [.86 + .14 * g.rng.random() for _ in rows]

        def material(r, k):
            return BAND_MATERIAL[names[k]]

        def tint(r, k):
            if names[k] == 'setts':
                return .9 + .1 * (r % 2)
            return shades[r] if g.lod < 2 else .93

        g.loft(rows, material, tint, up=True)
        # Corner pavement between the kerb top and the plot corner.
        inner = [row[-1] for row in rows]
        edge_a = [(sx * HALF, sy * y, PAVE) for y in (BORDER, TACTILE[0], TACTILE[1])] if g.lod < 2 else []
        edge_b = [(sx * x, sy * HALF, PAVE) for x in (TACTILE[1], TACTILE[0], BORDER)] if g.lod < 2 else []
        loop = list(reversed(inner)) + edge_a + [(sx * HALF, sy * HALF, PAVE)] + edge_b
        g.fan((sx * 5.1, sy * 5.1, PAVE), loop, 'stone', tint=.96, up=True, closed=True)
        # Carriageway rim: this corner's gutter edge, in counter-clockwise order.
        curve = [row[0] for row in rows]
        rim += curve if sx * sy > 0 else list(reversed(curve))
    # One polygon without an inner vertex: a hub under the manhole would bake a dark
    # occlusion value and smear it over the whole plate.
    g.add(rim, [tuple(range(len(rim)))], 'road', up=True)
    manhole(g, 0, 0)


def street_end(g):
    """Kerb block that closes the carriageway at a street end; +X points outward."""
    width = LINE + RUN
    stones = (7, 7, 1)[g.lod]
    ys = [-width + 2 * width * i / stones for i in range(stones + 1)]
    for i in range(stones):
        tint = .86 + .14 * g.rng.random()
        a, b = ys[i], ys[i + 1]
        gap = JOINT if g.lod == 0 else 0
        g.quad((-END_DEPTH, a + gap, PAVE), (0, a + gap, PAVE), (0, b - gap, PAVE), (-END_DEPTH, b - gap, PAVE),
               'kerb', tint, up=True)
        g.quad((-END_DEPTH - RUN, a + gap, ROAD), (-END_DEPTH, a + gap, PAVE), (-END_DEPTH, b - gap, PAVE),
               (-END_DEPTH - RUN, b - gap, ROAD), 'kerb', tint, up=True)
        if gap:
            for y in (a, b - gap):
                g.quad((-END_DEPTH, y, PAVE), (0, y, PAVE), (0, y + gap, PAVE), (-END_DEPTH, y + gap, PAVE),
                       'kerb', .42, up=True)
                g.quad((-END_DEPTH - RUN, y, ROAD), (-END_DEPTH, y, PAVE), (-END_DEPTH, y + gap, PAVE),
                       (-END_DEPTH - RUN, y + gap, ROAD), 'kerb', .42, up=True)
    # Outer wall across the whole street width, facing +X.
    g.quad((0, -HALF, BODY), (0, HALF, BODY), (0, HALF, PAVE), (0, -HALF, PAVE), 'stone', .9)


ASSETS = [
    ('street-plain', 'Street module, 4 m', 'street', street_plain),
    ('street-drain', 'Street module with gutter drains and manhole, 4 m', 'street', street_drain),
    ('street-zebra', 'Pedestrian crossing with dropped kerbs, 4 m', 'street', street_zebra),
    ('street-fill', 'Street filler module, 1 m', 'street', street_fill),
    ('street-crossing', 'Four-way intersection, 12 m', 'street', street_crossing),
    ('street-end', 'Street end kerb block', 'street', street_end),
]


def build(asset_id):
    for entry in ASSETS:
        if entry[0] == asset_id:
            return entry[3]
    raise KeyError(asset_id)
