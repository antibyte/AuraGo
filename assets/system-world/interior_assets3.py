"""System World kit v3: lobby interiors of the enterable district buildings (Blender 5.2 LTS).

Each interior is authored in the frame of its building (same origin, front towards -Y) and
is shown by the renderer in place of the building's `lobby` stand-in part. The building
itself already provides the podium (floor at PODIUM), the piers and the slab above; an
interior adds the floor finish, ceiling lights, the see-through glazing (`city.pane`) at
the position of the stand-in panes, sliding door leaves (`door_<n>_l` / `door_<n>_r`, each
a separate node with a pivot) and the furniture. Workpoints tell residents where to work;
colliders are [min x, min y, min z, max x, max y, max z] in glTF axes (Y up).
"""
from math import sin, cos, pi

from mathutils import Matrix

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
          light='ivory', pitch=2.8, lights=lambda x, y: True, first=0):
    """Floor finish, ceiling lights, glazing and door leaves of a rectangular lobby.

    Returns the door pivots; `first` is the number of the first door (for a second hall in one asset).
    """
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
    for n, x in enumerate(doors, first):
        cx, cy = at[0] + x, at[1] - d/2 + .25
        for tag, side in (('l', -1), ('r', 1)):
            g.part = f'door_{n}_{tag}'
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
    points = [workpoint('reception-west', -1.85, -1.0), workpoint('reception-east', 1.85, -1.0),
              workpoint('briefing-west', -4.5, 3.6), workpoint('briefing-east', 4.5, 3.6),
              workpoint('lounge-west', -4.3, -2.9), workpoint('lounge-east', 4.3, -2.9)]
    return {'navigation': {'colliders': colliders, 'workpoints': points}, 'pivots': pivots,
            'occlusion': {'ground': PODIUM, 'distance': 1.8}}


def cabinet(g, x, y, angle, size, lights=('cyan', 'jade'), rows=4, cap='bronze'):
    """Server rack row or archive stack with lit strips and cassettes on both long faces; size = (length, depth, height)."""
    lod = g.lod
    z = FLOOR
    length, depth, height = size
    g.box((x, y, z + height/2), size, 'graphite', .02, 1, angle=angle)
    if lod == 2:
        return
    g.box((x, y, z + height + .03), (length + .06, depth + .06, .06), cap, 0, angle=angle)
    cells = max(2, round(length / .9))
    for side in (-1, 1):
        for r in range(rows):
            level = z + height * (r + .5) / rows
            px, py = turn(x, y, angle, 0, side * (depth/2 + .006))
            g.box((px, py, level + height / rows * .38), (length - .12, .012, .03), lights[r % 2], 0, angle=angle, tint=.7)
            if lod == 0:
                for c in range(cells):
                    if (c * 7 + r * 3 + (side > 0)) % 4 == 0:
                        continue
                    cx, cy = turn(x, y, angle, -length/2 + (c + .5) * length / cells, side * (depth/2 + .008))
                    g.box((cx, cy, level - .03), (length / cells - .08, .014, height / rows * .56),
                          ('ceramic', 'titanium', 'ceramic', 'stone')[(c + r) % 4], 0, angle=angle, tint=.9)


def counter(g, x, y, width, angle, top='bronze', front='ceramic', screens=1, glow='cyan'):
    """Service counter: customers stand at its local -Y, staff at +Y."""
    lod = g.lod
    z = FLOOR
    g.box((x, y, z + .5), (width, .7, 1.0), front, .03, 1, angle=angle)
    g.box((x, y, z + 1.03), (width + .1, .82, .06), top, .015, 1, angle=angle)
    if lod == 2:
        return
    fx, fy = turn(x, y, angle, 0, -.356)
    g.box((fx, fy, z + .12), (width - .1, .012, .05), glow, 0, angle=angle, tint=.8)
    for i in range(screens):
        offset = (i - (screens - 1) / 2) * 1.1
        sx, sy = turn(x, y, angle, offset, .12)
        g.box((sx, sy, z + 1.28), (.46, .03, .3), 'graphite', .01, 1, angle=angle)
        qx, qy = turn(x, y, angle, offset, .138)
        g.box((qx, qy, z + 1.28), (.4, .012, .24), 'cyan', 0, angle=angle, tint=.6)


def table(g, x, y, width, depth, angle, lamp=True, stools=0):
    lod = g.lod
    z = FLOOR
    g.box((x, y, z + .74), (width, depth, .05), 'bronze', .012, 1, angle=angle, tint=.85)
    for side in (-1, 1):
        lx, ly = turn(x, y, angle, side * (width/2 - .2), 0)
        g.box((lx, ly, z + .36), (.06, depth - .2, .72), 'graphite', 0, angle=angle)
    if lamp and lod < 2:
        g.box((x, y, z + 1.16), (width - .5, .05, .025), 'ivory', 0, angle=angle)
        for side in (-1, 1):
            lx, ly = turn(x, y, angle, side * (width/2 - .3), 0)
            g.box((lx, ly, z + .96), (.02, .02, .4), 'bronze', 0, angle=angle)
    for i in range(stools if lod < 2 else 0):
        for side in (-1, 1):
            sx, sy = turn(x, y, angle, -width/2 + (i + .5) * width / stools, side * (depth/2 + .32))
            g.cyl((sx, sy, z + .25), .03, .5, 'graphite', n=(6, 5)[lod], caps=(False, False))
            g.cyl((sx, sy, z + .5), .17, .05, 'titanium', n=(12, 8)[lod])


def lobby_memory(g):
    """Archive hall: glowing data stacks, reading tables and the memory core on its plinth."""
    lod = g.lod
    at = (0, 1)
    pivots = shell(g, 22, 18, 7.0, (-5, 5), 2.8, 3.5, at=at, pitch=3.3, floor=('ceramic', 'kerb'),
                   lights=lambda x, y: abs(x) > 1.5 or abs(y - 1) > 2.5)
    z = FLOOR
    # Memory core: plinth, glowing core and orbit rings.
    n = (20, 12, 8)[lod]
    g.lathe((0, 1, 0), [(1.5, z), (1.5, z + .16), (1.05, z + .3), (.95, z + .9), (1.15, z + 1.0), (0, z + 1.0)], 'graphite', n=n)
    blob(g, (0, 1, z + 2.0), .62, 'jade', squash=1.0, n=(14, 10, 6)[lod], steps=(8, 6, 4)[lod], lumpy=.12)
    annulus(g, (0, 1), .5, 1.0, z + 1.004, 'jade', tint=.5)
    if lod < 2:
        torus(g, (0, 1, z + 2.0), 1.05, .03, 'bronze')
        torus(g, (0, 1, z + 2.0), 1.25, .025, 'ivory', rot=Matrix.Rotation(1.1, 3, 'X'))
        annulus(g, (0, 1), 2.6, 2.68, z + .012, 'bronze', tint=.8)
    # Stacks in the back half, reading tables at the front, an enquiry desk between the doors.
    colliders = [block(-1.5, 1.5, -.5, 2.5, 2.8)]
    for side in (-1, 1):
        for i, x in enumerate((3.4, 5.9, 8.4)):
            cabinet(g, side * x, 5.2, pi / 2, (6.2, .8, 3.9), ('jade', 'ivory'), rows=5)
            colliders.append(block(side * (x - .45), side * (x + .45), 2.0, 8.4, 4.0))
        table(g, side * 6.6, -3.4, 4.4, 1.2, 0, stools=4)
        colliders.append(block(side * 4.3, side * 8.9, -4.4, -2.4, .8))
        pot_plant(g, side * 9.5, -6.5, 1.15)
        pot_plant(g, side * 9.6, 8.7, 1.0)
        floor_lamp(g, side * 9.55, -1.0)
    counter(g, 0, -4.8, 3.6, 0, screens=2, glow='jade')
    colliders.append(block(-1.9, 1.9, -5.3, -4.3, 1.1))
    points = [workpoint('enquiry', 0, -3.4), workpoint('core', 0, -1.3), workpoint('stacks-west', -4.65, 4.2),
              workpoint('stacks-east', 4.65, 4.2), workpoint('reading-west', -6.6, -1.5), workpoint('reading-east', 6.6, -1.5)]
    return {'navigation': {'colliders': colliders, 'workpoints': points}, 'pivots': pivots,
            'occlusion': {'ground': PODIUM, 'distance': 1.8}}


def lobby_integrations(g):
    """Two gate halls: a switchboard in the west tower, a service counter in the east tower."""
    lod = g.lod
    z = FLOOR
    pivots = shell(g, 6, 9, 5.2, (0.0,), 2.0, 2.92, at=(-8, 0), tile=1.0, pitch=2.4)
    pivots.update(shell(g, 6, 9, 5.2, (0.0,), 2.0, 2.92, at=(8, 0), tile=1.0, pitch=2.4, first=1))
    # West: switchboard wall with jacks and patch cords, operator desk.
    g.box((-8, 3.55, z + 1.5), (4.3, .3, 2.7), 'graphite', .03, 1)
    g.box((-8, 3.55, z + 2.9), (4.4, .36, .08), 'bronze', .02, 1)
    if lod < 2:
        columns, rows = (14, 7)[lod], (7, 4)[lod]
        for c in range(columns):
            for r in range(rows):
                if (c * 3 + r * 5) % 7 == 0:
                    continue
                g.box((-8 - 1.95 + (c + .5) * 3.9 / columns, 3.39, z + .5 + (r + .5) * 2.2 / rows),
                      (.11, .012, .11), ('jade', 'cyan', 'ivory', 'jade', 'warm')[(c + 2 * r) % 5], 0, tint=.75)
    if lod == 0:
        for i in range(5):
            x0, x1 = -9.6 + i * .7, -8.9 + i * .62
            g.pipe((x0, 3.37, z + 2.3 - i * .22), (x1, 3.3, z + 1.0 + i * .12), .014, 'tactile', n=4)
    counter(g, -8, 1.4, 2.0, pi, screens=1, glow='jade')
    sofa(g, -10.0, -1.6, 1.8, pi / 2, 'titanium', .8)
    pot_plant(g, -6.1, -3.3, .9)
    # East: service counter, parcel shelves behind it, a queue totem.
    counter(g, 6.85, 1.2, 2.4, 0, screens=2)
    cabinet(g, 8, 3.5, 0, (4.0, .5, 2.6), ('cyan', 'ivory'), rows=4)
    totem(g, 9.8, -2.4, -pi / 2, height=2.0)
    pot_plant(g, 6.1, -3.3, .9)
    colliders = [block(-10.2, -5.8, 3.3, 3.9, 2.9), block(-9.05, -6.95, .9, 1.9, 1.1), block(-10.4, -9.5, -2.6, -.6, 1.0),
                 block(5.65, 8.05, .7, 1.7, 1.1), block(5.9, 10.1, 3.2, 3.8, 2.7), block(9.5, 10.1, -3.0, -1.8, 2.1)]
    points = [workpoint('switchboard', -8, 2.6), workpoint('patching', -7.0, .1), workpoint('counter', 6.9, 2.4),
              workpoint('queue', 6.9, -.2)]
    return {'navigation': {'colliders': colliders, 'workpoints': points}, 'pivots': pivots,
            'occlusion': {'ground': PODIUM, 'distance': 1.6}}


def lobby_missions(g):
    """Departure hall: the big board, four counters behind the four doors, a parcel belt along the back."""
    lod = g.lod
    at = (0, 1)
    doors = (-9, -3, 3, 9)
    pivots = shell(g, 26, 15, 7.0, doors, 4.0, 3.5, at=at, pitch=3.2, tile=1.3)
    z = FLOOR
    # Departure board hung from the slab.
    g.box((0, 3.6, 4.5), (10.5, .24, 2.3), 'graphite', .03, 1)
    for x in (-4.4, 4.4):
        g.pipe((x, 3.6, 5.65), (x, 3.6, 6.1), .03, 'titanium')
    if lod == 2:
        g.box((0, 3.47, 4.5), (10.0, .012, 1.8), 'warm', 0, tint=.3)
    else:
        for r in range(6):
            level = 5.33 - r * .33
            if lod == 1:
                g.box((0, 3.47, level), (9.9, .012, .17), 'warm', 0, tint=.55)
                continue
            for c in range(14):
                if (c * 5 + r * 3) % 7 == 0:
                    continue
                g.box((-4.95 + (c + .5) * 9.9 / 14, 3.47, level), (9.9 / 14 - .06, .012, .17),
                      'cyan' if c == 0 else 'warm', 0, tint=.7)
        g.box((0, 3.47, 3.52), (9.9, .012, .05), 'ivory', 0)
    # Counters and belt.
    colliders = []
    for i, x in enumerate(doors):
        counter(g, x, 1.4, 3.0, 0, screens=1, glow='warm')
        colliders.append(block(x - 1.6, x + 1.6, .9, 1.9, 1.1))
        if lod < 2:
            g.box((x + 1.9, 1.4, z + .12), (.7, .9, .24), 'titanium', .02, 1)
            g.box((x, 1.75, z + 2.5), (1.6, .05, .34), 'graphite', 0)
            g.box((x, 1.72, z + 2.5), (1.4, .012, .22), 'cyan', 0, tint=.6)
            g.pipe((x - .7, 1.75, z + 2.67), (x - .7, 1.75, 6.1), .012, 'titanium', n=4)
            g.pipe((x + .7, 1.75, z + 2.67), (x + .7, 1.75, 6.1), .012, 'titanium', n=4)
    g.box((0, 7.0, z + .4), (21, 1.0, .8), 'titanium', .03, 1, tint=.8)
    g.box((0, 7.0, z + .81), (20.6, .8, .03), 'road', 0, tint=1.5)
    colliders.append(block(-10.6, 10.6, 6.4, 7.6, 1.5))
    crates = (('bronze', .8, .7), ('ceramic', .9, .5), ('titanium', .9, .6), ('graphite', 1.6, .55), ('bronze', .65, .45))
    for i in range((12, 7, 0)[lod]):
        mat, tint, size = crates[i % 5]
        g.box((-9.6 + i * (19.2 / (11, 6, 1)[lod]), 7.0 + .08 * ((i * 3) % 3 - 1), z + .83 + size / 2), (size * 1.25, size, size), mat,
              .03, 1, tint=tint)
    for side in (-1, 1):
        for row in (-2.4, -4.3):
            sofa(g, side * 6.0, row, 2.4, pi, 'titanium', .78)
            colliders.append(block(side * 4.7, side * 7.3, row - .5, row + .5, 1.0))
        pot_plant(g, side * 11.6, -4.9, 1.2)
        pot_plant(g, side * 11.6, 5.2, 1.0)
        totem(g, side * 12.0, -1.0, -side * pi / 2, screen='warm' if side < 0 else 'cyan')
    points = [workpoint('counter-%d' % (i + 1), x, 2.5) for i, x in enumerate(doors)]
    points += [workpoint('belt-west', -5.5, 5.6), workpoint('belt-east', 5.5, 5.6), workpoint('waiting', 0, -3.4)]
    return {'navigation': {'colliders': colliders, 'workpoints': points}, 'pivots': pivots,
            'occlusion': {'ground': PODIUM, 'distance': 1.8}}


def lobby_infra(g):
    """Machine hall: two rows of racks under coolant pipes, a control desk between the doors."""
    lod = g.lod
    at = (0, 3)
    pivots = shell(g, 20, 12, 5.4, (-3.75, 3.75), 2.4, 3.12, at=at, pitch=3.0, floor=('stone', 'setts'), border='titanium')
    z = FLOOR
    colliders = []
    for row, y in enumerate((5.0, 7.4)):
        for i in range(4):
            x = -6.9 + i * 4.6
            cabinet(g, x, y, 0, (3.2, .95, 2.6), ('cyan', 'jade'), rows=4, cap='titanium')
            colliders.append(block(x - 1.65, x + 1.65, y - .5, y + .5, 2.7))
    # Cold aisle grating between the rows and coolant pipes overhead.
    g.grid([-9.2, 9.2], [5.75, 6.65], z + .012, 'iron', tint=1.8)
    if lod == 0:
        for i in range(46):
            g.grid([-9.1 + i * .4, -8.9 + i * .4], [5.8, 6.6], z + .016, 'iron', tint=.6)
    for y, mat in ((5.0, 'titanium'), (7.4, 'titanium')):
        g.pipe((-9.2, y, 4.05), (9.2, y, 4.05), .13, mat, n=(10, 7, 5)[lod])
        if lod < 2:
            for i in range(4):
                x = -6.9 + i * 4.6
                g.pipe((x, y, 3.92), (x, y, z + 2.66), .06, 'titanium', n=(8, 6)[lod])
                torus(g, (x + 1.0, y, 4.05), .135, .02, 'cyan', rot=Matrix.Rotation(pi / 2, 3, 'Y'), n=10, sides=4)
    # Control desk.
    counter(g, 0, .6, 5.0, pi, top='graphite', front='titanium', screens=4)
    colliders.append(block(-2.6, 2.6, .1, 1.1, 1.1))
    if lod < 2:
        g.box((0, 1.6, z + 2.3), (3.6, .08, 1.1), 'graphite', .02, 1)
        g.box((0, 1.55, z + 2.3), (3.4, .012, .9), 'cyan', 0, tint=.45)
        for x in (-1.6, 1.6):
            g.pipe((x, 1.6, z + 2.85), (x, 1.6, 4.5), .02, 'titanium', n=5)
    for side in (-1, 1):
        totem(g, side * 8.6, -1.3, -side * pi / 2, height=2.0)
        pot_plant(g, side * 8.9, 1.0, .9)
    points = [workpoint('console-west', -1.2, -.7), workpoint('console-east', 1.2, -.7), workpoint('aisle-west', -4.8, 6.2),
              workpoint('aisle-east', 4.8, 6.2), workpoint('racks-front', 0, 3.9)]
    return {'navigation': {'colliders': colliders, 'workpoints': points}, 'pivots': pivots,
            'occlusion': {'ground': PODIUM, 'distance': 1.6}}


def lobby_graph(g):
    """Garden court around the knowledge tree, open to the sky: paths, beds, benches and reading terminals."""
    lod = g.lod
    z = FLOOR
    sides, radius, wall = 12, 9.45, 4.7
    angles = [-pi / 2 - pi / sides + i * 2 * pi / sides for i in range(sides + 1)]
    # Paved court with an inlaid ring; the tree's own plinth stays visible in the middle.
    annulus(g, (0, 0), 2.75, 9.05, z + .012, 'kerb', n=(48, 24, 12)[lod])
    if lod < 2:
        annulus(g, (0, 0), 4.1, 4.18, z + .024, 'bronze', tint=.8)
        annulus(g, (0, 0), 7.0, 7.08, z + .024, 'bronze', tint=.8)
    # Glazing in every wall face except the front one, which holds the doors.
    for i in range(1, sides):
        a0, a1 = angles[i], angles[i + 1]
        p0, p1 = (radius * cos(a0), radius * sin(a0)), (radius * cos(a1), radius * sin(a1))
        width = ((p1[0] - p0[0]) ** 2 + (p1[1] - p0[1]) ** 2) ** .5
        u = ((p1[0] - p0[0]) / width, (p1[1] - p0[1]) / width)
        a, b = along(p0, u, .35, -.12), along(p0, u, width - .35, -.12)
        g.quad((a[0], a[1], PODIUM + .35), (b[0], b[1], PODIUM + .35), (b[0], b[1], wall), (a[0], a[1], wall), PANE)
    pivots = {}
    cy = -radius * cos(pi / sides) - .1 + .25
    for tag, side in (('l', -1), ('r', 1)):
        g.part = f'door_0_{tag}'
        lx = side * .8
        g.box((lx, cy, PODIUM + 1.55), (1.57, .04, 3.06), PANE, 0)
        g.box((lx, cy, PODIUM + 3.05), (1.57, .07, .1), 'titanium', 0)
        g.box((lx, cy, PODIUM + .06), (1.57, .07, .12), 'titanium', 0)
        g.box((lx - side * .76, cy, PODIUM + 1.55), (.05, .07, 3.1), 'titanium', 0)
        if lod < 2:
            g.box((lx - side * .56, cy - .07, PODIUM + 1.2), (.04, .05, .7), 'bronze', 0)
        pivots[g.part] = (lx, cy, PODIUM)
    g.part = 'structure'
    # Beds along the wall, benches around the tree, terminals between them.
    colliders = [block(-1.6, 1.6, -1.6, 1.6, 4.0)]
    colours = ('warm', 'ivory', 'cyan', 'jade')
    for i in range(5):
        a = pi / 2 + (i - 2) * .62
        arc_block(g, (0, 0), 7.7, 8.95, z, z + .34, a - .24, a + .24, ('kerb', 'kerb', 'road'), n=(8, 5, 3)[lod])
        for k in range((5, 3, 1)[lod]):
            b = a - .17 + .34 * k / max(1, (4, 2, 1)[lod])
            r = 8.3 + .2 * ((k * 3) % 3 - 1)
            blob(g, (r * cos(b), r * sin(b), z + .6), .42, tint=1.2 + .15 * (k % 2), squash=.75, lumpy=.24)
            if lod < 2:
                blob(g, ((r - .25) * cos(b + .03), (r - .25) * sin(b + .03), z + .98), .09, colours[(i + k) % 4], squash=.7,
                     n=(7, 5)[lod], steps=(4, 3)[lod])
        colliders.append(block(8.3 * cos(a) - 1.4, 8.3 * cos(a) + 1.4, 8.3 * sin(a) - 1.2, 8.3 * sin(a) + 1.2, .8))
    for i in range(4):
        a = pi / 4 + i * pi / 2
        arc_block(g, (0, 0), 5.6, 6.15, z, z + .44, a - .22, a + .22, ('bronze', 'bronze', 'bronze'), n=(8, 5, 3)[lod], tint=.8)
        if lod < 2:
            arc_block(g, (0, 0), 6.15, 6.25, z + .44, z + .86, a - .22, a + .22, ('bronze', 'bronze', 'bronze'), n=(8, 5)[lod], tint=.7)
        colliders.append(block(5.9 * cos(a) - 1.0, 5.9 * cos(a) + 1.0, 5.9 * sin(a) - 1.0, 5.9 * sin(a) + 1.0, .9))
    for i, a in enumerate((pi * .04, pi * .96, -pi * .1, -pi * .9)):
        x, y = 6.9 * cos(a), 6.9 * sin(a)
        totem(g, x, y, a - pi / 2, screen=('jade', 'cyan')[i % 2], height=2.0)
        colliders.append(block(x - .6, x + .6, y - .6, y + .6, 2.1))
        floor_lamp(g, 6.4 * cos(a + .32), 6.4 * sin(a + .32))
    points = [workpoint('terminal-east', 5.4, .75), workpoint('terminal-west', -5.4, .75), workpoint('tree-south', 0, -3.4),
              workpoint('tree-north', 0, 3.5), workpoint('garden', 2.6, 6.4)]
    return {'navigation': {'colliders': colliders, 'workpoints': points}, 'pivots': pivots,
            'occlusion': {'ground': PODIUM, 'distance': 1.6}}


ASSETS = [
    ('lobby-agent', 'Agent core lobby', 'interior', lobby_agent),
    ('lobby-memory', 'Archive hall', 'interior', lobby_memory),
    ('lobby-integrations', 'Gate halls', 'interior', lobby_integrations),
    ('lobby-missions', 'Departure hall', 'interior', lobby_missions),
    ('lobby-infra', 'Machine hall', 'interior', lobby_infra),
    ('lobby-graph', 'Knowledge garden court', 'interior', lobby_graph),
]
