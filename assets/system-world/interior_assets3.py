"""System World kit v3: lobby interiors of the enterable district buildings (Blender 5.2 LTS).

Each interior is authored in the frame of its building (same origin, front towards -Y) and
is shown by the renderer in place of the building's `lobby` stand-in part. The building
itself already provides the podium (floor at PODIUM), the piers and the slab above; an
interior adds the floor finish, ceiling lights, the see-through glazing (`city.pane`) at
the position of the stand-in panes, sliding door leaves (`door.<n>.l` / `door.<n>.r`, each
a separate node with a pivot) and the furniture. Workpoints tell residents where to work;
colliders are [min x, min y, min z, max x, max y, max z] in glTF axes (Y up).
"""
from math import sin, cos, pi

import kit3
from facade_assets3 import PODIUM, perimeter, along
from furniture_assets3 import blob, collider
from life_assets3 import torus, face

# The transparent glazing material joins the palette when this family is registered.
PANE = 'pane' if 'pane' in kit3.PALETTE else 'glass'
FLOOR = PODIUM + .02


def turn(x, y, angle, dx, dy):
    """Point (dx, dy) of a piece that stands at (x, y) turned by `angle`."""
    c, s = cos(angle), sin(angle)
    return x + dx * c - dy * s, y + dx * s + dy * c


def workpoint(name, x, y):
    return {'id': name, 'position': [round(x, 3), round(FLOOR, 3), round(-y, 3)]}


def block(xa, xb, ya, yb, height):
    """Collider of a fixture on the lobby floor from two X and two Y limits in any order."""
    return collider(min(xa, xb), min(ya, yb), max(xa, xb), max(ya, yb), PODIUM + height, PODIUM)


def annulus(g, center, r0, r1, z, mat, down=False, n=None, tint=1.0):
    """Flat ring between two radii, facing up or down."""
    n = n or (36, 20, 10)[g.lod]
    vertices = [(r * cos(2*pi*i/n), r * sin(2*pi*i/n), z) for i in range(n) for r in (r0, r1)]
    faces = [(2*i, 2*i + 1, 2*((i + 1) % n) + 1, 2*((i + 1) % n)) for i in range(n)]
    if down:
        faces = [tuple(reversed(f)) for f in faces]
    g.add(vertices, faces, mat, (center[0], center[1], 0), tint=tint)


def arc_block(g, center, r0, r1, z0, z1, a0, a1, mats, n=None, tint=1.0):
    """Curved block between radii r0 < r1 and angles a0 < a1; mats = (outer, inner, top)."""
    n = n or (28, 14, 6)[g.lod]
    rows = []
    for i in range(n + 1):
        a = a0 + (a1 - a0) * i / n
        c, s = cos(a), sin(a)
        rows.append([(center[0] + r0*c, center[1] + r0*s, z0), (center[0] + r0*c, center[1] + r0*s, z1),
                     (center[0] + r1*c, center[1] + r1*s, z1), (center[0] + r1*c, center[1] + r1*s, z0)])
    g.loft(rows, [mats[1], mats[2], mats[0]], [tint] * 3, flip=True)
    face(g, rows[0], mats[0], (sin(a0), -cos(a0), 0), tint)
    face(g, rows[-1], mats[0], (-sin(a1), cos(a1), 0), tint)


def glazing(g, p0, u, s0, s1, z0, z1):
    a, b = along(p0, u, s0, -.45), along(p0, u, s1, -.45)
    g.quad((a[0], a[1], z0), (b[0], b[1], z0), (b[0], b[1], z1), (a[0], a[1], z1), PANE)


def shell(g, w, d, z1, doors, door_w, door_h, at=(0, 0), tile=1.15, floor=('kerb', 'stone'), border='bronze',
          light='ivory', pitch=2.8, lights=lambda x, y: True):
    """Floor finish, ceiling lights, glazing and door leaves of a rectangular lobby. Returns the door pivots."""
    lod = g.lod
    z0, glass_top = PODIUM, z1 - .9
    x0, x1, y0, y1 = at[0] - w/2 + .62, at[0] + w/2 - .62, at[1] - d/2 + .62, at[1] + d/2 - .62
    # Floor: two-tone tiles inside a border strip.
    if lod == 2:
        g.grid([x0, x1], [y0, y1], FLOOR, floor[0])
    else:
        size = tile * (1, 2)[lod]
        nx, ny = max(1, round((x1 - x0 - .5) / size)), max(1, round((y1 - y0 - .5) / size))
        xs = [x0] + [x0 + .25 + (x1 - x0 - .5) * i / nx for i in range(nx + 1)] + [x1]
        ys = [y0] + [y0 + .25 + (y1 - y0 - .5) * i / ny for i in range(ny + 1)] + [y1]

        def edge(i, j):
            return i in (0, len(xs) - 2) or j in (0, len(ys) - 2)

        g.grid(xs, ys, FLOOR, lambda i, j: border if edge(i, j) else floor[(i + j) % 2],
               lambda i, j: .75 if edge(i, j) else 1.0)
    # Ceiling: square light panels below the slab.
    if lod < 2:
        nx, ny = max(1, round((x1 - x0) / pitch)), max(1, round((y1 - y0) / pitch))
        for i in range(nx):
            for j in range(ny):
                cx, cy = x0 + (x1 - x0) * (i + .5) / nx, y0 + (y1 - y0) * (j + .5) / ny
                if lights(cx, cy):
                    g.grid([cx - .45, cx + .45], [cy - .45, cy + .45], glass_top - .02, light, flip=True)
                    if lod == 0:
                        g.grid([cx - .55, cx + .55], [cy - .55, cy + .55], glass_top - .012, 'graphite', 1.6, flip=True)
    # Glazing at the position of the stand-in panes, open at the doors, with a transom above each door.
    for index, (p0, u, width) in enumerate(perimeter(w, d, at)):
        gaps = sorted((width/2 + x - door_w/2 - .28, width/2 + x + door_w/2 + .28) for x in doors) if index == 0 else []
        start = .6
        for g0, g1 in gaps + [(width - .6, None)]:
            if g0 > start + .05:
                glazing(g, p0, u, start, g0, z0 + .35, glass_top)
            start = g1
        for g0, g1 in gaps:
            if glass_top - (z0 + door_h + .42) > .1:
                glazing(g, p0, u, g0, g1, z0 + door_h + .42, glass_top)
    # Sliding door leaves, authored closed.
    pivots = {}
    for n, x in enumerate(doors):
        cx, cy = at[0] + x, at[1] - d/2 + .25
        for tag, side in (('l', -1), ('r', 1)):
            g.part = f'door.{n}.{tag}'
            lx = cx + side * door_w / 4
            g.box((lx, cy, z0 + door_h/2), (door_w/2 - .03, .04, door_h - .04), PANE, 0)
            g.box((lx, cy, z0 + door_h - .05), (door_w/2 - .03, .07, .1), 'titanium', 0)
            g.box((lx, cy, z0 + .06), (door_w/2 - .03, .07, .12), 'titanium', 0)
            g.box((lx - side * (door_w/4 - .04), cy, z0 + door_h/2), (.05, .07, door_h), 'titanium', 0)
            if lod < 2:
                g.box((lx - side * (door_w/4 - .24), cy - .07, z0 + 1.2), (.04, .05, .7), 'bronze', 0)
            pivots[g.part] = (lx, cy, z0)
        g.part = 'structure'
    return pivots


# -- furniture ------------------------------------------------------------------------------
def sofa(g, x, y, width, angle, mat='bronze', tint=.72):
    """Upholstered bench with back and arms; sits facing its local -Y."""
    lod = g.lod
    z = FLOOR
    g.box((x, y, z + .06), (width, .86, .12), 'graphite', 0, angle=angle)
    g.box((x, y, z + .3), (width - .04, .84, .32), mat, .05, 1, angle=angle, tint=tint)
    bx, by = turn(x, y, angle, 0, .34)
    g.box((bx, by, z + .62), (width - .04, .2, .5), mat, .05, 1, angle=angle, tint=tint * .92)
    if lod < 2:
        for side in (-1, 1):
            ax, ay = turn(x, y, angle, side * (width/2 - .09), -.02)
            g.box((ax, ay, z + .44), (.18, .8, .36), mat, .05, 1, angle=angle, tint=tint * .85)
    if lod == 0:
        seats = max(1, round(width / .8))
        for i in range(1, seats):
            sx, sy = turn(x, y, angle, -width/2 + i * width / seats, -.08)
            g.box((sx, sy, z + .462), (.015, .62, .01), 'graphite', 0, angle=angle)


def low_table(g, x, y, radius=.5, mat='bronze'):
    n = (18, 10, 6)[g.lod]
    z = FLOOR
    g.cyl((x, y, z + .2), .07, .4, 'graphite', n=(8, 6, 4)[g.lod], caps=(False, False))
    g.cyl((x, y, z + .02), radius * .55, .04, 'graphite', n=n)
    g.cyl((x, y, z + .42), radius, .04, mat, n=n)


def pot_plant(g, x, y, size=1.0):
    lod = g.lod
    z = FLOOR
    g.lathe((x, y, 0), [(.2 * size, z), (.3 * size, z + .52 * size), (.27 * size, z + .55 * size), (0, z + .5 * size)],
            'ceramic', n=(12, 8, 6)[lod])
    blob(g, (x, y, z + 1.0 * size), .52 * size, tint=1.25, squash=1.0, lumpy=.24)
    if lod < 2:
        blob(g, (x + .22 * size, y - .12 * size, z + 1.45 * size), .34 * size, tint=1.45, squash=.9, lumpy=.24)
        blob(g, (x - .2 * size, y + .16 * size, z + .78 * size), .3 * size, tint=1.1, squash=.8, lumpy=.2)


def floor_lamp(g, x, y):
    lod = g.lod
    z = FLOOR
    g.cyl((x, y, z + .015), .16, .03, 'graphite', n=(12, 8, 6)[lod])
    g.cyl((x, y, z + .85), .018, 1.7, 'bronze', n=(6, 5, 4)[lod], caps=(False, False))
    blob(g, (x, y, z + 1.78), .17, 'ivory', squash=1.0, n=(10, 7, 5)[lod], steps=(6, 4, 3)[lod])


def totem(g, x, y, angle, screen='cyan', height=2.3):
    """Free-standing display with a lit screen on the face towards its local -Y."""
    lod = g.lod
    z = FLOOR
    g.box((x, y, z + .05), (1.1, .4, .1), 'graphite', .02, 1, angle=angle)
    g.box((x, y, z + height/2 + .1), (.96, .14, height), 'graphite', .03, 1, angle=angle)
    sx, sy = turn(x, y, angle, 0, -.073)
    g.box((sx, sy, z + height * .62), (.8, .012, height * .5), screen, 0, angle=angle, tint=.6)
    if lod < 2:
        g.box((sx, sy, z + height * .27), (.8, .012, .05), 'ivory', 0, angle=angle)
        g.box((x, y, z + height + .13), (1.0, .18, .06), 'bronze', .015, 1, angle=angle)
    if lod == 0:
        for i in range(4):
            lx, ly = turn(x, y, angle, -.22 + i * .15, -.08)
            g.box((lx, ly, z + height * (.5 + .07 * ((i * 3) % 4))), (.09, .01, height * (.12 + .05 * (i % 3))), 'graphite', 0,
                  angle=angle, tint=1.6)


# -- lobbies --------------------------------------------------------------------------------
def lobby_agent(g):
    """Agent core: a reception ring around the reactor column, two lounges and two briefing corners."""
    lod = g.lod
    top = 5.0
    pivots = shell(g, 15, 15, 5.9, (0.0,), 4.0, 3.5, lights=lambda x, y: x*x + y*y > 30)
    z = FLOOR
    # Reactor column through the hall.
    n = (24, 14, 8)[lod]
    g.cyl((0, 0, (z + top) / 2), 1.15, top - z, 'graphite', n=n, caps=(False, False))
    for i in range((8, 8, 4)[lod]):
        a = i * 2 * pi / (8, 8, 4)[lod] + pi / 8
        g.box((1.155 * cos(a), 1.155 * sin(a), (z + top) / 2), (.03, .24, top - z - .7), 'cyan', 0, angle=a)
    if lod < 2:
        torus(g, (0, 0, z + .1), 1.24, .1, 'bronze')
        torus(g, (0, 0, top - .1), 1.24, .1, 'bronze')
    # Ring light above the desk and inlaid rings in the floor.
    annulus(g, (0, 0), 3.7, 4.05, top - .02, 'ivory', down=True)
    if lod < 2:
        annulus(g, (0, 0), 4.6, 4.68, z + .012, 'bronze', tint=.8)
        annulus(g, (0, 0), 5.9, 5.96, z + .012, 'bronze', tint=.8)
    # Reception desk: a ring open towards the back.
    a0, a1 = pi/2 + .55, pi/2 + 2*pi - .55
    arc_block(g, (0, 0), 2.75, 3.3, z, z + 1.02, a0, a1, ('ceramic', 'graphite', 'graphite'))
    arc_block(g, (0, 0), 2.68, 3.4, z + 1.02, z + 1.09, a0, a1, ('bronze', 'bronze', 'bronze'))
    if lod < 2:
        arc_block(g, (0, 0), 3.3, 3.325, z + .08, z + .14, a0, a1, ('cyan', 'cyan', 'cyan'), tint=.8)
        for a in (-pi/2 - .75, -pi/2, -pi/2 + .75):
            x, y = 2.95 * cos(a), 2.95 * sin(a)
            g.box((x, y, z + 1.32), (.5, .03, .32), 'graphite', .01, 1, angle=a + pi/2)
            g.box((x - .02 * cos(a), y - .02 * sin(a), z + 1.32), (.44, .012, .26), 'cyan', 0, angle=a + pi/2, tint=.6)
            g.box((x, y, z + 1.13), (.06, .06, .1), 'graphite', 0, angle=a + pi/2)
    # Lounges at the front, briefing corners at the back.
    for side in (-1, 1):
        sofa(g, side * 5.6, -3.6, 2.6, -side * pi / 2)
        sofa(g, side * 4.3, -5.55, 2.2, pi)
        low_table(g, side * 4.3, -3.9)
        pot_plant(g, side * 6.0, -5.75, 1.1)
        floor_lamp(g, side * 5.95, -1.9)
        totem(g, side * 5.2, 4.6, -side * .85)
        totem(g, side * 3.1, 5.9, 0)
        pot_plant(g, side * 6.05, 6.0, .95)
        sofa(g, side * 5.75, 2.1, 2.0, -side * pi / 2, 'titanium', .8)
    colliders = [block(-1.2, 1.2, -1.2, 1.2, top - PODIUM), block(-3.4, 3.4, -3.4, -2.5, 1.1),
                 block(-3.4, -2.5, -2.5, 1.9, 1.1), block(2.5, 3.4, -2.5, 1.9, 1.1)]
    for side in (-1, 1):
        colliders += [block(side * 5.15, side * 6.05, -4.9, -2.3, 1.0), block(side * 3.2, side * 5.4, -6.0, -5.1, 1.0),
                      block(side * 5.3, side * 6.2, 1.1, 3.1, 1.0), block(side * 4.6, side * 5.8, 4.2, 5.0, 2.4),
                      block(side * 2.6, side * 3.6, 5.7, 6.1, 2.4)]
    points = [workpoint('reception-west', -1.35, -1.5), workpoint('reception-east', 1.35, -1.5),
              workpoint('briefing-west', -4.5, 3.6), workpoint('briefing-east', 4.5, 3.6),
              workpoint('lounge-west', -4.3, -2.9), workpoint('lounge-east', 4.3, -2.9)]
    return {'navigation': {'colliders': colliders, 'workpoints': points}, 'pivots': pivots,
            'occlusion': {'ground': PODIUM, 'distance': 1.8}}


ASSETS = [
    ('lobby-agent', 'Agent core lobby', 'interior', lobby_agent),
]
