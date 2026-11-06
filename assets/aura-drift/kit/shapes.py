"""Aura Drift kit shapes: pure geometry helpers (MIT, AuraGolf-style code, AuraGo contributors).

A *shape* is ``(verts, faces)``: a list of (x, y, z) tuples and a list of vertex-index tuples
(quads, triangles or n-gons), always a closed outward-facing solid unless noted. Everything is in
the authored glTF frame (Y up, metres). Shapes are combined with ``core.Accum.add``.

Quick map
    primitives   box, box_raw, taper_box, cylinder (Y), cylinder_x, cylinder_z, sphere, ellipsoid,
                 torus, prism (XZ polygon along Y), extrude_z (XY polygon along Z), strut
    revolve      revolve (about Y), revolve_x, revolve_z: (radius, axial) profile -> solid
    sweeps       loft (rings -> solid), arc_band (ring sector), airfoil (wing section)
    hull         ring + Hull: lofted body sections with named face bands for material splits
    helpers      xf (placement matrix), aim, mirror_z, merge, spline, bevel_shape
"""

import math

import bmesh
from mathutils import Euler, Matrix, Vector

TAU = math.tau


# --------------------------------------------------------------------------
# orientation, transforms
# --------------------------------------------------------------------------


def signed_volume(verts, faces):
    total = 0.0
    for face in faces:
        a = verts[face[0]]
        for index in range(1, len(face) - 1):
            b = verts[face[index]]
            c = verts[face[index + 1]]
            total += (
                a[0] * (b[1] * c[2] - b[2] * c[1])
                - a[1] * (b[0] * c[2] - b[2] * c[0])
                + a[2] * (b[0] * c[1] - b[1] * c[0])
            )
    return total / 6.0


def orient_outward(verts, faces):
    """Flip every face when a closed solid is inside out (signed volume < 0)."""
    if signed_volume(verts, faces) < 0.0:
        return [tuple(reversed(face)) for face in faces]
    return faces


def face_normal(verts, face):
    nx = ny = nz = 0.0
    for index, current in enumerate(face):
        a = verts[current]
        b = verts[face[(index + 1) % len(face)]]
        nx += (a[1] - b[1]) * (a[2] + b[2])
        ny += (a[2] - b[2]) * (a[0] + b[0])
        nz += (a[0] - b[0]) * (a[1] + b[1])
    length = math.sqrt(nx * nx + ny * ny + nz * nz) or 1.0
    return (nx / length, ny / length, nz / length)


def xf(pos=(0.0, 0.0, 0.0), rot=None, scale=None):
    """Translation @ rotation (degrees, XYZ euler) @ scale, as a 4x4 matrix."""
    matrix = Matrix.Translation(Vector(pos))
    if rot is not None:
        matrix = matrix @ Euler(tuple(math.radians(a) for a in rot), "XYZ").to_matrix().to_4x4()
    if scale is not None:
        if isinstance(scale, (int, float)):
            scale = (scale, scale, scale)
        matrix = matrix @ Matrix.Diagonal((scale[0], scale[1], scale[2], 1.0))
    return matrix


def aim(origin, target, roll=0.0):
    """Matrix placing a +Y-axis primitive at ``origin`` pointing towards ``target``."""
    direction = Vector(target) - Vector(origin)
    quaternion = Vector((0.0, 1.0, 0.0)).rotation_difference(direction.normalized())
    matrix = Matrix.Translation(Vector(origin)) @ quaternion.to_matrix().to_4x4()
    if roll:
        matrix = matrix @ Matrix.Rotation(math.radians(roll), 4, "Y")
    return matrix


def transform(shape, matrix):
    verts, faces = shape
    out = [tuple(matrix @ Vector(v)) for v in verts]
    if matrix.determinant() < 0.0:
        faces = [tuple(reversed(f)) for f in faces]
    return out, faces


def mirror_z(shape):
    """Mirror across the car's centre plane (z -> -z); faces keep pointing outward."""
    verts, faces = shape
    return [(x, y, -z) for x, y, z in verts], [tuple(reversed(f)) for f in faces]


def merge(shapes):
    """Concatenate shapes into one (vertex lists are offset)."""
    verts, faces = [], []
    for sv, sf in shapes:
        base = len(verts)
        verts.extend(sv)
        faces.extend(tuple(i + base for i in f) for f in sf)
    return verts, faces


def spline(keys):
    """Smooth monotone-safe curve through (x, value) keys; returns f(x), clamped at the ends."""
    keys = sorted(keys)
    xs = [k[0] for k in keys]
    ys = [k[1] for k in keys]
    n = len(keys)
    slopes = [(ys[i + 1] - ys[i]) / (xs[i + 1] - xs[i]) for i in range(n - 1)]
    tangents = [slopes[0]] + [0.0] * (n - 2) + [slopes[-1]]
    for i in range(1, n - 1):
        a, b = slopes[i - 1], slopes[i]
        tangents[i] = 0.0 if a * b <= 0.0 else 2.0 * a * b / (a + b)

    def f(x):
        if x <= xs[0]:
            return ys[0]
        if x >= xs[-1]:
            return ys[-1]
        i = 0
        while xs[i + 1] < x:
            i += 1
        h = xs[i + 1] - xs[i]
        t = (x - xs[i]) / h
        h00 = 2 * t**3 - 3 * t**2 + 1
        h10 = t**3 - 2 * t**2 + t
        h01 = -2 * t**3 + 3 * t**2
        h11 = t**3 - t**2
        return h00 * ys[i] + h10 * h * tangents[i] + h01 * ys[i + 1] + h11 * h * tangents[i + 1]

    return f


# --------------------------------------------------------------------------
# revolve family
# --------------------------------------------------------------------------


def _dedupe(ids):
    out = []
    for value in ids:
        if not out or out[-1] != value:
            out.append(value)
    if len(out) > 1 and out[0] == out[-1]:
        out.pop()
    return out


def revolve(points, seg=24, closed=False, sweep=360.0, start=0.0):
    """Revolve a (radius, y) profile about the Y axis.

    Points on the axis (radius ~ 0) collapse into one pole vertex. ``closed`` treats the profile as
    a loop (tyre/torus); a partial ``sweep`` gets planar end caps.
    """
    count = len(points)
    full = abs(sweep - 360.0) < 1e-6
    steps = seg if full else seg + 1
    verts, table = [], []
    for radius, y in points:
        if radius <= 1e-9:
            table.append([len(verts)] * steps)
            verts.append((0.0, y, 0.0))
        else:
            ids = []
            for k in range(steps):
                angle = math.radians(start + sweep * k / seg)
                ids.append(len(verts))
                verts.append((radius * math.cos(angle), y, radius * math.sin(angle)))
            table.append(ids)
    faces = []
    for i in range(count if closed else count - 1):
        j = (i + 1) % count
        for k in range(seg):
            k2 = (k + 1) % steps if full else k + 1
            a, b, c, d = table[i][k], table[i][k2], table[j][k2], table[j][k]
            if a == b and c == d:
                continue
            if a == b:
                faces.append((a, c, d))
            elif c == d:
                faces.append((a, b, c))
            else:
                faces.append((a, b, c, d))
    if not full:
        faces.append(tuple(_dedupe([table[i][0] for i in range(count)])))
        faces.append(tuple(_dedupe([table[i][seg] for i in range(count - 1, -1, -1)])))
    return verts, orient_outward(verts, faces)


def revolve_z(points, seg=24, closed=False):
    """Revolve (radius, z) about the Z axis (wheels: axle along Z, +Z is the outer face)."""
    return transform(revolve(points, seg, closed), xf(rot=(90.0, 0.0, 0.0)))


def revolve_x(points, seg=24, closed=False):
    """Revolve (radius, x) about the X axis (exhaust pipes, noses: axial coordinate is x)."""
    return transform(revolve(points, seg, closed), xf(rot=(0.0, 0.0, -90.0)))


def annulus_z(r0, r1, z, seg=24):
    """Flat single-sided ring in the XY plane at height z, facing +Z (decals, stripes, hub rings)."""
    verts = []
    for r in (r0, r1):
        verts += [(r * math.cos(TAU * k / seg), r * math.sin(TAU * k / seg), z) for k in range(seg)]
    faces = [(k, seg + k, seg + (k + 1) % seg, (k + 1) % seg) for k in range(seg)]
    return verts, faces


def circle_profile(center_r, center_y, radius, count=12, squash=1.0):
    return [
        (center_r + radius * math.cos(TAU * k / count), center_y + radius * squash * math.sin(TAU * k / count))
        for k in range(count)
    ]


def torus(major, minor, seg=24, minor_seg=8, y=0.0):
    return revolve(circle_profile(major, y, minor, minor_seg), seg, closed=True)


def cylinder(r0, r1=None, height=1.0, seg=16, chamfer=0.0):
    """Y-axis frustum centred on the origin, optional chamfered rims."""
    r1 = r0 if r1 is None else r1
    h = height / 2.0
    c = min(chamfer, r0 * 0.9, r1 * 0.9, h * 0.9)
    pts = [(0.0, -h)]
    if c > 0.0:
        pts += [(r0 - c, -h), (r0, -h + c), (r1, h - c), (r1 - c, h)]
    else:
        pts += [(r0, -h), (r1, h)]
    pts.append((0.0, h))
    return revolve(pts, seg)


def cylinder_x(r0, r1=None, length=1.0, seg=12, chamfer=0.0):
    """Frustum along X, r0 at -x/2 and r1 at +x/2, centred on the origin."""
    return transform(cylinder(r0, r1, length, seg, chamfer), xf(rot=(0.0, 0.0, -90.0)))


def cylinder_z(r0, r1=None, length=1.0, seg=12, chamfer=0.0):
    """Frustum along Z, r0 at -z/2 and r1 at +z/2, centred on the origin."""
    return transform(cylinder(r0, r1, length, seg, chamfer), xf(rot=(90.0, 0.0, 0.0)))


def sphere(radius, seg=12, rings=6):
    pts = [(radius * math.sin(math.pi * k / rings), -radius * math.cos(math.pi * k / rings)) for k in range(rings + 1)]
    pts[0] = (0.0, -radius)
    pts[-1] = (0.0, radius)
    return revolve(pts, seg)


def ellipsoid(rx, ry, rz, seg=12, rings=6):
    return transform(sphere(1.0, seg, rings), xf(scale=(rx, ry, rz)))


# --------------------------------------------------------------------------
# boxes, bevels
# --------------------------------------------------------------------------


def box_raw(sx, sy, sz):
    hx, hy, hz = sx / 2.0, sy / 2.0, sz / 2.0
    verts = [
        (-hx, -hy, -hz),
        (hx, -hy, -hz),
        (hx, hy, -hz),
        (-hx, hy, -hz),
        (-hx, -hy, hz),
        (hx, -hy, hz),
        (hx, hy, hz),
        (-hx, hy, hz),
    ]
    faces = [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (2, 3, 7, 6), (1, 2, 6, 5), (0, 4, 7, 3)]
    return verts, orient_outward(verts, faces)


def bevel_shape(shape, offset, segments=2, min_angle=None):
    """Bevel the edges of a closed solid with bmesh (all edges, or those sharper than min_angle)."""
    verts, faces = shape
    bm = bmesh.new()
    made = [bm.verts.new(v) for v in verts]
    for face in faces:
        try:
            bm.faces.new([made[i] for i in face])
        except ValueError:
            pass
    bm.verts.index_update()
    edges = []
    for edge in bm.edges:
        if min_angle is None:
            edges.append(edge)
        elif len(edge.link_faces) == 2 and edge.calc_face_angle(0.0) >= min_angle:
            edges.append(edge)
    bmesh.ops.bevel(
        bm,
        geom=edges,
        offset=offset,
        offset_type="OFFSET",
        profile_type="SUPERELLIPSE",
        segments=segments,
        profile=0.5,
        affect="EDGES",
        clamp_overlap=True,
        loop_slide=True,
    )
    bm.verts.index_update()
    out_verts = [tuple(v.co) for v in bm.verts]
    out_faces = [tuple(v.index for v in f.verts) for f in bm.faces]
    bm.free()
    return out_verts, orient_outward(out_verts, out_faces)


def box(sx, sy, sz, bevel=0.0, segs=2):
    """Centred box; ``bevel`` rounds every edge (segs=1 is a chamfer, 2 a chunky round)."""
    shape = box_raw(sx, sy, sz)
    if bevel > 0.0:
        return bevel_shape(shape, min(bevel, min(sx, sy, sz) * 0.45), segs)
    return shape


def taper_box(length, w0, h0, w1, h1, dy=0.0, bevel=0.0, segs=2):
    """Box along X from -length/2 (w0 x h0) to +length/2 (w1 x h1); the front end shifts up by dy."""
    rings = []
    for x, w, h, y in ((-length / 2.0, w0, h0, 0.0), (length / 2.0, w1, h1, dy)):
        rings.append([(x, y - h / 2, -w / 2), (x, y - h / 2, w / 2), (x, y + h / 2, w / 2), (x, y + h / 2, -w / 2)])
    shape = loft(rings)
    if bevel > 0.0:
        return bevel_shape(shape, min(bevel, min(w0, w1, h0, h1, length) * 0.45), segs)
    return shape


def prism(polygon, y0, y1):
    """Extrude a planar polygon given as (x, z) points between two heights."""
    n = len(polygon)
    verts = [(x, y0, z) for x, z in polygon] + [(x, y1, z) for x, z in polygon]
    faces = [tuple(range(n - 1, -1, -1)), tuple(range(n, 2 * n))]
    for i in range(n):
        j = (i + 1) % n
        faces.append((i, j, n + j, n + i))
    return verts, orient_outward(verts, faces)


def extrude_z(polygon, z0, z1, bevel=0.0, segs=1):
    """Extrude a side-profile polygon given as (x, y) points between two z values (any winding)."""
    n = len(polygon)
    verts = [(x, y, z0) for x, y in polygon] + [(x, y, z1) for x, y in polygon]
    faces = [tuple(range(n - 1, -1, -1)), tuple(range(n, 2 * n))]
    for i in range(n):
        j = (i + 1) % n
        faces.append((i, j, n + j, n + i))
    shape = verts, orient_outward(verts, faces)
    return bevel_shape(shape, bevel, segs) if bevel > 0.0 else shape


def strut(p, q, radius, sides=6, radius2=None):
    """Capped prism/cone between two points."""
    p, q = Vector(p), Vector(q)
    d = (q - p).normalized()
    helper = Vector((1.0, 0.0, 0.0)) if abs(d.x) < 0.9 else Vector((0.0, 0.0, 1.0))
    u = d.cross(helper).normalized()
    v = d.cross(u)
    r2 = radius if radius2 is None else radius2
    verts = []
    for centre, rad in ((p, radius), (q, r2)):
        for k in range(sides):
            angle = TAU * k / sides
            verts.append(tuple(centre + (u * math.cos(angle) + v * math.sin(angle)) * rad))
    faces = [tuple(range(sides - 1, -1, -1)), tuple(range(sides, 2 * sides))]
    for k in range(sides):
        j = (k + 1) % sides
        faces.append((k, j, sides + j, sides + k))
    return verts, orient_outward(verts, faces)


def loft(rings, cap_start=True, cap_end=True):
    """Skin a list of equally sized vertex rings (3D points) into a solid.

    Face ``i * n + k`` is the quad between rings i and i + 1 at ring position k.
    """
    n = len(rings[0])
    verts = [tuple(p) for ring in rings for p in ring]
    faces = []
    for i in range(len(rings) - 1):
        for k in range(n):
            j = (k + 1) % n
            faces.append((i * n + k, i * n + j, (i + 1) * n + j, (i + 1) * n + k))
    if cap_start:
        faces.append(tuple(range(n - 1, -1, -1)))
    if cap_end:
        base = (len(rings) - 1) * n
        faces.append(tuple(range(base, base + n)))
    return verts, orient_outward(verts, faces)


def arc_band(r_in, r_out, a0, a1, z0, z1, seg=8, bevel=0.0):
    """Ring sector in the XY plane around the origin, from angle a0 to a1 (degrees, 90 = up)."""
    outer = [
        (r_out * math.cos(math.radians(a0 + (a1 - a0) * k / seg)), r_out * math.sin(math.radians(a0 + (a1 - a0) * k / seg)))
        for k in range(seg + 1)
    ]
    inner = [
        (r_in * math.cos(math.radians(a1 + (a0 - a1) * k / seg)), r_in * math.sin(math.radians(a1 + (a0 - a1) * k / seg)))
        for k in range(seg + 1)
    ]
    return extrude_z(outer + inner, z0, z1, bevel)


def airfoil(chord, thick, span, bevel=0.0):
    """Wing section extruded along Z (leading edge +X), centred; tilt it with xf(rot=...)."""
    c, t = chord, thick
    poly = [
        (0.50 * c, 0.00),
        (0.42 * c, 0.46 * t),
        (0.15 * c, 0.50 * t),
        (-0.20 * c, 0.30 * t),
        (-0.50 * c, 0.10 * t),
        (-0.50 * c, -0.10 * t),
        (-0.22 * c, -0.22 * t),
        (0.12 * c, -0.46 * t),
        (0.42 * c, -0.44 * t),
    ]
    return extrude_z(poly, -span / 2.0, span / 2.0, bevel)


# --------------------------------------------------------------------------
# lofted hull sections
# --------------------------------------------------------------------------


def ring(x, hw, yb, yt, power=3.0, m=6, top_flat=0.0, bot_flat=0.0, zc=0.0):
    """One cross-section in the YZ plane at station ``x`` (a rounded-rectangle superellipse).

    ``m`` vertices per side, 2m in total, ordered right-top -> right-bottom -> left-bottom ->
    left-top. ``top_flat`` / ``bot_flat`` are half widths of the flat top and bottom panels, so a
    centre stripe can be a whole face band. ``power`` 2 is an ellipse, 4+ a rounded box.
    """
    ym = (yb + yt) / 2.0
    hh = (yt - yb) / 2.0
    e = 2.0 / power
    right = []
    for i in range(m):
        a = math.pi / 2.0 - math.pi * i / (m - 1)
        c, s = math.cos(a), math.sin(a)
        cz = abs(c) ** e
        cy = math.copysign(abs(s) ** e, s)
        flat = bot_flat + (top_flat - bot_flat) * (cy + 1.0) / 2.0
        right.append((ym + hh * cy, flat + cz * (hw - flat)))
    pts = [(x, y, zc + z) for y, z in right]
    pts += [(x, y, zc - z) for y, z in reversed(right)]
    return pts


class Hull:
    """A body lofted through ``stations`` (dicts for ``ring``: x, hw, yb, yt, power, top_flat, ...).

    ``band(ks, i0, i1)`` returns the faces of ring positions ``ks`` between stations i0..i1, so a
    top stripe, flank stripe or underbody can be given another material:
    band positions (m = 6): TOP = 2m-1 flat top panel, 0 and 1 shoulder, 2 flank panel, 3 and 4
    lower flank, BOTTOM = m-1 flat underside.
    """

    def __init__(self, stations, m=6):
        self.m = m
        self.n = 2 * m
        self.stations = stations
        rings = []
        for s in stations:
            rings.append(
                ring(
                    s["x"],
                    s["hw"],
                    s["yb"],
                    s["yt"],
                    s.get("power", 3.0),
                    m,
                    s.get("top_flat", 0.0),
                    s.get("bot_flat", 0.0),
                    s.get("zc", 0.0),
                )
            )
        self.shape = loft(rings)

    def point(self, i, k):
        """Vertex k of the ring at station i (a point on the surface, for pillars and trim struts)."""
        return self.shape[0][i * self.n + k]

    TOP = property(lambda self: self.n - 1)
    BOTTOM = property(lambda self: self.m - 1)

    def span(self, x_min, x_max):
        """Station pair range (i0, i1) whose both stations lie within [x_min, x_max]."""
        idx = [i for i, s in enumerate(self.stations) if x_min - 1e-6 <= s["x"] <= x_max + 1e-6]
        return (idx[0], idx[-1]) if len(idx) > 1 else (0, 0)

    def band(self, ks, i0=0, i1=None):
        i1 = len(self.stations) - 1 if i1 is None else i1
        return [i * self.n + k for i in range(i0, i1) for k in ks]

    def pick(self, indices):
        return [self.shape[1][i] for i in indices]

    def rest(self, *picked):
        """Every face not in any of the given index lists (caps included)."""
        skip = set()
        for group in picked:
            skip.update(group)
        return [f for i, f in enumerate(self.shape[1]) if i not in skip]
