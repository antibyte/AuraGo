"""Blender helpers of the mechanic and obstacle parts (MIT License, see ../LICENSE).

Parts are built in a local frame (origin on the floor or at the body origin, +X along the heading, Z up) from the
kit's ``Prop`` helper and then either baked into world coordinates (``to_world``: static scenery, identity object
transform) or given the pose of the moving body (``pose_object``: the node origin is the body origin, the mesh stays
local). ``draped`` builds flat decals (plates, arrows, rings) that follow the floor a few millimetres above it.
"""

import math

import bmesh
from mathutils import Matrix, Quaternion

from . import colors, materials, shapes

C = Matrix(((1.0, 0.0, 0.0), (0.0, 0.0, 1.0), (0.0, -1.0, 0.0)))   # Blender -> runtime axes (x, z, -y)


def finish_object(obj, name, extras=None):
    """Name an object and its mesh, drop the builder's bookkeeping properties and set the given extras (``aurago_*``)."""
    obj.name = name
    obj.data.name = name + "-mesh"
    for key in list(obj.keys()):
        del obj[key]
    for key, value in (extras or {}).items():
        obj[key] = value
    return obj


def to_world(obj, location, heading=0.0):
    """Bake the local->world transform into the vertices (object transform stays the identity)."""
    obj.data.transform(Matrix.Translation(location) @ Matrix.Rotation(heading, 4, "Z"))
    obj.matrix_world = Matrix.Identity(4)
    return obj


def pose_matrix(location, rotation):
    """Blender object matrix of a body at runtime ``location`` (Blender metres) with the runtime quaternion ``[x, y, z, w]``."""
    rt = Quaternion((rotation[3], rotation[0], rotation[1], rotation[2])).to_matrix()
    return Matrix.Translation(location) @ (C.transposed() @ rt @ C).to_4x4()


def pose_object(obj, location, rotation):
    obj.matrix_world = pose_matrix(location, rotation)
    return obj


def add_box(prop, centre, half, grad, mat="matte", rot=(0, 0, 0), bevel=0.02, **opts):
    """A bevelled box of exactly ``2 * half`` (before ``rot``) around ``centre`` added to ``prop``."""
    part = shapes.box("part", (2 * half[0], 2 * half[1], 2 * half[2]))
    return prop.add(part, mat, grad, loc=centre, rot=rot, bevel=bevel, segments=opts.pop("segments", 1), dens=opts.pop("dens", 0.45), **opts)


def add_cylinder(prop, centre, radius, height, grad, mat="matte", axis="z", seg=16, bevel=0.015, top=None, **opts):
    """A cylinder of ``height`` along ``axis`` (z standing, x/y lying), ``top`` the optional top radius."""
    part = shapes.cylinder("part", radius, top, height, seg=seg, base=False)
    rot = {"z": (0, 0, 0), "x": (0, 90, 0), "y": (90, 0, 0)}[axis]
    return prop.add(part, mat, grad, loc=centre, rot=rot, bevel=bevel, segments=opts.pop("segments", 1), dens=opts.pop("dens", 0.45), **opts)


def circle(cx, cy, radius, count=32, start=0.0):
    return [(cx + radius * math.cos(start + math.tau * k / count), cy + radius * math.sin(start + math.tau * k / count)) for k in range(count)]


def ring_polygons(cx, cy, inner, outer, count=32, start=0.0):
    """Quads of a flat ring (counter-clockwise from above)."""
    a, b = circle(cx, cy, inner, count, start), circle(cx, cy, outer, count, start)
    return [[a[k], b[k], b[(k + 1) % count], a[(k + 1) % count]] for k in range(count)]


def draped(name, polygons, ground, lift, paint, mat, ao=0.0, max_edge=0.2, strength=1.0):
    """Flat polygons ``[[(x, y), ...], ...]`` laid on the surface ``ground(x, y)`` plus ``lift`` metres.

    ``paint(x, y, z)`` returns the vertex colour. The polygons are triangulated and split to ``max_edge`` so the surface follows
    slopes; faces point up. The object is linked into the active kit collection (``shapes.set_collection``).
    """
    bm = bmesh.new()
    for polygon in polygons:
        verts = [bm.verts.new((x, y, 0.0)) for x, y in polygon]
        try:
            bm.faces.new(verts)
        except ValueError:
            pass
    bmesh.ops.remove_doubles(bm, verts=bm.verts[:], dist=1e-6)
    bmesh.ops.triangulate(bm, faces=bm.faces[:])
    for _ in range(8):
        long_edges = [e for e in bm.edges if e.calc_length() > max_edge]
        if not long_edges:
            break
        bmesh.ops.subdivide_edges(bm, edges=long_edges, cuts=1, use_grid_fill=True)
    for v in bm.verts:
        v.co.z = ground(v.co.x, v.co.y) + lift
    bm.normal_update()
    for f in bm.faces:
        if f.normal.z < 0:
            f.normal_flip()
    obj = shapes.from_bmesh(name, bm, smooth=False)
    colors.paint_fn(obj, paint, ao=ao)
    obj.data.materials.append(materials.get(mat, strength=strength))
    return obj
