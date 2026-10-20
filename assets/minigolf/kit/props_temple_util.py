"""Shared helpers of the jungle temple props (MIT License).

Small, deterministic building blocks that only the temple series needs: smooth
steps, seeded noise, a ring-grid surface builder with pole support, outward
face orientation, a Boolean cut and the stone/moss colour functions.
"""

import math

import bmesh
import bpy
from mathutils import Vector, noise

from . import shapes
from .palette import mix


def clamp01(x):
    return min(1.0, max(0.0, x))


def smoothstep(x, a, b):
    """0 below ``a``, 1 above ``b`` (works for a > b too: a falling ramp)."""
    if a == b:
        return 1.0 if x >= a else 0.0
    t = clamp01((x - a) / (b - a))
    return t * t * (3.0 - 2.0 * t)


def nz(x, y, z, off=(0.0, 0.0, 0.0), scale=1.0):
    """Seeded Perlin noise in about [-1, 1] (``mathutils.noise`` is deterministic)."""
    return noise.noise(Vector((x * scale + off[0], y * scale + off[1], z * scale + off[2])))


def shade(c, k):
    return (c[0] * k, c[1] * k, c[2] * k, c[3])


def ring_grid(name, rings, closed=True, cap_first=False, cap_last=False, center=None, smooth=True):
    """Skin rings of 3D points into a surface; rings of equal points collapse to poles.

    ``closed`` wraps the columns. The result is oriented away from ``center`` (when
    given) so callers do not have to reason about the winding.
    """
    verts, idx = [], []
    for ring in rings:
        pts = [Vector(p) for p in ring]
        if max((q - pts[0]).length for q in pts) < 1e-6:
            idx.append([len(verts)] * len(pts))
            verts.append(tuple(pts[0]))
        else:
            idx.append(list(range(len(verts), len(verts) + len(pts))))
            verts.extend(tuple(q) for q in pts)
    cols = len(rings[0])
    span = cols if closed else cols - 1
    faces = []
    for k in range(len(rings) - 1):
        a, b = idx[k], idx[k + 1]
        for j in range(span):
            j2 = (j + 1) % cols
            quad = [a[j], a[j2], b[j2], b[j]]
            dedup = [q for n, q in enumerate(quad) if q != quad[n - 1]]
            if len(dedup) >= 3:
                faces.append(tuple(dedup))
    if cap_first and len(set(idx[0])) > 1:
        faces.append(tuple(reversed(idx[0])))
    if cap_last and len(set(idx[-1])) > 1:
        faces.append(tuple(idx[-1]))
    obj = shapes.make_object(name, verts, faces, smooth=smooth)
    if center is not None:
        orient(obj, center)
    return obj


def orient(obj, center):
    """Flip all faces when they mostly point towards ``center`` (outward is wanted)."""
    c = Vector(center)
    bm = bmesh.new()
    bm.from_mesh(obj.data)
    bm.faces.ensure_lookup_table()
    score = sum((f.normal.dot(f.calc_center_median() - c)) * f.calc_area() for f in bm.faces)
    if score < 0:
        bmesh.ops.reverse_faces(bm, faces=bm.faces)
        bm.to_mesh(obj.data)
    bm.free()
    obj.data.update()
    return obj


def apply_modifiers(obj):
    """Apply the whole modifier stack (evaluated mesh replaces the data)."""
    bpy.context.view_layer.update()
    dg = bpy.context.evaluated_depsgraph_get()
    evaluated = obj.evaluated_get(dg)
    new = bpy.data.meshes.new_from_object(evaluated, preserve_all_data_layers=True, depsgraph=dg)
    old = obj.data
    obj.modifiers.clear()
    obj.data = new
    new.name = old.name
    if old.users == 0:
        bpy.data.meshes.remove(old)
    return obj


def boolean_cut(obj, cutter):
    """Subtract ``cutter`` (exact solver) and delete it afterwards."""
    mod = obj.modifiers.new("kit_cut", "BOOLEAN")
    mod.operation = "DIFFERENCE"
    mod.solver = "EXACT"
    mod.object = cutter
    apply_modifiers(obj)
    obj.data.materials.clear()  # the Boolean leaves an empty slot that would shadow the real material
    bpy.data.objects.remove(cutter, do_unlink=True)
    return obj


def bevel_edges(obj, pick, width, segments=3, min_angle=50.0):
    """Round the sharp edges chosen by ``pick(midpoint, edge)`` (dihedral above ``min_angle`` degrees)."""
    bm = bmesh.new()
    bm.from_mesh(obj.data)
    bm.edges.ensure_lookup_table()
    limit = math.radians(min_angle)
    edges = []
    for e in bm.edges:
        if len(e.link_faces) != 2 or e.calc_face_angle(0.0) < limit:
            continue
        mid = (e.verts[0].co + e.verts[1].co) * 0.5
        if pick(mid, e):
            edges.append(e)
    if edges:
        bmesh.ops.bevel(bm, geom=edges, offset=width, offset_type="OFFSET", segments=segments, profile=0.5, affect="EDGES",
                        clamp_overlap=True)
        bm.to_mesh(obj.data)
    bm.free()
    obj.data.update()
    return obj


def arch_outline(half, spring, steps=14, z0=0.0, rise=None):
    """Closed (x, z) outline of an arched opening from z0: straight sides, rounded head.

    ``rise`` is the height of the head above the springline (default ``half``: a semicircle).
    """
    rise = half if rise is None else rise
    pts = [(-half, z0), (half, z0), (half, spring)]
    for k in range(1, steps):
        a = math.pi * k / steps
        pts.append((half * math.cos(a), spring + rise * math.sin(a)))
    pts.append((-half, spring))
    return pts


def stone_fn(pal, z_lo, z_hi, off=(0.0, 0.0, 0.0), moss=0.0, moss_z=None, noise_amp=0.07, deep=False, strata=0.0, moss_depth=0.9):
    """Colour function for stone: dark bottom to light top, value noise, optional moss on top.

    ``moss`` is the maximum moss mix at ``moss_z`` (default ``z_hi``) and above; the moss
    edge is broken up with noise so it never follows a straight line. ``strata`` adds
    wavy horizontal rock layers (relative brightness swing); ``moss_depth`` is how far (m) the
    moss creeps down from ``moss_z``.
    """
    lo = pal.c("stone_deep" if deep else "stone_dark")
    hi = pal.c("stone_light")
    mid = pal.c("stone")
    m_lo, m_hi = pal.c("moss_dark"), pal.c("moss_light")
    span = max(z_hi - z_lo, 1e-6)
    top = z_hi if moss_z is None else moss_z

    def fn(x, y, z):
        t = clamp01((z - z_lo) / span)
        c = mix(lo, mid, smoothstep(t, 0.0, 0.55))
        c = mix(c, hi, 0.55 * smoothstep(t, 0.4, 1.0))
        n = nz(x, y, z, off, 1.7)
        k = 1.0 + noise_amp * n
        if strata:
            k *= 1.0 + strata * math.sin(z * 9.0 + 2.5 * nz(x, y, 0.0, off, 0.8))
        c = (c[0] * k, c[1] * k, c[2] * k, 1.0)
        if moss:
            edge = top - moss_depth * (0.5 + 0.5 * nz(x, y, z, off, 2.0)) - 0.1 * moss_depth / 0.9
            w = moss * smoothstep(z, edge, edge + min(0.35, 0.6 * moss_depth))
            if w > 0.0:
                c = mix(c, mix(m_lo, m_hi, clamp01(0.5 + 0.8 * nz(x, y, z, off, 3.1))), w)
        return c

    return fn
