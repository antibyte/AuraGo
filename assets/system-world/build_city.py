"""Build AuraGo's original, texture-free city kit in Blender 5.2.1 LTS.

Run in a dedicated Blender scene, or through the Blender MCP execute tool:
    exec(compile(Path(...).read_text(), ..., 'exec'), {'__file__': ..., '__name__': '__main__'})
Exports only the kit to ui/3d/system-world/v1; authoring and renders stay outside ui.
"""
from pathlib import Path
from math import sin, cos, pi
from functools import lru_cache
import hashlib
import json
import random
import struct

import bpy
import bmesh
from mathutils import Vector, Matrix

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'ui/3d/system-world/v1'
SOURCE = ROOT / 'assets/system-world/production'
REPORT = ROOT / 'reports/system-world-assets'
COLLECTION = 'AURAGO_CITY_KIT'

# Linear material values; all objects share these named PBR materials.
PALETTE = {
    'graphite': ((.032, .048, .062, 1), .72, .28, 0),
    'titanium': ((.28, .36, .4, 1), .78, .26, 0),
    'ceramic': ((.57, .63, .59, 1), .25, .36, 0),
    'bronze': ((.39, .20, .073, 1), .82, .25, 0),
    'glass': ((.025, .10, .14, 1), .65, .15, 0),
    'warm': ((.65, .27, .06, 1), .1, .32, 1.3),
    'ivory': ((.8, .62, .32, 1), .1, .3, .8),
    'cyan': ((.09, .63, .8, 1), .15, .32, 2.1),
    'jade': ((.12, .48, .29, 1), .22, .38, 1.3),
    'leaf': ((.052, .135, .086, 1), .05, .66, 0),
    'road': ((.035, .045, .053, 1), .12, .55, 0),
    'stone': ((.16, .20, .21, 1), .12, .67, 0),
}


def materials():
    result = {}
    for key, (color, metal, rough, emission) in PALETTE.items():
        name = 'city.' + key
        mat = bpy.data.materials.get(name) or bpy.data.materials.new(name)
        mat.use_nodes = True
        mat.diffuse_color = color
        bsdf = mat.node_tree.nodes.get('Principled BSDF')
        bsdf.inputs['Base Color'].default_value = color
        bsdf.inputs['Metallic'].default_value = metal
        bsdf.inputs['Roughness'].default_value = rough
        bsdf.inputs['Emission Color'].default_value = color
        bsdf.inputs['Emission Strength'].default_value = emission
        result[key] = mat
    return result


@lru_cache(maxsize=512)
def box_template(size, bevel):
    bm = bmesh.new()
    bmesh.ops.create_cube(bm, size=1)
    for v in bm.verts:
        v.co.x *= size[0]
        v.co.y *= size[1]
        v.co.z *= size[2]
    if bevel:
        bmesh.ops.bevel(bm, geom=list(bm.edges), offset=bevel, segments=1,
                        affect='EDGES', clamp_overlap=True)
    bm.verts.ensure_lookup_table()
    bm.verts.index_update()
    result = ([tuple(v.co) for v in bm.verts],
              [tuple(v.index for v in f.verts) for f in bm.faces])
    bm.free()
    return result


class Geometry:
    def __init__(self, lod):
        self.lod = lod
        self.parts = {}
        self.part = 'structure'
        self.rng = random.Random(43021)

    def add(self, vertices, faces, material, center=(0, 0, 0), rotation=None, smooth=False):
        part = self.parts.setdefault(self.part, [[], [], [], []])
        start = len(part[0])
        offset = Vector(center)
        part[0].extend(tuple((rotation @ Vector(v) if rotation else Vector(v)) + offset)
                       for v in vertices)
        part[1].extend(tuple(start + i for i in f) for f in faces)
        part[2].extend([material] * len(faces))
        part[3].extend([smooth] * len(faces))

    def box(self, center, size, mat='graphite', bevel=.06, angle=0):
        bevel = min(bevel, min(size) * .24) if self.lod == 0 else 0
        vertices, faces = box_template(tuple(size), bevel)
        rotation = Matrix.Rotation(angle, 3, 'Z') if angle else None
        self.add(vertices, faces, mat, center, rotation)

    def cyl(self, center, radius, height, mat='graphite', top=None, n=None, rotation=None):
        n = n or (32, 20, 12)[self.lod]
        top = radius if top is None else top
        vertices = [(r * cos(2*pi*i/n), r * sin(2*pi*i/n), z)
                    for z, r in ((-height/2, radius), (height/2, top)) for i in range(n)]
        self.add(vertices, [(i, (i+1) % n, (i+1) % n+n, i+n) for i in range(n)],
                 mat, center, rotation, True)
        self.add(vertices, [tuple(reversed(range(n))), tuple(range(n, 2*n))],
                 mat, center, rotation)

    def beam(self, a, b, width, mat='titanium', depth=None):
        a, b = Vector(a), Vector(b)
        vector = b-a
        verts, faces = box_template((width, depth or width, vector.length),
                                    min(width * .14, .08) if self.lod == 0 else 0)
        self.add(verts, faces, mat, (a+b)/2, vector.to_track_quat('Z', 'Y').to_matrix())

    def pipe(self, a, b, radius=.16, mat='titanium'):
        a, b = Vector(a), Vector(b)
        self.cyl((a+b)/2, radius, (b-a).length, mat,
                 n=(12, 8, 6)[self.lod], rotation=(b-a).to_track_quat('Z', 'Y').to_matrix())

    def ring(self, center, radius, tube=.1, mat='bronze', rotation=None, arc=2*pi):
        n, sides = (48, 28, 16)[self.lod], (6, 5, 4)[self.lod]
        vertices = [((radius+tube*cos(j*2*pi/sides))*cos(i*arc/n),
                     (radius+tube*cos(j*2*pi/sides))*sin(i*arc/n), tube*sin(j*2*pi/sides))
                    for i in range(n+1) for j in range(sides)]
        faces = [(i*sides+j, i*sides+(j+1) % sides,
                  (i+1)*sides+(j+1) % sides, (i+1)*sides+j)
                 for i in range(n) for j in range(sides)]
        self.add(vertices, faces, mat, center, rotation, True)

    def windows(self, x, y, z, w, d, floors, cols=5):
        # Fine windows are actual flat geometry, never individual materials or textures.
        stride = (1, 1, 2)[self.lod]
        if self.lod < 2:
            # Raised vertical mullions and horizontal transoms provide real facade depth.
            height = floors*2.6
            for c in range(cols+1):
                px, py = x-w/2+.4+c*(w-.8)/cols, y-d/2+.4+c*(d-.8)/cols
                for sign in (-1, 1):
                    self.box((px, y+sign*(d/2+.065), z+height/2),
                             (.07, .11, height), 'titanium', 0)
                    self.box((x+sign*(w/2+.065), py, z+height/2),
                             (.11, .07, height), 'titanium', 0)
            for row in range(floors+1):
                for sign in (-1, 1):
                    self.box((x, y+sign*(d/2+.065), z+row*2.6+.2),
                             (w-.6, .14, .075), 'titanium', 0)
                    self.box((x+sign*(w/2+.065), y, z+row*2.6+.2),
                             (.14, d-.6, .075), 'titanium', 0)
        for row in range(0, floors, stride):
            for col in range(0, cols, stride):
                material = self.rng.choices(['glass', 'warm', 'ivory'], [6, 2, 1])[0]
                cw = (w-.8)/cols
                lo, hi = x-w/2+.4+col*cw+.13, x-w/2+.4+(col+1)*cw-.13
                bottom, upper = z+row*2.6+.35, z+row*2.6+2.02
                for sign in (-1, 1):
                    vertices = [(lo, y+sign*(d/2+.015), bottom),
                                (hi, y+sign*(d/2+.015), bottom),
                                (hi, y+sign*(d/2+.015), upper),
                                (lo, y+sign*(d/2+.015), upper)]
                    self.add(vertices, [(0, 1, 2, 3) if sign < 0 else (3, 2, 1, 0)], material)
                # Side faces have their own warm/dark room rhythm.
                sy = y-d/2+.4+col*(d-.8)/cols
                for sign in (-1, 1):
                    vertices = [(x+sign*(w/2+.015), sy+.13, bottom),
                                (x+sign*(w/2+.015), sy+(d-.8)/cols-.13, bottom),
                                (x+sign*(w/2+.015), sy+(d-.8)/cols-.13, upper),
                                (x+sign*(w/2+.015), sy+.13, upper)]
                    self.add(vertices, [(0, 1, 2, 3) if sign > 0 else (3, 2, 1, 0)], material)

    def podium(self, w, d):
        self.box((0, 0, .35), (w, d, .7), 'stone', .22)
        self.box((0, 0, .79), (w-.5, d-.5, .18), 'bronze')
        for side in (-1, 1):
            self.box((side*(w/2-.35), 0, .91), (.08, d-1, .07), 'ivory', 0)

    def door(self, x, y, z=1, width=2):
        self.box((x, y, z+1.4), (width, .16, 2.8), 'glass')
        for side in (-1, 1):
            self.box((x+side*(width/2+.1), y-.12, z+1.5), (.12, .13, 3), 'bronze')
        self.box((x, y-.12, z+3), (width+.3, .13, .1), 'ivory', 0)
        if self.lod < 2:
            self.box((x, y-.22, z+1.35), (.06, .16, 2.7), 'titanium', 0)
            self.box((x, y-1, z+3.15), (width+1, 2.1, .22), 'graphite')

    def roof_vents(self, x, y, z, w=2):
        self.box((x, y, z+.45), (w, 1.5, .9), 'titanium')
        if self.lod == 0:
            for i in range(6):
                self.box((x, y-.62+i*.24, z+.92), (w-.22, .08, .03), 'graphite', 0)


def agent_spire(g):
    g.podium(23, 23)
    g.box((0, 0, 3.4), (15, 15, 5), 'graphite', .3)
    g.door(0, -7.55, width=4)
    for z, w, h in ((9, 11.8, 7), (21, 9.6, 16), (52, 7.8, 19), (67, 6, 10)):
        g.box((0, 0, z), (w, w, h), 'graphite', .24)
        g.windows(0, 0, z-h/2, w, w, int(h/2.6), 5)
        g.box((0, 0, z+h/2), (w+.7, w+.7, .32), 'bronze')
    # Open reactor chamber interrupts the solid tower and remains visible at street level.
    g.cyl((0, 0, 35.8), 1.45, 11.6, 'cyan', top=.9)
    for z in (30.5, 33, 36, 39, 41):
        g.ring((0, 0, z), 2.2, .23, 'graphite')
        g.ring((0, 0, z+.3), 2.3, .055, 'ivory')
    for i in range(8):
        a=i*pi/4
        g.beam((2.8*cos(a), 2.8*sin(a), 29),
               (2.1*cos(a+.4), 2.1*sin(a+.4), 42.5), .18, 'bronze')
    for side in range(4):
        a = side*pi/2+pi/4
        points = [(r*cos(a), r*sin(a), z) for z, r in
                  ((1, 9), (7, 7.4), (26, 6.2), (49, 5.1), (69, 4.1), (77, 2.4))]
        for p, q in zip(points, points[1:]):
            g.beam(p, q, .5, 'titanium', .75)
        g.pipe(points[-2], points[-1], .12, 'ivory')
        for z in range(10, 67, 9):
            r = 7.4-(z-7)*.055
            g.beam((r*cos(a), r*sin(a), z),
                   ((r-1.3)*cos(a), (r-1.3)*sin(a), z-2), .24, 'bronze')
    for z, radius in ((74, 3.5), (79, 3.1), (82, 2.1)):
        g.ring((0, 0, z), radius, .13, 'bronze')
    g.cyl((0, 0, 77.8), 1.1, 10, 'cyan', top=.6)
    g.cyl((0, 0, 84.4), .17, 5, 'titanium', top=.02, n=12)
    for a in range(0, 360, 90):
        angle = a*pi/180
        g.beam((3*cos(angle), 3*sin(angle), 72), (0, 0, 84), .15, 'titanium')


def memory_archive(g):
    g.podium(26, 22)
    for z, w, d, h in ((4, 22, 18, 6), (10, 18, 14, 6), (16, 14, 10, 6)):
        g.box((0, 1, z), (w, d, h), 'ceramic', .28)
        g.windows(0, 1, z-h/2, w, d, 2, 9)
        g.box((0, 1, z+h/2+.1), (w+.5, d+.5, .25), 'graphite')
        g.box((0, 1-d/2, z+h/2+.22), (w+.5, .08, .05), 'bronze', 0)
        if g.lod < 2:
            for x in (-w/2+.7, w/2-.7):
                g.box((x, 1, z), (.2, d+.13, h), 'titanium')
    for x in (-5, 5):
        g.door(x, -8.1, width=2.8)
        g.roof_vents(x, 4, 19.2, 2)
    g.cyl((0, 1, 20), 3.4, 1.1, 'glass')
    for r in (3.2, 3.7):
        g.ring((0, 1, 20.7), r, .09, 'jade')
    for z in (3, 4, 5, 6):
        g.box((0, -8.25, z), (3, .2, .12), 'ivory', 0)


def integration_gate(g):
    g.podium(26, 16)
    for x, h in ((-8, 28), (8, 22)):
        g.box((x, 0, 1+h/2), (6, 9, h), 'graphite', .26)
        g.windows(x, 0, 1, 6, 9, int(h/2.6), 4)
        for y in (-4.65, 4.65):
            g.box((x, y, 1+h/2), (.24, .3, h), 'bronze')
        g.roof_vents(x, 1, h+1, 3)
        g.door(x, -4.85, width=2)
    g.box((0, 0, 18), (11, 4.5, 2.4), 'glass', .18)
    for z in (16.7, 19.3):
        g.box((0, 0, z), (12, 5, .3), 'ceramic')
    rotation = Matrix.Rotation(pi/2, 3, 'X')
    for r, mat, tube in ((5.2, 'graphite', .5), (4.85, 'cyan', .1), (5.7, 'bronze', .12)):
        g.ring((0, -.2, 10), r, tube, mat, rotation)
    for side in (-1, 1):
        g.beam((side*4, 0, 1), (side*5.1, 0, 7), .65, 'titanium')


def mission_terminal(g):
    g.podium(30, 21)
    g.box((0, 1, 4), (26, 15, 6), 'graphite', .35)
    for x in (-9, -3, 3, 9):
        g.door(x, -6.6, width=4)
        g.box((x, -6.65, 5.9), (4.2, .16, .3), 'warm', 0)
        for z in range(2, 5):
            g.box((x, -6.7, z), (3.8, .06, .1), 'titanium', 0)
        g.beam((x, -7.8, 6.5), (x, 1, 10), .38, 'ceramic')
        g.beam((x, 1, 10), (x, 8, 7.3), .38, 'ceramic')
    g.box((8, 2, 11.2), (5, 6, 8.5), 'ceramic', .2)
    g.windows(8, 2, 8, 5, 6, 3, 3)
    g.box((8, 2, 15.7), (6, 7, .45), 'bronze')
    g.cyl((-7, 1, 7.3), 4.6, .35, 'graphite')
    g.ring((-7, 1, 7.5), 4.1, .07, 'ivory')
    g.box((-7, 1, 7.55), (3.5, .18, .03), 'warm', 0)
    g.box((-7, 1, 7.55), (.18, 3.5, .03), 'warm', 0)
    g.pipe((8, 2, 15.8), (8, 2, 19), .07)


def compute_foundry(g):
    g.podium(25, 23)
    g.box((0, 3, 7), (20, 12, 12), 'graphite', .28)
    g.windows(0, 3, 2, 20, 12, 4, 9)
    for x in (-7, 0, 7):
        g.cyl((x, -5, 4.8), 2.5, 8, 'titanium', top=1.9)
        g.cyl((x, -5, 9.1), 2.8, 1, 'graphite', top=2.5)
        g.ring((x, -5, 9.65), 2.35, .09, 'cyan')
        for z in ((2, 3.4, 4.8, 6.2, 7.6) if g.lod < 2 else (3, 7)):
            g.ring((x, -5, z), 2.56-(z-1)*.075, .08, 'bronze')
        g.pipe((x, -5, 2), (x, -8.7, 2), .36, 'bronze')
        g.pipe((x, -8.7, 2), (x, -8.7, 1), .36, 'bronze')
        g.roof_vents(x, 4, 13.1, 3.5)
    if g.lod < 2:
        for x in (-10.25, 10.25):
            for y in range(-2, 9, 2):
                g.box((x, y, 6), (.38, .2, 9), 'ceramic')


def knowledge_atrium(g):
    g.podium(24, 24)
    g.cyl((0, 0, 2.1), 9.6, 2.4, 'graphite', n=12)
    g.cyl((0, 0, 3.35), 9.9, .25, 'graphite', n=12)
    g.ring((0, 0, 3.52), 9.4, .08, 'jade')
    for i in range(12):
        a = i*2*pi/12
        g.beam((8.6*cos(a), 8.6*sin(a), 3.5),
               (7.8*cos(a), 7.8*sin(a), 13), .22, 'ceramic')
        g.beam((7.8*cos(a), 7.8*sin(a), 13), (0, 0, 20), .22, 'ceramic')
    for z, r in ((3.6, 8.6), (13, 7.8), (17, 3.35)):
        g.ring((0, 0, z), r, .17, 'bronze')
    g.cyl((0, 0, 8), .7, 9.2, 'bronze', top=.25)
    for i in range(7):
        a = i*2*pi/7
        p, q = (0, 0, 6+i*.65), (4.2*cos(a), 4.2*sin(a), 10.5+i*.45)
        g.pipe(p, q, .19, 'bronze')
        g.ring(q, .78, .1, 'jade')
        g.cyl(q, .45, .6, 'glass', top=.25, n=8)
        if g.lod < 2:
            for sign in (-1, 1):
                r = (5.8*cos(a+sign*.24), 5.8*sin(a+sign*.24), q[2]+1.8)
                g.pipe(q, r, .085, 'titanium')
                g.cyl(r, .25, .4, 'jade', top=.14, n=8)
    g.door(0, -9.7, 1, 3.2)


def data_tower(g, variant=0):
    w, d, h = (11, 9, 33) if variant == 0 else (9, 12, 44)
    g.podium(w+4, d+4)
    g.box((0, 0, h/2+1), (w, d, h), 'glass', .3)
    g.windows(0, 0, 1.4, w, d, int(h/2.6), 6)
    for x in (-w/2-.14, w/2+.14):
        for y in (-d/2-.14, d/2+.14):
            g.box((x, y, h/2+1), (.42, .42, h+.2), 'ceramic')
    for z in range(4, h+2, 5 if g.lod < 2 else 10):
        g.box((0, 0, z), (w+.65, d+.65, .19), 'titanium', .03)
    g.box((0, 0, h+1.25), (w+1.2, d+1.2, .5), 'bronze')
    g.roof_vents(0, 2, h+1.5, 3)
    g.door(0, -d/2-.2, width=2.4)
    if variant:
        g.box((0, 0, h+3), (w-2, d-2, 3), 'graphite', .18)
        g.cyl((0, 0, h+6.4), .12, 4, 'bronze', top=.03, n=8)


def operations_beacon(g):
    g.podium(9, 9)
    g.cyl((0, 0, 5), 2.5, 8.2, 'graphite', top=1.6, n=8)
    for a in range(4):
        angle = a*pi/2+pi/4
        g.beam((2.4*cos(angle), 2.4*sin(angle), 1),
               (1.5*cos(angle), 1.5*sin(angle), 9), .22, 'bronze')
    g.cyl((0, 0, 10), 2.7, 2, 'glass', n=12)
    for z in (9, 11):
        g.ring((0, 0, z), 2.85, .17, 'titanium')
    g.part = 'signal'
    g.ring((0, 0, 11.5), 2.25, .12, 'warm')
    g.cyl((0, 0, 12.3), .28, 1.2, 'warm', top=.06, n=12)
    g.part = 'structure'
    g.door(0, -2.1, width=1.3)


def skybridge(g):
    g.box((0, 0, .2), (16, 4, .4), 'graphite', .12)
    for y in (-2, 2):
        g.box((0, y, 1.3), (16, .12, 1.4), 'glass', 0)
        for z in (.9, 2.1):
            g.box((0, y, z), (16, .12, .12), 'bronze')
        g.box((0, y, .47), (16, .05, .03), 'ivory', 0)
        for x in range(-8, 9, 4):
            g.beam((x, y, .4), (x, y, 4), .16, 'titanium')
            g.beam((x, y, 4), (x, 0, 4.6), .16, 'titanium')
    g.box((0, 0, 4.7), (16, 4.4, .18), 'graphite')


def street_tile(g):
    g.box((0, 0, .06), (16, 12, .12), 'road', 0)
    for y in (-4.65, 4.65):
        g.box((0, y, .25), (16, 2.7, .5), 'stone', .06)
        g.box((0, y+(-1 if y > 0 else 1)*1.23, .54), (16, .06, .06), 'ivory', 0)
        if g.lod < 2:
            for x in range(-8, 9, 2):
                g.box((x, y, .51), (.027, 2.5, .01), 'graphite', 0)
    for x in (-6, -2, 2, 6):
        g.box((x, 0, .13), (2, .09, .016), 'ceramic', 0)


def street_crossing(g):
    g.box((0, 0, .06), (12, 12, .12), 'road', 0)
    for x in (-4.65, 4.65):
        for y in (-4.65, 4.65):
            g.box((x, y, .25), (2.7, 2.7, .5), 'stone', .06)
            g.box((x, y+(-1 if y > 0 else 1)*1.23, .54),
                  (2.5, .06, .06), 'ivory', 0)
    if g.lod < 2:
        for sign in (-1, 1):
            for v in (-2.6, -1.55, -.5, .55, 1.6, 2.65):
                g.box((v, sign*4.4, .14), (.44, 1.15, .02), 'ceramic', 0)
                g.box((sign*4.4, v, .14), (1.15, .44, .02), 'ceramic', 0)


def service_drone(g):
    g.box((0, 0, .7), (1.2, 1.8, .65), 'ceramic', .14)
    g.box((0, -.94, .76), (.8, .2, .3), 'glass')
    g.box((0, -1.05, .76), (.44, .02, .12), 'cyan', 0)
    for x in (-1, 1):
        for y in (-.7, .7):
            g.beam((0, y*.8, .8), (x, y, .8), .15, 'graphite')
            g.ring((x, y, .86), .54, .08, 'graphite')
            g.part = 'rotor_' + ('l' if x < 0 else 'r') + ('f' if y < 0 else 'b')
            g.box((x, y, .88), (.92, .1, .055), 'titanium')
            g.box((x, y, .88), (.1, .92, .055), 'titanium')
            g.part = 'structure'
        g.beam((x*.5, -.5, .48), (x*.65, -.5, .13), .07)
        g.beam((x*.5, .5, .48), (x*.65, .5, .13), .07)


def data_tram(g):
    g.box((0, 0, 1.45), (7.5, 2.8, 2.35), 'ceramic', .32)
    g.box((0, 0, .45), (7.2, 2.55, .65), 'graphite', .2)
    for y in (-1.42, 1.42):
        g.box((0, y, 1.9), (6, .025, .7), 'glass', 0)
        g.box((0, y, 1.2), (6.6, .025, .07), 'cyan', 0)
        for x in (-2, -.65, .65, 2):
            g.box((x, y, 1.95), (.06, .07, .86), 'graphite', 0)
    for x in (-3.78, 3.78):
        for y in (-.82, .82):
            g.box((x, y, 1.3), (.03, .4, .16), 'ivory', 0)
    g.roof_vents(0, 0, 2.68, 2)


def street_lamp(g):
    g.cyl((0, 0, .12), .48, .24, 'stone', n=12)
    g.cyl((0, 0, 3.3), .1, 6.4, 'graphite', top=.06, n=8)
    g.beam((0, 0, 6.1), (1.6, 0, 6.6), .1, 'bronze')
    g.box((1.55, 0, 6.6), (1.4, .42, .12), 'graphite')
    g.box((1.55, 0, 6.52), (1.2, .3, .025), 'ivory', 0)


def planter(g):
    g.box((0, 0, .6), (3.5, 1.6, 1.2), 'ceramic', .16)
    g.box((0, 0, 1.22), (3.15, 1.25, .06), 'road', 0)
    for x, h in ((-1, 2.2), (0, 3.5), (1, 2.5)):
        g.cyl((x, 0, 1.5), .06, 1, 'bronze', n=6)
        for z, r in ((1.7, .48), (2.15, .59), (2.65, .48)):
            g.cyl((x, 0, z+(h-2.2)*.5), r, h*.5, 'leaf', top=.04, n=(10, 8, 6)[g.lod])


def server_rack(g):
    g.box((0, 0, 1.6), (1.4, 1.15, 3.2), 'graphite', .08)
    for i in range(8 if g.lod < 2 else 4):
        z = .3+i*(.35 if g.lod < 2 else .7)
        g.box((0, -.6, z), (1.1, .05, .22), 'titanium', .02)
        g.box((.42, -.64, z), (.08, .03, .06), 'jade', 0)
    g.box((0, -.58, 3), (.8, .05, .09), 'cyan', 0)


ASSETS = [
    ('agent-spire', 'Agent reactor spire', 'agent', agent_spire),
    ('memory-archive', 'Terraced memory archive', 'memory', memory_archive),
    ('integration-gate', 'Integration gateway', 'integration', integration_gate),
    ('mission-terminal', 'Mission logistics terminal', 'missions', mission_terminal),
    ('compute-foundry', 'Compute and cooling foundry', 'infrastructure', compute_foundry),
    ('knowledge-atrium', 'Knowledge tree atrium', 'knowledge', knowledge_atrium),
    ('data-tower-a', 'Data tower A', 'building', lambda g: data_tower(g, 0)),
    ('data-tower-b', 'Data tower B', 'building', lambda g: data_tower(g, 1)),
    ('operations-beacon', 'Operations beacon', 'operations', operations_beacon),
    ('skybridge', 'Modular covered skybridge', 'connector', skybridge),
    ('street-tile', '16 metre street and sidewalk', 'street', street_tile),
    ('street-crossing', 'Matching four-way street crossing', 'street', street_crossing),
    ('service-drone', 'Service quadrotor', 'vehicle', service_drone),
    ('data-tram', 'Data transit tram', 'vehicle', data_tram),
    ('street-lamp', 'Cantilever street lamp', 'prop', street_lamp),
    ('planter', 'Sculpted city planter', 'prop', planter),
    ('server-rack', 'Exposed service rack', 'prop', server_rack),
]


def create_asset(asset_id, factory, lod, collection, mats):
    g = Geometry(lod)
    factory(g)
    root = bpy.data.objects.new(f'{asset_id}.lod{lod}', None)
    collection.objects.link(root)
    root['asset_id'], root['lod'], root['license'] = asset_id, lod, 'MIT'
    for part, (verts, faces, keys, smooth) in g.parts.items():
        mesh = bpy.data.meshes.new(asset_id + '.' + part)
        mesh.from_pydata(verts, [], faces)
        mesh.update()
        unique = list(dict.fromkeys(keys))
        for key in unique:
            mesh.materials.append(mats[key])
        for polygon, key, smoothed in zip(mesh.polygons, keys, smooth):
            polygon.material_index = unique.index(key)
            polygon.use_smooth = smoothed
        obj = bpy.data.objects.new(part, mesh)
        obj.parent = root
        collection.objects.link(obj)
        if part.startswith('rotor_'):
            center = sum((v.co for v in mesh.vertices), Vector()) / len(mesh.vertices)
            mesh.transform(Matrix.Translation(-center))
            obj.location = center
        obj['component'] = part
    return root


def bounds_and_triangles(root):
    coords, triangles = [], 0
    for obj in root.children:
        if obj.type != 'MESH':
            continue
        obj.data.calc_loop_triangles()
        triangles += len(obj.data.loop_triangles)
        coords.extend(obj.matrix_local @ Vector(p) for p in obj.bound_box)
    low = [min(v[i] for v in coords) for i in range(3)]
    high = [max(v[i] for v in coords) for i in range(3)]
    # Blender Z-up -> glTF Y-up. The integration uses metres and base-centred pivots.
    return {'min': [low[0], low[2], -high[1]], 'max': [high[0], high[2], -low[1]]}, triangles


def export_asset(root, output):
    bpy.ops.object.select_all(action='DESELECT')
    root.select_set(True)
    for child in root.children:
        child.select_set(True)
    bpy.context.view_layer.objects.active = root
    bpy.context.view_layer.update()
    bounds, triangles = bounds_and_triangles(root)
    bpy.ops.export_scene.gltf(filepath=str(output), export_format='GLB', use_selection=True,
                             use_active_scene=True, export_animations=False, export_cameras=False, export_lights=False,
                             export_extras=True, export_texcoords=False, export_normals=True,
                             export_tangents=False, export_yup=True, export_apply=True)
    # Stable names eliminate Blender's session-dependent .001 suffixes.
    data = output.read_bytes()
    json_length = struct.unpack_from('<I', data, 12)[0]
    document = json.loads(data[20:20+json_length])
    for index, node in enumerate(document.get('nodes', [])):
        extras = node.get('extras', {})
        node['name'] = extras.get('component', extras.get('asset_id', f'node{index}'))
    for index, mesh in enumerate(document.get('meshes', [])):
        mesh['name'] = f'mesh{index}'
    for scene in document.get('scenes', []):
        scene['name'] = 'CityAsset'
    payload = json.dumps(document, separators=(',', ':'), sort_keys=True).encode()
    payload += b' ' * (-len(payload) % 4)
    tail = data[20+json_length:]
    data = struct.pack('<III', 0x46546C67, 2, 20+len(payload)+len(tail))
    data += struct.pack('<II', len(payload), 0x4E4F534A) + payload + tail
    output.write_bytes(data)
    return {'file': output.name, 'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest(),
            'triangles': triangles, 'bounds': bounds}


def build():
    for directory in (OUT, SOURCE, REPORT):
        directory.mkdir(parents=True, exist_ok=True)
    scene = bpy.data.scenes.get('AuraGo City Kit') or bpy.data.scenes.new('AuraGo City Kit')
    if bpy.context.window:
        bpy.context.window.scene = scene
    old = bpy.data.collections.get(COLLECTION)
    if old:
        for obj in list(old.all_objects):
            mesh = obj.data if obj.type == 'MESH' else None
            bpy.data.objects.remove(obj, do_unlink=True)
            if mesh and mesh.users == 0:
                bpy.data.meshes.remove(mesh)
        bpy.data.collections.remove(old)
    collection = bpy.data.collections.new(COLLECTION)
    scene.collection.children.link(collection)
    mats = materials()
    manifest = {'schema_version': 1, 'version': '1.0.0', 'license': 'MIT',
                'generator': 'Blender 5.2.1 LTS / build_city.py', 'units': 'metres',
                'up_axis': 'Y', 'textures': 0, 'budget_bytes': 8*1024*1024, 'assets': []}
    preview_roots = []
    for index, (asset_id, label, category, factory) in enumerate(ASSETS):
        entry = {'id': asset_id, 'name': label, 'category': category, 'lods': []}
        for lod in range(3):
            root = create_asset(asset_id, factory, lod, collection, mats)
            result = export_asset(root, OUT / f'{asset_id}.lod{lod}.glb')
            result['level'] = lod
            entry['lods'].append(result)
            if lod == 0:
                preview_roots.append(root)
            else:
                for child in list(root.children):
                    mesh = child.data
                    bpy.data.objects.remove(child, do_unlink=True)
                    if mesh.users == 0:
                        bpy.data.meshes.remove(mesh)
                bpy.data.objects.remove(root, do_unlink=True)
        entry['components'] = [child.get('component') for child in preview_roots[-1].children]
        manifest['assets'].append(entry)
        print('EXPORTED', asset_id, sum(v['bytes'] for v in entry['lods']))
    manifest['total_model_bytes'] = sum(l['bytes'] for a in manifest['assets'] for l in a['lods'])
    assert manifest['total_model_bytes'] <= manifest['budget_bytes'], 'City kit exceeds 8 MiB'
    (OUT / 'manifest.json').write_text(json.dumps(manifest, indent=2)+'\n', encoding='utf-8', newline='\n')
    (OUT / 'LICENSE.txt').write_text((ROOT / 'LICENSE').read_text(encoding='utf-8'), encoding='utf-8', newline='\n')
    # A clean authoring contact sheet; scene dressing is added by a separate preview script.
    for index, root in enumerate(preview_roots):
        root.location = ((index % 4)*38, (index//4)*38, 0)
    for obj in list(bpy.context.scene.objects):
        if obj.name in ('Cube', 'Camera', 'Light') and obj.name not in collection.all_objects:
            bpy.data.objects.remove(obj, do_unlink=True)
    bpy.data.libraries.write(str(SOURCE / 'aurago-city-kit.blend'), {scene}, compress=True)
    print('CITY_KIT', json.dumps({'models': len(ASSETS)*3, 'bytes': manifest['total_model_bytes']}))


if __name__ == '__main__':
    build()
