"""Diorama island chunk and surrounding water (MIT License).

The island is ONE mesh: a dense top grid (square mapped onto a rounded,
noise-wobbled superellipse outline, so vertex-baked occlusion has resolution)
continued by a layered side wall (strata with small ledges, bulges and
noise) that drops through the waterline. The water is a set of concentric
rings around the waterline with a foam / shallows / deep colour ramp.
"""

import math
import random

from mathutils import Vector, noise

from . import colors, shapes
from .palette import mix


def smoothstep(x, a, b):
    if b == a:
        return 1.0 if x >= b else 0.0
    t = min(1.0, max(0.0, (x - a) / (b - a)))
    return t * t * (3 - 2 * t)


class Outline:
    """Wobbled superellipse around ``center`` with half sizes ``half``."""

    def __init__(self, center, half, power=3.0, wobble=0.05, seed=1):
        self.center = Vector((center[0], center[1], 0.0))
        self.half = half
        self.power = power
        self.wobble = wobble
        self.rng = random.Random(seed)
        self.off = Vector((self.rng.uniform(-30, 30), self.rng.uniform(-30, 30), 0.0))

    def ellipse(self, theta):
        """Radius of the plain superellipse (no wobble) along ``theta``."""
        c, s = math.cos(theta), math.sin(theta)
        a, b, n = self.half[0], self.half[1], self.power
        return (abs(c / a) ** n + abs(s / b) ** n) ** (-1.0 / n)

    def radius(self, theta):
        c, s = math.cos(theta), math.sin(theta)
        r = self.ellipse(theta)
        w = 1.0 + self.wobble * noise.noise(Vector((c * 1.3, s * 1.3, 0.0)) + self.off)
        w += self.wobble * 0.5 * noise.noise(Vector((c * 3.1, s * 3.1, 0.0)) + self.off * 1.7)
        return r * w

    def deviation(self, samples=720):
        """Largest relative difference between the wobbled radius and the plain superellipse (sampled)."""
        worst = 0.0
        for k in range(samples):
            theta = math.tau * k / samples
            worst = max(worst, abs(self.radius(theta) / self.ellipse(theta) - 1.0))
        return worst

    def point(self, u, v):
        """Map grid coordinates in [-1, 1]^2 onto the outline (boundary at |.|=1)."""
        dx = u * math.sqrt(max(0.0, 1.0 - v * v / 2.0))
        dy = v * math.sqrt(max(0.0, 1.0 - u * u / 2.0))
        rr = math.hypot(dx, dy)
        if rr < 1e-9:
            return self.center.x, self.center.y, 1.0
        theta = math.atan2(dy, dx)
        r = self.radius(theta)
        return self.center.x + dx * r, self.center.y + dy * r, rr

    def contains_signed(self, x, y):
        """Approximate signed distance to the outline (negative inside)."""
        dx, dy = x - self.center.x, y - self.center.y
        theta = math.atan2(dy, dx)
        return math.hypot(dx, dy) - self.radius(theta)


class Layer:
    """One stratum of the side wall.

    ``z`` is the absolute bottom height; ``top`` / ``bottom`` the colours of the
    ramp inside the layer; ``off`` the outward offset at its bottom edge,
    ``bulge`` an extra roundness in the middle, ``noise`` bump amplitude,
    ``speck`` an optional ``(colour, threshold)`` for pebble / ore flecks,
    ``ledge`` how far the layer's top edge protrudes past the layer above and
    ``blocks`` an optional ``(length, seam_colour, phase)`` that carves a course
    of bulging stone blocks with painted seams into the wall.
    """

    def __init__(self, z, top, bottom, off=0.0, bulge=0.03, noise_amp=0.02, speck=None, scale=1.0,
                 steps=3, ledge=0.0, blocks=None):
        self.z, self.top, self.bottom = z, top, bottom
        self.ledge = ledge
        self.blocks = blocks
        self.off, self.bulge, self.noise_amp = off, bulge, noise_amp
        self.speck, self.scale, self.steps = speck, scale, steps


def build_island(outline, nx, ny, height_fn, color_fn, layers, water_z, seed=1,
                 tip=None, top_ao=0.7, wall_ao=0.7, clamp_down=False, warp=None):
    """Build the island mesh. Returns ``(obj, waterline)``.

    ``height_fn(x, y, edge_m)`` gives the top height (``edge_m`` is the rough
    distance to the outline in metres); ``color_fn(x, y, z, edge_m)`` its
    colour. ``waterline`` is the list of (x, y) points of the wall at
    ``water_z`` for the water rings. ``tip`` (z) closes the bottom into a point
    for floating islands. ``clamp_down`` keeps every wall ring at or below the
    ring above it, so a trench that cuts the rim below a stratum bottom does not
    fold the wall upwards (the probes leave it off). ``warp(x, y) -> (x, y)`` moves a top grid vertex before its
    height is read (the terrain snaps the vertices next to a trench onto the trench edge, so the walls come out
    straight).
    """
    rng = random.Random(seed)
    verts, cols, aow = [], [], []
    ring_index = {}
    min_half = min(outline.half)

    def vid(i, j):
        return j * (nx + 1) + i

    for j in range(ny + 1):
        for i in range(nx + 1):
            u, v = -1 + 2 * i / nx, -1 + 2 * j / ny
            x, y, rr = outline.point(u, v)
            if warp is not None:
                x, y = warp(x, y)
            edge = (1.0 - rr) * min_half
            z = height_fn(x, y, edge)
            verts.append((x, y, z))
            cols.append(color_fn(x, y, z, edge))
            aow.append(top_ao)
    faces = []
    for j in range(ny):
        for i in range(nx):
            faces.append((vid(i, j), vid(i + 1, j), vid(i + 1, j + 1), vid(i, j + 1)))
    # perimeter, counter-clockwise
    per = [vid(i, 0) for i in range(nx + 1)] + [vid(nx, j) for j in range(1, ny + 1)] + \
          [vid(i, ny) for i in range(nx - 1, -1, -1)] + [vid(0, j) for j in range(ny - 1, 0, -1)]
    m = len(per)
    base = [(verts[k][0], verts[k][1], verts[k][2]) for k in per]
    normals = []
    for k in range(m):
        a, b = base[(k - 1) % m], base[(k + 1) % m]
        n = Vector((b[1] - a[1], -(b[0] - a[0]), 0.0))
        normals.append(n.normalized() if n.length > 1e-9 else Vector((1, 0, 0)))

    arc = [0.0]
    for k in range(1, m):
        arc.append(arc[-1] + math.hypot(base[k][0] - base[k - 1][0], base[k][1] - base[k - 1][1]))
    prev_ids = per
    prev_z = [p[2] for p in base]
    prev_off = [0.0] * m
    prev_scale = 1.0
    center = outline.center
    offs_seed = Vector((rng.uniform(-30, 30), rng.uniform(-30, 30), rng.uniform(-30, 30)))

    def make_ring(zs, offs, scale, colour_fn, ao_w):
        ids = []
        for k in range(m):
            p = Vector((base[k][0], base[k][1], 0.0)) + normals[k] * offs[k]
            p = center + (p - center) * scale
            ids.append(len(verts))
            verts.append((p.x, p.y, zs[k]))
            cols.append(colour_fn(k, p.x, p.y, zs[k]))
            aow.append(ao_w)
        return ids

    def stitch(upper, lower):
        for k in range(m):
            k2 = (k + 1) % m
            faces.append((lower[k], lower[k2], upper[k2], upper[k]))

    for layer in layers:
        if layer.ledge:
            # A same-height ring pushed outwards forms a small lit ledge.
            offs = [o + layer.ledge for o in prev_off]
            ids = make_ring(prev_z, offs, prev_scale, lambda k, x, y, z, layer=layer: layer.top, wall_ao)
            stitch(prev_ids, ids)
            prev_ids, prev_off = ids, offs
        for step in range(1, layer.steps + 1):
            t = step / layer.steps
            zs, offs = [], []
            for k in range(m):
                zt = prev_z[k]
                zs.append(zt + ((min(layer.z, zt) if clamp_down else layer.z) - zt) * t)
                bump = layer.noise_amp * noise.noise(
                    Vector((base[k][0] * 2.4, base[k][1] * 2.4, zs[-1] * 2.0)) + offs_seed)
                block_bump = 0.0
                if layer.blocks:
                    ph = ((arc[k] / layer.blocks[0]) + layer.blocks[2]) % 1.0
                    block_bump = 0.035 * math.sin(math.pi * ph) ** 0.45 * math.sin(math.pi * (0.1 + 0.8 * t))
                offs.append(prev_off[k] + (layer.off - prev_off[k]) * t + layer.bulge * math.sin(math.pi * t) + bump
                            + block_bump)
            scale = prev_scale + (layer.scale - prev_scale) * t

            def colour_fn(k, x, y, z, layer=layer, t=t):
                c = mix(layer.top, layer.bottom, t)
                n1 = noise.noise(Vector((x * 1.7, y * 1.7, z * 3.1)) + offs_seed)
                c = (c[0] * (1 + 0.10 * n1), c[1] * (1 + 0.10 * n1), c[2] * (1 + 0.10 * n1), 1.0)
                if layer.blocks:
                    ph = ((arc[k] / layer.blocks[0]) + layer.blocks[2]) % 1.0
                    seam = 1.0 - smoothstep(min(ph, 1.0 - ph) * layer.blocks[0], 0.0, 0.09)
                    c = mix(c, layer.blocks[1], 0.8 * seam)
                if layer.speck is not None:
                    sc, thr = layer.speck
                    n2 = noise.noise(Vector((x * 5.3, y * 5.3, z * 6.0)) + offs_seed * 2.0)
                    if n2 > thr:
                        c = mix(c, sc, min(1.0, (n2 - thr) * 6.0))
                return c

            ids = make_ring(zs, offs, scale, colour_fn, wall_ao)
            stitch(prev_ids, ids)
            prev_ids, prev_off, prev_scale = ids, offs, scale
            if step == layer.steps:
                prev_z = zs
                if abs(layer.z - water_z) < 1e-6:
                    ring_index["water"] = ids
        if not clamp_down:
            prev_z = [layer.z] * m
    if tip is not None:
        apex = len(verts)
        verts.append((center.x, center.y, tip))
        cols.append(layers[-1].bottom)
        aow.append(wall_ao)
        for k in range(m):
            k2 = (k + 1) % m
            faces.append((apex, prev_ids[k2], prev_ids[k]))
    obj = shapes.make_object("island", verts, faces)
    colors.paint_values(obj, cols, aow)
    wl = ring_index.get("water")
    waterline = [(verts[i][0], verts[i][1]) for i in wl] if wl else []
    return obj, waterline


def build_water(waterline, center, water_z, color_fn, offsets, seed=1):
    """Concentric water rings around ``waterline`` (list of (x, y), CCW)."""
    m = len(waterline)
    base = [Vector((x, y, 0.0)) for x, y in waterline]
    normals = []
    for k in range(m):
        a, b = base[(k - 1) % m], base[(k + 1) % m]
        n = Vector((b.y - a.y, -(b.x - a.x), 0.0))
        normals.append(n.normalized())
    verts, cols, faces = [], [], []
    prev = None
    for ri, off in enumerate(offsets):
        ids = []
        w = smoothstep(off, 0.5, 3.0)
        for k in range(m):
            p_n = base[k] + normals[k] * (off - 0.03)
            radial = base[k] - Vector((center[0], center[1], 0.0))
            p_r = base[k] + radial.normalized() * (off - 0.03)
            p = p_n + (p_r - p_n) * w
            ids.append(len(verts))
            verts.append((p.x, p.y, water_z))
            cols.append(color_fn(p.x, p.y, off))
        if prev is not None:
            for k in range(m):
                k2 = (k + 1) % m
                faces.append((ids[k], ids[k2], prev[k2], prev[k]))
        prev = ids
    obj = shapes.make_object("water", verts, faces)
    colors.paint_values(obj, cols, ao=0.0)
    return obj
