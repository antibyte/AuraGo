"""System World kit v3: harbour craft, plaza furniture, signs and birds (Blender 5.2 LTS).

Things that make the city feel inhabited: boats and a buoy for the harbour basin, a flower
bed, a cafe table under a parasol, a charging post, a wayfinding pylon, a street cabinet,
neon signs for facades, a banner mast and a gull with separate wings.

Boats have their origin on the waterline amidships, bow towards +X. Street pieces sit on
the pavement surface (z = 0) and face -Y. Wall signs have their origin on the wall plane
and project towards -Y. Parts with a pivot (`rig`, `lamp`, `banner`, `wing_l`, `wing_r`)
export as separate nodes so that the renderer can move them.
"""
from math import sin, cos, pi

from mathutils import Matrix

from furniture_assets3 import blob, collider

RX = Matrix.Rotation(pi / 2, 3, 'X')     # local +Z becomes -Y: discs and rings face the street
RY = Matrix.Rotation(pi / 2, 3, 'Y')     # local +Z becomes +X: shapes run along the keel


def sample(table, t):
    """Linear interpolation in an evenly spaced table, t in 0..1."""
    f = t * (len(table) - 1)
    i = min(int(f), len(table) - 2)
    return table[i] + (table[i + 1] - table[i]) * (f - i)


def face(g, points, mat, toward, tint=1.0):
    """Planar polygon wound so that its normal points along `toward`."""
    n = [0.0, 0.0, 0.0]
    for a, b in zip(points, points[1:] + points[:1]):
        n[0] += (a[1] - b[1]) * (a[2] + b[2])
        n[1] += (a[2] - b[2]) * (a[0] + b[0])
        n[2] += (a[0] - b[0]) * (a[1] + b[1])
    if sum(n[i] * toward[i] for i in range(3)) < 0:
        points = list(reversed(points))
    g.add(points, [tuple(range(len(points)))], mat, tint=tint)


def sheet(g, rows, mat, tint=1.0):
    """Thin cloth seen from both sides."""
    bands = [mat] * (len(rows[0]) - 1)
    tints = tint if callable(tint) else [tint] * (len(rows[0]) - 1)
    g.loft(rows, bands, tints)
    g.loft(rows, bands, tints, flip=True)


def torus(g, center, radius, tube, mat, rot=None, arc=2 * pi, tint=1.0, n=None, sides=None):
    """Kit.ring with a lighter mesh for small tubes: tyres, coils, neon rings."""
    n, sides = n or (28, 16, 10)[g.lod], sides or (6, 5, 3)[g.lod]
    count = n if abs(arc - 2 * pi) < 1e-6 else n + 1
    vertices = [((radius + tube * cos(j * 2 * pi / sides)) * cos(i * arc / n),
                 (radius + tube * cos(j * 2 * pi / sides)) * sin(i * arc / n), tube * sin(j * 2 * pi / sides))
                for i in range(count) for j in range(sides)]
    faces = [(i * sides + j, ((i + 1) % count) * sides + j, ((i + 1) % count) * sides + (j + 1) % sides,
              i * sides + (j + 1) % sides) for i in range(n) for j in range(sides)]
    g.add(vertices, faces, mat, center, rot, tint)


def hull(g, length, beam, full, sheer, keel, topside, bottom, chine=False, stations=None, deck='bronze',
         deck_tint=.75, drop=.14, inner='ceramic', cap='bronze'):
    """Hull along X with the waterline at z = 0, a deck `drop` below the gunwale and closed ends.

    `full`, `sheer` and `keel` are tables from stern to bow: share of the half beam, gunwale
    height and keel depth. Returns a function t -> (x, half beam, gunwale z).
    """
    count = stations or (13, 8, 5)[g.lod]

    def at(t):
        return -length/2 + length * t, max(.02, beam/2 * sample(full, t)), sample(sheer, t)

    outer, top = [], []
    for i in range(count):
        x, b, z1 = at(i / (count - 1))
        z0 = -sample(keel, i / (count - 1))
        if chine:
            row = [(x, -b, z1), (x, -b, z0 * .25), (x, -b * .9, z0), (x, 0, z0), (x, b * .9, z0), (x, b, z0 * .25), (x, b, z1)]
        else:
            row = [(x, -b, z1), (x, -b * .88, .04), (x, -b * .55, z0 * .78), (x, 0, z0),
                   (x, b * .55, z0 * .78), (x, b * .88, .04), (x, b, z1)]
        outer.append(row)
        inset = min(.1, b * .6)
        top.append([(x, -b, z1), (x, -b + inset * .8, z1), (x, -b + inset, z1 - drop),
                    (x, b - inset, z1 - drop), (x, b - inset * .8, z1), (x, b, z1)])
    g.loft(outer, [topside[0], bottom[0], bottom[0], bottom[0], bottom[0], topside[0]],
           [topside[1], bottom[1], bottom[1], bottom[1], bottom[1], topside[1]], flip=True)
    g.loft(top, [cap, inner, deck, inner, cap], [1.0, .9, deck_tint, .9, 1.0])
    face(g, outer[0], topside[0], (-1, 0, 0), topside[1])
    face(g, [top[0][1], top[0][4], top[0][3], top[0][2]], inner, (1, 0, 0), .9)
    face(g, outer[-1], topside[0], (1, 0, 0), topside[1])
    return at


def rail(g, at, ts, height, inset=.1, radius=.022, mat='titanium', posts=True, close=True):
    """Guard rail along both gunwales through the stations `ts`; `close` joins the sides at the last one."""
    ends = []
    for side in (-1, 1):
        points = [(x, side * (b - inset), z + height) for x, b, z in (at(t) for t in ts)]
        for p, q in zip(points, points[1:]):
            g.pipe(p, q, radius, mat)
        if posts:
            for p in points:
                g.pipe((p[0], p[1], p[2] - height), p, radius * .8, mat)
        ends.append(points[-1])
    if close:
        g.pipe(ends[0], ends[1], radius, mat)


LAUNCH = dict(full=[.80, .92, .99, 1.0, .98, .90, .74, .50, .24, .03],
              sheer=[.80, .75, .72, .72, .75, .80, .88, .98, 1.10, 1.22],
              keel=[.28, .40, .45, .46, .45, .42, .36, .27, .15, .02])


def harbour_launch(g):
    """Harbour launch with a wheelhouse, 9 m."""
    lod = g.lod
    at = hull(g, 9.0, 2.9, **LAUNCH, topside=('ceramic', 1.0), bottom=('graphite', 1.2))
    # Wheelhouse.
    g.box((.9, 0, 1.335), (2.6, 1.9, 1.55), 'ceramic', .1, 2)
    g.box((2.205, 0, 1.62), (.03, 1.5, .62), 'glass', 0)
    g.box((-.405, -.42, 1.62), (.03, .7, .55), 'glass', 0)
    g.box((-.405, .4, 1.3), (.04, .62, 1.3), 'bronze', 0)
    for side in (-1, 1):
        g.box((.9, side * .955, 1.62), (2.1, .03, .6), 'glass', 0)
        if lod == 0:
            for x in (.2, 1.6):
                g.box((x, side * .962, 1.62), (.07, .035, .6), 'ceramic', 0)
        if lod < 2:
            g.box((.9, side * .962, 2.03), (2.5, .02, .05), 'cyan', 0)
            g.pipe((-.4, side * .95, 2.3), (2.0, side * .95, 2.3), .02, 'titanium')
            for x in (-.4, .8, 2.0):
                g.pipe((x, side * .95, 2.19), (x, side * .95, 2.3), .016, 'titanium')
    g.box((.8, 0, 2.155), (3.0, 2.2, .09), 'graphite', .03, 1)
    g.box((1.9, .98, 2.08), (.14, .05, .08), 'warm', 0)
    g.box((1.9, -.98, 2.08), (.14, .05, .08), 'jade', 0)
    g.cyl((.3, 0, 2.75), .035, 1.1, 'titanium', n=(8, 6, 5)[lod])
    g.cyl((.3, 0, 3.34), .055, .09, 'ivory', n=(8, 6, 5)[lod])
    if lod < 2:
        g.box((.3, 0, 2.96), (.1, .9, .08), 'ceramic', .02, 1)
        g.cyl((2.0, 0, 2.3), .09, .16, 'titanium', rot=RY, n=(10, 8)[lod])
        g.cyl((2.085, 0, 2.3), .075, .01, 'ivory', rot=RY, n=(10, 8)[lod])
        g.box((2.95, 0, .96), (.7, .7, .1), 'titanium', .03, 1)
    # Aft cockpit.
    g.box((-3.9, 0, .84), (.5, 2.0, .42), 'bronze', .03, 1, tint=.8)
    g.box((-2.6, 0, .84), (1.3, .8, .5), 'graphite', .05, 2)
    for side in (-1, 1):
        g.box((-2.7, side * .92, .8), (1.8, .4, .34), 'bronze', .03, 1, tint=.8)
    g.cyl((-4.42, 0, 1.3), .025, .7, 'titanium', n=(6, 5, 4)[lod])
    g.cyl((-4.42, 0, 1.68), .045, .07, 'ivory', n=(8, 6, 5)[lod])
    if lod < 2:
        rail(g, at, (.62, .74, .86, .97), .5)
        for side in (-1, 1):
            for t in (.24, .44, .62):
                x, b, z = at(t)
                g.cyl((x, side * (b + .1), z - .42), .1, .5, 'road', n=(10, 7)[lod], tint=1.6)
                g.pipe((x, side * (b - .02), z), (x, side * (b + .1), z - .17), .012, 'tactile', n=4)
    if lod == 0:
        for side in (-1, 1):
            points = [at(i / 14) for i in range(15)]
            for (x0, b0, z0), (x1, b1, z1) in zip(points, points[1:]):
                g.pipe((x0, side * (b0 + .012), z0 - .04), (x1, side * (b1 + .012), z1 - .04), .028, 'bronze', n=5)
        torus(g, (-.05, .985, 1.15), .19, .045, 'marking', RX, n=16)
        for x in (3.75, -4.1):
            g.box((x, 0, at((x + 4.5) / 9)[2] - .1), (.3, .08, .07), 'iron', .02, 1)
    return {'navigation': {'colliders': [collider(-4.5, -1.45, 4.5, 1.45, 2.3, -.5)]}, 'occlusion': {'distance': 1.4}}


BARGE = dict(full=[.86, 1, 1, 1, 1, 1, 1, 1, .97, .84],
             sheer=[1.05, .9, .9, .9, .9, .9, .9, .95, 1.1, 1.3],
             keel=[.2, .62, .7, .7, .7, .7, .7, .66, .45, .1])


def container(g, x, y, z, mat, tint, light):
    """Data container, 5.8 x 2.35 x 2.5 m, doors towards +X."""
    lod = g.lod
    g.box((x, y, z + 1.25), (5.8, 2.35, 2.5), mat, .04, 1, tint=tint)
    if lod < 2:
        for side in (-1, 1):
            g.box((x + 2.915, y + side * .4, z + 1.25), (.03, .05, 2.2), 'titanium', 0)
        g.box((x + 2.915, y, z + 2.28), (.02, 1.3, .08), light, 0)
        g.box((x, y, z + 2.52), (5.5, 2.0, .04), mat, 0, tint=tint * .8)
    if lod == 0:
        for i in range(7):
            for side in (-1, 1):
                g.box((x - 2.4 + i * .8, y + side * 1.185, z + 1.25), (.14, .03, 2.2), mat, 0, tint=tint * .78)


def bitt(g, x, y, z):
    """Double mooring bollard."""
    n = (10, 7, 5)[g.lod]
    g.box((x, y, z + .03), (.7, .3, .06), 'iron', 0)
    for dx in (-.2, .2):
        g.cyl((x + dx, y, z + .26), .08, .4, 'iron', n=n)
        g.cyl((x + dx, y, z + .48), .11, .05, 'iron', n=n)
    if g.lod < 2:
        g.pipe((x - .32, y, z + .3), (x + .32, y, z + .3), .035, 'iron', n=6)


def harbour_barge(g):
    """Cargo barge with data containers and an aft wheelhouse, 22 m."""
    lod = g.lod
    at = hull(g, 22.0, 6.2, **BARGE, topside=('bronze', .55), bottom=('graphite', 1.0), chine=True,
              stations=(10, 7, 5)[lod], deck='road', deck_tint=1.5, drop=.3, inner='graphite', cap='titanium')
    deck = .6
    cargo = [(-2.9, -1.3, 0, 'graphite', 1.6, 'cyan'), (-2.9, 1.3, 0, 'ceramic', .9, 'jade'),
             (3.2, -1.3, 0, 'bronze', .8, 'warm'), (3.2, 1.3, 0, 'titanium', .9, 'cyan'),
             (-2.9, -1.3, 2.56, 'ceramic', .72, 'warm'), (-2.5, 1.3, 2.56, 'graphite', 1.25, 'jade')]
    for x, y, z, mat, tint, light in cargo:
        container(g, x, y, deck + z, mat, tint, light)
    if lod < 2:
        for side in (-1, 1):
            g.box((.15, side * 2.72, deck + .25), (12.6, .1, .5), 'titanium', .02, 1)
        for x in (-6.2, 6.5):
            g.box((x, 0, deck + .25), (.1, 5.5, .5), 'titanium', .02, 1)
    # Wheelhouse and bridge.
    g.box((-8.5, 0, deck + 1.3), (3.4, 4.6, 2.6), 'ceramic', .08, 2, tint=.92)
    g.box((-8.3, 0, deck + 3.7), (2.6, 3.8, 2.2), 'ceramic', .08, 2)
    g.box((-6.99, 0, deck + 3.95), (.03, 3.4, .9), 'glass', 0)
    g.box((-9.61, 0, deck + 3.95), (.03, 3.0, .8), 'glass', 0)
    for side in (-1, 1):
        g.box((-8.3, side * 1.905, deck + 3.95), (2.2, .03, .9), 'glass', 0)
        g.box((-7.0, side * 2.0, deck + 4.9), (.14, .06, .1), 'warm' if side > 0 else 'jade', 0)
    g.box((-8.3, 0, deck + 4.87), (3.1, 4.3, .12), 'graphite', .04, 1)
    g.box((-6.79, 1.2, deck + 1.1), (.04, .8, 2.0), 'bronze', 0)
    for y in (-1.3, -.2):
        g.box((-6.79, y, deck + 1.6), (.03, .7, .7), 'glass', 0)
    g.cyl((-8.7, 0, deck + 5.9), .05, 2.0, 'titanium', n=(8, 6, 5)[lod])
    g.cyl((-8.7, 0, deck + 6.94), .07, .1, 'ivory', n=(8, 6, 5)[lod])
    g.box((-9.3, 1.1, deck + 5.3), (.55, .55, .8), 'graphite', .04, 1)
    g.box((-9.3, 1.1, deck + 5.73), (.4, .4, .06), 'cyan', 0)
    if lod < 2:
        g.box((-8.7, 0, deck + 6.3), (.12, 1.5, .1), 'ceramic', .02, 1)
        g.box((-8.3, 0, deck + 2.62), (3.6, 4.8, .06), 'titanium', 0)
        for side in (-1, 1):
            g.pipe((-10.0, side * 2.3, deck + 3.5), (-6.7, side * 2.3, deck + 3.5), .025, 'titanium')
            for x in (-10.0, -8.35, -6.7):
                g.pipe((x, side * 2.3, deck + 2.65), (x, side * 2.3, deck + 3.5), .02, 'titanium')
    # Bow gear.
    bow = at(.89)[2] - .3
    g.box((8.6, 0, bow + .12), (1.3, 1.7, .24), 'titanium', .03, 1)
    g.cyl((8.6, 0, bow + .6), .34, 1.2, 'iron', rot=RX, n=(14, 9, 6)[lod])
    if lod < 2:
        for side in (-1, 1):
            g.box((8.6, side * .72, bow + .55), (.5, .1, .7), 'titanium', .02, 1)
            bitt(g, 9.9, side * 1.7, at(.95)[2] - .3)
            bitt(g, -5.9, side * 2.45, deck)
            for t in (.2, .35, .5, .65, .8):
                x, b, z = at(t)
                torus(g, (x, side * (b + .07), .42), .3, .1, 'road', RX, tint=1.5, n=(16, 10)[lod])
    return {'navigation': {'colliders': [collider(-11, -3.1, 11, 3.1, 5.6, -.8)]}, 'occlusion': {'distance': 2.2}}


SKIFF = dict(full=[.72, .90, .99, 1.0, .96, .84, .62, .34, .04],
             sheer=[.52, .47, .44, .44, .47, .52, .60, .70, .82],
             keel=[.12, .22, .26, .27, .26, .22, .16, .08, .01])


def sail(g, tack, clew, head, belly, rows, columns, tint=1.0):
    """Triangular sail between three corners with a belly towards +Y."""
    grid = []
    for r in range(rows + 1):
        h = r / rows
        p = [tack[i] + (head[i] - tack[i]) * h for i in range(3)]
        q = [clew[i] + (head[i] - clew[i]) * h for i in range(3)]
        grid.append([(p[0] + (q[0] - p[0]) * s, p[1] + (q[1] - p[1]) * s + belly * sin(pi * s) * (1 - h) ** .7,
                      p[2] + (q[2] - p[2]) * s) for s in (c / columns for c in range(columns + 1))])
    sheet(g, grid, 'ceramic', lambda r, k: tint * (1.0 if r % 2 else .93))


def sail_skiff(g):
    """Wooden sailing skiff with main sail and jib, 5.4 m."""
    lod = g.lod
    at = hull(g, 5.4, 1.9, **SKIFF, topside=('bronze', .9), bottom=('ceramic', 1.0), stations=(11, 7, 5)[lod],
              deck='bronze', deck_tint=.6, drop=.2, inner='bronze', cap='bronze')
    g.pipe((.7, 0, .3), (.7, 0, 6.6), .045, 'titanium', n=(8, 6, 4)[lod])
    for x, width in ((-.9, 1.5), (-1.9, 1.3)):
        g.box((x, 0, .37), (.3, width, .05), 'bronze', .012, 1, tint=.75)
    g.box((-2.8, 0, .08), (.3, .035, .8), 'bronze', 0, tint=.7)
    sail(g, (2.5, 0, .9), (.38, .5, 1.05), (.75, 0, 4.9), .14, (5, 3, 2)[lod], (3, 2, 1)[lod], .97)
    face(g, [(.7, 0, 6.6), (.7, 0, 6.42), (.22, 0, 6.5)], 'warm', (0, 1, 0))
    face(g, [(.7, 0, 6.6), (.7, 0, 6.42), (.22, 0, 6.5)], 'warm', (0, -1, 0))
    if lod < 2:
        g.pipe((-2.72, 0, .62), (-1.9, 0, .74), .02, 'bronze', n=5)
        g.box((1.7, 0, at(.82)[2] - .18), (1.3, .5, .03), 'bronze', 0, tint=.8)
    if lod == 0:
        g.pipe((.7, 0, 6.5), (2.62, 0, .86), .008, 'titanium', n=4)
        for side in (-1, 1):
            g.pipe((.7, 0, 4.9), (.5, side * .88, .46), .008, 'titanium', n=4)
        torus(g, (.7, 0, 1.2), .07, .015, 'bronze', n=10, sides=4)
    # Boom and main sail swing about the mast.
    g.part = 'rig'
    g.pipe((.7, 0, 1.22), (-2.3, 0, 1.32), .035, 'titanium', n=(8, 6, 4)[lod])
    sail(g, (.66, 0, 1.3), (-2.2, 0, 1.4), (.66, 0, 6.35), .26, (7, 4, 2)[lod], (5, 3, 2)[lod])
    g.part = 'structure'
    return {'navigation': {'colliders': [collider(-2.7, -.95, 2.7, .95, 1.5, -.3)]},
            'pivots': {'rig': (.7, 0, 1.25)}, 'occlusion': {'distance': 1.2}}


def harbour_buoy(g):
    """Lit channel buoy; the lantern is a separate part."""
    lod = g.lod
    n = (16, 10, 6)[lod]
    g.lathe((0, 0, 0), [(0, -.55), (.5, -.5), (.78, -.12), (.78, .22), (.62, .42), (.3, .48), (0, .48)], 'bronze', n=n, tint=.85)
    g.lathe((0, 0, 0), [(.785, -.02), (.785, .16)], 'marking', n=n)
    for i in range(4):
        a = i * pi / 2 + pi / 4
        g.pipe((.44 * cos(a), .44 * sin(a), .45), (.16 * cos(a), .16 * sin(a), 2.0), .028, 'titanium', n=(6, 5, 4)[lod])
    g.cyl((0, 0, 2.02), .22, .05, 'graphite', n=n)
    if lod < 2:
        torus(g, (0, 0, 1.2), .3, .02, 'titanium', n=(16, 10)[lod], sides=4)
        g.box((0, 0, 1.55), (.5, .03, .38), 'marking', 0)
        g.box((0, 0, 1.55), (.03, .5, .38), 'marking', 0)
    g.part = 'lamp'
    g.cyl((0, 0, 2.17), .1, .24, 'warm', n=(10, 8, 6)[lod])
    g.lathe((0, 0, 0), [(.13, 2.29), (.13, 2.32), (0, 2.4)], 'graphite', n=(10, 8, 6)[lod])
    g.part = 'structure'
    return {'navigation': {'colliders': [collider(-.8, -.8, .8, .8, 2.4, -.5)]},
            'pivots': {'lamp': (0, 0, 2.05)}, 'occlusion': {'distance': 1.0}}


def mooring_post(g):
    """Quay bollard with a coiled line."""
    lod = g.lod
    n = (16, 10, 6)[lod]
    g.box((0, 0, .025), (.62, .62, .05), 'iron', .01, 1)
    g.lathe((0, 0, 0), [(.2, .05), (.19, .1), (.13, .16), (.12, .42), (.22, .5), (.23, .58), (.12, .63), (0, .64)], 'iron', n=n,
            tint=1.6)
    if lod < 2:
        g.pipe((-.3, 0, .34), (.3, 0, .34), .04, 'iron', n=6, tint=1.6)
        for i, radius in enumerate((.3, .27, .24)):
            torus(g, (.72, .1, .035 + i * .06), radius, .033, 'tactile', tint=.9, n=(18, 12)[lod], sides=(5, 4)[lod])
        for i in range(4):
            a = i * pi / 2 + pi / 4
            g.cyl((.24 * cos(a), .24 * sin(a), .06), .03, .03, 'titanium', n=6)
    return {'navigation': {'colliders': [collider(-.3, -.3, .3, .3, .64)]}, 'occlusion': {'ground': 0.0}}


def flower_bed(g):
    """Raised bed with shrub mounds in bloom and ornamental grass, 3.2 x 1.4 m."""
    lod = g.lod
    w, d, h = 3.2, 1.4, .26
    draws = [g.rng.random() for _ in range(120)]
    for side in (-1, 1):
        g.box((0, side * (d/2 - .07), h/2), (w, .14, h), 'kerb', .025, 1)
        g.box((side * (w/2 - .07), 0, h/2), (.14, d - .28, h), 'kerb', .025, 1)
    g.grid([-w/2 + .14, w/2 - .14], [-d/2 + .14, d/2 - .14], h - .06, 'road', tint=.6)
    # Mounds, blossoms and grass form the soft part `foliage` (smooth normals).
    g.part = 'foliage'
    mounds = [(-1.05, .1, .36), (-.25, -.14, .42), (.55, .12, .38), (1.18, -.08, .32)]
    for i, (x, y, r) in enumerate(mounds):
        blob(g, (x, y, h + r * .3), r, tint=.95 + .12 * (i % 2), squash=.62, lumpy=.07)
    colours = ('warm', 'ivory', 'cyan', 'jade', 'warm', 'ivory', 'cyan')
    for i in range((30, 14, 4)[lod]):
        x, y, r = mounds[i % 4]
        phi, theta = .2 + 1.0 * draws[i * 2], 2 * pi * draws[i * 2 + 1]
        px, py = x + r * 1.02 * sin(phi) * cos(theta), y + r * 1.02 * sin(phi) * sin(theta)
        if abs(py) > d/2 - .2 or abs(px) > w/2 - .2:
            continue
        size = (.05 + .035 * draws[70 + i]) * (1, 1.15, 1.9)[lod]
        blob(g, (px, py, h + r * .3 + r * .64 * cos(phi) + size * .3), size, colours[i % 7], squash=.65,
             n=(7, 5, 4)[lod], steps=(4, 3, 2)[lod])
    if lod < 2:
        for i in range((12, 6)[lod]):
            x = -w/2 + .28 + draws[100 + i] * (w - .56)
            y = (-1, 1)[i % 2] * (d/2 - .27)
            g.cyl((x, y, h + .2), .08, .56, 'leaf', top=.004, n=5, caps=(False, False), tint=1.5 + .5 * draws[i])
    g.part = 'structure'
    return {'navigation': {'colliders': [collider(-w/2, -d/2, w/2, d/2, .9)]}, 'soft': ['foliage'],
            'occlusion': {'ground': 0.0}}


def cafe_set(g):
    """Round table with three chairs under a striped parasol."""
    lod = g.lod
    n = (16, 10, 6)[lod]
    g.lathe((0, 0, 0), [(.26, 0), (.26, .025), (.05, .06), (.035, .7), (.09, .72)], 'titanium', n=n)
    g.cyl((0, 0, .74), .42, .035, 'ceramic', n=n)
    g.pipe((0, 0, .74), (0, 0, 2.74), .022, 'titanium', n=(8, 6, 4)[lod])
    segments, radius, rim_z, top_z = (12, 8, 6)[lod], 1.45, 2.26, 2.74
    vertices = [(0, 0, top_z)] + [(radius * cos(2*pi*i/segments), radius * sin(2*pi*i/segments), rim_z) for i in range(segments)]
    faces = [(0, 1 + i, 1 + (i + 1) % segments) for i in range(segments)]
    tints = [(1.0, .68)[i % 2] for i in range(segments)]
    g.add(vertices, faces, 'ceramic', tint=tints)
    if lod < 2:
        g.add(vertices, [tuple(reversed(f)) for f in faces], 'ceramic', tint=[t * .8 for t in tints])
        g.cyl((0, 0, 2.78), .03, .09, 'bronze', n=6)
    for i in range(3):
        a = pi / 2 + i * 2 * pi / 3
        ca, sa = cos(a), sin(a)
        cx, cy = .8 * ca, .8 * sa
        g.box((cx, cy, .45), (.4, .42, .04), 'bronze', .012, 1, angle=a, tint=.85)
        g.box((cx + .19 * ca, cy + .19 * sa, .7), (.03, .4, .42), 'bronze', .012, 1, angle=a, tint=.85)
        if lod < 2:
            for dr in (-.17, .17):
                for dt in (-.18, .18):
                    g.box((cx + dr * ca - dt * sa, cy + dr * sa + dt * ca, .22), (.03, .03, .44), 'titanium', 0)
    if lod == 0:
        g.cyl((.16, .1, .79), .04, .06, 'ceramic', n=8)
        g.cyl((-.13, -.15, .79), .04, .06, 'ceramic', n=8, tint=.8)
        g.box((0, .02, .85), (.2, .012, .16), 'cyan', 0)
    return {'navigation': {'colliders': [collider(-1.0, -1.0, 1.0, 1.0, 1.0)]}, 'occlusion': {'ground': 0.0}}


def charge_post(g):
    """Charging post for residents: a lit pad and a pylon with a connector arm; faces -Y."""
    lod = g.lod
    n = (20, 12, 8)[lod]
    g.cyl((0, 0, .015), .72, .03, 'setts', n=n)
    g.lathe((0, 0, 0), [(.58, .031), (.62, .031)], 'ivory', n=n, up=True)
    g.box((0, .62, .06), (.5, .42, .12), 'stone', .03, 1)
    g.box((0, .62, .95), (.26, .3, 1.7), 'graphite', .05, 2)
    g.box((0, .62, 1.84), (.34, .38, .1), 'bronze', .03, 1)
    g.box((0, .465, 1.0), (.06, .012, 1.2), 'cyan', 0)
    if lod < 2:
        arm = [(0, .52, 1.62), (0, .3, 1.74), (0, .1, 1.62), (0, .06, 1.42)]
        for a, b in zip(arm, arm[1:]):
            g.pipe(a, b, .028, 'titanium', n=(8, 6)[lod])
        g.cyl((0, .06, 1.36), .05, .12, 'bronze', n=(10, 8)[lod])
        g.box((.135, .62, 1.5), (.012, .16, .1), 'jade', 0)
    if lod == 0:
        for i in range(4):
            g.box((0, .468, .5 + i * .32), (.16, .01, .02), 'titanium', 0)
        g.lathe((0, 0, 0), [(.29, .031), (.31, .031)], 'cyan', n=n, up=True)
    # The pylon stands behind the pad (Blender +Y, glTF -Z); the pad itself stays free to stand on.
    return {'navigation': {'colliders': [collider(-.26, .4, .26, .84, 1.9)], 'workpoints': [{'id': 'charge', 'position': [0, 0, 0]}]},
            'occlusion': {'ground': 0.0}}


def info_pylon(g):
    """Wayfinding pylon with a lit city map on both faces."""
    lod = g.lod
    g.box((0, 0, .06), (.96, .44, .12), 'stone', .03, 1)
    g.box((0, 0, 1.37), (.8, .16, 2.5), 'graphite', .04, 2)
    g.box((0, 0, 2.66), (.86, .2, .08), 'bronze', .02, 1)
    for side in (-1, 1):
        y = side * .083
        g.box((0, y, 1.6), (.64, .012, .9), 'cyan', 0, tint=.55)
        g.box((0, y, 2.3), (.64, .012, .22), 'ivory', 0)
        if lod < 2:
            for i in range(3):
                g.box((-.1 + i * .02, y, .95 - i * .14), (.42 - i * .08, .012, .06), 'ivory', 0)
        if lod == 0:
            # The map: city blocks between the two street pairs.
            for i in range(4):
                for j in range(4):
                    g.box((-.225 + i * .15, side * .092, 1.3 + j * .2), (.11, .008, .15), 'graphite', 0, tint=1.6)
            g.box((.08, side * .096, 1.72), (.04, .008, .04), 'warm', 0)
    return {'navigation': {'colliders': [collider(-.48, -.22, .48, .22, 2.7)]}, 'occlusion': {'ground': 0.0}}


def utility_cabinet(g):
    """Street cabinet with louvred doors."""
    lod = g.lod
    g.box((0, 0, .05), (1.3, .5, .1), 'stone', .02, 1)
    g.box((0, 0, .68), (1.2, .42, 1.16), 'titanium', .03, 1)
    g.box((0, 0, 1.285), (1.26, .48, .05), 'graphite', .015, 1)
    if lod < 2:
        g.box((0, -.212, .68), (.012, .01, 1.06), 'graphite', 0)
        for side in (-1, 1):
            g.box((side * .08, -.215, .7), (.02, .02, .16), 'bronze', 0)
        g.box((.46, -.213, 1.12), (.06, .008, .03), 'jade', 0)
    if lod == 0:
        for side in (-1, 1):
            for i in range(6):
                g.box((side * .3, -.213, .98 + i * .035), (.42, .012, .014), 'graphite', 0)
            g.box((side * .3, -.213, .4), (.42, .008, .3), 'titanium', 0, tint=.85)
    return {'navigation': {'colliders': [collider(-.65, -.25, .65, .25, 1.3)]}, 'occlusion': {'ground': 0.0}}


GLYPHS = (
    ((0, .24, .5, .05), (0, -.24, .5, .05), (-.225, 0, .05, .43), (.225, 0, .05, .43)),
    ((0, .2, .5, .05), (0, -.2, .36, .05), (0, 0, .05, .35)),
    ((0, .24, .5, .05), (0, 0, .05, .43), (0, -.24, .5, .05), (.16, .06, .14, .05)),
    ((0, .22, .5, .05), (0, 0, .34, .05), (0, -.22, .5, .05), (-.2, -.11, .05, .17)),
)


def neon_blade(g):
    """Vertical blade sign on two brackets, 0.9 x 4 m, glyphs on both faces; projects towards -Y."""
    lod = g.lod
    g.box((0, -.75, 2.0), (.14, .9, 4.0), 'graphite', .03, 1)
    for z in (.6, 3.4):
        g.box((0, -.15, z), (.06, .3, .06), 'titanium', 0)
        if lod < 2:
            g.box((0, -.01, z), (.2, .02, .2), 'titanium', 0)
    for side in (-1, 1):
        x = side * .074
        if lod == 2:
            g.box((x, -.75, 2.0), (.008, .56, 3.5), 'cyan', 0, tint=.55)
            continue
        for index, glyph in enumerate(GLYPHS):
            cz = 3.42 - index * .92
            for dy, dz, wide, high in glyph:
                g.box((x, -.75 + side * dy, cz + dz), (.02, wide, high), 'cyan' if index != 2 else 'warm', 0)
        for dy in (-.4, .4):
            g.box((x, -.75 + dy, 2.0), (.012, .025, 3.84), 'warm', 0, tint=.8)
    if lod < 2:
        g.box((0, -.75, 4.03), (.2, .96, .06), 'bronze', .015, 1)
        g.box((0, -.75, -.03), (.2, .96, .06), 'bronze', .015, 1)
    return {'occlusion': {'distance': 1.0}}


def neon_wave(g):
    """Wide wall sign with two luminous wave lines, 3.4 x 1.1 m."""
    lod = g.lod
    g.box((0, -.05, .55), (3.4, .08, 1.1), 'graphite', .02, 1)
    if lod < 2:
        for x in (-1.685, 1.685):
            g.box((x, -.06, .55), (.04, .12, 1.14), 'bronze', 0)
        for z in (-.005, 1.105):
            g.box((0, -.06, z), (3.4, .12, .03), 'bronze', 0)
    count = (28, 14, 6)[lod]
    for mat, phase, radius, amplitude in (('cyan', 0, .03, .26), ('jade', 1.9, .022, .16)):
        points = [(-1.25 + 2.7 * i / count, -.12, .55 + amplitude * sin(phase + 2.6 * pi * i / count)) for i in range(count + 1)]
        for a, b in zip(points, points[1:]):
            g.pipe(a, b, radius, mat, n=(6, 5, 3)[lod])
    for i in range(3):
        g.cyl((-1.5, -.1, .82 - i * .27), .06, .04, 'warm', rot=RX, n=(10, 8, 5)[lod])
    return {'occlusion': {'distance': .8}}


def neon_ring(g):
    """Round sign with concentric luminous rings, 1.8 m across; origin at its lowest point on the wall."""
    lod = g.lod
    n = (28, 16, 10)[lod]
    g.cyl((0, -.05, .9), .9, .08, 'graphite', rot=RX, n=n)
    torus(g, (0, -.12, .9), .74, .03, 'warm', RX, n=(36, 20, 12)[lod])
    torus(g, (0, -.12, .9), .5, .028, 'cyan', RX @ Matrix.Rotation(.9, 3, 'Z'), 1.5 * pi, n=(28, 16, 9)[lod])
    g.cyl((0, -.11, .9), .15, .04, 'ivory', rot=RX, n=(14, 10, 6)[lod])
    if lod < 2:
        torus(g, (0, -.07, .9), .9, .025, 'bronze', RX, n=28, sides=4)
        for i in range(12):
            a = i * pi / 6
            g.box((.62 * cos(a), -.1, .9 + .62 * sin(a)), (.09, .02, .02), 'ivory', 0,
                  rot=Matrix.Rotation(-a, 3, 'Y'))
    return {'occlusion': {'distance': .8}}


def banner_mast(g):
    """Mast with a cloth banner and luminous emblem between two arms; the banner is a separate part."""
    lod = g.lod
    n = (12, 8, 6)[lod]
    g.lathe((0, 0, 0), [(.24, 0), (.24, .08), (.13, .14), (.1, .5), (.08, .56)], 'graphite', n=n)
    g.cyl((0, 0, 3.75), .075, 6.5, 'titanium', top=.05, n=n, caps=(False, True))
    g.lathe((0, 0, 0), [(.05, 7.0), (.09, 7.08), (.06, 7.2), (0, 7.24)], 'bronze', n=n)
    for z in (6.6, 3.2):
        g.pipe((0, 0, z), (1.05, 0, z), .03, 'titanium', n=(8, 6, 4)[lod])
        if lod < 2:
            g.cyl((1.05, 0, z), .045, .08, 'bronze', n=6)
    g.part = 'banner'
    rows_count, columns = (8, 4, 1)[lod], (2, 1, 1)[lod]
    rows = [[(.14 + .86 * c / columns, 0, 6.55 - 3.3 * r / rows_count) for c in range(columns + 1)] for r in range(rows_count + 1)]
    # Matt off-white cloth (the marking paint's material) with a bronze emblem and a luminous stripe.
    sheet(g, rows, 'marking', lambda r, k: 1.0 if r % 2 else .9)
    if lod < 2:
        for s in (-1, 1):
            emblem = [[(.34 + .46 * c, s * .006, 5.9 - .5 * r) for c in range(2)] for r in range(2)]
            g.loft(emblem, ['bronze'], flip=s > 0)
            stripe = [[(.2 + .74 * c, s * .006, 3.75 - .12 * r) for c in range(2)] for r in range(2)]
            g.loft(stripe, ['cyan'], flip=s > 0)
    g.part = 'structure'
    return {'navigation': {'colliders': [collider(-.25, -.25, .25, .25, 7.2)]},
            'pivots': {'banner': (.14, 0, 6.55)}, 'occlusion': {'ground': 0.0, 'distance': 1.2}}


def gull(g):
    """Gull in glide, 1.5 m span; the wings are separate parts hinged at the shoulders. Flies towards +X."""
    lod = g.lod
    n = (8, 6, 4)[lod]
    g.lathe((0, 0, 0), [(0, -.3), (.045, -.24), (.095, -.08), (.105, .06), (.075, .18), (.06, .25), (.035, .3), (0, .32)],
            'ceramic', n=n, rot=RY)
    g.cyl((.36, 0, -.005), .022, .1, 'tactile', top=.003, rot=RY, n=(6, 5, 3)[lod], tint=1.3)
    tail = [(-.26, -.03, 0), (-.26, .03, 0), (-.45, .08, .0), (-.45, -.08, .0)]
    face(g, tail, 'ceramic', (0, 0, 1), .82)
    face(g, tail, 'ceramic', (0, 0, -1), .82)
    if lod == 0:
        for side in (-1, 1):
            g.box((.27, side * .045, .03), (.02, .012, .02), 'iron', 0)
    for part, side in (('wing_l', 1), ('wing_r', -1)):
        g.part = part
        rows = [[(.11, side * .06, .03), (-.13, side * .06, .03)],
                [(.16, side * .36, .07), (-.11, side * .36, .06)],
                [(.1, side * .6, .06), (-.1, side * .6, .05)],
                [(.0, side * .78, .03), (-.07, side * .76, .03)]]
        for mat, tint, pair in (('ceramic', .95, rows[:2]), ('ceramic', .86, rows[1:3]), ('graphite', 2.2, rows[2:])):
            g.loft(pair, [mat], [tint])
            g.loft(pair, [mat], [tint], flip=True)
    g.part = 'structure'
    return {'pivots': {'wing_l': (0, .06, .03), 'wing_r': (0, -.06, .03)}, 'occlusion': {'distance': .25, 'floor': .55}}


ASSETS = [
    ('harbour-launch', 'Harbour launch with a wheelhouse', 'vehicle', harbour_launch),
    ('harbour-barge', 'Cargo barge with data containers', 'vehicle', harbour_barge),
    ('sail-skiff', 'Wooden sailing skiff', 'vehicle', sail_skiff),
    ('harbour-buoy', 'Lit channel buoy', 'prop', harbour_buoy),
    ('mooring-post', 'Quay bollard with a coiled line', 'prop', mooring_post),
    ('flower-bed', 'Raised flower bed', 'nature', flower_bed),
    ('cafe-set', 'Cafe table with chairs and a parasol', 'prop', cafe_set),
    ('charge-post', 'Charging post for residents', 'prop', charge_post),
    ('info-pylon', 'Wayfinding pylon with a city map', 'prop', info_pylon),
    ('utility-cabinet', 'Street cabinet', 'prop', utility_cabinet),
    ('neon-blade', 'Vertical neon blade sign', 'prop', neon_blade),
    ('neon-wave', 'Wide neon wall sign', 'prop', neon_wave),
    ('neon-ring', 'Round neon sign', 'prop', neon_ring),
    ('banner-mast', 'Mast with a cloth banner', 'prop', banner_mast),
    ('gull', 'Gull with hinged wings', 'nature', gull),
]
