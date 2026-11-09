"""System World kit v3: the district buildings (Blender 5.2 LTS).

Each building keeps the footprint, podium, tier heights and landmark parts of its v1
design, so navigation footprints, the sky deck, the lift shaft and labels stay valid.
What changes is everything one sees from the street: glazed lobbies between piers,
slab-and-glass storeys with room coordinates, terraces, roof plant and real entrances.
"""
from math import sin, cos, pi

from facade_assets3 import (PODIUM, FACE_OFFSET, perimeter, along, pane, face_box, podium,
                            glass_tier, terrace, roof_unit)


def lobby(g, w, d, z0, z1, door_w=4.0, accent='ivory', row=-1):
    """Glazed ground floor between graphite piers with a portal and canopy on the front (-Y)."""
    lod = g.lod
    glass_top = z1 - .9
    for index, (p0, u, width) in enumerate(perimeter(w, d)):
        cells = max(1, round(width / 3.6))
        pane(g, p0, u, .6, width - .6, z0 + .35, glass_top,
             (index * FACE_OFFSET, row), (index * FACE_OFFSET + cells, row + 1), .45)
        face_box(g, p0, u, width/2, -.4, z0 + .175, (width - 1.2, .22, .35), 'stone', .02, 1)
        if lod < 2:
            count = max(2, round(width / 3.6))
            for j in range(1, count):
                s = j * width / count
                if index == 0 and abs(s - width/2) < door_w/2 + .5:
                    continue
                face_box(g, p0, u, s, -.2, (z0 + glass_top) / 2, (.55, .8, glass_top - z0), 'graphite', .05, 1)
        x, y = along(p0, u, .55, -.55)
        g.box((x, y, (z0 + z1) / 2), (1.3, 1.3, z1 - z0), 'graphite', .1, 2)
    g.box((0, 0, z1 - .45), (w + .5, d + .5, .9), 'graphite', .06, 1)
    g.box((0, 0, z1 - .93), (w + .54, d + .54, .07), accent, 0)
    if lod < 2:
        g.box((0, 0, z1 - .3), (w + .56, d + .56, .1), 'bronze', 0)
    # Portal.
    front = -d/2
    g.box((0, front + .25, z0 + 1.75), (door_w, .1, 3.5), 'glass', 0)
    for side in (-1, 1):
        g.box((side * (door_w/2 + .14), front + .12, z0 + 1.85), (.28, .7, 3.7), 'bronze', .04, 1)
    g.box((0, front + .12, z0 + 3.78), (door_w + .56, .7, .28), 'bronze', .04, 1)
    if lod < 2:
        leaves = max(2, round(door_w / 1.1))
        for j in range(1, leaves):
            g.box((-door_w/2 + j * door_w / leaves, front + .2, z0 + 1.75), (.05, .08, 3.5), 'titanium', 0)
        g.box((0, front - 1.25, z0 + 4.15), (door_w + 2.6, 2.9, .18), 'graphite', .06, 1)
        g.box((0, front - 2.68, z0 + 4.1), (door_w + 2.6, .07, .34), 'bronze', 0)
        g.box((0, front - 1.25, z0 + 4.04), (door_w + 2.0, 2.3, .03), 'ivory', 0)
        for side in (-1, 1):
            g.box((side * (door_w/2 + .62), front - .08, z0 + 1.9), (.1, .16, 3.3), accent, 0)
    if lod == 0:
        for side in (-1, 1):
            g.beam((side * (door_w/2 + 1.0), front - .05, z0 + 5.2), (side * (door_w/2 + 1.0), front - 2.4, z0 + 4.26),
                   .06, 'bronze', bevel=0)
            for j in range(leaves):
                x = -door_w/2 + (j + .5) * door_w / leaves + side * .32
                g.box((x, front + .14, z0 + 1.3), (.04, .05, .8), 'titanium', 0)


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
        g.beam((2.8*cos(a), 2.8*sin(a), z0), (2.1*cos(a + .4), 2.1*sin(a + .4), z1), .18, 'bronze', bevel=0)
        if lod < 2:
            b = a + pi / 8
            g.pipe((3.0*cos(b), 3.0*sin(b), z0 + .3), (3.0*cos(b), 3.0*sin(b), z1 - .3), .06, 'titanium')
            g.box((2.62*cos(b), 2.62*sin(b), z0 + .52), (.5, .34, .22), 'jade', 0, angle=b)
        if lod == 0:
            for z in levels:
                g.beam((2.42*cos(b), 2.42*sin(b), z), (3.0*cos(b), 3.0*sin(b), z), .07, 'titanium', bevel=0)
    g.ring((0, 0, z1 - .08), 3.4, .05, 'ivory')


def agent_spire(g):
    lod = g.lod
    podium(g, 23, 23, steps=False)
    if lod < 2:
        for i in range(4):
            g.box((0, -11.5 - .19 - i * .38, .77 - .22 * i), (7.5, .38, .22), 'stone', .02, 1, tint=.95)
    lobby(g, 15, 15, PODIUM, 5.9, door_w=4.0, accent='cyan')
    tiers = ((5.9, 12.5, 11.8), (13.0, 29.0, 9.6), (42.5, 61.5, 7.8), (62.0, 72.0, 6.0))
    for k, (z0, z1, w) in enumerate(tiers):
        glass_tier(g, w, w, z0, z1, row0=20 * k, piers='graphite', belt=0, bay=1.9)
        terrace(g, w, w, z1, rail=k < 3)
    # Setback roof between the first two tiers carries plant behind the rail.
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
    # Crown.
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


ASSETS = [
    ('agent-spire', 'Agent reactor spire', 'agent', agent_spire),
]
