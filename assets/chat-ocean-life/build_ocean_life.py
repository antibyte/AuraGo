"""Deterministic Blender generator for the Ocean chat theme sea life (fish, sharks, turtle, whale).

Run:
    & 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/chat-ocean-life/build_ocean_life.py
    ... -- --preview <dir>        EEVEE renders (side, quarter and front per animal), never into the repo
    ... -- --only fish_tang,whale export only these animals (no manifest, no .blend)

Each animal is authored from parametric surfaces in the glTF frame (Y up, +Z = nose, +X = the
animal's left, origin at the body centre, metres) and exported as one compact GLB into
ui/3d/chat-ocean-life/v1/. Every vertex is converted with to_blender(p) = (p.x, -p.z, p.y) before it
enters Blender; the exporter (export_yup) converts back exactly. Animation happens in the runtime
vertex shaders, driven by two baked UV sets:

    TEXCOORD_0 = (along, part)    along: 0 at the nose, 1 at the tail tip (z position between the first
                                  and the last body station); on fins, flippers and flukes 0 at the
                                  root, 1 at the tip (Builder.plate/fan root_along continues the body
                                  value instead, e.g. for a caudal fin that must follow the body wave)
                                  part : 0 body/shell, 0.25 fin, 0.5 eye, 0.75 flipper, 1 fluke or caudal
    TEXCOORD_1 = (height, side)   height: 0 belly .. 1 back (fins: normalised chord/width coordinate)
                                  side  : across the body 0 at the +X flank, 0.5 on the midline, 1 at
                                          the -X flank (a ramp, so markings can use it as a coordinate);
                                          plates and eyes carry one value from the sign of their x

The species functions below specify the exported (glTF) UV values directly; Builder.build flips V
because Blender's UV origin is bottom-left and the glTF exporter writes v = 1 - v.

Materials are named after the part so the runtime can assign its own shaders:
body, fin, eye, shell, flipper, fluke.
"""

from bisect import bisect_right
from math import sin, cos, tan, radians, pi, sqrt, copysign
from pathlib import Path
import hashlib
import json
import os
import random
import struct
import sys

import bpy
import bmesh
from mathutils import Vector

ROOT = Path(__file__).resolve().parent
OUTPUT = ROOT.parents[1] / 'ui' / '3d' / 'chat-ocean-life' / 'v1'
PRODUCTION = ROOT / 'production'
TAU = 2 * pi
POLE = 1e-7
WELD = 1e7
DOUBLE_SIDED = ('fin', 'flipper', 'fluke')  # thin plates; the closed body surfaces are culled

LICENSE_TEXT = """MIT License

Copyright (c) 2026 AuraGo contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
"""

# Preview-only base colours so shapes read in the renders (the runtime shaders colour the animals).
PREVIEW_COLORS = {
    'body': (0.55, 0.60, 0.65), 'fin': (0.75, 0.75, 0.60), 'eye': (0.02, 0.02, 0.02),
    'shell': (0.35, 0.40, 0.25), 'flipper': (0.30, 0.36, 0.22), 'fluke': (0.25, 0.28, 0.32),
}


# ---------------------------------------------------------------------------
# small vector helpers (plain tuples keep the species code readable)
# ---------------------------------------------------------------------------

def sub(a, b):
    return (a[0] - b[0], a[1] - b[1], a[2] - b[2])


def scale(a, k):
    return (a[0] * k, a[1] * k, a[2] * k)


def dot(a, b):
    return a[0] * b[0] + a[1] * b[1] + a[2] * b[2]


def cross(a, b):
    return (a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0])


def normalize(v):
    length = sqrt(v[0] * v[0] + v[1] * v[1] + v[2] * v[2]) or 1.0
    return (v[0] / length, v[1] / length, v[2] / length)


def clamp(v, lo=0.0, hi=1.0):
    return lo if v < lo else hi if v > hi else v


def newell(points):
    """Area-weighted polygon normal (twice the area long)."""
    nx = ny = nz = 0.0
    count = len(points)
    for i in range(count):
        x0, y0, z0 = points[i]
        x1, y1, z1 = points[(i + 1) % count]
        nx += (y0 - y1) * (z0 + z1)
        ny += (z0 - z1) * (x0 + x1)
        nz += (x0 - x1) * (y0 + y1)
    return (nx, ny, nz)


def to_blender(point):
    """Authored glTF frame (Y up, +Z forward) -> Blender Z-up frame (the exporter undoes this)."""
    return (point[0], -point[2], point[1])


def curve(keys):
    """Monotone cubic (PCHIP) through (x, y) keys; returns f(x), clamped outside the key range."""
    xs = [float(k[0]) for k in keys]
    ys = [float(k[1]) for k in keys]
    n = len(xs)
    h = [xs[i + 1] - xs[i] for i in range(n - 1)]
    d = [(ys[i + 1] - ys[i]) / h[i] for i in range(n - 1)]

    def end_slope(h0, h1, d0, d1):
        m = ((2 * h0 + h1) * d0 - h0 * d1) / (h0 + h1)
        if m * d0 <= 0:
            return 0.0
        if d0 * d1 <= 0 and abs(m) > 3 * abs(d0):
            return 3 * d0
        return m

    if n == 2:
        m = [d[0], d[0]]
    else:
        m = [0.0] * n
        for i in range(1, n - 1):
            if d[i - 1] * d[i] > 0:
                w1 = 2 * h[i] + h[i - 1]
                w2 = h[i] + 2 * h[i - 1]
                m[i] = (w1 + w2) / (w1 / d[i - 1] + w2 / d[i])
        m[0] = end_slope(h[0], h[1], d[0], d[1])
        m[-1] = end_slope(h[-1], h[-2], d[-1], d[-2])

    def f(x):
        if x <= xs[0]:
            return ys[0]
        if x >= xs[-1]:
            return ys[-1]
        i = bisect_right(xs, x) - 1
        t = (x - xs[i]) / h[i]
        t2, t3 = t * t, t * t * t
        return ((2 * t3 - 3 * t2 + 1) * ys[i] + (t3 - 2 * t2 + t) * h[i] * m[i]
                + (-2 * t3 + 3 * t2) * ys[i + 1] + (t3 - t2) * h[i] * m[i + 1])

    return f


# ---------------------------------------------------------------------------
# Blender scene helpers
# ---------------------------------------------------------------------------

def reset_scene():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    for block in (bpy.data.meshes, bpy.data.materials, bpy.data.objects):
        for item in list(block):
            block.remove(item)


def material(name):
    mat = bpy.data.materials.get(name) or bpy.data.materials.new(name)
    mat.use_nodes = True
    mat.use_backface_culling = name not in DOUBLE_SIDED  # exported as glTF doubleSided
    return mat


class Builder:
    """Collects welded vertices and faces with two UV sets and a material per face (glTF frame)."""

    def __init__(self, name):
        self.name = name
        self.verts = []
        self.faces = []  # (indices, material, uv0 per corner, uv1 per corner)
        self.materials = []
        self._weld = {}

    def mat_index(self, name):
        if name not in self.materials:
            self.materials.append(name)
        return self.materials.index(name)

    def vert(self, p):
        key = (int(round(p[0] * WELD)), int(round(p[1] * WELD)), int(round(p[2] * WELD)))
        found = self._weld.get(key)
        if found is None:
            found = len(self.verts)
            self.verts.append((float(p[0]), float(p[1]), float(p[2])))
            self._weld[key] = found
        return found

    def face(self, indices, m, uv0, uv1):
        self.faces.append((list(indices), m, list(uv0), list(uv1)))

    def orient(self, first, outward):
        """Flip faces[first:] when their area-weighted normals point against outward(centroid)."""
        total = 0.0
        for indices, _, _, _ in self.faces[first:]:
            points = [self.verts[i] for i in indices]
            centroid = scale((sum(p[0] for p in points), sum(p[1] for p in points), sum(p[2] for p in points)),
                             1.0 / len(points))
            total += dot(newell(points), outward(centroid))
        if total < 0:
            for k in range(first, len(self.faces)):
                indices, m, c0, c1 = self.faces[k]
                self.faces[k] = (indices[::-1], m, c0[::-1], c1[::-1])

    def grid(self, rows, cols, point, uv0, uv1, mat, wrap=False, flip=False, outward=None):
        """Add a (rows+1) x (cols+1) surface; point(i, j) -> xyz; uv functions return exported UVs.
        Points that coincide (poles, collapsed edges) are welded, so their quads become triangles."""
        ncols = cols if wrap else cols + 1
        ids = {}
        for i in range(rows + 1):
            for j in range(ncols):
                ids[(i, j)] = self.vert(point(i, j))
        m = self.mat_index(mat)
        first = len(self.faces)
        for i in range(rows):
            for j in range(cols):
                quad = [(i, j), (i, j + 1), (i + 1, j + 1), (i + 1, j)]
                if flip:
                    quad.reverse()
                self.face([ids[(a, b % ncols if wrap else b)] for a, b in quad], m,
                          [uv0(a, b) for a, b in quad], [uv1(a, b) for a, b in quad])
        if outward is not None:
            self.orient(first, outward)

    def tube(self, path, radius, sides, mat, part=0.0):
        """Thin tube along a list of points; radius(t) tapers along the path (a zero radius closes it)."""
        segs = len(path) - 1
        frames = []
        for k in range(len(path)):
            tangent = normalize(sub(path[min(segs, k + 1)], path[max(0, k - 1)]))
            ref = (0.0, 0.0, 1.0) if abs(tangent[2]) < 0.9 else (1.0, 0.0, 0.0)
            n1 = normalize(cross(tangent, ref))
            frames.append((n1, cross(tangent, n1)))

        def point(i, j):
            n1, n2 = frames[i]
            ang = TAU * j / sides
            r = radius(i / segs)
            p = path[i]
            return (p[0] + (n1[0] * cos(ang) + n2[0] * sin(ang)) * r,
                    p[1] + (n1[1] * cos(ang) + n2[1] * sin(ang)) * r,
                    p[2] + (n1[2] * cos(ang) + n2[2] * sin(ang)) * r)

        self.grid(segs, sides, point,
                  lambda i, j: (i / segs, part),
                  lambda i, j: (0.5, 0.5),
                  mat, wrap=True, outward=lambda c: sub(c, min(path, key=lambda p: sum((c[a] - p[a]) ** 2 for a in range(3)))))

    @staticmethod
    def _station(s):
        """(z, half_width, half_height, center_y[, power[, bottom_half_height]]) -> 6-tuple."""
        z, hw, hh, cy = s[0], s[1], s[2], s[3]
        pw = s[4] if len(s) > 4 and s[4] else 2.0
        hb = s[5] if len(s) > 5 and s[5] is not None else hh
        return (z, hw, hh, cy, pw, hb)

    def loft(self, stations, sides, mat, part=0.0, along=None):
        """Closed body of revolution with (super)elliptic sections.
        stations: (z, half_width_x, half_height_y, center_y[, power[, bottom_half_height]]), nose first.
        A station with no radius is a pole; a station with a radius gets a flat cap, so the body is closed.
        along(i) defaults to the normalised z position between the first and the last station.
        UV0 = (along, part); UV1 = (height 0 belly..1 back from the ring, side 0 at +X .. 1 at -X)."""
        stations = [self._station(s) for s in stations]
        n = len(stations)
        zs = [s[0] for s in stations]
        span = zs[0] - zs[-1]

        def along_at(i):
            if along is not None:
                return along(i)
            return (zs[0] - zs[i]) / span if span else i / (n - 1)

        def ring(j, pw):
            a = TAU * j / sides
            c, s = cos(a), sin(a)
            c = 0.0 if abs(c) < 1e-12 else c
            s = 0.0 if abs(s) < 1e-12 else s
            e = 2.0 / pw
            return copysign(abs(c) ** e, c), copysign(abs(s) ** e, s)

        def point(i, j):
            z, hw, hh, cy, pw, hb = stations[i]
            c, s = ring(j, pw)
            return (hw * c, cy + (hh if s >= 0 else hb) * s, z)

        def uv1(i, j):
            z, hw, hh, cy, pw, hb = stations[i]
            if hw <= POLE and hh <= POLE:
                return (0.5, 0.5)
            c, s = ring(j, pw)
            return ((s + 1.0) / 2.0, 0.5 - 0.5 * c)

        def outward(c):
            k = min(range(n), key=lambda i: abs(stations[i][0] - c[2]))
            return (c[0], c[1] - stations[k][3], 0.0)

        self.grid(n - 1, sides, point, lambda i, j: (along_at(i), part), uv1, mat, wrap=True, outward=outward)
        nose_dir = 1.0 if zs[0] > zs[-1] else -1.0
        for i, direction in ((0, nose_dir), (n - 1, -nose_dir)):
            z, hw, hh, cy, pw, hb = stations[i]
            if hw <= POLE and hh <= POLE:
                continue
            self.grid(1, sides,
                      lambda r, j, i=i, cy=cy, z=z: point(i, j) if r == 0 else (0.0, cy, z),
                      lambda r, j, i=i: (along_at(i), part),
                      lambda r, j, i=i: uv1(i, j) if r == 0 else (0.5, 0.5),
                      mat, wrap=True, outward=lambda c, d=direction: (0.0, 0.0, d))

    def sphere(self, center, radius, mat, part=0.5, along=0.0, squash=(1.0, 1.0, 1.0), rings=8, segs=12):
        """Small closed ellipsoid (eyes). UV0 = (along, part); UV1 = (latitude height, side of the centre x)."""
        cx, cy, cz = center
        side = 0.0 if cx > 1e-6 else (1.0 if cx < -1e-6 else 0.5)

        def point(i, j):
            th, ph = pi * i / rings, TAU * j / segs
            return (cx + radius * squash[0] * sin(th) * cos(ph),
                    cy + radius * squash[1] * cos(th),
                    cz + radius * squash[2] * sin(th) * sin(ph))

        self.grid(rings, segs, point,
                  lambda i, j: (along, part),
                  lambda i, j: (0.5 + 0.5 * cos(pi * i / rings), side),
                  mat, wrap=True, outward=lambda c: sub(c, center))

    def _plane(self, outline_pts, origin, axis_u, axis_v, part, root_along, fit, box=None):
        """Shared by plate() and fan(): placement and exported UVs for (u, v) points of a flat fin.
        box = (umin, umax, vmin, vmax) overrides the UV normalisation range (pieces of one fin share it)."""
        us = [p[0] for p in outline_pts]
        vs = [p[1] for p in outline_pts]
        umin, umax, vmin, vmax = box if box else (min(us), max(us), min(vs), max(vs))
        urange = (umax - umin) or 1.0
        vrange = (vmax - vmin) or 1.0

        def place(u, v):
            p = (origin[0] + axis_u[0] * u + axis_v[0] * v,
                 origin[1] + axis_u[1] * u + axis_v[1] * v,
                 origin[2] + axis_u[2] * u + axis_v[2] * v)
            return fit(p) if fit else p

        x_mean = sum(place(u, v)[0] for u, v in outline_pts) / len(outline_pts)
        side = 0.0 if x_mean > 1e-4 else (1.0 if x_mean < -1e-4 else 0.5)

        def uv0(u, v):
            un = clamp((u - umin) / urange)
            return ((un if root_along is None else root_along + (1.0 - root_along) * un), part)

        def uv1(u, v):
            return (clamp((v - vmin) / vrange), side)

        return place, uv0, uv1

    def plate(self, outline, origin, axis_u, axis_v, mat, part, root_along=None, rows=2, fit=None):
        """Thin fin plate. `outline` lists the lower edge root->tip, then the upper edge root->tip, as
        (u, v) points in the plane spanned by axis_u (root->tip) and axis_v, placed at `origin`; column k
        joins lower[k] and upper[k] with `rows` quads across. UV0 = (u normalised root->tip, part) or, with
        root_along, root_along..1; UV1 = (v normalised 0..1, side flag from the sign of the plate's x).
        fit(p) -> p may bend the placed points (fins follow the body contour)."""
        n = len(outline) // 2
        lower, upper = outline[:n], outline[n:]
        place, uv0, uv1 = self._plane(outline, origin, axis_u, axis_v, part, root_along, fit)

        def uv_at(i, j):
            f = j / rows
            return (lower[i][0] + (upper[i][0] - lower[i][0]) * f, lower[i][1] + (upper[i][1] - lower[i][1]) * f)

        self.grid(n - 1, rows,
                  lambda i, j: place(*uv_at(i, j)),
                  lambda i, j: uv0(*uv_at(i, j)),
                  lambda i, j: uv1(*uv_at(i, j)),
                  mat)

    def fan(self, perimeter, origin, axis_u, axis_v, mat, part, apex=(0.0, 0.0), rings=3, root_along=None, fit=None,
            box=None):
        """Flat fin from a star-shaped perimeter (u, v), in order, around `apex`; `rings` radial quads.
        The outline is open: the apex must lie on the segment between its first and last point, or the
        first point must be repeated at the end (a closed lobe around an interior apex).
        Every step must advance the angle around the apex in one direction: a step that turns back makes
        folded quads whose opposite windings cancel in the smoothed normals (dark slivers).
        Same UV rule as plate(); suited to forked or notched tails and triangular fins."""
        place, uv0, uv1 = self._plane(list(perimeter) + [apex], origin, axis_u, axis_v, part, root_along, fit, box)
        m = len(perimeter)

        def uv_at(i, j):
            f = i / rings
            return (apex[0] + (perimeter[j][0] - apex[0]) * f, apex[1] + (perimeter[j][1] - apex[1]) * f)

        self.grid(rings, m - 1, lambda i, j: place(*uv_at(i, j)),
                  lambda i, j: uv0(*uv_at(i, j)), lambda i, j: uv1(*uv_at(i, j)), mat)

    def foil(self, stations, mat, part, rows=6):
        """Thin wedge-shaped wing (whale flukes and pectorals): a top and a bottom surface that meet along the
        leading edge, the trailing edge and, where the thickness reaches zero, the tip, so the wing has a
        visible profile from every side. `stations` runs root -> tip (or tip -> tip through the midline); each
        is (point, chord_axis, normal, lead, trail, thickness, along, side): `point` lies on the span line,
        `chord_axis` is the unit vector from the leading to the trailing edge, `normal` the unit up vector of
        the wing, lead/trail the chord offsets of both edges along chord_axis in metres, thickness the full
        thickness at the thickest chord position (a rounded leading edge fading to a thin trailing edge).
        UV0 = (along, part); UV1 = (chord position 0 leading .. 1 trailing, side)."""
        cols = len(stations) - 1

        def section(c):
            return sqrt(c) * (1.0 - c) / 0.3849  # peaks at c = 1/3

        def point(i, j, sign):
            p, v, n, lo, hi, th, _, _ = stations[j]
            c = i / rows
            d = lo + (hi - lo) * c
            h = 0.5 * th * section(c) * sign
            return (p[0] + v[0] * d + n[0] * h, p[1] + v[1] * d + n[1] * h, p[2] + v[2] * d + n[2] * h)

        for sign in (1.0, -1.0):
            self.grid(rows, cols, lambda i, j, s=sign: point(i, j, s),
                      lambda i, j: (clamp(stations[j][6]), part),
                      lambda i, j: (i / rows, clamp(stations[j][7])),
                      mat, outward=lambda c, s=sign: (0.0, s, 0.0))

    def extent_z(self):
        zs = [v[2] for v in self.verts]
        return max(zs) - min(zs)

    def scale_to_extent_z(self, length):
        """Scale every vertex uniformly about the origin so the nose-to-tail z extent equals `length`.
        Only the positions change (UVs are scale free); call it after all parts are added."""
        factor = length / self.extent_z()
        self.verts = [(v[0] * factor, v[1] * factor, v[2] * factor) for v in self.verts]
        return factor

    def warp(self, fn):
        """Move every vertex added so far through fn(p) -> p (a notch cut into a finished loft). The weld table
        is rebuilt, so parts added afterwards still weld against the moved positions; UVs are not touched."""
        self.verts = [tuple(float(c) for c in fn(v)) for v in self.verts]
        self._weld = {}
        for index, v in enumerate(self.verts):
            self._weld.setdefault((int(round(v[0] * WELD)), int(round(v[1] * WELD)), int(round(v[2] * WELD))), index)

    def build(self):
        mesh = bpy.data.meshes.new(self.name)
        bm = bmesh.new()
        bverts = [bm.verts.new(to_blender(v)) for v in self.verts]
        uv0 = bm.loops.layers.uv.new('UVMap')
        uv1 = bm.loops.layers.uv.new('Anim')
        for indices, m, c0, c1 in self.faces:
            # drop repeated corners (welded poles and collapsed fin tips)
            keep = [k for k in range(len(indices)) if indices[k] != indices[k - 1]]
            if len(keep) < 3:
                continue
            ids = [indices[k] for k in keep]
            if len(set(ids)) < 3:
                continue
            try:
                face = bm.faces.new([bverts[i] for i in ids])
            except ValueError:
                continue
            face.material_index = m
            face.smooth = True
            for loop, k in zip(face.loops, keep):
                loop[uv0].uv = (c0[k][0], 1.0 - c0[k][1])  # glTF exporter writes v = 1 - v
                loop[uv1].uv = (c1[k][0], 1.0 - c1[k][1])
        bm.to_mesh(mesh)
        bm.free()
        for name in self.materials:
            mesh.materials.append(material(name))
        obj = bpy.data.objects.new(self.name, mesh)
        bpy.context.scene.collection.objects.link(obj)
        return obj


# ---------------------------------------------------------------------------
# body profiles and fin helpers
# ---------------------------------------------------------------------------

class Profile:
    """Body of revolution described by keys over t = 0 (nose) .. 1 (tail end): half height, half width
    and centre line height. Also answers where the surface is, so fins and eyes can sit on it."""

    def __init__(self, z_nose, z_tail, hh, hw, cy=None, power=2.0):
        self.z0, self.z1, self.power = z_nose, z_tail, power
        self._hh, self._hw = curve(hh), curve(hw)
        self._cy = curve(cy) if cy else (lambda t: 0.0)

    def t_of(self, z):
        return clamp((self.z0 - z) / (self.z0 - self.z1))

    def z_of(self, t):
        return self.z0 + (self.z1 - self.z0) * t

    def half_height(self, z):
        return self._hh(self.t_of(z))

    def half_width(self, z):
        return self._hw(self.t_of(z))

    def center(self, z):
        return self._cy(self.t_of(z))

    def top(self, z):
        return self.center(z) + self.half_height(z)

    def bottom(self, z):
        return self.center(z) - self.half_height(z)

    def surface_x(self, z, y):
        """Half width of the body surface at height y (superelliptic section)."""
        t = self.t_of(z)
        q = clamp(abs(y - self._cy(t)) / (self._hh(t) or 1e-9), 0.0, 0.98)
        return self._hw(t) * (1.0 - q ** self.power) ** (1.0 / self.power)

    def stations(self, n):
        out = []
        for i in range(n):
            t = (1.0 - cos(pi * i / (n - 1))) / 2.0  # stations cluster at the nose and the tail
            out.append((self.z_of(t), self._hw(t), self._hh(t), self._cy(t), self.power))
        return out


def eyes(b, prof, z, y, radius, squash=0.6, sink=0.2):
    """Two flattened dark spheres seated on the flank of the body."""
    along = prof.t_of(z)
    for sx in (1.0, -1.0):
        x = prof.surface_x(z, y) - radius * sink
        b.sphere((sx * x, y, z), radius, 'eye', 0.5, along, squash=(squash, 1.0, 1.0))


def eyes_on_flank(b, prof, z, y, radius, lift, squash=0.6):
    """Two flattened dark spheres whose centres sit `lift` outside the body surface at (z, y): on a thin
    head (butterflyfish) each eye then shows on its own flank instead of merging at the midline."""
    along = prof.t_of(z)
    for sx in (1.0, -1.0):
        b.sphere((sx * (prof.surface_x(z, y) + lift), y, z), radius, 'eye', 0.5, along, squash=(squash, 1.0, 1.0))


def rounded(sweep=0.0, steps=8):
    """Half-elliptic fin outline factory: f(chord, height) -> perimeter [(u, v)] (u = height above the base)."""
    def make(chord, height):
        lead, trail = [], []
        for k in range(steps + 1):
            a = (pi / 2) * k / steps
            lead.append((height * sin(a), chord * (0.5 - 0.5 * cos(a) + sweep * sin(a))))
            trail.append((height * sin(a), chord * (0.5 + 0.5 * cos(a) + sweep * sin(a))))
        return lead + trail[-2::-1]
    return make


def triangular(tip=0.8, convex=1.2, concave=1.7, steps=8):
    """Swept triangular (falcate) fin: the tip sits `tip` of the chord behind the front of the base."""
    def make(chord, height):
        lead = [(height * k / steps, chord * tip * (k / steps) ** convex) for k in range(steps + 1)]
        trail = [(height * k / steps, chord * (1.0 - (1.0 - tip) * (k / steps) ** concave)) for k in range(steps)]
        return lead + trail[::-1]
    return make


def spiny(teeth=5, valley=0.90):
    """Dorsal/anal fin with a steep front and a saw-tooth free edge that falls toward the tail (spines);
    the notches between the spine tips drop to `valley` of the tip height."""
    def make(chord, height):
        pts = [(0.0, 0.0), (0.7 * height, 0.04 * chord)]
        span = 0.62
        for k in range(teeth):
            peak = height * (1.0 - 0.45 * k / teeth)
            v0 = chord * (0.10 + span * k / teeth)
            v1 = chord * (0.10 + span * (k + 1) / teeth)
            pts.append((peak, v0))
            pts.append((peak * valley, v1 - 0.08 * (v1 - v0)))
        pts += [(0.45 * height, 0.86 * chord), (0.15 * height, 0.96 * chord), (0.0, chord)]
        return pts
    make.free_edge = True  # a height field over the chord, not star-shaped: median_fin builds it column by column
    return make


def median_fin(b, prof, z_front, z_back, height, shape, up=True, skirt=0.006, part=0.25, rings=3):
    """Dorsal (up) or anal/ventral (down) fin whose base follows the body contour and sinks `skirt` into it.
    Outlines with a saw-tooth free edge (shape.free_edge) are built as a plate of columns from the base to the
    edge, so deep notches cannot fold a fan around its apex."""
    sign = 1.0 if up else -1.0
    edge = prof.top if up else prof.bottom
    z_mid = (z_front + z_back) / 2.0
    y0 = edge(z_mid) - sign * skirt
    total = height + skirt
    chord = z_front - z_back

    def fit(p):
        f = clamp(1.0 - abs(p[1] - y0) / total)
        return (p[0], p[1] + (edge(p[2]) - sign * skirt - y0) * f, p[2])

    outline = shape(chord, total)
    if getattr(shape, 'free_edge', False):
        b.plate([(0.0, p[1]) for p in outline] + list(outline), (0.0, y0, z_front), (0.0, sign, 0.0),
                (0.0, 0.0, -1.0), 'fin', part, rows=rings, fit=fit)
        return
    b.fan(outline, (0.0, y0, z_front), (0.0, sign, 0.0), (0.0, 0.0, -1.0), 'fin', part,
          apex=(0.0, chord / 2.0), rings=rings, fit=fit)


def wing(b, origin, direction, length, edges, mat, part, plane='horizontal', ortho=True, steps=8):
    """Paddle/fin plate. `direction` is root->tip; edges(s) -> (v_lead, v_trail) in metres at s = 0..1.
    plane 'horizontal': chord runs backwards (wings, flippers); 'vertical': chord runs up (fish pectorals).
    ortho=False keeps the chord axis on the pure body axis (flukes: the two halves then share the midline)."""
    u = normalize(direction)
    ref = (0.0, 0.0, -1.0) if plane == 'horizontal' else (0.0, 1.0, 0.0)
    v = normalize(sub(ref, scale(u, dot(ref, u)))) if ortho else ref
    lower, upper = [], []
    for k in range(steps + 1):
        s = k / steps
        lo, hi = edges(s)
        lower.append((length * s, lo))
        upper.append((length * s, hi))
    b.plate(lower + upper, origin, u, v, mat, part)


def leaf(chord, keys=((0, 0.5), (0.25, 0.95), (0.5, 1.0), (0.8, 0.7), (0.95, 0.38), (1.0, 0.0))):
    shape = curve(keys)
    return lambda s: (-0.5 * chord * shape(s), 0.5 * chord * shape(s))


def sickle(chord, keys=((0, 1.0), (0.3, 0.82), (0.6, 0.45), (0.85, 0.2), (1.0, 0.0))):
    shape = curve(keys)
    return lambda s: (0.0, chord * shape(s))


def fish_pectoral(b, prof, z, y, length, chord, out=0.4, down=0.12):
    for sx in (1.0, -1.0):
        origin = (sx * prof.surface_x(z, y) * 0.8, y, z)
        wing(b, origin, (sx * out, -down, -1.0), length, leaf(chord), 'fin', 0.25, plane='vertical', steps=6)


def caudal(b, root_z, root_y, perimeter, rings=4):
    """Caudal fin: flat star-shaped plate behind the peduncle, u back, v up."""
    b.fan(perimeter, (0.0, root_y, root_z), (0.0, 0.0, -1.0), (0.0, 1.0, 0.0), 'fin', 1.0, apex=(0.0, 0.0), rings=rings)


def symmetric(upper):
    """Full perimeter of a fin that is symmetric about v = 0, from its upper half given root -> edge -> centre."""
    lower = [(u, -v) for u, v in upper]
    return lower + upper[::-1][1:]


def forked_caudal(b, root_z, root_y, lobe, apex, rings=3):
    """Deeply forked caudal fin built as two mirrored lobe fans. `lobe` is the closed outline (u, v >= 0) of the
    upper lobe, in order around its own `apex` (an interior point); the lower lobe is its mirror image. Each lobe
    is a star-shaped polygon, so no quad folds over at the fork notch; the lobes meet along the midline and share
    one UV range. The mirrored lobe is walked in reverse so both lobes wind the same way: where they share
    vertices on the midline the smoothed normals must add up, not cancel (a dark patch)."""
    for sign in (1.0, -1.0):
        outline = [(u, sign * v) for u, v in lobe]
        if sign < 0:
            outline.reverse()
        lobe_apex = (apex[0], sign * apex[1])
        b.fan(outline + [outline[0]], (0.0, root_y, root_z), (0.0, 0.0, -1.0), (0.0, 1.0, 0.0), 'fin', 1.0,
              apex=lobe_apex, rings=rings,
              box=(0.0, max(u for u, _ in lobe), -max(v for _, v in lobe), max(v for _, v in lobe)))


def closed_stations(prof, n, depth, taper=((0.5, 0.866), (0.85, 0.527))):
    """Loft stations for a body that ends in a pole instead of a flat cap: the profile's own stations plus
    one per `taper` entry (fraction of `depth` behind the last section, radius factor; the default is a
    quarter ellipse) plus a pole `depth` metres behind it, so the tail stock closes smoothly.
    Returns (stations, along): along stays 1 at the closing stations, so the UVs of the body do not change."""
    stations = prof.stations(n)
    z, hw, hh, cy, pw = stations[-1]
    for frac, r in taper:
        stations.append((z - depth * frac, hw * r, hh * r, cy, pw))
    stations.append((z - depth, 0.0, 0.0, cy, pw))
    ts = [prof.t_of(s[0]) for s in stations]
    return stations, (lambda i: ts[i])


# ---------------------------------------------------------------------------
# species
# ---------------------------------------------------------------------------

def fish_tang():
    """Blue tang (Paracanthurus hepatus): tall laterally compressed disc, lunate tail. 0.30 m."""
    prof = Profile(0.130, -0.100,
                   hh=[(0, 0), (0.015, 0.014), (0.05, 0.027), (0.1, 0.041), (0.2, 0.057), (0.35, 0.067),
                       (0.5, 0.068), (0.65, 0.058), (0.8, 0.042), (0.92, 0.029), (1.0, 0.023)],
                   hw=[(0, 0), (0.02, 0.007), (0.06, 0.012), (0.15, 0.018), (0.4, 0.022), (0.7, 0.017), (1.0, 0.009)],
                   cy=[(0, -0.012), (0.15, -0.005), (0.4, 0.0), (1.0, 0.0)])
    b = Builder('fish_tang')
    stations, along = closed_stations(prof, 18, 0.020)  # the tail stock tapers to a pole, no flat cap behind it
    b.loft(stations, 20, 'body', along=along)
    median_fin(b, prof, 0.080, -0.082, 0.036, rounded(sweep=0.16))
    median_fin(b, prof, 0.030, -0.082, 0.030, rounded(sweep=0.18), up=False)
    fish_pectoral(b, prof, 0.040, -0.014, 0.048, 0.026)
    # lunate / truncate tail: pointed corners and a shallow notch (apex at 0.85 of the lobe tip distance)
    caudal(b, -0.092, 0.0, symmetric([(0.0, 0.018), (0.020, 0.032), (0.045, 0.049), (0.070, 0.058),
                                      (0.082, 0.062), (0.078, 0.046), (0.073, 0.026), (0.070, 0.0)]))
    eyes(b, prof, 0.100, 0.022, 0.009)
    return b, 0.30


def fish_clown():
    """Clownfish (Amphiprion): stubby ovoid with round fins. 0.12 m."""
    prof = Profile(0.050, -0.040,
                   hh=[(0, 0), (0.02, 0.012), (0.06, 0.021), (0.15, 0.028), (0.35, 0.032), (0.55, 0.031),
                       (0.75, 0.026), (0.9, 0.020), (1.0, 0.016)],
                   hw=[(0, 0), (0.03, 0.008), (0.1, 0.014), (0.35, 0.017), (0.6, 0.016), (0.8, 0.012), (1.0, 0.008)],
                   cy=[(0, -0.004), (0.3, 0.0), (1.0, 0.0)])
    b = Builder('fish_clown')
    stations, along = closed_stations(prof, 16, 0.012)  # the tail stock tapers to a pole, no flat cap behind it
    b.loft(stations, 20, 'body', along=along)
    median_fin(b, prof, 0.030, -0.030, 0.012, rounded(sweep=0.08), skirt=0.004)
    median_fin(b, prof, -0.006, -0.030, 0.009, rounded(sweep=0.08), up=False, skirt=0.004)
    fish_pectoral(b, prof, 0.014, -0.008, 0.020, 0.020, out=0.5)
    # broad rounded fan: half height 0.028 at its widest, 11 outline points per half (neck, flare, 9 on the ellipse)
    fan_half = [(0.0, 0.012), (0.004, 0.0185)]
    for k in range(9):
        phi = radians(100.0 - 100.0 * k / 8.0)
        fan_half.append((0.012 + 0.022 * cos(phi), 0.028 * sin(phi)))
    fan_half[-1] = (0.034, 0.0)
    caudal(b, -0.036, 0.0, symmetric(fan_half), rings=3)
    eyes(b, prof, 0.036, 0.010, 0.0045)
    return b, 0.12


def fish_butterfly():
    """Butterflyfish (Chaetodon): tall disc, pointed snout, spiny dorsal. 0.20 m."""
    prof = Profile(0.085, -0.075,
                   hh=[(0, 0), (0.02, 0.0045), (0.1, 0.0060), (0.17, 0.0075), (0.22, 0.014), (0.28, 0.032),
                       (0.35, 0.046), (0.43, 0.056), (0.52, 0.062), (0.62, 0.060), (0.72, 0.053),
                       (0.82, 0.043), (0.91, 0.033), (1.0, 0.024)],
                   hw=[(0, 0), (0.03, 0.003), (0.1, 0.0035), (0.2, 0.005), (0.3, 0.010), (0.5, 0.012),
                       (0.8, 0.009), (1.0, 0.006)],
                   cy=[(0, -0.004), (0.25, -0.002), (0.5, 0.0), (1.0, 0.0)])
    b = Builder('fish_butterfly')
    # the tail stock tapers to a pole 0.014 m behind the last section (half height 0.024 -> 0.0127 -> 0), about 0.2 of
    # the body depth where the fin starts, instead of ending in a flat 0.048 m tall cap
    stations, along = closed_stations(prof, 20, 0.014)
    b.loft(stations, 20, 'body', along=along)
    median_fin(b, prof, 0.032, -0.066, 0.030, spiny(6, valley=0.72), skirt=0.006, rings=3)
    median_fin(b, prof, 0.010, -0.066, 0.026, spiny(3), up=False, skirt=0.006, rings=3)
    fish_pectoral(b, prof, 0.028, -0.006, 0.030, 0.018)
    # the caudal root (half height 0.008) is no taller than the peduncle it grows out of
    caudal(b, -0.072, 0.0, symmetric([(0.0, 0.008), (0.014, 0.020), (0.030, 0.030), (0.040, 0.030),
                                      (0.042, 0.016), (0.042, 0.0)]), rings=3)
    # t ~ 0.28 (half width >= 0.008): each eye sits 1.5 mm outside the flank surface, above the centre line
    eyes_on_flank(b, prof, 0.040, 0.012, 0.0055, 0.0015)
    return b, 0.20


def fish_sardine():
    """Sardine (Sardina): slim fusiform schooling fish with a deeply forked tail. 0.15 m."""
    prof = Profile(0.065, -0.050,
                   hh=[(0, 0), (0.02, 0.007), (0.08, 0.012), (0.2, 0.0155), (0.4, 0.017), (0.6, 0.0145),
                       (0.8, 0.009), (1.0, 0.0055)],
                   hw=[(0, 0), (0.03, 0.005), (0.1, 0.009), (0.4, 0.0125), (0.7, 0.010), (1.0, 0.004)],
                   cy=[(0, 0.0), (1.0, 0.0)])
    b = Builder('fish_sardine')
    stations, along = closed_stations(prof, 16, 0.010)  # the tail stock tapers to a pole, no flat cap behind it
    b.loft(stations, 16, 'body', along=along)
    median_fin(b, prof, 0.014, -0.012, 0.011, triangular(tip=0.6), skirt=0.003, rings=2)
    median_fin(b, prof, -0.010, -0.020, 0.008, triangular(tip=0.5), up=False, skirt=0.003, rings=2)
    median_fin(b, prof, -0.028, -0.045, 0.008, triangular(tip=0.4, concave=1.2), up=False, skirt=0.003, rings=2)
    fish_pectoral(b, prof, 0.036, -0.006, 0.020, 0.010, out=0.5)
    # two pointed lobes (upper lobe outline root -> notch -> tip -> leading edge back to the root), each a
    # monotone fan around its own apex; the notch apex sits at u = 0.016 on the midline
    forked_caudal(b, -0.048, 0.0,
                  [(0.0, 0.0), (0.016, 0.0), (0.0205, 0.0085), (0.0275, 0.0175), (0.037, 0.026),
                   (0.0285, 0.0215), (0.019, 0.0155), (0.0095, 0.0105), (0.0, 0.0045)],
                  apex=(0.0145, 0.0065), rings=3)
    eyes(b, prof, 0.052, 0.004, 0.0042)
    return b, 0.15


# Reef shark body radius over t (nose 0 .. peduncle 1), metres; the hammerhead reuses it scaled.
SHARK_RADIUS = [(0, 0), (0.012, 0.036), (0.04, 0.068), (0.08, 0.094), (0.14, 0.116), (0.22, 0.130),
                (0.32, 0.136), (0.42, 0.133), (0.52, 0.119), (0.62, 0.099), (0.74, 0.075), (0.87, 0.052),
                (1.0, 0.040)]
# Heterocercal caudal outline (u back, v up), reef shark size: short lower lobe, long upper lobe at ~40 degrees.
SHARK_TAIL = [(0.0, -0.032), (0.05, -0.06), (0.12, -0.10), (0.20, -0.14), (0.215, -0.15),
              (0.17, -0.09), (0.15, -0.04), (0.15, 0.0),
              (0.18, 0.06), (0.24, 0.14), (0.30, 0.23), (0.36, 0.30), (0.40, 0.34),
              (0.33, 0.31), (0.25, 0.24), (0.16, 0.16), (0.08, 0.10), (0.03, 0.06), (0.0, 0.034)]


def shark_fins(b, prof, f, dorsal_height, dorsal_tip, tail_scale, pectoral):
    """Fins shared by both sharks; positions are fractions t of the body, sizes scale with f."""
    z = prof.z_of
    median_fin(b, prof, z(0.38), z(0.58), dorsal_height, triangular(tip=dorsal_tip), skirt=0.03 * f)
    median_fin(b, prof, z(0.85), z(0.93), 0.07 * f, triangular(tip=0.85), skirt=0.02 * f)
    median_fin(b, prof, z(0.875), z(0.945), 0.065 * f, triangular(tip=0.25, concave=1.2), up=False, skirt=0.02 * f)
    for sx in (1.0, -1.0):
        zp = z(0.33)
        yp = prof.center(zp) - 0.5 * prof.half_height(zp)
        wing(b, (sx * prof.surface_x(zp, yp) * 0.7, yp, zp), (sx * 0.85, -0.42, -0.45), pectoral[0],
             sickle(pectoral[1]), 'fin', 0.25)
        zv = z(0.74)
        yv = prof.center(zv) - 0.7 * prof.half_height(zv)
        wing(b, (sx * prof.surface_x(zv, yv) * 0.6, yv, zv), (sx * 0.5, -0.55, -0.7), 0.13 * f,
             sickle(0.08 * f), 'fin', 0.25, steps=5)
    perimeter = [(u * tail_scale, v * tail_scale) for u, v in SHARK_TAIL]
    zt = prof.z1 + 0.02 * f
    b.fan(perimeter, (0.0, prof.center(prof.z1), zt), (0.0, 0.0, -1.0), (0.0, 1.0, 0.0), 'fin', 1.0,
          apex=(0.0, 0.0), rings=4)


def shark_reef():
    """Blacktip reef shark (Carcharhinus melanopterus): torpedo body, tall first dorsal. 1.8 m."""
    flat = curve([(0, 0.85), (0.2, 0.88), (0.4, 1.0), (1.0, 1.0)])
    prof = Profile(0.810, -0.600,
                   hh=[(t, r * flat(t)) for t, r in SHARK_RADIUS],
                   hw=list(SHARK_RADIUS),
                   cy=[(0, -0.008), (0.3, 0.0), (0.7, 0.006), (1.0, 0.02)])
    b = Builder('shark_reef')
    stations, along = closed_stations(prof, 24, 0.07)  # the tail stock closes to a pole behind the peduncle
    b.loft(stations, 28, 'body', along=along)
    shark_fins(b, prof, 1.0, 0.20, 0.82, 1.03, (0.34, 0.18))
    eyes(b, prof, prof.z_of(0.085), 0.025, 0.02, squash=0.7, sink=0.3)
    return b, 1.8


def shark_hammer():
    """Great hammerhead (Sphyrna mokarran): flat cephalofoil with eyes on the lobe ends, tall sickle dorsal. 2.6 m."""
    k = 1.25
    flat = curve([(0, 0.85), (0.2, 0.88), (0.4, 1.0), (1.0, 1.0)])
    body = [(t, r) for t, r in SHARK_RADIUS if t >= 0.32]
    prof = Profile(1.100, -0.850,
                   # cephalofoil: a flat bar, half height 0.060-0.066 m (a 0.12-0.13 m deep slab) from the leading
                   # edge to t = 0.16, so the side view shows a thin horizontal plank; the depth then steps up to the
                   # body (half height 0.164) between t = 0.16 and 0.26 (the neck). The bar is as wide as the lobes at
                   # the leading edge (0.40 m) and reaches the full +/-0.42 m (32 % of the length) by t = 0.03; the
                   # trailing edge then falls off steeply (0.27 m at t = 0.08) to the 0.17 m neck by t ~ 0.15, so
                   # the lobes keep square shoulders instead of a spade outline
                   hh=[(0, 0.060), (0.02, 0.064), (0.08, 0.066), (0.16, 0.068), (0.20, 0.118),
                       (0.26, 0.164)] + [(t, r * k * flat(t)) for t, r in body],
                   hw=[(0, 0.40), (0.012, 0.418), (0.03, 0.42), (0.05, 0.41), (0.065, 0.36), (0.08, 0.27),
                       (0.095, 0.21), (0.115, 0.185), (0.15, 0.172), (0.25, 0.168)] + [(t, r * k) for t, r in body],
                   # the bar sits low so its underside runs on into the belly line (-0.15 vs -0.18 m)
                   cy=[(0, -0.085), (0.16, -0.085), (0.3, 0.0), (0.7, 0.008), (1.0, 0.025)])
    b = Builder('shark_hammer')
    # 40 sections: the cosine clustering puts four of them between t = 0.04 and 0.13, enough for the lobe shoulders
    stations, along = closed_stations(prof, 40, 0.10)  # the tail stock closes to a pole behind the peduncle

    def boxy(t):
        """Section power: 3.2 (a squared-off bar) over the cephalofoil, back to 2.0 (elliptic) at the neck."""
        f = clamp((t - 0.14) / 0.12)
        return 2.0 + 1.2 * (1.0 - f * f * (3.0 - 2.0 * f))

    stations = [s[:4] + (boxy(prof.t_of(s[0])),) for s in stations]
    b.loft(stations, 28, 'body', along=along)

    def notch(p):
        """Median indentation of the leading edge: the front 0.07 m of the bar, |x| < 0.15 m, moves 0.035 m back
        (smoothstep falloff both ways; the slope stays below 1, so no station folds over its neighbour)."""
        d, w = prof.z0 - p[2], abs(p[0])
        if d >= 0.07 or w >= 0.15:
            return p
        fd, fw = 1.0 - d / 0.07, 1.0 - w / 0.15
        return (p[0], p[1], p[2] - 0.035 * fd * fd * (3.0 - 2.0 * fd) * fw * fw * (3.0 - 2.0 * fw))

    b.warp(notch)
    shark_fins(b, prof, 1.45, 0.40, 0.75, 1.7, (0.42, 0.22))  # pectorals no wider than the lobes (~16 % of length)
    # eyes on the lobe ends (middle of the widest section, t = 0.04), centred on the lobe's own height
    z_eye = prof.z_of(0.04)
    eyes(b, prof, z_eye, prof.center(z_eye), 0.032, squash=0.8, sink=0.2)
    return b, 2.6


def turtle_flipper(b, sx, root, direction, length, chord, cap_from, thick_root, thick_tip, rows=6, steps=8,
                   cap_steps=5):
    """Turtle flipper as a thin wedge (Builder.foil), so it keeps a visible profile edge-on. Span s = 0 at the
    root .. 1 at the tip, `direction` runs root -> tip and the chord axis runs backwards. chord(s) is the chord in
    metres up to s = cap_from with a straight leading edge; from there the outline closes in a quarter ellipse
    (rounded tip) and the thickness closes with it. The thickness falls linearly from thick_root at the root to
    thick_tip at cap_from. A zero-thickness station 0.02 m inboard of the root pinches the root end shut, so no
    open end shows where a rear root sits outside the plastron. UV0 along = s, part 0.75; UV1 = (chord position,
    side from the sign of sx)."""
    u = normalize(direction)
    v = normalize(sub((0.0, 0.0, -1.0), scale(u, dot((0.0, 0.0, -1.0), u))))
    n = normalize(cross(u, v))
    if n[1] < 0:
        n = scale(n, -1.0)
    side = 0.0 if sx > 0 else 1.0
    w0 = chord(cap_from)

    def station(s, lead, trail, thickness, along):
        point = (root[0] + u[0] * length * s, root[1] + u[1] * length * s, root[2] + u[2] * length * s)
        return (point, v, n, lead, trail, thickness, along, side)

    stations = [station(-0.02 / length, 0.0, chord(0.0), 0.0, 0.0)]
    for k in range(steps + 1):
        s = cap_from * k / steps
        stations.append(station(s, 0.0, chord(s), thick_root + (thick_tip - thick_root) * k / steps, s))
    for k in range(1, cap_steps + 1):
        a = (pi / 2.0) * k / cap_steps
        w = w0 * cos(a) if k < cap_steps else 0.0
        stations.append(station(cap_from + (1.0 - cap_from) * sin(a), 0.5 * (w0 - w), 0.5 * (w0 + w),
                                thick_tip * cos(a) if k < cap_steps else 0.0, cap_from + (1.0 - cap_from) * sin(a)))
    b.foil(stations, 'flipper', 0.75, rows=rows)


def turtle():
    """Green sea turtle (Chelonia mydas): domed carapace, plastron, head, long front flippers.
    Built with a 1.0 m shell, then scaled uniformly so the whole animal (head tip to tail tip) is 1.0 m."""
    # carapace + plastron as one closed loft: widest at the rim (y = 0), low dome above, shallow belly below;
    # the outline is rounder in front and tapers toward the rear like a real heart-shaped carapace
    def arch(t, centre, p_front, p_rear):
        d = (centre - t) / centre if t < centre else (t - centre) / (1.0 - centre)
        return max(0.0, 1.0 - d ** (p_front if t < centre else p_rear)) ** (1.0 / (p_front if t < centre else p_rear))

    n = 26
    shell = []
    for i in range(n):
        t = (1.0 - cos(pi * i / (n - 1))) / 2.0
        w = arch(t, 0.40, 2.4, 1.8)
        h = arch(t, 0.46, 2.2, 2.0)
        shell.append((0.5 - t, 0.40 * w, 0.23 * h ** 0.9, 0.0, 2.4, 0.055 * w ** 0.75))
    b = Builder('turtle')
    b.loft(shell, 40, 'shell')
    # short head: the nose protrudes ~0.15 m beyond the carapace front (z = 0.5), like a green turtle's
    # the head sits on the mid-front of the carapace, not on its lower rim: centre line about +0.05 m, so its
    # underside (-0.008 m at the neck) stays above the plastron (-0.042 m there)
    head = Profile(0.650, 0.420,
                   hh=[(0, 0), (0.03, 0.020), (0.1, 0.040), (0.3, 0.058), (0.6, 0.066), (1.0, 0.062)],
                   hw=[(0, 0), (0.03, 0.026), (0.1, 0.048), (0.3, 0.062), (0.6, 0.068), (1.0, 0.064)],
                   cy=[(0, 0.042), (0.3, 0.050), (1.0, 0.054)])
    b.loft(head.stations(14), 20, 'body')
    b.tube([(0.0, -0.020, -0.44), (0.0, -0.026, -0.48), (0.0, -0.031, -0.52), (0.0, -0.035, -0.56)],
           lambda t: 0.034 * (1.0 - t) ** 0.8, 10, 'body')  # short tail, ends at z = -0.56
    front = curve([(0, 0.24), (0.3, 0.22), (0.7, 0.145), (0.8, 0.12)])
    rear = curve([(0, 0.14), (0.4, 0.125), (0.7, 0.10)])
    for sx in (1.0, -1.0):
        # front flippers: rooted at the carapace edge, held nearly level and swept back; rear flippers angled out
        turtle_flipper(b, sx, (sx * 0.30, -0.030, 0.26), (sx * 1.0, -0.06, -0.45), 0.68, front, 0.8, 0.05, 0.01, rows=6)
        turtle_flipper(b, sx, (sx * 0.15, -0.045, -0.33), (sx * 0.9, -0.08, -0.6), 0.30, rear, 0.7, 0.035, 0.01,
                       rows=5, steps=4, cap_steps=4)
    eyes(b, head, 0.59, 0.082, 0.018, squash=0.7, sink=0.2)  # same place on the head as before, lifted with it
    b.scale_to_extent_z(1.0)  # shell, head, tail tube, flippers and eyes together: z extent == manifest length
    return b, 1.0


def whale():
    """Humpback whale (Megaptera novaeangliae): long body, huge scalloped pectorals, broad flukes. 12 m."""
    rng = random.Random(8801)
    radius = [(0, 0), (0.008, 0.28), (0.025, 0.50), (0.05, 0.69), (0.1, 0.94), (0.18, 1.14), (0.28, 1.27),
              (0.38, 1.31), (0.5, 1.24), (0.62, 1.03), (0.74, 0.76), (0.86, 0.50), (0.94, 0.34), (1.0, 0.27)]
    deep = curve([(0, 0.78), (0.18, 0.82), (0.38, 0.98), (0.7, 1.05), (1.0, 1.1)])
    wide = curve([(0, 1.0), (0.6, 1.0), (0.85, 0.75), (1.0, 0.58)])
    prof = Profile(5.4, -5.0,
                   hh=[(t, r * deep(t)) for t, r in radius],
                   hw=[(t, r * wide(t)) for t, r in radius],
                   cy=[(t, -0.13 * r) for t, r in radius], power=2.2)
    b = Builder('whale')
    # the tail stock keeps tapering into the root of the flukes (no blunt stump under the thin wedge)
    stations, along = closed_stations(prof, 30, 1.5, taper=((0.3, 0.80), (0.6, 0.52), (0.85, 0.24)))
    b.loft(stations, 36, 'body', along=along)
    median_fin(b, prof, -1.9, -3.0, 0.65, triangular(tip=0.4, concave=1.4), skirt=0.12)
    whale_pectorals(b, rng)
    whale_flukes(b, prof)
    eyes(b, prof, 3.9, -0.3, 0.11, squash=0.7, sink=0.2)
    return b, 12.0


def whale_pectorals(b, rng):
    """Humpback pectorals: broad paddles (3.8 m long, up to 0.9 m chord) as thin wedges, scalloped leading edge
    (tubercles), angled down and out (about 35 degrees below horizontal) and swept back about 25 degrees from the
    lower flank, so most of the 3.8 m shows in profile instead of being foreshortened into a sliver under the
    flank. The sweep matters in the front quarter view: with the tips pointing forward (+z) the far flipper, seen
    nearly edge-on, stuck out beside the snout like a dark needle; swept back it stays behind the head."""
    chord = curve([(0, 0.55), (0.12, 0.78), (0.35, 0.90), (0.6, 0.78), (0.82, 0.52), (0.95, 0.20), (1.0, 0.0)])
    thick = curve([(0, 0.12), (0.5, 0.075), (1.0, 0.0)])
    phases = [rng.uniform(0, TAU) for _ in range(2)]
    amps = [0.065 * (0.85 + 0.3 * rng.random()) for _ in range(2)]
    length, cols = 3.8, 48
    for index, sx in enumerate((1.0, -1.0)):
        root = (sx * 0.9, -0.65, 2.6)
        u = normalize((sx * 0.75, -0.55, -0.35))
        v = normalize(sub((0.0, 0.0, -1.0), scale(u, dot((0.0, 0.0, -1.0), u))))  # chord runs backwards
        n = normalize(cross(u, v))
        if n[1] < 0:
            n = scale(n, -1.0)
        stations = []
        for j in range(cols + 1):
            s = j / cols
            tubercle = amps[index] * (0.5 + 0.5 * sin(TAU * 7.0 * s + phases[index])) * min(1.0, 4.0 * s) * sqrt(1.0 - s)
            point = (root[0] + u[0] * length * s, root[1] + u[1] * length * s, root[2] + u[2] * length * s)
            stations.append((point, v, n, -tubercle, chord(s), thick(s), s, 0.0 if sx > 0 else 1.0))
        b.foil(stations, 'flipper', 0.75, rows=5)


def whale_flukes(b, prof):
    """Flukes as one thin wedge across the tail (span +/-2.1 m, root chord ~1.3 m): thickest (0.14 m) on the
    midline and tapering to the tips, a median notch in the swept trailing edge, and a dihedral that raises the
    tips by 8 degrees, so the side view shows a wedge profile instead of an edge-on hairline."""
    span, tilt = 2.1, radians(8.0)
    lead = curve([(0, 0.0), (0.25, 0.20), (0.5, 0.45), (0.75, 0.85), (1.0, 1.15)])
    trail = curve([(0, 1.0), (0.1, 1.30), (0.25, 1.50), (0.5, 1.65), (0.75, 1.60), (0.9, 1.50), (1.0, 1.30)])
    thick = curve([(0, 0.14), (0.3, 0.115), (0.7, 0.06), (1.0, 0.02)])
    y0, z0, cols = prof.center(prof.z1), prof.z1 + 0.05, 32
    stations = []
    for j in range(cols + 1):
        s = -1.0 + 2.0 * j / cols  # -1 (-X tip) .. +1 (+X tip)
        a = abs(s)
        lean = copysign(sin(tilt), s) if a > 1e-9 else 0.0
        stations.append(((span * s, y0 + span * a * tan(tilt), z0), (0.0, 0.0, -1.0), normalize((-lean, cos(tilt), 0.0)),
                         lead(a), trail(a), thick(a), a, 0.5 - 0.5 * s))
    b.foil(stations, 'fluke', 1.0, rows=6)


SPECIES = [
    ('fish_tang', 'fish-tang.glb', fish_tang),
    ('fish_clown', 'fish-clown.glb', fish_clown),
    ('fish_butterfly', 'fish-butterfly.glb', fish_butterfly),
    ('fish_sardine', 'fish-sardine.glb', fish_sardine),
    ('shark_reef', 'shark-reef.glb', shark_reef),
    ('shark_hammer', 'shark-hammer.glb', shark_hammer),
    ('turtle', 'turtle.glb', turtle),
    ('whale', 'whale.glb', whale),
]


# ---------------------------------------------------------------------------
# export, manifest, previews
# ---------------------------------------------------------------------------

def triangle_count(obj):
    return sum(len(p.vertices) - 2 for p in obj.data.polygons)


def bounds(obj):
    xs = [v.co.x for v in obj.data.vertices]
    ys = [v.co.y for v in obj.data.vertices]
    zs = [v.co.z for v in obj.data.vertices]
    # Report in glTF Y-up axes.
    return {'min': [round(min(xs), 4), round(min(zs), 4), round(-max(ys), 4)],
            'max': [round(max(xs), 4), round(max(zs), 4), round(-min(ys), 4)]}


def export(obj, filename, asset_id, length):
    bpy.ops.object.select_all(action='DESELECT')
    obj.select_set(True)
    bpy.context.view_layer.objects.active = obj
    obj['asset_id'] = asset_id
    output = OUTPUT / filename
    options = {
        'filepath': str(output), 'export_format': 'GLB', 'use_selection': True, 'use_active_scene': True,
        'export_animations': False, 'export_cameras': False, 'export_lights': False, 'export_extras': True,
        'export_texcoords': True, 'export_normals': True, 'export_tangents': False, 'export_yup': True,
        'export_apply': True, 'export_materials': 'EXPORT', 'export_image_format': 'NONE',
        'export_skins': False, 'export_morph': False,
    }
    supported = set(bpy.ops.export_scene.gltf.get_rna_type().properties.keys())
    bpy.ops.export_scene.gltf(**{key: value for key, value in options.items() if key in supported})
    data = output.read_bytes()
    json_length = struct.unpack_from('<I', data, 12)[0]
    document = json.loads(data[20:20 + json_length])
    for index, node in enumerate(document.get('nodes', [])):
        node['name'] = asset_id if index == 0 else f'{asset_id}{index}'
    for index, mesh in enumerate(document.get('meshes', [])):
        mesh['name'] = f'{asset_id}_mesh{index}'
    for scene in document.get('scenes', []):
        scene['name'] = 'OceanLifeAsset'
    document.get('asset', {}).pop('generator', None)
    payload = json.dumps(document, separators=(',', ':'), sort_keys=True).encode()
    payload += b' ' * (-len(payload) % 4)
    tail = data[20 + json_length:]
    data = struct.pack('<III', 0x46546C67, 2, 20 + len(payload) + len(tail))
    data += struct.pack('<II', len(payload), 0x4E4F534A) + payload + tail
    output.write_bytes(data)
    return {'id': asset_id, 'file': filename, 'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest(),
            'triangles': triangle_count(obj), 'length': length, 'bounds': bounds(obj)}


def write_text(path, text):
    with open(path, 'w', encoding='utf-8', newline='\n') as handle:
        handle.write(text)


def layout_for_blend(built, direction=1.0):
    """Lay the animals out in a row in the saved source scene (after the exports); undone for the previews."""
    cursor = 0.0
    for obj, _, length in built:
        half = max(length, 1.0) * 0.75
        cursor += half
        obj.location.x += cursor * direction
        cursor += half
    return cursor


def parse_args():
    argv = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
    options = {'preview': None, 'only': None, 'views': ['side', 'quarter', 'front']}
    index = 0
    while index < len(argv):
        if argv[index] == '--preview' and index + 1 < len(argv):
            options['preview'] = argv[index + 1]
            index += 2
        elif argv[index] == '--only' and index + 1 < len(argv):
            options['only'] = argv[index + 1].split(',')
            index += 2
        elif argv[index] == '--views' and index + 1 < len(argv):
            options['views'] = argv[index + 1].split(',')
            index += 2
        else:
            index += 1
    return options


# camera direction (glTF frame, from the animal's centre) and distance factor times length per view
PREVIEW_VIEWS = {
    'side': ((1.0, 0.0, 0.0), 2.2),
    'quarter': ((1.0, 0.6, 1.1), 1.9),
    'top': ((0.0, 1.0, 0.1), 2.8),
    'front': ((0.0, 0.08, 1.0), 2.4),
}


def render_previews(built, outdir, views):
    os.makedirs(outdir, exist_ok=True)
    scene = bpy.context.scene
    try:
        scene.render.engine = 'BLENDER_EEVEE'
    except TypeError:
        scene.render.engine = 'BLENDER_EEVEE_NEXT'
    scene.render.resolution_x = 768
    scene.render.resolution_y = 512
    scene.render.film_transparent = False
    scene.render.image_settings.file_format = 'PNG'
    try:
        scene.eevee.taa_render_samples = 48
    except AttributeError:
        pass
    try:
        scene.view_settings.view_transform = 'Standard'
    except TypeError:
        pass
    world = bpy.data.worlds.new('OceanPreview')
    world.use_nodes = True
    background = world.node_tree.nodes.get('Background')
    background.inputs[0].default_value = (0.03, 0.09, 0.14, 1.0)
    background.inputs[1].default_value = 1.0
    scene.world = world

    for mat in bpy.data.materials:
        bsdf = mat.node_tree.nodes.get('Principled BSDF')
        color = PREVIEW_COLORS.get(mat.name)
        if bsdf is not None and color is not None:
            bsdf.inputs['Base Color'].default_value = (color[0], color[1], color[2], 1.0)
            bsdf.inputs['Roughness'].default_value = 0.55
            mat.diffuse_color = (color[0], color[1], color[2], 1.0)

    def sun(name, travel, energy, color):
        data = bpy.data.lights.new(name, 'SUN')
        data.energy = energy
        data.color = color
        lamp = bpy.data.objects.new(name, data)
        lamp.rotation_euler = Vector(travel).to_track_quat('-Z', 'Y').to_euler()
        scene.collection.objects.link(lamp)

    sun('Key', (-1.0, 0.5, -1.1), 3.5, (1.0, 0.95, 0.85))
    sun('Fill', (-0.6, -1.0, -0.3), 1.2, (0.6, 0.75, 1.0))
    sun('Rim', (0.6, -1.0, -0.8), 2.5, (0.7, 0.9, 1.0))

    camera_data = bpy.data.cameras.new('PreviewCam')
    camera_data.lens = 50
    camera_data.clip_start = 0.001
    camera_data.clip_end = 1000.0
    camera = bpy.data.objects.new('PreviewCam', camera_data)
    scene.collection.objects.link(camera)
    scene.camera = camera
    constraint = camera.constraints.new('TRACK_TO')
    constraint.track_axis = 'TRACK_NEGATIVE_Z'
    constraint.up_axis = 'UP_Y'
    target = bpy.data.objects.new('PreviewTarget', None)
    scene.collection.objects.link(target)
    constraint.target = target

    bpy.context.view_layer.update()  # matrix_world is stale after layout_for_blend undid the offsets
    for obj, asset_id, length in built:
        for other, _, _ in built:
            other.hide_render = other is not obj
        corners = [obj.matrix_world @ Vector(c) for c in obj.bound_box]
        center = sum(corners, Vector((0.0, 0.0, 0.0))) / len(corners)
        for view in views:
            direction, factor = PREVIEW_VIEWS[view]
            camera.location = center + Vector(to_blender(direction)) * (factor * length)
            target.location = center
            bpy.context.view_layer.update()
            scene.render.filepath = os.path.join(outdir, f'{asset_id}-{view}.png')
            bpy.ops.render.render(write_still=True)
            print('preview', scene.render.filepath)


def main():
    options = parse_args()
    reset_scene()
    OUTPUT.mkdir(parents=True, exist_ok=True)
    PRODUCTION.mkdir(parents=True, exist_ok=True)
    known = [s[0] for s in SPECIES]
    wanted = options['only'] or known
    unknown = [name for name in wanted if name not in known]
    if unknown:
        raise SystemExit(f'unknown animals {unknown}; choose from {known}')
    assets, built = [], []
    for asset_id, filename, factory in SPECIES:
        if asset_id not in wanted:
            continue
        builder, length = factory()
        obj = builder.build()
        assets.append(export(obj, filename, asset_id, length))
        built.append((obj, asset_id, length))
        print('OCEAN_LIFE_SHAPE', asset_id, 'z extent %.3f m, length %.3f m, ratio %.3f'
              % (builder.extent_z(), length, builder.extent_z() / length))

    if options['only'] is None:
        write_text(OUTPUT / 'LICENSE.txt', LICENSE_TEXT)
        source = Path(__file__).read_bytes()
        manifest = {
            'kit': 'aurago-chat-ocean-life',
            'version': 1,
            'license': 'MIT',
            'generator': 'assets/chat-ocean-life/build_ocean_life.py',
            'generator_sha256': hashlib.sha256(source.replace(b'\r\n', b'\n')).hexdigest(),
            'uv0': 'along, part',
            'uv1': 'height, side',
            'assets': assets,
        }
        write_text(OUTPUT / 'manifest.json', json.dumps(manifest, indent=2) + '\n')
        layout_for_blend(built)
        blend = PRODUCTION / 'aurago-ocean-life.blend'
        blend.unlink(missing_ok=True)
        bpy.ops.wm.save_as_mainfile(filepath=str(blend), compress=True)
        layout_for_blend(built, -1.0)
    print('OCEAN_LIFE_EXPORT', json.dumps([{k: a[k] for k in ('file', 'bytes', 'triangles')} for a in assets]))

    if options['preview']:
        render_previews(built, options['preview'], options['views'])


main()
