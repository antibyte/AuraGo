"""Soft stylized mesh builders and the ``finish`` helper (MIT License).

Conventions: Blender Z-up, metres. Builders return an unfinished mesh object
linked into the active collection (see ``set_collection``); call ``finish`` to
bevel, subdivide, add shading normals and APPLY the modifier stack so the
final result is plain geometry. All randomness is seeded by the caller.
"""

import math

import bmesh
import bpy
from mathutils import Euler, Matrix, Vector, noise

_state = {"collection": None}


def set_collection(collection):
    """Link subsequently built objects into ``collection`` (None = scene root)."""
    _state["collection"] = collection


def link(obj):
    coll = _state["collection"] or bpy.context.scene.collection
    coll.objects.link(obj)
    return obj


def make_object(name, verts, faces, smooth=True):
    """Create a mesh object from raw vertex and face lists."""
    mesh = bpy.data.meshes.new(name + "-mesh")
    mesh.from_pydata([tuple(v) for v in verts], [], [tuple(f) for f in faces])
    mesh.update()
    if smooth:
        mesh.shade_smooth()
    obj = bpy.data.objects.new(name, mesh)
    link(obj)
    return obj


def from_bmesh(name, bm, smooth=True):
    mesh = bpy.data.meshes.new(name + "-mesh")
    bm.to_mesh(mesh)
    bm.free()
    if smooth:
        mesh.shade_smooth()
    obj = bpy.data.objects.new(name, mesh)
    link(obj)
    return obj


# --------------------------------------------------------------------------
# Primitive builders
# --------------------------------------------------------------------------

_CUBE_FACES = [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)]


def box(name, size, base=False):
    """Axis-aligned box. ``base=True`` puts the bottom face at z=0."""
    sx, sy, sz = (s * 0.5 for s in size)
    z0 = sz if base else 0.0
    verts = [(x * sx, y * sy, z * sz + z0) for x, y, z in
             ((-1, -1, -1), (1, -1, -1), (1, 1, -1), (-1, 1, -1),
              (-1, -1, 1), (1, -1, 1), (1, 1, 1), (-1, 1, 1))]
    return make_object(name, verts, _CUBE_FACES, smooth=False)


def lathe(name, profile, seg=24, caps=True, start_angle=0.0, sweep=math.tau):
    """Revolve a ``[(radius, z), ...]`` profile (bottom to top) around Z.

    Points with radius ~0 become single pole vertices. ``sweep < tau`` makes
    an open wedge (no cap faces on the cut).
    """
    full = abs(sweep - math.tau) < 1e-6
    steps = seg if full else seg + 1
    verts, rings = [], []
    for r, z in profile:
        if r <= 1e-6:
            rings.append([len(verts)] * steps)
            verts.append((0.0, 0.0, z))
        else:
            ring = []
            for i in range(steps):
                a = start_angle + sweep * i / seg
                ring.append(len(verts))
                verts.append((r * math.cos(a), r * math.sin(a), z))
            rings.append(ring)
    faces = []
    for k in range(len(profile) - 1):
        a, b = rings[k], rings[k + 1]
        for i in range(seg):
            j = (i + 1) % steps if full else i + 1
            quad = [a[i], a[j], b[j], b[i]]
            dedup = [q for n, q in enumerate(quad) if q != quad[n - 1]]
            if len(dedup) >= 3:
                faces.append(tuple(dedup))
    if caps and full:
        if profile[0][0] > 1e-6:
            faces.append(tuple(reversed(rings[0])))
        if profile[-1][0] > 1e-6:
            faces.append(tuple(rings[-1]))
    return make_object(name, verts, faces)


def cylinder(name, r0, r1=None, h=1.0, seg=24, base=True, caps=True):
    """Cylinder or truncated cone from radius ``r0`` (bottom) to ``r1`` (top)."""
    r1 = r0 if r1 is None else r1
    z0 = 0.0 if base else -h / 2
    return lathe(name, [(r0, z0), (r1, z0 + h)], seg=seg, caps=caps)


def cone(name, r, h, seg=24, base=True):
    z0 = 0.0 if base else -h / 2
    return lathe(name, [(r, z0), (0.0, z0 + h)], seg=seg)


def sphere(name, r, seg=24, rings=12):
    prof = [(r * math.sin(math.pi * k / rings), -r * math.cos(math.pi * k / rings))
            for k in range(rings + 1)]
    prof[0] = (0.0, -r)
    prof[-1] = (0.0, r)
    return lathe(name, prof, seg=seg)


def ico_sphere(name, r, subdiv=2):
    bm = bmesh.new()
    bmesh.ops.create_icosphere(bm, subdivisions=subdiv, radius=r)
    return from_bmesh(name, bm)


def torus(name, major, minor, seg=32, rseg=12):
    verts, faces = [], []
    for i in range(seg):
        a = math.tau * i / seg
        for j in range(rseg):
            b = math.tau * j / rseg
            rr = major + minor * math.cos(b)
            verts.append((rr * math.cos(a), rr * math.sin(a), minor * math.sin(b)))
    for i in range(seg):
        for j in range(rseg):
            i2, j2 = (i + 1) % seg, (j + 1) % rseg
            faces.append((i * rseg + j, i2 * rseg + j, i2 * rseg + j2, i * rseg + j2))
    return make_object(name, verts, faces)


def recalc_normals(obj):
    """Orient face normals consistently outward (closed manifolds only)."""
    bm = bmesh.new()
    bm.from_mesh(obj.data)
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
    bm.to_mesh(obj.data)
    bm.free()
    return obj


def loft(name, rings, closed=True, cap_start=True, cap_end=True, flip=False, recalc=False):
    """Skin consecutive point rings (equal counts) into a mesh.

    With ``closed`` rings wrap around; outward normals assume counter-clockwise
    rings seen from the loft direction (use ``flip`` otherwise).
    """
    verts, idx = [], []
    for ring in rings:
        idx.append(list(range(len(verts), len(verts) + len(ring))))
        verts.extend(tuple(p) for p in ring)
    n = len(rings[0])
    faces = []
    span = n if closed else n - 1
    for i in range(len(rings) - 1):
        for j in range(span):
            j2 = (j + 1) % n
            quad = (idx[i][j], idx[i][j2], idx[i + 1][j2], idx[i + 1][j])
            faces.append(quad[::-1] if flip else quad)
    if closed:
        if cap_start:
            f = tuple(reversed(idx[0]))
            faces.append(f[::-1] if flip else f)
        if cap_end:
            f = tuple(idx[-1])
            faces.append(f[::-1] if flip else f)
    obj = make_object(name, verts, faces)
    if recalc:
        recalc_normals(obj)
    return obj


def sweep(name, points, outward, profile, closed_caps=True, recalc=True):
    """Sweep a (u, z) ``profile`` along ``points`` (Vectors incl. z); ``outward`` gives
    the direction of ``u`` at each point. Normals are re-oriented automatically."""
    rings = []
    for p, d in zip(points, outward):
        rings.append([(p.x + d.x * u, p.y + d.y * u, p.z + z) for u, z in profile])
    return loft(name, rings, closed=True, cap_start=closed_caps, cap_end=closed_caps, recalc=recalc)


def tube(name, pts, radius=0.05, seg=10, caps=True, squash=1.0, up=(0, 0, 1)):
    """Sweep a circular (or ellipse via ``squash``) section along a polyline.

    ``radius`` is a float or a per-point list. Parallel-transport frames keep
    the tube from twisting.
    """
    pts = [Vector(p) for p in pts]
    n = len(pts)
    radii = list(radius) if isinstance(radius, (list, tuple)) else [radius] * n
    tans = []
    for i in range(n):
        a, b = pts[max(0, i - 1)], pts[min(n - 1, i + 1)]
        tans.append((b - a).normalized())
    ref = Vector(up)
    if abs(tans[0].dot(ref)) > 0.95:
        ref = Vector((1, 0, 0))
    normal = tans[0].cross(ref).cross(tans[0]).normalized()
    rings = []
    for i in range(n):
        if i > 0:
            rot = tans[i - 1].rotation_difference(tans[i])
            normal = (rot @ normal).normalized()
        binorm = tans[i].cross(normal)
        ring = []
        for k in range(seg):
            a = math.tau * k / seg
            ring.append(pts[i] + normal * (math.cos(a) * radii[i]) + binorm * (math.sin(a) * radii[i] * squash))
        rings.append(ring)
    return loft(name, rings, closed=True, cap_start=caps, cap_end=caps)


def ribbon(name, spine, widths, normal_hint=(0, 0, 1), fold=0.0, across=4):
    """Flat strip following ``spine`` with a width per point and a V fold.

    ``fold`` lifts both edges relative to the spine (positive = up), which gives
    leaves and fronds their channel shape. Returns an open surface.
    """
    spine = [Vector(p) for p in spine]
    n = len(spine)
    hint = Vector(normal_hint)
    rings = []
    for i in range(n):
        a, b = spine[max(0, i - 1)], spine[min(n - 1, i + 1)]
        t = (b - a).normalized()
        side = t.cross(hint)
        if side.length < 1e-6:
            side = t.cross(Vector((1, 0, 0)))
        side.normalize()
        up = side.cross(t)
        ring = []
        for k in range(across + 1):
            s = -1 + 2 * k / across
            ring.append(spine[i] + side * (s * widths[i]) + up * (fold * s * s * widths[i]))
        rings.append(ring)
    return loft(name, rings, closed=False)


def blob(name, r, rng, subdiv=3, amp=0.22, freq=1.4, squash=(1, 1, 1), cuts=0,
         cut_depth=0.82, relax=2, flat_bottom=None):
    """Rounded rock: icosphere with seeded noise, optional planar cuts and relax."""
    bm = bmesh.new()
    bmesh.ops.create_icosphere(bm, subdivisions=subdiv, radius=1.0)
    off1 = Vector((rng.uniform(-50, 50), rng.uniform(-50, 50), rng.uniform(-50, 50)))
    off2 = Vector((rng.uniform(-50, 50), rng.uniform(-50, 50), rng.uniform(-50, 50)))
    planes = []
    for _ in range(cuts):
        d = Vector((rng.uniform(-1, 1), rng.uniform(-1, 1), rng.uniform(-0.6, 1))).normalized()
        planes.append((d, cut_depth * rng.uniform(0.85, 1.05)))
    for v in bm.verts:
        d = v.co.normalized()
        k = 1.0 + amp * noise.noise(d * freq + off1) + amp * 0.45 * noise.noise(d * freq * 2.7 + off2)
        p = d * k
        for pd, depth in planes:
            over = p.dot(pd) - depth
            if over > 0:
                p -= pd * over
        v.co = Vector((p.x * squash[0], p.y * squash[1], p.z * squash[2])) * r
    for _ in range(relax):
        bmesh.ops.smooth_vert(bm, verts=bm.verts, factor=0.5, use_axis_x=True, use_axis_y=True,
                              use_axis_z=True)
    if flat_bottom is not None:
        for v in bm.verts:
            v.co.z = max(v.co.z, -flat_bottom * r)
    return from_bmesh(name, bm)


def rounded_box(name, size, bevel="auto", segments=3, base=False, dens=0.12, subsurf=0):
    """Soft box: a plain box finished with bevel (and optional subdivision)."""
    return finish(box(name, size, base=base), bevel=bevel, segments=segments, subsurf=subsurf, dens=dens)


def plank(name, length, width, thickness, sag=0.0, cuts=6):
    """Board along Y (base at z=0) with an optional parabolic sag in metres at the middle."""
    rings = []
    for i in range(cuts + 1):
        t = i / cuts
        y, z = (t - 0.5) * length, -sag * 4 * t * (1 - t)
        rings.append([(-width / 2, y, z), (width / 2, y, z), (width / 2, y, z + thickness), (-width / 2, y, z + thickness)])
    return loft(name, rings, closed=True, recalc=True)


def barrel(name, r_end, r_belly, height, seg=24, rows=12, groove=0.972):
    """Barrel body (lathe with belly) whose even meridians are pulled in as stave grooves."""
    prof = [(0.0, 0.0)] + [(r_end + (r_belly - r_end) * math.sin(math.pi * k / rows) ** 0.85, height * k / rows)
                           for k in range(rows + 1)] + [(0.0, height)]
    body = lathe(name, prof, seg=seg)
    for v in body.data.vertices:
        r = math.hypot(v.co.x, v.co.y)
        if r > 1e-5 and stave_index(v.co.x, v.co.y, seg) % 2 == 0:
            v.co.x *= groove
            v.co.y *= groove
    return body


def stave_index(x, y, seg=24):
    return int(round((math.atan2(y, x) % math.tau) / (math.tau / seg))) % seg


def flame(name, r, h, seg=10):
    """Teardrop flame body (axis Z, pole at both ends)."""
    steps = 9
    prof = [(0.0, 0.0)]
    for k in range(1, steps):
        t = k / steps
        prof.append((r * math.sin(math.pi * t ** 0.7) ** 0.85 * (1 - 0.15 * t), h * t))
    prof.append((0.0, h))
    return lathe(name, prof, seg=seg)


def grid(name, nx, ny, sx, sy, height_fn=None):
    """Regular ``nx`` x ``ny`` cell grid centred on the origin (z from height_fn)."""
    verts, faces = [], []
    for j in range(ny + 1):
        for i in range(nx + 1):
            x = (i / nx - 0.5) * sx
            y = (j / ny - 0.5) * sy
            verts.append((x, y, height_fn(x, y) if height_fn else 0.0))
    for j in range(ny):
        for i in range(nx):
            a = j * (nx + 1) + i
            faces.append((a, a + 1, a + nx + 2, a + nx + 1))
    return make_object(name, verts, faces)


# --------------------------------------------------------------------------
# Transforms and statistics
# --------------------------------------------------------------------------

def matrix(loc=(0, 0, 0), rot=(0, 0, 0), scale=(1, 1, 1)):
    """TRS matrix; ``rot`` is XYZ Euler in DEGREES, ``scale`` float or triple."""
    if not isinstance(scale, (tuple, list)):
        scale = (scale, scale, scale)
    r = Euler([math.radians(a) for a in rot], "XYZ").to_matrix().to_4x4()
    s = Matrix.Diagonal((scale[0], scale[1], scale[2], 1.0))
    return Matrix.Translation(loc) @ r @ s


def transform(obj, loc=(0, 0, 0), rot=(0, 0, 0), scale=(1, 1, 1)):
    """Bake a TRS transform into the mesh data (object matrix stays identity)."""
    obj.data.transform(matrix(loc, rot, scale))
    return obj


def bounds(obj):
    cos = [v.co for v in obj.data.vertices]
    lo = Vector((min(v.x for v in cos), min(v.y for v in cos), min(v.z for v in cos)))
    hi = Vector((max(v.x for v in cos), max(v.y for v in cos), max(v.z for v in cos)))
    return lo, hi


def tri_count(obj):
    mesh = obj.data
    mesh.calc_loop_triangles()
    return len(mesh.loop_triangles)


# --------------------------------------------------------------------------
# finish(): bevel, subdivide, shade, weighted normals, apply
# --------------------------------------------------------------------------

def _apply_modifiers(obj):
    bpy.context.view_layer.update()
    dg = bpy.context.evaluated_depsgraph_get()
    evaluated = obj.evaluated_get(dg)
    new = bpy.data.meshes.new_from_object(evaluated, preserve_all_data_layers=True, depsgraph=dg)
    old = obj.data
    name = old.name
    obj.modifiers.clear()
    obj.data = new
    if old.users == 0:
        bpy.data.meshes.remove(old)
    new.name = name  # after the removal: while the old mesh lives, Blender would number the name (".001")


def densify(mesh, max_edge, passes=6):
    """Split edges longer than ``max_edge`` so vertex-baked AO has resolution."""
    bm = bmesh.new()
    bm.from_mesh(mesh)
    for _ in range(passes):
        long_edges = [e for e in bm.edges if e.calc_length() > max_edge]
        if not long_edges:
            break
        bmesh.ops.subdivide_edges(bm, edges=long_edges, cuts=1, use_grid_fill=True)
    bm.to_mesh(mesh)
    bm.free()


def smooth_by_angle(mesh, angle_deg):
    """Smooth-shade everything but edges sharper than ``angle_deg``."""
    angle = math.radians(angle_deg)
    bm = bmesh.new()
    bm.from_mesh(mesh)
    for f in bm.faces:
        f.smooth = True
    for e in bm.edges:
        e.smooth = True
        if len(e.link_faces) == 2 and e.calc_face_angle(0.0) > angle:
            e.smooth = False
    bm.to_mesh(mesh)
    bm.free()


def _auto_bevel(obj):
    lo, hi = bounds(obj)
    dims = sorted(d for d in (hi - lo) if d > 1e-3)
    if not dims:
        return 0.01
    return min(0.06, max(0.006, 0.09 * dims[0]))


def finish(obj, bevel="auto", segments=2, subsurf=0, weighted=True, dens=0.12,
           angle=30.0, smooth_angle=40.0, solidify=0.0):
    """Soften a raw builder result and apply the whole stack.

    * ``bevel`` width in metres (``"auto"`` = ~9 % of the smallest dimension,
      clamped to 6-60 mm, ``0`` disables), ``segments`` 2-3 round it.
    * ``subsurf`` Catmull-Clark levels for organic shapes (1-2).
    * ``dens`` maximum edge length after modifiers (0 disables); gives the
      vertex-baked ambient occlusion enough resolution on large flat parts.
    * ``weighted`` adds a Weighted Normal modifier (keep sharp) so large faces
      shade flat while bevels stay soft.
    * ``solidify`` thickens open surfaces (leaves, cloth) before bevelling.
    """
    mods = False
    if solidify:
        m = obj.modifiers.new("kit_solidify", "SOLIDIFY")
        m.thickness = solidify
        m.offset = 0.0
        mods = True
    if bevel == "auto":
        bevel = _auto_bevel(obj)
    if bevel:
        m = obj.modifiers.new("kit_bevel", "BEVEL")
        m.width = bevel
        m.segments = segments
        m.limit_method = "ANGLE"
        m.angle_limit = math.radians(angle)
        m.profile = 0.5
        m.use_clamp_overlap = True
        mods = True
    if subsurf:
        m = obj.modifiers.new("kit_subsurf", "SUBSURF")
        m.levels = subsurf
        m.render_levels = subsurf
        m.quality = 3
        mods = True
    if mods:
        _apply_modifiers(obj)
    if dens:
        densify(obj.data, dens)
    if subsurf and not bevel:
        smooth_by_angle(obj.data, 89.0)
    else:
        smooth_by_angle(obj.data, smooth_angle)
    if weighted:
        m = obj.modifiers.new("kit_wn", "WEIGHTED_NORMAL")
        m.mode = "FACE_AREA"
        m.weight = 50
        m.keep_sharp = True
        m.thresh = 0.01
        _apply_modifiers(obj)
    return obj


def join(name, objs):
    """Join objects into one (keeps colour attributes and material slots)."""
    objs = list(objs)
    first = objs[0]
    if len(objs) > 1:
        with bpy.context.temp_override(active_object=first, object=first, selected_objects=objs,
                                       selected_editable_objects=objs):
            bpy.ops.object.join()
    first.name = name
    first.data.name = name + "-mesh"
    return first
