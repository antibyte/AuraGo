"""System World kit v3: the district buildings (Blender 5.2 LTS).

Each building keeps the footprint, podium, tier heights and landmark parts of its v1
design, so navigation footprints, the sky deck, the lift shaft, the memory hologram and
labels stay valid. What changes is everything one sees from the street: glazed lobbies
between piers, slab-and-glass storeys with room coordinates, terraces with planting,
roof plant and real entrances. Fronts face -Y (glTF +Z).

Ground-floor glazing and door leaves form the separate part `lobby`: a stand-in that the
renderer hides once the real interior of an enterable building (its own kit asset with
see-through glass and sliding doors) is shown. Every door is at least 2.9 m high, so the
visitor's eye (2.4 m above the floor) passes below the lintel.
"""
from math import sin, cos, pi, atan2, hypot

from mathutils import Matrix

from facade_assets3 import (PODIUM, FACE_OFFSET, perimeter, along, pane, face_box, podium, stairs,
                            planter, glass_tier, terrace, roof_unit)


def sphere(g, center, radius, mat, n=None, tint=1.0, squash=1.0):
    steps = (8, 6, 4)[g.lod]
    profile = [(radius * sin(pi * i / steps), -radius * squash * cos(pi * i / steps)) for i in range(steps + 1)]
    profile[0], profile[-1] = (0, profile[0][1]), (0, profile[-1][1])
    g.lathe(center, profile, mat, n=n or (16, 10, 6)[g.lod], tint=tint)


def portal(g, x, y, z0, door_w, accent, canopy=True, height=3.5):
    """Bronze-framed glazed doors on a -Y facade line at (x, y), with an optional lit canopy."""
    lod = g.lod
    leaves = max(2, round(door_w / 1.1))
    # Closed door leaves: part of the stand-in.
    g.part = 'lobby'
    g.box((x, y + .25, z0 + height/2), (door_w, .1, height), 'glass', 0)
    if lod < 2:
        for j in range(1, leaves):
            g.box((x - door_w/2 + j * door_w / leaves, y + .2, z0 + height/2), (.05, .08, height), 'titanium', 0)
    if lod == 0:
        for j in range(leaves):
            for side in (-1, 1):
                g.box((x - door_w/2 + (j + .5) * door_w / leaves + side * .32, y + .14, z0 + 1.3), (.04, .05, .8), 'titanium', 0)
    g.part = 'structure'
    for side in (-1, 1):
        g.box((x + side * (door_w/2 + .14), y + .12, z0 + height/2 + .1), (.28, .7, height + .2), 'bronze', .04, 1)
    g.box((x, y + .12, z0 + height + .28), (door_w + .56, .7, .28), 'bronze', .04, 1)
    if lod < 2:
        for side in (-1, 1):
            g.box((x + side * (door_w/2 + .62), y - .08, z0 + height/2 + .15), (.1, .16, height - .2), accent, 0)
        if canopy:
            g.box((x, y - 1.25, z0 + height + .65), (door_w + 2.6, 2.9, .18), 'graphite', .06, 1)
            g.box((x, y - 2.68, z0 + height + .6), (door_w + 2.6, .07, .34), 'bronze', 0)
            g.box((x, y - 1.25, z0 + height + .54), (door_w + 2.0, 2.3, .03), 'ivory', 0)
        else:
            g.box((x, y - .1, z0 + height + .62), (door_w + .4, .12, .22), accent, 0)
    if lod == 0:
        if canopy:
            for side in (-1, 1):
                g.beam((x + side * (door_w/2 + 1.0), y - .05, z0 + height + 1.7),
                       (x + side * (door_w/2 + 1.0), y - 2.4, z0 + height + .76), .06, 'bronze', bevel=0)


def lobby(g, w, d, z0, z1, doors=(0.0,), door_w=4.0, accent='ivory', row=-1, at=(0, 0), pier='graphite',
          canopy=True):
    """Glazed ground floor between piers; `doors` are X offsets of portals on the front (-Y) face."""
    lod = g.lod
    glass_top = z1 - .9
    for index, (p0, u, width) in enumerate(perimeter(w, d, at)):
        cells = max(1, round(width / 3.6))
        g.part = 'lobby'
        pane(g, p0, u, .6, width - .6, z0 + .35, glass_top,
             (index * FACE_OFFSET, row), (index * FACE_OFFSET + cells, row + 1), .45)
        g.part = 'structure'
        face_box(g, p0, u, width/2, -.4, z0 + .175, (width - 1.2, .22, .35), 'stone', .02, 1)
        if lod < 2:
            count = max(2, round(width / 3.6))
            for j in range(1, count):
                s = j * width / count
                if index == 0 and any(abs(s - width/2 - x) < door_w/2 + .5 for x in doors):
                    continue
                face_box(g, p0, u, s, -.2, (z0 + glass_top) / 2, (.55, .8, glass_top - z0), pier, .05, 1)
        x, y = along(p0, u, .55, -.55)
        g.box((x, y, (z0 + z1) / 2), (1.3, 1.3, z1 - z0), pier, .1, 2)
    g.box((at[0], at[1], z1 - .45), (w + .5, d + .5, .9), pier, .06, 1)
    g.box((at[0], at[1], z1 - .93), (w + .54, d + .54, .07), accent, 0)
    if lod < 2:
        g.box((at[0], at[1], z1 - .3), (w + .56, d + .56, .1), 'bronze', 0)
    for x in doors:
        portal(g, at[0] + x, at[1] - d/2, z0, door_w, accent, canopy, min(3.5, glass_top - z0 - .5))


def reactor(g, z0, z1):
    """Open reactor chamber: emitter, glowing core, containment rings, guide rails, collector."""
    lod = g.lod
    g.lathe((0, 0, 0), [(0, z0 + .05), (2.9, z0 + .05), (3.1, z0 + .3), (2.3, z0 + .7), (1.7, z0 + 1.0), (0, z0 + 1.0)], 'graphite')
    g.lathe((0, 0, 0), [(0, z0 + 1.0), (1.5, z0 + 1.05), (1.45, z0 + 1.9), (1.05, (z0 + z1)/2 - .2),
                        (1.0, (z0 + z1)/2 + .5), (1.3, z1 - 1.9), (1.4, z1 - 1.0), (0, z1 - .95)], 'cyan')
    g.lathe((0, 0, 0), [(0, z1 - 1.0), (1.7, z1 - 1.0), (2.3, z1 - .7), (3.1, z1 - .3), (2.9, z1 - .05), (0, z1 - .05)], 'graphite')
    span = z1 - z0 - 3.6
    levels = [z0 + 1.9 + span * i / 4 for i in range(5)] if lod < 2 else [z0 + 1.9 + span * i / 2 for i in range(3)]
    for z in levels:
        g.ring((0, 0, z), 2.25, .2, 'graphite')
        g.ring((0, 0, z + .26), 2.33, .05, 'ivory')
    for i in range(8):
        a = i * pi / 4
        b = a + pi / 8
        g.beam((2.8*cos(a), 2.8*sin(a), z0), (2.1*cos(a + .4), 2.1*sin(a + .4), z1), .18, 'bronze', bevel=0)
        if lod < 2:
            g.pipe((3.0*cos(b), 3.0*sin(b), z0 + .3), (3.0*cos(b), 3.0*sin(b), z1 - .3), .06, 'titanium')
            g.box((2.62*cos(b), 2.62*sin(b), z0 + .52), (.5, .34, .22), 'jade', 0, angle=b)
        if lod == 0:
            for z in levels:
                g.beam((2.42*cos(b), 2.42*sin(b), z), (3.0*cos(b), 3.0*sin(b), z), .07, 'titanium', bevel=0)
    g.ring((0, 0, z1 - .08), 3.4, .05, 'ivory')


def agent_spire(g):
    lod = g.lod
    podium(g, 23, 23, stair_w=7.5)
    lobby(g, 15, 15, PODIUM, 5.9, door_w=4.0, accent='cyan')
    tiers = ((5.9, 12.5, 11.8), (13.0, 29.0, 9.6), (42.5, 61.5, 7.8), (62.0, 72.0, 6.0))
    for k, (z0, z1, w) in enumerate(tiers):
        glass_tier(g, w, w, z0, z1, row0=20 * k, piers='graphite', belt=0, bay=1.9)
        terrace(g, w, w, z1, rail=k < 3)
    if lod < 2:
        roof_unit(g, -4.9, 4.9, 12.82, (1.5, 1.3, .8))
        roof_unit(g, 4.9, 4.9, 12.82, (1.5, 1.3, .8))
    reactor(g, 29.3, 42.5)
    # Corner struts brace the stepped tower; the east side stays clear for the sky lift.
    for side in range(4):
        a = side * pi / 2 + pi / 4
        points = [(r*cos(a), r*sin(a), z) for z, r in ((1, 9), (7, 7.4), (26, 6.2), (49, 5.1), (69, 4.1), (77, 2.4))]
        for p, q in zip(points, points[1:]):
            g.beam(p, q, .5, 'titanium', .75, bevel=.05)
        g.box((points[0][0], points[0][1], 1.12), (1.5, 1.5, .5), 'bronze', .06, 1, angle=a)
        if lod < 2:
            for p in points[1:5]:
                g.box(p, (.95, .95, .46), 'bronze', .05, 1, angle=a)
                g.box((p[0] + .52*cos(a), p[1] + .52*sin(a), p[2]), (.06, .5, .2), 'ivory', 0, angle=a)
        for z in range(10, 67, 9):
            r = 7.4 - (z - 7) * .055
            g.beam((r*cos(a), r*sin(a), z), ((r - 1.3)*cos(a), (r - 1.3)*sin(a), z - 2), .24, 'bronze', bevel=0)
    for z, radius in ((74, 3.5), (79, 3.1), (82, 2.1)):
        g.ring((0, 0, z), radius, .13, 'bronze')
    g.lathe((0, 0, 0), [(0, 72.3), (1.15, 72.3), (1.1, 73.2), (.78, 80.4), (.6, 82.6), (0, 82.8)], 'cyan')
    g.cyl((0, 0, 84.4), .17, 5, 'titanium', top=.02, n=(12, 8, 6)[lod])
    for i in range(4):
        a = i * pi / 2
        g.beam((3*cos(a), 3*sin(a), 72.2), (0, 0, 84), .15, 'titanium', bevel=0)
    if lod == 0:
        for a in (pi / 4, 5 * pi / 4):
            g.lathe((2.7*cos(a), 2.7*sin(a), 79.5), [(0, 0), (.3, .06), (.48, .2), (.5, .26), (.0, .1)], 'titanium')
            g.pipe((2.7*cos(a), 2.7*sin(a), 79.1), (2.7*cos(a), 2.7*sin(a), 79.5), .04, 'graphite')
    return {'occlusion': {'ground': 0.0, 'distance': 2.6}}


def memory_archive(g):
    """Terraced ceramic archive with planted setbacks and the hologram drum on the roof."""
    lod = g.lod
    at = (0, 1)
    podium(g, 26, 22, steps=False)
    if lod < 2:
        for x in (-5, 5):
            stairs(g, x, -11, 4.2)
    lobby(g, 22, 18, PODIUM, 7.0, doors=(-5, 5), door_w=2.8, accent='jade', at=at, pier='ceramic')
    for k, (w, d, z0) in enumerate(((18, 14, 7.0), (14, 10, 13.0))):
        glass_tier(g, w, d, z0, z0 + 6, row0=20 * k, floors=2, piers='ceramic', slab='ceramic', belt=0, at=at, bay=2.0)
    for (w, d, z) in ((22, 18, 7.0), (18, 14, 13.0), (14, 10, 19.0)):
        terrace(g, w, d, z, at=at, cap='graphite', light='jade')
        g.box((at[0], at[1] - d/2 - .37, z + .3), (w + .5, .06, .05), 'bronze', 0)
    if lod < 2:
        for x in (-7.2, 0, 7.2):
            planter(g, x, at[1] - 8.05, 7.32)
            planter(g, x, at[1] + 8.05, 7.32)
        for y in (-3.2, 3.2):
            planter(g, -10.05, at[1] + y, 7.32, angle=pi/2)
            planter(g, 10.05, at[1] + y, 7.32, angle=pi/2)
        for x in (-5.2, 5.2):
            planter(g, x, at[1] - 6.05, 13.32, (2.0, .6, .45))
    # Sign ladder between the doors.
    for z in (3, 4, 5, 6):
        g.box((0, at[1] - 9.3, z), (3, .12, .12), 'ivory', 0)
    # Roof: hologram drum and plant.
    g.lathe((at[0], at[1], 0), [(0, 19.34), (3.9, 19.34), (3.9, 19.6), (3.4, 19.72), (3.4, 20.5), (3.1, 20.62), (0, 20.62)], 'graphite')
    for r in (3.2, 3.7):
        g.ring((at[0], at[1], 20.7), r, .09, 'jade')
    if lod < 2:
        for i in range(12):
            a = i * pi / 6
            g.box((at[0] + 3.42*cos(a), at[1] + 3.42*sin(a), 20.1), (.06, .5, .6), 'glass', 0, angle=a)
    for x in (-5, 5):
        roof_unit(g, x, at[1] + 3, 19.32, (2.0, 1.5, .9))
    return {'occlusion': {'ground': 0.0, 'distance': 2.4}}


def integration_gate(g):
    """Two exchange towers joined by a glazed bridge, framing the ring gate."""
    lod = g.lod
    podium(g, 26, 16, steps=False)
    for k, (x, h) in enumerate(((-8, 28), (8, 22))):
        at, top = (x, 0), 1 + h
        if lod < 2:
            stairs(g, x, -8, 3.4)
        lobby(g, 6, 9, PODIUM, 5.2, doors=(0,), door_w=2.0, accent='cyan', at=at, canopy=False)
        glass_tier(g, 6, 9, 5.2, top, row0=20 * k, piers='graphite', belt=4, at=at, bay=1.5)
        terrace(g, 6, 9, top, at=at, rail=False)
        roof_unit(g, x, 1, top + .32, (2.6, 1.5, .9))
        for y in (-4.65, 4.65):
            g.box((x, y * 1.04, (5.2 + top) / 2), (.24, .34, top - 5.2), 'bronze', .03, 1)
            if lod < 2:
                g.box((x, y * 1.08, (5.2 + top) / 2), (.06, .05, top - 6), 'ivory', 0)
    # Bridge.
    for z in (16.7, 19.3):
        g.box((0, 0, z), (12, 5, .3), 'ceramic', .04, 1)
    for index, (y, u) in enumerate(((-2.25, (1, 0)), (2.25, (-1, 0)))):
        p0 = (-5 * u[0], y)
        pane(g, p0, u, 0, 10, 16.85, 19.15, (index * FACE_OFFSET, 60), (index * FACE_OFFSET + 5, 61))
        if lod < 2:
            for j in range(1, 5):
                face_box(g, p0, u, j * 2, .08, 18, (.07, .16, 2.3), 'titanium')
    g.box((0, 0, 16.5), (9, 3.4, .06), 'ivory', 0)
    # Ring gate in the XZ plane.
    rotation = Matrix.Rotation(pi/2, 3, 'X')
    for r, mat, tube in ((5.2, 'graphite', .5), (4.85, 'cyan', .1), (5.7, 'bronze', .12)):
        g.ring((0, -.2, 10), r, tube, mat, rotation)
    if lod < 2:
        for i in range(12):
            a = i * pi / 6 + pi / 12
            g.box((5.2*cos(a), -.2, 10 + 5.2*sin(a)), (.5, 1.24, 1.3), 'bronze', .05, 1,
                  rot=Matrix.Rotation(-a, 3, 'Y'))
    for side in (-1, 1):
        g.beam((side*4, 0, 1), (side*5.1, 0, 7), .65, 'titanium', bevel=.06)
        g.box((side*4, 0, 1.1), (1.4, 1.4, .45), 'bronze', .06, 1)
    return {'occlusion': {'ground': 0.0, 'distance': 2.4}}


def mission_terminal(g):
    """Logistics hall with four dock portals, roof trusses, a control tower and a landing pad."""
    lod = g.lod
    at = (0, 1)
    doors = (-9, -3, 3, 9)
    podium(g, 30, 21, steps=False)
    if lod < 2:
        stairs(g, 0, -10.5, 23)
    lobby(g, 26, 15, PODIUM, 7.0, doors=doors, door_w=4.0, accent='warm', at=at, canopy=False)
    terrace(g, 26, 15, 7.0, at=at, rail=False, cap='graphite', light='warm')
    for x in doors:
        g.beam((x, -7.8, 6.6), (x, 1, 10), .38, 'ceramic', bevel=.04)
        g.beam((x, 1, 10), (x, 8, 7.4), .38, 'ceramic', bevel=.04)
        if lod < 2:
            for z in (2.2, 3.1, 4.0):
                g.box((x, at[1] - 7.32, z), (3.8, .06, .1), 'titanium', 0)
    g.beam((-10, 1, 10), (10, 1, 10), .3, 'ceramic', bevel=.03)
    # Control tower.
    tower = (8, 2)
    glass_tier(g, 5, 6, 7.3, 15.45, row0=20, floors=3, piers='ceramic', belt=0, at=tower, bay=1.7)
    terrace(g, 5, 6, 15.45, at=tower)
    g.pipe((8, 2, 15.8), (8, 2, 19), .07)
    if lod < 2:
        g.ring((8, 2, 18.2), .5, .03, 'titanium')
    # Landing pad.
    pad = (-7, 1)
    g.lathe((pad[0], pad[1], 0), [(0, 7.32), (4.6, 7.32), (4.6, 7.44), (4.45, 7.5), (0, 7.5)], 'graphite', n=(40, 24, 12)[lod])
    g.ring((pad[0], pad[1], 7.5), 4.1, .07, 'ivory')
    g.box((pad[0], pad[1], 7.52), (3.5, .18, .03), 'warm', 0)
    g.box((pad[0], pad[1], 7.52), (.18, 3.5, .03), 'warm', 0)
    if lod < 2:
        for i in range(8):
            a = i * pi / 4 + pi / 8
            g.box((pad[0] + 4.42*cos(a), pad[1] + 4.42*sin(a), 7.54), (.3, .14, .06), 'jade', 0, angle=a)
        roof_unit(g, 1.5, 6.4, 7.32, (3.0, 1.6, 1.1))
    return {'occlusion': {'ground': 0.0, 'distance': 2.4}}


def cooling_tower(g, x, y):
    lod = g.lod
    n = (28, 16, 10)[lod]
    g.lathe((x, y, 0), [(2.7, .88), (2.7, 1.2), (2.52, 1.32)], 'stone', n=n)
    g.lathe((x, y, 0), [(2.5, 1.3), (2.22, 3.4), (1.98, 6.0), (1.9, 7.4), (2.05, 8.8)], 'titanium', n=n)
    g.lathe((x, y, 0), [(2.05, 8.6), (2.8, 8.7), (2.8, 9.3), (2.5, 9.6), (2.1, 9.6), (2.0, 9.2)], 'graphite', n=n)
    g.ring((x, y, 9.65), 2.35, .09, 'cyan')
    for z in ((2, 3.4, 4.8, 6.2, 7.6) if lod < 2 else (3, 7)):
        g.ring((x, y, z), 2.6 - (z - 1) * .083, .08, 'bronze')
    g.pipe((x, y, 2), (x, y - 3.7, 2), .36, 'bronze')
    g.pipe((x, y - 3.7, 2), (x, y - 3.7, 1), .36, 'bronze')
    if lod < 2:
        g.lathe((x, y, 0), [(0, 9.15), (1.95, 9.15)], 'iron', n=n, tint=.5, up=True)
        for py in (y - 2.6, y - 3.2):
            g.ring((x, py, 2), .44, .05, 'titanium', Matrix.Rotation(pi/2, 3, 'X'))
    if lod == 0:
        for i in range(3):
            g.box((x, y, 9.2), (3.8, .12, .07), 'graphite', 0, angle=i * pi / 3)
        for z in [1.5 + .32 * i for i in range(22)]:
            radius = 2.55 - (z - 1.3) * .078
            g.box((x + radius + .1, y, z), (.05, .42, .03), 'titanium', 0)
        for side in (-.21, .21):
            g.beam((x + 2.62, y + side, 1.3), (x + 2.08, y + side, 8.6), .04, 'titanium', bevel=0)


def compute_foundry(g):
    """Server hall behind three cooling towers."""
    lod = g.lod
    at = (0, 3)
    podium(g, 25, 23, steps=False)
    if lod < 2:
        for x in (-3.75, 3.75):
            stairs(g, x, -11.5, 3.0)
    lobby(g, 20, 12, PODIUM, 5.4, doors=(-3.75, 3.75), door_w=2.4, accent='cyan', at=at, canopy=False)
    glass_tier(g, 20, 12, 5.4, 13.0, row0=0, floors=3, piers='graphite', belt=0, at=at, bay=2.2)
    terrace(g, 20, 12, 13.0, at=at, cap='graphite', light='cyan')
    for x in (-7.5, 0, 7.5):
        cooling_tower(g, x, -5)
        roof_unit(g, x, at[1] + 1.5, 13.32, (3.5, 1.5, .9))
        if lod < 2:
            g.pipe((x, at[1] + .4, 13.6), (x, -3.2, 13.6), .14, 'titanium')
            g.pipe((x, -3.2, 13.6), (x, -3.2, 9.9), .14, 'titanium')
    if lod < 2:
        for x in (-10.25, 10.25):
            for y in range(-2, 9, 2):
                g.box((x * 1.012, y, 8.8), (.38, .2, 8.2), 'ceramic', .03, 1)
        g.box((0, at[1] + 4.4, 13.75), (17, 1.1, .8), 'graphite', .05, 1)
    return {'occlusion': {'ground': 0.0, 'distance': 2.4}}


def knowledge_atrium(g):
    """Glazed rotunda under an open ribbed dome that shelters the knowledge tree."""
    lod = g.lod
    podium(g, 24, 24, stair_w=4.4)
    sides = 12
    wall = 4.7        # top of the glazed rotunda wall
    # Face 0 is centred on the front (-Y).
    angles = [-pi / 2 - pi / sides + i * 2 * pi / sides for i in range(sides + 1)]
    radius = 9.45
    for i in range(sides):
        a0, a1 = angles[i], angles[i + 1]
        p0 = (radius * cos(a0), radius * sin(a0))
        p1 = (radius * cos(a1), radius * sin(a1))
        width = hypot(p1[0] - p0[0], p1[1] - p0[1])
        u = ((p1[0] - p0[0]) / width, (p1[1] - p0[1]) / width)
        front = i == 0
        if not front:
            g.part = 'lobby'
            pane(g, p0, u, .35, width - .35, PODIUM + .35, wall, (i * FACE_OFFSET, -1), (i * FACE_OFFSET + 1, 0), .12)
            g.part = 'structure'
            face_box(g, p0, u, width/2, -.1, PODIUM + .175, (width - .5, .2, .35), 'stone', .02, 1)
        g.box((p0[0] * 1.012, p0[1] * 1.012, (PODIUM + wall) / 2 + .05), (.7, .7, wall - PODIUM + .1), 'ceramic', .06, 2, angle=a0)
    n = (48, 24, 12)[lod]
    g.lathe((0, 0, 0), [(9.75, wall), (9.95, wall + .06), (9.95, wall + .28), (9.7, wall + .36), (8.4, wall + .4)], 'graphite', n=n)
    g.ring((0, 0, wall + .36), 9.4, .08, 'jade')
    g.lathe((0, 0, 0), [(0, PODIUM + .02), (9.3, PODIUM + .02)], 'stone', n=n, tint=.85, up=True)
    portal(g, 0, -radius * cos(pi / sides) - .1, PODIUM, 3.2, 'jade', canopy=True, height=3.1)
    # Dome ribs and hoops.
    for i in range(sides):
        a = angles[i]
        g.beam((8.6*cos(a), 8.6*sin(a), wall + .3), (7.8*cos(a), 7.8*sin(a), 13), .22, 'ceramic', bevel=.03)
        g.beam((7.8*cos(a), 7.8*sin(a), 13), (0, 0, 20), .22, 'ceramic', bevel=.03)
    for z, r, tube in ((wall + .4, 8.6, .17), (13, 7.8, .17), (17, 3.35, .17)):
        g.ring((0, 0, z), r, tube, 'bronze')
    if lod < 2:
        g.ring((0, 0, 8.25), 8.2, .08, 'bronze')
        g.ring((0, 0, 15), 5.57, .08, 'bronze')
    g.lathe((0, 0, 0), [(0, 19.7), (.5, 19.8), (.6, 20.2), (.25, 20.7), (.12, 21.6), (0, 21.7)], 'bronze', n=(12, 8, 6)[lod])
    # Knowledge tree.
    g.lathe((0, 0, 0), [(1.5, PODIUM + .02), (1.5, 1.35), (1.3, 1.45), (.82, 2.4), (.62, 6), (.4, 10), (.25, 12.6), (0, 12.7)],
            'bronze', n=(16, 10, 6)[lod])
    if lod < 2:
        g.ring((0, 0, 1.5), 2.6, .16, 'bronze')
        g.ring((0, 0, 1.36), 2.6, .05, 'jade')
    for i in range(7):
        a = i * 2 * pi / 7
        p, q = (0, 0, 6 + i * .65), (4.2*cos(a), 4.2*sin(a), 10.5 + i * .45)
        g.pipe(p, q, .19, 'bronze')
        g.ring(q, .78, .1, 'jade')
        sphere(g, (q[0], q[1], q[2] + .5), 1.0, 'leaf', tint=.9, squash=.7)
        if lod < 2:
            for sign in (-1, 1):
                r = (5.8*cos(a + sign * .24), 5.8*sin(a + sign * .24), q[2] + 1.8)
                g.pipe(q, r, .085, 'titanium')
                sphere(g, (r[0], r[1], r[2] + .2), .55, 'leaf', tint=.95, squash=.75)
                g.cyl((r[0], r[1], r[2] - .25), .1, .22, 'jade', top=.04, n=6)
    return {'occlusion': {'ground': 0.0, 'distance': 2.4}}


def operations_beacon(g):
    """Small control tower with a glazed cab; the `signal` part pulses at run time."""
    lod = g.lod
    podium(g, 9, 9, stair_w=2.2)
    n = (16, 12, 8)[lod]
    g.lathe((0, 0, 0), [(2.5, PODIUM), (2.5, 1.3), (2.36, 1.45), (1.72, 8.6), (1.6, 9.05)], 'graphite', n=n)
    for z in (3.2, 5.6, 8.0) if lod < 2 else ():
        g.ring((0, 0, z), 2.42 - (z - 1.45) * .0895 + .02, .06, 'titanium')
    for i in range(4):
        a = i * pi / 2 + pi / 4
        g.beam((2.4*cos(a), 2.4*sin(a), 1), (1.5*cos(a), 1.5*sin(a), 9), .22, 'bronze', bevel=.03)
    sides = 12
    for i in range(sides):
        a0, a1 = i * 2 * pi / sides, (i + 1) * 2 * pi / sides
        p0, p1 = (2.7*cos(a0), 2.7*sin(a0)), (2.7*cos(a1), 2.7*sin(a1))
        width = hypot(p1[0] - p0[0], p1[1] - p0[1])
        u = ((p1[0] - p0[0]) / width, (p1[1] - p0[1]) / width)
        pane(g, p0, u, 0, width, 9.05, 10.95, (i, 0), (i + 1, 1))
        if lod < 2:
            g.box((p0[0], p0[1], 10), (.09, .09, 1.9), 'titanium', 0, angle=a0)
    for z in (9, 11):
        g.ring((0, 0, z), 2.85, .17, 'titanium')
    g.lathe((0, 0, 0), [(2.95, 11.1), (2.6, 11.4), (.5, 11.58), (0, 11.58)], 'graphite', n=n)
    g.lathe((0, 0, 0), [(0, 9.0), (2.7, 9.0)], 'graphite', n=n, up=True)
    g.box((0, -2.42, 1.95), (1.3, .16, 2.1), 'glass', 0)
    for side in (-1, 1):
        g.box((side * .75, -2.46, 2.0), (.14, .2, 2.3), 'bronze', .02, 1)
    g.box((0, -2.46, 3.1), (1.64, .2, .14), 'bronze', .02, 1)
    if lod == 0:
        for z in [1.6 + .3 * i for i in range(24)]:
            radius = 2.36 - (z - 1.45) * .0895
            g.box((radius + .08, 0, z), (.04, .4, .03), 'titanium', 0)
    g.part = 'signal'
    g.ring((0, 0, 11.5), 2.25, .12, 'warm')
    g.cyl((0, 0, 12.3), .28, 1.2, 'warm', top=.06, n=12)
    g.part = 'structure'
    return {'occlusion': {'ground': 0.0, 'distance': 2.0}}


def skybridge(g):
    """Covered glazed bridge, 16 m long."""
    lod = g.lod
    g.box((0, 0, .2), (16, 4, .4), 'graphite', .06, 1)
    g.box((0, 0, 4.7), (16, 4.4, .18), 'graphite', .05, 1)
    g.box((0, 0, -.02), (15, 2.6, .04), 'ivory', 0)
    for index, (y, u) in enumerate(((-2, (1, 0)), (2, (-1, 0)))):
        p0 = (-8 * u[0], y)
        pane(g, p0, u, 0, 16, .6, 4.0, (index * FACE_OFFSET, 0), (index * FACE_OFFSET + 4, 1))
        for z in (.9, 2.1):
            g.box((0, y * 1.03, z), (16, .1, .1), 'bronze', 0)
        g.box((0, y * 1.02, .47), (16, .05, .03), 'ivory', 0)
        for x in range(-8, 9, 4):
            g.beam((x, y * 1.03, .4), (x, y * 1.03, 4), .16, 'titanium', bevel=0)
            g.beam((x, y * 1.03, 4), (x, 0, 4.6), .16, 'titanium', bevel=0)
    return {'occlusion': {'distance': 1.6}}


ASSETS = [
    ('agent-spire', 'Agent reactor spire', 'agent', agent_spire),
    ('memory-archive', 'Terraced memory archive', 'memory', memory_archive),
    ('integration-gate', 'Integration gateway', 'integration', integration_gate),
    ('mission-terminal', 'Mission logistics terminal', 'missions', mission_terminal),
    ('compute-foundry', 'Compute and cooling foundry', 'infrastructure', compute_foundry),
    ('knowledge-atrium', 'Knowledge tree atrium', 'knowledge', knowledge_atrium),
    ('operations-beacon', 'Operations beacon', 'operations', operations_beacon),
    ('skybridge', 'Modular covered skybridge', 'connector', skybridge),
]
