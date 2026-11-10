"""System World kit v3: street furniture and planting (Blender 5.2 LTS).

Small things that make a street: lamps, trees in grated pits, planters, benches, bollards,
litter bins, crossing signs and a kiosk. Origins sit on the pavement surface (z = 0).
`street-lamp` and `planter` keep their v1 ids, footprint and the lamp head position
(1.55 m along +X at 6.5 m), so existing placements and the light cones still fit.
Colliders are [min x, min y, min z, max x, max y, max z] in glTF axes (Y up).
"""
from math import sin, cos, pi

from mathutils import Matrix


def blob(g, center, radius, mat='leaf', squash=.8, tint=1.0, n=None, steps=None, lumpy=.0):
    """Squashed sphere used for foliage and shrubs; `lumpy` pushes vertices in and out."""
    steps = steps or (7, 5, 3)[g.lod]
    n = n or (12, 8, 6)[g.lod]
    vertices, rings = [], []
    for i in range(steps + 1):
        ring_radius, z = radius * sin(pi * i / steps), -radius * squash * cos(pi * i / steps)
        if i in (0, steps):
            rings.append([len(vertices)])
            vertices.append((0, 0, z))
            continue
        rings.append(list(range(len(vertices), len(vertices) + n)))
        for j in range(n):
            # Deterministic bump per vertex, independent of the level of detail's vertex order.
            a = 2 * pi * j / n + (i % 2) * pi / n
            bump = 1 + lumpy * sin(a * 3 + i * 1.7 + center[0] * 2.1 + center[1] * 1.3) * cos(a * 2 - i * .9 + center[2])
            vertices.append((ring_radius * bump * cos(a), ring_radius * bump * sin(a), z * (1 + (bump - 1) * .5)))
    faces = []
    for lower, upper in zip(rings, rings[1:]):
        for j in range(n):
            k = (j + 1) % n
            if len(lower) == 1:
                faces.append((lower[0], upper[k], upper[j]))
            elif len(upper) == 1:
                faces.append((lower[j], lower[k], upper[0]))
            else:
                faces.append((lower[j], lower[k], upper[k], upper[j]))
    g.add(vertices, faces, mat, center, tint=tint)


def collider(x0, z0, x1, z1, height, base=0.0):
    """Box collider from a Blender XY footprint (glTF z = -Blender y)."""
    return [round(x0, 3), round(base, 3), round(-z1, 3), round(x1, 3), round(height, 3), round(-z0, 3)]


def street_lamp(g):
    lod = g.lod
    g.lathe((0, 0, 0), [(.3, 0), (.3, .1), (.2, .16), (.13, .5), (.1, .62)], 'graphite', n=(14, 10, 6)[lod])
    g.cyl((0, 0, 3.4), .1, 5.6, 'graphite', top=.07, n=(12, 8, 6)[lod], caps=(False, False))
    g.box((0, 0, 1.2), (.26, .2, .5), 'titanium', .03, 1)
    # Curved arm: three straight pieces.
    arm = [(0, 0, 6.15), (.35, 0, 6.5), (.9, 0, 6.66), (1.6, 0, 6.66)]
    for a, b in zip(arm, arm[1:]):
        g.pipe(a, b, .055, 'bronze')
    g.cyl((0, 0, 6.25), .09, .22, 'bronze', n=(10, 8, 6)[lod])
    # Luminaire with a glowing diffuser underneath.
    g.box((1.55, 0, 6.6), (1.4, .42, .12), 'graphite', .04, 2)
    g.box((1.55, 0, 6.525), (1.22, .3, .03), 'ivory', 0)
    if lod < 2:
        g.box((1.55, 0, 6.68), (1.1, .2, .05), 'titanium', .02, 1)
        g.ring((0, 0, .64), .11, .025, 'bronze')
    if lod == 0:
        g.box((.14, 0, 1.2), (.02, .12, .34), 'jade', 0)
        g.box((0, 0, 4.2), (.5, .03, .7), 'cyan', 0)
        g.box((0, 0, 4.2), (.54, .05, .74), 'graphite', 0, tint=.8)
    return {'navigation': {'colliders': [collider(-.3, -.3, .3, .3, 6.7)]}, 'occlusion': {'ground': 0.0}}


def planter(g):
    """Ceramic trough with shrubs and a flowering edge; same footprint as v1 (3.5 x 1.6 m)."""
    lod = g.lod
    g.box((0, 0, .45), (3.5, 1.6, .9), 'ceramic', .08, 2)
    g.box((0, 0, .9), (3.24, 1.34, .06), 'road', 0, tint=.7)
    if lod < 2:
        g.box((0, 0, .07), (3.56, 1.66, .14), 'stone', .03, 1)
    # Shrubs and blossoms form the soft part `foliage` (smooth normals): each shrub is a mound with two
    # smaller puffs leaning on it.
    g.part = 'foliage'
    spots = [(-1.15, .1, .6), (-.35, -.15, .74), (.45, .12, .68), (1.2, -.05, .58)]
    for index, (x, y, r) in enumerate(spots):
        blob(g, (x, y, .9 + r * .6), r, tint=.8 + .07 * (index % 3), squash=.85, n=(10, 8, 6)[lod], steps=(5, 4, 3)[lod],
             lumpy=.07)
        if lod == 2:
            continue
        for k in range((2, 1)[lod]):
            a = 1.1 + index * 1.9 + k * 2.6
            blob(g, (x + r * .72 * cos(a), y + r * .42 * sin(a), .9 + r * (.78 - .22 * k)), r * (.6 - .08 * k),
                 tint=.94 + .06 * k, squash=.85, n=(8, 6)[lod], steps=(4, 3)[lod], lumpy=.07)
    if lod < 2:
        for i in range(9):
            x = -1.5 + i * .375
            for y in (-.56, .56):
                blob(g, (x, y, .98), .13, 'warm' if i % 3 == 0 else 'jade', squash=.7, n=6, steps=3)
    g.part = 'structure'
    return {'navigation': {'colliders': [collider(-1.75, -.8, 1.75, .8, 1.6)]}, 'soft': ['foliage'],
            'occlusion': {'ground': 0.0}}


def tree_pit(g, size=1.7):
    """Cast iron grate flush with the pavement around the trunk."""
    lod = g.lod
    half = size / 2
    g.grid([-half, -half + .08, half - .08, half], [-half, -half + .08, half - .08, half], .012,
           lambda i, j: None if i == 1 and j == 1 else 'kerb', tint=.6)
    if lod == 2:
        g.grid([-half + .08, half - .08], [-half + .08, half - .08], .01, 'iron', tint=.7)
        return
    bars = (9, 5)[lod]
    xs = [-half + .08]
    pitch = (size - .16) / bars
    for i in range(bars):
        xs += [-half + .08 + pitch * i + pitch * .55, -half + .08 + pitch * (i + 1)]
    g.grid(xs, [-half + .08, half - .08], .01, 'iron', tint=lambda i, j: (.85, .28)[i % 2])


def tree(g, height, spread, columnar=False):
    """Street tree: grate, tapering trunk, limbs and a canopy of soft puffs.

    `spread` is the canopy's width. The canopy is its own part (`crown`), listed under
    `soft`, so it keeps smooth normals at every level of detail and reads as foliage. The
    canopy starts above head height: a street stays visible below it.
    """
    lod = g.lod
    # Every random value is drawn up front, so all three LODs describe the same tree.
    draws = [g.rng.random() for _ in range(56)]
    tree_pit(g)
    half = spread / 2
    trunk_h = height * (.45 if not columnar else .29)
    n = (10, 7, 5)[lod]
    g.lathe((0, 0, 0), [(.27, .0), (.2, .22), (.15, trunk_h * .5), (.115, trunk_h), (.06, height * .74)], 'bronze',
            n=n, tint=.4)
    limbs = 4 if not columnar else 3
    tips = []
    for i in range(limbs):
        r = draws[i * 4:i * 4 + 4]
        a = i * 2 * pi / limbs - .3 + .6 * r[0]
        reach = half * (.5 if not columnar else .34) * (.88 + .2 * r[1])
        tip = (reach * cos(a), reach * sin(a), trunk_h + height * (.13 + .07 * r[2]))
        tips.append(tip)
        if lod < 2:
            g.pipe((0, 0, trunk_h * (.8 + .16 * r[3])), tip, .055, 'bronze', n=(6, 5)[lod], tint=.42)
    # Canopy puffs as (centre, radius, squash, tint): darker below, lighter on top. The first `core` puffs
    # carry the silhouette and are all the lowest level of detail keeps.
    puffs = []
    if columnar:
        core = 3
        puffs.append(((0, 0, height * .46), half * .86, 1.25, .78))
        puffs.append(((0, 0, height * .62), half * .95, 1.3, .86))
        puffs.append(((0, 0, height * .8), half * .74, 1.5, .97))
        for i, tip in enumerate(tips):
            puffs.append(((tip[0], tip[1], tip[2] + .5 + .9 * i), half * (.56 + .08 * draws[16 + i]), 1.2,
                          .74 + .14 * draws[20 + i]))
    else:
        core = 1 + limbs
        crown_z = height * .74
        puffs.append(((0, 0, crown_z), half * .74, .86, .86))
        for i, tip in enumerate(tips):
            puffs.append(((tip[0], tip[1], tip[2] + half * .2), half * (.5 + .07 * draws[16 + i]), .84,
                          .72 + .12 * draws[20 + i]))
        for i in range(3):
            a = 2 * pi * (i / 3 + .25 * draws[24 + i])
            puffs.append(((half * .33 * cos(a), half * .33 * sin(a), crown_z + half * .34),
                          half * (.44 + .05 * draws[28 + i]), .8, .95 + .05 * draws[32 + i]))
    g.part = 'crown'
    rings, steps = ((12, 6), (8, 4), (6, 3))[lod]
    for center, radius, squash, tint in (puffs if lod < 2 else puffs[:core]):
        blob(g, center, radius, tint=tint, squash=squash, n=rings, steps=steps, lumpy=.07)
    if lod == 0:
        # Small tufts on the outside break the outline.
        count = 6 if not columnar else 4
        for i in range(count):
            a = 2 * pi * (i / count + .1 * draws[36 + i])
            if columnar:
                reach, z, radius = half * .82, height * (.42 + .42 * i / count + .05 * draws[42 + i]), half * (.3 + .08 * draws[48 + i])
            else:
                reach, z, radius = half * (.86 + .12 * draws[42 + i]), height * (.64 + .22 * draws[48 + i]), half * (.19 + .05 * draws[42 + i])
            blob(g, (reach * cos(a), reach * sin(a), z), radius, tint=.92 + .12 * draws[36 + i], squash=.85, n=7, steps=4,
                 lumpy=.07)
    g.part = 'structure'
    return {'navigation': {'colliders': [collider(-.3, -.3, .3, .3, trunk_h)]},
            'soft': ['crown'], 'occlusion': {'ground': 0.0, 'distance': 2.2}}


def tree_round(g):
    return tree(g, 6.4, 4.6)


def tree_column(g):
    return tree(g, 7.6, 2.4, columnar=True)


def street_bench(g):
    """Slatted bench on cast legs with a back rest; faces -Y."""
    lod = g.lod
    for x in (-.85, .85):
        g.box((x, 0, .22), (.08, .5, .44), 'graphite', .02, 1)
        g.box((x, .2, .62), (.07, .07, .5), 'graphite', 0, rot=Matrix.Rotation(-.18, 3, 'X'))
        if lod < 2:
            g.box((x, 0, .02), (.14, .58, .04), 'iron', 0)
    slats = (5, 3, 1)[lod]
    for i in range(slats):
        y = -.2 + i * (.4 / max(1, slats - 1)) if slats > 1 else 0
        g.box((0, y, .47), (2.0, (.07 if slats > 1 else .46), .04), 'bronze', .012, 1, tint=.75 + .08 * (i % 2))
    for i in range((3, 2, 1)[lod]):
        g.box((0, .25 + i * .012, .64 + i * .13), (2.0, .035, (.1 if lod < 2 else .34)), 'bronze', .01, 1,
              rot=Matrix.Rotation(-.18, 3, 'X'), tint=.78)
    return {'navigation': {'colliders': [collider(-1.0, -.3, 1.0, .32, .9)]}, 'occlusion': {'ground': 0.0}}


def bollard(g):
    lod = g.lod
    g.lathe((0, 0, 0), [(.11, 0), (.11, .06), (.085, .1), (.08, .78), (.095, .82), (.095, .9), (.05, .96), (0, .97)],
            'graphite', n=(12, 8, 6)[lod])
    g.ring((0, 0, .7), .083, .012, 'ivory') if lod < 2 else None
    return {'navigation': {'colliders': [collider(-.12, -.12, .12, .12, .97)]}, 'occlusion': {'ground': 0.0}}


def litter_bin(g):
    lod = g.lod
    n = (14, 10, 6)[lod]
    g.lathe((0, 0, 0), [(.25, .06), (.27, .1), (.27, .82), (.25, .86)], 'titanium', n=n)
    g.lathe((0, 0, 0), [(0, .06), (.25, .06)], 'graphite', n=n, up=False)
    g.lathe((0, 0, 0), [(.3, .9), (.3, .98), (.12, 1.06), (0, 1.07)], 'graphite', n=n)
    g.lathe((0, 0, 0), [(0, .84), (.24, .84)], 'iron', n=n, tint=.3, up=True)
    for a in (pi / 4, 3 * pi / 4, 5 * pi / 4, 7 * pi / 4):
        g.box((.24 * cos(a), .24 * sin(a), .88), (.03, .03, .1), 'graphite', 0)
    if lod < 2:
        g.ring((0, 0, .45), .272, .012, 'bronze')
        g.box((0, -.272, .62), (.2, .012, .12), 'jade', 0)
    return {'navigation': {'colliders': [collider(-.3, -.3, .3, .3, 1.07)]}, 'occlusion': {'ground': 0.0}}


def zebra_sign(g):
    """Illuminated pedestrian-crossing sign on a slim post; the sign faces -Y and +Y."""
    lod = g.lod
    g.cyl((0, 0, 1.5), .045, 3.0, 'titanium', n=(10, 8, 6)[lod])
    g.cyl((0, 0, .03), .12, .06, 'graphite', n=(10, 8, 6)[lod])
    g.box((0, 0, 2.75), (.7, .08, .7), 'graphite', .03, 1)
    for side in (-1, 1):
        g.box((0, side * .045, 2.75), (.6, .012, .6), 'cyan', 0)
        if lod < 2:
            # Walking figure: head, body, legs as a stencil of dark blocks.
            y = side * .053
            g.box((0, y, 2.93), (.09, .006, .09), 'graphite', 0)
            g.box((0, y, 2.77), (.07, .006, .2), 'graphite', 0)
            g.box((-.06, y, 2.6), (.05, .006, .2), 'graphite', 0, rot=Matrix.Rotation(.35, 3, 'Y'))
            g.box((.06, y, 2.6), (.05, .006, .2), 'graphite', 0, rot=Matrix.Rotation(-.35, 3, 'Y'))
    return {'navigation': {'colliders': [collider(-.1, -.1, .1, .1, 3.1)]}, 'occlusion': {'ground': 0.0}}


def kiosk(g):
    """Small pavement kiosk with a serving hatch, awning, lit sign and a rack of parts; front faces -Y."""
    lod = g.lod
    w, d, h = 3.2, 2.2, 2.7
    g.box((0, 0, .08), (w + .2, d + .2, .16), 'stone', .03, 1)
    g.box((0, 0, h/2 + .08), (w, d, h), 'graphite', .06, 2)
    # Serving hatch: counter, dark opening, shutter slats above.
    g.box((0, -d/2 - .02, 1.75), (2.2, .06, 1.1), 'glass', 0)
    g.box((0, -d/2 - .22, 1.14), (2.5, .5, .08), 'bronze', .02, 1)
    for side in (-1, 1):
        g.box((side * 1.16, -d/2 - .05, 1.75), (.1, .12, 1.2), 'bronze', .02, 1)
    g.box((0, -d/2 - .05, 2.36), (2.42, .12, .1), 'bronze', .02, 1)
    # Awning.
    awning = Matrix.Rotation(.32, 3, 'X')
    g.box((0, -d/2 - .7, 2.52), (2.9, 1.5, .05), 'warm', 0, rot=awning)
    if lod < 2:
        for i in range(6):
            g.box((-1.2 + i * .48, -d/2 - .7, 2.555), (.2, 1.5, .012), 'ceramic', 0, rot=awning)
        for side in (-1, 1):
            g.beam((side * 1.4, -d/2 - .02, 2.2), (side * 1.4, -d/2 - 1.36, 2.3), .03, 'titanium', bevel=0)
    # Roof with sign.
    g.box((0, 0, h + .14), (w + .5, d + .5, .12), 'bronze', .03, 1)
    g.box((0, -d/2 - .2, h + .5), (2.3, .12, .5), 'graphite', .03, 1)
    g.box((0, -d/2 - .27, h + .5), (2.1, .02, .34), 'cyan', 0)
    if lod < 2:
        g.box((0, 0, h + .26), (1.1, .9, .22), 'titanium', .03, 1)
        g.cyl((.5, .3, h + .5), .03, .7, 'titanium', n=6)
        # Side shelf with spare parts.
        for i, mat in enumerate(('jade', 'cyan', 'ivory', 'warm')):
            g.box((w/2 + .07, -.6 + i * .4, 1.5), (.1, .26, .26), mat, .02, 1)
        g.box((w/2 + .08, 0, 1.33), (.16, 1.8, .04), 'titanium', 0)
        g.box((-w/2 - .02, .2, 1.3), (.04, .8, 1.9), 'bronze', 0)
    if lod == 0:
        for i in range(5):
            g.box((0, -d/2 - .04, 2.26 - i * .02), (2.2, .02, .012), 'titanium', 0)
        for x in (-.7, 0, .7):
            g.cyl((x, -d/2 - .22, 1.26), .07, .16, ('jade', 'ivory', 'cyan')[int(x * 1.5 + 1)], n=8)
    return {'navigation': {'colliders': [collider(-w/2, -d/2 - .5, w/2, d/2, h + .3)]},
            'occlusion': {'ground': 0.0, 'distance': 2.0}}


ASSETS = [
    ('street-lamp', 'Street lamp with a curved arm', 'prop', street_lamp),
    ('planter', 'Planted ceramic trough', 'prop', planter),
    ('tree-round', 'Street tree with a round crown', 'nature', tree_round),
    ('tree-column', 'Columnar street tree', 'nature', tree_column),
    ('street-bench', 'Slatted street bench', 'prop', street_bench),
    ('bollard', 'Pavement bollard', 'prop', bollard),
    ('litter-bin', 'Litter bin', 'prop', litter_bin),
    ('zebra-sign', 'Illuminated crossing sign', 'prop', zebra_sign),
    ('kiosk', 'Pavement kiosk', 'building', kiosk),
]
