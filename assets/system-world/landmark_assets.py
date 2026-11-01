"""Original civic landmarks for System World 2.

Four authored landmarks that give the data metropolis a silhouette and a night
identity: an offshore tidal turbine, a kinetic plaza monument, a street gateway
and an elevated observation pod. Each one owns a named animated part, so the
city's motion comes from the exported clips instead of runtime improvisation.

Coordinates in this authoring file are Blender Z-up; exports use metres, Y-up,
+Z forward.
"""
from math import sin, cos, pi
from mathutils import Matrix, Vector

ASSETS = ('tidal-turbine', 'data-monument', 'neon-gate', 'sky-pod')

# Pivots of the articulated parts, mirrored by the animator in build_expansion.py.
PIVOTS = {
    'turbine_rotor': (0, 0, 18.6),
    'ring_outer': (0, 0, 4.2),
    'ring_mid': (0, 0, 5.8),
    'ring_inner': (0, 0, 7.2),
    'monument_core': (0, 0, 8.9),
    'light_band': (0, -.33, 7.0),
    'scanner': (0, 0, 11.9),
}


def wing(g, root, tip, root_chord, tip_chord, mat, thickness=.13, twist=0.0):
    """A tapered, twisted blade between two points.

    Local X runs along the span, Y along the chord and Z through the thickness.
    The blade is built around its own root, so the animator can spin it around
    its own pivot without disturbing the rest of the assembly.
    """
    root, tip = Vector(root), Vector(tip)
    span = tip - root
    x = span.normalized()
    y = Vector((0, 0, 1)).cross(x)
    if y.length < 1e-6:
        y = Vector((0, 1, 0))
    y.normalize()
    z = x.cross(y)
    rotation = Matrix(((x.x, y.x, z.x), (x.y, y.y, z.y), (x.z, y.z, z.z)))
    if twist:
        rotation = rotation @ Matrix.Rotation(twist, 3, 'X')
    vertices = []
    for length, chord in ((0.0, root_chord), (span.length, tip_chord)):
        for side in (-chord / 2, chord / 2):
            for face in (0.0, thickness):
                vertices.append((length, side, face))
    faces = [(0, 1, 3, 2), (4, 6, 7, 5), (0, 2, 6, 4), (1, 5, 7, 3), (0, 4, 5, 1), (2, 3, 7, 6)]
    g.add(vertices, faces, mat, tuple(root), rotation)


def tidal_turbine(g):
    """Offshore vertical-axis tidal turbine.

    The rotor turns about the vertical axis in the nacelle, so the whole
    machine reads as one moving silhouette from the quay and the overview.
    """
    # Octagonal pier with a cast collar; the machine is anchored in the harbour.
    g.cyl((0, 0, .55), 3.4, 1.1, 'stone', top=3.15, n=24)
    g.cyl((0, 0, 1.22), 2.5, .28, 'graphite', n=24)
    g.ring((0, 0, 1.08), 3.32, .13, 'bronze')
    for i in range(6):
        a = i * pi / 3
        g.cyl((2.6 * cos(a), 2.6 * sin(a), 1.5), .16, .5, 'titanium', n=8)
    # Three bronze legs taper from the pier to the nacelle.
    for i in range(3):
        a = i * 2 * pi / 3 + .4
        g.beam((2.5 * cos(a), 2.5 * sin(a), 1.4), (1.05 * cos(a), 1.05 * sin(a), 16.4), .25, 'bronze', .3)
    # Horizontal bracing rings and alternating cross ties.
    rings = ((4.0, 2.14), (7.4, 1.82), (10.8, 1.5), (14.0, 1.2))
    for z, r in rings:
        for i in range(3):
            a = i * 2 * pi / 3 + .4
            b = (i + 1) * 2 * pi / 3 + .4
            g.beam((r * cos(a), r * sin(a), z), (r * cos(b), r * sin(b), z), .12, 'titanium')
    for i in range(3):
        a = i * 2 * pi / 3 + .4
        b = (i + 1) * 2 * pi / 3 + .4
        g.beam((2.14 * cos(a), 2.14 * sin(a), 4.0), (1.82 * cos(b), 1.82 * sin(b), 7.4), .08, 'titanium')
        g.beam((1.82 * cos(a), 1.82 * sin(a), 7.4), (1.5 * cos(b), 1.5 * sin(b), 10.8), .08, 'titanium')
    # Nacelle with a service balcony on the seaward side.
    g.cyl((0, 0, 16.9), 1.55, 1.5, 'ceramic', n=16)
    g.cyl((0, 0, 17.72), 1.62, .18, 'bronze', n=16)
    g.box((0, -1.15, 16.5), (2.2, 1.1, .16), 'titanium', .04)
    for x in (-1.0, 1.0):
        g.pipe((x, -1.6, 16.5), (x, -.7, 16.5), .06, 'graphite')
    g.ring((0, 0, 17.9), 1.2, .07, 'cyan')
    # Rotor: hub and five cambered blades, all in one named part.
    g.part = 'turbine_rotor'
    g.cyl((0, 0, 18.6), .78, 1.2, 'titanium', n=16)
    g.cyl((0, 0, 18.6), .34, 1.9, 'bronze', n=12)
    for i in range(5):
        a = i * 2 * pi / 5
        root = (.72 * cos(a), .72 * sin(a), 18.6)
        tip = (6.1 * cos(a), 6.1 * sin(a), 17.75)
        wing(g, root, tip, 1.5, .72, 'ceramic', .13, .3)
        wing(g, (.7 * cos(a + .06), .7 * sin(a + .06), 18.6),
             (2.2 * cos(a + .04), 2.2 * sin(a + .04), 18.4), .42, .3, 'bronze', .1, .1)
    g.part = 'structure'
    # Mast with an aviation light above the swept disc.
    g.cyl((0, 0, 20.3), .16, 3.2, 'titanium', top=.06, n=10)
    g.cyl((0, 0, 22.0), .26, .34, 'warm', n=10)
    g.ring((0, 0, 22.3), .3, .05, 'graphite')


def data_monument(g):
    """Kinetic plaza monument: a levitating core inside three counter-turning rings."""
    g.cyl((0, 0, .22), 3.3, .44, 'stone', n=24)
    g.cyl((0, 0, .66), 2.75, .44, 'stone', top=2.6, n=20)
    g.cyl((0, 0, 1.1), 2.15, .44, 'graphite', n=20)
    g.ring((0, 0, .44), 3.22, .11, 'bronze')
    g.ring((0, 0, .88), 2.68, .08, 'bronze')
    for i in range(8):
        a = i * pi / 4
        g.box((1.4 * cos(a), 1.4 * sin(a), 1.33), (.16, .16, .05), 'cyan', 0, angle=a)
    # The mast carries the core; the rings turn around it.
    g.cyl((0, 0, 6.0), .3, 9.4, 'titanium', top=.22, n=12)
    for z in (2.2, 4.4, 6.6, 9.0):
        g.ring((0, 0, z), .42, .06, 'bronze')
    # Three gyroscope rings, each tilted so the assembly precesses.
    g.part = 'ring_outer'
    g.ring((0, 0, 4.2), 2.9, .085, 'bronze', rotation=Matrix.Rotation(.34, 3, 'X'))
    g.ring((0, 0, 4.2), 2.86, .022, 'cyan', rotation=Matrix.Rotation(.34, 3, 'X'))
    g.part = 'ring_mid'
    g.ring((0, 0, 5.8), 2.25, .075, 'ceramic', rotation=Matrix.Rotation(-.5, 3, 'Y'))
    g.part = 'ring_inner'
    g.ring((0, 0, 7.2), 1.62, .065, 'bronze', rotation=Matrix.Rotation(.78, 3, 'X'))
    g.part = 'monument_core'
    # A bipyramid core: two cones meet at the widest point, which is the pivot.
    # Jade keeps the crystal readable; the hot cyan studs carry the accent.
    g.cyl((0, 0, 9.4), .82, 1.0, 'jade', top=.02, n=8)
    g.cyl((0, 0, 8.4), .82, 1.0, 'jade', top=.02, n=8, rotation=Matrix.Rotation(pi, 3, 'X'))
    g.ring((0, 0, 8.9), .9, .05, 'ivory')
    for i in range(6):
        a = i * pi / 3
        g.box((.86 * cos(a), .86 * sin(a), 8.9), (.12, .12, .12), 'cyan', .02, angle=a)
    g.part = 'structure'
    # A thin light shaft leaves the core and fades into the haze.
    g.cyl((0, 0, 11.2), .1, 3.2, 'glass', n=8)
    g.cyl((0, 0, 10.3), .26, .3, 'ivory', n=8)


def neon_gate(g):
    """Street gateway that spans a road and carries a travelling light band."""
    for side in (-1, 1):
        x = side * 4.4
        g.cyl((x, 0, 3.5), .46, 7.0, 'graphite', top=.3, n=10)
        g.box((x, 0, .18), (1.5, 1.5, .36), 'stone', .06)
        g.box((x, 0, 7.16), (1.15, 1.15, .32), 'bronze', .05)
        g.cyl((x, 0, 7.5), .2, .36, 'ivory', n=8)
        # Vertical light strips on the inner face.
        for offset in (-.16, .16):
            g.box((x - side * .34, offset, 3.5), (.06, .1, 5.4), 'cyan', 0)
            g.box((x - side * .34, offset, 3.5), (.08, .13, .16), 'ivory', 0)
    # Lintel, lower tie and diagonal braces.
    g.beam((-4.4, 0, 7.0), (4.4, 0, 7.0), .34, 'bronze', .5)
    g.beam((-4.4, 0, 6.2), (4.4, 0, 6.2), .16, 'titanium')
    for side in (-1, 1):
        g.beam((side * 4.4, 0, 6.2), (side * 3.1, 0, 7.0), .11, 'titanium')
    # A warm lit channel runs along the lintel; the hot band travels over it.
    g.box((0, -.27, 6.98), (8.2, .08, .06), 'ivory', 0)
    # Hanging sign plates.
    for side in (-1, 1):
        g.box((side * 2.3, -.28, 5.7), (1.5, .08, .9), 'ceramic', .04)
        g.box((side * 2.3, -.34, 5.7), (1.2, .03, .1), 'cyan', 0)
        for x in (side * 2.3 - .6, side * 2.3 + .6):
            g.pipe((x, -.2, 6.2), (x, -.2, 6.15), .035, 'bronze')
    # The travelling band rides the lit face of the lintel.
    g.part = 'light_band'
    g.box((0, -.33, 7.0), (.92, .08, .1), 'cyan', .02)
    g.box((0, -.38, 7.0), (.74, .03, .13), 'jade', 0)
    g.part = 'structure'


def sky_pod(g):
    """Elevated observation pod with an external stair and a sweeping scanner."""
    g.cyl((0, 0, .25), 1.95, .5, 'stone', n=20)
    g.ring((0, 0, .5), 1.9, .1, 'bronze')
    g.cyl((0, 0, 5.6), 1.0, 10.2, 'ceramic', top=.74, n=16)
    for i in range(8):
        a = i * pi / 4
        g.box((.93 * cos(a), .93 * sin(a), 5.4), (.14, .14, 9.4), 'bronze', .02, angle=a)
    # Wrapping stair: cheap treads on a bronze helix.
    steps = (18, 12, 8)[g.lod]
    for i in range(steps):
        a = i * 1.35 * pi / steps
        z = 1.1 + 8.6 * i / steps
        g.box((1.62 * cos(a), 1.62 * sin(a), z), (1.18, .44, .1), 'bronze', .02, angle=a)
        if i % 3 == 0:
            g.pipe((1.95 * cos(a), 1.95 * sin(a), z + .5), (1.95 * cos(a), 1.95 * sin(a), z + 1.05), .045, 'graphite')
    for i in range(steps):
        a = i * 1.35 * pi / steps
        z = 1.1 + 8.6 * i / steps
        g.box((1.95 * cos(a), 1.95 * sin(a), z + 1.05), (.07, .07, .07), 'ivory', 0)
    # Pod: a bronze floor, a glazed drum, a ribbed frame and a conical roof.
    g.cyl((0, 0, 10.6), 2.32, .2, 'titanium', n=20)
    g.ring((0, 0, 10.72), 2.32, .08, 'bronze')
    g.cyl((0, 0, 11.9), 2.26, 2.3, 'garden-glass', n=20)
    for i in range(6):
        a = i * pi / 3
        g.box((2.28 * cos(a), 2.28 * sin(a), 11.9), (.12, .12, 2.3), 'bronze', .02, angle=a)
    g.cyl((0, 0, 13.2), 2.5, .5, 'graphite', top=.42, n=20)
    g.ring((0, 0, 13.42), 2.46, .06, 'bronze')
    g.cyl((0, 0, 13.9), .12, .8, 'titanium', top=.04, n=8)
    g.cyl((0, 0, 14.4), .16, .2, 'warm', n=8)
    # Warm interior light and a bench face the skyline.
    g.cyl((0, 0, 10.78), 1.86, .05, 'warm', n=16)
    g.box((0, 1.3, 10.98), (2.4, .5, .18), 'bronze', .04)
    # The scanner sweeps the city around the pod at eye level.
    g.part = 'scanner'
    g.ring((0, 0, 11.9), 2.44, .055, 'bronze')
    g.box((2.44, 0, 11.9), (.34, .5, .34), 'cyan', .04)
    g.cyl((2.44, 0, 11.9), .2, .26, 'ivory', n=8, rotation=Matrix.Rotation(pi / 2, 3, 'X'))
    g.part = 'structure'


BUILDERS = {
    'tidal-turbine': tidal_turbine,
    'data-monument': data_monument,
    'neon-gate': neon_gate,
    'sky-pod': sky_pod,
}


def build_landmark(g, kind):
    BUILDERS[kind](g)
    g.part = 'structure'


def landmark_navigation(kind):
    """Colliders only: the landmarks are scenery, never a walkable floor."""
    boxes = {
        'tidal-turbine': [],
        'data-monument': [[-3.15, 0, -3.15, 3.15, 1.35, 3.15]],
        'neon-gate': [[-4.95, 0, -.62, -3.85, 7.3, .62], [3.85, 0, -.62, 4.95, 7.3, .62]],
        'sky-pod': [[-1.95, 0, -1.95, 1.95, 1.0, 1.95]],
    }
    return {'colliders': boxes[kind], 'surfaces': [], 'portals': []}
