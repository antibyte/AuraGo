"""System World kit v3: facade toolkit and the skyline data towers (Blender 5.2 LTS).

Buildings are slab-and-glass: the glass core carries `city.window` panes whose UV0 holds
room coordinates (u: one unit per bay along the facade, left to right seen from outside,
offset by 16 per face; v: one unit per floor, 0 at the first upper floor, the ground floor
is row -1). The renderer turns those panes into rooms with depth. Floor slabs, fins, piers
and roof plant are real geometry in front of the glass and bake their own occlusion.
Footprints, podium height and roof heights follow the v1 kit so placements stay valid.
"""
from math import atan2, pi

PODIUM = .88          # walkable podium top (v1)
GROUND = 5.0          # ground floor height above the podium
FACE_OFFSET = 16      # UV u offset between the faces of one building


def perimeter(w, d, at=(0, 0)):
    """Faces of a box footprint centred on `at`, counter-clockwise: (corner, direction, width)."""
    x, y = at
    return [((x - w/2, y - d/2), (1, 0), w), ((x + w/2, y - d/2), (0, 1), d),
            ((x + w/2, y + d/2), (-1, 0), w), ((x - w/2, y + d/2), (0, -1), d)]


def along(p0, u, s, out=0.0):
    """Point at distance s along a face and `out` metres outside it."""
    return (p0[0] + u[0]*s + u[1]*out, p0[1] + u[1]*s - u[0]*out)


def pane(g, p0, u, s0, s1, z0, z1, uv0, uv1, recess=0.0):
    """Window quad facing outward with room UVs from (uv0) bottom-left to (uv1) top-right."""
    a, b = along(p0, u, s0, -recess), along(p0, u, s1, -recess)
    g.quad((a[0], a[1], z0), (b[0], b[1], z0), (b[0], b[1], z1), (a[0], a[1], z1), 'window',
           uvs=[(uv0[0], uv0[1]), (uv1[0], uv0[1]), (uv1[0], uv1[1]), (uv0[0], uv1[1])])


def face_box(g, p0, u, s, out, z, size, mat, bevel=0, segments=1, tint=1.0):
    """Box centred at distance s along a face; size = (along, outward depth, height)."""
    x, y = along(p0, u, s, out)
    g.box((x, y, z), size, mat, bevel, segments, angle=atan2(u[1], u[0]), tint=tint)


def podium(g, w, d, steps=True, stair_x=0.0, stair_w=4.4):
    g.box((0, 0, .35), (w, d, .7), 'stone', .1, 2)
    g.box((0, 0, .79), (w - .5, d - .5, .18), 'bronze', .03, 1)
    if steps and g.lod < 2:
        stairs(g, stair_x, -d/2, stair_w)


def stairs(g, x, y, width, count=3):
    """Steps from the pavement up to the podium on a -Y edge at (x, y)."""
    rise = .88 / (count + 1)
    for i in range(count):
        g.box((x, y - .17 - i*.34, .88 - rise*(i + 1) - rise/2 + .0), (width, .34, rise), 'stone', .02, 1, tint=.95)


def planter(g, x, y, z, size=(2.4, .7, .5), angle=0):
    """Ceramic trough with a clipped hedge."""
    g.box((x, y, z + size[2]/2), size, 'ceramic', .04, 1, angle=angle)
    g.box((x, y, z + size[2] + .2), (size[0] - .16, size[1] - .16, .44), 'leaf', .1, 2, angle=angle, tint=.9)


def roof_unit(g, x, y, z, size=(2.4, 1.6, 1.1), angle=0):
    """Air handling unit with fan cowls and a louvred front."""
    g.box((x, y, z + size[2]/2), size, 'titanium', .06, 2, angle=angle)
    if g.lod == 2:
        return
    fans = 2 if size[0] > 2 else 1
    for i in range(fans):
        offset = (i - (fans - 1)/2) * size[0] * .46
        fx, fy = (x + offset, y) if not angle else (x, y + offset)
        g.cyl((fx, fy, z + size[2] + .06), .42, .12, 'graphite', n=(16, 10)[g.lod])
        if g.lod == 0:
            g.cyl((fx, fy, z + size[2] + .13), .3, .03, 'iron', n=12, tint=.6)
    if g.lod == 0:
        for i in range(5):
            if angle:
                g.box((x - size[1]/2 - .012, y, z + .22 + i*.16), (.03, size[0] - .3, .05), 'graphite', 0)
            else:
                g.box((x, y - size[1]/2 - .012, z + .22 + i*.16), (size[0] - .3, .03, .05), 'graphite', 0)


def glass_tier(g, w, d, z0, z1, row0=0, floors=None, bay=1.85, piers='ceramic', belt=5, slab='graphite',
               at=(0, 0), fin='titanium'):
    """Slab-and-glass storeys between z0 and z1: panes with room UVs, slabs, fins, corner piers."""
    lod = g.lod
    floors = floors or max(1, round((z1 - z0) / 3.05))
    floor_h = (z1 - z0) / floors
    faces = perimeter(w, d, at)
    for index, (p0, u, width) in enumerate(faces):
        bays = max(2, round(width / bay))
        offset = index * FACE_OFFSET
        pane(g, p0, u, 0, width, z0, z1, (offset, row0), (offset + bays, row0 + floors))
        if lod < 2 and fin:
            for j in range(1, bays):
                face_box(g, p0, u, j * width / bays, .13, (z0 + z1) / 2, (.07, .26, z1 - z0), fin)
    step = (1, 1, 3)[lod]
    for k in range(0, floors + 1, step):
        z = z0 + k * floor_h
        if belt and k and k % belt == 0 and k < floors and lod < 2:
            g.box((at[0], at[1], z), (w + .7, d + .7, .95), 'titanium', .04, 1)
            if lod == 0:
                for (p0, u, width) in faces:
                    for i in range(4):
                        face_box(g, p0, u, width/2, .37, z - .3 + i*.2, (width - .8, .04, .07), 'graphite', 0)
        else:
            g.box((at[0], at[1], z), (w + .56, d + .56, .5), slab, .035, 1)
            if lod == 0:
                g.box((at[0], at[1], z + .27), (w + .6, d + .6, .035), 'titanium', 0)
    if piers:
        for (p0, u, width) in faces:
            x, y = along(p0, u, 0, .12)
            g.box((x - u[0]*.12, y - u[1]*.12, (z0 + z1) / 2), (.7, .7, z1 - z0), piers, .06, 2)
    return floors


def terrace(g, w, d, z, rail=True, at=(0, 0), cap='bronze', light='ivory'):
    """Cap with a light line and a slim guard rail around a setback roof."""
    g.box((at[0], at[1], z + .16), (w + .7, d + .7, .32), cap, .05, 1)
    g.box((at[0], at[1], z - .04), (w + .76, d + .76, .06), light, 0)
    if rail and g.lod < 2:
        for (p0, u, width) in perimeter(w + .5, d + .5, at):
            face_box(g, p0, u, width/2, -.03, z + 1.28, (width, .05, .05), 'titanium', 0)
            if g.lod == 0:
                face_box(g, p0, u, width/2, -.03, z + .82, (width, .03, .03), 'titanium', 0)
                posts = max(2, round(width / 1.6))
                for j in range(posts + 1):
                    face_box(g, p0, u, j * width / posts, -.03, z + .8, (.05, .05, .96), 'titanium', 0)


def crown(g, w, d, top, variant):
    """Bronze cap, parapet, gravel roof deck and roof plant."""
    g.box((0, 0, top + .15), (w + 1.2, d + 1.2, .3), 'bronze', .05, 1)
    g.box((0, 0, top - .06), (w + 1.26, d + 1.26, .07), 'ivory', 0)
    for (p0, u, width) in perimeter(w + .9, d + .9):
        face_box(g, p0, u, width/2, -.11, top + .62, (width, .22, .64), 'titanium', .03, 1)
    g.box((0, 0, top + .34), (w + .46, d + .46, .1), 'graphite', 0, tint=.9)
    deck = top + .39
    if variant == 0:
        roof_unit(g, -w*.2, d*.16, deck)
        roof_unit(g, w*.24, -d*.2, deck, (1.5, 1.5, .9))
        if g.lod < 2:
            g.box((w*.25, d*.24, deck + 1.1), (1.9, 1.9, 2.2), 'graphite', .06, 1)
            g.box((w*.25, d*.24 - .96, deck + 1.0), (.9, .05, 1.9), 'bronze', 0)
            g.pipe((-w*.2 + 1.3, d*.16, deck + .25), (w*.25 - 1, d*.16, deck + .25), .07)
            g.pipe((w*.25 - 1, d*.16, deck + .25), (w*.25 - 1, d*.24 - .4, deck + .25), .07)
        g.cyl((-w*.36, -d*.3, deck + 2.2), .05, 4.4, 'bronze', top=.02, n=(8, 6, 4)[g.lod])
        if g.lod == 0:
            for z in (2.9, 3.6):
                g.box((-w*.36, -d*.3, deck + z), (1.1, .03, .03), 'titanium', 0)
    else:
        g.box((0, 0, deck + 1.5), (w - 2, d - 2, 3), 'graphite', .12, 2)
        if g.lod < 2:
            for (p0, u, width) in perimeter(w - 2, d - 2):
                pane(g, p0, u, .5, width - .5, deck + .9, deck + 2.5, (0, 40), (max(1, round(width/2.4)), 41), -.02)
            g.box((0, 0, deck + 3.06), (w - 1.6, d - 1.6, .12), 'bronze', .03, 1)
            roof_unit(g, 0, d*.12, deck + 3.1, (2.2, 1.4, .9))
        g.cyl((0, -d*.2, deck + 5.3), .12, 4.4, 'bronze', top=.03, n=(8, 6, 4)[g.lod])
        if g.lod == 0:
            g.ring((0, -d*.2, deck + 6.2), .5, .025, 'titanium')


def tower(g, w, d, h, variant):
    """Slab-and-glass tower on a podium; the roof stays at v1's height (h + 1)."""
    lod = g.lod
    podium(g, w + 4, d + 4)
    base, top = PODIUM, h + 1.0
    shaft = base + GROUND
    floors = max(4, round((top - shaft) / 3.05))
    floor_h = (top - shaft) / floors
    faces = perimeter(w, d)
    # Glass core with room coordinates.
    for index, (p0, u, width) in enumerate(faces):
        bays = max(2, round(width / 1.85))
        offset = index * FACE_OFFSET
        pane(g, p0, u, 0, width, shaft, top, (offset, 0), (offset + bays, floors))
        lobby = max(1, round(width / 3.7))
        pane(g, p0, u, .3, width - .3, base + .3, shaft - .7, (offset, -1), (offset + lobby, 0), .35)
        # Fins on the bay lines.
        if lod < 2:
            for j in range(1, bays):
                face_box(g, p0, u, j * width / bays, .13, (shaft + top) / 2, (.07, .26, top - shaft), 'titanium')
        # Lobby columns on every second bay line and at the corners.
        if lod < 2:
            marks = [j * width / bays for j in range(0, bays + 1, 2)]
            if marks[-1] != width:
                marks.append(width)
            for s in marks:
                face_box(g, p0, u, s, -.12, base + GROUND/2 - .35, (.46, .46, GROUND - .7), 'graphite', .04, 1)
        # Plinth course under the lobby glazing.
        face_box(g, p0, u, width/2, -.3, base + .15, (width - .2, .2, .3), 'stone', .02, 1)
    g.box((0, 0, base + .02), (w - .5, d - .5, .04), 'stone', 0, tint=.8)
    # Floor slabs; every fifth is a deeper service belt.
    step = (1, 1, 3)[lod]
    for k in range(0, floors + 1, step):
        z = shaft + k * floor_h
        belt = k and k % 5 == 0 and k < floors
        if belt and lod < 2:
            g.box((0, 0, z), (w + .7, d + .7, .95), 'titanium', .04, 1)
            if lod == 0:
                for (p0, u, width) in faces:
                    for i in range(4):
                        face_box(g, p0, u, width/2, .37, z - .3 + i*.2, (width - .8, .04, .07), 'graphite', 0)
        else:
            g.box((0, 0, z), (w + .56, d + .56, .5), 'graphite', .035, 1)
            if lod == 0:
                g.box((0, 0, z + .27), (w + .6, d + .6, .035), 'titanium', 0)
    # Corner piers.
    for (p0, u, width) in faces:
        x, y = along(p0, u, 0, .12)
        x, y = x - u[0]*.12, y - u[1]*.12
        g.box((x, y, (base + top) / 2), (.7, .7, top - base), 'ceramic', .06, 2)
    # Fascia with a light line above the lobby.
    g.box((0, 0, shaft - .35), (w + .9, d + .9, .7), 'graphite', .05, 1)
    g.box((0, 0, shaft - .73), (w + .94, d + .94, .07), 'ivory', 0)
    # Entrance on the front (-Y): portal, glazed doors, canopy.
    front = -d/2
    g.box((0, front - .05, base + 1.5), (2.7, .2, 3.0), 'glass', 0)
    for side in (-1, 1):
        g.box((side * 1.45, front - .16, base + 1.55), (.16, .34, 3.1), 'bronze', .03, 1)
    g.box((0, front - .16, base + 3.16), (3.06, .34, .16), 'bronze', .03, 1)
    if lod < 2:
        g.box((0, front - .17, base + 1.5), (.05, .06, 3.0), 'titanium', 0)
        g.box((0, front - 1.1, base + 3.55), (4.4, 2.2, .16), 'graphite', .05, 1)
        g.box((0, front - 2.16, base + 3.5), (4.4, .06, .3), 'bronze', 0)
        g.box((0, front - 1.1, base + 3.45), (3.9, 1.7, .03), 'ivory', 0)
    if lod == 0:
        for side in (-1, 1):
            g.beam((side * 1.9, front - .1, base + 4.6), (side * 1.9, front - 2.0, base + 3.65), .05, 'bronze', bevel=0)
            g.box((side * .68, front - .2, base + 1.3), (.05, .05, .9), 'titanium', 0)
    # Identity blade on the front right corner.
    if lod < 2:
        g.box((w/2 + .52, -d/2 + 1.2, shaft + (top - shaft) * .46), (.16, .5, (top - shaft) * .78), 'bronze', .03, 1)
        g.box((w/2 + .62, -d/2 + 1.2, shaft + (top - shaft) * .46), (.04, .2, (top - shaft) * .74), 'ivory', 0)
    crown(g, w, d, top, variant)
    return {'occlusion': {'ground': 0.0, 'distance': 2.4}}


def data_tower_a(g):
    return tower(g, 11, 9, 33, 0)


def data_tower_b(g):
    return tower(g, 9, 12, 44, 1)


ASSETS = [
    ('data-tower-a', 'Data tower A', 'building', data_tower_a),
    ('data-tower-b', 'Data tower B', 'building', data_tower_b),
]
